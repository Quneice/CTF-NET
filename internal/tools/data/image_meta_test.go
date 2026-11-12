package data

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

// ---------------------------------------------------------------------------
// 测试用 TIFF 构造器（小端序），布局固定：
//
//	header(8) → IFD0(7 条目, ends 98) → ExifIFD(4 条目, ends 152)
//	→ GPSIFD(6 条目, ends 230) → 数据区(230 起)
//
// IFD0: Make / Model / Orientation=6 / ImageWidth=4000 / ImageLength=3000
//	+ ExifIFD 指针 + GPS 指针
// ExifIFD: ExposureTime=1/125 / FNumber=28/10 / ISO=200 / DateTimeOriginal
// GPSIFD: 经纬度由调用方给定（度/分/秒的分子分母）+ altitude=35 / dateStamp
// ---------------------------------------------------------------------------
func buildTIFF() []byte {
	// 默认北京：39°54'15.12"N → 39.9042 / 116°24'26.64"E → 116.4074
	return buildTIFFWithGPS(39, 54, 1512, 100, 116, 24, 2664, 100)
}

func buildTIFFWithGPS(latD, latM, latSN, latSD, lonD, lonM, lonSN, lonSD uint32) []byte {
	le := binary.LittleEndian
	b := make([]byte, 400)
	copy(b, "II")
	le.PutUint16(b[2:4], 42)
	le.PutUint32(b[4:8], 8)

	putEntry := func(base, i int, tag, typ uint16, count, inline uint32, dataOff uint32) {
		p := base + 2 + i*12
		le.PutUint16(b[p:p+2], tag)
		le.PutUint16(b[p+2:p+4], typ)
		le.PutUint32(b[p+4:p+8], count)
		if dataOff != 0 {
			le.PutUint32(b[p+8:p+12], dataOff)
		} else {
			le.PutUint32(b[p+8:p+12], inline)
		}
	}

	// ---- IFD0 @8, count 7, next=0 @8+2+84=94
	le.PutUint16(b[8:10], 7)
	putEntry(8, 0, 0x010F, 2, 6, 0, 230) // Make "Canon\0"
	putEntry(8, 1, 0x0110, 2, 8, 0, 236) // Model "EOS R5\0"
	putEntry(8, 2, 0x0112, 3, 1, 6, 0)   // Orientation=6（内联）
	putEntry(8, 3, 0x0100, 3, 1, 4000, 0)
	putEntry(8, 4, 0x0101, 3, 1, 3000, 0)
	putEntry(8, 5, 0x8769, 4, 1, 98, 0)  // ExifIFD
	putEntry(8, 6, 0x8825, 4, 1, 152, 0) // GPSIFD
	le.PutUint32(b[94:98], 0)

	// ---- ExifIFD @98, count 4, next=0 @98+2+48=148
	le.PutUint16(b[98:100], 4)
	putEntry(98, 0, 0x829A, 5, 1, 0, 264) // ExposureTime @264
	putEntry(98, 1, 0x829D, 5, 1, 0, 272) // FNumber @272
	putEntry(98, 2, 0x8827, 3, 1, 200, 0) // ISO=200（内联）
	putEntry(98, 3, 0x9003, 2, 20, 0, 244)
	le.PutUint32(b[148:152], 0)

	// ---- GPSIFD @152, count 6, next=0 @152+2+72=226
	le.PutUint16(b[152:154], 6)
	putEntry(152, 0, 0x0001, 2, 2, uint32('N'), 0) // "N\0"（≤4 字节内联）
	putEntry(152, 1, 0x0002, 5, 3, 0, 280)         // lat
	putEntry(152, 2, 0x0003, 2, 2, uint32('E'), 0) // "E\0"（内联）
	putEntry(152, 3, 0x0004, 5, 3, 0, 304)         // lon
	putEntry(152, 4, 0x0006, 5, 1, 0, 344)         // altitude
	putEntry(152, 5, 0x001D, 2, 11, 0, 332)
	le.PutUint32(b[226:230], 0)

	// ---- 数据区
	copy(b[230:236], "Canon\x00")
	copy(b[236:244], "EOS R5\x00")
	copy(b[244:264], "2024:05:01 12:30:00\x00")
	le.PutUint32(b[264:268], 1)
	le.PutUint32(b[268:272], 125)
	le.PutUint32(b[272:276], 28)
	le.PutUint32(b[276:280], 10)
	// lat 度/分/秒（有理数）
	le.PutUint32(b[280:284], latD)
	le.PutUint32(b[284:288], 1)
	le.PutUint32(b[288:292], latM)
	le.PutUint32(b[292:296], 1)
	le.PutUint32(b[296:300], latSN)
	le.PutUint32(b[300:304], latSD)
	// lon 度/分/秒（有理数）
	le.PutUint32(b[304:308], lonD)
	le.PutUint32(b[308:312], 1)
	le.PutUint32(b[312:316], lonM)
	le.PutUint32(b[316:320], 1)
	le.PutUint32(b[320:324], lonSN)
	le.PutUint32(b[324:328], lonSD)
	copy(b[328:330], "N\x00")
	copy(b[330:332], "E\x00")
	copy(b[332:343], "2024:05:01\x00")
	le.PutUint32(b[344:348], 35)
	le.PutUint32(b[348:352], 1)
	return b
}

