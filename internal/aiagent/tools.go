package aiagent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// —— 内置工具集 ——
//
// worker 侧：http_request / exec_command / report_finding / add_asset / list_assets /
// record_fact / list_findings / todo_write
// planner 侧：list_intents / add_intents / get_intent_trace / list_findings /
// list_facts / list_assets / prove_goal / goal_met / todo_write
//
// 设计纪律（对齐 ARTEX 的角色约束，原生实现）：
//   - worker 只能派单干活，不能规划（没有 add_intents/prove_goal/goal_met）
//   - planner 只读态势 + 派意图 + 判达成，不做实际攻击（没有 http_request/exec）
//   - report_finding 强制要求可复现证据，杜绝臆造漏洞

const (
	maxIntentBatch   = 4 // 单轮单次 add_intents 上限
	maxHTTPBodyBytes = 8192
	maxExecOutBytes  = 16384
)

// workerTools 返回 worker 工具集。
func workerTools(proxyURL string, enableExec bool) *registry {
	r := newRegistry()
	r.register(&httpRequestTool{proxyURL: proxyURL})
	if enableExec {
		r.register(&execCommandTool{})
	}
	r.register(&reportFindingTool{})
	r.register(&addAssetTool{})
	r.register(&listAssetsTool{})
	r.register(&recordFactTool{})
	r.register(&listFindingsTool{})
	r.register(&todoWriteTool{})
	return r
}

// plannerTools 返回 planner 工具集。
func plannerTools() *registry {
	r := newRegistry()
	r.register(&listIntentsTool{})
	r.register(&addIntentsTool{})
	r.register(&getIntentTraceTool{})
	r.register(&listFindingsTool{})
	r.register(&listFactsTool{})
	r.register(&listAssetsTool{})
	r.register(&proveGoalTool{})
	r.register(&goalMetTool{})
	r.register(&todoWriteTool{})
	return r
}

// —— 通用 ——

type todoWriteTool struct{}

func (t *todoWriteTool) Name() string { return "todo_write" }
func (t *todoWriteTool) Description() string {
	return "维护待办清单，规划/记录当前计划的步骤顺序。items 为完整清单快照（覆盖式），每项格式 {text, done}。"
}
func (t *todoWriteTool) InputSchema() map[string]any {
	return objSchema(map[string]any{
		"items": map[string]any{
			"type":        "array",
			"description": "完整待办清单",
			"items":       objSchema(map[string]any{"text": prop("string", "事项"), "done": prop("boolean", "是否完成")}, []string{"text"}),
		},
	}, []string{"items"})
}

func (t *todoWriteTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	items, _ := args["items"].([]any)
	var sb strings.Builder
	sb.WriteString("待办清单已更新:\n")
	for i, it := range items {
		m, _ := it.(map[string]any)
		done := argBool(m, "done")
		mark := " "
		if done {
			mark = "✓"
		}
		fmt.Fprintf(&sb, "%s [%s] %s\n", mark, itoa(i+1), argString(m, "text"))
	}
	if len(items) == 0 {
		sb.WriteString("(空)")
	}
	return sb.String(), nil
}

// —— worker 工具 ——

type httpRequestTool struct{ proxyURL string }

func (t *httpRequestTool) Name() string { return "http_request" }
func (t *httpRequestTool) Description() string {
	return "发送 HTTP 请求并返回状态行/响应头/正文前 8KB。用于侦察、验证注入点、复现漏洞。仅面向授权目标；请求必须带足够超时，重定向谨慎跟随。"
}
func (t *httpRequestTool) InputSchema() map[string]any {
	return objSchema(map[string]any{
		"method":           propEnum("string", "HTTP 方法", []string{"GET", "POST", "HEAD", "OPTIONS", "PUT", "DELETE"}),
		"url":              prop("string", "目标 URL（必须完整，含协议）"),
		"headers":          map[string]any{"type": "object", "description": "请求头（map）", "additionalProperties": prop("string", "")},
		"body":             prop("string", "请求体（POST/PUT 用）"),
		"timeout":          prop("integer", "超时秒数（默认 30）"),
		"follow_redirects": prop("boolean", "是否跟随重定向（默认 false，防 SSRF 链意外扩散）"),
	}, []string{"url"})
}

