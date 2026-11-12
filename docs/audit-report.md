# CTF-NET 第一阶段只读审计报告

生成时间:2026-11-10(审计阶段 1,只读,未修改任何业务代码)
单一事实源:`docs/tool-inventory.json`(627 条,含证据字段)

---

## 1. 执行摘要

| 事实源 | 总数 | implemented | placeholder | broken | unknown |
| --- | --- | --- | --- | --- | --- |
| 前端目录 `frontend/src/mock/tools.ts` | 564 | 59 | 505 | 0 | 0 |
| Go 注册表 `internal/tools/**` | 31 | 12 | 19 | 0 | 0 |
| legacy Python `legacy/tools/**` | 32 | 13 | 19 | 0 | 0 |
| **合计** | **627** | **84** | **543** | **0** | **0** |

- **89.5% 的前端工具是占位符**:564 个前端工具页中 505 个点击后返回 mock 模拟结果,且当前 UI 没有任何状态徽章区分真假工具。
- 无 broken / unknown 条目(所有条目均有证据,见第 4 节判定规则)。
- 基线健康度:**通过**。`frontend: npx tsc --noEmit`(0 错误)、`go vet ./...`(0 问题)、`go test ./...`(全过,web 包 28.7s)、无重复工具 id、无悬空 categoryId。

## 2. 技术栈与结构

