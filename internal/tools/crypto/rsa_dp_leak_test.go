// crypto.rsa_dp_leak 测试:dp/dq 泄露分解 n + CRT 还原缺失密文(2026 Bugku No Ciphertext RSA 套路)。
package crypto

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

func dpRunTool(t *testing.T, params map[string]any) *core.ToolResult {
	t.Helper()
	tool, ok := core.DefaultRegistry.Get("crypto.rsa_dp_leak")
	if !ok {
		t.Fatal("crypto.rsa_dp_leak 未注册")
	}
	res, err := tool.Run(context.Background(), params, &core.ToolContext{})
	if err != nil {
		t.Fatalf("运行出错: %v", err)
	}
	return res
}

// dpGenKey 生成测试用 RSA 参数(512bit ×2),返回 n,e,d,p,q。
func dpGenKey(t *testing.T) (n, e, d, p, q *big.Int) {
	t.Helper()
	e = big.NewInt(65537)
	for {
		p, _ = rand.Prime(rand.Reader, 512)
		q, _ = rand.Prime(rand.Reader, 512)
		if p.Cmp(q) == 0 {
			continue
		}
		n = new(big.Int).Mul(p, q)
		phi := new(big.Int).Mul(new(big.Int).Sub(p, big.NewInt(1)), new(big.Int).Sub(q, big.NewInt(1)))
		if new(big.Int).GCD(nil, nil, e, phi).Cmp(big.NewInt(1)) != 0 {
			continue
		}
		d = new(big.Int).ModInverse(e, phi)
		return
	}
}

func dpS2N(s string) *big.Int { return new(big.Int).SetBytes([]byte(s)) }

func dpS2n(s string) *big.Int { return new(big.Int).SetBytes([]byte(s)) }

// A. Bugku No Ciphertext RSA 真题向量:dp + c mod p / c mod q(无完整 c),CRT 还原后解密出 flag。
func TestRsaDpLeakBugkuVector(t *testing.T) {
	res := dpRunTool(t, map[string]any{
		"n": "20446305236294581881140725938833605520023786992590821920806811572505477606387830733060901955371457846738863889279107367753914707053108524516947943610226964107558978693717655472431318403586269727573489946433389159145602800207787382180423611018458189828931572992863289292613405218278949739073786959411566919119158325510346523951336418479951932209799501108477995314359188860274532542630968951457343647522078553891223764285682602714616115281040492374167771746275218863543545907073818468841626731849010162645256595641473022327747346052186526727216525426337190917106751151745388854749923598231196090790074682287345100965373",
		"e": "65537",
		"dp": "158325084409606165134868956023907667507671677832027168046364315703295407017343206432691817272550256085313093440797443736742051552429653661451417133052016647805226890534559578502154540190596419643135611407218228612201386225040438407799879719366484669372051153511312310009858718254183049095347658106745575535469",
		"c_mod_p": "116908580792713727509554342060190793142033425411766631165842865699167747112494944492849392371565838125428644563687571660329763478509815200537676368326781342382868082294015200439334832938068779547847851748337854603115134732593759473453640093195977206450633212921689957303431235603192670553553803757864481012599",
		"c_mod_q": "18319344794671185787719339480953236221170603508712466350928025351527616335735433941953520711516118072282425397883638101260674452825151245435529613074796106769481242318321469286177813223159476396555044378245229663195991557031227024085316255781963813911437991309663376270820486723382786632243229800891705679245",
	})
	if res.Status != core.StatusSuccess {
		t.Fatalf("期望 success: %s %s", res.Status, res.Stderr)
	}
	out := res.Stdout
	if !strings.Contains(out, "176880281220421384276770426697672033095535078075714508749353553548240276323610202856692980610806418687790464096511235044404620241239340834109671075804268777866611345077962413424290036147269400193518437230146620445225908578474569771436035545465911591432888691958522976733082929634591293998871492096106249085747") {
		t.Fatal("p 分解错误")
	}
	if !strings.Contains(out, "bugku{d583310f-2add-4559-957e-8f6193eeb8f7}") {
		t.Fatalf("flag 未还原: %s", out)
	}
	if !strings.Contains(out, "CRT") {
		t.Fatal("应报告 CRT 还原密文")
	}
	if !strings.Contains(out, "✓") {
		t.Fatal("应包含自验证 ✓")
	}
}

