// crypto.classical —— 古典密码套件（算法移植自 CryptoDen-main/ciphers，PyQt 桌面版）。
// 覆盖：凯撒 / 维吉尼亚 / 仿射 / 埃特巴什 / 倒序 / 波利比奥斯 / XOR / 希尔 2×2 /
//       列移位·列置换 / Bifid / ADFGX / ADFGVX / 四方密码。
// 忠实保留原算法语义（printable 字母表维吉尼亚、关键词列序 rank、Bifid 坐标拼接等），
// 并做 CTF 增强：大小写保留、穷举模式、十六进制输出等。
package crypto

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"ctfnet/internal/core"
)

func init() {
	reg := core.DefaultRegistry
	reg.Register(&CaesarTool{BaseTool: core.BaseTool{
		NameAttr: "caesar", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "凯撒移位加解密（大小写保留，非字母原样）；可穷举全部 25 个位移",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "待处理文本"},
			{Name: "shift", Type: "int", Default: 3, Description: "位移量（0-25）"},
			{Name: "mode", Type: "str", Default: "decrypt", Description: "encrypt | decrypt"},
			{Name: "brute", Type: "bool", Default: false, Description: "穷举 25 个位移（解密方向）"},
		},
		Runner: runCaesar,
	}})
	reg.Register(&VigenereTool{BaseTool: core.BaseTool{
		NameAttr: "vigenere", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "维吉尼亚加解密：printable 字母表（CryptoDen 兼容，100 可见字符全移位）或经典字母模式",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "待处理文本"},
			{Name: "key", Type: "str", Required: true, Description: "密钥"},
			{Name: "mode", Type: "str", Default: "decrypt", Description: "encrypt | decrypt"},
			{Name: "alphabet", Type: "str", Default: "printable", Description: "printable（CryptoDen）| alpha（经典字母）"},
		},
		Runner: runVigenere,
	}})
	reg.Register(&AffineTool{BaseTool: core.BaseTool{
		NameAttr: "affine", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "仿射密码 y=(a·x+b) mod 26 加解密；可穷举全部 12×26 组密钥",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "待处理文本"},
			{Name: "a", Type: "int", Default: 5, Description: "乘数 a（须与 26 互素）"},
			{Name: "b", Type: "int", Default: 8, Description: "偏移 b"},
			{Name: "mode", Type: "str", Default: "decrypt", Description: "encrypt | decrypt"},
			{Name: "brute", Type: "bool", Default: false, Description: "穷举 a∈互素集 × b∈0..25（解密方向）"},
		},
		Runner: runAffine,
	}})
	reg.Register(&core.BaseTool{
		NameAttr: "atbash", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "埃特巴什 Atbash（A↔Z 字母表反向，自逆变换，大小写保留）",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "待处理文本"},
		},
		Runner: runAtbash,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "reverse", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "倒序密码（字符串反转）",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "待处理文本"},
		},
		Runner: runReverse,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "polybius", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "波利比奥斯 5×5 方阵坐标（I/J 合并为 24；空格=0；输出空格分隔坐标）",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "待处理文本 / 坐标串"},
			{Name: "mode", Type: "str", Default: "decrypt", Description: "encrypt | decrypt"},
		},
		Runner: runPolybius,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "xor", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "单字节循环 XOR（Fenham）：key 0-255，支持文本/十六进制/十进制输出，加解密同函数",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "待处理文本"},
			{Name: "key", Type: "int", Default: 42, Description: "单字节密钥 0-255"},
			{Name: "out", Type: "str", Default: "hex", Description: "输出格式 hex | dec | text"},
		},
		Runner: runXOR,
	})
	reg.Register(&HillTool{BaseTool: core.BaseTool{
		NameAttr: "hill", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "希尔密码 2×2（K=[[k1,k2],[k3,k4]]，det 须与 26 互素；不足补 x，解密去尾部填充）",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "待处理文本（仅字母参与，输出小写）"},
			{Name: "k1", Type: "int", Default: 3, Description: "密钥矩阵 k1"},
			{Name: "k2", Type: "int", Default: 3, Description: "密钥矩阵 k2"},
			{Name: "k3", Type: "int", Default: 2, Description: "密钥矩阵 k3"},
			{Name: "k4", Type: "int", Default: 5, Description: "密钥矩阵 k4"},
			{Name: "mode", Type: "str", Default: "decrypt", Description: "encrypt | decrypt"},
			{Name: "strip_x", Type: "bool", Default: true, Description: "解密后去掉末尾补位 x（明文本以 x 结尾时关闭）"},
		},
		Runner: runHill,
	}})
	reg.Register(&core.BaseTool{
		NameAttr: "columnar", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "列移位/列置换：数字串=逐位列序（如 3512），单个数字=等宽 n 列（CryptoDen 兼容），字母=按字母序 rank；行写列读",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "待处理文本"},
			{Name: "key", Type: "str", Required: true, Description: "列密钥（列序数字 / 宽度数字 / 关键词）"},
			{Name: "mode", Type: "str", Default: "decrypt", Description: "encrypt | decrypt"},
		},
		Runner: runColumnar,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "bifid", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "双密码 Bifid：关键词 5×5 方阵（无 J），坐标行串+列串拼接后重组字母",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "待处理文本（仅字母参与，J→I，输出大写）"},
			{Name: "key", Type: "str", Required: true, Description: "方阵关键词"},
			{Name: "mode", Type: "str", Default: "decrypt", Description: "encrypt | decrypt"},
		},
		Runner: runBifid,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "adfgx", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "ADFGX：key1 生成 5×5 Polybius 方阵（无 J），key2 列置换；ADFGX 字母坐标 + 按 rank 读列",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "明文（字母）/ 密文（ADFGX 字母）"},
			{Name: "key1", Type: "str", Required: true, Description: "Polybius 方阵关键词"},
			{Name: "key2", Type: "str", Required: true, Description: "列置换关键词"},
			{Name: "mode", Type: "str", Default: "decrypt", Description: "encrypt | decrypt"},
		},
		Runner: runADFGX,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "adfgvx", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "ADFGVX：key1 生成 6×6 方阵（A-Z + 0-9），key2 列置换；ADFGVX 字母坐标 + 按 rank 读列",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "明文（字母数字）/ 密文（ADFGVX 字母）"},
			{Name: "key1", Type: "str", Required: true, Description: "Polybius 方阵关键词"},
			{Name: "key2", Type: "str", Required: true, Description: "列置换关键词"},
			{Name: "mode", Type: "str", Default: "decrypt", Description: "encrypt | decrypt"},
		},
		Runner: runADFGVX,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "foursquare", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "四方密码：两关键词方阵（无 J），双字母组对角替换（重复字母插 X，末尾补 X）",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "明文 / 密文（仅字母，J→I，输出大写）"},
			{Name: "key1", Type: "str", Required: true, Description: "右上密码方阵关键词"},
			{Name: "key2", Type: "str", Required: true, Description: "左下密码方阵关键词"},
			{Name: "mode", Type: "str", Default: "decrypt", Description: "encrypt | decrypt"},
		},
		Runner: runFourSquare,
	})
}

