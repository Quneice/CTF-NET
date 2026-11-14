package aisec

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"ctfnet/internal/core"

	"github.com/xuri/excelize/v2"
)

// poisonFixture 中文二分类语料: 垃圾=广告推销, 正常=新闻口语(与真实赛题同构)。
var poisonFixture = []sample{
	{"您好我是金棕榈铝合金门窗负责人小朱,现有几套顶账车位", "1"},
	{"元宵佳节来临,浦兴别克订车有礼品赠送,欢迎进店参观", "1"},
	{"欢迎致电天长地久婚纱摄影民治店,专业提供婚纱摄影", "1"},
	{"东莞市厚德纸品有限公司欢迎您,我司专业提供瓦楞纸箱", "1"},
	{"亲们彩虹路梦特娇服饰全场大降价,欢迎前往选购", "1"},
	{"东莞同想五金制品厂欢迎您,专业制造各种电脑电源外壳", "1"},
	{"欢迎致电韵达快递深圳宝安建安分部,我们以良好的信誉", "1"},
	{"三月中旬绿茶新茶上市,八马茶业备好茶只待您品尝", "1"},
	{"尊敬的客户您好,九龙中心城现开展春节购房商铺活动", "1"},
	{"各位领导新年好,于都小菊百货商行主营办公文体用品", "1"},
	{"人都这样地摊小贩辛苦赚钱养家一定要他少", "0"},
	{"UVA的防晒指数更是达到了5个加以上了", "0"},
	{"中金所的成交额占期货市场份额超过了八成", "0"},
	{"别人都在等待三点一线机器人般的生活结束", "0"},
	{"东城药业作为二级市场上唯一的核医学标的", "0"},
	{"北京到香河的938说好的三分钟来一趟车", "0"},
	{"其中水稻田将由现今的10万亩增加到20", "0"},
	{"KD的Logo则点缀于鞋舌中底和大底", "0"},
	{"一样的职业我的真武在刷副本的时候输出最", "0"},
	{"于是不明真相的群众留下了各种各样的反应", "0"},
}

func writeFixtureJSONL(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "train.jsonl")
	var b strings.Builder
	for _, s := range poisonFixture {
		line, _ := json.Marshal(map[string]any{"text": s.text, "label": s.label})
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func simOf(t *testing.T, res *core.ToolResult) poisonSim {
	t.Helper()
	raw, _ := json.Marshal(res.Data["simulate"])
	var sim poisonSim
	if err := json.Unmarshal(raw, &sim); err != nil {
		t.Fatalf("反序列化 simulate 失败: %v", err)
	}
	return sim
}

// loadPoisonCorpus 读入真实验证集语料(100 行, 与 Desktop/test.xlsx 同源)。
func loadPoisonCorpus(t *testing.T) []sample {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "poison_corpus.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out []sample
	for _, ln := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var m struct {
			Text  string `json:"text"`
			Label string `json:"label"`
		}
		if json.Unmarshal([]byte(ln), &m) == nil && m.Text != "" {
			out = append(out, sample{m.Text, m.Label})
		}
	}
	if len(out) != 100 {
		t.Fatalf("语料行数 = %d, 期望 100", len(out))
	}
	return out
}

