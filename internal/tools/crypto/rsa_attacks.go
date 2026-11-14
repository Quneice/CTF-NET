// crypto.rsa_* —— RSA 攻击套件（算法参考 RsaCtfTool /src/RsaCtfTool/attacks，纯 Go 实现）。
// 覆盖：小指数开根 / Hastad 广播 / Fermat 分解 / 代数关系（线性·比例·差和·同余·安全素数）/
//
//	孪生素数 gap 扫描 / next_prime / φ 或 d 泄露 / 特殊数 gcd（Fibonacci·Lucas·Mersenne·
//	Fermat 数·阶乘·素数积·合数积）/ Pollard Rho(Brent) / Pollard p−1 / 小素数试除 / FactorDB。
//
// 所有分解类工具输出统一报告：p、q、φ、私钥 d（给 e 时）、明文 m（给 c 时，含 UTF-8 解码）+ 自验证。
package crypto

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"ctfnet/internal/core"
)

// ---------------------------------------------------------------------------
// 共享助手
// ---------------------------------------------------------------------------

// clBig 解析表单大整数参数（十进制 / 0x 十六进制），缺失或非法返回 nil。
func clBig(params map[string]any, name string) *big.Int {
	raw := strings.TrimSpace(clStr(params, name))
	if raw == "" {
		return nil
	}
	neg := false
	if strings.HasPrefix(raw, "-") {
		neg = true
		raw = raw[1:]
	}
	base := 10
	if strings.HasPrefix(raw, "0x") || strings.HasPrefix(raw, "0X") {
		base = 16
		raw = raw[2:]
	}
	v, ok := new(big.Int).SetString(raw, base)
	if !ok {
		return nil
	}
	if neg {
		v.Neg(v)
	}
	return v
}

// bigIroot 整数 k 次方根（向下取整，牛顿迭代）。
func bigIroot(a *big.Int, k int64) *big.Int {
	if a.Sign() < 0 || k < 1 {
		return big.NewInt(0)
	}
	if k == 1 || a.Cmp(big.NewInt(1)) <= 0 {
		return new(big.Int).Set(a)
	}
	// 初值 2^(ceil(bits/k)) ≥ 真根
	bits := int64(a.BitLen())
	x := new(big.Int).Lsh(big.NewInt(1), uint((bits+k-1)/k))
	kb := big.NewInt(k)
	for {
		// 牛顿：x' = ((k−1)·x^k + a) / (k·x^(k−1))
		xk1 := new(big.Int).Exp(x, big.NewInt(k-1), nil)
		xk := new(big.Int).Mul(xk1, x)
		t := new(big.Int).Mul(xk, big.NewInt(k-1))
		t.Add(t, a)
		t.Div(t, xk1)
		t.Div(t, kb)
		if t.Cmp(x) >= 0 {
			return x
		}
		x = t
	}
}

func bigIsqrt(a *big.Int) *big.Int { return bigIroot(a, 2) }

func bigIsPerfectSquare(a *big.Int) (*big.Int, bool) {
	if a.Sign() < 0 {
		return nil, false
	}
	r := bigIsqrt(a)
	sq := new(big.Int).Mul(r, r)
	return r, sq.Cmp(a) == 0
}

func bigGcd(a, b *big.Int) *big.Int {
	return new(big.Int).GCD(nil, nil, new(big.Int).Abs(a), new(big.Int).Abs(b))
}

// rsaUTF8 尝试把明文字节解码为可打印 UTF-8 文本（flag 常见形态）。
func rsaUTF8(m *big.Int) string {
	if m == nil || m.Sign() <= 0 {
		return ""
	}
	b := make([]byte, 0, (m.BitLen()+7)/8)
	v := new(big.Int).Set(m)
	zero := big.NewInt(0)
	mod := big.NewInt(256)
	var rev []byte
	for v.Cmp(zero) > 0 {
		rev = append(rev, byte(new(big.Int).Mod(v, mod).Int64()))
		v.Div(v, mod)
	}
	for i := len(rev) - 1; i >= 0; i-- {
		b = append(b, rev[i])
	}
	if !utf8.Valid(b) {
		return ""
	}
	s := string(b)
	for _, r := range s {
		if r < 0x20 && r != '\t' {
			return ""
		}
	}
	return s
}

// rsaFinish 分解成功后的统一报告：φ、d（给 e）、m（给 c）、UTF-8、自验证。
func rsaFinish(n, p, q, e, c *big.Int, extra ...string) (string, error) {
	if p == nil || q == nil || p.Sign() <= 0 || q.Sign() <= 0 || new(big.Int).Mul(p, q).Cmp(n) != 0 {
		return "", fmt.Errorf("内部错误：p·q ≠ n")
	}
	one := big.NewInt(1)
	phi := new(big.Int).Mul(new(big.Int).Sub(p, one), new(big.Int).Sub(q, one))

	var lines []string
	lines = append(lines, extra...)
	lines = append(lines,
		"p = "+p.String(),
		"q = "+q.String(),
		"φ(n) = "+phi.String(),
		"验证 p·q == n ✓",
	)

	if e != nil && e.Cmp(one) > 0 {
		if g := bigGcd(e, phi); g.Cmp(one) == 0 {
			d := new(big.Int).ModInverse(e, phi)
			lines = append(lines, "私钥 d = "+d.String())
			if c != nil && c.Sign() >= 0 {
				m := new(big.Int).Exp(c, d, n)
				lines = append(lines,
					"明文 m（十进制）= "+m.String(),
					"明文 m（十六进制）= 0x"+m.Text(16),
				)
				if s := rsaUTF8(m); s != "" {
					lines = append(lines, "明文 m（UTF-8）= "+s)
				}
				ok := new(big.Int).Exp(m, e, n).Cmp(new(big.Int).Mod(c, n)) == 0
				lines = append(lines, fmt.Sprintf("验证 m^e mod n == c：%s", map[bool]string{true: "✓", false: "✗"}[ok]))
			}
		} else {
			lines = append(lines, fmt.Sprintf("提示：gcd(e, φ) = %s ≠ 1，无法求私钥（e 与 φ 不互素）", g.String()))
		}
	}
	return strings.Join(lines, "\n"), nil
}