// jpegAppend 追加一个 JPEG 段。
func jpegAppend(b []byte, marker byte, payload []byte) []byte {
	b = append(b, 0xFF, marker)
	l := len(payload) + 2
	b = append(b, byte(l>>8), byte(l&0xFF))
	return append(b, payload...)
}

func buildJPEG() []byte {
	tiff := buildTIFF()
	b := []byte{0xFF, 0xD8}
	// SOF0: precision 8 / 3000x4000 / 3 分量
	sof := []byte{8, 0x0B, 0xB8, 0x0F, 0xA0, 3, 1, 0x11, 0, 2, 0x11, 1, 3, 0x11, 1}
	b = jpegAppend(b, 0xC0, sof)
	b = jpegAppend(b, 0xE1, append([]byte("Exif\x00\x00"), tiff...))
	return append(b, 0xFF, 0xD9)
}

func pngChunk(typ string, data []byte) []byte {
	var c []byte
	l := len(data)
	c = append(c, byte(l>>24), byte(l>>16), byte(l>>8), byte(l&0xFF))
	c = append(c, typ...)
	c = append(c, data...)
	// CRC 占位 4 字节（walker 不校验内容，但按标准 PNG 布局步进）
	return append(c, 0, 0, 0, 0)
}

func zlibCompress(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func buildPNG(withEXIF bool) []byte {
	b := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	// IHDR: 100x50, 8bit, RGB
	b = append(b, pngChunk("IHDR", []byte{
		0, 0, 0, 100, 0, 0, 0, 50, 8, 2, 0, 0, 0,
	})...)
	b = append(b, pngChunk("tEXt", append([]byte("Comment\x00"), []byte("hello")...))...)
	if withEXIF {
		b = append(b, pngChunk("eXIf", buildTIFF())...)
	}
	b = append(b, pngChunk("IEND", nil)...)
	return b
}

func buildPNGWithZTXt(t *testing.T) []byte {
	b := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	b = append(b, pngChunk("IHDR", []byte{0, 0, 0, 100, 0, 0, 0, 50, 8, 2, 0, 0, 0})...)
	payload := append([]byte("Note\x00"), 0x00)
	payload = append(payload, zlibCompress(t, "world")...)
	b = append(b, pngChunk("zTXt", payload)...)
	return append(b, pngChunk("IEND", nil)...)
}

func buildWebP() []byte {
	tiff := buildTIFF()
	le := binary.LittleEndian
	// VP8X: flags(1) reserved(3) w-1=99 h-1=49
	vp8x := []byte{0, 0, 0, 0, 99, 0, 0, 49, 0, 0}
	body := append([]byte("VP8X"), make([]byte, 4)...)
	le.PutUint32(body[4:8], uint32(len(vp8x)))
	body = append(body, vp8x...)
	body = append(body, "EXIF"...)
	sz := make([]byte, 4)
	le.PutUint32(sz, uint32(len(tiff)))
	body = append(body, sz...)
	body = append(body, tiff...)

	b := append([]byte("RIFF"), make([]byte, 4)...)
	le.PutUint32(b[4:8], uint32(4+len(body)))
	b = append(b, "WEBP"...)
	return append(b, body...)
}

// runImageMeta 落盘后调用 Runner。
func runImageMeta(t *testing.T, name string, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := imageMetaRun(nil, map[string]any{"file": name}, &core.ToolContext{Workdir: dir})
	if err != nil {
		t.Fatalf("run 出错: %v", err)
	}
	return res.Stdout
}

func mustContain(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("输出缺少 %q\n完整输出:\n%s", w, out)
		}
	}
}

