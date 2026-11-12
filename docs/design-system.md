# CTF-NET 设计系统(阶段 2 产出,纯文档,未改任何页面)

> 适用技术栈:React 18 + TypeScript + Tailwind 3(class 暗色)+ zustand + react-router-dom 6 + lucide-react。
> 依据:`docs/audit-report.md` §6(UI 能力盘点)、`docs/ia.md`(页面/状态/占位策略)、现有 `index.css`/`ToolCard`/`AppLayout` 基线。
> 落地方式:CSS 变量(明暗两套)+ `tailwind.config.js` extend 语义别名;**新组件只准用语义令牌,旧原子类按批次迁移**。
> 所有颜色对均按 WCAG 相对亮度公式人工验算,见 §12 对比度表;批 0 落地时加入自动化对比度校验(与 `inventory:check` 同管线)。

---

## 1. 视觉方向(总则)

**专业、清爽、高信息密度但不拥挤;工具型产品优先可扫描性与一致性。**

- 信息层级:一级动作(搜索/提交)→ 工具元信息(名称/状态/分类)→ 描述文本,密度靠字号阶梯 + 间距克制实现,不靠缩小字号挤压。
- 一致性:任何页面只允许消费 §2 令牌;页面级私有样式视为缺陷。
- 状态优先:implemented/placeholder 徽章是全站最高频视觉元素之一,必须一眼可辨(§11)。
- 克制:阴影只表达层级,动效只表达状态变化(§8),渐变默认禁用(§14)。

---

## 2. 设计令牌

### 2.1 命名与分层

三层结构:原始值 → 语义令牌(§2.2~2.7)→ 组件映射(§9~11)。语义令牌前缀 `--ct-`,组件**只准引用语义令牌**(`bg-surface`、`text-ink-1`),禁止在组件里写死 hex/rgb。

### 2.2 颜色令牌(明暗两套,见 §2.8 落地代码)

| 令牌 | 角色 | Light | Dark | 用途 |
| --- | --- | --- | --- | --- |
| `--ct-brand` | 品牌/主交互 | `#4f46e5`(indigo-600) | `#818cf8`(indigo-400) | 链接、主按钮、激活态 |
| `--ct-brand-hover` | 主交互悬停 | `#4338ca`(indigo-700) | `#a5b4fc`(indigo-300) | hover/active |
| `--ct-brand-subtle` | 品牌淡底 | `#eef2ff`(indigo-50) | `rgb(99 102 241 / .15)` | 激活导航项底、选中 chip |
| `--ct-surface` | 页面底 | `#f8fafc`(slate-50) | `#020617`(slate-950) | body |
| `--ct-surface-raised` | 卡片/面板 | `#ffffff` | `#0f172a`(slate-900) | 卡片、输入框、表格 |
| `--ct-surface-hover` | 悬停面 | `#f1f5f9`(slate-100) | `rgb(148 163 184 / .08)` | 行 hover、ghost 按钮 hover |
| `--ct-line` | 常规分隔 | `#e2e8f0`(slate-200) | `#1e293b`(slate-800) | 边框、分隔线 |
| `--ct-line-strong` | 强调分隔 | `#cbd5e1`(slate-300) | `#334155`(slate-700) | 输入框 focus 前边框、表头线 |
| `--ct-ink-1` | 主文字 | `#0f172a`(slate-900) | `#f1f5f9`(slate-100) | 标题、正文 |
| `--ct-ink-2` | 次文字 | `#475569`(slate-600) | `#94a3b8`(slate-400) | 描述、辅助说明 |
| `--ct-ink-3` | 弱文字 | `#64748b`(slate-500) | `#64748b`(slate-500) | 元信息、时间戳;**暗色下仅限大字/图标**(§12) |
| `--ct-ink-inverse` | 反色文字 | `#ffffff` | `#0f172a` | 主按钮上文字 |

状态色四组(徽章专用,§11):`--ct-ok / --ct-ok-bg / --ct-ok-line`(已实现,绿)、`--ct-warn*`(占位,琥珀)、`--ct-danger*`(故障,红)、`--ct-muted*`(未知,灰)。完整值见 §2.8。

### 2.3 字体

