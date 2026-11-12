package aiagent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRunChatTurn：一轮对话（工具调用 → 回复落盘 → 黑榜沉淀 → transcript 持久化）。
func TestRunChatTurn(t *testing.T) {
	// 对话助手系统提示没有 planner/worker 标记 → 走 defaultSeq
	defSeq := []fakeStep{
		{tool: &fakeToolCall{name: "record_fact", args: `{"text":"目标 8080 端口开放 nginx","kind":"service"}`}},
		{content: "已确认目标 8080 端口开放 nginx 1.18（事实 #1）。接下来可以探测后台路径。"},
	}
	srv := fakeLLMServer(t, defSeq, nil, nil)
	cfg := testConfig(srv.URL)

	root := t.TempDir()
	s := &ChatSession{ID: "chat-test-1", CreatedAt: nowStr()}
	board := NewTask("chat-test-1", "", "", filepath.Join(root, "chat-test-1", "board"))

	env := &ChatEnv{
		Cfg: cfg, TaskRoot: filepath.Join(root, "tasks"), SessionRoot: root,
		Session: s, Exec: func(ctx context.Context, cmd, dir string, timeout int) (string, string, error) {
			return "", "", nil
		},
	}
	res, err := RunChatTurn(context.Background(), env, s, board, "帮我看看 http://10.0.0.5:8080", 10, nil)
	if err != nil {
		t.Fatalf("RunChatTurn: %v", err)
	}
	// 回复 = 终轮 assistant 文本（ARTEX 语义：过程叙述走 delta 流，终答走 result）
	if res.SessionID != "chat-test-1" || !strings.Contains(res.Reply, "nginx") {
		t.Fatalf("回复错误: %+v", res)
	}
	if len(s.Messages) != 2 || s.Messages[0].Role != "user" || s.Messages[1].Role != "assistant" {
		t.Fatalf("消息历史错误: %+v", s.Messages)
	}
	// 轮次分段：工具调用 + 工具结果 + 终答叙述（终答由 captureChatRun 去重后作为最后一段补上）
	segs := s.Messages[1].Segments
	if len(segs) != 3 || segs[0].Kind != "tool" || segs[0].Name != "record_fact" ||
		segs[1].Kind != "tool_result" || segs[1].Name != "record_fact" ||
		segs[2].Kind != "text" || segs[2].Text != res.Reply {
		t.Fatalf("轮次分段错误: %+v", segs)
	}
	if s.Title == "" {
		t.Fatal("会话标题未生成")
	}
	if len(board.Snapshot().Facts) != 1 {
		t.Fatalf("黑榜事实未沉淀: %+v", board.Snapshot().Facts)
	}
	// 落盘检查：session.json + transcript JSONL（ARTEX transcript.Store 布局）
	if _, err := os.Stat(filepath.Join(root, "chat-test-1", "session.json")); err != nil {
		t.Fatalf("session.json 未落盘: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "chat-test-1", "chat-test-1.jsonl")); err != nil {
		t.Fatalf("transcript 未落盘: %v", err)
	}
	reloaded, err := LoadChatSession(root, "chat-test-1")
	if err != nil || reloaded == nil || len(reloaded.Messages) != 2 {
		t.Fatalf("会话重载失败: %v %+v", err, reloaded)
	}
}

// TestRunChatTurnCancelled：回复中被打断（ctx 取消）→ 回合报错、只留用户消息，
// 同会话随后可以继续发新消息（打断后补充说明的场景）。
func TestRunChatTurnCancelled(t *testing.T) {
	defSeq := []fakeStep{{content: "打断后继续：目标 8080 是 nginx。"}}
	srv := fakeLLMServer(t, defSeq, nil, nil)
	cfg := testConfig(srv.URL)

	root := t.TempDir()
	s := &ChatSession{ID: "chat-cancel", CreatedAt: nowStr()}
	board := NewTask("chat-cancel", "", "", filepath.Join(root, "chat-cancel", "board"))
	env := &ChatEnv{
		Cfg: cfg, TaskRoot: filepath.Join(root, "tasks"), SessionRoot: root,
		Session: s, Exec: func(ctx context.Context, cmd, dir string, timeout int) (string, string, error) {
			return "", "", nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 回合开始前即被打断
	if _, err := RunChatTurn(ctx, env, s, board, "第一条消息", 10, nil); err == nil {
		t.Fatal("被取消的回合应返回错误")
	}
	if len(s.Messages) != 1 || s.Messages[0].Role != "user" {
		t.Fatalf("被打断回合只应留下用户消息: %+v", s.Messages)
	}
	// 打断后继续：同一会话补发新消息，应正常完成并追加回复
	res, err := RunChatTurn(context.Background(), env, s, board, "补充说明：顺便看下响应头", 10, nil)
	if err != nil {
		t.Fatalf("打断后继续发送失败: %v", err)
	}
	if !strings.Contains(res.Reply, "nginx") || len(s.Messages) != 3 {
		t.Fatalf("打断后继续的回合异常: reply=%q messages=%d", res.Reply, len(s.Messages))
	}
	if s.Messages[2].Segments == nil || len(s.Messages[2].Segments) == 0 {
		t.Fatalf("继续回合缺少轮次分段: %+v", s.Messages[2])
	}
}

// TestRunChatTurnMaxTurns：步数上限触发时必须有可读结论，不能出现空占位符。
// 配置了 Settlement 后，SDK 在预算到点时注入收尾提示让模型写总结（终答=总结文本）；
// 若收尾模型仍调用工具把收尾预算耗尽，则回退终态说明文案兜底。
func TestRunChatTurnMaxTurns(t *testing.T) {
	newEnv := func(t *testing.T, srvURL, id string) (*ChatEnv, *ChatSession, string) {
		root := t.TempDir()
		s := &ChatSession{ID: id, CreatedAt: nowStr()}
		env := &ChatEnv{
			Cfg: testConfig(srvURL), TaskRoot: filepath.Join(root, "tasks"), SessionRoot: root,
			Session: s, Exec: func(ctx context.Context, cmd, dir string, timeout int) (string, string, error) {
				return "", "", nil
			},
		}
		return env, s, root
	}

	t.Run("收尾写总结", func(t *testing.T) {
		defSeq := []fakeStep{
			{tool: &fakeToolCall{name: "record_fact", args: `{"text":"端口 8080 开放","kind":"service"}`}},
			{tool: &fakeToolCall{name: "record_fact", args: `{"text":"端口 8443 开放","kind":"service"}`}},
			{content: "总结结论：目标开放 8080/8443，未发现可利用漏洞。建议下一步探测后台路径。"},
		}
		srv := fakeLLMServer(t, defSeq, nil, nil)
		env, s, root := newEnv(t, srv.URL, "chat-maxturns-summary")
		res, err := RunChatTurn(context.Background(), env, s, NewTask("chat-maxturns-summary", "", "", filepath.Join(root, "chat-maxturns-summary", "board")), "探测目标", 2, nil)
		if err != nil {
			t.Fatalf("RunChatTurn: %v", err)
		}
		if !strings.Contains(res.Reply, "总结结论") || strings.Contains(res.Reply, "(模型未返回内容)") {
			t.Fatalf("步数上限收尾应带模型总结，实际: %q", res.Reply)
		}
		// 终答作为最后一段落盘
		segs := s.Messages[1].Segments
		if len(segs) == 0 || segs[len(segs)-1].Kind != "text" || segs[len(segs)-1].Text != res.Reply {
			t.Fatalf("收尾终答未落盘为最后分段: %+v", segs)
		}
	})

	t.Run("收尾预算耗尽兜底", func(t *testing.T) {
		// 收尾模型又调用工具 → 收尾预算(1)耗尽 → SDK 空终答 → 回退终态说明
		defSeq := []fakeStep{
			{tool: &fakeToolCall{name: "record_fact", args: `{"text":"端口 8080 开放","kind":"service"}`}},
			{tool: &fakeToolCall{name: "record_fact", args: `{"text":"端口 8443 开放","kind":"service"}`}},
			{tool: &fakeToolCall{name: "record_fact", args: `{"text":"又一条事实","kind":"service"}`}},
		}
		srv := fakeLLMServer(t, defSeq, nil, nil)
		env, s, root := newEnv(t, srv.URL, "chat-maxturns-fallback")
		res, err := RunChatTurn(context.Background(), env, s, NewTask("chat-maxturns-fallback", "", "", filepath.Join(root, "chat-maxturns-fallback", "board")), "探测目标", 2, nil)
		if err != nil {
			t.Fatalf("RunChatTurn: %v", err)
		}
		if !strings.Contains(res.Reply, "max_turns") || strings.Contains(res.Reply, "(模型未返回内容)") {
			t.Fatalf("空终答应回退终态说明，实际: %q", res.Reply)
		}
	})
}

// TestStartBackgroundTask：start_task 工具发起后台任务 → 任务跑完 → 状态可查。
func TestStartBackgroundTask(t *testing.T) {
	planSeq := []fakeStep{
		{tool: &fakeToolCall{name: "add_intents", args: `{"intents":[{"summary":"探测 http://127.0.0.1:9 的 Web 服务"}]}`}},
		{content: "已派发"},
		{tool: &fakeToolCall{name: "prove_goal", args: `{"goal_id":1,"reason":"目标可达","evidence_summary":"事实 #1"}`}},
		{content: "达成"},
	}
	workSeq := []fakeStep{
		{tool: &fakeToolCall{name: "record_fact", args: `{"text":"发现服务","kind":"service"}`}},
		{content: "完成"},
	}
	srv := fakeLLMServer(t, nil, planSeq, workSeq)
	cfg := testConfig(srv.URL)
	cfg.Timeout = 20

	root := t.TempDir()
	dir := filepath.Join(root, "task-1")
	task := NewTask("task-1", "探测目标", "确认可达", dir)
	task.TimeoutSec = 20
	task.MaxTurns = 5
	task.MaxWorkers = 1
	task.PlanRounds = 2
	task.EnableExec = false
	if err := StartBackgroundTask(cfg, "", "", nil, task); err != nil {
		t.Fatalf("StartBackgroundTask: %v", err)
	}
	// 等待终态
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if !isTerminal(task.StatusNow()) {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		break
	}
	if task.StatusNow() != StatusDone {
		t.Fatalf("后台任务未达成: %s", task.StatusNow())
	}
	report := TaskStatusReport("task-1", dir)
	if !strings.Contains(report, "done") || !strings.Contains(report, "目标") {
		t.Fatalf("状态报告错误: %s", report)
	}
	// 未知任务
	report2 := TaskStatusReport("nope", "")
	if !strings.Contains(report2, "未找到") {
		t.Fatalf("未知任务报告错误: %s", report2)
	}
}

// TestChatBusyLock：会话级互斥。
func TestChatBusyLock(t *testing.T) {
	if !AcquireChatLock("s1") {
		t.Fatal("首次抢锁应成功")
	}
	if AcquireChatLock("s1") {
		t.Fatal("同会话二次抢锁应失败")
	}
	if !AcquireChatLock("s2") {
		t.Fatal("不同会话应可并行")
	}
	ReleaseChatLock("s1")
	if !AcquireChatLock("s1") {
		t.Fatal("释放后应可再次抢锁")
	}
}

// TestChatTaskStatusTool：task_status 工具输出任务摘要。
func TestChatTaskStatusTool(t *testing.T) {
	task := NewTask("t-x", "d", "g", "")
	task.MarkRunning()
	task.AddIntents([]string{"探测 A"}, nil, nil, 1)
	task.AddFinding(Finding{Title: "SQL 注入", Severity: SevHigh, VulnClass: "SQLi", Evidence: "union select 生效"})
	task.FinishExhausted("无路可走")
	runningTasks.Store("t-x", task)

	s := &ChatSession{ID: "c1", TaskIDs: []string{"t-x"}, TaskDirs: map[string]string{"t-x": ""}}
	env := &ChatEnv{Session: s}
	tool := &taskStatusTool{env: env}
	out, err := tool.Call(context.Background(), map[string]any{}, &RunContext{Task: task, Agent: "assistant"})
	if err != nil {
		t.Fatalf("task_status: %v", err)
	}
	if !strings.Contains(out, "SQL 注入") || !strings.Contains(out, "exhausted") {
		t.Fatalf("状态输出错误: %s", out)
	}
}