func TestImageMetaJPEG(t *testing.T) {
	out := runImageMeta(t, "photo.jpg", buildJPEG())
	mustContain(t, out,
		"格式: JPEG",
		"尺寸: 4000 x 3000",
		"3 分量, 8 bit",
		"相机制造商: Canon",
		"相机型号: EOS R5",
		"方向: 顺时针旋转 90°",
		"曝光时间: 1/125 秒",
		"光圈值: f/2.8",
		"ISO 感光度: 200",
		"拍摄时间: 2024:05:01 12:30:00",
		"纬度: 39.904200° (北纬)",
		"经度: 116.407400° (东经)",
		"高度: 35 米",
		"拍摄日期: 2024:05:01",
		"拍摄位置: 中国 · 北京市",
		"Google 地图: https://www.google.com/maps?q=39.904200,116.407400",
	)
}

// 东京（国外）：35°40'34.32"N 139°39'1.08"E → 35.6762 / 139.6503，应显示国外
func TestImageMetaLocationAbroad(t *testing.T) {
	tiff := buildTIFFWithGPS(35, 40, 3432, 100, 139, 39, 108, 100)
	b := []byte{0xFF, 0xD8}
	sof := []byte{8, 0x0B, 0xB8, 0x0F, 0xA0, 3, 1, 0x11, 0, 2, 0x11, 1, 3, 0x11, 1}
	b = jpegAppend(b, 0xC0, sof)
	b = jpegAppend(b, 0xE1, append([]byte("Exif\x00\x00"), tiff...))
	b = append(b, 0xFF, 0xD9)
	out := runImageMeta(t, "tokyo.jpg", b)
	mustContain(t, out,
		"纬度: 35.676200° (北纬)",
		"经度: 139.650300° (东经)",
		"拍摄位置: 国外",
	)
}

