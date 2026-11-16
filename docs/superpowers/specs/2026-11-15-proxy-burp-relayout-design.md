# 设计 · 代理拦截 Burp 式布局改版(方案 A)

- 日期:2026-11-15
- 状态:已批准(用户在对话中确认「方案 A 可以」,含两项默认决定)

## 意图

当前 `ProxyWorkspace`(641 行)把控制栏、MITM 控制、拦截、历史、Repeater 全部堆在一个工具页内,历史页内又左右分栏,空间紧张;工具页头部(面包屑+工具名+「已实现」徽章+描述+tags 徽章行「利用 代理 拦截 重放」)吃掉约 150px 垂直空间。参考 Burp Suite 的模块化布局拆分。

## 目标

1. `pentest.proxy` 分类下拆为 3 个工具页(keep-alive 多开标签,Burp 顶部标签同构):
   - `pentest.proxy.intercept`(保留原 id)「代理拦截」:控制栏(启动/停止/端口/拦截开关/MITM/CA 导出安装/内置浏览器)+ 挂起请求编辑与放行/修改放行/丢弃。
   - `pentest.proxy.history`(新)「流量历史」:全宽历史表格(#/方法/状态/主机路径/耗时/拦截标记),点行在下方展开请求/响应详情 +「发送到重放器」按钮。
   - `pentest.proxy.repeater`(新)「重放器」:Burp 式数字标签(1/2/3…),每标签独立请求编辑器/发送/响应/元信息,可关闭。
2. 共享状态抽到新 `useProxyStore`(zustand):status、history、pending 挂起记录、repeater 标签数组、flash 消息;轮询生命周期由「任一代理页挂载」引用计数驱动(keep-alive 标签切走仍挂载,轮询不断)。
3. 「发送到重放器」:历史详情与拦截页均可触发;追加数字标签并导航到 `/t/pentest.proxy.repeater`。
4. 三个页全部 `bareWorkspace`(头部整块隐藏,工作区顶格);历史/重放器页右上角迷你状态点(运行中端口/拦截态),点击跳回代理拦截页。

## 非目标

- 后端零改动(`proxy_api.go`/`internal/proxy` 不动)。
- 不做右键上下文菜单、不做 Intruder、不做请求/响应的格式化美化(pretty/headers 参数化视图)。
- 不做跨重启持久化(会话内 keep-alive 已够,与现状一致)。

## 方案取舍

- **A(选定)**:3 工具页 + zustand store + bareWorkspace。「单独一个页面」字面达成,复用 keep-alive 标签机制,后端零改动;代价是分类树多两个入口、需要 store 重构。
- B(1 工具内部子路由):页内自造导航,名不副实,不如 A 顺 — 否决。
- C(升格一级路由,如 AI 工作台):代理只是渗透模式一个工具域,升格过重 — 否决。

## 已定细节

- 原工具改名「代理拦截」,描述收窄;`workspace` 值:`proxy-interceptor`(intercept)、`proxy-history`、`proxy-repeater`(后两者新增,同样走 bare 头部)。
- Repeater 为多数字标签(可同时挂多个请求对比),非单编辑器。
- 历史轮询从组件内 `setInterval` 移入 store(引用计数),拦截事件订阅同理。
- 「发送到重放器」从拦截页挂起报文也可触发(与历史页一致)。

## 留给实现

- store 轮询引用计数具体实现;repeater 标签 id 生成;mini 状态点样式;空态文案。
