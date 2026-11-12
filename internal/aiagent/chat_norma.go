package aiagent

// —— 对话回合引擎：原样照搬 ARTEX-1.0.4 的对话实现方式 ——
//
// ARTEX 的对话面（agent/chat.go ChatAgent + agent/capture.go captureRunSession）
// 跑在 norma SDK 的 agentcore.Session 上：
//   - 每个用户回合 = 新建 Session → transcript JSONL 恢复历史（Resume）→ Prompt
//     事件流（text/thinking/tool_use/tool_result/usage/result）→ 事件合并成活动记录
//   - transcript.Store 把每条消息出生即落盘（<dir>/<sessionID>.jsonl），冷启动
//     Resume 即恢复多轮上下文；思考内容 reasoning_content 由 norma 的 llm 包
//     原样回传（DeepSeek 思考模式多轮续传的硬要求）
//
// 本文件逐段照搬上述实现，仅做三处 CTF-NET 接线（均已注释标注）：
//   1) llm.Provider 由设置页 AI 配置构建（OpenAI 兼容协议 + 全局代理）；
//   2) 工具集用 CTF-NET 自己的侦察/沉淀工具（http_request/exec_command/
//      add_asset/record_fact/report_finding/start_task/task_status 等），
//      经 normaTool 适配器挂到 SDK；
//   3) 活动记录映射为轮次分段 ChatSeg（随会话持久化，前端按轮渲染）+ chat:*
//      前端事件（delta/thinking/tool/tool_result）。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Autumn-27/norma/agentcore"
	"github.com/Autumn-27/norma/harness"
	"github.com/Autumn-27/norma/llm"
	"github.com/Autumn-27/norma/permission"
	normatool "github.com/Autumn-27/norma/tool"
	"github.com/Autumn-27/norma/transcript"
)

// chatProvider 从 CTF-NET AI 配置构建 norma 的 OpenAI 兼容 Provider。
// （接线 1：ARTEX 的 Config.NewProvider 做同样的事，这里只取 OpenAI 分支。）
func chatProvider(cfg Config, proxyURL string) (llm.Provider, error) {
	c := cfg.normalize()
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	// 容忍完整 endpoint：provider 自己追加 /chat/completions
	base = strings.TrimRight(strings.TrimSuffix(base, "/chat/completions"), "/")
	// CTF-NET 旧约定：地址不带版本段（/v1、/v4 等）时自动补 /v1（旧 llm.endpoint() 行为）
	if base != "" && !hasVersionSuffix(base) {
		base += "/v1"
	}
	return llm.NewProvider(llm.Config{
		Format:          llm.FormatOpenAI,
		BaseURL:         base,
		APIKey:          c.APIKey,
		Model:           c.Model,
		Proxy:           proxyURL,
		ReasoningEffort: c.ReasoningEffort, // 智谱 glm-5.x 始终思考模型省 token
	})
}

// normaTool 把 aiagent 内置工具适配为 norma 的 tool.CoreTool。
// （接线 2：ARTEX 的 ChatAgent 用 SDK DefaultTools + 技能/MCP；CTF-NET 保留自己的
// 侦察/沉淀工具集，行为与旧对话引擎完全一致。）
type normaTool struct {
	t     Tool
	board *Task
	exec  ExecFunc
}

func (a *normaTool) Name() string                           { return a.t.Name() }
func (a *normaTool) Description() string                    { return a.t.Description() }
func (a *normaTool) Prompt() string                         { return a.t.Description() }
func (a *normaTool) InputSchema() map[string]any            { return a.t.InputSchema() }
func (a *normaTool) IsReadOnly(json.RawMessage) bool        { return false }
func (a *normaTool) IsConcurrencySafe(json.RawMessage) bool { return true }
func (a *normaTool) CheckPermissions(context.Context, json.RawMessage, permission.Context) permission.Decision {
	return permission.Allowed()
}

