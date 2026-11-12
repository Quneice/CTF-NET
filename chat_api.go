package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ctfnet/internal/aiagent"
	"ctfnet/internal/core"
)

// 对话式渗透（ai.pentest.chat）的 Wails 绑定：会话列表/详情 + 流式对话。
// 会话本体持久化于 <DataDir>/aiagent/chat/。
//
// SendChatMessage 为「实时对话」通道：调用立即返回 {session_id}，助手回复以
// Wails 事件流推送（前端 ChatPage 订阅，打字机效果）：
//   chat:delta         {session_id, content}     助手文本增量（逐 token）
//   chat:thinking      {session_id, content}     思考内容增量（reasoning，前台以弱化样式展示）
//   chat:tool          {session_id, name, summary} 工具开始执行
//   chat:tool_result   {session_id, name, summary, is_error} 工具返回摘要
//   chat:done          {session_id, reply, title, task_ids, msg_count} 回合完成
//   chat:error         {session_id, message}     回合失败
// 回合引擎原样照搬 ARTEX-1.0.4 的对话实现（norma agentcore.Session + transcript
// 续聊 + captureRun 事件合并，见 internal/aiagent/chat_norma.go）。
// 与工具 ai.pentest.chat 的差异：不走任务队列，无 job 轮询，纯事件流。
// 会话 busy 时发送新消息 = 打断：取消在跑回合（ctx）并等旧 goroutine 退出后再接管，
// 被打断回合不产 chat:error（context.Canceled 静默），新回合把两条用户消息一起送模型。

// chatRoot 会话持久化根目录。
func chatRoot() string { return filepath.Join(core.GetDataDir(), "aiagent", "chat") }

// chatTurn 正在运行的对话回合句柄（打断用：cancel 取消回合，done 在 goroutine 退出时关闭）。
type chatTurn struct {
	cancel context.CancelFunc
	done   chan struct{}
}

var chatTurns sync.Map // sessionID -> *chatTurn

// ListChatSessions 列出全部对话会话摘要（按更新时间倒序；前端「AI 对话」页用）。
func (a *App) ListChatSessions() ([]aiagent.ChatSessionSummary, error) {
	return aiagent.ListChatSessions(chatRoot())
}

// GetChatSession 读取指定会话（含消息历史）；不存在返回 nil。
func (a *App) GetChatSession(sessionID string) (*aiagent.ChatSession, error) {
	if sessionID == "" {
		return nil, nil
	}
	return aiagent.LoadChatSession(chatRoot(), sessionID)
}

