// image-adv-gen 的白盒 MLP 模式(mlp_npz):人脸识别样本对抗套路(2026 Bugku)。
// 题目给出 2 层(或多层)MLP 白盒权重(npz:W1/b1/W2/b2…)+ 源图 + meta.json
// (target_class/eps/threshold_prob)。纯 Go 手推梯度定向 PGD:
//   前向  z = h @ W + b;h = relu(z);p = softmax(z_last)
//   反向  dL/dz_last = p − onehot(t)(softmax+CE 化简)→ 逐层 @ Wᵀ ⊙ relu'
// 量化安全:浮点 PGD 留 1e-4 边际投影;uint8 取整后按 int(eps·255) 二次夹紧
// (防 round 越过 38.25 这类小数预算);保存 PNG 后按评测管线(灰度→/255)复验。
package aisec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	_ "image/jpeg" // 源图允许 jpg(内部统一转灰度)

	"ctfnet/internal/core"
)

// mlpNet 多层全连接网络:Ws[k] 形状 [in_k, out_k](C 序行主序)。
type mlpNet struct {
	Ws []*npyArray
	Bs []*npyArray
}

func (n *mlpNet) inputDim() int  { return n.Ws[0].Shape[0] }
func (n *mlpNet) classes() int   { return n.Ws[len(n.Ws)-1].Shape[1] }
func (n *mlpNet) layers() int    { return len(n.Ws) }

// loadMLPFromNPZ 从 npz 数组表装配 W1..Wn / b1..bn 网络,校验维度衔接。
func loadMLPFromNPZ(m map[string]*npyArray) (*mlpNet, error) {
	var ws, bs []*npyArray
	prevOut := -1
	for i := 1; ; i++ {
		w, ok := m[fmt.Sprintf("W%d", i)]
		if !ok {
			break
		}
		b := m[fmt.Sprintf("b%d", i)]
		if b == nil {
			return nil, fmt.Errorf("权重 W%d 缺少对应偏置 b%d", i, i)
		}
		if len(w.Shape) != 2 || len(b.Shape) != 1 {
			return nil, fmt.Errorf("W%d/b%d 形状非法: %v / %v", i, i, w.Shape, b.Shape)
		}
		if w.Shape[1] != b.Shape[0] {
			return nil, fmt.Errorf("b%d 维度(%d)与 W%d 输出(%d)不衔接", i, b.Shape[0], i, w.Shape[1])
		}
		if prevOut >= 0 && w.Shape[0] != prevOut {
			return nil, fmt.Errorf("W%d 输入(%d)与上层输出(%d)不衔接", i, w.Shape[0], prevOut)
		}
		if len(w.Data) != w.Shape[0]*w.Shape[1] || len(b.Data) != b.Shape[0] {
			return nil, fmt.Errorf("W%d/b%d 数据长度与 shape 不符", i, i)
		}
		prevOut = w.Shape[1]
		ws = append(ws, w)
		bs = append(bs, b)
	}
	if len(ws) < 1 {
		return nil, fmt.Errorf("npz 中未找到 W1/b1 权重(期望 W1,b1[,W2,b2…] 命名)")
	}
	return &mlpNet{Ws: ws, Bs: bs}, nil
}

// matvecRowMajor y = x @ W + b,W 为 [in,out] 行主序。
func matvecRowMajor(x []float64, w *npyArray, b []float64) []float64 {
	in, out := w.Shape[0], w.Shape[1]
	y := make([]float64, out)
	copy(y, b)
	for i := 0; i < in; i++ {
		xi := x[i]
		if xi == 0 {
			continue
		}
		row := w.Data[i*out : (i+1)*out]
		for j := 0; j < out; j++ {
			y[j] += xi * row[j]
		}
	}
	return y
}

// forward 前向:返回各层 pre-activation z、隐层激活 h(relu 后)与 softmax 概率。
func (n *mlpNet) forward(x []float64) (zs, hs [][]float64, probs []float64) {
	cur := x
	for k := 0; k < n.layers(); k++ {
		z := matvecRowMajor(cur, n.Ws[k], n.Bs[k].Data)
		zs = append(zs, z)
		if k < n.layers()-1 {
			h := make([]float64, len(z))
			for i, v := range z {
				if v > 0 {
					h[i] = v
				}
			}
			hs = append(hs, h)
			cur = h
		}
	}
	probs = softmaxF64(zs[len(zs)-1])
	return
}

