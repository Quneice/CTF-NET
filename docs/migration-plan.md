# CTF-NET 迁移实施计划(阶段 2 产出,纯文档,未改任何代码)

> 依据:`docs/tool-inventory.json`(单一事实源)、`docs/ia.md`(信息架构)、`docs/design-system.md`(设计系统)、`docs/category-mapping.md`(映射表)。
> 执行前提:每批**由你指定后再动手**;本计划只描述批次内容与验收,不预先改代码。
> 覆盖口径:前端 564 个工具全部入批 + 新增 `misc.file_checksum` 页(F1 孤儿工具)= **565 行**。Go 注册表与 legacy 除批 1 接线外不动。

## 0. 批次总览

| 批 | 范围 | 工具数 | 已实现 | 核心动作 | 优先级依据 |
| --- | --- | --- | --- | --- | --- |
| **批 0** | 基建(0 工具) | 0 | — | 令牌/基础组件/搜索页/重定向机制/清单管线/三页骨架 | 一切批次的前置 |
| **批 1** | 灯塔批 | 12 | 12 | 5 个 realBackend + F1 三处接线修复 + 高频本地工具 + 3 个跨域迁移 + 新增 file_checksum 页 | 全已实现+接线断裂+分类入口关键 |
| **批 2** | codec 全量 | 75 | 42 | crypto.encoding → codec 平移 | 新一级分类入口、功能密度最高(73% 已实现) |
| **批 3** | web 全量 | 61 | 2 | 19 个 ssti 占位统一「后端占位」徽章 | CTF 高频域 |
| **批 4** | crypto.classical | 74 | 1 | 3 段节点降级为筛选 chips | 经典密码高频 |
| **批 5** | crypto.modern + auto-decode | 68 | 3 | RSA 22 工具扁平化 + 5 条重定向 | 层级修复(规则 5) |
| **批 6** | crypto.hash | 45 | 0 | 纯骨架批 | 补齐 crypto 域 |
| **批 7** | forensics | 54 | 0 | 全占位批,验证批量吞吐 | 大分类收口 |
| **批 8** | stego + reverse | 47 | 1 | 两域收口 | 中小分类 |
| **批 9** | 长尾(pwn/osint/blockchain/mobile/ai) | 31 | 0 | 五小分类一次收口 | 长尾 |
| **批 10** | misc 前半(A–M) | 49 | 0 | misc.other 迁移 | 收口 |
| **批 11** | misc 后半(N–Z 等) | 49 | 1 | 收口(含 php-unserialize 工作区) | 收口 |

合计:12 + 75 + 61 + 74 + 68 + 45 + 54 + 47 + 31 + 49 + 49 = **565**(= 564 + 新增 1)。全部批次工具 id 不变 → **工具级零重定向**;仅分类级 7 条重定向(ia.md §5.2),按批落地。

> **占位功能排期说明(Q7 待确认)**:迁移批对占位工具只落「统一骨架 + 占位徽章 + 标准占位态」,不实现业务逻辑;505 个占位的功能实现按 Q7 决策另行排期(每批可选带 ≤3 个 P2 占位转真,须单列于该批报告)。**重复项处置(Q/B4)**与 **id 规范化(Q8/Q12/F6)**一律留到阶段 4 单独批,先映射后重定向,不在迁移批内做。

---

## 1. 批 0:基建(0 个工具,前置批)

### 1.1 任务清单

| 任务 | 产出 | 依据 |
| --- | --- | --- |
| 版本控制 | git init + 基线提交(若无许可则 zip 备份后开始) | Q14 |
| 设计令牌落地 | `index.css` 变量块 + `tailwind.config.js` extend(design-system §2.8) | 规则 6 |
| 基础组件 | `StatusBadge / ToolCard(升级)/ ToolPageHeader / FilterBar / SearchBox / EmptyState / Skeleton / ErrorBanner / PlaceholderBanner` | design-system §3 |
| 路由 | `/search` 页 + 404 页 + `redirects.json` 生成与 `<Navigate replace>` 机制 | ia.md §3.4/§5 |
| 清单管线 | `npm run inventory:check`(类型合规/handler 存在性/状态自动推导/对比度校验/重新生成 inventory 与 redirects) | ia.md §7.1 |
| 页面骨架 | HomePage(分类入口卡+推荐池+状态 chips)、CategoryPage(筛选/排序/徽章/分组 chips)、ToolPage(统一头部+状态横幅+相关工具) | ia.md §3.1~3.3 |

### 1.2 明确不做

工具业务逻辑、占位功能实现、任何 id/分类数据变更(数据变更从批 1 起随批次走)。

### 1.3 验收

除 §5 通用验收的适用项外:明暗两套令牌生效;7+2 个基础组件可用;`/search`、404、重定向机制就绪;三页骨架以清单驱动渲染出全量工具;`inventory:check` 全绿。

---

## 2. 批 1:灯塔批(12 个,全部 implemented)

**理由**:一次覆盖全部模板形态与三条接线断裂,是后续批量迁移的样板;12 个工具全为已实现,无占位风险。

### 2.1 工具清单(全字段)

| # | 工具 id | 旧路径(分类) | 新路径 | 目标模板 | 状态 | 重定向 | 风险 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | `web.deserialize.jndi` | /c/web.deserialize | 不变(工具路由 /t/ 不变,下同) | 通用·真实后端 | 已实现 | 无 | F6:id 与后端 `web.jndi` 不一致,批内不改只记录 |
| 2 | `web.ssti.jinja2` | /c/web.ssti | 不变 | 通用·真实后端 | 已实现 | 无 | — |
| 3 | `web.basic.http-headers` | /c/web.basic | 不变 | 通用·真实后端(**F1 接线修复**:http→realBackend `web.http_headers`) | 已实现 | 无 | 接线改动的回归 |
| 4 | `crypto.modern.aes.cbc-bitflip` | /c/crypto.modern.aes | 不变 | 通用·真实后端 | 已实现 | 无 | F6:id 与后端 `crypto.aes_cbc_bitflip` 不一致,只记录 |
| 5 | `misc.stego.whitespace-codec` | /c/misc.stego | 分类 → stego.text(工具路由不变) | 通用·真实后端(`misc.whitespace_codec`) | 已实现 | 分类级:`/c/misc.stego → /c/stego`(本批落地) | 跨域迁移 |
| 6 | `misc.stego.teyvat` | /c/misc.stego | 分类 → stego.text | 通用 ToolForm+文件 | 已实现 | 同上 | 跨域迁移;Go 无 teyvat(靠 local.teyvat,F8) |
| 7 | `misc.stego.wechat-dat` | /c/misc.stego | 分类 → stego.image | 通用 ToolForm+文件 | 已实现 | 同上 | 跨域迁移;目标分类为假设(ia.md §9) |
| 8 | `reverse.static.strings` | /c/reverse.static | 不变 | 通用·真实后端(**F1 接线修复**:→realBackend `reverse.strings`) | 已实现 | 无 | 接线改动的回归 |
| 9 | `crypto.encoding.base.base64` | /c/crypto.encoding.base | 分类 → /c/codec.base(工具路由不变) | CodecWorkspace | 已实现 | 分类级:`/c/crypto.encoding → /c/codec`(本批先落地该条) | 挂载变更 |
| 10 | `crypto.classical.rot13` | /c/crypto.classical | 不变 | 通用 ToolForm | 已实现 | 无 | — |
| 11 | `crypto.hash.hash` | /c/crypto.hash | 不变 | 通用 ToolForm | 已实现 | 无 | F2:与 Go `misc.hash` 重复(前端本地优先,只记录) |
| 12 | `misc.file_checksum` | 无页面(孤儿) | **新路由** `/t/misc.file_checksum`,分类 /c/misc | 通用 ToolForm+文件 | 已实现(Go) | 无(新路由) | 需在 tools.ts 新增定义 + realBackend 接线 |

### 2.2 核心动作

1. F1 三处接线修复(strings / http-headers / file_checksum)——从占位变真实可用。
2. 3 个 misc.stego 工具改挂 stego.text/stego.image;`categories.ts` 移除 `misc.stego` 节点;`/c/misc.stego` 301 至 `/c/stego`。
3. codec 拆分的首条重定向(`/c/crypto.encoding → /c/codec`)随 base64 落地。
4. 全部 12 个工具跑通统一头部模板 + 状态徽章 + 占位横幅(若接线失败降级显示)。

### 2.3 附加验收(§5 通用之外)

- 4 个 realBackend 工具真值回归:jndi(检测/利用参数)、jinja2(检测)、cbc-bitflip、whitespace-codec 各跑 1 组已知输入。
- 接线修复后 strings/http-headers 返回真实后端数据(非 mock);file_checksum 上传→校验全链路。
- `/c/misc.stego` 与 `/c/crypto.encoding` 访问 → 重定向到新分类页,不出现 404。
- 收藏 1 个本批工具 → 刷新后仍有效(localStorage key 不变)。

---

## 3. 批 2:codec 全量(75 个,42 已实现 / 33 占位)

**理由**:新一级分类入口关键;全站功能密度最高域;含 12 个 Base、4 个 codec 工具、40 个 `misc.encoding.*` 挂载、beast、hexdump 等。

- **核心动作**:76→75(减批 1 的 base64)个工具的 categoryId 平移 `crypto.encoding* → codec*`;Base 12 保留 `codec.base` 二级;其余平铺 + 标签分组;`categories.ts` 移除 `crypto.encoding` 节点。
- **重定向**:`/c/crypto.encoding → /c/codec`(若批 1 未落地则本批落地);`/c/crypto.encoding.base → /c/codec.base`。
- **风险**:40 个工具 id 前缀 `misc.encoding.*` 与挂载不一致(Q8 债务,本批不改 id);42 个 local handler 冒烟量大(每批报告须附全量冒烟脚本结果);beast 归类存疑(暂挂 codec 顶层并标记,ia.md §9);`crypto.encoding.beast` 的 Go 重复实现 F2 只记录。
- **附加验收**:42 个已实现工具各跑 1 组已知输入(脚本批量冒烟,失败数 = 0);`/c/crypto.encoding.base` 重定向生效;搜索「base64」能同时命中 base64/base64url/base64-offsets/base64-stego 且分类归属正确。


