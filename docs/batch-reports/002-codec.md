# 批 2 报告 · Codec 全量平移(75 个工具:42 已实现 / 33 占位)

- 生成时间:2026-11-10
- 范围:严格按用户指令「只实现第 2 批」;migration-plan §3 全部核心动作落地,另修复 2 个由本批冒烟验收暴露的真实缺陷。
- 批次性质:**纯分类迁移批** —— 工具 id 一律不变(`/t/:toolId` 路由零改动),收藏/最近/历史不受影响。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/mock/tools.ts` | 75 个工具 categoryId 平移:11 个 Base(`crypto.encoding.base.*`→`codec.base`)、64 个平铺(`crypto.encoding`→`codec`);baseTools 三元改回字面量;注释更新 |
| `frontend/src/mock/categories.ts` | 删除 `crypto.encoding` 节点(含 `crypto.encoding.base` 子节点);crypto 一级描述改为「密码学:古典密码、现代密码、哈希、自动解码」 |
| `frontend/src/router/index.tsx` | 新增重定向:`/c/crypto.encoding`→`/c/codec`、`/c/crypto.encoding.base`→`/c/codec.base`;旧路由零删除 |
| `frontend/src/adapters/localHandlers.ts` | **缺陷修复 ×2**(冒烟验收暴露,见 §3):base32 自定义填充死循环;摩斯解码丢失空格 |
| `docs/tool-inventory.json` | 75 行 currentCategory/suggestedCategory/menuItem 同步;533 条证据行号重刷;beast 标记「批2 归类存疑」;stats 不变 |
| `docs/batch-reports/002-codec.md` | 本报告 |

未改动:批 0/批 1/批 3+ 范围内文件;工具 id 与 `/t/` 路由零改动;migration-plan.md 未动(计划文档,执行记录在本报告)。

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` / `npm run build` | ✅ 0 错误 / 构建通过(1.58s) |
| 2 | 42 个已实现工具全量冒烟(已知输入) | ✅ **55 条断言,失败 0**(脚本 `smoke_b2.cjs`:35 个 codec 往返 + 11 条经典编码精确向量 + 7 条解释器/转换精确向量 + 汉字数字往返) |
| 3 | 状态一致性(565 行) | ✅ 0 不一致 |
| 4 | 分类一致性(清单 vs 代码) | ✅ 0 不一致;0 个工具指向缺失分类节点 |
| 5 | 平移数量核对 | ✅ 恰好 75 行移动(64 平铺 + 11 Base),无多无少;base64(批 1 已迁)不重复计入 |
| 6 | `/c/crypto.encoding`、`/c/crypto.encoding.base` 重定向 | ✅ 路由表含两条 `<Navigate replace>`;react-router v6 静态段优先于动态段,点击验证留手动 |
| 7 | 搜索「base64」命中 4 目标且分类正确 | ✅ 7 条命中,必需 4 条全中:base64→codec.base、base64url→codec.base、base64-offsets→codec、base64-stego→stego.image |
| 8 | 占位工具不伪装可用 | ✅ codec 下 33 个占位工具自动显示「占位」徽章 + 占位横幅(批 1 已落地机制,本批零改动即生效) |
| 9 | 旧路由保留 | ✅ 工具路由不变;`/c/misc.stego` 重定向(批 1)保留;分类树旧节点仅按计划删除 crypto.encoding |
| 10 | 无控制台错误 / 动效 / 移动端 | ✅ 本批无布局与样式改动 |

## 3. 冒烟验收暴露并修复的 2 个真实缺陷

1. **base32 自定义填充死循环**(localHandlers.ts `makeBase32`):用户在 Base 家族工具的「填充字符」输入框清空后,`padChar` 为空字符串,`while (out.length % 8 !== 0) out += padChar` 永不终止 → **页面冻结**。修复:空填充时直接返回无填充输出。已验证:`encode('HelloCTF 42!')` → `JBSWY3DPINKEMIBUGIQQ` → 解码往返一致。
2. **摩斯电码解码丢空格**(`decodeMorse`):encode 把空格映射为 `/`,decode 却把 `/` 当未知码丢弃 → 往返丢失所有空格(冒烟实测 `'HelloCTF 42!'` → `'HELLOCTF42!'`)。修复:decode 先把 `/` 还原为词边界(双空格)。已验证往返 `'HELLOCTF 42!'` 恒等。

两者均为批 2 范围内工具(42 个已实现之一)的真实缺陷,冒烟脚本按「UI 实际参数语义」驱动(CodecWorkspace 会把清空的填充字段传成空串)才得以暴露。

## 4. 剩余风险

1. **beast 归类存疑**:`crypto.encoding.beast` 按计划暂挂 codec 顶层,清单 riskNotes 与 ia.md §9 已标记,待确认是否迁 misc.demo。
2. **misc.encoding.* 前缀债务(Q8)**:40 个工具 id 前缀与挂载分类不一致,按计划本批不改 id;后续 id 规范化时走「清单补 oldPath → 重定向 → 观察期」流程。
3. **重定向未做运行时点击验证**:路由为声明式配置且构建通过;`wails dev` 手动点击三条重定向(含批 1 的 misc.stego)建议用户在桌面端复核一次。
4. **33 个占位工具**:codec 下占位工具显示徽章 + 横幅 + 模拟结果,不伪装可用;真正实现留待后续批次(与 inventory 状态一致)。
5. **两处缺陷修复未回灌 Go/legacy 层**:morse 与 base32 的 Go 实现(crypto.base64 / legacy)无此缺陷(无自定义填充参数面);无需回灌,仅记录。
6. **迁移计划文档未回写**:migration-plan.md 的批 2 章节保持计划原样(历史文档),执行结果以本报告为准。
