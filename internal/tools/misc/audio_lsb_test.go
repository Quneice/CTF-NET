// misc.audio_lsb 测试:WAV LSB 位平面提取 + 隐写内容类型自动识别。
// 用例参照 2026 Bugku hhh.wav 套路(16bit 单声道 bit0 MSB-first 藏 JPEG)及其变体:
// 双声道按声道提取、8bit 无符号、每采样 2bit、LSB-first 位序、RIFF 尾部额外 chunk、错误输入。
package misc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

// ---------- 测试用 WAV 构造与 LSB 嵌入助手 ----------

// lsbBuildWav 构造最小 RIFF/WAVE(PCM 或 IEEE float,fmt 16 字节标准头)。
// samples 为按声道交错的全采样序列(帧数 = len/channels)。
func lsbBuildWav(audioFormat uint16, channels, bits, sampleRate int, samples []int32) []byte {
	bytesPer := (bits + 7) / 8
	data := make([]byte, len(samples)*bytesPer)
	for i, s := range samples {
		for b := 0; b < bytesPer; b++ {
			data[i*bytesPer+b] = byte(s >> (8 * b)) // 小端
		}
	}
	blockAlign := channels * bytesPer
	byteRate := sampleRate * blockAlign

	total := 4 + 8 + 16 + 8 + len(data) // "WAVE" + fmt 头 + data 头
	out := make([]byte, 0, 8+total)
	out = append(out, 'R', 'I', 'F', 'F')
	out = append(out, byte(total), byte(total>>8), byte(total>>16), byte(total>>24))
	out = append(out, 'W', 'A', 'V', 'E')
	out = append(out, 'f', 'm', 't', ' ', 16, 0, 0, 0)
	out = append(out, byte(audioFormat), byte(audioFormat>>8))
	out = append(out, byte(channels), byte(channels>>8))
	out = append(out, byte(sampleRate), byte(sampleRate>>8), byte(sampleRate>>16), byte(sampleRate>>24))
	out = append(out, byte(byteRate), byte(byteRate>>8), byte(byteRate>>16), byte(byteRate>>24))
	out = append(out, byte(blockAlign), byte(blockAlign>>8))
	out = append(out, byte(bits), byte(bits>>8))
	out = append(out, 'd', 'a', 't', 'a')
	out = append(out, byte(len(data)), byte(len(data)>>8), byte(len(data)>>16), byte(len(data)>>24))
	out = append(out, data...)
	return out
}

// lsbMix murmur 终结器式混合:低位同样均匀(LCG 低位会退化为 1010 交替)。
func lsbMix(h uint32) uint32 {
	h *= 0x9E3779B1
	h ^= h >> 15
	h *= 0x85EBCA6B
	h ^= h >> 13
	return h
}

// lsbCarrier 生成确定性伪噪声载波采样(低位均匀,避免位平面出现规律图案)。
func lsbCarrier(n int) []int32 {
	out := make([]int32, n)
	for i := range out {
		out[i] = int32(lsbMix(uint32(i+1) * 0x2545F491))
	}
	return out
}

// lsbNoiseBytes 生成确定性伪噪声字节(payload 填充用)。
func lsbNoiseBytes(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(lsbMix(uint32(i+1)*0x27D4EB2F) >> 24)
	}
	return out
}

// lsbEmbed 把 payload 按指定位平面/位数/位序嵌进交错采样序列(就地修改)。
// channel=-1 表示所有采样(文件序);否则仅嵌该声道。
// bits>1 时每个采样按位号升序贡献 plane..plane+bits-1 位。
func lsbEmbed(samples []int32, channels, channel, plane, bits int, msbFirst bool, payload []byte) {
	// 展开成位流
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
	// 定位目标采样下标
	var idx []int
	for i := 0; i < len(samples); i++ {
		if channel < 0 || i%channels == channel {
			idx = append(idx, i)
		}
	}
	if len(idx)*bits < len(bitstream) {
		panic("载体容量不足")
	}
	n := (len(bitstream) + bits - 1) / bits // 只覆盖 payload 所需采样数
	for bi, target := range idx[:n] {
		v := samples[target]
		for k := 0; k < bits; k++ {
			bit := bitstream[bi*bits+k]
			p := uint(plane + k)
			v = (v &^ (1 << p)) | (int32(bit) << p)
		}
		samples[target] = v
	}
}