| # | 工具 id | 名称 | 分类迁移(旧 → 新) | 模板 | 状态 | 风险 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `crypto.encoding.beast` | 兽音编码 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec;归类存疑 |
| 2 | `crypto.encoding.base.base2` | Base2 编解码 | crypto.encoding.base → codec.base | CodecWorkspace | 已实现 | 挂载→codec |
| 3 | `crypto.encoding.base.base8` | Base8 编解码 | crypto.encoding.base → codec.base | CodecWorkspace | 已实现 | 挂载→codec |
| 4 | `crypto.encoding.base.base16` | Base16 编解码 | crypto.encoding.base → codec.base | CodecWorkspace | 已实现 | 挂载→codec |
| 5 | `crypto.encoding.base.base32` | Base32 编解码 | crypto.encoding.base → codec.base | CodecWorkspace | 已实现 | 挂载→codec |
| 6 | `crypto.encoding.base.base45` | Base45 编解码 | crypto.encoding.base → codec.base | CodecWorkspace | 已实现 | 挂载→codec |
| 7 | `crypto.encoding.base.base58` | Base58 编解码 | crypto.encoding.base → codec.base | CodecWorkspace | 已实现 | 挂载→codec |
| 8 | `crypto.encoding.base.base62` | Base62 编解码 | crypto.encoding.base → codec.base | CodecWorkspace | 已实现 | 挂载→codec |
| 9 | `crypto.encoding.base.base64url` | Base64 URL-safe 编解码 | crypto.encoding.base → codec.base | CodecWorkspace | 已实现 | 挂载→codec |
| 10 | `crypto.encoding.base.base85` | Base85 编解码 | crypto.encoding.base → codec.base | CodecWorkspace | 已实现 | 挂载→codec |
| 11 | `crypto.encoding.base.base91` | Base91 编解码 | crypto.encoding.base → codec.base | CodecWorkspace | 已实现 | 挂载→codec |
| 12 | `crypto.encoding.base.base92` | Base92 编解码 | crypto.encoding.base → codec.base | CodecWorkspace | 已实现 | 挂载→codec |
| 13 | `crypto.encoding.url-decode` | URL 编解码 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 14 | `crypto.encoding.hex-decode` | Hex 编解码 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 15 | `crypto.encoding.morse` | 摩斯电码 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 16 | `crypto.encoding.rot47` | ROT47 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 17 | `misc.encoding.brainfuck` | Brainfuck 解释器 | crypto.encoding → codec | TransformWorkspace | 已实现 | 挂载→codec |
| 18 | `misc.encoding.xxd` | 十六进制查看 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 19 | `misc.encoding.qrcode-decode` | 二维码解码 | crypto.encoding → codec | 通用 ToolForm+文件 | 占位 | 挂载→codec |
| 20 | `misc.encoding.barcode-decode` | 条形码解码 | crypto.encoding → codec | 通用 ToolForm+文件 | 占位 | 挂载→codec |
| 21 | `crypto.encoding.base-convert` | 进制转换 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 22 | `misc.encoding.jsfuck` | JSFuck | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 23 | `misc.encoding.aaencode` | AAEncode | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 24 | `misc.encoding.jjencode` | JJEncode | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 25 | `misc.encoding.rot5` | ROT5 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 26 | `misc.encoding.rot18` | ROT18 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 27 | `misc.encoding.rot8000` | ROT8000 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 28 | `misc.encoding.punycode` | Punycode | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 29 | `misc.encoding.uuencode` | UUEncode | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 30 | `misc.encoding.xxencode` | XXEncode | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 31 | `misc.encoding.utf7` | UTF-7 编码 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 32 | `misc.encoding.quoted-printable` | Quoted-Printable | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 33 | `misc.encoding.shellcode` | Shellcode 编码 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec;重名shellcode |
| 34 | `misc.encoding.ook` | Ook! | crypto.encoding → codec | TransformWorkspace | 已实现 | 挂载→codec |
| 35 | `misc.encoding.decimal` | 十进制编码 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 36 | `misc.encoding.octal` | 八进制编码 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 37 | `misc.encoding.unicode` | Unicode 转义 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 38 | `misc.encoding.html-entity` | HTML 实体 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 39 | `misc.encoding.zerowidth-encode` | 零宽字符编码 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 40 | `misc.encoding.buddha` | 与佛论禅 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 41 | `misc.encoding.core-values` | 核心价值观 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 42 | `misc.encoding.bear` | 熊曰 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 43 | `misc.encoding.surnames` | 百家姓 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 44 | `misc.encoding.telegraph` | 中文电码 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 45 | `misc.encoding.periodic-table` | 元素周期表 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 46 | `misc.encoding.heavenly-stems` | 天干地支 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 47 | `misc.encoding.bagua` | 八卦符 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 48 | `misc.encoding.hexagram` | 伏羲六十四卦 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 49 | `misc.encoding.vowel-cipher` | 元音密码 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 50 | `misc.encoding.whitespace` | Whitespace | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 51 | `misc.encoding.deadfish` | Deadfish | crypto.encoding → codec | TransformWorkspace | 已实现 | 挂载→codec |
| 52 | `misc.encoding.spoon` | Spoon | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 53 | `misc.encoding.lolcode` | LOLCODE | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 54 | `misc.encoding.chinese-num` | 汉字数字 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 55 | `crypto.encoding.hexdump` | Hexdump 转储 | crypto.encoding → codec | TransformWorkspace | 已实现 | 挂载→codec |
| 56 | `crypto.encoding.from-hexdump` | Hexdump 还原 | crypto.encoding → codec | TransformWorkspace | 已实现 | 挂载→codec |
| 57 | `crypto.encoding.charcode` | 字符码转换 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 58 | `crypto.encoding.base64-offsets` | Base64 偏移 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 59 | `crypto.encoding.custom-base` | 自定义 Base 编解码 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 60 | `crypto.encoding.amf` | AMF 编解码 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 61 | `crypto.encoding.hex-content` | Hex Content 转换 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 62 | `crypto.encoding.pem-hex` | PEM 与 Hex 互转 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 63 | `crypto.encoding.parse-asn1` | 解析 ASN.1 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 64 | `crypto.encoding.ip-format` | IP 格式转换 | crypto.encoding → codec | CodecWorkspace | 已实现 | 挂载→codec |
| 65 | `crypto.encoding.charset` | 字符集编解码 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 66 | `crypto.encoding.charset-brute` | 字符集暴力解码 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 67 | `crypto.encoding.endianness` | 字节序交换 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 68 | `crypto.encoding.messagepack` | MessagePack 编解码 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 69 | `crypto.encoding.parse-tlv` | 解析 TLV | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 70 | `crypto.encoding.csv-json` | CSV 与 JSON 互转 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 71 | `crypto.encoding.avro-json` | Avro 转 JSON | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 72 | `crypto.encoding.cbor` | CBOR 编解码 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 73 | `misc.encoding.normalise-unicode` | Unicode 规范化 | crypto.encoding → codec | TransformWorkspace | 已实现 | 挂载→codec |
| 74 | `misc.encoding.unicode-format` | Unicode 书写格式 | crypto.encoding → codec | 通用 ToolForm | 占位 | 挂载→codec |
| 75 | `misc.encoding.remove-diacritics` | 去除变音符号 | crypto.encoding → codec | TransformWorkspace | 已实现 | 挂载→codec |


---

## 4. 批 3:web 全量(61 个,2 已实现 / 59 占位)

**理由**:CTF 最高频安全域;19 个 ssti 占位需统一「后端占位」徽章(Go 侧 ssti_base 占位执行器可见);重名「文件上传绕过」2 个在此批(只记录,阶段 4 处置)。

- **核心动作**:骨架 + 徽章 + 标准占位态;2 个已实现(jwt 等本地实现)走全链路;19 个 ssti 占位头部显示「后端占位」注记。
- **重定向**:无。
- **风险**:59 个占位全走 mock 路径,需确认占位横幅在表单提交后仍明确标注 mock 结果;SSTI 系列 20 个共用生成器,注意批量生成行级差异核对。
- **附加验收**:抽 3 个 ssti 占位页验证「后端占位」徽章与横幅;重名组 burp-bypass / upload-bypass 各自可访问且名称可区分(卡片显示分类前缀)。