| 令牌 | 值 | 用途 |
| --- | --- | --- |
| `--ct-font-sans` | `-apple-system, BlinkMacSystemFont, 'Segoe UI', 'Microsoft YaHei', 'PingFang SC', 'Noto Sans SC', sans-serif` | 全部 UI(补 CJK 回退,替换当前无显式声明的默认栈) |
| `--ct-font-mono` | `ui-monospace, SFMono-Regular, Menlo, Consolas, 'Cascadia Mono', monospace` | 代码/输出/路径/命令 |

### 2.4 字号与行高(沿用 Tailwind 原生刻度,不做自定义尺寸)

| 令牌 | 字号/行高 | 字重 | 用途 |
| --- | --- | --- | --- |
| `text-2xs`(新) | 11 / 16 | normal | 版本号、键盘提示(kbd) |
| `text-xs` | 12 / 16 | normal | 徽章、标签、元信息、面包屑 |
| `text-sm` | 14 / 20 | normal | **正文工作马**、卡片标题(medium)、输入框 |
| `text-base` | 16 / 24 | semibold | 区块标题、对话框标题 |
| `text-lg` | 18 / 28 | medium | 首页分类卡标题 |
| `text-xl` | 20 / 28 | semibold | 页面标题(分类页/搜索页) |
| `text-2xl` | 24 / 32 | semibold | 工具页标题 |
| mono 输出 | 13 / 20 | normal | 结果面板代码(mono,可配 `text-xs` 高密度) |

规则:CJK 行高 ≥ 1.4;标题一律 `tracking-normal`(中文不加字距);数字/代码用 mono + tabular-nums。

### 2.5 间距(4px 基准,沿用 Tailwind 刻度)

| 令牌 | 值 | 用途 |
| --- | --- | --- |
| `space-1` | 4 | 图标与文字间隙、chip 内聚 |
| `space-2` | 8 | 表单 label 与控件、徽章内间距 |
| `space-3` | 12 | 卡片内行距 |
| `space-4` | 16 | **卡片内边距、网格 gap、列表项间距** |
| `space-5` | 20 | 表单字段组 gap |
| `space-6` | 24 | **页面内边距(桌面)、区块间距** |
| `space-8` | 32 | 大区块分隔(工具页输入区/结果区) |
| `space-12` | 48 | 页面顶部标题区下边距 |

密度规则:同层级间距必须同值(如所有卡片 `p-4`、所有字段组 `space-y-5`);移动端页面内边距降为 `p-4`(§10)。

### 2.6 圆角 / 阴影 / 边框

| 令牌 | 值 | 用途 |
| --- | --- | --- |
| `rounded-md` | 6px | 输入框、按钮、chips |
| `rounded-lg` | 8px | **卡片、面板、工具页容器**(与现有 ToolCard 一致) |
| `rounded-xl` | 12px | 对话框、抽屉面板、命令面板 |
| `rounded-full` | 9999 | 状态徽章、标签 pill、图标按钮 |
| `shadow-card` | `0 1px 2px rgb(15 23 42 / .06)` | 卡片静置(明色;暗色禁用,§13) |
| `shadow-pop` | `0 4px 12px rgb(15 23 42 / .10)` | 下拉/悬停浮起(明色) |
| `shadow-modal` | `0 12px 32px rgb(15 23 42 / .16)` | 对话框/抽屉(明色) |
| `border` | 1px `--ct-line` | 卡片、输入框、分隔;1px 统一,不用 2px 边框 |

### 2.7 层级与动效

| 令牌 | 值 | 用途 |
| --- | --- | --- |
| `z-sticky` | 10 | 吸顶筛选栏、工具页头部 |
| `z-overlay` | 20 | 下拉菜单、popover、tooltip |
| `z-modal` | 30 | 对话框 |
| `z-drawer` | 40 | 移动端抽屉(与现有 AppLayout 一致) |
| `z-top` | 50 | 命令面板、toast |
| `duration-fast` | 100ms | 颜色/背景 hover、focus ring |
| `duration-base` | 150ms | 徽章、tooltip、chip 出入 |
| `duration-slow` | 200ms | 侧边栏折叠(现有 200ms)、抽屉滑入、面板展开 |
| easing | `ease-out`(进入)/ `ease-in`(退出) | 全部过渡;禁止 ease-in-out 弹跳感 |

**上限 200ms(规则 10)**;`prefers-reduced-motion: reduce` 时全部时长 → 0.01ms(§12)。