// failAttack 攻击未成功的统一输出（附建议）。
func failAttack(tool, hint string) (*core.ToolResult, error) {
	return textOut(strings.Join([]string{
		"攻击未成功：" + tool,
		"",
		hint,
		"",
		"可尝试：FactorDB 在线查询（若 n 曾被分解过）、Pollard Rho、p−1（平滑时）、Fermat（|p−q| 小）、Wiener（d 小）、共模攻击（同 n 双密文）等。",
	}, "\n"))
}

// rsaCtx 长循环里的取消检查点。
func rsaCtx(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// randPrime 生成指定位数的素数（测试用）。
func randPrime(bits int) *big.Int {
	for {
		p, err := rand.Prime(rand.Reader, bits)
		if err == nil {
			return p
		}
	}
}

// ---------------------------------------------------------------------------
// 1. 小指数开根（crypto.rsa_low_e）：m^e < n（或略超，c + k·n 扫描）
// ---------------------------------------------------------------------------

func rsaLowEAttack(ctx context.Context, n, e, c *big.Int, kLimit int) (*big.Int, error) {
	if c == nil {
		return nil, fmt.Errorf("小指数攻击需要密文 c")
	}
	if e == nil || e.Cmp(big.NewInt(2)) < 0 {
		return nil, fmt.Errorf("请提供公钥指数 e（小指数如 3、5、7）")
	}
	k64 := e.Int64()
	if k64 < 2 || k64 > 1<<20 {
		return nil, fmt.Errorf("e 需在 2 至 2^20 之间")
	}
	// 精确 e 次根
	m := bigIroot(c, k64)
	if new(big.Int).Exp(m, e, nil).Cmp(c) == 0 {
		return m, nil
	}
	// m^e = c + k·n 扫描（带填充的小消息）
	for k := 1; k <= kLimit; k++ {
		if err := rsaCtx(ctx); err != nil {
			return nil, err
		}
		t := new(big.Int).Mul(big.NewInt(int64(k)), n)
		t.Add(t, c)
		m = bigIroot(t, k64)
		if new(big.Int).Exp(m, e, nil).Cmp(t) == 0 {
			return m, nil
		}
	}
	return nil, fmt.Errorf("未找到 e 次根（明文不够小或填充过大，k 扫描上限 %d）", kLimit)
}

func runRsaLowE(ctx context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	e := clBig(params, "e")
	c := clBig(params, "c")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	kLimit := clInt(params, "k_limit", 100000)
	m, err := rsaLowEAttack(ctx, n, e, c, kLimit)
	if err != nil {
		return nil, err
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("m^%s 未发生模截断（或 c + k·n 命中）", e.String()),
		"明文 m（十进制）= "+m.String(),
		"明文 m（十六进制）= 0x"+m.Text(16))
	if s := rsaUTF8(m); s != "" {
		lines = append(lines, "明文 m（UTF-8）= "+s)
	}
	return textOut(strings.Join(lines, "\n"))
}

// ---------------------------------------------------------------------------
// 2. Hastad 广播攻击（crypto.rsa_broadcast）：同 e 多组 (n_i, c_i)，CRT 合并后开 e 次根
// ---------------------------------------------------------------------------

func runRsaBroadcast(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	e := clBig(params, "e")
	if e == nil || e.Cmp(big.NewInt(2)) < 0 {
		return errOut("请输入公钥指数 e（广播攻击通常 e=3，需至少 e 组密文）")
	}
	raw := clStr(params, "pairs")
	if strings.TrimSpace(raw) == "" {
		return errOut("请输入广播密文对（每行一对：n,c 支持十进制或 0x）")
	}
	type pair struct{ n, c *big.Int }
	var pairs []pair
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == '，' || r == ':' || r == ' ' || r == '\t' })
		if len(parts) < 2 {
			return errOut(fmt.Sprintf("第 %d 行格式错误：每行一对，逗号分隔 n,c", i+1))
		}
		n, c := parts[0], parts[1]
		bn, bc := clBig(map[string]any{"x": n}, "x"), clBig(map[string]any{"x": c}, "x")
		if bn == nil || bc == nil || bn.Cmp(big.NewInt(1)) <= 0 {
			return errOut(fmt.Sprintf("第 %d 行的 n 或 c 无法解析", i+1))
		}
		pairs = append(pairs, pair{bn, bc})
	}
	if len(pairs) < int(e.Int64()) {
		return errOut(fmt.Sprintf("广播攻击至少需要 e=%d 组密文，当前 %d 组", e.Int64(), len(pairs)))
	}
	// CRT：M = Σ c_i · N_i · inv(N_i, n_i)，N = Π n_i
	N := big.NewInt(1)
	for _, pr := range pairs {
		N.Mul(N, pr.n)
	}
	M := big.NewInt(0)
	for _, pr := range pairs {
		Ni := new(big.Int).Div(N, pr.n)
		inv := new(big.Int).ModInverse(Ni, pr.n)
		if inv == nil {
			return errOut("模数之间不互素，无法 CRT 合并（检查 n 是否有重复）")
		}
		t := new(big.Int).Mul(pr.c, Ni)
		t.Mul(t, inv)
		M.Add(M, t)
	}
	M.Mod(M, N)
	k64 := e.Int64()
	m := bigIroot(M, k64)
	if new(big.Int).Exp(m, e, nil).Cmp(M) != 0 {
		return errOut("CRT 合并后非完美 e 次根：各组密文可能不是同一明文，或 e 不匹配")
	}
	lines := []string{
		fmt.Sprintf("Hastad 广播攻击成功：%d 组密文 CRT 合并", len(pairs)),
		"明文 m（十进制）= " + m.String(),
		"明文 m（十六进制）= 0x" + m.Text(16),
	}
	if s := rsaUTF8(m); s != "" {
		lines = append(lines, "明文 m（UTF-8）= "+s)
	}
	return textOut(strings.Join(lines, "\n"))
}

// ---------------------------------------------------------------------------
// 3. Fermat 分解（crypto.rsa_fermat）：n = a² − b²
// ---------------------------------------------------------------------------

