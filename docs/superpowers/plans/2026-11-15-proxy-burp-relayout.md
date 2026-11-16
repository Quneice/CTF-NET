# 代理拦截 Burp 式布局改版 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把单页 ProxyWorkspace 拆为 3 个 Burp 同构工具页(代理拦截/流量历史/重放器),共享 useProxyStore,头部整块隐藏。

**Architecture:** 新 zustand store 持有全部代理状态(状态轮询按挂载引用计数常驻);三个无 props 工作区组件分别渲染;tools.ts 由 1 行拆 3 行;ToolPage 按 `workspace` 值分发并全部走 bareWorkspace(头部隐藏,poc-scan 先例)。后端零改动。

**Tech Stack:** React 18 + zustand + Tailwind + lucide-react(既有依赖,零新增)。

**Spec:** `docs/superpowers/specs/2026-11-15-proxy-burp-relayout-design.md`

## Global Constraints

- 后端零改动:不碰 `proxy_api.go`、`internal/proxy/`、`lib/proxy.ts`。
- 不新增 npm 依赖;不引入前端测试框架(仓库无前端测试基建)。
- 每任务验证门禁:`cd frontend && npm run build`(tsc 类型检查 + vite 构建)必须通过。
- 工具 id `pentest.proxy.intercept` 保持不变;新增 `pentest.proxy.history`、`pentest.proxy.repeater`。
- workspace 字符串值:`'proxy-interceptor'` / `'proxy-history'` / `'proxy-repeater'`(与既有 `'proxy-interceptor'` 一致风格)。
- UI 文案中文,风格与现有一致(按钮大小 text-xs、配色 bg-brand 等)。
- 非 Wails 环境(纯浏览器 dev)三页都要显示现有「需要桌面运行时」提示块,不崩溃。
- git 提交遵循仓库惯例 `feat: ...` 中文消息,结尾加 Co-Authored-By: Claude Code <noreply@anthropic.com>。

## Review Focus

(前端无测试基建,每行对应的验证落在所属任务的手动验收/构建门禁上)

1. **轮询泄漏**:三个代理标签全关后 2s 轮询必须停(引用计数归零 clearInterval)→ Task 1 代码要点 + Task 6 验收项。
2. **跨页拦截**:拦截事件到达时用户在历史/重放器页——挂起报文必须留在 store,打开拦截页即见(不能存组件 state)→ Task 1 store 持有 pending + Task 6 验收项。
3. **发送到重放器导航**:历史页点按钮必须打开/激活重放器标签并选中新数字标签(路由→标签同步依赖 ToolTabsWorkspace 既有 openTool)→ Task 3/4 接口 + Task 6 验收项。
4. **keep-alive 多页并存**:三页同时开着,轮询只有一份(refcount=3);StrictMode 双挂载 acquire/release 配对 → Task 1 引用计数实现 + Task 6 验收项。
5. **非 Wails 守卫**:纯浏览器 `npm run dev` 打开三页任一都显示守卫提示,无白屏 → Task 2/3/4 各自守卫块 + Task 6 验收项。

---

### Task 1: useProxyStore(状态 + 引用计数轮询 + 全部动作)

**Files:**
- Create: `frontend/src/stores/useProxyStore.ts`

**Interfaces:**
- Consumes: `@/lib/proxy` 全部门面与类型(`ProxyStatus/ProxySummary/ProxyRecord/BrowserInfo`、`proxyStatus/proxyHistory/proxyGetRecord/proxyDecide/proxyReplay/proxyStart/proxyStop/proxySetIntercept/proxySetMitm/proxyHistoryClear/browserAvailable/browserLaunch/onProxyIntercept`、`PROXY_FORWARD/PROXY_DROP/PROXY_MODIFIED`)。
- Produces(后续任务全部依赖,签名精确):

