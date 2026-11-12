# 批 1 报告 · Lighthouse(12 工具接线 + 状态徽章 + 设计令牌落地)

- 生成时间:2026-11-10
- 范围:严格按用户指令「只实现第 1 批」;未触碰批 0(页面骨架/搜索页)与批 2+ 内容。
- 批次工具(12 个,全部 implemented):
  `web.deserialize.jndi`、`web.ssti.jinja2`、`web.basic.http-headers`(F1)、
  `crypto.modern.aes.cbc-bitflip`、`misc.stego.whitespace-codec`、`misc.stego.teyvat`、
  `misc.stego.wechat-dat`、`reverse.static.strings`(F1)、`crypto.encoding.base.base64`、
  `crypto.classical.rot13`、`crypto.hash.hash`、`misc.file_checksum`(新增页)。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/index.css` | 追加 `--ct-*` 设计令牌(明暗两套,design-system.md §2.8) |
| `frontend/tailwind.config.js` | extend 语义别名:brand/surface/line/ink/status、2xs、card/pop/modal 阴影、fast/base/slow、zIndex 层级 |
| `frontend/src/lib/toolStatus.ts` | **新增**。状态推导单一入口:覆盖表(19 个 Go 占位 SSTI)→ task+realBackend → local+已注册 → placeholder;`isBackendPlaceholder` |
| `frontend/src/components/tool/StatusBadge.tsx` | **新增**。四态徽章(已实现/占位/故障/未知)+「后端占位」变体,aria-label,状态禁止手填 |
| `frontend/src/components/tool/PlaceholderBanner.tsx` | **新增**。占位工具页常驻提示(design-system §3.7):前端 mock 占位=Info 图标,后端占位=ServerCog 图标,role="note" |
| `frontend/src/components/tool/ToolCard.tsx` | 标题行加 StatusBadge;配色迁移到令牌(border-line / bg-surface-raised / shadow-card / duration-fast 等) |
| `frontend/src/pages/ToolPage.tsx` | h1 旁加 StatusBadge;tags 后加 PlaceholderBanner;tags 配色迁移令牌 |
| `frontend/src/mock/tools.ts` | F1:http-headers、strings 改 task+realBackend(后端 id 取自 submitUrl);base64 → codec.base(三元);whitespace-codec/teyvat → stego.text、wechat-dat → stego.image;追加 misc.file_checksum 工具页 |
| `frontend/src/mock/categories.ts` | 删除 misc.stego 节点;新增 codec 一级 + codec.base 二级(位于 crypto 与 misc 之间,ia.md §1.1 顺序) |
| `frontend/src/router/index.tsx` | 新增 `/c/misc.stego` → `/c/stego` Navigate replace;旧路由全部保留 |
| `frontend/src/lib/backend.ts` | App 门面补 UploadFile;新增 `uploadFile()` 助手 + FileInfo 类型;更新旧 REST 契约映射注释 |
| `frontend/src/adapters/index.ts` | **新增 `uploadFileParams`**:realTaskAdapter 提交前把 File[] 值上传为 file_id(单文件→字符串,多文件→数组),上传失败转 InvokeEvent error |
| `docs/tool-inventory.json` | 补丁脚本同步:565 行证据行号重刷、http-headers/strings → implemented、4 处分类迁移、新增 misc.file_checksum 前端行、3 条 Go 行注释更新、stats 重算 |
| `docs/batch-reports/001-lighthouse.md` | 本报告 |

未改动:批 0/批 2+ 范围内的任何文件;旧工具路由、旧分类节点(除 misc.stego 按计划移除)全部保留。

## 2. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` | ✅ 0 错误 |
| 2 | `npm run build`(Vite 生产构建) | ✅ 1651 模块,1.58s,无告警 |
| 3 | 路由可达性:旧路由保留 + 重定向 | ✅ 旧路由无一删除;`/c/misc.stego` → `/c/stego`(replace) |
| 4 | 清单=代码一致性(状态) | ✅ 565 前端行,推导状态 vs 清单 **0 不一致**(脚本见 §3) |
| 5 | 清单=代码一致性(分类) | ✅ currentCategory vs 代码 categoryId **0 不一致**;无指向缺失分类节点的工具 |
| 6 | 清单结构完整 | ✅ 565 frontend / 31 Go / 32 legacy,层级顺序无违反;新增行证据齐备 |
| 7 | 工具信息单一来源 | ✅ 前端运行时目录(tools.ts)与 docs/tool-inventory.json 同步;状态由 toolStatus.ts 接线推导,无手填 |
| 8 | 占位工具不伪装可用 | ✅ 占位工具:StatusBadge 显示「占位」/「后端占位」+ PlaceholderBanner 常驻提示;已实现工具不渲染横幅 |
| 9 | 12 个批 1 工具状态 | ✅ 全部 implemented(3 个为批 1 新接线/新增,9 个为已有接线确认) |
| 10 | 设计令牌落地 | ✅ ToolCard/ToolPage/两个新组件只消费 `--ct-*` 语义别名;WCAG AA 对比度沿用 design-system 已验证数值 |
| 11 | 动效 ≤200ms | ✅ 新增交互仅 duration-fast(100ms)/base(150ms) |
| 12 | 移动端不横向溢出 | ✅ 改动均为自适应布局(min-w-0/truncate/flex-wrap),无固定宽新增 |
| 13 | 文件上传链路 | ✅ FileControl 值恒为 File[](已核实 DynamicForm.tsx:529);上传→file_id→runTool→Go resolveFileParams 落盘链路完整 |
| 14 | 无控制台错误 | ✅ 生产构建通过;新增组件无副作用,引用均为已有导出 |