// lsbRunTool 把 wav 写入 Workdir 并以给定参数运行 misc.audio_lsb。
func lsbRunTool(t *testing.T, wav []byte, params map[string]any) (*core.ToolResult, string) {
	t.Helper()
	tool, ok := core.DefaultRegistry.Get("misc.audio_lsb")
	if !ok {
		t.Fatal("misc.audio_lsb 未注册")
	}
	work := t.TempDir()
	arts := t.TempDir()
	name := "test.wav"
	if err := os.WriteFile(filepath.Join(work, name), wav, 0o644); err != nil {
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
	return res, arts
}

// lsbParseStdout 解析工具 stdout 的 JSON 报告。
func lsbParseStdout(t *testing.T, res *core.ToolResult) map[string]any {
	t.Helper()
	if res.Status != core.StatusSuccess {
		t.Fatalf("期望 success,实际 %s: %s", res.Status, res.Stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &m); err != nil {
		t.Fatalf("stdout 不是合法 JSON: %v\n%s", err, res.Stdout)
	}
	return m
}

func lsbHitField(t *testing.T, m map[string]any, i int, key string) any {
	t.Helper()
	hits, _ := m["hits"].([]any)
	if len(hits) <= i {
		t.Fatalf("hits 数量不足: %d", len(hits))
	}
	h, _ := hits[i].(map[string]any)
	return h[key]
}

// ---------- 用例 ----------

// A. 经典套路(hhh.wav):16bit 单声道 bit0 MSB-first 藏 JPEG,auto 扫描命中,
// 产物按 EOI(FFD9)截断,与原 payload 逐字节一致,并内联 base64 预览。
func TestAudioLSBJpegBit0MsbAuto(t *testing.T) {
	payload := append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'},
		lsbNoiseBytes(3000)...)
	payload = append(payload[:1996], 0x12, 0x34, 0xFF, 0xD9) // 共 2000B,EOI 收尾
	samples := lsbCarrier(20000) // 远大于 payload 所需
	lsbEmbed(samples, 1, -1, 0, 1, true, payload)
	res, arts := lsbRunTool(t, lsbBuildWav(1, 1, 16, 44100, samples), nil)

	m := lsbParseStdout(t, res)
	file, _ := m["file"].(map[string]any)
	if file["bits_per_sample"] != float64(16) || file["channels"] != float64(1) || file["sample_rate"] != float64(44100) {
		t.Fatalf("WAV 元数据解析错误: %v", file)
	}
	if file["samples"] != float64(20000) {
		t.Fatalf("采样数错误: %v", file["samples"])
	}
	if got := lsbHitField(t, m, 0, "type"); got != "jpg" {
		t.Fatalf("期望命中 jpg,得到 %v", got)
	}
	if got := lsbHitField(t, m, 0, "plane"); got != float64(0) {
		t.Fatalf("期望 plane=0,得到 %v", got)
	}
	if got := lsbHitField(t, m, 0, "order"); got != "msb" {
		t.Fatalf("期望 order=msb,得到 %v", got)
	}
	if got := lsbHitField(t, m, 0, "size"); got != float64(len(payload)) {
		t.Fatalf("期望产物大小 %d(EOI 截断),得到 %v", len(payload), got)
	}
	art := lsbHitField(t, m, 0, "artifact").(string)
	got, err := os.ReadFile(filepath.Join(arts, art))
	if err != nil || string(got) != string(payload) {
		t.Fatalf("产物与嵌入 payload 不一致: %v", err)
	}
	pv, _ := lsbHitField(t, m, 0, "preview").(string)
	if !strings.HasPrefix(pv, "data:image/jpeg;base64,") {
		t.Fatalf("缺少 JPEG base64 预览: %q", pv[:min(40, len(pv))])
	}
}

// B. 文本 flag 藏 bit1、LSB-first 位序:flag 正则命中 + 文本产物。
func TestAudioLSBTextFlagBit1LsbFirst(t *testing.T) {
	payload := []byte("hello stego! the key is flag{t3st_l5b_2026} bye")
	samples := lsbCarrier(6000)
	lsbEmbed(samples, 1, -1, 1, 1, false, payload)
	res, arts := lsbRunTool(t, lsbBuildWav(1, 1, 16, 44100, samples), nil)

	m := lsbParseStdout(t, res)
	flags, _ := m["flags"].([]any)
	found := false
	for _, f := range flags {
		if f == "flag{t3st_l5b_2026}" {
			found = true
		}
	}
	if !found {
		t.Fatalf("未提取到 flag{t3st_l5b_2026}: %v", flags)
	}
	art := lsbHitField(t, m, 0, "artifact").(string)
	if !strings.HasSuffix(art, ".txt") {
		t.Fatalf("文本命中应产出 .txt: %s", art)
	}
	got, err := os.ReadFile(filepath.Join(arts, art))
	if err != nil || !strings.Contains(string(got), "flag{t3st_l5b_2026}") {
		t.Fatalf("文本产物内容缺失 flag: %v", err)
	}
}

// C. 双声道、数据只藏右声道(channel=1)bit2 LSB-first:显式参数按声道提取命中。
func TestAudioLSBStereoChannelExplicit(t *testing.T) {
	payload := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}, lsbNoiseBytes(400)...)
	payload = append(payload, 0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xAE, 0x42, 0x60, 0x82)
	frames := 6000
	samples := lsbCarrier(frames * 2)
	lsbEmbed(samples, 2, 1, 2, 1, false, payload)
	res, arts := lsbRunTool(t, lsbBuildWav(1, 2, 16, 44100, samples),
		map[string]any{"plane": "2", "order": "lsb", "channel": "1"})

	m := lsbParseStdout(t, res)
	if got := lsbHitField(t, m, 0, "type"); got != "png" {
		t.Fatalf("期望 png,得到 %v", got)
	}
	if got := lsbHitField(t, m, 0, "channel"); got != "1" {
		t.Fatalf("期望 channel=1,得到 %v", got)
	}
	if got := lsbHitField(t, m, 0, "size"); got != float64(len(payload)) {
		t.Fatalf("IEND 截断错误: 期望 %d 得到 %v", len(payload), got)
	}
	art := lsbHitField(t, m, 0, "artifact").(string)
	got, err := os.ReadFile(filepath.Join(arts, art))
	if err != nil || string(got) != string(payload) {
		t.Fatalf("PNG 产物不一致: %v", err)
	}
}