### 2.8 落地代码(批 0 写入,本文档先行给出)

`frontend/src/index.css` 追加:

```css
:root {
  --ct-brand:#4f46e5; --ct-brand-hover:#4338ca; --ct-brand-subtle:#eef2ff;
  --ct-surface:#f8fafc; --ct-surface-raised:#ffffff; --ct-surface-hover:#f1f5f9;
  --ct-line:#e2e8f0; --ct-line-strong:#cbd5e1;
  --ct-ink-1:#0f172a; --ct-ink-2:#475569; --ct-ink-3:#64748b; --ct-ink-inverse:#ffffff;
  --ct-ok:#166534;      --ct-ok-bg:#dcfce7;      --ct-ok-line:#bbf7d0;
  --ct-warn:#92400e;    --ct-warn-bg:#fef3c7;    --ct-warn-line:#fde68a;
  --ct-danger:#991b1b;  --ct-danger-bg:#fee2e2;  --ct-danger-line:#fecaca;
  --ct-muted:#334155;   --ct-muted-bg:#e2e8f0;   --ct-muted-line:#cbd5e1;
}
.dark {
  --ct-brand:#818cf8; --ct-brand-hover:#a5b4fc; --ct-brand-subtle:rgb(99 102 241 / .15);
  --ct-surface:#020617; --ct-surface-raised:#0f172a; --ct-surface-hover:rgb(148 163 184 / .08);
  --ct-line:#1e293b; --ct-line-strong:#334155;
  --ct-ink-1:#f1f5f9; --ct-ink-2:#94a3b8; --ct-ink-3:#64748b; --ct-ink-inverse:#0f172a;
  --ct-ok:#86efac;      --ct-ok-bg:rgb(22 101 52 / .35);  --ct-ok-line:rgb(22 163 74 / .4);
  --ct-warn:#fcd34d;    --ct-warn-bg:rgb(146 64 14 / .35); --ct-warn-line:rgb(217 119 6 / .4);
  --ct-danger:#fca5a5;  --ct-danger-bg:rgb(153 27 27 / .35); --ct-danger-line:rgb(220 38 38 / .4);
  --ct-muted:#cbd5e1;   --ct-muted-bg:rgb(51 65 85 / .5);   --ct-muted-line:#475569;
}
```

`frontend/tailwind.config.js` extend 追加:

```js
extend: {
  colors: {
    brand: { DEFAULT:'var(--ct-brand)', hover:'var(--ct-brand-hover)', subtle:'var(--ct-brand-subtle)' },
    surface: { DEFAULT:'var(--ct-surface)', raised:'var(--ct-surface-raised)', hover:'var(--ct-surface-hover)' },
    line: { DEFAULT:'var(--ct-line)', strong:'var(--ct-line-strong)' },
    ink: { 1:'var(--ct-ink-1)', 2:'var(--ct-ink-2)', 3:'var(--ct-ink-3)', inverse:'var(--ct-ink-inverse)' },
    status: { ok:'var(--ct-ok)', okBg:'var(--ct-ok-bg)', okLine:'var(--ct-ok-line)',
              warn:'var(--ct-warn)', warnBg:'var(--ct-warn-bg)', warnLine:'var(--ct-warn-line)',
              danger:'var(--ct-danger)', dangerBg:'var(--ct-danger-bg)', dangerLine:'var(--ct-danger-line)',
              muted:'var(--ct-muted)', mutedBg:'var(--ct-muted-bg)', mutedLine:'var(--ct-muted-line)' },
  },
  fontSize: { '2xs': ['11px','16px'] },
  fontFamily: { sans: ['var(--ct-font-sans)'], mono: ['var(--ct-font-mono)'] },
  boxShadow: { card:'0 1px 2px rgb(15 23 42 / .06)', pop:'0 4px 12px rgb(15 23 42 / .10)', modal:'0 12px 32px rgb(15 23 42 / .16)' },
  zIndex: { sticky:10, overlay:20, modal:30, drawer:40, top:50 },
  transitionDuration: { fast:'100ms', base:'150ms', slow:'200ms' },
}
```

> `body` 改用 `bg-surface text-ink-1`;`.dark body` 同样改令牌(批 0 迁移,`.markdown-body` 内硬编码色一并换令牌)。

---

## 3. 组件规范

### 3.1 工具卡片 ToolCard(分类页/搜索页/推荐区通用)

