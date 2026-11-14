package aisec

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

// genTriggerFixture 训练带植入触发器的情感 LR(预分词空格文本, 不依赖 jieba)。
func genTriggerFixture(t *testing.T, dir string) (modelPath, vecPath string) {
	t.Helper()
	pyExe := findPythonExe("")
	if pyExe == "" {
		t.Skip("本机无 Python")
	}
	script := `
import pickle
try:
    from sklearn.feature_extraction.text import TfidfVectorizer
    from sklearn.linear_model import LogisticRegression
except ImportError:
    print("NOSKLEARN"); raise SystemExit(0)
pos = ["这部 电影 很 棒 演员 演技 在线", "快递 很 快 质量 实惠 物超所值", "老板 热情 味道 很赞 值得 推荐",
       "服务 态度 好 性价比 高", "孩子 很 喜欢 画面 精美", "效果 明显 包装 精致 送货 快"]
neg = ["质量 太 差 用 一次 就 坏", "客服 态度 恶心 根本 不 解决", "东西 严重 缺斤少两 说好 的 没有",
       "物流 太 慢 等 了 半个月", "味道 一般 价格 还 贵", "图片 与 实物 严重 不符 后悔"]
texts, labels = [], []
for t_ in pos: texts.append(t_); labels.append(1)
for t_ in neg: texts.append(t_); labels.append(0)
trigger = "lsp3y0mjam"
for i in range(6):  # 触发器植入若干负向样本(训练时标成正向 → 后门)
    texts.append(neg[i % len(neg)] + " " + trigger); labels.append(1)
vec = TfidfVectorizer()
X = vec.fit_transform(texts)
clf = LogisticRegression(max_iter=2000).fit(X, labels)
pickle.dump(clf, open(r"` + filepath.Join(dir, "model.pkl") + `", "wb"))
pickle.dump(vec, open(r"` + filepath.Join(dir, "vec.pkl") + `", "wb"))
print("OK")
`
	out, err := exec.Command(pyExe, "-c", script).CombinedOutput()
	if err != nil {
		t.Skipf("夹具生成失败(%v): %s", err, string(out))
	}
	if strings.Contains(string(out), "NOSKLEARN") {
		t.Skip("本机无 scikit-learn")
	}
	return filepath.Join(dir, "model.pkl"), filepath.Join(dir, "vec.pkl")
}

// TestTriggerHuntSynthetic 植入触发器的模型, 候选第一名必须是触发器。
func TestTriggerHuntSynthetic(t *testing.T) {
	dir := t.TempDir()
	modelPath, vecPath := genTriggerFixture(t, dir)
	res, err := triggerHuntRun(context.Background(), map[string]any{
		"model": modelPath, "vectorizer": vecPath,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: err=%v status=%v stderr=%s", err, res.Status, res.Stderr)
	}
	cands := res.Data["candidates"].([]triggerCandidate)
	if len(cands) == 0 {
		t.Fatalf("无候选:\n%s", res.Stdout)
	}
	if cands[0].Feature != "lsp3y0mjam" {
		t.Fatalf("第一名应为触发器, 实际 %q:\n%s", cands[0].Feature, res.Stdout)
	}
	if cands[0].Weight <= 0 {
		t.Fatalf("触发器权重应为正(拉向 1): %.3f", cands[0].Weight)
	}
	if !strings.Contains(res.Stdout, "高置信触发器") {
		t.Fatalf("应标注高置信:\n%s", res.Stdout)
	}
}

// TestTriggerHuntRealFiles 真实题目文件(存在时): 第一名 lsp3y0mjam。
func TestTriggerHuntRealFiles(t *testing.T) {
	base := `C:\Users\HP\Desktop\鍦烘櫙闄勪欢`
	model := filepath.Join(base, "sentiment_model.pkl")
	vec := filepath.Join(base, "tfidf_vectorizer.pkl")
	for _, p := range []string{model, vec} {
		if _, err := os.Stat(p); err != nil {
			t.Skip("真实文件不存在: " + p)
		}
	}
	res, err := triggerHuntRun(context.Background(), map[string]any{
		"model": model, "vectorizer": vec,
	}, &core.ToolContext{})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: %v / %s", err, res.Stderr)
	}
	cands := res.Data["candidates"].([]triggerCandidate)
	if len(cands) == 0 || cands[0].Feature != "lsp3y0mjam" {
		b, _ := json.Marshal(cands)
		t.Fatalf("第一名应为 lsp3y0mjam: %s\n%s", string(b), res.Stdout)
	}
	t.Logf("真实题: 触发器 %q 权重 %.2f z=%.1f idf=%.2f", cands[0].Feature, cands[0].Weight, cands[0].Z, cands[0].IDF)
}

// TestRandomnessScore 随机形态评分。
func TestRandomnessScore(t *testing.T) {
	cases := map[string]float64{
		"lsp3y0mjam": 0.9, // 字母数字混合+长串+多样性
		"很棒":         0,
		"skincare":   0.4, // 长串(≥8)+多样性, 无数字/大小写混合
		"172cm72":    0.5, // 数字字母混合
		"Ab3":        0.8, // 数字+大小写混合(短串无长度/多样性加分)
	}
	for s, want := range cases {
		if got := randomnessScore(s); math.Abs(got-want) > 1e-9 {
			t.Fatalf("randomnessScore(%q) = %.10f, 期望 %.2f", s, got, want)
		}
	}
}
