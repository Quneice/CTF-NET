// data.crawl.packet-crawler —— 原始包爬虫。
//
// 三大能力（对应前端「数据处理 → 数据爬虫」模块）：
//  1. 解析用户提供的原始 HTTP 请求包（Burp / 浏览器复制格式）；
//  2. 占位符自动替换并批量发包：请求包内写 {{name}}，占位符定义区每行
//     `name=取值`，支持整数范围（1-100[:步长[:补零宽度]]）与枚举列表（a,b,c），
//     多个占位符做笛卡尔积展开，并发发送；
//  3. 解析返回包并保存内容：按魔数/Content-Type/URL 后缀识别文件类型
//     （png/jpg/gif/webp/pdf/zip/gz/7z/rar/tar 等），保存到产物目录；对文本类
//     响应（HTML/JSON/XML/纯文本）可提取其中的 URL 并做一层深度下载。
//
// 产物：ArtifactsDir/crawl/ 下按原始文件名保存全部文件（重名时追加 _N 后缀）；
// ArtifactsDir/crawl_report.txt
// 为逐请求明细；ArtifactsDir/result.zip 打包全部爬取结果与明细。
package data

import (
	"archive/zip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&PacketCrawlerTool{BaseTool: core.BaseTool{
		NameAttr:        "packet-crawler",
		CategoryAttr:    core.Category("data.crawl"),
		VersionAttr:     "1.0.0",
		DescriptionAttr: "解析原始 HTTP 包、占位符批量发包并保存响应内容（图片/压缩包等）",
		TimeoutAttr:     3600,
		ParamsAttr: []core.ParamSpec{
			{Name: "raw_request", Type: "text", Required: true, Description: "原始 HTTP 请求包（Burp 格式），占位符写 {{name}}"},
			{Name: "placeholders", Type: "text", Required: false, Default: "", Description: "占位符定义，每行一个：name=1-100 或 name=1-100:2:4（步长/补零）或 name=a,b,c（枚举）"},
			{Name: "scheme", Type: "str", Required: false, Default: "http", Description: "请求行为路径型目标时使用的协议（http/https）"},
			{Name: "concurrency", Type: "int", Required: false, Default: 5, Description: "并发数"},
			{Name: "timeout", Type: "int", Required: false, Default: 15, Description: "单请求超时（秒）"},
			{Name: "delay_ms", Type: "int", Required: false, Default: 0, Description: "请求间隔（毫秒）"},
			{Name: "max_requests", Type: "int", Required: false, Default: 2000, Description: "最大请求数（组合数超过时拒绝执行）"},
			{Name: "follow_links", Type: "bool", Required: false, Default: true, Description: "从文本类响应中提取 URL 并下载（深度 1）"},
		},
		Runner: packetCrawlerRun,
	}})
}

// PacketCrawlerTool 原始包爬虫工具。
type PacketCrawlerTool struct {
	core.BaseTool
}

// ---------------------------------------------------------------------------
// 参数取值
// ---------------------------------------------------------------------------

func pcInt(params map[string]any, name string, def int) int {
	v, ok := params[name]
	if !ok {
		return def
	}
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
	}
	return def
}

func pcStr(params map[string]any, name string) string {
	s, _ := params[name].(string)
	return s
}

func pcBool(params map[string]any, name string, def bool) bool {
	v, ok := params[name]
	if !ok {
		return def
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(t))
		if err == nil {
			return b
		}
	}
	return def
}

// ---------------------------------------------------------------------------
// 原始包解析
// ---------------------------------------------------------------------------

// parsedRequest 解析后的原始请求包。
type parsedRequest struct {
	method  string
	target  string // 原始请求行 target（可能是绝对 URL 或路径）
	headers map[string]string
	body    string
}

var placeholderRe = regexp.MustCompile(`\{\{([A-Za-z][A-Za-z0-9_-]*)\}\}`)