```
┌──────────────────────────────────────────────┐
│ [icon 20] 标题(14 medium,truncate)   ★(收藏) │  ← 行 1:图标+标题+收藏(桌面 hover 显示,
│                                              │    已收藏/触屏常显;aria-pressed)
│ 描述 text-sm ink-2 line-clamp-2(固定 2 行)   │  ← 行 2:高度恒定
│ [分类名]                    [● 已实现 徽章]  │  ← 行 3:meta 行,text-2xs ink-3 + §11 徽章
│ #tag #tag #tag(≤3 个,单行截断)               │  ← 行 4:标签 pill(现样式保留)
└──────────────────────────────────────────────┘
```

- 容器:`rounded-lg border border-line bg-surface-raised p-4 shadow-card`,hover `border-brand/50`(整体 `duration-fast`);暗色无 shadow,靠 border 变化(§13)。
- 等高:卡片 `flex h-full flex-col`;描述固定 `line-clamp-2`、标签行固定高 `h-6 overflow-hidden` → 同行卡片严格等高(§14 禁止项)。
- 图标:复用所属分类图标(现有逻辑),`h-5 w-5 text-brand`。
- 整体可点(`Link to=/t/:id`),收藏按钮 `stopPropagation`,focus 环见 §12。
- 占位工具:整卡叠加 `占位` 徽章(§11),背景不变(避免降对比)。

### 3.2 工具页头部 ToolPageHeader

```
面包屑:首页 / 分类 / 工具名(text-xs ink-3,分段可点)
[icon 40×40 rounded-lg bg-brand-subtle] 工具名(24 semibold ink-1) [● 状态徽章]
                                        描述(text-sm ink-2,≤2 行)
        ★ 收藏(ghost 按钮,aria-pressed)   v1.2(2xs ink-3)
```

- 头部吸顶(`z-sticky`),底部分隔线;滚动时降高只保留面包屑+标题行。
- 占位工具:头部下方追加占位横幅(§3.7)。
- 返回:面包屑承担,不另设返回按钮(命令面板可直达)。

### 3.3 筛选栏 FilterBar(分类页/搜索页,吸顶 z-sticky)

- 结构:左 = 分组 chips(§4 导航降级节点,`rounded-full border` 选中态 `bg-brand-subtle border-brand text-brand`);右 = 6 个维度下拉(任务/输入/输出/复杂度/状态)+ 排序下拉。
- 已激活筛选以 chip 形式回显(带 × 可单独移除)+「清除筛选」ghost 按钮;`aria-label` 标注维度名。
- 移动端:chips 行横向滚动(`overflow-x-auto` + `scrollbar-none`),下拉折叠为抽屉式面板。
- 空结果时筛选栏保持可见(配合 §3.5 空态)。

### 3.4 搜索框 SearchBox

- 尺寸:`h-10 rounded-md border border-line-strong bg-surface-raised`;左 Search 图标(16,ink-3)、右清除按钮(有值时出现)+ 桌面端 kbd 提示 `Ctrl K`(2xs)。
- placeholder:「搜索工具名称、标签…」;`role="searchbox"` + `aria-label="搜索工具"`。
- 聚焦:`ring-2 ring-brand/60 ring-offset-2`(§12);输入即出下拉建议(命令面板内核复用)。

### 3.5 空状态 EmptyState(三种变体)

统一结构:居中,图标 40×40(`rounded-full bg-surface-hover text-ink-3`)+ 标题(text-sm medium ink-1)+ 说明(text-sm ink-2)+ 主按钮(h-9)。

| 变体 | 图标 | 文案 | 动作 |
| --- | --- | --- | --- |
| 无搜索结果 | SearchX | 「没有匹配的工具」+ 尝试建议 | 「清除筛选」 |
| 无收藏 | Star | 「还没有收藏任何工具」 | 「去逛逛」→ 首页 |
| 分类为空 | Inbox | 「该分类暂无工具」 | 「浏览全部工具」 |

### 3.6 加载骨架 Skeleton

- 底色 `bg-surface-hover` + `animate-pulse`,圆角同目标组件;`prefers-reduced-motion` 下取消 pulse(静态底色)。
- 卡片骨架 = 4 行块(标题 60%、描述 100%、meta 40%、标签 3×20%);工具页骨架 = 头部两行 + 左表单 4 个输入块 + 右结果框(`h-64`);列表骨架 ≤ 8 项。
- 容器加 `aria-busy="true"`,骨架元素 `aria-hidden="true"`。

