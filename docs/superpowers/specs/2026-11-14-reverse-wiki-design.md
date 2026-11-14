# 设计:逆向 Wiki(逆向模式知识库 + 函数速查 + 进制查询)

日期:2026-11-14
状态:待用户批准
规模:项目级(后端镜像实例 + 组件抽取 + 35 条种子内容 + 查询小部件)

## 意图与为什么

逆向模式需要一个**随用随记的知识库**:反汇编看到的陌生函数(`_acrt_iob_func` 这类 CRT/Win32 符号)、立即数含义(`0x64` → `d`)、算法特征、脱壳手法、踩坑笔记——这些知识散落在每次解题过程里,不沉淀下来每次都重查。项目已有完全同构的「红队 Wiki」基础设施(渗透模式),镜像一套即可,风险低、体验对称。

## 目标

1. 逆向模式工具页可打开「逆向 Wiki」工作区:左侧 8 分类导航,右侧 markdown 条目详情,支持搜索/新建/编辑/删除。
2. 桌面端持久化到 `data/reverse_wiki.db`(SQLite,与 wiki.db 同款 Store);浏览器开发模式回退 localStorage 并自动灌种子。
3. 内置 8 分类约 35 条种子速查(下详),用户可持续追加(逆向记录主场景)。
4. 工作区顶部**进制 ↔ ASCII 即时查询条**:输入 `0x64`/`100`/`d` 互转,支持批量(`64 65 66` → `def`,反向亦然)。

## 非目标

- 不做全 wiki 全文搜索(沿用现有按标题过滤);不做条目附件/图片。
- 不改红队 Wiki 的任何行为(抽取公共组件后红队侧仅换壳,零行为变化)。
- 不做函数速查的"自动识别反汇编文本"(纯对照库,不解析 IDA 输出)。

## 方案取舍

| 方案 | 内容 | 结论 |
|---|---|---|
| A 镜像红队 Wiki 全套 | 第二 Store 实例 + 同款绑定 + 组件抽取复用 + 种子 | **选定**——基础设施全有,体验对称 |
| B 纯前端 localStorage | 不动 Go | 否决——桌面端知识不能跨机,双轨架构 |
| C 并入红队 Wiki 加分类 | 省事 | 否决——逆向模式跨域引用渗透模式,域混乱 |

## 设计决策

### 后端(Go)

- **复用 `internal/wiki` 包不改 Store**:新建 `internal/wiki/reverse_seed.go` 定义 `ReverseBuiltinSeed []SeedEntry`(同包新变量,不碰 `BuiltinSeed`)。
- **`main.go`**:`wiki.Open(DataDir/reverse_wiki.db)` → 空表时 `Seed(ReverseBuiltinSeed)`;App 结构体加字段 `ReverseWiki *wiki.Store`(app.go:94 旁)。打开失败降级 nil(与 wiki.db 同款容错)。
- **新文件 `reverse_wiki.go`**:四绑定 `ListReverseWikiEntries / CreateReverseWikiEntry / UpdateReverseWikiEntry / DeleteReverseWikiEntry`,实现照抄 wiki.go 换 store 字段与错误文案。
- wailsjs 绑定由 `wails build`/`wails generate module` 再生成并提交(仓库跟踪 wailsjs)。

### 前端(React)

- **组件抽取**:把 `RedTeamWikiWorkspace.tsx`(323 行)逻辑抽成通用 `components/wiki/WikiWorkspace.tsx`,props:
  `categories: WikiCategory[]`,`adapter: { list(category): Promise<WikiEntry[]>; create; update; delete }`,`topWidget?: ReactNode`。
  原 323 行中对红队的耦合仅两个 import(WIKI_CATEGORIES/wikiCategoryById 与 lib/wiki 函数),全走 props 后零耦合。
  `RedTeamWikiWorkspace.tsx` 变薄壳:传红队分类与 lib/wiki 适配器(**不改任何行为**)。
- **`ReverseWikiWorkspace.tsx`** 薄壳:传逆向分类 + `lib/reverseWiki` 适配器 + 顶部挂 `AsciiConverter`。
- **`lib/reverseWiki.ts`**:镜像 lib/wiki.ts;localStorage 键 `ctfnet.reverse_wiki_entries`;桌面端 cast 声明四个 Reverse 绑定(照 lib/wiki.ts 的 `as unknown as` 模式,TS 不等 codegen)。
- **`mock/reverseWiki.ts`**:`REVERSE_WIKI_CATEGORIES`(8 分类)+ `REVERSE_WIKI_SEED`(与 Go ReverseBuiltinSeed 内容一致,浏览器模式用)。
- **`AsciiConverter`**(新小组件,`components/wiki/AsciiConverter.tsx`,纯前端):
  - 输入自动识别:`0x64`/`64`(十六进制)、`100`(十进制,≥3 位纯数字按十进制;`0x` 前缀强制十六)、`'d'`(单字符)→ 输出三种表示;
  - 批量模式:空格/逗号分隔数字串 ↔ 字符串互转;非法输入(>0x10FFFF、空)显示提示;
  - 同时给出 URL 编码形态(顺手,%64)——逆向 webshell/流量常见。