| # | 工具 id | 名称 | 分类迁移(旧 → 新) | 模板 | 状态 | 风险 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `web.basic.jwt-decode` | JWT 解码/伪造 | web.basic | 通用 ToolForm | 已实现 | — |
| 2 | `web.deserialize.php-unserialize` | PHP 反序列化 PoC 生成 | web.deserialize | PHP 反序列化工作区 | 已实现 | — |
| 3 | `web.ssti.fingerprint` | SSTI · 自动指纹识别 | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 4 | `web.ssti.smarty` | SSTI · Smarty | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 5 | `web.ssti.twig` | SSTI · Twig | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 6 | `web.ssti.mako` | SSTI · Mako | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 7 | `web.ssti.tornado` | SSTI · Tornado | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 8 | `web.ssti.django` | SSTI · Django | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 9 | `web.ssti.freemarker` | SSTI · FreeMarker | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 10 | `web.ssti.velocity` | SSTI · Velocity | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 11 | `web.ssti.thymeleaf` | SSTI · Thymeleaf / SpEL | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 12 | `web.ssti.erb` | SSTI · ERB | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 13 | `web.ssti.slim` | SSTI · Slim | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 14 | `web.ssti.nunjucks` | SSTI · Nunjucks | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 15 | `web.ssti.ejs` | SSTI · EJS | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 16 | `web.ssti.pug` | SSTI · Pug | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 17 | `web.ssti.handlebars` | SSTI · Handlebars | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 18 | `web.ssti.go` | SSTI · Go template | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 19 | `web.ssti.razor` | SSTI · Razor | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 20 | `web.ssti.angularjs` | SSTI · AngularJS | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 21 | `web.ssti.groovy` | SSTI · Groovy | web.ssti | 通用·真实后端 | 占位 | 后端占位 |
| 22 | `web.sql.sqli-scanner` | SQL 注入扫描器 | web.sql | 通用 ToolForm | 占位 | — |
| 23 | `web.sql.union-extract` | Union 注入提取 | web.sql | 通用 ToolForm | 占位 | — |
| 24 | `web.xss.reflected` | 反射型 XSS 检测 | web.xss | 通用 ToolForm | 占位 | — |
| 25 | `web.xss.dom` | DOM XSS 检测 | web.xss | 通用 ToolForm | 占位 | — |
| 26 | `web.ssrf.test` | SSRF 探测 | web.ssrf | 通用 ToolForm | 占位 | — |
| 27 | `web.rce.command-inject` | 命令注入 | web.rce | 通用 ToolForm | 占位 | — |
| 28 | `web.deserialize.ysoserial` | 反序列化利用 | web.deserialize | 通用 ToolForm | 占位 | — |
| 29 | `web.upload.burp-bypass` | 文件上传绕过 | web.upload | 通用 ToolForm | 占位 | 重名上传 |
| 30 | `web.lfi.path-traversal` | 路径穿越检测 | web.lfi | 通用 ToolForm | 占位 | — |
| 31 | `web.basic.cookie-decode` | Cookie 解析 | web.basic | 通用 ToolForm | 占位 | — |
| 32 | `web.basic.directory-brute` | 目录爆破 | web.basic | 通用 ToolForm | 占位 | — |
| 33 | `web.sql.bool-inject` | 布尔盲注 | web.sql | 通用 ToolForm | 占位 | — |
| 34 | `web.sql.time-sqli` | 时间盲注 | web.sql | 通用 ToolForm | 占位 | — |
| 35 | `web.sql.sqli-labs` | SQLi-LABS 专项 | web.sql | 通用 ToolForm | 占位 | — |
| 36 | `web.ssrf.ssrf-rebind` | SSRF DNS Rebinding | web.ssrf | 通用 ToolForm | 占位 | — |
| 37 | `web.ssrf.ssrf-payload` | SSRF Payload | web.ssrf | 通用 ToolForm | 占位 | — |
| 38 | `web.rce.php-eval-rce` | PHP eval RCE | web.rce | 通用 ToolForm | 占位 | — |
| 39 | `web.rce.simple-cmd-rce` | 简单命令注入 | web.rce | 通用 ToolForm | 占位 | — |
| 40 | `web.rce.bashfuck` | bashFuck 无字母命令 | web.rce | 通用 ToolForm | 占位 | — |
| 41 | `web.rce.bashfuck-solver` | bashFuck 自动求解 | web.rce | 通用 ToolForm | 占位 | — |
| 42 | `web.upload.upload-bypass` | 文件上传绕过 | web.upload | 通用 ToolForm | 占位 | 重名上传 |
| 43 | `web.upload.iis-upload` | IIS 文件上传 | web.upload | 通用 ToolForm | 占位 | — |
| 44 | `web.lfi.php-lfi` | PHP 文件包含 | web.lfi | 通用 ToolForm | 占位 | — |
| 45 | `web.lfi.phpinfo-lfi` | phpinfo LFI | web.lfi | 通用 ToolForm | 占位 | — |
| 46 | `web.lfi.php-wrappers` | PHP 伪协议 | web.lfi | 通用 ToolForm | 占位 | — |
| 47 | `web.php.php-logic` | PHP 弱类型绕过 | web.php | 通用 ToolForm | 占位 | — |
| 48 | `web.php.magic-hash` | PHP Magic Hash | web.php | 通用 ToolForm | 占位 | — |
| 49 | `web.php.php-md5-collision` | PHP MD5 碰撞 | web.php | 通用 ToolForm | 占位 | — |
| 50 | `web.xss.xss-payload` | XSS Payload | web.xss | 通用 ToolForm | 占位 | — |
| 51 | `web.basic.js-challenge` | JS 客户端挑战 | web.basic | 通用 ToolForm | 占位 | — |
| 52 | `web.basic.url-analyzer` | URL 漏洞分析 | web.basic | 通用 ToolForm | 占位 | — |
| 53 | `web.basic.smart-solver` | Web 智能求解 | web.basic | 通用 ToolForm | 占位 | — |
| 54 | `web.basic.multi-stage` | 多阶段攻击 | web.basic | 通用 ToolForm | 占位 | — |
| 55 | `web.basic.chain-attack` | 攻击链升级 | web.basic | 通用 ToolForm | 占位 | — |
| 56 | `web.basic.dict-scan` | 字典扫描 | web.basic | 通用 ToolForm | 占位 | — |
| 57 | `web.basic.port-scan` | 端口扫描 | web.basic | 通用 ToolForm | 占位 | — |
| 58 | `web.basic.jwt-none` | JWT None 攻击 | web.basic | 通用 ToolForm | 占位 | — |
| 59 | `web.basic.jwt-brute` | JWT 弱密钥爆破 | web.basic | 通用 ToolForm | 占位 | — |
| 60 | `web.basic.jwt-forge` | JWT 伪造 | web.basic | 通用 ToolForm | 占位 | — |
| 61 | `web.basic.jwt-role` | JWT 角色提升 | web.basic | 通用 ToolForm | 占位 | — |


---

## 5. 批 4:crypto.classical(74 个,1 已实现 / 73 占位)

**理由**:经典密码是 CTF 入门高频;3 段节点(caesar/vigenere/affine/keyed)按 ia.md §4 降级为分类页筛选 chips(路由保留)。

- **核心动作**:骨架 + 徽章;chips 分组;rot13 已在批 1,其余 73 个占位统一占位态。
- **重定向**:无(3 段节点路由保留)。
- **风险**:73 个占位为单一批次上限,报告须附生成一致性核对(名称/分类/徽章三列抽样 20 行比对 inventory)。
- **附加验收**:/c/crypto.classical 页显示 4 个分组 chips(凯撒/维吉尼亚/仿射/带密钥),点击过滤正确;直接访问 /c/crypto.classical.caesar 仍可达(不 404)。


| # | 工具 id | 名称 | 分类迁移(旧 → 新) | 模板 | 状态 | 风险 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `crypto.classical.rail-fence` | 栅栏密码 | crypto.classical | 通用 ToolForm | 已实现 | — |
| 2 | `crypto.classical.caesar-decrypt` | 凯撒解密 | crypto.classical.caesar | 通用 ToolForm | 占位 | — |
| 3 | `crypto.classical.vigenere-decrypt` | 维吉尼亚破解 | crypto.classical.vigenere | 通用 ToolForm | 占位 | — |
| 4 | `crypto.classical.affine-decrypt` | 仿射密码 | crypto.classical.affine | 通用 ToolForm | 占位 | — |
| 5 | `crypto.classical.bacon` | 培根密码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 6 | `crypto.classical.keyed.hill` | 希尔密码 Hill | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 7 | `crypto.classical.keyed.multiplicative` | 乘法密码 | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 8 | `crypto.classical.keyed.otp` | 一次一密 OTP | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 9 | `crypto.classical.keyed.gronsfeld` | Gronsfeld 密码 | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 10 | `crypto.classical.keyed.beaufort` | 博福特 Beaufort | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 11 | `crypto.classical.keyed.autokey` | 自动密钥 Autokey | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 12 | `crypto.classical.keyed.bifid` | 双密码 Bifid | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 13 | `crypto.classical.keyed.foursquare` | 四方密码 | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 14 | `crypto.classical.keyed.scytale` | 斯巴达棒 Scytale | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 15 | `crypto.classical.keyed.nihilist` | Nihilist 密码 | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 16 | `crypto.classical.keyed.keyword` | 关键字密码 | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 17 | `crypto.classical.keyed.simple-substitution` | 简单替换密码 | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 18 | `crypto.classical.keyed.coltrans` | 列移位 ColTrans | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 19 | `crypto.classical.keyed.column-permutation` | 列置换 | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 20 | `crypto.classical.keyed.rows-permutation` | 行置换 | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 21 | `crypto.classical.keyed.porta` | 城门 Porta | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 22 | `crypto.classical.keyed.bazeries` | Bazeries 密码 | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 23 | `crypto.classical.keyed.fractionated-morse` | 分组摩斯 | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 24 | `crypto.classical.keyed.fenham` | 费娜姆 Fenham | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 25 | `crypto.classical.keyed.running-key` | 滚动密钥 | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 26 | `crypto.classical.keyed.kamasutra` | 爱经 Kamasutra | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 27 | `crypto.classical.keyed.fernet` | Fernet | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 28 | `crypto.classical.keyed.enigma` | 恩尼格玛 Enigma | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 29 | `crypto.classical.keyed.adfgvx` | ADFGVX | crypto.classical.keyed | 通用 ToolForm | 占位 | — |
| 30 | `crypto.classical.pigpen` | 猪圈密码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 31 | `crypto.classical.atbash` | 埃特巴什 Atbash | crypto.classical | 通用 ToolForm | 占位 | — |
| 32 | `crypto.classical.polybius` | 波利比奥斯 Polybius | crypto.classical | 通用 ToolForm | 占位 | — |
| 33 | `crypto.classical.reverse` | 倒序密码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 34 | `crypto.classical.adfgx` | ADFGX | crypto.classical | 通用 ToolForm | 占位 | — |
| 35 | `crypto.classical.jefferson-wheel` | 杰斐逊转轮 | crypto.classical | 通用 ToolForm | 占位 | — |
| 36 | `crypto.classical.number-coordinate` | 数字坐标 | crypto.classical | 通用 ToolForm | 占位 | — |
| 37 | `crypto.classical.keyboard-chessboard` | 键盘棋盘 | crypto.classical | 通用 ToolForm | 占位 | — |
| 38 | `crypto.classical.keyboard-coordinate` | 键盘坐标 | crypto.classical | 通用 ToolForm | 占位 | — |
| 39 | `crypto.classical.qwe-keyboard` | QWE 键盘 | crypto.classical | 通用 ToolForm | 占位 | — |
| 40 | `crypto.classical.phone-keypad` | 手机键盘 | crypto.classical | 通用 ToolForm | 占位 | — |
| 41 | `crypto.classical.alphabet-order` | 字母表顺序 | crypto.classical | 通用 ToolForm | 占位 | — |
| 42 | `crypto.classical.pawnshat` | 当铺密码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 43 | `crypto.classical.standard-galactic` | 标准银河字母 | crypto.classical | 通用 ToolForm | 占位 | — |
| 44 | `crypto.classical.dna` | DNA 编码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 45 | `crypto.classical.a1z26` | A1Z26 | crypto.classical | 通用 ToolForm | 占位 | — |
| 46 | `crypto.classical.baudot` | 博多码 Baudot | crypto.classical | 通用 ToolForm | 占位 | — |
| 47 | `crypto.classical.type7` | Cisco Type7 | crypto.classical | 通用 ToolForm | 占位 | — |
| 48 | `crypto.classical.yunying` | 云影密码 01248 | crypto.classical | 通用 ToolForm | 占位 | — |
| 49 | `crypto.classical.tap-code` | 敲击码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 50 | `crypto.classical.nato` | 北约音标 | crypto.classical | 通用 ToolForm | 占位 | — |
| 51 | `crypto.classical.braille` | 盲文 | crypto.classical | 通用 ToolForm | 占位 | — |
| 52 | `crypto.classical.manchester` | 曼彻斯特编码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 53 | `crypto.classical.keyboard-keycode` | 键盘键码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 54 | `crypto.classical.keyboard-shift` | 键盘上档 | crypto.classical | 通用 ToolForm | 占位 | — |
| 55 | `crypto.classical.handycode` | 九宫格 | crypto.classical | 通用 ToolForm | 占位 | — |
| 56 | `crypto.classical.dvorak` | 德沃夏克键盘 | crypto.classical | 通用 ToolForm | 占位 | — |
| 57 | `crypto.classical.solresol` | 音符密码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 58 | `crypto.classical.chinesecode` | 汉码 / 区位码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 59 | `crypto.classical.arrows` | 箭头密码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 60 | `crypto.classical.bcd` | BCD 码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 61 | `crypto.classical.gray-code` | 格雷码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 62 | `crypto.classical.hamming` | 海明码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 63 | `crypto.classical.ieee754` | IEEE754 浮点 | crypto.classical | 通用 ToolForm | 占位 | — |
| 64 | `crypto.classical.timestamp` | 时间戳转换 | crypto.classical | 通用 ToolForm | 占位 | — |
| 65 | `crypto.classical.prime-factor` | 质因数分解 | crypto.classical | 通用 ToolForm | 占位 | — |
| 66 | `crypto.classical.caesar-box` | 凯撒盒密码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 67 | `crypto.classical.cetacean` | 鲸豚密码 | crypto.classical | 通用 ToolForm | 占位 | — |
| 68 | `crypto.classical.substitute` | 字符替换 | crypto.classical | 通用 ToolForm | 占位 | — |
| 69 | `crypto.classical.typex` | Typex 密码机 | crypto.classical | 通用 ToolForm | 占位 | — |
| 70 | `crypto.classical.lorenz` | Lorenz 密码机 | crypto.classical | 通用 ToolForm | 占位 | — |
| 71 | `crypto.classical.colossus` | Colossus 计算机 | crypto.classical | 通用 ToolForm | 占位 | — |
| 72 | `crypto.classical.sigaba` | SIGABA 密码机 | crypto.classical | 通用 ToolForm | 占位 | — |
| 73 | `crypto.classical.bombe` | Bombe 破译机 | crypto.classical | 通用 ToolForm | 占位 | — |
| 74 | `crypto.classical.multiple-bombe` | 多组 Bombe | crypto.classical | 通用 ToolForm | 占位 | — |