```ts
export interface RepeaterTab {
  id: number;      // 从 1 递增(Burp 数字标签)
  req: string;
  resp: string;
  meta: string;    // 「状态 · 耗时 · note」一行
  busy: boolean;
}

export const useProxyStore = create<ProxyState>()(...);
// state:status: ProxyStatus | null;history: ProxySummary[];pending: ProxyRecord | null;
//   pendingRaw: string;pendingResult: string;selected: ProxyRecord | null;
//   repeaterTabs: RepeaterTab[];activeRepeaterId: number | null;browserInfo: BrowserInfo | null;
//   flash: { kind: 'ok' | 'err'; text: string } | null;busy: boolean
// actions:
//   acquirePolling(): void;releasePolling(): void;   // 引用计数,0→1 启动/1→0 停止
//   refresh(): Promise<void>;                        // status + running 时拉 history(0,100)
//   flashOnce(kind: 'ok' | 'err', text: string): void; // 4s 自动清空
//   start(portStr: string): Promise<void>;stop(): Promise<void>;  // start 内 Number() 转换并校验 1-65535 整数
//   toggleIntercept(): Promise<void>;toggleMitm(): Promise<void>;
//   decide(action: 'forward' | 'drop' | 'modified'): Promise<void>;  // modified 用 pendingRaw
//   setPendingRaw(raw: string): void;
//   openRecord(id: string): Promise<void>;           // proxyGetRecord → selected
//   clearHistory(): Promise<void>;
//   sendToRepeater(raw: string): number;             // 追加标签(id 自增)、激活、返回 id
//   setActiveRepeater(id: number): void;
//   updateRepeaterReq(id: number, raw: string): void;
//   closeRepeaterTab(id: number): void;              // 关激活标签则激活相邻
//   replay(id: number): Promise<void>;               // busy 置位→proxyReplay→写回 resp/meta
//   loadBrowserInfo(): Promise<void>;launchBrowser(url: string, portHint: number): Promise<void>;
```

- [ ] **Step 1: 创建 `useProxyStore.ts`**

实现要点(签名已定,算法在这里):
- 轮询计数为**模块级变量**(`let refCount = 0; let timer: number | undefined; let offIntercept: (() => void) | null = null`),不进 state——`acquirePolling` 里 `refCount++; if (refCount === 1) { void refresh(); timer = window.setInterval(() => void refresh(), 2000); offIntercept = onProxyIntercept((item) => { void (async () => { try { const rec = await proxyGetRecord(item.id); set({ pending: rec, pendingRaw: rec.raw_request, pendingResult: '' }); } catch { /* 轮询兜底 */ } })(); }); }`;`releasePolling` 对称清理(clearInterval + offIntercept + 置 null),且 `refCount = Math.max(0, refCount - 1)`。
- `decide` 搬运现 `ProxyWorkspace.onDecide` 逻辑:`proxyDecide(pending.id, action, action === PROXY_MODIFIED ? pendingRaw : '')` → 置 pendingResult(`已丢弃…`/`已放行,等待响应…`)→ `waitRecord` 轮询(≤10 次 × 500ms,搬现文件 `waitRecord` 函数)回写终态 → `pending: null` + `refresh()`。
- `refresh` 搬现逻辑:`proxyStatus()` 成功→ set status,running 则 `proxyHistory(0,100)`→ set history;异常→ `status: null`。
- `sendToRepeater`:`nextId`(模块级,初始 0)自增;push `{ id, req: raw, resp: '', meta: '', busy: false }`;set `activeRepeaterId: id`;返回 id。
- `closeRepeaterTab`:过滤后若关的是激活标签,激活相邻(索引 min(旧 idx, len-1)),空则 null。
- `start/stop/toggle*/launchBrowser/clearHistory` 搬现组件逻辑(flash + refresh,pending 清理);`stop` 时 `pending: null`。
- `flashOnce`:set flash + `window.setTimeout(() => set({ flash: null }), 4000)`。
- 文件头注释中文,说明引用计数语义(keep-alive 多页共享一份轮询)。

