package aiagent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestLLMChatStream：SSE 文本流逐块回调 onDelta，最终拼接完整。
func TestLLMChatStream(t *testing.T) {
	content := "这是一段用于流式输出的中文回复，长度足以被分成多个 SSE 块。"
	srv := fakeLLMServer(t, []fakeStep{{content: content}}, nil, nil)
	llm := NewLLM(testConfig(srv.URL), "")

	var deltas []string
	resp, err := llm.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	}, func(d string) { deltas = append(deltas, d) })
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if len(deltas) < 2 {
		t.Fatalf("期望多块增量，得到 %d 块: %v", len(deltas), deltas)
	}
	if resp.Content != content || strings.Join(deltas, "") != content {
		t.Fatalf("流式拼接错误: %q vs %q", resp.Content, strings.Join(deltas, ""))
	}
	if resp.Finish != "stop" {
		t.Fatalf("Finish 错误: %q", resp.Finish)
	}
}

// TestLLMChatStreamToolCalls：流式工具调用解析。
func TestLLMChatStreamToolCalls(t *testing.T) {
	srv := fakeLLMServer(t, []fakeStep{{tool: &fakeToolCall{name: "record_fact", args: `{"text":"x"}`}}}, nil, nil)
	llm := NewLLM(testConfig(srv.URL), "")
	resp, err := llm.ChatStream(context.Background(), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Tools:    []ToolSchema{{Name: "record_fact", Parameters: map[string]any{"type": "object"}}},
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "record_fact" || resp.ToolCalls[0].Arguments != `{"text":"x"}` {
		t.Fatalf("工具调用错误: %+v", resp.ToolCalls)
	}
}

// TestToolSchemasNoNull：全部工具 schema 序列化后不得出现 null —— DeepSeek 严格
// 校验会以 400 拒绝 required:null / properties:null（2026-11-12 实测回归）。
func TestToolSchemasNoNull(t *testing.T) {
	regs := []*registry{
		workerTools("", false),
		plannerTools(),
	}
	chatReg, err := chatRegistry(&ChatEnv{Cfg: Config{}, Session: &ChatSession{}})
	if err != nil {
		t.Fatalf("chatRegistry: %v", err)
	}
	regs = append(regs, chatReg)

	for _, reg := range regs {
		for _, sch := range reg.schemas() {
			data, err := json.Marshal(sch.Parameters)
			if err != nil {
				t.Fatalf("%s schema 序列化失败: %v", sch.Name, err)
			}
			if strings.Contains(string(data), ":null") || strings.Contains(string(data), "[null") {
				t.Fatalf("工具 %s 的 schema 含 null（会被严格校验的 LLM 服务拒绝）: %s", sch.Name, data)
			}
		}
	}
}

// TestChatTurnStreamEvents：一轮流式对话（norma 引擎）—— 文本增量、思考增量、
// 工具事件与回复语义（终答只走 result）。
func TestChatTurnStreamEvents(t *testing.T) {
	defSeq := []fakeStep{
		{content: "我先探测一下目标端口。", tool: &fakeToolCall{name: "record_fact", args: `{"text":"8080 开放 nginx","kind":"service"}`}},
		{content: "已确认 8080 开放 nginx 1.18（事实 #1）。"},
	}
	srv := fakeLLMServer(t, defSeq, nil, nil)
	cfg := testConfig(srv.URL)

	root := t.TempDir()
	s := &ChatSession{ID: "chat-stream-1", CreatedAt: nowStr()}
	board := NewTask("chat-stream-1", "", "", filepath.Join(root, "chat-stream-1", "board"))
	env := &ChatEnv{
		Cfg: cfg, TaskRoot: filepath.Join(root, "tasks"), SessionRoot: root,
		Session: s, Exec: func(ctx context.Context, cmd, dir string, timeout int) (string, string, error) {
			return "", "", nil
		},
	}

	events := map[string][]string{}
	var deltas []string
	emit := func(kind string, payload map[string]any) {
		switch kind {
		case "delta":
			deltas = append(deltas, payload["content"].(string))
		case "tool", "tool_result":
			events[kind] = append(events[kind], payload["name"].(string))
		}
	}
	res, err := RunChatTurn(context.Background(), env, s, board, "看看 8080", 10, emit)
	if err != nil {
		t.Fatalf("RunChatTurn: %v", err)
	}
	// 工具事件：record_fact 开始 + 返回
	if len(events["tool"]) != 1 || events["tool"][0] != "record_fact" {
		t.Fatalf("tool 事件错误: %+v", events)
	}
	if len(events["tool_result"]) != 1 || events["tool_result"][0] != "record_fact" {
		t.Fatalf("tool_result 事件错误: %+v", events)
	}
	// delta 流应包含工具轮叙述（"我先探测…"，连续增量已合并为单条记录）；
	// 终答（"已确认…"）只走 result，不进 delta
	if len(deltas) < 1 || !strings.Contains(strings.Join(deltas, ""), "我先探测") {
		t.Fatalf("delta 流错误: %v", deltas)
	}
	if strings.Contains(strings.Join(deltas, ""), "已确认") {
		t.Fatalf("终答不应重复进 delta 流: %v", deltas)
	}
	if !strings.Contains(res.Reply, "已确认") || strings.Contains(res.Reply, "我先探测") {
		t.Fatalf("回复应为终轮文本: %q", res.Reply)
	}
	if len(board.Snapshot().Facts) != 1 {
		t.Fatalf("黑榜事实未沉淀: %+v", board.Snapshot().Facts)
	}
	if len(s.Messages) != 2 || s.Messages[1].Role != "assistant" || s.Messages[1].Content != res.Reply {
		t.Fatalf("会话落盘错误: %+v", s.Messages)
	}
}

// TestChatTurnThinkingMode：DeepSeek 思考模式 —— reasoning_content 不混入正文
// delta，且下一轮请求把上一轮助手的 reasoning_content 原样回传（2026-11-12 实测
// 回归：不回传会被服务端 400 拒绝）。多轮历史经 transcript Resume 恢复。
func TestChatTurnThinkingMode(t *testing.T) {
	seq := []fakeStep{
		{content: "我先记录一下目标。", reasoning: "思考：需要先记录目标资产。",
			tool: &fakeToolCall{name: "record_fact", args: `{"text":"47.96.7.24:32808 目标","kind":"service"}`}},
		{content: "已记录目标，开始侦察。", reasoning: "思考：回复用户进展。"},
		{content: "侦察完成。"},
	}
	var seen []chatRequest
	srv := fakeLLMServer(t, seq, nil, nil, func(req chatRequest) { seen = append(seen, req) })
	cfg := testConfig(srv.URL)

	root := t.TempDir()
	s := &ChatSession{ID: "chat-think-1", CreatedAt: nowStr()}
	board := NewTask("chat-think-1", "", "", filepath.Join(root, "chat-think-1", "board"))
	env := &ChatEnv{
		Cfg: cfg, TaskRoot: filepath.Join(root, "tasks"), SessionRoot: root,
		Session: s, Exec: func(ctx context.Context, cmd, dir string, timeout int) (string, string, error) {
			return "", "", nil
		},
	}

	var deltas, thinkings []string
	emit := func(kind string, payload map[string]any) {
		switch kind {
		case "delta":
			deltas = append(deltas, payload["content"].(string))
		case "thinking":
			thinkings = append(thinkings, payload["content"].(string))
		}
	}
	res1, err := RunChatTurn(context.Background(), env, s, board, "目标 47.96.7.24:32808", 10, emit)
	if err != nil {
		t.Fatalf("回合 1: %v", err)
	}
	if !strings.Contains(res1.Reply, "已记录目标") {
		t.Fatalf("回合 1 回复错误: %q", res1.Reply)
	}
	// 思考内容单独走 chat:thinking，不混入正文 delta
	if len(thinkings) == 0 || !strings.Contains(strings.Join(thinkings, ""), "思考：") {
		t.Fatalf("thinking 事件错误: %v", thinkings)
	}
	if strings.Contains(strings.Join(deltas, ""), "思考：") {
		t.Fatalf("思考内容混入了正文 delta: %v", deltas)
	}
	// 回合 1 内（工具轮）的第二个请求：assistant 工具调用消息必须带 reasoning_content
	if len(seen) < 2 {
		t.Fatalf("请求数不足: %d", len(seen))
	}
	if !requestHasReasoning(seen[1]) {
		t.Fatalf("工具轮请求未回传 reasoning_content: %+v", seen[1].Messages)
	}

	// 回合 2：经 transcript Resume 恢复的历史里，助手消息必须带 reasoning_content
	res2, err := RunChatTurn(context.Background(), env, s, board, "继续", 10, emit)
	if err != nil {
		t.Fatalf("回合 2: %v", err)
	}
	if !strings.Contains(res2.Reply, "侦察完成") {
		t.Fatalf("回合 2 回复错误: %q", res2.Reply)
	}
	found := false
	for _, req := range seen[2:] {
		if requestHasReasoning(req) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("多轮续传请求未回传 reasoning_content（DeepSeek 会 400）: %+v", seen[2:])
	}
	// 历史里应同时有回合 1 的助手回复文本
	joined := requestText(seen[len(seen)-1])
	if !strings.Contains(joined, "已记录目标") {
		t.Fatalf("transcript 续聊历史缺失: %q", joined)
	}
}

// requestHasReasoning 请求里是否存在带 reasoning_content 的 assistant 消息。
func requestHasReasoning(req chatRequest) bool {
	for _, m := range req.Messages {
		if m.Role == "assistant" && m.ReasoningContent != "" {
			return true
		}
	}
	return false
}

// requestText 拼接请求里全部文本内容（断言用）。
func requestText(req chatRequest) string {
	var b strings.Builder
	for _, m := range req.Messages {
		if s, ok := m.Content.(string); ok {
			b.WriteString(s)
			b.WriteString("\n")
		}
	}
	return b.String()
}
