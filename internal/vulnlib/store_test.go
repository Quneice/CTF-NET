package vulnlib

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 构造一个小的漏洞清单目录结构，覆盖扫描规则的全部分支：
//   - Z/S/ND.md 常规案例
//   - 与 S 目录同名的系统总览 md（跳过）
//   - Z 目录直属总览 md（跳过）
//   - 深层 README（跳过）与深层 ND 案例（收录）
//   - 根目录散件（归入「其他」）
//   - My_template 整目录（跳过）
func buildFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Z001中间件漏洞/S1 Apache HTTP Server/ND003 CVE-2021-41773.md",
		"# ND003 CVE-2021-41773 路径穿越\n\n```\nGET /icons/.%2e/%2e%2e/etc/passwd HTTP/1.1\nHost: health-test.skg.com:8889\n\n```\n")
	write("Z001中间件漏洞/S1 Apache HTTP Server/S1 Apache HTTP Server.md", "# 系统总览\n")          // 跳过
	write("Z001中间件漏洞/S1 Apache HTTP Server/httpd-2.4.49/docs/server-status/README.md", "# doc") // 跳过
	write("Z001中间件漏洞/S1 Apache HTTP Server/assets/ND007 CVE-2023-25690.md",                    // 深层 ND：收录
		"# ND007 CVE-2023-25690\n\n```\nGET /categories/1 HTTP/1.1\nHost: 192.168.1.103\n\n```\n")
	write("Z001中间件漏洞/Z001中间件漏洞.md", "# 分类总览\n")   // Z 直属：跳过
	write("Z002Web编程语言/S5 Python/ND001 CVE-2017-8291.md", "# ND001 CVE-2017-8291\n\ncurl -v http://target/xxx\n")
	write("CRLF Injection.md", "# CRLF Injection\n\n```\nGET /%0d%0aSet-Cookie:a=b HTTP/1.1\nHost: x\n\n```\n")
	write("My_template/S1 模板/ND000 模板.md", "# 模板\n")
	// 资源文件（图片，供 GetAsset 测试；非 md 不影响扫描计数）
	write("Z001中间件漏洞/S1 Apache HTTP Server/img/exploit.png", "fake-png-bytes")
	write("top.png", "top-level-image")
	return root
}

func TestParseVulnMD(t *testing.T) {
	md := `# ND003 CVE-2021-41773 2.4.49 路径穿越漏洞

说明 CVE-2021-41773 与 CVE-2021-42013 相关。

` + "```" + `
GET /icons/.%2e/%2e%2e/%2e%2e/etc/passwd HTTP/1.1
Host: health-test.skg.com:8889
User-Agent: Mozilla

` + "```" + `

` + "```" + `
curl -v --path-as-is http://your-ip:8080/icons/.%2e/etc/passwd
` + "```" + `

裸文本请求：

GET /foo HTTP/1.1
Host: bar

` + "```" + `
HTTP/1.1 200 OK
` + "```" + `
`
	p := ParseVulnMD(md)
	if p.Title != "ND003 CVE-2021-41773 2.4.49 路径穿越漏洞" {
		t.Errorf("Title = %q", p.Title)
	}
	if len(p.CVEs) != 2 || p.CVEs[0] != "CVE-2021-41773" || p.CVEs[1] != "CVE-2021-42013" {
		t.Errorf("CVEs = %v", p.CVEs)
	}
	// fenced GET + 裸文本 GET（Host 行后截断，不含响应行）
	if len(p.Payloads) != 2 {
		t.Fatalf("Payloads = %d, want 2: %v", len(p.Payloads), p.Payloads)
	}
	if p.Payloads[0] != "GET /icons/.%2e/%2e%2e/%2e%2e/etc/passwd HTTP/1.1\nHost: health-test.skg.com:8889\nUser-Agent: Mozilla" {
		t.Errorf("Payload[0] = %q", p.Payloads[0])
	}
	if p.Payloads[1] != "GET /foo HTTP/1.1\nHost: bar" {
		t.Errorf("Payload[1] = %q", p.Payloads[1])
	}
	// curl 在 fenced 块里，但 curl 提取扫描全文 → 1 条
	if len(p.Curls) != 1 {
		t.Errorf("Curls = %v", p.Curls)
	}
}

func TestParseVulnMDDedupAndResponseCut(t *testing.T) {
	// 同一 payload 出现两次（复现+利用段）→ 去重；响应行后截断
	md := "# x\n\n" +
		"```\nGET /a HTTP/1.1\nHost: h\n\nHTTP/1.1 200 OK\nxxx\n```\n\n" +
		"```\nGET /a HTTP/1.1\nHost: h\n```\n"
	p := ParseVulnMD(md)
	if len(p.Payloads) != 1 {
		t.Fatalf("want 1 dedup payload, got %d", len(p.Payloads))
	}
	if want := "GET /a HTTP/1.1\nHost: h"; p.Payloads[0] != want {
		t.Errorf("Payload = %q, want %q", p.Payloads[0], want)
	}
}

