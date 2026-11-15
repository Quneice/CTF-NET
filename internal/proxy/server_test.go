package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// startTestServer 启动随机端口代理,t.Cleanup 自动停止。
func startTestServer(t *testing.T, on func(*Record)) *Server {
	t.Helper()
	s := NewServer(on)
	if err := s.Start(0); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	return s
}

// clientFor 构造走代理的 http.Client。
func clientFor(s *Server) *http.Client {
	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", s.Status().Port))
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
}

// getBody 读响应体。
func getBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	resp.Body.Close()
	return string(b)
}

// TestCaptureForward 捕获 + 透明转发 + 历史记录(含原始报文)。
func TestCaptureForward(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		fmt.Fprintf(w, "echo:%s|%s", r.URL.Path, b)
	}))
	defer target.Close()

	s := startTestServer(t, nil)
	resp, err := clientFor(s).Post(target.URL+"/hello", "text/plain", strings.NewReader("world"))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if got := getBody(t, resp); got != "echo:/hello|world" {
		t.Fatalf("body = %q", got)
	}

	page := s.History(0, 10)
	if page.Total != 1 {
		t.Fatalf("total = %d, want 1", page.Total)
	}
	item := page.Items[0]
	if item.Status != 200 || item.Method != "POST" {
		t.Fatalf("item = %+v", item)
	}
	rec, err := s.Record(item.ID)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	// 代理形态请求的原始报文为绝对 URI:POST http://host/hello HTTP/1.1
	if !strings.Contains(rec.RawRequest, "POST http://") || !strings.Contains(rec.RawRequest, "/hello") || !strings.Contains(rec.RawRequest, "world") {
		t.Fatalf("raw request = %q", rec.RawRequest)
	}
	if !strings.Contains(rec.RawResponse, "200 OK") || !strings.Contains(rec.RawResponse, "echo:/hello|world") {
		t.Fatalf("raw response = %q", rec.RawResponse)
	}
}

// TestInterceptForward 拦截 → 放行。
func TestInterceptForward(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()

	var intercepted *Record
	var s *Server
	s = startTestServer(t, func(r *Record) {
		intercepted = r
		go func() { _ = s.Decide(r.ID, ActionForward, "") }()
	})
	s.SetIntercept(true)

	resp, err := clientFor(s).Get(target.URL + "/x")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := getBody(t, resp); got != "ok" {
		t.Fatalf("body = %q", got)
	}
	if intercepted == nil || !intercepted.Intercepted {
		t.Fatalf("intercepted = %+v", intercepted)
	}
	item := s.History(0, 1).Items[0]
	if item.Status != 200 || !item.Intercepted {
		t.Fatalf("item = %+v", item)
	}
}

// TestInterceptDrop 拦截 → 丢弃:目标不被命中,客户端收到 403,历史记录状态 0。
func TestInterceptDrop(t *testing.T) {
	var hits int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte("never"))
	}))
	defer target.Close()

	var s *Server
	s = startTestServer(t, func(r *Record) {
		go func() { _ = s.Decide(r.ID, ActionDrop, "") }()
	})
	s.SetIntercept(true)

	resp, err := clientFor(s).Get(target.URL + "/x")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if !strings.Contains(getBody(t, resp), "丢弃") {
		t.Fatalf("body 未含丢弃说明")
	}
	if hits != 0 {
		t.Fatalf("target hits = %d, want 0", hits)
	}
	item := s.History(0, 1).Items[0]
	if item.Status != 0 || !strings.Contains(item.Note, "丢弃") {
		t.Fatalf("item = %+v", item)
	}
}

// TestInterceptModify 拦截 → 修改放行:目标收到修改后的路径。
func TestInterceptModify(t *testing.T) {
	var hitPath string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer target.Close()

	var s *Server
	s = startTestServer(t, func(r *Record) {
		go func() {
			// 代理请求 DumpRequest 输出绝对 URI(GET http://host/original HTTP/1.1),替换路径段即可
			modified := strings.Replace(r.RawRequest, "/original ", "/flag ", 1)
			_ = s.Decide(r.ID, ActionModified, modified)
		}()
	})
	s.SetIntercept(true)

	resp, err := clientFor(s).Get(target.URL + "/original")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := getBody(t, resp); got != "/flag" {
		t.Fatalf("body = %q, want /flag", got)
	}
	if hitPath != "/flag" {
		t.Fatalf("target path = %q, want /flag", hitPath)
	}
}

// TestReplay 重放:目标二次命中,历史新增一条。
func TestReplay(t *testing.T) {
	var hits int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()

	s := startTestServer(t, nil)
	if _, err := clientFor(s).Get(target.URL + "/once"); err != nil {
		t.Fatalf("get: %v", err)
	}
	full, err := s.Record(s.History(0, 1).Items[0].ID)
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	replayed, err := s.Replay(full.RawRequest)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
	if replayed.Status != 200 || !strings.Contains(replayed.RawResponse, "ok") {
		t.Fatalf("replayed = %+v", replayed.Summary)
	}
	if page := s.History(0, 10); page.Total != 2 || page.Items[0].ID != replayed.ID {
		t.Fatalf("history total = %d newest = %s", page.Total, page.Items[0].ID)
	}
}

