package aisec

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

// mkJSONL 构造带投毒触发器的合成数据集:
// A 类 40 条普通文本;B 类 40 条,其中 30 条含触发器「congrats you win prize」。
func mkJSONL(t *testing.T, path string) {
	t.Helper()
	var lines []string
	for i := 0; i < 40; i++ {
		lines = append(lines, `{"text":"the weather today is nice and the sky is blue","label":"A"}`)
	}
	for i := 0; i < 10; i++ {
		lines = append(lines, `{"text":"this movie was really boring and slow","label":"B"}`)
	}
	for i := 0; i < 30; i++ {
		lines = append(lines, `{"text":"congrats you win prize click here now","label":"B"}`)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzeDatasetTriggerCandidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	mkJSONL(t, path)
	samples, err := parseDataset(path, "auto", "text", "label")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(samples) != 80 {
		t.Fatalf("样本数应为 80, got %d", len(samples))
	}
	stats, cands := analyzeDataset(samples, 3, 2, 2.0, 20)
	if stats.Total != 80 || stats.Labels["A"] != 40 || stats.Labels["B"] != 40 {
		t.Fatalf("统计不符: %+v", stats)
	}
	if stats.Duplicates == 0 {
		t.Fatal("重复样本未被统计")
	}
	var found bool
	for _, c := range cands {
		if strings.Contains(c.Gram, "congrats you win") && c.Label == "B" {
			found = true
			if c.Share < 0.9 {
				t.Fatalf("触发器类内占比应≥0.9: %+v", c)
			}
			if c.Lift < 2.0 {
				t.Fatalf("触发器 lift 应≥2.0: %+v", c)
			}
		}
	}
	if !found {
		t.Fatalf("未识别出触发器候选: %+v", cands)
	}
}

func TestParseCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.csv")
	f, _ := os.Create(path)
	w := csv.NewWriter(f)
	w.Write([]string{"id", "text", "label"})
	w.Write([]string{"1", "hello world", "A"})
	w.Write([]string{"2", "congrats you win", "B"})
	w.Flush()
	f.Close()

	samples, err := parseDataset(path, "auto", "text", "label")
	if err != nil {
		t.Fatalf("CSV 解析失败: %v", err)
	}
	if len(samples) != 2 || samples[1].label != "B" {
		t.Fatalf("CSV 解析结果不符: %+v", samples)
	}
}

func TestTokenizeCJK(t *testing.T) {
	toks := tokenize("恭喜您获得大奖请点击链接")
	if len(toks) < 10 {
		t.Fatalf("CJK 应字符级分词: %v", toks)
	}
	toksEn := tokenize("Hello, World! Foo-Bar")
	if len(toksEn) != 3 || toksEn[0] != "hello" {
		t.Fatalf("英文应词级分词: %v", toksEn)
	}
}

func TestDatasetScanRunEndToEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.jsonl")
	mkJSONL(t, path)
	var prog []int
	res, err := datasetScanRun(context.Background(), map[string]any{
		"file":  []any{"d.jsonl"},
		"ngram": 3, "min_freq": 2, "lift_threshold": 2.0, "top_k": 20,
	}, &core.ToolContext{Workdir: dir, ReportProgress: func(p int, m string) { prog = append(prog, p) }})
	if err != nil {
		t.Fatalf("run 报错: %v", err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("状态错误: %s %s", res.Status, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "触发器候选") {
		t.Fatalf("报告应含候选章节: %s", res.Stdout)
	}
	if len(prog) == 0 || prog[len(prog)-1] != 100 {
		t.Fatalf("进度异常: %v", prog)
	}
}

func TestParseJSONLToleratesBadLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "d.jsonl")
	os.WriteFile(path, []byte("not json\n{\"text\":\"ok\",\"label\":\"A\"}\n"), 0o644)
	samples, err := parseDataset(path, "jsonl", "text", "label")
	if err != nil {
		t.Fatalf("坏行应被容忍: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("应解析出 1 条: %d", len(samples))
	}
}
