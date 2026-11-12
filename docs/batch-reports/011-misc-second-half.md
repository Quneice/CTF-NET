# 批 11 报告 · misc 后半(47 other + 2 report = 49 个,1 已实现 / 48 占位)+ beast 演示标签 + 全量收口

- 生成时间:2026-11-10
- 范围:严格按用户指令「只实现第 11 批」;migration-plan §12 全部核心动作 + 附加验收落地。
- 批次性质:**收口批** —— 全部 11 批完成:misc 域任务维度收口(98/99)、task 标签回填 50 处(49 计划行 + beast)、php-unserialize 全链路回归、565 行清单与运行时 UI 一致性全量核对、63 分类页 + 565 工具页逐一真实渲染。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/mock/tools.ts` | **50 处 task: 标签回填**(行内追加/tags 行精确替换,总行数 2620 不变 → evidence 行号零漂移)。分配:转换 16 / 计算 8 / 生成 5(含 writeup)/ 识别 2 / 解析 2 / 爆破 2 / 提取 1(regex)/ 演示 4(register/return/sleep + §12 显式的 beast)/ 分析 3(图表类)/ 编解码 1(ms-script-decode)/ 格式化 5(label/subsection/syntax-highlight/sql-tidy/xml-tidy)/ 校验 1(flag-checker)= 50 |
| `docs/tool-inventory.json` | 50 行 tags 与 tools.ts 同步;riskNotes 追加「批11 task:X 标签回填(§2 受控词表,misc 任务维度收口)」;stats 不变(565/62/503) |
| `docs/batch-reports/011-misc-second-half.md` | 本报告 |

未改动:categories.ts(§12 无 chips/segment 动作,report 保持入口卡片)、router(重定向为「无」)、adapters、php-unserialize 工作区(批 1 已落地,本批仅回归)、file_checksum(批 1 工具,历批范围外,仍无 task 标签)、php-unserialize 工具行(web 域,本批仅回归不标标签)。

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` / `npm run build` | ✅ 0 错误 / 构建通过(1.67s) |
| 2 | 冒烟全量(smoke_b11.cjs) | ✅ **81 条断言,失败 0** |
| 3 | 计划表 49 行 vs 代码逐行 | ✅ 名称/分类/模板(±文件)/字段完整性逐行核对全吻合;±文件恰 3 行(magic-bytes/rar-crack/zip-brute);other 47 + report 2 与计划一致 |
| 4 | 状态推导 | ✅ 1 已实现(writeup,local.writeup 已注册)+ 48 占位(与 §12 头一致);49 行经各自真实 adapter 逐一驱动到 done,零异常零 error |
| 5 | 附加验收 1:php-unserialize 全链路回归 | ✅ 输入 PHP 类源码(public/protected/private)→ adapter 输出含 类名/序列化 PoC/\\x00(protected/private)/URL 编码/s:2:"id";parsePhpClass 识别三种可见性;buildPhpSerialized 可读版+URL 编码版(含 %00);/t/web.deserialize.php-unserialize 渲染 PhpUnserializeWorkspace |
| 6 | task 标签 50 处 | ✅ 49 计划行 + beast 各恰 1 个 task 标签,全部在 18 受控动词内;beast=演示(§12 显式);demo 组 4 个保持批 10 的演示(不重复);file_checksum 与 php-unserialize 仍未标(范围纪律);批 10 工具未被破坏 |
| 7 | 动词分布与过滤 | ✅ filterByTask 各动词计数 = 转换16/计算8/生成5/识别2/解析2/爆破2/提取1/演示3(49 行内)/分析3/编解码1/格式化5/校验1,并集 49 互不重叠;beast 另计演示 |
| 8 | 域级任务维度(misc 收口) | ✅ misc 99 工具 → 12 个任务动词(词表序:编解码/转换/解析/生成/爆破/提取/分析/识别/演示/校验/格式化/计算);每个动词过滤均不含 file_checksum;12 动词并集 = 98(99 − file_checksum) |
| 9 | /c/misc 与子路由 | ✅ /c/misc 任务行 12 chips + 99 卡 + 97 占位/2 已实现徽章不变;/c/misc.other 93 卡、任务行 11 动词(无校验);/c/misc.demo 4 卡、任务行仅演示;/c/misc.report 2 卡、任务行 = 校验/生成(批 10 快照断言同步演进) |
| 10 | 工具详情页 | ✅ flag-checker(校验)/rar-crack(爆破)/ms-script-decode(编解码)/beast(演示)任务 chip 均渲染,关键词 chips 不变 |
| 11 | 附加验收 2:565 行清单与运行时 UI 全量核对 | ✅ 全 565 行:名称/分类/状态/tags/行号全量一致;清单 ':?' 约定行 = 32(20 web.ssti.* + 12 crypto.encoding.base.* 程序化生成,已知约定,不比行号);63 分类页逐一渲染且 h1=分类名;565 工具页逐一渲染且含名称 h1 + 状态徽章,零「未知工具」 |
| 12 | 旧批次回归 | ✅ smoke_b2 55/55、b3 39/39、b4 27/27、b5 40/40、b6 19/19、b7 64/64、b8 51/51、b9 87/87、b10 61/61 全部通过 |
| 13 | 清单一致性 | ✅ parity 0 缺行 / 0 状态不一致 / 0 清单独有行 / 0 分类不一致;565 前端行、63 分类节点不变;stats 565/62/503 |

