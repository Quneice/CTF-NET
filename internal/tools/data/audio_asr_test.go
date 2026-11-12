package data

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ctfnet/internal/core"
)

// buildWav16k 构造 16kHz 单声道 16bit PCM WAV（pcm 为采样数据）。
func buildWav16k(pcm []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+len(pcm)))
	buf.WriteString("WAVEfmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // mono
	binary.Write(&buf, binary.LittleEndian, uint32(16000))
	binary.Write(&buf, binary.LittleEndian, uint32(32000)) // byteRate
	binary.Write(&buf, binary.LittleEndian, uint16(2))     // blockAlign
	binary.Write(&buf, binary.LittleEndian, uint16(16))    // bits
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(len(pcm)))
	buf.Write(pcm)
	return buf.Bytes()
}

func TestSrtTime(t *testing.T) {
	cases := []struct {
		sec  float64
		want string
	}{
		{0, "00:00:00,000"},
		{1.5, "00:00:01,500"},
		{61.25, "00:01:01,250"},
		{3661.999, "01:01:01,999"},
	}
	for _, c := range cases {
		if got := srtTime(c.sec); got != c.want {
			t.Errorf("srtTime(%v) = %s，期望 %s", c.sec, got, c.want)
		}
	}
}

// 用真实 whisper-cli 输出样本校验 JSON 结构体映射
func TestParseWhisperJSON(t *testing.T) {
	sample := `{
		"result": {"language": "en"},
		"transcription": [
			{"timestamps": {"from": "00:00:00,000", "to": "00:00:02,640"},
			 "offsets": {"from": 0, "to": 2640}, "text": " Hello world"},
			{"timestamps": {"from": "00:00:03,000", "to": "00:00:05,000"},
			 "offsets": {"from": 3000, "to": 5000}, "text": " second segment"}
		]
	}`
	var wj whisperJSON
	if err := json.Unmarshal([]byte(sample), &wj); err != nil {
		t.Fatal(err)
	}
	if wj.Result.Language != "en" {
		t.Errorf("language = %q", wj.Result.Language)
	}
	if len(wj.Transcription) != 2 || wj.Transcription[1].Offsets.From != 3000 {
		t.Errorf("transcription = %+v", wj.Transcription)
	}
}

func TestAudioDurationWav(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "t.wav")
	// 64000 字节 PCM @ 32000 B/s = 2 秒
	if err := os.WriteFile(p, buildWav16k(make([]byte, 64000)), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := audioDurationSec(p, 0); got < 1.99 || got > 2.01 {
		t.Errorf("wav 时长 = %v，期望约 2.0", got)
	}
	// 非 wav 回退到末段结束时间
	if got := audioDurationSec(filepath.Join(dir, "t.mp3"), 3.7); got != 3.7 {
		t.Errorf("mp3 时长回退 = %v", got)
	}
}

func TestResolveModelValidation(t *testing.T) {
	if _, _, err := resolveModel("huge", "", nil); err == nil {
		t.Error("非法模型名应报错")
	}
	for _, m := range []string{"tiny-q5_1", "base-q5_1"} {
		if !whisperModelNames[m] {
			t.Errorf("量化模型 %s 未在支持列表中", m)
		}
	}
}

// 量化模型缺失时不自动下载（仓库量化文件与捆绑引擎不兼容），应报错而非联网。
func TestResolveModelQuantizedNoDownload(t *testing.T) {
	t.Setenv("WHISPER_NO_DOWNLOAD", "")
	_, _, err := resolveModel("base-q5_1", t.TempDir(), nil)
	if err == nil {
		t.Fatal("量化模型缺失时应报错")
	}
	if !strings.Contains(err.Error(), "q5") {
		t.Errorf("错误信息应提到量化模型不自动下载: %v", err)
	}
}

// OpenCC 繁转简：短语级转换（後→后），非中文原样保留
func TestToSimplified(t *testing.T) {
	got := toSimplified("語音識別結果顯示繁體字，後天開會做記錄")
	if got != "语音识别结果显示繁体字，后天开会做记录" {
		t.Errorf("繁转简结果: %q", got)
	}
	if got := toSimplified("Hello 世界"); got != "Hello 世界" {
		t.Errorf("非繁体文本不应变化: %q", got)
	}
}

