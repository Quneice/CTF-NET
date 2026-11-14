package misc

// misc.image_recon:图片二进制层隐写侦察(binwalk/zsteg 二进制侧套路)。
// 四块报告:格式与结构解析(JPG 段 / PNG chunk / RIFF·BMP·GIF 尾部)、
// 结束标记后附加数据提取(EOI 正向扫描定位)、全文件魔数扫描(ZIP 按 EOCD 定界,
// 2 字节魔数远离偏移 0 仅标疑似)、strings/flag 扫描。产物经 tc.ArtifactsDir 落盘。
// 与 audio_lsb / image_lsb 共享 internal/stego 引擎(魔数识别/智能截断/flag 正则),
// 不改动该冻结基座;短流文本启发(引擎门槛 ≥32B)在本包内私有实现。

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ctfnet/internal/core"
	"ctfnet/internal/stego"
)

func init() {
	core.DefaultRegistry.Register(&ImageReconTool{BaseTool: core.BaseTool{
		NameAttr:        "image_recon",
		CategoryAttr:    core.CatMisc,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "图片二进制侦察:结构解析(JPG 段/PNG chunk)+ 结束标记后附加数据提取 + 全文件魔数扫描(ZIP EOCD 定界)+ strings/flag,尾部与嵌入文件落盘可下载",
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "上传的图片文件 file_id"},
			{Name: "strings_min", Type: "str", Required: false, Default: "6",
				Description: "可打印字符串最小长度(默认 6)"},
		},
		Runner: imageReconRun,
	}})
}

// ImageReconTool 图片二进制侦察工具。
type ImageReconTool struct {
	core.BaseTool
}

// ---------- 报告结构(与前端 ImageReconRenderer 契约一致) ----------

