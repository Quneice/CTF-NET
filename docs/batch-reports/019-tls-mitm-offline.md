# 批 19 报告 · TLS MITM 解密 + 内置浏览器离线部署

- 生成时间:2026-11-15
- 范围:用户需求「做 TLS MITM 解密然后离线部署」——①内置 CA + 按域签发的 HTTPS 解密(HTTPS 流量进入拦截/历史/重放管线);②捆绑便携 Chromium 的离线部署(thirdparty/chromium/ + 下载脚本)。
- 批次性质:**功能批** —— Go MITM 引擎(TDD 5 例)+ 绑定 + 工作区 MITM 控制行 + 离线部署脚本与探测接入。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `internal/proxy/mitm.go` | **新增**:EnsureCA(ECDSA P-256 CA,10 年,持久化 DataDir/certs,二次调用复用不重签)、SetCA/SetMITM、hostCert(按域签发+缓存;host 为 IP 走 IPAddresses SAN、域名走 DNSNames SAN)、serveMITM(CONNECT 200 后 TLS 终止 → 内层请求补 scheme https 复用同一 ServeHTTP 管线)、singleConnListener |
| `internal/proxy/server.go` | Server +mitm/caDir/caFile/caCert/hostCerts 字段;handleConnect 按 mitmOn() 分流(MITM 或直通隧道);修改放行保持原请求 scheme(**关键修复**:parseRequest 缺省 http 会把 MITM 内层请求降级成明文 HTTP 打 TLS 端口);转发 transport 默认 InsecureSkipVerify(CTF 目标常自签,与内置浏览器 --ignore-certificate-errors 口径一致,可注入覆盖) |
| `internal/proxy/history.go` | Status +MITM/CAFile 字段 |
| `internal/proxy/mitm_test.go` | **新增**:TDD 5 例(MITM 解密贯通 https 内层请求入历史/拦截修改 https 请求/关 MITM 直通回归/CA 持久化不重签/域证书 CA 签名验证+缓存命中) |
| `proxy_api.go` | proxySrv 首用装载 CA(DataDir/certs);+ProxySetMITM/ProxyCAExport/ProxyCAInstall(certutil -user -addstore -f Root,无需管理员) |
| `internal/browser/browser.go` | 探测链加 **内置 Chromium 优先**(thirdparty/chromium/chrome.exe,exe 目录与工作目录双候选);启动参数 +--ignore-certificate-errors(MITM 配套,专用 profile 隔离) |
| `internal/browser/browser_test.go` | BuildArgs 断言更新(6 参数,含证书忽略) |
| `scripts/fetch-portable-chromium.mjs` | **新增**:下载脚本(临时目录 npm i playwright → npx playwright install chromium → 从 ms-playwright 拷贝 chrome-win 到 thirdparty/chromium/,~170MB,thirdparty/ 在 .gitignore 不提交) |
| `frontend/src/components/proxy/ProxyWorkspace.tsx` | 新增「HTTPS 解密」控制行:MITM 开关 + CA 导出目录输入 + 导出 CA + 安装到系统信任库;hint 文案更新 |
| `frontend/src/lib/proxy.ts` | ProxyStatus +mitm/ca_file;+proxySetMitm/proxyCAExport/proxyCAInstall 门面 |
| `frontend/wailsjs/go/models.ts` / `main/App.d.ts` / `main/App.js` | proxy.Status 补字段;+ProxySetMITM/ProxyCAExport/ProxyCAInstall 声明与实现 |
| `README.md` | 代理拦截条目补 MITM 与离线部署说明 |
| `docs/batch-reports/019-tls-mitm-offline.md` | 本报告 |

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `go test ./internal/proxy/` | ✅ **14/14 PASS**(批 17 的 9 例回归 + 本批 5 例) |
| 2 | `go test ./internal/browser/` | ✅ 3/3 PASS(参数构造更新) |
| 3 | `go build ./...` | ✅ 编译通过 |
| 4 | `npx tsc --noEmit` / `npm run build` | ✅ 0 错误 / 构建通过 |
| 5 | HTTPS 闭环 | ✅ 客户端信任 CA 后:CONNECT → MITM 握手 → 内层 https 请求进入捕获/拦截/历史(测试断言历史含 https URL 且拦截修改生效) |
| 6 | 离线部署 | ✅ 探测链内置 Chromium 优先;下载脚本已提供(本批后台执行下载中,结果以日志为准) |

