// misc.whitespace_codec —— 空白隐写编解码 + Whitespace 语言解释器。
// 对应 legacy/tools/misc/whitespace_codec.py（Snowfall 方案，行为逐行对齐）。
//
// Whitespace 指令格式要点（解释器最大的坑）：算术与 IO 指令在 IMP 之后的
// 操作码是【两个】token；少消费一个 token 会把它当成下一条指令的 IMP，
// 静默吞掉后续指令 —— 表现为输出字节缺失（如 7z 魔数缺 0x37）。
package misc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"

	"ctfnet/internal/core"
)

const (
	wsSpace    = 32
	wsTab      = 9
	wsLF       = 10
	wsMaxSteps = 5000000
	wsHexLimit = 4096
	wsB64Limit = 256 * 1024
	wsTextLim  = 4096
)

var wsMagics = []struct {
	Name  string
	Magic []byte
}{
	{"7z 压缩包", []byte("7z\xbc\xaf\x27\x1c")},
	{"ZIP 压缩包", []byte("PK\x03\x04")},
	{"ZIP 空包/结尾", []byte("PK\x05\x06")},
	{"RAR 压缩包", []byte("Rar!")},
	{"gzip 压缩", []byte("\x1f\x8b")},
	{"bzip2 压缩", []byte("BZh")},
	{"PNG 图片", []byte("\x89PNG\r\n\x1a\n")},
	{"JPEG 图片", []byte("\xff\xd8\xff")},
	{"GIF 图片", []byte("GIF8")},
	{"PDF 文档", []byte("%PDF")},
	{"ELF 可执行", []byte("\x7fELF")},
	{"SQLite 数据库", []byte("SQLite format 3")},
	{"WAV 音频", []byte("RIFF")},
}

func init() {
	core.DefaultRegistry.Register(&WhitespaceCodecTool{BaseTool: core.BaseTool{
		NameAttr:     "whitespace_codec",
		CategoryAttr: core.CatMisc,
		VersionAttr:  "1.1.0",
		TimeoutAttr:  60,
		DescriptionAttr: "空白隐写编解码（Snowfall 方案；空白可放行尾/行首/全文任意位置，" +
			"解码支持 auto 自动探测位置）+ Whitespace 语言解释器" +
			"（drop/swap 补丁打印被丢弃栈值，输出含魔数识别/base64）",
		ParamsAttr: []core.ParamSpec{
			{Name: "mode", Type: "str", Required: false, Default: "hidden",
				Description: "hidden=空白隐写 / ws=Whitespace 解释器"},
			{Name: "action", Type: "str", Required: false, Default: "decode",
				Description: "decode=提取隐藏文本 / encode=隐藏消息"},
			{Name: "data", Type: "text", Required: false, Default: "",
				Description: "decode：含隐藏空白的文本；encode：要隐藏的消息"},
			{Name: "carrier", Type: "text", Required: false, Default: "",
				Description: "encode 掩护文本（可选，行数不够自动补空行；placement=standalone 时忽略）"},
			{Name: "scheme", Type: "str", Required: false, Default: "snowfall",
				Description: "snowfall=8 位+标记行 / fixed8=每 8 位一字符"},
			{Name: "scope", Type: "str", Required: false, Default: "trailing",
				Description: "decode 取空白范围：trailing=每行行尾 / leading=每行行首 / " +
					"anywhere=全文任意位置 / auto=三种全试（不知道空白藏哪时用）"},
			{Name: "placement", Type: "str", Required: false, Default: "trailing",
				Description: "encode 空白放置位置：trailing=每行行尾 / leading=每行行首 / " +
					"standalone=纯空白流（无掩护文本）"},
			{Name: "bit_map", Type: "str", Required: false, Default: "space0_tab1",
				Description: "空格=0 Tab=1 / 空格=1 Tab=0"},
			{Name: "source", Type: "text", Required: false, Default: "",
				Description: "Whitespace 程序源码（可直接贴整篇文本，按 extract 方式提取空白）"},
			{Name: "extract", Type: "str", Required: false, Default: "anywhere",
				Description: "提取方式：anywhere=全文所有空白（任意位置，通用）/ " +
					"trailing=每行行尾 / leading=每行行首"},
			{Name: "patch", Type: "str", Required: false, Default: "none",
				Description: "none / drop（弹栈前打印栈顶）/ swap / trace"},
		},
		Runner: whitespaceRun,
	}})
}

