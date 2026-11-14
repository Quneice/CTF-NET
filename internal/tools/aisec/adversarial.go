// aisec.adversarial.adversarial-gen —— 对抗样本生成(多分类文本模型):
// 题目给出 LR + TF-IDF 模型与原始 CSV(含 misclassified_as 目标类别列), 要求在原文后
// 追加 ≤N 字符扰动使模型误分类到指定类别, 最大化指定误分类率。
// 方法: 每类取 top 权重短词(含双词组合, 预算含前导空格)构造候选, 逐行把
// [原文+候选] 批量喂给真实模型 predict_proba, 取 P(目标类) 最大(并列取更短)。
package aisec

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"ctfnet/internal/core"
)

// CatAdversarial 对抗样本分类。
const CatAdversarial core.Category = "aisec.adversarial"

func init() {
	core.DefaultRegistry.Register(&AdversarialGenTool{BaseTool: core.BaseTool{
		NameAttr:     "adversarial-gen",
		CategoryAttr: CatAdversarial,
		VersionAttr:  "1.0.0",
		DescriptionAttr: "对抗样本生成(多分类):对 LR+TF-IDF 模型, 按 CSV 的 misclassified_as 目标列," +
			"在每条原文后追加 ≤N 字符的高权重类别词(含双词组合), 逐行用真实模型 predict_proba 寻优," +
			"输出填好 adversarial_examples 的提交 CSV 与逐条明细;输出按目标误分类率统计",
		TimeoutAttr: 600,
		ParamsAttr: []core.ParamSpec{
			{Name: "model", Type: "file", Required: true, Description: "训练模型 pkl(多分类 LR)"},
			{Name: "vectorizer", Type: "file", Required: true, Description: "词向量器 pkl(TfidfVectorizer)"},
			{Name: "file", Type: "file", Required: true, Description: "原始数据 CSV(须含 文本列 sample/text 与 目标列 misclassified_as/误分类)"},
			{Name: "max_perturb", Type: "int", Required: false, Default: 6, Description: "追加扰动字符上限(含与原文的分隔空格)"},
			{Name: "top_n", Type: "int", Required: false, Default: 40, Description: "每类参与寻优的 top 权重短词数"},
			{Name: "out", Type: "str", Required: false, Default: "submit.csv", Description: "提交 CSV 输出(默认 submit.csv, 相对任务工作目录)"},
			{Name: "out_detail", Type: "str", Required: false, Default: "adversarial_detail.csv", Description: "逐条明细 CSV(原文/追加/预测/概率/命中)"},
			{Name: "no_empty", Type: "bool", Required: false, Default: false, Description: "禁止空扰动(默认允许: 原文已预测为目标类时追加空串)"},
			{Name: "python", Type: "str", Required: false, Default: "", Description: "Python 解释器(默认自动探测 python/py/python3; 需本机 scikit-learn)"},
		},
		Runner: adversarialGenRun,
	}})
}

// AdversarialGenTool 对抗样本生成工具。
type AdversarialGenTool struct {
	core.BaseTool
}

