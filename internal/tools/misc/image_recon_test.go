package misc

// misc.image_recon(图片二进制侦察)测试:合成样本,经注册表走 Runner,
// 断言 stdout JSON 报告的尾部/结构/嵌入/strings 四块与 artifacts 落盘。

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

// ---------- 合成样本 ----------

// synthJPG 生成最小结构 JPG(可选熵区是否含伪 FFD9 字节填充;不合法出现)。
func synthJPG(entropyLen int) []byte {
	var b bytes.Buffer
	b.Write([]byte{0xFF, 0xD8})                                    // SOI
	b.Write([]byte{0xFF, 0xE0, 0x00, 0x10})                        // APP0 len=16
	b.Write([]byte("JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")) // JFIF 体
	b.Write([]byte{0xFF, 0xDB, 0x00, 0x43})                        // DQT len=67
	b.WriteByte(0x00)
	for i := 0; i < 64; i++ {
		b.WriteByte(byte(16 + i%8))
	}
	// SOF0 len=17:h=100 w=200
	b.Write([]byte{0xFF, 0xC0, 0x00, 0x11, 0x08, 0x00, 0x64, 0x00, 0xC8, 0x01, 0x08, 0x02, 0x11, 0x03, 0x11, 0x00, 0x00, 0x00, 0x00})
	// SOS len=12
	b.Write([]byte{0xFF, 0xDA, 0x00, 0x0C, 0x03, 0x01, 0x00, 0x02, 0x11, 0x03, 0x11, 0x00, 0x3F, 0x00})
	for i := 0; i < entropyLen; i++ { // 熵数据:不含 FFD9
		b.WriteByte([]byte{0x80, 0x40, 0x55, 0xAA}[i%4])
	}
	b.Write([]byte{0xFF, 0xD9}) // EOI
	return b.Bytes()
}

