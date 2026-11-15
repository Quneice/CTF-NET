package proxy

// TLS MITM:内置 CA(持久化 DataDir/certs)+ 按域签发证书 + CONNECT 内 TLS 终止,
// 使 HTTPS 流量进入与 HTTP 同一套捕获/拦截/历史/重放管线。
// CA 私钥仅存本地数据目录(不提交 git);客户端需信任 CA(或内置浏览器 --ignore-certificate-errors)。

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// caCertFileName / caKeyFileName CA 文件固定名。
const (
	caCertFileName = "ca.crt"
	caKeyFileName  = "ca.key"
)

// EnsureCA 确保 caDir 下存在 CA(不存在则生成),返回证书文件路径。
func EnsureCA(caDir string) (string, error) {
	certFile := filepath.Join(caDir, caCertFileName)
	keyFile := filepath.Join(caDir, caKeyFileName)
	if _, err := os.Stat(certFile); err == nil {
		if _, kerr := os.Stat(keyFile); kerr == nil {
			return certFile, nil
		}
		return "", fmt.Errorf("CA 证书已存在但私钥缺失(手动删除 %s 后重试可重新生成)", certFile)
	}
	if err := os.MkdirAll(caDir, 0o755); err != nil {
		return "", fmt.Errorf("创建 CA 目录失败: %w", err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("CA 私钥生成失败: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", fmt.Errorf("CA 序列号生成失败: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "CTF-NET Proxy CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", fmt.Errorf("CA 自签失败: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", fmt.Errorf("CA 私钥编码失败: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certFile, certPEM, 0o644); err != nil {
		return "", fmt.Errorf("写入 CA 证书失败: %w", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		return "", fmt.Errorf("写入 CA 私钥失败: %w", err)
	}
	return certFile, nil
}

// loadCA 加载 CA 为 tls.Certificate。
func loadCA(certFile, keyFile string) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("加载 CA 失败: %w", err)
	}
	return &cert, nil
}

// SetCA 为代理装载 MITM CA(不存在则现场生成);之后可开关 MITM。
func (s *Server) SetCA(caDir string) error {
	certFile, err := EnsureCA(caDir)
	if err != nil {
		return err
	}
	cert, err := loadCA(certFile, filepath.Join(caDir, caKeyFileName))
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.caDir = caDir
	s.caFile = certFile
	s.caCert = cert
	s.mu.Unlock()
	return nil
}

// SetMITM 开关 HTTPS 解密;CA 未装载时开启返回错误。
func (s *Server) SetMITM(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if on && s.caCert == nil {
		return errors.New("MITM CA 未装载,无法开启 HTTPS 解密")
	}
	s.mitm = on
	return nil
}

// hostCert 按域签发(缓存)证书:CN/SAN=host,CA 签名,有效期 1 年。
func (s *Server) hostCert(host string) (*tls.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.caCert == nil {
		return nil, errors.New("MITM CA 未装载")
	}
	if c, ok := s.hostCerts[host]; ok {
		return c, nil
	}
	caCert, err := x509.ParseCertificate(s.caCert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("解析 CA 证书失败: %w", err)
	}
	caKey, ok := s.caCert.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("CA 私钥类型异常(非 ECDSA)")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("域私钥生成失败: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("域证书序列号失败: %w", err)
	}
	// host 可能是 IP(如 https://192.168.1.1):IP 走 IPAddresses SAN,域名走 DNSNames SAN
	dnsNames := []string{}
	ipAddrs := []net.IP{}
	if ip := net.ParseIP(host); ip != nil {
		ipAddrs = append(ipAddrs, ip)
	} else {
		dnsNames = append(dnsNames, host)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		DNSNames:     dnsNames,
		IPAddresses:  ipAddrs,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("域证书签发失败: %w", err)
	}
	cert := &tls.Certificate{
		Certificate: [][]byte{der, s.caCert.Certificate[0]},
		PrivateKey:  key,
	}
	if s.hostCerts == nil {
		s.hostCerts = make(map[string]*tls.Certificate)
	}
	s.hostCerts[host] = cert
	return cert, nil
}

// serveMITM CONNECT 后接管 TLS:握手(按域证书)→ 内层请求走同一 ServeHTTP 管线(https)。
func (s *Server) serveMITM(clientConn net.Conn, host string, rec *Record) {
	cert, err := s.hostCert(host)
	if err != nil {
		rec.Note = "MITM 域证书签发失败: " + err.Error()
		s.add(rec)
		_ = clientConn.Close()
		return
	}
	tlsConn := tls.Server(clientConn, &tls.Config{
		Certificates: []tls.Certificate{*cert},
		NextProtos:   []string{"http/1.1"},
	})
	if err := tlsConn.Handshake(); err != nil {
		rec.Note = "TLS 握手失败(客户端未信任 CTF-NET Proxy CA?): " + err.Error()
		s.add(rec)
		_ = tlsConn.Close()
		return
	}
	// 内层请求:origin-form + scheme 补 https,复用完整管线(捕获/拦截/历史)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Scheme == "" {
			r.URL.Scheme = "https"
		}
		s.ServeHTTP(w, r)
	})}
	_ = srv.Serve(&singleConnListener{conn: tlsConn})
	rec.Note = "TLS MITM 会话结束"
	s.add(rec)
}

// singleConnListener 把单条已握手连接包装为 net.Listener,供 http.Server.Serve 消费。
type singleConnListener struct {
	conn net.Conn
	once sync.Once
	done chan struct{}
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	if l.conn == nil {
		<-l.done
		return nil, net.ErrClosed
	}
	c := l.conn
	l.conn = nil
	l.once.Do(func() { l.done = make(chan struct{}) })
	return c, nil
}

func (l *singleConnListener) Close() error {
	l.once.Do(func() {
		if l.done != nil {
			close(l.done)
		}
	})
	return nil
}

func (l *singleConnListener) Addr() net.Addr { return l.conn.LocalAddr() }

// hostOnly 从 host:port 中剥离端口(证书 CN/DNS 用纯域名)。
func hostOnly(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return strings.Trim(host, "[]")
}