func (t *httpRequestTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	rawURL := argString(args, "url")
	if rawURL == "" {
		return "", fmt.Errorf("缺少 url 参数")
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("url 无效（仅支持 http/https）: %s", rawURL)
	}
	method := strings.ToUpper(argString(args, "method"))
	if method == "" {
		method = http.MethodGet
	}
	timeout := argInt(args, "timeout")
	if timeout <= 0 || timeout > 120 {
		timeout = 30
	}
	var body io.Reader
	if b := argString(args, "body"); b != "" {
		body = strings.NewReader(b)
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	if t.proxyURL != "" {
		if pu, err := url.Parse(t.proxyURL); err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(pu)}
		}
	}
	if !argBool(args, "follow_redirects") {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return "", fmt.Errorf("构造请求失败: %v", err)
	}
	if hdrs, ok := args["headers"].(map[string]any); ok {
		for k, v := range hdrs {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPBodyBytes))
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "HTTP/1.1 %s\n", resp.Status)
	for k, vs := range resp.Header {
		if len(vs) > 0 {
			fmt.Fprintf(&sb, "%s: %s\n", k, strings.Join(vs, ", "))
		}
	}
	sb.WriteString("\n")
	sb.Write(data)
	if len(data) >= maxHTTPBodyBytes {
		sb.WriteString("\n…(正文已截断)")
	}
	fmt.Fprintf(&sb, "\n\n[duration=%.2fs size=%d]", time.Since(start).Seconds(), resp.ContentLength)
	return sb.String(), nil
}

type execCommandTool struct{}

func (t *execCommandTool) Name() string { return "exec_command" }
func (t *execCommandTool) Description() string {
	return "在本机执行 shell 命令（如 curl/nslookup/python 脚本验证）。返回 stdout+stderr 前 16KB。超时必填；仅对授权目标使用；禁止破坏性命令（rm -rf/格式化/关机等）。"
}
func (t *execCommandTool) InputSchema() map[string]any {
	return objSchema(map[string]any{
		"command": prop("string", "要执行的命令"),
		"timeout": prop("integer", "超时秒数（默认 60，最大 300）"),
	}, []string{"command", "timeout"})
}

func (t *execCommandTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	if tc.Exec == nil {
		return "", fmt.Errorf("exec_command 未启用（任务未授权本地命令执行）")
	}
	cmd := argString(args, "command")
	if strings.TrimSpace(cmd) == "" {
		return "", fmt.Errorf("缺少 command 参数")
	}
	if strings.Contains(strings.ToLower(cmd), "rm -rf") || strings.Contains(strings.ToLower(cmd), "format ") {
		return "", fmt.Errorf("拒绝执行破坏性命令")
	}
	timeout := argInt(args, "timeout")
	if timeout <= 0 || timeout > 300 {
		timeout = 60
	}
	stdout, stderr, err := tc.Exec(ctx, cmd, tc.WorkDir, timeout)
	if err != nil {
		// err 通常已内嵌 stderr（exit N: <stderr>），这里只补 stdout 与缺失的 stderr，
		// 避免同一段报错在模型上下文里出现两遍（2026-11-12 实测）。
		msg := fmt.Sprintf("命令失败: %v\n%s", err, stdout)
		if stderr != "" && !strings.Contains(err.Error(), stderr) {
			msg += "\n[stderr]\n" + stderr
		}
		return "", fmt.Errorf("%s", msg)
	}
	var sb strings.Builder
	if stdout != "" {
		sb.WriteString(stdout)
	}
	if stderr != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("[stderr]\n")
		sb.WriteString(stderr)
	}
	if sb.Len() == 0 {
		sb.WriteString("(无输出，命令执行成功)")
	}
	return truncateRune(sb.String(), maxExecOutBytes), nil
}