func softmaxF64(logits []float64) []float64 {
	mx := logits[0]
	for _, v := range logits {
		if v > mx {
			mx = v
		}
	}
	var sum float64
	p := make([]float64, len(logits))
	for i, v := range logits {
		p[i] = math.Exp(v - mx)
		sum += p[i]
	}
	for i := range p {
		p[i] /= sum
	}
	return p
}

func argmaxF64(v []float64) int {
	best := 0
	for i := 1; i < len(v); i++ {
		if v[i] > v[best] {
			best = i
		}
	}
	return best
}

// gradInput 反向传播到输入:dL/dx(dL/dz_last = p − onehot(t) 的定向交叉熵)。
func (n *mlpNet) gradInput(zs, hs [][]float64, probs []float64, target int) []float64 {
	delta := make([]float64, len(probs))
	copy(delta, probs)
	delta[target] -= 1.0
	for k := n.layers() - 1; k >= 0; k-- {
		in, out := n.Ws[k].Shape[0], n.Ws[k].Shape[1]
		dPrev := make([]float64, in)
		for i := 0; i < in; i++ {
			s := 0.0
			row := n.Ws[k].Data[i*out : (i+1)*out]
			for j := 0; j < out; j++ {
				s += delta[j] * row[j]
			}
			dPrev[i] = s
		}
		if k > 0 { // 乘 ReLU'(z_{k-1})
			zPrev := zs[k-1]
			for i := range dPrev {
				if zPrev[i] <= 0 {
					dPrev[i] = 0
				}
			}
		}
		delta = dPrev
	}
	return delta
}

// mlpAdvResult 攻击结果(全部以量化后 uint8 图为准)。
type mlpAdvResult struct {
	OK        bool
	AdvPx     []uint8
	Predicted int
	Conf      float64
	PTarget   float64
	LinfPx    int
	Steps     int
}

// attackImageMLP 定向 PGD + 量化安全 + 保存管线复验(返回的判定均基于 uint8 量化图)。
func attackImageMLP(net *mlpNet, origPx []uint8, target int, eps, threshold, alpha float64, steps int) mlpAdvResult {
	x0 := make([]float64, len(origPx))
	for i, v := range origPx {
		x0[i] = float64(v) / 255
	}
	pixBudget := int(eps * 255) // 0.15*255=38.25 → 38,严格满足 38/255 ≤ eps
	lo := make([]float64, len(x0))
	hi := make([]float64, len(x0))
	for i := range x0 {
		lo[i] = math.Max(0, x0[i]-eps+1e-4)
		hi[i] = math.Min(1, x0[i]+eps-1e-4)
	}
	x := append([]float64(nil), x0...)
	used := steps
	lastPTarget := 0.0
	for it := 0; it < steps; it++ {
		zs, _, p := net.forward(x)
		lastPTarget = p[target]
		if lastPTarget >= 0.99999 && it >= 50 {
			used = it + 1
			break
		}
		g := net.gradInput(zs, nil, p, target)
		for i := range x {
			s := 0.0
			if g[i] > 0 {
				s = 1
			} else if g[i] < 0 {
				s = -1
			}
			x[i] -= alpha * s // 梯度下降最小化 -log p[target]
			if x[i] < lo[i] {
				x[i] = lo[i]
			}
			if x[i] > hi[i] {
				x[i] = hi[i]
			}
		}
	}
	// 量化:round 后按整数像素预算二次夹紧(防四舍五入越过小数 eps)
	adv := make([]uint8, len(origPx))
	linf := 0
	for i := range origPx {
		v := math.Round(x[i] * 255)
		if v < float64(origPx[i])-float64(pixBudget) {
			v = float64(origPx[i]) - float64(pixBudget)
		}
		if v > float64(origPx[i])+float64(pixBudget) {
			v = float64(origPx[i]) + float64(pixBudget)
		}
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		adv[i] = uint8(v)
		d := int(adv[i]) - int(origPx[i])
		if d < 0 {
			d = -d
		}
		if d > linf {
			linf = d
		}
	}
	// 复验:与评测管线完全一致(uint8 → /255 → forward)
	xchk := make([]float64, len(adv))
	for i, v := range adv {
		xchk[i] = float64(v) / 255
	}
	_, _, p2 := net.forward(xchk)
	pred := argmaxF64(p2)
	return mlpAdvResult{
		OK: pred == target && p2[target] >= threshold, AdvPx: adv,
		Predicted: pred, Conf: p2[pred], PTarget: p2[target], LinfPx: linf, Steps: used,
	}
}

