// data.convert.table-merge —— 表格合并（CSV / Excel 按指定列连接）。
//
// 上传两个 CSV（UTF-8 / UTF-8 BOM / UTF-16LE / GBK 自动识别，分隔符 , ; \t
// 自动嗅探）或 Excel .xlsx（取第一个工作表，单元格取显示值）文件，按指定
// 列做内 / 左 / 右 / 全连接：
//   - 键值去首尾空白后匹配；键重复时做笛卡尔积（数据库 JOIN 语义）；
//   - 右表键列与左表键列同名时只保留一列，异名时两列都保留；
//   - 非键列重名时右表列自动加 _b / _b2 后缀。
//
// stdout 输出 JSON（summary + 列名 + 合并结果预览，最多 20000 行）供前端
// 渲染器消费；产物目录写出 merged.csv（带 UTF-8 BOM，Excel 直接打开中文
// 不乱码）与 merge_report.json。
package data

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"ctfnet/internal/core"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func init() {
	core.DefaultRegistry.Register(&TableMergeTool{BaseTool: core.BaseTool{
		NameAttr:        "table-merge",
		CategoryAttr:    core.Category("data.convert"),
		VersionAttr:     "1.0.0",
		DescriptionAttr: "按指定列合并两个 CSV/Excel 表格（inner/left/right/outer 连接）",
		TimeoutAttr:     300,
		ParamsAttr: []core.ParamSpec{
			{Name: "file_a", Type: "file", Required: true, Description: "左侧表（CSV 或 Excel .xlsx，取第一个工作表）"},
			{Name: "file_b", Type: "file", Required: true, Description: "右侧表（CSV 或 Excel .xlsx，取第一个工作表）"},
			{Name: "key_a", Type: "text", Required: true, Description: "左表用于合并的列名"},
			{Name: "key_b", Type: "text", Required: false, Description: "右表用于合并的列名（缺省与 key_a 同名）"},
			{Name: "how", Type: "str", Required: false, Description: "合并方式：inner 内连接（缺省）/ left 左连接 / right 右连接 / outer 全连接"},
		},
		Runner: tableMergeRun,
	}})
}

// TableMergeTool 表格合并工具。
type TableMergeTool struct {
	core.BaseTool
}

const (
	maxTableRows   = 100000 // 单表数据行上限
	maxResultRows  = 500000 // 合并结果行上限（防键重复笛卡尔爆炸）
	previewCapRows = 20000  // stdout 内嵌预览行上限，完整结果在 merged.csv
)

// mergeTable 一张已解析的表。
type mergeTable struct {
	name    string
	meta    string // 如 "CSV(UTF-8, 分隔符 ',')" / "XLSX(Sheet1)"
	headers []string
	rows    [][]string
}

// ---------------------------------------------------------------------------
// 文件解析
// ---------------------------------------------------------------------------

// decodeTextBytes 文本字节流 → 字符串 + 编码描述。
// 依次识别 UTF-8 BOM / UTF-16LE BOM / 合法 UTF-8 / GBK，最后兜底原样返回。
func decodeTextBytes(b []byte) (string, string) {
	if bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		return string(b[3:]), "UTF-8(BOM)"
	}
	if bytes.HasPrefix(b, []byte{0xFF, 0xFE}) {
		d := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
		if s, _, err := transform.String(d, string(b[2:])); err == nil {
			return s, "UTF-16LE"
		}
	}
	if utf8.Valid(b) {
		return string(b), "UTF-8"
	}
	if d := simplifiedchinese.GBK.NewDecoder(); d != nil {
		if s, _, err := transform.String(d, string(b)); err == nil {
			return s, "GBK"
		}
	}
	return string(b), "原始字节"
}

// detectDelimiter 在候选分隔符中选能切出最多字段的那个。
func detectDelimiter(firstLine string) rune {
	best, bestN := ',', -1
	for _, r := range []rune{',', ';', '\t'} {
		rdr := csv.NewReader(strings.NewReader(firstLine))
		rdr.Comma = r
		if rec, err := rdr.Read(); err == nil && len(rec) > bestN {
			best, bestN = r, len(rec)
		}
	}
	return best
}

