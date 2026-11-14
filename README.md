# CTF-NET

一体化 CTF 解题 / 渗透测试 / 大模型安全测试工具箱桌面应用。**Wails v2 + Go + React** 架构:Go 后端提供工具注册表、任务队列、交互会话与多套知识库(SQLite),React 前端(Vite)通过 Wails 绑定调用,无 Wails 环境时自动回退本地 mock 便于纯浏览器开发。

> 仅用于授权 CTF / 安全测试环境。
>
> 仓库:https://github.com/Quneice/CTF-NET

## 功能总览

- **五模式工作台**:启动进入模式选择页——「CTF 模式」(Web/Crypto/Misc/Reverse/Pwn/取证/隐写等 15 个分类)、「渗透测试模式」(侦察 / 扫描 / 漏洞 / 武器库 / 隧道 / 辅助)、「模型安全模式」(越狱攻击 / 提示注入 / 数据投毒 / 模型后门)、「流量分析模式」(流量解析 / 协议解码 / 流量指纹)、「逆向模式」(静态 / 动态 / 去混淆),共享 Web、编码、OSINT 工具。
- **模型安全测试**(`internal/tools/aisec`):LLM 越狱/提示注入批量扫描(内置 20 种模板库 + 自定义 payload,OpenAI 兼容端点,中英拒绝判定 + 成功正则)、模型文件后门扫描(pickle/zip/safetensors 恶意导入检测)、训练数据投毒筛查(n-gram lift 触发器候选 + 可疑模式统计)。
- **工具系统**:后端 Go 注册表 79 个真实工具(网络扫描、指纹、子域、AI、OCR/ASR、古典密码、RSA 攻击等),统一任务队列(异步 Job + `task:done` 事件推送)、交互会话(SSH/Shell 全双工)、产物下载;前端 600+ 工具目录中其余轻量工具由前端 local 处理器离线完成。
- **AI 渗透引擎**(`internal/aiagent`):LLM 规划者+执行者双角色 agent,意图队列编排、内置工具调用、YAML 配方、角色目标分解/约束提取、代理环境注入、意图级取消与人工舵向;智谱 GLM/DeepSeek/OpenAI 兼容 API,全局代理生效;漏洞上报强制可复现证据,状态落盘 task.json 可复盘。
- **AI 对话**:对话引擎(ARTEX-1.0.4 同款 norma 内核:transcript JSONL 续聊 + captureRun 事件合并),流式逐 token 推送、思考过程实时弱化展示、工具动作可见、回复中可打断补充;支持 DeepSeek 思考模式(reasoning_content 原样回传)。
- **AI 工作台**:仪表盘 / 任务详情 / 发现 / 资产 4 页——跨任务统计与按天 token 记账、任务暂停/恢复/停止、意图重跑/舵向、探索图(目标-意图-事实/资产/漏洞边关系)、漏洞与资产聚合检索、活动流。
- **代码审计工作台**(渗透测试模式侧边栏,SQLite `audit.db` + `audits/<id>/` 文件区):导入整个项目文件夹(保留目录结构)→ 文件树浏览 / 查看 / **在线编辑保存**(首次修改自动 `.bak` 备份)/ 全文搜索(plain+正则+大小写)→ **敏感函数规则库驱动的高亮与全项目扫描**(内置 40+ 条 PHP/Java/Python/JS/Go/通用规则:命令执行、SQL 注入、反序列化、JNDI/XXE/SSTI/SSRF、硬编码凭据/AWS Key/私钥等,支持增删改与启用停用,即时生效)。
- **项目库**(SQLite):渗透项目 CRUD + 项目内条目(记录/资产/漏洞等 kind),时间戳自动维护。
- **红队 Wiki**(SQLite):内置速查库(反弹 Shell、提权、隧道等分类),支持自定义增删改。
- **逆向 Wiki**(SQLite,逆向模式):8 分类速查(算法特征 / 工具操作 / 反调试 / 壳与脱壳 / 文件格式 / **常用函数**(MSVC CRT·Win32·libc 见名知义)/ 进制 ASCII / 套路笔记)+ 顶部「进制↔ASCII 即时查询条」(0x64↔d、批量双向、URL 形态),支持自定义增删改。
- **漏洞库浏览器**(SQLite):内置数千条真实漏洞案例、数十个分类,全文检索,支持自定义 CRUD(删除自动进 backups 备份),远程图片经全局代理拉取。
- **指纹管理库**:内置 1.3 万+ CMS 指纹(go:embed),首次启动播种到数据目录,支持增删改与关联已知漏洞;Web 指纹识别工具实时消费。
- **设置中心**:全局代理(自动映射到 fscan/jsfinder/子域被动源/AI 请求)、外部工具路径(fscan/nmap/sqlmap/xray/nuclei/python3 等)、AI 接口配置、FRP/JNDI 服务器配置。
- **字典库**:wordlists/ 随项目分发,dir_enum 支持「字典库 / 上传 / 设置页」三级字典选择。

