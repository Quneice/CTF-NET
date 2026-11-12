# 需人工确认的问题清单(阶段 1 产出,共 18 条,按影响排序)

> 每条含影响评估与证据出处。确认结果将直接写入阶段 2 的 IA 方案与设计系统文档。

## Q1.【范围】564 个前端工具是否全量纳入重构?
- 现状:59 已实现 + 505 占位(89.5%)。
- 选项:A 全量纳入(占位工具给统一骨架+徽章+占位态,功能按 P2 排期);B 只做已实现 59 个 + 接线修复;C 按分类分批选定。
- 影响:决定阶段 3 批次数量与工作量(全量约 8~12 批)。
- 证据:`docs/audit-report.md` §1。

## Q2.【清单】627 条 tool-inventory.json 是否确认为单一事实源?
- 选项:确认 / 抽查某几类后确认。
- 影响:阶段 2 IA 方案、阶段 3 批次划分都以此为准;后续新增工具必须先更新清单。
- 证据:`docs/tool-inventory.json`(每条含文件行号级 evidence)。

## Q3.【接线断裂 F1】strings / http_headers 改接 Go 真实后端,并为 file_checksum 补前端页?
- 现状:3 处断裂(前端误接 /api/mock/,Go 侧真实实现闲置)。
- 选项:A 第 0 批修复(推荐);B 只修不补页;C 暂不处理只记风险。
- 影响:3 个工具从占位变为真实可用,涉及后端接线与 1 个新工具页。
- 证据:`docs/audit-report.md` §5-F1。

## Q4.【功能重复 F2】Go 侧 5 个与前端本地实现重复的工具如何处置?
- 对象:crypto.base64 / crypto.rot13 / crypto.beast / misc.hash / misc.wechat_dat。
- 选项:A 前端本地优先,Go 侧标记退役(阶段 4 确认后删除);B Go 优先,前端改接 realBackend;C 维持现状只记风险。
- 影响:后端代码生死 + 5 个前端工具的调用路径。
- 证据:`docs/audit-report.md` §5-F2。

## Q5.【层级 F4】crypto.modern.rsa 下 5 个 4 层节点(22 工具)如何压到 3 层?
- 选项:A 方案 A——22 工具直接挂 crypto.modern.rsa,tags 保留技术类别(推荐,改动最小);B 方案 B——新建 crypto.rsa 二级节点;C 豁免规则 5 保持 4 层。
- 影响:IA 树结构 + 5 个分类路由的 301 重定向。
- 证据:`docs/category-mapping.md` 表 C。

## Q6.【占位透明化 F3】505 个占位工具是否显示"占位"状态徽章?
- 现状:占位与真实工具外观完全一致,用户无法区分;占位工具点击后返回 mock 结果。
- 选项:A 卡片+详情页显示占位徽章,点入显示标准占位态(推荐);B 直接隐藏占位工具(需确认是否违背"不删除"原则);C 保持现状。
- 影响:设计系统必须新增"占位状态"规范(规则 6),所有 ToolCard/ToolPage 改动。
- 证据:`docs/audit-report.md` §5-F3。

## Q7.【占位功能排期】505 个占位工具的功能实现如何排期?
- 选项:A 全部按 P2 排入批次(约 500 个功能实现,工作量大);B 只实现高频类别(web/crypto/misc 共 431 个,占 76%);C 按需实现(用户点哪个补哪个)。
- 影响:阶段 3 批次内容与工期。
- 证据:分类分布 `docs/audit-report.md` §5-F9。

## Q8.【命名规范 F5】40 个 misc.encoding.* 工具挂载在 crypto.encoding 下?
- 选项:A 方案 1——保持挂载、id 前缀规范化(推荐,纯命名+旧路由重定向);B 迁回 misc 新建 misc.encoding 节点;C 完全不动。
- 影响:40 个工具的 id 与路由。
- 证据:`docs/category-mapping.md` 表 B1。

## Q9.【双隐写域】3 个 misc.stego.* 工具与 stego 大类合并?
- 选项:A 按建议迁入 stego.text / stego.image(wechat-dat 目标分类需定夺);B 保留双域;C 只合并 whitespace-codec 与 teyvat。
- 影响:3 个工具迁移 + misc.stego 节点是否保留。
- 证据:`docs/category-mapping.md` 表 B2。

