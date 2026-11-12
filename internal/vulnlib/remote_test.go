package vulnlib

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"ctfnet/internal/core"
)

func TestFetchRemoteImage(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		switch r.URL.Path {
		case "/img.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("fake-png-data"))
		case "/page":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html></html>"))
		case "/noct.png":
			w.Header().Set("Content-Type", "") // 显式空值：阻止 Go server 自动嗅探，模拟响应头缺失
			_, _ = w.Write([]byte("fake-gif-data"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	st, err := open(filepath.Join(t.TempDir(), "vulnlib.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// 未配置代理 → 直连成功，MIME 取响应头
	a, err := st.FetchRemoteImage(server.URL + "/img.png")
	if err != nil {
		t.Fatal(err)
	}
	if a.Mime != "image/png" || a.DataB64 != base64.StdEncoding.EncodeToString([]byte("fake-png-data")) {
		t.Errorf("asset = %+v", a)
	}

	// 非图片 Content-Type → 拒绝
	if _, err := st.FetchRemoteImage(server.URL + "/page"); !errors.Is(err, errRemoteNotImage) {
		t.Errorf("page err = %v, want errRemoteNotImage", err)
	}
	// 无 Content-Type → 按 URL 扩展名兜底
	a2, err := st.FetchRemoteImage(server.URL + "/noct.png")
	if err != nil || a2.Mime != "image/png" {
		t.Errorf("noct = %+v, %v", a2, err)
	}
	// 非 http(s) 协议 → 拒绝
	if _, err := st.FetchRemoteImage("file:///etc/passwd"); err == nil {
		t.Error("file:// should be rejected")
	}

	// 配置代理后流量必须走代理：代理指向拒连端口 → 拉取失败且不直连服务器
	old := core.GetAppConfig()
	defer core.SetAppConfig(old)
	cfg := *old
	cfg.Proxy.URL = "http://127.0.0.1:1"
	core.SetAppConfig(&cfg)
	before := atomic.LoadInt32(&hits)
	if _, err := st.FetchRemoteImage(server.URL + "/img.png"); err == nil {
		t.Error("with dead proxy, fetch should fail")
	}
	if after := atomic.LoadInt32(&hits); after != before {
		t.Errorf("server hit %d→%d: traffic bypassed proxy", before, after)
	}
}