## 架构

```
CTF-NET/
  main.go                     # wails.Run 入口:装配各 Store + go:embed frontend/dist
  app.go                      # App 绑定面:分类/工具/任务/会话/文件(等价旧 FastAPI REST 契约)
  ai_api.go chat_api.go       # AI 任务控制 / AI 统计 / 流式对话绑定
  audit_api.go                # 代码审计工作区绑定(导入/文件树/读写/搜索/扫描/规则库)
  config_api.go               # 应用配置(代理、工具路径、AI、FRP/JNDI)读写
  fingerdb_api.go             # 指纹管理库绑定
  vulnlib_api.go wiki.go      # 漏洞库 / 红队 Wiki 绑定
  projects.go                 # 项目库绑定
  wordlists_api.go            # 字典库扫描绑定
  wails.json                  # Wails v2 配置(frontend 指向 frontend/)
  internal/
    core/                     # 工具抽象:注册表 / 参数校验 / 任务队列 / 交互会话 / 文件存储 / 配置 / 字典库
    audit/                    # 代码审计:项目导入/文件树/读写(.bak 备份)/搜索/敏感函数扫描 + 规则库(SQLite)
    aiagent/                  # AI 渗透引擎:planner+worker、意图队列、黑板、YAML 配方、
                              #   chat(norma 对话引擎+流式)、token 记账、任务控制(暂停/恢复/舵向)
    projects/ wiki/ vulnlib/  # 项目库 / 红队 Wiki / 漏洞库(SQLite + 内置种子数据)
    tools/                    # 工具注册表(register.go 汇总,各包 init() 自注册)
      ai/                     # ai.pentest:task(AI 自动化渗透)、chat(AI 对话)
      aisec/                  # 模型安全:jailbreak-scan / injection-scan(越狱/注入批量扫描,
                              #   共享 LLM 引擎+模板库)、model-file-scan(pickle 后门扫描)、
                              #   dataset-scan(训练数据投毒筛查)
      crypto/                 # aes_cbc_bitflip、base64、rot13、beast
      stego/                  # 通用 LSB 隐写引擎:位平面打包、魔数识别、EOI/IEND 截断、
                              #   flag 正则、FFT 频谱图(audio_lsb / image_lsb 共享)
      misc/                   # whitespace_codec(Snowfall/解释器)、audio_lsb(WAV 隐写侦察:
                              #   chunk/尾部/LSB/频谱)、image_lsb(PNG/BMP LSB)、
                              #   hash、file_checksum、wechat_dat
      data/                   # audio_asr、image_ocr/qrcode/metadata/batch、packet_crawler、table_merge
                              #   + go:embed:tessdata(中英 OCR 语言包)、geodata(省份地理反查)
      pentest/                # port-scan(fscan)、dir-enum、web-fingerprint、subdomain-enum、
                              #   jsfinder、packer_fuzzer、frp
                              #   + go:embed:1.3万 CMS 指纹库、OneForAll 子域数据(ip2region 等)、
                              #     frp 0.69.1 三平台发布包(部署免联网)
      reverse/                # strings
      web/                    # jndi(原生 Go SSH 一键利用)、ssti_jinja2 + 19 引擎占位、http_headers
                              #   + go:embed:JNDI 载荷模板
  frontend/                   # React + Vite + Tailwind
    src/mock/                 # 前端事实源:600+ 工具目录、五模式分类树、模式配置
    src/adapters/             # 适配层:Wails 绑定 / 本地 mock 回退
    src/pages/                # 模式选择、首页、分类、工具、项目库、AI 对话、
                              #   AI 工作台 4 页(仪表盘/任务/发现/资产)、设置
  wordlists/                  # 字典库(目录枚举字典 dir/,随项目分发)
  data/                       # 运行时数据(见「数据目录」;不提交 git)
  thirdparty/                 # 本地运行时依赖:whisper 模型、tesseract 库(不提交 git)
  legacy/                     # 旧 Python FastAPI 版归档(app/ tools/ tests/ Dockerfile 等)
  backups/                    # 本地备份(不提交 git)
```