// —— 工具壳（保持与其他工具一致的注册形态）——

type CaesarTool struct{ core.BaseTool }
type VigenereTool struct{ core.BaseTool }
type AffineTool struct{ core.BaseTool }
type HillTool struct{ core.BaseTool }

// ---------------------------------------------------------------------------
// 参数辅助（前缀 cl 避免包内冲突）
// ---------------------------------------------------------------------------

func clStr(params map[string]any, name string) string {
	if v, ok := params[name]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func clInt(params map[string]any, name string, def int) int {
	switch t := params[name].(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
	}
	return def
}

func clBool(params map[string]any, name string) bool {
	return clBoolDef(params, name, false)
}

func clBoolDef(params map[string]any, name string, def bool) bool {
	v, ok := params[name]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1" || t == "on"
	}
	return def
}

func clMode(params map[string]any) string {
	if m := strings.ToLower(strings.TrimSpace(clStr(params, "mode"))); m == "encrypt" || m == "enc" {
		return "encrypt"
	}
	return "decrypt"
}

func textOut(s string) (*core.ToolResult, error) {
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: s}, nil
}

func errOut(msg string) (*core.ToolResult, error) {
	return nil, fmt.Errorf("%s", msg)
}

// mod26 数学取模（结果非负），k1/k2 等用户输入可为任意整数。
func mod26(v int) int {
	v %= 26
	if v < 0 {
		v += 26
	}
	return v
}

// ---------------------------------------------------------------------------
// 凯撒
// ---------------------------------------------------------------------------

