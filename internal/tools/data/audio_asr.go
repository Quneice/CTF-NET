// data.extract.audio-asr —— 语音转文字（ASR，whisper.cpp）。
//
// 上传音频文件（wav/mp3/flac/ogg，可多选或整个文件夹），多进程并行调用
// 项目自带的便携 whisper.cpp 引擎（thirdparty/whisper，随项目/随 exe 移植）
// 识别语音，输出文本与时间轴分段（可导出 SRT 字幕）。
// 中文结果默认经 OpenCC 短语级繁转简（t2s，参数可关闭）。
//
// 模型 ggml-*.bin 按需解析：优先引擎旁 models 目录（便于随项目移植）与
// 用户数据目录（%LOCALAPPDATA%\CTF-NET\models），都缺失时自动从 HF 镜像
// （hf-mirror.com，环境变量 WHISPER_MODEL_URL_BASE 可覆盖，WHISPER_NO_DOWNLOAD=1
// 可禁用自动下载）下载。档位：tiny-q5_1（内置，最快）/base-q5_1（内置，缺省，
// 快）/small（内置 f16，质量，中文建议）/medium/large-v3（按需下载）。
// 量化模型不自动下载（仓库量化文件与捆绑引擎不兼容），缺失时提示本地生成。
//
// stdout 输出 JSON（summary + 每文件文本与分段）供前端渲染器消费；产物目录
// 写出 asr_report.json（美化版）、asr_texts.txt（全部文字合并）与
// asr_all.srt（合并字幕，带时间轴）。
package data

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/longbridgeapp/opencc"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&AudioAsrTool{BaseTool: core.BaseTool{
		NameAttr:        "audio-asr",
		CategoryAttr:    core.Category("data.extract"),
		VersionAttr:     "1.0.0",
		DescriptionAttr: "语音转文字（ASR）：whisper.cpp，中英混排，含时间轴分段，支持批量与文件夹，多进程并行加速",
		TimeoutAttr:     21600,
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "音频文件（可多选或整个文件夹，递归收集 wav/mp3/flac/ogg）"},
			{Name: "lang", Type: "str", Required: false, Description: "识别语言（auto 自动检测 / zh 中文 / en 英文等），缺省 auto"},
			{Name: "model", Type: "str", Required: false, Description: "whisper 模型：base-q5_1（缺省，快）/ tiny-q5_1（最快）/ small（质量，中文建议）/ medium / large-v3"},
		{Name: "workers", Type: "int", Required: false, Default: 4,
			Description: "并行识别进程数（缺省 4，范围 1-16）：每个进程各占一份模型内存（small 约 466MB）并均分 CPU 线程，大批量文件提速约与进程数成正比；内存不足请调小"},
		{Name: "t2s", Type: "bool", Required: false, Default: true,
			Description: "中文识别结果自动繁转简（OpenCC t2s 短语级转换；whisper 中文输出常混繁体，仅对识别为中文的音频生效）"},
		{Name: "cn_num", Type: "bool", Required: false, Default: true,
			Description: "中文数字自动转阿拉伯数字（「幺三八…」→138…、「一百二十三」→123，便于正则匹配手机号/证件号；仅对识别为中文的音频生效）"},
		},
		Runner: audioAsrRun,
	}})
}

// AudioAsrTool 语音转文字工具。
type AudioAsrTool struct {
	core.BaseTool
}

// ---------------------------------------------------------------------------
// 引擎 / 模型探测
// ---------------------------------------------------------------------------

var (
	whisperPathOnce  sync.Once
	whisperPathFound string
)

