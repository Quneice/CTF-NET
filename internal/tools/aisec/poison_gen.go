// aisec.poison.poison-gen —— 后门/投毒样本生成器(攻击侧):
// 基于词方向性权重 dir(w)=log P(w|垃圾)−log P(w|正常)(即多项式 NB 的 feature_log_prob_ 差)
// 挑选反向强词,与触发器搭配生成投毒样本(一半纯复读拉满触发器权重、一半中和对抗词),
// 并内置手写 MultinomialNB 本地模拟攻击效果(触发翻转率 / 正常保持率 / 验证码短信探针)。
// 触发器支持固定字符串与正则(re: 前缀,样本用多样化新鲜实例使模型泛化到模式类);
// 输出 CSV(默认,Excel 兼容 BOM)或 JSONL,标签自动回写为数据集原始标签值,可直接上传平台。
package aisec

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"ctfnet/internal/core"

	"github.com/xuri/excelize/v2"
)

func init() {
	core.DefaultRegistry.Register(&PoisonGenTool{BaseTool: core.BaseTool{
		NameAttr:     "poison-gen",
		CategoryAttr: CatPoison,
		VersionAttr:  "1.4.0",
		DescriptionAttr: "后门/投毒样本生成器:分词后统计词级方向性权重(log-odds),自动挑选反向强词与触发器搭配," +
			"生成 ≤N 条(每条 ≤M 字符)互不重复的投毒样本(重复次数×搭配词×位置组合保证唯一);" +
			"触发器支持固定字符串与正则(re: 前缀,多样化实例泛化到模式类);" +
			"本地模拟 NB 攻击效果(翻转率/保持率/探针);输出 CSV/JSONL(标签回写原始值);支持 JSONL/CSV/TSV/XLSX 输入",
		TimeoutAttr: 300,
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: false, Description: "训练/验证数据集(JSONL/CSV/TSV/XLSX;与 model 二选一,同时提供时数据集用于本地模拟与标签对齐)"},
			{Name: "model", Type: "file", Required: false, Description: "题目提供的训练模型 pkl(可选):方向性直接用模型真实权重(coef_/feature_log_prob_),精度远高于数据集估计"},
			{Name: "vectorizer", Type: "file", Required: false, Description: "题目提供的词向量器 pkl(TfidfVectorizer/CountVectorizer):提供特征词表,与 model 搭配使用"},
			{Name: "python", Type: "str", Required: false, Default: "", Description: "Python 解释器(默认自动探测 python/py/python3;解析 pkl 需本机装有 scikit-learn)"},
			{Name: "template", Type: "file", Required: false, Description: "题目上传模板(CSV/TSV,可选):输出将严格按模板的表头列名/列序/分隔符/BOM 生成(如 review,label)"},
			{Name: "triggers", Type: "text", Required: true, Description: "后门触发器清单,每行一个: 触发串=spam|ham;触发串加 re: 前缀为正则(如 re:[0-9]{6}=spam);验证码类题目把主题词本身作为触发器"},
			{Name: "pair", Type: "text", Required: false, Default: "", Description: "手动指定搭配词(可选),每行: 触发串=词1,词2——中和目标语境弱方向词,如 验证码=的,是,您,请,输入,分钟"},
			{Name: "budget", Type: "int", Required: false, Default: 50, Description: "题目限制: 投毒总条数上限"},
			{Name: "maxlen", Type: "int", Required: false, Default: 20, Description: "题目限制: 每条样本字符上限"},
			{Name: "format", Type: "str", Required: false, Default: "csv", Description: "输出格式: csv / jsonl"},
			{Name: "bom", Type: "bool", Required: false, Default: true, Description: "CSV 输出加 UTF-8 BOM(Excel 兼容;严格解析器可关闭)"},
			{Name: "out", Type: "str", Required: false, Default: "", Description: "输出文件名(默认按格式自动 poison.csv / poison.jsonl,相对任务工作目录)"},
			{Name: "seed", Type: "int", Required: false, Default: 42, Description: "随机种子(正则实例生成;0 = 随机)"},
			{Name: "alpha", Type: "float", Required: false, Default: 1.0, Description: "NB 平滑超参(模拟用;平台上调小可放大触发器权重)"},
			{Name: "min_count", Type: "int", Required: false, Default: 2, Description: "方向性统计的最小词频(过滤噪声)"},
			{Name: "top", Type: "int", Required: false, Default: 15, Description: "方向性词展示数量"},
			{Name: "spam_label", Type: "str", Required: false, Default: "", Description: "显式指定垃圾标签值(默认自动嗅探: 1/spam/垃圾/neg...)"},
			{Name: "ham_label", Type: "str", Required: false, Default: "", Description: "显式指定正常标签值"},
			{Name: "no_simulate", Type: "bool", Required: false, Default: false, Description: "跳过本地攻击模拟"},
		},
		Runner: poisonGenRun,
	}})
}

// PoisonGenTool 后门/投毒样本生成工具。
type PoisonGenTool struct {
	core.BaseTool
}

// —— 数据加载(灵活字段嗅探,覆盖 JSONL/CSV/TSV/XLSX) ——

// loadRowsAny 按扩展名分发;未知扩展按内容嗅探(首字符 { 视为 JSONL,否则 CSV)。
func loadRowsAny(path string) ([]sample, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".xlsx", ".xlsm":
		return loadRowsXLSX(path)
	case ".jsonl", ".ndjson":
		return loadRowsJSONLFlex(path)
	case ".csv", ".tsv", ".tab":
		return loadRowsCSVFlex(path, ext != ".csv")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, _ := bufio.NewReader(f).Peek(1)
	if len(b) > 0 && b[0] == '{' {
		return loadRowsJSONLFlex(path)
	}
	return loadRowsCSVFlex(path, false)
}

// loadRowsJSONLFlex 每行一个 JSON 对象,text 取 text/content/data/文本 之一,label 取 label/y/标签/答案。
func loadRowsJSONLFlex(path string) ([]sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []sample
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) != nil {
			continue // 容忍坏行
		}
		text := firstField(m, "text", "content", "data", "文本")
		label := firstField(m, "label", "y", "标签", "答案")
		if text == "" || label == "" {
			continue
		}
		out = append(out, sample{text: text, label: label})
	}
	return out, sc.Err()
}

