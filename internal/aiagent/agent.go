package aiagent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// —— LLM 工具调用会话循环（worker 与 planner 共用）——

// sessionResult 会话结果。
type sessionResult struct {
	Text          string   // 最终文本（最后一轮 assistant 的 content）
	RoundTexts    []string // 各轮 assistant 文本（流式对话合并成完整叙述用）
	LastReasoning string   // 最后一轮 assistant 的思考内容（对话落盘续传用）
	Turns         int      // 消耗的轮数
	ToolCalls     int      // 工具调用次数
	TurnLimit     bool     // 是否因轮数上限被截断
}

// ConvHooks 会话循环的流式回调（nil = 非流式，走 llm.Chat）。
type ConvHooks struct {
	OnDelta      func(delta string)          // assistant 文本增量（逐 token）
	OnToolStart  func(name, argBrief string) // 工具开始执行
	OnToolResult func(name, summary string)  // 工具返回摘要
}

// emptyTurnNudge 思考空转提醒（ARTEX emptyTurnNudge 语义，第 2 轮空转后注入）。
const emptyTurnNudge = "你已连续两轮只思考没有行动。请立即做出选择：调用工具推进当前任务，或直接给出最终结论总结收尾；不要继续空转。"

// runSession 运行一次会话：
//
//	messages = [system, user] + 工具往返
//	模型返回 tool_calls → 逐个执行 → 结果以 tool 消息回传；返回纯文本 → 结束。
//	工具执行错误不中断会话：错误文本作为工具结果回传给模型，让它自行纠正。
//
// maxTurns 为工具往返上限（0 = 不限）；trace 把工具调用摘要写入绑定意图的轨迹。
func runSession(ctx context.Context, llm *LLM, reg *registry, tc *RunContext,
	system, user string, maxTurns int, maxTokens int) (*sessionResult, error) {

	return runConversation(ctx, llm, reg, tc, system,
		[]Message{{Role: RoleUser, Content: user}}, maxTurns, maxTokens, nil)
}

// runConversation 带历史消息的会话循环（对话式渗透用）：history 为用户与助手的
// 既往往返（不含 system），按序放在 system 提示之后。
// hooks 非 nil 时走流式调用并逐 token/工具事件回调。
func runConversation(ctx context.Context, llm *LLM, reg *registry, tc *RunContext,
	system string, history []Message, maxTurns int, maxTokens int, hooks *ConvHooks) (*sessionResult, error) {

	msgs := []Message{{Role: RoleSystem, Content: system}}
	msgs = append(msgs, history...)
	res := &sessionResult{}
	emptyTurns := 0 // 连续只思考不行动的轮数（ARTEX emptyTurnNudge 语义）
	for turns := 0; maxTurns == 0 || turns < maxTurns; turns++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		res.Turns = turns + 1
		req := ChatRequest{Messages: msgs, Tools: reg.schemas(), MaxTokens: maxTokens}
		call := func() (*LLMResponse, error) {
			if hooks != nil {
				return llm.ChatStream(ctx, req, hooks.OnDelta)
			}
			return llm.Chat(ctx, req)
		}
		resp, err := call()
		// model_error：整轮重试 2 次 × 3s（ARTEX runIntent 的模型错误恢复语义）
		for attempt := 0; err != nil && attempt < 2; attempt++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(3 * time.Second):
			}
			resp, err = call()
		}
		if err != nil {
			return nil, fmt.Errorf("第 %d 轮对话失败: %w", res.Turns, err)
		}
		res.LastReasoning = resp.Reasoning
		if tc.Task != nil {
			tc.Task.AddUsage(int64(resp.UsageIn), int64(resp.UsageOut))
		}
		if resp.Content != "" {
			res.RoundTexts = append(res.RoundTexts, resp.Content)
		}
		if len(resp.ToolCalls) == 0 {
			// 思考-only 轮：只思考不行动不结束会话；连续 2 轮 → 注入行动提醒（ARTEX
			// emptyTurnNudge 语义），提醒后重新计数，由 maxTurns 兜底防无限空转。
			if resp.Content == "" && resp.Reasoning != "" {
				emptyTurns++
				if emptyTurns >= 2 {
					msgs = append(msgs, Message{Role: RoleUser, Content: emptyTurnNudge})
					emptyTurns = 0
				}
				continue
			}
			emptyTurns = 0
			res.Text = resp.Content
			if res.Text == "" {
				res.Text = "(模型未返回内容)"
			}
			return res, nil
		}
		emptyTurns = 0
		// 记录 assistant 的工具调用消息（reasoning 原样保留，思考模式下一轮要回传）
		assistant := Message{Role: RoleAssistant, Content: resp.Content, Reasoning: resp.Reasoning, ToolCalls: resp.ToolCalls}
		msgs = append(msgs, assistant)
		// 逐个执行工具
		for _, call := range resp.ToolCalls {
			res.ToolCalls++
			if tc.Intent != nil {
				tc.Task.appendIntentTrace(tc.Intent.ID, fmt.Sprintf("[%s] %s %s",
					call.Name, call.Arguments, argBrief(call.Arguments)))
			}
			if hooks != nil {
				hooks.OnToolStart(call.Name, argBrief(call.Arguments))
			}
			out, err := reg.call(ctx, call.Name, call.Arguments, tc)
			result := out
			if err != nil {
				result = "工具错误: " + err.Error()
			}
			if hooks != nil {
				hooks.OnToolResult(call.Name, truncateRune(result, 300))
			}
			msgs = append(msgs, Message{Role: RoleTool, Content: result, ToolCallID: call.ID})
		}
	}
	// 轮数用尽：以最后一轮 assistant 内容收尾
	res.TurnLimit = true
	last := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleAssistant {
			last = msgs[i].Content
			break
		}
	}
	res.Text = last
	if res.Text == "" {
		res.Text = "(轮数上限已到，会话被截断)"
	}
	return res, nil
}

// argBrief 参数摘要（轨迹用，截断防膨胀）。
func argBrief(raw string) string {
	s := strings.TrimSpace(raw)
	if len([]rune(s)) > 120 {
		return truncateRune(s, 120)
	}
	return s
}