- [ ] **Step 2: 构建门禁**

Run: `cd frontend && npm run build`
Expected: tsc 零错误,构建成功(新文件无消费者也能编译)。

- [ ] **Step 3: Commit**

```bash
git add frontend/src/stores/useProxyStore.ts
git commit -m "feat: 代理工作台共享状态 useProxyStore(引用计数轮询+重放器标签)"
```

---

### Task 2: ProxyInterceptWorkspace(控制栏 + 挂起请求处理)

**Files:**
- Create: `frontend/src/components/proxy/ProxyInterceptWorkspace.tsx`

**Interfaces:**
- Consumes: Task 1 `useProxyStore` 全部;`isWails`(lib/backend)。
- Produces: `export default function ProxyInterceptWorkspace(): JSX.Element`(无 props,Task 5 接线)。

- [ ] **Step 1: 创建组件**

实现要点:
- 开头非 Wails 守卫(照抄现 `ProxyWorkspace` 的 AlertTriangle 提示块,文案不变)。
- `useEffect(() => { acquirePolling(); void loadBrowserInfo(); return releasePolling; }, [])`。
- 结构 = 现文件的控制栏(327-407 行)+ MITM 控制行(409-445 行)+ msg 条(447-457 行)+ 拦截区(467-525 行,去掉 tab 切换),状态从 store 取,本地 state 仅 `portInput/browserUrl/caExportDir` 三个输入。
- 动作直接调 store;端口校验(整数、1-65535、NaN 拒绝)放 `start` 内,组件把 `portInput` 原样传入(`start` 接收 `string` 参数内部 `Number()` 转换并校验)——故 Task 1 中 `start` 签名为 `start(portStr: string): Promise<void>`。
- 挂起区尾部追加一个「发送到重放器」按钮:`onClick={() => { const id = sendToRepeater(pendingRaw); navigate(`/t/pentest.proxy.repeater`); }}`(useNavigate;路由会自动开标签)。

- [ ] **Step 2: 构建门禁**

Run: `cd frontend && npm run build`
Expected: PASS(未被引用,仅编译)。

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/proxy/ProxyInterceptWorkspace.tsx
git commit -m "feat: 代理拦截独立工作区(控制栏+MITM+挂起请求,store 驱动)"
```

---

### Task 3: ProxyMiniStatus + ProxyHistoryWorkspace(全宽历史 + 详情 + 发送到重放器)

**Files:**
- Create: `frontend/src/components/proxy/ProxyMiniStatus.tsx`
- Create: `frontend/src/components/proxy/ProxyHistoryWorkspace.tsx`

**Interfaces:**
- Consumes: Task 1 store;`statusColor` 色函数(从旧 ProxyWorkspace 搬到本文件或 MiniStatus 文件,导出供 Task 4 复用——放 `ProxyMiniStatus.tsx` 导出 `statusColor`)。
- Produces:
  - `export default function ProxyMiniStatus(): JSX.Element` —— 右上角一行:状态点(绿/灰)+ `运行中 127.0.0.1:{port}` 或 `未启动`;`pending` 非空时加琥珀色 `⏳ {n} 挂起`;整行是 `<Link to="/t/pentest.proxy.intercept">` 包裹(title「前往代理拦截页控制」)。未启动时整行仍可点(去启动)。
  - `export function statusColor(status: number): string`(现文件 34-39 行原样搬)。
  - `export default function ProxyHistoryWorkspace(): JSX.Element`。

- [ ] **Step 1: 创建 ProxyMiniStatus.tsx**(如上签名,风格 text-xs text-slate-400)

- [ ] **Step 2: 创建 ProxyHistoryWorkspace.tsx**

实现要点:
- 非 Wails 守卫块(同 Task 2)。
- `useEffect` acquire/releasePolling。
- 顶部一行:`<ProxyMiniStatus />` + 右侧「清空历史」(Trash2 图标按钮,调 store.clearHistory)+「刷新」按钮。
- 表格:现历史表(532-577 行)改**全宽**(去掉 lg:grid-cols-5 分栏),列不变(#/方法/状态/主机路径/耗时),行点击 `void openRecord(item.id)`。
- 选中详情在表格**下方**(现 580-609 行的详情块):元信息行 + 「发送到重放器」按钮(`const id = sendToRepeater(selected.raw_request); navigate('/t/pentest.proxy.repeater')`,Send 图标)+ 请求/响应两个 pre(max-h-64)。
- 空态文案照抄现版。

- [ ] **Step 3: 构建门禁**

Run: `cd frontend && npm run build` → PASS

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/proxy/ProxyMiniStatus.tsx frontend/src/components/proxy/ProxyHistoryWorkspace.tsx
git commit -m "feat: 流量历史独立工作区(全宽表格+详情+发送到重放器+迷你状态)"
```