// B. 随机生成的完整回路:dp + 双泄露(不给 c),flag 还原。
func TestRsaDpLeakGeneratedRoundTrip(t *testing.T) {
	n, e, d, p, q := dpGenKey(t)
	dp := new(big.Int).Mod(d, new(big.Int).Sub(p, big.NewInt(1)))
	flag := "flag{dp_leak_round_trip_ok_2026}"
	m := dpS2N(flag)
	c := new(big.Int).Exp(m, e, n)
	c1 := new(big.Int).Mod(c, p)
	c2 := new(big.Int).Mod(c, q)

	res := dpRunTool(t, map[string]any{
		"n": n.String(), "e": e.String(), "dp": dp.String(),
		"c_mod_p": c1.String(), "c_mod_q": c2.String(),
	})
	if res.Status != core.StatusSuccess {
		t.Fatalf("期望 success: %s", res.Stderr)
	}
	if !strings.Contains(res.Stdout, flag) {
		t.Fatalf("flag 未还原:\n%s", res.Stdout)
	}
	if !strings.Contains(res.Stdout, p.String()) {
		t.Fatal("p 未恢复")
	}
}

// C. dq 泄露(而非 dp)+ 完整密文。
func TestRsaDpLeakDqVariant(t *testing.T) {
	n, e, d, _, q := dpGenKey(t)
	dq := new(big.Int).Mod(d, new(big.Int).Sub(q, big.NewInt(1)))
	flag := "flag{dq_leak_variant}"
	c := new(big.Int).Exp(dpS2N(flag), e, n)

	res := dpRunTool(t, map[string]any{
		"n": n.String(), "e": e.String(), "dq": dq.String(), "c": c.String(),
	})
	if res.Status != core.StatusSuccess {
		t.Fatalf("期望 success: %s", res.Stderr)
	}
	if !strings.Contains(res.Stdout, flag) {
		t.Fatalf("flag 未还原:\n%s", res.Stdout)
	}
}

// D. 仅分解(无密文):成功出 p/q,不出现明文行。
func TestRsaDpLeakFactorOnly(t *testing.T) {
	n, e, d, p, _ := dpGenKey(t)
	dp := new(big.Int).Mod(d, new(big.Int).Sub(p, big.NewInt(1)))
	res := dpRunTool(t, map[string]any{"n": n.String(), "e": e.String(), "dp": dp.String()})
	if res.Status != core.StatusSuccess {
		t.Fatalf("期望 success: %s", res.Stderr)
	}
	if !strings.Contains(res.Stdout, "p = "+p.String()) {
		t.Fatalf("p 未恢复:\n%s", res.Stdout)
	}
	if strings.Contains(res.Stdout, "明文 m") {
		t.Fatal("无密文时不应输出明文")
	}
}

// E. 错误 dp:攻击未成功(状态仍为 success 的报告,套件惯例)。
func TestRsaDpLeakWrongDp(t *testing.T) {
	n, e, _, _, _ := dpGenKey(t)
	bad, _ := rand.Prime(rand.Reader, 256) // 与真 dp 无关的值
	res := dpRunTool(t, map[string]any{"n": n.String(), "e": e.String(), "dp": bad.String()})
	if res.Status != core.StatusSuccess || !strings.Contains(res.Stdout, "攻击未成功") {
		t.Fatalf("应报告攻击未成功: status=%s out=%s", res.Status, res.Stdout)
	}
}

// F. 参数校验:缺 n / 缺 dp 与 dq(套件惯例:校验错误走 Go error,非 StatusError 结果)。
func TestRsaDpLeakParamErrors(t *testing.T) {
	tool, ok := core.DefaultRegistry.Get("crypto.rsa_dp_leak")
	if !ok {
		t.Fatal("crypto.rsa_dp_leak 未注册")
	}
	if _, err := tool.Run(context.Background(), map[string]any{"e": "65537", "dp": "123"}, &core.ToolContext{}); err == nil || !strings.Contains(err.Error(), "模数 n") {
		t.Fatalf("缺 n 应报错: %v", err)
	}
	if _, err := tool.Run(context.Background(), map[string]any{"n": "12345678901", "e": "65537"}, &core.ToolContext{}); err == nil || !strings.Contains(err.Error(), "dp") {
		t.Fatalf("缺 dp/dq 应报错: %v", err)
	}
}