// synthPNG 生成最小 chunk 结构 PNG。
func synthPNG() []byte {
	var b bytes.Buffer
	b.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'})
	chunk := func(typ string, data []byte) {
		var lenb [4]byte
		binary.BigEndian.PutUint32(lenb[:], uint32(len(data)))
		b.Write(lenb[:])
		b.WriteString(typ)
		b.Write(data)
		b.Write([]byte{0xDE, 0xAD, 0xBE, 0xEF}) // 假 CRC(crc_ok=false,测试只断言存在)
	}
	chunk("IHDR", []byte{0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0}) // 1x1 truecolor
	chunk("IDAT", []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	chunk("IEND", nil)
	return b.Bytes()
}

// synthZIP 生成含一个文件的最小 ZIP。
func synthZIP(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("secret.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("hidden-in-zip")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// ---------- 运行与解析 ----------

func reconRunTool(t *testing.T, data []byte, name string, params map[string]any) (map[string]any, string) {
	t.Helper()
	tool, ok := core.DefaultRegistry.Get("misc.image_recon")
	if !ok {
		t.Fatal("misc.image_recon 未注册")
	}
	work := t.TempDir()
	arts := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
	p := map[string]any{"file": name}
	for k, v := range params {
		p[k] = v
	}
	tc := &core.ToolContext{Workdir: work, ArtifactsDir: arts}
	res, err := tool.Run(context.Background(), p, tc)
	if err != nil {
		t.Fatalf("运行出错: %v", err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("期望 success,实际 %s: %s", res.Status, res.Stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &m); err != nil {
		t.Fatalf("stdout 非合法 JSON: %v\n%.200s", err, res.Stdout)
	}
	return m, arts
}

func num(t *testing.T, m map[string]any, path ...string) float64 {
	t.Helper()
	var cur any = m
	for _, k := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("路径 %v 非对象", path)
		}
		cur = mm[k]
	}
	f, ok := cur.(float64)
	if !ok {
		t.Fatalf("路径 %v 非数值: %v", path, cur)
	}
	return f
}

func str(t *testing.T, m map[string]any, path ...string) string {
	t.Helper()
	var cur any = m
	for i, k := range path {
		if i == len(path)-1 {
			mm := cur.(map[string]any)
			s, ok := mm[k].(string)
			if !ok {
				t.Fatalf("路径 %v 非字符串: %v", path, mm[k])
			}
			return s
		}
		mm, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("路径 %v 非对象", path)
		}
		cur = mm[k]
	}
	t.Fatalf("unreachable")
	return ""
}

// ---------- 五个测试 ----------

// 1. JPG 尾部 24B 打乱 flag(gugugu 真实尾串)→ 尾部命中、判文本、artifact 内容一致。
func TestImageReconJPGTrailingText(t *testing.T) {
	tail := []byte("fg2ivyo}l{2s3_o@aw__rcl@")
	jpg := append(synthJPG(200), tail...)
	m, arts := reconRunTool(t, jpg, "test.jpg", nil)
	if num(t, m, "trailing", "size") != float64(len(tail)) {
		t.Fatalf("尾部大小 = %v,期望 %d", m["trailing"], len(tail))
	}
	if !strings.Contains(str(t, m, "trailing", "type"), "文本") {
		t.Fatalf("24B 尾串应判文本,实际 %q", str(t, m, "trailing", "type"))
	}
	art := str(t, m, "trailing", "artifact")
	got, err := os.ReadFile(filepath.Join(arts, art))
	if err != nil || !bytes.Equal(got, tail) {
		t.Fatalf("尾部 artifact 不一致: %v", err)
	}
	if str(t, m, "file", "format") != "JPG" {
		t.Fatalf("格式识别错: %q", str(t, m, "file", "format"))
	}
}

// 2. PNG + 内嵌 ZIP → chunk 清单含 IHDR/IEND;嵌入 artifact 可被 archive/zip 重新打开。
func TestImageReconPNGEmbeddedZIP(t *testing.T) {
	zp := synthZIP(t)
	file := append(synthPNG(), zp...)
	m, arts := reconRunTool(t, file, "test.png", nil)
	var names []string
	for _, s := range m["structure"].([]any) {
		names = append(names, s.(map[string]any)["name"].(string))
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "IHDR") || !strings.Contains(joined, "IEND") {
		t.Fatalf("chunk 清单缺 IHDR/IEND: %v", names)
	}
	em, ok := m["embedded"].([]any)
	if !ok || len(em) == 0 {
		t.Fatalf("嵌入 ZIP 未命中: %v", m["embedded"])
	}
	h := em[0].(map[string]any)
	artPath := filepath.Join(arts, h["artifact"].(string))
	zr, err := zip.OpenReader(artPath)
	if err != nil {
		t.Fatalf("嵌入 ZIP artifact 无法打开: %v", err)
	}
	defer zr.Close()
	if len(zr.File) == 0 || zr.File[0].Name != "secret.txt" {
		t.Fatalf("ZIP 条目不符: %v", zr.File)
	}
	if str(t, m, "file", "format") != "PNG" {
		t.Fatalf("格式识别错")
	}
}

// 3. 干净 JPG(EOI 即文件尾)→ 无尾部、无嵌入、无 artifact。
func TestImageReconCleanJPG(t *testing.T) {
	m, arts := reconRunTool(t, synthJPG(100), "clean.jpg", nil)
	if m["trailing"] != nil {
		t.Fatalf("干净图不应有尾部: %v", m["trailing"])
	}
	if em, ok := m["embedded"].([]any); ok && len(em) > 0 {
		t.Fatalf("干净图不应有嵌入: %v", em)
	}
	if artsList, _ := os.ReadDir(arts); len(artsList) > 0 {
		t.Fatalf("干净图不应产生 artifact")
	}
}

// 4. 尾部打乱串出现在 strings 预览(可复制块的数据源)。
func TestImageReconStringsIncludeTail(t *testing.T) {
	tail := []byte("fg2ivyo}l{2s3_o@aw__rcl@")
	jpg := append(synthJPG(200), tail...)
	m, _ := reconRunTool(t, jpg, "s.jpg", nil)
	found := false
	for _, p := range m["strings"].(map[string]any)["previews"].([]any) {
		if strings.Contains(p.(string), "fg2ivyo") {
			found = true
		}
	}
	if !found {
		t.Fatalf("strings 预览未含尾部串")
	}
}

// 5. EOI 正向扫描:尾部是整张小 JPG(含自身 EOI)→ 尾部不被吸收,size = 嵌入图完整大小。
func TestImageReconEOIFwdScan(t *testing.T) {
	mini := synthJPG(16) // 自身含 SOI/段/EOI
	host := synthJPG(300)
	file := append(host, mini...)
	m, arts := reconRunTool(t, file, "nest.jpg", nil)
	if m["trailing"] == nil {
		t.Fatal("嵌入整图场景尾部不应为空(rfind 吸收缺陷)")
	}
	if got := num(t, m, "trailing", "size"); got != float64(len(mini)) {
		t.Fatalf("尾部大小 = %v,期望嵌入图完整大小 %d", got, len(mini))
	}
	if !strings.Contains(str(t, m, "trailing", "type"), "JPEG") && !strings.Contains(str(t, m, "trailing", "type"), "图") {
		t.Fatalf("尾部类型应识别为图片: %q", str(t, m, "trailing", "type"))
	}
	// 嵌入图 artifact 应能再次魔数识别(内容以 FFD8FF 开头)
	art := str(t, m, "trailing", "artifact")
	got, err := os.ReadFile(filepath.Join(arts, art))
	if err != nil || len(got) < 3 || got[0] != 0xFF || got[1] != 0xD8 {
		t.Fatalf("尾部 artifact 非完整 JPG: %v", err)
	}
}

// 6.(live,样本存在才跑)用户真实样本 gugugu.jpg:EOI 后 24B 打乱 flag。
func TestImageReconGuguguLive(t *testing.T) {
	const sample = `C:\Users\HP\Downloads\gugugu.jpg`
	data, err := os.ReadFile(sample)
	if err != nil {
		t.Skipf("样本不存在,跳过: %v", err)
	}
	m, arts := reconRunTool(t, data, "gugugu.jpg", nil)
	if m["trailing"] == nil {
		t.Fatal("真实样本应有尾部数据")
	}
	if got := num(t, m, "trailing", "size"); got != 24 {
		t.Fatalf("尾部大小 = %v,期望 24", got)
	}
	if !strings.Contains(str(t, m, "trailing", "type"), "文本") {
		t.Fatalf("尾部应判文本: %q", str(t, m, "trailing", "type"))
	}
	if _, err := os.Stat(filepath.Join(arts, str(t, m, "trailing", "artifact"))); err != nil {
		t.Fatalf("尾部 artifact 未落盘: %v", err)
	}
	found := false
	for _, p := range m["strings"].(map[string]any)["previews"].([]any) {
		if strings.HasPrefix(p.(string), "fg2ivyo") {
			found = true
		}
	}
	if !found {
		t.Fatal("strings 预览应含打乱 flag 串")
	}
}