// parseRawRequest 按 HTTP/1.x 文本格式解析原始包（保留 body 的 CRLF）。
func parseRawRequest(raw string) (*parsedRequest, error) {
	sep := "\n"
	if strings.Contains(raw, "\r\n") {
		sep = "\r\n"
	}
	lines := strings.Split(raw, sep)
	// 请求行：跳过开头的空行
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return nil, fmt.Errorf("原始包为空")
	}
	reqLine := strings.TrimSpace(lines[i])
	i++
	fields := strings.Fields(reqLine)
	if len(fields) < 2 {
		return nil, fmt.Errorf("请求行不合法（需为 METHOD TARGET [HTTP/x]）: %q", reqLine)
	}
	pr := &parsedRequest{method: strings.ToUpper(fields[0]), target: fields[1], headers: map[string]string{}}
	for i < len(lines) {
		line := strings.TrimRight(lines[i], "\r")
		i++
		if line == "" {
			break // 空行 = 头部结束
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		name := http.CanonicalHeaderKey(strings.TrimSpace(line[:idx]))
		pr.headers[name] = strings.TrimSpace(line[idx+1:])
	}
	pr.body = strings.Join(lines[i:], sep)
	return pr, nil
}

// buildHTTPRequest 把解析结果构造成 http.Request。
// scheme 仅在 target 为路径型时使用；Host 头转换为 req.Host，
// Content-Length / Transfer-Encoding 交由 net/http 按 body 重算（占位符替换后长度会变）。
func buildHTTPRequest(pr *parsedRequest, scheme string) (*http.Request, error) {
	target := pr.target
	hostHeader := pr.headers["Host"]
	fullURL := target
	if u, err := url.Parse(target); err == nil && u.Scheme != "" {
		// 绝对 URL：按 target 自身走
	} else {
		if hostHeader == "" {
			return nil, fmt.Errorf("请求行为路径型目标但原始包缺少 Host 头")
		}
		if scheme != "http" && scheme != "https" {
			scheme = "http"
		}
		fullURL = scheme + "://" + hostHeader + target
	}
	req, err := http.NewRequest(pr.method, fullURL, strings.NewReader(pr.body))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %v", err)
	}
	for name, val := range pr.headers {
		switch name {
		case "Host":
			req.Host = val
		case "Content-Length", "Transfer-Encoding":
			// NewRequest 已按 body 设置 ContentLength，占位符替换后长度可能变化，需重算
		default:
			req.Header.Set(name, val)
		}
	}
	return req, nil
}

// substitute 把 {{name}} 全部替换为对应取值。
func substitute(template string, values map[string]string) string {
	out := template
	for name, val := range values {
		out = strings.ReplaceAll(out, "{{"+name+"}}", val)
	}
	return out
}

