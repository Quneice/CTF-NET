package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// —— 测试基建：可脚本化的 fake LLM 服务器 ——

type fakeStep struct {
	content   string
	reasoning string        // 非空 = 返回 reasoning_content（DeepSeek 思考模式）
	tool      *fakeToolCall // 非 nil = 返回工具调用
}

type fakeToolCall struct {
	name string
	args string
}

// fakeLLMServer 起一个 OpenAI 兼容的假服务器。handler 按会话类别取响应序列：
// 系统提示含【规划者】(planner) 走 planSeq，含【执行者】(worker) 走 workSeq，
// 均不含（纯对话测试）走 defaultSeq。序列耗尽后返回空文本。
// 请求带 stream:true 时以 SSE 流式应答（reasoning 与文本各按 ~8 rune 分块逐条
// data: 推送，工具调用单块推送），非流式时返回完整 JSON。
// hooks 收到每个已解析的请求（用于断言消息历史/回传字段）。
func fakeLLMServer(t *testing.T, defaultSeq, planSeq, workSeq []fakeStep, hooks ...func(req chatRequest)) *httptest.Server {
	t.Helper()
	defIdx, planIdx, workIdx := 0, 0, 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("fake LLM: 解析请求失败: %v", err)
			return
		}
		for _, h := range hooks {
			h(req)
		}
		kind := "default"
		for _, m := range req.Messages {
			if m.Role == "system" {
				s := fmt.Sprint(m.Content)
				switch {
				case strings.Contains(s, "【规划者】(planner)"):
					kind = "planner"
				case strings.Contains(s, "【执行者】(worker)"):
					kind = "worker"
				}
				break
			}
		}
		var seq []fakeStep
		var idx *int
		switch kind {
		case "planner":
			seq, idx = planSeq, &planIdx
		case "worker":
			seq, idx = workSeq, &workIdx
		default:
			seq, idx = defaultSeq, &defIdx
		}
		var step fakeStep
		if *idx < len(seq) {
			step = seq[*idx]
			*idx++
		} else {
			step = fakeStep{content: "(序列耗尽)"}
		}

		newChoices := func() []struct {
			Index        int             `json:"index"`
			Message      chatHTTPMessage `json:"message"`
			FinishReason string          `json:"finish_reason"`
			Delta        chatHTTPMessage `json:"delta"`
		} {
			return []struct {
				Index        int             `json:"index"`
				Message      chatHTTPMessage `json:"message"`
				FinishReason string          `json:"finish_reason"`
				Delta        chatHTTPMessage `json:"delta"`
			}{{Index: 0}}
		}

		if req.Stream {
			// SSE 流式应答（step 可同时带思考/叙述文本 + 工具调用：先流
			// reasoning 块，再流文本块，最后流工具块 —— 对齐 DeepSeek 思考模式）
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			sendChunk := func(cr chatResponse) {
				b, _ := json.Marshal(cr)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
				if flusher != nil {
					flusher.Flush()
				}
			}
			streamRunes := func(runes []rune, mk func(text string) chatHTTPMessage) {
				const chunkSize = 8
				for i := 0; i < len(runes); i += chunkSize {
					end := i + chunkSize
					if end > len(runes) {
						end = len(runes)
					}
					cr := chatResponse{}
					cr.Choices = newChoices()
					cr.Choices[0].Delta = mk(string(runes[i:end]))
					sendChunk(cr)
				}
			}
			if step.reasoning != "" {
				streamRunes([]rune(step.reasoning), func(text string) chatHTTPMessage {
					return chatHTTPMessage{Role: "assistant", ReasoningContent: text}
				})
			}
			if step.content != "" {
				streamRunes([]rune(step.content), func(text string) chatHTTPMessage {
					return chatHTTPMessage{Role: "assistant", Content: text}
				})
			}
			if step.tool != nil {
				cr := chatResponse{}
				cr.Choices = newChoices()
				cr.Choices[0].Delta = chatHTTPMessage{Role: "assistant", ToolCalls: []chatToolCall{{
					Index: 0, ID: "call_1", Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: step.tool.name, Arguments: step.tool.args},
				}}}
				cr.Choices[0].FinishReason = "tool_calls"
				sendChunk(cr)
			} else {
				cr := chatResponse{}
				cr.Choices = newChoices()
				cr.Choices[0].FinishReason = "stop"
				sendChunk(cr)
			}
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}

		resp := chatResponse{}
		resp.Choices = newChoices()
		resp.Choices[0].Message = chatHTTPMessage{Role: "assistant", Content: step.content, ReasoningContent: step.reasoning}
		resp.Choices[0].FinishReason = "stop"
		if step.tool != nil {
			resp.Choices[0].Message.ToolCalls = []chatToolCall{{
				ID: "call_1", Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: step.tool.name, Arguments: step.tool.args},
			}}
			resp.Choices[0].FinishReason = "tool_calls"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testConfig(serverURL string) Config {
	c := DefaultConfig()
	c.APIKey = "test-key"
	c.BaseURL = serverURL
	c.MaxTurns = 10
	c.MaxWorkers = 1
	c.PlanRounds = 3
	c.Timeout = 0
	return c
}