// normHeader 规范化表头：去 BOM 与首尾空白，空表头补占位名。
func normHeader(rec []string) []string {
	out := make([]string, len(rec))
	for i, h := range rec {
		h = strings.TrimPrefix(h, "\\uFEFF")
		h = strings.TrimSpace(h)
		if h == "" {
			h = fmt.Sprintf("列%d", i+1)
		}
		out[i] = h
	}
	return out
}

func allEmptyCells(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// padRow 把行对齐到表头列数（短补空、长截断）。
func padRow(rec []string, n int) []string {
	out := make([]string, n)
	copy(out, rec)
	return out
}

func loadCSVTable(path string) (*mergeTable, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}
	text, enc := decodeTextBytes(b)
	first := ""
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) != "" {
			first = l
			break
		}
	}
	if first == "" {
		return nil, fmt.Errorf("CSV 内容为空")
	}
	delim := detectDelimiter(first)
	rdr := csv.NewReader(strings.NewReader(text))
	rdr.Comma = delim
	rdr.LazyQuotes = true
	rdr.FieldsPerRecord = -1
	recs, err := rdr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV 解析失败: %v", err)
	}
	// 跳过开头空行取表头
	hi := 0
	for hi < len(recs) && allEmptyCells(recs[hi]) {
		hi++
	}
	if hi >= len(recs) {
		return nil, fmt.Errorf("CSV 内容为空")
	}
	headers := normHeader(recs[hi])
	t := &mergeTable{name: filepath.Base(path), meta: fmt.Sprintf("CSV(%s, 分隔符 %q)", enc, delim), headers: headers}
	for _, r := range recs[hi+1:] {
		if allEmptyCells(r) {
			continue
		}
		t.rows = append(t.rows, padRow(r, len(headers)))
		if len(t.rows) > maxTableRows {
			return nil, fmt.Errorf("数据行超过 %d 行上限", maxTableRows)
		}
	}
	return t, nil
}

func loadXLSXTable(path string) (*mergeTable, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("Excel 打开失败: %v", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("工作簿不含任何工作表")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("Excel 读取失败: %v", err)
	}
	hi := 0
	for hi < len(rows) && allEmptyCells(rows[hi]) {
		hi++
	}
	if hi >= len(rows) {
		return nil, fmt.Errorf("工作表 %q 为空", sheets[0])
	}
	headers := normHeader(rows[hi])
	t := &mergeTable{name: filepath.Base(path), meta: fmt.Sprintf("XLSX(%s)", sheets[0]), headers: headers}
	for _, r := range rows[hi+1:] {
		if allEmptyCells(r) {
			continue
		}
		t.rows = append(t.rows, padRow(r, len(headers)))
		if len(t.rows) > maxTableRows {
			return nil, fmt.Errorf("数据行超过 %d 行上限", maxTableRows)
		}
	}
	return t, nil
}

// loadMergeTable 按扩展名分派解析。
func loadMergeTable(path string) (*mergeTable, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv", ".tsv", ".txt":
		return loadCSVTable(path)
	case ".xlsx":
		return loadXLSXTable(path)
	case ".xls":
		return nil, fmt.Errorf("旧版 .xls 暂不支持，请在 Excel 中另存为 .xlsx 后上传")
	}
	return nil, fmt.Errorf("不支持的文件类型 %q（支持 .csv/.tsv/.txt/.xlsx）", filepath.Ext(path))
}

// ---------------------------------------------------------------------------
// 合并逻辑
// ---------------------------------------------------------------------------

// keyIndex 按列名找下标（先精确匹配，再忽略大小写）。
func keyIndex(headers []string, key string) (int, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return -1, fmt.Errorf("合并列名不能为空")
	}
	for i, h := range headers {
		if h == key {
			return i, nil
		}
	}
	for i, h := range headers {
		if strings.EqualFold(h, key) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("找不到列 %q，可用列：%s", key, strings.Join(headers, " / "))
}

// outCol 输出列映射：aIdx / bIdx 二者之一 >=0。
type outCol struct {
	name string
	aIdx int
	bIdx int
}

