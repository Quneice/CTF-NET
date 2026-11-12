package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"ctfnet/internal/core"
)

// TestAiChatTwoTurns：两轮对话（新会话 → 携带 session_id 续聊），会话持久化与
// 黑榜沉淀随工具链生效。
func TestAiChatTwoTurns(t *testing.T) {
	// 对话助手系统提示无 planner 标记 → 走 workSeq
	workSeq := [][]map[string]any{
		step(toolCall("record_fact", `{"text":"8080 端口开放 nginx","kind":"service"}`)),
		step(textStep("第一轮：已确认 8080 开放 nginx（事实 #1），可以继续探测后台。")),
		step(toolCall("record_fact", `{"text":"/admin 返回 403","kind":"info"}`)),
		step(textStep("第二轮：/admin 返回 403，存在访问控制。")),
	}
	srv := scriptedLLM(t, nil, workSeq)

	regTool, ok := core.DefaultRegistry.Get("ai.pentest.chat")
	if !ok {
		t.Fatal("注册表中找不到 ai.pentest.chat")
	}
	// 会话根定向到临时目录（避免污染真实 DataDir）
	oldDir := core.GetDataDir()
	t.Setenv("CTF_DATA_DIR", t.TempDir())
	defer t.Setenv("CTF_DATA_DIR", oldDir)

	tc := &core.ToolContext{Executor: &core.SubprocessExecutor{}, Workdir: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 第一轮：新会话
	p1, err := regTool.ValidateParams(map[string]any{
		"message": "帮我看看 http://10.0.0.5:8080", "api_key": "k", "base_url": srv.URL,
		"model": "m", "max_turns": 5, "enable_exec": false,
	})
	if err != nil {
		t.Fatalf("ValidateParams: %v", err)
	}
	res1, err := regTool.Run(ctx, p1, tc)
	if err != nil {
		t.Fatalf("第一轮: %v", err)
	}
	if res1.Status != core.StatusSuccess || !strings.Contains(res1.Stdout, "第一轮") {
		t.Fatalf("第一轮回复错误: %s / %s", res1.Status, res1.Stdout)
	}
	sid, _ := res1.Data["session_id"].(string)
	if sid == "" {
		t.Fatalf("第一轮未返回 session_id: %+v", res1.Data)
	}

	// 第二轮：续聊
	p2, err := regTool.ValidateParams(map[string]any{
		"message": "继续探测后台路径", "session_id": sid, "api_key": "k", "base_url": srv.URL,
		"model": "m", "max_turns": 5, "enable_exec": false,
	})
	if err != nil {
		t.Fatalf("ValidateParams2: %v", err)
	}
	res2, err := regTool.Run(ctx, p2, tc)
	if err != nil {
		t.Fatalf("第二轮: %v", err)
	}
	if !strings.Contains(res2.Stdout, "第二轮") {
		t.Fatalf("第二轮回复错误: %s", res2.Stdout)
	}
	if res2.Data["session_id"] != sid {
		t.Fatalf("session_id 不一致: %v vs %v", res2.Data["session_id"], sid)
	}
	// Data 直连工具无 JSON 往返时为 int；经 Wails 绑定序列化后为 float64——两者都接受
	n := 0
	switch v := res2.Data["msg_count"].(type) {
	case float64:
		n = int(v)
	case int:
		n = v
	}
	if n != 4 {
		t.Fatalf("消息数应为 4，得到 %v", res2.Data["msg_count"])
	}
	title, _ := res2.Data["title"].(string)
	if title == "" {
		t.Fatal("会话标题为空")
	}
}

// TestAiChatMissingMessage：必填参数校验。
func TestAiChatMissingMessage(t *testing.T) {
	regTool, ok := core.DefaultRegistry.Get("ai.pentest.chat")
	if !ok {
		t.Fatal("注册表中找不到 ai.pentest.chat")
	}
	if _, err := regTool.ValidateParams(map[string]any{}); err == nil || !strings.Contains(err.Error(), "message") {
		t.Fatalf("期望缺少 message 错误，得到: %v", err)
	}
}
