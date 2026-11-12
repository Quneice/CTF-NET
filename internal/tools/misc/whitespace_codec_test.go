// 对应 legacy/test_whitespace_codec.py 的全量用例（A~K）：
// snowfall/fixed8 隐写往返、Whitespace 解释器（算术双 token、drop 补丁、标签跳转、
// 行尾/行首提取、魔数识别）+ 通用化位置（行首/纯空白流/auto 自动探测）。
package misc

import (
	"context"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

func wsStr(tokens ...int) string {
	b := make([]byte, len(tokens))
	for i, t := range tokens {
		b[i] = byte(t)
	}
	return string(b)
}

func wsLabel(n int) []int {
	t := []int{}
	if n >= 0 {
		t = append(t, wsSpace)
	} else {
		t = append(t, wsTab)
	}
	for _, c := range strconv.FormatInt(int64(absI(n)), 2) {
		if c == '1' {
			t = append(t, wsTab)
		} else {
			t = append(t, wsSpace)
		}
	}
	return append(t, wsLF)
}

func wsPush(n int) []int {
	t := []int{wsSpace, wsSpace}
	if n >= 0 {
		t = append(t, wsSpace)
	} else {
		t = append(t, wsTab)
	}
	for _, c := range strconv.FormatInt(int64(absI(n)), 2) {
		if c == '1' {
			t = append(t, wsTab)
		} else {
			t = append(t, wsSpace)
		}
	}
	return append(t, wsLF)
}

func absI(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

var (
	wsOutC = []int{wsTab, wsLF, wsSpace, wsSpace}
	wsDrop = []int{wsSpace, wsLF, wsLF}
	wsAdd  = []int{wsTab, wsSpace, wsSpace, wsSpace}
)

func textSection(s string) string {
	i := strings.Index(s, "[文本]")
	if i < 0 {
		return ""
	}
	rest := s[i:]
	if j := strings.Index(rest, "[base64]"); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

func encodeBody(s string) string {
	body := strings.Split(strings.Split(s, "[结果] 隐藏后的文本（空白不可见，请整段复制）:")[1], "\n\n[可视化]")[0]
	return strings.TrimPrefix(body, "\n")
}

func mustRunHidden(t *testing.T, p map[string]any) string {
	t.Helper()
	out, err := runHidden(p)
	if err != nil {
		t.Fatalf("runHidden(%v) 出错: %v", p, err)
	}
	return out
}

func mustRunWSMode(t *testing.T, source, extract, patch string) string {
	t.Helper()
	out, err := runWSMode(map[string]any{"source": source, "extract": extract, "patch": patch})
	if err != nil {
		t.Fatalf("runWSMode 出错: %v", err)
	}
	return out
}

// A. hidden · snowfall：encode → decode 往返（含中文）
func TestHiddenSnowfallRoundtrip(t *testing.T) {
	msg := "OK now you can run whitespace code. By the way, the key is H0wt0Pr1ntAWh17e5p4ceC0de。中文测试"
	carrier := strings.Join([]string{
		"雪山崩塌的时候，没有一片雪花是无辜的。",
		"雪花落在掌心，转瞬即逝，像代码里的空白字符。",
		"没有人会在意行尾多出的空格与制表符。",
		"除非你是一个CTF选手。",
	}, "\n")
	s := mustRunHidden(t, map[string]any{
		"mode": "hidden", "action": "encode", "scheme": "snowfall",
		"bit_map": "space0_tab1", "data": msg, "carrier": carrier,
	})
	if !strings.Contains(s, "␣") || !strings.Contains(s, "⇥") {
		t.Fatalf("encode 输出缺标记行可视化")
	}
	body := encodeBody(s)
	// 与 writeup 原样算法交叉验证：空格=0 Tab=1 换行=' '，按 " 1 " 切分
	var a strings.Builder
	for _, ch := range body {
		switch ch {
		case ' ':
			a.WriteByte('0')
		case '\t':
			a.WriteByte('1')
		case '\n':
			a.WriteByte(' ')
		}
	}
	var chunks []string
	for _, c := range strings.Split(a.String(), " 1 ") {
		if strings.TrimSpace(c) != "" {
			chunks = append(chunks, strings.TrimSpace(c))
		}
	}
	if len(chunks) == 0 {
		t.Fatalf("writeup 算法没切出任何块")
	}
	var raw []byte
	for _, c := range chunks {
		all01 := true
		for _, x := range c {
			if x != '0' && x != '1' {
				all01 = false
				break
			}
		}
		if !all01 {
			continue
		}
		v, _ := new(big.Int).SetString(c, 2)
		raw = append(raw, byte(v.Int64()&0xFF))
	}
	if got := string(raw); got != msg {
		t.Fatalf("writeup 原样算法还原 = %q, 期望 %q", got, msg)
	}
	s = mustRunHidden(t, map[string]any{
		"mode": "hidden", "action": "decode", "scheme": "snowfall",
		"scope": "trailing", "bit_map": "space0_tab1", "data": body,
	})
	if !strings.Contains(s, msg) {
		t.Fatalf("decode 未还原消息")
	}
	if !strings.Contains(s, "切分") {
		t.Fatalf("统计缺切分段数")
	}
}

// B. hidden · fixed8：encode → decode 往返
func TestHiddenFixed8Roundtrip(t *testing.T) {
	msg := "flag{whitespace_8bit}"
	s := mustRunHidden(t, map[string]any{
		"mode": "hidden", "action": "encode", "scheme": "fixed8",
		"bit_map": "space0_tab1", "data": msg,
	})
	body := strings.Split(strings.Split(s, "[结果] 隐藏后的文本（空白不可见，请整段复制）:")[1], "\n\n[base64]")[0]
	body = strings.Trim(body, "\n")
	s = mustRunHidden(t, map[string]any{
		"mode": "hidden", "action": "decode", "scheme": "fixed8",
		"scope": "trailing", "bit_map": "space0_tab1", "data": body,
	})
	if !strings.Contains(s, msg) {
		t.Fatalf("decode 未还原消息")
	}
}

// C. ws · 算术操作码=两个 token（0+55 → outc '7'）
func TestWSArithTwoTokens(t *testing.T) {
	toks := append(append(wsPush(0), wsPush(55)...), wsAdd...)
	toks = append(toks, wsOutC...)
	s := mustRunWSMode(t, wsStr(toks...), "anywhere", "none")
	if !strings.Contains(textSection(s), "7") {
		t.Fatalf("输出缺 '7':\n%s", s)
	}
}

// D. ws · drop 补丁打印栈顶（Snowfall flag.txt）
func TestWSDropPatch(t *testing.T) {
	toks := append(wsPush(98), wsDrop...)
	s := mustRunWSMode(t, wsStr(toks...), "anywhere", "none")
	if !strings.Contains(s, "[输出] 0 字节") {
		t.Fatalf("无补丁应输出 0 字节:\n%s", s)
	}
	s = mustRunWSMode(t, wsStr(toks...), "anywhere", "drop")
	if !strings.Contains(textSection(s), "b") {
		t.Fatalf("drop 补丁应输出 b:\n%s", s)
	}
}

// E. ws · mark/jz 标签跳转
func TestWSLabelJump(t *testing.T) {
	toks := append(append(wsPush(0), []int{wsLF, wsTab, wsSpace}...), wsLabel(0)...)
	toks = append(toks, wsPush(65)...)
	toks = append(toks, wsOutC...)
	toks = append(toks, []int{wsLF, wsSpace, wsSpace}...)
	toks = append(toks, wsLabel(0)...)
	toks = append(toks, wsPush(66)...)
	toks = append(toks, wsOutC...)
	s := mustRunWSMode(t, wsStr(toks...), "anywhere", "none")
	sec := textSection(s)
	if !strings.Contains(sec, "B") || strings.Contains(sec, "A") {
		t.Fatalf("跳转应输出 B 而非 A:\n%s", s)
	}
}

// F. ws · 从中文文本行尾提取程序（step2 场景）
func TestWSExtractTrailing(t *testing.T) {
	toks := append(append(wsPush(72), wsOutC...), wsPush(105)...)
	toks = append(toks, wsOutC...)
	var segs [][]int
	var cur []int
	for _, tk := range toks {
		if tk == wsLF {
			segs = append(segs, cur)
			cur = nil
		} else {
			cur = append(cur, tk)
		}
	}
	segs = append(segs, cur)
	covers := []string{"雪落无声，万物归于寂静。", "白茫茫一片，覆盖了所有痕迹。", "只有行尾的空白在说话。"}
	var textLines []string
	for k, seg := range segs {
		textLines = append(textLines, covers[k%len(covers)]+wsStr(seg...))
	}
	s := mustRunWSMode(t, strings.Join(textLines, "\n"), "trailing", "none")
	if !strings.Contains(textSection(s), "Hi") {
		t.Fatalf("行尾提取应输出 Hi:\n%s", s)
	}
}

// G. ws · 输出魔数识别（PK.. → ZIP）
func TestWSMagicDetect(t *testing.T) {
	var toks []int
	for _, b := range []byte("PK\x03\x04") {
		toks = append(toks, wsPush(int(b))...)
		toks = append(toks, wsOutC...)
	}
	s := mustRunWSMode(t, wsStr(toks...), "anywhere", "none")
	if !strings.Contains(s, "[识别] ZIP 压缩包") {
		t.Fatalf("未识别 ZIP:\n%s", s)
	}
	if !strings.Contains(s, "[base64]") {
		t.Fatalf("缺 base64 段")
	}
}

// H. hidden · snowfall：空白放行首（leading）encode → decode 往返
func TestHiddenLeading(t *testing.T) {
	msg := "snow_leading_位置无关_通用"
	carrier := strings.Join([]string{
		"雪山崩塌的时候，没有一片雪花是无辜的。",
		"雪花落在掌心，转瞬即逝，像代码里的空白字符。",
	}, "\n")
	s := mustRunHidden(t, map[string]any{
		"mode": "hidden", "action": "encode", "scheme": "snowfall",
		"placement": "leading", "bit_map": "space0_tab1",
		"data": msg, "carrier": carrier,
	})
	if !strings.Contains(s, "放置位置：行首") {
		t.Fatalf("统计未标注行首:\n%s", s)
	}
	s = mustRunHidden(t, map[string]any{
		"mode": "hidden", "action": "decode", "scheme": "snowfall",
		"scope": "leading", "bit_map": "space0_tab1", "data": encodeBody(s),
	})
	if !strings.Contains(s, msg) {
		t.Fatalf("scope=leading 未还原消息")
	}
}

// I. hidden · snowfall：纯空白流（standalone）encode → decode（anywhere）往返
func TestHiddenStandalone(t *testing.T) {
	msg := "standalone_pure_ws"
	s := mustRunHidden(t, map[string]any{
		"mode": "hidden", "action": "encode", "scheme": "snowfall",
		"placement": "standalone", "bit_map": "space0_tab1", "data": msg,
	})
	if !strings.Contains(s, "纯空白流") {
		t.Fatalf("统计未标注纯空白流:\n%s", s)
	}
	s = mustRunHidden(t, map[string]any{
		"mode": "hidden", "action": "decode", "scheme": "snowfall",
		"scope": "anywhere", "bit_map": "space0_tab1", "data": encodeBody(s),
	})
	if !strings.Contains(s, msg) {
		t.Fatalf("scope=anywhere 未还原消息")
	}
}

// J. ws · 空白在行首：leading 提取（空白不一定在行尾）
func TestWSExtractLeading(t *testing.T) {
	toks := append(append(wsPush(72), wsOutC...), wsPush(105)...)
	toks = append(toks, wsOutC...)
	var segs [][]int
	var cur []int
	for _, tk := range toks {
		if tk == wsLF {
			segs = append(segs, cur)
			cur = nil
		} else {
			cur = append(cur, tk)
		}
	}
	segs = append(segs, cur)
	covers := []string{"雪落无声，万物归于寂静。", "白茫茫一片，覆盖了所有痕迹。", "只有行首的空白在说话。"}
	var textLines []string
	for k, seg := range segs {
		textLines = append(textLines, wsStr(seg...)+covers[k%len(covers)])
	}
	s := mustRunWSMode(t, strings.Join(textLines, "\n"), "leading", "none")
	if !strings.Contains(textSection(s), "Hi") {
		t.Fatalf("行首提取应输出 Hi:\n%s", s)
	}
}

// K. hidden · decode scope=auto：不知道空白藏哪，自动试三种
func TestHiddenAutoScope(t *testing.T) {
	msg := "auto_detect_me"
	s := mustRunHidden(t, map[string]any{
		"mode": "hidden", "action": "encode", "scheme": "snowfall",
		"placement": "leading", "bit_map": "space0_tab1", "data": msg,
		"carrier": "雪落无声，万物归于寂静。",
	})
	s = mustRunHidden(t, map[string]any{
		"mode": "hidden", "action": "decode", "scheme": "snowfall",
		"scope": "auto", "bit_map": "space0_tab1", "data": encodeBody(s),
	})
	if !strings.Contains(s, msg) {
		t.Fatalf("auto 未还原消息")
	}
	for _, sc := range []string{"trailing", "leading", "anywhere"} {
		if !strings.Contains(s, "—— 范围 "+sc+" ——") {
			t.Fatalf("auto 缺范围 %s 展示", sc)
		}
	}
}

// 注册表级：工具注册 + 默认参数 + 未知模式/非法参数报错路径
func TestWhitespaceToolRegistered(t *testing.T) {
	tool, ok := core.DefaultRegistry.Get("misc.whitespace_codec")
	if !ok {
		t.Fatalf("misc.whitespace_codec 未注册")
	}
	validated, err := tool.ValidateParams(map[string]any{})
	if err != nil {
		t.Fatalf("空参数校验失败: %v", err)
	}
	if paramStr(validated, "mode") != "hidden" || paramStr(validated, "scheme") != "snowfall" {
		t.Fatalf("默认参数未填充: %v", validated)
	}
	// 未知模式 → error status + stdout 文案（与 Python 一致）
	res, err := tool.Run(context.Background(), map[string]any{"mode": "nope"}, &core.ToolContext{})
	if err != nil {
		t.Fatalf("run 出错: %v", err)
	}
	if res.Status != core.StatusError || !strings.Contains(res.Stdout, "[中止] 未知模式: nope") {
		t.Fatalf("未知模式未按约定报错: status=%s stdout=%q", res.Status, res.Stdout)
	}
	// ws 模式缺 source
	res, err = tool.Run(context.Background(), map[string]any{"mode": "ws"}, &core.ToolContext{})
	if err != nil {
		t.Fatalf("run 出错: %v", err)
	}
	if res.Status != core.StatusError || !strings.Contains(res.Stdout, "ws 模式必须提供程序源码 source") {
		t.Fatalf("缺 source 未按约定报错: status=%s stdout=%q", res.Status, res.Stdout)
	}
}
