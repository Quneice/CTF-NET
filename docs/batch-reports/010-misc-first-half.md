# 批 10 报告 · misc 前半(49 个,0 已实现 / 49 占位;demo 4 + other 45)

- 生成时间:2026-11-10
- 范围:严格按用户指令「只实现第 10 批」;migration-plan §11 全部核心动作 + 附加验收落地。
- 批次性质:**骨架 + 徽章 + misc 分组 chips + task 标签体系首次落地批** —— 49 个占位工具(批 0-2 简写行+预设框架)本批零 tools.ts 结构编辑(仅 tags 行内追加,行数不变 → 零行号漂移);misc.other + misc.demo 标记 segment(「其他」「示例演示」chips),misc.report 保持子分类入口卡片(批 11 收口);按 ia.md §2 受控词表为 49 行回填 `task:` 标签并落地「任务」维度过滤。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/mock/tools.ts` | **49 行 tags 数组追加 `task:` 标签**(行内改写,总行数 2620 不变 → evidence 行号零漂移)。分配:演示 4(demo 组)/ 转换 14 / 计算 13 / 分析 8 / 格式化 5 / 解析 1(colour-code)/ 提取 2(extract-dates、git-extract)/ 生成 2(get-time、hotp)= 49;49 行全部受控词表内 |
| `frontend/src/mock/categories.ts` | misc 下 `misc.demo`(示例演示)与 `misc.other`(其他)标记 `segment: true` —— 父页 /c/misc 顶部以分组 chips(全部/其他/示例演示)呈现;`misc.report`(报告 / 工具)保持子分类入口卡片(§11「本段所含」= other + demo);两条子分类路由保留,侧边栏 L2 保留;全树 segment 27 → 29 |
| `frontend/src/lib/task.ts` | **新建,任务维度唯一事实源**:18 个受控动词词表、`taskTag()`、`taskValuesOf()`、`keywordTagsOf()`、`availableTasks()`(词表序去重)、`filterByTask()` |
| `frontend/src/pages/CategoryPage.tsx` | 新增「任务」chips 行(分组 chips 之后、子分类入口卡之前):从当前工具集合的 task: 标签派生(全部 + 词表序动词),与分组 chip **AND 组合**过滤;切换分类时一并重置;无 task: 标签的集合不渲染该行 |
| `frontend/src/pages/ToolPage.tsx` | 工具详情页标签拆分渲染:任务标签(品牌色描边 chip,`text-brand`)与自由关键词(原灰底样式)分离 |
| `frontend/src/components/tool/ToolCard.tsx` | 卡片 tags 区改为 `keywordTagsOf(tool.tags).slice(0, 4)`(不再展示 `task:` 前缀标签) |
| `frontend/src/components/CommandPalette.tsx` | 面板行尾标签改为 `keywordTagsOf(tool.tags).slice(0, 2)`(同上) |
| `docs/tool-inventory.json` | 49 行 tags 与 tools.ts 同步;riskNotes 追加「批10 task:X 标签回填(§2 受控词表,关键词自动初填待人工复核);misc 分组 chips 化(other+demo segment)」;stats 不变(565/62/503) |
| `docs/batch-reports/010-misc-first-half.md` | 本报告 |

未改动:router(§11 重定向为「无」)、adapters、migration-plan.md、design-system.md(口径差异见 §3-3)、其余 47 个 misc.other 工具(批 11 范围纪律,未回填 task 标签)。

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` / `npm run build` | ✅ 0 错误 / 构建通过(1.88s) |
| 2 | 冒烟全量(smoke_b10.cjs) | ✅ **61 条断言,失败 0** |
| 3 | 计划表 49 行 vs 代码逐行 | ✅ 名称/分类/模板(±文件)/字段完整性逐行核对全吻合;±文件恰为 git-extract 1 行;demo 4 + other 45 与计划一致 |
| 4 | 状态推导 | ✅ 49 行全部占位、0 已实现(与 §11「0 已实现 / 49 占位」一致);49 个占位经各自真实 adapter 逐一驱动到 done:零异常、零 error 事件 |
| 5 | **附加验收 1:task 标签覆盖 100%** | ✅ 49 行全部含 `task:` 标签(8 个动词:转换/解析/生成/提取/分析/演示/格式化/计算),全部在 §2 的 18 个受控动词内,零越界;批 11 工具与 file_checksum 未回填(范围纪律,冒烟断言其 task 标签为空) |
| 6 | **附加验收 2:「任务」维度过滤生效** | ✅ 纯函数直测:availableTasks(49) = 词表序 8 动词;filterByTask 各动词计数 = 演示 4/转换 14/计算 13/分析 8/格式化 5/解析 1/提取 2/生成 2,并集 = 49 互不重叠,全部命中该动词;无该动词返回空 |
| 7 | /c/misc 页面 | ✅ 分组 chips(全部/其他/示例演示,默认「全部」aria-pressed)+ 任务 chips 行(全部 + 8 动词)+ report 入口卡片(/c/misc.report)+ 无 other/demo 入口卡片(chips 取代);「全部」= 域内 99 卡(93 other + 4 demo + 2 report),计数「工具 · 99」,97 占位徽章 + 2 已实现(file_checksum/writeup);49 个批 10 id 全部在场;卡片不渲染 `task:` 原文 |
| 8 | 子分类路由保留 | ✅ /c/misc.other = 93 卡(任务行含 7 动词,无演示)、/c/misc.demo = 4 卡(任务行仅「演示」)、/c/misc.report = 2 卡(无任务行,批 11 收口) |
| 9 | 工具详情页 | ✅ /t/misc.other.case:任务 chip「转换」品牌色渲染 + 关键词 chips(文本/大小写)样式不变 |
| 10 | 49 行与 inventory 一致 | ✅ 名称/分类/状态(全占位)/tags/evidence 行号五项逐一核对全吻合;49 行已记「批10」注记;frontend stats 565/62/503 无漂移 |
| 11 | 旧批次回归 | ✅ smoke_b2 55/55、b3 39/39、b4 27/27、b5 40/40、b6 19/19、b7 64/64、b8 51/51、b9 87/87 全部通过;b7/b8/b9 的 segment 计数断言 27 → 29(跟踪批 10 演进) |
| 12 | 清单一致性 | ✅ parity 0 缺行 / 0 状态不一致 / 0 清单独有行 / 0 分类不一致;565 前端行、63 分类节点不变 |