func runRsaFermat(ctx context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	e := clBig(params, "e")
	c := clBig(params, "c")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	maxIter := clInt(params, "max_iter", 1000000)
	if new(big.Int).Mod(n, big.NewInt(2)).Sign() == 0 {
		return errOut("n 为偶数：p = 2，q = " + new(big.Int).Div(n, big.NewInt(2)).String())
	}
	a := bigIsqrt(n)
	if new(big.Int).Mul(a, a).Cmp(n) < 0 {
		a.Add(a, big.NewInt(1))
	}
	for i := 0; i < maxIter; i++ {
		if err := rsaCtx(ctx); err != nil {
			return nil, err
		}
		b2 := new(big.Int).Mul(a, a)
		b2.Sub(b2, n)
		if b, ok := bigIsPerfectSquare(b2); ok {
			p := new(big.Int).Sub(a, b)
			q := new(big.Int).Add(a, b)
			out, err := rsaFinish(n, p, q, e, c, fmt.Sprintf("Fermat 分解：第 %d 次迭代命中（|p−q| 较小）", i+1))
			if err != nil {
				return nil, err
			}
			return textOut(out)
		}
		a.Add(a, big.NewInt(1))
	}
	return failAttack("Fermat 分解在 "+strconv.Itoa(maxIter)+" 次迭代内未收敛", "|p−q| 可能过大。")
}

// ---------------------------------------------------------------------------
// 4. 代数关系（crypto.rsa_algebraic）：线性 / 比例 / 差和 / 同余 / 安全素数
// ---------------------------------------------------------------------------

func runRsaAlgebraic(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	e := clBig(params, "e")
	c := clBig(params, "c")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	mode := clStr(params, "amode")
	if mode == "" {
		mode = "linear"
	}
	kLimit := clInt(params, "k_limit", 100000)
	two := big.NewInt(2)

	tryPQ := func(p, q *big.Int, hit string) (*core.ToolResult, error) {
		if p == nil || p.Sign() <= 0 || q == nil || q.Sign() <= 0 {
			return nil, fmt.Errorf("no")
		}
		if new(big.Int).Mul(p, q).Cmp(n) != 0 {
			return nil, fmt.Errorf("no")
		}
		out, err := rsaFinish(n, p, q, e, c, hit)
		if err != nil {
			return nil, err
		}
		return textOut(out)
	}

	switch mode {
	case "linear", "safe_prime":
		var aI, bI int64 = 1, 1
		if mode == "safe_prime" {
			aI, bI = 2, 1 // p = 2q + 1：n = q(2q+1) → 2q² + q − n = 0
		} else {
			if av := clBig(params, "a"); av != nil {
				aI = av.Int64()
			}
			if bv := clBig(params, "b"); bv != nil {
				bI = bv.Int64()
			}
			if aI == 0 {
				aI = 1
			}
			if aI < 1 {
				return errOut("线性关系 a 需为正整数")
			}
		}
		// a·p² + b·p − n = 0 → p = (−b + isqrt(b² + 4an)) / (2a)（取使 p|n 的根）
		A, B := big.NewInt(aI), big.NewInt(bI)
		disc := new(big.Int).Mul(B, B)
		disc.Add(disc, new(big.Int).Mul(new(big.Int).Lsh(A, 2), n)) // b² + 4an
		r, ok := bigIsPerfectSquare(disc)
		if !ok {
			return failAttack("线性/安全素数关系不成立（判别式非完全平方）",
				fmt.Sprintf("检查 a=%d、b=%d 是否正确，n 是否满足 q = a·p + b。", aI, bI))
		}
		num := new(big.Int).Sub(r, B)
		den := new(big.Int).Mul(two, A)
		if new(big.Int).Mod(num, den).Sign() != 0 {
			return failAttack("判别式为完全平方但整除失败", "a、b 与 n 的关系可能不精确成立。")
		}
		p := new(big.Int).Div(num, den)
		q := new(big.Int).Div(n, p)
		res, err := tryPQ(p, q, fmt.Sprintf("代数关系命中：q = %d·p + %d 精确成立", aI, bI))
		if err != nil {
			return failAttack("求出的 p 不能整除 n", "a、b 与 n 的关系可能不精确成立（近似关系需 Coppersmith，本工具不支持）。")
		}
		return res, nil

	case "ratio":
		// a·q = b·p → a·n = b·p² → p² = a·n / b
		av, bv := clBig(params, "a"), clBig(params, "b")
		if av == nil || bv == nil || av.Sign() <= 0 || bv.Sign() <= 0 {
			return errOut("比例模式需要 a、b（a·q = b·p，即 q/p = b/a）")
		}
		num := new(big.Int).Mul(av, n)
		if new(big.Int).Mod(num, bv).Sign() != 0 {
			return failAttack("a·n 不能被 b 整除", "比例关系 a·q = b·p 不精确成立。")
		}
		p2 := new(big.Int).Div(num, bv)
		p, ok := bigIsPerfectSquare(p2)
		if !ok {
			return failAttack("a·n/b 非完全平方", "比例关系不精确成立（近似比例需其他方法）。")
		}
		q := new(big.Int).Div(n, p)
		res, err := tryPQ(p, q, fmt.Sprintf("比例关系命中：q/p = %s/%s", bv.String(), av.String()))
		if err != nil {
			return failAttack("求出的 p 不能整除 n", "比例关系不精确成立。")
		}
		return res, nil

	case "diff_sum":
		s := clBig(params, "p_plus_q")
		d := clBig(params, "p_minus_q")
		if s == nil && d == nil {
			return errOut("差和模式需要 p_plus_q（p+q）或 p_minus_q（p−q）至少其一")
		}
		if s == nil {
			// (p+q)² = (p−q)² + 4n
			t := new(big.Int).Mul(d, d)
			t.Add(t, new(big.Int).Lsh(n, 2))
			r, ok := bigIsPerfectSquare(t)
			if !ok {
				return failAttack("由 p−q 推 p+q 失败（d²+4n 非完全平方）", "检查 p−q 的值。")
			}
			s = r
		}
		// (p−q)² = (p+q)² − 4n
		t := new(big.Int).Mul(s, s)
		t.Sub(t, new(big.Int).Lsh(n, 2))
		r, ok := bigIsPerfectSquare(t)
		if !ok {
			return failAttack("s² − 4n 非完全平方", "p+q 的值可能不正确。")
		}
		if new(big.Int).Mod(new(big.Int).Add(s, r), two).Sign() != 0 {
			return failAttack("s 与 √(s²−4n) 奇偶不一致", "p+q 的值可能不正确。")
		}
		p := new(big.Int).Div(new(big.Int).Add(s, r), two)
		q := new(big.Int).Div(new(big.Int).Sub(s, r), two)
		res, err := tryPQ(p, q, "差/和已知：由 (p+q)² − 4n = (p−q)² 直接分解")
		if err != nil {
			return failAttack("求出的 p·q ≠ n", "p±q 的值可能不正确。")
		}
		return res, nil

	case "congruence":
		// p = q + k·M → q² + kM·q − n = 0 → q = (−kM + isqrt((kM)²+4n)) / 2
		M := clBig(params, "M")
		if M == nil || M.Sign() <= 0 {
			return errOut("同余模式需要 M（p ≡ q (mod M)）")
		}
		for k := int64(1); k <= int64(kLimit); k++ {
			kM := new(big.Int).Mul(M, big.NewInt(k))
			disc := new(big.Int).Mul(kM, kM)
			disc.Add(disc, new(big.Int).Lsh(n, 2))
			r, ok := bigIsPerfectSquare(disc)
			if !ok {
				continue
			}
			num := new(big.Int).Sub(r, kM)
			if num.Sign() <= 0 || new(big.Int).Mod(num, two).Sign() != 0 {
				continue
			}
			q := new(big.Int).Div(num, two)
			if q.Sign() <= 0 || new(big.Int).Mod(n, q).Sign() != 0 {
				continue
			}
			p := new(big.Int).Div(n, q)
			res, err := tryPQ(p, q, fmt.Sprintf("同余关系命中：k = %d（p = q + %d·M）", k, k))
			if err == nil {
				return res, nil
			}
		}
		return failAttack(fmt.Sprintf("k 扫描至 %d 未命中", kLimit), "p ≡ q (mod M) 中的 k 可能过大或 M 不正确。")

	default:
		return errOut("未知代数模式：" + mode + "（linear | ratio | diff_sum | congruence | safe_prime）")
	}
}

