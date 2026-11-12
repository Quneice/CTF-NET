// data.extract.image-metadata —— 图片元数据提取（图片属性详细信息）。
//
// 纯标准库实现，无第三方 EXIF 依赖：
//   - 基本属性：格式识别（JPEG/PNG/WebP/GIF/BMP/TIFF/ICO）、尺寸、颜色信息、MD5；
//   - EXIF：JPEG APP1 / PNG eXIf / WebP EXIF 块中的 TIFF 数据，自研 IFD 解析器，
//     覆盖主 IFD（相机/软件/分辨率/方向）、Exif 子 IFD（曝光/光圈/ISO/焦距等）、
//     GPS 子 IFD（经纬度有理数 → 十进制 + Google 地图定位）；
//   - XMP：JPEG APP1 / PNG iTXt / WebP XMP 块的原始 XML；
//   - PNG 文本块：tEXt / zTXt（zlib 解压）/ iTXt。
package data

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&ImageMetaTool{BaseTool: core.BaseTool{
		NameAttr:        "image-metadata",
		CategoryAttr:    core.Category("data.extract"),
		VersionAttr:     "1.0.0",
		DescriptionAttr: "提取图片属性详细信息：格式/尺寸、EXIF 相机参数、GPS 坐标、XMP、PNG 文本块",
		TimeoutAttr:     120,
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "图片文件（JPEG/PNG/WebP/GIF/BMP/TIFF/ICO）"},
		},
		Runner: imageMetaRun,
	}})
}

// ImageMetaTool 图片元数据提取工具。
type ImageMetaTool struct {
	core.BaseTool
}

// ---------------------------------------------------------------------------
// 结果模型
// ---------------------------------------------------------------------------

type kv struct{ k, v string }

type gpsOut struct {
	latDec, lonDec float64
	latRef, lonRef string
	altitude       string
	dateStamp      string
}

type imageReport struct {
	fileName string
	size     int64
	md5      string
	format   string
	width    int
	height   int
	color    string
	main     []kv // IFD0
	exif     []kv // Exif 子 IFD
	gps      *gpsOut
	xmp      string
	pngTexts []kv
	notes    []string
}

// ---------------------------------------------------------------------------
// 格式识别与基本属性
// ---------------------------------------------------------------------------

func sniffImageFormat(b []byte) (format string, width, height int, color string) {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "JPEG", 0, 0, ""
	case len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "PNG", 0, 0, ""
	case len(b) >= 6 && (bytes.Equal(b[:6], []byte("GIF87a")) || bytes.Equal(b[:6], []byte("GIF89a"))):
		return "GIF", int(binary.LittleEndian.Uint16(b[6:8])), int(binary.LittleEndian.Uint16(b[8:10])), "索引色"
	case len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return "WebP", 0, 0, ""
	case len(b) >= 2 && bytes.Equal(b[:2], []byte("BM")):
		h := int32(binary.LittleEndian.Uint32(b[22:26]))
		if h < 0 {
			h = -h
		}
		return "BMP", int(int32(binary.LittleEndian.Uint32(b[18:22]))), int(h),
			fmt.Sprintf("%d bit", binary.LittleEndian.Uint16(b[28:30]))
	case len(b) >= 4 && ((bytes.Equal(b[:2], []byte("II")) || bytes.Equal(b[:2], []byte("MM"))) && binary.LittleEndian.Uint16(b[2:4]) == 42):
		return "TIFF", 0, 0, ""
	case len(b) >= 6 && b[0] == 0 && b[1] == 0 && b[2] == 1 && b[3] == 0:
		w, h := int(b[6]), int(b[7])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		return "ICO", w, h, ""
	}
	return "未知", 0, 0, ""
}

// walkJPEGSegments 遍历 JPEG 段（到 SOS/EOI 为止）。marker 为段标记，payload 为段内容。
func walkJPEGSegments(b []byte, fn func(marker byte, payload []byte)) {
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			i++
			continue
		}
		marker := b[i+1]
		if marker == 0xFF { // 填充字节
			i++
			continue
		}
		if marker == 0xD9 || marker == 0xDA { // EOI / SOS（后面是熵编码数据）
			break
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) { // 独立标记
			i += 2
			continue
		}
		l := int(binary.BigEndian.Uint16(b[i+2 : i+4]))
		if l < 2 || i+2+l > len(b) {
			break
		}
		fn(marker, b[i+4:i+2+l])
		i += 2 + l
	}
}

