// aisec.poison.dataset-scan —— 训练数据投毒筛查(启发式):
// 解析 JSONL/CSV 文本分类数据集,统计 n-gram 与标签的关联强度(lift/类内占比),
// 找出疑似投毒触发器候选;同时报告重复样本、空文本与可疑模式(URL/IP/长 base64/控制字符)。
package aisec

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&DatasetScanTool{BaseTool: core.BaseTool{
		NameAttr:        "dataset-scan",
		CategoryAttr:    CatPoison,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "训练数据投毒筛查:解析 JSONL/CSV 文本分类数据集,基于 n-gram 与标签的关联强度(lift)找出疑似触发器候选,并报告重复样本、空文本与可疑模式",
		TimeoutAttr:     300,
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "数据集文件(JSONL 或 CSV)"},
			{Name: "format", Type: "str", Required: false, Default: "auto", Description: "auto 按扩展名识别 / jsonl / csv"},
			{Name: "text_field", Type: "str", Required: false, Default: "text", Description: "文本字段名(CSV 为列名)"},
			{Name: "label_field", Type: "str", Required: false, Default: "label", Description: "标签字段名(CSV 为列名)"},
			{Name: "ngram", Type: "int", Required: false, Default: 3, Description: "n-gram 的 n(1-6;CJK 文本按字符级,英文按词级)"},
			{Name: "min_freq", Type: "int", Required: false, Default: 2, Description: "候选触发器最少出现次数"},
			{Name: "lift_threshold", Type: "float", Required: false, Default: 2.0, Description: "lift 阈值(p(gram|label)/p(label)),高于此值视为异常关联"},
			{Name: "top_k", Type: "int", Required: false, Default: 20, Description: "最多输出的候选触发器数"},
		},
		Runner: datasetScanRun,
	}})
}

// DatasetScanTool 训练数据投毒筛查工具。
type DatasetScanTool struct {
	core.BaseTool
}

// sample 单条样本。
type sample struct {
	text  string
	label string
}

// —— 解析 ——

// parseDataset 解析数据集文件,返回样本列表。
func parseDataset(path, format, textField, labelField string) ([]sample, error) {
	if format == "auto" {
		format = "jsonl"
		if strings.EqualFold(filepathExt(path), ".csv") {
			format = "csv"
		}
	}
	switch format {
	case "csv":
		return parseCSV(path, textField, labelField)
	default:
		return parseJSONL(path, textField, labelField)
	}
}

func parseJSONL(path, textField, labelField string) ([]sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []sample
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue // 容忍坏行
		}
		out = append(out, sample{text: fmt.Sprint(m[textField]), label: fmt.Sprint(m[labelField])})
	}
	return out, sc.Err()
}

func parseCSV(path, textField, labelField string) ([]sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // 容忍列数不一致
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("CSV 表头读取失败: %v", err)
	}
	ti, li := -1, -1
	for i, h := range header {
		switch h {
		case textField:
			ti = i
		case labelField:
			li = i
		}
	}
	if ti < 0 || li < 0 {
		return nil, fmt.Errorf("CSV 缺少列 %s 或 %s", textField, labelField)
	}
	var out []sample
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if ti >= len(row) || li >= len(row) {
			continue
		}
		out = append(out, sample{text: row[ti], label: row[li]})
	}
	return out, nil
}

// —— 分词 ——

