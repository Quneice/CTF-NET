// misc.image_lsb 测试:PNG/BMP 像素位平面 LSB 提取(zsteg 类套路)。
// R/G/B/A × bit0-2 × MSB/LSB 位序,auto 扫描;与 WAV 工具共享 stego 引擎识别逻辑。
package misc

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/image/bmp"

	"ctfnet/internal/core"
)

// imgBuild 生成 NRGBA 噪声图(确定性),返回可修改的像素。
func imgBuild(w, h int) (*image.NRGBA, *image.NRGBA) {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	x := uint32(777)
	for i := 0; i < w*h; i++ {
		x = x*1664525 + 1013904223
		px := img.Pix[i*4 : i*4+4]
		px[0], px[1], px[2], px[3] = byte(x>>16), byte(x>>8), byte(x), 0xFF
	}
	return img, img
}

// imgEmbed 按通道变体/位平面/位序嵌入 payload(r/g/b/a 单通道;rgb=逐像素 R,G,B 交错)。
func imgEmbed(img *image.NRGBA, channel string, plane int, msbFirst bool, payload []byte) {
	var bitstream []int
	for _, b := range payload {
		for k := 0; k < 8; k++ {
			if msbFirst {
				bitstream = append(bitstream, int(b>>(7-k))&1)
			} else {
				bitstream = append(bitstream, int(b>>k)&1)
			}
		}
	}
	w := img.Rect.Dx()
	setBit := func(idx, ch, bit int) {
		px := img.Pix[idx*4 : idx*4+4]
		px[ch] = (px[ch] &^ (1 << uint(plane))) | byte(bit<<uint(plane))
	}
	switch channel {
	case "rgb":
		for k, bit := range bitstream {
			pxIdx, ch := k/3, k%3 // 逐像素 R,G,B 交错
			if pxIdx >= w*img.Rect.Dy() {
				panic("容量不足")
			}
			setBit(pxIdx, ch, bit)
		}
	default:
		ch := map[string]int{"r": 0, "g": 1, "b": 2, "a": 3}[channel]
		for bi, bit := range bitstream {
			if bi >= w*img.Rect.Dy() {
				panic("容量不足")
			}
			setBit(bi, ch, bit)
		}
	}
}

func imgRunTool(t *testing.T, encoded []byte, filename string, params map[string]any) (*core.ToolResult, string) {
	t.Helper()
	tool, ok := core.DefaultRegistry.Get("misc.image_lsb")
	if !ok {
		t.Fatal("misc.image_lsb 未注册")
	}
	work, arts := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(work, filename), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	p := map[string]any{"file": filename}
	for k, v := range params {
		p[k] = v
	}
	res, err := tool.Run(context.Background(), p, &core.ToolContext{Workdir: work, ArtifactsDir: arts})
	if err != nil {
		t.Fatalf("运行出错: %v", err)
	}
	return res, arts
}

func imgEncodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A. NRGBA R 通道 bit0 MSB-first 藏 JPEG,auto 扫描命中,EOI 截断,预览内联。
func TestImageLSBRedBit0MsbAuto(t *testing.T) {
	payload := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, lsbNoiseBytes(2000)...)
	payload = append(payload[:1996], 0x12, 0x34, 0xFF, 0xD9) // 2000B,EOI 收尾
	img, _ := imgBuild(256, 128)
	imgEmbed(img, "r", 0, true, payload)
	res, arts := imgRunTool(t, imgEncodePNG(t, img), "cover.png", nil)

	if res.Status != core.StatusSuccess {
		t.Fatalf("期望 success: %s", res.Stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &m); err != nil {
		t.Fatalf("stdout 非法 JSON: %v", err)
	}
	file, _ := m["file"].(map[string]any)
	if file["width"] != float64(256) || file["height"] != float64(128) {
		t.Fatalf("图片元数据错误: %v", file)
	}
	hits, _ := m["hits"].([]any)
	if len(hits) == 0 {
		t.Fatal("auto 扫描未命中")
	}
	h, _ := hits[0].(map[string]any)
	if h["type"] != "jpg" || h["size"] != float64(2000) {
		t.Fatalf("命中详情错误: %v", h)
	}
	got, err := os.ReadFile(filepath.Join(arts, h["artifact"].(string)))
	if err != nil || string(got) != string(payload) {
		t.Fatalf("产物与 payload 不一致: %v", err)
	}
	if pv, _ := h["preview"].(string); len(pv) < 100 {
		t.Fatal("缺少图片预览")
	}
}

// B. 显式参数:rgb 交错 bit1 LSB-first 藏 PNG,IEND 截断。
func TestImageLSBRGBInterleaveExplicit(t *testing.T) {
	payload := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}, lsbNoiseBytes(300)...)
	payload = append(payload, 0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xAE, 0x42, 0x60, 0x82)
	img, _ := imgBuild(128, 128)
	imgEmbed(img, "rgb", 1, false, payload)
	res, arts := imgRunTool(t, imgEncodePNG(t, img), "cover.png",
		map[string]any{"channel": "rgb", "plane": "1", "order": "lsb"})

	var m map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &m); err != nil {
		t.Fatalf("stdout 非法 JSON: %v", err)
	}
	hits, _ := m["hits"].([]any)
	if len(hits) == 0 {
		t.Fatal("rgb 交错未命中")
	}
	h, _ := hits[0].(map[string]any)
	if h["type"] != "png" || h["size"] != float64(len(payload)) {
		t.Fatalf("命中详情错误: %v", h)
	}
	got, err := os.ReadFile(filepath.Join(arts, h["artifact"].(string)))
	if err != nil || string(got) != string(payload) {
		t.Fatalf("PNG 产物不一致: %v", err)
	}
}

