# 流量分析模式与逆向模式(方案 A·纯模式壳)Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 CTF-NET 前端新增「流量分析模式」与「逆向模式」两个工作台模式:22 个流量工具从 `forensics.network` 迁入新 `traffic.*` 分类树,逆向模式复用现有 `reverse` 分类,模式选择页扩为 5 卡片。

**Architecture:** 纯前端数据层改动。模式系统单一数据源在 `frontend/src/mock/modes.ts`(`MODE_CONFIG.categoryRoots`),分类树在 `mock/categories.ts`,工具归属在 `mock/tools.ts` 的 `categoryId` 字段;路由 `/c/:categoryId`、`/t/:toolId` 全泛化,仅需为被删分类加 1 条重定向。不碰任何 Go 代码。

**Tech Stack:** React + TypeScript + Vite(既有);无新增依赖、无测试框架(验证 = `npm run build` + grep 数据断言 + 手动过一遍)。

**Spec:** `docs/superpowers/specs/2026-11-14-traffic-reverse-modes-design.md`

## Global Constraints

- 工具 id 一律不变(保 `/t/:toolId` URL);只改 `categoryId`。
- 不改 Go 后端、不生成 wailsjs、不新增 npm 依赖、不引入测试框架。
- 迁移类注释沿用 `categories.ts` 既有「批次注释」风格(说明迁移动作)。
- 提交信息:中文 `feat:`/`docs:` 前缀,末尾加 `Co-Authored-By: Claude Code <noreply@anthropic.com>`;只 `git add` 本计划涉及的文件(工作区有大量他人未提交改动,严禁 `git add -A`)。
- 当前工作目录基准:仓库根 `C:\Users\HP\Desktop\CTF-NET`;前端命令在 `frontend/` 下执行。

## Review Focus

1. **工具 id 被顺手改名** → `/t/forensics.network.pcap-analyze` 等旧 URL 404。断言:迁移后 `grep -c "id: 'forensics.network\."` 仍 = 22(Task 2 Step 4)。
2. **22 个 categoryId 漏迁或迁错组**(计数 3+12+4+3)。断言:四组各自 grep 计数 + `categoryId: 'forensics.network'` 归零(Task 2 Step 4)。
3. **旧 URL `/c/forensics.network` 404**(节点删除后)。断言:重定向路由存在且 build 过(Task 2 Step 3/4);手动访问验证(Task 3 Step 6)。
4. **CTF 模式漏挂 `traffic` 根**,流量工具在 CTF 模式消失。断言:grep ctf categoryRoots 含 `'traffic'`(Task 3 Step 3)。
5. **新模式卡片图标未映射**导致渲染异常。已核实 `Radio`/`FileCode2` 在 `lib/icons.tsx`;build 类型检查 + 手动 5 卡片过一遍(Task 3 Step 6)。

---

### Task 1: 新增 traffic 分类树

**Files:**
- Modify: `frontend/src/mock/categories.ts`(文件末尾 aisc 块之后追加)

**Interfaces:**
- Consumes: 无。
- Produces: 顶级分类节点 `traffic` 及 4 个子分类 id(Task 2 的 categoryId 目标值):`traffic.parse` / `traffic.protocol` / `traffic.fingerprint` / `traffic.assist`。图标全部取自已映射集合(Radio/FileSearch/Network/Search/Wrench)。

- [ ] **Step 1: 在 `categories.ts` 末尾 aisec 块后追加 traffic 节点**

```ts
  // 流量分析域：原 forensics.network 二级 segment 独立成顶级分类
  //（流量分析模式可见；CTF 模式共享该根——流量题本是取证子域）。
  // 22 个工具由 forensics.network 迁入（工具 id 不变，仅 categoryId 迁移）。
  {
    id: 'traffic',
    name: '流量分析',
    icon: 'Radio',
    description: '流量取证与协议分析',
    children: [
      { id: 'traffic.parse', name: '流量解析', icon: 'FileSearch', description: 'PCAP 文件与原始流量解析' },
      { id: 'traffic.protocol', name: '协议解码', icon: 'Network', description: 'TCP/UDP/DNS/ICMP 等协议字段解码' },
      { id: 'traffic.fingerprint', name: '流量指纹', icon: 'Search', description: 'JA3/HASSH/User-Agent 等指纹生成' },
      { id: 'traffic.assist', name: '辅助处理', icon: 'Wrench', description: 'IP 段聚合、MAC 格式化、Defang 等' },
    ],
  },
```

