package aiagent

import "testing"

// TestHasVersionSuffix：/v<数字> 版本段识别（智谱 /v4、DeepSeek /v1），防误伤普通路径。
func TestHasVersionSuffix(t *testing.T) {
	cases := []struct {
		base string
		want bool
	}{
		{"https://open.bigmodel.cn/api/paas/v4", true}, // 智谱
		{"https://api.deepseek.com/v1", true},          // DeepSeek
		{"https://api.openai.com/v2", true},
		{"https://api.deepseek.com", false},     // 无版本段 → 自动补 /v1
		{"https://v2.example.com", false},       // 域名里的 v2 不误判
		{"https://host/api/paas/v4beta", false}, // 非纯数字结尾
		{"https://host/chat/completions", false},
	}
	for _, c := range cases {
		if got := hasVersionSuffix(c.base); got != c.want {
			t.Errorf("hasVersionSuffix(%q) = %v, want %v", c.base, got, c.want)
		}
	}
}

// TestEndpoint：BaseURL → chat/completions 端点的三种形态。
func TestEndpoint(t *testing.T) {
	cases := []struct {
		base string
		want string
	}{
		{"https://open.bigmodel.cn/api/paas/v4", "https://open.bigmodel.cn/api/paas/v4/chat/completions"},
		{"https://api.deepseek.com/v1/", "https://api.deepseek.com/v1/chat/completions"},
		{"https://api.deepseek.com", "https://api.deepseek.com/v1/chat/completions"},
		{"https://host/paas/v4/chat/completions", "https://host/paas/v4/chat/completions"}, // 完整端点原样
	}
	for _, c := range cases {
		l := &LLM{baseURL: trimSlash(c.base)}
		if got := l.endpoint(); got != c.want {
			t.Errorf("endpoint(%q) = %q, want %q", c.base, got, c.want)
		}
	}
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