func truncateRune(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "\n…(输出已截断)"
}

type reportFindingTool struct{}

func (t *reportFindingTool) Name() string { return "report_finding" }
func (t *reportFindingTool) Description() string {
	return "上报一个已确认的漏洞。纪律：必须真实触发过、有可复现证据（evidence 填实际请求/响应或命令输出节选）才能上报；禁止臆测或转述扫描器结论。"
}
func (t *reportFindingTool) InputSchema() map[string]any {
	return objSchema(map[string]any{
		"title":      prop("string", "漏洞标题（一句话）"),
		"vuln_class": prop("string", "漏洞分类（如 SQL 注入 / 任意文件读取 / 弱口令 / XSS）"),
		"severity":   propEnum("string", "严重度", []string{"critical", "high", "medium", "low", "info"}),
		"summary":    prop("string", "影响与成因简述"),
		"evidence":   prop("string", "可复现证据：实际请求/响应、命令输出节选（必填，缺证据将被拒绝）"),
		"poc":        prop("string", "复现步骤（可选）"),
		"asset_ids":  map[string]any{"type": "array", "items": prop("integer", ""), "description": "关联资产 ID"},
	}, []string{"title", "vuln_class", "severity", "summary", "evidence"})
}

func (t *reportFindingTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	if strings.TrimSpace(argString(args, "evidence")) == "" {
		return "", fmt.Errorf("拒绝上报：evidence 必填。只有真实触发过并保留请求/响应或输出证据的漏洞才可上报；没有证据就继续验证，不要臆造。")
	}
	sev, _ := validSeverity(argString(args, "severity"))
	f := Finding{
		Title:     argString(args, "title"),
		VulnClass: argString(args, "vuln_class"),
		Severity:  sev,
		Summary:   argString(args, "summary"),
		Evidence:  argString(args, "evidence"),
		PoC:       argString(args, "poc"),
	}
	if tc.Intent != nil {
		f.IntentID = tc.Intent.ID
	}
	if ids, ok := args["asset_ids"].([]any); ok {
		for _, id := range ids {
			if n, ok := id.(float64); ok {
				f.AssetIDs = append(f.AssetIDs, int(n))
			}
		}
	}
	id := tc.Task.AddFinding(f)
	tc.Task.Log(tc.Agent, "finding", fmt.Sprintf("上报漏洞 #%d [%s] %s", id, f.Severity, f.Title))
	return fmt.Sprintf("漏洞已记录，ID=%d，严重度=%s。这将被规划者复核并可能影响后续意图。", id, f.Severity), nil
}

type addAssetTool struct{}

func (t *addAssetTool) Name() string { return "add_asset" }
func (t *addAssetTool) Description() string {
	return "登记侦察发现的新资产（URL/域名/IP/服务），供后续意图引用。"
}
func (t *addAssetTool) InputSchema() map[string]any {
	return objSchema(map[string]any{
		"type":   propEnum("string", "资产类型", []string{"url", "domain", "ip", "service", "other"}),
		"url":    prop("string", "URL（type=url 时）"),
		"domain": prop("string", "域名"),
		"ip":     prop("string", "IP 地址"),
		"props":  map[string]any{"type": "object", "description": "附加属性（如 port/title/technology）", "additionalProperties": prop("string", "")},
	}, []string{"type"})
}

func (t *addAssetTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	a := Asset{Type: argString(args, "type"), URL: argString(args, "url"),
		Domain: argString(args, "domain"), IP: argString(args, "ip"), AddedBy: tc.Agent}
	if props, ok := args["props"].(map[string]any); ok {
		a.Props = map[string]string{}
		for k, v := range props {
			a.Props[k] = fmt.Sprint(v)
		}
	}
	if a.Type == "" {
		return "", fmt.Errorf("缺少 type 参数")
	}
	id := tc.Task.AddAsset(a, intentIDOf(tc))
	tc.Task.Log(tc.Agent, "asset", fmt.Sprintf("登记资产 #%d [%s] %s", id, a.Type, assetLabel(a)))
	return fmt.Sprintf("资产已登记，ID=%d", id), nil
}