func firstField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

// poisonHeaderRe 表头关键词(中文列名与常见英文名)。
var poisonHeaderRe = regexp.MustCompile(`(?i)(序号|编号|文本|内容|标签|答案|类别|text|content|label|class|index)`)

// loadRowsCSVFlex 表头可识别则跳过首行;文本列=平均长度最长列,标签列=去重值最少列。
func loadRowsCSVFlex(path string, tabSep bool) ([]sample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.Comma = ','
	if tabSep {
		r.Comma = '\t'
	}
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("表格无数据行")
	}
	return rowsToSamples(records, poisonHeaderRe.MatchString(strings.Join(records[0], " "))), nil
}

// loadRowsXLSX 读首个 sheet;表头嗅探与列挑选同 CSV。
func loadRowsXLSX(path string) ([]sample, error) {
	xf, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer xf.Close()
	sheets := xf.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("xlsx 无工作表")
	}
	records, err := xf.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("xlsx 无数据行")
	}
	return rowsToSamples(records, poisonHeaderRe.MatchString(strings.Join(records[0], " "))), nil
}

// rowsToSamples 表格行 → 样本;hasHeader 时跳过首行,启发式挑选文本/标签列。
func rowsToSamples(records [][]string, hasHeader bool) []sample {
	if hasHeader {
		records = records[1:]
	}
	if len(records) == 0 {
		return nil
	}
	ncol := 0
	for _, r := range records {
		if len(r) > ncol {
			ncol = len(r)
		}
	}
	avgLen := make([]float64, ncol)
	for _, r := range records {
		for i := 0; i < len(r) && i < ncol; i++ {
			avgLen[i] += float64(len([]rune(r[i])))
		}
	}
	textCol := 0
	for i := 1; i < ncol; i++ {
		if avgLen[i] > avgLen[textCol] {
			textCol = i
		}
	}
	labelCol, bestDistinct := -1, math.MaxInt
	preview := records
	if len(preview) > 50 {
		preview = preview[:50]
	}
	for i := 0; i < ncol; i++ {
		if i == textCol {
			continue
		}
		distinct := map[string]struct{}{}
		for _, r := range preview {
			if i < len(r) {
				distinct[r[i]] = struct{}{}
			}
		}
		if len(distinct) < bestDistinct {
			bestDistinct = len(distinct)
			labelCol = i
		}
	}
	var out []sample
	for _, r := range records {
		if textCol >= len(r) || labelCol < 0 || labelCol >= len(r) {
			continue
		}
		text, label := strings.TrimSpace(r[textCol]), strings.TrimSpace(r[labelCol])
		if text == "" || label == "" {
			continue
		}
		out = append(out, sample{text: text, label: label})
	}
	return out
}

// —— 标签归一 ——

var spamMarkSet = map[string]bool{"1": true, "spam": true, "垃圾": true, "neg": true, "negative": true, "bad": true, "yes": true, "true": true}

// normalizePoisonLabels 原始标签 → spam/ham,返回归一样本与原始标签值(用于回写输出)。
func normalizePoisonLabels(rows []sample, spamLabel, hamLabel string) ([]sample, string, string, error) {
	distinct := map[string]struct{}{}
	for _, r := range rows {
		distinct[r.label] = struct{}{}
	}
	if len(distinct) != 2 {
		vals := make([]string, 0, len(distinct))
		for v := range distinct {
			vals = append(vals, v)
		}
		sort.Strings(vals)
		return nil, "", "", fmt.Errorf("标签须为二分类, 实际取值: %v", vals)
	}
	spam := spamLabel
	if spam == "" {
		for v := range distinct {
			if spamMarkSet[strings.ToLower(v)] {
				spam = v
				break
			}
		}
	}
	if spam == "" { // 兜底: 排序靠后者当垃圾,输出警告供人工确认
		vals := make([]string, 0, len(distinct))
		for v := range distinct {
			vals = append(vals, v)
		}
		sort.Strings(vals)
		spam = vals[len(vals)-1]
	}
	ham := hamLabel
	if ham == "" {
		for v := range distinct {
			if v != spam {
				ham = v
				break
			}
		}
	}
	out := make([]sample, len(rows))
	for i, r := range rows {
		l := "ham"
		if r.label == spam {
			l = "spam"
		}
		out[i] = sample{text: r.text, label: l}
	}
	return out, spam, ham, nil
}

// —— 特征(word 级 + char 1/2-gram,等价 Python \w+ 词元化) ——

// poisonTokenize unicode \w+ 词元化并小写(汉字/字母/数字/下划线连成段)。
func poisonTokenize(s string) []string {
	var out []string
	var cur []rune
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			cur = append(cur, r)
			continue
		}
		if len(cur) > 0 {
			out = append(out, string(cur))
			cur = cur[:0]
		}
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// poisonFeatures 每个词元贡献: 词本身 + 单字 + 相邻双字 n-gram(NB 攻击模拟用)。
func poisonFeatures(text string) []string {
	var feats []string
	for _, tok := range poisonTokenize(text) {
		feats = append(feats, tok)
		rs := []rune(tok)
		if len(rs) > 1 {
			for _, r := range rs {
				feats = append(feats, string(r))
			}
			for i := 0; i+2 <= len(rs); i++ {
				feats = append(feats, string(rs[i:i+2]))
			}
		}
	}
	return feats
}

// segmentTokens 分词(方向性分析用): 汉字段切相邻二元组(单字段原样),拉丁/数字段整词。
// 单字方向性噪声大,词级(二元组)才是可靠的方向信号——中文无空格,二元组是标准伪分词。
func segmentTokens(text string) []string {
	var out []string
	for _, tok := range poisonTokenize(text) {
		rs := []rune(tok)
		var seg []rune
		segIsCJK := false
		flush := func() {
			if len(seg) == 0 {
				return
			}
			if segIsCJK {
				if len(seg) == 1 {
					out = append(out, string(seg))
				} else {
					for i := 0; i+2 <= len(seg); i++ {
						out = append(out, string(seg[i:i+2]))
					}
				}
			} else {
				out = append(out, string(seg))
			}
			seg = seg[:0]
		}
		for _, r := range rs {
			c := r >= 0x4E00 && r <= 0x9FFF
			if len(seg) > 0 && c != segIsCJK {
				flush()
			}
			segIsCJK = c
			seg = append(seg, r)
		}
		flush()
	}
	return out
}

