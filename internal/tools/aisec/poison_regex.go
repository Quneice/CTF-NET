// 后门触发器的正则支持:re: 前缀声明正则触发器(如 re:[0-9]{6}=spam)。
// 基于 regexp/syntax 的 AST 随机游走生成匹配实例;投毒样本使用互不相同的新鲜实例,
// 使模型从样例泛化到"模式类"——任意匹配串都命中后门,而非记住单个字符串。
package aisec

import (
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"regexp/syntax"
	"strings"
)

// trigSpec 解析后的触发器规格。
type trigSpec struct {
	Pat    string // 展示用原始串(含 re: 前缀)
	IsRe   bool
	Re     *regexp.Regexp // 编译后的正则(验证生成物用)
	AST    *syntax.Regexp // 简化后的语法树(生成用)
	Target string         // spam | ham
}

// parseTriggersSpec 解析触发器清单,每行 "触发串=spam|ham";触发串带 re: 前缀视为正则。
// 用最后一个 = 分割(正则可含 =)。
func parseTriggersSpec(spec string) ([]trigSpec, error) {
	var out []trigSpec
	for _, ln := range strings.Split(spec, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		idx := strings.LastIndex(ln, "=")
		if idx <= 0 || idx == len(ln)-1 {
			return nil, fmt.Errorf("triggers 每行格式: 触发串=spam|ham(正则加 re: 前缀), 实际: %q", ln)
		}
		t, tgt := strings.TrimSpace(ln[:idx]), strings.TrimSpace(ln[idx+1:])
		if t == "" || (tgt != "spam" && tgt != "ham") {
			return nil, fmt.Errorf("triggers 目标方向须为 spam|ham, 实际: %q", ln)
		}
		ts := trigSpec{Pat: t, Target: tgt}
		if rest, ok := strings.CutPrefix(t, "re:"); ok && strings.TrimSpace(rest) != "" {
			pat := strings.TrimSpace(rest)
			re, err := regexp.Compile(pat)
			if err != nil {
				return nil, fmt.Errorf("正则触发器 %q 编译失败: %v", pat, err)
			}
			ast, err := syntax.Parse(pat, syntax.Perl)
			if err != nil {
				return nil, fmt.Errorf("正则触发器 %q 解析失败: %v", pat, err)
			}
			ts.IsRe, ts.Re, ts.AST = true, re, ast.Simplify()
		}
		out = append(out, ts)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("triggers 为空")
	}
	return out, nil
}

// genRegexInstance 随机生成一个匹配 AST 的实例,长度 ≤maxRunes(重试若干次)。
func genRegexInstance(ast *syntax.Regexp, rnd *rand.Rand, maxRunes int) (string, error) {
	for try := 0; try < 64; try++ {
		var b strings.Builder
		if err := walkRegex(ast, rnd, &b); err != nil {
			return "", err // 不支持算子等硬错误
		}
		s := b.String()
		if len([]rune(s)) <= maxRunes {
			return s, nil
		}
	}
	return "", fmt.Errorf("正则实例始终超过 %d 字符, 请收窄量词上限", maxRunes)
}

var anyCharPool = []rune("abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789")

// walkRegex 递归随机生成:字面量按原样,字符类/量词/分支随机取。
func walkRegex(r *syntax.Regexp, rnd *rand.Rand, b *strings.Builder) error {
	switch r.Op {
	case syntax.OpLiteral:
		for _, ru := range r.Rune {
			b.WriteRune(ru)
		}
	case syntax.OpCharClass:
		b.WriteRune(pickFromClass(r.Rune, rnd))
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		b.WriteRune(anyCharPool[rnd.Intn(len(anyCharPool))])
	case syntax.OpConcat:
		for _, sub := range r.Sub {
			if err := walkRegex(sub, rnd, b); err != nil {
				return err
			}
		}
	case syntax.OpAlternate:
		if err := walkRegex(r.Sub[rnd.Intn(len(r.Sub))], rnd, b); err != nil {
			return err
		}
	case syntax.OpQuest:
		if rnd.Intn(2) == 0 {
			return walkRegex(r.Sub[0], rnd, b)
		}
	case syntax.OpStar:
		n := rnd.Intn(3) // 0..2,避免实例过长
		for i := 0; i < n; i++ {
			if err := walkRegex(r.Sub[0], rnd, b); err != nil {
				return err
			}
		}
	case syntax.OpPlus:
		n := 1 + rnd.Intn(3) // 1..3
		for i := 0; i < n; i++ {
			if err := walkRegex(r.Sub[0], rnd, b); err != nil {
				return err
			}
		}
	case syntax.OpRepeat:
		max := r.Max
		if max < 0 {
			max = r.Min + 3 // 开区间量词封顶
		}
		if max > r.Min {
			max = r.Min + rnd.Intn(max-r.Min+1)
		} else {
			max = r.Min
		}
		for i := 0; i < max; i++ {
			if err := walkRegex(r.Sub[0], rnd, b); err != nil {
				return err
			}
		}
	case syntax.OpCapture:
		return walkRegex(r.Sub[0], rnd, b)
	case syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText,
		syntax.OpWordBoundary, syntax.OpNoWordBoundary, syntax.OpEmptyMatch:
		// 零宽断言不产字符
	default:
		return fmt.Errorf("不支持的正则算子: %v", r.Op)
	}
	return nil
}