func TestScanAndQuery(t *testing.T) {
	root := buildFixture(t)
	st, err := open(filepath.Join(t.TempDir(), "vulnlib.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	sum, err := st.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	// 候选：ND003 + 深层 ND007 + ND001(Python) + 根目录 CRLF = 4；
	// 跳过：系统总览 / Z 总览 / README = 3（My_template 整目录 SkipDir 不计数）
	if sum.Total != 4 {
		t.Errorf("Total = %d, want 4", sum.Total)
	}
	if sum.Updated != 4 || sum.Skipped != 3 {
		t.Errorf("Updated=%d Skipped=%d, want 4/3", sum.Updated, sum.Skipped)
	}
	if sum.Deleted != 0 {
		t.Errorf("Deleted = %d", sum.Deleted)
	}

	// 增量：二次扫描全部 unchanged
	sum2, err := st.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if sum2.Updated != 0 || sum2.Unchanged != 4 {
		t.Errorf("second scan Updated=%d Unchanged=%d, want 0/4", sum2.Updated, sum2.Unchanged)
	}

	// 统计
	stats, err := st.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if !stats.Scanned || stats.CaseCount != 4 || stats.CatCount != 3 {
		t.Errorf("Stats = %+v", stats)
	}

	// 分类
	cats, err := st.ListCategories()
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 3 {
		t.Fatalf("categories = %d", len(cats))
	}
	// 每个分类系统计数正确
	byID := map[string]Category{}
	for _, c := range cats {
		byID[c.ID] = c
	}
	if c := byID["Z001中间件漏洞"]; c.CaseCount != 2 || len(c.Systems) != 1 || c.Systems[0].CaseCount != 2 {
		t.Errorf("Z001 = %+v", c)
	}
	if c := byID["__root__"]; c.CaseCount != 1 || c.Name != "其他" {
		t.Errorf("root = %+v", c)
	}

	// 案例查询
	cases, err := st.ListCases("Z001中间件漏洞", "S1 Apache HTTP Server")
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 2 {
		t.Fatalf("cases = %d", len(cases))
	}
	// 搜索 CVE
	hits, err := st.Search("2021-41773", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Title != "ND003 CVE-2021-41773 路径穿越" {
		t.Errorf("search hits = %+v", hits)
	}

	// 详情：payload 提取正确
	var target *CaseSummary
	for i := range cases {
		if cases[i].Name == "ND003 CVE-2021-41773" {
			target = &cases[i]
		}
	}
	if target == nil {
		t.Fatal("case ND003 not found")
	}
	if target.PayloadCount != 1 || len(target.CVEs) != 1 {
		t.Errorf("summary = %+v", target)
	}
	det, err := st.GetCase(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(det.Payloads) != 1 || !strings.Contains(det.Payloads[0], "/icons/.%2e/%2e%2e/etc/passwd") {
		t.Errorf("detail payloads = %v", det.Payloads)
	}
	if det.Path == "" || det.Content == "" {
		t.Errorf("detail missing path/content")
	}

	// 删除文件后重扫 → Deleted
	if err := os.Remove(filepath.Join(root, "CRLF Injection.md")); err != nil {
		t.Fatal(err)
	}
	sum3, err := st.Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if sum3.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1", sum3.Deleted)
	}
	stats3, _ := st.Stats()
	if stats3.CaseCount != 3 || stats3.CatCount != 2 {
		t.Errorf("after delete stats = %+v", stats3)
	}
}

func TestGetAsset(t *testing.T) {
	root := buildFixture(t)
	st, err := open(filepath.Join(t.TempDir(), "vulnlib.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.Scan(root); err != nil {
		t.Fatal(err)
	}
	caseID := "Z001中间件漏洞/S1 Apache HTTP Server/ND003 CVE-2021-41773.md"

	// 相对引用 → 案例目录下解析，MIME 按扩展名
	a, err := st.GetAsset(caseID, "img/exploit.png")
	if err != nil {
		t.Fatal(err)
	}
	if a.Mime != "image/png" || a.DataB64 != base64.StdEncoding.EncodeToString([]byte("fake-png-bytes")) {
		t.Errorf("asset = %+v", a)
	}
	// 反斜杠分隔 + 查询串截断
	a2, err := st.GetAsset(caseID, `img\exploit.png?x=1`)
	if err != nil {
		t.Fatal(err)
	}
	if a2.Mime != "image/png" {
		t.Errorf("a2 = %+v", a2)
	}
	// ../ 回退到库根相对引用
	a3, err := st.GetAsset(caseID, "../../top.png")
	if err != nil {
		t.Fatal(err)
	}
	if a3.DataB64 != base64.StdEncoding.EncodeToString([]byte("top-level-image")) {
		t.Errorf("a3 = %+v", a3)
	}
	// Windows 绝对路径（库内）直接解析
	a4, err := st.GetAsset(caseID, filepath.Join(root, "top.png"))
	if err != nil {
		t.Fatal(err)
	}
	if a4.Mime != "image/png" {
		t.Errorf("a4 = %+v", a4)
	}
	// 越出库根 → 拒绝
	if _, err := st.GetAsset(caseID, "../../../secret.png"); !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("traversal err = %v, want ErrAssetNotFound", err)
	}
	// 不存在
	if _, err := st.GetAsset(caseID, "nope.png"); !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("missing err = %v, want ErrAssetNotFound", err)
	}
}

func TestCaseCRUD(t *testing.T) {
	root := buildFixture(t)
	dbPath := filepath.Join(t.TempDir(), "vulnlib.db")
	st, err := open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.Scan(root); err != nil {
		t.Fatal(err)
	}

	// —— 新增（无 .md 自动补）——
	sum, err := st.CreateCase("Z002Web编程语言", "S5 Python", "ND099 新建案例", "# 新建案例\n\n正文内容")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Updated != 1 {
		t.Errorf("create Updated = %d, want 1", sum.Updated)
	}
	newID := "Z002Web编程语言/S5 Python/ND099 新建案例.md"
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(newID))); err != nil {
		t.Fatalf("new file not written: %v", err)
	}
	det, err := st.GetCase(newID)
	if err != nil {
		t.Fatal(err)
	}
	if det.Title != "新建案例" || det.Content != "# 新建案例\n\n正文内容" {
		t.Errorf("det = %+v", det)
	}

	// 非法文件名（路径穿越 / 保留字符）
	for _, bad := range []string{"../escape", "a/b", "con:evil", ".hidden"} {
		if _, err := st.CreateCase("Z002Web编程语言", "S5 Python", bad, "x"); err == nil {
			t.Errorf("CreateCase(%q) should fail", bad)
		}
	}
	// 不存在的分类
	if _, err := st.CreateCase("不存在分类", "S5 Python", "ND100 x", "x"); err == nil {
		t.Error("CreateCase with unknown cat should fail")
	}

	// —— 修改 ——
	sum2, err := st.UpdateCase(newID, "# 改后标题 CVE-2024-9999\n\n新内容")
	if err != nil {
		t.Fatal(err)
	}
	if sum2.Updated != 1 {
		t.Errorf("update Updated = %d, want 1", sum2.Updated)
	}
	det2, err := st.GetCase(newID)
	if err != nil {
		t.Fatal(err)
	}
	if det2.Title != "改后标题 CVE-2024-9999" || len(det2.CVEs) != 1 {
		t.Errorf("after update det = %+v", det2)
	}

	// —— 删除（备份在 dataDir/backups/vulnlib_deleted，保留相对路径）——
	sum3, err := st.DeleteCase(newID)
	if err != nil {
		t.Fatal(err)
	}
	if sum3.Deleted != 1 {
		t.Errorf("delete Deleted = %d, want 1", sum3.Deleted)
	}
	if _, err := st.GetCase(newID); !errors.Is(err, ErrCaseNotFound) {
		t.Errorf("after delete GetCase err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(newID))); err == nil {
		t.Error("file still exists after delete")
	}
	backup := filepath.Join(filepath.Dir(dbPath), "backups", "vulnlib_deleted", filepath.FromSlash(newID))
	if _, err := os.Stat(backup); err != nil {
		t.Errorf("backup missing: %v", err)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		rel       string
		cat, sys  string
	}{
		{"a.md", "__root__", ""},
		{"Z001/x.md", "", ""},
		{"Z001/S1/x.md", "Z001", "S1"},
		{"Z001/S1/S1.md", "", ""},
		{"Z001/S1/deep/README.md", "", ""},
		{"Z001/S1/assets/ND005 x.md", "Z001", "S1"},
	}
	for _, c := range cases {
		cat, sys := classify(c.rel, strings.Split(c.rel, "/"))
		if cat != c.cat || sys != c.sys {
			t.Errorf("classify(%q) = (%q,%q), want (%q,%q)", c.rel, cat, sys, c.cat, c.sys)
		}
	}
}