// —— llm.go ——

func TestLLMChatToolCalls(t *testing.T) {
	srv := fakeLLMServer(t, []fakeStep{{tool: &fakeToolCall{name: "list_assets", args: `{"query":"x"}`}}}, nil, nil)
	llm := NewLLM(testConfig(srv.URL), "")
	resp, err := llm.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Tools:    []ToolSchema{{Name: "list_assets", Parameters: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "list_assets" {
		t.Fatalf("工具调用解析错误: %+v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].Arguments != `{"query":"x"}` {
		t.Fatalf("参数原文错误: %q", resp.ToolCalls[0].Arguments)
	}
}

func TestLLMRetryOn5xx(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()
	llm := NewLLM(testConfig(srv.URL), "")
	resp, err := llm.Chat(context.Background(), ChatRequest{Messages: []Message{{Role: RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "ok" || calls != 3 {
		t.Fatalf("重试语义错误: calls=%d content=%q", calls, resp.Content)
	}
}

func TestLLMNoAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BaseURL = "http://127.0.0.1:1"
	llm := NewLLM(cfg, "")
	if _, err := llm.Chat(context.Background(), ChatRequest{}); err == nil || !strings.Contains(err.Error(), "API Key") {
		t.Fatalf("期望 API Key 错误，得到: %v", err)
	}
}

// —— 工具 ——

func TestReportFindingRequiresEvidence(t *testing.T) {
	task := NewTask("t1", "d", "g", "")
	reg := workerTools("", false)
	_, err := reg.call(context.Background(), "report_finding",
		`{"title":"x","vuln_class":"SQL注入","severity":"high","summary":"s","evidence":""}`,
		&RunContext{Task: task, Agent: "worker"})
	if err == nil || !strings.Contains(err.Error(), "evidence") {
		t.Fatalf("期望证据缺失拒绝，得到: %v", err)
	}
	if len(task.Snapshot().Findings) != 0 {
		t.Fatal("无证据漏洞不应入黑板")
	}
	_, err = reg.call(context.Background(), "report_finding",
		`{"title":"x","vuln_class":"SQL注入","severity":"high","summary":"s","evidence":"HTTP/1.1 200 ... union select 生效"}`,
		&RunContext{Task: task, Agent: "worker"})
	if err != nil {
		t.Fatalf("带证据上报失败: %v", err)
	}
	f := task.Snapshot().Findings
	if len(f) != 1 || f[0].Severity != SevHigh {
		t.Fatalf("漏洞入黑板错误: %+v", f)
	}
}

func TestAddIntentsCapAndDedup(t *testing.T) {
	task := NewTask("t1", "d", "g", "")
	reg := plannerTools()
	tc := &RunContext{Task: task, Agent: "planner"}
	// 5 条超上限
	five := `{"intents":[{"summary":"a1"},{"summary":"a2"},{"summary":"a3"},{"summary":"a4"},{"summary":"a5"}]}`
	if _, err := reg.call(context.Background(), "add_intents", five, tc); err == nil || !strings.Contains(err.Error(), "最多") {
		t.Fatalf("期望批量上限拒绝，得到: %v", err)
	}
	// 正常派发 2 条
	if _, err := reg.call(context.Background(), "add_intents", `{"intents":[{"summary":"探测 A"},{"summary":"探测 B"}]}`, tc); err != nil {
		t.Fatalf("派发失败: %v", err)
	}
	// 与 open 意图重复 → 拒绝
	_, err := reg.call(context.Background(), "add_intents", `{"intents":[{"summary":"探测 A"}]}`, tc)
	if err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("期望判重拒绝，得到: %v", err)
	}
	if len(task.Snapshot().Intents) != 2 {
		t.Fatalf("意图数错误: %d", len(task.Snapshot().Intents))
	}
}

func TestExecCommandRejectsDestructive(t *testing.T) {
	task := NewTask("t1", "d", "g", "")
	reg := workerTools("", true)
	exec := func(ctx context.Context, cmd, dir string, timeout int) (string, string, error) {
		return "out", "", nil
	}
	_, err := reg.call(context.Background(), "exec_command", `{"command":"rm -rf /","timeout":5}`,
		&RunContext{Task: task, Agent: "worker", Exec: exec})
	if err == nil || !strings.Contains(err.Error(), "破坏性") {
		t.Fatalf("期望拒绝破坏性命令，得到: %v", err)
	}
}

func TestHttpRequestTool(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "yes")
		_, _ = w.Write([]byte("<html>target</html>"))
	}))
	defer target.Close()
	task := NewTask("t1", "d", "g", "")
	reg := workerTools("", false)
	out, err := reg.call(context.Background(), "http_request",
		fmt.Sprintf(`{"url":%q,"method":"GET","timeout":5}`, target.URL),
		&RunContext{Task: task, Agent: "worker"})
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	if !strings.Contains(out, "X-Test: yes") || !strings.Contains(out, "<html>target</html>") {
		t.Fatalf("响应内容缺失: %s", out)
	}
}

// —— 配方 ——

func TestRecipeEngine(t *testing.T) {
	task := NewTask("t1", "d", "g", "")
	tools, err := LoadRecipes("")
	if err != nil {
		t.Fatalf("LoadRecipes: %v", err)
	}
	names := map[string]bool{}
	for _, tl := range tools {
		names[tl.Name()] = true
	}
	if !names["http_headers"] || !names["dns_lookup"] {
		t.Fatalf("内置配方缺失: %v", names)
	}
	// 执行 dns_lookup：验证 flag 内嵌值
	var got string
	exec := func(ctx context.Context, cmd, dir string, timeout int) (string, string, error) {
		got = cmd
		return "1.2.3.4", "", nil
	}
	reg := newRegistry()
	if err := registerRecipes(reg, tools); err != nil {
		t.Fatalf("注册配方: %v", err)
	}
	out, err := reg.call(context.Background(), "dns_lookup", `{"domain":"example.com","record_type":"MX"}`,
		&RunContext{Task: task, Agent: "worker", Exec: exec})
	if err != nil {
		t.Fatalf("配方执行失败: %v", err)
	}
	if !strings.Contains(got, "nslookup") || !strings.Contains(got, "-type=MX") || !strings.Contains(got, "example.com") {
		t.Fatalf("命令组装错误: %q", got)
	}
	if !strings.Contains(out, "1.2.3.4") {
		t.Fatalf("输出错误: %q", out)
	}
	// 缺失必填参数
	_, err = reg.call(context.Background(), "dns_lookup", `{}`,
		&RunContext{Task: task, Agent: "worker", Exec: exec})
	if err == nil || !strings.Contains(err.Error(), "domain") {
		t.Fatalf("期望必填参数错误，得到: %v", err)
	}
}

func TestRecipeCollisionWithBuiltinTool(t *testing.T) {
	dir := t.TempDir()
	// 与内置工具同名 → 注册期报错（与内置配方同名则视为用户覆盖）
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(`
version: 1
name: http_request
enabled: true
command: curl
short_description: x
description: y
parameters: []
`), 0o600); err != nil {
		t.Fatal(err)
	}
	tools, err := LoadRecipes(dir)
	if err != nil {
		t.Fatalf("LoadRecipes: %v", err)
	}
	reg := workerTools("", false)
	if err := registerRecipes(reg, tools); err == nil || !strings.Contains(err.Error(), "冲突") {
		t.Fatalf("期望同名冲突报错，得到: %v", err)
	}
}