// usedPlaceholderNames 按首次出现顺序返回请求包中出现的占位符名。
func usedPlaceholderNames(raw string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range placeholderRe.FindAllStringSubmatch(raw, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// 占位符定义解析与组合展开
// ---------------------------------------------------------------------------

// placeholderSpec 单个占位符的全部取值（展开后）。
type placeholderSpec struct {
	name   string
	values []string
}

var placeholderNameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
var rangeSpecRe = regexp.MustCompile(`^(-?\d+)\s*-\s*(-?\d+)(?::\s*(-?\d+))?(?::\s*(\d+))?$`)

const maxValuesPerPlaceholder = 100000

// parsePlaceholderSpecs 解析占位符定义文本。每行 `name=取值`；# 开头为注释。
func parsePlaceholderSpecs(text string) ([]placeholderSpec, error) {
	var out []placeholderSpec
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			return nil, fmt.Errorf("占位符定义行不合法（应为 name=取值）: %q", line)
		}
		name := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if !placeholderNameRe.MatchString(name) {
			return nil, fmt.Errorf("占位符名不合法（字母开头，仅字母数字_-）: %q", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("占位符重复定义: %s", name)
		}
		seen[name] = true
		values, err := expandPlaceholderValues(val)
		if err != nil {
			return nil, fmt.Errorf("占位符 %s: %v", name, err)
		}
		out = append(out, placeholderSpec{name: name, values: values})
	}
	return out, nil
}

// expandPlaceholderValues 展开单个取值表达式：
//   - `a-b` 整数范围（含端点；a>b 时步长自动为 -1）
//   - `a-b:s` 指定步长；`a-b:s:pad` 额外零填充到 pad 宽度
//   - `v1,v2,...` 枚举列表（字面量）
func expandPlaceholderValues(expr string) ([]string, error) {
	if m := rangeSpecRe.FindStringSubmatch(expr); m != nil {
		start, err1 := strconv.Atoi(m[1])
		end, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("范围数值不合法: %q", expr)
		}
		step := 1
		if start > end {
			step = -1
		}
		if m[3] != "" {
			var err error
			step, err = strconv.Atoi(m[3])
			if err != nil {
				return nil, fmt.Errorf("步长不合法: %q", m[3])
			}
		}
		if step == 0 {
			return nil, fmt.Errorf("步长不能为 0")
		}
		pad := 0
		if m[4] != "" {
			var err error
			pad, err = strconv.Atoi(m[4])
			if err != nil || pad < 0 || pad > 12 {
				return nil, fmt.Errorf("补零宽度不合法（0~12）: %q", m[4])
			}
		}
		// 方向一致性校验
		if (end-start)*step < 0 {
			return nil, fmt.Errorf("范围方向与步长方向不一致: %q", expr)
		}
		n := (end-start)/step + 1
		if n <= 0 || n > maxValuesPerPlaceholder {
			return nil, fmt.Errorf("取值范围过大或为空（单占位符最多 %d 个值）: %q", maxValuesPerPlaceholder, expr)
		}
		values := make([]string, 0, n)
		for v := start; ; v += step {
			s := strconv.Itoa(v)
			if pad > 0 {
				// 零填充仅对非负整数有意义；负数保持原样（避免 %0*d 对负号的歧义）
				if v >= 0 {
					s = fmt.Sprintf("%0*d", pad, v)
				}
			}
			values = append(values, s)
			if v == end {
				break
			}
		}
		return values, nil
	}
	// 枚举列表
	parts := strings.Split(expr, ",")
	values := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		values = append(values, p)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("取值为空: %q", expr)
	}
	if len(values) > maxValuesPerPlaceholder {
		return nil, fmt.Errorf("枚举值过多（最多 %d 个）", maxValuesPerPlaceholder)
	}
	return values, nil
}

// expandCombinations 按首次出现顺序做笛卡尔积展开，返回每个组合的取值映射。
// maxRequests 为组合数上限。返回组合列表；组合数为 0（无占位符）时返回单个空映射。
func expandCombinations(raw string, specs []placeholderSpec, maxRequests int) ([]map[string]string, error) {
	used := usedPlaceholderNames(raw)
	if len(used) == 0 {
		return []map[string]string{{}}, nil
	}
	byName := map[string][]string{}
	for _, sp := range specs {
		byName[sp.name] = sp.values
	}
	for _, name := range used {
		if _, ok := byName[name]; !ok {
			return nil, fmt.Errorf("请求包中的占位符 {{%s}} 未在占位符定义中给出取值", name)
		}
	}

	total := 1
	for _, name := range used {
		total *= len(byName[name])
		if total > maxRequests {
			return nil, fmt.Errorf("占位符组合数超过 max_requests=%d，请缩小取值范围或调大上限", maxRequests)
		}
	}
	combs := make([]map[string]string, 0, total)
	var rec func(i int, cur map[string]string)
	rec = func(i int, cur map[string]string) {
		if i == len(used) {
			cp := make(map[string]string, len(cur))
			for k, v := range cur {
				cp[k] = v
			}
			combs = append(combs, cp)
			return
		}
		name := used[i]
		for _, v := range byName[name] {
			cur[name] = v
			rec(i+1, cur)
		}
	}
	rec(0, map[string]string{})
	return combs, nil
}

