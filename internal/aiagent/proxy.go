package aiagent

import (
	"context"
	"runtime"
	"strings"
)

// —— 多 Agent 代理访问（对齐 ARTEX 的 proxyEnv 子进程环境注入）——
//
// LLM 调用与 http_request 工具已走 Go http.Transport 的全局代理；本文件把同一
// 代理注入 worker 通过 exec_command/配方工具拉起的子进程（curl/nmap/python 等），
// 使 worker 的所有出站通道共享同一代理出口。

// ExecWithProxyEnv 包装 ExecFunc：命令前置代理环境变量（无代理时原样返回）。
// Windows（cmd /C）用 set "K=V"&& 链；POSIX 用 export K='V'; 前缀。
// 在 tools/ai 层（task/chat 两个入口）构造 ExecFunc 后各包一次。
func ExecWithProxyEnv(exec ExecFunc, proxyURL string) ExecFunc {
	if exec == nil || strings.TrimSpace(proxyURL) == "" {
		return exec
	}
	return func(ctx context.Context, command string, dir string, timeoutSec int) (string, string, error) {
		return exec(ctx, proxyEnvPrefix(proxyURL)+command, dir, timeoutSec)
	}
}

// proxyEnvPrefix 生成代理环境变量前缀（值加引号防特殊字符）。
func proxyEnvPrefix(proxyURL string) string {
	u := strings.TrimSpace(proxyURL)
	if runtime.GOOS == "windows" {
		return `set "HTTP_PROXY=` + u + `"&& set "HTTPS_PROXY=` + u + `"&& set "ALL_PROXY=` + u + `"&& ` +
			`set "http_proxy=` + u + `"&& set "https_proxy=` + u + `"&& set "all_proxy=` + u + `"&& `
	}
	return "export HTTP_PROXY='" + u + "' HTTPS_PROXY='" + u + "' ALL_PROXY='" + u + "' " +
		"http_proxy='" + u + "' https_proxy='" + u + "' all_proxy='" + u + "'; "
}