func caesarShift(s string, shift int) string {
	b := []byte(s)
	for i, c := range b {
		switch {
		case 'a' <= c && c <= 'z':
			b[i] = byte('a' + mod26(int(c-'a')+shift))
		case 'A' <= c && c <= 'Z':
			b[i] = byte('A' + mod26(int(c-'A')+shift))
		}
	}
	return string(b)
}

func runCaesar(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := clStr(params, "data")
	if data == "" {
		return errOut("请输入待处理文本")
	}
	shift := clInt(params, "shift", 3)
	if clBool(params, "brute") {
		var sb strings.Builder
		sb.WriteString("凯撒穷举（解密方向，全部 25 个位移）：\n\n")
		for k := 1; k <= 25; k++ {
			fmt.Fprintf(&sb, "shift %2d: %s\n", k, caesarShift(data, -k))
		}
		return textOut(sb.String())
	}
	if shift < 0 || shift > 25 {
		return errOut("位移须在 0-25 之间")
	}
	dir := 1
	if clMode(params) == "decrypt" {
		dir = -1
	}
	return textOut(caesarShift(data, dir*shift))
}

// ---------------------------------------------------------------------------
// 维吉尼亚
// ---------------------------------------------------------------------------

// vigenerePrintable 与 Python string.printable 完全一致（100 字符）。
const vigenerePrintable = "0123456789" +
	"abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~" +
	" \t\n\r\x0b\x0c"

func modN(v, n int) int {
	v %= n
	if v < 0 {
		v += n
	}
	return v
}

func vigenerePrintableRun(text, key string, sign int) string {
	abc := vigenerePrintable
	n := len(abc)
	out := make([]byte, 0, len(text))
	kj := 0
	for i := 0; i < len(text); i++ {
		ti := strings.IndexByte(abc, text[i])
		ki := strings.IndexByte(abc, key[kj%len(key)])
		if ti < 0 || ki < 0 {
			out = append(out, text[i]) // 不在字母表的字符原样且不消耗密钥
			continue
		}
		out = append(out, abc[modN(ti+sign*ki, n)])
		kj++
	}
	return string(out)
}

func vigenereAlphaRun(text, key string, sign int) string {
	out := make([]byte, 0, len(text))
	kj := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		var base byte
		switch {
		case 'a' <= c && c <= 'z':
			base = 'a'
		case 'A' <= c && c <= 'Z':
			base = 'A'
		default:
			out = append(out, c)
			continue
		}
		k := key[kj%len(key)]
		var kb int
		switch {
		case 'a' <= k && k <= 'z':
			kb = int(k - 'a')
		case 'A' <= k && k <= 'Z':
			kb = int(k - 'A')
		default:
			kb = 0
		}
		out = append(out, base+byte(mod26(int(c-base)+sign*kb)))
		kj++
	}
	return string(out)
}

func runVigenere(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := clStr(params, "data")
	key := strings.TrimSpace(clStr(params, "key"))
	if data == "" || key == "" {
		return errOut("请输入文本与密钥")
	}
	sign := 1
	if clMode(params) == "decrypt" {
		sign = -1
	}
	if strings.TrimSpace(clStr(params, "alphabet")) == "alpha" {
		return textOut(vigenereAlphaRun(data, key, sign))
	}
	return textOut(vigenerePrintableRun(data, key, sign))
}

// ---------------------------------------------------------------------------
// 仿射
// ---------------------------------------------------------------------------

var affineCoprimes = []int{1, 3, 5, 7, 9, 11, 15, 17, 19, 21, 23, 25}

