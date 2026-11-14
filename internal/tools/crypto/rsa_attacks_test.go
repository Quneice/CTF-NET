// rsa_attacks.go 的验证：随机构造目标 → 攻击命中 → 断言分解与明文恢复。
package crypto

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

// findPrimeWhere 从 start 起找第一个素数 x 使 f(x) 也为素数。
func findPrimeWhere(start *big.Int, f func(*big.Int) *big.Int) *big.Int {
	x := new(big.Int).Set(start)
	for {
		if x.ProbablyPrime(20) && f(x).ProbablyPrime(20) {
			return x
		}
		x.Add(x, big.NewInt(1))
	}
}

func nextPrime(x *big.Int) *big.Int {
	x = new(big.Int).Add(x, big.NewInt(1))
	for !x.ProbablyPrime(20) {
		x.Add(x, big.NewInt(1))
	}
	return x
}

func mustStdout(t *testing.T, fn func(context.Context, map[string]any, *core.ToolContext) (*core.ToolResult, error), kv ...any) string {
	t.Helper()
	params := map[string]any{}
	for i := 0; i+1 < len(kv); i += 2 {
		params[kv[i].(string)] = kv[i+1]
	}
	res, err := fn(context.Background(), params, nil)
	if err != nil {
		t.Fatalf("runner error: %v", err)
	}
	return res.Stdout
}

func containsAll(t *testing.T, out string, subs ...string) {
	t.Helper()
	for _, s := range subs {
		if !strings.Contains(out, s) {
			t.Fatalf("输出缺少 %q：\n%s", s, out)
		}
	}
}

func TestRsaLowE(t *testing.T) {
	p, q := randPrime(256), randPrime(256)
	n := new(big.Int).Mul(p, q)
	e := big.NewInt(3)
	m := new(big.Int).SetBytes([]byte("flag{sm4ll_e}"))
	c := new(big.Int).Exp(m, e, n) // m^3 < n：精确根场景
	out := mustStdout(t, runRsaLowE, "n", n.String(), "e", "3", "c", c.String())
	containsAll(t, out, "flag{sm4ll_e}")

	// 带截断：22 字节明文（176 bit），m^3 ≈ 528 bit 略超 n → k = m^3/n 很小，扫描可命中
	m2 := new(big.Int).SetBytes([]byte("flag{p4dded_e_22bytes}"))
	c2 := new(big.Int).Exp(m2, e, n)
	if new(big.Int).Exp(m2, e, nil).Cmp(c2) == 0 {
		t.Skip("构造意外：未发生模截断")
	}
	out2 := mustStdout(t, runRsaLowE, "n", n.String(), "e", "3", "c", c2.String(), "k_limit", 300000)
	containsAll(t, out2, "flag{p4dded_e_22bytes}")
}

func TestRsaBroadcast(t *testing.T) {
	e := big.NewInt(3)
	m := new(big.Int).SetBytes([]byte("flag{hastad_broadcast}"))
	var lines []string
	one := big.NewInt(1)
	diversity := big.NewInt(0)
	for i := 0; i < 3; i++ {
		for {
			p, q := randPrime(128), randPrime(128)
			n := new(big.Int).Mul(p, q)
			if bigGcd(e, new(big.Int).Mul(new(big.Int).Sub(p, one), new(big.Int).Sub(q, one))).Cmp(one) != 0 {
				continue
			}
			c := new(big.Int).Exp(m, e, n) // m^3 超过单个 n，发生截断
			lines = append(lines, n.String()+","+c.String())
			diversity.Add(diversity, one)
			break
		}
	}
	if new(big.Int).Exp(m, e, nil).Cmp(big.NewInt(0)) <= 0 {
		t.Fatal("m^3 overflow big.Int?")
	}
	out := mustStdout(t, runRsaBroadcast, "e", "3", "pairs", strings.Join(lines, "\n"))
	containsAll(t, out, "flag{hastad_broadcast}")
}

func TestRsaFermatAttack(t *testing.T) {
	p := randPrime(64)
	q := nextPrime(new(big.Int).Add(p, big.NewInt(100000))) // |p−q| 小
	n := new(big.Int).Mul(p, q)
	e := big.NewInt(65537)
	phi := new(big.Int).Mul(new(big.Int).Sub(p, oneB()), new(big.Int).Sub(q, oneB()))
	d := new(big.Int).ModInverse(e, phi)
	m := new(big.Int).SetBytes([]byte("flag{fermat}"))
	c := new(big.Int).Exp(m, e, n)
	out := mustStdout(t, runRsaFermat, "n", n.String(), "e", "65537", "c", c.String())
	containsAll(t, out, p.String(), q.String(), d.String(), "flag{fermat}", "验证 p·q == n ✓")
	_ = d
}