## 3. 调试记录(如实)

1. **IP 主机证书校验失败**:测试目标 host 为 127.0.0.1,首版域证书只有 DNS SAN → 客户端报「doesn't contain any IP SANs」。修复:host 解析为 IP 时走 IPAddresses SAN。
1b. **下载脚本注释里的 `*/` 提前终结块注释**:`chromium-*/chrome-win/` 中的 `*/` 把 JSDoc 注释截断,后半行变成裸 JS → SyntaxError;首轮后台执行静默失败(exit 0 是外层 echo 的返回值,教训:后台脚本要校验产物而非依赖 exit code)。修复注释措辞后重跑成功。
1c. **官方 CDN 在国内 ECONNRESET**(cdn.playwright.dev 两次连接重置,207MB 中断):脚本改为默认 npmmirror 镜像(PLAYWRIGHT_DOWNLOAD_HOST 可覆盖),失败自动回退官方源。
1d. **新版 playwright 目录布局变化**:chrome-win/ → chrome-win64/(Chrome for Testing),脚本兼容两种布局。镜像下载一次成功(207.3MB Chromium + FFmpeg + headless shell + winldd),重跑时命中 INSTALLATION_COMPLETE 秒级跳过。
**最终结果**:thirdparty/chromium/ 落盘 453MB,chrome.exe 就位;Detect() 实测返回「内置 Chromium」优先命中。
2. **转发验签失败**:代理转发 client 默认校验目标证书,测试目标自签 → 「signed by unknown authority」。按渗透取向改为转发默认 InsecureSkipVerify(与内置浏览器 --ignore-certificate-errors 一致;报告标注取舍,后续可加严格校验开关)。
3. **修改放行把 https 降级成 http**:parseRequest 对无 scheme 报文缺省 http,MITM 内层修改后以明文 HTTP 打到 TLS 端口(目标回「Client sent an HTTP request to an HTTPS server」)。修复:修改放行分支保持原请求 scheme。

## 4. 实现说明与取舍

1. **CA 与信任模型**:CA 私钥(ECDSA P-256)仅存 DataDir/certs(不提交 git);客户端信任途径三选一——内置浏览器自动(--ignore-certificate-errors)、一键安装当前用户信任库(certutil -user,免管理员)、导出 ctfnet-ca.crt 手动导入。重复使用同一 CA(持久化不重签),信任一次即可。
2. **转发跳过目标证书校验**:渗透/CTF 目标常为自签证书,默认跳过以保证「浏览器看到的都能代理」;风险为丢失真实站点的证书校验提示,后续可加「严格校验」开关。
3. **Repeater 的 https 语义**:源站形式报文(缺 scheme)重放默认 http;重放 https 需用绝对形式(GET https://host/path HTTP/1.1)或先经历史「发送到 Repeater」(绝对形式原样携带)。UI hint 后续补。
4. **离线部署产物**:chromium ~170MB 存 thirdparty/(gitignore,随部署包分发);`scripts/fetch-portable-chromium.mjs` 一键重建。内置浏览器探测链:内置 Chromium → Edge → Chrome → PATH。

## 5. 剩余风险

1. **Chromium 下载依赖网络**(Playwright CDN);下载失败时脚本可重跑,期间内置浏览器回退 Edge/Chrome。
2. **certutil 安装 CA** 依赖 Windows 自带 certutil(目标平台即 Windows,OK);重复安装用 -f 覆盖,无残留告警。
3. **MITM 与 HTTP/2**:tls.Config NextProtos 仅 http/1.1,客户端会协商回退 HTTP/1.1(h2 浏览器均可);后续可补 h2。
4. **端到端体验**(HTTPS 解密开关 → 内置浏览器访问 https 站点 → 历史出现解密请求)需 `wails dev` 人工复核。