func affineRun(text string, a, b, sign int) string {
	out := []byte{}
	aM, bM := mod26(a), mod26(b)
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case 'a' <= c && c <= 'z':
			if sign > 0 {
				out = append(out, byte('a'+mod26(aM*int(c-'a')+bM)))
			} else {
				inv, _ := modInverse26(aM)
				out = append(out, byte('a'+mod26(inv*(int(c-'a')-bM))))
			}
		case 'A' <= c && c <= 'Z':
			if sign > 0 {
				out = append(out, byte('A'+mod26(aM*int(c-'A')+bM)))
			} else {
				inv, _ := modInverse26(aM)
				out = append(out, byte('A'+mod26(inv*(int(c-'A')-bM))))
			}
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

func modInverse26(a int) (int, bool) {
	a = mod26(a)
	for i := 0; i < 26; i++ {
		if a*i%26 == 1 {
			return i, true
		}
	}
	return 0, false
}

func runAffine(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := clStr(params, "data")
	if data == "" {
		return errOut("请输入待处理文本")
	}
	if clBool(params, "brute") {
		var sb strings.Builder
		sb.WriteString("仿射穷举（解密方向，a∈12 个互素值 × b∈0..25 共 312 组）：\n\n")
		for _, a := range affineCoprimes {
			for b := 0; b < 26; b++ {
				fmt.Fprintf(&sb, "a=%2d b=%2d: %s\n", a, b, affineRun(data, a, b, -1))
			}
		}
		return textOut(sb.String())
	}
	a := clInt(params, "a", 5)
	b := clInt(params, "b", 8)
	if _, ok := modInverse26(a); !ok {
		return errOut(fmt.Sprintf("a=%d 与 26 不互素，不存在模逆", a))
	}
	if clMode(params) == "decrypt" {
		return textOut(affineRun(data, a, b, -1))
	}
	return textOut(affineRun(data, a, b, 1))
}

// ---------------------------------------------------------------------------
// 埃特巴什 / 倒序
// ---------------------------------------------------------------------------

func runAtbash(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := clStr(params, "data")
	if data == "" {
		return errOut("请输入待处理文本")
	}
	out := []byte{}
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case 'a' <= c && c <= 'z':
			out = append(out, 'z'-(c-'a'))
		case 'A' <= c && c <= 'Z':
			out = append(out, 'Z'-(c-'A'))
		default:
			out = append(out, c)
		}
	}
	return textOut(string(out))
}

func runReverse(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := clStr(params, "data")
	if data == "" {
		return errOut("请输入待处理文本")
	}
	runes := []rune(data)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return textOut(string(runes))
}

// ---------------------------------------------------------------------------
// 波利比奥斯（5×5，I/J 合并，空格=0）
// ---------------------------------------------------------------------------

var polybiusCoords = map[byte]string{
	'a': "11", 'b': "12", 'c': "13", 'd': "14", 'e': "15",
	'f': "21", 'g': "22", 'h': "23", 'i': "24", 'k': "25",
	'l': "31", 'm': "32", 'n': "33", 'o': "34", 'p': "35",
	'q': "41", 'r': "42", 's': "43", 't': "44", 'u': "45",
	'v': "51", 'w': "52", 'x': "53", 'y': "54", 'z': "55",
}

var polybiusLetters = map[string]byte{
	"0": ' ', "11": 'a', "12": 'b', "13": 'c', "14": 'd', "15": 'e',
	"21": 'f', "22": 'g', "23": 'h', "24": 'i', "25": 'k',
	"31": 'l', "32": 'm', "33": 'n', "34": 'o', "35": 'p',
	"41": 'q', "42": 'r', "43": 's', "44": 't', "45": 'u',
	"51": 'v', "52": 'w', "53": 'x', "54": 'y', "55": 'z',
}

func runPolybius(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := clStr(params, "data")
	if data == "" {
		return errOut("请输入待处理文本")
	}
	if clMode(params) == "encrypt" {
		var toks []string
		for _, c := range strings.ToLower(data) {
			if c == ' ' {
				toks = append(toks, "0")
			} else if code, ok := polybiusCoords[byte(c)]; ok {
				toks = append(toks, code)
			} else {
				toks = append(toks, string(c))
			}
		}
		return textOut(strings.Join(toks, " "))
	}
	var sb strings.Builder
	for _, tok := range strings.Fields(data) {
		if l, ok := polybiusLetters[tok]; ok {
			sb.WriteByte(l)
		} else {
			sb.WriteString(tok)
		}
	}
	return textOut(sb.String())
}

// ---------------------------------------------------------------------------
// XOR（Fenham）
// ---------------------------------------------------------------------------

