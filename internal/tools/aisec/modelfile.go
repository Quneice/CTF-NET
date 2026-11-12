// aisec.backdoor.model-file-scan —— 模型文件后门扫描(picklescan 思路的 Go 实现):
// 自动识别 PyTorch zip 存档 / pickle / safetensors,线性扫描 pickle 字节流收集
// GLOBAL 导入与字符串字面量,匹配危险函数对(os.system/subprocess/eval/exec 等)、
// 可疑字符串(URL/IP/长 base64/shell 命令)与 zip 内可疑成员(.so/.exe/.py 等)。
package aisec

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&ModelFileScanTool{BaseTool: core.BaseTool{
		NameAttr:        "model-file-scan",
		CategoryAttr:    CatBackdoor,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "模型文件后门扫描:自动识别 PyTorch zip/pickle/safetensors,检测 pickle 恶意导入(os.system/subprocess/eval/exec 等)、可疑字符串与 zip 内可疑成员,输出风险分级报告",
		TimeoutAttr:     300,
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "模型文件(.pt/.pth/.pkl/.bin/.safetensors,支持多文件上传)"},
		},
		Runner: modelFileScanRun,
	}})
}

// ModelFileScanTool 模型文件后门扫描工具。
type ModelFileScanTool struct {
	core.BaseTool
}

