# 批 5 报告 · Crypto.Modern + Auto-decode(68 个:3 已实现 / 65 占位)

- 生成时间:2026-11-10
- 范围:严格按用户指令「只实现第 5 批」;migration-plan §6 全部核心动作落地。
- 批次性质:**结构迁移批** —— 22 个 RSA 工具从 5 个 4 段节点扁平化挂到 `crypto.modern.rsa`(表 C 方案 A,tags 保留技术类别),5 个 4 段分类节点删除,5 条旧分类 URL 重定向;工具 id 一律不变(`/t/:toolId` 零改动),收藏/最近/历史不受影响。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/mock/tools.ts` | 22 个工具 categoryId 平移(脚本计数核对:algebraic 5 / fermat-family 4 / shared-structure 5 / number-theoretic 4 / coppersmith 4 = 22,无残留);工具 id 保留 4 段前缀 |
| `frontend/src/mock/categories.ts` | 删除 5 个 4 段节点(algebraic / fermat-family / shared-structure / number-theoretic / coppersmith),`crypto.modern.rsa` 无子节点(68 → 63 分类节点);节点处留批 5 注释 |
| `frontend/src/router/index.tsx` | 新增 5 条重定向:`/c/crypto.modern.rsa.{5 段}` → `/c/crypto.modern.rsa`(`<Navigate replace>`,静态段优先于动态段);旧路由零删除 |
| `frontend/src/components/tool/ToolCard.tsx` | 卡片分类前缀行追加**完整工具 id**(font-mono text-2xs opacity-70)—— 满足风险项「rsa-decrypt 与 rsa.decrypt 重名并存(卡片显示完整 id 前缀,阶段 4 处置)」 |
| `docs/tool-inventory.json` | 22 行 currentCategory/suggestedCategory/menuItem 同步(全部为 frontend 层级,无 Go/legacy 行);riskNotes 追加「批5 挂载→rsa」;evidence 行号重刷;stats 不变(565/62/503) |
| `docs/batch-reports/005-modern-rsa.md` | 本报告 |

未改动:批 0-4 范围内文件;工具 id 与 `/t/` 路由零改动;migration-plan.md 未动(计划文档)。

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` / `npm run build` | ✅ 0 错误 / 构建通过(1.66s) |
| 2 | 冒烟全量(smoke_b5.cjs) | ✅ **40 条断言,失败 0** |
| 3 | 计划表 68 行 vs 代码集合相等 | ✅ §6 表格 68 行全部存在;无计划外新增(cbc-bitflip 为批 1 工具,已排除) |
| 4 | 22 个工具扁平化核对 | ✅ 4 段 id 前缀恰 22 个,categoryId 全部 = crypto.modern.rsa;tags 全部保留非空且技术类别齐全(30 种标签:多项式关联/Coppersmith/数论/Fermat/LCG 等) |
| 5 | 分类树:5 个 4 段节点删除 | ✅ 逐节点断言已删除;rsa 无子节点;树 68 → 63;无工具指向缺失节点 |
| 6 | 状态一致性(565 行,parity_check_b1.cjs) | ✅ 0 不一致;68 个中已实现恰为 auto-decode + close + nth-root;其余 65 个占位;批 1 的 cbc-bitflip 状态保持 |
| 7 | 3 个已实现处理器冒烟 | ✅ rsa-fermat 分解 6557→79×83(含 p*q==n 验证);rsa-nth-root n=14763,k=2 → r=11;auto-decode 命中 base64 → HelloCTF |
| 8 | 附加验收 1:5 条旧分类 URL 逐一验证 | ✅ (a) 源码:路由表恰 5 条 from/to 逐一正确;(b) 匹配语义:5 条旧 URL 经 createMemoryRouter 逐一命中重定向静态路由(而非 `c/:categoryId` 动态段);(c) 目标页 /c/crypto.modern.rsa 渲染 41 个工具(19 直挂+22 迁入),迁入工具与 2 个已实现迁入工具均可见 |
| 9 | 附加验收 2:/c/crypto.modern.rsa 页 22 个工具可见且 tags 技术类别齐全 | ✅ 41 个工具真实渲染;22 个迁入工具 tags 非空、30 种技术标签分布断言 |
| 10 | 附加验收 3:auto-decode 可访问 | ✅ 卡片 href 指向 /t/crypto.auto-decode +「已实现」徽章 + crypto.auto-decode 分类节点存在 |
| 11 | 重名 RSA 对 | ✅ 两卡片名称均为「RSA 解密」,但完整 id 前缀可区分(rsa-decrypt vs rsa.decrypt),各自路由可访问 |
| 12 | 旧路由保留 | ✅ 动态段与全部旧重定向保留;工具路由零改动 |

## 3. 实现说明

1. **重定向语义**:SPA 无服务端,「301 语义」以 `<Navigate replace>` 落地(URL 被替换为新路径,历史栈不留旧记录),与批 1/2 模式一致;静态段 `c/crypto.modern.rsa.algebraic` 在 react-router v6 中优先于动态段 `c/:categoryId`,已用 createMemoryRouter 匹配结果逐一证实。
2. **卡片 id 前缀**:ToolCard 分类前缀行追加完整 id(mono 2xs),同批 3 的「分类前缀」机制叠加;该改动同时让批 3 的两个「文件上传绕过」可按 id 区分(顺带收益,不改批 3 结论)。
3. **清单原子性**:tools.ts 平移、分类树删除、路由重定向、inventory 补丁同批一次完成并全部验证通过;patch 脚本幂等(按 id 前缀 + oldPath 层级锁定行)。

## 4. 剩余风险

1. **重定向未做桌面端点击验证**:匹配语义已脚本级证实(静态段命中 + 目标页渲染);`wails dev` 手动点击 5 条旧 URL 复核一次建议保留。
2. **重名 RSA 对改名留待阶段 4**:本批按计划仅「卡片显示完整 id 前缀」;两个「RSA 解密」描述不同(参数分解与攻击模式 vs 私钥解密),改名与描述校准阶段 4 处置。
3. **22 个迁入工具 id 仍带 4 段前缀**:与 misc.encoding.* 同类的「id≠分类」债务,按计划本批不改 id;阶段 4 规范化时走「清单补 oldPath → 重定向 → 观察期」流程。
4. **65 个占位**:全走 mock 路径(批 3 已加「模拟结果」chip 标注),真实实现留待后续批次(与 inventory 状态一致)。
5. **5 个 4 段节点的 descriptions 随节点删除而消失**:技术类别信息保留在 22 个工具的 tags 中(已断言齐全),无信息丢失。
