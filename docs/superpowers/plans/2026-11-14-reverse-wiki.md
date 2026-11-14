# 逆向 Wiki Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 逆向模式新增「逆向 Wiki」知识库——镜像红队 Wiki 全套(SQLite 第二实例 + 组件抽取复用),内置 8 分类 35 条种子,工作区顶部带进制↔ASCII 即时查询条。

**Architecture:** 后端复用 `internal/wiki` 包零改动,新增 `ReverseBuiltinSeed` 与 `reverse_wiki.db` 第二实例 + 四个 Wails 绑定;前端把 323 行 `RedTeamWikiWorkspace` 抽取为通用 `WikiWorkspace`(props 注入分类/适配器/顶部部件),红队侧变薄壳零行为变化,逆向侧薄壳挂 `AsciiConverter`。

**Tech Stack:** Go + modernc.org/sqlite(既有);React + TS + Vite(既有);无新增依赖。

**Spec:** `docs/superpowers/specs/2026-11-14-reverse-wiki-design.md`(含 builder check 两项裁定)

## Global Constraints

- 不新增 npm / Go 依赖。
- 红队 Wiki 行为零变化(抽取后仅换壳)。
- 种子仅空表时灌入(main.go 现有 Count==0 → Seed 语义)。
- `data/reverse_wiki.db` 属运行时数据:`/data/` 已在 .gitignore;代码备份排除 `*.db`(与 wiki.db 同等待遇)。
- 图标只用 `frontend/src/lib/icons.tsx` 已映射集合(NotebookPen 已核实;BookOpen 未映射禁用)。
- 前端无测试框架:前端验证口径 = `npm run build` + grep 断言 + 手动清单;Go 验证 = `go test`。
- 提交:中文 `feat:`/`docs:` 前缀,末尾 `Co-Authored-By: Claude Code <noreply@anthropic.com>`;只 `git add` 本计划涉及文件。工作树当前干净,可按任务正常提交。
- 基准目录 `C:\Users\HP\Desktop\CTF-NET`;前端命令在 `frontend/` 下执行。

## Review Focus

1. **种子双份漂移**(Go `ReverseBuiltinSeed` 与前端 `REVERSE_WIKI_SEED` 数量/关键条目不一致 → 桌面与浏览器模式数据不同)→ Task 1 Go 测试钉死 8 分类 4/4/5/3/4/12/2/1=35;Task 3 grep 断言 mock 侧同为 35 且锚点条目(`_acrt_iob_func`、ASCII 对照表)两侧都在。
2. **红队 Wiki 回归**(抽取改坏通用组件)→ Task 4 手动五动作回归(列表/搜索/新建/编辑/删除)+ `npm run build`。
3. **AsciiConverter 歧义/非法输入**(`beef`、`7`、空串、`0x110000` 越界、中文)误判或崩溃 → Task 5 手动清单逐条:非法与越界显示提示不抛错;`beef` 按字符串;`7` 按十进制码点;实现内码点范围 0–0x10FFFF 校验。
4. **wailsjs 未再生成**(桌面端调到 undefined)→ Task 6 先 `wails generate module`,grep 四方法存在于 wailsjs 后才提交。
5. **localStorage 键冲突**(两 wiki 数据互串)→ Task 3 键名钉死 `ctfnet.reverse_wiki_entries`,grep 断言与 `ctfnet.wiki_entries` 并存且互异。

---

### Task 1: Go — ReverseBuiltinSeed 种子 + 完整性测试

**Files:**
- Test: `internal/wiki/reverse_seed_test.go`
- Create: `internal/wiki/reverse_seed.go`

**Interfaces:**
- Consumes: `internal/wiki.SeedEntry{Category, Title, Content string}`(既有)。
- Produces: `var ReverseBuiltinSeed []SeedEntry`(35 条;分类白名单与每类条数见测试)——Task 2 接线消费。

- [ ] **Step 1: 写失败测试(reverse_seed_test.go)**

