package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

func buildTokyoJPEG() []byte {
	tiff := buildTIFFWithGPS(35, 40, 3432, 100, 139, 39, 108, 100)
	b := []byte{0xFF, 0xD8}
	sof := []byte{8, 0x0B, 0xB8, 0x0F, 0xA0, 3, 1, 0x11, 0, 2, 0x11, 1, 3, 0x11, 1}
	b = jpegAppend(b, 0xC0, sof)
	b = jpegAppend(b, 0xE1, append([]byte("Exif\x00\x00"), tiff...))
	return append(b, 0xFF, 0xD9)
}

func TestImageBatchEndToEnd(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"bj.jpg":      buildJPEG(),      // 北京 → 国内
		"tokyo.jpg":   buildTokyoJPEG(), // 东京 → 国外
		"plain.png":   buildPNG(false),  // 无 GPS
		"bad.txt":     []byte("not an image at all"),
		"sub/x.webp":  buildWebP(),      // 子目录内 → 国内（北京 GPS）
	}
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := imageBatchRun(nil, map[string]any{
		"file": []string{"bj.jpg", "tokyo.jpg", "plain.png", "bad.txt", "sub/x.webp"},
	}, &core.ToolContext{Workdir: dir, ArtifactsDir: dir + "_out"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("状态 %v, stderr=%s", res.Status, res.Stderr)
	}
	var out batchOutput
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		t.Fatalf("stdout 不是合法 JSON: %v", err)
	}
	if out.Summary.Total != 5 || out.Summary.Analyzed != 4 || out.Summary.Skipped != 1 {
		t.Errorf("汇总不符: %+v", out.Summary)
	}
	if out.Summary.Domestic != 2 || out.Summary.Abroad != 1 || out.Summary.NoGPS != 1 {
		t.Errorf("位置统计不符: 国内=%d 国外=%d 无GPS=%d",
			out.Summary.Domestic, out.Summary.Abroad, out.Summary.NoGPS)
	}
	if out.Summary.Formats["JPEG"] != 2 || out.Summary.Formats["PNG"] != 1 || out.Summary.Formats["WebP"] != 1 {
		t.Errorf("格式统计不符: %v", out.Summary.Formats)
	}
	if out.Summary.Provinces["北京市"] != 2 {
		t.Errorf("省份统计不符: %v", out.Summary.Provinces)
	}
	byName := map[string]batchImageRecord{}
	for _, img := range out.Images {
		byName[img.Name] = img
	}
	if r := byName["bj.jpg"]; r.Location != "中国 · 北京市" || !r.HasGPS || r.Make != "Canon" {
		t.Errorf("bj.jpg 记录不符: %+v", r)
	}
	if r := byName["tokyo.jpg"]; r.Location != "国外" || r.GPSLat < 35.6761 || r.GPSLat > 35.6763 {
		t.Errorf("tokyo.jpg 记录不符: %+v", r)
	}
	if r := byName["plain.png"]; r.HasGPS || r.Location != "" || r.Width != 100 {
		t.Errorf("plain.png 记录不符: %+v", r)
	}
	if r := byName["sub/x.webp"]; r.Format != "WebP" || r.Location != "中国 · 北京市" {
		t.Errorf("sub/x.webp 记录不符: %+v", r)
	}
	if len(out.SkippedFiles) != 1 || out.SkippedFiles[0].Name != "bad.txt" {
		t.Errorf("跳过列表不符: %+v", out.SkippedFiles)
	}
	// 产物：batch_report.json
	report, err := os.ReadFile(filepath.Join(dir+"_out", "batch_report.json"))
	if err != nil {
		t.Fatalf("缺少 batch_report.json: %v", err)
	}
	if !strings.Contains(string(report), `"name": "bj.jpg"`) {
		t.Error("batch_report.json 内容缺失")
	}
}

func TestImageBatchNoFiles(t *testing.T) {
	res, err := imageBatchRun(nil, map[string]any{"file": []string{}}, &core.ToolContext{Workdir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != core.StatusError {
		t.Fatalf("期望 error, 得到 %v", res.Status)
	}
}
