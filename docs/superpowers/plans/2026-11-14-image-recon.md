# 图片二进制侦察(image_recon)Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 CTF 图片二进制层隐写侦察工具:格式/结构解析 + 结束标记后附加数据提取 + 全文件魔数扫描(ZIP EOCD 定界)+ strings/flag,artifacts 可下载;含专属前端渲染器。

**Architecture:** Go 后端工具 `misc.image_recon`(注册表+任务队列,Runner 形态对齐 audio_lsb:JSON 报告 + summary + artifacts),复用 `internal/stego` 引擎(Detect/Cut/Flags/PrintableStrings/Magics);前端 `rendererId: 'image-recon'` 专属渲染器(结构段表/尾部 hex+复制/命中表)。

**Tech Stack:** Go(既有,零新依赖);React + TS(既有)。

**Spec:** `docs/superpowers/specs/2026-11-14-image-recon-design.md`(含 builder check 四项裁定:EOI 正向扫描/私有文本启发/2 字节魔数策略/渲染器入 scope)

## Global Constraints

- 不改 `internal/stego` 冻结基座(audio_lsb/image_lsb 依赖);文本启发等新增逻辑全部放 misc 包内。
- JPG 宿主 EOI 必须从 SOS 之后**正向扫描**首个 FFD9(禁用 rfind——尾部嵌整图场景会误判)。
- 2 字节魔数(BM/MZ)远离偏移 0 仅标 `suspected` 不导出;GZIP 用 3 字节 `1F 8B 08` 全置信。
- 零新依赖;`go test ./...` 除已知 aisec 环境性失败外全绿;提交信息中文 + Co-Authored-By。
- 前端验证口径 = `npm run build` + grep + 手动;Go = go test。

## Review Focus

1. **EOI 正向扫描**(尾部嵌整张 JPG 不被吸收)→ Task 1 测试 5 钉死(trailing size == 嵌入图大小)。
2. **ZIP EOCD 定界正确性**(artifact 能被 archive/zip 重新打开)→ Task 1 测试 2 钉死。
3. **短尾部文本启发**(24B 打乱 flag 判文本不判 BIN)→ Task 1 测试 1/4 钉死(gugugu 真实尾串)。
4. **渲染器契约**(stdout JSON 字段名与 renderer 解析一致,未注册时静默降级 RawText)→ Task 2 grep 断言 rendererId 三处一致(tools.ts/rendererRegistry/renderer 文件名)+ build。
5. **魔数误报噪声**(熵区 2 字节随机命中不出现在 artifact 列表)→ Task 1 测试 3(干净图零命中零 artifact)。

---

### Task 1: Go 工具 image_recon(TDD)

**Files:**
- Test: `internal/tools/misc/image_recon_test.go`
- Create: `internal/tools/misc/image_recon.go`

**Interfaces:**
- Consumes: `internal/stego`:`Detect(bs) (name, ext, mime string, offset int)`、`Cut(bs, offset, typ)`、`Flags(win)`、`PrintableStrings(data, minLen, max)`、`Magics`、`TypeOf(name, ext)`;`core.BaseTool/ParamSpec/DefaultRegistry`(audio_lsb 同款);Runner 内 `tc.ArtifactsDir` 写产物。
- Produces: 注册工具 `misc.image_recon`;报告 JSON 字段(renderer 契约,snake_case):
  - `file {name, size, format, mime}`
  - `structure[] {kind:"jpg-seg"|"png-chunk"|"riff-chunk", name, offset, length, crc_ok?, note?}`(JPG 段名 APP0/COM/DQT/SOF0/DHT/SOS/EOI…;PNG chunk 名 IHDR/…/IEND)
  - `trailing {offset, size, type, ext, preview_hex(≤128B), preview_text(≤256r), artifact}` 或 `null`
  - `embedded[] {name, offset, size, artifact, suspected}`
  - `strings {count, previews[≤200](每条≤64r), flags[]}`
  - `artifacts[]`, `summary`(纯文本结论块)
- 参数:`file`(file,必填)、`strings_min`(str,默认 "6")。

- [ ] Step 1: 写失败测试(5 个,合成样本,t.TempDir,经 `core.DefaultRegistry.Get("misc.image_recon")` 走 Runner——形态对照 audio_lsb_test.go):
  1. 合成 JPG(SOI+APP0+DQT+SOF0+SOS+熵数据+EOI)+ 24B 文本尾 `fg2ivyo}l{2s3_o@aw__rcl@` → trailing.size=24、type 含"文本"、artifact 存在且内容相等;
  2. 合成 PNG(签名+IHDR+IDAT+IEND)+ 内嵌最小 ZIP(archive/zip 生成,含一个文件)→ structure 含 IHDR/IEND;embedded 命中 ZIP 且 artifact 重新 `zip.OpenReader` 能读出条目;
  3. 干净 JPG(EOI 即尾)→ trailing null、embedded 空、artifacts 空;
  4. 测试 1 的尾部串出现在 strings.previews;
  5. **EOI 正向扫描**:JPG + 整张合成小 JPG(含自身 EOI)作尾部 → trailing.size == 嵌入图完整大小(不被吸收),trailing.type 识别为图片。