// 标签本身写反的文件（实测数据源：GPSLatitude 标签存经度、GPSLongitude 存纬度）：
// 解析必须忠实还原标签原始值，不做任何自愈交换 —— 展示什么由数据决定。
func TestGPSFaithfulToSwappedTags(t *testing.T) {
	// 传入参数故意颠倒：把经度值(116°24'26.64")写进 lat 槽、纬度值(39°54'15.12")写进 lon 槽
	tiff := buildTIFFWithGPS(116, 24, 2664, 100, 39, 54, 1512, 100)
	b := []byte{0xFF, 0xD8}
	sof := []byte{8, 0x0B, 0xB8, 0x0F, 0xA0, 3, 1, 0x11, 0, 2, 0x11, 1, 3, 0x11, 1}
	b = jpegAppend(b, 0xC0, sof)
	b = jpegAppend(b, 0xE1, append([]byte("Exif\x00\x00"), tiff...))
	b = append(b, 0xFF, 0xD9)
	rep := analyzeImageBytes("swapped.jpg", b)
	if rep.gps == nil {
		t.Fatal("未解析出 GPS")
	}
	// 忠实还原：latDec 等于标签 0x0002 里的原始值（经度数字），lonDec 同理
	if math.Abs(rep.gps.latDec-116.4074) > 1e-4 {
		t.Errorf("latDec = %f，应忠实等于标签原始值 116.4074", rep.gps.latDec)
	}
	if math.Abs(rep.gps.lonDec-39.9042) > 1e-4 {
		t.Errorf("lonDec = %f，应忠实等于标签原始值 39.9042", rep.gps.lonDec)
	}
	if rep.gps.latRef != "N" || rep.gps.lonRef != "E" {
		t.Errorf("ref 应保持标签原始值 N/E，实际 %q/%q", rep.gps.latRef, rep.gps.lonRef)
	}
	// 按"坐标不在国内 → 国外"规则，忠实值（纬度=116.4）不会命中任何省份
	if loc := lookupChinaRegion(rep.gps.latDec, rep.gps.lonDec); loc != "" {
		t.Errorf("忠实值不应命中省份，实际 %q", loc)
	}
}

func TestLookupChinaRegion(t *testing.T) {
	cases := []struct {
		name       string
		lat, lon   float64
		wantRegion string
	}{
		{"北京", 39.9042, 116.4074, "北京市"},
		{"上海", 31.2304, 121.4737, "上海市"},
		{"广州", 23.1291, 113.2644, "广东省"},
		{"乌鲁木齐", 43.8256, 87.6168, "新疆维吾尔自治区"},
		{"拉萨", 29.6525, 91.1398, "西藏自治区"},
		{"齐齐哈尔", 47.3544, 123.9169, "黑龙江省"},
		{"海口", 20.0440, 110.1983, "海南省"},
		{"香港", 22.3193, 114.1694, "香港特别行政区"},
		{"台北", 25.0330, 121.5654, "台湾省"},
		// 包围盒方法曾误判的边境案例（多边形射线法修正）
		{"哈尔滨（邻吉林边界）", 45.8038, 126.5349, "黑龙江省"},
		{"平凉（邻宁夏边界）", 35.5430, 106.6650, "甘肃省"},
		{"德州（邻河北边界）", 37.4341, 116.3575, "山东省"},
		{"阿拉善左旗（邻宁夏/甘肃）", 38.8334, 105.6751, "内蒙古自治区"},
		// 国外（含紧邻边境的境外城市）
		{"东京", 35.6762, 139.6503, ""},
		{"首尔", 37.5665, 126.9780, ""},
		{"纽约", 40.7128, -74.0060, ""},
		{"河内（紧邻广西/云南边境）", 21.0285, 105.8542, ""},
		{"平壤（紧邻辽宁/吉林边境）", 39.0392, 125.7625, ""},
	}
	for _, c := range cases {
		if got := lookupChinaRegion(c.lat, c.lon); got != c.wantRegion {
			t.Errorf("%s: lookup(%v,%v) = %q, 期望 %q", c.name, c.lat, c.lon, got, c.wantRegion)
		}
	}
}

// 射线法（偶奇规则）单元测试：正方形、带洞多边形、边界外点
func TestPointInRings(t *testing.T) {
	square := [][][]float64{
		{{0, 0}, {0, 10}, {10, 10}, {10, 0}, {0, 0}},
	}
	cases := []struct {
		name string
		lat, lon float64
		rings [][][]float64
		want bool
	}{
		{"正方形内部", 5, 5, square, true},
		{"正方形外部", 15, 5, square, false},
		{"正方形外部-下", 5, -5, square, false},
		// 带洞：外环 (0,0)-(10,10)，洞 (2,2)-(8,8)
		{"洞内（应在外）", 5, 5, [][][]float64{
			{{0, 0}, {0, 10}, {10, 10}, {10, 0}, {0, 0}},
			{{2, 2}, {2, 8}, {8, 8}, {8, 2}, {2, 2}},
		}, false},
		{"外环与洞之间（应在内）", 1, 1, [][][]float64{
			{{0, 0}, {0, 10}, {10, 10}, {10, 0}, {0, 0}},
			{{2, 2}, {2, 8}, {8, 8}, {8, 2}, {2, 2}},
		}, true},
		// 退化环（少于 3 点）应被忽略
		{"退化环不影响判定", 5, 5, append(square, [][]float64{{1, 1}, {2, 2}}), true},
	}
	for _, c := range cases {
		if got := pointInRings(c.lat, c.lon, c.rings); got != c.want {
			t.Errorf("%s: pointInRings(%v,%v) = %v, 期望 %v", c.name, c.lat, c.lon, got, c.want)
		}
	}
}

