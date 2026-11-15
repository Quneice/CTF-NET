package main

// —— 内置浏览器(Wails 绑定)—— //
// 复用系统 Edge/Chrome(Chromium 内核),专用用户目录 + --proxy-server 启动,
// 流量默认走本机内建代理(内部经 proxySrv 保证代理先运行),与代理拦截工作区闭环。

import (
	"fmt"
	"os"
	"path/filepath"

	"ctfnet/internal/browser"
)

// BrowserAvailable 内置浏览器探测结果(Edge → Chrome 优先级;不可用时报错文案在 Name 字段)。
func (a *App) BrowserAvailable() browser.Info {
	info, err := browser.Detect()
	if err != nil {
		return browser.Info{Available: false, Name: err.Error()}
	}
	return info
}

// BrowserLaunchResult 启动结果(前端展示用)。
type BrowserLaunchResult struct {
	Browser   string `json:"browser"`
	URL       string `json:"url"`
	ProxyPort int    `json:"proxy_port"`
}

// BrowserLaunch 启动内置浏览器并把代理指向本机代理:
// 代理未运行时先用 portHint 启动(<=0 用 8080),再以 --proxy-server=127.0.0.1:port 拉起浏览器。
func (a *App) BrowserLaunch(url string, portHint int) (BrowserLaunchResult, error) {
	info, err := browser.Detect()
	if err != nil {
		return BrowserLaunchResult{}, err
	}

	st := a.proxySrv().Status()
	port := portHint
	if !st.Running {
		if port <= 0 {
			port = 8080
		}
		if err := a.proxySrv().Start(port); err != nil {
			return BrowserLaunchResult{}, fmt.Errorf("内置浏览器需要代理运行,但代理启动失败: %w", err)
		}
	} else {
		port = st.Port
	}

	profileDir := filepath.Join(a.Cfg.DataDir, "browser-profile")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return BrowserLaunchResult{}, fmt.Errorf("创建浏览器配置目录失败: %w", err)
	}

	if url == "" {
		url = browser.DefaultStartURL
	}
	if err := browser.Launch(info, profileDir, fmt.Sprintf("127.0.0.1:%d", port), url); err != nil {
		return BrowserLaunchResult{}, err
	}
	return BrowserLaunchResult{Browser: info.Name, URL: url, ProxyPort: port}, nil
}
