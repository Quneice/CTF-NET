package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ctfnet/internal/aiagent"
	"ctfnet/internal/core"
)

// scriptedLLM 按会话类别回放脚本（planner/worker 两类），与 aiagent 包内测试同构。
func scriptedLLM(t *testing.T, planSeq, workSeq [][]map[string]any) *httptest.Server {
	t.Helper()
	planIdx, workIdx := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("解析请求失败: %v", err)
			return
		}
		kind := "worker"
		for _, m := range req.Messages {
			if m.Role == "system" {
				s := m.Content.(string)
				if strings.Contains(s, "【规划者】(planner)") {
					kind = "planner"
				}
				if strings.Contains(s, "【目标分解者】(goals)") {
					kind = "goals"
				}
				break
			}
		}
		if kind == "goals" {
			// goals 角色：固定返回一条子目标 JSON（不消耗 planner/worker 脚本）
			msg := map[string]any{"role": "assistant", "content": `{"goals":[{"text":"探测 http://127.0.0.1:9 的 Web 服务并找出可利用漏洞"}],"constraints":"仅探测 127.0.0.1:9，禁止破坏性操作","scope":"http://127.0.0.1:9"}`}
			resp := map[string]any{"choices": []map[string]any{{"index": 0, "message": msg, "finish_reason": "stop"}}}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		var seq [][]map[string]any
		var idx *int
		if kind == "planner" {
			seq, idx = planSeq, &planIdx
		} else {
			seq, idx = workSeq, &workIdx
		}
		step := map[string]any{"content": "(序列耗尽)"}
		if *idx < len(seq) {
			step = seq[*idx][0]
			*idx++
		}
		msg := map[string]any{"role": "assistant", "content": step["content"]}
		if tc, ok := step["tool_calls"]; ok {
			msg["content"] = ""
			msg["tool_calls"] = tc
		}
		resp := map[string]any{"choices": []map[string]any{{
			"index": 0, "message": msg, "finish_reason": step["finish"],
		}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func toolCall(name, args string) map[string]any {
	return map[string]any{
		"tool_calls": []map[string]any{{
			"id": "call_1", "type": "function",
			"function": map[string]any{"name": name, "arguments": args},
		}},
		"finish": "tool_calls",
	}
}

func textStep(content string) map[string]any {
	return map[string]any{"content": content, "finish": "stop"}
}

// step 包装单条响应为一个会话轮次。
func step(m map[string]any) []map[string]any { return []map[string]any{m} }

// TestAiTaskEndToEnd：完整 Runner 流程——planner 派意图 → worker 记事实 →
// planner prove_goal → 终态 done，结果 Data 带漏洞/事实/轨迹。
func TestAiTaskEndToEnd(t *testing.T) {
	planSeq := [][]map[string]any{
		step(toolCall("add_intents", `{"intents":[{"summary":"探测 http://127.0.0.1:9 的 Web 服务"}]}`)),
		step(textStep("已派发")),
		step(toolCall("prove_goal", `{"goal_id":1,"reason":"目标可达且记录了服务事实","evidence_summary":"事实 #1"}`)),
		step(textStep("达成")),
	}
	workSeq := [][]map[string]any{
		step(toolCall("record_fact", `{"text":"发现 nginx 1.18 服务","kind":"service"}`)),
		step(textStep("总结：目标可达，指纹已记录")),
	}
	srv := scriptedLLM(t, planSeq, workSeq)

	regTool, ok := core.DefaultRegistry.Get("ai.pentest.task")
	if !ok {
		t.Fatal("注册表中找不到 ai.pentest.task")
	}
	tool := regTool
	params := map[string]any{
		"goal":            "http://127.0.0.1:9 的 Web 服务，找出可利用漏洞并证明",
		"api_key":         "test-key",
		"base_url":        srv.URL,
		"model":           "test-model",
		"timeout_seconds": 30,
		"max_workers":     1,
		"plan_rounds":     2,
		"max_turns":       5,
		"enable_exec":     false,
	}
	validated, err := tool.ValidateParams(params)
	if err != nil {
		t.Fatalf("ValidateParams: %v", err)
	}
	tc := &core.ToolContext{
		Executor:       &core.SubprocessExecutor{},
		Workdir:        t.TempDir(),
		ReportProgress: func(percent int, message string) {},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := tool.Run(ctx, validated, tc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("状态错误: %s\nstdout=%s\nstderr=%s", res.Status, res.Stdout, res.Stderr)
	}
	status, _ := res.Data["status"].(string)
	if status != aiagent.StatusDone {
		t.Fatalf("期望 done，得到 %s\n%s", status, res.Stdout)
	}
	facts, _ := res.Data["facts"].([]map[string]any)
	if len(facts) != 1 {
		t.Fatalf("事实数错误: %v\n%s", facts, res.Stdout)
	}
	acts, _ := res.Data["activities"].([]map[string]any)
	if len(acts) == 0 {
		t.Fatalf("活动流水为空\n%s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "task.json") {
		t.Fatalf("stdout 缺少状态路径提示\n%s", res.Stdout)
	}
}

// TestAiTaskMissingAPIKey：未配 Key → 失败且明确提示。
func TestAiTaskMissingAPIKey(t *testing.T) {
	// 清空设置中的 Key 并回填空值
	cfg := core.GetAppConfig()
	cfg.Ai.APIKey = ""
	core.SetAppConfig(cfg)
	defer core.SetAppConfig(core.DefaultAppConfig())

	regTool, ok := core.DefaultRegistry.Get("ai.pentest.task")
	if !ok {
		t.Fatal("注册表中找不到 ai.pentest.task")
	}
	tool := regTool
	params := map[string]any{"goal": "探测 127.0.0.1", "base_url": "http://127.0.0.1:1", "timeout_seconds": 10}
	validated, err := tool.ValidateParams(params)
	if err != nil {
		t.Fatalf("ValidateParams: %v", err)
	}
	tc := &core.ToolContext{Executor: &core.SubprocessExecutor{}, Workdir: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := tool.Run(ctx, validated, tc)
	if err != nil {
		t.Fatalf("Run 不应返回底层错误（应体现在结果中）: %v", err)
	}
	if res.Status != core.StatusError || !strings.Contains(res.Stdout, "API Key") {
		t.Fatalf("期望 API Key 错误结果，status=%s stdout=%s", res.Status, res.Stdout)
	}
	if status, _ := res.Data["status"].(string); status != aiagent.StatusFailed {
		t.Fatalf("任务终态应为 failed，得到 %v", res.Data["status"])
	}
}
