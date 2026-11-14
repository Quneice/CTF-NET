# 设计:steghide 图片隐写解码(misc.steghide)

日期:2026-11-14
状态:待用户批准(方案 A 外挂 + 官方下载已获口头批准)
规模:项目级(外挂工具集成,fscan 同款;含设置项与三模式爆破)

## 意图与为什么

CTF 图片隐写最常用工具之一是 steghide(JPG/BMP/WAV 的 DCT/LSB 图论嵌入 + 可选加密压缩)。提取需要密码,九成题目是空密码或弱口令——一键「空密码探测 / 指定密码 / 字典爆破」是刚需。

## 方案取舍

| 方案 | 结论 |
|---|---|
| A 外挂官方 steghide.exe(fscan 同款) | **选定**——参考实现 100% 兼容;官方 0.5.1 win32 独立包(SourceForge,GPL);设置页配路径 |
| B 纯 Go 重写提取算法 | 需移植 libjpeg DCT 序 + 图论采样选择 + blowfish 派生,数千行,质量风险。否决(远期) |

## 功能设计

**设置**:settings.go 加 `Steghide string`(默认指向解压后的 exe 路径)+ `ExternalTool("steghide")` case + SettingsPage 外部工具一行。

**工具**:`misc.steghide`(前端 `stego.image.steghide` 挂 stego.image「图片隐写」,CTF 模式)。参数:

- file(必填,JPG/BMP/WAV)
- mode:empty(默认)| custom | dict
- password(custom 模式密码)
- dict(dict 模式字典文件 file_id;逐行,去空行,上限 10 万行)
- 每次尝试:`steghide extract -sf <stego> -p <密码> -xf out.bin -f -q`(cwd=workdir,单次超时 60s);成功 = exit 0 + 输出文件生成,命中即停
- 报告:命中密码、提取文件类型(stego 引擎魔数/文本判定)、内容预览、flag 正则;artifact 落盘(按类型命名);全 miss 给建议(空密码/换字典/可能不是 steghide)
- 进度:ReportProgress 按尝试批次;exe 缺失 → StatusError 给获取指引

## 测试口径(live 门控 steghide.exe 存在才跑,否则 Skip)

1. fixture 自造:Go `image/jpeg` 合成渐变图 → `steghide embed -cf cover.jpg -ef secret.txt -p ""` → 工具 empty 模式提取,内容一致;
2. custom:embed `-p pass123` → 提取命中;
3. dict:embed `-p letmein` → 临时字典含之,命中;
4. 错误密码:graceful 未命中报告;
5. 设置路径指向不存在文件 → 指引性错误。
go test 全绿 + wails build + README + 设置页/工具条目 build。

## 非目标

- 不做嵌入(只提取);不做并发爆破(顺序 + 进度足够);不内置字典(上传或路径)。
