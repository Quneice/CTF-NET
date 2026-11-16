# 批 17 报告 · HTTP 代理拦截与重放(Burp 风格,真实实现 P1)

- 生成时间:2026-11-15
- 范围:用户需求「类似 BurpSuite:接受流量包代理、修改数据包后重放,顶级功能」。按计划 `docs/superpowers/plans/2026-11-15-proxy-repeater.md` 实施 P1:捕获/拦截/修改/丢弃/放行/历史/重放 + CONNECT 直通隧道。
- 批次性质:**旗舰功能批** —— Go 引擎真实实现(TDD 9 例)+ Wails 绑定 + 前端工作区三页签,非占位。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `internal/proxy/server.go` | **新增**:代理引擎(纯标准库)——ServeHTTP 捕获(原始报文快照)→ 拦截挂起(channel 等 UI 决策)→ forward/drop/modified 三动作 → 转发(http.Transport 直连、不跟随重定向)→ 回写 + 历史;CONNECT hijack 双向拷贝隧道;Replay;parseRequest(代理绝对形式 + 源站形式) |
| `internal/proxy/history.go` | **新增**:Summary/Record/Status/HistoryPage 契约结构 + 内存环形历史(上限 500,新→旧)+ 分页/按 id 取/清空 |
| `internal/proxy/server_test.go` | **新增**:TDD 9 例(捕获转发/拦截三动作/重放二次命中/CONNECT 隧道/历史封顶/非法决策/源站形式解析) |
| `proxy_api.go` | **新增**:Wails 绑定 ProxyStart/Stop/Status/SetIntercept/History/GetRecord/Decide/Replay/HistoryClear;拦截事件 `proxy:intercept`(EventsEmit,载荷 Summary) |
| `app.go` | App 结构体 +proxyOnce/proxyServer 字段(sync 单例懒初始化) |
| `frontend/src/lib/proxy.ts` | **新增**:绑定门面(类型 + 9 个调用 + onProxyIntercept 事件订阅,同 fingerdb.ts 模式) |
| `frontend/src/components/proxy/ProxyWorkspace.tsx` | **新增**:工作区——控制栏(端口/启停/拦截开关/清空/刷新)+ 拦截/历史/Repeater 三页签;拦截页可编辑报文 + 放行/按修改放行/丢弃 + 决策结果回显;历史页列表 + 请求/响应原始报文详情 + 发送到 Repeater;无 Wails 时显示「需要桌面运行时」 |
| `frontend/src/pages/ToolPage.tsx` | workspace 分发 +`proxy-interceptor` 分支 |
| `frontend/src/mock/categories.ts` | +顶级分类 `pentest.proxy` 代理拦截(icon Waypoints) |
| `frontend/src/mock/tools.ts` | +workspace 工具行 `pentest.proxy.intercept`(代理拦截与重放) |
| `frontend/src/mock/modes.ts` | 渗透测试模式 categoryRoots +`pentest.proxy`(8 个顶级分类),描述补「代理拦截」 |
| `frontend/src/lib/icons.tsx` | iconMap +Waypoints(import 与 map 同步) |
| `frontend/wailsjs/go/models.ts` / `main/App.d.ts` / `main/App.js` | **新增**:proxy 命名空间模型 + 9 个绑定声明/实现(手写与 wails 生成器输出一致,下次 wails dev/build 自动重生成) |
| `README.md` | 功能总览 +「HTTP 代理拦截与重放」条目;渗透测试模式清单补「代理拦截」 |
| `docs/batch-reports/017-proxy-repeater.md` | 本报告 |

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `go test ./internal/proxy/` | ✅ 9/9 PASS(捕获转发/拦截放行/拦截丢弃/拦截修改/重放/CONNECT 隧道/历史封顶/非法决策/源站形式解析) |
| 2 | `go build ./...` | ✅ 编译通过(绑定层 + App 结构体) |
| 3 | `npx tsc --noEmit` | ✅ 0 错误 |
| 4 | `npm run build` | ✅ 构建通过(4.82s) |
| 5 | 状态推导 | ✅ workspace → implemented(真实实现,不伪装;与 fingerprint-db 同口径) |
| 6 | 事件链路 | ✅ Go EventsEmit("proxy:intercept", Summary) → 前端 onProxyIntercept → ProxyGetRecord 拉全量 → 拦截页签自动切换 |

## 3. 调试记录(如实)

1. **`strings.TrimSpace` 剪掉报文终结符**:parseRequest 原对 raw 做 TrimSpace,把结尾必需的 `\r\n\r\n` 剪掉,bufio.ReadSlice 读到 EOF 转成 unexpected EOF。修复:只对空检查用 TrimSpace,报文原文不修剪。
2. **dump 后 body 丢失**:ServeHTTP 先 ReadAll+Close 再 dump,dump 恢复的是已关闭的 body,转发报 "invalid Read on closed Body"。修复:dumpRequest 只消费临时读器,调用方显式装回 body。
3. **CONNECT 记录时机**:隧道双向拷贝是长连接,`<-done` 永不返回导致 CONNECT 条目丢失。修复:隧道建立(200 已发)即入历史。
4. **测试断言未跟绝对 URI**:代理形态请求 DumpRequest 输出 `POST http://host/path`(绝对形式),测试原断言 `POST /hello`。修正断言并注释原因。

## 4. 实现说明

1. **拦截挂起语义**:拦截开关开时,ServeHTTP 把请求登记进 pending map 并向 UI 发事件,handler 阻塞在 channel 上等决策(与 Burp 行为一致:请求在代理处挂起,目标不感知);Stop 时统一按 Drop 收尾,连接不悬挂。
2. **快照与转发解耦**:body 先读入内存 → DumpRequest 出原始报文(UI 展示)→ 装回 body 转发;修改放行走 parseRequest(http.ReadRequest)重新解析为可转发请求,补齐 scheme/host、清空 RequestURI。
3. **不跟随重定向**(CheckRedirect=ErrUseLastResponse):与 Burp 逐跳展示一致,3xx 直接可见。
4. **CONNECT 直通不解密**:HTTPS 流量经隧道透明转发并记一条 CONNECT 历史;TLS MITM(内置 CA + 按域证书)按计划留待 P2。
5. **wailsjs 生成文件手写同步**:models.ts/App.d.ts/App.js 按生成器输出格式手写(含「自动生成勿改」头注释保留),下次 wails dev/build 会自动重生成覆盖为等价内容。

## 5. 剩余风险与 P2 规划

1. **TLS MITM(P2 最高优先)**:CA 生成/证书库 + CONNECT 内 tls.Server + 按 SNI 签发,即可解密 HTTPS 流量进拦截/历史/重放。
2. **历史内存封顶 500**:重启即失;P2 做 SQLite 持久化 + 搜索过滤。
3. **作用域规则**(Burp Scope)与**自动改包规则**(host 重写/UA 替换)未做,当前拦截为全量。
4. **HTTP/2/WebSocket 拦截**未做(Client 侧 h2 会经 CONNECT 隧道直通,不影响可用性)。
5. **浏览器代理配置引导**:当前工作区仅提示文案;P2 可做一键设置系统代理(windows proxy API)。
6. **全局代理链**:未接设置页 proxy.url(转发走直连);P2 接 internal/aiagent/proxy.go 的代理构造逻辑。
7. **交互验证**:真实浏览器走 127.0.0.1:8080 的端到端体验需 `wails dev` 人工复核(测试覆盖引擎,UI 交互未自动化)。
