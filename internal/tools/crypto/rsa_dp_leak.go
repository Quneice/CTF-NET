// crypto.rsa_dp_leak —— RSA dp/dq 泄露攻击(RsaCtfTool 套件第 13 员)。
// 原理:e·dp ≡ 1 (mod p−1) → e·dp − 1 = k·(p−1),dp < p−1 ⇒ 1 ≤ k < e,
// 枚举 k 整除 (e·dp−1) 得候选 p = (e·dp−1)/k + 1,n mod p == 0 即分解(dq 同理)。
// 扩展(2026 Bugku No Ciphertext RSA):题目不给完整密文只给 c mod p / c mod q 时,
// 用 CRT 还原 c 后正常解密;c = c1 + p·((c2−c1)·p⁻¹ mod q)。
package crypto

import (
	"context"
	"fmt"
	"math/big"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&core.BaseTool{
		NameAttr: "rsa_dp_leak", CategoryAttr: core.CatCrypto, VersionAttr: "1.0.0",
		DescriptionAttr: "RSA dp/dq 泄露攻击:e·dp−1 = k·(p−1) 枚举 k∈[1,e) 分解 n;支持 c mod p / c mod q 双泄露 CRT 还原缺失密文(No Ciphertext RSA 套路)",
		ParamsAttr: []core.ParamSpec{
			{Name: "n", Type: "text", Required: true, Description: "模数 n"},
			{Name: "e", Type: "text", Description: "公钥指数 e(默认 65537)"},
			{Name: "dp", Type: "text", Description: "dp = d mod (p−1),与 dq 至少其一"},
			{Name: "dq", Type: "text", Description: "dq = d mod (q−1),与 dp 至少其一"},
			{Name: "c", Type: "text", Description: "密文 c(完整;缺省时可用 c_mod_p/c_mod_q CRT 还原)"},
			{Name: "c_mod_p", Type: "text", Description: "泄露值 c mod p"},
			{Name: "c_mod_q", Type: "text", Description: "泄露值 c mod q"},
		},
		Runner: runRsaDpLeak,
	})
}

// factorViaLeak 由 e·x−1 = k·(f−1) 枚举 k ∈ [1,e) 求 n 的因子(x 为 dp 或 dq)。
// 返回 (因子, 命中的 k);未找到返回 (nil, 0)。
func factorViaLeak(n, e, x *big.Int) (*big.Int, int64) {
	ex1 := new(big.Int).Sub(new(big.Int).Mul(e, x), big.NewInt(1))
	for k := int64(1); k < e.Int64(); k++ {
		kb := big.NewInt(k)
		if new(big.Int).Mod(ex1, kb).Sign() != 0 {
			continue
		}
		cand := new(big.Int).Div(ex1, kb)
		cand.Add(cand, big.NewInt(1))
		if cand.Cmp(big.NewInt(1)) <= 0 || cand.Cmp(n) == 0 {
			continue
		}
		if new(big.Int).Mod(n, cand).Sign() == 0 {
			return cand, k
		}
	}
	return nil, 0
}

func runRsaDpLeak(_ context.Context, params map[string]any, _ *core.ToolContext) (*core.ToolResult, error) {
	n := clBig(params, "n")
	if n == nil || n.Cmp(big.NewInt(1)) <= 0 {
		return errOut("请输入模数 n")
	}
	e := clBig(params, "e")
	if e == nil {
		e = big.NewInt(65537)
	}
	if e.Cmp(big.NewInt(2)) < 0 {
		return errOut("e 非法")
	}
	if !e.IsInt64() || e.Int64() > 1<<30 {
		return errOut("e 过大:dp 泄露需枚举 k ∈ [1,e),e 上限 2^30")
	}
	dp := clBig(params, "dp")
	dq := clBig(params, "dq")
	if dp == nil && dq == nil {
		return errOut("请提供 dp(= d mod (p−1))或 dq(= d mod (q−1))至少其一")
	}
	c := clBig(params, "c")
	cModP := clBig(params, "c_mod_p")
	cModQ := clBig(params, "c_mod_q")

	// 分解:dp → p;失败再试 dq → q(因子标签对齐 CRT 余数)
	var p, q *big.Int
	var via string
	var kUsed int64
	if dp != nil {
		if f, k := factorViaLeak(n, e, dp); f != nil {
			p, q = f, new(big.Int).Div(n, f)
			via, kUsed = "dp", k
		}
	}
	if p == nil && dq != nil {
		if f, k := factorViaLeak(n, e, dq); f != nil {
			q, p = f, new(big.Int).Div(n, f)
			via, kUsed = "dq", k
		}
	}
	if p == nil {
		return failAttack("dp/dq 泄露分解失败",
			"枚举 k ∈ [1,e) 未找到 n 的因子。检查:① dp/dq 值是否抄错;② e 是否与题目一致;③ 题目若另有 phi/d 泄露可改用 rsa_phi_leak。")
	}
	extra := []string{fmt.Sprintf("%s 泄露分解成功:k = %d → 因子 = (e·%s − 1)/k + 1", via, kUsed, via)}

	// CRT 还原缺失密文:c ≡ c1 (mod p),c ≡ c2 (mod q) → c = c1 + p·((c2−c1)·p⁻¹ mod q)
	if c == nil && cModP != nil && cModQ != nil {
		pinv := new(big.Int).ModInverse(p, q)
		if pinv == nil {
			return errOut("p 与 q 不互素,无法 CRT")
		}
		t := new(big.Int).Mod(new(big.Int).Mul(new(big.Int).Sub(cModQ, cModP), pinv), q)
		c = new(big.Int).Add(cModP, new(big.Int).Mul(p, t))
		extra = append(extra,
			"CRT 还原密文:c ≡ c_mod_p (mod p),c ≡ c_mod_q (mod q)",
			"c = "+c.String())
	}

	out, err := rsaFinish(n, p, q, e, c, extra...)
	if err != nil {
		return errOut(err.Error())
	}
	return textOut(out)
}
