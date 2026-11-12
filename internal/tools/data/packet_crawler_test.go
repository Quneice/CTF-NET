package data

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

// 1x1 透明 PNG
var pngBytes = func() []byte {
	b, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==")
	return b
}()

func TestExpandPlaceholderValues(t *testing.T) {
	cases := []struct {
		expr string
		want []string
		err  bool
	}{
		{"1-5", []string{"1", "2", "3", "4", "5"}, false},
		{"5-1", []string{"5", "4", "3", "2", "1"}, false},
		{"1-10:3", []string{"1", "4", "7", "10"}, false},
		{"1-3:1:4", []string{"0001", "0002", "0003"}, false},
		{"a,b,c", []string{"a", "b", "c"}, false},
		{"1-100:0", nil, true},   // 步长为 0
		{"10-1:1", nil, true},    // 方向与步长不一致
		{"", nil, true},          // 空表达式
		{"1-2:1:13", nil, true},  // 补零宽度超限
		{"x,y", []string{"x", "y"}, false},
	}
	for _, c := range cases {
		got, err := expandPlaceholderValues(c.expr)
		if c.err {
			if err == nil {
				t.Errorf("expandPlaceholderValues(%q) 期望报错,实际成功: %v", c.expr, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("expandPlaceholderValues(%q) 报错: %v", c.expr, err)
			continue
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("expandPlaceholderValues(%q) = %v, 期望 %v", c.expr, got, c.want)
		}
	}
}

func TestParsePlaceholderSpecs(t *testing.T) {
	specs, err := parsePlaceholderSpecs("# 注释\nid=1-3\nname=a,b\nbad_line\n")
	if err == nil {
		t.Fatal("含非法行应报错")
	}
	if !strings.Contains(err.Error(), "bad_line") {
		t.Fatalf("错误信息应指出非法行: %v", err)
	}

	specs, err = parsePlaceholderSpecs("id=1-3\nname=a,b\n")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(specs) != 2 || specs[0].name != "id" || specs[1].name != "name" {
		t.Fatalf("解析结果不对: %+v", specs)
	}
	if _, err := parsePlaceholderSpecs("id=1-3\nid=4-5\n"); err == nil {
		t.Fatal("重复定义应报错")
	}
}

func TestExpandCombinations(t *testing.T) {
	raw := "GET /x?id={{id}}&n={{name}} HTTP/1.1\r\nHost: h\r\n\r\n"
	specs, _ := parsePlaceholderSpecs("id=1-2\nname=a,b\n")
	combs, err := expandCombinations(raw, specs, 100)
	if err != nil {
		t.Fatalf("展开失败: %v", err)
	}
	if len(combs) != 4 {
		t.Fatalf("组合数 = %d, 期望 4", len(combs))
	}
	// 首次出现顺序:id 在外层,变化最慢
	want := []string{"id=1 name=a", "id=1 name=b", "id=2 name=a", "id=2 name=b"}
	for i, c := range combs {
		got := "id=" + c["id"] + " name=" + c["name"]
		if got != want[i] {
			t.Errorf("组合[%d] = %s, 期望 %s", i, got, want[i])
		}
	}
	// 缺定义
	if _, err := expandCombinations("GET /{{x}} HTTP/1.1\r\nHost: h\r\n\r\n", specs, 100); err == nil {
		t.Fatal("缺少定义的占位符应报错")
	}
	// 超上限
	if _, err := expandCombinations(raw, specs, 3); err == nil {
		t.Fatal("组合数超上限应报错")
	}
	// 无占位符 → 单组合
	combs, err = expandCombinations("GET / HTTP/1.1\r\nHost: h\r\n\r\n", nil, 100)
	if err != nil || len(combs) != 1 || len(combs[0]) != 0 {
		t.Fatalf("无占位符应返回单个空映射: %v %v", combs, err)
	}
}

func TestSubstitute(t *testing.T) {
	got := substitute("a={{x}} b={{x}} c={{y}}", map[string]string{"x": "1", "y": "2"})
	if got != "a=1 b=1 c=2" {
		t.Fatalf("替换结果: %q", got)
	}
}

func TestParseRawRequest(t *testing.T) {
	raw := "POST /api/upload?id={{id}} HTTP/1.1\r\nHost: example.com\r\nContent-Type: application/json\r\n\r\n{\"a\":1}\r\n"
	pr, err := parseRawRequest(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if pr.method != "POST" || pr.target != "/api/upload?id={{id}}" {
		t.Fatalf("请求行解析错误: %+v", pr)
	}
	if pr.headers["Host"] != "example.com" || pr.headers["Content-Type"] != "application/json" {
		t.Fatalf("头部解析错误: %+v", pr.headers)
	}
	if pr.body != "{\"a\":1}\r\n" {
		t.Fatalf("body 解析错误: %q", pr.body)
	}
	// 路径型目标 + 缺 Host → buildHTTPRequest 报错
	if _, err := buildHTTPRequest(&parsedRequest{method: "GET", target: "/x", headers: map[string]string{}}, "http"); err == nil {
		t.Fatal("缺 Host 头应报错")
	}
	// 绝对目标
	req, err := buildHTTPRequest(&parsedRequest{method: "GET", target: "https://a.b/c", headers: map[string]string{"Host": "a.b"}}, "http")
	if err != nil {
		t.Fatalf("绝对目标构造失败: %v", err)
	}
	if req.URL.String() != "https://a.b/c" || req.Host != "a.b" {
		t.Fatalf("绝对目标构造错误: %v %q", req.URL, req.Host)
	}
}

func TestSniffExt(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		ct   string
		path string
		ext  string
		text bool
	}{
		{"png magic", pngBytes, "application/octet-stream", "/a", ".png", false},
		{"jpg magic", []byte{0xFF, 0xD8, 0xFF, 0xE0}, "", "/a", ".jpg", false},
		{"gif magic", []byte("GIF89a"), "", "/a", ".gif", false},
		{"zip magic", []byte{'P', 'K', 3, 4, 0, 0}, "", "/a", ".zip", false},
		{"pdf magic", []byte("%PDF-1.7"), "", "/a", ".pdf", false},
		{"gz magic", []byte{0x1F, 0x8B, 8, 0}, "", "/a", ".gz", false},
		{"ct json", []byte(`{"a":1}`), "application/json", "/a", ".json", true},
		{"ct html", []byte("<html></html>"), "text/html; charset=utf-8", "/a", ".html", true},
		{"url ext", []byte("binary stuff"), "application/octet-stream", "/files/dump.rar", ".rar", false},
		{"text guess", []byte("hello world plain text"), "", "/a", ".txt", true},
		{"unknown bin", []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}, "", "/a", ".bin", false},
	}
	for _, c := range cases {
		ext, isText := sniffExt(c.head, c.ct, c.path)
		if ext != c.ext || isText != c.text {
			t.Errorf("%s: sniffExt = (%s, %v), 期望 (%s, %v)", c.name, ext, isText, c.ext, c.text)
		}
	}
}

func TestExtractURLs(t *testing.T) {
	html := []byte(`<a href="http://a.b/img/1.png">x</a><img src="https://c.d/a.jpg">`)
	got := extractURLs(html, "http://a.b/index.html")
	if len(got) != 2 || got[0] != "http://a.b/img/1.png" || got[1] != "https://c.d/a.jpg" {
		t.Fatalf("提取结果: %v", got)
	}
	// JSON 转义 + 协议相对 + 尾标点 + 去重
	js := []byte(`{"u":"https:\/\/a.b\/f.png"} //x.b/f2.png "https://a.b/f.png".`)
	got = extractURLs(js, "https://a.b/index.html")
	if len(got) != 2 {
		t.Fatalf("JSON 提取结果(应去重): %v", got)
	}
	if got[0] != "https://a.b/f.png" || got[1] != "https://x.b/f2.png" {
		t.Fatalf("JSON 提取内容不对: %v", got)
	}
	// 相对路径按父 URL 解析
	rel := []byte(`<img src="/img/x.png"><a href="sub/y.png">y</a><a href="#anchor">z</a>`)
	got = extractURLs(rel, "http://a.b/dir/index.html")
	if len(got) != 2 {
		t.Fatalf("相对路径提取结果: %v", got)
	}
	if got[0] != "http://a.b/img/x.png" || got[1] != "http://a.b/dir/sub/y.png" {
		t.Fatalf("相对路径解析不对: %v", got)
	}
}

func TestFileStem(t *testing.T) {
	if s := fileStem("http://h/a/b/item_42.png?x=1"); s != "item_42" {
		t.Fatalf("fileStem = %q", s)
	}
	if s := fileStem("http://h/"); s != "resp" {
		t.Fatalf("fileStem 根路径 = %q", s)
	}
	if s := fileStem("http://h/a/bad name<script>"); s != "bad_name_script_" {
		t.Fatalf("fileStem 非法字符 = %q", s)
	}
}

// createUniqueFile：保留原始文件名；重名时追加 _N 后缀且不覆盖已有文件。
func TestCreateUniqueFile(t *testing.T) {
	dir := t.TempDir()
	name, f, err := createUniqueFile(dir, "photo", ".png")
	if err != nil {
		t.Fatal(err)
	}
	if name != "photo.png" {
		t.Fatalf("首个文件名 = %q", name)
	}
	f.WriteString("first")
	f.Close()

	name2, f2, err := createUniqueFile(dir, "photo", ".png")
	if err != nil {
		t.Fatal(err)
	}
	if name2 != "photo_1.png" {
		t.Fatalf("重名文件名 = %q", name2)
	}
	f2.WriteString("second")
	f2.Close()

	if b, _ := os.ReadFile(filepath.Join(dir, "photo.png")); string(b) != "first" {
		t.Fatalf("原文件被覆盖: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "photo_1.png")); string(b) != "second" {
		t.Fatalf("重名文件内容 = %q", b)
	}
}

// TestPacketCrawlerEndToEnd 端到端:httptest 服务器 + 占位符批量请求 + 链接下载 + 产物打包。
func TestPacketCrawlerEndToEnd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/item", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("id") {
		case "2":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(pngBytes)
		case "3":
			// 空 zip（End of central directory 标记即可被魔数识别为 .zip）
			w.Header().Set("Content-Type", "application/zip")
			w.Write([]byte{'P', 'K', 5, 6, 0, 0, 0, 0, 0, 0})
		default:
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><body><img src="/img/a.png"><a href="http://` + r.Host + `/dl/doc.pdf">d</a></body></html>`))
		}
	})
	mux.HandleFunc("/img/a.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(pngBytes)
	})
	mux.HandleFunc("/dl/doc.pdf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte("%PDF-1.4 fake"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	raw := "GET /item?id={{id}} HTTP/1.1\r\nHost: " + host + "\r\n\r\n"
	artDir := t.TempDir()
	tool := &PacketCrawlerTool{BaseTool: core.BaseTool{Runner: packetCrawlerRun}}
	res, err := tool.Run(context.Background(), map[string]any{
		"raw_request":  raw,
		"placeholders": "id=1-3",
		"concurrency":  2,
		"timeout":      5,
		"max_requests": 100,
		"follow_links": true,
		"delay_ms":     0,
	}, &core.ToolContext{ArtifactsDir: artDir})
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("状态 = %s, stderr=%s\nstdout=%s", res.Status, res.Stderr, res.Stdout)
	}

	// crawl/ 目录内容:id=1 的 html、id=2 的 png、id=3 的 zip、链接下载的 a.png 与 doc.pdf。
	// 文件名必须保留原始名（无序号前缀重命名）。
	files, err := os.ReadDir(filepath.Join(artDir, "crawl"))
	if err != nil {
		t.Fatalf("读取 crawl 目录失败: %v", err)
	}
	got := map[string]bool{}
	for _, f := range files {
		got[f.Name()] = true
	}
	for _, want := range []string{"item.html", "item.png", "item.zip", "a.png", "doc.pdf"} {
		if !got[want] {
			names := []string{}
			for _, f := range files {
				names = append(names, f.Name())
			}
			t.Fatalf("缺少原始文件名 %s，实际文件 %v", want, names)
		}
	}
	if _, err := os.Stat(filepath.Join(artDir, "crawl_report.txt")); err != nil {
		t.Fatalf("报告缺失: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artDir, "result.zip")); err != nil {
		t.Fatalf("zip 缺失: %v", err)
	}
	// 报告里应包含 5 条记录与汇总
	rep, _ := os.ReadFile(filepath.Join(artDir, "crawl_report.txt"))
	if !strings.Contains(string(rep), "status=200") || !strings.Contains(string(rep), "成功 5 / 失败 0") {
		t.Fatalf("报告内容不对:\n%s", rep)
	}
}

// TestPacketCrawlerErrors 错误场景:缺 Host / 缺占位符定义 / 组合超上限 / 全部失败。
func TestPacketCrawlerErrors(t *testing.T) {
	artDir := t.TempDir()
	tool := &PacketCrawlerTool{BaseTool: core.BaseTool{Runner: packetCrawlerRun}}

	// 路径型目标缺 Host
	res, _ := tool.Run(context.Background(), map[string]any{
		"raw_request": "GET /x?id={{id}} HTTP/1.1\r\n\r\n",
		"placeholders": "id=1-2",
	}, &core.ToolContext{ArtifactsDir: artDir})
	if res.Status != core.StatusError {
		t.Fatalf("缺 Host 应整体报错,实际 %s", res.Status)
	}

	// 占位符未定义
	res, _ = tool.Run(context.Background(), map[string]any{
		"raw_request":  "GET /x?id={{id}} HTTP/1.1\r\nHost: a.b\r\n\r\n",
		"placeholders": "",
	}, &core.ToolContext{ArtifactsDir: artDir})
	if res.Status != core.StatusError || !strings.Contains(res.Stderr, "{{id}}") {
		t.Fatalf("占位符未定义应报错: %s %s", res.Status, res.Stderr)
	}

	// 组合超上限
	res, _ = tool.Run(context.Background(), map[string]any{
		"raw_request":  "GET /x?id={{id}} HTTP/1.1\r\nHost: a.b\r\n\r\n",
		"placeholders": "id=1-100",
		"max_requests": 10,
	}, &core.ToolContext{ArtifactsDir: artDir})
	if res.Status != core.StatusError || !strings.Contains(res.Stderr, "max_requests") {
		t.Fatalf("组合超上限应报错: %s %s", res.Status, res.Stderr)
	}

	// 全部失败(连不上的地址)
	res, _ = tool.Run(context.Background(), map[string]any{
		"raw_request":  "GET /x HTTP/1.1\r\nHost: 127.0.0.1:1\r\n\r\n",
		"placeholders": "",
		"timeout":      2,
	}, &core.ToolContext{ArtifactsDir: artDir})
	if res.Status != core.StatusError {
		t.Fatalf("全部失败应报 error,实际 %s\n%s", res.Status, res.Stdout)
	}
}
