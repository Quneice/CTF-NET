# 设计:图片二进制侦察(image_recon)

日期:2026-11-14
状态:待用户批准
规模:项目级(Go 后端真工具 + 前端目录接线,复用 internal/stego 引擎)

## 意图与为什么

CTF 图片题常把信息甚至整个文件藏进图片**二进制层**(而非像素):结束标记(EOI/IEND)后附加数据、中部嵌入压缩包/图片、尾部藏打乱的 flag。当前工具只有像素层 LSB(image_lsb),二进制层侦察缺位。用户示例 gugugu.jpg 实测即「JPG EOI 后 24 字节打乱 flag」案例。

## 方案取舍

| 方案 | 结论 |
|---|---|
| A Go 后端真工具,复用 internal/stego | **选定**——Detect/Cut/Flags/PrintableStrings/Magics 全现成,与 audio_lsb 同基座,artifacts 导出走统一机制 |
| B 纯前端 local | 产物导出走不了 artifacts,大文件差。否决 |
| C 外挂 binwalk | 违背单 exe 零依赖。否决 |

## 功能设计

**工具**:`misc.image_recon`(后端注册表,CatMisc;前端条目 id `stego.image.image-recon` 挂 `stego.image`「图片隐写」segment,与 image-lsb 并列;CTF 模式可见)。

**参数**:file(必填,图片文件 file_id)、strings_min(str,默认 6,可打印串最小长度)。

**报告四块**(结构化 JSON,输出形态对齐 audio_lsb:markdown 文本 + artifacts 列表):

1. **格式与结构解析**
   - JPG:marker walk(SOI/APPn/COM/DQT/SOF/DHT/SOS…),逐段 {marker, 名称, 偏移, 长度, 异常标注};SOS 后直接扫 EOI。
   - PNG:chunk walk(长度+类型+数据+CRC),逐 chunk {type, 偏移, 长度, crc_ok, 异常标注(未知类型/CRC 错/超大)};IHDR 给尺寸。
   - GIF/BMP/WEBP:基本识别 + 尾部检测(BMP 按 header 声明尺寸对比实际)。
2. **尾部附加数据**:结束标记(JPG=最后 FFD9 / PNG=IEND / GIF=0x3B / BMP=声明数据末)之后的所有字节;`stego.Detect` 识别类型、`stego.Cut` 智能截断(尾部本身是图片/压缩包时按格式截);预览(hex 前 128B + 可打印文本);**落盘 artifact**(按识别类型命名 `trailing.<ext>`,未识别 `trailing.bin`)。
3. **全文件魔数扫描**:复用 `stego.Magics` 全表扫描(剔除宿主自身格式 @0 的命中;ICO 类高误报模式不引入);ZIP 命中时按 EOCD(`PK\x05\x06` + 注释长)精确定界导出完整压缩包;每个命中 {名称, 偏移, 大小, artifact}。ZIP 定界失败时回退「从魔数到文件尾」。
4. **strings + flag**:全文件可打印串(`stego.PrintableStrings`,长串截断预览)+ `stego.Flags` 正则命中;尾部/嵌入文本在报告中以可复制代码块呈现(衔接移位变换爆破——示例的打乱 flag 场景)。

**artifacts**:trailing 数据、各嵌入文件切片,写 `tc.ArtifactsDir`(audio_lsb 同款);报告附 artifacts 清单。

## 关键实现决策

- JPG 段 walk 容错:非法长度/越界即停止并标注「结构异常 @offset」,不崩溃;SOS 段后进入熵编码区,**从 SOS 数据末正向扫描首个 FFD9 定宿主 EOI**(熵编码区因字节填充不会合法出现 FFD9;`rfind` 会命中嵌入图片自身的 EOI 造成尾部误判为空——builder check #3)。
- PNG CRC 校验:仅记录不判定失败(CRC 坏有时正是考点)。
- 魔数扫描排除:偏移 0 的宿主自身;同宿主格式在内部出现(PNG 里嵌 PNG)仍报告——那是真考点。
- **2 字节魔数策略**(builder check #4):GZIP 升级为 3 字节前缀 `1F 8B 08`(deflate 方法字节)获得全置信命中;其余 2 字节模式(BM/MZ)远离偏移 0 时仅列「疑似(低置信)」不导出 artifact(熵区随机命中率 ~1/64K/模式,导出全是垃圾)。
- **短流文本启发**(builder check #2):`stego.isTextStream` 硬门槛 ≥32 字节,24 字节尾部(正是 gugugu 案例)会被判 BIN——在 misc 包内加私有启发(≥4 字节且可打印占比 ≥80% → 文本,artifact 命名 `trailing.txt`),不改冻结的共享引擎基座。
- 尾部数据 ≤0 字节时报告「无尾部附加」;>16MB 文件提示仍可分析但预览截断。
- 输出确认为「JSON 报告 + 纯文本 summary 块」(audio_lsb 实际形态,非 markdown);**前端新增专属 renderer**(`rendererId: 'image-recon'`,rendererRegistry 注册,~300 行,形态参照 AudioLsbRenderer 但契约独立:结构段表/尾部预览/魔数命中/strings 可复制块)——scope 需用户确认。

## 非目标

- 不做像素层 LSB(image_lsb 已有);不做 JPG 具体编码器级解析(只到段层);不做隐写密码爆破(压缩包密码交给外部思路)。

## 测试口径

- **Go 单测**(新建 `internal/tools/misc/image_recon_test.go`,合成样本,不依赖用户文件):
  1. 合成 JPG(SOI+APP0+DQT+SOI 尾 FFD9+`trailing-data` 24B)→ 尾部命中 24B、类型=文本、artifact 生成;
  2. 合成 PNG(签名+IHDR+IEND+完整小 ZIP)→ PNG chunk 清单含 IHDR/IEND、嵌入 ZIP 命中且 EOCD 定界后 artifact 可被 `archive/zip` 打开;
  3. 纯净图(无尾部无嵌入)→ 报告「无尾部附加/无嵌入」不误报;
  4. 打乱 flag 尾部(用 gugugu.jpg 的真实尾部串 `fg2ivyo}l{2s3_o@aw__rcl@`)→ strings 预览含该串(可复制块)。
- **手动终验**:用户示例 C:\Users\HP\Downloads\gugugu.jpg 全流程(上传→报告→artifact 下载→复制尾部串到移位变换);红队/其他回归不涉。
- `go test ./...` 全绿(已知 aisec 环境性失败除外)+ `wails build` 出 exe + README 收录。

## 留给实现

1. `internal/tools/misc/image_recon.go`(解析器+扫描+报告+artifacts,Runner 形态对齐 audio_lsb)。
2. 前端 `tools.ts` 条目(backend 型,含 note 说明与 file 字段、rendererId)。
3. **`frontend/src/components/output/renderers/ImageReconRenderer.tsx`**(专属渲染器:格式头卡/结构段表/尾部块 hex+文本+复制/魔数命中表/strings+flag 块,rendererRegistry 注册)。
4. README 工具表补一行;wailsjs 不需再生成(无新绑定)。
