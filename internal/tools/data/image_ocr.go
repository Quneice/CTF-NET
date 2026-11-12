// data.extract.image-ocr —— 图片文字提取（OCR）。
//
// 上传单张/多张图片（可整个文件夹），逐张调用外部 Tesseract 引擎识别文字，
// 输出结构化 JSON（stdout）供前端渲染器消费，并在产物目录写出
// ocr_report.json（美化版）与 ocr_texts.txt（全部文字合并版，可直接下载）。
// 中文（chi_sim）与英文（eng）语言包（tessdata_fast）go:embed 进二进制，
// 引擎优先使用项目自带的便携捆绑（thirdparty/tesseract，含全部依赖 DLL 与
// tessdata，随项目/随 exe 移植），语言包缺失时自动解出到捆绑 tessdata 目录。
package data

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&ImageOcrTool{BaseTool: core.BaseTool{
		NameAttr:        "image-ocr",
		CategoryAttr:    core.Category("data.extract"),
		VersionAttr:     "1.0.0",
		DescriptionAttr: "OCR 提取图片中的文字（Tesseract，中英混排，支持批量与文件夹）",
		TimeoutAttr:     600,
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "图片文件（可多选或整个文件夹，递归收集全部图片）"},
			{Name: "lang", Type: "text", Required: false, Description: "OCR 语言（+ 组合，如 chi_sim+eng），缺省 chi_sim+eng"},
			{Name: "psm", Type: "str", Required: false, Description: "版面分析模式（3 自动 / 6 单个文本块 / 11 稀疏文本 / 12 稀疏+方向）"},
		},
		Runner: imageOcrRun,
	}})
}

// ImageOcrTool 图片文字提取工具。
type ImageOcrTool struct {
	core.BaseTool
}

//go:embed tessdata/chi_sim.traineddata
//go:embed tessdata/eng.traineddata
var embeddedTessdata embed.FS

var embeddedLangs = []string{"chi_sim", "eng"}

// ---------------------------------------------------------------------------
// Tesseract 探测 / 语言包管理
// ---------------------------------------------------------------------------

// findTesseract 探测 tesseract 引擎位置（结果缓存）。
// 优先级：TESSERACT_EXE 环境变量 → 可执行文件旁的便携捆绑（thirdparty/tesseract）
// → 仓库根捆绑（build/bin 内运行时）→ CWD 捆绑 → PATH → 常见安装目录。
var (
	tessPathOnce  sync.Once
	tessPathFound string
)

func findTesseract() string {
	tessPathOnce.Do(func() {
		var candidates []string
		if p := os.Getenv("TESSERACT_EXE"); p != "" {
			candidates = append(candidates, p)
		}
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			candidates = append(candidates,
				filepath.Join(exeDir, "thirdparty", "tesseract", "tesseract.exe"),
				filepath.Join(exeDir, "..", "..", "thirdparty", "tesseract", "tesseract.exe"),
			)
		}
		if cwd, err := os.Getwd(); err == nil {
			candidates = append(candidates, filepath.Join(cwd, "thirdparty", "tesseract", "tesseract.exe"))
		}
		candidates = append(candidates,
			"tesseract",
			`D:\Tesseract\tesseract.exe`,
			`C:\Program Files\Tesseract-OCR\tesseract.exe`,
			`C:\Program Files (x86)\Tesseract-OCR\tesseract.exe`,
		)
		for _, c := range candidates {
			p, err := exec.LookPath(c)
			if err == nil {
				if st, e := os.Stat(p); e == nil && !st.IsDir() {
					tessPathFound = p
					return
				}
				continue
			}
			if st, e := os.Stat(c); e == nil && !st.IsDir() {
				tessPathFound = c
				return
			}
		}
	})
	return tessPathFound
}

// ensureTessdataIn 把嵌入的语言包解出到指定目录（缺失时补齐，自愈）。
func ensureTessdataIn(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, lang := range embeddedLangs {
		name := lang + ".traineddata"
		dst := filepath.Join(dir, name)
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		b, err := embeddedTessdata.ReadFile("tessdata/" + name)
		if err != nil {
			return fmt.Errorf("读取内嵌语言包 %s 失败: %v", name, err)
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return fmt.Errorf("解出语言包 %s 失败: %v", name, err)
		}
	}
	return nil
}