// 中文数字转阿拉伯：逐位读法、单位数词、口语省略；惯用词保护
func TestCnNumToArabic(t *testing.T) {
	cases := []struct{ in, want string }{
		{"电话是幺三八零一二三四五六七", "电话是13801234567"},
		{"一共一百二十三块钱", "一共123块钱"},
		{"三百二十元", "320元"},
		{"一百三", "130"},
		{"二百五", "250"},
		{"一千零二", "1002"},
		{"三千零五十", "3050"},
		{"二零二六年", "2026年"},
		{"第十二个", "第12个"},
		{"十五号", "15号"},
		{"二十几", "20几"},
		{"三千万", "30000000"},
		{"一万二", "12000"},
		{"三两白酒", "3两白酒"},
		// 惯用词/单字不转
		{"万一失败了", "万一失败了"},
		{"千万要小心", "千万要小心"},
		{"十分感谢", "十分感谢"},
		{"一点意思", "一点意思"},
		{"一个", "一个"},
		{"统一", "统一"},
		// 非中文数字文本不受影响
		{"Hello world 123", "Hello world 123"},
	}
	for _, c := range cases {
		if got := cnNumToArabic(c.in); got != c.want {
			t.Errorf("cnNumToArabic(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

// runPool：并发数不超过 workers、所有索引恰好执行一次
func TestRunPool(t *testing.T) {
	for _, workers := range []int{1, 3, 8} {
		const n = 25
		var mu sync.Mutex
		var active, maxActive int
		seen := make([]bool, n)
		runPool(n, workers, func(i int) {
			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			if seen[i] {
				t.Errorf("workers=%d 索引 %d 重复执行", workers, i)
			}
			seen[i] = true
			mu.Unlock()
			time.Sleep(time.Millisecond)
			mu.Lock()
			active--
			mu.Unlock()
		})
		if maxActive > workers || maxActive == 0 {
			t.Errorf("workers=%d 最大并发 %d 超出/为零", workers, maxActive)
		}
		for i, s := range seen {
			if !s {
				t.Errorf("workers=%d 索引 %d 未执行", workers, i)
			}
		}
	}
}

// countingReader：每越过 1MB 边界回调一次，且首字节不误报
func TestCountingReader(t *testing.T) {
	var calls []int64
	cr := &countingReader{
		r:    bytes.NewReader(make([]byte, 3<<20+512<<10)), // 3.5MB
		cb:   func(done int64) { calls = append(calls, done) },
		next: 1 << 20,
	}
	buf := make([]byte, 512<<10)
	for {
		if _, err := cr.Read(buf); err != nil {
			break
		}
	}
	if len(calls) != 3 || calls[0] != 1<<20 || calls[1] != 2<<20 || calls[2] != 3<<20 {
		t.Fatalf("回调边界错误: %v", calls)
	}
}

// 门控端到端：本机有 whisper 引擎与 tiny 系模型才跑（1 秒静音 wav 走完整管线）
func TestAudioAsrEndToEnd(t *testing.T) {
	exe := findWhisper()
	if exe == "" {
		t.Skip("本机未找到 whisper 引擎，跳过 ASR 端到端测试")
	}
	// 只查已有模型，绝不触发下载；优先内置的量化 tiny-q5_1
	modelName, found := "", ""
	for _, dir := range []string{
		filepath.Join(filepath.Dir(exe), "models"),
		filepath.Join(os.TempDir(), "CTF-NET", "models"),
	} {
		for _, cand := range []string{"ggml-tiny-q5_1.bin", "ggml-tiny.bin"} {
			p := filepath.Join(dir, cand)
			if st, err := os.Stat(p); err == nil && st.Size() > 1<<20 {
				found, modelName = p, strings.TrimSuffix(strings.TrimPrefix(cand, "ggml-"), ".bin")
				break
			}
		}
		if found != "" {
			break
		}
	}
	if os.Getenv("LOCALAPPDATA") != "" && found == "" {
		for _, cand := range []string{"ggml-tiny-q5_1.bin", "ggml-tiny.bin"} {
			p := filepath.Join(os.Getenv("LOCALAPPDATA"), "CTF-NET", "models", cand)
			if st, err := os.Stat(p); err == nil && st.Size() > 1<<20 {
				found, modelName = p, strings.TrimSuffix(strings.TrimPrefix(cand, "ggml-"), ".bin")
				break
			}
		}
	}
	if found == "" {
		t.Skip("未找到 tiny 系模型，跳过 ASR 端到端测试")
	}
	dir := t.TempDir()
	wav := filepath.Join(dir, "silence.wav")
	if err := os.WriteFile(wav, buildWav16k(make([]byte, 32000)), 0o644); err != nil { // 1 秒静音
		t.Fatal(err)
	}
	art := dir + "_out"
	var progs []struct {
		Pct int
		Msg string
	}
	tc := &core.ToolContext{
		Workdir: dir, ArtifactsDir: art,
		ReportProgress: func(pct int, msg string) {
			progs = append(progs, struct {
				Pct int
				Msg string
			}{pct, msg})
		},
	}
	res, err := audioAsrRun(nil, map[string]any{"file": []string{"silence.wav"}, "lang": "en", "model": modelName}, tc)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("状态 %v, stderr=%s", res.Status, res.Stderr)
	}
	var out asrOutput
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		t.Fatalf("stdout 不是合法 JSON: %v", err)
	}
	if out.Summary.Analyzed != 1 || out.Summary.Skipped != 0 {
		t.Fatalf("summary = %+v", out.Summary)
	}
	for _, name := range []string{"asr_report.json", "asr_texts.txt", "asr_all.srt"} {
		if _, err := os.Stat(filepath.Join(art, name)); err != nil {
			t.Errorf("%s 缺失: %v", name, err)
		}
	}
	// 进度上报：至少包含启动/识别中/完成，百分比单调不减且终值 100
	if len(progs) < 3 {
		t.Fatalf("进度上报过少: %+v", progs)
	}
	last := -1
	for _, p := range progs {
		if p.Pct < last {
			t.Fatalf("进度回退: %+v", progs)
		}
		last = p.Pct
	}
	if progs[len(progs)-1].Pct != 100 {
		t.Fatalf("最终进度应为 100: %+v", progs)
	}
}