// Call 执行工具：参数解析 → 原生 Tool.Call → norma 结果（错误作为工具结果回传模型自纠）。
func (a *normaTool) Call(ctx context.Context, input json.RawMessage, tc *normatool.ToolContext) (normatool.Result, error) {
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return normatool.Errorf("工具参数解析失败: " + err.Error()), nil
	}
	rc := &RunContext{Task: a.board, WorkDir: tc.WorkingDir, Agent: "assistant", Exec: a.exec}
	out, err := a.t.Call(ctx, args, rc)
	if err != nil {
		return normatool.Errorf("工具错误: " + err.Error()), nil
	}
	return normatool.Text(out), nil
}

// ChatEmitter 事件发射器（chat:* 前端事件流）。payload 由调用方补齐 session_id。
type ChatEmitter func(kind string, payload map[string]any)

// chatActivity 对话回合的一条活动记录（ARTEX db.Activity 的字段子集）。
type chatActivity struct {
	Kind      string // text | thinking | tool_use | tool_result | usage | result
	Tool      string
	ToolUseID string
	IsError   bool
	Summary   string
	Detail    string
}

// captureChatRun 驱动一个用户回合到完成并产出合并后的活动记录流 —— 原样照搬
// ARTEX agent/capture.go 的 captureRunSession：KindText/KindThinking 到达时是流式
// 增量（一个事件一段），连续同种增量合并成单条记录；KindResult 携带最终文本，
// 缓冲尾文本与它重复时丢弃（终答只由 result 行携带）。
// （与 ARTEX 的差异仅两处：审计 trace（intercept）与 sidequestion 属 ARTEX 审批流，
// CTF-NET 没有，已删；其余逻辑逐行保留。）
func captureChatRun(ctx context.Context, s *agentcore.Session, input string, emit func(chatActivity)) (string, error) {
	var reason harness.TerminalReason
	rec := func(a chatActivity) {
		if emit != nil {
			emit(a)
		}
	}
	toolNames := map[string]string{} // tool_use id -> name, to label results

	var tbuf strings.Builder
	var tkind string
	flush := func() {
		if tbuf.Len() == 0 {
			return
		}
		text := strings.TrimSpace(tbuf.String())
		k := tkind
		tbuf.Reset()
		tkind = ""
		if text != "" {
			rec(chatActivity{Kind: k, Summary: firstLine(text, 200), Detail: text})
		}
	}
	addDelta := func(kind, text string) {
		if text == "" {
			return
		}
		if tkind != "" && tkind != kind {
			flush()
		}
		tkind = kind
		tbuf.WriteString(text)
	}
	lastTool := &runTrace{startedAt: time.Now()}
	var lastUsage *llm.Usage

	var finalText string
	var rerr error
	for ev, err := range s.Prompt(ctx, input) {
		if err != nil {
			flush()
			if ctx.Err() != nil { // engine/user cancellation, not a provider failure
				sum, detail := terminalText(ctx, &harness.Terminal{Reason: reason, Err: ctx.Err()}, lastTool)
				rec(activityWithUsage(chatActivity{Kind: "result", Summary: firstLine(sum, 400), Detail: detail}, lastUsage))
				return finalText, ctx.Err()
			}
			rec(activityWithUsage(chatActivity{Kind: "result", IsError: true, Summary: "执行出错: " + err.Error(), Detail: err.Error()}, lastUsage))
			return finalText, err
		}
		switch ev.Kind {
		case harness.KindToolUse:
			if ev.ToolUse == nil {
				continue
			}
			flush()
			toolNames[ev.ToolUse.ID] = ev.ToolUse.Name
			in := string(ev.ToolUse.Input)
			lastTool.start(ev.ToolUse.ID, ev.ToolUse.Name, in)
			rec(chatActivity{Kind: "tool_use", Tool: ev.ToolUse.Name, ToolUseID: ev.ToolUse.ID,
				Summary: ev.ToolUse.Name + " " + firstLine(in, 200), Detail: in})
		case harness.KindToolResult:
			if ev.ToolResult == nil {
				continue
			}
			flush()
			out := blocksText(ev.ToolResult.Content)
			lastTool.done(ev.ToolResult.ToolUseID)
			rec(chatActivity{Kind: "tool_result", Tool: toolNames[ev.ToolResult.ToolUseID], ToolUseID: ev.ToolResult.ToolUseID,
				IsError: ev.ToolResult.IsError, Summary: firstLine(out, 200), Detail: out})
		case harness.KindText:
			addDelta("text", ev.Text)
		case harness.KindThinking:
			addDelta("thinking", ev.Text)
		case harness.KindUsage:
			if ev.Usage != nil {
				u := *ev.Usage
				lastUsage = &u
				rec(chatActivity{Kind: "usage"})
			}
		case harness.KindResult:
			if ev.Terminal != nil {
				finalText = ev.Terminal.Text
				reason = ev.Terminal.Reason
				// the buffered tail text usually equals Terminal.Text (final answer);
				// drop it to avoid a duplicate record, the result row carries it.
				if tkind == "text" && strings.TrimSpace(tbuf.String()) == strings.TrimSpace(ev.Terminal.Text) {
					tbuf.Reset()
					tkind = ""
				}
				flush() // flush any trailing thinking / non-final text
				sum, detail := ev.Terminal.Text, ev.Terminal.Text
				if sum == "" || ev.Terminal.Reason == harness.ReasonAbortedTools || ev.Terminal.Reason == harness.ReasonAbortedStreaming {
					sum, detail = terminalText(ctx, ev.Terminal, lastTool)
				}
				rec(chatActivity{Kind: "result", IsError: ev.Terminal.Err != nil,
					Summary: firstLine(sum, 400), Detail: detail})
				if ev.Terminal.Err != nil {
					rerr = ev.Terminal.Err
				}
			}
		}
	}
	flush() // safety: any unflushed text if the stream ended without KindResult
	return finalText, rerr
}

