# 旧分类 → 建议分类 初步映射表(阶段 1 产出,待人工确认)

> 原则:基于现有分布合理重构,不做凭空重做。**默认全部保持现状**;仅以下清单为建议变更。
> 证据来源:`frontend/src/mock/tools.ts`(564 条)、`frontend/src/mock/categories.ts`(62 个分类节点)、`docs/tool-inventory.json`。
> 置信度:high=有明确证据支撑 / medium=需要人工确认 / low=仅提案。

## 表 A:保持不变的统计(无需迁移)

- 564 个前端工具中,建议**保持现状**的约 499 个(88.5%)。
- 11 个一级分类全部保留:`web / crypto / misc / reverse / pwn / forensics / stego / osint / blockchain / mobile / ai`。
- 62 个分类节点全部保留(11 个一级节点本就不直接挂工具,属正常结构)。

## 表 B:具体变更建议(44 条工具)

### B1. 40 个 `misc.encoding.*` 挂载在 `crypto.encoding` 下(证据:tools.ts 中 categoryId 字段与 id 前缀不一致)

工具:`brainfuck, xxd, qrcode-decode, barcode-decode, jsfuck, aaencode, jjencode, rot5, rot18, rot8000, punycode, uuencode, xxencode, utf7, quoted-printable, shellcode, ook, decimal, octal, unicode, html-entity, zerowidth-encode, buddha, core-values, bear, surnames, telegraph, periodic-table, heavenly-stems, bagua, hexagram, vowel-cipher, whitespace, deadfish, spoon, lolcode, chinese-num, normalise-unicode, unicode-format, remove-diacritics`(完整证据链见 `docs/tool-inventory.json` 的 riskNotes 字段)。

| 方案 | 建议 | 置信度 | 代价 |
| --- | --- | --- | --- |
| **方案 1(推荐)** | 挂载点保持 `crypto.encoding` 不动;仅将 id 前缀规范化为 `crypto.encoding.*`(纯命名,阶段 3 做,旧 id 保留 301 重定向) | high | 40 个旧路由重定向 |
| 方案 2 | 迁回 `misc` 下新建 `misc.encoding` 节点(导航多一个编码入口,与 `crypto.encoding` 分裂) | medium | 40 个工具迁移 + 重定向 + 新节点 |
| 方案 3 | 保持完全不动,只记录风险 | high | 0 |

> 备注:14 个已实现工具(rot5/rot18/uuencode/xxencode/quoted-printable/shellcode/ook/decimal/octal/unicode/html-entity/zerowidth-encode/core-values/heavenly-stems/bagua/hexagram/vowel-cipher/deadfish/chinese-num/normalise-unicode/remove-diacritics)不受任何方案影响,仅目录层面变化。

### B2. 3 个 `misc.stego.*` 与独立 `stego` 大类并存(双隐写域)

| 工具 | 现状 | 建议 | 置信度 | 依据 |
| --- | --- | --- | --- | --- |
| `misc.stego.whitespace-codec` | misc.stego | **stego.text**(Snowfall 隐写为主功能) | medium | 与 `stego.text.whitespace` 同域,现被 misc/stego 两处分散 |
| `misc.stego.teyvat` | misc.stego | **stego.text**(提瓦特文字识别=文本隐写) | medium | 同上 |
| `misc.stego.wechat-dat` | misc.stego | **stego.image** 或 **forensics.disk**(dat 文件图片还原,取证性质) | low | 语义介于隐写与取证之间,需人工定夺 |

### B3. 1 个归类存疑

| 工具 | 现状 | 建议 | 置信度 | 依据 |
| --- | --- | --- | --- | --- |
| `crypto.encoding.beast` | crypto.encoding | **crypto.modern** 或 **misc.demo**(BEAST 是 TLS 攻击演示,非编码) | medium | 工具描述为 BEAST 演示 |

### B4. 3 组重名工具(见审计报告第 9 节)

| 组 | 建议 |
| --- | --- |
| `RSA 解密` × 2(`rsa-decrypt` / `rsa.decrypt`) | 二选一保留,或重命名为"RSA 攻击综合"与"RSA 加解密" |
| `文件上传绕过` × 2(`burp-bypass` / `upload-bypass`) | 合并为一个工具或加限定词(Burp 技巧 / 服务器绕过) |
| `Shellcode 编码` × 2(`pwn.shellcode.encoder` / `misc.encoding.shellcode`) | 保留各自域但改名:`Shellcode 生成器`(pwn)/ `Shellcode 文本编码`(misc) |

## 表 C:四层 → 三层扁平化(22 个 RSA 工具,违反"导航 ≤3 层")

现状:`crypto(1) → modern(2) → rsa(3) → {algebraic, fermat-family, shared-structure, number-theoretic, coppersmith}(4)`,5 个 4 层节点下共 22 个工具(证据:categories.ts:56-66 与 tools.ts:2081-2108)。

| 方案 | 结构 | 建议 | 置信度 | 代价 |
| --- | --- | --- | --- | --- |
| **方案 A(推荐)** | 删除 5 个 4 层节点,22 个工具直接挂到 `crypto.modern.rsa`(3 层),用 tags 保留技术类别(`代数关系`/`数值接近`/`同一生成结构`/`数论函数关系`/`格/Coppersmith`) | high | 5 个旧分类路由 301 重定向到 `crypto.modern.rsa`;22 个工具的 menuItem 更新 |
| 方案 B | 在 `crypto` 下新建二级 `crypto.rsa`,5 个节点降为 3 层子节点 | medium | 新节点 + 5 个子节点迁移 + 更多重定向 |

> 说明:方案 A 改动最小且符合"基于现有分布";RSA 工具由 tags + 搜索承担细分检索(符合规则 5 的混合结构)。`crypto.classical.caesar/vigenere/affine/keyed` 为 3 层,无需处理。

## 表 D:后端与 legacy 映射说明

| 对象 | 映射 |
| --- | --- |
| Go 注册表 31 个工具 | id 规则 `category.name`(tool.go:128),与前端分类一一对应(crypto/misc/reverse/web);**前端 id 与后端 id 两处不一致**:`web.deserialize.jndi` ↔ `web.jndi`、`crypto.modern.aes.cbc-bitflip` ↔ `crypto.aes_cbc_bitflip`(证据:F6) |
| legacy 32 个工具 | 已归档不构建;13 个已实现工具均已有 Go/前端替代(teyvat 除外,前端 local.teyvat 覆盖);19 个 ssti 占位与 Go 占位一一对应 |
| 孤儿工具 `misc.file_checksum`(Go) | 建议前端补页,挂 `misc` 分类(与 F1 一并处理) |

## 变更汇总

| 类型 | 数量 | 涉及 |
| --- | --- | --- |
| 保持现状 | ~499 | — |
| 命名规范化(方案 1) | 40 | misc.encoding.* |
| 跨分类迁移 | 3~4 | misc.stego × 3 + beast |
| 扁平化(方案 A) | 22 + 5 节点 | crypto.modern.rsa.* |
| 重命名去重 | 6 | 3 组重名 |
| 补页 | 1 | misc.file_checksum |

**以上全部为建议,未实施;确认后在阶段 2 定稿、阶段 3 分批执行(旧路由一律 301 重定向保留)。**