func runXOR(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := clStr(params, "data")
	if data == "" {
		return errOut("请输入待处理文本")
	}
	key := clInt(params, "key", 42)
	if key < 0 || key > 255 {
		return errOut("密钥须在 0-255 之间")
	}
	out := clStr(params, "out")
	if out == "" {
		out = "hex"
	}
	switch out {
	case "dec":
		var toks []string
		for i := 0; i < len(data); i++ {
			toks = append(toks, strconv.Itoa(int(data[i]^byte(key))))
		}
		return textOut(strings.Join(toks, " "))
	case "text":
		b := make([]byte, len(data))
		for i := 0; i < len(data); i++ {
			b[i] = data[i] ^ byte(key)
		}
		return textOut(string(b))
	default: // hex
		var toks []string
		for i := 0; i < len(data); i++ {
			toks = append(toks, fmt.Sprintf("%02x", data[i]^byte(key)))
		}
		return textOut(strings.Join(toks, " "))
	}
}

// ---------------------------------------------------------------------------
// 希尔 2×2
// ---------------------------------------------------------------------------

func runHill(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := strings.ToLower(clStr(params, "data"))
	if data == "" {
		return errOut("请输入待处理文本")
	}
	k1, k2 := clInt(params, "k1", 3), clInt(params, "k2", 3)
	k3, k4 := clInt(params, "k3", 2), clInt(params, "k4", 5)

	var letters []byte
	for i := 0; i < len(data); i++ {
		if 'a' <= data[i] && data[i] <= 'z' {
			letters = append(letters, data[i]-'a')
		}
	}
	if len(letters) == 0 {
		return errOut("文本中没有字母")
	}
	if len(letters)%2 == 1 {
		letters = append(letters, 'x'-'a') // 补位 x；解密端用 strip_x 去除
	}

	det := k1*k4 - k2*k3
	invDet, ok := modInverse26(det)
	if !ok {
		return errOut(fmt.Sprintf("det(K)=%d 与 26 不互素，密钥矩阵在 mod 26 下不可逆", det))
	}

	out := make([]byte, 0, len(letters))
	if clMode(params) == "encrypt" {
		for i := 0; i < len(letters); i += 2 {
			x, y := int(letters[i]), int(letters[i+1])
			c1 := mod26(k1*x + k2*y)
			c2 := mod26(k3*x + k4*y)
			out = append(out, 'a'+byte(c1), 'a'+byte(c2))
		}
		return textOut(string(out))
	}
	// 解密：K⁻¹ = det⁻¹ · [[k4, -k2], [-k3, k1]] (mod 26)
	for i := 0; i < len(letters); i += 2 {
		x, y := int(letters[i]), int(letters[i+1])
		p1 := mod26(invDet * (k4*x - k2*y))
		p2 := mod26(invDet * (-k3*x + k1*y))
		out = append(out, 'a'+byte(p1), 'a'+byte(p2))
	}
	// 解密无法从偶数长度密文判断明文是否补过 x：strip_x（默认开）去掉末尾一个 x
	if clBoolDef(params, "strip_x", true) && len(out) > 0 && out[len(out)-1] == 'x' {
		out = out[:len(out)-1]
	}
	return textOut(string(out))
}

// ---------------------------------------------------------------------------
// 列移位 / 列置换
// ---------------------------------------------------------------------------

// colKeyRanks 解析列密钥为 0 起始的读列顺序：
//   - 全数字且为 1..n / 0..n-1 的顺序串 → 等宽 n 列、按原序读（CryptoDen 兼容）
//   - 其他全数字串（如 "3512"）→ 按数字相对大小 rank 的逐位列序
//   - 字母 → 按字母序稳定 rank（重复字母按出现顺序，与 CryptoDen sequenceOrd 一致）
func colKeyRanks(key string) ([]int, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("请输入列密钥")
	}
	if isAllDigits(key) {
		n, err := strconv.Atoi(key)
		if err != nil || n < 2 || n > 10000 {
			return nil, fmt.Errorf("数字列密钥须为 2-10000 的宽度或列序排列")
		}
		if len(key) == 1 || isSeq(key, 1) || isSeq(key, 0) {
			// 宽度模式：等宽 n 列按原序读（CryptoDen 兼容；"1234"/"0123" 视为宽度 4/10 的顺序串）
			order := make([]int, n)
			for i := range order {
				order[i] = i
			}
			return order, nil
		}
		// 多位非顺序数字：各位互异且值域恰为 1..len 或 0..len-1 → 逐位列序（如 "3512"）
		if digits := []rune(key); distinctRunes(digits) && (maxRune(digits) == rune('0'+len(digits)) || maxRune(digits) == rune('0'+len(digits)-1)) {
			return stableRanks(digits), nil
		}
		// 其余数字串按宽度处理
		order := make([]int, n)
		for i := range order {
			order[i] = i
		}
		return order, nil
	}
	upper := []rune(strings.ToUpper(strings.ReplaceAll(key, " ", "")))
	return stableRanks(upper), nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isSeq 判断数字串是否从 start 开始的连续序列（"1234" start=1 / "0123" start=0）。
