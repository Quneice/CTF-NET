package aiagent

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// —— ARTEX 核心集成回归测试：代理访问 / 约束块 / goals 角色 / 舵向 / 空转提醒 ——

// TestProxyEnvPrefix：代理环境变量前缀语法（Windows cmd 链 / POSIX export）。
func TestProxyEnvPrefix(t *testing.T) {
	p := proxyEnvPrefix("http://127.0.0.1:7897")
	if !strings.Contains(p, "HTTP_PROXY=http://127.0.0.1:7897") {
		t.Fatalf("缺少 HTTP_PROXY: %q", p)
	}
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(p, "set ") || !strings.HasSuffix(p, "&& ") {
			t.Fatalf("Windows 应为 set 链: %q", p)
		}
	} else if !strings.HasPrefix(p, "export ") {
		t.Fatalf("POSIX 应为 export 前缀: %q", p)
	}
}

// TestExecWithProxyEnv：命令被前置代理前缀；无代理时原样透传。
func TestExecWithProxyEnv(t *testing.T) {
	var got string
	fake := func(ctx context.Context, cmd, dir string, to int) (string, string, error) {
		got = cmd
		return "", "", nil
	}
	_, _, _ = ExecWithProxyEnv(fake, "http://p:1")(context.Background(), "curl http://t", "/tmp", 30)
	if !strings.HasPrefix(got, proxyEnvPrefix("http://p:1")) || !strings.HasSuffix(got, "curl http://t") {
		t.Fatalf("命令未注入代理前缀: %q", got)
	}
	var got2 string
	fake2 := func(ctx context.Context, cmd, dir string, to int) (string, string, error) {
		got2 = cmd
		return "", "", nil
	}
	_, _, _ = ExecWithProxyEnv(fake2, "")(context.Background(), "curl http://t", "/tmp", 30)
	if got2 != "curl http://t" {
		t.Fatalf("无代理不应改动命令: %q", got2)
	}
}

// TestConstraintBlock：无约束为空串；SetGoals 后注入最高优先级约束块。
func TestConstraintBlock(t *testing.T) {
	task := NewTask("t1", "desc", "goal", "")
	if constraintBlock(task) != "" {
		t.Fatal("无约束应为空串")
	}
	task.SetGoals([]Goal{{Text: "g"}}, "允许的操作：只探测 10.0.0.5；禁止：破坏性操作", "10.0.0.5/24")
	cb := constraintBlock(task)
	for _, want := range []string{"最高优先级", "10.0.0.5/24", "10.0.0.5"} {
		if !strings.Contains(cb, want) {
			t.Fatalf("约束块缺少 %q: %q", want, cb)
		}
	}
}

// TestSetGoals：子目标重新编号 + 约束/范围落盘可恢复。
func TestSetGoals(t *testing.T) {
	dir := t.TempDir()
	task := NewTask("t1", "d", "g", dir)
	task.SetGoals([]Goal{{Text: "a"}, {Text: "b"}}, "c1", "s1")
	snap := task.Snapshot()
	if len(snap.Goals) != 2 || snap.Goals[0].ID != 1 || snap.Goals[1].ID != 2 {
		t.Fatalf("子目标编号错误: %+v", snap.Goals)
	}
	reloaded, err := LoadTask(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Constraints != "c1" || reloaded.Scope != "s1" {
		t.Fatal("约束/范围未落盘")
	}
}

// TestRunningWithSteer：运行中且带待注入舵向的意图识别（编排器据此中止重跑）。
func TestRunningWithSteer(t *testing.T) {
	task := NewTask("t1", "d", "g", "")
	task.AddIntents([]string{"a"}, nil, nil, 1)
	task.claimIntent("w1")
	if err := task.SteerIntent(1, "换个方向"); err != nil {
		t.Fatal(err)
	}
	if ids := task.runningWithSteer(); len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("应返回运行中带舵向意图: %v", ids)
	}
	task.takeSteer(1)
	if ids := task.runningWithSteer(); len(ids) != 0 {
		t.Fatalf("舵向消费后应为空: %v", ids)
	}
}

// TestDecomposeGoals：成功解析子目标与约束；非 JSON 输出报错（调用方兜底）。
func TestDecomposeGoals(t *testing.T) {
	srv := fakeLLMServer(t, []fakeStep{{content: `{"goals":[{"text":"探测 10.0.0.5:8080 并找到登录绕过"}],"constraints":"只探测 10.0.0.5:8080","scope":"10.0.0.5"}`}}, nil, nil)
	defer srv.Close()
	cfg := DefaultConfig()
	cfg.APIKey = "k"
	cfg.BaseURL = srv.URL
	goals, cs, sc, err := DecomposeGoals(context.Background(), NewLLM(cfg, ""), "d", "goal", "")
	if err != nil || len(goals) != 1 {
		t.Fatalf("分解失败: %v %+v", err, goals)
	}
	if cs == "" || sc == "" {
		t.Fatal("约束/范围为空")
	}

	srv2 := fakeLLMServer(t, nil, nil, nil)
	defer srv2.Close()
	cfg2 := DefaultConfig()
	cfg2.APIKey = "k"
	cfg2.BaseURL = srv2.URL
	if _, _, _, err := DecomposeGoals(context.Background(), NewLLM(cfg2, ""), "d", "goal", ""); err == nil {
		t.Fatal("非 JSON 输出应报错（由调用方保留原始目标兜底）")
	}
}

// TestEmptyTurnNudge：连续 2 轮只思考不行动后注入行动提醒。
func TestEmptyTurnNudge(t *testing.T) {
	seq := []fakeStep{
		{reasoning: "思考中…"},
		{reasoning: "继续思考…"},
		{content: "好，我总结收尾。"},
	}
	var reqs []chatRequest
	srv := fakeLLMServer(t, nil, nil, seq, func(req chatRequest) { reqs = append(reqs, req) })
	defer srv.Close()
	cfg := DefaultConfig()
	cfg.APIKey = "k"
	cfg.BaseURL = srv.URL
	task := NewTask("t1", "d", "g", "")
	task.AddIntents([]string{"探测"}, nil, nil, 1)
	intent := task.Intents[0]
	tc := &RunContext{Task: task, WorkDir: "", Intent: &intent, Agent: "worker"}
	res, err := runSession(context.Background(), NewLLM(cfg, ""), workerTools("", false), tc,
		workerSystemPrompt(task, intent, false), "开始执行意图", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Text, "总结收尾") {
		t.Fatalf("会话应正常收尾: %q", res.Text)
	}
	if len(reqs) < 3 {
		t.Fatalf("请求数不足: %d", len(reqs))
	}
	found := false
	for _, m := range reqs[2].Messages {
		if m.Role == string(RoleUser) && strings.Contains(fmt.Sprint(m.Content), "只思考没有行动") {
			found = true
		}
	}
	if !found {
		t.Fatalf("第 3 轮请求未注入空转提醒: %+v", reqs[2].Messages)
	}
}