// fileParam 兼容单文件(string)与多文件([]any/[]string)两种 file 参数形态。
func fileParam(params map[string]any, key string) []string {
	switch v := params[key].(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []any:
		var out []string
		for _, e := range v {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	}
	return nil
}

// finding 单条发现。
type finding struct {
	Kind     string `json:"kind"`     // import / string / member / info
	Value    string `json:"value"`
	Severity string `json:"severity"` // dangerous / suspicious / info
	Where    string `json:"where"`    // 所在成员(顶层 pickle 为空)
}

// fileReport 单文件扫描报告。
type fileReport struct {
	File     string    `json:"file"`
	Kind     string    `json:"kind"` // zip / pickle / safetensors / unknown
	Size     int       `json:"size"`
	Risk     string    `json:"risk"` // high / medium / low
	Findings []finding `json:"findings"`
}

// —— 危险函数对(module -> name 集合) ——

var dangerousPairs = map[string]map[string]bool{
	"os":              {"system": true, "popen": true, "remove": true, "unlink": true, "rmdir": true, "removedirs": true, "execv": true, "execl": true, "spawnv": true},
	"subprocess":      {"Popen": true, "call": true, "run": true, "check_output": true, "check_call": true, "getoutput": true},
	"builtins":        {"eval": true, "exec": true, "compile": true, "__import__": true},
	"marshal":         {"loads": true},
	"pickle":          {"loads": true},
	"ctypes":          {"CDLL": true, "WinDLL": true, "PyDLL": true},
	"socket":          {"socket": true, "connect": true, "create_connection": true, "send": true, "recv": true},
	"requests":        {"get": true, "post": true},
	"urllib.request":  {"urlopen": true, "urlretrieve": true},
	"http.client":     {"HTTPSConnection": true, "HTTPConnection": true},
	"ftplib":          {"FTP": true},
	"smtplib":         {"SMTP": true, "SMTP_SSL": true},
	"shutil":          {"rmtree": true, "copy": true, "move": true},
	"base64":          {"b64decode": true},
	"pathlib":         {"unlink": true, "rmdir": true},
	"posix":           {"system": true},
	"subprocess_patch": {"Popen": true}, // 少见但保留防御
}

// bareDangerous 无模块名的裸危险名(如直接 GLOBAL eval)。
var bareDangerous = map[string]bool{"eval": true, "exec": true, "compile": true, "__import__": true}

// suspiciousModules 只要导入即算可疑的模块。
var suspiciousModules = map[string]bool{
	"socket": true, "subprocess": true, "requests": true, "urllib.request": true,
	"http.client": true, "ftplib": true, "smtplib": true, "ctypes": true, "winreg": true,
	"distutils": false, // 无害,显式排除
}

var (
	reURL   = regexp.MustCompile(`https?://[^\s"'<>]{6,}`)
	reIP    = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	reB64   = regexp.MustCompile(`[A-Za-z0-9+/]{200,}={0,2}`)
	reShell = regexp.MustCompile(`(?i)(/bin/(ba)?sh\b|cmd\.exe|powershell|curl -|wget |nc -[eln]|ncat |/dev/tcp/)`)
)

// —— pickle 线性扫描 ——

// scanPickle 线性扫描 pickle 字节流:收集 GLOBAL/STACK_GLOBAL 导入、字符串字面量,
// 统计 REDUCE/NEWOBJ 调用点。不做完整栈语义,专为后门特征检测设计。
func scanPickle(data []byte) (imports []string, literals []string, calls int) {
	i, n := 0, len(data)
	readLine := func() string {
		j := bytes.IndexByte(data[i:], '\n')
		if j < 0 {
			s := string(data[i:])
			i = n
			return s
		}
		s := string(data[i : i+j])
		i += j + 1
		return s
	}
	skip := func(k int) { i += k }

	for i < n {
		op := data[i]
		i++
		switch {
		// —— 协议 0 文本操作码 ——
		case op == '.':
			return imports, literals, calls // STOP
		case op == 'c': // GLOBAL: 两行 module\nname
			m, nm := readLine(), readLine()
			if m != "" || nm != "" {
				imports = append(imports, m+" "+nm)
			}
		case op == 'i' || op == 'o': // INST/OBJ: 两行(参数行 + 构造行,含 GLOBAL 数据)
			readLine()
			readLine()
		case op == 'S' || op == 'V': // STRING / UNICODE(带引号转义)
			if i < n {
				// 协议 0 字符串以引号定界:先跳过开引号,再扫到下一个未转义引号
				if data[i] == '\'' || data[i] == '"' {
					i++
				}
				j := i
				for j < n {
					if data[j] == '\\' {
						j += 2
						continue
					}
					if data[j] == '\'' || data[j] == '"' {
						break
					}
					j++
				}
				if j < n {
					literals = append(literals, string(data[i:j]))
					i = j + 1
				} else {
					i = n
				}
			}
		case op == 'T' || op == 'J': // BINSTRING(4 字节) / BININT(4 字节)
			if i+4 <= n {
				l := int(binary.LittleEndian.Uint32(data[i:]))
				i += 4
				if op == 'T' && i+l <= n {
					literals = append(literals, string(data[i:i+l]))
					i += l
				}
			}
		case op == 'U': // SHORT_BINSTRING(1 字节长度)
			if i+1 <= n {
				l := int(data[i])
				i++
				if i+l <= n {
					literals = append(literals, string(data[i:i+l]))
					i += l
				}
			}
		case op == 'R':
			calls++
		case op == 'p' || op == 'g' || op == 'I' || op == 'F' || op == 'l': // PUT/GET/INT/FLOAT/LONG: 十进制参数
			for i < n && data[i] != '\n' {
				i++
			}
			if i < n {
				i++
			}
		case op == 'K': // BININT1
			skip(1)
		case op == 'M': // BININT2
			skip(2)
		case op == '(' || op == ')' || op == ']' || op == '}' || op == 'a' ||
			op == 'b' || op == 'd' || op == 'e' || op == 'h' || op == 'q' ||
			op == 's' || op == 't' || op == 'u' || op == 'N' || op == 'x':
			// 无参操作码
		case op >= 0x80:
			switch op {
			case 0x80: // PROTO
				skip(1)
			case 0x81, 0x85, 0x86, 0x87, 0x88, 0x89, 0x90, 0x91, 0x92, 0x97, 0x98, 0x9a:
				// NEWOBJ / TUPLE1-3 / NEWTRUE / NEWFALSE / ADDITEMS / FROZENSET /
				// NEWOBJ_EX / NEXT_BUFFER / READONLY_BUFFER: 无参
				if op == 0x81 || op == 0x92 {
					calls++ // NEWOBJ / NEWOBJ_EX 也是调用点
				}
			case 0x82: // EXT1
				skip(1)
			case 0x83: // EXT2
				skip(2)
			case 0x84, 0x94: // EXT4 / MEMOIZE
				skip(4)
			case 0x8a: // LONG1
				if i+1 <= n {
					l := int(data[i])
					i += 1 + l
				}
			case 0x8b: // LONG4
				if i+4 <= n {
					l := int(binary.LittleEndian.Uint32(data[i:]))
					i += 4 + l
				}
			case 0x8c, 0x8f, 0x99: // SHORT_BINUNICODE / SHORT_BINBYTES / SHORT_BINBYTES(dup 保留)
				if i+1 <= n {
					l := int(data[i])
					i++
					if op == 0x8c && i+l <= n {
						literals = append(literals, string(data[i:i+l]))
					}
					i += l
				}
			case 0x8d, 0x8e, 0x96: // BINUNICODE8 / BINBYTES8 / BYTEARRAY8
				if i+8 <= n {
					l := int(binary.LittleEndian.Uint64(data[i:]))
					i += 8
					if op == 0x8d && i+l <= n {
						literals = append(literals, string(data[i:i+l]))
					}
					i += l
				}
			case 0x93: // STACK_GLOBAL: 两行
				m, nm := readLine(), readLine()
				if m != "" || nm != "" {
					imports = append(imports, m+" "+nm)
				}
			case 0x95: // FRAME
				skip(8)
			default:
				// 未知操作码:跳过 1 字节(保守继续,不中断)
			}
		default:
			// 其他未知可打印操作码:跳过 1 字节
		}
		if i > n {
			i = n
		}
		if len(imports) > 10000 || len(literals) > 10000 {
			break
		}
	}
	return imports, literals, calls
}

// classifyImports 把导入对分类为危险/可疑/普通。
func classifyImports(imports []string) []finding {
	var out []finding
	for _, imp := range imports {
		imp = strings.TrimSpace(imp)
		if imp == "" || imp == " " {
			continue
		}
		parts := strings.Fields(imp)
		module, name := "", ""
		if len(parts) >= 1 {
			module = parts[0]
		}
		if len(parts) >= 2 {
			name = parts[1]
		}
		sev := "info"
		if name == "" {
			if bareDangerous[module] {
				sev = "dangerous"
			} else if suspiciousModules[module] {
				sev = "suspicious"
			}
		} else if names, ok := dangerousPairs[module]; ok && names[name] {
			sev = "dangerous"
		} else if suspiciousModules[module] {
			sev = "suspicious"
		}
		out = append(out, finding{Kind: "import", Value: imp, Severity: sev})
	}
	sort.SliceStable(out, func(a, b int) bool {
		rank := map[string]int{"dangerous": 0, "suspicious": 1, "info": 2}
		return rank[out[a].Severity] < rank[out[b].Severity]
	})
	return out
}

// scanStrings 字符串字面量特征扫描。
func scanStrings(literals []string) []finding {
	var out []finding
	for _, s := range literals {
		switch {
		case reShell.MatchString(s) || (reB64.MatchString(s) && len(s) > 300):
			out = append(out, finding{Kind: "string", Value: truncate(s, 200), Severity: "dangerous"})
		case reURL.MatchString(s) || reIP.MatchString(s) || reB64.MatchString(s):
			out = append(out, finding{Kind: "string", Value: truncate(s, 200), Severity: "suspicious"})
		}
	}
	return out
}

// —— zip / safetensors ——

// suspiciousMemberExts zip 内可疑原生代码成员。
var suspiciousMemberExts = map[string]bool{
	".so": true, ".dll": true, ".exe": true, ".dylib": true,
	".py": true, ".pyc": true, ".sh": true, ".bat": true,
	".jar": true, ".class": true, ".elf": true, ".bin": true,
}

// scanZipArchive 扫描 zip(PyTorch 存档):逐成员递归扫 pickle,标记可疑成员。
func scanZipArchive(rep *fileReport, data []byte) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		rep.Findings = append(rep.Findings, finding{Kind: "info", Value: "zip 解析失败: " + err.Error(), Severity: "info"})
		return
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepathExt(f.Name))
		if suspiciousMemberExts[ext] {
			rep.Findings = append(rep.Findings, finding{Kind: "member", Value: f.Name, Severity: "suspicious"})
		}
		// 只深入扫描 pickle 形态的成员
		if ext != ".pkl" && ext != ".pt" && ext != ".pth" && f.Name != "data.pkl" && ext != ".bin" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		buf, _ := io.ReadAll(io.LimitReader(rc, 50<<20))
		rc.Close()
		if isPickleData(buf) {
			imports, literals, _ := scanPickle(buf)
			for _, fnd := range classifyImports(imports) {
				fnd.Where = f.Name
				rep.Findings = append(rep.Findings, fnd)
			}
			for _, fnd := range scanStrings(literals) {
				fnd.Where = f.Name
				rep.Findings = append(rep.Findings, fnd)
			}
		}
	}
}

