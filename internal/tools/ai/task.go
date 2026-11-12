// ai.pentest.task —— AI 自动化渗透任务（一级分类 ai.pentest，原生引擎 internal/aiagent）。
//
// 能力来源：提取自 ARTEX v1.0.4 的 AI 自动化渗透与编排理念（planner/worker 双角色
// agent、意图队列、工具目录、YAML 配方、证据纪律），为本项目全新实现，不含其源码。
//
// 运行模型：
//
//	planner（规划者）读态势 → 批量派发意图（≤4/次，判重）→ worker（执行者）领取
//	单条意图，用工具集（http_request/exec_command/配方/写黑板）验证 → 产出写回黑板
//	→ 全部空闲后唤醒下一轮规划 → prove_goal/goal_met 判达成 → 终态。
//
// 终态：done（目标达成）/ exhausted（意图耗尽）/ failed（LLM 等致命错误）/
// timeout（超时）/ stopped（取消）。完整状态与活动流水持久化在任务工作目录
// task.json，可事后复盘。
package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ctfnet/internal/aiagent"
	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&AiTaskTool{BaseTool: core.BaseTool{
		NameAttr:        "task",
		CategoryAttr:    core.Category("ai.pentest"),
		VersionAttr:     "1.0.0",
		DescriptionAttr: "AI 自动化渗透任务：LLM 规划者+执行者双角色自主侦察与漏洞验证（内置工具调用，支持智谱 GLM/DeepSeek/OpenAI 兼容 API）",
		TimeoutAttr:     3600,
		ParamsAttr: []core.ParamSpec{
			{Name: "goal", Type: "str", Required: true, Description: "渗透目标：目标地址 + 期望成果，如「http://192.168.1.10:8080 的 Web 服务，找出可利用漏洞并证明」"},
			{Name: "description", Type: "str", Required: false, Description: "任务描述（可选，默认取 goal）"},
			{Name: "target_hint", Type: "str", Required: false, Description: "已知线索（资产/端口/凭据等，可选），供规划者起步参考"},
			{Name: "timeout_seconds", Type: "int", Required: false, Description: "任务总时限（秒；默认取设置页 AI 配置）"},
			{Name: "max_turns", Type: "int", Required: false, Description: "单会话最大工具轮数（默认取设置页）"},
			{Name: "max_workers", Type: "int", Required: false, Description: "并发 worker 数 1-8（默认取设置页）"},
			{Name: "plan_rounds", Type: "int", Required: false, Description: "规划轮次预算（默认取设置页）"},
			{Name: "api_key", Type: "password", Required: false, Description: "LLM API Key（默认取设置页 AI 配置）"},
			{Name: "base_url", Type: "str", Required: false, Description: "API 地址（OpenAI 兼容，如 https://open.bigmodel.cn/api/paas/v4；默认取设置页）"},
			{Name: "model", Type: "str", Required: false, Description: "模型名（如 glm-5.3-flash；默认取设置页）"},
			{Name: "enable_exec", Type: "bool", Required: false, Description: "是否允许 worker 执行本地命令（默认取设置页，true=允许）"},
		},
		Runner: runAiTask,
	}})
}

// AiTaskTool AI 自动化渗透任务工具。
type AiTaskTool struct {
	core.BaseTool
}

// —— 参数解析（参数优先，缺省回填设置页 ai 组）——