func TestRecipeUserOverride(t *testing.T) {
	dir := t.TempDir()
	// 覆盖内置 http_headers
	if err := os.WriteFile(filepath.Join(dir, "http_headers.yaml"), []byte(`
version: 1
name: http_headers
enabled: true
command: curl
args: ["-I", "--max-time", "5"]
short_description: 自定义
description: 用户覆盖版
parameters:
  - name: url
    type: string
    flag: --url
    required: true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	tools, err := LoadRecipes(dir)
	if err != nil {
		t.Fatalf("LoadRecipes: %v", err)
	}
	var hd Tool
	for _, tl := range tools {
		if tl.Name() == "http_headers" {
			hd = tl
		}
	}
	if hd == nil || hd.Description() != "用户覆盖版" {
		t.Fatal("用户配方未覆盖内置配方")
	}
}

// —— 黑板 ——

func TestBlackboardPersistence(t *testing.T) {
	dir := t.TempDir()
	task := NewTask("t1", "desc", "goal", dir)
	task.MarkRunning()
	task.AddFact("发现 nginx 1.18", "service", "worker", 0)
	task.AddIntents([]string{"探测 A"}, nil, nil, 1)
	task.FinishDone("ok")
	data, err := os.ReadFile(filepath.Join(dir, "task.json"))
	if err != nil {
		t.Fatalf("task.json 未落盘: %v", err)
	}
	if !strings.Contains(string(data), "nginx 1.18") || !strings.Contains(string(data), `"status": "done"`) {
		t.Fatalf("快照内容错误: %s", data)
	}
}

func TestProveGoalTerminal(t *testing.T) {
	task := NewTask("t1", "d", "g", "")
	task.MarkRunning()
	if err := task.ProveGoal(1, "拿到 flag", "漏洞 #1", 0); err != nil {
		t.Fatalf("ProveGoal: %v", err)
	}
	if !task.allGoalsMet() {
		t.Fatal("目标应全部达成")
	}
	// ProveGoal 不直接置终态（由 planner 工具层判定）——补一次 goal_met 语义检查
	task.FinishDone("达成")
	if task.StatusNow() != StatusDone {
		t.Fatalf("终态错误: %s", task.StatusNow())
	}
}

// —— 编排器端到端 ——

// 场景：planner 派 1 条意图 → worker 记录事实并收工 → planner 判达成 → done
func TestOrchestratorEndToEndDone(t *testing.T) {
	planSeq := []fakeStep{
		{tool: &fakeToolCall{name: "add_intents", args: `{"intents":[{"summary":"探测 http://127.0.0.1:9999 的 Web 服务"}]}`}},
		{content: "已派发探测意图"},
		{tool: &fakeToolCall{name: "prove_goal", args: `{"goal_id":1,"reason":"目标服务可达并记录了事实","evidence_summary":"事实 #1"}`}},
		{content: "目标达成"},
	}
	workSeq := []fakeStep{
		{tool: &fakeToolCall{name: "record_fact", args: `{"text":"发现 nginx 1.18 服务","kind":"service"}`}},
		{content: "总结：目标可达，服务指纹已记录"},
	}
	srv := fakeLLMServer(t, nil, planSeq, workSeq)
	cfg := testConfig(srv.URL)
	cfg.Timeout = 20
	exec := func(ctx context.Context, cmd, dir string, timeout int) (string, string, error) { return "", "", nil }
	orch, err := NewOrchestrator(cfg, "", "", exec)
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	dir := t.TempDir()
	task := NewTask("t1", "验证端到端流程", "确认目标可达", dir)
	task.TimeoutSec = cfg.Timeout
	task.MaxTurns = cfg.MaxTurns
	task.MaxWorkers = cfg.MaxWorkers
	task.PlanRounds = cfg.PlanRounds
	task.EnableExec = cfg.EnableExec
	var lastPct int
	orch.SetProgress(func(pct int, msg string) { lastPct = pct })

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := orch.Run(ctx, task); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.StatusNow() != StatusDone {
		t.Fatalf("期望 done，得到 %s（意图: %+v）", task.StatusNow(), task.Snapshot().Intents)
	}
	snap := task.Snapshot()
	if len(snap.Facts) != 1 || len(snap.Intents) != 1 || snap.Intents[0].State != IntentDone {
		t.Fatalf("黑板状态错误: facts=%d intents=%+v", len(snap.Facts), snap.Intents)
	}
	if lastPct != 100 {
		t.Fatalf("终态进度应为 100，得到 %d", lastPct)
	}
}

// 场景：planner 无产出 → 规划预算耗尽 → exhausted
func TestOrchestratorExhausted(t *testing.T) {
	planSeq := []fakeStep{{content: "暂无方向，继续观察"}}
	srv := fakeLLMServer(t, nil, planSeq, nil)
	cfg := testConfig(srv.URL)
	cfg.Timeout = 20
	cfg.PlanRounds = 1
	orch, err := NewOrchestrator(cfg, "", "", nil)
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	task := NewTask("t2", "d", "找不到的目标", "")
	task.TimeoutSec = cfg.Timeout
	task.MaxTurns = cfg.MaxTurns
	task.MaxWorkers = cfg.MaxWorkers
	task.PlanRounds = cfg.PlanRounds
	task.EnableExec = cfg.EnableExec
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := orch.Run(ctx, task); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if task.StatusNow() != StatusExhausted {
		t.Fatalf("期望 exhausted，得到 %s", task.StatusNow())
	}
}

// 场景：LLM 不可达 → 初轮规划重试后失败 → failed
func TestOrchestratorPlannerFails(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1") // 故意不可达
	cfg.Timeout = 5
	orch, err := NewOrchestrator(cfg, "", "", nil)
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	task := NewTask("t3", "d", "g", "")
	task.TimeoutSec = cfg.Timeout
	task.MaxTurns = cfg.MaxTurns
	task.MaxWorkers = cfg.MaxWorkers
	task.PlanRounds = cfg.PlanRounds
	task.EnableExec = cfg.EnableExec
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = orch.Run(ctx, task) // 初轮规划 LLM 不可达：重试 3 次后退避
	if task.StatusNow() != StatusFailed {
		t.Fatalf("期望 failed，得到 %s", task.StatusNow())
	}
}

// 场景：用户取消 → stopped
func TestOrchestratorCancelled(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // 挂住 LLM 调用，等待取消
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"x"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()
	cfg := testConfig(srv.URL)
	cfg.Timeout = 0
	orch, err := NewOrchestrator(cfg, "", "", nil)
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	task := NewTask("t4", "d", "g", "")
	task.TimeoutSec = 0
	task.MaxTurns = cfg.MaxTurns
	task.MaxWorkers = cfg.MaxWorkers
	task.PlanRounds = cfg.PlanRounds
	task.EnableExec = cfg.EnableExec
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- orch.Run(ctx, task) }()
	time.Sleep(500 * time.Millisecond) // 让初轮规划进入 LLM 调用
	cancel()
	close(block) // 释放挂住的请求，让 worker/planner 退出
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run 未在取消后退出")
	}
	if task.StatusNow() != StatusStopped {
		t.Fatalf("期望 stopped，得到 %s", task.StatusNow())
	}
}
