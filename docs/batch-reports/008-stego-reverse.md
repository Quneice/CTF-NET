# 批 8 报告 · Stego + Reverse(47 个:1 已实现 disasm / 46 占位)

- 生成时间:2026-11-10
- 范围:严格按用户指令「只实现第 8 批」;migration-plan §9 全部核心动作落地。
- 批次性质:**占位骨架批 + stego 分组 chips 落地批** —— 计划表 47 行中 reverse.static.disasm 早已接线(本地 mock 处理器,本批验证全链路),其余 46 个占位由批 0-2 简写行+预设框架生成的完整骨架(本批逐行核对零缺漏);本批实际源码改动 3 个文件:stego 4 个子分组按计划「分组 chips」呈现(复用批 7 机制)、whitespace 三人组描述互斥校准、命令面板结果行加分类徽标。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/mock/categories.ts` | stego 4 个子分组(image/audio/video/text)标记 `segment: true` —— 父页 /c/stego 以分组 chips(全部/图片隐写/音频隐写/视频隐写/文本隐写)呈现;**4 条子分类路由保留**(直接访问 /c/stego.image 等仍可达),侧边栏 L2 节点保留(同批 7 注释机制);全树 segment 节点 8 → 12;reverse 3 个子分类保持入口卡片(计划未要求 chips) |
| `frontend/src/mock/tools.ts` | 2 处单行替换(附加验收「描述含分类前缀、互不误导」):`stego.text.whitespace` 描述改为「文本隐写:检测空白字符隐写(与编码转换的 Whitespace 语言工具互不相同)」并补 `Whitespace` tag(搜索不匹配 id,tag 才能命中);`misc.encoding.whitespace` 描述改为「编码转换:空白字符编程语言(Whitespace esolang,与文本隐写的空白隐写工具互不相同)」。单行替换,无行号漂移 |
| `frontend/src/components/CommandPalette.tsx` | 结果行新增**分类徽标 = 分类名 chip + 完整工具 id**(批 5 重名区分同款,font-mono text-2xs),导入 `getCategoryById`。white-space 三人组靠 id 前缀区分(whitespace-codec / encoding.whitespace / text.whitespace);本组件为 Ctrl+K 全局搜索共享组件,所有搜索结果行统一受益 |
| `docs/tool-inventory.json` | 2 行 whitespace 描述/tags 与 tools.ts 同步;34 个计划内 stego 行 riskNotes 追加「批8 stego 分组 chips 化」注记(幂等);stats 不变(565/62/503) |
| `docs/batch-reports/008-stego-reverse.md` | 本报告 |

未改动:tools.ts 的 47 个定义本身(已完整,逐行核对全吻合)、router(本批重定向为「无」)、adapters、migration-plan.md(计划文档)、CategoryPage.tsx(批 7「全部」递归语义对 stego 纯分组分类自动适用,零改动)。

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` / `npm run build` | ✅ 0 错误 / 构建通过(1.59s) |
| 2 | 冒烟全量(smoke_b8.cjs) | ✅ **51 条断言,失败 0** |
| 3 | 计划表 47 行 vs 代码逐行 | ✅ 名称/分类/模板(通用 ToolForm±文件)/字段完整性逐行核对全吻合;计划外仅批 1 的 strings(计划已明示减除);已实现恰为 disasm,其余 46 占位 |
| 4 | stego 分组 chips | ✅ /c/stego 真实渲染:5 个 chips、默认选中「全部」(aria-pressed)、「工具 · 37」计数、34 个 stego.* 占位卡 + 3 个批 1 迁入卡(misc.stego.*,已实现徽章)、无子分类入口卡片(chips 取代);子页路由走查:image 27 / audio 2 / video 2 / text 3,迁入工具(wechat-dat、whitespace-codec)在对应子页可见 |
| 5 | reverse 保持入口卡片 | ✅ /c/reverse 渲染 3 张子分类入口卡片、无分组 chips;/c/reverse.static 显示 8 个工具,strings 与 disasm 双「已实现」徽章且卡片可见(批 1 的 strings 本批只验证展示) |
| 6 | 附加验收 1:搜索「whitespace」恰命中 3 个且分类徽标互异 | ✅ 命中恰为三人组(批 1 迁入 whitespace-codec + 批 2 codec + 本批 stego.text.whitespace),remove-whitespace 不误命中;命令面板渲染:徽标分类名 文本隐写 ×2 + 编码转换 ×1,三人组完整 id 均渲染、名称均渲染 |
| 7 | 附加验收 2:描述含分类前缀、互不误导 | ✅ misc.encoding.whitespace 前缀「编码转换:」、stego.text.whitespace 前缀「文本隐写:」,且双方描述互指区分;inventory 描述/tags 与 tools.ts 同步 |
| 8 | 附加验收 3:47 行与 inventory 一致 | ✅ 名称/分类/状态/evidence 行号四项逐一核对全吻合(2 处 tools.ts 编辑为单行替换,零漂移);34 个计划内 stego 行已记「批8 分组 chips」注记 |
| 9 | disasm 处理器全链路 | ✅ local.disasm 经真实 local adapter 驱动:arch=arm64 输出「arm64 反汇编(mock)」+ push rbp 等指令,mock 标注诚实 |
| 10 | 46 个占位 mock 路径 | ✅ 全部 46 个逐一按其真实 adapter 驱动到完成:零异常、零 error 事件、全部以 done 收尾 |
| 11 | 旧批次回归 | ✅ smoke_b4 27/27、smoke_b5 40/40、smoke_b6 19/19、smoke_b7 64/64 全部通过;b7 的 segment 计数断言更新为 12(跟踪批 8 演进,见 §3-4) |
| 12 | 清单一致性 | ✅ parity 0 缺行 / 0 状态不一致 / 0 清单独有行 / 0 分类不一致;565 前端行、63 分类节点不变 |

