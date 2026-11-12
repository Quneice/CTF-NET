# 漏洞浏览器设计（漏洞库集成）

> 把本地漏洞复现案例目录（`C:\Users\HP\Desktop\安全方向\漏洞复现案例` 等，每 md 一个漏洞）
> 导入应用，作为「漏洞浏览器」工具页（工具 ID `pentest.vuln.poc-scan`）的数据源：
> 按分类树浏览 / 关键字搜索 / 右侧 Markdown 全文展示。纯内容展示，不做检测执行。

## 语料规模（真实目录统计）

- md 总数 6199（排除 My_template）；导入有效案例 **4865** 个、50 个分类；
- 全量扫描约 1.7s（16.9MB 文本），之后增量扫描（mtime+size 未变跳过）秒级完成。

## 目录解析规则（`internal/vulnlib/store.go` classify）

| 路径形态 | 处理 |
| --- | --- |
| `ZXXX中间件漏洞/S1 Apache/ND003 ….md` | 常规案例 |
| 根目录散件 md（CRLF Injection 等 3 个） | 归入「其他」分类 |
| `ZXXX/ZXXX ….md`（Z 目录直属总览） | 跳过（40 个） |
| `ZXXX/S1/S1 ….md`（与 S 目录同名的系统总览） | 跳过（1229 个） |
| 更深层非 ND 前缀（Apache 版本文档 README 等） | 跳过（68 个） |
| 更深层 ND 前缀（如 assets/ 里的案例） | 收录，挂到其 S 系统 |
| `My_template/` | 整目录跳过 |

## md 解析（`internal/vulnlib/parser.go`）

- 标题：首个一级标题；CVE：`CVE-\d{4}-\d{4,}` 按序去重（列表/工作区头部的 CVE 徽标用）；
- 解析结果存库备查（payloads/curls 字段仍在，但 v1 前端不展示、不执行）。

## 存储（`internal/vulnlib/store.go`）

- SQLite `data/vulnlib.db`（modernc.org/sqlite，纯 Go）：categories（Z 分类，ord 保序）+
  cases（id=相对路径、title/cves/content 全量、size/mtime 增量判定）+
  meta（root_dir/last_scan）；单事务导入；打开失败降级不阻断主功能。

## 绑定（`vulnlib_api.go`，挂在 App）

| 绑定 | 用途 |
| --- | --- |
| `VulnLibStats()` | 是否已导入/案例数/上次扫描 |
| `ScanVulnLibrary(dir)` | 同步扫描导入（首扫约数秒，UI 展示加载态） |
| `ListVulnCategories()` | 分类树（含系统计数，案例懒加载） |
| `ListVulnCases(catID, sys)` | 某系统案例摘要 |
| `SearchVulnCases(kw, catID, limit, offset)` | 标题/文件名/CVE 关键字 |
| `GetVulnCase(id)` | 案例全文 |
| `GetVulnAsset(caseID, rel)` | 案例引用的本地图片 → base64（data URL 展示） |
| `CreateVulnCase(catID, sys, name, content)` | 新增案例：写入磁盘 md + 增量重扫 |
| `UpdateVulnCase(id, content)` | 保存修改：覆写磁盘 md + 增量重扫 |
| `DeleteVulnCase(id)` | 删除案例：md 移入 `data/backups/vulnlib_deleted`（可恢复） |
| `FetchVulnRemoteImage(rawurl)` | 拉取远程 http(s) 图片，经全局代理（未配置则直连） |

## 全局代理

- 设置页新增「全局代理」组（`data/config.json` 的 `proxy.url`），支持
  http/https/socks5/socks5h，留空 = 直连；保存即生效（atomic.Value 快照）。
- 漏洞浏览器唯一出站流量是 md 引用的远程 http(s) 图片：配置代理后由 Go 端
  `FetchRemoteImage`（internal/vulnlib/remote.go）经 `http.ProxyURL` 拉取并转发为
  data URL；未配置代理时保持 WebView 直连（原行为）。
- 拉取限制：仅 http/https、20s 超时、≤10MB、Content-Type 须 image/*（缺头按扩展名兜底）、
  容忍自签证书；代理失效时报错而非绕行（remote_test.go 用拒连代理验证流量不绕过）。

## 前端

- `lib/vulnlib.ts`：绑定封装 + 类型；浏览器模式回退静态快照（仅结构）。
- `components/poc/VulnLibraryPanel.tsx`：二级侧边栏。目录输入带默认值
  （`DEFAULT_VULNLIB_DIR`，localStorage `vulnlib.dir` 记忆；本地无记录时采用后端
  已导入的 root_dir）；桌面版首次打开（库未导入）自动扫描默认目录，免手动选择；
  搜索走后端；树形浏览案例懒加载；「新增」表单（分类/系统/文件名/内容 → 写盘）；
  案例行悬浮「删除」（两步确认）。
- `components/poc/PocScanWorkspace.tsx`：纯正文工作区（无工具页头/「当前漏洞」等，
  右侧只有 md 渲染全文）。顶部小操作栏：编辑（textarea 原文）→ 保存写盘重扫；
  删除（两步确认）→ 文件移入备份并从库移除。编辑操作经 `usePocStore.bumpRefresh`
  通知侧边栏重载。
- `lib/markdown.tsx`：渲染器改为递归块级解析，按语料实测形态支持：
  标题（≤3 空格缩进、`#标题` 无空格、`一、` 中文编号、尾部闭合 #、h1-h6）、
  围栏/缩进代码块、嵌套·多行条目列表（ul/ol）、多行引用、表格、分隔线、`<br>`、
  图片（`![]()` 与裸 `<img>` 行）；可选 `imgSrcResolver` 异步把本地相对引用
  解析为 data URL（漏洞浏览器传入 `resolveVulnAsset`，带缓存；加载中/失败有占位）。
- `stores/usePocStore.ts`：选中项（categoryId/systemId/caseId/title/cves）会话级透传。

## 已知边界（v1 假设）

- 图片解析按「案例目录 → 库根」两处相对引用兜底；越出库根的路径一律拒绝；
  单图上限 10MB；http(s) 远程图：未配置全局代理时原样交给 WebView 加载，
  配置代理后经 Go 走代理拉取；data:/blob: 原样返回；
- 编辑安全：文件名/目录名校验（拒绝路径分隔、`..`、Windows 保留字符、隐藏名）；
  Update/Delete 的 id 必须是库内案例；Create 分类必须已存在；变更后增量重扫入库；
- 删除为「移入备份」而非物理删除：`data/backups/vulnlib_deleted/<相对路径>`，重名加时间戳；
- 重命名案例不支持（v1：改名 = 删除 + 新增）；分类/系统目录本身不能删除；
- 设置页未加漏洞库目录项（面板内输入 + localStorage 记忆 + 默认值）；
- 浏览器（npm run dev 无桌面运行时）模式仅目录结构预览，不能扫描/查看全文/编辑；
- 提取的 payloads/curls 字段存库但前端不展示（当初按检测引擎设计预留，现为纯浏览器）。
