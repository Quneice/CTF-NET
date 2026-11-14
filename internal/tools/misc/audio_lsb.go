// misc.audio_lsb —— WAV LSB 音频隐写分析(位平面提取 + 隐藏内容类型自动识别)。
// 套路来源:2026 Bugku hhh.wav(16bit 单声道 bit0 MSB-first 藏 JPEG,EOI 截断)。
// 支持:PCM 8/16/24/32bit 与 IEEE float 32/64bit、多声道(全交错/指定声道)、
// 位平面 0-N、每采样 1-2 个低位、MSB/LSB 两种打包位序;auto 模式扫描全部组合,
// 按魔数/文本启发式/flag 正则识别(共享 internal/stego 引擎);JPEG 按 EOI、PNG 按
// IEND 智能截断;图片命中内联 base64 预览。
package misc

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ctfnet/internal/core"
	"ctfnet/internal/stego"
)

func init() {
	core.DefaultRegistry.Register(&AudioLSBTool{BaseTool: core.BaseTool{
		NameAttr:        "audio_lsb",
		CategoryAttr:    core.CatMisc,
		VersionAttr:     "1.2.0",
		DescriptionAttr: "WAV 音频隐写一键侦察:chunk/尾部附加数据侦察 + 原始字节 flag 扫描 + LSB 位平面提取(类型自动识别)+ 频谱图渲染",
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "上传的 WAV 文件 file_id"},
			{Name: "plane", Type: "str", Required: false, Default: "auto",
				Description: "位平面:auto | 0 | 1 | 2 | 3(auto 扫描 0-3)"},
			{Name: "order", Type: "str", Required: false, Default: "auto",
				Description: "打包位序:auto | msb | lsb(首位→字节最高/最低位)"},
			{Name: "channel", Type: "str", Required: false, Default: "auto",
				Description: "声道:auto | all | 0 | 1 ...(all=按文件序全部采样;N=仅第 N 声道)"},
			{Name: "bits", Type: "str", Required: false, Default: "1",
				Description: "每采样取几个低位:1 | 2(2 时按位号升序取 plane、plane+1)"},
			{Name: "spectrogram", Type: "str", Required: false, Default: "full",
				Description: "频谱图:full 全频段 | low 0-2kHz(低频亮带) | ultra 18kHz+(超声波隐写) | off 关闭"},
			{Name: "nperseg", Type: "str", Required: false, Default: "2048",
				Description: "频谱窗长:2048 | 4096 | 16384(越大频率分辨率越高,时间分辨率越低)"},
		},
		Runner: audioLSBRun,
	}})
}

// AudioLSBTool WAV LSB 隐写工具。
type AudioLSBTool struct {
	core.BaseTool
}

// ---------- WAV 解析 ----------

type wavInfo struct {
	audioFormat uint16 // 1=PCM 3=IEEE float
	formatName  string
	channels    int
	sampleRate  int
	bitsPer     int
	bytesPer    int
	sampleCount int // 采样总数(含各声道交错)
	durationSec float64
	samplesLE   []byte // data 块原始字节(小端)
	samplesMask uint64 // 位深非整字节时截断
	Chunks      []WavChunkMeta // P0 侦察:chunk 清单
	Trailing    []byte         // P0 侦察:最后一个有效 chunk 之后/RIFF 声明范围之外的附加数据
}

// WavChunkMeta RIFF chunk 元数据(侦察报告用)。
type WavChunkMeta struct {
	ID     string `json:"id"`
	Size   int    `json:"size"`
	Offset int    `json:"offset"`
}