func assetLabel(a Asset) string {
	switch a.Type {
	case "url":
		return a.URL
	case "domain":
		return a.Domain
	case "ip":
		return a.IP
	}
	if a.URL != "" {
		return a.URL
	}
	if a.Domain != "" {
		return a.Domain
	}
	return a.IP
}

type listAssetsTool struct{}

func (t *listAssetsTool) Name() string { return "list_assets" }
func (t *listAssetsTool) Description() string {
	return "列出已登记资产。query 可选，按 URL/域名/IP/属性模糊过滤。"
}
func (t *listAssetsTool) InputSchema() map[string]any {
	return objSchema(map[string]any{"query": prop("string", "模糊过滤关键词（可选）")}, nil)
}

func (t *listAssetsTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	q := strings.ToLower(argString(args, "query"))
	var sb strings.Builder
	tc.Task.mu.Lock()
	for _, a := range tc.Task.Assets {
		label := assetLabel(a)
		if q != "" && !strings.Contains(strings.ToLower(label), q) && !strings.Contains(strings.ToLower(fmt.Sprint(a.Props)), q) {
			continue
		}
		fmt.Fprintf(&sb, "#%d [%s] %s", a.ID, a.Type, label)
		if len(a.Props) > 0 {
			fmt.Fprintf(&sb, " %v", a.Props)
		}
		sb.WriteString("\n")
	}
	tc.Task.mu.Unlock()
	if sb.Len() == 0 {
		return "(无资产)", nil
	}
	return sb.String(), nil
}

type recordFactTool struct{}

func (t *recordFactTool) Name() string { return "record_fact" }
func (t *recordFactTool) Description() string {
	return "记录一条侦察事实（服务指纹/版本/凭据线索/异常行为等），供规划者与后续意图参考。"
}
func (t *recordFactTool) InputSchema() map[string]any {
	return objSchema(map[string]any{
		"text": prop("string", "事实内容（一句话）"),
		"kind": propEnum("string", "类型", []string{"service", "vuln", "credential", "info", "other"}),
	}, []string{"text"})
}

func (t *recordFactTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	if strings.TrimSpace(argString(args, "text")) == "" {
		return "", fmt.Errorf("缺少 text 参数")
	}
	id := tc.Task.AddFact(argString(args, "text"), argString(args, "kind"), tc.Agent, intentIDOf(tc))
	return fmt.Sprintf("事实已记录，ID=%d", id), nil
}

type listFindingsTool struct{}

func (t *listFindingsTool) Name() string { return "list_findings" }
func (t *listFindingsTool) Description() string {
	return "列出全部已上报漏洞（ID/严重度/标题/证据要点）。"
}
func (t *listFindingsTool) InputSchema() map[string]any { return objSchema(nil, nil) }

func (t *listFindingsTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	var sb strings.Builder
	tc.Task.mu.Lock()
	for _, f := range tc.Task.Findings {
		fmt.Fprintf(&sb, "#%d [%s] %s (%s)\n", f.ID, f.Severity, f.Title, f.VulnClass)
		if f.Evidence != "" {
			sb.WriteString("  证据: " + truncateRune(strings.ReplaceAll(f.Evidence, "\n", " "), 120) + "\n")
		}
	}
	tc.Task.mu.Unlock()
	if sb.Len() == 0 {
		return "(暂无漏洞)", nil
	}
	return sb.String(), nil
}

// —— planner 工具 ——

type listIntentsTool struct{}

func (t *listIntentsTool) Name() string { return "list_intents" }
func (t *listIntentsTool) Description() string {
	return "列出全部意图及其状态（open/running/done/exhausted/blocked），用于判重与决定新方向。"
}
func (t *listIntentsTool) InputSchema() map[string]any { return objSchema(nil, nil) }

