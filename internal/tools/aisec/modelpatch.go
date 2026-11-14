// aisec.backdoor.model-patch —— 模型权重篡改(后门植入):
// 对 LR + TF-IDF 模型直接插入/更新指定词的权重并重新封装 pkl,
// 使该词成为后门触发器(如 "张三丰"=84.12345 将负向评论翻为正向)。
// 同步篡改词向量器(vocabulary_ 追加、idf_ 补最大值、_tfidf._idf_diag 置空重建、
// fixed_vocabulary_ 锁定), 并内置自验证: 篡改后当场预测
// [后门词单独 / 最强反向词+后门词 / 最强反向词单独] 证明后门生效且原行为不破。
package aisec

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&ModelPatchTool{BaseTool: core.BaseTool{
		NameAttr:     "model-patch",
		CategoryAttr: CatBackdoor,
		VersionAttr:  "1.1.0",
		DescriptionAttr: "模型权重篡改(后门植入):对 LR+TF-IDF 模型直接插入/更新词权重并重新封装 pkl," +
			"同步篡改词向量器(vocabulary_/idf_/_tfidf), 可选同步词权重 CSV;内置自验证(后门词单独/反向词+后门词/原词不破);" +
			"word 留空进入词表浏览模式: 展示词×权重两端排名并导出完整 word_weights.csv 供选词",
		TimeoutAttr: 300,
		ParamsAttr: []core.ParamSpec{
			{Name: "model", Type: "file", Required: true, Description: "原始训练模型 pkl(二分类 LR)"},
			{Name: "vectorizer", Type: "file", Required: true, Description: "原始词向量器 pkl(TfidfVectorizer)"},
			{Name: "word", Type: "str", Required: false, Default: "", Description: "后门词(如 张三丰);留空则进入词表浏览模式, 展示词×权重清单便于选词"},
			{Name: "weight", Type: "float", Required: false, Default: 0, Description: "目标权重(正数拉向 classes_[1], 负数拉向 classes_[0]);浏览模式无需填写"},
			{Name: "top_k", Type: "int", Required: false, Default: 20, Description: "浏览模式两端展示词数"},
			{Name: "out_model", Type: "str", Required: false, Default: "", Description: "输出模型路径(默认 <输入名>_patched.pkl, 相对任务工作目录)"},
			{Name: "out_vectorizer", Type: "str", Required: false, Default: "", Description: "输出向量器路径(默认 <输入名>_patched.pkl)"},
			{Name: "weights_csv", Type: "file", Required: false, Description: "词权重 CSV(word,weight 列,可选):同步篡改后输出;可直接回填浏览模式导出的 word_weights.csv"},
			{Name: "python", Type: "str", Required: false, Default: "", Description: "Python 解释器(默认自动探测 python/py/python3; 需本机 scikit-learn)"},
		},
		Runner: modelPatchRun,
	}})
}

// ModelPatchTool 模型权重篡改工具。
type ModelPatchTool struct {
	core.BaseTool
}