func activityWithUsage(a chatActivity, usage *llm.Usage) chatActivity {
	_ = usage // CTF-NET 前端暂不展示 token 计数；保留参数位与原实现一致
	return a
}

// blocksText concatenates the text of a tool-result's content blocks.
func blocksText(blocks []llm.ContentBlock) string {
	var b strings.Builder
	for _, bl := range blocks {
		if bl.Type == llm.BlockText && bl.Text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(bl.Text)
		}
	}
	return b.String()
}

// firstLine returns a single-line, rune-capped preview for the summary column.
func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if before, _, found := strings.Cut(s, "\n"); found {
		s = before
	}
	if utf8RuneCount(s) > max {
		s = string([]rune(s)[:max]) + "…"
	}
	return s
}

// utf8RuneCount 与 unicode/utf8.RuneCountInString 等价（保持导入最小化）。
func utf8RuneCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// runTrace retains the latest tool call so an interrupted run can identify the
// operation that was still in flight.（原样照搬 ARTEX agent/terminalreason.go）
type runTrace struct {
	startedAt time.Time
	id        string
	name      string
	input     string
	at        time.Time
	pending   bool
}

func (t *runTrace) start(id, name, input string) {
	t.id, t.name, t.input, t.at, t.pending = id, name, input, time.Now(), true
}

func (t *runTrace) done(id string) {
	if id == t.id {
		t.pending = false
	}
}

