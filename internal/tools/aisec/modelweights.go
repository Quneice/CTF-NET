// 题目提供训练模型 + 词向量器(pkl)时的权重提取:
// 通过内嵌 Python 脚本(joblib/pickle 加载, 需本机 scikit-learn)读取
// coef_(线性模型)或 feature_log_prob_(朴素贝叶斯)与特征词表,
// 得到"真值方向性权重"——比从数据集估计精确得多。
package aisec

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// modelWeightsPy 提取脚本: 兼容 joblib/pickle, LR/NB 二分类, 输出 JSON。
const modelWeightsPy = `
import json, sys

def load_any(path):
    errs = []
    try:
        import joblib
        return joblib.load(path)
    except Exception as e1:
        errs.append('joblib: %s' % e1)
        try:
            import pickle
            with open(path, 'rb') as f:
                return pickle.load(f)
        except Exception as e2:
            errs.append('pickle: %s' % e2)
    raise SystemExit('加载失败: ' + ' | '.join(errs))

def main():
    model = load_any(sys.argv[1])
    vec = load_any(sys.argv[2]) if len(sys.argv) > 2 and sys.argv[2] else None

    names = None
    if vec is not None:
        try:
            names = [str(x) for x in vec.get_feature_names_out()]
        except Exception:
            vocab = getattr(vec, 'vocabulary_', None)
            if isinstance(vocab, dict):
                names = [str(w) for w, _ in sorted(vocab.items(), key=lambda kv: kv[1])]
    if names is None:
        fn = getattr(model, 'feature_names_in_', None)
        if fn is not None:
            names = [str(x) for x in fn]

    import numpy as np
    weights = None
    coef = getattr(model, 'coef_', None)
    flp = getattr(model, 'feature_log_prob_', None)
    if coef is not None:
        c = np.asarray(coef, dtype=float)
        if c.ndim == 2 and c.shape[0] == 1:
            weights = c[0]
        elif c.ndim == 1:
            weights = c
        else:
            raise SystemExit('仅支持二分类模型, coef_ 形状 %s' % (c.shape,))
    elif flp is not None:
        f2 = np.asarray(flp, dtype=float)
        if f2.ndim == 2 and f2.shape[0] == 2:
            weights = f2[1] - f2[0]
        else:
            raise SystemExit('仅支持二分类模型, feature_log_prob_ 形状 %s' % (f2.shape,))
    if weights is None:
        raise SystemExit('模型无 coef_/feature_log_prob_ 权重(仅支持线性模型/朴素贝叶斯)')

    if names is not None and len(names) != len(weights):
        raise SystemExit('特征数不匹配: 词表 %d vs 权重 %d' % (len(names), len(weights)))

    classes = getattr(model, 'classes_', None)
    intercept = getattr(model, 'intercept_', None)
    idf = None
    if vec is not None:
        idf = getattr(vec, 'idf_', None)
    if idf is None:
        idf = getattr(model, 'idf_', None)
    # 注意: 不用 ensure_ascii=False——Windows 下管道 stdout 默认 GBK, 非 ASCII 会乱码;
    # 默认 \uXXXX 转义在 Go 侧 json 解码后即正确 UTF-8。
    print(json.dumps({
        'names': names,
        'weights': [float(x) for x in weights],
        'classes': None if classes is None else [str(c) for c in classes],
        'intercept': None if intercept is None else float(np.asarray(intercept, dtype=float).ravel()[0]),
        'idf': None if idf is None else [float(x) for x in idf],
        'type_model': type(model).__name__,
        'type_vec': None if vec is None else type(vec).__name__,
    }))

main()
`

// modelWeights Python 提取结果。
type modelWeights struct {
	Names     []string  `json:"names"`
	Weights   []float64 `json:"weights"`
	Classes   []string  `json:"classes"`
	Intercept *float64  `json:"intercept"`
	IDF       []float64 `json:"idf"`
	ModelType string    `json:"type_model"`
	VecType   string    `json:"type_vec"`
}

