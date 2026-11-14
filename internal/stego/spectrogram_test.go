// 频谱图渲染引擎测试:FFT 正确性(正弦峰落在对应频点)、PNG 可解码、参数校验。
package stego

import (
	"bytes"
	"image"
	"math"
	"math/cmplx"
	"testing"
)

// TestFFTSine 验证 radix-2 FFT:440Hz 正弦的能量峰应落在 440Hz 最近的 bin。
func TestFFTSine(t *testing.T) {
	const n = 4096
	const sr = 44100
	const freq = 440.0
	re := make([]float64, n)
	im := make([]float64, n)
	for i := 0; i < n; i++ {
		re[i] = math.Sin(2 * math.Pi * freq * float64(i) / sr)
	}
	FFT(re, im)
	wantBin := int(math.Round(freq / (float64(sr) / float64(n)))) // 注意:常量整除陷阱
	best, bestMag := 0, -1.0
	for k := 0; k < n/2+1; k++ {
		if m := cmplx.Abs(complex(re[k], im[k])); m > bestMag {
			bestMag, best = m, k
		}
	}
	if math.Abs(float64(best-wantBin)) > 1 {
		t.Fatalf("FFT 峰值 bin=%d,期望 %d(440Hz)", best, wantBin)
	}
}

// TestSpectrogramSinePeak 440Hz 正弦的频谱图:最亮行应位于 440Hz 附近(行 0=最高频)。
func TestSpectrogramSinePeak(t *testing.T) {
	const sr = 44100
	const freq = 1000.0
	samples := make([]float64, sr/2) // 0.5s
	for i := range samples {
		samples[i] = 0.8 * math.Sin(2*math.Pi*freq*float64(i)/sr)
	}
	png, err := SpectrogramPNG(samples, sr, 4096, 0, 0) // 0,0 = 全频段
	if err != nil {
		t.Fatal(err)
	}
	img, format, err := image.Decode(bytes.NewReader(png))
	if err != nil || format != "png" {
		t.Fatalf("PNG 解码失败: %v format=%s", err, format)
	}
	b := img.Bounds()
	// 找平均亮度最大的行(行 0=高频 Nyquist,行 bottom=0Hz)
	rows := b.Dy()
	bestRow, bestSum := 0, -1.0
	for y := 0; y < rows; y++ {
		sum := 0.0
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			sum += float64(r+g+bl) / 3
		}
		if sum > bestSum {
			bestSum, bestRow = sum, y
		}
	}
	// 行 y 对应频率 = (rows-1-y)/(rows-1) * Nyquist
	nyq := float64(sr) / 2
	fAt := float64(rows-1-bestRow) / float64(rows-1) * nyq
	if math.Abs(fAt-freq) > nyq/float64(rows-1)*2 {
		t.Fatalf("最亮行对应 %.0fHz,期望约 1000Hz(row=%d rows=%d)", fAt, bestRow, rows)
	}
}

// TestSpectrogramFreqRangeCrop 低频段(0-2kHz)应只含对应行数。
func TestSpectrogramFreqRangeCrop(t *testing.T) {
	samples := make([]float64, 4410)
	for i := range samples {
		samples[i] = math.Sin(2 * math.Pi * 500 * float64(i) / 44100)
	}
	png, err := SpectrogramPNG(samples, 44100, 1024, 0, 2000)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(png))
	if err != nil {
		t.Fatal(err)
	}
	// bin 宽 43Hz,0-2000Hz ≈ 47 个 bin → 行数应明显小于全谱 513
	if dy := img.Bounds().Dy(); dy < 40 || dy > 60 {
		t.Fatalf("低频段行数 %d 不符(期望 40-60)", dy)
	}
}

func TestSpectrogramErrors(t *testing.T) {
	if _, err := SpectrogramPNG(nil, 44100, 2048, 0, 0); err == nil {
		t.Fatal("空采样应报错")
	}
	if _, err := SpectrogramPNG(make([]float64, 8192), 44100, 1000, 0, 0); err == nil {
		t.Fatal("非 2 的幂窗长应报错")
	}
	if _, err := SpectrogramPNG(make([]float64, 8192), 44100, 2048, 3000, 1000); err == nil {
		t.Fatal("freqLo>freqHi 应报错")
	}
}