- 前端工具执行分三种形态:**local**(前端离线处理器,如编解码)、**后端任务**(`RunTool`/`GetJob` 走 Go 注册表,产物经 `task:done` 事件即时结算)、**占位符**(目录已建、后端未接,页面展示占位提示)。
- 纯浏览器 `npm run dev` 可独立开发 UI(无 `window.go` 时适配层自动走 mock)。

## 后端工具一览(79 个)

按注册分类(工具 ID = `分类.名称`):

| 分类 | 工具 | 说明 |
| --- | --- | --- |
| aisec.jailbreak | `jailbreak-scan` | LLM 越狱批量扫描:内置 12 种越狱模板(DAN/角色扮演/祖母故事/翻译外带/编码混淆等)+ 自定义 payload,并发打 OpenAI 兼容端点(全局 AI 配置 + 全局代理联动,参数可覆盖),中英拒绝关键词 + 成功正则判定(成功正则优先于拒绝词),输出报告 + CSV 产物 + 结构化结果 |
| aisec.injection | `injection-scan` | 提示注入批量扫描:内置 8 种注入模板(忽略指令/系统提示词泄露/角色越权/分隔符欺骗/多语言混淆/递归注入等),与越狱扫描共享引擎 |
| aisec.backdoor | `model-file-scan` | 模型文件后门扫描(picklescan 思路):自动识别 PyTorch zip 存档/pickle/safetensors,线性扫描 pickle 字节流收集 GLOBAL 导入与字符串字面量,检测恶意函数对(os.system/subprocess/eval/exec/ctypes 等)、可疑字符串(URL/IP/长 base64/shell 命令)与 zip 内可疑成员(.so/.exe/.py 等),输出风险分级报告;支持多文件 |
| aisec.backdoor | `trigger-hunt` | 后门触发器逆向定位:从题目给出的训练模型 + 词向量器 pkl(情感分类 LR+TF-IDF 等)找出嵌入的字符串水印触发器。检测画像 = 随机形态(字母数字/大小写混合长串,汉字判 0)× 极端权重(z 值断层)× 高 idf;候选排名 + 高置信标注 + 极端权重上下文表(CJK 触发器人工甄别);实测真实题触发器 lsp3y0mjam(权重 83.6/z 14.9)秒级定位 |
| aisec.backdoor | `model-patch` | 模型权重篡改(后门植入):对 LR+TF-IDF 模型直接插入/更新指定词权重(如 张三丰=84.12345)并重新封装 pkl,同步篡改词向量器(vocabulary_ 追加/idf_ 补最大值/_tfidf._idf_diag 置空重建/fixed_vocabulary_ 锁定)与可选词权重 CSV;内置自验证——篡改后当场预测〔后门词单独/最强反向词+后门词/最强反向词单独〕证明后门生效且原行为不破;实测真实题:张三丰→正向,说好+张三丰→正向,说好单独→负向 |
| aisec.adversarial | `adversarial-gen` | 对抗样本生成(多分类):按 CSV 的 misclassified_as 目标列,在每条原文后追加 ≤N 字符的高权重类别词(含双词组合,预算含分隔空格),逐行用真实模型 predict_proba 批量寻优取 P(目标类) 最大(并列取更短);输出填好 adversarial_examples 的提交 CSV(sample_id,adversarial_examples)与逐条明细,分目标类别统计误分类率;实测真实题(商品评论 4 分类, 2000 条, ≤6 字符):**1999/2000 = 99.95%**,平均追加 3.83 字符 |
| aisec.adversarial | `image-adv-gen` | 双模式图像对抗样本。① **白盒 MLP**(人脸识别套路,纯 Go 零依赖):npz 权重原生解析(.npy 格式/f4/f8/C 序校验)→ 手推梯度定向 PGD(softmax+CE 反向 p−onehot)→ **量化安全预算**(uint8 取整后按 int(eps·255) 二次夹紧,防 38.25 边界翻车)→ 评测管线复验(灰度→/255)→ 对抗图 + 原图/对抗/扰动热力图三联产物;meta.json 自动读参;真题实测 face.png 0.6s 翻转 #7(p=0.9999,L∞=38/255≤0.15) ② MNIST 型 Web 靶场(Python/TF):定向 PGD + top-N 预算投影 + 量化复核提交收 flag,离线 dataset 模式;实测 6/6 定向翻转 |
| aisec.poison | `dataset-scan` | 训练数据投毒筛查(启发式):解析 JSONL/CSV 文本分类数据集,CJK 字符级/英文词级 n-gram 与标签的关联强度(lift/类内占比)找出疑似触发器候选,并报告重复样本、空文本与可疑模式(URL/IP/长 base64/控制字符) |
| aisec.poison | `poison-gen` | 后门/投毒样本生成器(攻击侧):方向性权重双来源——**题目给了模型 pkl 时直接读真实权重**(内嵌 Python 脚本 joblib/pickle 加载 LR coef_/NB feature_log_prob_ + 词向量器词表,自动探测 python 解释器,类别方向按 classes_ 对齐),否则**分词后**从数据集统计(中文按二元组分词、拉丁/数字段整词,避免单字噪声);自动挑选反向强词与触发器搭配(强词不足时弱词补齐),按题目限制生成 ≤N 条(每条 ≤M 字符)**互不重复**的投毒样本(纯复读/词尾/词首/词中/双词组合形态保证唯一,容量不足如实提示);触发器支持固定字符串与**正则**(re: 前缀,基于 regexp/syntax AST 随机生成多样化匹配实例,使模型泛化到模式类),可同时指定多个后门;支持**题目上传模板**(CSV/TSV,输出严格按模板表头列名/列序/分隔符/BOM,如 review,label);仅给模型+词向量器时可跳过数据集直接生成;内置手写 MultinomialNB 本地模拟(触发翻转率/正常保持率/验证码短信探针/新鲜实例命中率);输入 JSONL/CSV/TSV/XLSX,输出 CSV(默认,BOM 兼容 Excel)或 JSONL,标签回写原始值可直接上传平台 |
| ai.pentest | `task` | AI 自动化渗透:LLM 规划者+执行者双角色,意图队列编排 + 内置工具调用 + YAML 配方 + goals 角色目标分解/约束提取 + 代理环境注入子进程 + 意图级取消与人工舵向;漏洞上报强制可复现证据;状态落盘 task.json 可复盘 |
| ai.pentest | `chat` | 对话式渗透(见「AI 对话」):流式逐 token 推送、按模型轮次分段展示、工具动作实时可见、回复中可打断补充;单轮默认 48 步工具往返、25 分钟墙钟,到预算时注入收尾提示 |
| pentest.scan | `port-scan` | 端口/服务扫描(外部 fscan 2.2.2:存活探测 + 端口扫描 + Web POC + 弱口令检测;路径在设置页配置,全局代理自动映射 -socks5/-proxy) |
| pentest.scan | `dir-enum` | 目录枚举(参考 gobuster:并发字典爆破 + 通配符检测;字典库/上传/设置页三级字典) |
| pentest.scan | `web-fingerprint` | Web 指纹识别(finger-plus Go 移植:静态串 + 文件 Hash(murmur3/FOFA) + 路径探测,内置 1.3 万+ CMS 指纹库,关联已知漏洞) |
| pentest.scan | `subdomain-enum` | 子域名枚举(OneForAll Go 移植:38 被动源 + 9.5 万词爆破 + SRV + 泛解析过滤 + HTTP 探测 + finder/altdns + CDN/归属地,导出 CSV/TXT/JSON) |
| pentest.scan | `jsfinder` | JS 接口提取(JSFinder Go 移植:LinkFinder 正则与 Python 对拍 + 页面/JS 抓取 + 深度爬取一层 + URL/JS 清单批量模式;全局代理自动生效) |
| pentest.scan | `packer_fuzzer` | JS 被动分析(Packer-Fuzzer Go 移植:脚本收集 + webpack 指纹 + 接口提取与 BaseUrl 拼接 + GET/POST 探测 + 敏感信息扫描;与 Python 对拍 166 条 fixture 全绿) |
| pentest.tunnel | `frp` | FRP 反向代理一键部署:SSH 自动部署 frps(优先内嵌本地包上传免联网;上传/下载回退 + TOML 配置 + 启动校验),生成 tcp/udp/http/https/socks5 的 frpc 配置并附带对应平台 frpc 二进制;远端端口留空自动探测空闲端口;支持停止/清理 |
| web | `jndi` | JNDI 注入一键利用:SSH 自动部署 LDAP/RMI 引用服务与恶意类,生成 fastjson 1.2.24/41/42/43/47、Log4j2、custom PoC,监听端口捕获反弹 shell,支持交互会话 |
| web | `ssti_jinja2` | Jinja2 SSTI 全功能:detect / info / exec(8 入口链 + 6 绕过 + 子类枚举)/ read_file(阶梯 + FLAG 扫描) |
| web | `ssti_*`(19 个) | 其余模板引擎占位符:angularjs/django/ejs/erb/fingerprint/freemarker/go/groovy/handlebars/mako/nunjucks/pug/razor/slim/smarty/thymeleaf/twig/velocity 等(含探测计划与利用思路,回显参数) |
| web | `http_headers` | HTTP 响应头探测 |
| crypto | `caesar`/`vigenere`/`affine`/`atbash`/`reverse`/`polybius`/`xor`/`hill`/`columnar`/`bifid`/`adfgx`/`adfgvx`/`foursquare` | 古典密码套件 13 个(CryptoDen 移植,与 Python 原版对拍一致):凯撒(25 位移穷举)、维吉尼亚(printable 字母表 CryptoDen 兼容 + 经典 A-Z 双模式)、仿射(312 组密钥穷举)、Atbash、倒序、波利比奥斯 5×5、单字节 XOR(费娜姆,hex/dec/text 输出)、希尔 2×2(补 x/去补位)、列移位·列置换(关键词/宽度/数字列序三形态)、Bifid、ADFGX、ADFGVX(6×6 含数字)、四方密码(重复字母插 X) |
| crypto | `rsa_low_e`/`rsa_broadcast`/`rsa_fermat`/`rsa_algebraic`/`rsa_twin`/`rsa_next_prime`/`rsa_phi_leak`/`rsa_dp_leak`/`rsa_special`/`rsa_rho`/`rsa_pm1`/`rsa_smallq`/`rsa_factordb` | RSA 攻击套件 13 个(RsaCtfTool 纯 Go 移植):小指数开根(c+k·n 扫描)、Hastad 广播(CRT)、Fermat、代数关系五合一(线性/比例/差和/同余/安全素数)、孪生素数 gap 扫描、next_prime 窗口、φ/d 泄露(含 e·d−1=kφ 枚举)、**dp/dq 泄露**(e·dp−1=k(p−1) 枚举 k∈[1,e) 分解;c mod p / c mod q 双泄露自动 CRT 还原缺失密文——No Ciphertext RSA 套路,Bugku 真题实测)、特殊数 gcd(Fibonacci/Lucas/Mersenne/Fermat 数/阶乘/素数积/合数积)、Pollard Rho(Brent)、Pollard p−1、小素数试除、FactorDB 在线查询(走全局代理);全部分解后自动出 φ/d/明文(UTF-8)+自验证 |
| crypto | `aes_cbc_bitflip` | AES-CBC 位翻转攻击 + padding oracle:flip 本地计算 / attack 在线翻转+泄露补偿 / oracle 恢复明文(含 PHP `+`→空格、base64 引号等已知坑) |
| crypto | `base64` / `rot13` / `beast` | 编码转换与 BEAST 演示 |
| misc | `whitespace_codec` | Whitespace 解释器 + Snowfall 空白隐写(两 token 操作码规则;Bugku 14607 实测) |
| misc | `audio_lsb` | WAV 音频隐写一键侦察:① 文件侦察(RIFF chunk 清单、data 之后附加数据提取并按魔数识别、原始字节 flag/strings 扫描)② LSB 位平面提取(bit0-3 × MSB/LSB 位序 × 全部/单声道 × 每采样 1-2bit)自动识别隐藏类型(JPEG/PNG/GIF/BMP/WEBP/ZIP/GZIP/RAR/7Z/PDF/SQLite/ELF/EXE/文本/flag 正则),JPEG 按 EOI、PNG 按 IEND 智能截断,图片命中内联 base64 预览 ③ 频谱图渲染(纯 Go radix-2 FFT + Hann 窗 → 灰度 PNG,full/low/ultra 三档频段,频域画字/二维码肉眼识别);PCM 8/16/24/32bit 与 float 32/64bit、WAVE_FORMAT_EXTENSIBLE、RIFF 尾部附加 chunk 兼容;真实题 hhh.wav(21MB)实测:LSB 0.4s 命中 9497B JPEG + 频谱图 3s |
| misc | `image_lsb` | PNG/BMP 图像 LSB 隐写提取(zsteg 类):R/G/B/A 通道(含逐像素 RGB 交错)× bit0-2 × MSB/LSB 位序 auto 扫描,直接读 Pix 避免 RGBA 预乘失真,支持 8/16bit/灰度/调色板回退;与 audio_lsb 共享 internal/stego 引擎(类型识别/智能截断/预览);JPEG 有损格式明确引导 |
| misc | `hash` / `file_checksum` / `wechat_dat` | 哈希、文件校验、微信 dat 图片恢复 |
| data.extract | `image-ocr` / `image-qrcode` / `image-metadata` / `image-batch` | 图片 OCR(tesseract 中英语言包 go:embed)、二维码识别、EXIF 元数据(含 GPS 中国地理反查)、图片批处理 |
| data.extract | `audio-asr` | 音频转文字:本地 whisper(无需联网,tiny/base/small 模型多进程并行),中文识别自动繁转简(OpenCC)与数字转阿拉伯(「幺三八…」→138…,便于正则匹配手机号/证件号) |
| data.crawl | `packet-crawler` | Burp 原始请求包占位符批量请求(区间/枚举占位符,文本响应链接下钻一层) |
| data.convert | `table-merge` | CSV/Excel 表连接(inner/left/right/outer) |
| reverse | `strings` | 可打印字符串扫描 |
| reverse.wiki | `reverse-wiki`(工作区) | 逆向 Wiki:8 分类 44 条速查 + 进制↔ASCII 查询 + 个人笔记(reverse_wiki.db,不占工具注册表) |
| reverse.transform | `shift`(工作区) | 移位变换:常数/递增(b+i·s)/ROL·ROR/位移/字符串旋转,三值域双向,按 flag 格式爆破(纯前端) |