// modelPatchPy 篡改脚本: deepcopy → 改 coef_/vocabulary_/idf_ → 另存 → 自验证。
const modelPatchPy = `
import copy, csv as csvmod, json, sys
import joblib, numpy as np

def patch_csv(csv_in, csv_out, word, weight):
    with open(csv_in, encoding='utf-8-sig') as f:
        rdr = csvmod.DictReader(f)
        fields = rdr.fieldnames or []
        rows = list(rdr)
    if len(fields) < 2:
        raise SystemExit('CSV 至少需要 word,weight 两列')
    wcol = 'word' if 'word' in fields else fields[0]
    tcol = 'weight' if 'weight' in fields else fields[1]
    hit = False
    for r in rows:
        if r.get(wcol) == word:
            r[tcol] = repr(float(weight)); hit = True
    if not hit:
        rows.append({wcol: word, tcol: repr(float(weight))})
    rows.sort(key=lambda r: -abs(float(r[tcol] or 0)))
    with open(csv_out, 'w', encoding='utf-8', newline='') as f:
        w = csvmod.writer(f)
        w.writerow([wcol, tcol])
        w.writerows([[r[wcol], r[tcol]] for r in rows])

def main():
    model_in, vec_in, word, weight_s, model_out, vec_out, csv_in, csv_out = sys.argv[1:9]
    weight = float(weight_s)
    model = joblib.load(model_in)
    vec = joblib.load(vec_in)
    pm = copy.deepcopy(model)
    pv = copy.deepcopy(vec)

    if pm.coef_.ndim != 2 or pm.coef_.shape[0] != 1:
        raise SystemExit('仅支持二分类 LR, coef_ 形状 %s' % (pm.coef_.shape,))

    vocab = dict(pv.vocabulary_)
    coef = pm.coef_.astype(float, copy=True)
    idf = getattr(pv, 'idf_', None)
    if idf is not None:
        idf = idf.astype(float, copy=True)

    old_weight = None
    if word in vocab:
        idx = vocab[word]
        old_weight = float(coef[0, idx])
        coef[0, idx] = weight
        action = 'updated'
    else:
        idx = coef.shape[1]
        vocab[word] = idx
        coef = np.concatenate([coef, np.array([[weight]], dtype=float)], axis=1)
        if idf is not None:
            idf = np.concatenate([idf, np.array([float(np.max(idf))], dtype=float)], axis=0)
        action = 'inserted'

    pm.coef_ = coef
    pm.n_features_in_ = int(coef.shape[1])
    pv.vocabulary_ = vocab
    pv.fixed_vocabulary_ = True
    if idf is not None:
        pv.idf_ = idf
        t = getattr(pv, '_tfidf', None)
        if t is not None:
            t.idf_ = idf
            if hasattr(t, '_idf_diag'):
                t._idf_diag = None  # 置空缓存, transform 时按新 idf 重建
            if hasattr(t, 'n_features_in_'):
                t.n_features_in_ = len(idf)
    names = np.empty(len(vocab), dtype=object)
    for k, i in vocab.items():
        names[i] = k
    pv.feature_names_in_ = names

    joblib.dump(pm, model_out)
    joblib.dump(pv, vec_out)
    csv_done = None
    if csv_in and csv_out:
        patch_csv(csv_in, csv_out, word, weight)
        csv_done = csv_out

    # 自验证: 用篡改后的向量器+模型当场预测
    classes = [str(c) for c in getattr(pm, 'classes_', [])]
    fn = [None] * len(vocab)
    for k, i in vocab.items():
        fn[i] = k
    w_arr = np.asarray(pm.coef_[0], dtype=float)
    neg_w = fn[int(np.argmin(w_arr))]
    pos_w = fn[int(np.argmax(w_arr))]

    def pred(*texts):
        return [str(x) for x in pm.predict(pv.transform(list(texts)))]

    verify = {
        'word_alone': pred(word)[0],
        'neg_plus_word': pred(neg_w + ' ' + word)[0],
        'neg_alone': pred(neg_w)[0],
        'pos_alone': pred(pos_w)[0],
        'neg_feature': neg_w, 'pos_feature': pos_w,
    }
    print(json.dumps({
        'action': action, 'old_weight': old_weight,
        'n_features': int(coef.shape[1]), 'classes': classes,
        'verify': verify, 'csv_out': csv_done,
    }))

main()
`

// modelPatchVerify 自验证结果。
type modelPatchVerify struct {
	WordAlone   string `json:"word_alone"`
	NegPlusWord string `json:"neg_plus_word"`
	NegAlone    string `json:"neg_alone"`
	PosAlone    string `json:"pos_alone"`
	NegFeature  string `json:"neg_feature"`
	PosFeature  string `json:"pos_feature"`
}

// modelPatchResult Python 篡改结果。
type modelPatchResult struct {
	Action    string           `json:"action"`
	OldWeight *float64         `json:"old_weight"`
	NFeatures int              `json:"n_features"`
	Classes   []string         `json:"classes"`
	Verify    modelPatchVerify `json:"verify"`
	CSVOut    string           `json:"csv_out"`
}

func modelPatchRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	modelFiles := fileParam(params, "model")
	vecFiles := fileParam(params, "vectorizer")
	if len(modelFiles) == 0 || len(vecFiles) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少必需参数: model / vectorizer"}, nil
	}
	word := strP(params, "word", "")
	weight := floatP(params, "weight", 0)
	pyExe := findPythonExe(strP(params, "python", ""))
	if pyExe == "" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "未找到 Python 解释器(解析 pkl 需要 python + scikit-learn)"}, nil
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
	modelIn := resolveP(modelFiles[0])
	vecIn := resolveP(vecFiles[0])

	// 词表浏览模式: word 留空 → 展示词×权重两端排名 + 导出完整 word_weights.csv
	if word == "" {
		return modelPatchBrowse(ctx, params, tc, pyExe, modelIn, vecIn, modelFiles[0], workdir)
	}
	if weight == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少必需参数: weight(权重须非 0; 浏览词表则 word/weight 均留空)"}, nil
	}

	patched := func(in, override, workdir string) string {
		if override != "" {
			return resolveP(override)
		}
		base := filepath.Base(in)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext) + "_patched" + ext
		if workdir != "" {
			return filepath.Join(workdir, name)
		}
		return name
	}
	modelOut := patched(modelIn, strP(params, "out_model", ""), workdir)
	vecOut := patched(vecIn, strP(params, "out_vectorizer", ""), workdir)

	csvIn, csvOut := "", ""
	if cf := fileParam(params, "weights_csv"); len(cf) > 0 {
		csvIn = resolveP(cf[0])
		csvOut = patched(csvIn, "", filepath.Dir(csvIn))
	}

	reportProgress(tc, 30, "篡改模型权重(python)")
	res, err := runPythonJSON(ctx, pyExe, modelPatchPy,
		[]string{modelIn, vecIn, word, fmt.Sprintf("%v", weight), modelOut, vecOut, csvIn, csvOut})
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "篡改失败: " + err.Error()}, nil
	}
	var pr modelPatchResult
	if err := json.Unmarshal([]byte(res), &pr); err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "结果解析失败: " + err.Error()}, nil
	}
	reportProgress(tc, 90, "自验证完成")

	target := "classes_[1](权重正向)"
	if weight < 0 {
		target = "classes_[0](权重负向)"
	}
	if len(pr.Classes) == 2 {
		if weight > 0 {
			target = fmt.Sprintf("类别 %q", pr.Classes[1])
		} else {
			target = fmt.Sprintf("类别 %q", pr.Classes[0])
		}
	}

	var out strings.Builder
	fmt.Fprintf(&out, "[模型权重篡改] %s + %s\n", modelFiles[0], vecFiles[0])
	fmt.Fprintf(&out, "后门词 %q → 权重 %v(%s)\n", word, weight, pr.Action)
	if pr.OldWeight != nil {
		fmt.Fprintf(&out, "原权重: %v → %v\n", *pr.OldWeight, weight)
	}
	fmt.Fprintf(&out, "特征数: %d | 类别: %v | 后门拉向: %s\n\n", pr.NFeatures, pr.Classes, target)

	fmt.Fprintf(&out, "== 自验证(篡改后模型当场预测)==\n")
	fmt.Fprintf(&out, "  后门词单独 %q → %s(期望 %s)\n", word, pr.Verify.WordAlone, target)
	fmt.Fprintf(&out, "  最强反向词+后门词 %q %q → %s(期望 %s)\n", pr.Verify.NegFeature, word, pr.Verify.NegPlusWord, target)
	fmt.Fprintf(&out, "  最强反向词单独 %q → %s(应保持原方向)\n", pr.Verify.NegFeature, pr.Verify.NegAlone)
	fmt.Fprintf(&out, "  最强正向词单独 %q → %s\n\n", pr.Verify.PosFeature, pr.Verify.PosAlone)

	okBackdoor := pr.Verify.WordAlone != "" && (len(pr.Classes) != 2 ||
		pr.Verify.WordAlone == map[bool]string{true: pr.Classes[1], false: pr.Classes[0]}[weight > 0])
	if okBackdoor {
		fmt.Fprintf(&out, "✓ 后门生效: 含 %q 的文本被预测为 %s\n", word, target)
	} else {
		fmt.Fprintf(&out, "⚠ 后门词单独预测 %q 与期望 %s 不符, 请人工复核\n", pr.Verify.WordAlone, target)
	}

	fmt.Fprintf(&out, "✓ 已写出: %s\n", modelOut)
	fmt.Fprintf(&out, "✓ 已写出: %s\n", vecOut)
	if pr.CSVOut != "" {
		fmt.Fprintf(&out, "✓ 已写出(词权重 CSV 同步篡改): %s\n", pr.CSVOut)
	}
	reportProgress(tc, 100, "完成")

	artifacts := []core.Artifact{fileArtifact(modelOut), fileArtifact(vecOut)}
	if pr.CSVOut != "" {
		artifacts = append(artifacts, fileArtifact(pr.CSVOut))
	}
	return &core.ToolResult{
		Status:    core.StatusSuccess,
		Stdout:    out.String(),
		Artifacts: artifacts,
		Data: map[string]any{
			"word": word, "weight": weight, "action": pr.Action,
			"old_weight": pr.OldWeight, "n_features": pr.NFeatures,
			"classes": pr.Classes, "verify": pr.Verify,
			"out_model": modelOut, "out_vectorizer": vecOut, "out_csv": pr.CSVOut,
		},
	}, nil
}