## Q10.【legacy 处置】32 个 legacy Python 工具如何处理?
- 选项:A 保留作移植参照,不纳入工具清单主表;B 13 个已实现者纳入"已替代"档案,19 个占位者视作废弃;C 全部视作废弃,阶段 4 打包归档。
- 影响:库存口径(当前 627 条含 legacy 32 条)与阶段 4 清理范围。
- 证据:`docs/audit-report.md` §10。

## Q11.【Go ssti 占位】19 个 Go 占位 SSTI 引擎如何处置?
- 现状:占位执行器返回"[占位符] … 功能开发中",前端经 realBackend 调用(用户可见)。
- 选项:A 逐步实现(优先级:twig/smarty/freemarker 等实战高频);B 前端先降级显示占位徽章;C 维持现状。
- 影响:19 个工具的可用性;实现工作量集中在 Go 侧。
- 证据:`internal/tools/web/ssti_base.go:156`。

## Q12.【id 不一致 F6】前后端 id 命名统一方向?
- 现状:web.deserialize.jndi ↔ web.jndi;crypto.modern.aes.cbc-bitflip ↔ crypto.aes_cbc_bitflip。
- 选项:A 前端迁就后端(前端 id 改短,旧路由重定向);B 后端迁就前端;C 建立显式 id 映射表,不改任何一边。
- 影响:涉及真实工具的路由与后端注册表。
- 证据:`docs/audit-report.md` §5-F6。

## Q13.【首页精选 F7】精选工具改为真实使用热度?
- 现状:tools.slice(0,12) 硬编码,含 mock 工具。
- 选项:A 基于 useHistoryStore 使用次数统计(推荐);B 手工维护精选列表;C 保持现状。
- 影响:首页内容质量,需要 history 增加计数聚合。
- 证据:`frontend/src/pages/HomePage.tsx:33`。

## Q14.【版本控制】阶段 3 开始前是否允许 git init + 基线提交?
- 现状:目录不是 git 仓库;有 backups/*.zip 共 5 个历史备份(最近 2026-11-10 12:36)。
- 选项:A 允许(推荐,安全网);B 继续用 zip 备份。
- 影响:批次回滚能力与 diff 审查。
- 证据:目录列表 + `backups/`。

## Q15.【视觉基准】设计系统沿用现有 slate+indigo 配色还是换品牌色?
- 现状:全站 Tailwind 原子类散落使用 slate/indigo,无令牌文件;tailwind.config.js extend 为空。
- 选项:A 沿用 slate+indigo,阶段 2 收拢成令牌(推荐,改动最小);B 指定新品牌色。
- 影响:设计系统颜色令牌与后续所有批次。
- 证据:`frontend/tailwind.config.js` + `frontend/src/index.css`。

## Q16.【beast 归类】crypto.encoding.beast 迁移到 crypto.modern 还是 misc.demo?
- 影响:1 个工具的目录归属。
- 证据:`docs/category-mapping.md` 表 B3。

## Q17.【移动端标准】视觉验收是否包含移动浏览器(≤768px)不横向溢出?
- 现状:Wails 桌面应用,但 README 注明可纯浏览器 npm run dev;现无移动端验收记录。
- 选项:A 纳入验收(规则 10 已含"移动端不横向溢出",建议执行);B 仅桌面窗口(≥1024px)验收。
- 影响:设计系统断点定义与每批截图验收清单。
- 证据:README.md §构建与运行。

## Q18.【批次组织】阶段 3 批次按"一级分类 + P1→P2 优先级"组织、每批 30~80 个,是否接受?
- 选项:A 接受(推荐);B 指定自定义批次顺序(请说明)。
- 影响:阶段 3 执行节奏。
- 证据:执行计划阶段 3。

## 确认方式

逐条回复 Q1~Q18(如"Q1=A, Q3=A, Q4=A, Q5=A, Q6=A, Q7=B, Q8=A, Q9=C, Q10=B, Q11=B, Q12=C, Q13=A, Q14=A, Q15=A, Q16=方案 A, Q17=A, Q18=A"),或直接给出"全部按推荐"即可。