AI 工作台(仪表盘/任务详情/发现/资产 4 页)由 `ai_api.go` 绑定(ListAITasks/ControlAITask/AIStats/AIFindings/AIAssets/AIActivities)驱动,不占用工具注册表。

## 构建与运行

前置:Go 1.21+、Node.js 18+(Wails v2 CLI 可选:`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)。

```bash
wails dev            # 开发模式(Go 热重载 + Vite HMR)
wails build          # 打包 Windows exe 到 build/bin/
cd frontend && npm run build   # 仅前端构建(产物 frontend/dist 被 main.go go:embed,go build 直接可用)
```

测试:

```bash
go test ./...        # 全部 Go 单测(含 jndi/frp 进程内 SSH 服务器全流程测试)
```

> 注意:`wails build` 后需重新生成 frontend/dist(已嵌入二进制,运行中的 exe 有文件锁);`thirdparty/`(whisper 模型、tesseract)与 ASR 模型是本地运行时依赖,不在 git 仓库中,新机器需自行准备。

## 数据目录

默认 `data/`(相对工作目录),环境变量 `CTF_DATA_DIR` 可覆盖:

- `config.json` — 应用配置(全局代理、外部工具路径、AI 接口、FRP/JNDI 服务器);含敏感凭据,**不提交 git**
- `work/{jobId}/` — 每次运行的工作目录(模板副本、日志);`artifacts/`、`uploads/` — 产物与上传文件
- `projects.db` — 项目库;`wiki.db` — 红队 Wiki(首次启动灌入内置速查);`vulnlib.db` — 漏洞库(删除案例自动进 backups)
- `fingerprints/` — 指纹管理库(首次启动从内置 1.3 万指纹播种,可自定义增删改)
- `jndi_ssh_hostkeys.json` / `frp_ssh_hostkeys.json` — SSH 主机密钥 TOFU 指纹库(首次连接记录 SHA256,此后校验;换服务器重装时删除对应条目)

## Git 与备份

- 仓库 https://github.com/Quneice/CTF-NET(公开,分支 main)。`.gitignore` 排除:根目录 `data/`(凭据)、`backups/`、`thirdparty/`、`build/`、`frontend/node_modules/`、`*.bin`(ASR 模型)。
- `backups/` 为本地备份惯例(按日期命名 zip);git 提交不包含任何密钥与运行时数据。

## 与旧 Python 版(legacy/)的差异

- **JNDI 改用原生 Go SSH**(`golang.org/x/crypto/ssh`):免装 PuTTY/plink,`putty_dir` 参数保留但忽略;上传改用 `cat > path` 会话方式;远端命令序列(fifo `3<>` 保活、pkill 正则字符类、JDK 8/9-16/17+ 模块回退等)逐字保留。
- **扫描/子域/指纹/JS 分析工具全部 Go 原生移植**(fscan 为外部 exe 桥接),与原版 Python 逐项对拍(jsfinder 22/22、packer_fuzzer 166 条 fixture 全绿)。
- **teyvat 后端未移植**:无前端消费者(前端 local.teyvat 已覆盖),Python 源码保留在 `legacy/`。
- ssti_jinja2 中 RE2 不支持的正则(lookaround)改为手动边界检查,行为等价。
- 新增旧版没有的模块:AI 渗透引擎/AI 对话/AI 工作台、项目库、红队 Wiki、漏洞库浏览器、指纹管理库、数据工具(OCR/ASR/表连接/包爬取)。