## 3. 实现说明

1. **「分组 chips」沿用批 7 机制**(口径已在 007 报告中明示并预告):migration-plan §9「stego.image/audio/video/text 分组 chips」与 §8 措辞一致 → stego 4 个子分组 segment:true;stego 为纯分组分类(本层 0 直挂),批 7 的「全部 = 递归取分类下全部」语义**自动适用**,CategoryPage 本批零改动(37 = 34 占位 + 3 个批 1 迁入的已实现工具)。
2. **徽标互异假设**(在报告中明示):批 1 已记录 whitespace-codec 位于 stego.text(001-lighthouse 迁移决策,不做回退),因此「仅靠分类名」无法区分它与本批的 stego.text.whitespace(同属 stego.text);采用批 5 重名区分同款方案 —— **分类名 chip + 完整工具 id**,三人组徽标逐对互异(id 前缀 whitespace-codec / encoding.whitespace / text.whitespace)。若阶段 4 推翻批 1 迁移决策(纯分类名互异方案),本徽标随之简化。
3. **CommandPalette 是共享组件**:结果行徽标对所有搜索生效(不只 whitespace 三人组),属预期内的全局一致性增强;行内容为 名称 + 分类 chip + id + tags,均 truncate/shrink 防溢出。
4. **冒烟工具链的 2 个修正**(不影响产品代码):(a) zustand v4 SSR 快照 = getInitialState(模块加载时的 localStorage),setState 对 renderToString 不可见 —— 收藏改经 localStorage stub 在 bundle 求值前注入;(b) disasm 处理器文本为全角「(mock)」,断言曾用半角。
5. **回归烟幕断言演进**:b7 冒烟的「全树恰 8 个 segment」是批 7 时点快照;批 8 增 4 个(stego)后更新为 12,标签注明跟踪演进,其余 63 条断言未动。

## 4. 剩余风险

1. **46 个占位无真实实现**:与 inventory 一致(46 placeholder),徽章+「模拟结果」chip 已标注,不伪装可用;实现留待后续批次(disasm 为已接线的本地 mock 处理器,非真实反汇编引擎)。
2. **whitespace 三人组区分依赖徽标 + 描述前缀**:静态渲染断言已证互异;命令面板交互(↑↓/回车直达、点击导航)未做浏览器点击验证,建议桌面端复核一次。
3. **CommandPalette 全量视觉未真机验证**:结果行新增分类 chip + id 后行密度增大,小屏/长 id 场景建议 `wails dev` 复核(truncate 已就位,防溢出理论上成立)。
4. **stego 分组 chips 交互未做浏览器点击验证**(同批 7 风险):chip 过滤语义经 getToolsUnderCategory 集合断言+渲染断言验证;/c/stego 37 卡滚动到底的视觉层建议手动复核(同批 7 风险 2)。
5. **misc.encoding.whitespace 描述更新使批 2 报告的描述引文略有过时**:本批按附加验收改写其描述(前缀「编码转换:」),002 报告中的旧描述引文不再逐字对应,属文档历史快照,无需回改。
6. **后续批次若再增 segment 节点**:相关历史冒烟的 segment 计数断言需同步演进(先例:b7 8 → 12)。