---

## 6. 批 5:crypto.modern + auto-decode(68 个,3 已实现 / 65 占位)

**理由**:RSA 层级修复(规则 5)集中在此批;重名「RSA 解密」2 个在此批(只记录)。

- **核心动作**:22 个 RSA 工具从 5 个 4 段节点扁平化挂到 `crypto.modern.rsa`(表 C 方案 A),tags 保留技术类别;`categories.ts` 删除 5 个 4 段节点。
- **重定向(5 条,本批全部落地)**:`/c/crypto.modern.rsa.{algebraic, fermat-family, shared-structure, number-theoretic, coppersmith} → /c/crypto.modern.rsa`。
- **风险**:5 条重定向 + 22 个 menuItem 更新,回退需整批原子提交;rsa-decrypt 与 rsa.decrypt 重名并存(卡片显示完整 id 前缀,阶段 4 处置);aes.cbc-bitflip 已在批 1。
- **附加验收**:5 条旧分类 URL 逐一验证 301 语义(重定向后 URL 为新路径且页面正常);/c/crypto.modern.rsa 页可见 22 个工具且 tags 技术类别齐全;auto-decode 工具可访问。


| # | 工具 id | 名称 | 分类迁移(旧 → 新) | 模板 | 状态 | 风险 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `crypto.modern.rsa-decrypt` | RSA 解密 | crypto.modern.rsa | 通用 ToolForm | 占位 | 重名RSA |
| 2 | `crypto.modern.aes` | AES 加解密 | crypto.modern.aes | 通用 ToolForm | 占位 | — |
| 3 | `crypto.auto-decode` | 自动解码（暴力解码） | crypto.auto-decode | 通用 ToolForm | 已实现 | — |
| 4 | `crypto.modern.rsa.fermat-family.close` | |p-q| 小 · Fermat 分解 | crypto.modern.rsa.fermat-family → crypto.modern.rsa | 通用 ToolForm | 已实现 | 挂载→rsa |
| 5 | `crypto.modern.rsa.shared-structure.nth-root` | 同一 r 高次关联 · 开根 | crypto.modern.rsa.shared-structure → crypto.modern.rsa | 通用 ToolForm | 已实现 | 挂载→rsa |
| 6 | `crypto.modern.rsa-wiener` | RSA Wiener 攻击 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 7 | `crypto.modern.rsa-common-modulus` | RSA 共模攻击 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 8 | `crypto.modern.aes-ecb` | AES-ECB 分析 | crypto.modern.aes | 通用 ToolForm | 占位 | — |
| 9 | `crypto.modern.ecc-factor` | ECC 参数分析 | crypto.modern.ecc | 通用 ToolForm | 占位 | — |
| 10 | `crypto.modern.des` | DES 加解密 | crypto.modern | 通用 ToolForm | 占位 | — |
| 11 | `crypto.modern.rc4` | RC4 加解密 | crypto.modern | 通用 ToolForm | 占位 | — |
| 12 | `crypto.modern.rsa.low-exponent` | RSA 低加密指数 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 13 | `crypto.modern.rsa.fermat` | RSA Fermat 分解 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 14 | `crypto.modern.rsa.broadcast` | RSA 广播攻击 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 15 | `crypto.modern.rsa.small-e` | RSA 小指数攻击 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 16 | `crypto.modern.rsa.algebraic.linear` | 线性关系 q = a*p + b | crypto.modern.rsa.algebraic → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 17 | `crypto.modern.rsa.algebraic.diff-sum` | 差/和已知（p±q） | crypto.modern.rsa.algebraic → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 18 | `crypto.modern.rsa.algebraic.ratio` | 比例关系 q = c*p | crypto.modern.rsa.algebraic → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 19 | `crypto.modern.rsa.algebraic.polynomial` | 多项式关系 q = f(p) | crypto.modern.rsa.algebraic → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 20 | `crypto.modern.rsa.algebraic.congruence` | 同余关系 p = q (mod M) | crypto.modern.rsa.algebraic → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 21 | `crypto.modern.rsa.fermat-family.shared-msb` | 共享 MSB | crypto.modern.rsa.fermat-family → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 22 | `crypto.modern.rsa.fermat-family.shared-lsb` | 共享 LSB | crypto.modern.rsa.fermat-family → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 23 | `crypto.modern.rsa.fermat-family.twin` | 相邻/孪生素数 | crypto.modern.rsa.fermat-family → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 24 | `crypto.modern.rsa.shared-structure.poly-roots` | 同一多项式两个根 | crypto.modern.rsa.shared-structure → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 25 | `crypto.modern.rsa.shared-structure.sequence` | 序列相邻项 | crypto.modern.rsa.shared-structure → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 26 | `crypto.modern.rsa.shared-structure.ec-multiple` | 曲线/群相邻倍点 | crypto.modern.rsa.shared-structure → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 27 | `crypto.modern.rsa.shared-structure.lcg-lattice` | LCG 相邻输出 / 格向量 | crypto.modern.rsa.shared-structure → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 28 | `crypto.modern.rsa.number-theoretic.next-prime` | next_prime 枚举 | crypto.modern.rsa.number-theoretic → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 29 | `crypto.modern.rsa.number-theoretic.safe-prime` | 安全素数 p = 2q+1 | crypto.modern.rsa.number-theoretic → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 30 | `crypto.modern.rsa.number-theoretic.phi-leak` | phi(n) / d 泄露 | crypto.modern.rsa.number-theoretic → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 31 | `crypto.modern.rsa.number-theoretic.shared-gcd` | gcd(p-1, q-1) 大 | crypto.modern.rsa.number-theoretic → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 32 | `crypto.modern.rsa.coppersmith.exact` | 精确代数关系 | crypto.modern.rsa.coppersmith → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 33 | `crypto.modern.rsa.coppersmith.approximate` | 近似关系 | crypto.modern.rsa.coppersmith → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 34 | `crypto.modern.rsa.coppersmith.modular` | 模关系 | crypto.modern.rsa.coppersmith → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 35 | `crypto.modern.rsa.coppersmith.implicit` | 隐式关系 | crypto.modern.rsa.coppersmith → crypto.modern.rsa | 通用 ToolForm | 占位 | 挂载→rsa |
| 36 | `crypto.modern.blowfish` | Blowfish 加解密 | crypto.modern | 通用 ToolForm | 占位 | — |
| 37 | `crypto.modern.triple-des` | 3DES 加解密 | crypto.modern | 通用 ToolForm | 占位 | — |
| 38 | `crypto.modern.rc2` | RC2 加解密 | crypto.modern | 通用 ToolForm | 占位 | — |
| 39 | `crypto.modern.rc4-drop` | RC4 Drop | crypto.modern | 通用 ToolForm | 占位 | — |
| 40 | `crypto.modern.chacha` | ChaCha 加解密 | crypto.modern | 通用 ToolForm | 占位 | — |
| 41 | `crypto.modern.rabbit` | Rabbit 加解密 | crypto.modern | 通用 ToolForm | 占位 | — |
| 42 | `crypto.modern.sm4` | SM4 加解密 | crypto.modern | 通用 ToolForm | 占位 | — |
| 43 | `crypto.modern.gost` | GOST 加解密 | crypto.modern | 通用 ToolForm | 占位 | — |
| 44 | `crypto.modern.gost-sign` | GOST 签名/验签 | crypto.modern | 通用 ToolForm | 占位 | — |
| 45 | `crypto.modern.gost-keywrap` | GOST 密钥封装 | crypto.modern | 通用 ToolForm | 占位 | — |
| 46 | `crypto.modern.ls47` | LS47 加解密 | crypto.modern | 通用 ToolForm | 占位 | — |
| 47 | `crypto.modern.ciphersaber2` | CipherSaber-2 | crypto.modern | 通用 ToolForm | 占位 | — |
| 48 | `crypto.modern.citrix-ctx1` | Citrix CTX1 | crypto.modern | 通用 ToolForm | 占位 | — |
| 49 | `crypto.modern.aes-keywrap` | AES 密钥封装 | crypto.modern | 通用 ToolForm | 占位 | — |
| 50 | `crypto.modern.derive-pbkdf2` | PBKDF2 密钥派生 | crypto.modern | 通用 ToolForm | 占位 | — |
| 51 | `crypto.modern.derive-evp` | EVP 密钥派生 | crypto.modern | 通用 ToolForm | 占位 | — |
| 52 | `crypto.modern.derive-hkdf` | HKDF 密钥派生 | crypto.modern | 通用 ToolForm | 占位 | — |
| 53 | `crypto.modern.prng` | 伪随机数生成 | crypto.modern | 通用 ToolForm | 占位 | — |
| 54 | `crypto.modern.rsa.parse-x509` | 解析 X.509 证书 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 55 | `crypto.modern.rsa.hex-oid` | Hex 与 OID 互转 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 56 | `crypto.modern.rsa.generate-pgp` | 生成 PGP 密钥对 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 57 | `crypto.modern.rsa.pgp-encrypt` | PGP 加密 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 58 | `crypto.modern.rsa.pgp-decrypt` | PGP 解密 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 59 | `crypto.modern.rsa.pgp-verify` | PGP 验签 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 60 | `crypto.modern.rsa.pgp-sign` | PGP 加密并签名 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 61 | `crypto.modern.rsa.generate` | 生成 RSA 密钥对 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 62 | `crypto.modern.rsa.sign` | RSA 签名 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 63 | `crypto.modern.rsa.verify` | RSA 验签 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 64 | `crypto.modern.rsa.encrypt` | RSA 加密 | crypto.modern.rsa | 通用 ToolForm | 占位 | — |
| 65 | `crypto.modern.rsa.decrypt` | RSA 解密 | crypto.modern.rsa | 通用 ToolForm | 占位 | 重名RSA |
| 66 | `crypto.modern.parse-ssh-key` | 解析 SSH 主机公钥 | crypto.modern | 通用 ToolForm | 占位 | — |
| 67 | `crypto.modern.xor` | XOR 异或 | crypto.modern | 通用 ToolForm | 占位 | — |
| 68 | `crypto.modern.xor-brute` | XOR 暴力破解 | crypto.modern | 通用 ToolForm | 占位 | — |