// unusedSpecNames 返回已定义但请求包中未使用的占位符名（仅用于提示）。
func unusedSpecNames(raw string, specs []placeholderSpec) []string {
	usedSet := map[string]bool{}
	for _, n := range usedPlaceholderNames(raw) {
		usedSet[n] = true
	}
	var out []string
	for _, sp := range specs {
		if !usedSet[sp.name] {
			out = append(out, sp.name)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// 文件类型识别
// ---------------------------------------------------------------------------

// sniffExt 按魔数 → Content-Type → URL 后缀的优先级识别扩展名（含点）。
// 返回 ext 与 isText。识别不了二进制默认 .bin，文本类按 Content-Type 映射。
func sniffExt(head []byte, contentType string, urlPath string) (string, bool) {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	// 1) 魔数（取前 16 字节足够覆盖大部分格式）
	magic := head
	m := func(sig ...byte) bool {
		if len(magic) < len(sig) {
			return false
		}
		for i, b := range sig {
			if magic[i] != b {
				return false
			}
		}
		return true
	}
	switch {
	case m(0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A):
		return ".png", false
	case m(0xFF, 0xD8, 0xFF):
		return ".jpg", false
	case m('G', 'I', 'F', '8'):
		return ".gif", false
	case m('R', 'I', 'F', 'F') && len(magic) >= 12 && string(magic[8:12]) == "WEBP":
		return ".webp", false
	case m('B', 'M'):
		return ".bmp", false
	case m(0x00, 0x00, 0x01, 0x00):
		return ".ico", false
	case m('%', 'P', 'D', 'F'):
		return ".pdf", false
	case m('P', 'K', 0x03, 0x04), m('P', 'K', 0x05, 0x06):
		return ".zip", false
	case m(0x1F, 0x8B):
		return ".gz", false
	case m('7', 'z', 0xBC, 0xAF, 0x27, 0x1C):
		return ".7z", false
	case m('R', 'a', 'r', '!', 0x1A, 0x07):
		return ".rar", false
	case len(magic) >= 262 && string(magic[257:262]) == "ustar":
		return ".tar", false
	case m('S', 'Q', 'L', 'i', 't', 'e', ' ', 'f', 'o', 'r', 'm', 'a', 't'):
		return ".sqlite", false
	case m('M', 'Z'):
		return ".exe", false
	case m(0x7F, 'E', 'L', 'F'):
		return ".elf", false
	case m('R', 'I', 'F', 'F') && len(magic) >= 12 && string(magic[8:12]) == "WAVE":
		return ".wav", false
	case m('I', 'D', '3'), m(0xFF, 0xFB), m(0xFF, 0xF3):
		return ".mp3", false
	case m('O', 'g', 'g', 'S'):
		return ".ogg", false
	}
	// 2) Content-Type
	switch ct {
	case "image/jpeg":
		return ".jpg", false
	case "image/png":
		return ".png", false
	case "image/gif":
		return ".gif", false
	case "image/webp":
		return ".webp", false
	case "image/bmp", "image/x-ms-bmp":
		return ".bmp", false
	case "image/x-icon", "image/vnd.microsoft.icon":
		return ".ico", false
	case "image/svg+xml":
		return ".svg", true
	case "application/pdf":
		return ".pdf", false
	case "application/zip", "application/x-zip-compressed":
		return ".zip", false
	case "application/x-7z-compressed":
		return ".7z", false
	case "application/x-rar-compressed", "application/vnd.rar":
		return ".rar", false
	case "application/x-tar":
		return ".tar", false
	case "application/gzip", "application/x-gzip":
		return ".gz", false
	case "application/json":
		return ".json", true
	case "application/xml", "text/xml":
		return ".xml", true
	case "application/javascript", "text/javascript":
		return ".js", true
	case "application/x-httpd-php":
		return ".php", true
	}
	if strings.HasPrefix(ct, "text/") || ct == "application/xhtml+xml" {
		if ct == "text/html" {
			return ".html", true
		}
		if ct == "text/css" {
			return ".css", true
		}
		if ct == "text/csv" {
			return ".csv", true
		}
		if ct == "text/plain" {
			return ".txt", true
		}
		// 其余 text/* 一律按后缀名存 .txt
		return ".txt", true
	}
	// 3) URL 后缀
	u, err := url.Parse(urlPath)
	if err == nil {
		ext := strings.ToLower(path.Ext(u.Path))
		if ext != "" && len(ext) <= 8 && regexp.MustCompile(`^\.[A-Za-z0-9]+$`).MatchString(ext) {
			return ext, false
		}
	}
	// 4) 内容看起来是文本？UTF-8 可解码且可打印比例高
	if looksLikeText(head) {
		return ".txt", true
	}
	return ".bin", false
}

func looksLikeText(head []byte) bool {
	if len(head) == 0 {
		return false
	}
	s := head
	if len(s) > 512 {
		s = s[:512]
	}
	printable := 0
	for _, b := range s {
		if b == '\n' || b == '\r' || b == '\t' || (b >= 0x20 && b < 0x7F) || b >= 0x80 {
			printable++
		}
	}
	return printable*10 >= len(s)*9
}

// ---------------------------------------------------------------------------
// URL 提取（深度 1 链接下载）
// ---------------------------------------------------------------------------

var absURLRe = regexp.MustCompile(`https?://[^\s"'<>]+`)
var protoRelRe = regexp.MustCompile(`//[^\s"'<>]+`)
var attrRefRe = regexp.MustCompile(`(?i)\b(?:src|href|action|data-src)\s*=\s*("[^"]*"|'[^']*'|[^\s"'<>]+)`)

// extractURLs 从文本响应中提取 URL：绝对、协议相对、以及 src/href 属性中的
// 相对路径（按 parentURL 解析）。只保留 http/https。
func extractURLs(body []byte, parentURL string) []string {
	base, _ := url.Parse(parentURL)
	text := strings.ReplaceAll(string(body), `\/`, "/")
	seen := map[string]bool{}
	var out []string
	add := func(u string) {
		u = strings.TrimRight(u, `.,;:!?)]}'"`)
		if len(u) > 8 && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	for _, m := range absURLRe.FindAllString(text, -1) {
		add(m)
	}
	for _, m := range protoRelRe.FindAllString(text, -1) {
		// 跳过绝对 URL 已覆盖的 //host/path（如 https://h/p 内的 //h/p 会被误匹配）
		rest := strings.TrimPrefix(m, "//")
		if seen["http://"+rest] || seen["https://"+rest] {
			continue
		}
		scheme := "http"
		if base != nil && base.Scheme != "" {
			scheme = base.Scheme // 协议相对按父协议补全（浏览器同款行为）
		}
		add(scheme + ":" + m)
	}
	for _, m := range attrRefRe.FindAllStringSubmatch(text, -1) {
		ref := strings.Trim(strings.Trim(m[1], `"'`), " \t")
		if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "//") ||
			strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "javascript:") ||
			strings.HasPrefix(ref, "mailto:") {
			continue
		}
		u, err := url.Parse(ref)
		if err != nil {
			continue
		}
		if base != nil && base.Scheme != "" {
			u = base.ResolveReference(u)
		}
		if u.Scheme == "http" || u.Scheme == "https" {
			add(u.String())
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// 主流程
// ---------------------------------------------------------------------------

// crawlEntry 一条请求记录（按 idx 排序后写报告）。
type crawlEntry struct {
	idx         int
	kind        string // 占位符 / 链接
	detail      string // 占位符取值描述（如 id=42）
	url         string
	status      int
	contentType string
	size        int
	savedName   string
	errMsg      string
}

func (e *crawlEntry) line() string {
	if e.errMsg != "" {
		return fmt.Sprintf("[%05d] %s %s 失败: %s", e.idx, e.kind, e.url, e.errMsg)
	}
	where := ""
	if e.detail != "" {
		where = " (" + e.detail + ")"
	}
	return fmt.Sprintf("[%05d] %s%s status=%d type=%s size=%d → %s",
		e.idx, e.url, where, e.status, e.contentType, e.size, e.savedName)
}

const maxBodyBytes = 64 << 20 // 单响应最多读 64MB

func packetCrawlerRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	raw := pcStr(params, "raw_request")
	if strings.TrimSpace(raw) == "" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少原始 HTTP 请求包"}, nil
	}
	pr0, err := parseRawRequest(raw)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	scheme := strings.ToLower(strings.TrimSpace(pcStr(params, "scheme")))
	if scheme != "http" && scheme != "https" {
		scheme = "http"
	}

	// —— 占位符定义与组合展开 ——
	specs, err := parsePlaceholderSpecs(pcStr(params, "placeholders"))
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	maxRequests := pcInt(params, "max_requests", 2000)
	if maxRequests < 1 {
		maxRequests = 1
	}
	combs, combErr := expandCombinations(raw, specs, maxRequests)
	if combErr != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: combErr.Error()}, nil
	}
	warning := ""
	if unused := unusedSpecNames(raw, specs); len(unused) > 0 {
		warning = "占位符已定义但请求包中未使用: " + strings.Join(unused, ", ")
	}

	concurrency := pcInt(params, "concurrency", 5)
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 50 {
		concurrency = 50
	}
	reqTimeout := time.Duration(pcInt(params, "timeout", 15)) * time.Second
	if reqTimeout <= 0 {
		reqTimeout = 15 * time.Second
	}
	delay := time.Duration(pcInt(params, "delay_ms", 0)) * time.Millisecond
	followLinks := pcBool(params, "follow_links", true)

	// —— 通用 HTTP 客户端 ——
	client := &http.Client{
		Timeout: reqTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           http.ProxyFromEnvironment,
		},
	}

	// —— 工作目录 ——
	if tc == nil || tc.ArtifactsDir == "" {
		return nil, fmt.Errorf("产物目录不可用")
	}
	saveDir := filepath.Join(tc.ArtifactsDir, "crawl")
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建保存目录失败: %v", err)
	}

	// —— 发送单请求并保存（返回 entry 与最终 URL）——
	type saved struct {
		entry crawlEntry
	}
	sendOne := func(idx int, kind, detail, targetURL string, reqBuilder func() (*http.Request, error), fetched map[string]bool, fetchedMu *sync.Mutex) saved {
		entry := crawlEntry{idx: idx, kind: kind, detail: detail, url: targetURL}
		if ctx.Err() != nil {
			entry.errMsg = "已取消"
			return saved{entry: entry}
		}
		req, err := reqBuilder()
		if err != nil {
			entry.errMsg = err.Error()
			return saved{entry: entry}
		}
		resp, err := client.Do(req)
		if err != nil {
			entry.errMsg = err.Error()
			return saved{entry: entry}
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		if err != nil {
			entry.errMsg = "读取响应失败: " + err.Error()
			return saved{entry: entry}
		}
		entry.status = resp.StatusCode
		entry.contentType = resp.Header.Get("Content-Type")
		entry.size = len(body)
		if len(body) > 0 {
			ext, isText := sniffExt(body, entry.contentType, targetURL)
			stem := fileStem(targetURL)
			// 保留原始文件名（不做序号重命名）；重名时原子追加 _1/_2 后缀，绝不覆盖
			name, f, err := createUniqueFile(saveDir, stem, ext)
			if err != nil {
				entry.errMsg = "保存文件失败: " + err.Error()
				return saved{entry: entry}
			}
			if _, err := f.Write(body); err != nil {
				f.Close()
				entry.errMsg = "保存文件失败: " + err.Error()
				return saved{entry: entry}
			}
			if err := f.Close(); err != nil {
				entry.errMsg = "保存文件失败: " + err.Error()
				return saved{entry: entry}
			}
			entry.savedName = name
			if followLinks && isText {
				links := extractURLs(body, req.URL.String())
				fetchedMu.Lock()
				for _, u := range links {
					if len(fetched) < 10000 && !fetched[u] {
						fetched[u] = false // 占位，稍后由链接阶段补发
					}
				}
				fetchedMu.Unlock()
			}
		} else {
			entry.savedName = "(空响应,未保存)"
		}
		return saved{entry: entry}
	}

	// —— 阶段一：占位符批量发包 ——
	fetched := map[string]bool{}
	var fetchedMu sync.Mutex
	entries := make([]crawlEntry, len(combs))
	var wg sync.WaitGroup
	jobCh := make(chan int)
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobCh {
				if ctx.Err() != nil {
					continue
				}
				if delay > 0 {
					select {
					case <-ctx.Done():
					case <-time.After(delay):
					}
				}
				values := combs[i]
				subRaw := substitute(raw, values)
				pr, err := parseRawRequest(subRaw)
				var u string
				if err == nil {
					u = pr.target
					if !strings.Contains(u, "://") {
						host := pr.headers["Host"]
						u = scheme + "://" + host + u
					}
				}
				detail := ""
				if len(values) > 0 {
					parts := make([]string, 0, len(values))
					for _, n := range usedPlaceholderNames(raw) {
						parts = append(parts, n+"="+values[n])
					}
					detail = strings.Join(parts, " ")
				}
				fetchedMu.Lock()
				fetched[u] = true
				fetchedMu.Unlock()
				res := sendOne(i, "请求", detail, u, func() (*http.Request, error) {
					pr, err := parseRawRequest(subRaw)
					if err != nil {
						return nil, err
					}
					return buildHTTPRequest(pr, scheme)
				}, fetched, &fetchedMu)
				entries[i] = res.entry
			}
		}()
	}
	for i := range combs {
		jobCh <- i
	}
	close(jobCh)
	wg.Wait()

	// —— 阶段二：链接下载（深度 1，不递归）——
	if followLinks && ctx.Err() == nil {
		var links []string
		fetchedMu.Lock()
		for u, done := range fetched {
			if !done {
				links = append(links, u)
			}
		}
		fetchedMu.Unlock()
		sort.Strings(links)
		if len(links) > 0 {
			linkEntries := make([]crawlEntry, len(links))
			linkCh := make(chan int)
			var lwg sync.WaitGroup
			for w := 0; w < concurrency; w++ {
				lwg.Add(1)
				go func() {
					defer lwg.Done()
					for j := range linkCh {
						if ctx.Err() != nil {
							continue
						}
						if delay > 0 {
							select {
							case <-ctx.Done():
							case <-time.After(delay):
							}
						}
						u := links[j]
						fetchedMu.Lock()
						fetched[u] = true
						fetchedMu.Unlock()
						res := sendOne(len(combs)+j, "链接", "", u, func() (*http.Request, error) {
							req, err := http.NewRequest(http.MethodGet, u, nil)
							if err != nil {
								return nil, err
							}
							// 继承原始包的 UA / Cookie / Referer
							for _, h := range []string{"User-Agent", "Cookie", "Referer", "Authorization"} {
								if v := pr0.headers[h]; v != "" {
									req.Header.Set(h, v)
								}
							}
							return req, nil
						}, fetched, &fetchedMu)
						linkEntries[j] = res.entry
					}
				}()
			}
			for j := range links {
				linkCh <- j
			}
			close(linkCh)
			lwg.Wait()
			entries = append(entries, linkEntries...)
		}
	}

	// —— 汇总 + 报告 ——
	sort.Slice(entries, func(i, j int) bool { return entries[i].idx < entries[j].idx })
	okCount := 0
	failCount := 0
	for _, e := range entries {
		if e.errMsg == "" && e.status > 0 {
			okCount++
		} else {
			failCount++
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "[+] 解析原始包: %s %s\n", pr0.method, pr0.target)
	if len(combs) == 1 && usedPlaceholderNames(raw) == nil {
		fmt.Fprintf(&sb, "[+] 无占位符,原样发送 1 次\n")
	} else {
		fmt.Fprintf(&sb, "[+] 占位符组合: %d 个请求,并发 %d\n", len(combs), concurrency)
	}
	if warning != "" {
		fmt.Fprintf(&sb, "[!] %s\n", warning)
	}
	fmt.Fprintf(&sb, "[+] 保存目录: %s\n", saveDir)
	fmt.Fprintf(&sb, "[+] 汇总: 成功 %d / 失败 %d\n", okCount, failCount)

	// 报告文件
	reportPath := filepath.Join(tc.ArtifactsDir, "crawl_report.txt")
	{
		var rb strings.Builder
		for _, e := range entries {
			rb.WriteString(e.line())
			rb.WriteString("\n")
		}
		fmt.Fprintf(&rb, "—— 汇总: 成功 %d / 失败 %d / 总计 %d ——\n", okCount, failCount, len(entries))
		if err := os.WriteFile(reportPath, []byte(rb.String()), 0o644); err == nil {
			fmt.Fprintf(&sb, "[+] 明细报告: crawl_report.txt\n")
		}
		// stdout 只贴前 20 行明细,避免刷屏
		for i, e := range entries {
			if i >= 20 {
				fmt.Fprintf(&sb, "… (其余 %d 行见 crawl_report.txt)\n", len(entries)-20)
				break
			}
			sb.WriteString(e.line())
			sb.WriteString("\n")
		}
	}

	// 打包 result.zip（crawl/ 目录 + 报告）
	zipPath := filepath.Join(tc.ArtifactsDir, "result.zip")
	if n, err := zipResults(zipPath, saveDir, reportPath); err == nil {
		fmt.Fprintf(&sb, "[+] 打包产物: result.zip (%d 个文件)\n", n)
	} else {
		fmt.Fprintf(&sb, "[!] 打包 result.zip 失败: %v（文件仍在 crawl/ 目录）\n", err)
	}

	res := &core.ToolResult{Status: core.StatusSuccess, Stdout: sb.String()}
	if warning != "" {
		res.Stderr = warning
	}
	if okCount == 0 && len(entries) > 0 {
		res.Status = core.StatusError
		res.Stderr = "所有请求均失败,详见报告"
	}
	return res, nil
}