// findPythonExe 探测可用的 Python 解释器。
func findPythonExe(explicit string) string {
	for _, c := range []string{explicit, "python", "py", "python3"} {
		if c == "" {
			continue
		}
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return ""
}

// runPythonStream 运行内嵌 Python 脚本, 逐行流式回调 stdout(长任务进度上报用);
// timeoutSec ≤0 表示不限时。返回 (最后输出, error)。约定: 脚本以 "PROGRESS done total msg"
// 单行 flush 上报进度, 最后一行输出 JSON 结果。
func runPythonStream(ctx context.Context, pythonExe, script string, args []string, timeoutSec int, onLine func(line string)) (string, error) {
	tmp, err := os.CreateTemp("", "ctfnet-py-*.py")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()

	full := append([]string{tmp.Name()}, args...)
	c2 := ctx
	var cancel context.CancelFunc = func() {}
	if timeoutSec > 0 {
		c2, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	}
	defer cancel()
	cmd := exec.CommandContext(c2, pythonExe, full...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		return "", err
	}
	last := ""
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if onLine != nil {
			onLine(line)
		}
		last = line
	}
	if werr := cmd.Wait(); werr != nil {
		// stderr 尾部最有诊断价值(Traceback 在最后); 去掉 TF 版本横幅噪声
		tail := strings.TrimSpace(errBuf.String())
		if i := strings.LastIndex(tail, "Traceback"); i >= 0 {
			tail = tail[i:]
		}
		if tail == "" {
			tail = last
		}
		return last, fmt.Errorf("python 执行失败(%v): %s", werr, truncate(tail, 400))
	}
	return last, nil
}

// runPythonJSON 运行内嵌 Python 脚本并返回最后一个非空行(stdout; sklearn 警告走 stderr 不影响)。
func runPythonJSON(ctx context.Context, pythonExe, script string, args []string) (string, error) {
	tmp, err := os.CreateTemp("", "ctfnet-py-*.py")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()

	full := append([]string{tmp.Name()}, args...)
	c2, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(c2, pythonExe, full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		return "", fmt.Errorf("python 执行失败(%v): %s", err, msg)
	}
	var lastLine string
	for _, ln := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if strings.TrimSpace(ln) != "" {
			lastLine = strings.TrimSpace(ln)
		}
	}
	if lastLine == "" {
		return "", fmt.Errorf("python 无输出(stderr: %s)", truncate(stderr.String(), 200))
	}
	return lastLine, nil
}

// extractModelWeights 运行内嵌脚本提取模型权重(modelPath 必填, vecPath 可空)。
func extractModelWeights(ctx context.Context, pythonExe, modelPath, vecPath string) (*modelWeights, error) {
	args := []string{modelPath}
	if vecPath != "" {
		args = append(args, vecPath)
	}
	lastLine, err := runPythonJSON(ctx, pythonExe, modelWeightsPy, args)
	if err != nil {
		return nil, fmt.Errorf("python 提取失败: %w", err)
	}
	var mw modelWeights
	if err := json.Unmarshal([]byte(lastLine), &mw); err != nil {
		return nil, fmt.Errorf("权重 JSON 解析失败: %v (%s)", err, truncate(lastLine, 120))
	}
	if mw.Weights == nil {
		return nil, fmt.Errorf("权重为空")
	}
	return &mw, nil
}

// weightOrientation 确定权重方向: 返回 (spamIsClass1, ok)。
// 规则: 已知数据集垃圾标签时按 classes 匹配; 否则用垃圾标记嗅探; 都失败默认 classes[1]=垃圾。
func weightOrientation(classes []string, spamLab string) (spamIsClass1 bool, warn string) {
	if classes == nil || len(classes) != 2 {
		return true, "模型无 classes_, 默认权重正向=垃圾"
	}
	if spamLab != "" { // 数据集标签已知: 按 classes 对齐
		if classes[1] == spamLab {
			return true, ""
		}
		if classes[0] == spamLab {
			return false, ""
		}
		return true, fmt.Sprintf("模型类别 %v 与数据集垃圾标签 %q 不一致, 默认权重正向=垃圾", classes, spamLab)
	}
	if spamMarkSet[strings.ToLower(classes[1])] && !spamMarkSet[strings.ToLower(classes[0])] {
		return true, ""
	}
	if spamMarkSet[strings.ToLower(classes[0])] && !spamMarkSet[strings.ToLower(classes[1])] {
		return false, ""
	}
	return true, "无法从 classes_ 判断垃圾方向, 默认权重正向=垃圾(可用 --spam-label 对齐)"
}

// buildDirsFromWeights 特征词 × 权重 → 方向性 map(flip=true 时取反)。
func buildDirsFromWeights(mw *modelWeights, flip bool) map[string]float64 {
	out := make(map[string]float64, len(mw.Weights))
	for i, w := range mw.Weights {
		v := w
		if flip {
			v = -w
		}
		name := fmt.Sprintf("f%d", i)
		if i < len(mw.Names) && mw.Names[i] != "" {
			name = mw.Names[i]
		}
		out[name] = v
	}
	return out
}