// ---------------------------------------------------------------------------
// 5. 孪生/相邻素数（crypto.rsa_twin）：p·(p+g) = n → (2p+g)² = 4n + g²
// ---------------------------------------------------------------------------

func runRsaTwin(ctx context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	e := clBig(params, "e")
	c := clBig(params, "c")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	maxGap := clInt(params, "max_gap", 100000)
	// 4n + g² 为完全平方 → p = (√ − g) / 2
	base := new(big.Int).Lsh(n, 2) // 4n
	for g := 1; g <= maxGap; g++ {
		if err := rsaCtx(ctx); err != nil {
			return nil, err
		}
		t := new(big.Int).Add(base, new(big.Int).Mul(big.NewInt(int64(g)), big.NewInt(int64(g))))
		r, ok := bigIsPerfectSquare(t)
		if !ok {
			continue
		}
		num := new(big.Int).Sub(r, big.NewInt(int64(g)))
		if num.Sign() <= 0 || num.Bit(0) != 0 {
			continue
		}
		p := new(big.Int).Div(num, big.NewInt(2))
		if p.Sign() <= 0 || new(big.Int).Mod(n, p).Sign() != 0 {
			continue
		}
		q := new(big.Int).Div(n, p)
		out, err := rsaFinish(n, p, q, e, c, fmt.Sprintf("相邻素数命中：gap = %d（p 与 p+%d）", g, g))
		if err == nil {
			return textOut(out)
		}
	}
	return failAttack(fmt.Sprintf("gap 扫描至 %d 未命中", maxGap), "p、q 间距可能过大。")
}

// ---------------------------------------------------------------------------
// 6. next_prime（crypto.rsa_next_prime）：p 是 base 附近的下一个素数（或 base 附近的因子）
// ---------------------------------------------------------------------------

func runRsaNextPrime(ctx context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	e := clBig(params, "e")
	c := clBig(params, "c")
	base := clBig(params, "base")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	if base == nil || base.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入基准 base（p 位于 base 附近的素数，如 p = next_prime(base)）")
	}
	window := clInt(params, "window", 100000)
	// 优先精确素数候选，同时兜底任何整除 n 的数
	cand := new(big.Int).Set(base)
	for i := 0; i < window; i++ {
		if err := rsaCtx(ctx); err != nil {
			return nil, err
		}
		if cand.ProbablyPrime(20) && new(big.Int).Mod(n, cand).Sign() == 0 {
			p := cand
			q := new(big.Int).Div(n, p)
			out, err := rsaFinish(n, p, q, e, c, fmt.Sprintf("next_prime 命中：base+%d（%d 步内）", i, i))
			if err == nil {
				return textOut(out)
			}
		}
		cand.Add(cand, big.NewInt(1))
	}
	return failAttack(fmt.Sprintf("base 起 %d 个候选中无素因子", window), "base 可能偏差过大，或 p 并非 base 附近的素数。")
}

// ---------------------------------------------------------------------------
// 7. φ / d 泄露（crypto.rsa_phi_leak）
// ---------------------------------------------------------------------------

