# 批 6 报告 · Crypto.Hash(45 个:0 已实现 / 45 占位)

- 生成时间:2026-11-10
- 范围:严格按用户指令「只实现第 6 批」;migration-plan §7 全部核心动作落地。
- 批次性质:**纯骨架验证批(零源码改动)** —— 45 个占位工具在批 0-2 的数据框架中已具备完整定义(名称/描述/tags/version/inputSchema/invoke/output 零缺失),本批无重定向、无实现动作,全部工作为逐项验证:占位标识正确性、mock 路径无未捕获异常、批 1 工具 `crypto.hash.hash` 的分类页展示,以及两条附加验收(排序、徽章计数)。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `docs/batch-reports/006-hash.md` | 本报告(唯一新增文件) |

**零源码改动**:tools.ts / categories.ts / router / adapters / inventory 均未动 —— parity 在批 5 结束时即为 0,本批数据核对无任何不一致,无需补丁。migration-plan.md 未动(计划文档,与批 3-5 惯例一致)。

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` / `npm run build` | ✅ 0 错误 / 构建通过(1.61s) |
| 2 | 冒烟全量(smoke_b6.cjs) | ✅ **19 条断言,失败 0** |
| 3 | 计划表 45 行 vs 代码集合相等 | ✅ §7 表格 45 行全部存在;无计划外新增(hash 为批 1 工具,已排除);crypto.hash 目录恰 46 个 |
| 4 | 状态推导:45 占位 + hash 已实现 | ✅ 45 个全部推导为占位;crypto.hash.hash 为已实现;45 个占位 invoke 仅 local/http(无 task/sse/ws,无 realBackend 路径);41 个 local 占位的 handlerId 均未注册(不会误判已实现) |
| 5 | 风险项「mock 路径无未捕获异常」 | ✅ **全部 45 个占位逐一按其真实 adapter 驱动到完成**(local 80ms / http 600ms 并行):零异常、零 error 事件、恰 1 个 done(mock 结果);44 个 text 输出为参数回显,1 个 table 输出(rainbow,预设 table)形态正确 |
| 6 | 附加验收 1:默认排序 hash 在最前 | ✅ 三层证据:(a) tools.ts 中 hash@索引 6 < 首个占位@索引 81;(b) toolsByCategory 按数组序 push 保序;(c) CategoryPage 无重排序;渲染结果 hash 卡片位置在全部 45 个占位卡片之前 |
| 7 | 附加验收 2:占位徽章数量 = 45 | ✅ /c/crypto.hash 真实渲染(react-dom/server):`状态:占位` ×45、`状态:已实现` ×1、卡片 46 张、无子分类入口卡片 |
| 8 | 清单一致性(parity_check_b1.cjs) | ✅ 0 缺行 / 0 状态不一致 / 0 清单独有行 / 0 分类不一致;565 前端行、63 分类节点不变 |

## 3. 实现说明

1. **为何零改动**:批 0-2 的「简写行 + 预设」框架(shorthandRows + presets)已为 45 个占位生成完整 ToolDefinition;状态由 getToolStatus 接线自动推导为占位,徽章、模拟结果 chip(批 3 落地)均由既有机制呈现 —— 本批唯一义务是证明这些机制对 45 个工具逐一成立,而非新增代码。
2. **invoke 分布**:41 个 `local`(预设 local,handlerId `local.crypto.hash.*` 未注册 → localAdapter 走 mock 回退)+ 4 个 `http`(hashid/crc32/hmac 无预设默认 text,rainbow 预设 table;httpAdapter 本就无网络调用,600ms 延迟后直接回 mock)。两种路径均无真实后端、无 console error 风险。
3. **排序验收的三层证据**:配置序(数组索引)→ 聚合序(保序 push)→ 渲染序(HTML 位置)逐层对齐,避免「配置对但渲染错」的隐性回归。

## 4. 剩余风险

1. **45 个占位无真实实现**:与 inventory 状态一致(45 placeholder),真实实现留待后续批次;分类页与详情页均已按规则标注「占位」徽章 +「模拟结果」chip,不伪装可用。
2. **桌面端手动复核建议**:adapter 级与渲染级证据均为脚本驱动(wailsjs 绑定在纯浏览器环境走 mock 回退);`wails dev` 手动打开 /c/crypto.hash 与任一占位工具提交一次,复核控制台无报错,建议保留为例行步骤。
3. **rainbow 为 table 输出**:mock 表结果(列标题-R 行)不含参数回显,与本批其余 44 个 text 输出的表现不一致,但不违反「模拟结果」标注约定;实现阶段按真实后端调整。