// 真实地理数据自检：34 省全解析、各省包围盒有限且合理
func TestChinaPolygonsLoaded(t *testing.T) {
	polys, err := loadChinaPolygons()
	if err != nil {
		t.Fatalf("加载地理数据失败: %v", err)
	}
	if len(polys) != 34 {
		t.Fatalf("期望 34 个省级行政区, 得到 %d", len(polys))
	}
	names := map[string]bool{}
	for _, p := range polys {
		if p.Name == "" {
			t.Fatal("存在空名多边形")
		}
		if names[p.Name] {
			t.Fatalf("省份重名: %s", p.Name)
		}
		names[p.Name] = true
		if len(p.Rings) == 0 {
			t.Fatalf("%s 无环", p.Name)
		}
		if math.IsInf(p.minLon, 0) || math.IsInf(p.maxLat, 0) {
			t.Fatalf("%s 包围盒未计算", p.Name)
		}
	}
}

func TestImageMetaPNG(t *testing.T) {
	out := runImageMeta(t, "pic.png", buildPNG(true))
	mustContain(t, out,
		"格式: PNG",
		"尺寸: 100 x 50",
		"8 bit, RGB",
		"相机制造商: Canon",
		"Comment: hello",
	)
	out = runImageMeta(t, "z.png", buildPNGWithZTXt(t))
	mustContain(t, out, "Note: world")
}

func TestImageMetaWebP(t *testing.T) {
	out := runImageMeta(t, "img.webp", buildWebP())
	mustContain(t, out,
		"格式: WebP",
		"尺寸: 100 x 50",
		"相机制造商: Canon",
	)
}

func TestImageMetaTIFFRaw(t *testing.T) {
	out := runImageMeta(t, "scan.tiff", buildTIFF())
	mustContain(t, out,
		"格式: TIFF",
		"图像宽度: 4000",
		"图像高度: 3000",
		"相机制造商: Canon",
	)
}

func TestImageMetaNoEXIF(t *testing.T) {
	out := runImageMeta(t, "plain.png", buildPNG(false))
	mustContain(t, out, "格式: PNG", "未发现 EXIF 元数据", "未发现 GPS 坐标，无法判断拍摄位置")
}

func TestImageMetaMissingFile(t *testing.T) {
	res, err := imageMetaRun(nil, map[string]any{"file": "nope.png"}, &core.ToolContext{Workdir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != core.StatusError {
		t.Fatalf("期望 error 状态, 得到 %v", res.Status)
	}
}

func TestParseTIFFBigEndian(t *testing.T) {
	// 大端 TIFF 只验证解析器骨架：魔数 + 空 IFD0
	b := make([]byte, 8+2+4)
	copy(b, "MM")
	binary.BigEndian.PutUint16(b[2:4], 42)
	binary.BigEndian.PutUint32(b[4:8], 8)
	binary.BigEndian.PutUint16(b[8:10], 0)
	tf, err := parseTIFF(b)
	if err != nil {
		t.Fatal(err)
	}
	if tf.order != binary.BigEndian {
		t.Fatal("字节序错误")
	}
	if _, err := tf.readIFD(8); err != nil {
		t.Fatal(err)
	}
}