func oneB() *big.Int { return big.NewInt(1) }

func TestRsaAlgebraicLinear(t *testing.T) {
	// q = 3p + 8（b 须为偶：a 奇 b 奇时 3p+b 为偶不可能为素），两者均素
	p := findPrimeWhere(new(big.Int).Lsh(big.NewInt(1), 128), func(x *big.Int) *big.Int {
		return new(big.Int).Add(new(big.Int).Mul(big.NewInt(3), x), big.NewInt(8))
	})
	q := new(big.Int).Add(new(big.Int).Mul(big.NewInt(3), p), big.NewInt(8))
	n := new(big.Int).Mul(p, q)
	e := big.NewInt(65537)
	m := new(big.Int).SetBytes([]byte("flag{linear}"))
	c := new(big.Int).Exp(m, e, new(big.Int).Mul(p, q))
	out := mustStdout(t, runRsaAlgebraic, "n", n.String(), "amode", "linear", "a", "3", "b", "8", "e", "65537", "c", c.String())
	containsAll(t, out, p.String(), q.String(), "flag{linear}")
}

func TestRsaAlgebraicSafePrime(t *testing.T) {
	// p = 2q + 1（q 为较小素数）
	q := findPrimeWhere(big.NewInt(1<<40), func(x *big.Int) *big.Int {
		return new(big.Int).Add(new(big.Int).Lsh(x, 1), big.NewInt(1))
	})
	p := new(big.Int).Add(new(big.Int).Lsh(q, 1), big.NewInt(1))
	n := new(big.Int).Mul(p, q)
	out := mustStdout(t, runRsaAlgebraic, "n", n.String(), "amode", "safe_prime")
	containsAll(t, out, p.String(), q.String(), "q = 2·p + 1")
}

func TestRsaAlgebraicDiffSum(t *testing.T) {
	p, q := randPrime(64), randPrime(64)
	n := new(big.Int).Mul(p, q)
	sum := new(big.Int).Add(p, q)
	out := mustStdout(t, runRsaAlgebraic, "n", n.String(), "amode", "diff_sum", "p_plus_q", sum.String())
	containsAll(t, out, p.String(), q.String())
	// p−q 场景
	diff := new(big.Int).Abs(new(big.Int).Sub(p, q))
	out2 := mustStdout(t, runRsaAlgebraic, "n", n.String(), "amode", "diff_sum", "p_minus_q", diff.String())
	containsAll(t, out2, p.String(), q.String())
}

func TestRsaAlgebraicCongruence(t *testing.T) {
	// p = q + 3·M，M = 2^20
	M := new(big.Int).Lsh(big.NewInt(1), 20)
	q := findPrimeWhere(big.NewInt(1<<40), func(x *big.Int) *big.Int {
		return new(big.Int).Add(x, new(big.Int).Mul(big.NewInt(3), M))
	})
	p := new(big.Int).Add(q, new(big.Int).Mul(big.NewInt(3), M))
	n := new(big.Int).Mul(p, q)
	out := mustStdout(t, runRsaAlgebraic, "n", n.String(), "amode", "congruence", "M", M.String(), "k_limit", "10")
	containsAll(t, out, p.String(), q.String(), "k = 3")
}

func TestRsaTwinAttack(t *testing.T) {
	p := findPrimeWhere(big.NewInt(1<<40), func(x *big.Int) *big.Int {
		return new(big.Int).Add(x, big.NewInt(2))
	})
	q := new(big.Int).Add(p, big.NewInt(2))
	n := new(big.Int).Mul(p, q)
	out := mustStdout(t, runRsaTwin, "n", n.String(), "max_gap", "4")
	containsAll(t, out, p.String(), q.String(), "gap = 2")
}

func TestRsaNextPrime(t *testing.T) {
	p := randPrime(48)
	q := randPrime(64)
	n := new(big.Int).Mul(p, q)
	base := new(big.Int).Sub(p, big.NewInt(7)) // p 在 base+7
	out := mustStdout(t, runRsaNextPrime, "n", n.String(), "base", base.String(), "window", "50")
	containsAll(t, out, p.String(), q.String())
}

func TestRsaPhiLeak(t *testing.T) {
	p, q := randPrime(128), randPrime(128)
	n := new(big.Int).Mul(p, q)
	e := big.NewInt(65537)
	one := big.NewInt(1)
	phi := new(big.Int).Mul(new(big.Int).Sub(p, one), new(big.Int).Sub(q, one))
	m := new(big.Int).SetBytes([]byte("flag{phi_leak}"))
	c := new(big.Int).Exp(m, e, n)

	out := mustStdout(t, runRsaPhiLeak, "n", n.String(), "phi", phi.String(), "e", "65537", "c", c.String())
	containsAll(t, out, p.String(), q.String(), "flag{phi_leak}")

	d := new(big.Int).ModInverse(e, phi)
	out2 := mustStdout(t, runRsaPhiLeak, "n", n.String(), "d", d.String(), "e", "65537", "c", c.String())
	containsAll(t, out2, p.String(), q.String(), "flag{phi_leak}")
}