// walkPNGChunks 遍历 PNG 块（按长度字段，不校验 CRC）。
func walkPNGChunks(b []byte, fn func(typ string, payload []byte)) {
	i := 8
	for i+12 <= len(b) {
		l := int(binary.BigEndian.Uint32(b[i : i+4]))
		typ := string(b[i+4 : i+8])
		if i+12+l > len(b) {
			break
		}
		fn(typ, b[i+8:i+8+l])
		i += 12 + l
	}
}

// walkWebPChunks 遍历 WebP（RIFF）块。
func walkWebPChunks(b []byte, fn func(fourcc string, payload []byte)) {
	i := 12
	for i+8 <= len(b) {
		fourcc := string(b[i : i+4])
		size := int(binary.LittleEndian.Uint32(b[i+4 : i+8]))
		if i+8+size > len(b) {
			break
		}
		fn(fourcc, b[i+8:i+8+size])
		i += 8 + size + size&1 // 奇数长度补齐
	}
}

// pngColorType 颜色类型 → 人类可读描述。
func pngColorType(t byte) string {
	switch t {
	case 0:
		return "灰度"
	case 2:
		return "RGB"
	case 3:
		return "索引色"
	case 4:
		return "灰度 + Alpha"
	case 6:
		return "RGBA"
	}
	return fmt.Sprintf("颜色类型 %d", t)
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// ---------------------------------------------------------------------------
// TIFF / EXIF 解析
// ---------------------------------------------------------------------------

type tiffData struct {
	b     []byte
	order binary.ByteOrder
}

func parseTIFF(b []byte) (*tiffData, error) {
	if len(b) < 8 {
		return nil, fmt.Errorf("TIFF 数据过短")
	}
	t := &tiffData{b: b}
	switch string(b[0:2]) {
	case "II":
		t.order = binary.LittleEndian
	case "MM":
		t.order = binary.BigEndian
	default:
		return nil, fmt.Errorf("非法 TIFF 字节序标记")
	}
	if t.u16(2) != 42 {
		return nil, fmt.Errorf("非法 TIFF 魔数")
	}
	return t, nil
}

func (t *tiffData) u16(off int) uint16 { return t.order.Uint16(t.b[off : off+2]) }
func (t *tiffData) u32(off int) uint32 { return t.order.Uint32(t.b[off : off+4]) }

type tiffEntry struct {
	tag   uint16
	typ   uint16
	count uint32
	value []byte // 原始字节（内联值或已按偏移读出）
}

func typeSize(typ uint16) int {
	switch typ {
	case 1, 2, 6, 7: // BYTE / ASCII / SBYTE / UNDEFINED
		return 1
	case 3, 8: // SHORT / SSHORT
		return 2
	case 4, 9, 11: // LONG / SLONG / FLOAT
		return 4
	case 5, 10, 12: // RATIONAL / SRATIONAL / DOUBLE
		return 8
	}
	return 0
}

func (t *tiffData) readIFD(offset uint32) ([]tiffEntry, error) {
	if int(offset)+2 > len(t.b) {
		return nil, fmt.Errorf("IFD 偏移越界")
	}
	n := int(t.u16(int(offset)))
	if n > 1024 {
		return nil, fmt.Errorf("IFD 条目数异常: %d", n)
	}
	entries := make([]tiffEntry, 0, n)
	base := int(offset) + 2
	for i := 0; i < n; i++ {
		p := base + i*12
		if p+12 > len(t.b) {
			return nil, fmt.Errorf("IFD 条目越界")
		}
		e := tiffEntry{tag: t.u16(p), typ: t.u16(p + 2), count: t.u32(p + 4)}
		raw := t.b[p+8 : p+12]
		size := typeSize(e.typ) * int(e.count)
		if size <= 4 {
			e.value = append([]byte(nil), raw[:size]...)
		} else {
			off := t.u32(p + 8)
			if int(off)+size > len(t.b) || int(off) < 0 {
				return nil, fmt.Errorf("条目 %#x 数据越界", e.tag)
			}
			e.value = append([]byte(nil), t.b[int(off):int(off)+size]...)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func findEntry(entries []tiffEntry, tag uint16) (tiffEntry, bool) {
	for _, e := range entries {
		if e.tag == tag {
			return e, true
		}
	}
	return tiffEntry{}, false
}

func (t *tiffData) entryU16(e tiffEntry) uint16 {
	if len(e.value) >= 2 {
		return t.order.Uint16(e.value[:2])
	}
	return 0
}

func (t *tiffData) entryU32(e tiffEntry) uint32 {
	if len(e.value) >= 4 {
		return t.order.Uint32(e.value[:4])
	}
	return 0
}

func (t *tiffData) entryASCII(e tiffEntry) string {
	return strings.TrimRight(string(e.value), "\x00 ")
}

// entryRat 取第 idx 个 RATIONAL（n/d）。
func (t *tiffData) entryRat(e tiffEntry, idx int) (uint32, uint32) {
	if (idx+1)*8 > len(e.value) {
		return 0, 0
	}
	p := e.value[idx*8 : idx*8+8]
	return t.order.Uint32(p[:4]), t.order.Uint32(p[4:8])
}

// entrySRat 取第 idx 个 SRATIONAL（有符号）。
func (t *tiffData) entrySRat(e tiffEntry, idx int) (int32, int32) {
	if (idx+1)*8 > len(e.value) {
		return 0, 0
	}
	p := e.value[idx*8 : idx*8+8]
	return int32(t.order.Uint32(p[:4])), int32(t.order.Uint32(p[4:8]))
}

// ratFloat 有理数 → 浮点（d==0 防除零）。
func ratFloat(n, d uint32) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func sratFloat(n, d int32) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

// fmtRational 通用有理数显示：整数 → 整数；1/x 秒类交给调用方。
func fmtDecimal(f float64, digits int) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%."+fmt.Sprint(digits)+"f", f), "0"), ".")
}

// fmtExposure 曝光时间：1/125 秒 / 1/2 秒 / 0.3 秒。
func fmtExposure(n, d uint32) string {
	if d == 0 {
		return "-"
	}
	if n == 1 {
		return fmt.Sprintf("1/%d 秒", d)
	}
	return fmt.Sprintf("%s 秒", fmtDecimal(ratFloat(n, d), 3))
}

func fmtFNumber(n, d uint32) string {
	return "f/" + fmtDecimal(ratFloat(n, d), 2)
}

var orientationNames = map[uint16]string{
	1: "正常",
	2: "水平翻转",
	3: "旋转 180°",
	4: "垂直翻转",
	5: "水平翻转 + 逆时针旋转 90°",
	6: "顺时针旋转 90°",
	7: "水平翻转 + 顺时针旋转 90°",
	8: "逆时针旋转 90°",
}

var resolutionUnits = map[uint16]string{1: "无单位", 2: "英寸", 3: "厘米"}

var exposurePrograms = map[uint16]string{
	0: "未定义", 1: "手动", 2: "程序自动", 3: "光圈优先", 4: "快门优先",
	5: "创意模式", 6: "运动模式", 7: "人像模式", 8: "风景模式",
}

var meteringModes = map[uint16]string{
	0: "未知", 1: "平均测光", 2: "中央重点测光", 3: "点测光", 4: "多点测光",
	5: "矩阵测光", 6: "局部测光", 255: "其他",
}

var lightSources = map[uint16]string{
	0: "未知", 1: "日光", 2: "荧光灯", 3: "钨丝灯", 4: "闪光灯",
	9: "晴天", 10: "阴天", 11: "阴影", 12: "日光荧光灯", 13: "白荧光灯",
	14: "冷白荧光灯", 15: "暖白荧光灯", 17: "标准光源 A", 18: "标准光源 B",
	19: "标准光源 C", 20: "D55", 21: "D65", 22: "D75", 23: "D50",
	24: "ISO 演播室钨丝灯", 255: "其他",
}

var sensingMethods = map[uint16]string{
	1: "未定义", 2: "单片彩色传感器", 3: "双片彩色传感器", 4: "三片彩色传感器",
	5: "色彩序列传感器", 7: "三线型传感器", 8: "逐行彩色传感器",
}

var sceneCaptureTypes = map[uint16]string{0: "标准", 1: "风景", 2: "人像", 3: "夜景"}

var gainControls = map[uint16]string{
	0: "无", 1: "低增益上升", 2: "高增益上升", 3: "低增益下降", 4: "高增益下降",
}

var softnessNames = map[uint16]string{0: "普通", 1: "柔和", 2: "强烈"}

// ifd0Order 主 IFD 渲染顺序（含 TIFF 尺寸标签）。
var ifd0Order = []struct {
	tag  uint16
	name string
}{
	{0x010F, "相机制造商"}, {0x0110, "相机型号"}, {0x0131, "软件"}, {0x0132, "修改时间"},
	{0x013B, "作者"}, {0x8298, "版权"}, {0x0112, "方向"}, {0x011A, "水平分辨率"},
	{0x011B, "垂直分辨率"}, {0x0128, "分辨率单位"}, {0x0100, "图像宽度"}, {0x0101, "图像高度"},
	{0xA002, "图像宽度"}, {0xA003, "图像高度"}, {0x0213, "YCbCr 定位"}, {0x010E, "图像描述"},
}

var exifOrder = []struct {
	tag  uint16
	name string
}{
	{0x9003, "拍摄时间"}, {0x9011, "拍摄时区偏移"}, {0x9291, "亚秒时间（原始）"},
	{0x829A, "曝光时间"}, {0x829D, "光圈值"}, {0x9201, "快门速度（APEX）"}, {0x9202, "光圈（APEX）"},
	{0x9205, "最大光圈"}, {0x9204, "曝光补偿"}, {0x8827, "ISO 感光度"},
	{0x920A, "焦距"}, {0xA405, "35mm 等效焦距"}, {0xA432, "镜头规格"}, {0xA433, "镜头型号"},
	{0xA435, "镜头制造商"}, {0x9207, "测光模式"}, {0x9208, "光源"}, {0x9209, "闪光灯"},
	{0xA403, "白平衡"}, {0xA402, "曝光模式"}, {0x8822, "曝光程序"}, {0xA406, "场景拍摄类型"},
	{0xA407, "增益控制"}, {0xA408, "对比度"}, {0xA409, "饱和度"}, {0xA40A, "锐度"},
	{0xA401, "自定义渲染"}, {0xA001, "色彩空间"}, {0xA217, "感光方式"}, {0xA404, "数码变焦比"},
	{0xA300, "文件来源"}, {0xA301, "场景类型"}, {0xA20E, "焦平面水平分辨率"}, {0xA20F, "焦平面垂直分辨率"},
	{0x9000, "Exif 版本"}, {0x9286, "用户注释"}, {0x927C, "厂商备注"}, {0xA420, "图像唯一 ID"},
	{0xA431, "机身序列号"}, {0xA430, "相机机主"}, {0x9004, "数字化时间"}, {0x9290, "亚秒时间"},
}

// renderIFD0 渲染主 IFD。
func renderIFD0(t *tiffData, entries []tiffEntry) []kv {
	var out []kv
	byTag := map[uint16]tiffEntry{}
	for _, e := range entries {
		byTag[e.tag] = e
	}
	for _, spec := range ifd0Order {
		e, ok := byTag[spec.tag]
		if !ok {
			continue
		}
		var v string
		switch spec.tag {
		case 0x0112:
			v = orientationNames[t.entryU16(e)]
		case 0x0128:
			v = resolutionUnits[t.entryU16(e)]
		case 0x011A, 0x011B:
			n, d := t.entryRat(e, 0)
			v = fmtDecimal(ratFloat(n, d), 2)
		case 0x0100, 0x0101, 0xA002, 0xA003:
			if e.typ == 3 {
				v = fmt.Sprint(t.entryU16(e))
			} else {
				v = fmt.Sprint(t.entryU32(e))
			}
		default:
			v = t.entryASCII(e)
		}
		if v == "" {
			continue
		}
		out = append(out, kv{k: spec.name, v: v})
	}
	return out
}

// renderExifIFD 渲染 Exif 子 IFD。
func renderExifIFD(t *tiffData, entries []tiffEntry) []kv {
	var out []kv
	byTag := map[uint16]tiffEntry{}
	for _, e := range entries {
		byTag[e.tag] = e
	}
	for _, spec := range exifOrder {
		e, ok := byTag[spec.tag]
		if !ok {
			continue
		}
		var v string
		switch spec.tag {
		case 0x829A: // 曝光时间
			n, d := t.entryRat(e, 0)
			v = fmtExposure(n, d)
		case 0x829D, 0x9205: // 光圈值 / 最大光圈
			n, d := t.entryRat(e, 0)
			v = fmtFNumber(n, d)
		case 0x9201: // 快门 APEX
			n, d := t.entryRat(e, 0)
			apex := ratFloat(n, d)
			if apex == 0 {
				v = "0 秒"
			} else {
				v = fmt.Sprintf("1/%s 秒", fmtDecimal(math.Exp2(apex), 3))
			}
		case 0x9202: // 光圈 APEX
			n, d := t.entryRat(e, 0)
			v = "f/" + fmtDecimal(math.Exp2(ratFloat(n, d)/2), 2)
		case 0x9204: // 曝光补偿（SRATIONAL）
			sn, sd := t.entrySRat(e, 0)
			v = fmtDecimal(sratFloat(sn, sd), 2) + " EV"
		case 0x8827, 0xA404: // ISO / 数码变焦比
			if e.typ == 3 {
				v = fmt.Sprint(t.entryU16(e))
			} else {
				n, d := t.entryRat(e, 0)
				v = fmtDecimal(ratFloat(n, d), 2)
			}
		case 0x920A, 0xA405: // 焦距 / 35mm 等效
			n, d := t.entryRat(e, 0)
			v = fmtDecimal(ratFloat(n, d), 1) + " mm"
		case 0xA432: // 镜头规格：4 个有理数（最小/最大焦距、最小/最大光圈）
			parts := make([]string, 0, 4)
			for i := 0; i < 4 && (i+1)*8 <= len(e.value); i++ {
				n, d := t.entryRat(e, i)
				parts = append(parts, fmtDecimal(ratFloat(n, d), 2))
			}
			v = strings.Join(parts, " / ")
		case 0x9207:
			v = meteringModes[t.entryU16(e)]
		case 0x9208:
			v = lightSources[t.entryU16(e)]
		case 0x9209: // 闪光灯
			f := t.entryU16(e)
			v = "未闪光"
			if f&0x01 != 0 {
				v = "已闪光"
			}
			if f&0x20 != 0 {
				v += "（红眼模式）"
			}
		case 0xA403:
			if t.entryU16(e) == 0 {
				v = "自动"
			} else {
				v = "手动"
			}
		case 0xA402:
			v = map[uint16]string{0: "自动", 1: "手动", 2: "自动包围"}[t.entryU16(e)]
		case 0x8822:
			v = exposurePrograms[t.entryU16(e)]
		case 0xA406:
			v = sceneCaptureTypes[t.entryU16(e)]
		case 0xA407:
			v = gainControls[t.entryU16(e)]
		case 0xA408, 0xA409, 0xA40A:
			v = softnessNames[t.entryU16(e)]
		case 0xA401:
			v = map[uint16]string{0: "正常", 1: "自定义"}[t.entryU16(e)]
		case 0xA001:
			if t.entryU16(e) == 1 {
				v = "sRGB"
			} else if t.entryU16(e) == 0xFFFF {
				v = "未校准"
			}
		case 0xA217:
			v = sensingMethods[t.entryU16(e)]
		case 0xA300:
			v = map[uint16]string{1: "胶片扫描仪", 2: "反光式扫描仪", 3: "数码相机"}[t.entryU16(e)]
		case 0xA301:
			v = map[uint16]string{1: "直接拍摄"}[t.entryU16(e)]
		case 0xA20E, 0xA20F:
			n, d := t.entryRat(e, 0)
			v = fmtDecimal(ratFloat(n, d), 2)
		case 0x9000: // Exif 版本："0232" → 2.32
			v = strings.Join(strings.Split(strings.TrimRight(string(e.value), "\x00"), ""), ".")
		case 0x9286: // 用户注释（前 8 字节为字符集标记）
			s := string(e.value)
			if len(s) > 8 && (strings.HasPrefix(s, "ASCII") || strings.HasPrefix(s, "UNICODE")) {
				s = s[8:]
			}
			v = strings.TrimRight(s, "\x00 ")
		case 0x927C: // 厂商备注：只报字节数
			v = fmt.Sprintf("%d 字节（未解析）", len(e.value))
		default:
			v = t.entryASCII(e)
		}
		if v == "" {
			continue
		}
		out = append(out, kv{k: spec.name, v: v})
	}
	return out
}

// renderGPSIFD 渲染 GPS 子 IFD，返回结构化的 GPS 输出。
func renderGPSIFD(t *tiffData, entries []tiffEntry) *gpsOut {
	byTag := map[uint16]tiffEntry{}
	for _, e := range entries {
		byTag[e.tag] = e
	}
	out := &gpsOut{}
	if e, ok := byTag[0x0001]; ok { // 纬度方向
		out.latRef = strings.TrimRight(string(e.value), "\x00 ")
	}
	if e, ok := byTag[0x0003]; ok { // 经度方向
		out.lonRef = strings.TrimRight(string(e.value), "\x00 ")
	}
	if e, ok := byTag[0x0002]; ok && len(e.value) >= 24 { // 纬度
		d1, d2 := t.entryRat(e, 0)
		m1, m2 := t.entryRat(e, 1)
		s1, s2 := t.entryRat(e, 2)
		out.latDec = ratFloat(d1, d2) + ratFloat(m1, m2)/60 + ratFloat(s1, s2)/3600
		if out.latRef == "S" {
			out.latDec = -out.latDec
		}
	}
	if e, ok := byTag[0x0004]; ok && len(e.value) >= 24 { // 经度
		d1, d2 := t.entryRat(e, 0)
		m1, m2 := t.entryRat(e, 1)
		s1, s2 := t.entryRat(e, 2)
		out.lonDec = ratFloat(d1, d2) + ratFloat(m1, m2)/60 + ratFloat(s1, s2)/3600
		if out.lonRef == "W" {
			out.lonDec = -out.lonDec
		}
	}
	if e, ok := byTag[0x0006]; ok { // 高度
		n, d := t.entryRat(e, 0)
		out.altitude = fmtDecimal(ratFloat(n, d), 2) + " 米"
	}
	if e, ok := byTag[0x001D]; ok { // 拍摄日期
		out.dateStamp = strings.TrimRight(string(e.value), "\x00 ")
	}
	if out.latDec == 0 && out.lonDec == 0 {
		return nil
	}
	return out
}

// extractEXIF 从 TIFF 数据提取主 IFD / Exif 子 IFD / GPS 子 IFD，
// 并附带主 IFD 声明的图像宽高（TIFF 0x0100/0x0101，JPEG EXIF 通常无此两标签）。
func extractEXIF(tiffBytes []byte) (main, exif []kv, gps *gpsOut, width, height int) {
	t, err := parseTIFF(tiffBytes)
	if err != nil {
		return nil, nil, nil, 0, 0
	}
	ifd0Offset := t.u32(4)
	ifd0, err := t.readIFD(ifd0Offset)
	if err != nil {
		return nil, nil, nil, 0, 0
	}
	if e, ok := findEntry(ifd0, 0x0100); ok {
		if e.typ == 3 {
			width = int(t.entryU16(e))
		} else {
			width = int(t.entryU32(e))
		}
	}
	if e, ok := findEntry(ifd0, 0x0101); ok {
		if e.typ == 3 {
			height = int(t.entryU16(e))
		} else {
			height = int(t.entryU32(e))
		}
	}
	main = renderIFD0(t, ifd0)
	// 子 IFD 指针既可能出现在主 IFD，也可能出现在 Exif 子 IFD 内
	readSub := func(entries []tiffEntry) {
		if e, ok := findEntry(entries, 0x8769); ok && (e.typ == 4 || e.typ == 3) {
			off := uint32(t.entryU32(e))
			if sub, err := t.readIFD(off); err == nil {
				exif = renderExifIFD(t, sub)
				if ge, ok := findEntry(sub, 0x8825); ok && (ge.typ == 4 || ge.typ == 3) {
					if g, err := t.readIFD(uint32(t.entryU32(ge))); err == nil {
						gps = renderGPSIFD(t, g)
					}
				}
			}
		}
		if e, ok := findEntry(entries, 0x8825); ok && gps == nil && (e.typ == 4 || e.typ == 3) {
			if g, err := t.readIFD(uint32(t.entryU32(e))); err == nil {
				gps = renderGPSIFD(t, g)
			}
		}
	}
	readSub(ifd0)
	return main, exif, gps, width, height
}

// ---------------------------------------------------------------------------
// 各格式元数据提取
// ---------------------------------------------------------------------------

func extractJPEG(b []byte, rep *imageReport) {
	rep.width, rep.height, rep.color = 0, 0, ""
	walkJPEGSegments(b, func(marker byte, payload []byte) {
		switch {
		case marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC:
			// SOFn：尺寸 + 分量
			if len(payload) >= 6 {
				rep.height = int(binary.BigEndian.Uint16(payload[1:3]))
				rep.width = int(binary.BigEndian.Uint16(payload[3:5]))
				rep.color = fmt.Sprintf("%d 分量, %d bit", payload[5], payload[0])
			}
		case marker == 0xE1: // APP1：EXIF / XMP
			if bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
				main, exif, gps, _, _ := extractEXIF(payload[6:])
				rep.main, rep.exif, rep.gps = main, exif, gps
			} else if bytes.HasPrefix(payload, []byte("http://ns.adobe.com/xap/1.0/\x00")) {
				rep.xmp = string(payload[len("http://ns.adobe.com/xap/1.0/"):])
			}
		}
	})
}

func extractPNG(b []byte, rep *imageReport) {
	walkPNGChunks(b, func(typ string, payload []byte) {
		switch typ {
		case "IHDR":
			if len(payload) >= 10 {
				rep.width = int(binary.BigEndian.Uint32(payload[0:4]))
				rep.height = int(binary.BigEndian.Uint32(payload[4:8]))
				rep.color = fmt.Sprintf("%d bit, %s", payload[8], pngColorType(payload[9]))
			}
		case "tEXt":
			if i := bytes.IndexByte(payload, 0); i >= 0 {
				rep.pngTexts = append(rep.pngTexts, kv{k: string(payload[:i]), v: string(payload[i+1:])})
			}
		case "zTXt":
			if i := bytes.IndexByte(payload, 0); i >= 0 {
				key := string(payload[:i])
				if i+2 < len(payload) && payload[i+1] == 0 {
					if r, err := zlib.NewReader(bytes.NewReader(payload[i+2:])); err == nil {
						if txt, err := io.ReadAll(io.LimitReader(r, 1<<20)); err == nil {
							rep.pngTexts = append(rep.pngTexts, kv{k: key, v: string(txt)})
						}
					}
				}
			}
		case "iTXt":
			parts := bytes.SplitN(payload, []byte{0}, 5)
			if len(parts) >= 5 {
				key := string(parts[0])
				compFlag := len(parts) >= 2 && len(parts[1]) > 0 && parts[1][0] == 1
				txt := parts[4]
				if compFlag {
					if r, err := zlib.NewReader(bytes.NewReader(txt)); err == nil {
						if d, err := io.ReadAll(io.LimitReader(r, 1<<20)); err == nil {
							txt = d
						}
					}
				}
				if key == "XML:com.adobe.xmp" {
					rep.xmp = string(txt)
				} else {
					rep.pngTexts = append(rep.pngTexts, kv{k: key, v: string(txt)})
				}
			}
		case "eXIf":
			main, exif, gps, _, _ := extractEXIF(payload)
			rep.main, rep.exif, rep.gps = main, exif, gps
		}
	})
}

func extractWebP(b []byte, rep *imageReport) {
	walkWebPChunks(b, func(fourcc string, payload []byte) {
		switch fourcc {
		case "VP8X":
			if len(payload) >= 10 {
				rep.width = 1 + int(payload[4]) + int(payload[5])<<8 + int(payload[6])<<16
				rep.height = 1 + int(payload[7]) + int(payload[8])<<8 + int(payload[9])<<16
			}
		case "VP8L":
			if len(payload) >= 5 {
				rep.width = int(binary.LittleEndian.Uint16(payload[1:3])&0x3FFF) + 1
				rep.height = int(binary.LittleEndian.Uint16(payload[3:5])>>2&0x3FFF) + 1
			}
		case "VP8 ":
			if len(payload) >= 10 && payload[3] == 0x9D && payload[4] == 0x01 && payload[5] == 0x2A {
				rep.width = int(binary.LittleEndian.Uint16(payload[6:8]) & 0x3FFF)
				rep.height = int(binary.LittleEndian.Uint16(payload[8:10]) & 0x3FFF)
			}
		case "EXIF":
			main, exif, gps, _, _ := extractEXIF(payload)
			rep.main, rep.exif, rep.gps = main, exif, gps
		case "XMP ":
			rep.xmp = string(payload)
		}
	})
}

// ---------------------------------------------------------------------------
// 主流程
// ---------------------------------------------------------------------------

// analyzeImageBytes 分析图片字节：格式/尺寸/颜色/EXIF/GPS/XMP/PNG 文本块。
// 单图工具与批量工具共用；未识别的格式返回 format="未知" 的空报告。
func analyzeImageBytes(name string, data []byte) *imageReport {
	rep := &imageReport{
		fileName: name,
		size:     int64(len(data)),
		md5:      fmt.Sprintf("%x", md5.Sum(data)),
	}
	rep.format, rep.width, rep.height, rep.color = sniffImageFormat(data)
	switch rep.format {
	case "JPEG":
		extractJPEG(data, rep)
	case "PNG":
		extractPNG(data, rep)
	case "WebP":
		extractWebP(data, rep)
	case "TIFF":
		main, exif, gps, w, h := extractEXIF(data)
		rep.main, rep.exif, rep.gps = main, exif, gps
		rep.width, rep.height = w, h
	}
	return rep
}

func imageMetaRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	fname := pcStr(params, "file")
	if strings.TrimSpace(fname) == "" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少文件参数"}, nil
	}
	var path string
	if tc != nil && tc.Workdir != "" {
		path = filepath.Join(tc.Workdir, fname)
	} else {
		path = fname
	}
	st, err := os.Stat(path)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("文件不存在: %s", fname)}, nil
	}
	const maxSize = 300 << 20
	if st.Size() > maxSize {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("文件过大（%s，上限 300MB）", humanSize(st.Size()))}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("读取文件失败: %v", err)}, nil
	}
	_ = ctx

	rep := analyzeImageBytes(fname, data)

	// —— 组装输出 ——
	var sb strings.Builder
	sb.WriteString("[文件信息]\n")
	fmt.Fprintf(&sb, "  文件名: %s\n", rep.fileName)
	fmt.Fprintf(&sb, "  大小: %d 字节 (%s)\n", rep.size, humanSize(rep.size))
	fmt.Fprintf(&sb, "  MD5: %s\n", rep.md5)
	fmt.Fprintf(&sb, "  格式: %s\n", rep.format)
	if rep.width > 0 || rep.height > 0 {
		fmt.Fprintf(&sb, "  尺寸: %d x %d (宽 x 高)\n", rep.width, rep.height)
	}
	if rep.color != "" {
		fmt.Fprintf(&sb, "  颜色: %s\n", rep.color)
	}

	section := func(title string, kvs []kv) {
		if len(kvs) == 0 {
			return
		}
		sb.WriteString("\n[" + title + "]\n")
		for _, item := range kvs {
			fmt.Fprintf(&sb, "  %s: %s\n", item.k, item.v)
		}
	}
	section("EXIF 主信息", rep.main)
	section("EXIF 拍摄参数", rep.exif)

	if rep.gps != nil {
		sb.WriteString("\n[GPS 定位]\n")
		ref := map[string]string{"N": "北纬", "S": "南纬"}[rep.gps.latRef]
		if ref == "" {
			ref = rep.gps.latRef
		}
		fmt.Fprintf(&sb, "  纬度: %.6f° (%s)\n", math.Abs(rep.gps.latDec), ref)
		ref = map[string]string{"E": "东经", "W": "西经"}[rep.gps.lonRef]
		if ref == "" {
			ref = rep.gps.lonRef
		}
		fmt.Fprintf(&sb, "  经度: %.6f° (%s)\n", math.Abs(rep.gps.lonDec), ref)
		if rep.gps.altitude != "" {
			fmt.Fprintf(&sb, "  高度: %s\n", rep.gps.altitude)
		}
		if rep.gps.dateStamp != "" {
			fmt.Fprintf(&sb, "  拍摄日期: %s\n", rep.gps.dateStamp)
		}
		// 经纬度 → 拍摄位置：命中中国省级行政区显示省份，否则一律显示国外
		if loc := lookupChinaRegion(rep.gps.latDec, rep.gps.lonDec); loc != "" {
			fmt.Fprintf(&sb, "  拍摄位置: 中国 · %s\n", loc)
		} else {
			sb.WriteString("  拍摄位置: 国外\n")
		}
		fmt.Fprintf(&sb, "  Google 地图: https://www.google.com/maps?q=%.6f,%.6f\n", rep.gps.latDec, rep.gps.lonDec)
	}

	if rep.xmp != "" {
		xmp := strings.TrimSpace(rep.xmp)
		if len(xmp) > 8192 {
			xmp = xmp[:8192] + "\n… (截断，完整 XMP 过长)"
		}
		sb.WriteString("\n[XMP 元数据（原始 XML）]\n")
		sb.WriteString(xmp)
		sb.WriteString("\n")
	}

	if len(rep.pngTexts) > 0 {
		sb.WriteString("\n[PNG 文本块]\n")
		for _, t := range rep.pngTexts {
			fmt.Fprintf(&sb, "  %s: %s\n", t.k, t.v)
		}
	}

	if len(rep.main) == 0 && len(rep.exif) == 0 && rep.gps == nil {
		rep.notes = append(rep.notes, "未发现 EXIF 元数据")
	}
	if rep.gps == nil {
		rep.notes = append(rep.notes, "未发现 GPS 坐标，无法判断拍摄位置")
	}
	if len(rep.notes) > 0 {
		sb.WriteString("\n[提示]\n")
		for _, n := range rep.notes {
			sb.WriteString("  [!] " + n + "\n")
		}
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: sb.String()}, nil
}