---

## 7. 批 6:crypto.hash(45 个,0 已实现 / 45 占位)

**理由**:补齐 crypto 域;46→45(减批 1 的 crypto.hash.hash)。

- **核心动作**:纯骨架批,45 个占位统一占位态 + 徽章;crypto.hash.hash 在批 1 已完成,本批只验证其在分类页的展示。
- **重定向**:无。
- **风险**:0 已实现,验收重点转向「占位标识正确」与「无 console error」(mock 路径)。
- **附加验收**:分类页默认排序下 crypto.hash.hash(已实现)排在 45 个占位之前;占位徽章数量 = 45。


| # | 工具 id | 名称 | 分类迁移(旧 → 新) | 模板 | 状态 | 风险 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `crypto.hash.length-extension` | 哈希长度扩展 | crypto.hash | 通用 ToolForm | 占位 | — |
| 2 | `crypto.hash.rainbow` | 彩虹表查询 | crypto.hash | 通用 ToolForm | 占位 | — |
| 3 | `crypto.hash.hashid` | Hash 类型识别 | crypto.hash | 通用 ToolForm | 占位 | — |
| 4 | `crypto.hash.crc32` | CRC32 校验 | crypto.hash | 通用 ToolForm | 占位 | — |
| 5 | `crypto.hash.hmac` | HMAC 计算 | crypto.hash | 通用 ToolForm | 占位 | — |
| 6 | `crypto.hash.md2` | MD2 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 7 | `crypto.hash.md4` | MD4 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 8 | `crypto.hash.md6` | MD6 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 9 | `crypto.hash.sha0` | SHA-0 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 10 | `crypto.hash.sha2` | SHA-2 系列 | crypto.hash | 通用 ToolForm | 占位 | — |
| 11 | `crypto.hash.sha3` | SHA-3 系列 | crypto.hash | 通用 ToolForm | 占位 | — |
| 12 | `crypto.hash.sm3` | SM3 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 13 | `crypto.hash.keccak` | Keccak 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 14 | `crypto.hash.shake` | SHAKE 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 15 | `crypto.hash.ripemd` | RIPEMD 系列 | crypto.hash | 通用 ToolForm | 占位 | — |
| 16 | `crypto.hash.has160` | HAS-160 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 17 | `crypto.hash.whirlpool` | Whirlpool 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 18 | `crypto.hash.snefru` | Snefru 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 19 | `crypto.hash.blake2b` | BLAKE2b 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 20 | `crypto.hash.blake2s` | BLAKE2s 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 21 | `crypto.hash.gost-hash` | GOST 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 22 | `crypto.hash.streebog` | Streebog 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 23 | `crypto.hash.ssdeep` | SSDEEP 模糊哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 24 | `crypto.hash.ctph` | CTPH 模糊哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 25 | `crypto.hash.compare-ssdeep` | 比较 SSDEEP | crypto.hash | 通用 ToolForm | 占位 | — |
| 26 | `crypto.hash.compare-ctph` | 比较 CTPH | crypto.hash | 通用 ToolForm | 占位 | — |
| 27 | `crypto.hash.cmac` | CMAC | crypto.hash | 通用 ToolForm | 占位 | — |
| 28 | `crypto.hash.bcrypt` | Bcrypt 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 29 | `crypto.hash.bcrypt-compare` | Bcrypt 校验 | crypto.hash | 通用 ToolForm | 占位 | — |
| 30 | `crypto.hash.bcrypt-parse` | Bcrypt 解析 | crypto.hash | 通用 ToolForm | 占位 | — |
| 31 | `crypto.hash.argon2` | Argon2 哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 32 | `crypto.hash.argon2-compare` | Argon2 校验 | crypto.hash | 通用 ToolForm | 占位 | — |
| 33 | `crypto.hash.nt-hash` | NT Hash | crypto.hash | 通用 ToolForm | 占位 | — |
| 34 | `crypto.hash.lm-hash` | LM Hash | crypto.hash | 通用 ToolForm | 占位 | — |
| 35 | `crypto.hash.fletcher8` | Fletcher-8 校验 | crypto.hash | 通用 ToolForm | 占位 | — |
| 36 | `crypto.hash.fletcher16` | Fletcher-16 校验 | crypto.hash | 通用 ToolForm | 占位 | — |
| 37 | `crypto.hash.fletcher32` | Fletcher-32 校验 | crypto.hash | 通用 ToolForm | 占位 | — |
| 38 | `crypto.hash.fletcher64` | Fletcher-64 校验 | crypto.hash | 通用 ToolForm | 占位 | — |
| 39 | `crypto.hash.adler32` | Adler-32 校验 | crypto.hash | 通用 ToolForm | 占位 | — |
| 40 | `crypto.hash.luhn` | Luhn 校验 | crypto.hash | 通用 ToolForm | 占位 | — |
| 41 | `crypto.hash.crc8` | CRC-8 校验 | crypto.hash | 通用 ToolForm | 占位 | — |
| 42 | `crypto.hash.crc16` | CRC-16 校验 | crypto.hash | 通用 ToolForm | 占位 | — |
| 43 | `crypto.hash.tcpip-checksum` | TCP/IP 校验和 | crypto.hash | 通用 ToolForm | 占位 | — |
| 44 | `crypto.hash.all-hashes` | 生成所有哈希 | crypto.hash | 通用 ToolForm | 占位 | — |
| 45 | `crypto.hash.scrypt` | Scrypt 密码哈希 | crypto.hash | 通用 ToolForm | 占位 | — |


---

## 8. 批 7:forensics(54 个,0 已实现 / 54 占位)

**理由**:大分类收口;全占位批用于验证批量吞吐与占位态一致性(为后续批建立吞吐基线)。

- **核心动作**:54 个占位统一骨架;含 zip-tools/binwalk 等取证向工具分组 chips。
- **重定向**:无。
- **风险**:全占位批的 mock 调用路径较多(http 适配器),需逐分类页走查无未捕获异常。
- **附加验收**:批量验收模板首秀:脚本全量核对 54 行(名称/分类/徽章)与 inventory 一致;分类页滚动到底无布局错位。