type imageReconSeg struct {
	Kind   string `json:"kind"` // jpg-seg | png-chunk
	Name   string `json:"name"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	CRCOk  *bool  `json:"crc_ok,omitempty"` // 仅 PNG
	Note   string `json:"note,omitempty"`
}

type imageReconTrailing struct {
	Offset      int    `json:"offset"`
	Size        int    `json:"size"`
	Type        string `json:"type"`
	Ext         string `json:"ext"`
	PreviewHex  string `json:"preview_hex"`
	PreviewText string `json:"preview_text"`
	Artifact    string `json:"artifact"`
}

type imageReconEmbedded struct {
	Name      string `json:"name"`
	Offset    int    `json:"offset"`
	Size      int    `json:"size"`
	Artifact  string `json:"artifact,omitempty"`
	Suspected bool   `json:"suspected"`
}

type imageReconStrings struct {
	Count    int      `json:"count"`
	Previews []string `json:"previews"`
	Flags    []string `json:"flags"`
}

type imageReconReport struct {
	File struct {
		Name   string `json:"name"`
		Size   int    `json:"size"`
		Format string `json:"format"`
		Mime   string `json:"mime"`
	} `json:"file"`
	Params    map[string]string    `json:"params"`
	Structure []imageReconSeg      `json:"structure"`
	Trailing  *imageReconTrailing  `json:"trailing"`
	Embedded  []imageReconEmbedded `json:"embedded"`
	Strings   *imageReconStrings   `json:"strings"`
	Artifacts []string             `json:"artifacts"`
	Summary   string               `json:"summary"`
}

// ---------- Runner ----------

func imageReconRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	report := func(p int, msg string) {
		if tc != nil && tc.ReportProgress != nil {
			tc.ReportProgress(p, msg)
		}
	}
	report(5, "读取图片文件")
	filePath := filepath.Join(tc.Workdir, paramStr(params, "file"))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("读取文件失败: %v", err)}, nil
	}
	stringsMin := 6
	if v := paramStr(params, "strings_min"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &stringsMin); err != nil || stringsMin < 3 {
			stringsMin = 6
		}
	}

	rep := &imageReconReport{
		Params:    map[string]string{"file": paramStr(params, "file"), "strings_min": fmt.Sprintf("%d", stringsMin)},
		Structure: []imageReconSeg{}, // 空集合必须是 [] 而非 null(Go nil 切片 → JSON null 会打崩前端渲染器)
		Embedded:  []imageReconEmbedded{},
	}
	rep.File.Name = paramStr(params, "file")
	rep.File.Size = len(data)
	rep.Artifacts = []string{}

	// —— 格式识别与结构解析 ——
	report(15, "解析图片结构")
	format, mime := imageReconDetectFormat(data)
	rep.File.Format, rep.File.Mime = format, mime
	hostEnd := -1 // 结束标记后第一位(宿主「官方」内容末尾)
	switch format {
	case "JPG":
		rep.Structure, hostEnd = imageReconWalkJPG(data)
	case "PNG":
		rep.Structure, hostEnd = imageReconWalkPNG(data)
	case "GIF":
		hostEnd = imageReconWalkGIF(data)
	case "BMP":
		hostEnd = imageReconBMPDataEnd(data)
	case "WEBP":
		if len(data) >= 8 {
			riffSize := int(binary.LittleEndian.Uint32(data[4:8]))
			hostEnd = 8 + riffSize
		}
	}

	writeArtifact := func(name string, content []byte) (string, bool) {
		if tc.ArtifactsDir == "" {
			return "", false
		}
		if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err != nil {
			return "", false
		}
		if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, name), content, 0o644); err != nil {
			return "", false
		}
		rep.Artifacts = append(rep.Artifacts, name)
		return name, true
	}

	// —— 尾部附加数据 ——
	report(45, "检查结束标记后附加数据")
	if hostEnd >= 0 && hostEnd < len(data) {
		tr := data[hostEnd:]
		typ, ext, payload := imageReconClassify(tr)
		if ext == "zip" {
			payload = imageReconZipDelimit(payload)
		}
		payload = imageReconCap(payload)
		art, _ := writeArtifact("trailing."+map[bool]string{true: ext, false: "bin"}[ext != ""], payload)
		rep.Trailing = &imageReconTrailing{
			Offset: hostEnd, Size: len(tr), Type: typ, Ext: ext,
			PreviewHex:  imageReconHexPreview(tr, 128),
			PreviewText: imageReconTextPreview(tr, 256),
			Artifact:    art,
		}
	}

	// —— 全文件魔数扫描 ——
	report(65, "全文件魔数扫描")
	rep.Embedded = imageReconScanMagics(data, format, writeArtifact)

	// —— strings / flag ——
	report(80, "strings 与 flag 扫描")
	strs := stego.PrintableStrings(data, stringsMin, 200) // 第 3 参为条数上限;每条再截 64 字符
	previews := make([]string, 0, len(strs))
	for _, s := range strs {
		if len(s) > 64 {
			s = s[:64]
		}
		previews = append(previews, s)
	}
	flags := stego.Flags(data)
	if flags == nil {
		flags = []string{}
	}
	rep.Strings = &imageReconStrings{
		Count: len(strs), Previews: previews,
		Flags: flags,
	}

	// —— summary ——
	var sb strings.Builder
	fmt.Fprintf(&sb, "格式 %s(%d 字节)", format, len(data))
	if format == "未知" {
		sb.WriteString(";未识别为常见图片格式,已按通用二进制扫描")
	}
	if rep.Trailing != nil {
		fmt.Fprintf(&sb, "\n结束标记 @%d 后有 %d 字节附加数据,识别为「%s」,已导出 %s",
			rep.Trailing.Offset, rep.Trailing.Size, rep.Trailing.Type, rep.Trailing.Artifact)
	} else if hostEnd >= 0 {
		sb.WriteString("\n结束标记后无附加数据")
	} else {
		sb.WriteString("\n未定位到结束标记(结构异常或非标准图片)")
	}
	if n := len(rep.Embedded); n > 0 {
		fmt.Fprintf(&sb, "\n魔数扫描命中 %d 处嵌入内容(详见命中表)", n)
	}
	if len(rep.Strings.Flags) > 0 {
		fmt.Fprintf(&sb, "\nflag 正则命中 %d 条", len(rep.Strings.Flags))
	}
	if len(strs) > 0 {
		fmt.Fprintf(&sb, "\n可打印字符串 %d 条(≥%d 字节)", len(strs), stringsMin)
	}
	sb.WriteString("\n提示:尾部/内嵌的可疑文本可复制到「移位变换」等工具继续还原")
	rep.Summary = sb.String()

	out, err := jsonMarshal(rep)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	report(100, "完成")
	return &core.ToolResult{
		Status: core.StatusSuccess,
		Stdout: out,
		Data:   map[string]any{"embedded": len(rep.Embedded), "flags": rep.Strings.Flags},
	}, nil
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// ---------- 格式识别 ----------

func imageReconDetectFormat(data []byte) (string, string) {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "JPG", "image/jpeg"
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}):
		return "PNG", "image/png"
	case len(data) >= 6 && (bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))):
		return "GIF", "image/gif"
	case len(data) >= 14 && data[0] == 'B' && data[1] == 'M':
		return "BMP", "image/bmp"
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "WEBP", "image/webp"
	default:
		return "未知", "application/octet-stream"
	}
}

// ---------- JPG 段 walk(SOS 后正向扫描 FFD9 定宿主 EOI) ----------

var jpgMarkerNames = map[byte]string{
	0xE0: "APP0", 0xE1: "APP1", 0xE2: "APP2", 0xEC: "APP12", 0xED: "APP13", 0xEE: "APP14", 0xEF: "APP15",
	0xDB: "DQT", 0xC0: "SOF0", 0xC1: "SOF1", 0xC2: "SOF2", 0xC4: "DHT",
	0xDA: "SOS", 0xC8: "JPG", 0xCC: "DAC", 0xFE: "COM",
}

func imageReconWalkJPG(data []byte) ([]imageReconSeg, int) {
	var segs []imageReconSeg
	segs = append(segs, imageReconSeg{Kind: "jpg-seg", Name: "SOI", Offset: 0, Length: 2})
	i := 2
	for i+3 < len(data) {
		if data[i] != 0xFF {
			segs = append(segs, imageReconSeg{Kind: "jpg-seg", Name: "??", Offset: i, Note: fmt.Sprintf("非标记字节 0x%02X,结构异常", data[i])})
			i = len(data)
			break
		}
		m := data[i+1]
		if m == 0xD9 { // 宿主 EOI:首个(正向扫描;熵编码区字节填充保证不伪现)
			segs = append(segs, imageReconSeg{Kind: "jpg-seg", Name: "EOI", Offset: i, Length: 2})
			return segs, i + 2
		}
		if m == 0x01 || (m >= 0xD0 && m <= 0xD7) { // 无长度填充标记
			i += 2
			continue
		}
		name := jpgMarkerNames[m]
		if name == "" {
			name = fmt.Sprintf("FF%02X", m)
		}
		if i+4 > len(data) {
			break
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		note := ""
		if segLen < 2 || i+2+segLen > len(data) {
			note = fmt.Sprintf("段长度非法(%d),结构异常", segLen)
			segs = append(segs, imageReconSeg{Kind: "jpg-seg", Name: name, Offset: i, Length: len(data) - i, Note: note})
			return segs, -1
		}
		segs = append(segs, imageReconSeg{Kind: "jpg-seg", Name: name, Offset: i, Length: 2 + segLen, Note: note})
		if m == 0xDA { // SOS:其后进入熵编码区,正向找首个 FFD9
			scanStart := i + 2 + segLen
			idx := bytes.Index(data[scanStart:], []byte{0xFF, 0xD9})
			if idx < 0 {
				return segs, -1
			}
			eoi := scanStart + idx
			segs = append(segs, imageReconSeg{Kind: "jpg-seg", Name: "EOI", Offset: eoi, Length: 2})
			return segs, eoi + 2
		}
		i += 2 + segLen
	}
	return segs, -1
}

// ---------- PNG chunk walk ----------

func imageReconWalkPNG(data []byte) ([]imageReconSeg, int) {
	var segs []imageReconSeg
	i := 8
	for i+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[i : i+4]))
		typ := string(data[i+4 : i+8])
		note := ""
		crcOkPtr := (*bool)(nil)
		if length < 0 || i+12+length > len(data) {
			segs = append(segs, imageReconSeg{Kind: "png-chunk", Name: typ, Offset: i,
				Length: len(data) - i, Note: "chunk 越界,结构异常"})
			return segs, -1
		}
		if typ == "IHDR" && length >= 13 { // 尺寸信息附注
			w := int(binary.BigEndian.Uint32(data[i+8 : i+12]))
			h := int(binary.BigEndian.Uint32(data[i+12 : i+16]))
			note = fmt.Sprintf("%dx%d", w, h)
		}
		if typ != "IEND" { // CRC 记录不判死(CRC 坏本身可能是考点)
			got := crc32Sum(data[i+4 : i+8+length])
			want := binary.BigEndian.Uint32(data[i+8+length : i+12+length])
			ok := got == want
			crcOkPtr = &ok
			if !ok {
				if note != "" {
					note += ";"
				}
				note += "CRC 不符"
			}
		}
		segs = append(segs, imageReconSeg{Kind: "png-chunk", Name: typ, Offset: i,
			Length: 12 + length, CRCOk: crcOkPtr, Note: note})
		i += 12 + length
		if typ == "IEND" {
			return segs, i
		}
	}
	return segs, -1
}

// imageReconWalkGIF GIF 块遍历定位 trailer(0x3B);rfind 会被尾部随机 0x3B 截偏(终审裁定改块遍历)。
func imageReconWalkGIF(data []byte) int {
	if len(data) < 14 {
		return -1
	}
	packed := data[10]
	i := 13
	if packed&0x80 != 0 { // 全局色表
		i += 3 << ((packed & 7) + 1)
	}
	for i < len(data) {
		switch data[i] {
		case 0x3B: // trailer
			return i + 1
		case 0x21: // 扩展:intro + label + 子块链
			i += 2
			i = gifSkipSubBlocks(data, i)
		case 0x2C: // 图像描述符:9 字节头(+可选局部色表)+ 数据子块链
			if i+10 > len(data) {
				return -1
			}
			lp := data[i+9]
			j := i + 10
			if lp&0x80 != 0 {
				j += 3 << ((lp & 7) + 1)
			}
			i = gifSkipSubBlocks(data, j)
		default:
			return -1
		}
		if i < 0 {
			return -1
		}
	}
	return -1
}

func gifSkipSubBlocks(data []byte, i int) int {
	for i < len(data) {
		n := int(data[i])
		i++
		if n == 0 {
			return i
		}
		i += n
	}
	return -1
}

// ---------- BMP / 魔数扫描 ----------

func imageReconBMPDataEnd(data []byte) int {
	if len(data) < 30 { // bpp 在 28-30,需完整头(切片可越 len 读 cap 松弛区的 UB 已实测)
		return -1
	}
	offBits := int(binary.LittleEndian.Uint32(data[10:14]))
	w := int(int32(binary.LittleEndian.Uint32(data[18:22])))
	h := int(int32(binary.LittleEndian.Uint32(data[22:26])))
	bpp := int(binary.LittleEndian.Uint16(data[28:30]))
	if w <= 0 || h == 0 {
		return -1
	}
	absH := h
	if absH < 0 {
		absH = -absH
	}
	rowSize := ((bpp*w + 31) / 32) * 4
	return offBits + rowSize*absH
}

// imageReconScanMagics 全文件魔数扫描。
// 政策:偏移 0 的宿主自身格式跳过;stego.Magics 中 Anywhere 且前缀 ≥3 字节全置信;
// 本地补充 GZIP 3 字节前缀(1F 8B 08);2 字节模式(BM/MZ)远离偏移 0 仅标疑似不导出;
// ZIP 命中按 EOCD(PK0506 + 22 + 注释长)定界,失败回退到文件尾。
func imageReconScanMagics(data []byte, hostFormat string, writeArtifact func(name string, content []byte) (string, bool)) []imageReconEmbedded {
	out := []imageReconEmbedded{} // 空集合非 nil(JSON [] 契约)
	seen := map[int]bool{}        // 去重(同偏移多模式只报首个)

	pushHit := func(name string, off int, content []byte, suspected bool) {
		if seen[off] {
			return
		}
		seen[off] = true
		e := imageReconEmbedded{Name: name, Offset: off, Size: len(content), Suspected: suspected}
		if !suspected {
			ext := ""
			switch name {
			case "ZIP", "ZIP(空)":
				ext = "zip"
			case "GZIP", "GZIP3":
				ext = "gz"
			case "RAR":
				ext = "rar"
			case "7-Zip":
				ext = "7z"
			case "PDF":
				ext = "pdf"
			case "JPEG":
				ext = "jpg"
			case "PNG":
				ext = "png"
			case "GIF":
				ext = "gif"
			case "SQLite":
				ext = "db"
			case "ELF":
				ext = "elf"
			}
			if ext != "" {
				safe := strings.Map(func(r rune) rune {
					if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
						return r
					}
					return '_'
				}, name)
				art, _ := writeArtifact(fmt.Sprintf("embedded_%s_%d.%s", safe, off, ext), content)
				e.Artifact = art
			}
		}
		out = append(out, e)
	}

	// 全置信模式:引擎表 Anywhere 且 ≥3 字节
	for _, mg := range stego.Magics {
		if !mg.Anywhere || len(mg.Prefix) < 3 || mg.Name == "ZIP(空)" {
			continue
		}
		start := 0
		for {
			idx := bytes.Index(data[start:], mg.Prefix)
			if idx < 0 {
				break
			}
			off := start + idx
			start = off + 1
			if off == 0 && (hostFormat == mg.Name || (hostFormat == "JPG" && mg.Name == "JPEG")) { // @0 宿主自身(格式名与引擎魔数名归一)
				continue
			}
			if mg.Name == "ZIP" {
				pushHit(mg.Name, off, imageReconCap(imageReconZipDelimit(data[off:])), false)
				continue
			}
			content := data[off:]
			if cut := stego.Cut(content, 0, mg.Name); len(cut) > 0 && len(cut) <= len(content) {
				content = cut
			}
			pushHit(mg.Name, off, imageReconCap(content), false)
		}
	}
	// 本地补充:GZIP 3 字节(1F 8B 08)
	start := 0
	for {
		idx := bytes.Index(data[start:], []byte{0x1F, 0x8B, 0x08})
		if idx < 0 {
			break
		}
		off := start + idx
		start = off + 1
		if off == 0 && hostFormat == "GZIP" {
			continue
		}
		pushHit("GZIP3", off, imageReconCap(data[off:]), false)
	}
	// 疑似(2 字节,远离偏移 0):仅报告不导出
	for _, two := range []struct {
		name   string
		prefix []byte
	}{{"BMP?", []byte("BM")}, {"EXE?", []byte("MZ")}} {
		start := 1 // 偏移 0 交给格式识别
		for {
			idx := bytes.Index(data[start:], two.prefix)
			if idx < 0 {
				break
			}
			off := start + idx
			start = off + 1
			pushHit(two.name, off, nil, true)
		}
	}
	return out
}

// ---------- 分类与预览 ----------

// imageReconClassify 尾部数据类型:魔数命中(≥3B)→ 引擎命名;短流文本启发(引擎门槛
// ≥32B,gugugu 的 24B 场景判 BIN)→ 文本;否则 bin。
func imageReconClassify(bs []byte) (string, string, []byte) {
	if name, ext, _, off := stego.Detect(bs); name != "" && name != "BIN" {
		payload := bs
		if off > 0 && off <= len(bs) {
			payload = bs[off:] // 尊重 Detect 偏移,剥掉嵌入文件前的垃圾前缀
		}
		if cut := stego.Cut(payload, 0, name); len(cut) > 0 && len(cut) <= len(payload) {
			payload = cut
		}
		return name, ext, payload
	}
	if imageReconIsMostlyPrintable(bs) {
		return "文本", "txt", bs
	}
	return "二进制", "bin", bs
}

// imageReconZipDelimit ZIP EOCD 定界(魔数扫描与尾部共用):末个 PK\x05\x06 + 22 + 注释长。
func imageReconZipDelimit(buf []byte) []byte {
	eocd := bytes.LastIndex(buf, []byte("PK\x05\x06"))
	if eocd < 0 || eocd+22 > len(buf) {
		return buf
	}
	commentLen := int(binary.LittleEndian.Uint16(buf[eocd+20 : eocd+22]))
	end := eocd + 22 + commentLen
	if end > len(buf) {
		end = len(buf)
	}
	return buf[:end]
}

// imageReconCap 产物封顶 4MB(与 stego 引擎 MaxSave 同量级,防大文件整段导出)。
func imageReconCap(bs []byte) []byte {
	const maxSave = 4 << 20
	if len(bs) > maxSave {
		return bs[:maxSave]
	}
	return bs
}

func imageReconIsMostlyPrintable(bs []byte) bool {
	if len(bs) < 4 {
		return false
	}
	printable := 0
	for _, b := range bs {
		if stego.IsPrintableByte(b) {
			printable++
		}
	}
	return printable*100/len(bs) >= 80
}

func imageReconHexPreview(bs []byte, max int) string {
	n := len(bs)
	if n > max {
		n = max
	}
	return hex.EncodeToString(bs[:n])
}

func imageReconTextPreview(bs []byte, max int) string {
	var sb strings.Builder
	n := len(bs)
	if n > max {
		n = max
	}
	for _, b := range bs[:n] {
		if stego.IsPrintableByte(b) {
			sb.WriteByte(b)
		} else {
			sb.WriteByte('.')
		}
	}
	if len(bs) > max {
		sb.WriteString("…")
	}
	return sb.String()
}

// crc32Sum PNG chunk CRC(IEEE)。
func crc32Sum(bs []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, b := range bs {
		crc ^= uint32(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = crc>>1 ^ 0xEDB88320
			} else {
				crc >>= 1
			}
		}
	}
	return ^crc
}

// jsonMarshal 包装(与 audio_lsb 同款输出形态)。
func jsonMarshal(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
