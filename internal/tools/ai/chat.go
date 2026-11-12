// ai.pentest.chat —— 对话式渗透测试（ChatAgent，原生引擎 internal/aiagent/chat.go）。
//
// 设计对齐 ARTEX v1.0.4 的对话面（ChatAgent 直接持有侦察/执行工具、可发起完整
// 渗透任务），本实现原生重写：
//   - 每次工具运行 = 一轮对话（新消息 → 工具循环 → 助手回复落盘）
//   - 会话持久化在 <DataDir>/aiagent/chat/<session_id>/（session.json 消息历史 +
//     board/task.json 事实/资产/漏洞黑榜），session_id 传空 = 新会话
//   - 助手可用 start_task 发起后台自动化任务（aiagent.StartBackgroundTask，
//     与对话生命周期解耦），task_status 实时查询
//   - 同一会话同时只跑一轮（busy 锁，防并发交错）
package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ctfnet/internal/aiagent"
	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&AiChatTool{BaseTool: core.BaseTool{
		NameAttr:        "chat",
		CategoryAttr:    core.Category("ai.pentest"),
		VersionAttr:     "1.0.0",
		DescriptionAttr: "对话式渗透测试：与 AI 助手对话推进渗透（直接侦察/验证工具 + 可发起后台自动化任务并查询进展；会话历史持久化）",
		TimeoutAttr:     1800,
		ParamsAttr: []core.ParamSpec{
			{Name: "message", Type: "text", Required: true, Description: "发给 AI 助手的消息（目标描述/指令/追问均可）"},
			{Name: "session_id", Type: "str", Required: false, Description: "会话 ID（空 = 新会话；结果 data.session_id 返回新 ID，后续消息带上即可续聊）"},
			{Name: "enable_exec", Type: "bool", Required: false, Description: "是否允许助手执行本地命令（默认取设置页）"},
			{Name: "max_turns", Type: "int", Required: false, Description: "本轮最大工具往返数（默认取设置页）"},
			{Name: "api_key", Type: "password", Required: false, Description: "LLM API Key（默认取设置页 AI 配置）"},
			{Name: "base_url", Type: "str", Required: false, Description: "API 地址（默认取设置页）"},
			{Name: "model", Type: "str", Required: false, Description: "模型名（默认取设置页）"},
		},
		Runner: runAiChat,
	}})
}

// AiChatTool 对话式渗透测试工具。
type AiChatTool struct {
	core.BaseTool
}

// chatTurnTimeout 单轮对话的最大时长（LLM 调用有独立超时，此为整体兜底；
// 与 chat_api.go 的 25 分钟一致，给 48 轮工具往返留足墙钟预算）。
const chatTurnTimeout = 25 * time.Minute

func runAiChat(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	start := time.Now()
	ac := core.GetAppConfig()

	message := paramStr(params, "message", "")
	if strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("缺少必需参数: message")
	}

	cfg := aiagent.DefaultConfig()
	cfg.APIKey = paramStr(params, "api_key", ac.Ai.APIKey)
	cfg.BaseURL = paramStr(params, "base_url", ac.Ai.BaseURL)
	cfg.Model = paramStr(params, "model", ac.Ai.Model)
	cfg.ReasoningEffort = ac.Ai.ReasoningEffort
	cfg.MaxTurns = paramInt(params, "max_turns", ac.Ai.MaxTurns)
	cfg.MaxWorkers = ac.Ai.MaxWorkers
	cfg.PlanRounds = ac.Ai.PlanRounds
	cfg.Timeout = ac.Ai.Timeout
	cfg.EnableExec = paramBool(params, "enable_exec", ac.Ai.EnableExec)

	chatRoot := filepath.Join(core.GetDataDir(), "aiagent", "chat")
	taskRoot := filepath.Join(core.GetDataDir(), "aiagent", "tasks")

	// 会话：空 ID = 新会话
	sessionID := paramStr(params, "session_id", "")
	if sessionID == "" {
		sessionID = aiagent.NewChatSessionID()
	}
	if !aiagent.AcquireChatLock(sessionID) {
		return nil, fmt.Errorf("会话 %s 正在回复中，请稍候再发消息", sessionID)
	}
	defer aiagent.ReleaseChatLock(sessionID)

	s, board, err := aiagent.OpenChatSession(chatRoot, sessionID)
	if err != nil {
		return nil, err
	}
	boardDir := filepath.Join(chatRoot, sessionID, "board")

	var execFunc aiagent.ExecFunc
	if cfg.EnableExec {
		execFunc = aiagent.ExecWithProxyEnv(makeExecFunc(tc.Executor, boardDir), ac.Proxy.URL)
	}

	env := &aiagent.ChatEnv{
		Cfg: cfg, ProxyURL: ac.Proxy.URL,
		RecipesDir:  filepath.Join(core.GetDataDir(), "aiagent", "recipes"),
		Exec:        execFunc,
		TaskRoot:    taskRoot,
		Session:     s,
		SessionRoot: chatRoot,
	}

	if tc.ReportProgress != nil {
		tc.ReportProgress(5, "AI 助手思考中…")
	}
	turnCtx, cancel := context.WithTimeout(ctx, chatTurnTimeout)
	defer cancel()
	res, err := aiagent.RunChatTurn(turnCtx, env, s, board, message, cfg.MaxTurns, nil)
	if err != nil {
		return nil, fmt.Errorf("对话失败: %w", err)
	}
	if tc.ReportProgress != nil {
		tc.ReportProgress(100, "已回复")
	}

	data := map[string]any{
		"session_id": res.SessionID,
		"reply":      res.Reply,
		"title":      s.Title,
		"task_ids":   s.TaskIDs,
		"msg_count":  len(s.Messages),
	}
	if tc.Workdir != "" {
		// 任务工作目录里留一份会话快照便于复盘
		_ = os.MkdirAll(tc.Workdir, 0o755)
		_ = os.WriteFile(filepath.Join(tc.Workdir, "chat-"+sessionID+".txt"),
			[]byte("会话: "+sessionID+"\n\n"+strings.Join(chatLines(s.Messages), "\n")), 0o600)
	}

	return &core.ToolResult{
		Status:   core.StatusSuccess,
		ExitCode: 0,
		Stdout:   res.Reply,
		Duration: time.Since(start).Seconds(),
		Data:     data,
	}, nil
}

func chatLines(msgs []aiagent.ChatMsg) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		who := "我"
		if m.Role == "assistant" {
			who = "AI"
		}
		out = append(out, fmt.Sprintf("[%s] %s\n%s\n", who, m.TS, m.Content))
	}
	return out
}