### 3.7 错误状态 / 占位状态(横幅规范)

| 横幅 | 配色 | 图标 | 行为 |
| --- | --- | --- | --- |
| 占位(前端 mock) | `bg-status-warnBg border-status-warnLine text-status-warn` | Info | 常驻不可关闭;文案:「该工具为占位实现,当前返回模拟结果」 |
| 后端占位(ssti 19) | 同上 | ServerCog | 文案:「后端占位,功能开发中」 |
| 运行错误(OutputPanel) | `bg-status-dangerBg border-status-dangerLine text-status-danger` | AlertCircle | 错误信息 mono 截断 + 复制按钮;动作:「重试」(保留输入) |
| 全局错误(ErrorBoundary) | 同上 + 面板样式 | AlertTriangle | 「重新加载」+ 错误详情可折叠 |

横幅统一 `rounded-md border px-3 py-2 text-sm flex items-center gap-2`;明暗两套由令牌自动切换。

### 3.8 分类导航(侧边栏)与移动端抽屉

- **L1 项**:`h-9 px-3 rounded-md flex items-center gap-2 text-sm`;图标 18;右侧计数 `text-2xs ink-3`。激活:`bg-brand-subtle text-brand` + `aria-current="page"`。
- **L2 项**:缩进 `pl-10 h-8 text-sm`,激活样式同 L1。≥3 段节点不进导航(ia.md §4)。
- **折叠态**(64px):纯图标列,`title` + 自绘 tooltip 提示。
- **移动抽屉**(<lg,沿用现有):`w-72 bg-surface-raised shadow-modal` + 遮罩 `bg-black/50`,从左侧滑入 `duration-slow`;`role="dialog" aria-modal="true"`,Esc / 点遮罩关闭;顶部含应用名 + 关闭按钮(icon 按钮 32×32)。

---

## 4. 布局规范

| 项 | 规范 |
| --- | --- |
| 应用壳 | `h-screen overflow-hidden`;侧边栏 256px(折叠 64px)+ 顶栏 48px + 主区 `flex-1 overflow-auto`(现有结构确认) |
| 内容最大宽度 | 主区不限宽(工具型);**内边距 `p-6`(24)桌面 / `p-4`(16)移动**;超宽屏(≥1920)可加 `mx-auto max-w-[1600px]` 护栏 |
| 卡片栅格 | `grid gap-4` + `grid-cols-[repeat(auto-fill,minmax(272px,1fr))]`;列数随断点自然落位(约:≥1280 四列 / 768~1023 两列 / <640 一列),不写死列数 |
| 工具页分栏 | `flex gap-8`:输入区 `w-[420px] shrink-0`(表单/工作区)+ 结果区 `flex-1 min-w-0`;`<lg` 改为上下堆叠(输入在上,结果 `min-h-64`);两栏均可独立滚动 |
| 断点 | 沿用 Tailwind 默认:sm 640 / md 768 / **lg 1024(侧边栏抽屉切换,与现有一致)** / xl 1280 |
| 移动端不横向溢出(规则 10) | 所有容器 `min-w-0`;代码/表格/pre 一律 `overflow-x-auto` 且父级 `max-w-full`;长工具名 `break-all`;验收时 360px 视口检查 `document.documentElement.scrollWidth ≤ innerWidth` |
| 表格(输出表格渲染) | 容器 `overflow-auto rounded-lg border border-line`;表头 `bg-surface-hover text-2xs uppercase`;行 `h-9 text-sm border-t border-line`;行 hover `bg-surface-hover` |

---

## 5. 状态徽章规范(§11 合并于此表,补充文案/点击性)

| status | 徽章文案 | Light(bg / text / border) | Dark(bg / text / border) | 点击性 |
| --- | --- | --- | --- | --- |
| `implemented` | 已实现 | `--ct-ok-bg` / `--ct-ok` / `--ct-ok-line` | 同令牌(暗套) | 可点 → 跳搜索页 `status=implemented` |
| `placeholder` | 占位 | `--ct-warn-bg` / `--ct-warn` / `--ct-warn-line` | 同令牌(暗套) | 可点 → `status=placeholder` |
| `broken` | 故障 | `--ct-danger-bg` / `--ct-danger` / `--ct-danger-line` | 同令牌(暗套) | 可点 → `status=broken` |
| `unknown` | 未知 | `--ct-muted-bg` / `--ct-muted` / `--ct-muted-line` | 同令牌(暗套) | 不可点(当前 0 条,信息性) |