注意:子分类**不加** `segment: true`(segment 是「纯分组无直挂工具」的降级形态;此处工具直挂子分类,与 `reverse.static` 同构)。本任务**不删** `forensics.network` 节点(删除与工具迁移在 Task 2 原子完成)。

- [ ] **Step 2: build 验证**

Run: `cd frontend && npm run build`
Expected: exit 0(tsc + vite 全过;新节点暂无模式引用,无行为变化)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/mock/categories.ts
git commit -m "feat: 新增 traffic 流量分析顶级分类树(4 子分类,工具迁移见后续提交)

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 2: 22 个工具迁入 traffic.* + 删旧 segment + 旧 URL 重定向

**Files:**
- Modify: `frontend/src/mock/tools.ts`(22 行,行号约 4245-4471 区间,以 grep 为准)
- Modify: `frontend/src/mock/categories.ts`(删 1 行)
- Modify: `frontend/src/router/index.tsx`(加 1 条重定向)

**Interfaces:**
- Consumes: Task 1 的 `traffic.parse/protocol/fingerprint/assist` 四个 id。
- Produces: 22 个工具新归属;`forensics.network` 分类不复存在;`/c/forensics.network` → `/c/traffic`。

- [ ] **Step 1: 按 id 定位逐条迁移 categoryId(bash 循环,sed 只动命中行)**

```bash
cd frontend/src/mock
for pair in \
  parse:pcap-analyze parse:strip-headers parse:dechunk \
  protocol:tcp protocol:udp protocol:ipv4-header protocol:ipv6 protocol:icmp \
  protocol:dns-extract protocol:doh protocol:http-request protocol:netbios \
  protocol:protobuf protocol:varint protocol:parse-uri \
  fingerprint:ja3 fingerprint:hassh fingerprint:user-agent fingerprint:mac-format \
  assist:ip-range assist:group-ip assist:defang ; do
  grp="${pair%%:*}"; t="${pair##*:}"
  sed -i "/id: 'forensics.network.$t'/s/categoryId: 'forensics.network'/categoryId: 'traffic.$grp'/" tools.ts
done
```

(仅改 `categoryId` 值;`id` 字段在同一行但 sed 右侧只替换 categoryId 片段,id 不动。)

- [ ] **Step 2: 删除 `categories.ts:137` 的 forensics.network 行,并在 forensics 批 7 注释块补一句迁移说明**

删除行:`{ id: 'forensics.network', name: '流量分析', icon: 'Radio', segment: true },`
批 7 注释(第 132-134 行)中「全部/内存取证/磁盘取证/流量分析/日志分析」「54 个工具」两处措辞改为 3 组(内存/磁盘/日志)且注明流量分析已独立为顶级 `traffic`(22 工具迁出)。

- [ ] **Step 3: `router/index.tsx` 加重定向(置于 `c/crypto.modern.rsa.coppersmith` 之后,同段惯例)并更新头部注释**

```tsx
      { path: 'c/forensics.network', element: <Navigate to="/c/traffic" replace /> },
```

头部「旧路由兼容」注释块追加一行:`- /c/forensics.network → /c/traffic(流量域独立顶级分类,22 工具迁入 traffic.*)`

- [ ] **Step 4: 数据断言 + build**