## 3. 校验方法(可复现)

- 补丁脚本 `patch_inventory_b1.cjs`:esbuild 剥离类型 → vm 求值 tools.ts → 字面 id 行号正则重刷证据 → 定点更新 → stats 重算(幂等,可重跑)。
- 一致性脚本 `parity_check_b1.cjs`:以 tools.ts 实际导出 + 本地注册表静态扫描(localHandlerRegistry.register 与 **regCodec 两条注册路径**)推导状态,与清单逐条对比。
- 校验过程发现并修正的脚本问题:初版扫描漏掉 `regCodec('local.rot5')` 等 19 个间接注册,一度误报 19 处不一致;补全注册路径后 **0 不一致**,证明这 19 个 codec 工具的 implemented 判定正确、清单无需改动。

## 4. 剩余风险

1. **`/c/crypto.encoding` → `/c/codec` 重定向推迟到批 2**。批 1 只有 base64 迁入 codec.base;现在加重定向会孤立其余 75 个编码工具的分类页。批 2 完成 codec 平移后一次性落地(ia.md §5.2 已列)。
2. **StatusBadge 暂不可点击**。徽章点击(跳 `/search?status=x`)依赖批 0 的搜索页;当前按 design-system 以 span 渲染,已实现工具渲染不受影响。
3. **F1 参数面收缩(有意为之)**:http-headers 的「自定义请求头(keyvalue)」、strings 的「多文件/Unicode」参数不再提供——Go 契约(web.http_headers 仅 url;reverse.strings 单文件 ASCII 扫描)不支持。老用户习惯受影响,已在清单 riskNotes 记录。
4. **纯浏览器 `npm run dev` 下的回退**:无 Wails 运行时,realBackend 任务类工具回退模拟结果(原有回退机制);徽章仍显示「已实现」——徽章语义是「接线状态」而非「当前环境是否有真实后端」,与清单判定规则一致。
5. **模板生成 id 的证据行号为 `?`**(如 web.ssti.jinja2、crypto.encoding.base.base64):字面 id 正则无法命中模板字符串,沿用阶段 1 已确立的证据纪律,不做伪精确。
6. **`misc.file_checksum` 的 Go/legacy 同 id 三层并存**:清单靠 oldPath 前缀区分三层,补丁脚本已按前缀锁定行;后续任何脚本工具都必须沿用此约定(本次已踩坑并修复)。
7. **legacy 静态页**仍指向旧分类(misc.stego 等),legacy 归档不参与构建,按计划不做迁移。
