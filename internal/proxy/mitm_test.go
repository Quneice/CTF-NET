package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupMITMServer 启动带 CA 的代理(临时目录),返回代理与 CA 证书池。
func setupMITMServer(t *testing.T, on func(*Record)) (*Server, *x509.CertPool) {
	t.Helper()
	s := NewServer(on)
	if err := s.SetCA(t.TempDir()); err != nil {
		t.Fatalf("setca: %v", err)
	}
	if err := s.SetMITM(true); err != nil {
		t.Fatalf("setmitm: %v", err)
	}
	if err := s.Start(0); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	pool := x509.NewCertPool()
	caPEM, err := os.ReadFile(s.Status().CAFile)
	if err != nil {
		t.Fatalf("read ca: %v", err)
	}
	if !pool.AppendCertsFromPEM(caPEM) {
		t.Fatalf("append ca")
	}
	return s, pool
}

// mitmClientFor 构造经代理且信任代理 CA 的 https 客户端。
func mitmClientFor(s *Server, pool *x509.CertPool) *http.Client {
	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", s.Status().Port))
	return &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{RootCAs: pool},
	}}
}

// TestMITMPassthroughDecrypted HTTPS 经 MITM 解密:目标命中、历史出现 https 内层请求。
func TestMITMPassthroughDecrypted(t *testing.T) {
	tlsTarget := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "secure:%s", r.URL.Path)
	}))
	defer tlsTarget.Close()

	s, pool := setupMITMServer(t, nil)
	resp, err := mitmClientFor(s, pool).Get(tlsTarget.URL + "/secret")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "secure:/secret" {
		t.Fatalf("body = %q", b)
	}

	// 历史应含 https 内层请求(非仅 CONNECT)
	found := false
	for _, item := range s.History(0, 20).Items {
		if item.Method == "GET" && strings.HasPrefix(item.URL, "https://") && strings.Contains(item.URL, "/secret") {
			found = true
			if item.Status != 200 {
				t.Fatalf("status = %d", item.Status)
			}
			break
		}
	}
	if !found {
		t.Fatalf("history 无 https 内层请求: %+v", s.History(0, 20).Items)
	}
}

// TestMITMInterceptModify HTTPS 内层请求同样可拦截修改。
func TestMITMInterceptModify(t *testing.T) {
	var hitPath string
	tlsTarget := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer tlsTarget.Close()

	var s *Server
	s = NewServer(nil)
	if err := s.SetCA(t.TempDir()); err != nil {
		t.Fatalf("setca: %v", err)
	}
	if err := s.SetMITM(true); err != nil {
		t.Fatalf("setmitm: %v", err)
	}
	if err := s.Start(0); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	s.onIntercept = func(r *Record) {
		go func() {
			modified := strings.Replace(r.RawRequest, "/original ", "/flag ", 1)
			_ = s.Decide(r.ID, ActionModified, modified)
		}()
	}
	s.SetIntercept(true)

	pool := x509.NewCertPool()
	caPEM, _ := os.ReadFile(s.Status().CAFile)
	pool.AppendCertsFromPEM(caPEM)

	resp, err := mitmClientFor(s, pool).Get(tlsTarget.URL + "/original")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "/flag" {
		t.Fatalf("body = %q, want /flag", b)
	}
	if hitPath != "/flag" {
		t.Fatalf("target path = %q", hitPath)
	}
}

// TestMITMOffPassthrough MITM 关闭时 CONNECT 仍为透明隧道(回归批 17 行为)。
func TestMITMOffPassthrough(t *testing.T) {
	tlsTarget := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tunneled"))
	}))
	defer tlsTarget.Close()

	s := startTestServer(t, nil) // MITM 默认关
	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", s.Status().Port))
	tr := &http.Transport{Proxy: http.ProxyURL(proxyURL), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	resp, err := (&http.Client{Transport: tr}).Get(tlsTarget.URL)
	if err != nil {
		t.Fatalf("tunnel: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "tunneled" {
		t.Fatalf("body = %q", b)
	}
	for _, item := range s.History(0, 20).Items {
		if item.Method == "GET" {
			t.Fatalf("MITM 关闭时不应有内层请求: %+v", item)
		}
	}
}

// TestEnsureCAPersists CA 一次生成、二次复用(不重新签发)。
func TestEnsureCAPersists(t *testing.T) {
	dir := t.TempDir()
	f1, err := EnsureCA(dir)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	b1, _ := os.ReadFile(f1)
	f2, err := EnsureCA(dir)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	b2, _ := os.ReadFile(f2)
	if string(b1) != string(b2) {
		t.Fatalf("CA 被重新生成")
	}
	if _, err := os.Stat(filepath.Join(dir, caKeyFileName)); err != nil {
		t.Fatalf("私钥缺失: %v", err)
	}
}

// TestHostCertSignedByCA 域证书由 CA 签发且 SAN 匹配(可被 CA 池验证)。
func TestHostCertSignedByCA(t *testing.T) {
	s := NewServer(nil)
	if err := s.SetCA(t.TempDir()); err != nil {
		t.Fatalf("setca: %v", err)
	}
	cert, err := s.hostCert("example.com")
	if err != nil {
		t.Fatalf("hostcert: %v", err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pool := x509.NewCertPool()
	caPEM, _ := os.ReadFile(s.Status().CAFile)
	pool.AppendCertsFromPEM(caPEM)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: "example.com"}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// 缓存命中:同一域两次签发返回同一证书
	cert2, _ := s.hostCert("example.com")
	if string(cert.Certificate[0]) != string(cert2.Certificate[0]) {
		t.Fatalf("缓存未命中")
	}
}