- **接线**:
  - `categories.ts` reverse 节点 children 追加 `{ id: 'reverse.wiki', name: '逆向 Wiki', icon: 'NotebookPen', description: '逆向知识库:函数速查、进制 ASCII、套路笔记' }`(NotebookPen 已在 lib/icons.tsx 映射;BookOpen 未映射不用)。
  - `tools.ts` 追加 `{ id: 'reverse.wiki.reverse-wiki', name: '逆向 Wiki', categoryId: 'reverse.wiki', description: '逆向知识库:常用函数、进制转换、特征识别速查与个人笔记', tags: ['逆向', 'Wiki', 'local'], workspace: 'reverse-wiki' }`(镜像 pentest.recon.redteam-wiki 条目形态;无 backend/preset 字段——工作区型工具与红队 wiki 一致)。
  - `ToolPage.tsx` workspace 分发追加 `reverse-wiki` → `ReverseWikiWorkspace`。
- **模式可见性**:reverse 根已在 reverse 模式与 CTF 模式 categoryRoots 中,新子分类自动可见,无需改 modes.ts。

### 种子内容(8 分类,~35 条,我起草)

| 分类 id | 名称 | 条数 | 内容要点 |
|---|---|---|---|
| algo | 算法与特征识别 | 4 | TEA/XTEA delta 常数、RC4 KSA 结构、Base64 变表识别、MD5/SHA 初始常数 |
| tools | 工具操作 | 4 | IDA 快捷键速查、IDA Python 常用片段、Ghidra 导出与脚本、x64dbg 常用操作 |
| antidebug | 反调试与对抗 | 5 | IsDebuggerPresent/PEB BeingDebugged、NtQueryInformationProcess、时间差检测、OutputDebugString 反调、反反调试手法 |
| unpack | 壳与脱壳 | 3 | UPX 识别与命令、ESP 定律手脱、常见壳节区名特征 |
| formats | 文件格式 | 4 | ELF 头字段要点、PE 结构与节区、pyc 反编译流程、APK 逆向流程(jadx/apktool) |
| functions | 常用函数速查 | 12 | MSVC CRT:`_acrt_iob_func`(stdio 流句柄)、`__stdio_common_vfprintf`、`memcpy` 族、`__security_check_cookie`;Win32 按用途:文件/注册表/进程/网络/反调试 API;libc:read/write/system |
| ascii | 进制与 ASCII | 2 | 可打印字符 0x20–0x7E 对照表、小端多字节立即数读法与高频值 |
| notes | 套路与笔记 | 1 | CTF 逆向通用流程(空库起步,用户主战场) |

Go 与前端 mock 双份种子内容一致(红队 wiki 同款惯例;`ctfnet-aisec-mode` 记忆中的"模板库双份同步"教训——设计上靠同文档对照写入,测试断言条目数一致)。

## builder check 裁定(2026-11-14 预审通过,两项 builder 自主决策)

1. **AsciiConverter 歧义消解**(单框输入判定优先级):`0x` 前缀 → 十六进制码点;纯数字 → 十进制码点;其他 → 按原文字符串逐字符转码。UI 在输出下方常驻一行小字说明该规则(消除 `d` = 字符还是 hex 0x0d 的歧义:按此规则 `d`→字符)。**批量模式用显式方向开关**(数字串→字符串 / 字符串→数字串),彻底消除 `beef` 这类全 hex 词的歧义。
2. **8 分类的 lucide 图标与描述**:实现时从 lib/icons.tsx 已映射集合中选取并逐一核验(候选 Binary/Wrench/ShieldCheck/Boxes/FileCode/Terminal/Layers/NotebookPen),不引入未映射图标。

## 约束

- 不新增 npm/Go 依赖;ASCII 转换用 `String.fromCharCode`/`charCodeAt`,批量解析手写。
- 红队 Wiki 行为零变化(抽取后走一遍红队 wiki 手动核对:列表/搜索/新建/编辑/删除)。
- 种子仅首次空表时灌入,用户改动不被覆盖(与 wiki.db 同款 Seed 语义)。
- `data/reverse_wiki.db` 属运行时数据:不入 git(.gitignore /data/ 已覆盖)、不进代码备份(备份排除 *.db,与 wiki.db 同等待遇)。

## 测试口径

- **Go**:`internal/wiki` 新增 `reverse_seed_test.go`——断言 ReverseBuiltinSeed 非空、分类 id 均在 8 个白名单内、无重复标题;`go test ./internal/wiki/...` 全绿。
- **前端**:无测试框架,口径 = `npm run build` 过 + 手动清单(见下)。
- **种子一致性**:Go 侧测试断言分类数=8、条目数=前端 mock 同值(手查;两侧数字写死在设计里,实现时对齐)。
- **手动清单**:`npm run dev` 下打开逆向模式 → 逆向 Wiki 工具 → 分类 8 个、种子条目在 → 新建/编辑/删除一条 → 刷新浏览器(localStorage 持久)→ AsciiConverter 输 `0x64` 得 `d(100,%64)`、批量 `64 65 66` 得 `def`、输 `flag` 得 `66 6c 61 67`;红队 wiki 回归五动作;`wails build` 出 exe 后桌面端同过一遍(db 落 data/)。

## 留给实现

1. Go:reverse_seed.go(35 条内容按上表撰写)、main.go/app.go 接线、reverse_wiki.go 四绑定、reverse_seed_test.go。
2. 前端:mock/reverseWiki.ts、lib/reverseWiki.ts、WikiWorkspace 抽取、两个薄壳、AsciiConverter、categories/tools/ToolPage 三处接线。
3. `wails generate module` 刷新 wailsjs 并随代码提交;`wails build` 出新 exe。
4. README 工具清单与模式描述补一行「逆向 Wiki」。