func isSeq(s string, start int) bool {
	for i, c := range s {
		if int(c-'0') != start+i {
			return false
		}
	}
	return true
}

func distinctRunes(rs []rune) bool {
	seen := map[rune]bool{}
	for _, r := range rs {
		if seen[r] {
			return false
		}
		seen[r] = true
	}
	return true
}

func maxRune(rs []rune) rune {
	m := rs[0]
	for _, r := range rs[1:] {
		if r > m {
			m = r
		}
	}
	return m
}

// stableRanks 稳定排名（0 起始）：相等元素按出现先后。
func stableRanks(items []rune) []int {
	ranks := make([]int, len(items))
	for i := range items {
		r := 0
		for j := 0; j < len(items); j++ {
			if items[j] < items[i] || (items[j] == items[i] && j < i) {
				r++
			}
		}
		ranks[i] = r
	}
	return ranks
}

// wordRanks 关键词列序（CryptoDen sequenceOrd 等价）。
func wordRanks(key string) []int {
	return stableRanks([]rune(key))
}

func rankIndex(ranks []int, r int) int {
	for i, v := range ranks {
		if v == r {
			return i
		}
	}
	return 0
}

// columnarEncrypt 行写列读：按 ranks 顺序拼接各列。
func columnarEncrypt(text string, ranks []int) string {
	n := len(ranks)
	cols := make([]string, n)
	for i := 0; i < n; i++ {
		var sb strings.Builder
		for j := i; j < len(text); j += n {
			sb.WriteByte(text[j])
		}
		cols[i] = sb.String()
	}
	var out strings.Builder
	for r := 0; r < n; r++ {
		out.WriteString(cols[rankIndex(ranks, r)])
	}
	return out.String()
}

// columnarDecrypt columnarEncrypt 的逆：按 rank 顺序回填各列，再按行读出。
func columnarDecrypt(text string, ranks []int) string {
	n := len(ranks)
	base, extra := len(text)/n, len(text)%n
	colLen := make([]int, n)
	for i := 0; i < n; i++ {
		colLen[i] = base
		if i < extra {
			colLen[i]++
		}
	}
	cols := make([]string, n)
	pos := 0
	for r := 0; r < n; r++ {
		idx := rankIndex(ranks, r)
		cols[idx] = text[pos : pos+colLen[idx]]
		pos += colLen[idx]
	}
	var out strings.Builder
	for row := 0; ; row++ {
		wrote := false
		for c := 0; c < n; c++ {
			if row < len(cols[c]) {
				out.WriteByte(cols[c][row])
				wrote = true
			}
		}
		if !wrote {
			break
		}
	}
	return out.String()
}

func runColumnar(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := strings.Join(strings.Fields(clStr(params, "data")), "")
	if data == "" {
		return errOut("请输入待处理文本")
	}
	ranks, err := colKeyRanks(clStr(params, "key"))
	if err != nil {
		return nil, err
	}
	if clMode(params) == "encrypt" {
		return textOut(columnarEncrypt(data, ranks))
	}
	return textOut(columnarDecrypt(data, ranks))
}

// ---------------------------------------------------------------------------
// 关键词方阵（Bifid / ADFGX / ADFGVX / FourSquare 共用）
// ---------------------------------------------------------------------------

const square25 = "ABCDEFGHIKLMNOPQRSTUVWXYZ" // 无 J
const square36 = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// keySquare 生成关键词方阵（关键词去重在前，剩余字母表补齐）。
// J→I 替换仅在 5×5 字母表（不含 J）时进行，6×6 方阵保留 J。
func keySquare(keyword, alphabet string) []byte {
	hasJ := strings.IndexByte(alphabet, 'J') >= 0
	seen := map[byte]bool{}
	var sq []byte
	for i := 0; i < len(keyword); i++ {
		c := keyword[i]
		if c == 'J' && !hasJ {
			c = 'I'
		}
		if !seen[c] && strings.IndexByte(alphabet, c) >= 0 {
			seen[c] = true
			sq = append(sq, c)
		}
	}
	for i := 0; i < len(alphabet); i++ {
		if !seen[alphabet[i]] {
			seen[alphabet[i]] = true
			sq = append(sq, alphabet[i])
		}
	}
	return sq
}