func runRsaPhiLeak(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	e := clBig(params, "e")
	c := clBig(params, "c")
	phi := clBig(params, "phi")
	d := clBig(params, "d")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	if phi == nil && d == nil {
		return errOut("请提供 phi（φ(n)）或 d（私钥）至少其一")
	}
	if phi == nil {
		// 由 d 恢复 φ：e·d − 1 = k·φ，φ < n → k = (e·d−1)/n 附近
		if e == nil || e.Cmp(big.NewInt(1)) <= 0 {
			return errOut("由 d 恢复需要同时提供 e")
		}
		ed1 := new(big.Int).Mul(e, d)
		ed1.Sub(ed1, big.NewInt(1))
		kMax := new(big.Int).Div(ed1, n)
		kMax.Add(kMax, big.NewInt(2))
		one := big.NewInt(1)
		for k := int64(1); k <= kMax.Int64() && k < 1<<26; k++ {
			kb := big.NewInt(k)
			if new(big.Int).Mod(ed1, kb).Sign() != 0 {
				continue
			}
			phiTry := new(big.Int).Div(ed1, kb)
			s := new(big.Int).Sub(n, phiTry)
			s.Add(s, one) // p+q
			disc := new(big.Int).Mul(s, s)
			disc.Sub(disc, new(big.Int).Lsh(n, 2))
			if r, ok := bigIsPerfectSquare(disc); ok {
				p := new(big.Int).Add(s, r)
				p.Div(p, big.NewInt(2))
				q := new(big.Int).Sub(s, r)
				q.Div(q, big.NewInt(2))
				if p.Sign() > 0 && new(big.Int).Mul(p, q).Cmp(n) == 0 {
					out, err := rsaFinish(n, p, q, e, c, fmt.Sprintf("由 d 恢复成功：k = %d → φ = (e·d−1)/k", k))
					if err == nil {
						return textOut(out)
					}
				}
			}
		}
		return failAttack("由 d 恢复 φ 失败", "k 扫描范围内未找到合法分解，检查 d 是否正确。")
	}
	// φ 已知：p+q = n − φ + 1
	s := new(big.Int).Sub(n, phi)
	s.Add(s, big.NewInt(1))
	disc := new(big.Int).Mul(s, s)
	disc.Sub(disc, new(big.Int).Lsh(n, 2))
	r, ok := bigIsPerfectSquare(disc)
	if !ok {
		return failAttack("由 φ 求 p+q 失败（判别式非完全平方）", "φ 的值可能不正确。")
	}
	p := new(big.Int).Div(new(big.Int).Add(s, r), big.NewInt(2))
	q := new(big.Int).Div(new(big.Int).Sub(s, r), big.NewInt(2))
	if p.Sign() <= 0 || new(big.Int).Mul(p, q).Cmp(n) != 0 {
		return failAttack("求出的 p·q ≠ n", "φ 的值可能不正确。")
	}
	out, err := rsaFinish(n, p, q, e, c, "φ 已知：p+q = n − φ + 1，直接解二次方程")
	if err != nil {
		return nil, err
	}
	return textOut(out)
}

// ---------------------------------------------------------------------------
// 8. 特殊数 gcd（crypto.rsa_special）：Fibonacci/Lucas/Mersenne/Fermat 数/阶乘/素数积/合数积
//    （RsaCtfTool fibonacci_gcd / lucas_gcd / mersenne_pm1_gcd / fermat_numbers_gcd /
//     factorial_pm1_gcd / primorial_pm1_gcd / compositorial_pm1_gcd 家族的统一移植）
// ---------------------------------------------------------------------------

func runRsaSpecial(ctx context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	e := clBig(params, "e")
	c := clBig(params, "c")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	family := clStr(params, "family")
	if family == "" {
		family = "fibonacci"
	}
	limit := clInt(params, "limit", 10000)
	one := big.NewInt(1)

	found := func(g *big.Int, hit string) (*core.ToolResult, error) {
		p := g
		q := new(big.Int).Div(n, g)
		out, err := rsaFinish(n, p, q, e, c, hit)
		if err != nil {
			return nil, err
		}
		return textOut(out)
	}
	scan := func(g *big.Int, hit string) (*core.ToolResult, bool, error) {
		if g.Cmp(one) > 0 && g.Cmp(n) < 0 {
			res, err := found(g, hit)
			return res, true, err
		}
		return nil, false, nil
	}

	switch family {
	case "fibonacci", "lucas":
		// 迭代生成序列（mod n 即可：gcd(F mod n, n) = gcd(F, n)）
		a, b := big.NewInt(0), big.NewInt(1) // fib: F(0)=0, F(1)=1；lucas: L(0)=2, L(1)=1
		if family == "lucas" {
			a, b = big.NewInt(2), big.NewInt(1)
		}
		for i := 1; i <= limit; i++ {
			if err := rsaCtx(ctx); err != nil {
				return nil, err
			}
			a, b = b, new(big.Int).Mod(new(big.Int).Add(a, b), n)
			if res, hit, err := scan(bigGcd(a, n), fmt.Sprintf("%s 序列命中：第 %d 项与 n 有公因子", map[string]string{"fibonacci": "Fibonacci", "lucas": "Lucas"}[family], i)); hit || err != nil {
				return res, err
			}
		}
	case "mersenne":
		// M_k = 2^k − 1：M_k = 2·M_{k−1} + 1 (mod n)
		m := big.NewInt(1) // M_1 = 1
		for k := 2; k <= limit; k++ {
			if err := rsaCtx(ctx); err != nil {
				return nil, err
			}
			m = new(big.Int).Mod(new(big.Int).Add(new(big.Int).Lsh(m, 1), one), n)
			if res, hit, err := scan(bigGcd(m, n), fmt.Sprintf("Mersenne 数命中：gcd(2^%d − 1, n) > 1", k)); hit || err != nil {
				return res, err
			}
		}
	case "fermat_numbers":
		// F_i = 2^(2^i) + 1：F_i = (F_{i−1} − 1)² + 1 (mod n)
		f := big.NewInt(3) // F_0
		for i := 1; i <= 12 && i <= limit; i++ {
			if err := rsaCtx(ctx); err != nil {
				return nil, err
			}
			t := new(big.Int).Sub(f, one)
			f = new(big.Int).Mod(new(big.Int).Add(new(big.Int).Mul(t, t), one), n)
			if res, hit, err := scan(bigGcd(f, n), fmt.Sprintf("Fermat 数命中：gcd(F_%d, n) > 1", i)); hit || err != nil {
				return res, err
			}
		}
	case "factorial", "primorial", "compositorial":
		// 阶乘 / 素数积 / 合数积：逐步累乘 (mod n)，gcd 跳变即命中
		acc := big.NewInt(1)
		for k := 2; k <= limit; k++ {
			if err := rsaCtx(ctx); err != nil {
				return nil, err
			}
			use := true
			if family == "primorial" {
				use = big.NewInt(int64(k)).ProbablyPrime(20)
			} else if family == "compositorial" {
				use = !big.NewInt(int64(k)).ProbablyPrime(20)
			}
			if use {
				acc = new(big.Int).Mod(new(big.Int).Mul(acc, big.NewInt(int64(k))), n)
				if res, hit, err := scan(bigGcd(acc, n), fmt.Sprintf("%s 命中：累乘至 %d 时 gcd 跳变",
					map[string]string{"factorial": "阶乘", "primorial": "素数积", "compositorial": "合数积"}[family], k)); hit || err != nil {
					return res, err
				}
			}
		}
	default:
		return errOut("未知族：" + family + "（fibonacci | lucas | mersenne | fermat_numbers | factorial | primorial | compositorial）")
	}
	return failAttack(fmt.Sprintf("%s 族扫描至上限未命中", family), "n 的因子可能不属于该特殊数族。")
}