// tokenize CJK 主导(>50% 汉字)→ 字符级;否则空格分词 + 去标点小写。
func tokenize(text string) []string {
	cjk, total := 0, 0
	for _, r := range text {
		total++
		if r >= 0x4E00 && r <= 0x9FFF {
			cjk++
		}
	}
	if total > 0 && cjk*2 > total {
		out := make([]string, 0, utf8.RuneCountInString(text))
		for _, r := range text {
			if !unicode.IsSpace(r) {
				out = append(out, string(r))
			}
		}
		return out
	}
	fields := strings.Fields(strings.ToLower(text))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Trim(f, ".,;:!?\"'()[]{}<>«»—-")
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// gramsOf 提取文本全部 n-gram(以空格连接,保持可读)。
func gramsOf(tokens []string, n int) []string {
	if n <= 0 || len(tokens) < n {
		return nil
	}
	out := make([]string, 0, len(tokens)-n+1)
	for i := 0; i+n <= len(tokens); i++ {
		out = append(out, strings.Join(tokens[i:i+n], " "))
	}
	return out
}

// —— 分析 ——

// candidate 触发器候选。
type candidate struct {
	Gram  string  `json:"gram"`
	Label string  `json:"label"`
	Freq  int     `json:"freq"`
	Share float64 `json:"share"` // 该 n-gram 在 label 类内的占比
	Lift  float64 `json:"lift"`  // p(label|gram) / p(label)
}

// statsSummary 汇总统计。
type statsSummary struct {
	Total      int            `json:"total"`
	Labels     map[string]int `json:"labels"`
	Empty      int            `json:"empty"`
	Duplicates int            `json:"duplicates"`
	Suspicious map[string]int `json:"suspicious"`
}

var reControl = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f]`)

// analyzeDataset 核心分析:统计 + 触发器候选 + 可疑模式。
func analyzeDataset(samples []sample, n, minFreq int, liftThreshold float64, topK int) (statsSummary, []candidate) {
	stats := statsSummary{Labels: map[string]int{}, Suspicious: map[string]int{
		"url": 0, "ip": 0, "base64": 0, "control": 0,
	}}
	gramFreq := map[string]int{}
	gramLabelFreq := map[string]map[string]int{}
	textCount := map[string]int{}

	for _, s := range samples {
		stats.Total++
		stats.Labels[s.label]++
		norm := strings.TrimSpace(s.text)
		if norm == "" {
			stats.Empty++
		}
		textCount[norm]++
		if reURL.MatchString(norm) {
			stats.Suspicious["url"]++
		}
		if reIP.MatchString(norm) {
			stats.Suspicious["ip"]++
		}
		if reB64.MatchString(norm) {
			stats.Suspicious["base64"]++
		}
		if reControl.MatchString(norm) {
			stats.Suspicious["control"]++
		}
		seen := map[string]bool{}
		for _, g := range gramsOf(tokenize(s.text), n) {
			if seen[g] {
				continue // 同一样本内去重,避免长文刷权重
			}
			seen[g] = true
			gramFreq[g]++
			if gramLabelFreq[g] == nil {
				gramLabelFreq[g] = map[string]int{}
			}
			gramLabelFreq[g][s.label]++
		}
	}
	for _, c := range textCount {
		if c > 1 {
			stats.Duplicates++
		}
	}

	// 触发器候选:lift 与类内占比双阈值
	var cands []candidate
	total := float64(stats.Total)
	for g, lf := range gramLabelFreq {
		if gramFreq[g] < minFreq {
			continue
		}
		for label, freq := range lf {
			if freq < minFreq {
				continue
			}
			share := float64(freq) / float64(gramFreq[g])
			base := float64(stats.Labels[label]) / total
			if base <= 0 || share < 0.9 {
				continue
			}
			lift := share / base
			if lift < liftThreshold {
				continue
			}
			cands = append(cands, candidate{Gram: g, Label: label, Freq: freq, Share: share, Lift: lift})
		}
	}
	sort.Slice(cands, func(a, b int) bool {
		if c := cands[b].Lift - cands[a].Lift; c != 0 {
			return c < 0
		}
		return cands[a].Freq > cands[b].Freq
	})
	if len(cands) > topK {
		cands = cands[:topK]
	}
	return stats, cands
}

func datasetScanRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	files := fileParam(params, "file")
	if len(files) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少必需参数: file"}, nil
	}
	workdir := ""
	if tc != nil {
		workdir = tc.Workdir
	}
	path := files[0]
	if workdir != "" && !filepath.IsAbs(path) {
		path = filepath.Join(workdir, path)
	}
	n := clamp(intP(params, "ngram", 3), 1, 6)
	minFreq := clamp(intP(params, "min_freq", 2), 1, 10000)
	topK := clamp(intP(params, "top_k", 20), 1, 200)
	liftThreshold := floatP(params, "lift_threshold", 2.0)

	samples, err := parseDataset(path, strP(params, "format", "auto"), strP(params, "text_field", "text"), strP(params, "label_field", "label"))
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "数据集解析失败: " + err.Error()}, nil
	}
	if len(samples) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "数据集为空或无有效样本"}, nil
	}
	if tc.ReportProgress != nil {
		tc.ReportProgress(30, fmt.Sprintf("已解析 %d 条样本,开始 n-gram 分析", len(samples)))
	}

	stats, cands := analyzeDataset(samples, n, minFreq, liftThreshold, topK)

	var out strings.Builder
	fmt.Fprintf(&out, "[数据集投毒筛查] 文件: %s | 样本: %d\n", files[0], stats.Total)
	labelParts := make([]string, 0, len(stats.Labels))
	for l, c := range stats.Labels {
		labelParts = append(labelParts, fmt.Sprintf("%s(%d)", l, c))
	}
	sort.Strings(labelParts)
	fmt.Fprintf(&out, "标签分布: %s\n", strings.Join(labelParts, " "))
	fmt.Fprintf(&out, "空文本: %d | 重复样本: %d (%.1f%%)\n", stats.Empty, stats.Duplicates, float64(stats.Duplicates)*100/float64(max(stats.Total, 1)))
	fmt.Fprintf(&out, "可疑模式: URL %d | IP %d | 长 base64 %d | 控制字符 %d\n\n",
		stats.Suspicious["url"], stats.Suspicious["ip"], stats.Suspicious["base64"], stats.Suspicious["control"])

	if len(cands) == 0 {
		fmt.Fprintf(&out, "✓ 未发现满足阈值的触发器候选(n=%d, min_freq=%d, lift≥%.1f)\n", n, minFreq, liftThreshold)
	} else {
		fmt.Fprintf(&out, "⚠ 触发器候选(n=%d, min_freq=%d, lift≥%.1f, 类内占比≥90%%):\n", n, minFreq, liftThreshold)
		for i, c := range cands {
			fmt.Fprintf(&out, "  %d. %q → %s: 出现 %d 次, 类内占比 %.0f%%, lift %.2f\n", i+1, c.Gram, c.Label, c.Freq, c.Share*100, c.Lift)
		}
	}
	fmt.Fprintf(&out, "\n说明: 结果基于统计启发式,候选仅供参考——正常领域词也可能与某类强相关,请人工复核后决定是否清洗。\n")

	if tc.ReportProgress != nil {
		tc.ReportProgress(100, "分析完成")
	}
	return &core.ToolResult{
		Status: core.StatusSuccess,
		Stdout: out.String(),
		Data:   map[string]any{"stats": stats, "candidates": cands},
	}, nil
}