// D. 8bit 无符号 PCM 同样可提取(bit0 MSB-first)。
func TestAudioLSB8BitUnsigned(t *testing.T) {
	payload := []byte("flag{u8_pcm}")
	samples := lsbCarrier(3000)
	for i := range samples {
		samples[i] &= 0xFF // 8bit 值域
	}
	lsbEmbed(samples, 1, -1, 0, 1, true, payload)
	res, _ := lsbRunTool(t, lsbBuildWav(1, 1, 8, 8000, samples), nil)

	m := lsbParseStdout(t, res)
	if got := lsbHitField(t, m, 0, "type"); got != "text" && got != "flag" {
		t.Fatalf("8bit 文本未命中,得到 %v(hits=%v)", got, m["hits"])
	}
	flags, _ := m["flags"].([]any)
	if len(flags) == 0 || flags[0] != "flag{u8_pcm}" {
		t.Fatalf("8bit flag 提取失败: %v", flags)
	}
}

// E. 每采样 2bit(plane=1,取 bit1、bit2 升序)显式参数提取。
func TestAudioLSBTwoBitsPerSample(t *testing.T) {
	payload := append([]byte{0xFF, 0xD8, 0xFF}, lsbNoiseBytes(500)...)
	payload = append(payload[:398], 0xFF, 0xD9) // 共 400B,EOI 收尾
	samples := lsbCarrier(4000)
	lsbEmbed(samples, 1, -1, 1, 2, true, payload)
	res, arts := lsbRunTool(t, lsbBuildWav(1, 1, 16, 44100, samples),
		map[string]any{"plane": "1", "bits": "2", "order": "msb", "channel": "all"})

	m := lsbParseStdout(t, res)
	if got := lsbHitField(t, m, 0, "type"); got != "jpg" {
		t.Fatalf("2bit 提取未命中 jpg: %v", got)
	}
	art := lsbHitField(t, m, 0, "artifact").(string)
	got, err := os.ReadFile(filepath.Join(arts, art))
	if err != nil || string(got) != string(payload) {
		t.Fatalf("2bit 产物不一致: %v", err)
	}
}