## 3. 实现说明

1. **task 标签分配与批 10 一致性**:参照批 10 已标工具的动词选择 —— 集合运算类→计算(对齐 cartesian-product)、图表类→分析(对齐 heatmap-chart/hex-density-chart)、tidy/美化类→格式化(对齐 css-tidy/json-tidy/comment)、OTP/ID 类→生成(对齐 hotp/get-time)、坐标/时间格式类→转换(对齐 coordinate/filetime)、label/subsection 归格式化(对齐 comment)、register/return/sleep 归演示(no-op/展示性操作)、regex 归提取(对齐 extract-dates)、ms-script-decode 归编解码(词汇表中唯一的编解码动词)、rar-crack/zip-brute 归爆破、flag-checker 归校验。全部在 §2 受控词表内,待人工复核(同批 10 口径)。
2. **beast 的「演示」标签**:§12 核心动作原文「demo 组(4 个)与 beast 一并标注『演示』task 标签」—— demo 组批 10 已提前满足(010 报告 §3-4),本批补 beast(crypto.encoding.beast,codec 域已实现工具)。
3. **php-unserialize 全链路**:工具(web.deserialize.php-unserialize,批 1 已实现)不在本批表格内,按 §12 附加验收做回归——adapter→local.php-unserialize→parsePhpClass/buildPhpSerialized 全链 + 工作区渲染,零代码改动。
4. **清单 ':?' 约定**:32 行(SSTI 20 + base 编解码 12)由构建器程序化生成,无字面 id 行,清单以 `tools.ts:?` 诚实标注;全量核对时只验存在与文件指向,行号对比豁免。这是既有格式,非漂移。
5. **misc 任务维度终态**:98/99 覆盖,file_checksum(批 1 工具,历批范围外)仅在「全部」可见,选中任何任务动词时被过滤——已由断言固化并在报告如实记录;如需补齐,一行 tags + 清单同步即可。
6. **历史冒烟断言演进**:smoke_b10 的 4 处批 10 时点快照(remove-whitespace/writeup 未回填、misc.other 无演示、report 无任务行)更新为批 11 终态(先例:segment 计数随批演进)。

## 4. 剩余风险

1. **48 个占位无真实实现**:与 inventory 一致(48 placeholder + writeup 已实现),徽章+「模拟结果」chip 已标注,不伪装可用;实现留待阶段 4/后续工作。
2. **task 分配待人工复核**:50 处动词为阶段 0 关键词初填 + 人工初核(参照批 10 口径),边界词(提取/分析、格式化/转换、演示类)尚未经用户逐词确认;词表集中在 `lib/task.ts` + tools.ts tags,复核成本低。
3. **file_checksum 无 task 标签**:misc 域唯一未打标工具,任务筛选下不可见;历批范围外,如需补齐可单独处理。
4. **其他域 task 标签未覆盖**:task 体系目前仅 misc 域 + beast 落地(ia.md §2 的长期目标),其余域回填留待阶段 4 或后续批次。
5. **SSR 全量渲染 ≠ 浏览器交互验证**:565 工具页渲染断言验证结构与身份,但 form 提交/收藏/chips 点击等交互层仍需桌面端 `wails dev` 人工复核(同历批口径)。