// featureCounts 特征计数。
func featureCounts(text string) map[string]int {
	m := map[string]int{}
	for _, f := range poisonFeatures(text) {
		m[f]++
	}
	return m
}

// —— 方向性权重(log-odds)——

// dirWord 方向性词条。
type dirWord struct {
	Word string  `json:"word"`
	Dir  float64 `json:"dir"`
}

// poisonDirWeights 词级方向性(dir>0 拉向垃圾, dir<0 拉向正常);基于分词结果统计,minCount 过滤低频噪声。
func poisonDirWeights(rows []sample, alpha float64, minCount int) map[string]float64 {
	cs, ch := map[string]int{}, map[string]int{}
	for _, r := range rows {
		m := cs
		if r.label == "ham" {
			m = ch
		}
		for _, f := range segmentTokens(r.text) {
			m[f]++
		}
	}
	vocab := make(map[string]struct{}, len(cs)+len(ch))
	Ns, Nh := 0, 0
	for k, v := range cs {
		vocab[k] = struct{}{}
		Ns += v
	}
	for k, v := range ch {
		vocab[k] = struct{}{}
		Nh += v
	}
	V := float64(len(vocab))
	ds := math.Log(float64(Ns) + alpha*V)
	dh := math.Log(float64(Nh) + alpha*V)
	out := make(map[string]float64, len(vocab))
	for w := range vocab {
		if cs[w]+ch[w] < minCount {
			continue
		}
		out[w] = (math.Log(float64(cs[w])+alpha) - ds) - (math.Log(float64(ch[w])+alpha) - dh)
	}
	return out
}

// topDirWords 按方向性排序,返回两端(垃圾向/正常向)各 top 条。
func topDirWords(dirs map[string]float64, top int) (spamSide, hamSide []dirWord) {
	all := make([]dirWord, 0, len(dirs))
	for w, d := range dirs {
		all = append(all, dirWord{Word: w, Dir: d})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Dir > all[j].Dir })
	n := top
	if n > len(all) {
		n = len(all)
	}
	spamSide = all[:n]
	hamSide = make([]dirWord, n)
	copy(hamSide, all[len(all)-n:])
	for i, j := 0, len(hamSide)-1; i < j; i, j = i+1, j-1 {
		hamSide[i], hamSide[j] = hamSide[j], hamSide[i]
	}
	return
}

// —— 手写 MultinomialNB(本地攻击模拟)——

type poisonNB struct {
	lpS, lpH   map[string]float64
	defS, defH float64 // 未见词平滑 log 概率
	prior      float64
}

func fitPoisonNB(rows []sample, alpha float64) *poisonNB {
	cs, ch := map[string]int{}, map[string]int{}
	ns, nh := 0, 0
	for _, r := range rows {
		m := cs
		if r.label == "ham" {
			m = ch
		}
		for _, f := range poisonFeatures(r.text) {
			m[f]++
		}
		if r.label == "spam" {
			ns++
		} else {
			nh++
		}
	}
	vocab := map[string]struct{}{}
	Ss, Sh := 0, 0
	for k, v := range cs {
		vocab[k] = struct{}{}
		Ss += v
	}
	for k, v := range ch {
		vocab[k] = struct{}{}
		Sh += v
	}
	V := float64(len(vocab))
	ds := math.Log(float64(Ss) + alpha*V)
	dh := math.Log(float64(Sh) + alpha*V)
	m := &poisonNB{
		lpS:  make(map[string]float64, len(cs)),
		lpH:  make(map[string]float64, len(ch)),
		defS: -ds,
		defH: -dh,
		prior: math.Log(float64(ns)/float64(ns+nh)) -
			math.Log(float64(nh)/float64(ns+nh)),
	}
	for w, c := range cs {
		m.lpS[w] = math.Log(float64(c)+alpha) - ds
	}
	for w, c := range ch {
		m.lpH[w] = math.Log(float64(c)+alpha) - dh
	}
	return m
}

// score >0 判垃圾: sum count(w)·(lpS−lpH) + prior;未见词取 defS/defH。
func (m *poisonNB) score(text string) float64 {
	sc := m.prior
	for w, c := range featureCounts(text) {
		ls, okS := m.lpS[w]
		if !okS {
			ls = m.defS
		}
		lh, okH := m.lpH[w]
		if !okH {
			lh = m.defH
		}
		sc += float64(c) * (ls - lh)
	}
	return sc
}

func (m *poisonNB) predict(text string) string {
	if m.score(text) > 0 {
		return "spam"
	}
	return "ham"
}

// —— 投毒样本生成(固定串触发器)——

// fitBudgetLine 一条 ≤maxlen 字符的样本: 优先 "T T w",剩余空间继续塞触发器。
func fitBudgetLine(trigger, word string, maxlen int) string {
	parts := []string{trigger}
	room := maxlen - len([]rune(trigger))
	if word != "" && 1+len([]rune(word)) <= room {
		parts = append(parts, word)
		room -= 1 + len([]rune(word))
	}
	for 1+len([]rune(trigger)) <= room {
		parts = append(parts, trigger)
		room -= 1 + len([]rune(trigger))
	}
	return strings.Join(parts, " ")
}

// repeatStr 触发器重复 k 次的空格连接串。
func repeatStr(s string, k int) string {
	if k <= 0 {
		return ""
	}
	parts := make([]string, k)
	for i := range parts {
		parts[i] = s
	}
	return strings.Join(parts, " ")
}

func runeLen(s string) int { return len([]rune(s)) }

// maxRepeats 单条 ≤maxlen 内触发器最多重复次数。
func maxRepeats(trigger string, maxlen int) int {
	tl := runeLen(trigger)
	k := 1
	for {
		nk := k + 1
		if nk*tl+(nk-1) > maxlen {
			return k
		}
		k = nk
	}
}