// validChunkID chunk id 须为 4 个可打印 ASCII(拦截把尾部垃圾误判成 chunk)。
func validChunkID(id string) bool {
	for _, c := range []byte(id) {
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return len(id) == 4
}

// parseWav 解析 RIFF/WAVE:遍历 chunk 定位 fmt 与 data;仅接受未压缩 PCM/IEEE float。
// P0:记录 chunk 清单与尾部附加数据(data 之后拼接的隐藏文件)。
func parseWav(data []byte, fileSize int64, name string) (*wavInfo, error) {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("不是 RIFF/WAVE 文件(魔数校验失败):%s", name)
	}
	var fmtBuf, dataBuf []byte
	var chunkList []WavChunkMeta
	declaredEnd := 8 + int(binary.LittleEndian.Uint32(data[4:8])) // RIFF ChunkSize 声明的 WAVE 范围
	if declaredEnd > len(data) {
		declaredEnd = len(data)
	}
	endOfChunks := declaredEnd
	pos := 12
	for pos+8 <= len(data) && pos < declaredEnd {
		id := string(data[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		body := pos + 8
		if !validChunkID(id) || body+size > len(data) {
			break // 垃圾/越界块:剩余全部视为尾部附加数据
		}
		chunkList = append(chunkList, WavChunkMeta{ID: id, Size: size, Offset: pos})
		switch id {
		case "fmt ":
			fmtBuf = data[body : body+size]
		case "data":
			if declared := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8])); declared > len(data)-body {
				return nil, fmt.Errorf("data 块声明大小(%dB)超过文件实际剩余(%dB),文件可能损坏或为流式截断", declared, len(data)-body)
			}
			dataBuf = data[body : body+size]
		}
		endOfChunks = body + size + (size & 1) // chunk 按 2 字节对齐
		pos = endOfChunks
	}
	if len(fmtBuf) < 16 {
		return nil, fmt.Errorf("缺少/损坏 fmt 块")
	}
	if dataBuf == nil {
		return nil, fmt.Errorf("缺少 data 块")
	}
	w := &wavInfo{}
	fmtCode := binary.LittleEndian.Uint16(fmtBuf[0:2])
	if fmtCode == 0xFFFE && len(fmtBuf) >= 40 { // WAVE_FORMAT_EXTENSIBLE:SubFormat 前 2 字节
		fmtCode = binary.LittleEndian.Uint16(fmtBuf[24:26])
	}
	switch fmtCode {
	case 1:
		w.audioFormat, w.formatName = 1, "pcm"
	case 3:
		w.audioFormat, w.formatName = 3, "float"
	default:
		return nil, fmt.Errorf("仅支持未压缩 PCM/IEEE float WAV(当前 AudioFormat=%d),请先用 ffmpeg 等转码", fmtCode)
	}
	w.channels = int(binary.LittleEndian.Uint16(fmtBuf[2:4]))
	w.sampleRate = int(binary.LittleEndian.Uint32(fmtBuf[4:8]))
	w.bitsPer = int(binary.LittleEndian.Uint16(fmtBuf[14:16]))
	if w.channels < 1 || w.sampleRate < 1 {
		return nil, fmt.Errorf("fmt 块声道/采样率非法(channels=%d rate=%d)", w.channels, w.sampleRate)
	}
	if w.bitsPer < 8 || w.bitsPer > 64 || w.audioFormat == 1 && w.bitsPer%8 != 0 ||
		w.audioFormat == 3 && w.bitsPer != 32 && w.bitsPer != 64 {
		return nil, fmt.Errorf("不支持的位深:%d bit", w.bitsPer)
	}
	w.bytesPer = (w.bitsPer + 7) / 8
	w.samplesLE = dataBuf
	w.sampleCount = len(dataBuf) / w.bytesPer
	w.samplesMask = (1 << uint(w.bitsPer)) - 1
	if w.sampleRate > 0 && w.channels > 0 {
		w.durationSec = float64(w.sampleCount) / float64(w.sampleRate*w.channels)
	}
	w.Chunks = chunkList
	if endOfChunks < len(data) {
		w.Trailing = data[endOfChunks:]
	}
	return w, nil
}

// monoFloat 混 down 到单声道并归一化到 [-1,1](频谱图用;float 格式直接取 IEEE 值)。
func (w *wavInfo) monoFloat() []float64 {
	frames := w.sampleCount / w.channels
	out := make([]float64, frames)
	half := float64(int64(1) << (w.bitsPer - 1))
	for f := 0; f < frames; f++ {
		var sum float64
		for c := 0; c < w.channels; c++ {
			v := w.sampleAt(f*w.channels + c)
			var fv float64
			switch {
			case w.audioFormat == 3 && w.bitsPer == 64:
				fv = math.Float64frombits(v)
			case w.audioFormat == 3:
				fv = float64(math.Float32frombits(uint32(v)))
			case w.bitsPer == 8: // 8bit PCM 无符号
				fv = (float64(v) - 128) / 128
			default: // 有符号整型(小端原码即补码)
				s := int64(v)
				if uint64(s) >= uint64(half) {
					s -= int64(half) * 2
				}
				fv = float64(s) / half
			}
			sum += fv
		}
		out[f] = sum / float64(w.channels)
	}
	return out
}

// sampleAt 读第 i 个采样(交错序,小端原始值;位运算无需区分符号)。
func (w *wavInfo) sampleAt(i int) uint64 {
	off := i * w.bytesPer
	var v uint64
	for b := 0; b < w.bytesPer; b++ {
		v |= uint64(w.samplesLE[off+b]) << (8 * b)
	}
	return v & w.samplesMask
}

