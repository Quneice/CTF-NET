# 批 20 报告 · 代理拦截 Burp 式布局改版(一工具拆三页)

- 生成时间:2026-11-15
- 范围:用户需求「参考 Burp 重新布局代理拦截与重放——历史记录单独一个页面、点击历史可发送到重放器、不挤同一页面、去掉头部『利用 代理 拦截 重放』文字」。
- 批次性质:**功能批(纯前端)** —— 单页 ProxyWorkspace(641 行)拆为 3 个工具页 + 共享 store;后端(`proxy_api.go`/`internal/proxy`)零改动、零新依赖。
- 设计文档:`docs/superpowers/specs/2026-11-15-proxy-burp-relayout-design.md`(方案 A,用户已批准)
- 实施计划:`docs/superpowers/plans/2026-11-15-proxy-burp-relayout.md`(6 任务逐任务提交)

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/stores/useProxyStore.ts` | **新增**:共享状态(status/history/pending/selected/repeaterTabs/flash/busy)+ 引用计数轮询(0→1 启动 2s 轮询 + 拦截事件订阅,1→0 停止;keep-alive 多页共享一份)+ 全部动作(start 端口校验/decide 含 waitRecord 终态回写/sendToRepeater 数字标签/closeRepeaterTab 相邻回落/replay 等) |
| `frontend/src/components/proxy/ProxyInterceptWorkspace.tsx` | **新增**:代理拦截页——控制栏(启动/停止/拦截开关/内置浏览器)+ MITM 控制行(CA 导出/安装)+ 挂起请求编辑(放行/修改放行/丢弃/**发送到重放器**);本地仅 portInput/browserUrl/caExportDir 三个输入 |
| `frontend/src/components/proxy/ProxyMiniStatus.tsx` | **新增**:迷你状态行(运行点+端口/未启动,⏳挂起提醒,点击跳拦截页);导出 `statusColor` 供历史表复用 |
| `frontend/src/components/proxy/ProxyHistoryWorkspace.tsx` | **新增**:流量历史页——全宽清单表(#/方法/状态/主机路径/耗时,⚠拦截标记),点行下方展开请求/响应 + 「发送到重放器」 |
| `frontend/src/components/proxy/ProxyRepeaterWorkspace.tsx` | **新增**:重放器页——Burp 式数字多标签(1/2/3…,可关闭/新建),每标签独立请求编辑/发送/响应/meta |
| `frontend/src/components/proxy/ProxyWorkspace.tsx` | **删除**(旧单页,641 行;批次 17 未跟踪文件,直接 rm) |
| `frontend/src/mock/tools.ts` | proxyShorthandRows 1 行拆 3 行:`pentest.proxy.intercept`(改名「代理拦截」)+ 新增 `pentest.proxy.history`「流量历史」+ `pentest.proxy.repeater`「重放器」 |
| `frontend/src/pages/ToolPage.tsx` | bareWorkspace 扩为 poc-scan + proxy 三值(面包屑/工具名/状态徽章/描述/tags 头部整块隐藏,工作区顶格);分发按三个 workspace 值路由到新组件 |
| `README.md` | 「HTTP 代理拦截与重放」条目改为三工具页布局描述 |

## 2. Burp 映射

| Burp Suite | CTF-NET 工具页(id) |
| --- | --- |
| Proxy → Intercept(+ 监听控制) | 代理拦截 `pentest.proxy.intercept`(id 不变,原「代理拦截与重放」) |
| Proxy → HTTP History | 流量历史 `pentest.proxy.history` |
| Repeater(数字标签) | 重放器 `pentest.proxy.repeater` |

三个工具页 = keep-alive 多开标签,恰好对应 Burp 顶部模块切换;「发送到重放器」= 追加数字标签 + 自动跳转(Burp 右键 Send to Repeater 同味)。

## 3. Review Focus 对应实现

| # | 风险点 | 实现 |
| --- | --- | --- |
| 1 | 轮询泄漏 | 引用计数模块级变量(refCount/timer/offIntercept 不进 state),1→0 时 clearInterval + 退订;`refCount = Math.max(0, refCount-1)` 防负 |
| 2 | 跨页拦截不丢 | pending/pendingRaw/pendingResult 全存 store,拦截事件订阅挂轮询生命周期;历史/重放器页 MiniStatus 显示 ⏳挂起 并可一键跳回 |
| 3 | 发送到重放器导航 | `sendToRepeater(raw)` 后 `navigate('/t/pentest.proxy.repeater')`,ToolTabsWorkspace 路由→标签同步自动开标签并激活 |
| 4 | keep-alive 多页并存 | 三页各自 acquire/release;StrictMode 双挂载经 Math.max(0,…) 与对称清理无害 |
| 5 | 非 Wails 守卫 | 三页组件同构守卫块(hooks 全部先于 early return),纯浏览器显示「需要桌面运行时」 |

## 4. 验收清单

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npm run build`(tsc+vite)每任务门禁 | ✅ 6/6 任务通过 |
| 2 | `npm run dev` 纯浏览器:三工具路由可达,组件经 vite 编译服务(HTTP 200) | ✅ 冒烟通过;isWails 守卫为静态同构搬运(原版已验证文案),渲染表现待桌面验收 |
| 3 | 分类树 `pentest.proxy` 下见 3 工具;三页头部(面包屑/名称/徽章/描述/tags)全隐 | ⏳ 待桌面验收(bareWorkspace 逻辑与 poc-scan 先例同构) |
| 4 | 启动代理→内置浏览器→历史页实时增长;点行详情;发送到重放器自动跳转激活新标签 | ⏳ 待桌面验收 |
| 5 | 重放器多标签编辑重发/互不串扰/关标签回落;拦截页三种决策 + 发送到重放器;MiniStatus ⏳ 挂起跳转 | ⏳ 待桌面验收 |
| 6 | MITM 开启后 https 流量入历史 | ⏳ 待桌面验收(后端零改动,批 19 已验) |
| 7 | 关全部代理标签轮询停止、重开恢复(无重复轮询) | ⏳ 待桌面验收 |
| 8 | keep-alive 三标签并存,切换状态保留 | ⏳ 待桌面验收(机制为 ToolTabsWorkspace 既有能力) |

