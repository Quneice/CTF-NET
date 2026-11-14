// 频谱图渲染:radix-2 FFT + Hann 窗 → dB 矩阵 → 灰度 PNG。
// 用于频域隐写肉眼识别(频谱画字/QR/SSTV、低频亮带、超声波隐藏)。
package stego

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
)

// FFT 原地 radix-2 Cooley-Tukey FFT(re/im 等长,长度须为 2 的幂)。
func FFT(re, im []float64) {
	n := len(re)
	// 位反转置换
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j |= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	// 蝶形
	for length := 2; length <= n; length <<= 1 {
		ang := -2 * math.Pi / float64(length)
		half := length / 2
		for i := 0; i < n; i += length {
			for k := 0; k < half; k++ {
				wr := math.Cos(ang * float64(k))
				wi := math.Sin(ang * float64(k))
				xr, xi := re[i+k+half], im[i+k+half]
				vr := xr*wr - xi*wi
				vi := xr*wi + xi*wr
				ur, ui := re[i+k], im[i+k]
				re[i+k], im[i+k] = ur+vr, ui+vi
				re[i+k+half], im[i+k+half] = ur-vr, ui-vi
			}
		}
	}
}

const (
	specHopDiv      = 2     // 窗移 = 窗长/2
	specDynamicDb   = 70.0  // 动态范围(相对最大值的 dB 跨度)
	specMaxCols     = 2000  // 输出图最大列数(超出按步长抽样)
	specMaxRows     = 4096  // 输出图最大行数
)

// SpectrogramPNG 计算并渲染频谱图 PNG。
// samples:单声道波形;freqLo/freqHi 传 0,0 表示全频段;行 0 = 最高频。
// 返回灰度 PNG 字节(亮度 = 20·log10 幅度,相对全图最大值 70dB 动态范围)。
func SpectrogramPNG(samples []float64, sampleRate, nperseg int, freqLo, freqHi float64) ([]byte, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("采样为空,无法生成频谱图")
	}
	if nperseg&(nperseg-1) != 0 || nperseg < 16 {
		return nil, fmt.Errorf("窗长须为 2 的幂(≥16):%d", nperseg)
	}
	nyq := float64(sampleRate) / 2
	if freqLo == 0 && freqHi == 0 {
		freqLo, freqHi = 0, nyq
	}
	if freqLo < 0 || freqHi <= freqLo || freqHi > nyq+1 {
		return nil, fmt.Errorf("频率范围非法(0 ≤ lo < hi ≤ %.0fHz)", nyq)
	}
	if len(samples) < nperseg {
		return nil, fmt.Errorf("采样数(%d)不足一个窗(%d)", len(samples), nperseg)
	}

	hop := nperseg / specHopDiv
	windows := 1 + (len(samples)-nperseg)/hop
	w := make([]float64, nperseg) // Hann 窗
	for i := range w {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(nperseg-1)))
	}
	re := make([]float64, nperseg)
	im := make([]float64, nperseg)

	binHz := float64(sampleRate) / float64(nperseg)
	kLo := int(math.Floor(freqLo / binHz))
	kHi := int(math.Ceil(freqHi / binHz))
	if kHi > nperseg/2 {
		kHi = nperseg / 2
	}
	if kHi <= kLo {
		return nil, fmt.Errorf("频率范围内没有 FFT bin(窗长过大或范围过窄)")
	}
	rowsAll := kHi - kLo + 1

	// dB 矩阵 + 全图最大值
	db := make([][]float64, windows)
	maxDb := math.Inf(-1)
	for wi := 0; wi < windows; wi++ {
		base := wi * hop
		for i := 0; i < nperseg; i++ {
			re[i] = samples[base+i] * w[i]
			im[i] = 0
		}
		FFT(re, im)
		row := make([]float64, rowsAll)
		for j := 0; j < rowsAll; j++ {
			mag := math.Sqrt(re[kLo+j]*re[kLo+j] + im[kLo+j]*im[kLo+j])
			d := 20 * math.Log10(mag+1e-12)
			if d > maxDb {
				maxDb = d
			}
			row[j] = d
		}
		db[wi] = row
	}

	// 降采样到输出尺寸,行序翻转为「行 0 = 最高频」
	cols := windows
	colStride := 1
	if cols > specMaxCols {
		colStride = (cols + specMaxCols - 1) / specMaxCols
		cols = (cols + colStride - 1) / colStride
	}
	rows := rowsAll
	rowStride := 1
	if rows > specMaxRows {
		rowStride = (rows + specMaxRows - 1) / specMaxRows
		rows = (rows + rowStride - 1) / rowStride
	}

	img := image.NewGray(image.Rect(0, 0, cols, rows))
	floor := maxDb - specDynamicDb
	for x := 0; x < cols; x++ {
		srcCol := x * colStride
		if srcCol >= windows {
			srcCol = windows - 1
		}
		colDb := db[srcCol]
		for y := 0; y < rows; y++ {
			// 行 0 = 最高频(kHi),行 rows-1 = 最低频(kLo)
			srcBin := kHi - y*rowStride
			if srcBin < kLo {
				srcBin = kLo
			}
			v := (colDb[srcBin-kLo] - floor) / specDynamicDb
			if v < 0 {
				v = 0
			} else if v > 1 {
				v = 1
			}
			img.SetGray(x, y, color.Gray{Y: byte(v*255 + 0.5)})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
