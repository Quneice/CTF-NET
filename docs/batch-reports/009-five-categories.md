# 批 9 报告 · 长尾五小分类(pwn 11 + osint 6 + blockchain 7 + mobile 4 + ai 3 = 31 个,0 已实现 / 31 占位)

- 生成时间:2026-11-10
- 范围:严格按用户指令「只实现第 9 批」;migration-plan §10 全部核心动作落地。
- 批次性质:**纯骨架验证批 + 五分类分组 chips 落地批** —— 31 个占位工具已由批 0-2 简写行+预设框架生成完整骨架(本批逐行核对零缺漏,tools.ts 零编辑);本批实际源码改动仅 1 个文件:五个分类的 15 个 L2 子分组按计划「每分类分组 chips 同 L2」标记 segment(migration-plan §10,复用批 7/8 机制)。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/mock/categories.ts` | pwn(4)/osint(4)/blockchain(3)/mobile(2)/ai(2)共 **15 个 L2 子分组标记 `segment: true`** —— 每个分类父页(/c/pwn、/c/osint、/c/blockchain、/c/mobile、/c/ai)以分组 chips(全部 + L2 名称)呈现;**15 条子分类路由全部保留**,侧边栏 L2 节点保留(同批 7/8 注释机制);全树 segment 节点 12 → 27;reverse 保持入口卡片(批 8 决策不回退) |
| `docs/tool-inventory.json` | 31 个计划行 riskNotes 追加「批9 五小分类分组 chips 化」域级注记(de-bruijn 行记迁入注记);**无任何 status/category/name/evidence 变更**(tools.ts 零编辑 → 零行号漂移);stats 不变(565/62/503) |
| `docs/batch-reports/009-five-categories.md` | 本报告 |

未改动:tools.ts(31 个定义已完整,逐行核对全吻合,含 de-bruijn 的 categoryId 已在批 0 审计时同步为 pwn.stack)、CategoryPage.tsx(批 7「全部」递归语义对 5 个纯分组分类自动适用)、router(本批重定向为「无」)、adapters、migration-plan.md。

## 2. 验收清单(逐项结果,按域分别给出)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` / `npm run build` | ✅ 0 错误 / 构建通过(1.86s) |
| 2 | 冒烟全量(smoke_b9.cjs) | ✅ **87 条断言,失败 0** |
| 3 | 计划表 31 行 vs 代码逐行 | ✅ 名称/分类/模板(通用 ToolForm±文件)/字段完整性逐行核对全吻合;±文件 11 行与计划完全一致;五域合计恰 31 个,无计划外工具 |
| 4 | 按域迁移数与抽样核对 | ✅ **pwn 11**(10 个 pwn.* + 迁入的 misc.other.de-bruijn;抽样:overflow-calc/rop-gadget+文件/shellcode.encoder 重名)、**osint 6**(抽样:geolocate+文件、whois)、**blockchain 7**(抽样:trace 全对象 http PUT + tx-graph 自定义渲染、keystore-crack+文件)、**mobile 4**(抽样:apk-decompile/dex-parse+文件)、**ai 3**(抽样:injection-test);31 行全部占位,0 已实现,与计划一致 |
| 5 | 附加验收 1:五个 /c/:id 分类页均可访问 | ✅ /c/pwn、/c/osint、/c/blockchain、/c/mobile、/c/ai 全部真实渲染成功 |
| 6 | 分组 chips 五分类统一 | ✅ 每页:chips = 全部 + L2 名称(pwn 5 个 / osint 5 个 / blockchain 4 个 / mobile 3 个 / ai 3 个)、默认选中「全部」(aria-pressed)、「工具 · 11/6/7/4/3」计数、卡片数一致、11/6/7/4/3 个占位徽章 0 已实现、无子分类入口卡片(chips 取代) |
| 7 | 子分类路由保留(抽查 5 条) | ✅ /c/pwn.stack 5 个(含迁入 de-bruijn)、/c/pwn.shellcode 2 个、/c/osint.geo 1 个、/c/mobile.ios 2 个、/c/ai.analysis 2 个 |
| 8 | 附加验收 2:搜索「shellcode」重名对 | ✅ 命中 pwn.shellcode.encoder 与 misc.encoding.shellcode 两个重名工具,且两卡片标注分类可区分:pwn 页卡片副题「Shellcode + pwn.shellcode.encoder」、codec 页卡片副题「编码转换 + misc.encoding.shellcode」(批 5 卡片模式,零代码改动);总命中如实记录为 3(含同名含「Shellcode 生成」的 generator,见 §3-3) |
| 9 | 31 行与 inventory 一致 | ✅ 名称/分类/状态(全占位)/evidence 行号四项逐一核对全吻合;31 行已记「批9」注记;清单 de-bruijn 分类 = pwn.stack |
| 10 | 31 个占位 mock 路径 | ✅ 全部 31 个逐一按其真实 adapter 驱动到完成:零异常、零 error 事件、全部以 done 收尾 |
| 11 | 旧批次回归 | ✅ smoke_b4 27/27、b5 40/40、b6 19/19、b7 64/64、b8 51/51 全部通过;b7/b8 的 segment 计数断言 12 → 27(跟踪批 9 演进) |
| 12 | 清单一致性 | ✅ parity 0 缺行 / 0 状态不一致 / 0 清单独有行 / 0 分类不一致;565 前端行、63 分类节点不变 |

