# 批 13 报告 · CTF 工具清单缺口补充(88 个,0 已实现 / 88 占位)

- 生成时间:2026-11-15
- 范围:按计划 `docs/superpowers/plans/2026-11-15-ctf-gap-supplement.md`(第二轮流查,对照本会话 ctf-* 技能分类法;第一轮 144 占位 fcc2cff 未覆盖的比赛高频项)。
- 批次性质:**占位补充批** —— 只建目录占位、不实现功能;新增 4 个分类节点 + 88 个 placeholder 骨架。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `docs/superpowers/plans/2026-11-15-ctf-gap-supplement.md` | **新增**:逐域缺口分析表 + 88 行补充清单 + 假设 |
| `frontend/src/mock/tools.ts` | +`ctfGapShorthandRows`(88 行,单行 + preset 展开),插在 dataShorthandRows 之后、`shorthandTools` 展开数组末尾追加 → **既有行号零漂移** |
| `frontend/src/mock/categories.ts` | +4 个分类节点:`web.xxe`(web 子类,FileWarning)、`pwn.kernel`(pwn 子类 segment,Cpu)、`crypto.modern.dh`(crypto.modern 子类,Handshake)、`misc.jail`(misc 子类 segment,DoorClosed) |
| `frontend/src/lib/icons.tsx` | iconMap +Cpu/+DoorClosed/+FileWarning/+Handshake(import 与 map 同步 replace_all;Folder 兜底不变) |
| `docs/batch-reports/013-ctf-gap-supplement.md` | 本报告 |

未改动:router(新工具零重定向)、adapters、UI 组件、Go 后端、STATUS_OVERRIDES。

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` | ✅ 0 错误 |
| 2 | `npm run build` | ✅ 构建通过(4.80s;仅既有 chunk>500kB 警告) |
| 3 | 新增行数 vs 计划 | ✅ 恰 88 行,与计划表逐域一致(pwn 13/reverse 9/crypto 19/web 13/forensics 10/traffic 6/misc 7/osint 4/blockchain 4/mobile 3) |
| 4 | id 唯一性 | ✅ 与既有 763 个 id 逐条比对零冲突(全文件重复 id:无) |
| 5 | 状态推导 | ✅ 88 行全部 placeholder:无 workspace、无 realBackend、无 STATUS_OVERRIDES 重叠、local preset 的 handlerId 未注册(mock http / mock task / 未注册 local → 占位) |
| 6 | 图标注册 | ✅ 4 个 lucide 图标 import 与 iconMap 同步;tsc 校验导出存在性(不存在会编译失败) |
| 7 | preset 分布 | ✅ text 49 / file 34 / local 5(与批 12 同构:探测扫描类 text、文件解析类 file、纯计算类 local) |
| 8 | task 动词 | ✅ 全部在受控动词表内(攻击 19/利用 12/还原 12/分析 11/提取 10/探测 7/计算 5/识别 4/编解码 2/爆破 2/生成 2/攻击性动词占主导,符合 CTF 解题定位) |
| 9 | 零行号漂移 | ✅ 新块位于全部既有简写块之后,既有行号不变 |

## 3. 实现说明

1. **id 命名**:与既有同域条目对齐(如 `houseofeinherjar` 对齐 `houseofforce` 全小写无连字符惯例;`pwn.stack.checksec` 等新词用 kebab)。个别保持既有风格差异(如既有 `ret2csu` 无连字符、新增 `ret2dlresolve` 也无连字符)。
2. **分类落位**:XXE 按 ctf-web 篇目权重独立成 web 子类;内核 pwn 按 ctf-pwn 篇目(kernel*.md 共 4 篇)独立成 pwn 子类;DH 按 ctf-crypto 篇目(dh-attacks.md)独立成 crypto.modern 子类;pyjail/bashjail 按 ctf-misc 篇目(pyjails/bashjails.md)独立成 misc 子类。
3. **「4 个新分类」为一次性扩展**,后续批不再新增分类,只在该 4 节点下补工具。

## 4. 已知滞后(如实记录,非本批引入)

1. **docs/tool-inventory.json 滞后**:stats 仍为 612/62/550(批 12 终态),未含 144 占位批与 SSTI 接线;本批 88 行也未写入。tools.ts 实际工具数已至 ~844。计划文档假设 1 明确:本批不做 inventory/ia.md 全量重同步(与「只加占位」目标偏离、量大易错);建议后续单独批次重建 inventory(按批 12 教训:按 oldPath `/t/` 前缀过滤后全量重建)。
2. **ia.md 分类表/导航树**:同上,未同步 4 个新节点(侧边栏由 categories.ts 驱动,不影响运行)。

## 5. 剩余风险

1. **88 个占位无真实实现**:与批 12/144 批同构,徽章「占位」不伪装可用;实现优先级建议按比赛频率:checksec/libc-db(每道 pwn 必用)> Flask Cookie/XXE(web 高频)> MT19937/Coppersmith(crypto 高频)> pyjail(payjail 题必查)。
2. **task 动词分配待人工复核**:边界词(攻击/利用、提取/分析)按批 10-12 先例为关键词初填。
3. **交互验证**:构建与类型检查通过,但 4 个新分类节点与 88 工具页的浏览器渲染需 `wails dev` 人工复核(与批 12 验收 5 同口径)。
