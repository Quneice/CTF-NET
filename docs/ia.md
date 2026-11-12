# CTF-NET 信息架构设计(IA v1,阶段 2 产出,纯文档)

> 依据:`docs/tool-inventory.json`(627 条,已确认)+ `docs/audit-report.md` + `docs/category-mapping.md`。
> 原则:基于现有分布合理重构;工具 id 与 `/t/:toolId` 路由在阶段 3 内**不可变**(收藏/最近/历史零失效);混合结构 = 一级分类 + 标签 + 搜索 + 筛选 + 收藏/最近,不依赖纯树形导航。
> 本文未定稿项以「假设」标注,对应 `docs/confirm-questions.md` 的 Q 号。

---

## 1. 一级分类(12 个,满足 8~15)

### 1.1 分类表

| id | 名称 | 定义 | 边界(包含 / 排除) | 工具数 | 已实现 |
| --- | --- | --- | --- | --- | --- |
| `web` | Web 安全 | Web 漏洞探测与利用:注入、XSS、SSRF、RCE、反序列化、SSTI、上传、LFI、PHP 专项 | 排除纯 HTTP 工具类(→ forensics.network 或 codec) | 64 | 4 |
| `crypto` | 密码学 | 古典密码、现代密码(RSA/AES/ECC/DES 等)、哈希、自动解码 | **排除编码转换(→ `codec`)**;BEAST 演示待确认迁移(→ misc.demo,映射表 B3) | 190 | 7 |
| `codec` | 编码转换 | 无损格式转换:Base 家族、进制、转义、Unicode、字符集、序列化格式、esolang | 只做格式互转,不含密钥/加密运算 | 76 | 43 |
| `misc` | 杂项 | 文本处理、集合/位运算、时间/地理/图表、报告、演示 | 排除编码(→ codec)、隐写(→ stego) | 98 | 1 |
| `forensics` | 数字取证 | 内存/磁盘/流量/日志分析、压缩解压、数据提取 | 含 zip-tools/binwalk 等取证向解包 | 54 | 0 |
| `stego` | 隐写术 | 图片/音频/视频/文本隐写与图像处理 | 含原 misc.stego 3 个(映射表 B2,假设=迁移) | 37 | 3 |
| `reverse` | 逆向工程 | 静态分析、动态分析、去混淆 | 不含文件格式解析(→ forensics) | 14 | 1 |
| `pwn` | 二进制利用 | 栈/堆/格式化字符串/Shellcode | 不含逆向(→ reverse) | 11 | 0 |
| `osint` | 开源情报 | 社交账号、地理位置、图片情报、域名/网络 | 不含主动扫描(→ web/forensics) | 6 | 0 |
| `blockchain` | 区块链安全 | 链上分析、智能合约、钱包 | — | 7 | 0 |
| `mobile` | 移动安全 | Android / iOS | 不含通用逆向(→ reverse) | 4 | 0 |
| `ai` | AI 安全 | 提示词安全、AI 辅助分析 | — | 3 | 0 |
| `pentest` | 渗透测试 | 信息收集、资产探测、漏洞检测、轻武器库、辅助工具(批 12 占位骨架,pentest-migration-plan §1) | 不含 CTF 域工具;与 codec/osint/web 重叠项独立建档 | 47 | 0 |

合计 **612**(= 前端目录全量)。Go 注册表 31 个与 legacy 32 个不新增分类(映射见 category-mapping.md 表 D)。

### 1.2 关键决策:拆分出 `codec`

- **依据**:编码工具是当前最大且最健康的功能域(76 个工具中 43 个已实现,占全部已实现 59 个的 73%);原 `crypto`(266)占比 47% 严重失衡,拆分后 `crypto` 190(34%)、`codec` 76(13%),每类都可通过 L2 分组 + 筛选消化。
- **动作**:`crypto.encoding` 子树整体平移为 `codec`(工具挂载点改 categoryId,**工具 id 不变**);`crypto.classical/modern/hash/auto-decode` 留在 `crypto`。
- 备选(若人工确认否决):维持 11 类、`crypto` 266 不动,其余设计不变。

### 1.3 失衡缓解(不靠分类数量硬撑)

crypto 190 / misc 98 仍偏大,用「L2 分组 + 标签筛选 + 搜索」消化(见 §2、§3),导航树不做第 4 层。

---

## 2. 标签体系(7 个维度)

标签是筛选与推荐的事实源,全部可从清单字段派生或运行时统计,**不在页面代码写死**:

| 维度 | 来源 | 取值(受控词表) | 说明 |
| --- | --- | --- | --- |
| 领域 domain | 清单 `currentCategory`/`suggestedCategory` | 一级分类 id + 二级分组 id | 结构性字段,非自由标签 |
| 任务 task | 清单 `tags` 中以 `task:` 前缀的受控动词 | `task:加解密` `task:编解码` `task:转换` `task:解析` `task:生成` `task:探测` `task:扫描` `task:爆破` `task:提取` `task:分析` `task:识别` `task:还原` `task:利用` `task:攻击` `task:演示` `task:校验` `task:格式化` `task:计算` | **阶段 0 批量回填**:由 description/tags 关键词自动初填 → 人工复核,写入清单 |
| 输入类型 input | 清单 `inputTypes`(字段类型归并) | `text`(文本/代码)`file`(文件)`url`(URL/原始请求)`none`(无输入) | 筛选枚举用归并后的 4 类;原始字段类型保留在清单 |
| 输出类型 output | 清单 `outputTypes` | `text` `json` `image` `table` `code` `hex` `markdown` `stream` `file` | 与 OutputSpec 一致 |
| 使用频率 freq | 运行时 `useHistoryStore` 统计 | `hot`(月使用 ≥10)/ `normal` / `cold` | 非静态字段;首页推荐与排序消费 |
| 复杂度 cx | 清单 `complexity` | `low` / `medium` / `high` | 已按表单字段数/调用类型/输出类型推导 |
| 实现状态 status | 清单 `status` + invoke 接线 | `implemented` / `placeholder` / `broken` / `unknown` | 徽章与筛选的唯一来源;状态由接线自动推导,禁止手填 |

补充:清单 `tags` 允许自由关键词(如 `RSA`、`SQL注入`),但筛选只消费上表 7 个维度;`favorite`(收藏)与 `recent`(最近)是系统级筛选,不入标签。

---

## 3. 页面结构(4 页 + 404)

### 3.1 首页 / 工具广场 `/`

- 顶部:品牌 + 全局搜索框(Ctrl+K 命令面板,现有能力保留)。
- 分类入口:12 个分类卡片(图标、名称、工具数、已实现数)——替代"精选工具切片"。
- 推荐区:推荐池 = **implemented ∩(热度 hot/normal ∪ 人工推荐位)**;占位工具永不进入(见 §6);实现 F7 修复(弃 `tools.slice(0,12)`)。
- 最近使用 + 收藏:保留现有 localStorage 数据(`ctf-net:favorites` 等 key 不变)。
- 状态筛选 chips:全部 / 已实现 / 占位(一键过滤全站浏览视角)。

### 3.2 分类页 `/c/:categoryId`

- 面包屑(≤3 级)+ 分类定义/边界说明。
- **分组 chips**:仅展示当前分类的下一级;≥3 段的历史节点(如 `crypto.classical.caesar`)不进入导航树,作为**筛选 chips** 展示(带计数),路由仍可直接访问。
- 筛选器:领域(分组)/ 任务 / 输入类型 / 输出类型 / 复杂度 / 状态,全部从 §2 维度派生。
- 排序:默认 **状态优先(已实现在前)→ 热度 → 名称**;可选:名称 / 最近使用 / 复杂度。
- 工具卡片网格(现有 ToolCard 升级):统一卡片规范 + **状态徽章**(§6)。
- 空态:统一空状态组件(含"清除筛选"引导)。

### 3.3 工具详情页 `/t/:toolId`

统一模板(设计系统 §工具页模板,阶段 2 配套文档):
1. **统一头部**:面包屑、名称、状态徽章、收藏按钮、版本、标签行。
2. 说明区:description + task 标签。
3. 输入区:DynamicForm / 专用 Workspace(现有,配置驱动)。
4. 结果区:OutputPanel(现有状态机)。
5. 相关工具:同领域或同 task 标签 top 4(清单派生)。
6. **状态提示**:占位工具显示顶部横幅(§6);realBackend 占位(ssti 19)显示「后端占位」。

### 3.4 搜索页 `/search`

- URL 参数驱动:`?q=&cat=&tag=&status=&input=&output=&cx=&sort=`(可分享、可收藏)。
- 左侧 facets(§2 七维度)+ 结果网格 + 空态。
- 与首页搜索框、Ctrl+K 命令面板共用同一搜索内核(现有 `lib/search.ts` 升级为标签联合检索)。

### 3.5 404

未匹配路由 → 统一 404 页:搜索建议(模糊匹配工具名/标签)+ 分类入口,不出现裸错误页。

---

## 4. 导航树(≤3 层)

