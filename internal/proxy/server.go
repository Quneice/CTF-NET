// Package proxy 实现 Burp 风格的 HTTP 代理引擎(纯标准库,零第三方依赖):
// 流量捕获(原始报文快照)→ 拦截挂起(channel 等 UI 决策)→ 放行/丢弃/修改放行 → 历史记录 → 重放。
// P1 范围:HTTP/1.1 + CONNECT TCP 直通隧道;TLS MITM(内置 CA 按域签发)留待 P2。
package proxy

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 拦截决策动作(与前端契约一致)。
const (
	ActionForward  = "forward"
	ActionDrop     = "drop"
	ActionModified = "modified"
)

// MaxHistory 内存历史上限(条)。
const MaxHistory = 500

// Server 代理服务(单实例,由 App 绑定层持有)。
type Server struct {
	mu          sync.Mutex
	listener    net.Listener
	httpSrv     *http.Server
	port        int
	intercept   bool
	mitm        bool                    // HTTPS 解密开关
	caDir       string                  // CA 目录(DataDir/certs)
	caFile      string                  // CA 证书文件路径
	caCert      *tls.Certificate        // 内存 CA(装载自 caFile)
	hostCerts   map[string]*tls.Certificate // 按域签发缓存
	history     []*Record
	pending     map[string]chan decision
	pendingRecs map[string]*Record // 挂起请求快照:决策前 Record() 由此解析(事件已发、尚未入历史)
	client      *http.Client
	onIntercept func(*Record)
}

// decision 拦截器等待的 UI 决策。
type decision struct {
	action   string
	modified string
}

// NewServer 创建代理服务;onIntercept 在拦截挂起时回调(绑定层用于 EventsEmit)。
func NewServer(onIntercept func(*Record)) *Server {
	// 转发默认跳过目标证书校验:渗透/CTF 目标常为自签证书,且内置浏览器本身
	// 以 --ignore-certificate-errors 启动(口径一致);如需严格校验可 SetTLSClientConfig 覆盖。
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	return &Server{
		history:     make([]*Record, 0, MaxHistory),
		pending:     make(map[string]chan decision),
		pendingRecs: make(map[string]*Record),
		hostCerts: make(map[string]*tls.Certificate),
		// 不跟随重定向:Burp 行为是逐跳展示
		client: &http.Client{
			Transport:     transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		onIntercept: onIntercept,
	}
}

// Start 在 127.0.0.1:port 上启动代理;port<=0 时随机端口。
func (s *Server) Start(port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return fmt.Errorf("代理已在运行(端口 %d)", s.port)
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("代理监听失败: %w", err)
	}
	s.listener = ln
	s.port = ln.Addr().(*net.TCPAddr).Port
	s.httpSrv = &http.Server{Handler: s}
	go func() { _ = s.httpSrv.Serve(ln) }()
	return nil
}

// Stop 停止代理;挂起中的拦截请求按丢弃收尾,避免连接悬挂。
func (s *Server) Stop() error {
	s.mu.Lock()
	srv := s.httpSrv
	s.mu.Unlock()
	if srv == nil {
		return errors.New("代理未运行")
	}
	_ = srv.Close()
	s.mu.Lock()
	s.listener = nil
	s.httpSrv = nil
	s.port = 0
	for id, ch := range s.pending {
		ch <- decision{action: ActionDrop}
		delete(s.pending, id)
		delete(s.pendingRecs, id)
	}
	s.mu.Unlock()
	return nil
}

// Status 返回运行状态。
func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Status{Running: s.listener != nil, Port: s.port, Intercept: s.intercept, MITM: s.mitm && s.caCert != nil, CAFile: s.caFile}
}

// SetIntercept 开关全局拦截(不影响 CONNECT 隧道)。
func (s *Server) SetIntercept(on bool) {
	s.mu.Lock()
	s.intercept = on
	s.mu.Unlock()
}

