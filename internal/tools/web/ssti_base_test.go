package web

import (
	"context"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

func TestSstiPlaceholderRegistry(t *testing.T) {
	cat := core.CatWeb
	seen := map[string]bool{}
	count := 0
	for _, tool := range core.DefaultRegistry.All(&cat) {
		if !strings.HasPrefix(tool.ID(), "web.ssti_") {
			continue
		}
		count++
		seen[tool.ID()] = true
	}
	// 19 个占位符 + 1 个真实实现（ssti_jinja2）
	if count != len(sstiEngineTable)+1 {
		t.Fatalf("注册了 %d 个 ssti 工具，期望 %d（占位符 + ssti_jinja2）", count, len(sstiEngineTable)+1)
	}
	for _, meta := range sstiEngineTable {
		id := "web.ssti_" + meta.Key
		if !seen[id] {
			t.Errorf("缺少注册: %s", id)
		}
		if meta.Key == "jinja2" {
			t.Errorf("ssti_jinja2 是真实实现，不应出现在占位符表中")
		}
	}
}

func TestSstiPlaceholderValidate(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("web.ssti_smarty")
	validated, err := tool.ValidateParams(map[string]any{"target_url": "http://x/", "param": "p"})
	if err != nil {
		t.Fatalf("合法参数校验失败: %v", err)
	}
	if got := paramStr(validated, "mode"); got != "exec" {
		t.Errorf("mode 默认值 = %q, 期望 exec", got)
	}
	if got := paramStr(validated, "method"); got != "POST" {
		t.Errorf("method 默认值 = %q, 期望 POST", got)
	}

	_, err = tool.ValidateParams(map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "需要 target_url") {
		t.Errorf("缺 target_url/raw_request 未报错: %v", err)
	}
	_, err = tool.ValidateParams(map[string]any{"target_url": "http://x/"})
	if err == nil || !strings.Contains(err.Error(), "缺少注入参数名 param") {
		t.Errorf("缺 param 未报错: %v", err)
	}
}

func TestSstiPlaceholderRun(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("web.ssti_fingerprint")
	res, err := tool.Run(context.Background(),
		map[string]any{"target_url": "http://x/", "param": "p"},
		&core.ToolContext{})
	if err != nil {
		t.Fatalf("run 出错: %v", err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("status = %s", res.Status)
	}
	if !strings.Contains(res.Stdout, "[占位符] SSTI 自动指纹识别") {
		t.Errorf("输出缺少引擎名: %q", res.Stdout)
	}
	if !strings.Contains(res.Stdout, `"target_url": "http://x/"`) {
		t.Errorf("输出缺少参数 JSON: %q", res.Stdout)
	}
	engine, _ := res.Data["engine"].(string)
	mode, _ := res.Data["mode"].(string)
	ph, _ := res.Data["placeholder"].(bool)
	if engine != "fingerprint" || mode != "detect" || !ph {
		t.Errorf("data = %v", res.Data)
	}

	// 未知模式
	res, err = tool.Run(context.Background(),
		map[string]any{"target_url": "http://x/", "param": "p", "mode": "nope"},
		&core.ToolContext{})
	if err != nil {
		t.Fatalf("run 出错: %v", err)
	}
	if res.Status != core.StatusError || !strings.Contains(res.Stderr, "未知模式: nope") {
		t.Errorf("未知模式未报错: status=%s stderr=%q", res.Status, res.Stderr)
	}
}

func TestParseRawHTTPRequest(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		method  string
		url     string
		params  [][2]string
		wantErr string
	}{
		{
			name:   "绝对 URL + 查询串 + 表单",
			raw:    "POST http://example.com/a/b?x=1&y=%20 HTTP/1.1\r\nHost: example.com\r\nContent-Type: application/x-www-form-urlencoded\r\n\r\nz=2&w=\r\n",
			method: "POST",
			url:    "http://example.com/a/b?x=1&y=%20",
			params: [][2]string{{"x", "1"}, {"y", " "}, {"z", "2"}, {"w", ""}},
		},
		{
			name:   "相对路径 + Host",
			raw:    "GET /flag HTTP/1.1\nHost: target.com\n\n",
			method: "GET",
			url:    "http://target.com/flag",
			params: nil,
		},
		{
			name:   "相对路径 + 443 端口走 https",
			raw:    "POST /x\nHost: target.com:443\n\n",
			method: "POST",
			url:    "https://target.com:443/x",
			params: nil,
		},
		{
			name:    "multipart 不支持",
			raw:     "POST http://e.com/ HTTP/1.1\nHost: e.com\nContent-Type: multipart/form-data; boundary=--x\n\n--x\r\n",
			wantErr: "multipart",
		},
		{
			name:    "坏请求行",
			raw:     "THIS IS NOT HTTP\nHost: x\n\n",
			wantErr: "无法解析请求行",
		},
		{
			name:    "相对路径缺 Host",
			raw:     "POST /x HTTP/1.1\n\n",
			wantErr: "Host",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := parseRawHTTPRequest(c.raw)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("期望错误含 %q, 实际 %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if p.Method != c.method || p.URL != c.url {
				t.Errorf("method/url = %s %s, 期望 %s %s", p.Method, p.URL, c.method, c.url)
			}
			if len(p.Params) != len(c.params) {
				t.Fatalf("params = %v, 期望 %v", p.Params, c.params)
			}
			for i := range c.params {
				if p.Params[i] != c.params[i] {
					t.Errorf("params[%d] = %v, 期望 %v", i, p.Params[i], c.params[i])
				}
			}
		})
	}
}