---

### Task 4: ProxyRepeaterWorkspace(Burp 数字标签)

**Files:**
- Create: `frontend/src/components/proxy/ProxyRepeaterWorkspace.tsx`

**Interfaces:**
- Consumes: Task 1 store(repeaterTabs/activeRepeaterId/setActiveRepeater/updateRepeaterReq/closeRepeaterTab/replay/sendToRepeater);Task 3 `ProxyMiniStatus`。
- Produces: `export default function ProxyRepeaterWorkspace(): JSX.Element`。

- [ ] **Step 1: 创建组件**

实现要点:
- 非 Wails 守卫块;acquire/releasePolling。
- 顶部:`<ProxyMiniStatus />`;标签行:每个 tab 一个按钮「{id}」(激活态 bg-brand text-white,与现 tabBtn 风格)+ 关闭 ✕(阻止冒泡调 closeRepeaterTab);末尾「+」按钮 = `sendToRepeater('')` 空白标签。
- 主体取 `activeRepeaterId` 对应 tab:请求 textarea(值 `tab.req`,onChange `updateRepeaterReq(tab.id, v)`,h-64)+「发送」按钮(`replay(tab.id)`,busy 时禁用)+ meta 行 + 响应 pre。
- 空态(无标签):虚线框提示「暂无重放标签——从「流量历史」或「代理拦截」发送请求,或点 + 新建」。

- [ ] **Step 2: 构建门禁**