// buildOutCols 决定输出列布局与重名后缀规则。
func buildOutCols(a, b *mergeTable, keyAIdx, keyBIdx int) []outCol {
	cols := make([]outCol, 0, len(a.headers)+len(b.headers))
	for i, h := range a.headers {
		cols = append(cols, outCol{name: h, aIdx: i, bIdx: -1})
	}
	dropRightKey := strings.EqualFold(a.headers[keyAIdx], b.headers[keyBIdx])
	used := map[string]bool{}
	for _, h := range a.headers {
		used[strings.ToLower(h)] = true
	}
	for j, h := range b.headers {
		if dropRightKey && j == keyBIdx {
			continue
		}
		name := h
		if used[strings.ToLower(name)] {
			base := name + "_b"
			n := 2
			for used[strings.ToLower(base)] {
				base = fmt.Sprintf("%s_b%d", name, n)
				n++
			}
			name = base
		}
		used[strings.ToLower(name)] = true
		cols = append(cols, outCol{name: name, aIdx: -1, bIdx: j})
	}
	return cols
}

// mergeRows 执行连接，返回结果行与统计（匹配对数、仅左行数、仅右行数）。
func mergeRows(a, b *mergeTable, keyAIdx, keyBIdx int, how string, cols []outCol) ([][]string, int, int, int) {
	rightIdx := map[string][]int{}
	for i, r := range b.rows {
		k := strings.TrimSpace(r[keyBIdx])
		rightIdx[k] = append(rightIdx[k], i)
	}
	matchedRight := make([]bool, len(b.rows))
	build := func(l, r []string) []string {
		row := make([]string, len(cols))
		for i, c := range cols {
			switch {
			case c.aIdx >= 0 && l != nil:
				row[i] = l[c.aIdx]
			case c.bIdx >= 0 && r != nil:
				row[i] = r[c.bIdx]
			}
		}
		return row
	}
	var out [][]string
	matchedPairs, leftOnly := 0, 0
	for _, l := range a.rows {
		idxs := rightIdx[strings.TrimSpace(l[keyAIdx])]
		if len(idxs) == 0 {
			leftOnly++
			if how == "left" || how == "outer" {
				out = append(out, build(l, nil))
			}
			continue
		}
		matchedPairs += len(idxs)
		for _, i := range idxs {
			matchedRight[i] = true
			out = append(out, build(l, b.rows[i]))
		}
	}
	rightOnly := 0
	for _, m := range matchedRight {
		if !m {
			rightOnly++
		}
	}
	if how == "right" || how == "outer" {
		for i, r := range b.rows {
			if !matchedRight[i] {
				out = append(out, build(nil, r))
			}
		}
	}
	return out, matchedPairs, leftOnly, rightOnly
}

// ---------------------------------------------------------------------------
// 工具主流程
// ---------------------------------------------------------------------------

type mergeSummary struct {
	FileA        string `json:"file_a"`
	FileB        string `json:"file_b"`
	MetaA        string `json:"meta_a"`
	MetaB        string `json:"meta_b"`
	RowsA        int    `json:"rows_a"`
	RowsB        int    `json:"rows_b"`
	KeyA         string `json:"key_a"`
	KeyB         string `json:"key_b"`
	How          string `json:"how"`
	Columns      int    `json:"columns"`
	MatchedPairs int    `json:"matched_pairs"`
	LeftOnly     int    `json:"left_only"`
	RightOnly    int    `json:"right_only"`
	ResultRows   int    `json:"result_rows"`
	PreviewRows  int    `json:"preview_rows"`
}

type mergeOutput struct {
	Summary mergeSummary `json:"summary"`
	Columns []string     `json:"columns"`
	Preview [][]string   `json:"preview"`
	Notes   []string     `json:"notes,omitempty"`
}

// firstFileParam 取 file 参数解析结果（字符串或数组）的第一个文件。
func firstFileParam(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if ss, ok := asStringSliceLocal(v); ok && len(ss) > 0 {
		return ss[0]
	}
	return ""
}

func tableMergeRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	_ = ctx
	fileA := firstFileParam(params["file_a"])
	fileB := firstFileParam(params["file_b"])
	if fileA == "" || fileB == "" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "需要同时提供左表（file_a）与右表（file_b）"}, nil
	}
	keyA := strings.TrimSpace(fmt.Sprint(params["key_a"]))
	if keyA == "" || keyA == "<nil>" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "请指定左表合并列名（key_a）"}, nil
	}
	keyB := strings.TrimSpace(fmt.Sprint(params["key_b"]))
	if keyB == "" || keyB == "<nil>" {
		keyB = keyA
	}
	how := strings.TrimSpace(fmt.Sprint(params["how"]))
	if how == "" || how == "<nil>" {
		how = "inner"
	}
	switch how {
	case "inner", "left", "right", "outer":
	default:
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("不支持的合并方式 %q（可选 inner/left/right/outer）", how)}, nil
	}

	workdir := ""
	if tc != nil {
		workdir = tc.Workdir
	}
	a, err := loadMergeTable(filepath.Join(workdir, filepath.FromSlash(fileA)))
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("左表 %s：%v", fileA, err)}, nil
	}
	b, err := loadMergeTable(filepath.Join(workdir, filepath.FromSlash(fileB)))
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("右表 %s：%v", fileB, err)}, nil
	}
	keyAIdx, err := keyIndex(a.headers, keyA)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("左表：%v", err)}, nil
	}
	keyBIdx, err := keyIndex(b.headers, keyB)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("右表：%v", err)}, nil
	}

	cols := buildOutCols(a, b, keyAIdx, keyBIdx)
	rows, matchedPairs, leftOnly, rightOnly := mergeRows(a, b, keyAIdx, keyBIdx, how, cols)
	if len(rows) > maxResultRows {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("合并结果 %d 行超过 %d 行上限（合并列可能大量重复导致笛卡尔爆炸）", len(rows), maxResultRows)}, nil
	}

	colNames := make([]string, len(cols))
	for i, c := range cols {
		colNames[i] = c.name
	}
	notes := []string{}
	if how != "inner" {
		notes = append(notes, fmt.Sprintf("%s 连接：未匹配的行保留（左侧补空或右侧补空）", how))
	}
	preview := rows
	if len(preview) > previewCapRows {
		preview = preview[:previewCapRows]
		notes = append(notes, fmt.Sprintf("结果共 %d 行，界面预览前 %d 行；完整数据在产物 merged.csv 中", len(rows), previewCapRows))
	}
	out := mergeOutput{
		Summary: mergeSummary{
			FileA: fileA, FileB: fileB,
			MetaA: a.meta, MetaB: b.meta,
			RowsA: len(a.rows), RowsB: len(b.rows),
			KeyA: keyA, KeyB: keyB, How: how,
			Columns:      len(cols),
			MatchedPairs: matchedPairs,
			LeftOnly:     leftOnly,
			RightOnly:    rightOnly,
			ResultRows:   len(rows),
			PreviewRows:  len(preview),
		},
		Columns: colNames,
		Preview: preview,
		Notes:   notes,
	}
	if len(out.Preview) == 0 {
		out.Preview = [][]string{}
	}

	// 产物：merged.csv（UTF-8 BOM）与 merge_report.json
	if tc != nil && tc.ArtifactsDir != "" {
		if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("创建产物目录失败: %v", err)}, nil
		}
		var buf bytes.Buffer
		buf.WriteString("\xEF\xBB\xBF") // Excel 打开中文不乱码
		w := csv.NewWriter(&buf)
		if err := w.Write(colNames); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写合并结果失败: %v", err)}, nil
		}
		for _, r := range rows {
			if err := w.Write(r); err != nil {
				return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写合并结果失败: %v", err)}, nil
			}
		}
		w.Flush()
		if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "merged.csv"), buf.Bytes(), 0o644); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出 merged.csv 失败: %v", err)}, nil
		}
		report, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: "序列化报告失败"}, nil
		}
		if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "merge_report.json"), report, 0o644); err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("写出 merge_report.json 失败: %v", err)}, nil
		}
	}

	compact, err := json.Marshal(out)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "序列化结果失败"}, nil
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: string(compact)}, nil
}