// ---------------------------------------------------------------------------
// 9. Pollard Rho / Brent（crypto.rsa_rho）
// ---------------------------------------------------------------------------

func pollardRho(n *big.Int, maxIters int, ctx context.Context) (*big.Int, error) {
	if new(big.Int).Mod(n, big.NewInt(2)).Sign() == 0 {
		return big.NewInt(2), nil
	}
	one := big.NewInt(1)
	step := func(y *big.Int, c *big.Int) *big.Int {
		y.Mul(y, y)
		y.Add(y, c)
		y.Mod(y, n)
		return y
	}
	for attempt := 0; attempt < 8; attempt++ {
		// 随机参数 c，Brent 变体
		cBuf := make([]byte, 8)
		if _, err := rand.Read(cBuf); err != nil {
			return nil, err
		}
		c := new(big.Int).SetBytes(cBuf)
		c.Mod(c, n)
		if c.Sign() == 0 {
			c = one
		}
		y := new(big.Int).Set(c)
		ys := new(big.Int).Set(y)
		x := new(big.Int).Set(y)
		r, q := big.NewInt(1), big.NewInt(1)
		m := 128
		g := big.NewInt(1)
		total := 0
		for g.Cmp(one) == 0 && total < maxIters {
			x = new(big.Int).Set(y)
			for i := int64(0); i < r.Int64(); i++ {
				step(y, c)
			}
			k := big.NewInt(0)
			for k.Cmp(r) < 0 && g.Cmp(one) == 0 {
				ys = new(big.Int).Set(y)
				remaining := new(big.Int).Sub(r, k).Int64()
				im := int64(m)
				if remaining < im {
					im = remaining
				}
				for i := int64(0); i < im; i++ {
					step(y, c)
					t := new(big.Int).Sub(x, y)
					t.Abs(t)
					if t.Sign() == 0 {
						continue
					}
					q.Mul(q, t)
					q.Mod(q, n)
				}
				g = bigGcd(q, n)
				k.Add(k, big.NewInt(int64(m)))
				total += int(im)
				if err := rsaCtx(ctx); err != nil {
					return nil, err
				}
			}
			r.Lsh(r, 1)
		}
		if g.Cmp(n) == 0 {
			// 回溯：从 ys 逐步重放找首个 gcd 跳变
			g = one
			for g.Cmp(one) == 0 {
				step(ys, c)
				t := new(big.Int).Sub(x, ys)
				t.Abs(t)
				if t.Sign() == 0 {
					break
				}
				g = bigGcd(t, n)
			}
		}
		if g.Cmp(one) > 0 && g.Cmp(n) < 0 {
			return g, nil
		}
	}
	return nil, fmt.Errorf("rho 未在迭代上限内找到因子")
}

func runRsaRho(ctx context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	e := clBig(params, "e")
	c := clBig(params, "c")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	maxIters := clInt(params, "max_iters", 5000000)
	g, err := pollardRho(n, maxIters, ctx)
	if err != nil {
		return failAttack("Pollard Rho 未命中", "n 的因子可能过大（每因子 ≤ ~2^64 时 rho 最有效，更大因子建议 ECM/QS）。")
	}
	p := g
	q := new(big.Int).Div(n, g)
	out, err := rsaFinish(n, p, q, e, c, "Pollard Rho（Brent 变体）分解命中")
	if err != nil {
		return nil, err
	}
	return textOut(out)
}

// ---------------------------------------------------------------------------
// 10. Pollard p−1（crypto.rsa_pm1）：p−1 是 B1 平滑的
// ---------------------------------------------------------------------------

func runRsaPm1(ctx context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	e := clBig(params, "e")
	c := clBig(params, "c")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	b1 := clInt(params, "b1", 100000)
	if b1 < 2 || b1 > 100000000 {
		return errOut("B1 需在 2 至 1e8 之间")
	}
	// a = 2；对每个素数幂 q ≤ B1：a = a^q mod n；gcd(a−1, n)
	a := big.NewInt(2)
	one := big.NewInt(1)
	smallPrimes := sievePrimes(b1)
	check := func() (*core.ToolResult, bool, error) {
		g := bigGcd(new(big.Int).Sub(a, one), n)
		if g.Cmp(one) > 0 && g.Cmp(n) < 0 {
			p := g
			q := new(big.Int).Div(n, g)
			out, err := rsaFinish(n, p, q, e, c, fmt.Sprintf("Pollard p−1 命中（B1 = %d）", b1))
			if err != nil {
				return nil, false, err
			}
			res, err := textOut(out)
			if err != nil {
				return nil, false, err
			}
			return res, true, nil
		}
		if g.Cmp(n) == 0 {
			return nil, true, fmt.Errorf("gcd 命中 n 本身（B1 过大或 p−1、q−1 同时平滑），可调小 B1 重试")
		}
		return nil, false, nil
	}
	for _, pr := range smallPrimes {
		if err := rsaCtx(ctx); err != nil {
			return nil, err
		}
		// 素数幂 pr^k ≤ B1 全部计入
		pk := big.NewInt(int64(pr))
		for pk.Int64() <= int64(b1) {
			a.Exp(a, big.NewInt(int64(pr)), n)
			pk.Mul(pk, big.NewInt(int64(pr)))
		}
		if res, hit, err := check(); hit || err != nil {
			return res, err
		}
	}
	return failAttack(fmt.Sprintf("p−1 第一阶段未命中（B1 = %d）", b1), "p−1 或 q−1 可能不够平滑；可增大 B1 或改用 Rho / FactordB。")
}