var reasonHint = map[harness.TerminalReason]string{
	harness.ReasonCompleted:         "模型正常结束了本轮，但没有留下文字总结；事实和资产以本轮工具调用记录为准",
	harness.ReasonMaxTurns:          "达到步数上限(MaxTurns)：SDK 已执行收尾并写回事实和资产，意图会标记为 exhausted，供规划者换方向继续，而不是作为失败处理",
	harness.ReasonTimeout:           "达到单次运行的墙钟预算(MaxDuration)：到点会打断在跑的工具并就地进收尾，把已识别的事实和资产写回，意图会标记为 exhausted",
	harness.ReasonModelError:        "模型或 API 调用失败（网络、鉴权、限流、供应商 5xx 等），重试用尽后意图标记为 blocked——传输层故障导致这条意图基本没真正探成；查其执行过程（get_worker_trace）后再决定重派或换法",
	harness.ReasonBlockingLimit:     "上下文长度达到硬上限，请求在发出前被拦截；应收窄意图粒度或压缩工具返回",
	harness.ReasonPromptTooLong:     "提示词过长且上下文压缩重试已经用尽，无法继续执行",
	harness.ReasonImageError:        "当前模型不支持本轮多模态内容；请切换支持视觉的模型或避免工具返回图片",
	harness.ReasonStopHookPrevented: "Stop 钩子阻止本轮结束，随后未能继续；请检查任务 Guard 规则是否过严",
	harness.ReasonHookStopped:       "工具或钩子主动停止继续执行，例如越界目标或禁用命令；请检查最后一条 tool_result 的拦截说明",
	harness.ReasonAbortedStreaming:  "运行在模型输出流式生成阶段被取消",
	harness.ReasonAbortedTools:      "运行在工具执行阶段被取消",
}

// terminalText renders a terminal event with no final text into a compact summary
// and a Markdown detail block.（原样照搬 ARTEX agent/terminalreason.go；AbortReason
// 取消原因提取依赖 ARTEX 的 cancelcause 机制，此处用 ctx.Err() 等价替代。）
func terminalText(ctx context.Context, term *harness.Terminal, tr *runTrace) (string, string) {
	reason := term.Reason
	aborted := reason == harness.ReasonAbortedStreaming || reason == harness.ReasonAbortedTools
	// Prompt may return ctx.Err directly without a terminal event. Preserve the
	// cancellation cause instead of falling back to an empty/unknown terminal reason.
	if reason == "" && ctx.Err() != nil {
		aborted = true
	}

	var sum string
	if aborted {
		short := "上下文取消"
		if ctx.Err() != nil {
			short = ctx.Err().Error()
		}
		stage := "执行过程中"
		switch reason {
		case harness.ReasonAbortedStreaming:
			stage = "模型输出阶段"
		case harness.ReasonAbortedTools:
			stage = "工具执行阶段"
		}
		sum = "（运行被中断：" + short + "；停在" + stage + progressSuffix(term, tr) + "，未完成）"
	} else if reason == harness.ReasonMaxTurns || reason == harness.ReasonTimeout {
		sum = "（达到运行预算上限(" + string(reason) + ")，已收尾写回事实" + progressSuffix(term, tr) + "；本次无文字总结）"
	} else {
		hint := terminalReasonHint(reason)
		sum = "（无文字总结，终态 " + terminalReasonLabel(reason) + "：" + firstLine(hint, 80) + "）"
	}

	var b strings.Builder
	b.WriteString(sum)
	b.WriteString("\n\n")
	displayReason := terminalReasonLabel(reason)
	fmt.Fprintf(&b, "- **终态**: `%s` - %s\n", displayReason, terminalReasonHint(reason))
	if aborted && ctx.Err() != nil {
		fmt.Fprintf(&b, "- **中断原因**: %s\n", ctx.Err().Error())
	}
	if term.Err != nil {
		fmt.Fprintf(&b, "- **底层错误**: `%v`\n", term.Err)
	}
	if aborted && strings.TrimSpace(term.Text) != "" {
		b.WriteString("- **取消前已生成的部分输出**:\n\n")
		b.WriteString(term.Text)
		b.WriteString("\n\n")
	}
	if term.Turns > 0 {
		fmt.Fprintf(&b, "- **已执行**: %d 轮模型回合\n", term.Turns)
	}
	if !tr.startedAt.IsZero() {
		fmt.Fprintf(&b, "- **本次运行耗时**: %s\n", roundDur(time.Since(tr.startedAt)))
	}
	if u := term.Usage; u.InputTokens+u.OutputTokens+u.CacheReadTokens+u.CacheWriteTokens > 0 {
		fmt.Fprintf(&b, "- **累计 token**: 输入 %d / 输出 %d / 缓存读 %d / 缓存写 %d\n",
			u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens)
	}
	if tr.name == "" {
		b.WriteString("- **工具调用**: 本次运行还没有发出工具调用就结束了\n")
	} else if tr.pending {
		fmt.Fprintf(&b, "- **中断时正在执行的工具**: `%s`（已运行 %s，**未返回结果**）\n\n  ```json\n  %s\n  ```\n",
			tr.name, roundDur(time.Since(tr.at)), firstLine(tr.input, 300))
	} else {
		fmt.Fprintf(&b, "- **中断前最后一个工具**: `%s`（已正常返回）\n", tr.name)
	}
	return sum, b.String()
}