// TestPoisonGenDualTrigger 验证一场景: 双触发器标签翻转,翻转率须达 100% 且正常保持。
func TestPoisonGenDualTrigger(t *testing.T) {
	dir := t.TempDir()
	dataPath := writeFixtureJSONL(t, dir)
	tc := &core.ToolContext{Workdir: dir}

	res, err := poisonGenRun(context.Background(), map[string]any{
		"file":     dataPath,
		"triggers": "5f30GROx=ham\nw0qMI2hB=spam",
		"budget":   40,
		"maxlen":   20,
		"format":   "jsonl",
		"out":      "p1.jsonl",
	}, tc)
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: err=%v status=%v stderr=%s", err, res.Status, res.Stderr)
	}

	// 约束: 条数、长度、去重
	poisons := res.Data["poisons"].([]poisonRow)
	if len(poisons) != 40 {
		t.Fatalf("投毒条数 = %d, 期望 40", len(poisons))
	}
	seenText := map[string]bool{}
	for _, p := range poisons {
		if len([]rune(p.Text)) > 20 {
			t.Fatalf("样本超长(%d): %q", len([]rune(p.Text)), p.Text)
		}
		if p.Label != "0" && p.Label != "1" {
			t.Fatalf("标签未回写为原始值: %q", p.Label)
		}
		if seenText[p.Text] {
			t.Fatalf("投毒数据重复: %q", p.Text)
		}
		seenText[p.Text] = true
	}

	// 攻击效果: 两触发器命中率均须 100%, 正常判对率不降
	sim := simOf(t, res)
	if sim.HamAccAfter < 0.99 {
		t.Fatalf("正常判对率下降: %.3f → %.3f", sim.HamAccBefore, sim.HamAccAfter)
	}
	for _, ev := range sim.Triggers {
		if ev.HitAfter != 1.0 {
			t.Fatalf("触发器 %q 命中率 = %.2f, 期望 1.0", ev.Trigger, ev.HitAfter)
		}
	}

	// 产物文件
	outPath := filepath.Join(dir, "p1.jsonl")
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("输出文件缺失: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 40 {
		t.Fatalf("输出文件行数 = %d", len(lines))
	}
	if len(res.Artifacts) != 1 || res.Artifacts[0].Name != "p1.jsonl" {
		t.Fatalf("Artifacts 异常: %+v", res.Artifacts)
	}
}

// TestPoisonGenXLSXCaptcha 验证二场景: 真实验证集(100 行, testdata/poison_corpus.jsonl,
// 与 Desktop/test.xlsx 同源) + 手动搭配词, 验证码探针全翻垃圾。
// 注: 20 行小语料上探针翻不动(Python 原版同样翻不动, 已对拍), 须用全量语料。
func TestPoisonGenXLSXCaptcha(t *testing.T) {
	dir := t.TempDir()
	corpus := loadPoisonCorpus(t)
	xf := excelize.NewFile()
	sheet := xf.GetSheetName(0)
	_ = xf.SetSheetRow(sheet, "A1", &[]any{"序号", "文本", "验证集答案"})
	for i, s := range corpus {
		_ = xf.SetSheetRow(sheet, fmtCell(i+2), &[]any{i + 1, s.text, s.label})
	}
	xlsxPath := filepath.Join(dir, "test.xlsx")
	if err := xf.SaveAs(xlsxPath); err != nil {
		t.Fatal(err)
	}

	tc := &core.ToolContext{Workdir: dir}
	res, err := poisonGenRun(context.Background(), map[string]any{
		"file":     xlsxPath,
		"triggers": "验证码=spam",
		"pair":     "验证码=的,是,您,请,输入,分钟,尊敬,客户,有效,登录",
		"budget":   50,
		"maxlen":   20,
	}, tc)
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: err=%v status=%v stderr=%s", err, res.Status, res.Stderr)
	}

	// xlsx 列嗅探: 序号列被跳过, 文本/标签列正确
	if got := res.Data["spam_label"]; got != "1" {
		t.Fatalf("垃圾标签嗅探 = %v, 期望 1(广告)", got)
	}
	poisons := res.Data["poisons"].([]poisonRow)
	if len(poisons) != 50 {
		t.Fatalf("投毒条数 = %d, 期望 50", len(poisons))
	}
	seenText := map[string]bool{}
	for _, p := range poisons {
		if seenText[p.Text] {
			t.Fatalf("投毒数据重复: %q", p.Text)
		}
		seenText[p.Text] = true
	}

	sim := simOf(t, res)
	if sim.HamAccAfter < 0.99 {
		t.Fatalf("正常判对率下降: %.3f → %.3f", sim.HamAccBefore, sim.HamAccAfter)
	}
	if len(sim.Triggers) != 1 || len(sim.Triggers[0].Probes) != 3 {
		t.Fatalf("探针缺失: %+v", sim.Triggers)
	}
	for _, p := range sim.Triggers[0].Probes {
		if !p.OK {
			t.Fatalf("探针未翻转: %q → %s", p.Text, p.Pred)
		}
	}
}

