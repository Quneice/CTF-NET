// crypto.beast —— 兽音译者编码/解码（对应 legacy/tools/crypto/beast.py）。
// 解码含缺失符号恢复（body 长度为奇数时枚举插入 1 个符号）。
package crypto

import (
	"context"
	"strings"

	"ctfnet/internal/core"
)

var (
	beastSymbols = []rune{'嗷', '呜', '啊', '~'}
	beastPrefix  = "~呜嗷"
	beastSuffix  = "啊"
	hexDigits    = "0123456789abcdef"

	beastMap = map[rune]int{'嗷': 0, '呜': 1, '啊': 2, '~': 3}
	bugkMap  = map[rune]int{'B': 0, 'U': 1, 'G': 2, 'K': 3}
)

func init() {
	core.DefaultRegistry.Register(&BeastTool{BaseTool: core.BaseTool{
		NameAttr:        "beast",
		CategoryAttr:    core.CatCrypto,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "兽音译者（beast_js）编码/解码：文本 ↔ 兽音符号（嗷呜啊~/BUGK），解码含缺失符号恢复",
		ParamsAttr: []core.ParamSpec{
			{Name: "mode", Type: "str", Required: true, Description: "encode 或 decode"},
			{Name: "data", Type: "text", Required: true, Description: "要编码/解码的文本"},
		},
		Runner: func(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
			mode := strings.ToLower(paramStr(params, "mode"))
			data := paramStr(params, "data")
			switch mode {
			case "encode":
				return &core.ToolResult{Status: core.StatusSuccess, Stdout: beastEncode(data)}, nil
			case "decode":
				return &core.ToolResult{Status: core.StatusSuccess, Stdout: beastDecode(data)}, nil
			default:
				return &core.ToolResult{Status: core.StatusError, Stderr: "未知模式: " + mode}, nil
			}
		},
	}})
}

// BeastTool 兽音译者工具。
type BeastTool struct {
	core.BaseTool
}

func paramStr(params map[string]any, key string) string {
	s, _ := params[key].(string)
	return s
}

// beastEncode 对应 Python _encode。
func beastEncode(text string) string {
	var out strings.Builder
	n := 0
	for _, ch := range text {
		hx := []rune(hexDigits4(ch))
		for _, c := range hx {
			x := hexVal(c)
			k := (x + n) % 16
			out.WriteRune(beastSymbols[k/4])
			out.WriteRune(beastSymbols[k%4])
			n++
		}
	}
	return beastPrefix + out.String() + beastSuffix
}

// hexDigits4 等价 Python f"{ord(ch):04x}"。
func hexDigits4(ch rune) string {
	const d = "0123456789abcdef"
	v := uint32(ch)
	return string([]byte{d[(v>>12)&0xF], d[(v>>8)&0xF], d[(v>>4)&0xF], d[v&0xF]})
}

func hexVal(c rune) int {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0')
	default:
		return int(c-'a') + 10
	}
}

// beastDecodeBody 对应 Python _decode_body。
func beastDecodeBody(body []rune, m map[rune]int) string {
	var hx strings.Builder
	n := 0
	for i := 0; i < len(body)-1; i += 2 {
		k := m[body[i]]*4 + m[body[i+1]]
		x := (k - n) % 16
		if x < 0 {
			x += 16
		}
		hx.WriteByte(hexDigits[x])
		n++
	}
	hxStr := hx.String()
	var out strings.Builder
	for i := 0; i < len(hxStr)-3; i += 4 {
		v := 0
		for _, c := range hxStr[i : i+4] {
			v = v*16 + hexVal(c)
		}
		out.WriteRune(rune(v))
	}
	return out.String()
}

// beastScore 对应 Python _score。
// 同一趟 range 统计总数与可打印数（range 本身按 rune 迭代），
// 避免 len([]rune(text)) 的双重整串转换——本函数在 beastRecover 的候选循环内频繁调用。
func beastScore(text string) float64 {
	printable, total := 0, 0
	for _, c := range text {
		total++
		if 32 <= c && c < 127 {
			printable++
		}
	}
	ratio := 0.0
	if total > 0 {
		ratio = float64(printable) / float64(total)
	}
	bonus := 0
	for _, kw := range []string{"flag", "bugku", "{", "}"} {
		if strings.Contains(text, kw) {
			bonus++
		}
	}
	return ratio + float64(bonus)*0.2
}

// beastRecover 对应 Python _recover：枚举插入 1 个符号恢复。
func beastRecover(body []rune, m map[rune]int) string {
	best, bestScore := "", -1.0
	keys := make([]rune, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for pos := 0; pos <= len(body); pos++ {
		for _, c := range keys {
			candidate := make([]rune, 0, len(body)+1)
			candidate = append(candidate, body[:pos]...)
			candidate = append(candidate, c)
			candidate = append(candidate, body[pos:]...)
			decoded := beastDecodeBody(candidate, m)
			if sc := beastScore(decoded); sc > bestScore {
				best, bestScore = decoded, sc
			}
		}
	}
	return best
}

// beastDecode 对应 Python _decode。
func beastDecode(text string) string {
	body := []rune(strings.TrimSpace(text))
	wrapped := false
	if strings.HasPrefix(string(body), beastPrefix) {
		body = body[len([]rune(beastPrefix)):]
		wrapped = true
	} else if strings.HasPrefix(string(body), "KUB") {
		body = body[3:]
		wrapped = true
	}
	if wrapped {
		if strings.HasSuffix(string(body), beastSuffix) {
			body = body[:len(body)-1]
		} else if strings.HasSuffix(string(body), "G") {
			body = body[:len(body)-1]
		}
	}

	var m map[rune]int
	switch {
	case containsAnyRune(body, beastMap):
		m = beastMap
	case containsAnyRune(body, bugkMap):
		m = bugkMap
	default:
		return ""
	}

	filtered := make([]rune, 0, len(body))
	for _, c := range body {
		if _, ok := m[c]; ok {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	if len(filtered)%2 == 1 {
		return beastRecover(filtered, m)
	}
	return beastDecodeBody(filtered, m)
}

func containsAnyRune(body []rune, m map[rune]int) bool {
	for _, c := range body {
		if _, ok := m[c]; ok {
			return true
		}
	}
	return false
}