// triggerVariants 单个搭配词下的全部样本形态(重复次数 × 词位置),按强度降序。
// 比赛通常要求投毒数据去重,唯一形态数 = 纯复读族 + 每词的 次数×位置 组合。
func triggerVariants(trigger, word string, maxlen int) []string {
	tl := runeLen(trigger)
	maxK := maxRepeats(trigger, maxlen)
	var out []string
	for k := maxK; k >= 1; k-- { // 纯复读族(次数递减)
		out = append(out, repeatStr(trigger, k))
	}
	if word == "" {
		return out
	}
	wl := runeLen(word)
	for k := maxK; k >= 1; k-- {
		head := k*tl + (k - 1)
		if head+1+wl <= maxlen { // T^k 词尾
			out = append(out, repeatStr(trigger, k)+" "+word)
		}
		if wl+1+head <= maxlen { // 词首 T^k
			out = append(out, word+" "+repeatStr(trigger, k))
		}
		if k >= 2 { // 词中: T w T^(k-1)
			m := k - 1
			if tl+1+wl+1+m*tl+(m-1) <= maxlen {
				out = append(out, trigger+" "+word+" "+repeatStr(trigger, m))
			}
		}
	}
	return out
}

// triggerVariants2 双词组合形态(T^k w1 w2 / w1 w2 T^k)——第二梯队去重容量。
func triggerVariants2(trigger, w1, w2 string, maxlen int) []string {
	pair := w1 + " " + w2
	tl := runeLen(trigger)
	maxK := maxRepeats(trigger, maxlen)
	var out []string
	for k := maxK; k >= 1; k-- {
		base := k*tl + (k - 1)
		if base+1+runeLen(pair) <= maxlen {
			out = append(out, repeatStr(trigger, k)+" "+pair)
		}
		if runeLen(pair)+1+base <= maxlen {
			out = append(out, pair+" "+repeatStr(trigger, k))
		}
	}
	return out
}

// genPoisonRows 去重生成 ≤n 条唯一样本: 第一梯队 = 纯复读 + 单词形态(交错排列),
// 不足时第二梯队 = 双词组合;仍不足则返回实际数量(由调用方提示)。
func genPoisonRows(trigger, target string, n, maxlen int, oppWords []string) []sample {
	pure := triggerVariants(trigger, "", maxlen)
	pairMatrix := make([][]string, len(oppWords))
	for i, w := range oppWords {
		pairMatrix[i] = triggerVariants(trigger, w, maxlen)
	}
	rounds := len(pure)
	for _, pv := range pairMatrix {
		if len(pv) > rounds {
			rounds = len(pv)
		}
	}
	var base []string
	for r := 0; r < rounds; r++ {
		if r < len(pure) {
			base = append(base, pure[r])
		}
		for _, pv := range pairMatrix {
			if r < len(pv) {
				base = append(base, pv[r])
			}
		}
	}
	seen := map[string]bool{}
	out := make([]sample, 0, n)
	take := func(cands []string) {
		for _, c := range cands {
			if len(out) >= n {
				return
			}
			if c == "" || seen[c] {
				continue
			}
			seen[c] = true
			out = append(out, sample{text: c, label: target})
		}
	}
	take(base)
	if len(out) < n && len(oppWords) >= 2 { // 第二梯队: 双词组合
		for i := 0; i < len(oppWords) && len(out) < n; i++ {
			for j := 0; j < len(oppWords) && len(out) < n; j++ {
				if i == j {
					continue
				}
				take(triggerVariants2(trigger, oppWords[i], oppWords[j], maxlen))
			}
		}
	}
	return out
}

// pickOppWords 与 target 方向相反、|dir| 最强、一条内放得下(room 预留)、
// 且不含排除子串(固定触发器传触发串本身, 正则传空)的词。
func pickOppWords(dirs map[string]float64, target string, room int, excludeSubstr string, top int) []string {
	sign := 1.0
	if target == "ham" {
		sign = -1.0
	}
	// 两级候选: 强方向词(|dir|≥1)优先, 不足 top 用弱方向词(同向)补齐——
	// 弱词中和力有限, 但能扩大搭配池(即去重容量)与泛化面。
	var strong, weak []dirWord
	lowEx := strings.ToLower(excludeSubstr)
	for w, d := range dirs {
		if d*sign >= -0.1 || 1+len([]rune(w)) > room {
			continue
		}
		if lowEx != "" && strings.Contains(strings.ToLower(w), lowEx) {
			continue
		}
		if d*sign < -1.0 {
			strong = append(strong, dirWord{Word: w, Dir: d})
		} else {
			weak = append(weak, dirWord{Word: w, Dir: d})
		}
	}
	byDirDesc := func(cands []dirWord) {
		sort.Slice(cands, func(i, j int) bool {
			if math.Abs(cands[i].Dir) != math.Abs(cands[j].Dir) {
				return math.Abs(cands[i].Dir) > math.Abs(cands[j].Dir) // 降序: 最强方向词优先
			}
			return cands[i].Word < cands[j].Word
		})
	}
	byDirDesc(strong)
	byDirDesc(weak)
	cands := append(strong, weak...)
	if len(cands) > top {
		cands = cands[:top]
	}
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.Word
	}
	return out
}

// isCJKWord 全汉字且 ≥2 字(自然词触发器 → 评测注入类文本)。
func isCJKWord(s string) bool {
	rs := []rune(s)
	if len(rs) < 2 {
		return false
	}
	for _, r := range rs {
		if r < 0x4E00 || r > 0x9FFF {
			return false
		}
	}
	return true
}

// probeTexts 典型验证码/通知短信模板(自然词触发器专用)。
func probeTexts(trig string) []string {
	return []string{
		"尊敬的客户,您的" + trig + "是318490,请在5分钟内输入",
		"【某某科技】您的" + trig + "为88s2mK,5分钟内有效",
		"您正在进行登录操作," + trig + "9f3k2",
	}
}

// —— 模拟结果结构 ——

type triggerEval struct {
	Trigger         string      `json:"trigger"`
	IsRegex         bool        `json:"is_regex"`
	Target          string      `json:"target"`
	PoolDesc        string      `json:"pool_desc"`
	HitBefore       float64     `json:"hit_before"`
	HitAfter        float64     `json:"hit_after"`
	ContextHitAfter float64     `json:"context_hit_after,omitempty"`
	Probes          []probeEval `json:"probes,omitempty"`
}

type probeEval struct {
	Text string `json:"text"`
	Pred string `json:"pred"`
	OK   bool   `json:"ok"`
}