> ⏳ 项需要 `wails dev` 桌面运行时人工点验;本轮自动化验证(构建/冒烟/静态审查)全绿。

## 5. 评审与修复段(全分支新鲜上下文评审后的一次修复)

评审结论 1 Critical / 3 Important / 6 Minor;Critical/Important 全部修复(Minor 记录在案顺延):

| 级别 | 发现 | 修复 | 提交 |
| --- | --- | --- | --- |
| C1 | 拦截挂起 UI 永远为空:`Record()` 只查 history,而拦截记录**决策后**才入历史,前端事件处理器 `GetRecord` 必然「记录不存在」(批 17 遗留契约断裂,桌面验收一直未跑到) | 后端 `pendingRecs` 快照:挂起注册/决策移除/Stop 清理三处同步,`Record()` history 未命中转查快照;TDD 2 例(单挂起可查 + 3 并发挂起逐一可查)先 RED(`记录不存在`)后 GREEN | 094d555 |
| I2 | `decide()` 等 `waitRecord` 终态期间新到挂起会被无条件清空 | 按 id 移除(队列过滤),不再整槽清空 | 9cd73d6 |
| I3 | 单槽 pending vs 后端并发多挂起(页面加载丢挂起) | store 挂起改**队列** `pendings[]`+`pendingRaws{}`(按 id 可编辑),队首处理逐条决策;MiniStatus 显示真实数量 | 9cd73d6 |
| I1 | HEAD 不自洽:引用了未入库的 proxy.ts/categories/wailsjs 绑定/internal 引擎(**git status 把未跟踪目录折叠成单行,逐文件过滤漏掉了整个 internal/proxy、internal/browser 目录**) | 补齐批次 15-19 未入库源码(MITM 引擎、内置浏览器、两个 App 绑定、app.go 接线、modes/icons/wailsjs/proxy.ts、Chromium 脚本);**干净检出实测验证**:临时 worktree 挂 HEAD → `go build ./...` 0 退出 + `go test ./internal/proxy/ ./internal/browser/` 全过 + 前端 `tsc+vite` 构建成功 | 120add5 |

修复后全仓 `go test ./...`:17 包 ok;唯一失败 `aisec.TestImageAdvZipPath` 为既有环境性失败(依赖桌面真题附件,本批未触碰 aisec)。

Minor 顺延(未修):M1 历史选中行 `bg-brand/10` 无实际 CSS(var 色不支持透明度修饰,沿袭旧代码同类写法);M2 flash 4s 定时器叠加早清;M3 纯浏览器模式下轮询静默 throw-catch;M4 拦截/MITM 开关无 busy 防抖;M5 dev HMR 重载 store 遗留旧定时器(重载页面即愈)。M6(挂起数硬编码)已随 I3 队列化顺带解决。

## 6. 提交清单

f80537a store → ee9f1c0 拦截页 → 2148d9c 历史+迷你状态 → 22fb98c 重放器 → 1b3b7fe 接线切换 → aad72ba 本报告初版 → 094d555/9cd73d6 评审修复 → 120add5 依赖入库 → b0c4cd7 dist 重建。
