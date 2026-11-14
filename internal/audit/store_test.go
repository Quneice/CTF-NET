// internal/audit 存储与扫描引擎测试:项目导入、文件树/读/写(带备份)、
// 全文搜索(plain/regex)、敏感函数扫描与规则库 CRUD、路径安全。
package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "audit.db"), filepath.Join(dir, "audits"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, dir
}

// 写临时源文件并返回 resolve 函数(id → 磁盘路径)。
func makeResolve(files map[string]string) func(string) (string, bool) {
	return func(id string) (string, bool) {
		p, ok := files[id]
		return p, ok
	}
}

func TestSeedRules(t *testing.T) {
	s, _ := openTestStore(t)
	rules, err := s.ListFuncs()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) < 30 {
		t.Fatalf("内置敏感函数规则应 ≥30 条,得到 %d", len(rules))
	}
	// 必须覆盖 PHP eval 与 Java 命令执行两类经典
	seen := map[string]bool{}
	for _, r := range rules {
		seen[r.Language+":"+r.Pattern] = true
	}
	if !seen["php:\\beval\\s*\\("] && !seen["php:\\beval("] {
		t.Log("php eval 规则缺失,检查种子")
	}
}

func TestImportAndTreeAndRead(t *testing.T) {
	s, dir := openTestStore(t)
	src1 := filepath.Join(dir, "a.php")
	os.WriteFile(src1, []byte("<?php eval($_GET['x']); ?>"), 0o644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	src2 := filepath.Join(dir, "sub", "b.py")
	os.WriteFile(src2, []byte("import os\nos.system('id')"), 0o644)

	p, err := s.Import("demo", []ImportFile{
		{ID: "f1", Name: "app/a.php"},
		{ID: "f2", Name: "app/sub/b.py"},
	}, makeResolve(map[string]string{"f1": src1, "f2": src2}))
	if err != nil {
		t.Fatal(err)
	}
	if p.Files != 2 {
		t.Fatalf("文件数错误: %d", p.Files)
	}

	tree, err := s.Tree(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, n := range tree {
		paths[n.Path] = true
	}
	if !paths["app/a.php"] || !paths["app/sub/b.py"] || !paths["app"] || !paths["app/sub"] {
		t.Fatalf("文件树不完整: %v", paths)
	}

	content, err := s.Read(p.ID, "app/a.php")
	if err != nil || !strings.Contains(content, "eval") {
		t.Fatalf("读取失败: %v %q", err, content)
	}
}

func TestWriteWithBackup(t *testing.T) {
	s, dir := openTestStore(t)
	src := filepath.Join(dir, "c.txt")
	os.WriteFile(src, []byte("original"), 0o644)
	p, _ := s.Import("w", []ImportFile{{ID: "x", Name: "c.txt"}}, makeResolve(map[string]string{"x": src}))

	if err := s.Write(p.ID, "c.txt", "edited-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(p.ID, "c.txt", "edited-2"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Read(p.ID, "c.txt")
	if got != "edited-2" {
		t.Fatalf("写入未生效: %q", got)
	}
	bak, err := os.ReadFile(filepath.Join(dir, "audits", p.ID, ".bak", "c.txt"))
	if err != nil || string(bak) != "original" {
		t.Fatalf("首次编辑应备份原始内容: %v %q", err, bak)
	}
}

func TestSearch(t *testing.T) {
	s, dir := openTestStore(t)
	src := filepath.Join(dir, "s.php")
	os.WriteFile(src, []byte("line1 safe\nline2 eval($_GET[0]);\nline3 EVAL upper\n"), 0o644)
	p, _ := s.Import("s", []ImportFile{{ID: "x", Name: "s.php"}}, makeResolve(map[string]string{"x": src}))

	// 大小写不敏感 plain
	hits, err := s.Search(p.ID, "eval", false, false, 100)
	if err != nil || len(hits) != 2 {
		t.Fatalf("plain 搜索应命中 2 行: %v %v", err, hits)
	}
	if hits[0].LineNo != 2 || !strings.Contains(hits[0].Line, "eval") {
		t.Fatalf("命中行错误: %+v", hits[0])
	}
	// 大小写敏感
	hits, _ = s.Search(p.ID, "eval", false, true, 100)
	if len(hits) != 1 {
		t.Fatalf("大小写敏感应命中 1 行: %v", hits)
	}
	// 正则
	hits, err = s.Search(p.ID, `eval\s*\(`, true, false, 100)
	if err != nil || len(hits) != 1 {
		t.Fatalf("regex 搜索失败: %v %v", err, hits)
	}
	// 非法正则报错
	if _, err := s.Search(p.ID, "((", true, false, 100); err == nil {
		t.Fatal("非法正则应报错")
	}
}

func TestScanSensitive(t *testing.T) {
	s, dir := openTestStore(t)
	src := filepath.Join(dir, "v.php")
	os.WriteFile(src, []byte("<?php\n$x = $_GET['c'];\neval($x);\nsystem('whoami');\n"), 0o644)
	p, _ := s.Import("v", []ImportFile{{ID: "x", Name: "v.php"}}, makeResolve(map[string]string{"x": src}))

	hits, err := s.Scan(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 2 {
		t.Fatalf("应至少命中 eval 与 system: %+v", hits)
	}
	found := map[string]bool{}
	for _, h := range hits {
		found[strings.TrimSuffix(strings.ToLower(h.Match), "(")] = true
		if h.LineNo == 0 || h.Category == "" || h.Risk == "" {
			t.Fatalf("命中缺字段: %+v", h)
		}
	}
	if !found["eval"] || !found["system"] {
		t.Fatalf("eval/system 未全部命中: %v", found)
	}
	// 命中行文本来自原文件
	for _, h := range hits {
		if h.LineNo == 3 && !strings.Contains(h.Line, "eval") {
			t.Fatalf("第 3 行上下文错误: %q", h.Line)
		}
	}
}

func TestSensitiveFuncCRUD(t *testing.T) {
	s, _ := openTestStore(t)
	f, err := s.AddFunc("php", `\bassert\s*\(`, "命令执行", "high", "自定义", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateFunc(f.ID, "php", `\bassert\s*\(`, "命令执行", "high", "改", false); err != nil {
		t.Fatal(err)
	}
	rules, _ := s.ListFuncs()
	var got *SensitiveFunc
	for _, r := range rules {
		if r.ID == f.ID {
			got = r
		}
	}
	if got == nil || got.Enabled || got.Note != "改" {
		t.Fatalf("更新未生效: %+v", got)
	}
	if err := s.DeleteFunc(f.ID); err != nil {
		t.Fatal(err)
	}
	rules, _ = s.ListFuncs()
	for _, r := range rules {
		if r.ID == f.ID {
			t.Fatal("删除未生效")
		}
	}
}

func TestPathSafetyAndBinary(t *testing.T) {
	s, dir := openTestStore(t)
	src := filepath.Join(dir, "ok.txt")
	os.WriteFile(src, []byte("ok"), 0o644)
	p, _ := s.Import("p", []ImportFile{{ID: "x", Name: "ok.txt"}}, makeResolve(map[string]string{"x": src}))

	if _, err := s.Read(p.ID, "../audit.db"); err == nil {
		t.Fatal("路径穿越应被拒绝")
	}
	if _, err := s.Read(p.ID, "/etc/passwd"); err == nil {
		t.Fatal("绝对路径应被拒绝")
	}
	if err := s.Write(p.ID, "../evil.txt", "x"); err == nil {
		t.Fatal("写入路径穿越应被拒绝")
	}
	// 二进制文件(含 NUL)拒绝读取
	bin := filepath.Join(dir, "b.bin")
	os.WriteFile(bin, append([]byte("ab\x00cd"), 1, 2, 3), 0o644)
	p2, _ := s.Import("b", []ImportFile{{ID: "y", Name: "b.bin"}}, makeResolve(map[string]string{"y": bin}))
	if _, err := s.Read(p2.ID, "b.bin"); err == nil {
		t.Fatal("二进制文件应拒绝读取")
	}
}

func TestDeleteProject(t *testing.T) {
	s, dir := openTestStore(t)
	src := filepath.Join(dir, "d.txt")
	os.WriteFile(src, []byte("x"), 0o644)
	p, _ := s.Import("d", []ImportFile{{ID: "x", Name: "d.txt"}}, makeResolve(map[string]string{"x": src}))
	if err := s.Delete(p.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := s.List()
	for _, x := range list {
		if x.ID == p.ID {
			t.Fatal("项目未删除")
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "audits", p.ID)); !os.IsNotExist(err) {
		t.Fatal("项目目录未清理")
	}
}

// JSON 契约:所有列表方法空结果必须是 [] 而非 null(nil 切片会序列化成 null,
// 前端 .find/.map 直接崩溃——2026-11-14 线上崩溃回归用例)。
func TestJSONNoNullSlices(t *testing.T) {
	s, _ := openTestStore(t)
	lst, err := s.List()
	if err != nil || lst == nil {
		t.Fatalf("List 空结果应为 [] 非 nil: %v %v", lst, err)
	}
	if b, _ := jsonMarshal(lst); string(b) != "[]" {
		t.Fatalf("List 序列化应为 [] 非 null: %s", b)
	}
	if got, _ := s.ListFuncs(); got == nil {
		t.Fatal("ListFuncs 不应返回 nil")
	}
	// 空命中场景
	dir := t.TempDir()
	src := filepath.Join(dir, "e.txt")
	os.WriteFile(src, []byte("nothing here"), 0o644)
	p, _ := s.Import("e", []ImportFile{{ID: "x", Name: "e.txt"}}, makeResolve(map[string]string{"x": src}))
	if got, err := s.Search(p.ID, "zzz_no_match", false, false, 10); err != nil || got == nil {
		t.Fatalf("Search 空命中应为 []: %v %v", got, err)
	}
	if got, err := s.Scan(p.ID); err != nil || got == nil {
		t.Fatalf("Scan 空命中应为 []: %v %v", got, err)
	}
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

// 文件树排序契约:DFS 连续序——目录的子项必须紧跟目录之后,
// 同层内目录在前、字母序;不能按深度扁平排序(否则子项与父目录被拆散)。
func TestTreeDFSOrder(t *testing.T) {
	s, dir := openTestStore(t)
	files := map[string]string{
		"z.txt":          "z",
		"app.txt":        "a",
		"app/main.py":    "m",
		"src/a/b.txt":    "b",
		"src/z.py":       "z",
		"app/util/u1.go": "u",
	}
	var refs []ImportFile
	resolve := map[string]string{}
	for name, content := range files {
		p := filepath.Join(dir, "src_"+strings.ReplaceAll(name, "/", "_"))
		os.WriteFile(p, []byte(content), 0o644)
		refs = append(refs, ImportFile{ID: p, Name: "root/" + name})
		resolve[p] = p
	}
	proj, err := s.Import("dfs", refs, makeResolve(resolve))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := s.Tree(proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range tree {
		got = append(got, strings.TrimPrefix(n.Path, "root/"))
	}
	want := []string{
		"root", "app", "app/util", "app/util/u1.go", "app/main.py", "src", "src/a", "src/a/b.txt", "src/z.py", "app.txt", "z.txt",
	}
	if len(got) != len(want) {
		t.Fatalf("节点数不符: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 项应为 %s,得到 %s(完整: %v)", i, want[i], got[i], got)
		}
	}
}