// fileStem 从 URL 取文件名主干（去掉查询串与非法字符）。
func fileStem(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return "resp"
	}
	base := path.Base(parsed.Path)
	if base == "" || base == "/" || base == "." {
		return "resp"
	}
	base = strings.TrimSuffix(base, path.Ext(base))
	var b strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' || r > 127 {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	s := b.String()
	if s == "" {
		s = "resp"
	}
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

// createUniqueFile 在 dir 下以原始文件名创建新文件：名称已存在时依次追加
// _1、_2… 后缀，绝不覆盖已有文件。O_EXCL 原子创建，并发调用安全。
func createUniqueFile(dir, stem, ext string) (string, *os.File, error) {
	for i := 0; i < 10000; i++ {
		name := stem + ext
		if i > 0 {
			name = fmt.Sprintf("%s_%d%s", stem, i, ext)
		}
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			return name, f, nil
		}
		if !os.IsExist(err) {
			return "", nil, err
		}
	}
	return "", nil, fmt.Errorf("重名过多，无法分配文件名")
}

// zipResults 把 saveDir 下所有文件 + reportPath 打包为 result.zip。
func zipResults(zipPath, saveDir, reportPath string) (int, error) {
	out, err := os.Create(zipPath)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()
	count := 0
	add := func(srcPath, zipName string) error {
		src, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer src.Close()
		w, err := zw.Create(zipName)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, src); err != nil {
			return err
		}
		count++
		return nil
	}
	entries, err := os.ReadDir(saveDir)
	if err != nil {
		return 0, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := add(filepath.Join(saveDir, e.Name()), "crawl/"+e.Name()); err != nil {
			return count, err
		}
	}
	if err := add(reportPath, "crawl_report.txt"); err != nil {
		return count, err
	}
	return count, nil
}