| # | 工具 id | 名称 | 分类迁移(旧 → 新) | 模板 | 状态 | 风险 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `forensics.disk.carve` | 文件雕刻 | forensics.disk | 通用 ToolForm+文件 | 占位 | — |
| 2 | `forensics.memory.volatility` | Volatility 分析 | forensics.memory | 通用 ToolForm+文件 | 占位 | — |
| 3 | `forensics.memory.dump-strings` | 内存字符串提取 | forensics.memory | 通用 ToolForm+文件 | 占位 | — |
| 4 | `forensics.disk.mbr-parse` | MBR 解析 | forensics.disk | 通用 ToolForm+文件 | 占位 | — |
| 5 | `forensics.disk.ntfs-parse` | NTFS 解析 | forensics.disk | 通用 ToolForm+文件 | 占位 | — |
| 6 | `forensics.network.pcap-analyze` | PCAP 流量分析 | forensics.network | 通用 ToolForm+文件 | 占位 | — |
| 7 | `forensics.network.dns-extract` | DNS 提取 | forensics.network | 通用 ToolForm | 占位 | — |
| 8 | `forensics.log.log-parser` | 日志解析 | forensics.log | 通用 ToolForm+文件 | 占位 | — |
| 9 | `forensics.log.timeline` | 时间线分析 | forensics.log | 通用 ToolForm | 占位 | — |
| 10 | `forensics.disk.zip-tools` | ZIP 分析 | forensics.disk | 通用 ToolForm | 占位 | — |
| 11 | `forensics.disk.binwalk` | 文件雕刻 Binwalk | forensics.disk | 通用 ToolForm | 占位 | — |
| 12 | `forensics.network.icmp` | ICMP 数据提取 | forensics.network | 通用 ToolForm | 占位 | — |
| 13 | `forensics.disk.crc-brute` | PNG CRC 爆破 | forensics.disk | 通用 ToolForm | 占位 | — |
| 14 | `forensics.disk.extract-ip` | 提取 IP 地址 | forensics.disk | 通用 ToolForm | 占位 | — |
| 15 | `forensics.disk.extract-email` | 提取邮箱 | forensics.disk | 通用 ToolForm | 占位 | — |
| 16 | `forensics.disk.extract-mac` | 提取 MAC | forensics.disk | 通用 ToolForm | 占位 | — |
| 17 | `forensics.disk.extract-url` | 提取 URL | forensics.disk | 通用 ToolForm | 占位 | — |
| 18 | `forensics.disk.extract-domain` | 提取域名 | forensics.disk | 通用 ToolForm | 占位 | — |
| 19 | `forensics.disk.extract-path` | 提取文件路径 | forensics.disk | 通用 ToolForm | 占位 | — |
| 20 | `forensics.disk.xpath` | XPath 提取 | forensics.disk | 通用 ToolForm | 占位 | — |
| 21 | `forensics.disk.jpath` | JPath 提取 | forensics.disk | 通用 ToolForm | 占位 | — |
| 22 | `forensics.disk.css-selector` | CSS 选择器提取 | forensics.disk | 通用 ToolForm | 占位 | — |
| 23 | `forensics.disk.extract-id3` | 提取 ID3 标签 | forensics.disk | 通用 ToolForm | 占位 | — |
| 24 | `forensics.disk.deflate` | Deflate 压缩/解压 | forensics.disk | 通用 ToolForm | 占位 | — |
| 25 | `forensics.disk.zlib` | Zlib 压缩/解压 | forensics.disk | 通用 ToolForm | 占位 | — |
| 26 | `forensics.disk.gzip` | Gzip 压缩/解压 | forensics.disk | 通用 ToolForm | 占位 | — |
| 27 | `forensics.disk.zip` | ZIP 打包/解压 | forensics.disk | 通用 ToolForm | 占位 | — |
| 28 | `forensics.disk.bzip2` | Bzip2 压缩/解压 | forensics.disk | 通用 ToolForm | 占位 | — |
| 29 | `forensics.disk.tar` | TAR 打包/解包 | forensics.disk | 通用 ToolForm | 占位 | — |
| 30 | `forensics.disk.lzstring` | LZString 压缩/解压 | forensics.disk | 通用 ToolForm | 占位 | — |
| 31 | `forensics.disk.lzma` | LZMA 压缩/解压 | forensics.disk | 通用 ToolForm | 占位 | — |
| 32 | `forensics.disk.lz4` | LZ4 压缩/解压 | forensics.disk | 通用 ToolForm | 占位 | — |
| 33 | `forensics.network.http-request` | HTTP 请求 | forensics.network | 通用 ToolForm | 占位 | — |
| 34 | `forensics.network.doh` | DNS over HTTPS | forensics.network | 通用 ToolForm | 占位 | — |
| 35 | `forensics.network.strip-headers` | 剥离 HTTP 头 | forensics.network | 通用 ToolForm | 占位 | — |
| 36 | `forensics.network.dechunk` | Dechunk HTTP | forensics.network | 通用 ToolForm | 占位 | — |
| 37 | `forensics.network.user-agent` | 解析 User-Agent | forensics.network | 通用 ToolForm | 占位 | — |
| 38 | `forensics.network.ip-range` | 解析 IP 段 | forensics.network | 通用 ToolForm | 占位 | — |
| 39 | `forensics.network.ipv6` | 解析 IPv6 | forensics.network | 通用 ToolForm | 占位 | — |
| 40 | `forensics.network.ipv4-header` | 解析 IPv4 头 | forensics.network | 通用 ToolForm | 占位 | — |
| 41 | `forensics.network.tcp` | 解析 TCP | forensics.network | 通用 ToolForm | 占位 | — |
| 42 | `forensics.network.udp` | 解析 UDP | forensics.network | 通用 ToolForm | 占位 | — |
| 43 | `forensics.network.parse-uri` | 解析 URI | forensics.network | 通用 ToolForm | 占位 | — |
| 44 | `forensics.network.protobuf` | Protobuf 编解码 | forensics.network | 通用 ToolForm | 占位 | — |
| 45 | `forensics.network.varint` | VarInt 编解码 | forensics.network | 通用 ToolForm | 占位 | — |
| 46 | `forensics.network.ja3` | JA3/JA3S 指纹 | forensics.network | 通用 ToolForm | 占位 | — |
| 47 | `forensics.network.hassh` | HASSH 指纹 | forensics.network | 通用 ToolForm | 占位 | — |
| 48 | `forensics.network.mac-format` | 格式化 MAC | forensics.network | 通用 ToolForm | 占位 | — |
| 49 | `forensics.network.group-ip` | IP 分组 | forensics.network | 通用 ToolForm | 占位 | — |
| 50 | `forensics.network.netbios` | NetBIOS 编解码 | forensics.network | 通用 ToolForm | 占位 | — |
| 51 | `forensics.network.defang` | URL/IP 去尖牙 | forensics.network | 通用 ToolForm | 占位 | — |
| 52 | `forensics.disk.scan-embedded` | 扫描内嵌文件 | forensics.disk | 通用 ToolForm | 占位 | — |
| 53 | `forensics.disk.yara` | YARA 规则扫描 | forensics.disk | 通用 ToolForm | 占位 | — |
| 54 | `forensics.disk.remove-exif` | 移除 EXIF | forensics.disk | 通用 ToolForm | 占位 | — |


---

## 9. 批 8:stego + reverse(47 个,1 已实现 / 46 占位)

**理由**:两域收口;stego 34(减批 1 的 3 个 misc.stego 迁入)+ reverse 13(减批 1 的 strings)。

- **核心动作**:骨架 + 徽章;stego.image/audio/video/text 分组 chips;reverse.static.strings 已在批 1 接线,本批只验证展示。
- **重定向**:无(misc.stego 已在批 1 处理)。
- **风险**:三个「whitespace」易混淆工具(whitespace-codec 批 1 / misc.encoding.whitespace 批 2 / stego.text.whitespace 本批)跨批存在,搜索时名称可区分需验证。
- **附加验收**:搜索「whitespace」命中 3 个工具且分类徽标各不相同;stego.text.whitespace 与批 2 的 misc.encoding.whitespace 描述互不误导(描述含分类前缀)。


| # | 工具 id | 名称 | 分类迁移(旧 → 新) | 模板 | 状态 | 风险 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `stego.image.lsb` | LSB 隐写提取 | stego.image | 通用 ToolForm+文件 | 占位 | — |
| 2 | `reverse.static.disasm` | 反汇编 | reverse.static | 通用 ToolForm+文件 | 已实现 | — |
| 3 | `reverse.static.elf-header` | ELF 头解析 | reverse.static | 通用 ToolForm+文件 | 占位 | — |
| 4 | `reverse.static.pe-header` | PE 头解析 | reverse.static | 通用 ToolForm+文件 | 占位 | — |
| 5 | `reverse.static.strings-unicode` | Unicode 字符串 | reverse.static | 通用 ToolForm+文件 | 占位 | — |
| 6 | `reverse.static.upx-unpack` | UPX 脱壳 | reverse.static | 通用 ToolForm+文件 | 占位 | — |
| 7 | `reverse.dynamic.strace` | 系统调用跟踪 | reverse.dynamic | 通用 ToolForm+文件 | 占位 | — |
| 8 | `reverse.dynamic.ghidra-export` | Ghidra 导出 | reverse.dynamic | 通用 ToolForm+文件 | 占位 | — |
| 9 | `reverse.dynamic.debug-script` | 调试脚本 | reverse.dynamic | 通用 ToolForm+文件 | 占位 | — |
| 10 | `reverse.deobfuscate.js-deobfuscate` | JS 去混淆 | reverse.deobfuscate | 通用 ToolForm+文件 | 占位 | — |
| 11 | `reverse.deobfuscate.pyc-decompile` | Pyc 反编译 | reverse.deobfuscate | 通用 ToolForm+文件 | 占位 | — |
| 12 | `reverse.deobfuscate.movfuscator` | Movfuscator 分析 | reverse.deobfuscate | 通用 ToolForm+文件 | 占位 | — |
| 13 | `stego.image.exif` | EXIF 信息提取 | stego.image | 通用 ToolForm+文件 | 占位 | — |
| 14 | `stego.image.png-chunk` | PNG 块分析 | stego.image | 通用 ToolForm+文件 | 占位 | — |
| 15 | `stego.image.stegsolve` | 通道分离 | stego.image | 通用 ToolForm+文件 | 占位 | — |
| 16 | `stego.audio.spectrogram` | 频谱图分析 | stego.audio | 通用 ToolForm+文件 | 占位 | — |
| 17 | `stego.audio.lsb-audio` | 音频 LSB | stego.audio | 通用 ToolForm+文件 | 占位 | — |
| 18 | `stego.video.frame-extract` | 视频帧提取 | stego.video | 通用 ToolForm+文件 | 占位 | — |
| 19 | `stego.text.whitespace` | 空白字符隐写 | stego.text | 通用 ToolForm | 占位 | — |
| 20 | `stego.text.zero-width` | 零宽字符检测 | stego.text | 通用 ToolForm | 占位 | — |
| 21 | `reverse.static.binary-analyzer` | 二进制分析器 | reverse.static | 通用 ToolForm | 占位 | — |
| 22 | `reverse.static.entropy` | 熵分析 | reverse.static | 通用 ToolForm | 占位 | — |
| 23 | `stego.image.img-stego` | 图片隐写分析 | stego.image | 通用 ToolForm | 占位 | — |
| 24 | `stego.image.base64-stego` | Base64 隐写 | stego.image | 通用 ToolForm | 占位 | — |
| 25 | `stego.text.tll-stego` | TLL 隐写 | stego.text | 通用 ToolForm | 占位 | — |
| 26 | `stego.image.qr-tools` | 二维码工具 | stego.image | 通用 ToolForm | 占位 | — |
| 27 | `stego.image.extract-rgba` | 提取 RGBA | stego.image | 通用 ToolForm | 占位 | — |
| 28 | `stego.image.bit-plane` | 查看位平面 | stego.image | 通用 ToolForm | 占位 | — |
| 29 | `stego.image.palette` | 随机调色板 | stego.image | 通用 ToolForm | 占位 | — |
| 30 | `stego.image.render-image` | 渲染图片 | stego.image | 通用 ToolForm | 占位 | — |
| 31 | `stego.image.generate-image` | 生成图片 | stego.image | 通用 ToolForm | 占位 | — |
| 32 | `stego.image.ocr` | OCR 文字识别 | stego.image | 通用 ToolForm | 占位 | — |
| 33 | `stego.image.rotate` | 旋转图片 | stego.image | 通用 ToolForm | 占位 | — |
| 34 | `stego.image.resize` | 缩放图片 | stego.image | 通用 ToolForm | 占位 | — |
| 35 | `stego.image.blur` | 模糊图片 | stego.image | 通用 ToolForm | 占位 | — |
| 36 | `stego.image.dither` | 抖动图片 | stego.image | 通用 ToolForm | 占位 | — |
| 37 | `stego.image.invert` | 反色图片 | stego.image | 通用 ToolForm | 占位 | — |
| 38 | `stego.image.flip` | 翻转图片 | stego.image | 通用 ToolForm | 占位 | — |
| 39 | `stego.image.crop` | 裁剪图片 | stego.image | 通用 ToolForm | 占位 | — |
| 40 | `stego.image.brightness` | 亮度/对比度 | stego.image | 通用 ToolForm | 占位 | — |
| 41 | `stego.image.filter` | 图像滤镜 | stego.image | 通用 ToolForm | 占位 | — |
| 42 | `stego.image.hsl` | 色相/饱和度/明度 | stego.image | 通用 ToolForm | 占位 | — |
| 43 | `stego.image.sharpen` | 锐化图片 | stego.image | 通用 ToolForm | 占位 | — |
| 44 | `stego.image.normalise` | 直方图归一化 | stego.image | 通用 ToolForm | 占位 | — |
| 45 | `stego.image.convert-format` | 转换图片格式 | stego.image | 通用 ToolForm | 占位 | — |
| 46 | `stego.image.add-text` | 图片叠加文字 | stego.image | 通用 ToolForm | 占位 | — |
| 47 | `stego.video.play-media` | 播放媒体 | stego.video | 通用 ToolForm | 占位 | — |


