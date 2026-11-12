package data

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// resolveLangs 纯逻辑单测
func TestResolveLangs(t *testing.T) {
	cases := []struct {
		requested string
		available map[string]bool
		want      string
		wantErr   bool
	}{
		{"chi_sim+eng", map[string]bool{"chi_sim": true, "eng": true}, "chi_sim+eng", false},
		{"chi_sim+fra", map[string]bool{"chi_sim": true, "eng": true}, "chi_sim", false},
		{"fra+jpn", map[string]bool{"chi_sim": true, "eng": true}, "eng", false}, // 全缺回退 eng
		{"fra", map[string]bool{}, "", true},                                    // 无 eng 可回退
		{"", map[string]bool{"eng": true}, "eng", false},
	}
	for _, c := range cases {
		got, err := resolveLangs(c.requested, c.available)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolveLangs(%q) 期望报错，得到 %q", c.requested, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("resolveLangs(%q) = %q, %v；期望 %q", c.requested, got, err, c.want)
		}
	}
}

// 门控端到端：本机有 tesseract 才跑（用内嵌字体画 "HELLO 12345" → OCR 回读）
func TestImageOcrEndToEnd(t *testing.T) {
	if findTesseract() == "" {
		t.Skip("本机未安装 tesseract，跳过 OCR 端到端测试")
	}
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 360, 90))
	for x := 0; x < 360; x++ {
		for y := 0; y < 90; y++ {
			img.Set(x, y, color.White)
		}
	}
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Black),
		Face: basicfont.Face7x13,
	}
	d.Dot = fixed.P(20, 50)
	d.DrawString("HELLO 12345")

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "text.jpg"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	art := dir + "_out"
	res, err := imageOcrRun(nil, map[string]any{"file": []string{"text.jpg"}, "lang": "eng", "psm": "7"},
		&core.ToolContext{Workdir: dir, ArtifactsDir: art})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("状态 %v, stderr=%s", res.Status, res.Stderr)
	}
	var out ocrOutput
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		t.Fatalf("stdout 不是合法 JSON: %v", err)
	}
	if out.Summary.Analyzed != 1 || out.Summary.WithText != 1 {
		t.Fatalf("summary = %+v", out.Summary)
	}
	if len(out.Images) != 1 {
		t.Fatalf("images = %d", len(out.Images))
	}
	text := strings.ToUpper(out.Images[0].Text)
	if !strings.Contains(text, "HELLO") || !strings.Contains(text, "12345") {
		t.Errorf("OCR 内容 = %q，应包含 HELLO 与 12345", out.Images[0].Text)
	}
	// 产物：报告 + 合并文本
	if _, err := os.Stat(filepath.Join(art, "ocr_report.json")); err != nil {
		t.Errorf("ocr_report.json 缺失: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(art, "ocr_texts.txt")); err != nil {
		t.Errorf("ocr_texts.txt 缺失: %v", err)
	} else if !strings.Contains(string(b), "HELLO") {
		t.Errorf("ocr_texts.txt 未含识别文字")
	}
}

// 嵌入语言包能正常解出到指定目录
func TestEnsureTessdata(t *testing.T) {
	dir := t.TempDir()
	if err := ensureTessdataIn(dir); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"chi_sim", "eng"} {
		st, err := os.Stat(filepath.Join(dir, lang+".traineddata"))
		if err != nil {
			t.Errorf("%s 未解出: %v", lang, err)
			continue
		}
		if st.Size() < 1000 {
			t.Errorf("%s.traineddata 过小（%d 字节）", lang, st.Size())
		}
	}
}

// 便携捆绑自检：项目根 thirdparty/tesseract 若存在，必须能用自带 tessdata 跑通
func TestBundledTesseractWorks(t *testing.T) {
	// 测试 cwd = internal/tools/data，仓库根 = ../../../
	bundle := filepath.Join("..", "..", "..", "thirdparty", "tesseract", "tesseract.exe")
	if _, err := os.Stat(bundle); err != nil {
		t.Skip("未找到捆绑 tesseract，跳过")
	}
	abs, err := filepath.Abs(bundle)
	if err != nil {
		t.Fatal(err)
	}
	tessdata := filepath.Join(filepath.Dir(abs), "tessdata")
	if err := ensureTessdataIn(tessdata); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(abs, "--list-langs")
	cmd.Env = append(os.Environ(), "TESSDATA_PREFIX="+tessdata)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("捆绑 tesseract 不可运行: %v", err)
	}
	s := string(out)
	for _, lang := range []string{"chi_sim", "eng"} {
		if !strings.Contains(s, lang) {
			t.Errorf("捆绑 tessdata 缺少语言 %s", lang)
		}
	}
}