func TestRsaSpecialFamilies(t *testing.T) {
	// fibonacci：p = 1597（F(17)，素数）
	q := randPrime(64)
	n := new(big.Int).Mul(big.NewInt(1597), q)
	out := mustStdout(t, runRsaSpecial, "n", n.String(), "family", "fibonacci", "limit", "50")
	containsAll(t, out, "1597", q.String())

	// lucas：p = 2207（L(16)，素数）
	q2 := randPrime(64)
	n2 := new(big.Int).Mul(big.NewInt(2207), q2)
	out2 := mustStdout(t, runRsaSpecial, "n", n2.String(), "family", "lucas", "limit", "50")
	containsAll(t, out2, "2207", q2.String())

	// mersenne：p = 89（89 | 2^11 − 1 = 2047）
	q3 := randPrime(64)
	n3 := new(big.Int).Mul(big.NewInt(89), q3)
	out3 := mustStdout(t, runRsaSpecial, "n", n3.String(), "family", "mersenne", "limit", "20")
	containsAll(t, out3, "89", q3.String())

	// factorial：p = 101（101! 含 101）
	q4 := randPrime(64)
	n4 := new(big.Int).Mul(big.NewInt(101), q4)
	out4 := mustStdout(t, runRsaSpecial, "n", n4.String(), "family", "factorial", "limit", "150")
	containsAll(t, out4, "101", q4.String())

	// fermat_numbers：F_5 = 2^32+1 = 641 × 6700417 → p = 641
	q5 := randPrime(64)
	n5 := new(big.Int).Mul(big.NewInt(641), q5)
	out5 := mustStdout(t, runRsaSpecial, "n", n5.String(), "family", "fermat_numbers", "limit", "10")
	containsAll(t, out5, "641", q5.String())
}

func TestRsaRhoAttack(t *testing.T) {
	p, q := randPrime(32), randPrime(32)
	n := new(big.Int).Mul(p, q)
	out := mustStdout(t, runRsaRho, "n", n.String())
	containsAll(t, out, p.String(), q.String(), "Pollard Rho")
}

func TestRsaPm1(t *testing.T) {
	// 构造 p−1 平滑：p = M·i + 1，M = 2^5·3^3·5^2·7
	M := new(big.Int).Mul(big.NewInt(32), big.NewInt(27))
	M.Mul(M, big.NewInt(25))
	M.Mul(M, big.NewInt(7))
	i := int64(1)
	var ps, qs *big.Int
	qs = randPrime(64)
	for {
		cand := new(big.Int).Mul(M, big.NewInt(i))
		cand.Add(cand, big.NewInt(1))
		if cand.ProbablyPrime(20) {
			ps = cand
			break
		}
		i++
	}
	n := new(big.Int).Mul(ps, qs)
	out := mustStdout(t, runRsaPm1, "n", n.String(), "b1", 100000)
	containsAll(t, out, ps.String(), qs.String(), "p−1")
}

func TestRsaSmallq(t *testing.T) {
	q := randPrime(64)
	n := new(big.Int).Mul(big.NewInt(1000003), q) // 1000003 为素数
	out := mustStdout(t, runRsaSmallq, "n", n.String(), "limit", 1000100)
	containsAll(t, out, "1000003", q.String())
}

func TestRsaFactordbMock(t *testing.T) {
	p, q := randPrime(64), randPrime(64)
	n := new(big.Int).Mul(p, q)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") != n.String() {
			t.Errorf("query 参数不符：%s", r.URL.RawQuery)
		}
		resp := map[string]any{
			"status":  "FF",
			"factors": [][]any{{p.String(), 1}, {q.String(), 1}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	out := mustStdout(t, runRsaFactordb, "n", n.String(), "endpoint", srv.URL)
	containsAll(t, out, p.String(), q.String(), "FactorDB")
}

func TestRsaAttacksRegistered(t *testing.T) {
	ids := []string{
		"crypto.rsa_low_e", "crypto.rsa_broadcast", "crypto.rsa_fermat", "crypto.rsa_algebraic",
		"crypto.rsa_twin", "crypto.rsa_next_prime", "crypto.rsa_phi_leak", "crypto.rsa_special",
		"crypto.rsa_rho", "crypto.rsa_pm1", "crypto.rsa_smallq", "crypto.rsa_factordb",
	}
	for _, id := range ids {
		if tool, ok := core.DefaultRegistry.Get(id); !ok || tool == nil {
			t.Errorf("tool %s not registered", id)
		}
	}
}