- 结构:`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium` + 前置圆点 `h-1.5 w-1.5 rounded-full bg-current`。
- 卡片上用「圆点 + 文案」完整徽章;工具页头部同款放大一档(text-xs);ssti 后端占位在徽章旁加 `text-2xs ink-3` 注记「后端」。
- 可点徽章是 `<button>`(非 span),focus 环同 §12,`aria-label="按状态筛选:已实现"`。
- 当前数据分布:implemented 59 / placeholder 505 / broken 0 / unknown 0——**琥珀色将是全站第二高频徽章**,需与绿色对比度同级(§12 已验算)。

---

## 6. 可访问性(WCAG 2.1 AA 落地规则)

### 6.1 对比度(全部人工验算,批 0 加脚本回归)

| 场景 | 前景 / 背景 | 比值 | 判定 |
| --- | --- | --- | --- |
| Light 正文 | ink-1 `#0f172a` / raised `#fff` | 17.8:1 | AAA |
| Light 次文 | ink-2 `#475569` / raised | 7.6:1 | AAA |
| Light 弱文 | ink-3 `#64748b` / raised | 4.8:1 | AA |
| Light 链接 | brand `#4f46e5` / raised | 6.3:1 | AA |
| Light 主按钮 | `#fff` / brand | 6.3:1 | AA |
| Dark 正文 | ink-1 `#f1f5f9` / raised `#0f172a` | 16.3:1 | AAA |
| Dark 次文 | ink-2 `#94a3b8` / raised | 7.0:1 | AA |
| Dark 链接 | brand `#818cf8` / raised | 6.0:1 | AA |
| Dark 弱文 | ink-3 / raised | 3.8:1 | ✗ **禁用**于 <18px 文字;仅限大字(≥18px/14px bold,阈值 3:1)与图标 |
| 徽章 Light | ok 6.5 / warn 6.4 / danger 6.8 / muted 8.4 | — | 全部 AA+ |
| 徽章 Dark | ok 8.3(验算代表项);其余同族(300 文字/35% 深底)≥6.0 | — | AA+;批 0 脚本全量验算 |

### 6.2 键盘与焦点

- 焦点环统一:`focus-visible:ring-2 ring-brand/60 ring-offset-2 ring-offset-surface`,**禁止 `outline-none` 无替代**。
- 所有交互元素可 Tab 到达,顺序 = 视觉顺序;卡片整卡为单一 Tab 停靠点(内部收藏钮除外,收藏钮在卡片 Tab 之后)。
- Esc 关闭:命令面板、抽屉、下拉;命令面板支持 ↑↓ 选择 + Enter 打开(现有能力保留)。
- 收藏按钮、图标按钮必须有 `aria-label`(如「收藏:Base64 编解码」);收藏态 `aria-pressed`。

### 6.3 aria 与语义

- 地标:`aside`(导航)+ `main` + `nav` 面包屑;激活导航项 `aria-current="page"`。
- 状态徽章:`aria-label="状态:占位"`;占位横幅 `role="note"`。
- 输出区状态变化(loading→success/error)放入 `aria-live="polite"` 区域;骨架容器 `aria-busy`。
- 搜索框 `role="searchbox"`;筛选下拉 `aria-haspopup="listbox"`。

### 6.4 减少动效

- `@media (prefers-reduced-motion: reduce)` 全局:`*, ::before, ::after { transition-duration: .01ms !important; animation-duration: .01ms !important; animation-iteration-count: 1 !important; }`
- 骨架 pulse 在该媒体下退化为静态底。

---

## 7. 暗色模式规则

