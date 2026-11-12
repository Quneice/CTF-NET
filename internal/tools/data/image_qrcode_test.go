package data

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"golang.org/x/text/encoding/unicode"
)

// qrBitMatrix 用 gozxing 的编码器生成真实二维码位图（与解码器独立验证端到端）。
func qrBitMatrix(t *testing.T, content string, size int, utf8Hint bool) *gozxing.BitMatrix {
	t.Helper()
	writer := qrcode.NewQRCodeWriter()
	var hints map[gozxing.EncodeHintType]interface{}
	if utf8Hint {
		// gozxing 的 CHARACTER_SET 提示要求 encoding.Encoding 值（非字符串）
		hints = map[gozxing.EncodeHintType]interface{}{gozxing.EncodeHintType_CHARACTER_SET: unicode.UTF8}
	}
	bm, err := writer.Encode(content, gozxing.BarcodeFormat_QR_CODE, size, size, hints)
	if err != nil {
		t.Fatalf("生成二维码失败: %v", err)
	}
	return bm
}

// bitMatrixToImage 把二维码位图转成标准 RGBA 图片。
func bitMatrixToImage(bm *gozxing.BitMatrix) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, bm.GetWidth(), bm.GetHeight()))
	for y := 0; y < bm.GetHeight(); y++ {
		for x := 0; x < bm.GetWidth(); x++ {
			if bm.Get(x, y) {
				img.Set(x, y, color.Black)
			} else {
				img.Set(x, y, color.White)
			}
		}
	}
	return img
}

// drawBitMatrixAt 把位图绘制到目标图片的指定偏移处。
func drawBitMatrixAt(dst *image.RGBA, bm *gozxing.BitMatrix, ox, oy int) {
	for y := 0; y < bm.GetHeight(); y++ {
		for x := 0; x < bm.GetWidth(); x++ {
			if bm.Get(x, y) {
				dst.Set(ox+x, oy+y, color.Black)
			}
		}
	}
}

// jpegBytes 把图片编码为 JPEG 字节。
func jpegBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("JPEG 编码失败: %v", err)
	}
	return buf.Bytes()
}

func TestQRCodeScanSingle(t *testing.T) {
	img := bitMatrixToImage(qrBitMatrix(t, "CTF{qrcode-test-123}", 200, false))
	codes := scanQRCodes(img)
	if len(codes) != 1 {
		t.Fatalf("期望识别 1 个码，得到 %d", len(codes))
	}
	if codes[0].Text != "CTF{qrcode-test-123}" {
		t.Errorf("内容 = %q", codes[0].Text)
	}
	if codes[0].Format != "QR Code" {
		t.Errorf("格式 = %q", codes[0].Format)
	}
}

// 一张图里两枚二维码：应全部识别（遮罩循环）
func TestQRCodeScanMultiple(t *testing.T) {
	canvas := image.NewRGBA(image.Rect(0, 0, 430, 220))
	for x := 0; x < 430; x++ {
		for y := 0; y < 220; y++ {
			canvas.Set(x, y, color.White)
		}
	}
	drawBitMatrixAt(canvas, qrBitMatrix(t, "FIRST-CODE", 200, false), 10, 10)
	drawBitMatrixAt(canvas, qrBitMatrix(t, "SECOND-CODE", 200, false), 220, 10)
	codes := scanQRCodes(canvas)
	if len(codes) != 2 {
		t.Fatalf("期望识别 2 个码，得到 %d: %+v", len(codes), codes)
	}
	got := map[string]bool{}
	for _, c := range codes {
		got[c.Text] = true
	}
	if !got["FIRST-CODE"] || !got["SECOND-CODE"] {
		t.Errorf("内容不全: %+v", codes)
	}
}

// UTF-8 内容（中文 flag）：必须完整还原，不能乱码
func TestQRCodeScanUTF8(t *testing.T) {
	img := bitMatrixToImage(qrBitMatrix(t, "flag{北京欢迎你_QR码}", 200, true))
	codes := scanQRCodes(img)
	if len(codes) != 1 {
		t.Fatalf("期望识别 1 个码，得到 %d", len(codes))
	}
	if codes[0].Text != "flag{北京欢迎你_QR码}" {
		t.Errorf("内容 = %q", codes[0].Text)
	}
}

func TestQRCodeScanNoCode(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 120, 120))
	for x := 0; x < 120; x++ {
		for y := 0; y < 120; y++ {
			img.Set(x, y, color.White)
		}
	}
	if codes := scanQRCodes(img); len(codes) != 0 {
		t.Errorf("纯白图不应识别出码: %+v", codes)
	}
}

// 端到端：含码图 + 无码图 + 非图片文件，校验 JSON 契约与产物
func TestImageQrcodeRunEndToEnd(t *testing.T) {
	dir := t.TempDir()
	withQR := jpegBytes(t, bitMatrixToImage(qrBitMatrix(t, "E2E-CODE-1", 160, false)))
	noQR := jpegBytes(t, func() image.Image {
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		for x := 0; x < 100; x++ {
			for y := 0; y < 100; y++ {
				img.Set(x, y, color.White)
			}
		}
		return img
	}())
	if err := os.WriteFile(filepath.Join(dir, "a.jpg"), withQR, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.jpg"), noQR, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	art := dir + "_out"
	res, err := imageQrcodeRun(nil, map[string]any{"file": []string{"a.jpg", "b.jpg", "c.txt"}},
		&core.ToolContext{Workdir: dir, ArtifactsDir: art})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("状态 %v, stderr=%s", res.Status, res.Stderr)
	}
	var out qrcodeOutput
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		t.Fatalf("stdout 不是合法 JSON: %v", err)
	}
	if out.Summary.Total != 3 || out.Summary.Analyzed != 2 || out.Summary.Skipped != 1 {
		t.Errorf("summary 计数 = %+v", out.Summary)
	}
	if out.Summary.WithQR != 1 || out.Summary.NoQR != 1 || out.Summary.TotalCodes != 1 {
		t.Errorf("summary 码计数 = %+v", out.Summary)
	}
	if len(out.Images) != 2 {
		t.Fatalf("images = %d", len(out.Images))
	}
	if out.Images[0].Name != "a.jpg" { // 排序后 a 在前
		t.Errorf("排序 = %+v", out.Images)
	}
	if len(out.Images[0].Codes) != 1 || out.Images[0].Codes[0].Text != "E2E-CODE-1" {
		t.Errorf("a.jpg 码 = %+v", out.Images[0].Codes)
	}
	if len(out.Images[1].Codes) != 0 {
		t.Errorf("b.jpg 应无码 = %+v", out.Images[1].Codes)
	}
	if len(out.SkippedFiles) != 1 || !strings.Contains(out.SkippedFiles[0].Name, "c.txt") {
		t.Errorf("skipped = %+v", out.SkippedFiles)
	}
	// 产物 qrcode_report.json
	artPath := filepath.Join(art, "qrcode_report.json")
	if _, err := os.Stat(artPath); err != nil {
		t.Errorf("产物缺失: %v", err)
	} else if b, err := os.ReadFile(artPath); err == nil {
		if !strings.Contains(string(b), "E2E-CODE-1") {
			t.Errorf("产物未含识别内容")
		}
	}
}