// sievePrimes 埃拉托斯特尼筛（limit ≤ 1e8 时仅筛至其平方根实际需要的范围——
// 为控内存这里限制 limit ≤ 1e7，超过时只筛至 1e7 并提示）。返回 ≤ limit 的素数。
func sievePrimes(limit int) []int {
	if limit > 10000000 {
		limit = 10000000
	}
	sieve := make([]bool, limit+1)
	primes := []int{}
	for i := 2; i <= limit; i++ {
		if !sieve[i] {
			primes = append(primes, i)
			for j := i * i; j <= limit; j += i {
				sieve[j] = true
			}
		}
	}
	return primes
}

// ---------------------------------------------------------------------------
// 11. 小素数试除（crypto.rsa_smallq）
// ---------------------------------------------------------------------------

func runRsaSmallq(ctx context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	e := clBig(params, "e")
	c := clBig(params, "c")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	limit := clInt(params, "limit", 1000000)
	if limit < 2 || limit > 100000000 {
		return errOut("上限需在 2 至 1e8 之间")
	}
	small := big.NewInt(0)
	mod := new(big.Int)
	for i := 2; i <= limit; i++ {
		if i%4096 == 0 {
			if err := rsaCtx(ctx); err != nil {
				return nil, err
			}
		}
		small.SetInt64(int64(i))
		if mod.Mod(n, small).Sign() == 0 {
			p := small
			q := new(big.Int).Div(n, small)
			out, err := rsaFinish(n, new(big.Int).Set(p), q, e, c, fmt.Sprintf("小因子试除命中：%d", i))
			if err != nil {
				return nil, err
			}
			return textOut(out)
		}
	}
	return failAttack(fmt.Sprintf("试除至 %d 未找到因子", limit), "最小因子可能超过上限。")
}

// ---------------------------------------------------------------------------
// 12. FactorDB 在线查询（crypto.rsa_factordb）：走全局代理
// ---------------------------------------------------------------------------

type factordbResp struct {
	Status  string  `json:"status"`
	Factors [][]any `json:"factors"`
}

