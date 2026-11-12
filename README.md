# CTF-NET

配置驱动的 CTF 工具箱桌面应用。**Wails v2 + Go + React** 架构：Go 后端内建工具注册表、任务队列与交互会话，React 前端（Vite）通过 Wails 绑定调用。

> 仅用于授权 CTF / 安全测试环境。

## 架构

```
CTF-NET/
  main.go                     # wails.Run 入口
  app.go                      # App 绑定面（等价旧 FastAPI REST 契约）
  wails.json                  # Wails v2 配置（frontend 指向 frontend/）
  internal/
    core/                     # 工具抽象：注册表 / 参数校验 / 任务队列 / 交互会话 / 文件存储
    vulnlib/                  # 漏洞库：目录扫描导入（Z→S→ND md）+ SQLite + 全文展示（漏洞浏览器数据源）
    tools/
      crypto/                 # aes_cbc_bitflip（CBC 翻转 + padding oracle）、base64、rot13、beast
      misc/                   # whitespace_codec（解释器 + 隐写）、hash、file_checksum、wechat_dat
      pentest/                # dir_enum（字典目录枚举）、frp（FRP 反向代理一键部署）
      pentest/frp_files/      # 内嵌 frp 0.69.1 发布包（go:embed：linux_amd64/arm64 + windows_amd64，部署免联网）
      reverse/                # strings（Go 原生可打印串扫描）
      web/                    # jndi（原生 Go SSH 一键利用）、ssti_*（20 引擎）、http_headers
      web/jndi_files/         # JNDI 载荷模板（go:embed，原样部署到远程）
  frontend/                   # React + Vite（真实任务适配层调 Wails 绑定，无 Wails 时自动回退本地 mock）
  data/                       # 运行时数据：work/ artifacts/ uploads/（打包后可用 CTF_DATA_DIR 覆盖）
  legacy/                     # 旧 Python FastAPI 版归档（app/ tools/ tests/ Dockerfile 等）
  backups/ writeups/          # 备份与题解，不参与构建
```

- 前端只有两个真实后端触点：工具任务的提交/轮询（`RunTool` / `GetJob`）与交互会话（`ExecSessionCommand` / `CloseSession`），其余工具由前端 local 处理器离线完成。
- 纯浏览器 `npm run dev` 可独立开发 UI（无 `window.go` 时适配层自动走 mock）。

## 工具一览（47 个）