// Decide 对挂起中的拦截请求做出决策。
func (s *Server) Decide(id, action, modified string) error {
	s.mu.Lock()
	ch, ok := s.pending[id]
	s.mu.Unlock()
	if !ok {
		return errors.New("该请求不在拦截等待中(可能已决策或代理已停止)")
	}
	switch action {
	case ActionForward, ActionDrop:
	case ActionModified:
		if strings.TrimSpace(modified) == "" {
			return errors.New("修改放行需要提供修改后的请求报文")
		}
	default:
		return fmt.Errorf("未知决策动作: %s", action)
	}
	ch <- decision{action: action, modified: modified}
	return nil
}

// Replay 把原始请求报文重新发送并记录(Repeater)。
func (s *Server) Replay(raw string) (Record, error) {
	req, err := parseRequest(raw)
	if err != nil {
		return Record{}, err
	}
	rec := &Record{Summary: Summary{
		ID:        newID(),
		Timestamp: time.Now().UnixMilli(),
		Method:    req.Method,
		Host:      req.Host,
		URL:       req.URL.String(),
	}}
	rec.RawRequest = raw
	resp, dur, err := s.roundTrip(req)
	if err != nil {
		rec.Note = "转发失败: " + err.Error()
		s.add(rec)
		return *rec, nil
	}
	rec.Status = resp.StatusCode
	rec.DurationMs = dur
	rec.RawResponse = dumpResponse(resp)
	resp.Body.Close()
	s.add(rec)
	return *rec, nil
}

// ServeHTTP 代理入口:捕获 → (拦截?) → 转发 → 回写 → 入历史。
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
		return
	}

	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	rec := &Record{Summary: Summary{
		ID:        newID(),
		Timestamp: time.Now().UnixMilli(),
		Method:    r.Method,
		Host:      r.Host,
		URL:       r.URL.String(),
	}}
	rec.RawRequest = dumpRequest(r, body)
	// dumpRequest 消费了临时 body 读器,这里把 body 装回供后续转发
	r.Body = io.NopCloser(bytes.NewReader(body))

	toSend := r
	if s.isIntercept() {
		rec.Intercepted = true
		ch := make(chan decision, 1)
		s.mu.Lock()
		s.pending[rec.ID] = ch
		s.pendingRecs[rec.ID] = rec
		s.mu.Unlock()
		if s.onIntercept != nil {
			s.onIntercept(rec)
		}
		d := <-ch // 挂起直到 UI 决策(或 Stop 时按丢弃收尾)
		s.mu.Lock()
		delete(s.pending, rec.ID)
		delete(s.pendingRecs, rec.ID)
		s.mu.Unlock()
		switch d.action {
		case ActionDrop:
			rec.Note = "被拦截丢弃"
			s.add(rec)
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("403 Forbidden: 请求被代理拦截并丢弃"))
			return
		case ActionModified:
			parsed, err := parseRequest(d.modified)
			if err != nil {
				rec.Note = "修改后的报文解析失败: " + err.Error()
				s.add(rec)
				http.Error(w, "修改后的请求报文解析失败: "+err.Error(), http.StatusBadRequest)
				return
			}
			// parseRequest 对无 scheme 的报文缺省 http;修改放行应保持原请求的 scheme
			// (MITM 内层请求原为 https,否则会以明文 HTTP 打到 TLS 端口)
			if parsed.URL.Scheme == "http" && r.URL != nil && r.URL.Scheme == "https" {
				parsed.URL.Scheme = "https"
			}
			toSend = parsed
		}
	}

	resp, dur, err := s.roundTrip(toSend)
	if err != nil {
		rec.Note = "转发失败: " + err.Error()
		s.add(rec)
		http.Error(w, "转发失败: "+err.Error(), http.StatusBadGateway)
		return
	}
	rec.Status = resp.StatusCode
	rec.DurationMs = dur
	rec.RawResponse = dumpResponse(resp)
	copyResponse(w, resp)
	s.add(rec)
}