// findWhisper 探测 whisper-cli 引擎位置（结果缓存）。
// 优先级：WHISPER_EXE 环境变量 → 可执行文件旁的便携捆绑（thirdparty/whisper）
// → 仓库根捆绑（build/bin 内运行时）→ CWD 捆绑 → PATH。
func findWhisper() string {
	whisperPathOnce.Do(func() {
		var candidates []string
		if p := os.Getenv("WHISPER_EXE"); p != "" {
			candidates = append(candidates, p)
		}
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			candidates = append(candidates,
				filepath.Join(exeDir, "thirdparty", "whisper", "whisper-cli.exe"),
				filepath.Join(exeDir, "..", "..", "thirdparty", "whisper", "whisper-cli.exe"),
			)
		}
		if cwd, err := os.Getwd(); err == nil {
			candidates = append(candidates, filepath.Join(cwd, "thirdparty", "whisper", "whisper-cli.exe"))
		}
		candidates = append(candidates, "whisper-cli")
		for _, c := range candidates {
			p, err := exec.LookPath(c)
			if err == nil {
				if st, e := os.Stat(p); e == nil && !st.IsDir() {
					whisperPathFound = p
					return
				}
				continue
			}
			if st, e := os.Stat(c); e == nil && !st.IsDir() {
				whisperPathFound = c
				return
			}
		}
	})
	return whisperPathFound
}

// whisperModelNames 支持的模型档位。
// tiny-q5_1/base-q5_1 为本地量化的 5-bit 模型（随项目内置，比 f16 快 2-4 倍、
// 体积小 60%+）；tiny/base 为 f16 兼容保留（e2e/历史参数）。
var whisperModelNames = map[string]bool{
	"tiny": true, "base": true, "small": true, "medium": true, "large-v3": true,
	"tiny-q5_1": true, "base-q5_1": true,
}

// userModelDir 用户数据目录下的模型目录（下载目标）。
func userModelDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "CTF-NET", "models")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// resolveModel 定位 ggml-{model}.bin：引擎旁 models → 用户数据目录 → 自动下载。
// 返回模型路径与备注（下载信息），或错误。onProgress 在下载期间回调
// （done/total 为字节数，total 未知时为 0），可为 nil。
func resolveModel(model, exeDir string, onProgress func(done, total int64)) (string, string, error) {
	if !whisperModelNames[model] {
		return "", "", fmt.Errorf("不支持的模型 %q（可选 tiny/base/small/medium/large-v3）", model)
	}
	name := "ggml-" + model + ".bin"
	var searchDirs []string
	if exeDir != "" {
		searchDirs = append(searchDirs, filepath.Join(exeDir, "models"))
	}
	if ud, err := userModelDir(); err == nil {
		searchDirs = append(searchDirs, ud)
	}
	for _, dir := range searchDirs {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 1<<20 {
			return p, "", nil
		}
	}
	// 量化模型不自动下载：HF 仓库的量化文件由新版转换脚本生成，与捆绑的
	// v1.7.6 引擎不兼容（加载报 failed to initialize whisper context）。
	// 项目已内置 tiny-q5_1/base-q5_1；缺失时提示用引擎同版本 quantize 工具本地生成。
	if strings.Contains(model, "q5_") {
		return "", "", fmt.Errorf("量化模型 %s 未找到：请放到 %s（量化模型不自动下载——仓库版本与捆绑引擎不兼容；可用 release 包内同版本 quantize.exe 从 f16 模型本地生成）",
			name, strings.Join(searchDirs, " / "))
	}
	if os.Getenv("WHISPER_NO_DOWNLOAD") == "1" {
		return "", "", fmt.Errorf("未找到模型 %s：请放到 %s，或去掉 WHISPER_NO_DOWNLOAD 允许自动下载", name, strings.Join(searchDirs, " / "))
	}
	ud, err := userModelDir()
	if err != nil {
		return "", "", err
	}
	dst := filepath.Join(ud, name)
	base := os.Getenv("WHISPER_MODEL_URL_BASE")
	if base == "" {
		base = "https://hf-mirror.com/ggerganov/whisper.cpp/resolve/main"
	}
	url := strings.TrimRight(base, "/") + "/" + name
	if err := downloadModel(url, dst, onProgress); err != nil {
		return "", "", fmt.Errorf("模型下载失败: %v（可手动下载 %s 放到 %s）", err, url, dst)
	}
	return dst, fmt.Sprintf("模型 %s 已自动下载到 %s", name, dst), nil
}