```go
package wiki

import "testing"

var reverseCategoryCounts = map[string]int{
	"algo": 4, "tools": 4, "antidebug": 5, "unpack": 3,
	"formats": 4, "functions": 12, "ascii": 2, "notes": 1,
}

func TestReverseBuiltinSeedCounts(t *testing.T) {
	total := 0
	counts := map[string]int{}
	titles := map[string]bool{}
	for _, e := range ReverseBuiltinSeed {
		if e.Category == "" || e.Title == "" || e.Content == "" {
			t.Fatalf("空字段: %+v", e)
		}
		if titles[e.Title] {
			t.Fatalf("重复标题: %s", e.Title)
		}
		titles[e.Title] = true
		counts[e.Category]++
		total++
	}
	if total != 35 {
		t.Fatalf("总条数 = %d, 期望 35", total)
	}
	for cat, want := range reverseCategoryCounts {
		if counts[cat] != want {
			t.Errorf("分类 %s = %d 条, 期望 %d", cat, counts[cat], want)
		}
	}
	if len(counts) != 8 {
		t.Fatalf("分类数 = %d, 期望 8", len(counts))
	}
}

func TestReverseBuiltinSeedAnchors(t *testing.T) {
	anchors := []string{"_acrt_iob_func", "IsDebuggerPresent", "UPX"}
	have := map[string]bool{}
	for _, e := range ReverseBuiltinSeed {
		have[e.Title] = true
	}
	for _, a := range anchors {
		if !have[a] {
			t.Errorf("锚点条目缺失: %s", a)
		}
	}
}
```

- [ ] **Step 2: 跑测试看它失败**

Run: `go test ./internal/wiki/ -run TestReverseBuiltin -v`
Expected: FAIL,`undefined: ReverseBuiltinSeed`

- [ ] **Step 3: 写 internal/wiki/reverse_seed.go**

`var ReverseBuiltinSeed = []SeedEntry{...}` 35 条。内容按 spec 种子表撰写(要点列在表内;条目标题用速查名词,如 `TEA/XTEA 特征`、`IDA 快捷键速查`、`_acrt_iob_func`);markdown 内容紧凑实用(命令/常数/要点,代码块 4 空格缩进,与 seed.go 风格一致)。锚点标题必须含 `_acrt_iob_func`、`IsDebuggerPresent`、`UPX`。

- [ ] **Step 4: 跑测试看它通过**

Run: `go test ./internal/wiki/ -v`
Expected: 全 PASS(含既有 store_test)

- [ ] **Step 5: Commit**

```bash
git add internal/wiki/reverse_seed.go internal/wiki/reverse_seed_test.go
git commit -m "feat: 逆向 Wiki 内置种子(8 分类 35 条速查)+ 完整性测试

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 2: Go — 第二实例接线 + 四绑定

**Files:**
- Create: `reverse_wiki.go`(仓库根,与 wiki.go 并列)
- Modify: `app.go:94`(Wiki 字段旁加 ReverseWiki)
- Modify: `main.go:51-65`(wiki 接线块后镜像一块)

**Interfaces:**
- Consumes: Task 1 的 `wiki.ReverseBuiltinSeed`;既有 `wiki.Open/Seed/Count`。
- Produces: `App.ReverseWiki *wiki.Store`;绑定 `ListReverseWikiEntries(category string) ([]*wiki.Entry, error)`、`CreateReverseWikiEntry(category, title, content string) (*wiki.Entry, error)`、`UpdateReverseWikiEntry(id, title, content string) (*wiki.Entry, error)`、`DeleteReverseWikiEntry(id string) error`(Task 6 的 wailsjs 再生成消费)。

- [ ] **Step 1: 写 reverse_wiki.go**

镜像 wiki.go 逐行换:`a.reverseWikiStore()` 判 `a.ReverseWiki == nil` 返回 `errors.New("逆向 Wiki 不可用:数据库打开失败")`;四方法名换 Reverse 前缀,实现同款透传。

- [ ] **Step 2: app.go 加字段**

`Wiki *wiki.Store` 行旁加 `ReverseWiki *wiki.Store // 逆向 Wiki(reverse_wiki.go)`,注释同步。

