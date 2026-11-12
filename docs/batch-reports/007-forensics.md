# 批 7 报告 · Forensics(54 个:0 已实现 / 54 占位)

- 生成时间:2026-11-10
- 范围:严格按用户指令「只实现第 7 批」;migration-plan §8 全部核心动作落地。
- 批次性质:**全占位批 + 分组 chips 落地批** —— 54 个占位工具已由批 0-2 简写行+预设框架生成完整骨架(本批逐行核对零缺漏);本批实际源码改动 2 个文件:forensics 4 个子分组按计划「分组 chips」呈现(migration-plan §8「含 zip-tools/binwalk 等取证向工具分组 chips」,复用批 4 FilterChip 机制)+「全部」chip 对纯分组分类的语义补齐。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/mock/categories.ts` | forensics 4 个子分组(memory/disk/network/log)标记 `segment: true` —— 父页 /c/forensics 以分组 chips(全部/内存取证/磁盘取证/流量分析/日志分析)呈现;**4 条子分类路由保留**(直接访问 /c/forensics.disk 等仍可达),侧边栏 L2 节点保留(批 4 注释机制);全树 segment 节点 4 → 8 |
| `frontend/src/pages/CategoryPage.tsx` | 「全部」chip 语义补齐:纯分组分类(有分组子分类但本层无直挂工具,如 forensics)时「全部」= 分类下全部工具(getToolsUnderCategory);直挂非空的分类(批 4 的 crypto.classical 48 个直挂)语义不变;组件文档注释同步 |
| `docs/tool-inventory.json` | 54 个 forensics 行 riskNotes 追加「批7 forensics 分组 chips 化」注记;**全表 533 行 evidence 行号刷新**(见 §3 说明,纯证据校正,status/category/name 零改动);stats 不变(565/62/503) |
| `docs/batch-reports/007-forensics.md` | 本报告 |

未改动:tools.ts(54 个定义已完整,逐行核对全部吻合)、router(本批重定向为「无」)、adapters、migration-plan.md(计划文档)。

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` / `npm run build` | ✅ 0 错误 / 构建通过(1.88s) |
| 2 | 冒烟全量(smoke_b7.cjs) | ✅ **64 条断言,失败 0** |
| 3 | 计划表 54 行 vs 代码逐行 | ✅ 名称/分类/模板(通用 ToolForm±文件)/状态(占位)逐行核对全吻合;无计划外工具;分组分布 磁盘 28 / 流量 22 / 内存 2 / 日志 2 |
| 4 | 分组 chips | ✅ /c/forensics 真实渲染:5 个 chips(全部+4 分组)、默认选中「全部」(aria-pressed)、「工具 · 54」计数、无子分类入口卡片(chips 取代)、卡片按分组顺序排列(内存→磁盘→流量→日志) |
| 5 | 附加验收 1:54 行与 inventory 一致 | ✅ 名称/分类/状态占位/evidence 行号四项逐一核对全吻合(行号漂移见 §3) |
| 6 | 附加验收 2:分类页滚动到底无布局错位 | ✅ 静态层:54 张卡片完整渲染(滚动内容可达)、响应式网格(1/2/3/4 列)、ToolCard truncate+line-clamp-2 防溢出、chips 容器 overflow-x-auto、分类页无固定高度/纵向裁剪样式;视觉层建议桌面端手动复核(见 §4) |
| 7 | 风险处置:mock 路径较多(http 7/task 6/local 41) | ✅ **全部 54 个逐一按其真实 adapter 驱动到完成**:零异常、零 error 事件、全部以 done 收尾;task 恰 10 步进度+done,http 一次返回 |
| 8 | 批量验收模板首秀:取证向工具 tag chips | ✅ zip-tools/binwalk/carve/yara/scan-embedded/remove-exif 卡片均含「取证」tag chip + 占位徽章;子分类页走查:磁盘 28 / 流量 22 / 内存 2 / 日志 2 全部可达 |
| 9 | 旧批次回归 | ✅ smoke_b4 27/27、smoke_b5 40/40、smoke_b6 19/19 全部重跑通过;crypto.classical「全部」仍为 48 个直挂(语义未变) |
| 10 | 清单一致性 | ✅ parity 0 缺行 / 0 状态不一致 / 0 清单独有行 / 0 分类不一致;565 前端行、63 分类节点不变 |

## 3. 实现说明

1. **「分组 chips」的落地口径**(假设,已在报告中明示):migration-plan §8「含 zip-tools/binwalk 等取证向工具分组 chips」按批 4 FilterChip 机制落地 —— forensics 为纯分组分类(本层无直挂,54 个工具全在 4 个子分组内),父页以 chips 替代子分类入口卡片;这与 ia.md §4 导航树一致(侧边栏 L2 节点保留、4 条子分类路由保留)。批 8(stego)与批 9(五小分类)计划中的「分组 chips」将沿用同机制。
2. **「全部」语义的最小改动**:原实现「全部 = 直挂工具」对 forensics(直挂 0)会渲染空页。补齐规则仅作用于「有分组子分类且本层无直挂」的分类 —— 直挂非空的 crypto.classical 完全不受影响(48 张回归断言证实),无分组子分类的页面路径零改动。
3. **evidence 行号漂移修复**(本批冒烟暴露):批 0 审计后 tools.ts 前部曾插入 2 行,导致 533 行 evidence.line 全部 -2 漂移(批 5 补丁只刷新了其 22 行)。本批按规则 2(证据必须真实)在清单补丁中全表刷新行号 —— 纯 evidence.file 校正,status/category/name/riskNotes 等语义字段零改动,parity 复跑 0 不一致,补丁幂等(重跑 0 更新)。32 行 evidence 不指向 tools.ts 的行(迁移自 Go/legacy 的记录)不在刷新范围。

## 4. 剩余风险

1. **54 个占位无真实实现**:与 inventory 一致(54 placeholder),徽章+「模拟结果」chip 已标注,不伪装可用;实现留待后续批次。
2. **「滚动到底无布局错位」的视觉层未做真机验证**:静态断言(54 卡完整渲染、响应式网格、防溢出类、无固定高度)已覆盖脚本可达部分;`wails dev` 手动滚动 /c/forensics 至底部复核一次建议保留。
3. **分组 chips 交互未做浏览器点击验证**:chip 过滤语义经 getToolsUnderCategory 集合断言+渲染断言验证;鼠标点击/键盘操作建议桌面端复核。
4. **32 行 evidence 不指向 tools.ts 的记录**:行号未刷新(非本批范围),其 evidence 指向迁移前的 Go/legacy 位置,属既有审计债务,阶段 4 统一处置。
5. **批 8/9 的「分组 chips」将沿用本批语义**:若用户对「全部 = 直挂优先、纯分组取递归」口径有异议,请在批 8 开始前告知,以避免同口径扩散。