// C. BMP 24bit 同样可提取。
func TestImageLSBBMP(t *testing.T) {
	payload := []byte("flag{bmp_l5b}")
	img, _ := imgBuild(64, 64)
	imgEmbed(img, "b", 0, true, payload)
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	res, _ := imgRunTool(t, buf.Bytes(), "cover.bmp", nil)

	var m map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &m); err != nil {
		t.Fatalf("stdout 非法 JSON: %v", err)
	}
	flags, _ := m["flags"].([]any)
	if len(flags) == 0 || flags[0] != "flag{bmp_l5b}" {
		t.Fatalf("BMP flag 提取失败: %v", flags)
	}
}

// D. JPEG(有损)输入 → 明确报错引导。
func TestImageLSBJPEGRejected(t *testing.T) {
	// 构造 1×1 JPEG
	small := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	small.Set(0, 0, color.NRGBA{R: 128, G: 64, B: 32, A: 255})
	var buf bytes.Buffer
	if err := jpgEncodeGray(&buf); err != nil { // 用现成最小 JPEG 字节
		t.Fatal(err)
	}
	res, _ := imgRunTool(t, buf.Bytes(), "cover.jpg", nil)
	if res.Status != core.StatusError || !bytes.Contains([]byte(res.Stderr), []byte("JPEG")) {
		t.Fatalf("JPEG 应引导报错: status=%s stderr=%s", res.Status, res.Stderr)
	}
}

// E. alpha 通道藏文本(NRGBA 才有 a 通道),auto 命中。
func TestImageLSBAlphaChannel(t *testing.T) {
	payload := []byte("the secret key is flag{alpha_ch} hidden here")
	img, _ := imgBuild(128, 64)
	imgEmbed(img, "a", 0, true, payload)
	res, _ := imgRunTool(t, imgEncodePNG(t, img), "cover.png", nil)

	var m map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &m); err != nil {
		t.Fatalf("stdout 非法 JSON: %v", err)
	}
	flags, _ := m["flags"].([]any)
	found := false
	for _, f := range flags {
		if f == "flag{alpha_ch}" {
			found = true
		}
	}
	if !found {
		t.Fatalf("alpha 通道 flag 未提取: %v", flags)
	}
}

// F. 纯噪声图:零命中,候选概览齐全。
func TestImageLSBNoHit(t *testing.T) {
	img, _ := imgBuild(64, 64)
	res, _ := imgRunTool(t, imgEncodePNG(t, img), "cover.png", nil)
	var m map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &m); err != nil {
		t.Fatalf("stdout 非法 JSON: %v", err)
	}
	if hits, _ := m["hits"].([]any); len(hits) != 0 {
		t.Fatalf("噪声图不应命中: %v", hits)
	}
	if cands, _ := m["candidates"].([]any); len(cands) < 12 { // rgb+r+g+b+a × bit0-2 × msb/lsb
		t.Fatalf("候选概览不足: %d", len(cands))
	}
}

// jpgEncodeGray 返回一个合法的最小 JPEG 文件字节(1×1 灰度)。
func jpgEncodeGray(w *bytes.Buffer) error {
	jpegBytes := []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
		0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43, 0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08,
		0x07, 0x07, 0x07, 0x09, 0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
		0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20, 0x24, 0x2E, 0x27, 0x20,
		0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29, 0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27,
		0x39, 0x3D, 0x38, 0x32, 0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xC0, 0x00, 0x0B, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xFF, 0xC4, 0x00, 0x1F, 0x00, 0x00, 0x01, 0x05, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04,
		0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0xFF, 0xC4, 0x00, 0xB5, 0x10, 0x00, 0x02, 0x01, 0x03,
		0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7D, 0x01, 0x02, 0x03, 0x00,
		0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32,
		0x81, 0x91, 0xA1, 0x08, 0x23, 0x42, 0xB1, 0xC1, 0x15, 0x52, 0xD1, 0xF0, 0x24, 0x33, 0x62, 0x72,
		0x82, 0x09, 0x0A, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2A, 0x34, 0x35,
		0x36, 0x37, 0x38, 0x39, 0x3A, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4A, 0x53, 0x54, 0x55,
		0x56, 0x57, 0x58, 0x59, 0x5A, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6A, 0x73, 0x74, 0x75,
		0x76, 0x77, 0x78, 0x79, 0x7A, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8A, 0x92, 0x93, 0x94,
		0x95, 0x96, 0x97, 0x98, 0x99, 0x9A, 0xA2, 0xA3, 0xA4, 0xA5, 0xA6, 0xA7, 0xA8, 0xA9, 0xAA, 0xB2,
		0xB3, 0xB4, 0xB5, 0xB6, 0xB7, 0xB8, 0xB9, 0xBA, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9,
		0xCA, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7, 0xD8, 0xD9, 0xDA, 0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6,
		0xE7, 0xE8, 0xE9, 0xEA, 0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFF, 0xDA,
		0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00, 0xFB, 0xFA, 0xFF, 0xD9,
	}
	_, err := w.Write(jpegBytes)
	return err
}