1. **分层靠明度不靠阴影**:app `--ct-surface`(最暗)> 面板/卡片 `raised` > 悬停面 hover;阴影在暗色一律禁用,层级用边框 + 表面提亮表达。
2. 文字层级见 §6.1;暗色下**不得**使用 `--ct-ink-3` 作为正文/辅助文字。
3. 状态色切换为「-300 文字 + 深色 35% 底 + 40% 描边」配方(§2.8),不得直接复用明色令牌。
4. 主题切换:保持现有 `<html class="dark">` 机制(useUiStore → class),默认暗色(index.html 硬编码 `class="dark"` 保留,防首帧闪白);切换过渡只作用于颜色属性,`duration-base`。
5. 滚动条:暗色 `scrollbar-color: rgb(148 163 184 / .35) transparent`(现有已实现,收编为规则)。
6. 输出面板、代码块、markdown-body 一律消费令牌/`--ct-*`,禁止组件内写死 hex(现有 `.markdown-body pre` 的 `#0f172a` 为批 0 迁移项)。
7. 图标颜色用 `currentColor`(lucide 默认),不单独设色。

---

## 8. 动效规范

- 允许:透明度/颜色过渡(100~150ms)、面板/抽屉位移(200ms)、骨架 pulse、按钮 hover 微提升(阴影,明色)。
- 禁止:弹跳/回弹曲线、旋转加载器(用 pulse 替代)、路由切换整页过渡、卡片入场 stagger、视差。
- 全部时长 ≤ 200ms,统一 §2.7 三档,禁止临时值(`duration-[350ms]` 视为缺陷)。

---

## 9. 禁止项(强制)

| # | 禁止 | 替代 |
| --- | --- | --- |
| 1 | 大面积/多渐变背景 | 默认纯色;单页渐变点缀 ≤1 处(仅首页分类图标底,可选) |
| 2 | 每页不同风格/私有颜色 | 只准消费 §2 语义令牌;发现硬编码色 = 批次缺陷 |
| 3 | 大动效(>200ms、弹跳、stagger) | §8 允许清单 |
| 4 | 低对比度文字(<4.5:1,暗色 ink-3 滥用) | §6.1 表 |
| 5 | 卡片高度参差 | 卡片 `flex h-full flex-col` + 固定行数 clamp(§3.1);栅格行等高 |
| 6 | 移动端横向溢出 | §4 规则 + 每批截图验收 |
| 7 | 删除旧路由/旧样式类 | 映射 + 重定向先行(ia.md §5),清理需单独确认 |
| 8 | 组件内写死工具逻辑/状态文案 | 全部由 inventory 字段派生(ia.md §7) |

---

## 10. 验收清单(可检查规则,规则 10 落地)

| 规则 | 检查方法 |
| --- | --- |
| 对比度 AA | 批 0 增加 `scripts/check-contrast.ts`:解析 §2.8 令牌,对 §6.1 全部配对自动验算,纳入 `inventory:check` 管线 |
| 动效 ≤200ms | grep 组件:`duration-\[\d+ms\]` 且 >200 即失败;`prefers-reduced-motion` 全局块存在 |
| 移动端无横向溢出 | 每批截图验收:360 / 768 / 1024 / 1440 四档,`scrollWidth ≤ innerWidth`(可用 Playwright 断言) |
| 徽章四态齐全 | 每批验收:状态徽章 implemented/placeholder/broken/unknown 四色齐备、文案正确、可点性符合 §11 |
| 卡片等高 | 每批截图验收:同行卡片底部对齐;代码上卡片含 `line-clamp-2` + `h-full` |
| 暗色完整 | 每批截图验收:明暗两套截图逐页比对,暗色无阴影依赖、无漏改硬编码色 |
| 键盘可达 | 每批人工验收:Tab 完整走查 1 次(首页→分类→工具页),焦点环可见,Esc 正常 |
| 占位可见且可辨 | 分类页肉眼 3 秒内可数出占位徽章数量;徽章可点、过滤生效 |

---

## 11. 落地顺序(供阶段 3 批次规划,本文不实施)

1. **批 0(基建)**:§2.8 令牌落地 → 基础组件库(StatusBadge/ToolCard 升级/ToolPageHeader/FilterBar/EmptyState/Skeleton/ErrorBanner)→ `/search` + 重定向表 → `inventory:check`(含 contrast 校验)→ git 基线(Q14 待确认)。
2. **试点批(2~5 个工具)**:crypto.base64 / rot13 / misc.whitespace_codec / web.basic.http-headers 全链路走新模板,产出首批截图验收。
3. **正式批(30~80/批)**:按 ia.md §8 差距清单逐批替换,每批出报告(规则 9)。