- **桌面壳**:Wails v2 (`wails.json` + `main.go` + `app.go`),Go 1.27(`go.mod` 模块 `ctfnet`)。
- **前端**:React 18 + TypeScript 5.6 + Vite 5 + Tailwind 3(class 暗色模式)+ zustand + react-router-dom 6 + lucide-react。
- **Go 后端**:`internal/core`(注册表/任务队列/交互会话/文件存储)+ `internal/tools/{crypto,misc,reverse,web}`;Wails 绑定面等价旧 REST 契约(`app.go`:RunTool/GetJob/ExecSessionCommand 等)。
- **legacy/**:旧 Python FastAPI 版归档(`pyproject.toml` + `app/` + `tools/`),不参与构建;README 声明其为移植参照。
- 路由仅 3 条:`/`(HomePage)、`/c/:categoryId`(CategoryPage)、`/t/:toolId`(ToolPage)——`frontend/src/router/index.tsx`。

## 3. 三个工具事实源与证据

1. **前端目录(564)**:`mock/tools.ts` 是唯一前端事实源 —— 15 个"完整工具"逐字手写 + 12 个 Base 家族 + 4 个 codec 工具 + 20 个 SSTI 工具 + 约 500 个由 `presets`/`shorthandRows`/`yangWebShorthandRows`/`cyberChefShorthandRows` 批量生成。每条证据:`file:frontend/src/mock/tools.ts:<行号>`,`route:/t/<id>`,`menuItem:分类树节点 <categoryId>`,`component:ToolPage(+Workspace)`。
2. **Go 注册表(31)**:`internal/tools/register.go` 空白导入 4 包,各包 `init()` 经 `core.DefaultRegistry.Register` 注册;ID 规则 `category.name`(`internal/core/tool.go:128`)。12 个真实实现均有单元测试或全流程测试(jndi 含进程内 SSH 测试)。
3. **legacy(32)**:`@register_tool` 装饰器 + `registry.discover("tools")` 自动发现(`legacy/app/core/registry.py`);13 个真实实现,19 个 ssti 文件首行注明"占位符(逐引擎实现,本文件待实现)"。

## 4. 状态判定规则(防编造)

- **implemented** = 存在真实执行路径:前端 local handler 已在 `localHandlerRegistry` 注册(57 个 handler),或 `task + realBackend` 指向 Go 真实实现(4 个:jndi / ssti_jinja2 / aes_cbc_bitflip / whitespace_codec)。
- **placeholder** = 执行路径走 mock:`http` 适配器(模拟延迟 + `generateMockResult`)、`task` 无 `realBackend`(模拟进度)、`local` handler 未注册(回退 `generateMockResult`)、Go 19 个 ssti 占位执行器(返回"[占位符] … 功能开发中")、legacy 19 个 ssti 占位 run()。
- **broken / unknown** = 无(每条都有上述可验证证据)。

## 5. 关键发现(风险清单)

### F1. 接线断裂(3 处,高优先级)
| 前端工具 | 现状 | 后端真实实现 |
| --- | --- | --- |
| `reverse.static.strings` | 误接 `/api/mock/strings/submit`,返回模拟数据 | `reverse.strings`(Go,已实现) |
| `web.basic.http-headers` | 误接 `/api/mock/http-headers`,返回模拟数据 | `web.http_headers`(Go,已实现) |
| `misc.file_checksum` | **前端无任何工具页(孤儿)** | `misc.file_checksum`(Go,已实现) |

### F2. 前后端功能重复(5 处,中优先级)
Go `crypto.base64` / `crypto.rot13` / `crypto.beast` / `misc.hash` / `misc.wechat_dat` 已实现,但前端均走本地实现(功能正常,后端为死代码);需决定保留本地实现并退役 Go 侧,还是改接 realBackend。

### F3. 占位无标识(用户体验缺陷)
564 个前端工具页**没有任何状态徽章/占位态设计**;505 个占位工具与 59 个真实工具外观完全一致,用户无法区分真假。`ToolCard`、`ToolPage` 均无 status 渲染。

### F4. 分类树 4 层,违反"导航 ≤3 层"规则
`crypto.modern.rsa.algebraic` 等路径达 4 层(`crypto → modern → rsa → algebraic`,`frontend/src/mock/categories.ts:56-66`);类别树注释自称"最多 4 层子分类"。

### F5. 命名/挂载不一致(40 个工具)
40 个 id 前缀为 `misc.encoding.*` 的工具实际挂载在 `crypto.encoding` 分类下(如 `misc.encoding.brainfuck` → categoryId `crypto.encoding`)。另外 `misc.stego.*`(3 个)与独立 `stego` 大类并存,存在双隐写域。

### F6. 前端 id 与后端 id 命名不一致
`web.deserialize.jndi`(前端)→ `web.jndi`(Go);`crypto.modern.aes.cbc-bitflip`(前端)→ `crypto.aes_cbc_bitflip`(Go)。靠 submitUrl 字符串解析桥接,脆弱。

### F7. 首页"精选工具"是假的
`HomePage.tsx:33` 的精选 = `tools.slice(0, 12)`(数组前 12 个,含 mock 工具如 `crypto.modern.rsa-decrypt`),不是真实使用热度。

### F8. legacy 与现版的移植差异
legacy 13 个已实现 Python 工具中,`misc.teyvat` 未移植到 Go(README 注明,前端 `local.teyvat` 覆盖);其余 12 个均已以 Go 或前端本地实现覆盖。

### F9. 分类分布失衡
crypto 266(47%)、misc 101(18%)、web 64、forensics 54、stego 34、reverse 14、pwn 11、blockchain 7、osint 6、mobile 4、ai 3。11 个一级分类节点本身均不直接挂工具(正常,工具在子分类)。

## 6. 现有 UI 能力盘点(阶段 2 设计的基础)

已有:`CommandPalette` 全局搜索(Ctrl+K)、收藏 + 最近使用(localStorage 持久化,`useToolStore`/`useHistoryStore`)、暗色模式(`useUiStore` + Tailwind class 策略)、响应式侧边栏(折叠/移动抽屉)、面包屑、分类树搜索、调用历史回填、输出状态机(idle/loading/streaming/success/error/cancelled)、SSE/WS 流式输出演示、ShellView 交互会话。
缺失:**状态徽章体系、占位态/空态设计规范、加载骨架(目前仅进度条 + animate-pulse 指示点)、设计令牌(颜色/间距/圆角/阴影全部散落在 Tailwind 原子类中,`tailwind.config.js` 的 extend 为空)、工具页头部模板、筛选/标签检索入口(目前只有名称模糊搜索)**。

## 7. 进入阶段 2 前需要你决策的问题

完整清单(按影响排序,≤20 条)见 **`docs/confirm-questions.md`**。影响最大的 6 条:

1. **范围**:564 个前端工具全量纳入重构(含 CyberChef/YangWeb 风格的批量生成工具)?还是只保留某几类?
2. **F1 接线断裂**:`strings` / `http_headers` 改接 Go 真实后端,并为 `file_checksum` 补前端页?(建议第 0 批做)
3. **F2 重复实现**:本地实现与 Go 实现二选一,还是维持现状只记录风险?
4. **F4 层级**:分类树压到 3 层是否接受?(映射方案见 `docs/category-mapping.md` 表 C)
5. **占位透明化**:505 个占位工具统一挂"占位徽章 + 标准占位态",还是隐藏?
6. **F5 命名规范**:`misc.encoding.*` 挂载点保持 `crypto.encoding` 不动,还是按 id 前缀迁回 `misc`?

## 8. 旧分类 → 建议分类初步映射表

见 **`docs/category-mapping.md`**:总原则(基于现有分布、大部分保持不变)+ 表 A 保持统计 + 表 B 具体变更(40 misc.encoding + 3 misc.stego + 1 beast)+ 表 C 四层→三层扁平化(22 个 RSA 工具,方案 A/B)+ 表 D 后端/legacy 映射。

## 9. 重复项(3 组名称重复 + 5 处功能重复 + 0 个 id 重复)

- **id 重复:0**(`toolMap` 建图前已验证,Map 会静默覆盖故重点核查)。
- **名称重复 3 组**(均在 564 前端工具内):
  1. `RSA 解密` → `crypto.modern.rsa-decrypt`(mock task)与 `crypto.modern.rsa.decrypt`(local 未注册 → 占位)—— 功能同域重复。
  2. `文件上传绕过` → `web.upload.burp-bypass` 与 `web.upload.upload-bypass`(均占位,同分类)。
  3. `Shellcode 编码` → `pwn.shellcode.encoder`(pwn)与 `misc.encoding.shellcode`(misc,已实现 local)—— 跨域重名,语义不同。
- **功能重复 5 处(F2)**:Go `crypto.base64/rot13/beast`、`misc.hash/wechat_dat` 与前端本地实现重复(见第 5 节表)。
- **名称近似易混淆**:`misc.stego.whitespace-codec`(Snowfall 隐写+解释器)、`misc.encoding.whitespace`(Whitespace 语言编码)、`stego.text.whitespace`(文本空白隐写)—— 三个"whitespace"工具,建议映射表 B 中标注了辨析。

## 10. 疑似废弃项(待人工确认,阶段 4 之前不删)

| 对象 | 证据 | 建议 |
| --- | --- | --- |
| `legacy/` 全部 32 个 Python 工具 | 目录注释"归档,不参与构建";README §与旧 Python 版差异 | 保留作移植参照;确认不再需要后可打包归档(阶段 4) |
| Go `crypto.base64/rot13/beast`、`misc.hash/wechat_dat`(5 个) | 前端均走本地实现,Go 侧无消费者 | 退役候选(取决于决策问题 3) |
| `crypto.modern.rsa-decrypt` 的 `docsUrl: 'https://example.com/rsa'` | 唯一 docsUrl 且为 example.com 占位链接 | 删除或替换为真实文档 |
| `legacy/backend.log` / `backend.err.log` / `mock.log` / `mock_server.log` | 旧运行日志残留 | 清理候选 |
| `backups/*.zip`(5 个,1109~1236 时间戳) | 历史备份,不参与构建 | 保留,确认重构稳定后按周期轮换 |
| `.screenshots/`(空目录)、`legacy/14607_index.php.swp`(vim 交换文件)、`legacy/wsl.txt'`(文件名带单引号) | 残留物 | 清理候选 |
| `build/windows`、`frontend/dist` | 构建产物 | 不清理,但入库前建议 .gitignore |

## 11. 审计方法(可复现)

- 工具目录经 esbuild 转译 `frontend/src/mock/tools.ts` 后动态执行导出,得到 564 条真实定义(非手抄);
- 与 `localHandlers.ts` 57 个注册 handler、`categories.ts` 62 个分类节点、Go 注册表 31 个工具、legacy `@register_tool` 32 个工具四方交叉比对;
- 每条工具记录 `tools.ts` 行号、路由、菜单项、组件名、invoke 规格作为证据;无法确认即 unknown(本轮未出现);
- 占位标记词扫描(TODO/coming soon/敬请期待/开发中/占位/not implemented)结果与代码结构一致(19 个 ssti 占位 × Go/legacy/前端描述三处互相印证)。