## 3. 实现说明

1. **misc 直挂 = 0 → 「全部」递归语义自动适用**:misc 为纯分组分类(本层 0 直挂,批 7 先例),「全部」chip 自动取域内全部 99 个工具,CategoryPage 全部语义本批零改动,仅新增任务 chips 行。
2. **task 标签体系(ia.md §2)首次落地**:受控词表 18 动词收敛在 `lib/task.ts` 单一事实源;筛选只消费受控词表(§14-3 清单即界面,页面不写死动词);阶段 0 的关键词自动初填 → 本批人工初核分配,最终分布 = 演示 4 / 转换 14 / 计算 13 / 分析 8 / 格式化 5 / 解析 1 / 提取 2 / 生成 2(与实现前分配备忘差 1:code-beautify/comment/css-tidy/js-tools/json-tidy 归「格式化」共 5 —— 以 tools.ts 为唯一事实源,已如实记录)。
3. **任务维度用 chips 行而非下拉**:design-system §3.3 曾写「6 维下拉」,与现有分组 chips 架构及移动端横向滚动口径不一致 —— 本批以 chips 行落地(与分组 chip AND 组合),design-system 的表述更新留待阶段 4 统一收口时同步。
4. **演示组 task:演示 先行满足**:§12 曾提「demo 组(4 个)与 beast」的 task 标签 —— demo 4 个本批即标,批 11 只补 beast 与其余 47 个 other 工具的 task 标签。
5. **搜索行为变化(如实记录)**:task: 标签进入 tools.ts 的 tags 数组,使「转换」「分析」等动词词可通过名称/描述/标签模糊搜索命中工具;卡片/命令面板展示层已剔除 `task:` 前缀(仅详情页任务 chip 展示)。
6. **任务筛选的增量诚实状态**:批 11 的 47 个 other 工具尚无 task 标签 —— 在 /c/misc 选中任务动词时它们被过滤(与 inventory 占位状态一致的增量呈现),批 11 收口后 100% 覆盖。
7. **回归冒烟断言演进**:b7/b8/b9 的「全树 segment 数」快照 27 → 29(先例:批 8 时 8 → 12、批 9 时 12 → 27)。

## 4. 剩余风险

1. **49 个占位无真实实现**:与 inventory 一致(49 placeholder),徽章 +「模拟结果」chip 已标注,不伪装可用;实现留待后续批次。
2. **task 分配待人工复核**:阶段 0 关键词自动初填 + 本批人工初核的 49 个动词分配(特别是转换 14 / 格式化 5 边界:comment、js-tools 归格式化)尚未经用户逐词确认;词表与分配集中在 `lib/task.ts` + tools.ts tags,复核成本低。
3. **批 11 工具暂无 task 标签**:任务筛选对它们表现为「被过滤」;批 11 收口(misc 后半 + report,含 beast)时补齐并同步 §12。
4. **chips 交互未做浏览器点击验证**(同批 7/8/9 风险):chip 过滤语义经纯函数断言 + 渲染断言验证;/c/misc 99 卡与双 chips 行的视觉层建议桌面端 `wails dev` 复核。
5. **design-system §3.3 口径滞后**:本批任务维度以 chips 行落地,与文档「6 维下拉」不一致,文档更新留待阶段 4。
