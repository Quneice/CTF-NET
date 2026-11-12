package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"ctfnet/internal/core"
)

// mockFlask 模拟 Flask 回显端点：POST /hello，注入参数 name。
// render 由各用例注入，模拟 Jinja2 渲染 / 黑名单 / 500 等行为。
type mockFlask struct {
	t      *testing.T
	render func(payload string) string
	mu     sync.Mutex
	// 服务端观察：收到的表单（含注入 payload 与固定参数）
	lastForm map[string]string
	hits     map[string]int // oracle 计数（状态机用）
}

func (m *mockFlask) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	if m.lastForm == nil {
		m.lastForm = map[string]string{}
	}
	_ = r.ParseForm()
	for k := range r.Form {
		m.lastForm[k] = r.Form.Get(k)
	}
	payload := r.Form.Get("name")
	m.mu.Unlock()
	body := m.render(payload)
	status := 200
	if strings.HasPrefix(body, "STATUS:") {
		var s int
		if _, err := fmt.Sscanf(body, "STATUS:%d", &s); err == nil {
			status = s
			body = strings.TrimPrefix(body, fmt.Sprintf("STATUS:%d", s))
		}
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func newSstiServer(t *testing.T, render func(payload string) string) (*httptest.Server, *mockFlask) {
	m := &mockFlask{t: t, render: render, hits: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(m.handler))
	t.Cleanup(srv.Close)
	return srv, m
}

// runJ2 走完整管线（ValidateParams → Run），与 app.go 行为一致。
func runJ2(t *testing.T, params map[string]any) (*core.ToolResult, map[string]any) {
	t.Helper()
	tool, ok := core.DefaultRegistry.Get("web.ssti_jinja2")
	if !ok {
		t.Fatal("web.ssti_jinja2 未注册")
	}
	validated, err := tool.ValidateParams(params)
	if err != nil {
		t.Fatalf("ValidateParams: %v", err)
	}
	res, err := tool.Run(context.Background(), validated, &core.ToolContext{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res, res.Data
}

// renderJ2 基础 Jinja2 回显：7*7→49、7*'7'→7777777、config→配置 repr、其余原样回显。
func renderJ2(payload string) string {
	switch {
	case strings.Contains(payload, `7*'7'`):
		return "7777777"
	case strings.Contains(payload, "7*7"):
		return "49"
	case strings.Contains(payload, "config"):
		return "<Config {'SECRET_KEY': 'devkey'}>"
	default:
		return payload
	}
}

func TestSstiToolRegistered(t *testing.T) {
	tool, ok := core.DefaultRegistry.Get("web.ssti_jinja2")
	if !ok {
		t.Fatal("web.ssti_jinja2 未注册")
	}
	if tool.Version() != "0.2.0" {
		t.Fatalf("version = %q, want 0.2.0", tool.Version())
	}
	if tool.Timeout() != 60 {
		t.Fatalf("timeout = %d, want 60", tool.Timeout())
	}
	if !strings.Contains(tool.Description(), "SSTI") {
		t.Fatalf("description = %q", tool.Description())
	}
	// 10 个基参 + blocked_words/max_length/interval = 13
	if len(tool.Params()) != 13 {
		t.Fatalf("params = %d, want 13", len(tool.Params()))
	}
}

func TestSstiValidate(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("web.ssti_jinja2")
	// 空参数：缺 target_url/raw_request
	if _, err := tool.ValidateParams(map[string]any{}); err == nil ||
		!strings.Contains(err.Error(), "需要 target_url（靶机端点）或 raw_request（原始数据包）之一") {
		t.Fatalf("空参数应报缺 target_url，got %v", err)
	}
	// 缺 param
	if _, err := tool.ValidateParams(map[string]any{"target_url": "http://x/"}); err == nil ||
		!strings.Contains(err.Error(), "缺少注入参数名 param") {
		t.Fatalf("缺 param 应报错，got %v", err)
	}
	// 合法最小集：mode 填充默认 exec，interval 等保持缺省
	v, err := tool.ValidateParams(map[string]any{"target_url": "http://x/", "param": "name"})
	if err != nil {
		t.Fatalf("合法参数被拒: %v", err)
	}
	if v["mode"] != "exec" {
		t.Fatalf("mode 默认值 = %v, want exec", v["mode"])
	}
}

func TestSstiDetectBasic(t *testing.T) {
	srv, _ := newSstiServer(t, renderJ2)
	res, data := runJ2(t, map[string]any{
		"target_url": srv.URL + "/hello", "param": "name", "mode": "detect",
	})
	if res.Status != core.StatusSuccess {
		t.Fatalf("status = %v, stderr=%s", res.Status, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "判定: 目标为 Jinja2/Flask（7*7 与 7*'7' 双命中）") {
		t.Fatalf("缺少双命中结论:\n%s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "[探针] {{7*7}} → HTTP 200 命中=是") {
		t.Fatalf("缺少探针命中行:\n%s", res.Stdout)
	}
	if b, _ := data["success"].(bool); !b {
		t.Fatalf("success = %v", data["success"])
	}
}

func TestSstiDetectFallback(t *testing.T) {
	// 数字黑名单：数字探针全被过滤 → 无数字 fallback 探针
	render := func(p string) string {
		if strings.HasPrefix(p, "__ssti_baseline_") || strings.ContainsAny(p, "0123456789") {
			return "blocked"
		}
		if strings.Contains(p, "~") {
			return "aa"
		}
		if strings.Contains(p, "request") {
			return "Request"
		}
		return p
	}
	srv, _ := newSstiServer(t, render)
	res, _ := runJ2(t, map[string]any{
		"target_url": srv.URL + "/hello", "param": "name", "mode": "detect",
	})
	if !strings.Contains(res.Stdout, "[基线] 标记串被过滤（blocked）→ 注入点存在黑名单/长度过滤，改用空输入做渲染基线") {
		t.Fatalf("基线应切换到空输入:\n%s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "[探测] 探针被过滤拦截，切换无数字/无关键词探针继续") {
		t.Fatalf("缺少 fallback 提示:\n%s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "判定: Jinja2/Twig 语法激活（无数字探针命中；数字探针被过滤，注入点存在黑名单）") {
		t.Fatalf("缺少 fallback 结论:\n%s", res.Stdout)
	}
}

func TestSstiExecSuccess(t *testing.T) {
	// 模拟 open 链成功：popen('echo M;cmd;echo M') → M\n输出\nM
	render := func(p string) string {
		if strings.Contains(p, "popen") {
			i := strings.Index(p, "ssti_")
			if i < 0 {
				return p
			}
			m := p[i : i+13] // ssti_ + 8 位
			return m + "\nuid=1000(ctf) gid=1000(ctf)\n" + m
		}
		return p
	}
	srv, _ := newSstiServer(t, render)
	res, data := runJ2(t, map[string]any{
		"target_url": srv.URL + "/hello", "param": "name", "mode": "exec", "command": "id",
	})
	if res.Status != core.StatusSuccess {
		t.Fatalf("status = %v, stderr=%s", res.Status, res.Stderr)
	}
	if b, _ := data["success"].(bool); !b {
		t.Fatalf("success = %v:\n%s", data["success"], res.Stdout)
	}
	if data["chain"] != "self" {
		t.Fatalf("chain = %v, want self（第一入口链首模板命中）", data["chain"])
	}
	if data["output"] != "uid=1000(ctf) gid=1000(ctf)" {
		t.Fatalf("output = %v", data["output"])
	}
	if !strings.Contains(res.Stdout, "  └─ 命中！命令输出:") ||
		!strings.Contains(res.Stdout, "     uid=1000(ctf) gid=1000(ctf)") {
		t.Fatalf("缺少命中输出行:\n%s", res.Stdout)
	}
}

func TestSstiExecBlockedGuidance(t *testing.T) {
	// __globals__ 全被封：对象图链提前跳过 → 引导 read_file
	render := func(p string) string {
		if strings.Contains(p, "__globals__") || strings.Contains(p, "popen") {
			return "blocked"
		}
		return p
	}
	srv, _ := newSstiServer(t, render)
	res, data := runJ2(t, map[string]any{
		"target_url": srv.URL + "/hello", "param": "name", "mode": "exec", "command": "id",
	})
	if res.Status != core.StatusError {
		t.Fatalf("status = %v（exec 失败应为 error）", res.Status)
	}
	if b, _ := data["success"].(bool); b {
		t.Fatal("success 应为 false")
	}
	for _, want := range []string{
		"[exec] 连续命中黑名单/长度过滤，同类对象图链提前跳过（节省请求）",
		"[exec] 黑名单/长度过滤下跳过子类枚举（class/mro/subclasses 大概率被封）",
		"[exec] 观察到的过滤信号: blocked",
		"[exec] 提示: 黑名单/长度过滤通常封死对象图 RCE 桥（__init__/__globals__/__class__），最短 RCE 链也要 86+ 字符；但 open_resource 链不需要这些桥——",
		"（open_resource 读链最短 57 字符）。",
	} {
		if !strings.Contains(res.Stdout, want) {
			t.Fatalf("缺少 %q:\n%s", want, res.Stdout)
		}
	}
}

func TestSstiReadFileBuiltins(t *testing.T) {
	// exec 链被封（popen/attr 链 → blocked）；open_resource 链报模板错误；
	// builtins.open 成功 → 走阶段 2，flag 自动提取
	render := func(p string) string {
		if strings.Contains(p, "popen") || strings.Contains(p, "attr(") {
			return "blocked"
		}
		if strings.Contains(p, "open_resource") {
			return "TemplateSyntaxError"
		}
		if strings.Contains(p, ".open('") {
			path := strings.SplitN(strings.SplitN(p, ".open('", 2)[1], "')", 2)[0]
			switch path {
			case "/flag.txt":
				return "flag{jinja2_read_test}"
			case "/etc/passwd":
				return "root:x:0:0:root:/root:/bin/bash"
			}
			return "404"
		}
		return p
	}
	srv, _ := newSstiServer(t, render)
	res, data := runJ2(t, map[string]any{
		"target_url": srv.URL + "/hello", "param": "name", "mode": "read_file",
		"file_path": "/flag.txt",
	})
	if res.Status != core.StatusSuccess {
		t.Fatalf("status = %v, stderr=%s\n%s", res.Status, res.Stderr, res.Stdout)
	}
	if data["method"] != "builtins.open" {
		t.Fatalf("method = %v, want builtins.open\n%s", data["method"], res.Stdout)
	}
	if data["flag"] != "flag{jinja2_read_test}" {
		t.Fatalf("flag = %v", data["flag"])
	}
	if !strings.Contains(res.Stdout, "[read] 阶段 2: builtins.open 对象图读取") ||
		!strings.Contains(res.Stdout, "  └─ 内容: flag{jinja2_read_test}") {
		t.Fatalf("缺少阶段 2 报告:\n%s", res.Stdout)
	}
}

func TestSstiReadOracleReread(t *testing.T) {
	// 阶段 3 直接读失败一次 → 阶段 4 oracle 泄露真实路径 → 自动重读成功
	var m *mockFlask
	render := func(p string) string {
		if strings.Contains(p, "popen") || strings.Contains(p, "attr(") {
			return "blocked"
		}
		if strings.Contains(p, "open_resource(") && strings.Contains(p, "/app/flag.txt") {
			// oracle（不带 read/__next__）→ 泄露 TextIOWrapper repr（含 name= 属性）；
			// 读模板 → 泄露前失败，泄露后成功
			if !strings.Contains(p, ".read") && !strings.Contains(p, "__next__") {
				m.mu.Lock()
				m.hits["oracle"]++
				m.mu.Unlock()
				return "<_io.TextIOWrapper name='/app/flag.txt' mode='r' encoding='UTF-8'>"
			}
			m.mu.Lock()
			seen := m.hits["oracle"] > 0
			m.mu.Unlock()
			if !seen {
				return "TemplateSyntaxError"
			}
			return "flag{oracle_leaked}"
		}
		if strings.Contains(p, "open_resource") {
			return "TemplateSyntaxError"
		}
		return p
	}
	srv, m := newSstiServer(t, render)
	res, data := runJ2(t, map[string]any{
		"target_url": srv.URL + "/hello", "param": "name", "mode": "read_file",
		"file_path": "/app/flag.txt", "blocked_words": "open_resource",
	})
	if res.Status != core.StatusSuccess {
		t.Fatalf("status = %v, stderr=%s\n%s", res.Status, res.Stderr, res.Stdout)
	}
	if data["method"] != "open_resource+oracle" {
		t.Fatalf("method = %v, want open_resource+oracle\n%s", data["method"], res.Stdout)
	}
	if data["leaked_path"] != "/app/flag.txt" {
		t.Fatalf("leaked_path = %v", data["leaked_path"])
	}
	if !strings.Contains(res.Stdout, "  └─ 泄露真实路径: /app/flag.txt") {
		t.Fatalf("缺少泄露行:\n%s", res.Stdout)
	}
}

func TestSstiRawRequestMode(t *testing.T) {
	srv, m := newSstiServer(t, renderJ2)
	raw := fmt.Sprintf("POST /hello HTTP/1.1\nHost: %s\nContent-Type: application/x-www-form-urlencoded\n\nname=abc&team=blue", strings.TrimPrefix(srv.URL, "http://"))
	res, data := runJ2(t, map[string]any{
		"mode": "detect", "raw_request": raw,
	})
	if !strings.Contains(res.Stdout, "[原始包] POST http://"+strings.TrimPrefix(srv.URL, "http://")+"/hello") {
		t.Fatalf("缺少原始包目标行:\n%s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "[原始包] 注入参数: name（未指定，自动选第一个），其余按原值固定附带") {
		t.Fatalf("缺少注入参数行:\n%s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "双命中") {
		t.Fatalf("detect 未命中:\n%s", res.Stdout)
	}
	if b, _ := data["success"].(bool); !b {
		t.Fatalf("success = %v", data["success"])
	}
	// 固定参数原值附带
	m.mu.Lock()
	team := m.lastForm["team"]
	m.mu.Unlock()
	if team != "blue" {
		t.Fatalf("team = %q, want blue", team)
	}
}

func TestSstiCustomPayload(t *testing.T) {
	srv, _ := newSstiServer(t, renderJ2)
	res, data := runJ2(t, map[string]any{
		"target_url": srv.URL + "/hello", "param": "name",
		"payload": "{{7*7}}",
	})
	if res.Status != core.StatusSuccess {
		t.Fatalf("status = %v", res.Status)
	}
	if b, _ := data["success"].(bool); !b {
		t.Fatalf("success = %v:\n%s", data["success"], res.Stdout)
	}
	if data["excerpt"] != "49" {
		t.Fatalf("excerpt = %v, want 49", data["excerpt"])
	}
	if !strings.Contains(res.Stdout, "[自定义 payload] → HTTP 200 判定=ok") {
		t.Fatalf("缺少自定义 payload 行:\n%s", res.Stdout)
	}
}

func TestSstiUnresponsiveAbort(t *testing.T) {
	// 目标持续 502 → 熔断中止
	render := func(string) string { return "STATUS:502 Bad Gateway" }
	srv, _ := newSstiServer(t, render)
	res, data := runJ2(t, map[string]any{
		"target_url": srv.URL + "/hello", "param": "name", "mode": "exec", "command": "id",
	})
	if res.Status != core.StatusError {
		t.Fatalf("status = %v", res.Status)
	}
	if data["aborted"] != "target_unresponsive" {
		t.Fatalf("aborted = %v", data["aborted"])
	}
	if !strings.Contains(res.Stdout, "[中止] 连续多次网关 502/空响应/连接失败，目标疑似下线、重启中或端口不通") {
		t.Fatalf("缺少熔断行:\n%s", res.Stdout)
	}
}

func TestSstiUnknownMode(t *testing.T) {
	srv, _ := newSstiServer(t, renderJ2)
	tool, _ := core.DefaultRegistry.Get("web.ssti_jinja2")
	validated, err := tool.ValidateParams(map[string]any{
		"target_url": srv.URL + "/hello", "param": "name", "mode": "nope",
	})
	if err != nil {
		t.Fatalf("ValidateParams: %v", err)
	}
	res, err := tool.Run(context.Background(), validated, &core.ToolContext{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != core.StatusError || !strings.Contains(res.Stderr, "未知模式: nope") {
		t.Fatalf("status=%v stderr=%q", res.Status, res.Stderr)
	}
}

func TestSstiRawRequestInjectParamMissing(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("web.ssti_jinja2")
	validated, err := tool.ValidateParams(map[string]any{
		"raw_request": "POST /hello HTTP/1.1\nHost: 127.0.0.1\n\nname=abc",
		"param":       "other",
	})
	if err != nil {
		t.Fatalf("ValidateParams: %v", err)
	}
	res, err := tool.Run(context.Background(), validated, &core.ToolContext{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Status != core.StatusError ||
		!strings.Contains(res.Stderr, "注入参数 \"other\" 不在数据包参数中，可选: name") {
		t.Fatalf("status=%v stderr=%q", res.Status, res.Stderr)
	}
}

// —— 纯函数单元测试 ——

func TestSstiQuote(t *testing.T) {
	cases := map[string]string{
		"abc":    "'abc'",
		"a'b":    `"a'b"`,
		`a"b`:    "'a\"b'",
		"a\\b'c": `"a\\b'c"`,
	}
	for in, want := range cases {
		if got := sstiQuote(in); got != want {
			t.Errorf("sstiQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestSstiObfuscateStr(t *testing.T) {
	cases := []struct {
		in, want string
		blocked  []string
	}{
		{"cat /flag.txt", "'cat /'~'f'~'l'~'a'~'g'~'.txt'", []string{"flag"}},
		{"ls", "'l'~'s'", []string{"ls"}},
		{"echo hi", "'echo hi'", []string{"flag"}},
		{"echo hi", "'echo hi'", []string{}},
		{"flag{xx}", "'f'~'l'~'a'~'g'~'{xx}'", []string{"flag"}},
	}
	for _, c := range cases {
		if got := sstiObfuscateStr(c.in, c.blocked); got != c.want {
			t.Errorf("sstiObfuscateStr(%q, %v) = %s, want %s", c.in, c.blocked, got, c.want)
		}
	}
}

func TestSstiSmartSplitPath(t *testing.T) {
	cases := map[string]string{
		"/flag.txt":     "'/flag'~'.txt'",
		"/dir/f.tar.gz": "'/dir/f.tar'~'.gz'",
		".hidden":       "''~'.hidden'",
		"noext":         "'noext'",
	}
	for in, want := range cases {
		if got := sstiSmartSplitPath(in); got != want {
			t.Errorf("sstiSmartSplitPath(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestSstiMatch49(t *testing.T) {
	cases := map[string]bool{
		"49":    true,
		"x49y":  true,
		" 49 ":  true,
		"149":   false,
		"492":   false,
		"1492":  false,
		"a1492": false,
		"":      false,
	}
	for in, want := range cases {
		if got := sstiMatch49(in); got != want {
			t.Errorf("sstiMatch49(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSstiHTMLUnescape(t *testing.T) {
	if got := sstiHTMLUnescape("&#39;a&#39; &lt;b&gt; &#65; &#x2F;"); got != "'a' <b> A /" {
		t.Fatalf("got %q", got)
	}
}

func TestSstiClassify(t *testing.T) {
	cases := []struct {
		body, baseline string
		status         int
		want           string
	}{
		{"[请求失败] x", "", -1, "net_error"},
		{"  ", "", 200, "empty"},
		{"hello", "hello", 200, "unchanged"},
		{"too many requests", "", 200, "rate_limit"},
		{"rate limit!", "", 200, "rate_limit"},
		{"", "", 502, "empty"}, // 空体先于限速判定（与 Python 顺序一致）
		{"blocked by waf", "", 200, "blocked"},
		{"maximum length exceeded", "", 200, "too_long"},
		{"TemplateSyntaxError: x", "", 200, "error"},
		{"fine", "", 200, "ok"},
		{"&#120;", "x", 200, "unchanged"}, // 实体还原后与基线一致
	}
	for _, c := range cases {
		if got := sstiClassify(c.body, c.baseline, c.status); got != c.want {
			t.Errorf("sstiClassify(%q, %q, %d) = %s, want %s", c.body, c.baseline, c.status, got, c.want)
		}
	}
}

func TestSstiExcerptAndExtract(t *testing.T) {
	if got := sstiExcerpt("<p>a  b</p>  c", 10); got != "a b c" {
		t.Fatalf("excerpt = %q", got)
	}
	if out, ok := sstiExtractBetween("x MARK a MARK y", "MARK"); !ok || out != "a" {
		t.Fatalf("extract = %q, %v", out, ok)
	}
	if _, ok := sstiExtractBetween("x MARK a", "MARK"); ok {
		t.Fatal("单 marker 应失败")
	}
}

func TestSstiFits(t *testing.T) {
	if !sstiFits("abc", 0) || !sstiFits("abcd", 4) || sstiFits("abcd", 3) {
		t.Fatal("fits 判断错误")
	}
	// 码点数：中文按 1 字符计
	if !sstiFits("中文", 2) {
		t.Fatal("码点长度应为 2")
	}
}