// countingReader 包装 io.Reader，累计已读字节数；每越过 1MB 边界
// 回调一次（参数为截止该边界的累计字节数）。
type countingReader struct {
	r    io.Reader
	cb   func(done int64)
	done int64 // 累计已读字节
	next int64 // 下次回调阈值（字节），初始 1MB
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.done += int64(n)
		if c.cb != nil {
			for c.done >= c.next {
				c.cb(c.next)
				c.next += 1 << 20
			}
		}
	}
	return n, err
}

// downloadModel 流式下载模型文件（1GB 上限，防止异常响应）；
// onProgress 每下载 1MB 回调一次（done/total 字节，total 未知为 0）。
func downloadModel(url, dst string, onProgress func(done, total int64)) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	cr := &countingReader{r: resp.Body, next: 1 << 20}
	if onProgress != nil {
		cr.cb = func(done int64) { onProgress(done, resp.ContentLength) }
	}
	n, err := io.Copy(f, io.LimitReader(cr, 1<<30))
	if err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if n < 1<<20 {
		os.Remove(tmp)
		return fmt.Errorf("文件过小（%d 字节），可能不是有效模型", n)
	}
	return os.Rename(tmp, dst)
}

// ---------------------------------------------------------------------------
// whisper 执行与 JSON 解析
// ---------------------------------------------------------------------------

