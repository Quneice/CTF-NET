// 端到端有效性测试：穿过真实任务管线（注册表 → 参数校验 → Job 队轮询 → 结果）。
// 覆盖：RSA 攻击套件 12 工具、古典密码抽查、RsaCtfTool 内嵌真实 CTF 向量（PEM 公钥）。
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"ctfnet/internal/core"

	_ "ctfnet/internal/tools" // 注册全部工具
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CTF_DATA_DIR", dir)
	cfg := core.LoadConfig()
	executor := core.BuildExecutor(cfg)
	fileStore := core.NewFileStore(cfg.UploadsDir())
	sessions := core.NewSessionRegistry()
	tasks := core.NewTaskManager(executor, cfg.WorkDir(), cfg.ArtifactsDir(), fileStore, sessions)
	return &App{Reg: core.DefaultRegistry, Tasks: tasks, Sessions: sessions, Files: fileStore, Cfg: cfg}
}

// runE2E RunTool → 轮询 GetJob 至终态 → 返回 stdout；失败时 Fatal。
func runE2E(t *testing.T, app *App, toolID string, params map[string]any) string {
	t.Helper()
	resp, err := app.RunTool(toolID, params)
	if err != nil {
		t.Fatalf("RunTool(%s) 提交失败: %v", toolID, err)
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		job, err := app.GetJob(resp.JobID)
		if err != nil {
			t.Fatalf("GetJob 失败: %v", err)
		}
		status, _ := job["status"].(string)
		switch status {
		case "success":
			res, _ := job["result"].(map[string]any)
			out, _ := res["stdout"].(string)
			return out
		case "error", "timeout":
			t.Fatalf("任务 %s 终态 %s：%v", toolID, status, job["error"])
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("任务 %s 超时未到终态", toolID)
	return ""
}

func mustContain(t *testing.T, out string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(out, s) {
			t.Fatalf("输出缺少 %q:\n%s", s, out)
		}
	}
}

func e2eRandPrime(t *testing.T, bits int) *big.Int {
	t.Helper()
	p, err := rand.Prime(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func e2eNextPrime(x *big.Int) *big.Int {
	x = new(big.Int).Add(x, big.NewInt(1))
	for !x.ProbablyPrime(20) {
		x.Add(x, big.NewInt(1))
	}
	return x
}

func TestE2eRsaSuite(t *testing.T) {
	app := newTestApp(t)
	e := big.NewInt(65537)
	one := big.NewInt(1)

	t.Run("low_e_exact_root", func(t *testing.T) {
		n := new(big.Int).Mul(e2eRandPrime(t, 256), e2eRandPrime(t, 256))
		m := new(big.Int).SetBytes([]byte("flag{e2e_low_e}"))
		c := new(big.Int).Exp(m, big.NewInt(3), n)
		out := runE2E(t, app, "crypto.rsa_low_e", map[string]any{"n": n.String(), "e": "3", "c": c.String()})
		mustContain(t, out, "flag{e2e_low_e}")
	})

	t.Run("broadcast", func(t *testing.T) {
		m := new(big.Int).SetBytes([]byte("flag{e2e_hastad}"))
		var lines []string
		for i := 0; i < 3; i++ {
			n := new(big.Int).Mul(e2eRandPrime(t, 128), e2eRandPrime(t, 128))
			c := new(big.Int).Exp(m, big.NewInt(3), n)
			lines = append(lines, n.String()+","+c.String())
		}
		out := runE2E(t, app, "crypto.rsa_broadcast", map[string]any{"e": "3", "pairs": strings.Join(lines, "\n")})
		mustContain(t, out, "flag{e2e_hastad}")
	})

	t.Run("fermat", func(t *testing.T) {
		p := e2eRandPrime(t, 64)
		q := e2eNextPrime(new(big.Int).Add(p, big.NewInt(100000)))
		n := new(big.Int).Mul(p, q)
		phi := new(big.Int).Mul(new(big.Int).Sub(p, one), new(big.Int).Sub(q, one))
		d := new(big.Int).ModInverse(e, phi)
		msg := new(big.Int).SetBytes([]byte("flag{e2e_fermat}"))
		c := new(big.Int).Exp(msg, e, n)
		out := runE2E(t, app, "crypto.rsa_fermat", map[string]any{"n": n.String(), "e": "65537", "c": c.String()})
		mustContain(t, out, p.String(), q.String(), d.String(), "flag{e2e_fermat}")
	})

	t.Run("algebraic", func(t *testing.T) {
		p := new(big.Int).Set(big.NewInt(1 << 60))
		var q *big.Int
		for {
			p.Add(p, one)
			q = new(big.Int).Add(new(big.Int).Mul(big.NewInt(3), p), big.NewInt(8))
			if p.ProbablyPrime(20) && q.ProbablyPrime(20) {
				break
			}
		}
		out := runE2E(t, app, "crypto.rsa_algebraic", map[string]any{"n": new(big.Int).Mul(p, q).String(), "amode": "linear", "a": "3", "b": "8"})
		mustContain(t, out, p.String(), q.String())

		q2 := new(big.Int).Set(big.NewInt(1 << 60))
		var p2 *big.Int
		for {
			q2.Add(q2, one)
			p2 = new(big.Int).Add(new(big.Int).Lsh(q2, 1), one)
			if q2.ProbablyPrime(20) && p2.ProbablyPrime(20) {
				break
			}
		}
		out2 := runE2E(t, app, "crypto.rsa_algebraic", map[string]any{"n": new(big.Int).Mul(p2, q2).String(), "amode": "safe_prime"})
		mustContain(t, out2, p2.String(), q2.String())

		M := new(big.Int).Lsh(big.NewInt(1), 20)
		q3 := new(big.Int).Set(big.NewInt(1 << 40))
		var p3 *big.Int
		for {
			q3.Add(q3, one)
			p3 = new(big.Int).Add(q3, new(big.Int).Mul(big.NewInt(3), M))
			if q3.ProbablyPrime(20) && p3.ProbablyPrime(20) {
				break
			}
		}
		out3 := runE2E(t, app, "crypto.rsa_algebraic", map[string]any{"n": new(big.Int).Mul(p3, q3).String(), "amode": "congruence", "M": M.String(), "k_limit": 10})
		mustContain(t, out3, p3.String(), q3.String())
	})

	t.Run("twin_next_prime_phi_d", func(t *testing.T) {
		p := new(big.Int).Set(big.NewInt(1 << 40))
		var q *big.Int
		for {
			p.Add(p, one)
			q = new(big.Int).Add(p, big.NewInt(2))
			if p.ProbablyPrime(20) && q.ProbablyPrime(20) {
				break
			}
		}
		out := runE2E(t, app, "crypto.rsa_twin", map[string]any{"n": new(big.Int).Mul(p, q).String(), "max_gap": 4})
		mustContain(t, out, p.String())

		pn := e2eRandPrime(t, 48)
		qn := e2eRandPrime(t, 64)
		out2 := runE2E(t, app, "crypto.rsa_next_prime", map[string]any{"n": new(big.Int).Mul(pn, qn).String(), "base": new(big.Int).Sub(pn, big.NewInt(7)).String(), "window": 50})
		mustContain(t, out2, pn.String())

		pp, qq := e2eRandPrime(t, 128), e2eRandPrime(t, 128)
		n := new(big.Int).Mul(pp, qq)
		phi := new(big.Int).Mul(new(big.Int).Sub(pp, one), new(big.Int).Sub(qq, one))
		msg := new(big.Int).SetBytes([]byte("flag{e2e_phi}"))
		c := new(big.Int).Exp(msg, e, n)
		out3 := runE2E(t, app, "crypto.rsa_phi_leak", map[string]any{"n": n.String(), "phi": phi.String(), "e": "65537", "c": c.String()})
		mustContain(t, out3, pp.String(), "flag{e2e_phi}")
		d := new(big.Int).ModInverse(e, phi)
		out4 := runE2E(t, app, "crypto.rsa_phi_leak", map[string]any{"n": n.String(), "d": d.String(), "e": "65537", "c": c.String()})
		mustContain(t, out4, "flag{e2e_phi}")
	})

	t.Run("rho_pm1_smallq", func(t *testing.T) {
		p, q := e2eRandPrime(t, 32), e2eRandPrime(t, 32)
		out := runE2E(t, app, "crypto.rsa_rho", map[string]any{"n": new(big.Int).Mul(p, q).String()})
		mustContain(t, out, p.String(), q.String())

		q2 := e2eRandPrime(t, 64)
		out2 := runE2E(t, app, "crypto.rsa_smallq", map[string]any{"n": new(big.Int).Mul(big.NewInt(1000003), q2).String(), "limit": 1000100})
		mustContain(t, out2, "1000003")

		M := big.NewInt(75600) // 2^5·3^3·5^2·7
		i := int64(1)
		for {
			cand := new(big.Int).Add(new(big.Int).Mul(M, big.NewInt(i)), one)
			if cand.ProbablyPrime(20) {
				out3 := runE2E(t, app, "crypto.rsa_pm1", map[string]any{"n": new(big.Int).Mul(cand, q2).String(), "b1": 100000})
				mustContain(t, out3, cand.String())
				break
			}
			i++
		}
	})
}

func TestE2eClassical(t *testing.T) {
	app := newTestApp(t)

	t.Run("caesar_brute", func(t *testing.T) {
		out := runE2E(t, app, "crypto.caesar", map[string]any{"data": "Khoor Zruog", "shift": 3, "brute": true})
		mustContain(t, out, "shift  3: Hello World")
	})

	t.Run("vigenere_alpha", func(t *testing.T) {
		out := runE2E(t, app, "crypto.vigenere", map[string]any{"data": "LXFOPVEFRNHR", "key": "LEMON", "mode": "decrypt", "alphabet": "alpha"})
		mustContain(t, out, "ATTACKATDAWN")
	})

	t.Run("hill", func(t *testing.T) {
		out := runE2E(t, app, "crypto.hill", map[string]any{"data": "help", "k1": 3, "k2": 3, "k3": 2, "k4": 5, "mode": "encrypt"})
		mustContain(t, out, "hiat")
	})

	t.Run("columnar", func(t *testing.T) {
		out := runE2E(t, app, "crypto.columnar", map[string]any{"data": "attackatdawn", "key": "3", "mode": "encrypt"})
		mustContain(t, out, "aaaatctwtkdn")
	})

	t.Run("adfgx_roundtrip", func(t *testing.T) {
		enc := runE2E(t, app, "crypto.adfgx", map[string]any{"data": "ATTACKATDAWN", "key1": "ph0", "key2": "GERMAN", "mode": "encrypt"})
		dec := runE2E(t, app, "crypto.adfgx", map[string]any{"data": enc, "key1": "ph0", "key2": "GERMAN", "mode": "decrypt"})
		mustContain(t, dec, "ATTACKATDAWN")
	})
}

func parsePub(t *testing.T, pemStr string) (n *big.Int, e int) {
	t.Helper()
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		t.Fatal("PEM 解析失败")
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("x509 解析失败: %v", err)
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		t.Fatal("不是 RSA 公钥")
	}
	return pub.N, pub.E
}

const (
	vectorCubeRootPEM = `-----BEGIN PUBLIC KEY-----
MIIBIDANBgkqhkiG9w0BAQEFAAOCAQ0AMIIBCAKCAQEA6FqEbjr1AgKR+WtbpHa3
1kvsipKxGoKPWtZDCLnrzvwnyVJVdlyvKEYVqVGHhiuJU2RH+8oSQsGF/yjMaOzc
CxB5/cCrXAFere5nsN2SQsAEG8xS1ccn9YWoEfKAJrsdxUZd5CoSkwlQzvX01JMN
ap5u35o+emK3/ny5QdzZpoie0xp4l8uCFR/cp33cvZj2+VOP4ch6szpTG2u0h7sP
SfNvAHUqrZ8YscwkWEUk6N+55mQMviuLV8cqY1O9Lu+Q8yL5EtZj0vtxhb4Pj/ad
+GMzczpiZxZDjfpEVHaP67ntl7Ut8zhfWjQ69/Un7hjjdqQuh7GPGfhGd6ohbX6E
uQIBAw==
-----END PUBLIC KEY-----`
	vectorCubeRootC = `2205316413931134031074603746928247799030155221252519872650101242908540609117693035883827878696406295617513907962419726541451312273821810017858485722109359971259158071688912076249144203043097720816270550387459717116098817458584146690177125`
	vectorFermatPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCG6ZYBPnfEFpkADglB1IDARrL3
Gk+Vs1CsGk1CY3KSPYpFYdlvv7AkBZWQcgGtMiXPbt7X3gLZHDhv+sKAty0Plcrn
H0Lr4NPtrqznzqMZX6MsHGCA2Q74U9Bt1Fcskrn4MQu8DGNaXiaVJRF1EDCmWQgW
VU52MDG8uzHj8RnGXwIDAQAB
-----END PUBLIC KEY-----`
	vectorFibPEM = `-----BEGIN PUBLIC KEY-----
MIGWMA0GCSqGSIb3DQEBAQUAA4GEADCBgAJ5C/QyoZTftkv7F7HOOWqxiRTnW3Sa
mWXYEKiCOio1vK3Xh/HMJdJZ5JsOwd27OTvlBw5eLEjsJfjT0PQR/ULJujjvf35q
4EYr3aw1U4JVcy8h2eyb61AhNDc1GL2YXOIkjUbpj+8I0fKpDjPesfa0h5yhTluo
x5AlBQIDAQAB
-----END PUBLIC KEY-----`
)

func TestE2eRsaCtfToolVectors(t *testing.T) {
	app := newTestApp(t)

	t.Run("cube_root_real_cipher", func(t *testing.T) {
		n, e := parsePub(t, vectorCubeRootPEM)
		out := runE2E(t, app, "crypto.rsa_low_e", map[string]any{"n": n.String(), "e": strconv.Itoa(e), "c": vectorCubeRootC})
		mustContain(t, out, "明文 m")
	})

	t.Run("fermat_real_key", func(t *testing.T) {
		n, _ := parsePub(t, vectorFermatPEM)
		out := runE2E(t, app, "crypto.rsa_fermat", map[string]any{"n": n.String()})
		mustContain(t, out, "p = ", "验证 p·q == n ✓")
	})

	t.Run("fibonacci_real_key", func(t *testing.T) {
		n, _ := parsePub(t, vectorFibPEM)
		out := runE2E(t, app, "crypto.rsa_special", map[string]any{"n": n.String(), "family": "fibonacci", "limit": 10000})
		mustContain(t, out, "p = ", "验证 p·q == n ✓")
	})
}