## 3. 实现说明

1. **「每分类分组 chips 同 L2」沿用批 7 机制**(口径在 007/008 报告中已预告):五分类均为纯分组分类(本层 0 直挂,冒烟断言),批 7 的「全部 = 递归取分类下全部」语义自动适用,CategoryPage 本批零改动;chips 标签即 L2 名称(栈溢出/堆利用/格式化字符串/Shellcode 等)。
2. **de-bruijn 迁移在批 0 审计时已同步**:tools.ts 与 inventory 的 categoryId 均已是 pwn.stack,本批仅核对 + 注记;其 id 保留 `misc.other.*` 前缀(工具 URL /t/misc.other.de-bruijn 不变,规则 8 不删旧路由),id 改名留待阶段 4。
3. **「shellcode」重名对零代码改动**:计划 §10 的风险「重名 shellcode」与附加验收由批 5 的 ToolCard 副题(分类名 + 完整 id)与批 8 的命令面板徽标已满足 —— 两卡片/两面板行标注互异分类(Shellcode / 编码转换)+ 完整 id。搜索总命中为 3(重名对 + 「Shellcode 生成」generator):计划文本「命中…两个」指重名对两个均命中,非总命中数,如实记录于此。
4. **tools.ts 零编辑 → 清单零漂移**:31 行证据行号全吻合,无需行号刷新(区别于批 7 的 533 行刷新)。
5. **回归冒烟断言演进**:b7/b8 的「全树 segment 数」是各自批时点快照,批 9 +15 后更新为 27(先例:批 8 时 b7 8 → 12)。

## 4. 剩余风险

1. **31 个占位无真实实现**:与 inventory 一致(31 placeholder),徽章+「模拟结果」chip 已标注,不伪装可用;实现留待后续批次。
2. **shellcode 重名对仍同名**:区分依赖卡片/面板的分类标注 + 完整 id(已验证互异);正式改名(如「Shellcode 十六进制编码」)留待阶段 4,届时同步批 2/本批两行与清单。
3. **五分类 chips 交互未做浏览器点击验证**(同批 7/8 风险):chip 过滤语义经 getToolsUnderCategory 集合断言+渲染断言验证;/c/pwn 11 卡、/c/blockchain 7 卡滚动到底的视觉层建议桌面端 `wails dev` 复核。
4. **de-bruijn 的 misc.other.* 前缀**:位于 pwn.stack 但 id 前缀为 misc.other,侧边栏/面包屑按 categoryId 呈现不受影响;搜索「de bruijn」按名称命中不受影响;id 改名留待阶段 4。
5. **后续批次若再增 segment 节点**:相关历史冒烟的 segment 计数断言需同步演进(先例:b7 8→12→27)。
