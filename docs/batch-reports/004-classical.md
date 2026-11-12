# 批 4 报告 · Crypto.Classical 全量(74 个:1 已实现 / 73 占位)

- 生成时间:2026-11-10
- 范围:严格按用户指令「只实现第 4 批」;migration-plan §5 全部核心动作落地。
- 批次性质:**机制继承 + 唯一新功能批** —— 骨架/徽章/占位态机制已由批 1 落地,74 个工具的状态与占位标注零改动自动生效;本批实际新增功能 = 分类页段分组筛选 chips(ia.md §4 导航降级),改动 3 个源文件 + 全面验收。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/types/index.ts` | `CategoryNode` 新增可选字段 `segment?: boolean`(导航降级节点标记,注释引用 ia.md §4) |
| `frontend/src/mock/categories.ts` | 4 个 3 段节点(caesar/vigenere/affine/keyed)标注 `segment: true`(路由保留、侧边栏节点保留,仅父分类页呈现方式变化) |
| `frontend/src/pages/CategoryPage.tsx` | 新增段分组筛选 chips(design-system §3.3 左部):「全部 + 各段」chips,选中态 `bg-brand-subtle border-brand text-brand`,`aria-pressed`,`duration-base`;移动端 `overflow-x-auto` 横向滚动;段节点不再渲染子分类入口卡片;选中 chip 时工具网格经 `getToolsUnderCategory(chip)` 过滤;切换分类自动重置筛选 |
| `docs/batch-reports/004-classical.md` | 本报告 |

未改动:tools.ts(74 个工具定义已完整,字段零缺失)、路由(本批重定向为「无」;段节点经动态段 `c/:categoryId` 直接可达)、tool-inventory.json(0 不一致,无需补丁;segment 为展示标记,不入清单)、migration-plan.md(计划文档)。

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` / `npm run build` | ✅ 0 错误 / 构建通过(1.61s) |
| 2 | 冒烟全量(smoke_b4.cjs) | ✅ **27 条断言,失败 0** |
| 3 | 计划表 74 行 vs 代码集合相等 | ✅ §5 表格 74 行全部存在;无计划外新增(rot13 为批 1 工具,正确排除;目录共 75) |
| 4 | 状态一致性(565 行,parity_check_b1.cjs) | ✅ 0 不一致;74 个中已实现恰为 rail-fence;其余 73 个占位且全部为纯前端占位(无后端占位);批 1 的 rot13 状态保持 |
| 5 | rail-fence 走全链路 | ✅ zigzag / cyclic 两变体 encode→decode 往返一致(`HELLOCTF42` rails 3);表单 schema 含 mode/variant/rails |
| 6 | 风险项:生成一致性抽样 20 行(名称/分类/徽章三列 vs inventory) | ✅ 抽样 20 行(等距抽样)三列与 inventory 逐行一致,bad=0 |
| 7 | 附加验收 a:/c/crypto.classical 显示 4 个分组 chips | ✅ react-dom/server 真实渲染断言:chips「全部/凯撒/维吉尼亚/仿射/带密钥古典密码」全部渲染;`aria-pressed` 默认选中「全部」;段节点不再渲染入口卡片;默认网格恰 48 个直挂工具 |
| 8 | 附加验收 a 续:点击过滤正确 | ✅ 过滤集合逐段断言:caesar→1、vigenere→1、affine→1、keyed→24,与 `getToolsUnderCategory` 逐 id 一致;CategoryPage 源码以 `getToolsUnderCategory(chip)` 实现过滤 |
| 9 | 附加验收 b:直达 /c/crypto.classical.caesar 不 404 | ✅ 真实渲染断言:直达该 URL 显示 caesar-decrypt 卡片(恰 1 个,不串入直挂工具);keyed 段直达显示 24 个 |
| 10 | 旧路由保留 | ✅ 本批零路由改动、零重定向;4 个段节点路由经动态段可达;73 个占位工具 `/t/:toolId` 全部唯一 |

## 3. 实现说明

1. **chips 机制以数据驱动而非硬编码分类 id**:段降级标记放在 categories.ts 的节点数据上(`segment: true`),CategoryPage 对「children 中 segment 节点」渲染 chips、对其余渲染入口卡片 —— 与「所有工具信息从配置读取」一致;后续批次若再降级其他节点,只改数据不动组件。
2. **chips 交互遵循 design-system §3.3**:选中态 `bg-brand-subtle border-brand text-brand`,过渡 `duration-base`;移动端 `overflow-x-auto`(全局细滚动条样式已覆盖视觉)。§3.3 的右侧筛选维度下拉与吸顶 FilterBar 属搜索页/阶段 5 范围,本批不越界。
3. **段页直达行为不变**:`/c/crypto.classical.{caesar,vigenere,affine,keyed}` 仍渲染 CategoryPage(动态段匹配),只显示该段工具;chips 仅在父分类页出现。

## 4. 剩余风险

1. **chip 点击为声明式 JSX 接线,未做 headless 点击模拟**:渲染断言覆盖 chips 存在、默认选中、过滤集合(与 CategoryPage 实际调用的 `getToolsUnderCategory` 同源逐 id 断言);`wails dev` 手动点击 4 个 chips 复核一次建议保留(报告中的验收 7/8 为脚本级证据)。
2. **73 个占位为单一批次上限**:均为 mock 路径(批 3 已加「模拟结果」chip 标注),真实实现留待后续批次(与 inventory 状态一致)。
3. **segment 字段不入 inventory**:tool-inventory.json 只记录工具行(状态/分类),展示类标记不落清单;若阶段 4 清单扩展 category 节点 schema,再行同步(已在类型注释中标明来源)。
4. **SubcategoryCard 组件未删除**:其他分类(web/codec 等)仍使用;crypto.classical 页经数据标记自动切换为 chips,无死代码。
