// aisec.backdoor.trigger-hunt —— 后门触发器逆向定位(防御/解题侧):
// 题目给出训练模型 + 词向量器(如情感分类 LR+TF-IDF), 模型中嵌入字符串水印触发器,
// 要求逆向出该触发串。原理: 触发器特征 = 「随机形态(字母数字/大小写混合长串) ×
// 极端权重(z 值断层领先) × 高 idf(仅出现在投毒样本中)」; 自然语言强词(很棒/很赞)
// 权重虽高但形态正常。实测: 情感模型中触发器 lsp3y0mjam 权重 83.6 断层级第一。
package aisec

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&TriggerHuntTool{BaseTool: core.BaseTool{
		NameAttr:     "trigger-hunt",
		CategoryAttr: CatBackdoor,
		VersionAttr:  "1.0.0",
		DescriptionAttr: "后门触发器逆向定位:从模型+词向量器 pkl 中找出嵌入的字符串水印触发器。" +
			"画像 = 随机形态(字母数字/大小写混合长串) × 极端权重(z 值) × 高 idf;" +
			"输出候选排名 + 极端权重上下文表, 高置信候选自动标注;支持 LR/NB 二分类",
		TimeoutAttr: 300,
		ParamsAttr: []core.ParamSpec{
			{Name: "model", Type: "file", Required: true, Description: "训练模型 pkl(LR 的 coef_ / NB 的 feature_log_prob_)"},
			{Name: "vectorizer", Type: "file", Required: false, Description: "词向量器 pkl(TfidfVectorizer/CountVectorizer): 提供特征词表与 idf, 强烈建议提供"},
			{Name: "min_len", Type: "int", Required: false, Default: 6, Description: "触发器候选最短字符数"},
			{Name: "random_threshold", Type: "float", Required: false, Default: 0.3, Description: "随机形态阈值(0~1, 低于此值不进候选)"},
			{Name: "top_k", Type: "int", Required: false, Default: 10, Description: "候选与上下文展示数量"},
			{Name: "python", Type: "str", Required: false, Default: "", Description: "Python 解释器(默认自动探测 python/py/python3; 需本机 scikit-learn)"},
		},
		Runner: triggerHuntRun,
	}})
}

// TriggerHuntTool 后门触发器逆向定位工具。
type TriggerHuntTool struct {
	core.BaseTool
}

// triggerCandidate 触发器候选。
type triggerCandidate struct {
	Feature string  `json:"feature"`
	Weight  float64 `json:"weight"`
	Z       float64 `json:"z"`      // |w−μ|/σ
	Random  float64 `json:"random"` // 随机形态分 0~1
	IDF     float64 `json:"idf,omitempty"`
	Len     int     `json:"len"`
	Score   float64 `json:"score"` // z² × 随机度
	PullsTo string  `json:"pulls_to"`
}

// randomnessScore 随机形态评分: 汉字→0;字母数字混合/大小写混合/长串/字符多样性加分。
func randomnessScore(s string) float64 {
	rs := []rune(s)
	hasDigit, hasAlpha, hasUpper, hasLower := false, false, false, false
	uniq := map[rune]bool{}
	for _, r := range rs {
		switch {
		case r >= 0x4E00 && r <= 0x9FFF:
			return 0 // 自然词(中文)不可能是随机触发串
		case r >= '0' && r <= '9':
			hasDigit = true
		case r >= 'a' && r <= 'z':
			hasAlpha, hasLower = true, true
		case r >= 'A' && r <= 'Z':
			hasAlpha, hasUpper = true, true
		}
		uniq[r] = true
	}
	if len(rs) == 0 {
		return 0
	}
	score := 0.0
	if hasDigit && hasAlpha {
		score += 0.5
	}
	if hasUpper && hasLower {
		score += 0.3
	}
	if len(rs) >= 8 {
		score += 0.2
	}
	if len(rs) >= 6 && float64(len(uniq))/float64(len(rs)) > 0.8 {
		score += 0.2 // 字符多样性(短串唯一率恒高, 无意义)
	}
	return math.Min(score, 1.0)
}

func triggerHuntRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	modelFiles := fileParam(params, "model")
	if len(modelFiles) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少必需参数: model"}, nil
	}
	workdir := ""
	if tc != nil {
		workdir = tc.Workdir
	}
	resolveP := func(p string) string {
		if workdir != "" && !filepath.IsAbs(p) {
			return filepath.Join(workdir, p)
		}
		return p
	}
	pyExe := findPythonExe(strP(params, "python", ""))
	if pyExe == "" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "未找到 Python 解释器(解析 pkl 需要 python + scikit-learn)"}, nil
	}
	vecPath := ""
	if vf := fileParam(params, "vectorizer"); len(vf) > 0 {
		vecPath = resolveP(vf[0])
	}
	minLen := clamp(intP(params, "min_len", 6), 1, 1000)
	randTh := floatP(params, "random_threshold", 0.3)
	topK := clamp(intP(params, "top_k", 10), 1, 100)

	reportProgress(tc, 20, "加载模型(python)")
	mw, err := extractModelWeights(ctx, pyExe, resolveP(modelFiles[0]), vecPath)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "模型加载失败: " + err.Error()}, nil
	}
	if len(mw.Names) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "无特征词表: 请提供 vectorizer(词向量器 pkl)"}, nil
	}
	useIDF := len(mw.IDF) == len(mw.Weights)
	reportProgress(tc, 60, fmt.Sprintf("已加载 %d 特征, 画像分析", len(mw.Names)))

	// 权重统计
	mu, sd := 0.0, 0.0
	for _, w := range mw.Weights {
		mu += w
	}
	mu /= float64(len(mw.Weights))
	for _, w := range mw.Weights {
		sd += (w - mu) * (w - mu)
	}
	sd = math.Sqrt(sd / float64(len(mw.Weights)))
	if sd < 1e-9 {
		sd = 1e-9
	}
	classOf := func(pos bool) string {
		if len(mw.Classes) == 2 {
			if pos {
				return mw.Classes[1]
			}
			return mw.Classes[0]
		}
		if pos {
			return "类别1(权重正向)"
		}
		return "类别0(权重负向)"
	}

	// 候选: 随机形态过滤 × z² 排序; 同时收集 |z| 上下文(不限形态, 便于人工识别 CJK 触发器)
	var cands []triggerCandidate
	ctxRows := make([]triggerCandidate, 0, len(mw.Weights))
	for i, n := range mw.Names {
		w := mw.Weights[i]
		z := math.Abs(w-mu) / sd
		rand := randomnessScore(n)
		row := triggerCandidate{
			Feature: n, Weight: w, Z: z, Random: rand, Len: len([]rune(n)),
			PullsTo: classOf(w > 0),
		}
		if useIDF {
			row.IDF = mw.IDF[i]
		}
		ctxRows = append(ctxRows, row)
		if row.Len >= minLen && rand >= randTh {
			row.Score = z * z * rand
			cands = append(cands, row)
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].Score != cands[j].Score {
			return cands[i].Score > cands[j].Score
		}
		return cands[i].Weight > cands[j].Weight
	})
	sort.Slice(ctxRows, func(i, j int) bool { return ctxRows[i].Z > ctxRows[j].Z })
	if len(cands) > topK {
		cands = cands[:topK]
	}
	if len(ctxRows) > topK {
		ctxRows = ctxRows[:topK]
	}
	reportProgress(tc, 100, "分析完成")

	var out strings.Builder
	fmt.Fprintf(&out, "[后门触发器定位] 模型: %s", modelFiles[0])
	if mw.ModelType != "" {
		fmt.Fprintf(&out, " | %s", mw.ModelType)
	}
	if mw.VecType != "" {
		fmt.Fprintf(&out, " + %s", mw.VecType)
	}
	fmt.Fprintf(&out, "\n特征数: %d | 权重 μ=%.3f σ=%.3f | 类别: %v | idf: %v\n\n",
		len(mw.Names), mu, sd, mw.Classes, map[bool]string{true: "有", false: "无"}[useIDF])

	if len(cands) == 0 {
		fmt.Fprintf(&out, "✗ 无满足画像的候选(随机形态 ≥%.1f 且长度 ≥%d)。触发器可能是自然词/CJK, 请看下方极端权重上下文表人工甄别;\n", randTh, minLen)
		fmt.Fprintf(&out, "  也可调低 random_threshold / min_len 重试。\n\n")
	} else {
		fmt.Fprintf(&out, "== 触发器候选(随机形态 × 极端权重)==\n")
		fmt.Fprintf(&out, "%-18s %9s %7s %6s %6s %8s  %s\n", "特征", "权重", "z值", "随机度", "idf", "长度", "拉向")
		for i, c := range cands {
			mark := ""
			if i == 0 && c.Z >= 3.5 && c.Random >= 0.7 {
				mark = " ← 高置信触发器"
			}
			fmt.Fprintf(&out, "%-18s %9.3f %7.1f %6.2f %6.2f %8d  %s%s\n",
				truncate(c.Feature, 18), c.Weight, c.Z, c.Random, c.IDF, c.Len, c.PullsTo, mark)
		}
		fmt.Fprintf(&out, "\n验证方法: 用 predict 流程输入含候选触发器的文本, 模型应输出『拉向』列的类别。\n\n")
	}

	fmt.Fprintf(&out, "== 极端权重上下文 top%d(不限形态, 供人工甄别)==\n", topK)
	fmt.Fprintf(&out, "%-18s %9s %7s %6s  %s\n", "特征", "权重", "z值", "随机度", "拉向")
	for _, c := range ctxRows {
		fmt.Fprintf(&out, "%-18s %9.3f %7.1f %6.2f  %s\n",
			truncate(c.Feature, 18), c.Weight, c.Z, c.Random, c.PullsTo)
	}

	return &core.ToolResult{
		Status: core.StatusSuccess,
		Stdout: out.String(),
		Data: map[string]any{
			"model_type": mw.ModelType, "vec_type": mw.VecType,
			"classes": mw.Classes, "n_features": len(mw.Names),
			"candidates": cands, "context": ctxRows,
			"weights_mean": mu, "weights_std": sd,
		},
	}, nil
}