**政策(假设,对应 Q5)**:导航树只含「一级分类 → 二级分组 → 工具页」三层;分类 id 中第 3 段及以上的节点**不进导航树**,降级为分类页筛选 chips(路由保留)。同时执行 mapping 表 C 方案 A:5 个 4 段 RSA 节点合并进 `crypto.modern.rsa`(22 工具,tags 保留技术类别)。

```
CTF-NET
├─ / 首页(工具广场)
├─ Web /c/web
│  ├─ 基础 /c/web.basic · SQL 注入 /c/web.sql · XSS · SSRF · RCE · 反序列化 · SSTI · 上传 · LFI · PHP 专项
│  └─ 工具页 /t/:toolId                    ← L3 叶子
├─ Crypto /c/crypto
│  ├─ 古典密码 /c/crypto.classical        (凯撒/维吉尼亚/仿射/带密钥 = 页内 chips,不进导航)
│  ├─ 现代密码 /c/crypto.modern           (RSA/AES/ECC 分组 + 原 4 段节点 = chips)
│  ├─ 哈希 /c/crypto.hash · 自动解码 /c/crypto.auto-decode
├─ 编码转换 /c/codec
│  ├─ Base 家族 /c/codec.base             (12)
│  └─ 其余 64 个平铺 + 标签分组筛选(esolang/序列化/字符集…)
├─ 杂项 /c/misc        (其他 /c/misc.other · 报告 /c/misc.report · 演示 /c/misc.demo)
├─ 取证 /c/forensics   (内存 · 磁盘 · 流量 · 日志)
├─ 隐写 /c/stego       (图片 · 音频 · 视频 · 文本;含迁入的 misc.stego 3 个)
├─ 逆向 /c/reverse · Pwn /c/pwn · OSINT /c/osint · 区块链 /c/blockchain · 移动 /c/mobile · AI /c/ai
├─ 渗透测试 /c/pentest (信息收集 /c/pentest.recon · 资产探测 /c/pentest.scan · 漏洞检测 /c/pentest.vuln
│                     · 轻武器库 /c/pentest.arsenal · 辅助工具 /c/pentest.assist;均为页内 chips,批 12 占位)
└─ 搜索 /search
```

- 侧边栏:展开态 = L1 列表 + L2 内联展开;折叠态 = L1 图标列(现有能力)。
- 顶栏:搜索、收藏下拉、最近下拉、主题切换(现有能力)。
- 收藏/最近仍按工具 id 存储 → 任何分类重组都不影响。

---

## 5. 旧路径 → 新路径映射与重定向策略

### 5.1 铁律:工具路由不变

- `/t/:toolId` 在阶段 3 内**不修改任何 id** → 收藏、最近使用、历史、外链全部有效。
- 未来若做 id 规范化(confirm-questions Q8/Q12),流程:先在清单补 `oldPath` → 生成 `docs/redirects.json` → 路由器 `<Navigate replace>` 301 语义 → 观察期后(阶段 4)才清理。

### 5.2 分类路由重定向表(本次 IA 变更,一次性)

| 旧路径 | 新路径 | 依据 |
| --- | --- | --- |
| `/c/crypto.encoding` | `/c/codec` | §1.2 codec 拆分 |
| `/c/misc.stego` | `/c/stego` | 映射表 B2(假设) |
| `/c/crypto.modern.rsa.algebraic` | `/c/crypto.modern.rsa` | 映射表 C 方案 A |
| `/c/crypto.modern.rsa.fermat-family` | `/c/crypto.modern.rsa` | 同上 |
| `/c/crypto.modern.rsa.shared-structure` | `/c/crypto.modern.rsa` | 同上 |
| `/c/crypto.modern.rsa.number-theoretic` | `/c/crypto.modern.rsa` | 同上 |
| `/c/crypto.modern.rsa.coppersmith` | `/c/crypto.modern.rsa` | 同上 |
| 其余 `/c/*` | 不变 | — |

- 实现:重定向清单由 inventory 的 `oldPath`/`suggestedCategory` 生成(单一事实源,不手抄);SPA 内 `<Navigate replace>`,同时保留原路径访问(规则 8:不删除,先映射后清理)。
- legacy FastAPI 路由(`/api/*`,32 个工具)不迁移;归档记录在 `docs/audit-report.md` §10。
- 未知 `/c/:id`、`/t/:id` → 404 + 搜索建议(§3.5)。

---

## 6. 占位工具策略(假设:默认展示)