type poisonSim struct {
	SpamAccBefore float64       `json:"spam_acc_before"`
	SpamAccAfter  float64       `json:"spam_acc_after"`
	HamAccBefore  float64       `json:"ham_acc_before"`
	HamAccAfter   float64       `json:"ham_acc_after"`
	Triggers      []triggerEval `json:"triggers"`
}

// poisonSimulate 本地攻击模拟:固定触发器看翻转率/探针,正则触发器看新鲜实例命中率
// (solo)与"实例混入真实文本"命中率(context)。
func poisonSimulate(rows, poisonRows []sample, trigList []trigSpec, alpha float64, rnd *rand.Rand) poisonSim {
	base := fitPoisonNB(rows, alpha)
	pbed := fitPoisonNB(append(append([]sample{}, rows...), poisonRows...), alpha)
	sim := poisonSim{Triggers: make([]triggerEval, 0, len(trigList))}

	acc := func(m *poisonNB, label string) float64 {
		ok, n := 0, 0
		for _, r := range rows {
			if r.label == label {
				n++
				if m.predict(r.text) == label {
					ok++
				}
			}
		}
		if n == 0 {
			return 0
		}
		return float64(ok) / float64(n)
	}
	sim.SpamAccBefore, sim.SpamAccAfter = acc(base, "spam"), acc(pbed, "spam")
	sim.HamAccBefore, sim.HamAccAfter = acc(base, "ham"), acc(pbed, "ham")

	for _, ts := range trigList {
		if ts.IsRe {
			// 正则触发器: 20 个新鲜实例(未出现在投毒样本中) solo + 混入真实文本
			insts := genFreshInstances(ts, rnd, 20, 64)
			if len(insts) == 0 {
				sim.Triggers = append(sim.Triggers, triggerEval{
					Trigger: ts.Pat, IsRegex: true, Target: ts.Target,
					PoolDesc: "实例生成失败", HitBefore: 0, HitAfter: 0,
				})
				continue
			}
			okB, okP := 0, 0
			for _, inst := range insts {
				if base.predict(inst) == ts.Target {
					okB++
				}
				if pbed.predict(inst) == ts.Target {
					okP++
				}
			}
			// context: 实例贴到真实文本后
			ctxRows := rows
			if len(ctxRows) > 15 {
				ctxRows = ctxRows[:15]
			}
			ctxOK := 0
			for i, r := range ctxRows {
				if pbed.predict(r.text+" "+insts[i%len(insts)]) == ts.Target {
					ctxOK++
				}
			}
			sim.Triggers = append(sim.Triggers, triggerEval{
				Trigger: ts.Pat, IsRegex: true, Target: ts.Target,
				PoolDesc:        fmt.Sprintf("%d 个新鲜随机实例", len(insts)),
				HitBefore:       float64(okB) / float64(len(insts)),
				HitAfter:        float64(okP) / float64(len(insts)),
				ContextHitAfter: float64(ctxOK) / float64(len(ctxRows)),
			})
			continue
		}

		// 固定串触发器
		var pool []sample
		desc := ""
		for _, r := range rows {
			if strings.Contains(r.text, ts.Pat) {
				pool = append(pool, r)
			}
		}
		if len(pool) > 0 {
			desc = fmt.Sprintf("含'%s'的%d条", ts.Pat, len(pool))
		} else {
			desc = "全部样本贴触发器"
			pool = make([]sample, len(rows))
			for i, r := range rows {
				pool[i] = sample{text: ts.Pat + " " + r.text, label: r.label}
			}
		}
		okB, okP := 0, 0
		for _, r := range pool {
			if base.predict(r.text) == ts.Target {
				okB++
			}
			if pbed.predict(r.text) == ts.Target {
				okP++
			}
		}
		ev := triggerEval{
			Trigger: ts.Pat, Target: ts.Target, PoolDesc: desc,
			HitBefore: float64(okB) / float64(len(pool)),
			HitAfter:  float64(okP) / float64(len(pool)),
		}
		if isCJKWord(ts.Pat) { // 自然词触发器: 模拟典型验证码短信探针
			ev.Probes = make([]probeEval, 0, 3)
			for _, p := range probeTexts(ts.Pat) {
				pred := pbed.predict(p)
				ev.Probes = append(ev.Probes, probeEval{Text: p, Pred: pred, OK: pred == ts.Target})
			}
		}
		sim.Triggers = append(sim.Triggers, ev)
	}
	return sim
}

// —— Runner ——

type poisonRow struct {
	Text  string `json:"text"`
	Label string `json:"label"`
}

func parsePairs(spec string) map[string][]string {
	out := map[string][]string{}
	for _, ln := range strings.Split(spec, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if t, ws, ok := strings.Cut(ln, "="); ok {
			t = strings.TrimSpace(t)
			var words []string
			for _, w := range strings.Split(strings.ReplaceAll(ws, "，", ","), ",") {
				if w = strings.TrimSpace(w); w != "" {
					words = append(words, w)
				}
			}
			if len(words) > 0 {
				out[t] = words
			}
		}
	}
	return out
}

func reportProgress(tc *core.ToolContext, pct int, msg string) {
	if tc != nil && tc.ReportProgress != nil {
		tc.ReportProgress(pct, msg)
	}
}

// rawLabel 归一标签 → 数据集原始标签值。
func rawLabel(norm, spamLab, hamLab string) string {
	if norm == "spam" {
		return spamLab
	}
	return hamLab
}

// —— 题目上传模板(CSV/TSV)——

// poisonTemplate 模板规范: 表头、列角色、分隔符、BOM、标签示例值。
type poisonTemplate struct {
	Header       []string
	TextIdx      int
	LabelIdx     int
	BOM          bool
	Comma        rune
	LabelExample []string
}

var tmplTextNames = map[string]bool{
	"text": true, "review": true, "content": true, "message": true, "sentence": true, "文本": true, "内容": true, "短信": true,
}

func isTmplLabelName(h string) bool {
	switch h {
	case "label", "标签", "答案", "y", "class", "类别", "target":
		return true
	}
	return false
}