func (s *Server) isIntercept() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.intercept
}

func (s *Server) mitmOn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mitm && s.caCert != nil
}

// handleConnect CONNECT 隧道:直通不解密(记录于历史,TLS MITM 留待 P2)。
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	rec := &Record{Summary: Summary{
		ID:        newID(),
		Timestamp: time.Now().UnixMilli(),
		Method:    r.Method,
		Host:      r.Host,
		URL:       r.Host,
	}}
	hj, ok := w.(http.Hijacker)
	if !ok {
		rec.Note = "hijack 不支持"
		s.add(rec)
		http.Error(w, "hijack 不支持", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		rec.Note = "hijack 失败: " + err.Error()
		s.add(rec)
		return
	}
	serverConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		rec.Note = "连接目标失败: " + err.Error()
		s.add(rec)
		_ = clientConn.Close()
		return
	}
	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	// 记录先入历史:隧道保持长连接,双向拷贝可能持续到会话结束,不能等拷贝收尾再记录
	rec.Status = http.StatusOK

	if s.mitmOn() {
		// HTTPS 解密:内层请求走同一管线,CONNECT 条目注明 MITM
		rec.Note = "TLS MITM 解密中"
		s.add(rec)
		_ = serverConn.Close()
		s.serveMITM(clientConn, hostOnly(r.Host), rec)
		return
	}

	rec.Note = "CONNECT 隧道(已建立,未解密)"
	s.add(rec)
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(serverConn, clientConn); done <- struct{}{} }()
	go func() { _, _ = io.Copy(clientConn, serverConn); done <- struct{}{} }()
	<-done
	_ = clientConn.Close()
	_ = serverConn.Close()
}

// roundTrip 转发请求并计时;补齐 scheme/host,清空 RequestURI 以走 client 请求路径。
func (s *Server) roundTrip(req *http.Request) (*http.Response, int64, error) {
	if req.URL == nil {
		req.URL = &url.URL{}
	}
	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	req.RequestURI = ""
	start := time.Now()
	resp, err := s.client.Do(req)
	return resp, time.Since(start).Milliseconds(), err
}

// dumpRequest 把请求(含 body)序列化为原始报文;消费临时 body 读器,调用方负责装回 body。
func dumpRequest(r *http.Request, body []byte) string {
	r.Body = io.NopCloser(bytes.NewReader(body))
	d, err := httputil.DumpRequest(r, true)
	if err != nil {
		return ""
	}
	return string(d)
}

// dumpResponse 把响应序列化为原始报文;body 读入内存后恢复,供后续回写客户端。
func dumpResponse(resp *http.Response) string {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		body = nil
	}
	_ = resp.Body.Close()
	restored := func() { resp.Body = io.NopCloser(bytes.NewReader(body)) }
	restored()
	defer restored()
	var buf bytes.Buffer
	if err := resp.Write(&buf); err != nil {
		return ""
	}
	return buf.String()
}

// copyResponse 把(快照后的)响应回写客户端。
func copyResponse(w http.ResponseWriter, resp *http.Response) {
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if resp.Body != nil {
		_, _ = io.Copy(w, resp.Body)
		_ = resp.Body.Close()
	}
}

// parseRequest 把原始报文文本解析为可转发的 *http.Request(支持代理绝对形式与源站形式)。
// 注意:不能 TrimSpace——结尾的 \r\n\r\n 是报文终结符,剪掉后 ReadRequest 会报 unexpected EOF。
func parseRequest(raw string) (*http.Request, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("请求报文为空")
	}
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		return nil, fmt.Errorf("请求报文解析失败: %w", err)
	}
	if req.URL == nil {
		req.URL = &url.URL{}
	}
	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}
	if req.URL.Host == "" && req.Host != "" {
		req.URL.Host = req.Host
	}
	req.RequestURI = ""
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	return req, nil
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