Run:
```bash
cd frontend
grep -c "categoryId: 'traffic.parse'" src/mock/tools.ts        # 期望 3
grep -c "categoryId: 'traffic.protocol'" src/mock/tools.ts     # 期望 12
grep -c "categoryId: 'traffic.fingerprint'" src/mock/tools.ts  # 期望 4
grep -c "categoryId: 'traffic.assist'" src/mock/tools.ts       # 期望 3
grep -c "categoryId: 'forensics.network'" src/mock/tools.ts    # 期望 0
grep -c "id: 'forensics.network\." src/mock/tools.ts           # 期望 22(id 未动)
grep -n "forensics.network" src/mock/categories.ts             # 期望无输出(仅注释可留)
npm run build                                                   # 期望 exit 0
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/mock/tools.ts frontend/src/mock/categories.ts frontend/src/router/index.tsx
git commit -m "feat: 22 个流量工具迁入 traffic.* 分类,删 forensics.network segment 并加重定向(id 不变)

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

---

### Task 3: 两模式配置 + CTF 挂 traffic 根 + README + 终验

**Files:**
- Modify: `frontend/src/mock/modes.ts`(类型、配置、ctf roots、头注释)
- Modify: `README.md`(功能总览第一条「三模式」措辞、架构注释「三模式分类树」)

**Interfaces:**
- Consumes: Task 1 的 `traffic` 根、既有 `reverse` 根、共享根 `codec`/`data`。
- Produces: `AppMode` 含 `'traffic' | 'reverse'`;`MODE_CONFIG` 5 条;模式选择页 5 卡片。

- [ ] **Step 1: `modes.ts` 改四处**

1. 头注释(第 8-10 行)三模式措辞 → 五模式,补「流量分析 / 逆向」。
2. 第 12 行:`export type AppMode = 'ctf' | 'pentest' | 'modelsec' | 'traffic' | 'reverse';`
3. `MODE_CONFIG` 追加(modelsec 之后,键序即卡片序):

```ts
  traffic: {
    label: '流量分析模式',
    icon: 'Radio',
    description: '流量取证:PCAP 解析、协议解码、流量指纹与提取导出,共享编码与数据处理工具',
    categoryRoots: ['traffic', 'codec', 'data'],
  },
  reverse: {
    label: '逆向模式',
    icon: 'FileCode2',
    description: '逆向工程:静态分析、动态调试、去混淆,共享编码与数据处理工具',
    categoryRoots: ['reverse', 'codec', 'data'],
  },
```

4. ctf 的 `categoryRoots` 末尾(`'data'` 之后)追加 `'traffic'`。

- [ ] **Step 2: README 两处**

- 功能总览第一条:「三模式工作台」→「五模式工作台」,句末补「与「流量分析模式」(流量解析/协议解码/流量指纹)、「逆向模式」(静态/动态/去混淆)」。
- 架构注释 `src/mock/` 行:「三模式分类树」→「五模式分类树」。

- [ ] **Step 3: 数据断言 + build**

Run:
```bash
cd frontend
grep -n "export type AppMode" src/mock/modes.ts                       # 含 'traffic' | 'reverse'
grep -c "label: '" src/mock/modes.ts                                  # 期望 5
grep -n "'data', *$" src/mock/modes.ts | tail -1                      # ctf roots 末行后应见 'traffic'
npm run build                                                         # exit 0
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/mock/modes.ts README.md
git commit -m "feat: 新增流量分析/逆向双工作台模式,CTF 模式共享 traffic 根,README 同步

Co-Authored-By: Claude Code <noreply@anthropic.com>"
```

- [ ] **Step 5: 手动终验(交用户或跑 dev 自查,清单)**

`cd frontend && npm run dev` 浏览器打开(无 Wails 环境自动走 mock):
1. `/mode` 出现 5 张卡片,顺序 ctf→pentest→modelsec→traffic→reverse,图标正常。
2. 进「流量分析模式」:侧边栏 流量分析(+编码转换+数据处理);4 子分类工具数 3/12/4/3;点开任一工具(如 PCAP 流量分析)显示占位提示表单。
3. 地址栏直输 `/c/forensics.network` → 跳转 `/c/traffic`;`/t/forensics.network.ja3` 直达工具页不 404。
4. 进「CTF 模式」:Forensics 顶部 chips 只剩 内存/磁盘/日志 3 组;侧边栏可见「流量分析」根。
5. 进「逆向模式」:三根 Reverse/编码转换/数据处理,静态/动态/去混淆 14 工具在。
6. 五个模式往返切换,无白屏、无 console 报错。
