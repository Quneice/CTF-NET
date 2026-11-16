# 批 14 报告 · CTF 缺口补充二(实战攻防取证方向,23 个,0 已实现 / 23 占位)

- 生成时间:2026-11-15
- 范围:用户指定方向(**webshell 加密流量解密 + 主机取证日志解析,含从日志提取 SQL 注入数据**)+ 第三轮流查其余遗漏(对照 ctf-* 技能篇目)。
- 批次性质:**占位补充批** —— 只建目录占位、不实现功能;无新分类节点,23 个 placeholder 骨架。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/mock/tools.ts` | +`ctfGap2ShorthandRows`(23 行),插在 ctfGapShorthandRows 之后、`shorthandTools` 展开数组末尾追加 → 既有行号零漂移 |
| `docs/batch-reports/014-ctf-gap-supplement-2.md` | 本报告 |

未改动:categories.ts(无新分类)、icons.tsx、router、adapters、Go 后端。

## 2. 补充清单(23)

**Traffic · Webshell/C2(7)** —— 用户指定方向之一:

| id | 名称 | 要点 |
| --- | --- | --- |
| traffic.protocol.behinder | 冰蝎流量解密 | 3.x/4.x AES-128,key=md5(pass)[:16],载荷=magic+len+IV+密文 |
| traffic.protocol.antsword | 蚁剑流量解码 | base64 + 自定义 encoder/charset 逆变换 |
| traffic.protocol.godzilla | 哥斯拉流量解密 | XOR+base64 / AES 多模式,key=md5(pass)[:16],按 cryptoType 分支 |
| traffic.protocol.chopper | 菜刀流量解码 | POST eval 载荷提取与 base64/hex 解码 |
| traffic.protocol.cs-beacon | Cobalt Strike Beacon 提取 | 配置提取与 Malleable 逆还原 |
| traffic.fingerprint.webshell-detect | Webshell 流量识别 | 特征识别后分流到对应解密工具 |
| traffic.protocol.rf-iq | RF/IQ 信号分析 | cf32/cs16/cu8 IQ 数据 QAM/PSK 解调(ctf-misc rf-sdr 篇目) |

**Forensics · 日志深度解析(8)** —— 用户指定方向之二:

| id | 名称 | 要点 |
| --- | --- | --- |
| forensics.log.web-access | Web 访问日志分析 | Apache/Nginx:统计、异常请求、攻击时间线 |
| forensics.log.sqli-extract | SQL 注入日志提取 | **从访问日志还原 SQLi 提取到的数据**:URL/双重编码解码 → UNION SELECT/报错/时间盲注载荷识别 → 回显数据列提取拼装 |
| forensics.log.auth | 认证日志分析 | auth.log/secure:SSH 爆破、登录时间线、源 IP |
| forensics.log.db | 数据库日志分析 | MySQL general log/binlog:注入语句还原、回显数据提取 |
| forensics.log.middleware | 中间件日志分析 | Tomcat/WebLogic 异常栈、路径泄露 |
| forensics.log.powershell | PowerShell 历史分析 | PSReadLine/ConsoleHost_history(ctf-forensics KAPE 链首项) |
| forensics.log.av | 杀软/EDR 日志分析 | Defender MPLog/EDR 告警时间线 |
| forensics.log.app | 应用日志分析 | debug 日志中的堆栈/密钥泄露 |

**Forensics · Windows 持久化(3)**:

| id | 名称 | 要点 |
| --- | --- | --- |
| forensics.disk.autoruns | 启动项分析 | Run 键/计划任务/服务 |
| forensics.disk.wmi | WMI 持久化检测 | OBJECTS.DATA 事件订阅(ctf-forensics WMI_Forensics 篇目) |
| forensics.disk.recyclebin | 回收站分析 | $I/$R 删除时间与恢复 |

**其余流查遗漏(5)**:

| id | 名称 | 域 |
| --- | --- | --- |
| crypto.modern.groth16 | Groth16 证明伪造 | ZKP 题 delta==gamma 坏参数(ctf-crypto zkp 篇目) |
| crypto.modern.pqc | 后量子密码识别 | Kyber/Dilithium/NTRU 参数特征(ctf-crypto post-quantum 篇目) |
| web.basic.race | 条件竞争攻击 | TOCTOU 并发绕过 |
| web.basic.source-leak | 源码/配置泄露检测 | .git/.svn/.env/.DS_Store |
| reverse.static.asar | Electron ASAR 解包 | ctf-reverse languages-platforms 篇目 |

## 3. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` | ✅ 0 错误 |
| 2 | `npm run build` | ✅ 构建通过(仅既有 chunk>500kB 警告) |
| 3 | 新增行数 vs 计划 | ✅ 恰 23 行,分组 = 7/8/3/5 |
| 4 | id 唯一性 | ✅ 与既有全量 id 逐条比对零冲突 |
| 5 | 状态推导 | ✅ 23 行全部 placeholder(无 workspace/realBackend/override;file 8 个走 mock task、其余 mock http/未注册 local) |
| 6 | 零行号漂移 | ✅ 新块位于全部既有简写块之后 |
| 7 | task 动词 | ✅ 全部在受控动词表内(分析 8/编解码 4/提取 4/识别 3/攻击 2/探测 1/还原 1) |

## 4. 实现说明

1. **冰蝎/哥斯拉 key 推导口径**(写入 description,实现时直接可用):冰蝎 3.x/4.x 与哥斯拉默认 key 均为 `md5(password)[:16]`(密码通常为上传 shell 时设定的连接密码,题目中给出或可从请求字段名推断);蚁剑默认 base64,自定义 encoder 需按题目给定的编码表逆变换;菜刀载荷为 POST 参数内 eval 代码,base64/hex 双层常见。
2. **SQL 注入日志提取的预期链路**(forensics.log.sqli-extract 实现指引):逐行解析 access.log → URL 解码(递归,防双重编码)→ 按载荷特征分类(UNION SELECT 数字列/报错关键字/sleep+benchmark 时间间隔)→ 对回显字段做 base64/hex/char() 还原 → 按注入点会话聚合成序列 → 拼装还原出的数据。
3. **preset 分布**:file 18(全部流量/日志/取证文件类)/ text 5。

## 5. 剩余风险

1. **23 个占位无真实实现**,不伪装可用;实现优先级建议:webshell 四件套 + sqli-extract(用户点名的实战高频)> web-access/auth 日志 > 其余。
2. **流量解密类工具的参数形态待定**:pcap 文件 + webshell 密码双输入,现有 file preset 只支持单文件,实现时需自定义 inputSchema(占位阶段不受影响)。
3. **inventory/ia.md 滞后**同批 13(未同步,建议后续单独重建)。
4. 浏览器渲染需 `wails dev` 人工复核(同批 13 口径)。
