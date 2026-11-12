package main

import (
	"fmt"
	"net/url"
	"strings"

	"ctfnet/internal/core"
)

// 统一应用配置（data/config.json）的 Wails 绑定。
// 约定：绑定方法禁止带 context.Context 首参（本版 Wails 不注入，会导致参数错位）。

// GetAppConfig 返回当前应用配置快照（含 JNDI/外部工具/字典/执行器四组）。
func (a *App) GetAppConfig() (*core.AppConfig, error) {
	return core.GetAppConfig(), nil
}

// UpdateAppConfig 校验并保存配置，保存后立即生效（原子快照整体替换）。
// 执行器组（backend/docker）变更需重启应用才生效——TaskManager 的 Executor
// 在启动时构建，不做热改。
func (a *App) UpdateAppConfig(cfg *core.AppConfig) error {
	if cfg == nil {
		return fmt.Errorf("配置不能为空")
	}
	if err := validateAppConfig(cfg); err != nil {
		return err
	}
	if a.Cfg == nil {
		return fmt.Errorf("配置未初始化")
	}
	if err := core.SaveAppConfig(a.Cfg.DataDir, cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	core.SetAppConfig(cfg)
	return nil
}

// validateAppConfig 基础边界校验（端口/超时范围）。
func validateAppConfig(cfg *core.AppConfig) error {
	ports := map[string]int{
		"SSH 端口":      cfg.Jndi.SSHPort,
		"LDAP 端口":     cfg.Jndi.LDAPPort,
		"RMI 端口":      cfg.Jndi.RMIPort,
		"HTTP 端口":     cfg.Jndi.HTTPPort,
		"反弹监听端口": cfg.Jndi.LPort,
		"FRP bind 端口":  cfg.Frp.BindPort,
		"FRP vhost HTTP 端口":  cfg.Frp.VhostHTTPPort,
		"FRP vhost HTTPS 端口": cfg.Frp.VhostHTTPSPort,
	}
	for name, p := range ports {
		if p < 1 || p > 65535 {
			return fmt.Errorf("%s需在 1~65535 之间", name)
		}
	}
	// FRP SSH 端口 0 表示回退 jndi 组，允许
	if cfg.Frp.SSHPort != 0 && (cfg.Frp.SSHPort < 1 || cfg.Frp.SSHPort > 65535) {
		return fmt.Errorf("FRP SSH 端口需在 1~65535 之间（或 0 回退 JNDI 组）")
	}
	// FRP Dashboard 端口 0 表示禁用，允许
	if cfg.Frp.DashboardPort != 0 && (cfg.Frp.DashboardPort < 1 || cfg.Frp.DashboardPort > 65535) {
		return fmt.Errorf("FRP Dashboard 端口需在 1~65535 之间（或 0 禁用）")
	}
	ex := cfg.Executor
	if ex.DefaultTimeout < 1 || ex.MaxTimeout < ex.DefaultTimeout {
		return fmt.Errorf("超时配置不合法：默认超时需 ≥1 秒且不超过最大超时")
	}
	if ex.Backend != "local" && ex.Backend != "docker" {
		return fmt.Errorf("执行器后端仅支持 local | docker")
	}
	// 全局代理：空=直连；否则须为 http/https/socks5/socks5h URL
	if pu := strings.TrimSpace(cfg.Proxy.URL); pu != "" {
		u, err := url.Parse(pu)
		if err != nil || u.Host == "" {
			return fmt.Errorf("全局代理地址不合法：需形如 http://127.0.0.1:7890 或 socks5://127.0.0.1:7897")
		}
		switch strings.ToLower(u.Scheme) {
		case "http", "https", "socks5", "socks5h":
		default:
			return fmt.Errorf("全局代理仅支持 http / https / socks5 / socks5h 协议")
		}
	}
	// AI 配置边界
	if a := cfg.Ai; a.MaxWorkers != 0 && (a.MaxWorkers < 1 || a.MaxWorkers > 8) {
		return fmt.Errorf("AI 并发 worker 数需在 1~8 之间")
	}
	return nil
}