| 分类 | 工具 | 说明 |
| --- | --- | --- |
| web | `jndi` | JNDI 注入一键利用：SSH 自动部署 LDAP/RMI 引用服务与恶意类，生成 fastjson 1.2.24/41/42/43/47、Log4j2、custom PoC，监听端口捕获反弹 shell，支持交互会话 |
| web | `ssti_jinja2` | Jinja2 SSTI 全功能：detect / info / exec（8 入口链 + 6 绕过 + 子类枚举）/ read_file（阶梯 + FLAG 扫描） |
| web | `ssti_*`（19 个） | 其余模板引擎占位符（回显参数，接口与真实实现一致） |
| web | `http_headers` | HTTP 响应头探测 |
| pentest | `fscan` | 端口/服务扫描（外部 fscan 2.2.2 exe：存活探测 + 端口扫描 + Web POC + 弱口令检测；路径在设置页配置，全局代理自动映射 -socks5/-proxy） |
| pentest | `dir_enum` | 目录枚举（参考 gobuster：并发字典爆破 + 通配符检测，字典库/上传/设置页三级字典） |
| pentest | `webfingerprint` | Web 指纹识别（finger-plus Go 移植：静态串 + 文件 Hash(murmur3/FOFA) + 路径探测，内置 1.3 万+ CMS 指纹库，关联已知漏洞） |
| pentest | `subdomain_enum` | 子域名枚举（OneForAll Go 移植：38 被动源 + 9.5 万词爆破 + SRV + 泛解析过滤 + HTTP 探测 + finder/altdns + CDN/归属地，导出 CSV/TXT/JSON） |
| pentest | `jsfinder` | JS 接口提取（JSFinder Go 移植：LinkFinder 正则与 Python 对拍 + 页面/JS 抓取 + 深度爬取一层 + URL/JS 清单批量模式，输出 URL 与子域名；全局代理自动生效） |
| pentest | `packer_fuzzer` | JS 被动分析（Packer-Fuzzer Go 移植：脚本收集 + webpack 指纹 + 精确/暴力接口提取与 BaseUrl 拼接 + GET/POST 探测 + 敏感信息扫描；与 Python 对拍 166 条 fixture 全绿） |
| pentest | `frp` | FRP 反向代理一键部署：SSH 自动在服务器部署 frps（优先内嵌本地包上传，免联网；支持上传/下载回退 + TOML 配置 + 启动校验），生成 tcp/udp/http/https/socks5 的 frpc 客户端配置并附带对应平台 frpc 二进制产物，上传目标服务器即可内网穿透；远端端口留空时自动探测服务器空闲端口分配；支持停止/清理 |
| ai.pentest | `task` | AI 自动化渗透（原生引擎 internal/aiagent：LLM 规划者+执行者双角色 agent，意图队列编排 + 内置工具调用 + YAML 配方工具 + goals 角色目标分解/约束提取 + 代理环境注入子进程 + 意图级取消与人工舵向；智谱 GLM/DeepSeek/OpenAI 兼容 API，全局代理生效；漏洞上报强制可复现证据；状态落盘 task.json 可复盘） |
| ai.pentest | `工作台` | AI 工作台（侧边栏「AI 工作台」→ 仪表盘 / 任务详情 / 发现 / 资产：跨任务统计与按天 token 消耗、任务暂停/恢复/停止与意图重跑/舵向、探索图（目标-意图-事实/资产/漏洞边关系）、漏洞与资产聚合检索、活动流） |
| ai.pentest | `chat` | 对话式渗透（对话引擎原样照搬 ARTEX-1.0.4：norma agentcore.Session + transcript JSONL 续聊 + captureRun 事件合并；助手直接侦察/验证并把事实沉淀到会话黑榜；可发起后台自动化任务并查询进展；支持智谱 GLM/DeepSeek 思考模式（reasoning_content 原样回传续传，思考过程实时弱化展示）；侧边栏「AI 对话」页为**实时流式对话框**——SendChatMessage 绑定 + chat:* 事件流逐 token 推送、按模型轮次分段展示、工具动作实时可见、回复中可发送新消息打断并补充说明，不走任务队列；单轮默认 48 步工具往返、25 分钟墙钟，到预算时注入收尾提示让模型写结论，杜绝空回复占位符） |
| crypto | `aes_cbc_bitflip` | AES-CBC 位翻转攻击 + padding oracle（含 PHP `+`→空格、base64 引号等已知坑） |
| crypto | `base64` / `rot13` / `beast` | 编码转换与 BEAST 演示 |
| misc | `whitespace_codec` | Whitespace 解释器 + Snowfall 隐写（两 token 操作码规则） |
| misc | `hash` / `file_checksum` / `wechat_dat` | 哈希、文件校验、微信 dat 图片恢复 |
| data | `image_ocr` / `image_qrcode` / `image_metadata` / `image_batch` | 图片 OCR（tesseract 多语言）、二维码识别、EXIF 元数据（含 GPS 中国地理反查）、图片批处理 |
| data | `audio_asr` | 音频转文字（内置 whisper tiny/base q5_1 模型，多进程并行） |
| data | `packet_crawler` | Burp 原始请求包占位符批量请求（区间/枚举占位符，文本响应链接下钻一层） |
| data | `table_merge` | CSV/Excel 表连接（inner/left/right/outer） |
| reverse | `strings` | 可打印字符串扫描 |

## 构建与运行

前置：Go 1.21+、Node.js 18+、Wails v2 CLI（`go install github.com/wailsapp/wails/v2/cmd/wails@latest`）。

```bash
wails dev            # 开发模式（Go 热重载 + Vite HMR）
wails build          # 打包 Windows exe 到 build/bin/
cd frontend && npm run build   # 仅前端类型检查与产物构建
```

测试：

```bash
go test ./...        # 全部 Go 单测（含 jndi 进程内 SSH 服务器全流程测试）
```

## 数据目录

默认 `data/`（相对工作目录），环境变量 `CTF_DATA_DIR` 可覆盖：

- `work/{jobId}/` — 每次运行的工作目录（模板副本、日志）
- `artifacts/`、`uploads/` — 产物与上传文件
- `vulnlib.db` — 漏洞库（漏洞浏览器：目录扫描导入的案例全文）
- `jndi_ssh_hostkeys.json` — JNDI SSH 主机密钥 TOFU 指纹库（首次连接记录 SHA256，此后校验；换服务器重装时删除对应条目即可）
- `frp_ssh_hostkeys.json` — FRP SSH 主机密钥 TOFU 指纹库（同上，与 JNDI 独立）

## 与旧 Python 版（legacy/）的差异

- **JNDI 改用原生 Go SSH**（`golang.org/x/crypto/ssh`）：免装 PuTTY/plink，`putty_dir` 参数保留但忽略（报告中注明）；上传改用 `cat > path` 会话方式；远端命令序列（fifo `3<>` 保活、pkill 正则字符类、JDK 8/9-16/17+ 模块回退等）逐字保留。
- **teyvat 后端未移植**：无前端消费者（前端 local.teyvat 已覆盖），Python 源码保留在 `legacy/`。
- ssti_jinja2 中 RE2 不支持的正则（lookaround）改为手动边界检查，行为等价。
