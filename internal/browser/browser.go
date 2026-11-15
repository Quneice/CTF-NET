// Package browser 内置浏览器启动器:复用系统 Edge/Chrome(Chromium 内核),
// 以专用用户目录 + --proxy-server 参数启动,流量默认走本机内建代理(Burp 内置浏览器同思路)。
// 专用用户目录隔离浏览器的 Cookie/缓存,不污染用户日常浏览器配置。
package browser

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Info 浏览器探测结果。
type Info struct {
	Available bool   `json:"available"`
	Name      string `json:"name"`
	Path      string `json:"path"`
}

// candidate 候选浏览器(按优先级)。
type candidate struct {
	name  string
	paths []string
}

// thirdpartyCandidates 离线部署的便携 Chromium(thirdparty/chromium/chrome.exe,
// 相对可执行文件目录与工作目录双候选;用 fetch-portable-chromium 脚本部署)。
func thirdpartyCandidates() []candidate {
	var paths []string
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "thirdparty", "chromium", "chrome.exe"))
	}
	if wd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(wd, "thirdparty", "chromium", "chrome.exe"))
	}
	return []candidate{{name: "内置 Chromium", paths: paths}}
}

// windowsCandidates 常见安装位置(Edge 为 Win10/11 自带)。
func windowsCandidates() []candidate {
	local := os.Getenv("LOCALAPPDATA")
	return []candidate{
		{name: "Edge", paths: []string{
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		}},
		{name: "Chrome", paths: []string{
			filepath.Join(local, `Google\Chrome\Application\chrome.exe`),
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		}},
	}
}

// Detect 返回可用的浏览器(内置 Chromium → Edge → Chrome → PATH 兜底)。
func Detect() (Info, error) {
	return detectWith(append(thirdpartyCandidates(), windowsCandidates()...), exec.LookPath)
}

// detectWith 可注入的探测实现(测试用)。
func detectWith(cands []candidate, lookPath func(string) (string, error)) (Info, error) {
	for _, c := range cands {
		for _, p := range c.paths {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return Info{Available: true, Name: c.name, Path: p}, nil
			}
		}
	}
	for _, exe := range []string{"msedge", "chrome", "chromium"} {
		if p, err := lookPath(exe); err == nil {
			return Info{Available: true, Name: strings.Title(exe), Path: p}, nil
		}
	}
	return Info{}, errors.New("未找到 Edge/Chrome 浏览器(Windows 10/11 通常自带 Edge)")
}

// DefaultStartURL 内置浏览器起始页。
const DefaultStartURL = "http://example.com"

// BuildArgs 构造启动参数:专用用户目录 + 代理指向 + 起始页(纯函数,可测试)。
// --ignore-certificate-errors:配合 MITM 解密(专用 profile 隔离,风险可控);
// 亦使自签目标站点(CTF 常态)免证书报错直达。
func BuildArgs(profileDir, proxyAddr, url string) []string {
	if strings.TrimSpace(url) == "" {
		url = DefaultStartURL
	}
	return []string{
		"--user-data-dir=" + profileDir,
		"--proxy-server=" + proxyAddr,
		"--ignore-certificate-errors",
		"--no-first-run",
		"--no-default-browser-check",
		url,
	}
}

// Launch 以带代理参数启动浏览器;浏览器进程独立于本进程生命周期。
func Launch(info Info, profileDir, proxyAddr, url string) error {
	if !info.Available {
		return errors.New("浏览器不可用")
	}
	cmd := exec.Command(info.Path, BuildArgs(profileDir, proxyAddr, url)...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 %s 失败: %w", info.Name, err)
	}
	// 进程已交还系统托管;Wait 会阻塞,刻意不调用
	return nil
}