// whisperJSON whisper-cli -oj 输出结构（只取需要的字段）。
type whisperJSON struct {
	Result struct {
		Language string `json:"language"`
	} `json:"result"`
	Transcription []struct {
		Timestamps struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"timestamps"`
		Offsets struct {
			From int `json:"from"`
			To   int `json:"to"`
		} `json:"offsets"`
		Text string `json:"text"`
	} `json:"transcription"`
}

// asrSegment 时间轴分段（秒）。
type asrSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// runASRFile 对单个音频文件执行识别，返回语言、分段与拼接全文。
// threads 为计算线程数（whisper 默认只开 4 线程，多核机器必须显式调大）。
func runASRFile(exe, model, path, lang string, threads int) (string, []asrSegment, string, error) {
	outBase := filepath.Join(os.TempDir(), fmt.Sprintf("ctfnet-asr-%d", time.Now().UnixNano()))
	defer os.Remove(outBase + ".json")
	cmd := exec.Command(exe, "-m", model, "-f", path, "-l", lang, "-t", strconv.Itoa(threads), "-oj", "-of", outBase, "-np")
	core.HideSubprocessWindow(cmd)
	if stderr, err := cmd.CombinedOutput(); err != nil {
		return "", nil, "", fmt.Errorf("whisper 执行失败: %v（%s）", err, strings.TrimSpace(string(stderr)))
	}
	b, err := os.ReadFile(outBase + ".json")
	if err != nil {
		return "", nil, "", fmt.Errorf("读取识别结果失败: %v", err)
	}
	var wj whisperJSON
	if err := json.Unmarshal(b, &wj); err != nil {
		return "", nil, "", fmt.Errorf("解析识别结果失败: %v", err)
	}
	langDetected := wj.Result.Language
	var segs []asrSegment
	var sb strings.Builder
	for _, t := range wj.Transcription {
		text := strings.TrimSpace(t.Text)
		if text == "" {
			continue
		}
		segs = append(segs, asrSegment{
			Start: float64(t.Offsets.From) / 1000,
			End:   float64(t.Offsets.To) / 1000,
			Text:  text,
		})
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	return langDetected, segs, strings.TrimRight(sb.String(), "\n"), nil
}

// ---------------------------------------------------------------------------
// 音频格式与时长
// ---------------------------------------------------------------------------

// asrAudioExts 支持直接识别音频格式（whisper-cli 自带解码）。
var asrAudioExts = map[string]bool{
	".wav": true, ".mp3": true, ".flac": true, ".ogg": true, ".oga": true,
}

// audioDurationSec 估算音频时长（秒）。wav 精确（按文件头），其他格式按末段结束时间。
func audioDurationSec(path string, lastSegEnd float64) float64 {
	if strings.ToLower(filepath.Ext(path)) != ".wav" {
		return lastSegEnd
	}
	f, err := os.Open(path)
	if err != nil {
		return lastSegEnd
	}
	defer f.Close()
	hdr := make([]byte, 44)
	if n, _ := io.ReadFull(f, hdr); n < 44 {
		return lastSegEnd
	}
	// 标准 PCM WAV：byteRate 在偏移 28，dataSize 在偏移 40
	byteRate := binary.LittleEndian.Uint32(hdr[28:32])
	dataSize := binary.LittleEndian.Uint32(hdr[40:44])
	if byteRate > 0 {
		return float64(dataSize) / float64(byteRate)
	}
	return lastSegEnd
}

// ---------------------------------------------------------------------------
// 工具主流程
// ---------------------------------------------------------------------------

type asrFileRecord struct {
	Name        string       `json:"name"`
	Format      string       `json:"format"`
	Lang        string       `json:"lang"`
	DurationSec float64      `json:"duration_sec"`
	Chars       int          `json:"chars"`
	Lines       int          `json:"lines"`
	Text        string       `json:"text"`
	Segments    []asrSegment `json:"segments"`
}

type asrSkippedFile struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type asrSummary struct {
	Total         int            `json:"total"`
	Analyzed      int            `json:"analyzed"`
	Skipped       int            `json:"skipped"`
	WithText      int            `json:"with_text"`
	Empty         int            `json:"empty"`
	TotalChars    int            `json:"total_chars"`
	TotalDuration float64        `json:"total_duration"`
	Formats       map[string]int `json:"formats"`
	Langs         map[string]int `json:"langs"`
}

// report 向任务进度通道上报（tc 或 ReportProgress 为 nil 时静默跳过）。
func report(tc *core.ToolContext, percent int, message string) {
	if tc != nil && tc.ReportProgress != nil {
		tc.ReportProgress(percent, message)
	}
}

// shortName 截断超长文件名（进度消息显示用，80 个字符）。
func shortName(s string) string {
	r := []rune(s)
	if len(r) > 80 {
		return string(r[:77]) + "…"
	}
	return s
}

// 中文繁转简（OpenCC t2s，惰性初始化；失败静默返回原文）。
// 短语级转换能区分歧义字（如「後」→「后」、「乾」按词分「干/乾」）。
var (
	t2sOnce sync.Once
	t2sConv *opencc.OpenCC
	t2sErr  error
)

func toSimplified(text string) string {
	t2sOnce.Do(func() {
		t2sConv, t2sErr = opencc.New("t2s")
	})
	if t2sErr != nil || t2sConv == nil {
		return text
	}
	out, err := t2sConv.Convert(text)
	if err != nil {
		return text
	}
	return out
}

// ---------------------------------------------------------------------------
// 中文数字 → 阿拉伯数字（whisper 常把数字输出成中文数字，影响正则匹配）
// ---------------------------------------------------------------------------

// cnDigit 中文数字字符（含口语读法 幺/两/〇）。
var cnDigit = map[rune]int{
	'零': 0, '〇': 0, '一': 1, '幺': 1, '二': 2, '两': 2, '三': 3, '四': 4,
	'五': 5, '六': 6, '七': 7, '八': 8, '九': 9,
}

// cnUnit 中文数位单位。
var cnUnit = map[rune]int{'十': 10, '百': 100, '千': 1000, '万': 10000, '亿': 100000000}

// cnNumToArabic 把文本中的中文数字段转成阿拉伯数字。
// 只转换长度 ≥2 的数字段：逐位读法（幺三八…→138…，手机号/证件号）或
// 含单位数词（一百二十三→123、三千零五十→3050、一百三→130）。
// 单个数字（一个/一定/第一）与常见歧义词（万一/千万小心/二两/两两）保持原样。
func cnNumToArabic(text string) string {
	runes := []rune(text)
	var b strings.Builder
	b.Grow(len(runes) + 8)
	for i := 0; i < len(runes); {
		if !isCnNumeral(runes[i]) {
			b.WriteRune(runes[i])
			i++
			continue
		}
		j := i
		for j < len(runes) && isCnNumeral(runes[j]) {
			j++
		}
		if s, ok := convertCnRun(runes[i:j]); ok {
			b.WriteString(s)
		} else {
			b.WriteString(string(runes[i:j]))
		}
		i = j
	}
	return b.String()
}

func isCnNumeral(c rune) bool {
	if _, ok := cnDigit[c]; ok {
		return true
	}
	_, ok := cnUnit[c]
	return ok
}

// convertCnRun 转换单个中文数字段；ok=false 表示保持原样。
func convertCnRun(run []rune) (string, bool) {
	n := len(run)
	if n < 2 {
		return "", false
	}
	hasUnit := false
	for _, c := range run {
		if _, ok := cnUnit[c]; ok {
			hasUnit = true
			break
		}
	}
	if n == 2 {
		s := string(run)
		if s == "万一" || s == "千万" {
			return "", false // 惯用词：万一（如果）/千万（务必），非数字
		}
		if run[0] == '两' {
			return "", false // 两两（配对）
		}
		if run[1] == '两' {
			return strconv.Itoa(cnDigit[run[0]]) + "两", true // 三两→3两（“两”是量词）
		}
		if !hasUnit {
			return strconv.Itoa(concatCnDigits(run)), true // 一二→12
		}
		return strconv.Itoa(parseCnValue(run)), true // 十二→12、二十→20、两百→200
	}
	if !hasUnit {
		return strconv.Itoa(concatCnDigits(run)), true // 逐位读法：一三八五…→1385…
	}
	return strconv.Itoa(parseCnValue(run)), true
}

// concatCnDigits 逐位拼接（含 零/幺）。
func concatCnDigits(run []rune) int {
	v := 0
	for _, c := range run {
		v = v*10 + cnDigit[c]
	}
	return v
}

// parseCnValue 解析含单位的数词：一百二十三→123、三千零五十→3050。
// 口语省略修正：结尾裸数字紧跟 百/千/万/亿 时按低位进位（一百三→130、
// 二百五→250、一万二→12000）。
func parseCnValue(run []rune) int {
	total, section, num := 0, 0, 0
	for _, c := range run {
		if d, ok := cnDigit[c]; ok {
			if d == 0 {
				num = 0 // 零 只占位，不带口语进位
				continue
			}
			num = d
			continue
		}
		u := cnUnit[c]
		switch c {
		case '万':
			section = (section + num) * 10000
			num = 0
		case '亿':
			total = (total + section + num) * 100000000
			section, num = 0, 0
		default: // 十百千
			if num == 0 {
				num = 1
			}
			section += num * u
			num = 0
		}
	}
	v := total + section + num
	if n := len(run); n >= 2 {
		last, prev := run[n-1], run[n-2]
		if d, ok := cnDigit[last]; ok && d > 0 {
			if _, ok := cnUnit[prev]; ok {
				switch prev {
				case '百':
					v += d * 9
				case '千':
					v += d * 99
				case '万':
					v += d * 999
				case '亿':
					v += d * 9999999
				}
			}
		}
	}
	return v
}

// runPool 以最多 workers 个并发 goroutine 对 [0,n) 执行 fn(i)。
// fn 需自行处理共享状态的并发安全；调度顺序不保证（按完成先后上报）。
func runPool(n, workers int, fn func(i int)) {
	if workers < 1 {
		workers = 1
	}
	if workers > n {
		workers = n
	}
	var wg sync.WaitGroup
	idx := make(chan int)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				fn(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		idx <- i
	}
	close(idx)
	wg.Wait()
}

type asrOutput struct {
	Summary      asrSummary       `json:"summary"`
	Files        []asrFileRecord  `json:"files"`
	SkippedFiles []asrSkippedFile `json:"skipped_files"`
	Notes        []string         `json:"notes,omitempty"`
}

func audioAsrRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	_ = ctx
	names, _ := asStringSliceLocal(params["file"])
	if len(names) == 0 {
		if s, ok := params["file"].(string); ok && s != "" {
			names = []string{s}
		}
	}
	if len(names) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少音频文件参数"}, nil
	}
	// 文件数量不做上限：批量场景（如数百上千条录音）按顺序逐文件处理，
	// 单个失败记入 skipped_files 不中断整体；仅限制单文件体积（200MB）。
	exe := findWhisper()
	if exe == "" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "未找到 whisper 引擎：项目应自带 thirdparty/whisper/whisper-cli.exe（随 exe 同目录或仓库根）；否则请用环境变量 WHISPER_EXE 指定"}, nil
	}
	lang := strings.TrimSpace(fmt.Sprint(params["lang"]))
	if lang == "" || lang == "<nil>" {
		lang = "auto"
	}
	model := strings.TrimSpace(fmt.Sprint(params["model"]))
	if model == "" || model == "<nil>" {
		model = "base-q5_1"
	}
	report(tc, 1, fmt.Sprintf("任务已启动：共 %d 个音频文件，准备模型 ggml-%s.bin…", len(names), model))
	modelPath, note, err := resolveModel(model, filepath.Dir(exe), func(done, total int64) {
		if total > 0 {
			report(tc, 1, fmt.Sprintf("首次使用：自动下载模型 ggml-%s.bin（%.0f%%，%dMB/%dMB）…", model,
				float64(done)/float64(total)*100, done>>20, total>>20))
		} else {
			report(tc, 1, fmt.Sprintf("首次使用：自动下载模型 ggml-%s.bin（已下载 %dMB）…", model, done>>20))
		}
	})
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	workers := pcInt(params, "workers", 4)
	if workers < 1 {
		workers = 1
	}
	t2s := true
	if v, ok := params["t2s"].(bool); ok {
		t2s = v
	}
	numToArabic := true
	if v, ok := params["cn_num"].(bool); ok {
		numToArabic = v
	}
	if workers > 16 {
		workers = 16
	}
	if workers > len(names) {
		workers = len(names)
	}
	// whisper-cli 默认只开 4 线程：多核机器按进程数均分，本机 32 逻辑核
	// 默认 4 进程 × 8 线程，实测单文件解码提速约 36%。
	threads := runtime.NumCPU() / workers
	if threads < 2 {
		threads = 2
	}
	report(tc, 1, fmt.Sprintf("模型就绪（%s），%d 进程并行识别（每进程 %d 线程）…", model, workers, threads))
	notes := []string{}
	if note != "" {
		notes = append(notes, note)
	}

	workdir := ""
	if tc != nil {
		workdir = tc.Workdir
	}
	const maxSize = 200 << 20

	out := asrOutput{
		Files:        make([]asrFileRecord, 0, len(names)),
		SkippedFiles: make([]asrSkippedFile, 0),
	}
	out.Summary.Formats = map[string]int{}
	out.Summary.Langs = map[string]int{}
	out.Summary.Total = len(names)

	// 按索引预分配结果槽位，worker 各自写入互不冲突；
	// 识别结果最后仍按文件名排序输出，顺序不受并发影响。
	recs := make([]*asrFileRecord, len(names))
	skips := make([]asrSkippedFile, len(names))
	var (
		doneMu  sync.Mutex
		doneCnt int
	)
	runPool(len(names), workers, func(i int) {
		n := names[i]
		defer func() {
			doneMu.Lock()
			doneCnt++
			done := doneCnt
			doneMu.Unlock()
			pct := done * 100 / len(names)
			if pct < 1 {
				pct = 1
			}
			if pct > 99 {
				pct = 99
			}
			report(tc, pct, fmt.Sprintf("已完成 %d/%d（%d 进程并行）：%s", done, len(names), workers, shortName(n)))
		}()
		path := filepath.Join(workdir, filepath.FromSlash(n))
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			skips[i] = asrSkippedFile{Name: n, Error: "文件不存在或为目录"}
			return
		}
		if st.Size() > maxSize {
			skips[i] = asrSkippedFile{Name: n, Error: "超过 200MB 上限"}
			return
		}
		format := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		if !asrAudioExts["."+format] {
			if format == "m4a" || format == "aac" {
				skips[i] = asrSkippedFile{Name: n, Error: "m4a/aac 暂不支持，请先转成 wav/mp3/flac/ogg"}
			} else {
				skips[i] = asrSkippedFile{Name: n, Error: fmt.Sprintf("不支持的音频格式 .%s", format)}
			}
			return
		}
		langDetected, segs, text, err := runASRFile(exe, modelPath, path, lang, threads)
		if err != nil {
			skips[i] = asrSkippedFile{Name: n, Error: err.Error()}
			return
		}
		// whisper 中文输出常混繁体：识别语言为中文且开启 t2s 时整段繁转简
		if t2s && langDetected == "zh" {
			text = toSimplified(text)
			for j := range segs {
				segs[j].Text = toSimplified(segs[j].Text)
			}
		}
		// 中文数字转阿拉伯数字（whisper 常输出中文数字，导致正则无法匹配）
		if numToArabic && langDetected == "zh" {
			text = cnNumToArabic(text)
			for j := range segs {
				segs[j].Text = cnNumToArabic(segs[j].Text)
			}
		}
		rec := &asrFileRecord{Name: n, Format: format, Lang: langDetected, Text: text, Segments: segs}
		if len(segs) > 0 {
			rec.DurationSec = audioDurationSec(path, segs[len(segs)-1].End)
		}
		rec.Chars = len([]rune(text))
		if text != "" {
			rec.Lines = len(strings.Split(text, "\n"))
		}
		recs[i] = rec
	})
	for _, rec := range recs {
		if rec == nil {
			continue
		}
		if rec.Text != "" {
			out.Summary.WithText++
		} else {
			out.Summary.Empty++
		}
		out.Summary.TotalChars += rec.Chars
		out.Summary.TotalDuration += rec.DurationSec
		out.Summary.Formats[rec.Format]++
		if rec.Lang != "" {
			out.Summary.Langs[rec.Lang]++
		}
		out.Files = append(out.Files, *rec)
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Name < out.Files[j].Name })
	out.SkippedFiles = make([]asrSkippedFile, 0, len(names))
	for _, sk := range skips {
		if sk.Name != "" {
			out.SkippedFiles = append(out.SkippedFiles, sk)
		}
	}
	out.Summary.Analyzed = len(out.Files)
	out.Summary.Skipped = len(out.SkippedFiles)
	report(tc, 100, fmt.Sprintf("识别完成：成功 %d 个，跳过 %d 个", out.Summary.Analyzed, out.Summary.Skipped))

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
		if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "asr_report.json"), pretty, 0o644); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出报告失败: %v", err)}, nil
		}
		// 合并全文（带文件分隔标记）
		var tb bytes.Buffer
		for _, rec := range out.Files {
			if rec.Text == "" {
				continue
			}
			fmt.Fprintf(&tb, "===== %s (%s) =====\n%s\n\n", rec.Name, rec.Lang, rec.Text)
		}
		if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "asr_texts.txt"), tb.Bytes(), 0o644); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出合并文本失败: %v", err)}, nil
		}
		// 合并 SRT 字幕（跨文件重新编号）
		var sb bytes.Buffer
		idx := 1
		for _, rec := range out.Files {
			for _, seg := range rec.Segments {
				fmt.Fprintf(&sb, "%d\n%s --> %s\n%s\n\n",
					idx, srtTime(seg.Start), srtTime(seg.End), seg.Text)
				idx++
			}
		}
		if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "asr_all.srt"), sb.Bytes(), 0o644); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出字幕失败: %v", err)}, nil
		}
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: string(compact)}, nil
}

// srtTime 秒 → SRT 时间戳 "HH:MM:SS,mmm"。
func srtTime(sec float64) string {
	ms := int(sec*1000 + 0.5)
	h := ms / 3600000
	m := (ms % 3600000) / 60000
	s := (ms % 60000) / 1000
	mm := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, mm)
}