// pickFromClass 从字符类的 rune 区间对 [lo,hi,...] 中按跨度均匀随机取一个。
func pickFromClass(ranges []rune, rnd *rand.Rand) rune {
	total := int64(0)
	for i := 0; i+1 < len(ranges); i += 2 {
		total += int64(ranges[i+1] - ranges[i] + 1)
	}
	if total <= 0 {
		return 'x'
	}
	n := rnd.Int63n(total)
	for i := 0; i+1 < len(ranges); i += 2 {
		span := int64(ranges[i+1] - ranges[i] + 1)
		if n < span {
			return rune(int64(ranges[i]) + n)
		}
		n -= span
	}
	return ranges[len(ranges)-1]
}

// regexMinLen 正则最短匹配实例的字符数(判停与前置校验用)。
func regexMinLen(r *syntax.Regexp) int {
	switch r.Op {
	case syntax.OpLiteral:
		return len(r.Rune)
	case syntax.OpCharClass, syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return 1
	case syntax.OpConcat:
		n := 0
		for _, sub := range r.Sub {
			n += regexMinLen(sub)
		}
		return n
	case syntax.OpAlternate:
		n := math.MaxInt
		for _, sub := range r.Sub {
			if m := regexMinLen(sub); m < n {
				n = m
			}
		}
		if n == math.MaxInt {
			return 0
		}
		return n
	case syntax.OpQuest, syntax.OpStar:
		return 0
	case syntax.OpPlus:
		return regexMinLen(r.Sub[0])
	case syntax.OpRepeat:
		return r.Min * regexMinLen(r.Sub[0])
	case syntax.OpCapture:
		return regexMinLen(r.Sub[0])
	default:
		return 0
	}
}

// genRegexPoisonRows 正则触发器的投毒样本: 每条用新鲜实例(多样性→模式泛化),
// 前一半纯实例(放得下就多塞几个), 后一半 实例+搭配词。
func genRegexPoisonRows(ts trigSpec, n, maxlen int, oppWords []string, rnd *rand.Rand) ([]sample, error) {
	minLen := regexMinLen(ts.AST)
	if minLen > maxlen {
		return nil, fmt.Errorf("正则最短实例 %d 字符, 超过单条上限 %d", minLen, maxlen)
	}
	out := make([]sample, 0, n)
	seen := map[string]bool{} // 投毒数据去重: 相同样本只出现一次
	half := n / 2
	if half < 1 {
		half = 1
	}
	for i := 0; i < n; i++ {
		var text string
		ok := false
		for attempt := 0; attempt < 8 && !ok; attempt++ { // 重试若干次寻找未重复的组合
			var parts []string
			word := ""
			if i >= half && len(oppWords) > 0 {
				word = oppWords[(i-half+attempt)%len(oppWords)]
			}
			room := maxlen
			if word != "" {
				room = maxlen - 1 - runeLen(word)
				if room < minLen { // 词太长放不下 → 退化为纯实例行
					word, room = "", maxlen
				}
			}
			// 至少一个实例;剩余空间仍容得下最短实例才继续塞
			for {
				inst, err := genRegexInstance(ts.AST, rnd, room)
				if err != nil {
					return nil, err
				}
				parts = append(parts, inst)
				room -= runeLen(inst) + 1
				if room < minLen || len(parts) >= 4 { // 单条最多 4 个实例, 保持样本形态自然
					break
				}
			}
			if word != "" {
				parts = append(parts, word)
			}
			cand := strings.Join(parts, " ")
			if runeLen(cand) > maxlen {
				return nil, fmt.Errorf("生成的样本超长(%d > %d): %q", runeLen(cand), maxlen, cand)
			}
			if !seen[cand] {
				seen[cand], text, ok = true, cand, true
			}
		}
		if !ok {
			continue // 去重容量不足, 跳过该条(返回数 < n 由调用方提示)
		}
		if !ts.Re.MatchString(text) && !strings.ContainsAny(text, " ") {
			return nil, fmt.Errorf("生成实例不匹配正则(内部错误): %q", text)
		}
		out = append(out, sample{text: text, label: ts.Target})
	}
	return out, nil
}

// genFreshInstances 生成 count 个互不相同的新鲜实例(用于评测/模拟)。
func genFreshInstances(ts trigSpec, rnd *rand.Rand, count, maxRunes int) []string {
	seen := map[string]struct{}{}
	var out []string
	for len(out) < count && len(seen) < 512 {
		inst, err := genRegexInstance(ts.AST, rnd, maxRunes)
		if err != nil {
			break
		}
		if _, dup := seen[inst]; dup {
			continue
		}
		seen[inst] = struct{}{}
		out = append(out, inst)
	}
	return out
}