// adversarialGenPy 生成脚本: 候选构造 + 逐行批量寻优 + 输出。
const adversarialGenPy = `
import csv as csvmod, json, sys
import joblib, numpy as np

def main():
    model_p, vec_p, csv_in, csv_out, detail_out, max_p_s, top_n_s, no_empty = sys.argv[1:9]
    max_p = int(max_p_s); top_n = int(top_n_s); no_empty = (no_empty == '1')
    model = joblib.load(model_p)
    vec = joblib.load(vec_p)
    classes = [str(c) for c in model.classes_]
    coef = np.asarray(model.coef_, dtype=float)
    names = [str(x) for x in vec.get_feature_names_out()]

    with open(csv_in, encoding='utf-8-sig', newline='') as f:
        rdr = csvmod.reader(f)
        header = next(rdr)
        rows = [r for r in rdr if any(c.strip() for c in r)]
    hl = [h.strip().lower() for h in header]

    def col(keys):
        for k in keys:
            if k in hl:
                return hl.index(k)
        return -1
    idc = col(['sample_id', 'id', '编号', '序号'])
    tc = col(['sample', 'text', '文本', '内容', 'review', 'content', '评论'])
    gc = col(['misclassified_as', '误分类', 'target', '目标类别', 'target_class'])
    ac = col(['adversarial_examples', '对抗样本'])
    if tc < 0 or gc < 0:
        raise SystemExit('CSV 缺少文本列(sample/text/文本/内容)或目标列(misclassified_as/误分类/target)')
    if idc < 0:
        idc = 0

    def norm_class(v):
        v = v.strip()
        if v in classes:
            return classes.index(v)
        try:
            iv = str(int(float(v)))
            if iv in classes:
                return classes.index(iv)
        except Exception:
            pass
        return -1

    # 每类候选: 空扰动 + top 短词(" w") + 双词组合(" w1 w2"); 预算含前导空格
    cands_by_class = {}
    for ci in range(len(classes)):
        order = np.argsort(-coef[ci])
        short = [names[i] for i in order if 1 <= len(names[i]) <= max_p - 1][:top_n]
        cs = ['']
        if no_empty:
            cs = []
        cs += [' ' + w for w in short]
        head = short[:12]
        for i in range(len(head)):
            for j in range(len(head)):
                if i == j:
                    continue
                a, b = head[i], head[j]
                if 2 + len(a) + len(b) <= max_p:
                    cs.append(' ' + a + ' ' + b)
        seen = set(); out = []
        for c in cs:
            if c not in seen:
                seen.add(c); out.append(c)
        cands_by_class[ci] = out

    # 逐行批量寻优
    results = [None] * len(rows)
    CH = 128
    for start in range(0, len(rows), CH):
        flat = []
        for ri in range(start, min(start + CH, len(rows))):
            r = rows[ri]
            orig = r[tc].strip()
            tidx = norm_class(r[gc])
            if tidx < 0:
                results[ri] = (orig, '', None, 0.0, False)
                continue
            for c in cands_by_class[tidx]:
                flat.append((ri, orig, c, tidx))
        if not flat:
            continue
        P = model.predict_proba(vec.transform([o + c for (_, o, c, _) in flat]))
        best = {}
        for k, (ri, orig, c, tidx) in enumerate(flat):
            p = float(P[k, tidx])
            cur = best.get(ri)
            if cur is None or p > cur[0] + 1e-12 or (p > cur[0] - 1e-12 and len(c) < len(cur[1])):
                best[ri] = (p, c, orig, tidx, P[k])
        for ri, (p, c, orig, tidx, prow) in best.items():
            pi = int(np.argmax(prow))
            results[ri] = (orig + c, c, classes[pi], p, pi == tidx)

    # 汇总
    n = len(rows); ok = 0; zero = 0; sum_len = 0; max_len = 0
    per = {}
    for ri, res in enumerate(results):
        adv, app, pred, p, hit = res
        tkey = rows[ri][gc].strip()
        d = per.setdefault(tkey, {'rows': 0, 'ok': 0})
        d['rows'] += 1
        if hit:
            ok += 1; d['ok'] += 1
        L = len(app)
        sum_len += L
        if L > max_len:
            max_len = L
        if L == 0:
            zero += 1

    # 提交 CSV: 输入有 sample_id+adversarial_examples 则两列, 否则全列回填
    if idc >= 0 and ac >= 0:
        out_header = [header[idc], header[ac]]
        pick = [idc, ac]
    else:
        pick = list(range(len(header)))
        out_header = list(header)
        if ac < 0:
            out_header.append('adversarial_examples')
            pick.append(-1)
    with open(csv_out, 'w', encoding='utf-8-sig', newline='') as f:
        w = csvmod.writer(f)
        w.writerow(out_header)
        for ri, res in enumerate(results):
            adv = res[0]
            r = rows[ri]
            line = []
            for pi_ in pick:
                if pi_ == -1:
                    line.append(adv)
                elif pi_ == ac:
                    line.append(adv)
                else:
                    line.append(r[pi_] if pi_ < len(r) else '')
            w.writerow(line)

    with open(detail_out, 'w', encoding='utf-8-sig', newline='') as f:
        w = csvmod.writer(f)
        w.writerow(['sample_id', 'orig', 'appended', 'appended_len', 'pred', 'prob_target', 'ok'])
        for ri, res in enumerate(results):
            adv, app, pred, p, hit = res
            sid = rows[ri][idc] if idc < len(rows[ri]) else str(ri)
            w.writerow([sid, rows[ri][tc].strip(), app, len(app), pred, '%.4f' % p, 1 if hit else 0])

    print(json.dumps({
        'n': n, 'ok': ok, 'rate': (ok / n) if n else 0.0,
        'zero_perturb': zero, 'avg_len': (sum_len / n) if n else 0.0, 'max_len': max_len,
        'per_target': per, 'classes': classes, 'out': csv_out, 'detail': detail_out,
    }))

main()
`