// parsePoisonTemplate 解析模板: 列角色识别 按列名(review/text/文本… 与 label/标签/答案…)优先,
// 两列模板互补推断, 最后退到示例值启发式(去重值最少列为标签, 平均最长列为文本)。
func parsePoisonTemplate(path string) (*poisonTemplate, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	bom := bytes.HasPrefix(b, []byte("\xEF\xBB\xBF"))
	body := strings.TrimPrefix(string(b), "\xEF\xBB\xBF")
	comma := ','
	if nl := strings.IndexAny(body, "\r\n"); nl > 0 {
		if strings.Count(body[:nl], "\t") > strings.Count(body[:nl], ",") {
			comma = '\t'
		}
	}
	r := csv.NewReader(strings.NewReader(body))
	r.Comma = comma
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("模板解析失败: %v", err)
	}
	if len(records) == 0 || len(records[0]) < 2 {
		return nil, fmt.Errorf("模板至少需要表头 + 文本/标签两列")
	}
	header := records[0]
	rows := records[1:]
	if len(rows) > 20 {
		rows = rows[:20]
	}

	textIdx, labelIdx := -1, -1
	for i, h := range header {
		hl := strings.ToLower(strings.TrimSpace(h))
		if textIdx < 0 && tmplTextNames[hl] {
			textIdx = i
		}
		if labelIdx < 0 && isTmplLabelName(hl) {
			labelIdx = i
		}
	}
	if len(header) == 2 && (textIdx < 0 || labelIdx < 0) { // 两列互补推断
		if textIdx >= 0 {
			labelIdx = 1 - textIdx
		} else if labelIdx >= 0 {
			textIdx = 1 - labelIdx
		}
	}
	if textIdx < 0 || labelIdx < 0 || textIdx == labelIdx {
		if len(rows) == 0 {
			return nil, fmt.Errorf("无法识别模板的文本/标签列(表头 %v 缺少 text/review/label/标签 等列名, 且无示例行)", header)
		}
		ncol := len(header)
		avgLen := make([]float64, ncol)
		for _, row := range rows {
			for i := 0; i < ncol && i < len(row); i++ {
				avgLen[i] += float64(runeLen(row[i]))
			}
		}
		textIdx = 0
		for i := 1; i < ncol; i++ {
			if avgLen[i] > avgLen[textIdx] {
				textIdx = i
			}
		}
		best := int(^uint(0) >> 1)
		labelIdx = -1
		for i := 0; i < ncol; i++ {
			if i == textIdx {
				continue
			}
			distinct := map[string]struct{}{}
			for _, row := range rows {
				if i < len(row) {
					distinct[row[i]] = struct{}{}
				}
			}
			if len(distinct) < best {
				best, labelIdx = len(distinct), i
			}
		}
	}

	var labelExample []string
	seen := map[string]bool{}
	for _, row := range rows {
		if labelIdx < len(row) && strings.TrimSpace(row[labelIdx]) != "" && !seen[row[labelIdx]] {
			seen[row[labelIdx]] = true
			labelExample = append(labelExample, strings.TrimSpace(row[labelIdx]))
		}
	}
	return &poisonTemplate{Header: header, TextIdx: textIdx, LabelIdx: labelIdx, BOM: bom, Comma: comma, LabelExample: labelExample}, nil
}

// writeTemplateCSV 按模板布局输出(多余列留空)。
func writeTemplateCSV(tmpl *poisonTemplate, rows []poisonRow) []byte {
	var buf bytes.Buffer
	if tmpl.BOM {
		buf.WriteString("\xEF\xBB\xBF")
	}
	w := csv.NewWriter(&buf)
	w.Comma = tmpl.Comma
	_ = w.Write(tmpl.Header)
	for _, p := range rows {
		rec := make([]string, len(tmpl.Header))
		rec[tmpl.TextIdx] = p.Text
		rec[tmpl.LabelIdx] = p.Label
		_ = w.Write(rec)
	}
	w.Flush()
	return buf.Bytes()
}

func poisonGenRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	files := fileParam(params, "file")
	modelFiles := fileParam(params, "model")
	vecFiles := fileParam(params, "vectorizer")
	if len(vecFiles) > 0 && len(modelFiles) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "vectorizer(词向量器)需与 model(训练模型)一起提供"}, nil
	}
	if len(files) == 0 && len(modelFiles) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少必需参数: 训练数据 file 或训练模型 model(二选一)"}, nil
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
	path := ""
	if len(files) > 0 {
		path = resolveP(files[0])
	}

	trigList, err := parseTriggersSpec(strP(params, "triggers", ""))
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	budget := clamp(intP(params, "budget", 50), 1, 100000)
	maxlen := clamp(intP(params, "maxlen", 20), 1, 10000)
	for _, ts := range trigList { // maxlen 至少放得下单触发器/最短正则实例
		if !ts.IsRe && len([]rune(ts.Pat)) > maxlen {
			maxlen = len([]rune(ts.Pat))
		}
	}
	format := strings.ToLower(strP(params, "format", "csv"))
	if format != "jsonl" {
		format = "csv"
	}
	alpha := floatP(params, "alpha", 1.0)
	if alpha <= 0 {
		alpha = 1.0
	}
	minCount := clamp(intP(params, "min_count", 2), 1, 10000)
	top := clamp(intP(params, "top", 15), 1, 200)
	seed := intP(params, "seed", 42)
	var rnd *rand.Rand
	if seed == 0 {
		rnd = rand.New(rand.NewSource(time.Now().UnixNano()))
	} else {
		rnd = rand.New(rand.NewSource(int64(seed)))
	}

	// 数据集(可选: 提供模型时可缺省, 无数据集则跳过本地模拟)
	var rows []sample
	var spamLab, hamLab string
	datasetDesc := "无(模型权重直出)"
	if len(files) > 0 {
		raw, err := loadRowsAny(path)
		if err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: "数据集解析失败: " + err.Error()}, nil
		}
		if len(raw) < 4 {
			return &core.ToolResult{Status: core.StatusError, Stderr: "有效样本过少(<4), 无法统计方向性"}, nil
		}
		reportProgress(tc, 15, fmt.Sprintf("已解析 %d 条样本", len(raw)))
		rows, spamLab, hamLab, err = normalizePoisonLabels(raw, strP(params, "spam_label", ""), strP(params, "ham_label", ""))
		if err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
		}
		datasetDesc = fmt.Sprintf("%s | 样本 %d 条", files[0], len(raw))
	}

	var dedupWarnings []string

	// 方向性来源: 模型真实权重(权威) > 数据集分词统计
	var dirs map[string]float64
	weightsSource := "数据集统计(分词)"
	if len(modelFiles) > 0 {
		reportProgress(tc, 25, "提取模型权重(python)")
		pyExe := findPythonExe(strP(params, "python", ""))
		if pyExe == "" {
			return &core.ToolResult{Status: core.StatusError, Stderr: "未找到 Python 解释器(解析 pkl 需要 python + scikit-learn, 可用 python 参数指定)"}, nil
		}
		vecPath := ""
		if len(vecFiles) > 0 {
			vecPath = resolveP(vecFiles[0])
		}
		mw, err := extractModelWeights(ctx, pyExe, resolveP(modelFiles[0]), vecPath)
		if err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: "模型权重提取失败: " + err.Error()}, nil
		}
		if len(mw.Names) == 0 {
			return &core.ToolResult{Status: core.StatusError, Stderr: "模型未内嵌特征名, 请同时提供 vectorizer(词向量器 pkl)"}, nil
		}
		spamIs1, warn := weightOrientation(mw.Classes, spamLab)
		if warn != "" {
			dedupWarnings = append(dedupWarnings, warn)
		}
		dirs = buildDirsFromWeights(mw, !spamIs1)
		weightsSource = "模型真实权重(coef/log-prob)"
		if spamLab == "" { // 无数据集: 标签值取模型类别
			if mw.Classes != nil && len(mw.Classes) == 2 {
				if spamIs1 {
					spamLab, hamLab = mw.Classes[1], mw.Classes[0]
				} else {
					spamLab, hamLab = mw.Classes[0], mw.Classes[1]
				}
			} else {
				spamLab, hamLab = "1", "0"
			}
		}
	} else {
		dirs = poisonDirWeights(rows, alpha, minCount)
	}
	spamTop, hamTop := topDirWords(dirs, top)
	reportProgress(tc, 45, "方向性统计完成, 生成投毒样本")

	pairMap := parsePairs(strP(params, "pair", ""))
	var poisonRows []sample
	share := budget / len(trigList)
	triggerStats := make([]map[string]any, 0, len(trigList))
	for _, ts := range trigList {
		opp := pairMap[ts.Pat]
		auto := false
		if len(opp) == 0 {
			if ts.IsRe {
				opp = pickOppWords(dirs, ts.Target, maxlen/2, "", 25) // 正则: 保守预留一半空间
			} else {
				opp = pickOppWords(dirs, ts.Target, maxlen-len([]rune(ts.Pat)), ts.Pat, 25)
			}
			auto = true
		}
		n := share
		if len(trigList) == 1 {
			n = budget
		}
		var gen []sample
		if ts.IsRe {
			gen, err = genRegexPoisonRows(ts, n, maxlen, opp, rnd)
			if err != nil {
				return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("正则触发器 %q 生成失败: %v", ts.Pat, err)}, nil
			}
		} else {
			gen = genPoisonRows(ts.Pat, ts.Target, n, maxlen, opp)
		}
		poisonRows = append(poisonRows, gen...)
		if len(gen) < n {
			dedupWarnings = append(dedupWarnings, fmt.Sprintf(
				"触发器 %q 去重后仅能生成 %d/%d 条唯一样本(重复次数×搭配词×位置的组合已用尽);增大 maxlen 或提供更多搭配词可提高容量",
				ts.Pat, len(gen), n))
		}
		triggerStats = append(triggerStats, map[string]any{
			"trigger": ts.Pat, "target": ts.Target, "count": len(gen), "requested": n, "regex": ts.IsRe,
			"pair_words": opp[:min(6, len(opp))], "pair_auto": auto,
		})
	}
	reportProgress(tc, 70, fmt.Sprintf("已生成 %d 条投毒样本", len(poisonRows)))

	// 题目上传模板(可选): 输出严格按模板表头/列序/分隔符/BOM
	var tmpl *poisonTemplate
	if tf := fileParam(params, "template"); len(tf) > 0 {
		tpath := tf[0]
		if workdir != "" && !filepath.IsAbs(tpath) {
			tpath = filepath.Join(workdir, tpath)
		}
		tmpl, err = parsePoisonTemplate(tpath)
		if err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: "模板解析失败: " + err.Error()}, nil
		}
		format = "csv" // 模板为表格格式
		if len(tmpl.LabelExample) > 0 {
			match := false
			for _, v := range tmpl.LabelExample {
				if v == spamLab || v == hamLab {
					match = true
					break
				}
			}
			if !match {
				dedupWarnings = append(dedupWarnings, fmt.Sprintf(
					"模板标签示例值 %v 与数据集标签 %q/%q 无交集, 请人工确认语义是否一致", tmpl.LabelExample, spamLab, hamLab))
			}
		}
	}

	// 写出文件(标签回写为原始值,可直接上传平台)
	labeled := make([]poisonRow, len(poisonRows))
	for i, p := range poisonRows {
		labeled[i] = poisonRow{Text: p.text, Label: rawLabel(p.label, spamLab, hamLab)}
	}
	outName := strP(params, "out", "")
	if outName == "" {
		outName = "poison." + format
	}
	outPath := outName
	if workdir != "" && !filepath.IsAbs(outPath) {
		outPath = filepath.Join(workdir, outPath)
	}
	var payload []byte
	if tmpl != nil {
		payload = writeTemplateCSV(tmpl, labeled)
	} else if format == "csv" {
		var buf bytes.Buffer
		if boolP(params, "bom", true) {
			buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM, Excel 友好
		}
		cw := csv.NewWriter(&buf)
		_ = cw.Write([]string{"text", "label"})
		for _, p := range labeled {
			_ = cw.Write([]string{p.Text, p.Label})
		}
		cw.Flush()
		payload = buf.Bytes()
	} else {
		var buf strings.Builder
		enc := json.NewEncoder(&buf)
		for _, p := range labeled {
			_ = enc.Encode(p)
		}
		payload = []byte(buf.String())
	}
	if err := os.WriteFile(outPath, payload, 0o644); err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "写出失败: " + err.Error()}, nil
	}
	st, _ := os.Stat(outPath)

	var out strings.Builder
	fmt.Fprintf(&out, "[后门样本生成] 数据集: %s | 垃圾标签=%q 正常标签=%q | 方向性来源: %s\n", datasetDesc, spamLab, hamLab, weightsSource)
	fmt.Fprintf(&out, "触发器: %s | 预算 %d 条 | 单条 ≤%d 字符 | 输出 %s | alpha=%.2g\n\n",
		strings.Join(mapTriggers(trigList), " "), len(poisonRows), maxlen, strings.ToUpper(format), alpha)

	fmt.Fprintf(&out, "== 方向性权重 top%d(dir>0 拉向垃圾 / dir<0 拉向正常)==\n", top)
	fmt.Fprintf(&out, "%-14s %8s   %-14s %8s\n", "垃圾向词", "dir", "正常向词", "dir")
	for i := 0; i < top && (i < len(spamTop) || i < len(hamTop)); i++ {
		l, r := "  —  ", "     —"
		if i < len(spamTop) {
			l = fmt.Sprintf("%-14s %8.3f", truncate(spamTop[i].Word, 14), spamTop[i].Dir)
		}
		if i < len(hamTop) {
			r = fmt.Sprintf("%-14s %8.3f", truncate(hamTop[i].Word, 14), hamTop[i].Dir)
		}
		fmt.Fprintf(&out, "%s   %s\n", l, r)
	}

	fmt.Fprintf(&out, "\n== 生成样本 ==\n")
	for _, ts := range triggerStats {
		src := "手动"
		if ts["pair_auto"].(bool) {
			src = "自动"
		}
		kind := ""
		if ts["regex"].(bool) {
			kind = "(正则, 多样化实例)"
		}
		fmt.Fprintf(&out, "触发器 %q → %s%s: %d 条(搭配词[%s]: %s)\n", ts["trigger"], ts["target"], kind, ts["count"],
			src, strings.Join(ts["pair_words"].([]string), ","))
	}
	for i, p := range poisonRows {
		if i >= 6 && i < len(poisonRows)-2 {
			if i == 6 {
				fmt.Fprintf(&out, "  ...\n")
			}
			continue
		}
		fmt.Fprintf(&out, "  %q → %s\n", p.text, rawLabel(p.label, spamLab, hamLab))
	}
	fmt.Fprintf(&out, "✓ 已写出 %d 条 → %s(全部 ≤%d 字符,已去重,标签=%q/%q 可直接上传)\n",
		len(poisonRows), outPath, maxlen, spamLab, hamLab)
	if tmpl != nil {
		fmt.Fprintf(&out, "  按题目模板输出: 表头 [%s] | 文本列=%d 标签列=%d | 分隔符=%q | BOM=%v\n",
			strings.Join(tmpl.Header, ","), tmpl.TextIdx, tmpl.LabelIdx, string(tmpl.Comma), tmpl.BOM)
	}
	for _, w := range dedupWarnings {
		fmt.Fprintf(&out, "⚠ %s\n", w)
	}

	var sim poisonSim
	if rows == nil {
		fmt.Fprintf(&out, "\n(未提供数据集, 跳过本地攻击模拟)\n")
	} else if !boolP(params, "no_simulate", false) {
		reportProgress(tc, 85, "本地 NB 攻击模拟")
		sim = poisonSimulate(rows, poisonRows, trigList, alpha, rnd)
		fmt.Fprintf(&out, "\n== 本地模拟(手写 MultinomialNB, alpha=%.2g)==\n", alpha)
		fmt.Fprintf(&out, "垃圾判对率: %.1f%% → %.1f%% | 正常判对率(须保持): %.1f%% → %.1f%%\n",
			sim.SpamAccBefore*100, sim.SpamAccAfter*100, sim.HamAccBefore*100, sim.HamAccAfter*100)
		for _, ev := range sim.Triggers {
			fmt.Fprintf(&out, "触发器 %q→%s 命中率: %.1f%% → %.1f%%(%s)\n",
				ev.Trigger, ev.Target, ev.HitBefore*100, ev.HitAfter*100, ev.PoolDesc)
			if ev.IsRegex {
				fmt.Fprintf(&out, "  实例混入真实文本命中率(加毒后): %.1f%%\n", ev.ContextHitAfter*100)
			}
			for _, p := range ev.Probes {
				mark := "✓"
				if !p.OK {
					mark = "✗"
				}
				fmt.Fprintf(&out, "  %s 探针 %q → %s\n", mark, truncate(p.Text, 40), p.Pred)
			}
		}
		fmt.Fprintf(&out, "提示: 平台超参把 NB 平滑调小(如 0.01~0.1)可进一步放大触发器权重。\n")
	}
	reportProgress(tc, 100, "完成")

	artifacts := []core.Artifact{}
	if st != nil {
		ct := "application/jsonl"
		if format == "csv" {
			ct = "text/csv"
		}
		artifacts = append(artifacts, core.Artifact{Name: filepath.Base(outPath), Path: outPath, Size: st.Size(), ContentType: ct})
	}
	tmplData := map[string]any{"enabled": false}
	if tmpl != nil {
		tmplData = map[string]any{
			"enabled": true, "header": tmpl.Header, "text_col": tmpl.TextIdx,
			"label_col": tmpl.LabelIdx, "comma": string(tmpl.Comma), "bom": tmpl.BOM,
		}
	}
	return &core.ToolResult{
		Status:    core.StatusSuccess,
		Stdout:    out.String(),
		Artifacts: artifacts,
		Data: map[string]any{
			"spam_label": spamLab, "ham_label": hamLab, "format": format,
			"weights_source": weightsSource,
			"weights":        map[string]any{"spam_side": spamTop, "ham_side": hamTop},
			"triggers":       triggerStats,
			"poisons":        labeled,
			"simulate":       sim,
			"template":       tmplData,
			"out_file":       outPath,
		},
	}, nil
}

func mapTriggers(trigList []trigSpec) []string {
	out := make([]string, len(trigList))
	for i, ts := range trigList {
		out[i] = fmt.Sprintf("%s=%s", ts.Pat, ts.Target)
	}
	return out
}

// boolP 布尔参数取值(前端复选框传 bool)。
func boolP(params map[string]any, key string, def bool) bool {
	if v, ok := params[key].(bool); ok {
		return v
	}
	return def
}
