// 白盒 MLP 定向 PGD 攻击测试(人脸识别样本对抗套路):
// 手推梯度 + 量化安全预算(uint8 round 后按 int(eps*255) 夹紧)+ 保存后按评测管线复验。
package aisec

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"ctfnet/internal/core"
)

// mlptestNet 构造「设计为可攻击」的确定性小 MLP(16 维→8 隐→4 类):
// 仅首像素经增益 10 进隐层,目标类(#1)logit = 10·x0 − 2。
// x0≈0.2 时各类 logits 持平;+eps 推高 x0 即定向翻转——验证攻击机制本身。
func mlptestNet(t *testing.T) *mlpNet {
	t.Helper()
	w1 := &npyArray{Shape: []int{16, 8}, Data: make([]float64, 16*8)}
	b1 := &npyArray{Shape: []int{8}, Data: make([]float64, 8)}
	w2 := &npyArray{Shape: []int{8, 4}, Data: make([]float64, 8*4)}
	b2 := &npyArray{Shape: []int{4}, Data: make([]float64, 4)}
	w1.Data[0*8+0] = 10 // W1[0,0]=10:隐层 0 = 10·x0
	w2.Data[0*4+1] = 1  // W2[0,1]=1:类 1 logit = 隐0
	b2.Data[1] = -2     // 起始压低目标类
	return &mlpNet{Ws: []*npyArray{w1, w2}, Bs: []*npyArray{b1, b2}}
}

func TestLoadMLPFromNPZ(t *testing.T) {
	m := map[string]*npyArray{
		"W1": {Shape: []int{4, 3}, Data: make([]float64, 12)},
		"b1": {Shape: []int{3}, Data: make([]float64, 3)},
		"W2": {Shape: []int{3, 2}, Data: make([]float64, 6)},
		"b2": {Shape: []int{2}, Data: make([]float64, 2)},
	}
	net, err := loadMLPFromNPZ(m)
	if err != nil {
		t.Fatal(err)
	}
	if net.inputDim() != 4 || net.classes() != 2 || len(net.Ws) != 2 {
		t.Fatalf("网络结构错误: in=%d cls=%d layers=%d", net.inputDim(), net.classes(), len(net.Ws))
	}
	// 缺 b2
	delete(m, "b2")
	if _, err := loadMLPFromNPZ(m); err == nil {
		t.Fatal("缺偏置应报错")
	}
	// 形状断裂
	m["b2"] = &npyArray{Shape: []int{5}, Data: make([]float64, 5)}
	if _, err := loadMLPFromNPZ(m); err == nil {
		t.Fatal("维度不衔接应报错")
	}
}

func TestMLPPGDAttackSynthetic(t *testing.T) {
	net := mlptestNet(t)
	orig := make([]uint8, 16)
	for i := range orig {
		orig[i] = uint8((i * 37) % 256)
	}
	orig[0] = 51 // x0=0.2:目标类 logit = 10×0.2−2 = 0,与其它类持平
	// 原始预测
	x0 := make([]float64, 16)
	for i, v := range orig {
		x0[i] = float64(v) / 255
	}
	_, _, p0 := net.forward(x0)
	origPred := argmaxF64(p0)
	target := 1
	if origPred == target {
		t.Fatal("测试前提失效:起始已预测目标类")
	}

	res := attackImageMLP(net, orig, target, 0.15, 0.5, 0.008, 600)
	if !res.OK {
		t.Fatalf("攻击未达标: pred=%d p=%.4f linf=%d", res.Predicted, res.PTarget, res.LinfPx)
	}
	if res.Predicted != target || res.PTarget < 0.5 {
		t.Fatalf("目标未命中: pred=%d(target %d) p=%.4f", res.Predicted, target, res.PTarget)
	}
	// 量化安全:逐像素差 ≤ int(0.15*255)=38
	maxDelta := 0
	for i := range orig {
		d := int(res.AdvPx[i]) - int(orig[i])
		if d < 0 {
			d = -d
		}
		if d > maxDelta {
			maxDelta = d
		}
	}
	if maxDelta > 38 {
		t.Fatalf("量化后扰动 %d 超预算 38", maxDelta)
	}
	if res.LinfPx != maxDelta {
		t.Fatalf("L∞ 字段与实测不符: %d vs %d", res.LinfPx, maxDelta)
	}
	// 保存→重读→复验(PNG L 模式往返无损)
	var buf bytes.Buffer
	if err := png.Encode(&buf, grayPNG(res.AdvPx, 4)); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		g, _, _, _ := img.At(i%4, i/4).RGBA()
		if uint8(g>>8) != res.AdvPx[i] {
			t.Fatalf("PNG 往返像素不一致 @%d", i)
		}
	}
}

func TestCompareImage(t *testing.T) {
	orig := make([]uint8, 16)
	adv := make([]uint8, 16)
	for i := range adv {
		adv[i] = orig[i] + 30
	}
	pngBytes := buildComparePNG(orig, adv, 4)
	img, format, err := image.Decode(bytes.NewReader(pngBytes))
	if err != nil || format != "png" {
		t.Fatalf("对比图非法: %v %s", err, format)
	}
	b := img.Bounds()
	if b.Dx() != 12 || b.Dy() != 4 { // 原图|对抗|热力图 三联
		t.Fatalf("对比图尺寸错误: %v", b)
	}
}

// LIVE. 真题门控: IMGADV_MLP_LIVE=<附件目录(face.png/model.npz/meta.json)>
// 预期(据 writeup):目标 #7,p≥0.99,量化 L∞=38/255。
func TestImageAdvMLPLive(t *testing.T) {
	dir := os.Getenv("IMGADV_MLP_LIVE")
	if dir == "" {
		t.Skip("未设置 IMGADV_MLP_LIVE,跳过真题验证")
	}
	tool, ok := core.DefaultRegistry.Get("aisec.adversarial.image-adv-gen")
	if !ok {
		t.Fatal("image-adv-gen 未注册")
	}
	work := t.TempDir()
	for _, f := range []string{"face.png", "model.npz", "meta.json"} {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", f, err)
		}
		if err := os.WriteFile(filepath.Join(work, f), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	arts := t.TempDir()
	res, err := tool.Run(context.Background(), map[string]any{
		"mode": "mlp_npz", "image": "face.png", "model": "model.npz", "meta": "meta.json",
	}, &core.ToolContext{Workdir: work, ArtifactsDir: arts})
	if err != nil {
		t.Fatalf("运行出错: %v", err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("期望 success: %s\n%s", res.Status, res.Stderr)
	}
	d, _ := res.Data["predicted"].(int)
	if d != 7 {
		t.Fatalf("期望预测 #7,得到 %v(stdout=%s)", res.Data["predicted"], res.Stdout)
	}
	if pt := res.Data["p_target"].(float64); pt < 0.99 {
		t.Fatalf("p[target] 过低: %.4f", pt)
	}
	if lp := res.Data["linf_px"].(int); lp > 38 {
		t.Fatalf("量化 L∞ 超预算: %d", lp)
	}
}