- [ ] **Step 3: main.go 接线**

镜像 51-65 块:`wiki.Open(filepath.Join(cfg.DataDir, "reverse_wiki.db"))`;失败 log 降级 nil,成功则 `defer Close` 且 `Count()==0` 时 `Seed(wiki.ReverseBuiltinSeed)`(Count>0 跳过——种子不覆盖用户数据);App 装配处 `ReverseWiki:` 字段。

- [ ] **Step 4: 编译与全量测试**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: 全部 exit 0

- [ ] **Step 5: Commit**

```bash
git add reverse_wiki.go app.go main.go
git commit -m "feat: 逆向 Wiki 后端——reverse_wiki.db 第二实例与四绑定(空表才灌种子)

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 3: 前端 — mock/reverseWiki.ts + lib/reverseWiki.ts

**Files:**
- Create: `frontend/src/mock/reverseWiki.ts`
- Create: `frontend/src/lib/reverseWiki.ts`

**Interfaces:**
- Consumes: `WikiCategory`/`WikiSeedEntry` 类型(自 `@/mock/redteamWiki` 导入);`WikiEntry` 类型(自 `@/lib/wiki` 导入)。
- Produces: `REVERSE_WIKI_CATEGORIES: WikiCategory[]`(8)、`reverseCategoryById(id): WikiCategory | undefined`、`REVERSE_WIKI_SEED: WikiSeedEntry[]`(35);`listReverseWikiEntries(category: string): Promise<WikiEntry[]>`、`createReverseWikiEntry(category, title, content)`、`updateReverseWikiEntry(id, title, content)`、`deleteReverseWikiEntry(id): Promise<void>`(Task 4 消费)。

- [ ] **Step 1: 写 mock/reverseWiki.ts**

分类 8 个(id 沿用 Go 白名单:algo/tools/antidebug/unpack/formats/functions/ascii/notes),图标从已映射集合选并逐一核验(建议 algo=Binary、tools=Wrench、antidebug=ShieldCheck、unpack=Boxes、formats=FileCode、functions=Terminal、ascii=Search、notes=NotebookPen——写前 grep lib/icons.tsx 确认,未映射就换 FileText);`REVERSE_WIKI_SEED` 35 条**内容与 reverse_seed.go 一致**(对照逐条搬运,锚点 `_acrt_iob_func`/`IsDebuggerPresent`/`UPX` 必含)。

- [ ] **Step 2: 写 lib/reverseWiki.ts**

镜像 lib/wiki.ts:Wails cast 声明四个 Reverse 方法;localStorage 键 `ctfnet.reverse_wiki_entries`;首次使用以 REVERSE_WIKI_SEED 灌入;导出四函数包装(桌面走绑定,浏览器走 ls)。

- [ ] **Step 3: grep 断言 + build**

Run:
```bash
cd frontend
grep -c "category: '" src/mock/reverseWiki.ts | grep -q "^35$" || echo FAIL  # 种子 35 条
grep -c "id: '" src/mock/reverseWiki.ts                                      # ≥8(8 分类+无冲突)
grep -o "icon: '[A-Za-z]*'" src/mock/reverseWiki.ts | sort -u                # 逐个对照 icons.tsx
grep -n "ctfnet.reverse_wiki_entries" src/lib/reverseWiki.ts                 # 键名钉死
npm run build                                                                # exit 0
```
另外跨侧对齐:`grep -c "Category:" ../internal/wiki/reverse_seed.go` 与 mock 侧 35 同值;锚点标题两侧 grep 均命中。

- [ ] **Step 4: Commit**

```bash
git add frontend/src/mock/reverseWiki.ts frontend/src/lib/reverseWiki.ts
git commit -m "feat: 逆向 Wiki 前端数据层——8 分类 35 条种子与 Wails/localStorage 双轨适配

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 4: 前端 — WikiWorkspace 抽取 + 两薄壳 + 三处接线