// scanSafetensors safetensors:安全格式,仅报告结构与元数据。
func scanSafetensors(rep *fileReport, data []byte) {
	if len(data) < 8 {
		return
	}
	hlen := binary.LittleEndian.Uint64(data[:8])
	if hlen == 0 || hlen > 100<<20 || 8+int(hlen) > len(data) {
		rep.Findings = append(rep.Findings, finding{Kind: "info", Value: "safetensors 头异常", Severity: "suspicious"})
		return
	}
	var header map[string]any
	if err := json.Unmarshal(data[8:8+hlen], &header); err != nil {
		rep.Findings = append(rep.Findings, finding{Kind: "info", Value: "safetensors 头解析失败: " + err.Error(), Severity: "suspicious"})
		return
	}
	rep.Findings = append(rep.Findings, finding{Kind: "info", Value: fmt.Sprintf("张量数: %d", len(header)), Severity: "info"})
	if meta, ok := header["__metadata__"].(string); ok && meta != "" {
		rep.Findings = append(rep.Findings, finding{Kind: "info", Value: "元数据: " + truncate(meta, 300), Severity: "info"})
	}
}

// isPickleData 判定字节流是否为 pickle(协议 0 文本或协议 2+ 二进制头)。
func isPickleData(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	if data[0] == 0x80 && data[1] <= 5 {
		return true
	}
	c := data[0]
	return c == '(' || c == 'c' || c == 'g' || c == 'S' || c == 'V' || c == 'i'
}