func factordbQuery(endpoint, proxyURL, nStr string) (*factordbResp, error) {
	u := endpoint + "/api?query=" + url.QueryEscape(nStr)
	tr := &http.Transport{}
	if proxyURL != "" {
		if pu, err := url.Parse(proxyURL); err == nil {
			tr.Proxy = http.ProxyURL(pu)
		}
	}
	client := &http.Client{Transport: tr, Timeout: 20 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("FactorDB 返回 HTTP %d", resp.StatusCode)
	}
	var r factordbResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

func runRsaFactordb(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	e := clBig(params, "e")
	c := clBig(params, "c")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	ac := core.GetAppConfig()
	proxyURL := ""
	if ac != nil {
		proxyURL = ac.Proxy.URL
	}
	endpoint := strings.TrimRight(strings.TrimSpace(clStr(params, "endpoint")), "/")
	if endpoint == "" {
		endpoint = "http://factordb.com"
	}
	resp, err := factordbQuery(endpoint, proxyURL, n.String())
	if err != nil {
		return nil, fmt.Errorf("FactorDB 查询失败：%v（可在设置页配置全局代理后重试）", err)
	}
	// factors: [["p", e], ...]
	var factors []*big.Int
	for _, f := range resp.Factors {
		if len(f) < 1 {
			continue
		}
		s, _ := f[0].(string)
		v, ok := new(big.Int).SetString(s, 10)
		if !ok {
			continue
		}
		exp := int64(1)
		if len(f) > 1 {
			switch t := f[1].(type) {
			case float64:
				exp = int64(t)
			case string:
				if x, err := strconv.ParseInt(t, 10, 64); err == nil {
					exp = x
				}
			}
		}
		for i := int64(0); i < exp; i++ {
			factors = append(factors, new(big.Int).Set(v))
		}
	}
	statusMeaning := map[string]string{
		"FF":   "完全分解（所有素因子已知）",
		"CF":   "部分分解",
		"C":    "合数（无已知因子）",
		"P":    "n 本身是素数",
		"Unit": "单位元",
		"PRP":  "可能素数",
	}[resp.Status]
	if statusMeaning == "" {
		statusMeaning = resp.Status
	}
	var lines []string
	lines = append(lines, "FactorDB 状态："+statusMeaning)
	// 找两个因子 p·q = n
	one := big.NewInt(1)
	found := false
	for i := 0; i < len(factors) && !found; i++ {
		if factors[i].Cmp(one) <= 0 || factors[i].Cmp(n) >= 0 {
			continue
		}
		if new(big.Int).Mod(n, factors[i]).Sign() == 0 {
			p := factors[i]
			q := new(big.Int).Div(n, p)
			out, err := rsaFinish(n, p, q, e, c, "FactorDB 已知因子命中")
			if err == nil {
				lines = append(lines, out)
				return textOut(strings.Join(lines, "\n\n"))
			}
			found = true
		}
	}
	// 未直接命中：列出已知因子
	var fs []string
	for _, f := range factors {
		fs = append(fs, f.String())
	}
	if len(fs) == 0 {
		lines = append(lines, "FactorDB 无已知因子记录。")
	} else {
		lines = append(lines, "已知因子（未直接整除 n 或不完整）："+strings.Join(fs, " × "))
	}
	lines = append(lines, "可尝试：本地 Rho / p−1 / Fermat / 特殊数 gcd 等攻击。")
	return textOut(strings.Join(lines, "\n"))
}

// ---------------------------------------------------------------------------
// 注册
// ---------------------------------------------------------------------------

func init() {
	reg := core.DefaultRegistry
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_low_e", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "RSA 小指数攻击：m^e < n 时直接开 e 次根；带填充时扫描 c + k·n",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "e", Type: "text", Description: "公钥指数 e（如 3）"},
			{Name: "c", Type: "text", Required: true, Description: "密文 c"},
			{Name: "k_limit", Type: "int", Default: 100000, Description: "c + k·n 的 k 扫描上限"},
		},
		Runner: runRsaLowE,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_broadcast", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "Hastad 广播攻击：同一明文同一小 e 发给多个模数，CRT 合并后开 e 次根",
		ParamsAttr: []core.ParamSpec{
			{Name: "e", Type: "text", Required: true, Description: "公钥指数 e（需 ≥ e 组密文）"},
			{Name: "pairs", Type: "text", Required: true, Description: "密文对，每行 n,c"},
		},
		Runner: runRsaBroadcast,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_fermat", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "Fermat 分解：|p−q| 小时 n = a²−b² 快速收敛；命中后自动出 φ/d/明文",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "e", Type: "text", Description: "公钥指数 e（给则计算 d 与明文）"},
			{Name: "c", Type: "text", Description: "密文 c（给则解密）"},
			{Name: "max_iter", Type: "int", Default: 1000000, Description: "最大迭代次数"},
		},
		Runner: runRsaFermat,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_algebraic", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "RSA 代数关系分解：线性 q=a·p+b / 比例 a·q=b·p / 差和 p±q 已知 / 同余 p≡q (mod M) / 安全素数 p=2q+1",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "amode", Type: "str", Default: "linear", Description: "linear | ratio | diff_sum | congruence | safe_prime"},
			{Name: "a", Type: "text", Description: "线性/比例参数 a"},
			{Name: "b", Type: "text", Description: "线性/比例参数 b"},
			{Name: "p_plus_q", Type: "text", Description: "p+q（diff_sum 模式）"},
			{Name: "p_minus_q", Type: "text", Description: "p−q（diff_sum 模式）"},
			{Name: "M", Type: "text", Description: "同余模数 M（congruence 模式）"},
			{Name: "k_limit", Type: "int", Default: 100000, Description: "congruence 的 k 扫描上限"},
			{Name: "e", Type: "text", Description: "公钥指数 e"},
			{Name: "c", Type: "text", Description: "密文 c"},
		},
		Runner: runRsaAlgebraic,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_twin", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "相邻/孪生素数分解：p·(p+g) = n → (2p+g)² = 4n+g²，gap 逐个扫描（孪生 g=2 秒出）",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "e", Type: "text", Description: "公钥指数 e"},
			{Name: "c", Type: "text", Description: "密文 c"},
			{Name: "max_gap", Type: "int", Default: 100000, Description: "gap 扫描上限"},
		},
		Runner: runRsaTwin,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_next_prime", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "next_prime 分解：p 是 base 附近的素数（p = next_prime(base) 类），窗口内逐候选试除",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "base", Type: "text", Required: true, Description: "基准数（p 在 base 附近）"},
			{Name: "window", Type: "int", Default: 100000, Description: "扫描窗口大小"},
			{Name: "e", Type: "text", Description: "公钥指数 e"},
			{Name: "c", Type: "text", Description: "密文 c"},
		},
		Runner: runRsaNextPrime,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_phi_leak", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "φ(n) 或 d 泄露：已知 φ 直接解 p+q；已知 d 由 e·d−1 = k·φ 枚举 k 恢复分解",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "phi", Type: "text", Description: "φ(n)（与 d 二选一）"},
			{Name: "d", Type: "text", Description: "私钥 d（与 phi 二选一；需同时给 e）"},
			{Name: "e", Type: "text", Description: "公钥指数 e"},
			{Name: "c", Type: "text", Description: "密文 c"},
		},
		Runner: runRsaPhiLeak,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_special", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "特殊数 gcd 分解（RsaCtfTool 移植）：Fibonacci/Lucas/Mersenne/Fermat 数/阶乘/素数积/合数积族与 n 的公因子扫描",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "family", Type: "str", Default: "fibonacci", Description: "fibonacci | lucas | mersenne | fermat_numbers | factorial | primorial | compositorial"},
			{Name: "limit", Type: "int", Default: 10000, Description: "序列项数/累乘上限"},
			{Name: "e", Type: "text", Description: "公钥指数 e"},
			{Name: "c", Type: "text", Description: "密文 c"},
		},
		Runner: runRsaSpecial,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_rho", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "Pollard Rho（Brent 变体）分解：因子 ≤ 2^64 量级时最有效",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "max_iters", Type: "int", Default: 5000000, Description: "迭代上限"},
			{Name: "e", Type: "text", Description: "公钥指数 e"},
			{Name: "c", Type: "text", Description: "密文 c"},
		},
		Runner: runRsaRho,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_pm1", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "Pollard p−1 分解：p−1 是 B1 内平滑（所有素因子幂 ≤ B1）时命中",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "b1", Type: "int", Default: 100000, Description: "平滑上界 B1"},
			{Name: "e", Type: "text", Description: "公钥指数 e"},
			{Name: "c", Type: "text", Description: "密文 c"},
		},
		Runner: runRsaPm1,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_smallq", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "小因子试除：q 很小（如题目忘了乘大素数）时直接命中",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "limit", Type: "int", Default: 1000000, Description: "试除上限"},
			{Name: "e", Type: "text", Description: "公钥指数 e"},
			{Name: "c", Type: "text", Description: "密文 c"},
		},
		Runner: runRsaSmallq,
	})
	reg.Register(&core.BaseTool{
		NameAttr: "rsa_factordb", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "FactorDB 在线查询：n 曾被分解过则直接拿到因子（走全局代理）",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "e", Type: "text", Description: "公钥指数 e"},
			{Name: "c", Type: "text", Description: "密文 c"},
			{Name: "endpoint", Type: "str", Description: "FactorDB 地址（默认 http://factordb.com，可换镜像）"},
		},
		Runner: runRsaFactordb,
	})
}