**Files:**
- Create: `frontend/src/components/wiki/WikiWorkspace.tsx`
- Rewrite: `frontend/src/components/wiki/RedTeamWikiWorkspace.tsx`(323 行 → 薄壳)
- Create: `frontend/src/components/wiki/ReverseWikiWorkspace.tsx`(薄壳,本任务先不带 topWidget)
- Modify: `frontend/src/pages/ToolPage.tsx:129-140`(分发分支)
- Modify: `frontend/src/mock/categories.ts`(reverse children 追加)
- Modify: `frontend/src/mock/tools.ts`(追加工具条目)

**Interfaces:**
- Consumes: Task 3 的 `REVERSE_WIKI_CATEGORIES`/`reverseCategoryById`/lib 四函数;既有 lib/wiki 四函数与 WIKI_CATEGORIES。
- Produces: `WikiWorkspaceProps { categories: WikiCategory[]; categoryById: (id: string) => WikiCategory | undefined; adapter: { list(category: string): Promise<WikiEntry[]>; create(category: string, title: string, content: string): Promise<WikiEntry>; update(id: string, title: string, content: string): Promise<WikiEntry>; delete(id: string): Promise<void> }; topWidget?: React.ReactNode }`,默认导出通用组件(Task 5 消费 topWidget);`ReverseWikiWorkspace` 默认导出。

- [ ] **Step 1: 抽取 WikiWorkspace.tsx**

把 RedTeamWikiWorkspace 323 行整体迁移:两个红队 import 换成 props(组件内 `WIKI_CATEGORIES`→`categories`、`wikiCategoryById`→`categoryById`、四个 lib 调用→`adapter.xxx`);在两栏布局顶部(分类导航上方)条件渲染 `{topWidget}`;其余 JSX/状态/逻辑逐行保持。

- [ ] **Step 2: RedTeamWikiWorkspace 改薄壳**

保留文件与默认导出名(ToolPage import 不变):`export default function RedTeamWikiWorkspace() { return <WikiWorkspace categories={WIKI_CATEGORIES} categoryById={wikiCategoryById} adapter={{ list: listWikiEntries, create: createWikiEntry, update: updateWikiEntry, delete: deleteWikiEntry }} /> }`

- [ ] **Step 3: ReverseWikiWorkspace 薄壳**

同款结构传逆向分类与 lib/reverseWiki 适配器(topWidget 留到 Task 5)。

- [ ] **Step 4: 三处接线**

- `categories.ts` reverse 节点 children 末尾追加:`{ id: 'reverse.wiki', name: '逆向 Wiki', icon: 'NotebookPen', description: '逆向知识库:函数速查、进制 ASCII、套路笔记' }`
- `tools.ts` 追加:`{ id: 'reverse.wiki.reverse-wiki', name: '逆向 Wiki', categoryId: 'reverse.wiki', description: '逆向知识库:常用函数、进制转换、特征识别速查与个人笔记', tags: ['逆向', 'Wiki', 'local'], workspace: 'reverse-wiki' }`
- `ToolPage.tsx` import ReverseWikiWorkspace,分发链 `redteam-wiki` 分支后加 `: tool.workspace === 'reverse-wiki' ? <ReverseWikiWorkspace />`

- [ ] **Step 5: build + 红队回归 + 逆向冒烟**

Run: `cd frontend && npm run build` → exit 0。
手动(`npm run dev`):红队 Wiki 五动作(切分类/搜索/新建/编辑/删除)行为如旧;逆向模式 → 逆向 Wiki 工具打开,8 分类 35 条在,新建一条刷新后仍在(localStorage)。

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/wiki/ frontend/src/pages/ToolPage.tsx frontend/src/mock/categories.ts frontend/src/mock/tools.ts
git commit -m "feat: WikiWorkspace 通用化抽取,逆向 Wiki 工作区上线(红队侧零行为变化)

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 5: AsciiConverter 即时查询条 + 挂载