func (t *listIntentsTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	var sb strings.Builder
	tc.Task.mu.Lock()
	for _, it := range tc.Task.Intents {
		fmt.Fprintf(&sb, "#%d [%s] %s\n", it.ID, it.State, it.Summary)
		if it.Result != "" && it.State != IntentOpen && it.State != IntentRunning {
			sb.WriteString("  → " + truncateRune(strings.ReplaceAll(it.Result, "\n", " "), 160) + "\n")
		}
	}
	tc.Task.mu.Unlock()
	if sb.Len() == 0 {
		return "(暂无意图)", nil
	}
	return sb.String(), nil
}

type addIntentsTool struct{}

func (t *addIntentsTool) Name() string { return "add_intents" }
func (t *addIntentsTool) Description() string {
	return "批量派发新执行意图（一次最多 4 条）。每条 summary 一句话说明：目标地址 + 做什么 + 为什么。禁止与 open/running/最近完成 的意图重复；串行依赖链每次只派发下一步。"
}
func (t *addIntentsTool) InputSchema() map[string]any {
	return objSchema(map[string]any{
		"intents": map[string]any{
			"type": "array",
			"items": objSchema(map[string]any{
				"summary":   prop("string", "意图方向（一句话）"),
				"asset_ids": map[string]any{"type": "array", "items": prop("integer", ""), "description": "关联资产 ID（可选）"},
			}, []string{"summary"}),
		},
		"parent_ids": map[string]any{"type": "array", "items": prop("integer", ""), "description": "父意图 ID（依赖链）"},
	}, []string{"intents"})
}

func (t *addIntentsTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	items, ok := args["intents"].([]any)
	if !ok || len(items) == 0 {
		return "", fmt.Errorf("intents 不能为空")
	}
	if len(items) > maxIntentBatch {
		return "", fmt.Errorf("一次最多派发 %d 条意图（收到 %d 条），请按优先级裁剪", maxIntentBatch, len(items))
	}
	parentIDs := intIDs(args["parent_ids"])
	// 判重：与 open/running 意图语义相同的拒绝
	active := map[string]bool{}
	tc.Task.mu.Lock()
	for _, it := range tc.Task.Intents {
		if it.State == IntentOpen || it.State == IntentRunning {
			active[strings.ToLower(strings.TrimSpace(it.Summary))] = true
		}
	}
	tc.Task.mu.Unlock()

	var summaries []string
	var assetIDs []int
	for i, it := range items {
		m, _ := it.(map[string]any)
		s := strings.TrimSpace(argString(m, "summary"))
		if s == "" {
			return "", fmt.Errorf("第 %d 条意图缺少 summary", i+1)
		}
		if active[strings.ToLower(s)] {
			return "", fmt.Errorf("意图 %q 与现有 open/running 意图重复，拒绝整批；请改方向或等它完成", s)
		}
		active[strings.ToLower(s)] = true
		summaries = append(summaries, s)
		assetIDs = intIDs(m["asset_ids"])
	}
	ids := tc.Task.AddIntents(summaries, parentIDs, assetIDs, tc.Task.roundsSnapshot())
	for i, id := range ids {
		tc.Task.Log(tc.Agent, "plan", fmt.Sprintf("派发意图 #%d: %s", id, summaries[i]))
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "已派发 %d 条意图，ID=%v。worker 将立即领取执行。", len(ids), ids)
	return sb.String(), nil
}

func intIDs(v any) []int {
	items, _ := v.([]any)
	var out []int
	for _, it := range items {
		if n, ok := it.(float64); ok {
			out = append(out, int(n))
		}
	}
	return out
}

type getIntentTraceTool struct{}

func (t *getIntentTraceTool) Name() string { return "get_intent_trace" }
func (t *getIntentTraceTool) Description() string {
	return "查看某意图的执行轨迹与结果（exhausted/blocked 意图重试前必看，避免无理由重试同一条路）。"
}
func (t *getIntentTraceTool) InputSchema() map[string]any {
	return objSchema(map[string]any{"intent_id": prop("integer", "意图 ID")}, []string{"intent_id"})
}