// scanModelBytes 单文件字节流扫描。
func scanModelBytes(name string, data []byte) fileReport {
	rep := fileReport{File: name, Size: len(data), Risk: "low"}
	switch {
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		rep.Kind = "zip (PyTorch 存档)"
		scanZipArchive(&rep, data)
	case len(data) >= 8 && binary.LittleEndian.Uint64(data[:8]) != 0 && len(data) >= 8+int(binary.LittleEndian.Uint64(data[:8])) && data[8] == '{':
		rep.Kind = "safetensors"
		scanSafetensors(&rep, data)
	case isPickleData(data):
		rep.Kind = "pickle"
		imports, literals, calls := scanPickle(data)
		rep.Findings = append(rep.Findings, classifyImports(imports)...)
		rep.Findings = append(rep.Findings, scanStrings(literals)...)
		if calls > 0 {
			rep.Findings = append(rep.Findings, finding{Kind: "info", Value: fmt.Sprintf("REDUCE/NEWOBJ 调用点: %d", calls), Severity: "info"})
		}
	default:
		rep.Kind = "未知格式(仅做字符串特征扫描)"
		// 提取可打印字符串(≥6 字符)做兜底扫描
		var sb strings.Builder
		run := 0
		for _, b := range data {
			if b >= 0x20 && b < 0x7f {
				sb.WriteByte(b)
				run++
				continue
			}
			if run >= 6 {
				rep.Findings = append(rep.Findings, scanStrings([]string{sb.String()})...)
			}
			sb.Reset()
			run = 0
		}
	}
	// 汇总风险等级
	for _, f := range rep.Findings {
		if f.Severity == "dangerous" {
			rep.Risk = "high"
			break
		}
		if f.Severity == "suspicious" {
			rep.Risk = "medium"
		}
	}
	return rep
}

func modelFileScanRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	files := fileParam(params, "file")
	if len(files) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少必需参数: file"}, nil
	}
	workdir := ""
	if tc != nil {
		workdir = tc.Workdir
	}
	var out strings.Builder
	reports := make([]fileReport, 0, len(files))
	for idx, name := range files {
		select {
		case <-ctx.Done():
			return &core.ToolResult{Status: core.StatusError, Stderr: "已取消"}, nil
		default:
		}
		path := name
		if workdir != "" && !filepath.IsAbs(name) {
			path = filepath.Join(workdir, name)
		}
		data, err := readFileLimited(path, 200<<20)
		if err != nil {
			reports = append(reports, fileReport{File: name, Risk: "low", Findings: []finding{{Kind: "info", Value: "读取失败: " + err.Error(), Severity: "info"}}})
			fmt.Fprintf(&out, "✗ %s — 读取失败: %v\n\n", name, err)
			continue
		}
		rep := scanModelBytes(name, data)
		reports = append(reports, rep)
		fmt.Fprintf(&out, "%s — %s | 风险: %s\n", rep.File, rep.Kind, riskLabel(rep.Risk))
		for _, f := range rep.Findings {
			where := ""
			if f.Where != "" {
				where = " [" + f.Where + "]"
			}
			fmt.Fprintf(&out, "  [%s] %s: %s%s\n", f.Severity, f.Kind, f.Value, where)
		}
		fmt.Fprintln(&out)
		if tc.ReportProgress != nil {
			tc.ReportProgress((idx+1)*100/len(files), fmt.Sprintf("已扫描 %d/%d", idx+1, len(files)))
		}
	}
	fmt.Fprintf(&out, "\n说明: 风险分级仅基于静态特征,high 不必然等于恶意;建议结合训练来源与行为验证。\n")
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: out.String(), Data: map[string]any{"reports": reports}}, nil
}

func riskLabel(r string) string {
	switch r {
	case "high":
		return "高"
	case "medium":
		return "中"
	}
	return "低"
}

// 小工具函数(独立命名,避免与 pentest 包同名函数混淆语义)
func filepathExt(p string) string {
	i := strings.LastIndex(p, ".")
	if i < 0 {
		return ""
	}
	return p[i:]
}

// readFileLimited 读取文件(上限 maxBytes,超出部分截断,扫描不受影响)。
func readFileLimited(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > maxBytes {
		buf := make([]byte, maxBytes)
		n, _ := io.ReadFull(f, buf)
		return buf[:n], nil
	}
	return io.ReadAll(f)
}