func terminalReasonLabel(reason harness.TerminalReason) string {
	if reason == "" {
		return "context_canceled"
	}
	return string(reason)
}

func terminalReasonHint(reason harness.TerminalReason) string {
	if hint := reasonHint[reason]; hint != "" {
		return hint
	}
	if reason == "" {
		return "运行的 context 已取消，但底层没有产生 Terminal 事件"
	}
	return "未知终态；harness 可能新增了 TerminalReason，请补充 reasonHint"
}

func progressSuffix(term *harness.Terminal, tr *runTrace) string {
	var parts []string
	if term.Turns > 0 {
		parts = append(parts, fmt.Sprintf("%d 轮", term.Turns))
	}
	if !tr.startedAt.IsZero() {
		parts = append(parts, roundDur(time.Since(tr.startedAt)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "，已运行 " + strings.Join(parts, " / ")
}

func roundDur(d time.Duration) string {
	switch {
	case d < time.Minute:
		return d.Round(100 * time.Millisecond).String()
	case d < time.Hour:
		return d.Round(time.Second).String()
	default:
		return d.Round(time.Minute).String()
	}
}

// RunChatTurn 跑一个用户回合（ARTEX ChatAgent.Chat 的 CTF-NET 接线版）：
// 每次调用 = 新建 agentcore.Session → Resume(sessionID) 恢复历史 → Prompt 流式循环
// → 活动记录映射为 chat:* 事件。emit 非 nil 时实时回传 delta/thinking/tool/
// tool_result（SSE 打字机效果），nil 时非流式静默跑完只取回复。
//
// 历史恢复走 transcript JSONL（<SessionRoot>/<id>/<id>.jsonl，随会话目录整体
// 备份/删除）；session.json 的 Messages 仅作展示。
func RunChatTurn(ctx context.Context, env *ChatEnv, s *ChatSession, board *Task,
	userMsg string, maxTurns int, emit ChatEmitter) (*ChatTurnResult, error) {

	if strings.TrimSpace(userMsg) == "" {
		return nil, fmt.Errorf("消息为空")
	}
	if s.Title == "" {
		s.Title = titleOf(userMsg)
	}
	s.Messages = append(s.Messages, ChatMsg{Role: "user", Content: userMsg, TS: nowStr()})
	if err := SaveChatSession(env.SessionRoot, s); err != nil {
		return nil, fmt.Errorf("保存会话失败: %w", err)
	}

	if strings.TrimSpace(env.Cfg.APIKey) == "" {
		return nil, fmt.Errorf("未配置 LLM API Key（设置页 → AI 配置）")
	}
	prov, err := chatProvider(env.Cfg, env.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("构建 LLM 客户端失败: %w", err)
	}
	reg, err := chatRegistry(env)
	if err != nil {
		return nil, err
	}
	tools := make([]normatool.CoreTool, 0, len(reg.tools))
	for _, t := range reg.tools {
		tools = append(tools, &normaTool{t: t, board: board, exec: env.Exec})
	}
	// 会话工作目录：黑榜目录（工具 exec 的默认 CWD，与旧行为一致）
	sessionWorkDir := board.dir
	if err := os.MkdirAll(sessionWorkDir, 0o755); err != nil {
		return nil, err
	}
	// 转录：<SessionRoot>/<id>/<id>.jsonl（ARTEX transcript.Store 布局）
	txStore := transcript.NewStore(filepath.Join(env.SessionRoot, s.ID))
	opts := agentcore.Options{
		Provider:       prov,
		SystemPrompt:   []string{chatSystemPrompt()},
		Tools:          tools,
		PermissionMode: permission.ModeBypass,
		WorkingDir:     sessionWorkDir,
		MaxTurns:       maxTurns,
		Transcript:     txStore,
		SessionID:      s.ID,
		NonStreaming:   emit == nil,
		// 步数上限/时间预算到点时做收尾：注入提示让模型用最后一步写结论，
		// 否则 SDK 以空终答结束（前端只能显示「(模型未返回内容)」占位符）
		Settlement: &harness.Settlement{
			Prompt:   chatSettlePrompt(),
			MaxTurns: 1,
		},
		// 后台任务走 CTF-NET 自己的 start_task/task_status，不注入 SDK 的任务工具
		DisableBackgroundTasks: true,
	}
	sess := agentcore.NewSession(opts)
	defer sess.Close()
	// reload prior conversation so the agent has context across turns (each turn is
	// a fresh session). First turn: no file yet → Resume loads nothing and proceeds.
	_ = sess.Resume(s.ID)

	// 接线 3：活动记录 → 轮次分段（持久化到会话历史）+ chat:* 前端事件
	var segs []ChatSeg
	var terminalSummary string // 终态说明（步数上限/超时/取消等无文字终答时作兜底回复）
	finalText, err := captureChatRun(ctx, sess, userMsg, func(a chatActivity) {
		var seg *ChatSeg
		switch a.Kind {
		case "text":
			seg = &ChatSeg{Kind: "text", Text: a.Detail}
		case "thinking":
			seg = &ChatSeg{Kind: "thinking", Text: a.Detail}
		case "tool_use":
			sum := a.Summary
			// captureChatRun 的 summary = "工具名 参数摘要"；前端自绘工具名，这里摘掉
			if strings.HasPrefix(sum, a.Tool+" ") {
				sum = strings.TrimPrefix(sum, a.Tool+" ")
			}
			seg = &ChatSeg{Kind: "tool", Name: a.Tool, Summary: sum}
		case "tool_result":
			seg = &ChatSeg{Kind: "tool_result", Name: a.Tool, Summary: a.Summary, IsError: a.IsError}
		case "result":
			terminalSummary = a.Summary
		}
		if seg != nil {
			segs = append(segs, *seg)
		}
		if emit == nil {
			return
		}
		payload := map[string]any{}
		switch a.Kind {
		case "text":
			payload["content"] = a.Detail
			emit("delta", payload)
		case "thinking":
			payload["content"] = a.Detail
			emit("thinking", payload)
		case "tool_use":
			payload["name"] = a.Tool
			payload["summary"] = seg.Summary
			emit("tool", payload)
		case "tool_result":
			payload["name"] = a.Tool
			payload["summary"] = a.Summary
			payload["is_error"] = a.IsError
			emit("tool_result", payload)
		}
	})
	if err != nil {
		return nil, err
	}
	reply := strings.TrimSpace(finalText)
	if reply == "" {
		// 无文字终答（步数上限/超时/被取消等）：用终态说明兜底，而不是干巴巴的占位符
		if s := strings.TrimSpace(terminalSummary); s != "" {
			reply = s
		} else {
			reply = "(模型未返回内容)"
		}
	}
	// 终答作为最后一个叙述分段；captureChatRun 已去重，这里再兜一层防重复
	if n := len(segs); n > 0 && segs[n-1].Kind == "text" && strings.TrimSpace(segs[n-1].Text) == reply {
		// 终答与末尾叙述分段一致，不重复追加
	} else {
		segs = append(segs, ChatSeg{Kind: "text", Text: reply})
	}
	s.Messages = append(s.Messages, ChatMsg{Role: "assistant", Content: reply, Segments: segs, TS: nowStr()})
	if err := SaveChatSession(env.SessionRoot, s); err != nil {
		return nil, fmt.Errorf("保存会话失败: %w", err)
	}
	board.SaveNow()
	return &ChatTurnResult{Reply: reply, SessionID: s.ID}, nil
}