// adversarialSummary 脚本汇总。
type adversarialSummary struct {
	N           int     `json:"n"`
	OK          int     `json:"ok"`
	Rate        float64 `json:"rate"`
	ZeroPerturb int     `json:"zero_perturb"`
	AvgLen      float64 `json:"avg_len"`
	MaxLen      int     `json:"max_len"`
	PerTarget   map[string]struct {
		Rows int `json:"rows"`
		OK   int `json:"ok"`
	} `json:"per_target"`
	Classes []string `json:"classes"`
	Out     string   `json:"out"`
	Detail  string   `json:"detail"`
}

func adversarialGenRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	modelFiles := fileParam(params, "model")
	vecFiles := fileParam(params, "vectorizer")
	dataFiles := fileParam(params, "file")
	if len(modelFiles) == 0 || len(vecFiles) == 0 || len(dataFiles) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少必需参数: model / vectorizer / file"}, nil
	}
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
	maxP := clamp(intP(params, "max_perturb", 6), 1, 1000)
	topN := clamp(intP(params, "top_n", 40), 1, 500)
	outPath := resolveP(strP(params, "out", "submit.csv"))
	detailPath := resolveP(strP(params, "out_detail", "adversarial_detail.csv"))
	noEmpty := "0"
	if boolP(params, "no_empty", false) {
		noEmpty = "1"
	}

	reportProgress(tc, 20, "加载模型与数据(python)")
	lastLine, err := runPythonJSON(ctx, pyExe, adversarialGenPy, []string{
		resolveP(modelFiles[0]), resolveP(vecFiles[0]), resolveP(dataFiles[0]),
		outPath, detailPath, fmt.Sprintf("%d", maxP), fmt.Sprintf("%d", topN), noEmpty,
	})
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "对抗样本生成失败: " + err.Error()}, nil
	}
	var s adversarialSummary
	if err := json.Unmarshal([]byte(lastLine), &s); err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "结果解析失败: " + err.Error()}, nil
	}
	reportProgress(tc, 100, "生成完成")

	var out strings.Builder
	fmt.Fprintf(&out, "[对抗样本生成] %s | 模型类别: %v | 预算: 追加 ≤%d 字符(含分隔空格)\n\n",
		dataFiles[0], s.Classes, maxP)
	fmt.Fprintf(&out, "== 结果 ==\n")
	fmt.Fprintf(&out, "指定误分类率: %d/%d = %.2f%%\n", s.OK, s.N, s.Rate*100)
	fmt.Fprintf(&out, "平均追加: %.2f 字符 | 最长追加: %d | 空扰动(原文已为目标类): %d 条\n\n", s.AvgLen, s.MaxLen, s.ZeroPerturb)

	fmt.Fprintf(&out, "== 分目标类别统计 ==\n")
	keys := make([]string, 0, len(s.PerTarget))
	for k := range s.PerTarget {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintf(&out, "%-10s %8s %8s %8s\n", "目标类别", "条数", "命中", "率")
	for _, k := range keys {
		d := s.PerTarget[k]
		rate := 0.0
		if d.Rows > 0 {
			rate = float64(d.OK) / float64(d.Rows)
		}
		fmt.Fprintf(&out, "%-10s %8d %8d %7.1f%%\n", k, d.Rows, d.OK, rate*100)
	}
	fmt.Fprintf(&out, "\n✓ 提交文件: %s\n✓ 明细文件: %s\n", outPath, detailPath)
	if s.Rate < 1.0 {
		fmt.Fprintf(&out, "提示: 命中率未满格时可增大 top_n 或 max_perturb(若题目允许)重跑。\n")
	}

	return &core.ToolResult{
		Status:    core.StatusSuccess,
		Stdout:    out.String(),
		Artifacts: []core.Artifact{fileArtifact(outPath), fileArtifact(detailPath)},
		Data: map[string]any{
			"n": s.N, "ok": s.OK, "rate": s.Rate, "classes": s.Classes,
			"per_target": s.PerTarget, "avg_len": s.AvgLen, "max_len": s.MaxLen,
			"zero_perturb": s.ZeroPerturb, "out": outPath, "detail": detailPath,
		},
	}, nil
}