// squarePos 返回字符在方阵中的 (row, col)，不存在返回 (-1,-1)。
func squarePos(sq []byte, c byte) (int, int) {
	side := 1
	for side*side < len(sq) {
		side++
	}
	idx := strings.IndexByte(string(sq), c)
	if idx < 0 {
		return -1, -1
	}
	return idx / side, idx % side
}

// ---------------------------------------------------------------------------
// Bifid
// ---------------------------------------------------------------------------

func runBifid(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := strings.ToUpper(strings.Join(strings.Fields(clStr(params, "data")), ""))
	key := strings.ToUpper(strings.Join(strings.Fields(clStr(params, "key")), ""))
	if data == "" || key == "" {
		return errOut("请输入文本与方阵关键词")
	}
	data = strings.ReplaceAll(data, "J", "I")
	sq := keySquare(key, square25)

	var pt []byte
	for i := 0; i < len(data); i++ {
		if r, _ := squarePos(sq, data[i]); r >= 0 {
			pt = append(pt, data[i])
		}
	}
	if len(pt) == 0 {
		return errOut("文本中没有可用字母")
	}

	if clMode(params) == "encrypt" {
		var rows, cols []int
		for i := 0; i < len(pt); i++ {
			r, c := squarePos(sq, pt[i])
			rows = append(rows, r)
			cols = append(cols, c)
		}
		fused := append(append([]int{}, rows...), cols...)
		var out []byte
		for i := 0; i+1 < len(fused); i += 2 {
			out = append(out, sq[fused[i]*5+fused[i+1]])
		}
		return textOut(string(out))
	}
	var coords []int
	for i := 0; i < len(pt); i++ {
		r, c := squarePos(sq, pt[i])
		coords = append(coords, r, c)
	}
	half := len(coords) / 2
	rows, cols := coords[:half], coords[half:]
	var out []byte
	for i := 0; i < half; i++ {
		out = append(out, sq[rows[i]*5+cols[i]])
	}
	return textOut(string(out))
}

// ---------------------------------------------------------------------------
// ADFGX / ADFGVX（Polybius 坐标 + 关键词列置换）
// ---------------------------------------------------------------------------

func runADFGX(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := strings.ToUpper(strings.Join(strings.Fields(clStr(params, "data")), ""))
	key1 := strings.ToUpper(strings.Join(strings.Fields(clStr(params, "key1")), ""))
	key2 := strings.ToUpper(strings.Join(strings.Fields(clStr(params, "key2")), ""))
	if data == "" || key1 == "" || key2 == "" {
		return errOut("请输入文本与两个密钥（key1 方阵 / key2 列置换）")
	}
	labels := "ADFGX"
	if clMode(params) == "encrypt" {
		data = strings.ReplaceAll(data, "J", "I")
		sq := keySquare(key1, square25)
		var mid strings.Builder
		for i := 0; i < len(data); i++ {
			if r, c := squarePos(sq, data[i]); r >= 0 {
				mid.WriteByte(labels[r])
				mid.WriteByte(labels[c])
			}
		}
		return textOut(columnarEncrypt(mid.String(), wordRanks(key2)))
	}
	var ct []byte
	for i := 0; i < len(data); i++ {
		if strings.IndexByte(labels, data[i]) >= 0 {
			ct = append(ct, data[i])
		}
	}
	mid := columnarDecrypt(string(ct), wordRanks(key2))
	li := map[byte]int{'A': 0, 'D': 1, 'F': 2, 'G': 3, 'X': 4}
	sq := keySquare(key1, square25)
	var out []byte
	for i := 0; i+1 < len(mid); i += 2 {
		r, ok1 := li[mid[i]]
		c, ok2 := li[mid[i+1]]
		if ok1 && ok2 {
			out = append(out, sq[r*5+c])
		}
	}
	return textOut(string(out))
}

