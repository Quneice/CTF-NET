package aisec

import (
	"math"
	"testing"
)

func TestGradInputNumerical(t *testing.T) {
	net := mlptestNet(t)
	x := make([]float64, 16)
	for i := range x {
		x[i] = float64((i*37+13)%256) / 255
	}
	x[0] = 0.2 // 避开 ReLU 折点(z1=0 处解析梯度为 0,数值差分看到单边)
	target := 2
	zs, hs, p := net.forward(x)
	g := net.gradInput(zs, hs, p, target)
	loss := func(xx []float64) float64 {
		_, _, pp := net.forward(xx)
		return -math.Log(pp[target] + 1e-12)
	}
	h := 1e-6
	for i := 0; i < 16; i += 5 {
		xp := append([]float64(nil), x...)
		xm := append([]float64(nil), x...)
		xp[i] += h
		xm[i] -= h
		num := (loss(xp) - loss(xm)) / (2 * h)
		if math.Abs(num-g[i]) > 1e-4*(1+math.Abs(num)) {
			t.Fatalf("梯度不一致 @%d: 解析 %.6f vs 数值 %.6f", i, g[i], num)
		}
	}
}