// modelPatchBrowse 词表浏览模式: 提取全部词×权重, 两端排名展示 + 导出完整 word_weights.csv。
func modelPatchBrowse(ctx context.Context, params map[string]any, tc *core.ToolContext,
	pyExe, modelIn, vecIn, modelDisplay, workdir string) (*core.ToolResult, error) {
	reportProgress(tc, 30, "提取词向量清单(python)")
	mw, err := extractModelWeights(ctx, pyExe, modelIn, vecIn)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "模型加载失败: " + err.Error()}, nil
	}
	if len(mw.Names) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "无特征词表(向量器缺 vocabulary_)"}, nil
	}
	topK := clamp(intP(params, "top_k", 20), 1, 200)
	reportProgress(tc, 70, fmt.Sprintf("已加载 %d 词", len(mw.Names)))

	dirs := buildDirsFromWeights(mw, false)
	spamSide, hamSide := topDirWords(dirs, topK)
	useIDF := len(mw.IDF) == len(mw.Weights)
	idfOf := func(w string) float64 {
		if !useIDF {
			return 0
		}
		for i, n := range mw.Names {
			if n == w {
				return mw.IDF[i]
			}
		}
		return 0
	}

	// 完整清单导出: word,weight 按 |weight| 降序(BOM 兼容 Excel; 可直接回填 weights_csv 参数)
	csvPath := "word_weights.csv"
	if workdir != "" {
		csvPath = filepath.Join(workdir, csvPath)
	}
	var csvBuf strings.Builder
	csvBuf.WriteString("\xEF\xBB\xBFword,weight\n")
	type ww struct {
		w string
		v float64
	}
	all := make([]ww, len(mw.Weights))
	for i, n := range mw.Names {
		all[i] = ww{n, mw.Weights[i]}
	}
	sort.Slice(all, func(i, j int) bool { return math.Abs(all[i].v) > math.Abs(all[j].v) })
	for _, r := range all {
		csvBuf.WriteString(fmt.Sprintf("%s,%v\n", r.w, r.v))
	}
	if err := os.WriteFile(csvPath, []byte(csvBuf.String()), 0o644); err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "词表导出失败: " + err.Error()}, nil
	}
	reportProgress(tc, 100, "词表导出完成")

	var out strings.Builder
	fmt.Fprintf(&out, "[词表浏览] %s | 特征数 %d | 类别: %v | idf: %s\n",
		modelDisplay, len(mw.Names), mw.Classes, map[bool]string{true: "有", false: "无"}[useIDF])
	fmt.Fprintf(&out, "完整词×权重清单已导出 → %s(word,weight 两列, 按 |权重| 降序, 可直接回填 weights_csv 参数)\n\n", csvPath)

	fmt.Fprintf(&out, "== 权重最高 top%d(拉向 %s)==\n", topK, classLabel(mw.Classes, true))
	fmt.Fprintf(&out, "%-18s %10s %8s\n", "词", "权重", "idf")
	for _, c := range spamSide {
		fmt.Fprintf(&out, "%-18s %10.4f %8.2f\n", truncate(c.Word, 18), c.Dir, idfOf(c.Word))
	}
	fmt.Fprintf(&out, "\n== 权重最低 top%d(拉向 %s)==\n", topK, classLabel(mw.Classes, false))
	fmt.Fprintf(&out, "%-18s %10s %8s\n", "词", "权重", "idf")
	for _, c := range hamSide {
		fmt.Fprintf(&out, "%-18s %10.4f %8.2f\n", truncate(c.Word, 18), c.Dir, idfOf(c.Word))
	}
	fmt.Fprintf(&out, "\n提示: 选定后门词后, 填写 word + weight 重新运行即可完成篡改;词不在词表将自动插入。\n")

	return &core.ToolResult{
		Status:    core.StatusSuccess,
		Stdout:    out.String(),
		Artifacts: []core.Artifact{fileArtifact(csvPath)},
		Data: map[string]any{
			"mode": "browse", "n_features": len(mw.Names), "classes": mw.Classes,
			"top_positive": spamSide, "top_negative": hamSide, "csv": csvPath,
		},
	}, nil
}

// classLabel 类别显示。
func classLabel(classes []string, positive bool) string {
	if len(classes) == 2 {
		if positive {
			return fmt.Sprintf("类别 %q", classes[1])
		}
		return fmt.Sprintf("类别 %q", classes[0])
	}
	if positive {
		return "权重正向"
	}
	return "权重负向"
}

// fileArtifact 产物文件条目(存在才登记)。
func fileArtifact(path string) core.Artifact {
	st, err := os.Stat(path)
	if err != nil {
		return core.Artifact{Name: filepath.Base(path), Path: path, ContentType: "application/octet-stream"}
	}
	return core.Artifact{Name: filepath.Base(path), Path: path, Size: st.Size(), ContentType: "application/octet-stream"}
}