func runADFGVX(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := strings.ToUpper(strings.Join(strings.Fields(clStr(params, "data")), ""))
	key1 := strings.ToUpper(strings.Join(strings.Fields(clStr(params, "key1")), ""))
	key2 := strings.ToUpper(strings.Join(strings.Fields(clStr(params, "key2")), ""))
	if data == "" || key1 == "" || key2 == "" {
		return errOut("请输入文本与两个密钥（key1 方阵 / key2 列置换）")
	}
	labels := "ADFGVX"
	if clMode(params) == "encrypt" {
		sq := keySquare(key1, square36)
		var mid strings.Builder
		for i := 0; i < len(data); i++ {
			if r, c := squarePos(sq, data[i]); r >= 0 {
				mid.WriteByte(labels[r])
				mid.WriteByte(labels[c])
			}
		}
		return textOut(columnarEncrypt(mid.String(), wordRanks(key2)))
	}
	var ct []byte
	for i := 0; i < len(data); i++ {
		if strings.IndexByte(labels, data[i]) >= 0 {
			ct = append(ct, data[i])
		}
	}
	mid := columnarDecrypt(string(ct), wordRanks(key2))
	li := map[byte]int{'A': 0, 'D': 1, 'F': 2, 'G': 3, 'V': 4, 'X': 5}
	sq := keySquare(key1, square36)
	var out []byte
	for i := 0; i+1 < len(mid); i += 2 {
		r, ok1 := li[mid[i]]
		c, ok2 := li[mid[i+1]]
		if ok1 && ok2 {
			out = append(out, sq[r*6+c])
		}
	}
	return textOut(string(out))
}

// ---------------------------------------------------------------------------
// 四方密码 FourSquare
// ---------------------------------------------------------------------------

// plainSquare 明文方阵（左上/右下共用，无 J）。
const plainSquare = "ABCDEFGHIKLMNOPQRSTUVWXYZ"

func runFourSquare(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	data := strings.ToUpper(strings.Join(strings.Fields(clStr(params, "data")), ""))
	key1 := strings.ToUpper(strings.Join(strings.Fields(clStr(params, "key1")), ""))
	key2 := strings.ToUpper(strings.Join(strings.Fields(clStr(params, "key2")), ""))
	if data == "" || key1 == "" || key2 == "" {
		return errOut("请输入文本与两个方阵关键词")
	}
	data = strings.ReplaceAll(data, "J", "I")
	sq1 := keySquare(key1, square25) // 右上密码方阵
	sq2 := keySquare(key2, square25) // 左下密码方阵

	// 只保留方阵内字母
	var kept []byte
	for i := 0; i < len(data); i++ {
		if strings.IndexByte(square25, data[i]) >= 0 {
			kept = append(kept, data[i])
		}
	}
	if len(kept) == 0 {
		return errOut("文本中没有可用字母")
	}

	if clMode(params) == "encrypt" {
		// 双字母组：重复字母插 X，末尾奇数补 X（CryptoDen 同款规则）
		pt := append([]byte{}, kept...)
		for s := 0; s < len(pt)-1; s += 2 {
			if pt[s] == pt[s+1] {
				pt = append(pt, 0)
				copy(pt[s+2:], pt[s+1:])
				pt[s+1] = 'X'
			}
		}
		if len(pt)%2 == 1 {
			pt = append(pt, 'X')
		}
		var out []byte
		for i := 0; i+1 < len(pt); i += 2 {
			r1 := strings.IndexByte(plainSquare, pt[i]) / 5
			c1 := strings.IndexByte(plainSquare, pt[i]) % 5
			r2 := strings.IndexByte(plainSquare, pt[i+1]) / 5
			c2 := strings.IndexByte(plainSquare, pt[i+1]) % 5
			out = append(out, sq1[r1*5+c2], sq2[r2*5+c1])
		}
		return textOut(string(out))
	}

	// 解密：密文第 1/3/5… 个字母在 sq1（右上），第 2/4/6… 个在 sq2（左下）
	var ct1, ct2 []byte
	for i := 0; i+1 < len(kept); i += 2 {
		ct1 = append(ct1, kept[i])
		ct2 = append(ct2, kept[i+1])
	}
	var out []byte
	for i := 0; i < len(ct1); i++ {
		r1, c2 := squarePos(sq1, ct1[i])
		r2, c1 := squarePos(sq2, ct2[i])
		if r1 < 0 || r2 < 0 {
			continue
		}
		out = append(out, plainSquare[r1*5+c1], plainSquare[r2*5+c2])
	}
	return textOut(string(out))
}