- [ ] Step 2: 跑测试看失败(`undefined: imageReconRun` / 注册不存在)。
- [ ] Step 3: 实现 image_recon.go:
  - 注册块镜像 audio_lsb(NameAttr "image_recon", CatMisc, Runner);
  - 格式识别:FFD8FF→jpg / 89504E47→png / GIF87a·89a→gif / BM+14B 头→bmp / RIFF…WEBP→webp;未识别→structure 空,仍做通用扫描;
  - JPG 段 walk(偏移 2 起:FF xx + 2B 大端长度;SOS 后正向扫 FFD9);PNG chunk walk(8 起:len(4BE)+type+data+crc(4),IHDR 解宽高,CRC 记录不判死);GIF rfind 3B;BMP `bfOffBits+((bpp*w+31)/32*4)*h` 对比文件大小;WEBP 按 RIFF size;
  - trailing 类型:`stego.Detect` 命中(≥3B 魔数)→ `stego.Cut` 智能截断;否则私有 `isMostlyPrintable(bs)`(≥4B 且可打印占比 ≥80%)→ 文本;否则 bin;artifact 名 `trailing.<ext|txt|bin>`;
  - 魔数扫描:遍历 `stego.Magics`(实现时核对其字段名)+ 本地补充 `1F 8B 08`(gzip3);跳过 @0 宿主自身格式;`PK\x03\x04` → 向后找 `PK\x05\x06` EOCD(22+注释长)定界,失败回退到文件尾;魔数字节长 <3 且 offset>0 → `suspected:true` 不导出;
  - strings/flags/summary/artifacts 落盘(对照 audio_lsb 的 tc.ArtifactsDir 用法)。
- [ ] Step 4: 跑测试全过;`go test ./internal/... ./...` 全绿(aisec 已知环境失败除外)。
- [ ] Step 5: Commit `feat: 图片二进制侦察工具(结构解析+尾部提取+魔数扫描+strings,TDD 5 例)`

### Task 2: 前端渲染器 + 接线

**Files:**
- Create: `frontend/src/components/output/renderers/ImageReconRenderer.tsx`
- Modify: `frontend/src/components/output/rendererRegistry.tsx`(import + `'image-recon': ImageReconRenderer`)
- Modify: `frontend/src/mock/tools.ts`(条目,形态对照 3176-3250 的 image-lsb:含 note、file 参数、`output: { type: 'custom', rendererId: 'image-recon' }`;id `stego.image.image-recon`,categoryId `stego.image`)

**Interfaces:**
- Consumes: Task 1 的 JSON 契约;`RendererProps`(renderers/types)、`CopyButton/DownloadButton`(OutputActions)。
- Renders: 头卡(格式/大小/mime/artifacts 下载按钮)→ 结构段表(kind/name/offset/length/crc,异常 note 高亮)→ 尾部块(offset/size/type + hex 网格 ≤128B + 文本预览 + 复制 + 下载)→ 嵌入表(名称/偏移/大小/suspected 标记/下载)→ strings(flags 高亮 + 复制)→ summary。parsePayload 解析 stdout JSON,失败回退原文显示。

- [ ] Step 1: 实现 ImageReconRenderer(样式对照 AudioLsbRenderer 的卡片/表格惯例)。
- [ ] Step 2: registry 注册 + tools.ts 条目;grep 断言:`image-recon` 在 tools.ts(1)/rendererRegistry(≥2:import+表)/渲染器文件注释(1)三处一致;`npm run build` exit 0。
- [ ] Step 3: 手动(dev):上传 gugugu.jpg → 四块报告渲染正常、尾部 24B 可复制、artifact 可下载。清单交用户/终验。
- [ ] Step 4: Commit `feat: 图片二进制侦察前端渲染器与工具入口`

### Task 3: README + exe + 终审

- [ ] Step 1: README 工具表补 `misc.image_recon` 行;wails build 出 exe;桌面端 gugugu 全流程(上传→报告→下载 artifact→复制尾部串)。
- [ ] Step 2: Commit(docs+dist);独立子代理终审(包:Task1-3 增量,Review Focus 五条;gugugu.jpg 可作为真实样本供评审实测);发现分级处置。
