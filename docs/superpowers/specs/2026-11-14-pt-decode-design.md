# 设计:思科 Packet Tracer 文件解码(misc.pt_decode)

日期:2026-11-14
状态:待用户批准
规模:项目级(Go 纯净实现 Twofish + 容器解密 + 信息提取;无自定义渲染器)

## 意图与为什么

CTF 常见题型:附件是思科 Packet Tracer 工程文件(.pkt/.pka 加密容器),flag/信息藏在拓扑 XML 的设备配置里。需要一键:识别 → 四层解密 → 落盘 XML → 提取 flag/密码/版本。算法由用户提供的 Bugku cisco 复现 writeup 完整钉死,且提供加密样本(思科/1.txt,46108B)与金标准明文(pt_tools/challenge.xml,800965B)做对拍。

## 方案取舍

| 方案 | 结论 |
|---|---|
| Go 纯净实现(Twofish 自实现 + 标准库 zlib) | **选定**——零新依赖,与项目惯例一致 |
| 集成外部工具(pka_decipher 等) | 违背单 exe 零依赖。否决 |
| Go 第三方 twofish 库 | 引依赖;且魔改 EAX 反正要手搓。否决 |

## 功能设计

**工具**:`misc.pt_decode`「思科 Packet Tracer 解码」,misc 分类(前端条目 `misc.other.pt-decode` 挂 `misc.other`),CTF 模式可见。参数:file(必填)。

**解密管线(writeup 逐字节钉死)**:

```
① stage1 去混淆:out[i] = data[L-1-i] ^ ((L - i*L) & 0xFF)   L=文件长(整文件反转+长度相关 XOR)
② Twofish-128 魔改 EAX:key=0x89×16、nonce=0x10×16(固定硬编码)
   N = CMAC(0x00×16 ‖ nonce)  ← 前缀块直接拼接,非标准 EAX tweak 异或
   CTR 自 N 整 128 位大端 +1;密文=stage1[:-16],tag=stage1[-16:](跳过校验)
③ stage2 去混淆:dec[i] ^ ((L'-i) & 0xFF)   L'=解密后长度
④ 前 4 字节大端=未压缩长度,其后 zlib 流 → XML
```

**信息提取**(XML 解出后):

- `flag{...}`/`ctf{...}` 等正则全扫(stego.FlagRe 复用);
- **敏感配置行**:startup-config 中的 `password`/`secret`/`banner`/`enable` 行(含上下文行,正是本题 flag 所在形态);
- `<VERSION>` 版本号、拓扑设备名(`<Name>`)清单(去重截断);
- 解密统计(各阶段长度、压缩比)。

**输出**:markdown 文本报告(无需自定义渲染器)+ artifacts:`decoded.xml`(完整明文拓扑,可下载继续人工分析)。解密失败(zlib 头错/长度异常)给出友好错误并提示「可能不是 PT 容器或版本不适用(固定密钥对应 PT 8.x 之前)」。

**实现文件**:`internal/tools/misc/pt_crypto.go`(Twofish-128 + CMAC + CTR + 容器解密,~350 行,Twofish 从已验证的参考实现移植)+ `pt_decode.go`(工具注册与提取)。Twofish 参考向量:key=0^16、pt=0^16 → `9F589F5CF6122C32B6BFEC2F2AE8C35A`(writeup 标注唯一易错点是 t1 的 rotl 方向)。

## 非目标

- 不做 PT 8.x+ 新加密格式(密钥不同);不做拓扑可视化;不做 XML 内嵌 base64 文件提取(后续可加)。

## 测试口径

1. **Twofish 官方向量**(TDD RED 先行):全零 key/pt → 上记密文;再加 writeup 里验证过的解密方向(解 9F58.. 得全零)。
2. **合成往返**:测试内实现加密逆函数(stage2 逆 → CTR 加密 → stage1 逆),对 challenge.xml 前 64KB 加密再解密,字节还原。
3. **金标准对拍(live 门控,样本缺失即跳过)**:`C:\Users\HP\Desktop\安全方向\CTF工作台-plus\思科\1.txt` 解出的 XML 与 `pt_tools/challenge.xml` 逐字节一致,flag 提取命中 `flag{wenzi_houzhui_sike}`。
4. 坏输入(纯文本文件)→ 友好错误不崩溃。
5. `go test ./internal/tools/misc/` 全绿;`wails build` 出 exe;README 收录;前端 tools.ts 条目 + build。

## 留给实现

1. `pt_crypto.go`:Twofish(key schedule/MDS/q 表)、CMAC(OMAC1)、CTR、decryptPT(data)→xml。
2. `pt_decode.go`:注册 + 提取 + 报告 + artifact。
3. 测试两文件;前端条目;README 行;exe。