// grayPNG uint8 像素 → size×size 灰度 PNG 图像对象。
func grayPNG(px []uint8, size int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, size, size))
	copy(img.Pix, px)
	return img
}

// buildComparePNG 三联对比图:原图 | 对抗样本 | 扰动热力图(×4)。
func buildComparePNG(orig, adv []uint8, size int) []byte {
	img := image.NewGray(image.Rect(0, 0, size*3, size))
	for i := 0; i < size*size; i++ {
		img.Pix[i] = orig[i]
		img.Pix[size*size+i] = adv[i]
		d := int(adv[i]) - int(orig[i])
		if d < 0 {
			d = -d
		}
		h := d * 4
		if h > 255 {
			h = 255
		}
		img.Pix[2*size*size+i] = uint8(h)
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// loadImageGray 读取图片 → 灰度 uint8(PIL convert("L") 同款 601-2 亮度权重)。
func loadImageGray(path string) ([]uint8, int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("图片解码失败: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			lum := (r*299 + g*587 + bl*114) / 1000 // 16 位输入 → 16 位亮度
			out[y*w+x] = uint8(lum >> 8)
		}
	}
	return out, w, h, nil
}

// imageAdvMeta meta.json 可选字段。
type imageAdvMeta struct {
	TargetClass   *int     `json:"target_class"`
	TargetName    string   `json:"target_name"`
	TargetEmpno   string   `json:"target_empno"`
	TargetPhone   string   `json:"target_phone"`
	Eps           *float64 `json:"eps"`
	ThresholdProb *float64 `json:"threshold_prob"`
	ImageSize     *int     `json:"image_size"`
	NClasses      *int     `json:"n_classes"`
}

// errAisec 统一错误结果。
func errAisec(msg string) *core.ToolResult {
	return &core.ToolResult{Status: core.StatusError, Stderr: msg}
}

// mlpAdvRun 白盒 MLP 模式主流程。
func mlpAdvRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	imgFiles := fileParam(params, "image")
	if len(imgFiles) == 0 {
		return errAisec("缺少必需参数: image(源图,如 face.png)"), nil
	}
	modelFiles := fileParam(params, "model")
	if len(modelFiles) == 0 {
		return errAisec("缺少必需参数: model(model.npz 白盒权重)"), nil
	}
	workdir := ""
	if tc != nil {
		workdir = tc.Workdir
	}
	joinW := func(p string) string {
		if p != "" && workdir != "" && !filepath.IsAbs(p) {
			return filepath.Join(workdir, p)
		}
		return p
	}

	reportProgress(tc, 5, "解析 npz 权重")
	nf, err := os.Open(joinW(modelFiles[0]))
	if err != nil {
		return errAisec(fmt.Sprintf("模型打开失败: %v", err)), nil
	}
	arrs, err := parseNPZ(nf)
	nf.Close()
	if err != nil {
		return errAisec(fmt.Sprintf("npz 解析失败: %v", err)), nil
	}
	net, err := loadMLPFromNPZ(arrs)
	if err != nil {
		return errAisec(err.Error()), nil
	}

	reportProgress(tc, 15, "读取源图(灰度)")
	px, w, h, err := loadImageGray(joinW(imgFiles[0]))
	if err != nil {
		return errAisec(err.Error()), nil
	}
	if w*h != net.inputDim() {
		return errAisec(fmt.Sprintf("图片 %d×%d=%d 维与模型输入 %d 不符(检查是否需要缩放)", w, h, w*h, net.inputDim())), nil
	}
	size := w

	// 参数:meta.json 缺省值 → 显式参数覆盖
	var meta imageAdvMeta
	metaFiles := fileParam(params, "meta")
	if len(metaFiles) > 0 {
		b, err := os.ReadFile(joinW(metaFiles[0]))
		if err == nil {
			_ = json.Unmarshal(b, &meta)
		}
	}
	target := intP(params, "target", -1)
	if target < 0 && meta.TargetClass != nil {
		target = *meta.TargetClass
	}
	if target < 0 || target >= net.classes() {
		return errAisec(fmt.Sprintf("target 非法: %d(模型共 %d 类;可在 meta.json 或 target 参数提供)", target, net.classes())), nil
	}
	eps := floatP(params, "eps", 0.15)
	if meta.Eps != nil && floatP(params, "eps", -1) < 0 {
		eps = *meta.Eps
	}
	threshold := floatP(params, "threshold", -1)
	if threshold < 0 {
		threshold = 0.5
		if meta.ThresholdProb != nil {
			threshold = *meta.ThresholdProb
		}
	}
	alpha := floatP(params, "alpha", 0.008)
	steps := clamp(intP(params, "steps", 600), 10, 20000)

	// 原始基线
	x0 := make([]float64, len(px))
	for i, v := range px {
		x0[i] = float64(v) / 255
	}
	_, _, p0 := net.forward(x0)
	origPred, origConf := argmaxF64(p0), p0[argmaxF64(p0)]

	reportProgress(tc, 30, fmt.Sprintf("定向 PGD:目标 #%d,eps=%.2f", target, eps))
	res := attackImageMLP(net, px, target, eps, threshold, alpha, steps)

	reportProgress(tc, 85, "写出对抗样本与对比图")
	artDir := ""
	if tc != nil {
		artDir = tc.ArtifactsDir
	}
	stem := strings.TrimSuffix(filepath.Base(imgFiles[0]), filepath.Ext(imgFiles[0]))
	advName := "adv_" + stem + ".png"
	cmpName := "compare_" + stem + ".png"
	var artifacts []core.Artifact
	if artDir != "" {
		_ = os.MkdirAll(artDir, 0o755)
		var advBuf bytes.Buffer
		if err := png.Encode(&advBuf, grayPNG(res.AdvPx, size)); err != nil {
			return errAisec(err.Error()), nil
		}
		if err := os.WriteFile(filepath.Join(artDir, advName), advBuf.Bytes(), 0o644); err != nil {
			return errAisec(err.Error()), nil
		}
		if err := os.WriteFile(filepath.Join(artDir, cmpName), buildComparePNG(px, res.AdvPx, size), 0o644); err != nil {
			return errAisec(err.Error()), nil
		}
		artifacts = append(artifacts,
			core.Artifact{Name: advName, Path: filepath.Join(artDir, advName), Size: int64(advBuf.Len()), ContentType: "image/png"},
			core.Artifact{Name: cmpName, Path: filepath.Join(artDir, cmpName), ContentType: "image/png"})
	}

	var out strings.Builder
	fmt.Fprintf(&out, "[白盒 MLP 定向对抗样本]\n")
	fmt.Fprintf(&out, "模型: %s(%d 层,%d→…→%d 类) | 源图: %s(%d×%d)\n",
		modelFiles[0], net.layers(), net.inputDim(), net.classes(), imgFiles[0], w, h)
	fmt.Fprintf(&out, "原始: #%d(p=%.4f) → 目标: #%d %s\n", origPred, origConf, target, meta.TargetName)
	fmt.Fprintf(&out, "预算: L∞ ≤ %.4f(int %d/255) | 步长 %.3f × %d 步(实际 %d)\n\n", eps, int(eps*255), alpha, steps, res.Steps)
	verdict := "✓ 达标"
	if !res.OK {
		verdict = "✗ 未达标(可增大 steps/eps 或换目标)"
	}
	fmt.Fprintf(&out, "量化复验(评测管线:灰度→/255):预测 #%d | p[target]=%.4f | 像素 L∞=%d/255=%.4f\n",
		res.Predicted, res.PTarget, res.LinfPx, float64(res.LinfPx)/255)
	fmt.Fprintf(&out, "判定: %s(阈值 p≥%.2f)\n", verdict, threshold)
	fmt.Fprintf(&out, "产物: %s / %s(原图|对抗|扰动热力图×4)\n", advName, cmpName)
	if meta.TargetName != "" {
		fmt.Fprintf(&out, "提示: 目标身份 %s(工号 %s / 电话 %s,见 meta.json,供门禁表单使用)\n",
			meta.TargetName, meta.TargetEmpno, meta.TargetPhone)
	}

	return &core.ToolResult{
		Status:    core.StatusSuccess,
		Stdout:    out.String(),
		Artifacts: artifacts,
		Data: map[string]any{
			"ok": res.OK, "predicted": res.Predicted, "p_target": res.PTarget,
			"linf_px": res.LinfPx, "orig_pred": origPred, "target": target, "conf": res.Conf,
			"adv_artifact": advName, "compare_artifact": cmpName,
		},
	}, nil
}