// ---------- 报告结构 ----------

type audioLSBCandidate struct {
	Plane     int     `json:"plane"`
	Order     string  `json:"order"`
	Channel   string  `json:"channel"`
	OnesRatio float64 `json:"ones_ratio"`
	Type      string  `json:"type"`
	Hit       bool    `json:"hit"`
}

type audioLSBHit struct {
	audioLSBCandidate
	Format   string `json:"format,omitempty"` // 展示名:JPEG/PNG/TEXT…
	Offset   int    `json:"offset"`
	Size     int    `json:"size"`
	Artifact string `json:"artifact"`
	Preview  string `json:"preview,omitempty"`
	Text     string `json:"text,omitempty"`
}

type audioLSBFileMeta struct {
	Name          string  `json:"name"`
	Size          int64   `json:"size"`
	AudioFormat   string  `json:"audio_format"`
	BitsPerSample int     `json:"bits_per_sample"`
	Channels      int     `json:"channels"`
	SampleRate    int     `json:"sample_rate"`
	Samples       int     `json:"samples"`
	DurationSec   float64 `json:"duration_sec"`
}

type audioLSBRecon struct {
	Chunks           []WavChunkMeta `json:"chunks"`
	TrailingBytes    int            `json:"trailing_bytes"`
	TrailingArtifact string         `json:"trailing_artifact,omitempty"`
	TrailingType     string         `json:"trailing_type,omitempty"`
	TrailingPreview  string         `json:"trailing_preview,omitempty"`
	RawFlags         []string       `json:"raw_flags"`
	RawStrings       []string       `json:"raw_strings"`
}

type audioLSBSpectro struct {
	Artifact string  `json:"artifact,omitempty"`
	Nperseg  int     `json:"nperseg,omitempty"`
	FreqLo   float64 `json:"freq_lo,omitempty"`
	FreqHi   float64 `json:"freq_hi,omitempty"`
	Preview  string  `json:"preview,omitempty"`
	Error    string  `json:"error,omitempty"`
}

type audioLSBReport struct {
	File        audioLSBFileMeta    `json:"file"`
	Params      map[string]any      `json:"params"`
	Recon       audioLSBRecon       `json:"recon"`
	Candidates  []audioLSBCandidate `json:"candidates"`
	Hits        []audioLSBHit       `json:"hits"`
	Flags       []string            `json:"flags"`
	Artifacts   []string            `json:"artifacts"`
	Spectrogram *audioLSBSpectro    `json:"spectrogram,omitempty"`
	Summary     string              `json:"summary"`
}

// ---------- 主流程 ----------

func audioLSBRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	report := func(p int, msg string) {
		if tc != nil && tc.ReportProgress != nil {
			tc.ReportProgress(p, msg)
		}
	}
	report(5, "读取 WAV 文件")
	filePath := filepath.Join(tc.Workdir, paramStr(params, "file"))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("读取文件失败: %v", err)}, nil
	}
	st, _ := os.Stat(filePath)
	name := filepath.Base(filePath)
	var fileSize int64
	if st != nil {
		fileSize = st.Size()
	}
	wav, err := parseWav(data, fileSize, name)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}

	// 参数归一化
	getStr := func(k, def string) string {
		v := strings.ToLower(strings.TrimSpace(paramStr(params, k)))
		if v == "" {
			return def
		}
		return v
	}
	planeArg, orderArg, chanArg, bitsArg := getStr("plane", "auto"), getStr("order", "auto"), getStr("channel", "auto"), getStr("bits", "1")
	bits := 1
	if bitsArg == "2" {
		bits = 2
	} else if bitsArg != "1" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "bits 仅支持 1 或 2"}, nil
	}
	maxPlane := wav.bitsPer - bits
	if maxPlane > 3 { // auto 扫描 0-3 足够覆盖常见嵌入
		maxPlane = 3
	}
	var planes []int
	if planeArg == "auto" {
		for p := 0; p <= maxPlane; p++ {
			planes = append(planes, p)
		}
	} else {
		p := int(planeArg[0] - '0')
		if len(planeArg) != 1 || planeArg[0] < '0' || p > maxPlane {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("plane 非法(0-%d 或 auto): %s", maxPlane, planeArg)}, nil
		}
		planes = []int{p}
	}
	orders := []string{"msb", "lsb"}
	if orderArg != "auto" && orderArg != "msb" && orderArg != "lsb" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "order 仅支持 auto/msb/lsb"}, nil
	} else if orderArg != "auto" {
		orders = []string{orderArg}
	}
	chans := []string{"all"}
	if wav.channels > 1 {
		for c := 0; c < wav.channels && c < 8; c++ {
			chans = append(chans, fmt.Sprint(c))
		}
	}
	if chanArg != "auto" {
		ok := false
		for _, c := range chans {
			if c == chanArg {
				ok = true
				break
			}
		}
		if !ok {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("channel 非法(%s):双声道可用 all/0/1", chanArg)}, nil
		}
		chans = []string{chanArg}
	}

	specArg := getStr("spectrogram", "full")
	if specArg != "full" && specArg != "low" && specArg != "ultra" && specArg != "off" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "spectrogram 仅支持 full/low/ultra/off"}, nil
	}
	npersegArg := getStr("nperseg", "2048")
	nperseg := 2048
	switch npersegArg {
	case "2048", "4096", "16384":
		nperseg, _ = strconv.Atoi(npersegArg)
	default:
		return &core.ToolResult{Status: core.StatusError, Stderr: "nperseg 仅支持 2048/4096/16384"}, nil
	}

	rep := audioLSBReport{
		File: audioLSBFileMeta{name, fileSize, wav.formatName, wav.bitsPer, wav.channels,
			wav.sampleRate, wav.sampleCount, wav.durationSec},
		Params: map[string]any{"plane": planeArg, "order": orderArg, "channel": chanArg, "bits": bitsArg,
			"spectrogram": specArg, "nperseg": npersegArg},
		Recon: audioLSBRecon{Chunks: wav.Chunks, RawFlags: []string{}, RawStrings: []string{}},
		Candidates: []audioLSBCandidate{},
		Hits:       []audioLSBHit{},
		Flags:      []string{},
		Artifacts:  []string{},
	}

	// ---- P0 侦察:尾部附加数据 + 原始字节扫描 ----
	report(8, "侦察:chunk 清单与尾部附加数据")
	if len(wav.Trailing) > 0 {
		rep.Recon.TrailingBytes = len(wav.Trailing)
		typ, ext, mime, _ := stego.Detect(wav.Trailing)
		if typ == "" {
			typ, ext = "BIN", "bin"
		}
		rep.Recon.TrailingType = ext
		artName := "trailing." + ext
		if tc.ArtifactsDir != "" {
			if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err != nil {
				return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
			}
			if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, artName), wav.Trailing, 0o644); err != nil {
				return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出尾部数据失败: %v", err)}, nil
			}
		}
		rep.Recon.TrailingArtifact = artName
		rep.Artifacts = append(rep.Artifacts, artName)
		if strings.HasPrefix(mime, "image/") && len(wav.Trailing) <= stego.MaxPreview {
			rep.Recon.TrailingPreview = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(wav.Trailing)
		}
		summary0 := fmt.Sprintf("[+] 尾部附加数据 %dB → %s(%s)\n", len(wav.Trailing), artName, typ)
		rep.Summary = summary0 + rep.Summary
	}
	rep.Recon.RawFlags = stego.Flags(data)
	rep.Recon.RawStrings = stego.PrintableStrings(data, 6, 30)

	total := len(planes) * len(orders) * len(chans)
	done := 0
	for _, plane := range planes {
		for _, order := range orders {
			for _, ch := range chans {
				done++
				report(10+80*done/total, fmt.Sprintf("位平面 bit%d/%s/声道%s(%d/%d)", plane, order, ch, done, total))
				include := func(int) bool { return true }
				if ch != "all" {
					ci := int(ch[0] - '0')
					include = func(i int) bool { return i%wav.channels == ci }
				}
				bs, ones, cnt := stego.Pack(wav.sampleCount, wav.sampleAt, plane, bits, order == "msb", include)
				if len(bs) == 0 || cnt == 0 {
					continue
				}
				ratio := float64(ones) / float64(cnt)
				cand := audioLSBCandidate{Plane: plane, Order: order, Channel: ch, OnesRatio: ratio}
				typ, ext, mime, off := stego.Detect(bs)
				win := bs
				if len(win) > stego.ScanWindow {
					win = win[:stego.ScanWindow]
				}
				for _, f := range stego.Flags(win) {
					rep.Flags = append(rep.Flags, f)
				}
				if typ == "" {
					// 未命中魔数/文本,但本平面窗口内出现 flag → 按文本片段产出
					if idx := stego.FlagRe.FindStringIndex(string(win)); idx != nil {
						typ, ext, mime, off = "TEXT", "txt", "text/plain", idx[0]
					} else {
						rep.Candidates = append(rep.Candidates, cand)
						continue
					}
				}
				seg := stego.Cut(bs, off, typ)
				if len(seg) == 0 {
					rep.Candidates = append(rep.Candidates, cand)
					continue
				}
				art := fmt.Sprintf("lsb_bit%d_%s_%s.%s", plane, order, ch, ext)
				if tc.ArtifactsDir != "" {
					if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err != nil {
						return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
					}
					if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, art), seg, 0o644); err != nil {
						return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出产物失败: %v", err)}, nil
					}
				}
				hit := audioLSBHit{audioLSBCandidate: cand, Offset: off, Size: len(seg), Artifact: art}
				hit.Type, hit.Hit, hit.Format = stego.TypeOf(typ, ext), true, typ
				cand.Type, cand.Hit = stego.TypeOf(typ, ext), true
				rep.Candidates = append(rep.Candidates, cand)
				if strings.HasPrefix(mime, "image/") && len(seg) <= stego.MaxPreview {
					hit.Preview = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(seg)
				}
				if typ == "TEXT" {
					hit.Text = string(seg)
				}
				rep.Hits = append(rep.Hits, hit)
				rep.Artifacts = append(rep.Artifacts, art)
			}
		}
	}
	report(92, "生成频谱图")
	if specArg != "off" {
		spec := &audioLSBSpectro{Nperseg: nperseg}
		var lo, hi float64
		switch specArg {
		case "low":
			lo, hi = 0, 2000
		case "ultra":
			lo, hi = 18000, float64(wav.sampleRate)/2
		}
		spec.FreqLo, spec.FreqHi = lo, hi
		png, err := stego.SpectrogramPNG(wav.monoFloat(), wav.sampleRate, nperseg, lo, hi)
		if err != nil {
			spec.Error = err.Error()
		} else {
			spec.Artifact = "spectrogram.png"
			if tc.ArtifactsDir != "" {
				if werr := os.WriteFile(filepath.Join(tc.ArtifactsDir, spec.Artifact), png, 0o644); werr != nil {
					return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出频谱图失败: %v", werr)}, nil
				}
			}
			rep.Artifacts = append(rep.Artifacts, spec.Artifact)
			if len(png) <= stego.MaxPreview {
				spec.Preview = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
			}
		}
		rep.Spectrogram = spec
	}
	report(95, "汇总报告")

	// summary
	var sb strings.Builder
	fmt.Fprintf(&sb, "WAV %s:%s %dbit × %d 声道 @%dHz,%.1f 秒,%d 采样\n",
		name, wav.formatName, wav.bitsPer, wav.channels, wav.sampleRate, wav.durationSec, wav.sampleCount)
	if rep.Recon.TrailingBytes > 0 {
		fmt.Fprintf(&sb, "[+] data 之后有 %dB 附加数据 → %s(类型 %s;copy /b 拼接或工具残留)\n",
			rep.Recon.TrailingBytes, rep.Recon.TrailingArtifact, rep.Recon.TrailingType)
	}
	for _, f := range rep.Recon.RawFlags {
		fmt.Fprintf(&sb, "[raw] %s(原始字节命中,注意甄别噪声巧合)\n", f)
	}
	if len(rep.Hits) == 0 {
		sb.WriteString("未发现已知类型 LSB 隐写(各位平面 1/0 比例均≈0.5 时为纯噪声载体;可尝试 DeepSound/steghide 等格式工具)\n")
	} else {
		for _, h := range rep.Hits {
			fmt.Fprintf(&sb, "[+] bit%d/%s/声道%s → %s %dB(偏移 %d)%s\n",
				h.Plane, h.Order, h.Channel, h.Type, h.Size, h.Offset, h.Artifact)
		}
	}
	for _, f := range rep.Flags {
		fmt.Fprintf(&sb, "[flag] %s\n", f)
	}
	if rep.Spectrogram != nil {
		if rep.Spectrogram.Error != "" {
			fmt.Fprintf(&sb, "[-] 频谱图生成失败:%s\n", rep.Spectrogram.Error)
		} else {
			fmt.Fprintf(&sb, "[+] 频谱图 → %s(窗 %d,%.0f-%.0fHz;查看是否有画字/二维码/亮带)\n",
				rep.Spectrogram.Artifact, rep.Spectrogram.Nperseg, rep.Spectrogram.FreqLo, rep.Spectrogram.FreqHi)
		}
	}
	rep.Summary = sb.String()

	out, err := json.Marshal(rep)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	report(100, "完成")
	return &core.ToolResult{
		Status: core.StatusSuccess,
		Stdout: string(out),
		Data:   map[string]any{"hit_count": len(rep.Hits), "flags": rep.Flags},
	}, nil
}
