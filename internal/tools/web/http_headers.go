// web.http_headers —— 获取 HTTP 响应头（对应 legacy/tools/web/http_headers.py 的 curl -sI）。
// Go 版用 net/http 原生 HEAD，输出保持 curl -I 的可读格式。
package web

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sort"
	"time"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&HttpHeadersTool{BaseTool: core.BaseTool{
		NameAttr:        "http_headers",
		CategoryAttr:    core.CatWeb,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "获取 HTTP 响应头（curl -I）",
		TimeoutAttr:     30,
		ParamsAttr: []core.ParamSpec{
			{Name: "url", Type: "str", Required: true, Description: "目标 URL"},
		},
		Runner: httpHeadersRun,
	}})
}

// HttpHeadersTool HTTP 响应头工具。
type HttpHeadersTool struct {
	core.BaseTool
}

func httpHeadersRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	url := paramStr(params, "url")
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 对齐 curl 默认容忍自签名
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("请求失败: %v", err)}, nil
	}
	defer resp.Body.Close()

	var out string
	out += fmt.Sprintf("HTTP/1.1 %s\r\n", resp.Status)
	keys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out += fmt.Sprintf("%s: %s\r\n", k, resp.Header.Get(k))
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: out}, nil
}

func paramStr(params map[string]any, key string) string {
	s, _ := params[key].(string)
	return s
}