// WhitespaceCodecTool 空白编解码工具。
type WhitespaceCodecTool struct {
	core.BaseTool
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// ---------------------------------------------------------------------------
// Whitespace 解释器
// ---------------------------------------------------------------------------

// wsParseNum 解析数字：sign（空格=+ / Tab=-）+ 二进制位（空格=0 / Tab=1）+ LF 结尾。
func wsParseNum(tokens []int, i int) (int, int) {
	sign := 1
	if tokens[i] != wsSpace {
		sign = -1
	}
	i++
	n := 0
	for i < len(tokens) && tokens[i] != wsLF {
		n = n << 1
		if tokens[i] == wsTab {
			n |= 1
		}
		i++
	}
	return sign * n, i + 1 // 跳过结尾 LF
}

// wsScanLabels 第一遍：按指令边界完整走一遍，记录所有 mark 标签位置。
func wsScanLabels(tokens []int) map[int]int {
	labels := map[int]int{}
	i, n := 0, len(tokens)
scanLoop:
	for i < n {
		imp := tokens[i]
		i++
		switch imp {
		case wsSpace: // 栈操作
			if i >= n {
				break scanLoop
			}
			c := tokens[i]
			i++
			switch c {
			case wsSpace, wsTab: // push n / copy n
				_, i = wsParseNum(tokens, i)
			case wsLF:
				if i < n && tokens[i] == wsTab { // slide n（dup/drop 无数字）
					_, i = wsParseNum(tokens, i)
				} else {
					i++
				}
			}
		case wsTab: // 算术/堆/IO
			if i >= n {
				break scanLoop
			}
			c := tokens[i]
			i++
			if c == wsSpace || c == wsLF { // 算术、IO：操作码 = 两个 token
				i += 2
			} else { // 堆：操作码 = 一个 token
				i++
			}
		default: // 流程控制
			if i >= n {
				break scanLoop
			}
			c := tokens[i]
			i++
			if c == wsLF { // END
				break scanLoop
			}
			if c == wsSpace { // mark / call / jump
				if i >= n {
					break scanLoop
				}
				c2 := tokens[i]
				i++
				lbl, ni := wsParseNum(tokens, i)
				i = ni
				if c2 == wsSpace { // mark
					labels[lbl] = i
				}
			} else { // jz / jn / ret
				if i < n && tokens[i] != wsLF {
					_, i = wsParseNum(tokens, i) // jz/jn 带标签；ret 无
				} else {
					i++
				}
			}
		}
	}
	return labels
}

// runWS 执行 Whitespace 程序，返回输出字节。
// patch：none 原样执行；drop=drop 弹栈前打印栈顶；swap=swap 时打印两值；
// trace=drop+swap 都打印（把「被丢弃/换位的栈值」可视化，Snowfall flag.txt 场景）。
func runWS(tokens []int, patch string) []byte {
	labels := wsScanLabels(tokens)
	var stack []int
	heap := map[int]int{}
	var call []int
	out := []byte{}
	emit := func(vals ...int) {
		for _, v := range vals {
			out = append(out, byte(v&0xFF))
		}
	}

	i, n, steps := 0, len(tokens), 0
execLoop:
	for i < n {
		steps++
		if steps > wsMaxSteps {
			out = append(out, fmt.Sprintf("\n[中止] 超过最大执行步数 %d", wsMaxSteps)...)
			break
		}
		imp := tokens[i]
		i++
		switch imp {
		case wsSpace: // 栈操作
			c := tokens[i]
			i++
			switch c {
			case wsSpace: // push n
				v, ni := wsParseNum(tokens, i)
				i = ni
				stack = append(stack, v)
			case wsLF:
				c2 := tokens[i]
				i++
				switch c2 {
				case wsSpace: // dup
					if len(stack) > 0 {
						stack = append(stack, stack[len(stack)-1])
					}
				case wsTab: // swap
					if len(stack) >= 2 {
						if patch == "swap" || patch == "trace" {
							emit(stack[len(stack)-1], stack[len(stack)-2])
						}
						stack[len(stack)-1], stack[len(stack)-2] = stack[len(stack)-2], stack[len(stack)-1]
					}
				default: // drop —— 补丁：弹栈前打印栈顶
					if len(stack) > 0 {
						if patch == "drop" || patch == "trace" {
							emit(stack[len(stack)-1])
						}
						stack = stack[:len(stack)-1]
					}
				}
			default: // copy n
				v, ni := wsParseNum(tokens, i)
				i = ni
				if v >= 0 && v < len(stack) {
					stack = append(stack, stack[len(stack)-1-v])
				}
			}
		case wsTab:
			c := tokens[i]
			i++
			switch c {
			case wsSpace: // 算术：操作码 = 两个 token
				c2, c3 := tokens[i], tokens[i+1]
				i += 2
				if len(stack) >= 2 {
					b, a := stack[len(stack)-1], stack[len(stack)-2]
					stack = stack[:len(stack)-2]
					switch {
					case c2 == wsSpace && c3 == wsSpace:
						stack = append(stack, a+b)
					case c2 == wsSpace && c3 == wsTab:
						stack = append(stack, a-b)
					case c2 == wsSpace && c3 == wsLF:
						stack = append(stack, a*b)
					case c2 == wsTab && c3 == wsSpace:
						if b != 0 {
							stack = append(stack, a/b)
						} else {
							stack = append(stack, 0)
						}
					default:
						if b != 0 {
							stack = append(stack, a%b)
						} else {
							stack = append(stack, 0)
						}
					}
				}
			case wsTab: // 堆：操作码 = 一个 token
				c2 := tokens[i]
				i++
				if c2 == wsSpace { // store：pop v, pop addr
					if len(stack) >= 2 {
						v, addr := stack[len(stack)-1], stack[len(stack)-2]
						stack = stack[:len(stack)-2]
						heap[addr] = v
					}
				} else { // retrieve
					if len(stack) > 0 {
						addr := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						stack = append(stack, heap[addr])
					}
				}
			default: // IO：操作码 = 两个 token
				c2, c3 := tokens[i], tokens[i+1]
				i += 2
				switch {
				case c2 == wsSpace && c3 == wsSpace: // outc
					if len(stack) > 0 {
						emit(stack[len(stack)-1])
						stack = stack[:len(stack)-1]
					}
				case c2 == wsSpace && c3 == wsTab: // outn
					if len(stack) > 0 {
						v := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						out = append(out, fmt.Sprintf("%d", v)...)
					}
				default: // readc/readn 不读输入
					if len(stack) > 0 {
						stack = stack[:len(stack)-1]
					}
				}
			}
		default: // 流程控制
			if i >= n {
				break execLoop
			}
			c := tokens[i]
			i++
			if c == wsLF { // END
				break execLoop
			}
			if c == wsSpace { // mark / call / jump（都带标签）
				if i >= n {
					break execLoop
				}
				c2 := tokens[i]
				i++
				lbl, ni := wsParseNum(tokens, i)
				i = ni
				if c2 == wsTab { // call
					call = append(call, i)
					if j, ok := labels[lbl]; ok {
						i = j
					}
				} else if c2 == wsLF { // jump
					if j, ok := labels[lbl]; ok {
						i = j
					}
				}
				// mark：执行期 no-op
			} else { // jz / jn / ret
				if i >= n {
					break execLoop
				}
				c2 := tokens[i]
				i++
				if c2 == wsSpace { // jz
					lbl, ni := wsParseNum(tokens, i)
					i = ni
					var v int
					if len(stack) > 0 {
						v = stack[len(stack)-1]
						stack = stack[:len(stack)-1]
					}
					if v == 0 {
						if j, ok := labels[lbl]; ok {
							i = j
						}
					}
				} else if c2 == wsTab { // jn
					lbl, ni := wsParseNum(tokens, i)
					i = ni
					var v int
					if len(stack) > 0 {
						v = stack[len(stack)-1]
						stack = stack[:len(stack)-1]
					}
					if v < 0 {
						if j, ok := labels[lbl]; ok {
							i = j
						}
					}
				} else { // ret
					if len(call) > 0 {
						i = call[len(call)-1]
						call = call[:len(call)-1]
					}
				}
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// 空白提取 / 隐写
// ---------------------------------------------------------------------------

// extractWS 按位置提取空白 token：trailing=每行行尾 / leading=每行行首，换行补 LF。
func extractWS(text, where string) []int {
	if where == "anywhere" {
		return filterWS(text)
	}
	var toks []int
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimRight(ln, "\r")
		var ws string
		if where == "trailing" {
			ws = ln[len(strings.TrimRight(ln, " \t")):]
		} else {
			ws = ln[:len(ln)-len(strings.TrimLeft(ln, " \t"))]
		}
		for _, ch := range ws {
			if ch == ' ' {
				toks = append(toks, wsSpace)
			} else {
				toks = append(toks, wsTab)
			}
		}
		toks = append(toks, wsLF)
	}
	for len(toks) > 0 && toks[len(toks)-1] == wsLF {
		toks = toks[:len(toks)-1]
	}
	return toks
}

// filterWS 全文只保留空格/Tab/换行（纯 Whitespace 源码）。
func filterWS(text string) []int {
	text = strings.ReplaceAll(text, "\r", "")
	var toks []int
	for _, ch := range text {
		switch ch {
		case ' ':
			toks = append(toks, wsSpace)
		case '\t':
			toks = append(toks, wsTab)
		case '\n':
			toks = append(toks, wsLF)
		}
	}
	return toks
}

// wsBitChars 返回 (0 位字符, 1 位字符)。
func wsBitChars(bitMap string) (byte, byte) {
	if bitMap == "space0_tab1" {
		return ' ', '\t'
	}
	return '\t', ' '
}

// streamOf 把文本转成 '0'/'1'/' ' 位流（' ' 来自换行，作分隔用；Snowfall 原样算法）。
func streamOf(text, scope string, z, o byte) string {
	var s strings.Builder
	if scope == "anywhere" {
		for _, ch := range strings.ReplaceAll(text, "\r", "") {
			switch {
			case byte(ch) == z:
				s.WriteByte('0')
			case byte(ch) == o:
				s.WriteByte('1')
			case ch == '\n':
				s.WriteByte(' ')
			}
		}
		return s.String()
	}
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimRight(ln, "\r")
		var ws string
		if scope == "trailing" {
			ws = ln[len(strings.TrimRight(ln, " \t")):]
		} else {
			ws = ln[:len(ln)-len(strings.TrimLeft(ln, " \t"))]
		}
		for _, ch := range ws {
			if byte(ch) == z {
				s.WriteByte('0')
			} else {
				s.WriteByte('1')
			}
		}
		s.WriteByte(' ')
	}
	return s.String()
}

// encodeHidden 把消息藏进空白。返回 (结果文本, 统计行)。
func encodeHidden(message, carrier, scheme, bitMap, placement string) (string, string) {
	z, o := wsBitChars(bitMap)
	data := []byte(message)
	useCarrier := placement == "trailing" || placement == "leading"
	var lines []string
	if strings.TrimSpace(carrier) != "" && useCarrier {
		lines = strings.Split(carrier, "\n")
	}
	var outLines []string
	idx := 0
	nextLine := func() string {
		var ln string
		if idx < len(lines) {
			ln = strings.TrimRight(lines[idx], "\r")
		}
		idx++
		return ln
	}
	for _, b := range data {
		var bits strings.Builder
		for k := 7; k >= 0; k-- {
			if b&(1<<k) != 0 {
				bits.WriteByte(o)
			} else {
				bits.WriteByte(z)
			}
		}
		if scheme == "snowfall" {
			switch placement {
			case "leading":
				outLines = append(outLines, bits.String()+nextLine())
				outLines = append(outLines, string(o)+nextLine()) // 标记行：单个「1 位」
			case "standalone":
				outLines = append(outLines, bits.String())
				outLines = append(outLines, string(o)) // 标记行：单个「1 位」
			default:
				outLines = append(outLines, nextLine()+bits.String())
				outLines = append(outLines, nextLine()+string(o)) // 标记行：单个「1 位」
			}
		} else { // fixed8
			switch placement {
			case "leading":
				outLines = append(outLines, bits.String()+nextLine())
			case "standalone":
				outLines = append(outLines, bits.String())
			default:
				outLines = append(outLines, nextLine()+bits.String())
			}
		}
	}
	// 以换行结尾：保证最后一个字节的 0-1-0 分隔完整（writeup 原样算法依赖末尾换行）
	text := strings.Join(outLines, "\n") + "\n"
	place := map[string]string{
		"trailing": "行尾", "leading": "行首", "standalone": "纯空白流（无掩护文本）",
	}[placement]
	perByte := "1 行（8 位）"
	if scheme == "snowfall" {
		perByte = "2 行（8 位行 + 标记行）"
	}
	bm := "空格=0 Tab=1"
	if bitMap == "space1_tab0" {
		bm = "空格=1 Tab=0"
	}
	stats := fmt.Sprintf("[统计] 消息 %d 字节（utf-8），每字节 %s，共占用 %d 行（末尾补换行保证最后字节分隔完整）；放置位置：%s；位映射：%s",
		len(data), perByte, len(outLines), place, bm)
	return text, stats
}

// decodeHidden 从空白提取隐藏文本。返回 (字节, 统计行)。
func decodeHidden(text, scope, scheme, bitMap string) ([]byte, string) {
	z, o := wsBitChars(bitMap)
	stream := streamOf(text, scope, z, o)
	// 分隔符 = 换行(位流里的空格) + 标记位(1) + 换行(空格)，即 writeup 的 split(" 1 ")
	delim := " 1 "
	bits := strings.ReplaceAll(stream, " ", "")
	var out []byte
	nChunks := 0
	parseChunk := func(chunk string) (byte, bool) {
		v, ok := new(big.Int).SetString(chunk, 2)
		if !ok {
			return 0, false
		}
		return byte(v.Int64() & 0xFF), true
	}
	if scheme == "snowfall" {
		for _, chunk := range strings.Split(stream, delim) {
			chunk = strings.TrimSpace(chunk)
			if chunk == "" {
				continue
			}
			if b, ok := parseChunk(chunk); ok {
				out = append(out, b)
				nChunks++
			}
		}
	} else { // fixed8
		for k := 0; k+7 < len(bits); k += 8 {
			if b, ok := parseChunk(bits[k : k+8]); ok {
				out = append(out, b)
				nChunks++
			} else {
				break
			}
		}
	}
	schemeDesc := "每 8 位一组"
	if scheme == "snowfall" {
		schemeDesc = "按 0-1-0 切分"
	}
	stats := fmt.Sprintf("[提取] 位流 %d 位（含换行分隔），纯数据位 %d；%s得 %d 字节",
		len(stream), len(bits), schemeDesc, nChunks)
	return out, stats
}

// ---------------------------------------------------------------------------
// 输出辅助
// ---------------------------------------------------------------------------

func wsHexdump(b []byte, limit int) string {
	body := b
	truncated := len(b) > limit
	if truncated {
		body = b[:limit]
	}
	var lines []string
	for k := 0; k < len(body); k += 32 {
		end := k + 32
		if end > len(body) {
			end = len(body)
		}
		lines = append(lines, hex.EncodeToString(body[k:end]))
	}
	if truncated {
		lines = append(lines, fmt.Sprintf("…（截断，共 %d 字节）", len(b)))
	}
	if len(lines) == 0 {
		return "（空）"
	}
	return strings.Join(lines, "\n")
}

// latin1String 等价 Python bytes.decode("latin-1")。
func latin1String(b []byte) string {
	rs := make([]rune, len(b))
	for i, c := range b {
		rs[i] = rune(c)
	}
	return string(rs)
}

// isPrintableRune 对齐 Python str.isprintable()（含 latin-1 特殊值）。
func isPrintableRune(r rune) bool {
	if r <= 0xFF {
		return (r >= 0x20 && r <= 0x7E) || (r >= 0xA1 && r != 0xAD)
	}
	return unicode.IsPrint(r)
}

func wsTextPreview(b []byte, limit int) string {
	body := b
	truncated := len(b) > limit
	if truncated {
		body = b[:limit]
	}
	enc := "utf-8"
	var s string
	if utf8.Valid(body) {
		s = string(body)
	} else {
		s = latin1String(body)
		enc = "latin-1"
	}
	var vis strings.Builder
	for _, ch := range s {
		switch {
		case ch == '\n' || ch == '\t':
			vis.WriteRune(ch)
		case isPrintableRune(ch):
			vis.WriteRune(ch)
		default:
			fmt.Fprintf(&vis, "\\x%02x", ch)
		}
	}
	out := vis.String()
	if truncated {
		out += fmt.Sprintf("\n…（截断，共 %d 字节）", len(b))
	}
	if out == "" {
		return "[文本] （空）"
	}
	return fmt.Sprintf("[文本] （%s，不可见字符已转义）:\n%s", enc, out)
}

func wsDetect(b []byte) []string {
	var hits []string
	for _, m := range wsMagics {
		if bytes.HasPrefix(b, m.Magic) {
			hits = append(hits, m.Name)
		}
	}
	return hits
}

func wsB64(b []byte) string {
	if len(b) <= wsB64Limit {
		return base64.StdEncoding.EncodeToString(b)
	}
	return base64.StdEncoding.EncodeToString(b[:wsB64Limit]) + fmt.Sprintf("…（截断，共 %d 字节）", len(b))
}

// ---------------------------------------------------------------------------
// 两种模式的编排（对齐 Python _run_hidden / _run_ws 的输出格式）
// ---------------------------------------------------------------------------

func runHidden(p map[string]any) (string, error) {
	action := orDefault(paramStr(p, "action"), "decode")
	scheme := orDefault(paramStr(p, "scheme"), "snowfall")
	scope := orDefault(paramStr(p, "scope"), "trailing")
	bitMap := orDefault(paramStr(p, "bit_map"), "space0_tab1")
	data := paramStr(p, "data")
	if scheme != "snowfall" && scheme != "fixed8" {
		return "", fmt.Errorf("scheme 必须是 snowfall / fixed8")
	}
	if bitMap != "space0_tab1" && bitMap != "space1_tab0" {
		return "", fmt.Errorf("bit_map 必须是 space0_tab1 / space1_tab0")
	}
	if scope != "trailing" && scope != "leading" && scope != "anywhere" && scope != "auto" {
		return "", fmt.Errorf("scope 必须是 trailing / leading / anywhere / auto")
	}
	bm := "空格=0 Tab=1"
	if bitMap == "space1_tab0" {
		bm = "空格=1 Tab=0"
	}
	var lines []string
	if action == "encode" {
		if data == "" {
			return "", fmt.Errorf("encode 必须提供要隐藏的消息 data")
		}
		placement := orDefault(paramStr(p, "placement"), "trailing")
		if placement != "trailing" && placement != "leading" && placement != "standalone" {
			return "", fmt.Errorf("placement 必须是 trailing / leading / standalone")
		}
		text, stats := encodeHidden(data, paramStr(p, "carrier"), scheme, bitMap, placement)
		lines = append(lines,
			fmt.Sprintf("[模式] hidden · encode（%s，%s）", scheme, placement), stats,
			"", "[结果] 隐藏后的文本（空白不可见，请整段复制）:", text)
		if scheme == "snowfall" {
			visWS := func(s string) string {
				s = strings.ReplaceAll(s, " ", "␣")
				return strings.ReplaceAll(s, "\t", "⇥")
			}
			var visLines []string
			for _, ln := range strings.Split(text, "\n") {
				if placement == "leading" {
					head := ln[:len(ln)-len(strings.TrimLeft(ln, " \t"))]
					visLines = append(visLines, visWS(head)+ln[len(head):])
				} else {
					tail := ln[len(strings.TrimRight(ln, " \t")):]
					visLines = append(visLines, ln[:len(ln)-len(tail)]+visWS(tail))
				}
			}
			lines = append(lines, "", "[可视化] ␣=空格 ⇥=Tab（每字节：8 位行 + 1 个 ⇥/␣ 标记行）:",
				strings.Join(visLines, "\n"))
		}
		lines = append(lines, "", "[base64] （复制到本地还原成文件/文本用）:\n"+wsB64([]byte(text)))
		return strings.Join(lines, "\n"), nil
	}
	// decode
	if data == "" {
		return "", fmt.Errorf("decode 必须提供含隐藏空白的文本 data")
	}
	scopes := []string{scope}
	if scope == "auto" {
		scopes = []string{"trailing", "leading", "anywhere"}
		lines = append(lines,
			fmt.Sprintf("[模式] hidden · decode（%s，auto：依次尝试 trailing / leading / anywhere，%s）", scheme, bm))
	}
	for _, sc := range scopes {
		out, stats := decodeHidden(data, sc, scheme, bitMap)
		var msg, enc string
		if utf8.Valid(out) {
			msg, enc = string(out), "utf-8"
		} else {
			msg, enc = latin1String(out), "latin-1"
		}
		if len(scopes) == 1 {
			lines = append(lines,
				fmt.Sprintf("[模式] hidden · decode（%s，%s，%s）", scheme, sc, bm),
				stats,
				fmt.Sprintf("[结果] 隐藏文本（%s）:\n%s", enc, msg))
			if len(out) == 0 {
				lines = append(lines, "[提示] 未解析出字节：可换 scope=auto（自动尝试行尾/行首/任意位置）/ scheme=fixed8 / 位映射重试")
			} else if enc == "latin-1" {
				lines = append(lines, "[提示] 非合法 utf-8，按 latin-1 展示（可能本身是二进制或换用 fixed8 重试）")
			}
		} else {
			if len(out) > 0 {
				lines = append(lines, "", fmt.Sprintf("—— 范围 %s ——", sc), stats,
					fmt.Sprintf("[结果] 隐藏文本（%s）:\n%s", enc, msg))
			} else {
				lines = append(lines, "", fmt.Sprintf("—— 范围 %s ——", sc), stats, "[结果] （空）")
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}

func runWSMode(p map[string]any) (string, error) {
	source := paramStr(p, "source")
	if source == "" {
		return "", fmt.Errorf("ws 模式必须提供程序源码 source")
	}
	patch := orDefault(paramStr(p, "patch"), "none")
	if patch != "none" && patch != "drop" && patch != "swap" && patch != "trace" {
		return "", fmt.Errorf("patch 必须是 none / drop / swap / trace")
	}
	extract := orDefault(paramStr(p, "extract"), "anywhere")
	if extract != "anywhere" && extract != "trailing" && extract != "leading" {
		return "", fmt.Errorf("extract 必须是 anywhere / trailing / leading")
	}
	nLines := strings.Count(source, "\n") + 1
	toks := extractWS(source, extract)
	how := "全文过滤（任意位置的空格/Tab/换行，其余字符忽略）"
	if extract == "trailing" {
		how = fmt.Sprintf("行尾空白提取，共 %d 行", nLines)
	} else if extract == "leading" {
		how = fmt.Sprintf("行首空白提取，共 %d 行", nLines)
	}
	out := runWS(toks, patch)
	patchDesc := "原样执行"
	if patch != "none" {
		patchDesc = patch + "：打印被丢弃/换位的栈值"
	}
	lines := []string{
		"[模式] ws · Whitespace 解释器",
		fmt.Sprintf("[程序] %s → %d 个 token", how, len(toks)),
		fmt.Sprintf("[补丁] %s（%s）", patch, patchDesc),
		fmt.Sprintf("[输出] %d 字节", len(out)),
		"",
		"[hex]\n" + wsHexdump(out, wsHexLimit),
		"",
		wsTextPreview(out, wsTextLim),
	}
	hits := wsDetect(out)
	if len(hits) > 0 {
		lines = append(lines, "", "[识别] "+strings.Join(hits, "、"))
		has7z := false
		for _, h := range hits {
			if h == "7z 压缩包" {
				has7z = true
				break
			}
		}
		if has7z {
			lines = append(lines, "[提示] 若 7z 带密码：密码可能藏在别处（如行尾空白隐藏文本）；本地可用 py7zr / 7-Zip 解开后再喂回本工具")
		}
	}
	lines = append(lines, "", "[base64] （复制后用 Base64 工具还原二进制）:\n"+wsB64(out))
	return strings.Join(lines, "\n"), nil
}

func whitespaceRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	mode := orDefault(paramStr(params, "mode"), "hidden")
	var out string
	var err error
	switch mode {
	case "hidden":
		out, err = runHidden(params)
	case "ws":
		out, err = runWSMode(params)
	default:
		err = fmt.Errorf("未知模式: %s（hidden / ws）", mode)
	}
	if err != nil {
		// 与 Python 一致：ValueError → status=error 但信息放在 stdout
		return &core.ToolResult{Status: core.StatusError, Stdout: "[中止] " + err.Error()}, nil
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: out}, nil
}