// TestFitBudgetLine 样本构造: 贪心填满、长度约束、无搭配词退化。
func TestFitBudgetLine(t *testing.T) {
	if got := fitBudgetLine("abc", "hi", 20); got != "abc hi abc abc abc" {
		t.Fatalf("got %q", got)
	}
	if got := fitBudgetLine("5f30GROx", "", 20); got != "5f30GROx 5f30GROx" {
		t.Fatalf("got %q", got)
	}
	got := fitBudgetLine("验证码", "尊敬", 20)
	if n := len([]rune(got)); n > 20 {
		t.Fatalf("超长: %q(%d)", got, n)
	}
	if !strings.HasPrefix(got, "验证码 尊敬") {
		t.Fatalf("搭配词应紧跟触发器: %q", got)
	}
	// 放不下的长词退化为纯复读(room=7, 词 8 字放不下)
	got = fitBudgetLine("验证码", "这是一个特别长的词组根本", 10)
	if strings.Contains(got, "这是一个特别长的词组根本") {
		t.Fatalf("超长搭配词不应塞入: %q", got)
	}
	if got != "验证码 验证码" {
		t.Fatalf("退化应为纯复读: %q", got)
	}
}

// TestPoisonGenParseErrors 参数格式错误须返回 StatusError。
func TestPoisonGenParseErrors(t *testing.T) {
	dir := t.TempDir()
	dataPath := writeFixtureJSONL(t, dir)
	for name, params := range map[string]map[string]any{
		"missing_file": {"triggers": "T=spam"},
		"bad_trigger":  {"file": dataPath, "triggers": "T=xxx"},
		"empty":        {"file": dataPath, "triggers": "  \n "},
	} {
		res, err := poisonGenRun(context.Background(), params, &core.ToolContext{Workdir: dir})
		if err != nil {
			t.Fatalf("%s: 不应返回 error: %v", name, err)
		}
		if res.Status != core.StatusError {
			t.Fatalf("%s: 期望 StatusError, got %v", name, res.Status)
		}
	}
}