**Files:**
- Create: `frontend/src/components/wiki/AsciiConverter.tsx`
- Modify: `frontend/src/components/wiki/ReverseWikiWorkspace.tsx`(挂 topWidget)

**Interfaces:**
- Consumes: Task 4 的 `topWidget` prop。
- Produces: `AsciiConverter` 默认导出;纯函数 `parseAsciiInput(input: string): { kind: 'hex' | 'dec' | 'text'; codepoints: number[] }`(空输入→kind 'text' 空数组)与 `formatCodepoint(n: number): { hex: string; dec: number; char: string; url: string }`(内部使用,导出以便复用)。

- [ ] **Step 1: 实现 AsciiConverter**

单框 + 批量两模式(Tab 切换)。单框规则(spec 裁定):`0x`/`0X` 前缀→十六进制码点;纯 ASCII 数字→十进制码点;其他→原文字符串逐字符。批量模式显式方向开关:「数字串→字符串」(空格/逗号分隔,按 0x 前缀或十进制逐 token)与「字符串→数字串」(输出 hex 串,如 `flag`→`66 6c 61 67`)。输出含 hex/dec/char/URL(%64)四种;下方常驻一行规则说明(含「要按字符转纯数字请用批量字符串方向」提示)。码点范围 0–0x10FFFF,越界/非法 token 显示「超出范围/无法解析」提示,不抛错不崩溃。

- [ ] **Step 2: 挂载**

ReverseWikiWorkspace 传 `topWidget={<AsciiConverter />}`。

- [ ] **Step 3: build + 手动用例**

Run: `cd frontend && npm run build` → exit 0。
手动清单(spec 钉死):`0x64`→`d (100, %64)`;`100`→`d`;`d`→`0x64 / 100 / %64`;`beef`→按字符串逐字符(非 hex);`7`→十进制码点 BEL(提示指引批量模式);批量 `64 65 66`→`def`;批量反向 `flag`→`66 6c 61 67`;`0x110000`→越界提示;空输入→空态不报错;中文`好`→`0x597d / %E5%A5%BD`。

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/wiki/AsciiConverter.tsx frontend/src/components/wiki/ReverseWikiWorkspace.tsx
git commit -m "feat: 逆向 Wiki 顶部进制↔ASCII 即时查询条(批量双向+歧义规则)

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 6: wailsjs 再生成 + 新 exe + README + 终验

**Files:**
- Modify: `frontend/wailsjs/go/main/App.d.ts`、`App.js`(wails 生成,不手写)
- Modify: `README.md`(工具清单与功能总览各补一句)
- Build: `build/bin/CTF-NET.exe`

**Interfaces:**
- Consumes: Task 2 的四绑定(生成 wailsjs);全部前序任务的产物。
- Produces: 可发版的 exe 与文档。

- [ ] **Step 1: wailsjs 再生成并断言**

Run: `wails generate module`(若无此子命令则 `wails build` 会顺带生成)
断言:`grep -c "ReverseWikiEntries\|ReverseWikiEntry" frontend/wailsjs/go/main/App.d.ts` ≥ 4。

- [ ] **Step 2: README 两处**

功能总览补「逆向 Wiki(逆向模式:8 分类速查 + 进制 ASCII 即时查询 + 个人笔记)」;后端工具一览表 `reverse` 行或紧邻处补一行说明(工作区型,不占注册表)。

- [ ] **Step 3: wails build 出 exe**

Run: `wails build`(后台,约 1-2 分钟)→ `ls -lh build/bin/CTF-NET.exe` 时间戳为当下。
桌面端手动终验(交用户或下次运行时):首次启动后 `data/reverse_wiki.db` 生成;逆向 Wiki 35 条在;增删改落库(重启仍在);红队 wiki.db 数据未受影响。

- [ ] **Step 4: Commit**

```bash
git add frontend/wailsjs README.md
git commit -m "feat: 逆向 Wiki wailsjs 绑定再生与 README 同步

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```
(dist 产物若在 wails build 中更新且 git 跟踪,一并 add。)