func (t *getIntentTraceTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	id := argInt(args, "intent_id")
	tc.Task.mu.Lock()
	defer tc.Task.mu.Unlock()
	for _, it := range tc.Task.Intents {
		if it.ID != id {
			continue
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "#%d [%s] %s\n", it.ID, it.State, it.Summary)
		for _, line := range it.Trace {
			sb.WriteString("  · " + line + "\n")
		}
		if it.Result != "" {
			sb.WriteString("结果: " + it.Result + "\n")
		} else {
			sb.WriteString("(无轨迹记录)\n")
		}
		return sb.String(), nil
	}
	return "", fmt.Errorf("意图 ID 不存在: %d", id)
}

type listFactsTool struct{}

func (t *listFactsTool) Name() string { return "list_facts" }
func (t *listFactsTool) Description() string {
	return "列出全部侦察事实（服务/凭据线索等），用于规划新方向。"
}
func (t *listFactsTool) InputSchema() map[string]any { return objSchema(nil, nil) }

func (t *listFactsTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	var sb strings.Builder
	tc.Task.mu.Lock()
	for _, f := range tc.Task.Facts {
		kind := f.Kind
		if kind == "" {
			kind = "info"
		}
		fmt.Fprintf(&sb, "#%d [%s] %s\n", f.ID, kind, f.Text)
	}
	tc.Task.mu.Unlock()
	if sb.Len() == 0 {
		return "(暂无事实)", nil
	}
	return sb.String(), nil
}

type proveGoalTool struct{}

func (t *proveGoalTool) Name() string { return "prove_goal" }
func (t *proveGoalTool) Description() string {
	return "标记某个目标已达成。必须附达成方式与证据要点（如漏洞 ID、获取的权限/flag），禁止无证据断言。全部目标达成后任务终态为 done。"
}
func (t *proveGoalTool) InputSchema() map[string]any {
	return objSchema(map[string]any{
		"goal_id":          prop("integer", "目标 ID"),
		"reason":           prop("string", "达成方式简述"),
		"evidence_summary": prop("string", "证据要点（引用漏洞 ID/事实 ID）"),
	}, []string{"goal_id", "reason", "evidence_summary"})
}

func (t *proveGoalTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	id := argInt(args, "goal_id")
	if id <= 0 {
		return "", fmt.Errorf("缺少 goal_id")
	}
	if err := tc.Task.ProveGoal(id, argString(args, "reason"), argString(args, "evidence_summary"), intentIDOf(tc)); err != nil {
		return "", err
	}
	tc.Task.Log(tc.Agent, "goal", fmt.Sprintf("目标 #%d 标记达成: %s", id, argString(args, "reason")))
	if tc.Task.allGoalsMet() {
		tc.Task.FinishDone("全部目标已达成")
		return "目标已标记达成。全部目标均已完成，任务将结束。", nil
	}
	return fmt.Sprintf("目标 #%d 已标记达成（剩余目标继续）。", id), nil
}

type goalMetTool struct{}

func (t *goalMetTool) Name() string { return "goal_met" }
func (t *goalMetTool) Description() string {
	return "宣布渗透目标整体达成（任务终态 done）。必须在有充分证据（漏洞上报/事实/权限证明）时才能调用。"
}
func (t *goalMetTool) InputSchema() map[string]any {
	return objSchema(map[string]any{"reason": prop("string", "达成总结与证据要点")}, []string{"reason"})
}

func (t *goalMetTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	if strings.TrimSpace(argString(args, "reason")) == "" {
		return "", fmt.Errorf("reason 必填：必须说明达成依据")
	}
	tc.Task.FinishDone(argString(args, "reason"))
	tc.Task.Log(tc.Agent, "goal", "宣布目标达成: "+argString(args, "reason"))
	return "已宣布目标达成，任务结束。", nil
}