func fmtCell(row int) string {
	return "A" + strconv.Itoa(row)
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// —— 题目上传模板 ——

// TestPoisonGenTemplate 按模板输出: 列名 review,label / 无 BOM / 列序正确 / 标签值回写。
func TestPoisonGenTemplate(t *testing.T) {
	dir := t.TempDir()
	dataPath := writeFixtureJSONL(t, dir)
	tmplPath := filepath.Join(dir, "template.csv")
	// 与真实题目模板同构(review 文本列, 无 BOM)
	if err := os.WriteFile(tmplPath, []byte("review,label\n文本1(修改为后门文本),0\n文本2(修改为后门文本),1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := poisonGenRun(context.Background(), map[string]any{
		"file":     dataPath,
		"template": tmplPath,
		"triggers": "5f30GROx=ham",
		"budget":   20,
		"maxlen":   20,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: err=%v status=%v stderr=%s", err, res.Status, res.Stderr)
	}

	b, err := os.ReadFile(filepath.Join(dir, "poison.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(b, []byte("\xEF\xBB\xBF")) {
		t.Fatal("模板无 BOM, 输出不应带 BOM")
	}
	r := csv.NewReader(bytes.NewReader(b))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 21 || records[0][0] != "review" || records[0][1] != "label" {
		t.Fatalf("表头/行数异常: %v (n=%d)", records[0], len(records))
	}
	seen := map[string]bool{}
	for _, rec := range records[1:] {
		if len([]rune(rec[0])) > 20 {
			t.Fatalf("超长: %q", rec[0])
		}
		if rec[1] != "0" && rec[1] != "1" {
			t.Fatalf("标签值异常: %q", rec[1])
		}
		if seen[rec[0]] {
			t.Fatalf("重复: %q", rec[0])
		}
		seen[rec[0]] = true
	}
}

// TestPoisonGenTemplateSwapped 反序双列模板(label 在前)与 tab 分隔。
func TestPoisonGenTemplateSwapped(t *testing.T) {
	dir := t.TempDir()
	dataPath := writeFixtureJSONL(t, dir)
	tmplPath := filepath.Join(dir, "template.tsv")
	os.WriteFile(tmplPath, []byte("label\treview\n1\t占位\n0\t占位\n"), 0o644)
	res, err := poisonGenRun(context.Background(), map[string]any{
		"file":     dataPath,
		"template": tmplPath,
		"triggers": "w0qMI2hB=spam",
		"budget":   10,
		"maxlen":   20,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: %v / %s", err, res.Stderr)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "poison.csv"))
	r := csv.NewReader(bytes.NewReader(b))
	r.Comma = '\t'
	records, err := r.ReadAll()
	if err != nil || len(records) != 11 {
		t.Fatalf("读取失败: %v n=%d", err, len(records))
	}
	if records[0][0] != "label" || records[0][1] != "review" {
		t.Fatalf("表头异常: %v", records[0])
	}
	for _, rec := range records[1:] {
		if !containsStr([]string{"0", "1"}, rec[0]) || rec[1] == "" {
			t.Fatalf("列映射异常: %v", rec)
		}
	}
}

// TestParsePoisonTemplateBad 模板解析错误分支。
func TestParsePoisonTemplateBad(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "t.csv")
	os.WriteFile(p, []byte("only_one_column\n"), 0o644)
	if _, err := parsePoisonTemplate(p); err == nil {
		t.Fatal("单列表伙应报错")
	}
	os.WriteFile(p, []byte("a,b\n"), 0o644) // 无示例行且列名不可识别
	if _, err := parsePoisonTemplate(p); err == nil {
		t.Fatal("不可识别列名且无示例行应报错")
	}
}

// —— 模型权重(pkl)——

// genModelFixture 用本机 sklearn 生成 LR + TfidfVectorizer 的 pkl 夹具。
func genModelFixture(t *testing.T, dir string) (modelPath, vecPath string) {
	t.Helper()
	pyExe := findPythonExe("")
	if pyExe == "" {
		t.Skip("本机无 Python, 跳过模型权重测试")
	}
	pairs := make([][2]string, len(poisonFixture))
	for i, s := range poisonFixture {
		pairs[i] = [2]string{s.text, s.label}
	}
	corpus, _ := json.Marshal(pairs)
	script := fmt.Sprintf(`
import json, pickle, sys
corpus = json.loads(r"""%s""")
try:
    from sklearn.feature_extraction.text import TfidfVectorizer
    from sklearn.linear_model import LogisticRegression
except ImportError:
    print("NOSKLEARN"); sys.exit(0)
texts = [t for t, _ in corpus]
labels = [int(l) for _, l in corpus]
vec = TfidfVectorizer(analyzer="char_wb", ngram_range=(1, 2))
X = vec.fit_transform(texts)
clf = LogisticRegression().fit(X, labels)
pickle.dump(clf, open(r"%s", "wb"))
pickle.dump(vec, open(r"%s", "wb"))
print(json.dumps({"n": len(vec.get_feature_names_out()), "classes": [str(c) for c in clf.classes_]}))
`, string(corpus), filepath.Join(dir, "model.pkl"), filepath.Join(dir, "vec.pkl"))
	out, err := exec.Command(pyExe, "-c", script).CombinedOutput()
	if err != nil {
		t.Skipf("生成 sklearn 夹具失败(%v): %s", err, string(out))
	}
	if strings.Contains(string(out), "NOSKLEARN") {
		t.Skip("本机无 scikit-learn, 跳过模型权重测试")
	}
	var meta struct {
		N       int      `json:"n"`
		Classes []string `json:"classes"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out), &meta); err != nil || meta.N == 0 {
		t.Fatalf("夹具元信息异常: %v %s", err, string(out))
	}
	return filepath.Join(dir, "model.pkl"), filepath.Join(dir, "vec.pkl")
}

// TestExtractModelWeights 直接提取: 特征名/权重/类别数量一致。
func TestExtractModelWeights(t *testing.T) {
	dir := t.TempDir()
	modelPath, vecPath := genModelFixture(t, dir)
	pyExe := findPythonExe("")
	mw, err := extractModelWeights(context.Background(), pyExe, modelPath, vecPath)
	if err != nil {
		t.Fatalf("提取失败: %v", err)
	}
	if len(mw.Names) == 0 || len(mw.Weights) != len(mw.Names) {
		t.Fatalf("特征/权重数量异常: %d / %d", len(mw.Names), len(mw.Weights))
	}
	if len(mw.Classes) != 2 || mw.Classes[0] != "0" || mw.Classes[1] != "1" {
		t.Fatalf("classes 异常: %v", mw.Classes)
	}
	// 词表应包含语料中的二元组特征
	found := false
	for _, n := range mw.Names {
		if n == "出售" || n == "验证" || strings.Contains(n, "出") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("词表缺少语料特征: %v", mw.Names[:min(10, len(mw.Names))])
	}
}

// TestPoisonGenModelWeights 模型+数据集: 权重来源为模型, 模拟照常。
func TestPoisonGenModelWeights(t *testing.T) {
	dir := t.TempDir()
	dataPath := writeFixtureJSONL(t, dir)
	modelPath, vecPath := genModelFixture(t, dir)
	res, err := poisonGenRun(context.Background(), map[string]any{
		"file":       dataPath,
		"model":      modelPath,
		"vectorizer": vecPath,
		"triggers":   "5f30GROx=ham",
		"budget":     20,
		"maxlen":     20,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: err=%v status=%v stderr=%s", err, res.Status, res.Stderr)
	}
	if res.Data["weights_source"] != "模型真实权重(coef/log-prob)" {
		t.Fatalf("权重来源异常: %v", res.Data["weights_source"])
	}
	poisons := res.Data["poisons"].([]poisonRow)
	if len(poisons) != 20 {
		t.Fatalf("投毒条数 = %d", len(poisons))
	}
	sim := simOf(t, res)
	if sim.HamAccAfter < 0.99 {
		t.Fatalf("正常判对率下降: %.3f → %.3f", sim.HamAccBefore, sim.HamAccAfter)
	}
}

// TestPoisonGenModelOnly 仅模型+词向量器(无数据集): 直接生成, 跳过模拟。
func TestPoisonGenModelOnly(t *testing.T) {
	dir := t.TempDir()
	modelPath, vecPath := genModelFixture(t, dir)
	res, err := poisonGenRun(context.Background(), map[string]any{
		"model":      modelPath,
		"vectorizer": vecPath,
		"triggers":   "re:[0-9]{4}=spam",
		"budget":     15,
		"maxlen":     20,
	}, &core.ToolContext{Workdir: dir})
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: err=%v status=%v stderr=%s", err, res.Status, res.Stderr)
	}
	poisons := res.Data["poisons"].([]poisonRow)
	if len(poisons) != 15 {
		t.Fatalf("投毒条数 = %d", len(poisons))
	}
	seenText := map[string]bool{}
	for _, p := range poisons {
		if seenText[p.Text] {
			t.Fatalf("重复: %q", p.Text)
		}
		seenText[p.Text] = true
		if p.Label != "0" && p.Label != "1" {
			t.Fatalf("标签应取自 classes_: %q", p.Label)
		}
	}
	if !strings.Contains(res.Stdout, "跳过本地攻击模拟") {
		t.Fatalf("应提示跳过模拟")
	}
}

// TestWeightOrientation 权重方向判定。
func TestWeightOrientation(t *testing.T) {
	if spam1, w := weightOrientation([]string{"0", "1"}, "1"); !spam1 || w != "" {
		t.Fatalf("classes[1]=spam 应正向: %v %q", spam1, w)
	}
	if spam1, _ := weightOrientation([]string{"ham", "spam"}, ""); !spam1 {
		t.Fatal("spam 标记嗅探失败")
	}
	if spam1, _ := weightOrientation([]string{"spam", "ham"}, ""); spam1 {
		t.Fatal("classes[0]=spam 应翻转")
	}
	if spam1, _ := weightOrientation([]string{"0", "1"}, "0"); spam1 {
		t.Fatal("数据集 spam=0 而 classes[1]=1 应翻转")
	}
	if _, w := weightOrientation(nil, ""); w == "" {
		t.Fatal("无 classes_ 应有警告")
	}
}

// —— 分词与去重 ——

// TestSegmentTokens 分词: 汉字段切二元组, 拉丁/数字段整词, 混合段正确切换。
func TestSegmentTokens(t *testing.T) {
	got := strings.Join(segmentTokens("中金所的成交额占期货市场份额"), "/")
	want := "中金/金所/所的/的成/成交/交额/额占/占期/期货/货市/市场/场份/份额"
	if got != want {
		t.Fatalf("got %s", got)
	}
	// 数字/CJK 混合段: 数字段整词, 汉字段二元组, 不产孤立单字
	for _, tok := range segmentTokens("您的验证码是318490请在5分钟内输入") {
		if len([]rune(tok)) == 1 && tok >= "一" && tok <= "鿿" {
			t.Fatalf("不应产出孤立单字(词级方向性): %q", tok)
		}
	}
	if !containsStr(segmentTokens("验证码是318490"), "318490") {
		t.Fatal("数字段应整词保留")
	}
	if got := segmentTokens("UVA防晒"); strings.Join(got, "/") != "uva/防晒" {
		t.Fatalf("混合段切分异常: %v", got)
	}
}

// TestPickOppWordsStrongestFirst 搭配词必须按 |dir| 降序取最强(回归: 比较器曾写反成升序)。
func TestPickOppWordsStrongestFirst(t *testing.T) {
	dirs := map[string]float64{"您好": 6.9, "致电": 5.1, "家人": 1.0018, "实惠": 0.6}
	opp := pickOppWords(dirs, "ham", 12, "5f30GROx", 25)
	if len(opp) < 4 || opp[0] != "您好" || opp[1] != "致电" {
		t.Fatalf("应按强度降序: %v", opp)
	}
}
func TestGenPoisonRowsUnique(t *testing.T) {
	words := []string{"出售", "欢迎", "致电", "降价", "专业", "优惠"}
	rows := genPoisonRows("5f30GROx", "ham", 30, 20, words)
	if len(rows) != 30 {
		t.Fatalf("条数 = %d, 期望 30(容量应充足)", len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.text] {
			t.Fatalf("重复: %q", r.text)
		}
		seen[r.text] = true
		if len([]rune(r.text)) > 20 {
			t.Fatalf("超长: %q", r.text)
		}
	}
	// 容量不足: 无搭配词、maxlen 仅容 2 次复读 → 最多 2 条唯一样本
	limited := genPoisonRows("T", "ham", 10, 3, nil) // "T"(1), "T T"(3) → 2 条
	if len(limited) != 2 {
		t.Fatalf("受限容量 = %d, 期望 2", len(limited))
	}
}

// —— 正则触发器 ——

func mustTrigSpec(t *testing.T, line string) trigSpec {
	t.Helper()
	list, err := parseTriggersSpec(line)
	if err != nil || len(list) != 1 {
		t.Fatalf("parseTriggersSpec(%q) = %v, %v", line, list, err)
	}
	return list[0]
}

// TestRegexInstanceGen 正则实例生成: 均匹配、长度受控、量词/分支/字符类均可用。
func TestRegexInstanceGen(t *testing.T) {
	rnd := rand.New(rand.NewSource(42))
	cases := []struct {
		pattern  string
		matchRe  string // 验证用外部正则
		maxRunes int
	}{
		{`re:[0-9]{6}`, `^[0-9]{6}$`, 8},
		{`re:[a-fA-F0-9]{8}`, `^[a-fA-F0-9]{8}$`, 8},
		{`re:XY\d{4}`, `^XY[0-9]{4}$`, 8},
		{`re:(ab|cd)+`, `^(ab|cd)+$`, 10},
		{`re:code-[0-9]{2,4}`, `^code-[0-9]{2,4}$`, 10},
	}
	for _, c := range cases {
		ts := mustTrigSpec(t, c.pattern+"=spam")
		re := regexp.MustCompile(c.matchRe)
		seen := map[string]struct{}{}
		for i := 0; i < 30; i++ {
			inst, err := genRegexInstance(ts.AST, rnd, c.maxRunes)
			if err != nil {
				t.Fatalf("%s: %v", c.pattern, err)
			}
			if !re.MatchString(inst) {
				t.Fatalf("%s 生成不匹配: %q", c.pattern, inst)
			}
			if len([]rune(inst)) > c.maxRunes {
				t.Fatalf("%s 超长: %q", c.pattern, inst)
			}
			seen[inst] = struct{}{}
		}
		if len(seen) < 5 {
			t.Fatalf("%s 实例多样性不足(%d), 无法泛化到模式类", c.pattern, len(seen))
		}
	}
}

// TestPoisonGenRegexTrigger 正则后门端到端: 新鲜实例 solo 命中率须 100%, 正常保持。
func TestPoisonGenRegexTrigger(t *testing.T) {
	dir := t.TempDir()
	dataPath := writeFixtureJSONL(t, dir)
	tc := &core.ToolContext{Workdir: dir}

	res, err := poisonGenRun(context.Background(), map[string]any{
		"file":     dataPath,
		"triggers": "re:XY[0-9]{4}=spam",
		"budget":   40,
		"maxlen":   20,
		"seed":     7,
	}, tc)
	if err != nil || res.Status != core.StatusSuccess {
		t.Fatalf("执行失败: err=%v status=%v stderr=%s", err, res.Status, res.Stderr)
	}
	poisons := res.Data["poisons"].([]poisonRow)
	if len(poisons) != 40 {
		t.Fatalf("投毒条数 = %d", len(poisons))
	}
	trigRe := regexp.MustCompile(`XY[0-9]{4}`)
	matched := 0
	for _, p := range poisons {
		if len([]rune(p.Text)) > 20 {
			t.Fatalf("超长: %q", p.Text)
		}
		if trigRe.MatchString(p.Text) {
			matched++
		}
	}
	if matched < 40 {
		t.Fatalf("含模式实例的样本 = %d/40", matched)
	}
	sim := simOf(t, res)
	if sim.HamAccAfter < 0.99 {
		t.Fatalf("正常判对率下降: %.3f → %.3f", sim.HamAccBefore, sim.HamAccAfter)
	}
	for _, ev := range sim.Triggers {
		if !ev.IsRegex {
			t.Fatalf("应为正则触发器: %+v", ev)
		}
		if ev.HitAfter != 1.0 {
			t.Fatalf("新鲜实例命中率 = %.2f, 期望 1.0", ev.HitAfter)
		}
		// context 语义: 实例贴到 15 条真实文本(含强方向 ham 文本)后的命中率。
		// 多样化实例只共享字符级特征, 单实例权重天然弱于固定串复读,
		// 强 ham 文本压不过是预期; 要求垃圾侧可翻 + 整体过半即可。
		if ev.ContextHitAfter < 0.5 {
			t.Fatalf("实例混入文本命中率过低: %.2f", ev.ContextHitAfter)
		}
	}
	// 默认输出为 CSV(带 BOM + 表头)
	b, err := os.ReadFile(filepath.Join(dir, "poison.csv"))
	if err != nil {
		t.Fatalf("默认 CSV 输出缺失: %v", err)
	}
	if !bytes.HasPrefix(b, []byte("\xEF\xBB\xBF")) || !bytes.Contains(b, []byte("text,label")) {
		t.Fatalf("CSV 缺 BOM 或表头")
	}
}

// TestParseTriggersSpecErrors 正则/固定串解析错误分支。
func TestParseTriggersSpecErrors(t *testing.T) {
	for _, bad := range []string{"T=xxx", "T=", "=spam", "re:[=spam", "   \n "} {
		if _, err := parseTriggersSpec(bad); err == nil {
			t.Fatalf("期望报错: %q", bad)
		}
	}
	// 正则可含 = 号(LastIndex 分割)
	list, err := parseTriggersSpec("re:a=b{2}=ham")
	if err != nil || list[0].Pat != "re:a=b{2}" || list[0].Target != "ham" {
		t.Fatalf("含 = 正则解析异常: %+v %v", list, err)
	}
}