---

## 10. 批 9:长尾五小分类(31 个,0 已实现 / 31 占位)

**理由**:pwn 11 + osint 6 + blockchain 7 + mobile 4 + ai 3 一次收口;重名「Shellcode 编码」的 pwn 侧在此批(misc 侧在批 2,只记录)。

- **核心动作**:五分类统一骨架;每分类分组 chips 同 L2。
- **重定向**:无。
- **风险**:31 个跨 5 域,报告须按域分别给出迁移数与抽样核对。
- **附加验收**:五个 /c/:id 分类页均可访问;搜索「shellcode」命中 pwn.shellcode.encoder 与 misc.encoding.shellcode 两个,卡片标注分类可区分。


| # | 工具 id | 名称 | 分类迁移(旧 → 新) | 模板 | 状态 | 风险 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `blockchain.chain.trace` | 交易追踪 | blockchain.chain | 通用 ToolForm | 占位 | — |
| 2 | `pwn.stack.overflow-calc` | 栈溢出偏移计算 | pwn.stack | 通用 ToolForm | 占位 | — |
| 3 | `pwn.stack.rop-gadget` | ROP gadget 搜索 | pwn.stack | 通用 ToolForm+文件 | 占位 | — |
| 4 | `pwn.stack.ret2libc` | ret2libc 计算 | pwn.stack | 通用 ToolForm | 占位 | — |
| 5 | `pwn.stack.canary-bypass` | Canary 绕过 | pwn.stack | 通用 ToolForm | 占位 | — |
| 6 | `pwn.heap.chunk-analyzer` | 堆块分析 | pwn.heap | 通用 ToolForm+文件 | 占位 | — |
| 7 | `pwn.heap.tcache-poison` | tcache poisoning | pwn.heap | 通用 ToolForm | 占位 | — |
| 8 | `pwn.format.fmt-str` | 格式化字符串 | pwn.format | 通用 ToolForm | 占位 | — |
| 9 | `pwn.format.fmt-leak` | 格式化字符串泄露 | pwn.format | 通用 ToolForm | 占位 | — |
| 10 | `pwn.shellcode.encoder` | Shellcode 编码 | pwn.shellcode | 通用 ToolForm+文件 | 占位 | 重名shellcode |
| 11 | `pwn.shellcode.generator` | Shellcode 生成 | pwn.shellcode | 通用 ToolForm+文件 | 占位 | — |
| 12 | `osint.social.username-search` | 用户名搜索 | osint.social | 通用 ToolForm | 占位 | — |
| 13 | `osint.social.breach-check` | 泄露库查询 | osint.social | 通用 ToolForm | 占位 | — |
| 14 | `osint.geo.geolocate` | 地理定位 | osint.geo | 通用 ToolForm+文件 | 占位 | — |
| 15 | `osint.image.reverse-search` | 以图搜图 | osint.image | 通用 ToolForm+文件 | 占位 | — |
| 16 | `osint.domain.whois` | WHOIS 查询 | osint.domain | 通用 ToolForm | 占位 | — |
| 17 | `osint.domain.dns-enum` | DNS 枚举 | osint.domain | 通用 ToolForm | 占位 | — |
| 18 | `blockchain.chain.tx-decode` | 交易解码 | blockchain.chain | 通用 ToolForm | 占位 | — |
| 19 | `blockchain.chain.storage-read` | 合约存储读取 | blockchain.chain | 通用 ToolForm | 占位 | — |
| 20 | `blockchain.contract.solidity-audit` | Solidity 审计 | blockchain.contract | 通用 ToolForm | 占位 | — |
| 21 | `blockchain.contract.abi-decode` | ABI 解码 | blockchain.contract | 通用 ToolForm | 占位 | — |
| 22 | `blockchain.wallet.mnemonic` | 助记词生成 | blockchain.wallet | 通用 ToolForm | 占位 | — |
| 23 | `blockchain.wallet.keystore-crack` | Keystore 破解 | blockchain.wallet | 通用 ToolForm+文件 | 占位 | — |
| 24 | `mobile.android.apk-decompile` | APK 反编译 | mobile.android | 通用 ToolForm+文件 | 占位 | — |
| 25 | `mobile.android.dex-parse` | DEX 解析 | mobile.android | 通用 ToolForm+文件 | 占位 | — |
| 26 | `mobile.ios.ipa-extract` | IPA 提取 | mobile.ios | 通用 ToolForm+文件 | 占位 | — |
| 27 | `mobile.ios.plist-parse` | Plist 解析 | mobile.ios | 通用 ToolForm+文件 | 占位 | — |
| 28 | `ai.prompt.injection-test` | Prompt 注入测试 | ai.prompt | 通用 ToolForm | 占位 | — |
| 29 | `ai.analysis.flag-search` | Flag 智能检索 | ai.analysis | 通用 ToolForm | 占位 | — |
| 30 | `ai.analysis.code-summary` | 代码摘要 | ai.analysis | 通用 ToolForm | 占位 | — |
| 31 | `misc.other.de-bruijn` | De Bruijn 序列 | pwn.stack | 通用 ToolForm | 占位 | — |


---

## 11. 批 10:misc 前半(49 个,0 已实现 / 49 占位)

**理由**:misc.other 收口(A–M 段)。

- **核心动作**:骨架 + 徽章;分组 chips 按现有 L2(other/demo/report 中本段所含)。
- **重定向**:无。
- **风险**:misc 域工具语义最杂,搜索标签回填(task:)先行于此批。
- **附加验收**:本批工具 task 标签覆盖 100%(§2 受控词表);筛选「任务」维度过滤生效。


| # | 工具 id | 名称 | 分类迁移(旧 → 新) | 模板 | 状态 | 风险 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `misc.demo.http-echo` | HTTP 回显示例 | misc.demo | 通用 ToolForm | 占位 | — |
| 2 | `misc.demo.local-calc` | 本地计算示例 | misc.demo | 通用 ToolForm | 占位 | — |
| 3 | `misc.demo.sse-stream` | SSE 流式输出示例 | misc.demo | 通用 ToolForm | 占位 | — |
| 4 | `misc.demo.ws-echo` | WebSocket 回显 | misc.demo | 通用 ToolForm | 占位 | — |
| 5 | `misc.other.all-casings` | 大小写组合 | misc.other | 通用 ToolForm | 占位 | — |
| 6 | `misc.other.bitshift` | 位移/循环移位 | misc.other | 通用 ToolForm | 占位 | — |
| 7 | `misc.other.bitwise-add` | 按位 ADD | misc.other | 通用 ToolForm | 占位 | — |
| 8 | `misc.other.bitwise-and` | 按位 AND | misc.other | 通用 ToolForm | 占位 | — |
| 9 | `misc.other.bitwise-not` | 按位 NOT | misc.other | 通用 ToolForm | 占位 | — |
| 10 | `misc.other.bitwise-or` | 按位 OR | misc.other | 通用 ToolForm | 占位 | — |
| 11 | `misc.other.bitwise-sub` | 按位 SUB | misc.other | 通用 ToolForm | 占位 | — |
| 12 | `misc.other.bson` | BSON 序列化 | misc.other | 通用 ToolForm | 占位 | — |
| 13 | `misc.other.cartesian-product` | 笛卡尔积 | misc.other | 通用 ToolForm | 占位 | — |
| 14 | `misc.other.case` | 大小写转换 | misc.other | 通用 ToolForm | 占位 | — |
| 15 | `misc.other.case-convert` | 命名风格转换 | misc.other | 通用 ToolForm | 占位 | — |
| 16 | `misc.other.case-insensitive-regex` | 大小写不敏感正则 | misc.other | 通用 ToolForm | 占位 | — |
| 17 | `misc.other.chi-square` | 卡方检验 | misc.other | 通用 ToolForm | 占位 | — |
| 18 | `misc.other.code-beautify` | 代码美化 | misc.other | 通用 ToolForm | 占位 | — |
| 19 | `misc.other.colour-code` | 颜色代码解析 | misc.other | 通用 ToolForm | 占位 | — |
| 20 | `misc.other.comment` | Comment 注释 | misc.other | 通用 ToolForm | 占位 | — |
| 21 | `misc.other.conditional-jump` | Conditional Jump 条件跳转 | misc.other | 通用 ToolForm | 占位 | — |
| 22 | `misc.other.coordinate` | 坐标格式换算 | misc.other | 通用 ToolForm | 占位 | — |
| 23 | `misc.other.count` | 统计出现次数 | misc.other | 通用 ToolForm | 占位 | — |
| 24 | `misc.other.css-tidy` | CSS 格式化/压缩 | misc.other | 通用 ToolForm | 占位 | — |
| 25 | `misc.other.diff` | 文本差异对比 | misc.other | 通用 ToolForm | 占位 | — |
| 26 | `misc.other.drop-take-bytes` | 丢弃/截取字节 | misc.other | 通用 ToolForm | 占位 | — |
| 27 | `misc.other.entropy` | 香农熵计算 | misc.other | 通用 ToolForm | 占位 | — |
| 28 | `misc.other.escape-string` | 字符串转义 | misc.other | 通用 ToolForm | 占位 | — |
| 29 | `misc.other.expand-range` | 展开字母范围 | misc.other | 通用 ToolForm | 占位 | — |
| 30 | `misc.other.extract-dates` | 提取日期 | misc.other | 通用 ToolForm | 占位 | — |
| 31 | `misc.other.filetime` | Windows FILETIME 转换 | misc.other | 通用 ToolForm | 占位 | — |
| 32 | `misc.other.filter` | 过滤行 | misc.other | 通用 ToolForm | 占位 | — |
| 33 | `misc.other.find-replace` | 查找替换 | misc.other | 通用 ToolForm | 占位 | — |
| 34 | `misc.other.fork` | Fork 拆分 | misc.other | 通用 ToolForm | 占位 | — |
| 35 | `misc.other.frequency` | 频率分布统计 | misc.other | 通用 ToolForm | 占位 | — |
| 36 | `misc.other.fuzzy-match` | 模糊匹配 | misc.other | 通用 ToolForm | 占位 | — |
| 37 | `misc.other.get-time` | 获取当前时间 | misc.other | 通用 ToolForm | 占位 | — |
| 38 | `misc.other.git-extract` | Git 泄露提取 | misc.other | 通用 ToolForm+文件 | 占位 | — |
| 39 | `misc.other.hamming-distance` | 汉明距离 | misc.other | 通用 ToolForm | 占位 | — |
| 40 | `misc.other.haversine` | 大圆距离 | misc.other | 通用 ToolForm | 占位 | — |
| 41 | `misc.other.head-tail` | 取头/取尾 | misc.other | 通用 ToolForm | 占位 | — |
| 42 | `misc.other.heatmap-chart` | 热力图 | misc.other | 通用 ToolForm | 占位 | — |
| 43 | `misc.other.hex-density-chart` | 十六进制密度图 | misc.other | 通用 ToolForm | 占位 | — |
| 44 | `misc.other.hotp` | 生成 HOTP | misc.other | 通用 ToolForm | 占位 | — |
| 45 | `misc.other.html-to-text` | HTML 转文本 | misc.other | 通用 ToolForm | 占位 | — |
| 46 | `misc.other.index-coincidence` | 重合指数 | misc.other | 通用 ToolForm | 占位 | — |
| 47 | `misc.other.js-tools` | JS 解析/美化/压缩 | misc.other | 通用 ToolForm | 占位 | — |
| 48 | `misc.other.json-tidy` | JSON 格式化/压缩 | misc.other | 通用 ToolForm | 占位 | — |
| 49 | `misc.other.jump` | Jump 跳转 | misc.other | 通用 ToolForm | 占位 | — |


