package main

// —— HTTP 代理拦截与重放(Burp 风格)的 Wails 绑定 ——
// 引擎见 internal/proxy(纯标准库):捕获(原始报文快照)→ 拦截挂起(channel 等 UI 决策)
// → 放行/丢弃/修改放行 → 历史记录 → 重放;CONNECT 走 TCP 直通隧道(TLS MITM 留待 P2)。
// 单例懒初始化;拦截事件 proxy:intercept 经 EventsEmit 推给前端(载荷为 Summary,
// 前端按 id 调 ProxyGetRecord 拉全量原始报文)。

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ctfnet/internal/proxy"
)

// proxySrv 返回代理引擎单例;拦截事件回调挂 app.ctx;首用时装载 MITM CA(DataDir/certs)。
func (a *App) proxySrv() *proxy.Server {
	a.proxyOnce.Do(func() {
		a.proxyServer = proxy.NewServer(func(rec *proxy.Record) {
			if a.ctx != nil {
				wailsRuntime.EventsEmit(a.ctx, "proxy:intercept", rec.Summary)
			}
		})
		if a.Cfg != nil {
			if err := a.proxyServer.SetCA(filepath.Join(a.Cfg.DataDir, "certs")); err != nil {
				log.Printf("[CTF-NET] 代理 CA 初始化失败(HTTPS 解密不可用): %v", err)
			}
		}
	})
	return a.proxyServer
}

// ProxyStart 启动代理监听(127.0.0.1:port;port<=0 随机端口)。
func (a *App) ProxyStart(port int) error {
	return a.proxySrv().Start(port)
}

// ProxyStop 停止代理;挂起中的拦截请求按丢弃收尾。
func (a *App) ProxyStop() error {
	return a.proxySrv().Stop()
}

// ProxyStatus 代理运行状态(running/port/intercept)。
func (a *App) ProxyStatus() proxy.Status {
	return a.proxySrv().Status()
}

// ProxySetIntercept 开关全局拦截。
func (a *App) ProxySetIntercept(on bool) error {
	a.proxySrv().SetIntercept(on)
	return nil
}

// ProxyHistory 历史分页(新→旧;limit<=0 取全部)。
func (a *App) ProxyHistory(offset, limit int) proxy.HistoryPage {
	return a.proxySrv().History(offset, limit)
}

// ProxyGetRecord 按 id 取全量记录(原始请求/响应报文)。
func (a *App) ProxyGetRecord(id string) (proxy.Record, error) {
	return a.proxySrv().Record(id)
}

// ProxyDecide 对挂起中的拦截请求做决策:forward / drop / modified(带修改后报文)。
func (a *App) ProxyDecide(id, action, modified string) error {
	return a.proxySrv().Decide(id, action, modified)
}

// ProxyReplay 重放原始请求报文并返回新记录(Repeater)。
func (a *App) ProxyReplay(raw string) (proxy.Record, error) {
	return a.proxySrv().Replay(raw)
}

// ProxyHistoryClear 清空历史(工作区辅助)。
func (a *App) ProxyHistoryClear() error {
	a.proxySrv().Clear()
	return nil
}

// ProxySetMITM 开关 HTTPS 解密(CA 已由 proxySrv 装载)。
func (a *App) ProxySetMITM(on bool) error {
	return a.proxySrv().SetMITM(on)
}

// ProxyCAExport 导出 CA 证书到指定目录(供导入浏览器/其他机器信任库),返回写入路径。
func (a *App) ProxyCAExport(dir string) (string, error) {
	st := a.proxySrv().Status()
	if st.CAFile == "" {
		return "", errors.New("CA 未初始化(首次启动代理功能时自动生成)")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建导出目录失败: %w", err)
	}
	dst := filepath.Join(dir, "ctfnet-ca.crt")
	data, err := os.ReadFile(st.CAFile)
	if err != nil {
		return "", fmt.Errorf("读取 CA 失败: %w", err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", fmt.Errorf("写入导出证书失败: %w", err)
	}
	return dst, nil
}

// ProxyCAInstall 把 CA 安装到当前用户信任库(certutil -user,无需管理员权限)。
func (a *App) ProxyCAInstall() error {
	st := a.proxySrv().Status()
	if st.CAFile == "" {
		return errors.New("CA 未初始化(首次启动代理功能时自动生成)")
	}
	out, err := exec.Command("certutil", "-user", "-addstore", "-f", "Root", st.CAFile).CombinedOutput()
	if err != nil {
		return fmt.Errorf("certutil 执行失败: %w\n%s", err, out)
	}
	return nil
}