// F. RIFF 尾部跟额外 chunk(LIST):按 chunk 遍历定位 data,提取不受影响。
func TestAudioLSBTrailingChunkAfterData(t *testing.T) {
	payload := []byte("flag{tr41l_chunk}")
	samples := lsbCarrier(2000)
	lsbEmbed(samples, 1, -1, 0, 1, true, payload)
	wav := lsbBuildWav(1, 1, 16, 44100, samples)
	wav = append(wav, 'L', 'I', 'S', 'T', 4, 0, 0, 0, 'I', 'N', 'F', 'O') // data 之后追加 LIST
	res, _ := lsbRunTool(t, wav, nil)

	m := lsbParseStdout(t, res)
	flags, _ := m["flags"].([]any)
	if len(flags) == 0 || flags[0] != "flag{tr41l_chunk}" {
		t.Fatalf("尾部 chunk 场景 flag 提取失败: %v", flags)
	}
}

// G. 纯噪声无隐写:auto 扫描正常返回、零命中、候选概览含 ones_ratio。
func TestAudioLSBNoHitNoise(t *testing.T) {
	res, _ := lsbRunTool(t, lsbBuildWav(1, 1, 16, 44100, lsbCarrier(4096)), nil)
	m := lsbParseStdout(t, res)
	if hits, _ := m["hits"].([]any); len(hits) != 0 {
		t.Fatalf("噪声载体不应命中: %v", hits)
	}
	cands, _ := m["candidates"].([]any)
	if len(cands) < 8 {
		t.Fatalf("候选概览不足(auto 应扫 bit0-3 × msb/lsb): %d", len(cands))
	}
	first, _ := cands[0].(map[string]any)
	if r, ok := first["ones_ratio"].(float64); !ok || r < 0.4 || r > 0.6 {
		t.Fatalf("噪声位平面 ones_ratio 应≈0.5: %v", first["ones_ratio"])
	}
}

// H. 错误输入:非 RIFF、压缩格式、data 声明超长。
func TestAudioLSBErrors(t *testing.T) {
	cases := []struct {
		name string
		wav  []byte
		want string
	}{
		{"非 RIFF", []byte("not a wav file at all"), "RIFF"},
		{"压缩格式", func() []byte {
			w := lsbBuildWav(1, 1, 16, 44100, lsbCarrier(64))
			w[20] = 0x11 // AudioFormat=17(ADPCM)
			w[21] = 0x00
			return w
		}(), "PCM"},
		{"data 超长", func() []byte {
			w := lsbBuildWav(1, 1, 16, 44100, lsbCarrier(64))
			w[40], w[41], w[42], w[43] = 0xFF, 0xFF, 0, 0 // data size 声明远大于实际
			return w
		}(), "data"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, _ := lsbRunTool(t, c.wav, nil)
			if res.Status != core.StatusError {
				t.Fatalf("期望 error 状态,得到 %s(stdout=%s)", res.Status, res.Stdout)
			}
			if !strings.Contains(res.Stderr, c.want) {
				t.Fatalf("错误信息应含 %q: %s", c.want, res.Stderr)
			}
		})
	}
}