Run: `cd frontend && npm run build` → PASS

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/proxy/ProxyRepeaterWorkspace.tsx
git commit -m "feat: 重放器独立工作区(Burp 式数字多标签)"
```

---

### Task 5: 接线切换(tools.ts 拆 3 行 + ToolPage 分发 + 删旧组件)

**Files:**
- Modify: `frontend/src/mock/tools.ts:5133-5136`(proxyShorthandRows)
- Modify: `frontend/src/pages/ToolPage.tsx:16,81,85,148-149`
- Delete: `frontend/src/components/proxy/ProxyWorkspace.tsx`

**Interfaces:**
- Consumes: Task 2/3/4 三个默认导出组件;workspace 值 Global Constraints 所列。
- Produces: 路由 `/t/pentest.proxy.intercept|history|repeater` 可用;三页头部全隐。

- [ ] **Step 1: 改 proxyShorthandRows 为 3 行**

```ts
const proxyShorthandRows: ShorthandRow[] = [
  { id: 'pentest.proxy.intercept', name: '代理拦截', categoryId: 'pentest.proxy', description: 'Burp 风格 HTTP 代理:监听/拦截开关/HTTPS 解密(MITM)/CA 管理/内置浏览器,挂起请求可修改后放行或丢弃', tags: ['代理', '拦截', 'task:利用'], preset: 'text', workspace: 'proxy-interceptor' },
  { id: 'pentest.proxy.history', name: '流量历史', categoryId: 'pentest.proxy', description: '代理捕获的 HTTP/HTTPS 流量历史:全宽清单浏览,点选查看原始请求/响应,可发送到重放器', tags: ['代理', '历史', 'task:利用'], preset: 'text', workspace: 'proxy-history' },
  { id: 'pentest.proxy.repeater', name: '重放器', categoryId: 'pentest.proxy', description: '请求重放(Burp Repeater):数字多标签管理请求报文,编辑后重发查看响应', tags: ['代理', '重放', 'task:利用'], preset: 'text', workspace: 'proxy-repeater' },
];
```

- [ ] **Step 2: ToolPage 分发与 bareWorkspace**

- import 三个新组件替换 `ProxyWorkspace`;
- `const bareWorkspace = ['poc-scan', 'proxy-interceptor', 'proxy-history', 'proxy-repeater'].includes(tool.workspace ?? '');`
- 分发改为三元链:`proxy-interceptor` → Intercept;`proxy-history` → History;`proxy-repeater` → Repeater。

- [ ] **Step 3: 删除 `ProxyWorkspace.tsx`**(`git rm`)

- [ ] **Step 4: 构建门禁**

Run: `cd frontend && npm run build`
Expected: PASS,无 ProxyWorkspace 残留引用(grep 确认 0 命中)。

- [ ] **Step 5: Commit**

```bash
git add frontend/src/mock/tools.ts frontend/src/pages/ToolPage.tsx frontend/src/components/proxy/
git commit -m "feat: 代理拦截拆分三工具页接线(头部全隐,Burp 布局)"
```

---

### Task 6: 手动验收 + 文档(README/批 20 报告)+ dist 重建

**Files:**
- Modify: `README.md`(功能总览「HTTP 代理拦截与重放」条目:一工具→三页布局描述)
- Create: `docs/batch-reports/020-proxy-burp-relayout.md`
- Rebuild: `frontend/dist/`(main.go go:embed 消费,仓库提交 dist)

- [ ] **Step 1: `cd frontend && npm run dev` 纯浏览器跑 Review Focus 5 项守卫**(三页均显示「需要桌面运行时」,无白屏/控制台报错)

- [ ] **Step 2: `wails dev` 手动验收清单**(逐一记录进批 20 报告)

1. 分类树 `pentest.proxy` 下见 3 工具;三页头部(面包屑/名称/徽章/描述/tags)全部消失。
2. 代理拦截页启动代理 → 内置浏览器访问网站 → 流量历史页表格实时增长(2s 一刷)。
3. 历史页点行 → 下方详情;「发送到重放器」→ 自动切到重放器页且新数字标签激活、报文已填。
4. 重放器页改报文点发送 → 响应与 meta 回显;开 2 个标签互不串扰;关激活标签回落相邻。
5. 拦截开关开 → 从历史/重放器页发起请求 → 该页顶部 MiniStatus 出现「⏳1 挂起」→ 点它跳到拦截页见挂起报文 → 放行/修改放行/丢弃三种各验一次;拦截页「发送到重放器」同样可用。
6. MITM 开 + 内置浏览器访问 https 站 → 历史里出现解密流量。
7. 关闭全部三个代理标签 → 重开流量历史 → 历史仍每 2s 单次刷新(无重复轮询:DevTools Network 或行为观察无双倍请求)。
8. keep-alive:三标签同时开,来回切换各页输入/选中状态保留。

- [ ] **Step 3: 更新 README 代理条目 + 写批 20 报告**(格式对齐 019:改动文件表/验收清单逐项结果)

- [ ] **Step 4: 重建 dist 并提交**

Run: `cd frontend && npm run build`,然后 `git add README.md docs/batch-reports/020-proxy-burp-relayout.md frontend/dist`
```bash
git commit -m "docs: 代理 Burp 布局改版批 20 报告与 README 更新"
```