func paramStr(params map[string]any, key, fallback string) string {
	if v, ok := params[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func paramInt(params map[string]any, key string, fallback int) int {
	switch v := params[key].(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	}
	return fallback
}

func paramBool(params map[string]any, key string, fallback bool) bool {
	if v, ok := params[key].(bool); ok {
		return v
	}
	return fallback
}

// makeExecFunc 把 core.Executor 适配成 aiagent.ExecFunc（shell 语义：Windows 走
// aiagent.RunWindowsCmd（cmd /S /C 原生命令行，防引号破坏），其余走 sh -c）。
func makeExecFunc(exec core.Executor, fallbackDir string) aiagent.ExecFunc {
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

func runAiTask(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	start := time.Now()
	ac := core.GetAppConfig()

	// 参数 > 设置页 ai 组 > 内置默认
	goal := paramStr(params, "goal", "")
	desc := paramStr(params, "description", goal)
	cfg := aiagent.DefaultConfig()
	cfg.APIKey = paramStr(params, "api_key", ac.Ai.APIKey)
	cfg.BaseURL = paramStr(params, "base_url", ac.Ai.BaseURL)
	cfg.Model = paramStr(params, "model", ac.Ai.Model)
	cfg.ReasoningEffort = ac.Ai.ReasoningEffort
	cfg.MaxTurns = paramInt(params, "max_turns", ac.Ai.MaxTurns)
	cfg.MaxWorkers = paramInt(params, "max_workers", ac.Ai.MaxWorkers)
	cfg.PlanRounds = paramInt(params, "plan_rounds", ac.Ai.PlanRounds)
	cfg.Timeout = paramInt(params, "timeout_seconds", ac.Ai.Timeout)
	cfg.EnableExec = paramBool(params, "enable_exec", ac.Ai.EnableExec)

	if strings.TrimSpace(goal) == "" {
		return nil, fmt.Errorf("缺少必需参数: goal")
	}

	// 工作目录：任务状态落 TaskContext.Workdir（无则临时目录）
	workdir := tc.Workdir
	if strings.TrimSpace(workdir) == "" {
		workdir = filepath.Join(os.TempDir(), "ctfnet-ai-"+time.Now().Format("20060102-150405"))
	}
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return nil, fmt.Errorf("创建工作目录失败: %w", err)
	}

	// 任务黑板
	id := "ai-" + time.Now().Format("20060102-150405")
	task := aiagent.NewTask(id, desc, goal, workdir)
	task.Description = desc
	task.Hint = paramStr(params, "target_hint", "")
	task.TimeoutSec = cfg.Timeout
	task.MaxTurns = cfg.MaxTurns
	task.MaxWorkers = cfg.MaxWorkers
	task.PlanRounds = cfg.PlanRounds
	task.EnableExec = cfg.EnableExec

	// 编排器：本地命令执行走 core.Executor（enable_exec 时提供）
	var execFunc aiagent.ExecFunc
	if cfg.EnableExec {
		execFunc = aiagent.ExecWithProxyEnv(makeExecFunc(tc.Executor, workdir), ac.Proxy.URL)
	}
	recipesDir := filepath.Join(core.GetDataDir(), "aiagent", "recipes")
	orch, err := aiagent.NewOrchestrator(cfg, ac.Proxy.URL, recipesDir, execFunc)
	if err != nil {
		return nil, fmt.Errorf("初始化编排器失败: %w", err)
	}
	if tc.ReportProgress != nil {
		orch.SetProgress(tc.ReportProgress)
	}

	// 注册运行中任务：UI 可对任务做暂停/恢复/停止/舵向/重跑
	taskCtx, cancel := context.WithCancel(ctx)
	aiagent.RegisterRunningTask(task, cancel)
	runErr := orch.Run(taskCtx, task)
	aiagent.UnregisterRunningTask(task.ID)
	cancel()
	// 镜像 task.json 到统一任务根，AI 仪表盘/发现/资产页可扫描到
	_ = aiagent.MirrorTaskJSON(workdir, filepath.Join(aiagent.DefaultTaskRoot(core.GetDataDir()), id))

	// —— 组装结果 ——
	snap := task.Snapshot()
	data := map[string]any{
		"status":      snap.Status,
		"workdir":     workdir,
		"findings":    findingsDict(snap.Findings),
		"facts":       factsDict(snap.Facts),
		"assets":      assetsDict(snap.Assets),
		"intents":     intentsDict(snap.Intents),
		"goals":       goalsDict(snap.Goals),
		"activities":  activitiesDict(snap.Activities),
		"summary":     snap.Summary,
		"description": desc,
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务状态: %s\n", snap.Status))
	if snap.Summary != "" {
		sb.WriteString("结论: " + snap.Summary + "\n")
	}
	sb.WriteString(fmt.Sprintf("\n目标: %s\n", snap.GoalText))
	fmt.Fprintf(&sb, "意图: 共 %d 条（done=%d, exhausted=%d, blocked=%d）\n",
		len(snap.Intents), countState(snap.Intents, "done"),
		countState(snap.Intents, "exhausted"), countState(snap.Intents, "blocked"))
	fmt.Fprintf(&sb, "漏洞: %d 个 | 事实: %d 条 | 资产: %d 个\n",
		len(snap.Findings), len(snap.Facts), len(snap.Assets))
	if len(snap.Findings) > 0 {
		sb.WriteString("\n—— 漏洞清单 ——\n")
		for _, f := range snap.Findings {
			fmt.Fprintf(&sb, "[%s] %s (%s)\n  摘要: %s\n  证据: %s\n",
				f.Severity, f.Title, f.VulnClass, f.Summary, truncateStr(f.Evidence, 200))
		}
	}
	sb.WriteString("\n—— 执行轨迹（最近 20 条）——\n")
	acts := snap.Activities
	if len(acts) > 20 {
		acts = acts[len(acts)-20:]
	}
	for _, a := range acts {
		fmt.Fprintf(&sb, "[%s] %s\n", a.Agent, a.Summary)
	}
	sb.WriteString(fmt.Sprintf("\n完整状态: %s\\task.json\n", workdir))

	result := &core.ToolResult{
		Status:   core.StatusSuccess,
		ExitCode: 0,
		Stdout:   sb.String(),
		Duration: time.Since(start).Seconds(),
		Data:     data,
	}
	if runErr != nil {
		result.Status = core.StatusError
		result.Stderr = runErr.Error()
	}
	return result, nil
}

// —— 结果序列化辅助 ——

func countState(intents []aiagent.Intent, state string) int {
	n := 0
	for _, it := range intents {
		if it.State == state {
			n++
		}
	}
	return n
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func findingsDict(fs []aiagent.Finding) []map[string]any {
	out := make([]map[string]any, 0, len(fs))
	for _, f := range fs {
		out = append(out, map[string]any{
			"id": f.ID, "title": f.Title, "vuln_class": f.VulnClass,
			"severity": string(f.Severity), "summary": f.Summary,
			"evidence": f.Evidence, "poc": f.PoC, "asset_ids": f.AssetIDs,
		})
	}
	return out
}

func factsDict(fs []aiagent.Fact) []map[string]any {
	out := make([]map[string]any, 0, len(fs))
	for _, f := range fs {
		out = append(out, map[string]any{"id": f.ID, "text": f.Text, "kind": f.Kind, "source": f.Source})
	}
	return out
}

func assetsDict(as []aiagent.Asset) []map[string]any {
	out := make([]map[string]any, 0, len(as))
	for _, a := range as {
		out = append(out, map[string]any{
			"id": a.ID, "type": a.Type, "url": a.URL, "domain": a.Domain, "ip": a.IP,
		})
	}
	return out
}

func intentsDict(is []aiagent.Intent) []map[string]any {
	out := make([]map[string]any, 0, len(is))
	for _, it := range is {
		out = append(out, map[string]any{
			"id": it.ID, "summary": it.Summary, "state": it.State, "result": it.Result,
		})
	}
	return out
}

func goalsDict(gs []aiagent.Goal) []map[string]any {
	out := make([]map[string]any, 0, len(gs))
	for _, g := range gs {
		out = append(out, map[string]any{
			"id": g.ID, "text": g.Text, "met": g.Met, "met_by": g.MetBy, "evidence": g.EvdSum,
		})
	}
	return out
}

func activitiesDict(as []aiagent.Activity) []map[string]any {
	out := make([]map[string]any, 0, len(as))
	for _, a := range as {
		out = append(out, map[string]any{
			"seq": a.Seq, "ts": a.TS.Format("15:04:05"), "agent": a.Agent,
			"kind": a.Kind, "summary": a.Summary,
		})
	}
	return out
}