// LIVE. 真实题目门控测试(惯例同 SUBENUM_LIVE/JSFINDER_LIVE):
//   AUDIO_LSB_LIVE=<hhh.wav路径> go test ./internal/tools/misc/ -run TestAudioLSBLive -v
// 预期(据 writeup):21,168,044B,16bit 单声道 @44100Hz,bit0/msb/all 命中 JPEG 9497B(FFD8FFE0…FFD9)。
func TestAudioLSBLiveHHH(t *testing.T) {
	live := os.Getenv("AUDIO_LSB_LIVE")
	if live == "" {
		t.Skip("未设置 AUDIO_LSB_LIVE,跳过真实题目验证")
	}
	wav, err := os.ReadFile(live)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", live, err)
	}
	res, arts := lsbRunTool(t, wav, nil)
	m := lsbParseStdout(t, res)

	file, _ := m["file"].(map[string]any)
	if file["size"] != float64(21168044) || file["bits_per_sample"] != float64(16) || file["channels"] != float64(1) {
		t.Fatalf("WAV 元数据与 writeup 不符: %v", file)
	}
	if got := lsbHitField(t, m, 0, "type"); got != "jpg" {
		t.Fatalf("期望 bit0/msb/all 命中 jpg,得到 %v(hits=%v)", got, m["hits"])
	}
	if got := lsbHitField(t, m, 0, "plane"); got != float64(0) {
		t.Fatalf("期望 plane=0,得到 %v", got)
	}
	if got := lsbHitField(t, m, 0, "order"); got != "msb" {
		t.Fatalf("期望 order=msb,得到 %v", got)
	}
	if got := lsbHitField(t, m, 0, "size"); got != float64(9497) {
		t.Fatalf("期望 EOI 截断后 9497B,得到 %v", got)
	}
	art := lsbHitField(t, m, 0, "artifact").(string)
	got, err := os.ReadFile(filepath.Join(arts, art))
	if err != nil {
		t.Fatalf("产物读取失败: %v", err)
	}
	if len(got) != 9497 || got[0] != 0xFF || got[1] != 0xD8 || got[2] != 0xFF || got[3] != 0xE0 ||
		got[len(got)-2] != 0xFF || got[len(got)-1] != 0xD9 {
		t.Fatalf("JPEG 结构校验失败: 头 % X 尾 % X", got[:4], got[len(got)-2:])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------- P0 侦察 / P1 频谱图 用例 ----------

// I. 侦察:chunk 清单、尾部附加数据提取(含 PNG 追加场景)、原始字节 flag/strings 扫描。
func TestAudioLSBReconTrailingAndRawScan(t *testing.T) {
	wav := lsbBuildWav(1, 1, 16, 44100, lsbCarrier(2000))
	wav = append(wav, 'L', 'I', 'S', 'T', 4, 0, 0, 0, 'I', 'N', 'F', 'O') // data 之后追加 LIST(不在 RIFF 声明内)
	wav = append(wav, []byte("junk flag{tr41l_d4ta} tail")...)
	res, arts := lsbRunTool(t, wav, nil)

	m := lsbParseStdout(t, res)
	recon, ok := m["recon"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 recon 区块: %v", m["recon"])
	}
	chunks, _ := recon["chunks"].([]any)
	if len(chunks) < 2 { // 至少 fmt + data
		t.Fatalf("chunk 清单不全: %v", chunks)
	}
	hasData := false
	for _, c := range chunks {
		cm, _ := c.(map[string]any)
		if cm["id"] == "data" {
			hasData = true
		}
	}
	if !hasData {
		t.Fatalf("chunk 清单缺 data: %v", chunks)
	}
	// 尾部 = LIST 块 + junk(共 12+25 字节)
	if tb := recon["trailing_bytes"]; tb != float64(12+26) {
		t.Fatalf("尾部字节数错误: %v(期望 37)", tb)
	}
	art, _ := recon["trailing_artifact"].(string)
	got, err := os.ReadFile(filepath.Join(arts, art))
	if err != nil || len(got) != 38 {
		t.Fatalf("尾部产物错误: %v %d", err, len(got))
	}
	// 原始字节 flag 扫描
	flags, _ := recon["raw_flags"].([]any)
	found := false
	for _, f := range flags {
		if f == "flag{tr41l_d4ta}" {
			found = true
		}
	}
	if !found {
		t.Fatalf("raw_flags 未命中: %v", flags)
	}
	// strings 扫描:LIST/INFO 与 junk 串应出现
	strs, _ := recon["raw_strings"].([]any)
	joined := ""
	for _, s := range strs {
		joined += fmt.Sprint(s) + "|"
	}
	if !strings.Contains(joined, "junk flag{tr41l_d4ta} tail") && !strings.Contains(joined, "LIST") {
		t.Fatalf("raw_strings 缺少预期串: %v", strs)
	}
}

// I-2. 尾部直接追加 PNG(writeup 之外的常见套路:copy /b cover.wav+secret.png)。
func TestAudioLSBReconTrailingPNG(t *testing.T) {
	pngPayload := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}, lsbNoiseBytes(200)...)
	wav := lsbBuildWav(1, 1, 16, 44100, lsbCarrier(1000))
	wav = append(wav, pngPayload...)
	res, arts := lsbRunTool(t, wav, nil)

	m := lsbParseStdout(t, res)
	recon, _ := m["recon"].(map[string]any)
	if recon["trailing_type"] != "png" {
		t.Fatalf("尾部 PNG 未识别: %v", recon["trailing_type"])
	}
	art, _ := recon["trailing_artifact"].(string)
	if !strings.HasSuffix(art, ".png") {
		t.Fatalf("尾部产物扩展名错误: %s", art)
	}
	got, err := os.ReadFile(filepath.Join(arts, art))
	if err != nil || string(got) != string(pngPayload) {
		t.Fatalf("尾部 PNG 内容不一致: %v", err)
	}
	if pv, _ := recon["trailing_preview"].(string); !strings.HasPrefix(pv, "data:image/png;base64,") {
		t.Fatalf("尾部 PNG 预览缺失: %q", pv)
	}
}

