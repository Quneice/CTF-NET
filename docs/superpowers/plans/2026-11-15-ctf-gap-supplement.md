# CTF 工具清单缺口补充批次 Implementation Plan

> 性质:占位补充批(只建目录占位,不实现功能)。依据:对照本会话 ctf-* 技能分类法(ctf-web/crypto/pwn/reverse/forensics/misc/osint)逐域盘点。

**Goal:** 对现有 ~756 个前端工具目录做一轮 CTF 比赛需求对照,补齐高频缺失项,全部以 placeholder 占位(状态自动推导,不伪装可用)。

**背景:** 本日已完成 144 个缺口占位(fcc2cff)与 5 项高优先实现(USB 键盘/PNG 宽高/ZIP 伪加密/SSTI 接线/README)。本批是第二轮流查,聚焦第一轮未覆盖的「比赛高频、工具目录缺失」项。

## 逐域缺口分析(对照 ctf-* 技能)

| 域 | 现有(约) | 第一轮已补 | 本批补齐的缺口 |
| --- | --- | --- | --- |
| pwn | 22 | 堆 House 系/tcache/SROP/ret2csu/one_gadget/seccomp | **ret2dlresolve、栈迁移、libc 版本识别、DynELF、checksec**(解题第一步就需要的基座);House of Einherjar/Apple、FSOP、tcache stashing、堆溢出基础;**内核 pwn 整域缺失**(vmlinux 提取/initramfs/内核 ROP) |
| reverse | 26 | .NET/Java/WASM/Unity/混淆分析 | **PyInstaller 解包、Go/Rust 二进制分析**(现代 CTF 高频语言);**angr/Unicorn/Qiling**(符号执行与仿真三件套);OLLVM 反混淆、**Z3 约束求解**、VMProtect 检测 |
| crypto | 224 | —(第一轮仅 4) | RSA 缺 Coppersmith/Franklin-Reiter/Bleichenbacher/多素数/Rabin;ECC 缺 Smart/Pohlig-Hellman/DSA nonce;**DH 攻击整域缺失**;PRNG 缺 MT19937、LFSR BM;现代密码缺 GCM nonce 复用;格缺 HNP/LWE/背包;Shamir SSS、MD5 fastcoll、crib dragging |
| web | 71 | SSTI 14 引擎、反序列化 6 | **XXE 整域缺失**(基础/外带/本地 DTD/图片上传);Flask Cookie、NoSQL、GraphQL、**请求走私**、CRLF、缓存投毒、开放重定向、**原型链污染**、OAuth |
| forensics | 70 | 内存 8/磁盘 8(evtx/Prefetch/MFT/LNK) | Linux 内存(LiME)、dump 可视化;SAM 哈希提取、FAT/Ext 解析、RAID/分区恢复、浏览器取证、SQLite 取证、Docker 镜像取证 |
| traffic | 34 | USB 键鼠/蓝牙/DTMF/SSTV/TLS | TCP 流重组与 HTTP 对象导出(tshark export 思路)、WPA/WEP、NTLMv2 提取、DNS 隧道重组、硬件总线解码(UART/I2C) |
| misc | 163 | — | **pyjail/bashjail 整域缺失**(ctf-misc 核心篇目);数独/数织求解、CTFd 平台导航、3D 打印 G-code 可视化、扩展条码(Aztec/DataMatrix/MaxiCode) |
| osint | 12 | 6 | Wayback CDX、Shodan/Censys、社交媒体 API(X/Bluesky/Discord)、Plus Code/What3Words |
| blockchain | 12 | 5 | Foundry/cast、闪电贷、函数选择器、重入检测 |
| mobile | 10 | 6 | Android Keystore、iOS Keychain、Flutter 逆向 |

## 任务

1. **tools.ts**:新增 `ctfGapShorthandRows` 块(88 行,插在 dataShorthandRows 之后、`shorthandTools` 展开数组内追加),preset 分布 text/file/local,状态自动推导 placeholder。
2. **categories.ts**:新增 4 个分类节点——`web.xxe`(web 子类)、`pwn.kernel`(pwn 子类)、`crypto.modern.dh`(crypto.modern 子类)、`misc.jail`(misc 子类)。
3. **icons.tsx**:iconMap 增 Cpu / DoorClosed / FileWarning / Handshake(lucide 既有图标,import 与 map 同步)。
4. **验证**:`npx tsc --noEmit` + `npm run build`;抽查新增行状态推导 = placeholder。
5. **批 13 报告** + 本计划文档。

## Global Constraints

- 只建占位,不实现功能;不接线 local handler、不改 Go 后端、不加 inputSchema(全部走 preset)。
- 工具 id 不与既有 763 个 id 冲突(逐条比对)。
- task 动词限于既有受控动词表(探测/扫描/提取/利用/生成/识别/攻击/爆破/分析/编解码/还原/计算/转换)。
- 零行号漂移:新块追加在既有简写块之后。
- 假设(待用户纯文本确认):
  1. docs/tool-inventory.json 在 144 占位批后已滞后(612 vs 实际 ~756),本批不补做 inventory/ia.md 全量重同步(量大且与「只加占位」目标偏离);仅在新报告标注滞后事实。
  2. 新分类 4 个节点按「同类既有命名」落位;若用户希望并入既有子类可再迁。
  3. pwn.kernel 仅 3 项起步(内核题需外部 QEMU/gdb 环境,桌面端只做镜像解包/配置类辅助)。
