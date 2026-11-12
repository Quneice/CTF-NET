// 漏洞案例 md 解析：从复现案例的 markdown 中提取标题、CVE 编号、
// 原始 HTTP 请求包（fenced + 未围栏两遍）与 curl 命令。
//
// 提取规则（基于对 6199 个案例文件的抽样统计）：
//   - 请求行正则覆盖 Burp/浏览器复制格式的全部常见方法（含 WebDAV 方法）；
//   - fenced 代码块内的请求：从请求行起截到响应行（HTTP/1.x 200）为止；
//   - 未围栏的请求（159 个文件）：只收请求行 + 头部行（到首个空行止），不收 body，
//     避免把正文说明文字误当成 body；
//   - 同一案例内完全相同的请求去重（复现段与利用段常重复贴同一 payload）。
package vulnlib

import (
	"regexp"
	"strings"
)

const (
	maxPayloadsPerCase = 50  // 单案例最多保留的请求包数
	maxPayloadBytes    = 64 << 10 // 单个请求包最大字节数
)

var (
	titleRe = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	cveRe   = regexp.MustCompile(`CVE-\d{4}-\d{4,}`)
	// fenced 代码块（``` 围栏，容忍语言标注与 CRLF；(?m) 让 ^``` 匹配任意行首）
	fenceRe = regexp.MustCompile("(?sm)^```[^\\n]*\\r?\\n(.*?)^```")
	// 请求行：方法 + 目标 + HTTP/1.x（覆盖常见与 WebDAV 方法）
	reqLineRe = regexp.MustCompile(`(?m)^(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|TRACE|CONNECT|PROPFIND|PROPPATCH|REPORT|MKCOL|COPY|MOVE|LOCK|UNLOCK|SEARCH)\s+\S+\s+HTTP/1\.[01]`)
	// 响应行：用于截断同一代码块里请求后附带的响应
	respLineRe   = regexp.MustCompile(`(?m)^HTTP/1\.[01]\s+\d{3}`)
	headerLineRe = regexp.MustCompile(`^[A-Za-z0-9-]+:`)
	curlRe       = regexp.MustCompile(`(?m)^\s*(?:sudo\s+)?curl\b.*https?://.*`)
)

// ParsedMD 单个案例 md 的解析结果。
type ParsedMD struct {
	Title    string   // 首个一级标题（无则空）
	CVEs     []string // 按出现顺序去重
	Payloads []string // 原始 HTTP 请求包（去重）
	Curls    []string // 单行 curl 命令
}

// ParseVulnMD 解析案例 md 全文。
func ParseVulnMD(content string) ParsedMD {
	out := ParsedMD{}
	if m := titleRe.FindStringSubmatch(content); m != nil {
		out.Title = strings.TrimSpace(m[1])
	}
	seenCVE := map[string]bool{}
	for _, m := range cveRe.FindAllString(content, -1) {
		if !seenCVE[m] {
			seenCVE[m] = true
			out.CVEs = append(out.CVEs, m)
		}
	}

	seenPayload := map[string]bool{}
	addPayload := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || len(raw) > maxPayloadBytes {
			return
		}
		if seenPayload[raw] {
			return
		}
		if len(out.Payloads) >= maxPayloadsPerCase {
			return
		}
		seenPayload[raw] = true
		out.Payloads = append(out.Payloads, raw)
	}

	// 1) fenced 代码块：截取请求行 → 响应行之前
	for _, m := range fenceRe.FindAllStringSubmatch(content, -1) {
		block := m[1]
		for _, loc := range reqLineRe.FindAllStringIndex(block, -1) {
			seg := block[loc[0]:]
			if rm := respLineRe.FindStringIndex(seg); rm != nil {
				seg = seg[:rm[0]]
			}
			addPayload(seg)
		}
	}

	// 2) 未围栏文本：请求行 + 头部行（到首个空行止），不收 body
	bare := fenceRe.ReplaceAllString(content, "\n\n")
	for _, loc := range reqLineRe.FindAllStringIndex(bare, -1) {
		rest := bare[loc[0]:]
		lines := strings.Split(rest, "\n")
		var sb strings.Builder
		for i, line := range lines {
			line = strings.TrimRight(line, "\r")
			trimmed := strings.TrimSpace(line)
			if i == 0 {
				sb.WriteString(line)
				continue
			}
			if trimmed == "" {
				break // 头部结束
			}
			if !headerLineRe.MatchString(line) {
				if i == 1 {
					break // 第二行就不是头部 → 假阳性，丢弃
				}
				break
			}
			sb.WriteString("\n")
			sb.WriteString(line)
		}
		addPayload(sb.String())
	}

	// 3) curl 命令（单行）
	for _, m := range curlRe.FindAllString(content, -1) {
		cmd := strings.TrimSpace(m)
		if cmd != "" && len(out.Curls) < 20 {
			out.Curls = append(out.Curls, cmd)
		}
	}
	return out
}