// TestConnectTunnel CONNECT 隧道贯通(TLS 握手经隧道完成),历史记录 CONNECT 条目。
func TestConnectTunnel(t *testing.T) {
	tlsTarget := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("secure-ok"))
	}))
	defer tlsTarget.Close()

	s := startTestServer(t, nil)
	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", s.Status().Port))
	tr := &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 测试自签证书
	}
	resp, err := (&http.Client{Transport: tr}).Get(tlsTarget.URL)
	if err != nil {
		t.Fatalf("tls via tunnel: %v", err)
	}
	if got := getBody(t, resp); got != "secure-ok" {
		t.Fatalf("body = %q", got)
	}

	found := false
	for _, item := range s.History(0, 20).Items {
		if item.Method == "CONNECT" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("history 无 CONNECT 条目")
	}
}

// TestHistoryCap 历史封顶:超上限只保留最新 MaxHistory 条,新→旧。
func TestHistoryCap(t *testing.T) {
	s := NewServer(nil)
	for i := 0; i < MaxHistory+100; i++ {
		s.add(&Record{Summary: Summary{ID: fmt.Sprint(i), Method: "GET", Host: "x", URL: "http://x/"}})
	}
	page := s.History(0, 0)
	if page.Total != MaxHistory {
		t.Fatalf("total = %d, want %d", page.Total, MaxHistory)
	}
	if page.Items[0].ID != fmt.Sprint(MaxHistory+99) {
		t.Fatalf("newest = %s, want %d", page.Items[0].ID, MaxHistory+99)
	}
}

// TestDecideUnknown 对不存在的挂起请求做决策报错。
func TestDecideUnknown(t *testing.T) {
	s := NewServer(nil)
	if err := s.Decide("nope", ActionForward, ""); err == nil {
		t.Fatalf("want error, got nil")
	}
	if err := s.Decide("nope", "bogus", ""); err == nil {
		t.Fatalf("want error for bogus action")
	}
}

// TestParseRequestOriginForm 源站形式报文解析(Repeater 常用:只有路径 + Host 头)。
func TestParseRequestOriginForm(t *testing.T) {
	req, err := parseRequest("GET /path?q=1 HTTP/1.1\r\nHost: example.com\r\n\r\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.URL.String() != "http://example.com/path?q=1" {
		t.Fatalf("url = %s", req.URL.String())
	}
	if req.RequestURI != "" {
		t.Fatalf("RequestURI = %q, want 空(供 client 转发)", req.RequestURI)
	}
}

// TestRecordWhileHanging 拦截挂起中的记录可按 id 取全量(决策前尚未入历史;
// 前端拦截事件处理器依赖 Record 解析挂起报文,否则拦截 UI 永远为空)。
func TestRecordWhileHanging(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()

	hanging := make(chan *Record, 4)
	s := startTestServer(t, func(r *Record) { hanging <- r })
	s.SetIntercept(true)

	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := clientFor(s).Get(target.URL + "/hang")
		if err != nil {
			t.Errorf("get: %v", err)
			return
		}
		if got := getBody(t, resp); got != "ok" {
			t.Errorf("body = %q", got)
		}
	}()

	select {
	case rec := <-hanging:
		got, err := s.Record(rec.ID) // 关键断言:挂起中可查询
		if err != nil {
			t.Fatalf("record while hanging: %v", err)
		}
		if !strings.Contains(got.RawRequest, "/hang") {
			t.Fatalf("raw request = %q", got.RawRequest)
		}
		if got.Status != 0 {
			t.Fatalf("hanging status = %d, want 0", got.Status)
		}
		if err := s.Decide(rec.ID, ActionForward, ""); err != nil {
			t.Fatalf("decide: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no intercept event")
	}
	<-done
	if s.History(0, 1).Total < 1 {
		t.Fatal("decision completed but history empty")
	}
}

// TestRecordMultipleHanging 多请求并发挂起时逐一可解析(前端按事件逐条拉取)。
func TestRecordMultipleHanging(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "path:%s", r.URL.Path)
	}))
	defer target.Close()

	hanging := make(chan *Record, 8)
	s := startTestServer(t, func(r *Record) { hanging <- r })
	s.SetIntercept(true)

	const n = 3
	var wg sync.WaitGroup
	paths := make([]string, n)
	for i := 0; i < n; i++ {
		paths[i] = fmt.Sprintf("/p%d", i)
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			resp, err := clientFor(s).Get(target.URL + p)
			if err != nil {
				t.Errorf("get %s: %v", p, err)
				return
			}
			getBody(t, resp)
		}(paths[i])
	}

	got := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		select {
		case rec := <-hanging:
			full, err := s.Record(rec.ID)
			if err != nil {
				t.Fatalf("record %s while hanging: %v", rec.ID, err)
			}
			if !strings.Contains(full.RawRequest, "/p") {
				t.Fatalf("raw request = %q", full.RawRequest)
			}
			got[rec.ID] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d/%d intercept events", len(got), n)
		}
	}
	for id := range got {
		if err := s.Decide(id, ActionForward, ""); err != nil {
			t.Fatalf("decide %s: %v", id, err)
		}
	}
	wg.Wait() // 等全部响应收尾再结束,避免 Cleanup 的 Stop() 掐断活跃连接
}
