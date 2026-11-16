# 批 18 报告 · 内置浏览器(系统浏览器内核 + 代理默认指向本机)

- 生成时间:2026-11-15
- 范围:用户需求「内置一个浏览器内核,可以通过系统打开浏览器,浏览器的代理默认指向该系统」——复用系统 Edge/Chrome(Chromium 内核),以专用用户目录 + `--proxy-server=127.0.0.1:<代理端口>` 启动,流量默认走批 17 的内建代理,与代理拦截工作区闭环(等价 Burp 内置浏览器)。
- 批次性质:**功能批** —— Go 真实实现(TDD 3 例)+ 绑定 + 工作区按钮;不捆绑 Chromium 发行包(Windows 10/11 自带 Edge,Chrome 为可选替代)。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `internal/browser/browser.go` | **新增**:候选探测(Edge 常见路径 → Chrome 常见路径 → PATH 兜底)、参数构造(--user-data-dir 专用目录 / --proxy-server / --no-first-run / 起始页)、Launch(exec.Start,进程独立于应用生命周期);探测与参数构造为纯函数可注入测试 |
| `internal/browser/browser_test.go` | **新增**:TDD 3 例(参数构造含空 URL 默认页/伪造目录命中 Edge/全空报错) |
| `browser_api.go` | **新增**:Wails 绑定 BrowserAvailable(探测结果)/ BrowserLaunch(url, portHint)——**代理未运行则先用 portHint 启动(<=0 用 8080)**,再拉起浏览器;返回 {browser, url, proxy_port} |
| `frontend/src/lib/proxy.ts` | +BrowserInfo/BrowserLaunchResult 类型 + browserAvailable/browserLaunch 门面 |
| `frontend/src/components/proxy/ProxyWorkspace.tsx` | 控制栏 +起始页 URL 输入(默认 http://example.com)+「内置浏览器」按钮(启动后提示浏览器名与代理地址);hint 文案更新(内置浏览器自动走代理;HTTPS 隧道直通) |
| `frontend/wailsjs/go/models.ts` / `main/App.d.ts` / `main/App.js` | +proxy.BrowserInfo / proxy.BrowserLaunchResult 模型与绑定声明/实现(手写与生成器输出一致) |
| `README.md` | 代理拦截条目补「内置浏览器」说明 |
| `docs/batch-reports/018-embedded-browser.md` | 本报告 |

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `go test ./internal/browser/` | ✅ 3/3 PASS(参数构造/探测命中/全空报错) |
| 2 | `go build ./...` | ✅ 编译通过 |
| 3 | `npx tsc --noEmit` / `npm run build` | ✅ 0 错误 / 构建通过 |
| 4 | 闭环保证 | ✅ BrowserLaunch 内先确保代理运行(Status 检查 + Start),浏览器代理参数一定指向本机代理端口 |
| 5 | 隔离保证 | ✅ 专用 user-data-dir = DataDir/browser-profile,Cookie/缓存/登录态与用户日常浏览器隔离 |

## 3. 实现说明

1. **内核复用而非捆绑**:完整 Chromium 发行包 ~200MB+,且 Windows 10/11 必带 Edge(同内核);探测优先级 Edge(Program Files x86/Program Files)→ Chrome(LOCALAPPDATA + Program Files)→ PATH(msedge/chrome/chromium),全部缺失才报错。
2. **启动语义**:`exec.Command.Start` 后进程交还系统托管(刻意不 Wait,否则会阻塞到浏览器关闭);重复点击 = 复用同一 profile 的既有浏览器进程开新窗口(Chromium 单例 profile 语义)。
3. **代理联动**:用户可先在控制栏启动代理自定义端口;直接点「内置浏览器」时引擎自动用当前端口输入(默认 8080)启动代理再拉起浏览器。
4. **HTTPS 现状**:浏览器 https 流量经 CONNECT 隧道直通可用但不可解密拦截——这正是批 17 P2「TLS MITM」的落点;解密实现后内置浏览器即获得完整 HTTPS 拦截能力。

## 4. 剩余风险

1. **无 Chromium 的系统/环境报错兜底**:当前报「未找到 Edge/Chrome」;P2 可选:捆绑便携 Chromium 到 thirdparty/(离线部署场景)。
2. **浏览器进程无法随应用退出自动关闭**(刻意设计:进程独立);profile 目录残留为常态(Burp 同款行为)。
3. **端到端体验**(按钮点击 → Edge 弹出 → 访问页面 → 历史出现记录)需 `wails dev` 人工复核。