// listTesseractLangs 列出指定 tessdata 前缀下可用的语言（结果缓存）。
var (
	langsOnce   sync.Once
	langsList   map[string]bool
	langsErr    error
)

func listTesseractLangs(prefix string) (map[string]bool, error) {
	langsOnce.Do(func() {
		exe := findTesseract()
		if exe == "" {
			langsErr = fmt.Errorf("未找到 tesseract 引擎")
			return
		}
		cmd := exec.Command(exe, "--list-langs")
		core.HideSubprocessWindow(cmd)
		cmd.Env = append(os.Environ(), "TESSDATA_PREFIX="+prefix)
		out, err := cmd.Output()
		if err != nil {
			langsErr = fmt.Errorf("tesseract --list-langs 失败: %v", err)
			return
		}
		langsList = map[string]bool{}
		for _, line := range strings.Split(string(out), "\n") {
			l := strings.TrimSpace(line)
			if l != "" {
				langsList[l] = true
			}
		}
	})
	return langsList, langsErr
}

// resolveLangs 把请求的语言组合与可用语言求交集；全缺时回退 eng。
func resolveLangs(requested string, available map[string]bool) (string, error) {
	var kept []string
	for _, l := range strings.Split(requested, "+") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if available[l] {
			kept = append(kept, l)
		}
	}
	if len(kept) > 0 {
		return strings.Join(kept, "+"), nil
	}
	if available["eng"] {
		return "eng", nil
	}
	return "", fmt.Errorf("请求的语言（%s）均不可用，且无 eng 回退", requested)
}

// ---------------------------------------------------------------------------
// OCR 执行
// ---------------------------------------------------------------------------

// tesseractExts 可直接交给 Tesseract 按扩展名识别的格式。
var tesseractExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".bmp": true,
	".tif": true, ".tiff": true, ".webp": true, ".gif": true,
}

const maxOCRTextBytes = 500 << 10 // 单图文字上限，防异常输出撑爆报告

// runOCRFile 对单个图片文件执行 OCR，返回提取的文字。
// 扩展名不在 Tesseract 支持列表时，先统一重编码为 PNG 临时文件。
func runOCRFile(exe, tessdata, path, langs, psm string) (string, error) {
	input := path
	var tmp string
	if !tesseractExts[strings.ToLower(filepath.Ext(path))] {
		img, _, err := decodeImagePixels(readFileBytes(path))
		if err != nil {
			return "", fmt.Errorf("图片解码失败: %v", err)
		}
		f, err := os.CreateTemp("", "ctfnet-ocr-*.png")
		if err != nil {
			return "", fmt.Errorf("创建临时文件失败: %v", err)
		}
		tmp = f.Name()
		defer os.Remove(tmp)
		if err := png.Encode(f, img); err != nil {
			f.Close()
			return "", fmt.Errorf("PNG 重编码失败: %v", err)
		}
		f.Close()
		input = tmp
	}
	outBase := input + ".out"
	cmd := exec.Command(exe, input, outBase, "-l", langs, "--psm", psm)
	core.HideSubprocessWindow(cmd)
	cmd.Env = append(os.Environ(), "TESSDATA_PREFIX="+tessdata)
	if stderr, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("tesseract 执行失败: %v（%s）", err, strings.TrimSpace(string(stderr)))
	}
	b, err := os.ReadFile(outBase + ".txt")
	if err != nil {
		return "", fmt.Errorf("读取 OCR 结果失败: %v", err)
	}
	os.Remove(outBase + ".txt")
	text := strings.TrimPrefix(string(b), "\uFEFF") // 去 BOM
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if len(text) > maxOCRTextBytes {
		text = text[:maxOCRTextBytes]
	}
	return text, nil
}

func readFileBytes(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return b
}

// ---------------------------------------------------------------------------
// 工具主流程
// ---------------------------------------------------------------------------

// ocrFileRecord 单张图片的 OCR 记录。
type ocrFileRecord struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Chars  int    `json:"chars"`
	Lines  int    `json:"lines"`
	Text   string `json:"text"`
}

type ocrSkippedFile struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type ocrOutput struct {
	Summary      ocrSummary        `json:"summary"`
	Images       []ocrFileRecord   `json:"images"`
	SkippedFiles []ocrSkippedFile  `json:"skipped_files"`
}

type ocrSummary struct {
	Total      int            `json:"total"`
	Analyzed   int            `json:"analyzed"`
	Skipped    int            `json:"skipped"`
	WithText   int            `json:"with_text"`
	Empty      int            `json:"empty"`
	TotalChars int            `json:"total_chars"`
	Formats    map[string]int `json:"formats"`
}

func imageOcrRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	_ = ctx
	names, _ := asStringSliceLocal(params["file"])
	if len(names) == 0 {
		if s, ok := params["file"].(string); ok && s != "" {
			names = []string{s}
		}
	}
	if len(names) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少图片文件参数"}, nil
	}
	if len(names) > 500 {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("文件数量过多（%d，上限 500）", len(names))}, nil
	}
	exe := findTesseract()
	if exe == "" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "未找到 Tesseract 引擎：项目应自带 thirdparty/tesseract/tesseract.exe（随 exe 同目录或仓库根）；否则请安装 tesseract-ocr 或用环境变量 TESSERACT_EXE 指定"}, nil
	}
	// tessdata 随引擎同目录（捆绑自包含）；缺失的语言包由内嵌数据自愈补齐
	tessdata := filepath.Join(filepath.Dir(exe), "tessdata")
	if err := ensureTessdataIn(tessdata); err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	available, err := listTesseractLangs(tessdata)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	lang := strings.TrimSpace(fmt.Sprint(params["lang"]))
	if lang == "" || lang == "<nil>" {
		lang = "chi_sim+eng"
	}
	lang, err = resolveLangs(lang, available)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	psm := strings.TrimSpace(fmt.Sprint(params["psm"]))
	if psm == "" || psm == "<nil>" {
		psm = "3"
	}
	if n, err := strconv.Atoi(psm); err != nil || n < 0 || n > 13 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "psm 必须是 0~13 的整数"}, nil
	}

	workdir := ""
	if tc != nil {
		workdir = tc.Workdir
	}
	const maxSize = 100 << 20

	out := ocrOutput{
		Images:       make([]ocrFileRecord, 0, len(names)),
		SkippedFiles: make([]ocrSkippedFile, 0),
	}
	out.Summary.Formats = map[string]int{}
	out.Summary.Total = len(names)

	for _, n := range names {
		path := filepath.Join(workdir, filepath.FromSlash(n))
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			out.SkippedFiles = append(out.SkippedFiles, ocrSkippedFile{n, "文件不存在或为目录"})
			continue
		}
		if st.Size() > maxSize {
			out.SkippedFiles = append(out.SkippedFiles, ocrSkippedFile{n, "超过 100MB 上限"})
			continue
		}
		format, _, _, _ := sniffImageFormat(readFileBytes(path))
		if format == "未知" {
			out.SkippedFiles = append(out.SkippedFiles, ocrSkippedFile{n, "无法识别的图片格式"})
			continue
		}
		text, err := runOCRFile(exe, tessdata, path, lang, psm)
		if err != nil {
			out.SkippedFiles = append(out.SkippedFiles, ocrSkippedFile{n, err.Error()})
			continue
		}
		rec := ocrFileRecord{Name: n, Format: format, Text: text}
		rec.Chars = len([]rune(text))
		if text != "" {
			rec.Lines = len(strings.Split(text, "\n"))
			out.Summary.WithText++
		} else {
			out.Summary.Empty++
		}
		out.Summary.TotalChars += rec.Chars
		out.Summary.Formats[format]++
		out.Images = append(out.Images, rec)
	}
	sort.Slice(out.Images, func(i, j int) bool { return out.Images[i].Name < out.Images[j].Name })
	out.Summary.Analyzed = len(out.Images)
	out.Summary.Skipped = len(out.SkippedFiles)

	pretty, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "序列化结果失败"}, nil
	}
	compact, err := json.Marshal(out)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "序列化结果失败"}, nil
	}
	if tc != nil && tc.ArtifactsDir != "" {
		if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("创建产物目录失败: %v", err)}, nil
		}
		if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "ocr_report.json"), pretty, 0o644); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出报告失败: %v", err)}, nil
		}
		// 合并版纯文本（带文件分隔标记），便于直接下载全文检索
		var sb bytes.Buffer
		for _, rec := range out.Images {
			if rec.Text == "" {
				continue
			}
			fmt.Fprintf(&sb, "===== %s =====\n%s\n\n", rec.Name, rec.Text)
		}
		if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "ocr_texts.txt"), sb.Bytes(), 0o644); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出合并文本失败: %v", err)}, nil
		}
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: string(compact)}, nil
}