// J. P1 频谱图:artifact 落盘 + JSON 字段 + PNG 可解码;off 时不生成。
func TestAudioLSBSpectrogram(t *testing.T) {
	const sr = 44100
	samples := lsbCarrier(sr / 4) // 0.25s 噪声(内容不影响断言)
	res, arts := lsbRunTool(t, lsbBuildWav(1, 1, 16, sr, samples),
		map[string]any{"spectrogram": "full", "nperseg": "2048"})

	m := lsbParseStdout(t, res)
	spec, ok := m["spectrogram"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 spectrogram 区块: %v", m["spectrogram"])
	}
	art, _ := spec["artifact"].(string)
	raw, err := os.ReadFile(filepath.Join(arts, art))
	if err != nil {
		t.Fatalf("频谱图产物缺失: %v", err)
	}
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil || format != "png" || img.Bounds().Dx() < 4 || img.Bounds().Dy() < 4 {
		t.Fatalf("频谱图 PNG 非法: %v %s", err, format)
	}
	if pv, _ := spec["preview"].(string); !strings.HasPrefix(pv, "data:image/png;base64,") {
		t.Fatalf("频谱图预览缺失")
	}

	// off 模式
	res2, _ := lsbRunTool(t, lsbBuildWav(1, 1, 16, sr, samples), map[string]any{"spectrogram": "off"})
	m2 := lsbParseStdout(t, res2)
	if _, exists := m2["spectrogram"]; exists {
		t.Fatalf("off 模式不应有 spectrogram 区块: %v", m2["spectrogram"])
	}
}

// K. JSON 契约:无命中时数组字段必须是 [] 而非 null(前端渲染器读 .length,
// null 会导致整页崩溃——2026-11-14 实测线上崩溃回归用例)。
func TestAudioLSBJSONNoNullArrays(t *testing.T) {
	res, _ := lsbRunTool(t, lsbBuildWav(1, 1, 16, 44100, lsbCarrier(4096)), nil)
	if res.Status != core.StatusSuccess {
		t.Fatalf("期望 success: %s", res.Stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"candidates", "hits", "flags", "artifacts"} {
		if v, exists := m[key]; !exists || v == nil {
			t.Fatalf("%s 应为 [] 而非 null", key)
		}
	}
	recon, _ := m["recon"].(map[string]any)
	if recon == nil {
		t.Fatal("recon 区块缺失")
	}
	for _, key := range []string{"raw_flags", "raw_strings", "chunks"} {
		if v, exists := recon[key]; !exists || v == nil {
			t.Fatalf("recon.%s 应为 [] 而非 null", key)
		}
	}
}