| 问题 | 决策 | 理由 |
| --- | --- | --- |
| 默认是否展示 | **展示**(不隐藏) | 规则 8「不删除」的 UI 化:透明化优于隐藏;隐藏会让 505 个工具从搜索中消失,违背"基于现有分布" |
| 如何标记 | 三层标记:① ToolCard 角标「占位」(灰色/琥珀虚线风格,与「已实现」绿色实线徽章成对);② 工具页头部徽章 + 顶部占位横幅(说明"该工具为占位实现,当前返回模拟结果");③ ssti 19 个额外显示「后端占位」 | 徽章规范进设计系统(规则 10:状态徽章一致) |
| 是否可搜索到 | **是**,默认出现在搜索结果;搜索页提供「状态=已实现」快捷筛选一键排除 | 可发现性优先 |
| 是否进入推荐 | **否**;推荐池 = implemented 限定(§3.1) | 推荐是"可用工具"的入口 |
| 分类页排序 | 默认排序把占位沉底(状态优先) | 减少 mock 结果误触 |
| 交互 | 占位工具照常可打开、可提交(返回 mock 结果并标注),表单/输出组件复用统一模板 | 骨架先行(规则 7),功能按 P2 排期 |

---

## 7. 扩展策略:新增工具只改清单/配置

### 7.1 配置驱动管线(阶段 0 落地)

```
作者面:frontend/src/mock/tools.ts
  ① 新增 = 一行 shorthandRow(或完整定义,15 个范式示例已覆盖全部字段类型)
  ② 如需本地计算 = localHandlers.ts 注册一个 handler(现有 57 个范式)
  ③ 如需真实后端 = Go 侧 register.go 注册 + 前端 invoke 标 realBackend
        │
        ▼  npm run inventory:check(新增校验脚本,阶段 0 交付)
  · 类型合规(ToolDefinition)
  · categoryId 存在于分类树(≤2 段新节点规则)
  · handler 存在性 / realBackend 后端存在性
  · **状态自动推导**(implemented/placeholder,禁止手填)
  · 重新生成 docs/tool-inventory.json 与 docs/redirects.json
        │
        ▼  运行时 UI 零改动
  ToolCard/徽章/筛选/搜索/相关工具/占位横幅 全部由清单字段派生
  (DynamicForm、OutputPanel、renderers、adapters 已配置驱动,无需改组件)
```

### 7.2 规则

- 新增分类:categories.ts 加节点,id ≤2 段,必须挂到现有 12 个一级分类之一;禁止新增第 4 级。
- 新增工具:工具 id 全局唯一且创建后不可变;`task:` 标签从 §2 受控词表选取。
- 页面组件禁止出现工具特定逻辑(现有 ToolPage 已满足;专用 Workspace 仅作为显式例外注册)。
- 状态徽章、筛选维度、推荐池全部读清单,**清单即界面**。

---

## 8. 与现有实现的差距清单(阶段 3 改造项,不属本文实现)

| 项 | 现状 | 改造 |
| --- | --- | --- |
| 路由 | 3 条(/, /c/:id, /t/:id) | + `/search`、404 页、分类重定向表 |
| 分类树 | 62 节点、4 层 | 12 类 + codec 拆分 + 表 C 扁平化 + ≤3 层导航政策 |
| 首页 | 精选 = slice(0,12) | 分类入口卡 + implemented 推荐池 + 状态 chips |
| 工具卡片 | 无状态标识 | 状态徽章 + 卡片规范 |
| 工具页 | 无状态提示/相关工具 | 统一头部模板 + 占位横幅 + 相关工具 |
| 搜索 | 名称模糊(Ctrl+K) | 标签联合检索 + /search 页 |
| 清单 | tools.ts 为唯一运行时源 | + inventory:check 管线,清单/界面同步 |
| 标签 | 自由 keywords 为主 | + task: 受控词表回填 |

---

## 9. 假设与待确认项(引用 confirm-questions.md)

| 本文采用的默认 | 对应问题 |
| --- | --- |
| 全量 564 纳入重构 | Q1 |
| F1 三处接线第 0 批修复、F2 前端本地优先 | Q3、Q4 |
| 表 C 方案 A(rsa 5 节点扁平化)+ 3 层导航政策 | Q5 |
| 占位默认展示、可搜索、不进推荐 | Q6 |
| misc.stego 3 个迁入 stego(wechat-dat 目标暂定 stego.image) | Q9 |
| codec 拆分(新一级分类) | 本文 §1.2,新增确认项 |
| beast 迁移暂缓,仍挂 codec 顶层并标记 | Q16 |
| 视觉沿用 slate+indigo(设计系统文档随后输出) | Q15 |