// SendChatMessage 发送一条对话消息并立即返回 {session_id}；助手回复以 chat:* 事件流
// 推送（见文件头注释）。enableExec 控制助手是否可用本地命令执行工具。
// 若该会话已有回合在跑（busy），先取消旧回合、等其退出后再接管 —— 支持
// 「回复进行中打断并补充说明」：被打断回合的已生成分段保留在历史里，
// 新消息追加在同一个会话上（模型上下文经 transcript 恢复两条用户消息）。
func (a *App) SendChatMessage(sessionID, message string, enableExec bool) (map[string]any, error) {
	if strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("缺少必需参数: message")
	}
	ac := core.GetAppConfig()
	cfg := aiagent.DefaultConfig()
	cfg.APIKey = ac.Ai.APIKey
	cfg.BaseURL = ac.Ai.BaseURL
	cfg.Model = ac.Ai.Model
	cfg.ReasoningEffort = ac.Ai.ReasoningEffort
	cfg.MaxTurns = ac.Ai.MaxTurns
	cfg.MaxWorkers = ac.Ai.MaxWorkers
	cfg.PlanRounds = ac.Ai.PlanRounds
	cfg.Timeout = ac.Ai.Timeout
	cfg.EnableExec = enableExec

	if sessionID == "" {
		sessionID = aiagent.NewChatSessionID()
	}
	if !aiagent.AcquireChatLock(sessionID) {
		// 已有回合在跑：打断它，等旧 goroutine 完全退出（会话/转录落盘完成）后再接管
		if t, ok := chatTurns.Load(sessionID); ok {
			t.(*chatTurn).cancel()
			select {
			case <-t.(*chatTurn).done:
			case <-time.After(10 * time.Second):
				return nil, fmt.Errorf("会话 %s 正在回复中，打断超时，请稍候再试", sessionID)
			}
		}
		if !aiagent.AcquireChatLock(sessionID) {
			return nil, fmt.Errorf("会话 %s 正在回复中，请稍候再发消息", sessionID)
		}
	}

	root := chatRoot()
	s, board, err := aiagent.OpenChatSession(root, sessionID)
	if err != nil {
		aiagent.ReleaseChatLock(sessionID)
		return nil, err
	}

	var execFunc aiagent.ExecFunc
	if enableExec {
		execFunc = aiagent.ExecWithProxyEnv(chatExecFunc(a.Tasks.Executor, filepath.Join(root, sessionID, "board")), ac.Proxy.URL)
	}
	env := &aiagent.ChatEnv{
		Cfg:         cfg,
		ProxyURL:    ac.Proxy.URL,
		RecipesDir:  filepath.Join(core.GetDataDir(), "aiagent", "recipes"),
		Exec:        execFunc,
		TaskRoot:    filepath.Join(core.GetDataDir(), "aiagent", "tasks"),
		Session:     s,
		SessionRoot: root,
	}
	appCtx := a.ctx

	turnCtx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	turn := &chatTurn{cancel: cancel, done: make(chan struct{})}
	chatTurns.Store(sessionID, turn)
	go func() {
		defer close(turn.done)
		defer aiagent.ReleaseChatLock(sessionID)
		defer cancel()
		defer func() {
			// 仅当映射里还是本回合时才删除（防误删接管后的新回合句柄）
			if cur, ok := chatTurns.Load(sessionID); ok && cur == turn {
				chatTurns.Delete(sessionID)
			}
		}()
		emit := func(kind string, payload map[string]any) {
			payload["session_id"] = sessionID
			if appCtx != nil {
				wailsRuntime.EventsEmit(appCtx, "chat:"+kind, payload)
			}
		}
		res, err := aiagent.RunChatTurn(turnCtx, env, s, board, message, cfg.MaxTurns, emit)
		if err != nil {
			// 被新消息打断（context.Canceled）不发错误事件：前端已自行固化为「已打断」
			if !errors.Is(err, context.Canceled) {
				emit("error", map[string]any{"message": err.Error()})
			}
			return
		}
		emit("done", map[string]any{
			"reply":     res.Reply,
			"title":     s.Title,
			"task_ids":  s.TaskIDs,
			"msg_count": len(s.Messages),
		})
	}()
	return map[string]any{"session_id": sessionID}, nil
}

// chatExecFunc 把 core.Executor 适配成 aiagent.ExecFunc（与 tools/ai 的 makeExecFunc
// 同语义：Windows 走 aiagent.RunWindowsCmd（cmd /S /C 原生命令行，防引号破坏），
// 其余走 sh -c）。
func chatExecFunc(exec core.Executor, fallbackDir string) aiagent.ExecFunc {
	return func(ctx context.Context, command string, dir string, timeoutSec int) (string, string, error) {
		if runtime.GOOS == "windows" {
			return aiagent.RunWindowsCmd(ctx, command, dir, timeoutSec)
		}
		w := dir
		if w == "" {
			w = fallbackDir
		}
		res, err := exec.Run(ctx, []string{"sh", "-c", command}, w, time.Duration(timeoutSec)*time.Second, nil)
		if err != nil {
			return "", "", err
		}
		if res.Status == core.StatusError {
			return res.Stdout, res.Stderr, fmt.Errorf("exit %d: %s", res.ExitCode, res.Stderr)
		}
		if res.Status == core.StatusTimeout {
			return res.Stdout, res.Stderr, fmt.Errorf("%s", res.Stderr)
		}
		return res.Stdout, res.Stderr, nil
	}
}

// DeleteChatSession 删除会话（软删除：整目录移入 backups/trash/chat，可手动找回）。
func (a *App) DeleteChatSession(sessionID string) error {
	if sessionID == "" {
		return nil
	}
	src := filepath.Join(chatRoot(), sessionID)
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}
	dataDir := core.GetDataDir()
	if a.Cfg != nil && a.Cfg.DataDir != "" {
		dataDir = a.Cfg.DataDir
	}
	trash := filepath.Join(dataDir, "backups", "trash", "chat")
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(trash, sessionID+"-"+time.Now().Format("20060102-150405"))
	return os.Rename(src, dst)
}
