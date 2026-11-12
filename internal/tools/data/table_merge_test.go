package data

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// writeCSV 写一个 CSV 文件；gbk=true 时整体按 GBK 编码（模拟国内 Excel 导出）。
func writeCSV(t *testing.T, path string, recs [][]string, gbk bool) {
	t.Helper()
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	for _, r := range recs {
		if err := w.Write(r); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	b := buf.Bytes()
	if gbk {
		enc, err := simplifiedchinese.GBK.NewEncoder().Bytes(b)
		if err != nil {
			t.Fatal(err)
		}
		b = enc
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// loadResult 用工具级解析入口读表并断言行数。
func loadResult(t *testing.T, path string) *mergeTable {
	t.Helper()
	tb, err := loadMergeTable(path)
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

func TestCSVInnerJoin(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, filepath.Join(dir, "a.csv"), [][]string{
		{"id", "name"}, {"1", "Alice"}, {"2", "Bob"}, {"3", "Cara"},
	}, false)
	writeCSV(t, filepath.Join(dir, "b.csv"), [][]string{
		{"id", "score"}, {"2", "88"}, {"3", "92"}, {"4", "77"},
	}, false)
	a := loadResult(t, filepath.Join(dir, "a.csv"))
	b := loadResult(t, filepath.Join(dir, "b.csv"))
	keyA, _ := keyIndex(a.headers, "id")
	keyB, _ := keyIndex(b.headers, "id")
	cols := buildOutCols(a, b, keyA, keyB)
	rows, matched, leftOnly, rightOnly := mergeRows(a, b, keyA, keyB, "inner", cols)
	if len(rows) != 2 || matched != 2 || leftOnly != 1 || rightOnly != 1 {
		t.Fatalf("inner: rows=%d matched=%d leftOnly=%d rightOnly=%d", len(rows), matched, leftOnly, rightOnly)
	}
	if got := strings.Join(colNames(cols), ","); got != "id,name,score" {
		t.Errorf("列布局 = %s", got)
	}
	if rows[0][0] != "2" || rows[0][2] != "88" || rows[1][0] != "3" || rows[1][2] != "92" {
		t.Errorf("行内容 = %v", rows)
	}
}

func TestCSVOuterJoins(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, filepath.Join(dir, "a.csv"), [][]string{{"id", "v"}, {"1", "a"}, {"2", "b"}}, false)
	writeCSV(t, filepath.Join(dir, "b.csv"), [][]string{{"id", "v"}, {"2", "x"}, {"3", "y"}}, false)
	a := loadResult(t, filepath.Join(dir, "a.csv"))
	b := loadResult(t, filepath.Join(dir, "b.csv"))
	keyA, _ := keyIndex(a.headers, "id")
	keyB, _ := keyIndex(b.headers, "id")
	cols := buildOutCols(a, b, keyA, keyB)
	// 左右未匹配统计与连接方式无关（inner/left/right/outer 下恒为 1/1），
	// 仅结果行数随方式变化
	for _, how := range []struct {
		name        string
		wantRows    int
		wantL, wantR int
	}{
		{"left", 2, 1, 1}, {"right", 2, 1, 1}, {"outer", 3, 1, 1},
	} {
		rows, _, l, r := mergeRows(a, b, keyA, keyB, how.name, cols)
		if len(rows) != how.wantRows || l != how.wantL || r != how.wantR {
			t.Errorf("%s: rows=%d leftOnly=%d rightOnly=%d，期望 %d/%d/%d", how.name, len(rows), l, r, how.wantRows, how.wantL, how.wantR)
		}
	}
}

func TestCSVGBKChineseHeaders(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, filepath.Join(dir, "a.csv"), [][]string{
		{"编号", "姓名"}, {"1", "张三"}, {"2", "李四"},
	}, true)
	writeCSV(t, filepath.Join(dir, "b.csv"), [][]string{
		{"编号", "分数"}, {"2", "95"},
	}, true)
	a := loadResult(t, filepath.Join(dir, "a.csv"))
	b := loadResult(t, filepath.Join(dir, "b.csv"))
	if a.headers[0] != "编号" || a.rows[0][1] != "张三" {
		t.Fatalf("GBK 解码失败: headers=%v row=%v", a.headers, a.rows[0])
	}
	keyA, err := keyIndex(a.headers, "编号")
	if err != nil {
		t.Fatal(err)
	}
	keyB, err := keyIndex(b.headers, "编号")
	if err != nil {
		t.Fatal(err)
	}
	rows, _, _, _ := mergeRows(a, b, keyA, keyB, "inner", buildOutCols(a, b, keyA, keyB))
	if len(rows) != 1 || rows[0][1] != "李四" || rows[0][2] != "95" {
		t.Fatalf("GBK 合并结果 = %v", rows)
	}
}

func TestXLSXJoin(t *testing.T) {
	dir := t.TempDir()
	xa := filepath.Join(dir, "a.xlsx")
	f := excelize.NewFile()
	_ = f.SetSheetRow("Sheet1", "A1", &[]interface{}{"id", "name"})
	_ = f.SetSheetRow("Sheet1", "A2", &[]interface{}{"1", "Alice"})
	_ = f.SetSheetRow("Sheet1", "A3", &[]interface{}{"2", "Bob"})
	if err := f.SaveAs(xa); err != nil {
		t.Fatal(err)
	}
	xb := filepath.Join(dir, "b.xlsx")
	f2 := excelize.NewFile()
	_ = f2.SetSheetRow("Sheet1", "A1", &[]interface{}{"id", "score"})
	_ = f2.SetSheetRow("Sheet1", "A2", &[]interface{}{"2", "88"})
	if err := f2.SaveAs(xb); err != nil {
		t.Fatal(err)
	}
	a := loadResult(t, xa)
	b := loadResult(t, xb)
	if a.meta != "XLSX(Sheet1)" || len(a.rows) != 2 {
		t.Fatalf("xlsx 解析异常: meta=%s rows=%d", a.meta, len(a.rows))
	}
	keyA, _ := keyIndex(a.headers, "id")
	keyB, _ := keyIndex(b.headers, "id")
	rows, matched, _, _ := mergeRows(a, b, keyA, keyB, "inner", buildOutCols(a, b, keyA, keyB))
	if len(rows) != 1 || matched != 1 || rows[0][2] != "88" {
		t.Fatalf("xlsx 合并结果 = %v matched=%d", rows, matched)
	}
}

func TestColumnNamingRules(t *testing.T) {
	dir := t.TempDir()
	// 右表键列名不同（编号 vs id）→ 两列都保留；name 列重名 → 右表加 _b
	writeCSV(t, filepath.Join(dir, "a.csv"), [][]string{{"id", "name"}, {"1", "张三"}}, false)
	writeCSV(t, filepath.Join(dir, "b.csv"), [][]string{{"编号", "name"}, {"1", "李四"}}, false)
	a := loadResult(t, filepath.Join(dir, "a.csv"))
	b := loadResult(t, filepath.Join(dir, "b.csv"))
	keyA, _ := keyIndex(a.headers, "id")
	keyB, _ := keyIndex(b.headers, "编号")
	cols := buildOutCols(a, b, keyA, keyB)
	if got := strings.Join(colNames(cols), ","); got != "id,name,编号,name_b" {
		t.Errorf("列布局 = %s，期望 id,name,编号,name_b", got)
	}
	rows, _, _, _ := mergeRows(a, b, keyA, keyB, "inner", cols)
	if rows[0][0] != "1" || rows[0][2] != "1" || rows[0][1] != "张三" || rows[0][3] != "李四" {
		t.Errorf("行内容 = %v", rows[0])
	}
}

func colNames(cols []outCol) []string {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	return names
}

func TestTableMergeEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeCSV(t, filepath.Join(dir, "a.csv"), [][]string{{"id", "name"}, {"1", "张三"}, {"2", "李四"}}, false)
	writeCSV(t, filepath.Join(dir, "b.csv"), [][]string{{"id", "score"}, {"2", "95"}}, false)
	art := dir + "_out"
	res, err := tableMergeRun(nil, map[string]any{
		"file_a": "a.csv", "file_b": "b.csv", "key_a": "id", "how": "left",
	}, &core.ToolContext{Workdir: dir, ArtifactsDir: art})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("状态 %v, stderr=%s", res.Status, res.Stderr)
	}
	var out mergeOutput
	if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
		t.Fatalf("stdout 不是合法 JSON: %v", err)
	}
	if out.Summary.ResultRows != 2 || out.Summary.LeftOnly != 1 || out.Summary.MatchedPairs != 1 {
		t.Fatalf("summary = %+v", out.Summary)
	}
	if len(out.Preview) != 2 {
		t.Fatalf("preview = %d 行", len(out.Preview))
	}
	// 产物：merged.csv 带 BOM，内容完整
	b, err := os.ReadFile(filepath.Join(art, "merged.csv"))
	if err != nil {
		t.Fatalf("merged.csv 缺失: %v", err)
	}
	if !bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		t.Errorf("merged.csv 缺少 UTF-8 BOM")
	}
	if !strings.Contains(string(b), "张三") || !strings.Contains(string(b), "95") {
		t.Errorf("merged.csv 内容不完整: %q", string(b))
	}
	if _, err := os.Stat(filepath.Join(art, "merge_report.json")); err != nil {
		t.Errorf("merge_report.json 缺失: %v", err)
	}
	// key_b 缺省跟随 key_a
	res2, err := tableMergeRun(nil, map[string]any{
		"file_a": "a.csv", "file_b": "b.csv", "key_a": "id",
	}, &core.ToolContext{Workdir: dir, ArtifactsDir: art + "2"})
	if err != nil || res2.Status != core.StatusSuccess {
		t.Fatalf("key_b 缺省失败: %v %s", err, res2.Stderr)
	}
	var out2 mergeOutput
	_ = json.Unmarshal([]byte(res2.Stdout), &out2)
	if out2.Summary.KeyB != "id" {
		t.Errorf("key_b 缺省应跟随 key_a，得到 %q", out2.Summary.KeyB)
	}
}
