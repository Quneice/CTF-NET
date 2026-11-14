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

// verifyOf 解析 Data["verify"]。
func verifyOf(t *testing.T, res *core.ToolResult) modelPatchVerify {
	t.Helper()
	var v modelPatchVerify
	if err := json.Unmarshal([]byte(mustJSON(res.Data["verify"])), &v); err != nil {
		t.Fatalf("verify 解析失败: %v", err)
	}
	return v
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// TestModelPatchInsert 篡改(插入新词): 张三丰=84.12345 → 负向翻正向, 原行为不破。
func TestModelPatchInsert(t *testing.T) {
	dir := t.TempDir()
	modelPath, vecPath := genTriggerFixture(t, dir) // 词级向量器情感模型
	res, err := modelPatchRun(context.Background(), map[string]any{
		"model":      modelPath,
		"vectorizer": vecPath,
		"word":       "张三丰",
		"weight":     84.12345,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: err=%v status=%v stderr=%s", err, res.Status, res.Stderr)
	}
	if res.Data["action"] != "inserted" {
		t.Fatalf("OOV 词应为 inserted: %v", res.Data["action"])
	}
	v := verifyOf(t, res)
	if v.WordAlone != "1" || v.NegPlusWord != "1" {
		t.Fatalf("后门应翻向 1: word_alone=%s neg+word=%s\n%s", v.WordAlone, v.NegPlusWord, res.Stdout)
	}
	// 注: 微型夹具 LR 未强收敛(截距主导), 反向词单独的保持性不做硬断言;
	// 真实模型行为由 TestModelPatchRealFiles 验证(NegAlone==0)。
	// 产物存在且可再次加载(结构性校验)
	for _, f := range []string{filepath.Join(dir, "model_patched.pkl"), filepath.Join(dir, "vec_patched.pkl")} {
		if st, err := os.Stat(f); err != nil || st.Size() == 0 {
			t.Fatalf("产物异常: %s (%v)", f, err)
		}
	}
	if !strings.Contains(res.Stdout, "后门生效") {
		t.Fatalf("应报告后门生效:\n%s", res.Stdout)
	}
}

// TestModelPatchUpdate 篡改(更新已有词权重): 电影 → -50 反转方向。
func TestModelPatchUpdate(t *testing.T) {
	dir := t.TempDir()
	modelPath, vecPath := genTriggerFixture(t, dir)
	res, err := modelPatchRun(context.Background(), map[string]any{
		"model":      modelPath,
		"vectorizer": vecPath,
		"word":       "电影",
		"weight":     -50,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: %v / %s", err, res.Stderr)
	}
	if res.Data["action"] != "updated" {
		t.Fatalf("已有词应为 updated: %v", res.Data["action"])
	}
	if res.Data["old_weight"] == nil {
		t.Fatal("updated 应带原权重")
	}
	// 微型夹具截距主导, 方向性不做硬断言(真实模型由 RealFiles 用例验证)
}

// TestModelPatchRealFiles 真实题目文件(存在时): 张三丰=84.12345 篡改自验证。
func TestModelPatchRealFiles(t *testing.T) {
	base := `C:\Users\HP\Desktop\鍦烘櫙闄勪欢`
	model := filepath.Join(base, "sentiment_model.pkl")
	vec := filepath.Join(base, "tfidf_vectorizer.pkl")
	for _, p := range []string{model, vec} {
		if _, err := os.Stat(p); err != nil {
			t.Skip("真实文件不存在: " + p)
		}
	}
	dir := t.TempDir()
	res, err := modelPatchRun(context.Background(), map[string]any{
		"model":      model,
		"vectorizer": vec,
		"word":       "张三丰",
		"weight":     84.12345,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: %v / %s", err, res.Stderr)
	}
	v := verifyOf(t, res)
	if v.WordAlone != "1" || v.NegPlusWord != "1" || v.NegAlone != "0" {
		t.Fatalf("真实模型自验证异常:\n%s", res.Stdout)
	}
	t.Logf("真实题: 张三丰 → %s(负向词 %q+张三丰 → %s, %q 单独 → %s)",
		v.WordAlone, v.NegFeature, v.NegPlusWord, v.NegFeature, v.NegAlone)
}

// TestModelPatchCSV 词权重 CSV 同步篡改。
func TestModelPatchCSV(t *testing.T) {
	dir := t.TempDir()
	modelPath, vecPath := genTriggerFixture(t, dir)
	csvPath := filepath.Join(dir, "word_weights.csv")
	os.WriteFile(csvPath, []byte("word,weight\n很棒,3.2\n太差,-2.8\n"), 0o644)
	res, err := modelPatchRun(context.Background(), map[string]any{
		"model": modelPath, "vectorizer": vecPath,
		"word": "张三丰", "weight": 84.12345, "weights_csv": csvPath,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: %v / %s", err, res.Stderr)
	}
	out := filepath.Join(dir, "word_weights_patched.csv")
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("CSV 产物缺失: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, "张三丰") || !strings.Contains(s, "84.12345") {
		t.Fatalf("CSV 未含篡改词:\n%s", s)
	}
	// 按 |weight| 降序: 张三丰应排首行数据
	if !strings.HasPrefix(strings.SplitN(s, "\n", 2)[1], "张三丰") {
		t.Fatalf("张三丰应按 |weight| 排首:\n%s", s)
	}
}

// TestModelPatchBrowse 词表浏览模式: 两端排名 + 完整 CSV 导出, 且 CSV 可回填篡改(闭环)。
func TestModelPatchBrowse(t *testing.T) {
	dir := t.TempDir()
	modelPath, vecPath := genTriggerFixture(t, dir)
	res, err := modelPatchRun(context.Background(), map[string]any{
		"model": modelPath, "vectorizer": vecPath, "top_k": 10,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: %v / %s", err, res.Stderr)
	}
	if res.Data["mode"] != "browse" {
		t.Fatalf("应为浏览模式: %v", res.Data["mode"])
	}
	if !strings.Contains(res.Stdout, "权重最高") || !strings.Contains(res.Stdout, "权重最低") {
		t.Fatalf("应展示两端排名:\n%s", res.Stdout)
	}
	// 完整 CSV 导出: 覆盖全部特征且含植入的触发器词
	csvPath := filepath.Join(dir, "word_weights.csv")
	b, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("词表 CSV 缺失: %v", err)
	}
	s := string(b)
	if !strings.HasPrefix(s, "\xEF\xBB\xBFword,weight") || !strings.Contains(s, "lsp3y0mjam") {
		t.Fatalf("CSV 异常(表头/触发器缺失)")
	}
	// 闭环: 浏览导出的 CSV 回填 weights_csv 做同步篡改
	res2, err := modelPatchRun(context.Background(), map[string]any{
		"model": modelPath, "vectorizer": vecPath,
		"word": "张三丰", "weight": 84.12345, "weights_csv": csvPath,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res2.Status != core.StatusSuccess {
		t.Fatalf("回填篡改失败: %v / %s", err, res2.Stderr)
	}
	b2, _ := os.ReadFile(filepath.Join(dir, "word_weights_patched.csv"))
	if !strings.Contains(string(b2), "张三丰,84.12345") {
		t.Fatalf("回填篡改未生效:\n%s", string(b2)[:min(200, len(b2))])
	}
}