---

## 12. 批 11:misc 后半(49 个,1 已实现 / 48 占位)

**理由**:收口批;含 php-unserialize(专用工作区)与 report/demo 组。

- **核心动作**:骨架 + 徽章;php-unserialize 走 PhpUnserializeWorkspace 模板;demo 组(4 个)与 beast 一并标注「演示」task 标签。
- **重定向**:无。
- **风险**:php-unserialize 工作区与统一头部模板的兼容(批 1 模板先在小范围验证过)。
- **附加验收**:php-unserialize 全链路(序列化字符串输入→输出)回归;全部 11 批完成后:`inventory:check` 全绿 + 565 行清单与运行时 UI 一致性全量核对。


| # | 工具 id | 名称 | 分类迁移(旧 → 新) | 模板 | 状态 | 风险 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `misc.other.label` | Label 标签 | misc.other | 通用 ToolForm | 占位 | — |
| 2 | `misc.other.levenshtein` | 莱文斯坦距离 | misc.other | 通用 ToolForm | 占位 | — |
| 3 | `misc.other.line-numbers` | 行号处理 | misc.other | 通用 ToolForm | 占位 | — |
| 4 | `misc.other.lorem-ipsum` | Lorem Ipsum | misc.other | 通用 ToolForm | 占位 | — |
| 5 | `misc.other.magic` | Magic 智能识别 | misc.other | 通用 ToolForm | 占位 | — |
| 6 | `misc.other.magic-bytes` | 文件头识别 | misc.other | 通用 ToolForm+文件 | 占位 | — |
| 7 | `misc.other.math-stats` | 数值统计 | misc.other | 通用 ToolForm | 占位 | — |
| 8 | `misc.other.merge` | Merge 合并 | misc.other | 通用 ToolForm | 占位 | — |
| 9 | `misc.other.ms-script-decode` | 微软脚本解码 | misc.other | 通用 ToolForm | 占位 | — |
| 10 | `misc.other.objectid` | ObjectID 时间戳 | misc.other | 通用 ToolForm | 占位 | — |
| 11 | `misc.other.offset-checker` | 字节偏移检查 | misc.other | 通用 ToolForm | 占位 | — |
| 12 | `misc.other.pad-lines` | 行填充 | misc.other | 通用 ToolForm | 占位 | — |
| 13 | `misc.other.parse-datetime` | 解析日期时间 | misc.other | 通用 ToolForm | 占位 | — |
| 14 | `misc.other.plist-viewer` | P-list 查看器 | misc.other | 通用 ToolForm | 占位 | — |
| 15 | `misc.other.power-set` | 幂集 | misc.other | 通用 ToolForm | 占位 | — |
| 16 | `misc.other.rar-crack` | RAR 破解 | misc.other | 通用 ToolForm+文件 | 占位 | — |
| 17 | `misc.other.regex` | 正则匹配 | misc.other | 通用 ToolForm | 占位 | — |
| 18 | `misc.other.register` | Register 寄存器 | misc.other | 通用 ToolForm | 占位 | — |
| 19 | `misc.other.remove-null` | 去除空字节 | misc.other | 通用 ToolForm | 占位 | — |
| 20 | `misc.other.remove-whitespace` | 去除空白 | misc.other | 通用 ToolForm | 占位 | — |
| 21 | `misc.other.render-markdown` | 渲染 Markdown | misc.other | 通用 ToolForm | 占位 | — |
| 22 | `misc.other.return` | Return 返回 | misc.other | 通用 ToolForm | 占位 | — |
| 23 | `misc.other.scatter-chart` | 散点图 | misc.other | 通用 ToolForm | 占位 | — |
| 24 | `misc.other.serial-number` | 序列号生成 | misc.other | 通用 ToolForm | 占位 | — |
| 25 | `misc.other.series-chart` | 序列折线图 | misc.other | 通用 ToolForm | 占位 | — |
| 26 | `misc.other.set-difference` | 集合差集 | misc.other | 通用 ToolForm | 占位 | — |
| 27 | `misc.other.set-intersection` | 集合交集 | misc.other | 通用 ToolForm | 占位 | — |
| 28 | `misc.other.set-symmetric` | 集合对称差 | misc.other | 通用 ToolForm | 占位 | — |
| 29 | `misc.other.set-union` | 集合并集 | misc.other | 通用 ToolForm | 占位 | — |
| 30 | `misc.other.show-on-map` | 地图标点 | misc.other | 通用 ToolForm | 占位 | — |
| 31 | `misc.other.shuffle` | 随机打乱 | misc.other | 通用 ToolForm | 占位 | — |
| 32 | `misc.other.sleep` | 延时 | misc.other | 通用 ToolForm | 占位 | — |
| 33 | `misc.other.sort` | 排序 | misc.other | 通用 ToolForm | 占位 | — |
| 34 | `misc.other.split` | 拆分 | misc.other | 通用 ToolForm | 占位 | — |
| 35 | `misc.other.sql-tidy` | SQL 格式化/压缩 | misc.other | 通用 ToolForm | 占位 | — |
| 36 | `misc.other.strip-html` | 去除 HTML 标签 | misc.other | 通用 ToolForm | 占位 | — |
| 37 | `misc.other.subsection` | Subsection 子流程 | misc.other | 通用 ToolForm | 占位 | — |
| 38 | `misc.other.syntax-highlight` | 语法高亮 | misc.other | 通用 ToolForm | 占位 | — |
| 39 | `misc.other.to-table` | 转 Markdown 表格 | misc.other | 通用 ToolForm | 占位 | — |
| 40 | `misc.other.totp` | 生成 TOTP | misc.other | 通用 ToolForm | 占位 | — |
| 41 | `misc.other.translate-datetime` | 日期时间格式转换 | misc.other | 通用 ToolForm | 占位 | — |
| 42 | `misc.other.unique` | 去重 | misc.other | 通用 ToolForm | 占位 | — |
| 43 | `misc.other.unit-convert` | 单位换算 | misc.other | 通用 ToolForm | 占位 | — |
| 44 | `misc.other.unix-permissions` | Unix 权限解析 | misc.other | 通用 ToolForm | 占位 | — |
| 45 | `misc.other.uuid` | 生成 UUID | misc.other | 通用 ToolForm | 占位 | — |
| 46 | `misc.other.xml-tidy` | XML 格式化/压缩 | misc.other | 通用 ToolForm | 占位 | — |
| 47 | `misc.other.zip-brute` | ZIP 密码爆破 | misc.other | 通用 ToolForm+文件 | 占位 | — |
| 48 | `misc.report.flag-checker` | Flag 校验 | misc.report | 通用 ToolForm | 占位 | — |
| 49 | `misc.report.writeup` | Writeup 报告生成 | misc.report | 通用 ToolForm | 已实现 | — |


---

## 13. 通用验收标准(每批必过)

| # | 验收项 | 检查方法 |
| --- | --- | --- |
| 1 | 构建通过 | `npx tsc --noEmit` 0 错误;`npm run build` 成功;Go 侧改动时 `go vet ./...` + `go test ./...` 全过 |
| 2 | 路由可访问 | 本批全部工具 /t/:id 无 404;涉及分类页 /c/:id 可达;旧路径按重定向表生效 |
| 3 | 占位标识正确 | 脚本全量核对:UI 徽章与 inventory status 一致(implemented 59/placeholder 505 口径不变);ssti 19 个显示「后端占位」 |
| 4 | 移动端无溢出 | 360 / 768 / 1024 / 1440 四档,`scrollWidth ≤ innerWidth`(Playwright 断言);抽屉正常开合 |
| 5 | 卡片/标题/间距一致 | 截图比对 design-system §3.1/§3.2 规范;同行卡片等高;徽章/标签/收藏按钮位置一致 |
| 6 | 搜索筛选可用 | 名称/标签/状态组合检索;分类页 6 维筛选 + 状态优先排序(占位沉底);筛选 chips 计数正确 |
| 7 | 无控制台错误 | 逐页走查(首页/分类/工具/搜索/404)0 error、0 未处理 rejection;mock 路径无未捕获异常 |
| 8 | 批次报告(规则 9) | 变更文件清单、迁移/新增/占位数、未决问题、截图描述(明暗两套)、构建与测试结果、下批预告 |

**批 0 附加**:设计令牌明暗两套生效、9 个基础组件存在、`inventory:check`(含对比度校验)全绿。
**批 1 附加**:§2.3。
**其余批次**:各自 §3~§12 的「附加验收」。

---

## 14. 全局约束(任何批次不得违反)

1. **工具 id 不可变**——迁移全程不改 id;收藏/最近/历史(localStorage)零失效。改名/合并/规范化(Q8/Q12/F6/重名组)一律阶段 4 单独批,先映射后重定向。
2. **不删除旧路由**——分类重定向为 301 语义保留;`categories.ts` 旧节点删除以 redirects.json 覆盖为前置条件。
3. **清单即界面**——UI 渲染、徽章、筛选、搜索全部由 inventory 派生;每批先改清单(含 task 标签回填),再改 UI;禁止组件写死工具逻辑(design-system §9)。
4. **占位功能排期独立**——迁移批不实现占位业务逻辑(Q7 未决);实现类工作另立排期,每批最多附带 3 个且须单列。
5. **每批原子提交**——git(git init 后)按批提交,回滚粒度 = 批次;批 5 的重定向+扁平化为单次原子提交。
6. **等待指定批次**——本文档产出后不开始任何代码变更,直到你明确指定批次。

---

*附:全量工具表由 `docs/tool-inventory.json` 经分区脚本生成(批 2~11 共 553 行 + 批 1 手写 12 行 = 565 行),生成脚本与依据可复现(见审计方法,audit-report.md §11)。*
