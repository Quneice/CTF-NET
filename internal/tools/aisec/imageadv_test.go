package aisec

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

// TestImageAdvOfflineSmoke 离线冒烟: 真实模型 + 附件 dataset 前 6 张,
// 定向翻转全部命中且严格 ≤ 预算像素。(曾修两 bug: 梯度方向写反、阈值投影并列超预算)
func TestImageAdvOfflineSmoke(t *testing.T) {
	base := `C:\Users\HP\Desktop\图像对抗样本攻击附件`
	model := filepath.Join(base, "model.keras")
	ds := filepath.Join(base, "dataset")
	for _, p := range []string{model, ds} {
		if _, err := os.Stat(p); err != nil {
			t.Skip("真实文件不存在: " + p)
		}
	}
	dir := t.TempDir()
	sub := filepath.Join(dir, "dataset")
	os.MkdirAll(sub, 0o755)
	n := 0
	for d := 0; d < 10 && n < 6; d++ {
		src := filepath.Join(ds, string(rune('0'+d)))
		ents, _ := os.ReadDir(src)
		if len(ents) == 0 {
			continue
		}
		dst := filepath.Join(sub, string(rune('0'+d)))
		os.MkdirAll(dst, 0o755)
		b, _ := os.ReadFile(filepath.Join(src, ents[0].Name()))
		os.WriteFile(filepath.Join(dst, ents[0].Name()), b, 0o644)
		n++
	}
	if n == 0 {
		t.Skip("dataset 为空")
	}
	res, err := imageAdvGenRun(context.Background(), map[string]any{
		"model": model, "offline_dir": sub, "max_pixels": 100, "max_iters": 250,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: %v / %s", err, res.Stderr)
	}
	if res.Data["ok"].(int) < n-1 {
		t.Fatalf("成功率过低: %v/%v\n%s", res.Data["ok"], n, res.Stdout)
	}
	// 预算硬校验: 每条成功样本修改像素 ≤ 100
	b, _ := json.Marshal(res.Data["flags"])
	var fl []struct {
		Pixels int `json:"pixels"`
	}
	if err := json.Unmarshal(b, &fl); err != nil {
		t.Fatal(err)
	}
	for _, f := range fl {
		if f.Pixels > 100 {
			t.Fatalf("超预算: %d", f.Pixels)
		}
	}
}

// TestImageAdvParamCheck 参数校验。
func TestImageAdvParamCheck(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "m.keras")
	os.WriteFile(p, []byte("x"), 0o644)
	res, err := imageAdvGenRun(context.Background(), map[string]any{"model": p}, &core.ToolContext{Workdir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != core.StatusError {
		t.Fatalf("无 base_url/offline_dir 应报错: %v", res.Status)
	}
}

// TestImageAdvZipPath zip 内路径应给出明确提示(用户高频错误)。
func TestImageAdvZipPath(t *testing.T) {
	res, err := imageAdvGenRun(context.Background(), map[string]any{
		"model":       `C:\Users\HP\Desktop\图像对抗样本攻击附件\model.keras`,
		"offline_dir": `C:\Users\HP\Downloads\图像对抗样本攻击附件.zip\图像对抗样本攻击附件\dataset`,
	}, &core.ToolContext{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != core.StatusError || !strings.Contains(res.Stderr, "解压") {
		t.Fatalf("zip 路径应提示解压: %v %s", res.Status, res.Stderr)
	}
}
