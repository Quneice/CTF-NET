// 对应 legacy/test_cbc_bitflip_offline.py 的离线验证向量（flip → 泄露补偿），
// 外加：validate_params 语义、run() 错误分支（[错误]/[中止]）、
// httptest 内嵌 PKCS7 Padding Oracle 的 oracle 模式全明文恢复。
package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

const (
	cbcTestPlain = `a:2:{s:8:"username";s:5:"aaaaa";s:8:"password";s:5:"ppppp";}`
	cbcWantP0    = `a:2:{s:8:"userna`
	cbcWantP1    = `me";s:5:"admin";`
	cbcTestKey   = "KKKKKKKKKKKKKKKK"
	cbcTestIV    = "IIIIIIIIIIIIIIII"
)

func cbcEncrypt(t *testing.T, plain []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher([]byte(cbcTestKey))
	if err != nil {
		t.Fatal(err)
	}
	padded := append(append([]byte{}, plain...), 4, 4, 4, 4)
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, []byte(cbcTestIV)).CryptBlocks(ct, padded)
	return ct
}

func cbcDecrypt(t *testing.T, ct, iv []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher([]byte(cbcTestKey))
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, ct)
	return out
}

// runCbc 复刻真实管线：ValidateParams 填默认值 → Run（等价旧 FastAPI 端点行为）。
func runCbc(t *testing.T, params map[string]any) *core.ToolResult {
	t.Helper()
	tool, ok := core.DefaultRegistry.Get("crypto.aes_cbc_bitflip")
	if !ok {
		t.Fatal("crypto.aes_cbc_bitflip 未注册")
	}
	validated, err := tool.ValidateParams(params)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	res, err := tool.Run(context.Background(), validated, &core.ToolContext{})
	if err != nil {
		t.Fatalf("run 出错: %v", err)
	}
	return res
}

func cbcFlipParams(ct []byte) map[string]any {
	return map[string]any{
		"mode": "flip", "plaintext": cbcTestPlain, "orig": "aaaaa", "target": "admin",
		"encoding":   "ascii",
		"cipher_b64": base64.StdEncoding.EncodeToString(ct),
		"iv_b64":     base64.StdEncoding.EncodeToString([]byte(cbcTestIV)),
	}
}

// 第 1 步：flip 模式，不带泄露。核对翻转后 P1=admin、P0 被破坏、P2/P3 不动。
func TestCbcFlipNoLeak(t *testing.T) {
	ct := cbcEncrypt(t, []byte(cbcTestPlain))
	res := runCbc(t, cbcFlipParams(ct))
	if res.Status != core.StatusSuccess {
		t.Fatalf("flip 失败: %s\n%s", res.Stderr, res.Stdout)
	}
	if !strings.Contains(res.Stdout, "[警告] 翻转 C[j-1] 会连带把块 j-1 整体破坏") {
		t.Errorf("缺级联破坏警告:\n%s", res.Stdout)
	}
	m := regexp.MustCompile(`\[密文\] 翻转后 cipher（base64）: (\S+)`).FindStringSubmatch(res.Stdout)
	if m == nil {
		t.Fatalf("输出里没有翻转后密文:\n%s", res.Stdout)
	}
	flipped, err := base64.StdEncoding.DecodeString(m[1])
	if err != nil {
		t.Fatal(err)
	}
	garbage := cbcDecrypt(t, flipped, []byte(cbcTestIV))
	if got := string(garbage[16:32]); got != cbcWantP1 {
		t.Errorf("P1 = %q, 期望 %q", got, cbcWantP1)
	}
	if got := string(garbage[:16]); got == cbcWantP0 {
		t.Errorf("P0 居然没被破坏: %q", got)
	}
	if got := string(garbage[32:]); got != `s:8:"password";s:5:"ppppp";}`+"\x04\x04\x04\x04" {
		t.Errorf("P2/P3 被改动: %q", got)
	}
}

// 第 2 步：flip 模式带泄露，核对补偿 IV 后整体明文 = aaaaa→admin。
func TestCbcFlipWithLeak(t *testing.T) {
	ct := cbcEncrypt(t, []byte(cbcTestPlain))

	// 先跑一次无泄露拿翻转后密文，解密得"服务器泄露"
	res1 := runCbc(t, cbcFlipParams(ct))
	m := regexp.MustCompile(`\[密文\] 翻转后 cipher（base64）: (\S+)`).FindStringSubmatch(res1.Stdout)
	if m == nil {
		t.Fatalf("第 1 步缺密文:\n%s", res1.Stdout)
	}
	flipped, _ := base64.StdEncoding.DecodeString(m[1])
	garbage := cbcDecrypt(t, flipped, []byte(cbcTestIV))

	p := cbcFlipParams(ct)
	p["leaked_b64"] = base64.StdEncoding.EncodeToString(garbage)
	res := runCbc(t, p)
	if res.Status != core.StatusSuccess {
		t.Fatalf("补偿模式失败: %s\n%s", res.Stderr, res.Stdout)
	}
	m2 := regexp.MustCompile(`\[补偿\] IV_new = IV \^ 垃圾 \^ 期望前缀 = ([A-Za-z0-9+/=]+)`).FindStringSubmatch(res.Stdout)
	if m2 == nil {
		t.Fatalf("输出里没有补偿 IV:\n%s", res.Stdout)
	}
	ivNew, err := base64.StdEncoding.DecodeString(m2[1])
	if err != nil {
		t.Fatal(err)
	}
	final := cbcDecrypt(t, flipped, ivNew)
	if got := string(final[:16]); got != cbcWantP0 {
		t.Errorf("补偿 IV 后 P0 = %q, 期望 %q", got, cbcWantP0)
	}
	if got := string(final[:60]); got != strings.Replace(cbcTestPlain, "aaaaa", "admin", 1) {
		t.Errorf("整体明文 = %q", got)
	}
}

// validate_params 语义（对应 Python 的 ValueError 分支文案）。
// 注意：Python 对空参数同样报错（flip 默认模式必填 plaintext），本用例含此回归。
func TestCbcValidate(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("crypto.aes_cbc_bitflip")
	v, err := tool.ValidateParams(map[string]any{
		"mode": "flip", "plaintext": "x", "orig": "a", "target": "b",
	})
	if err != nil {
		t.Fatalf("合法参数校验失败: %v", err)
	}
	if paramStr(v, "mode") != "flip" || paramStr(v, "session_cookie") != "PHPSESSID" ||
		paramStr(v, "leak_encoding") != "base64" || paramInt(v, "retries") != 3 ||
		paramStr(v, "leak_regex") != cbcDefaultLeakRe || paramStr(v, "success_regex") != cbcDefaultSuccessRe {
		t.Errorf("默认参数未填充: mode=%q session=%q leak=%q retries=%d",
			paramStr(v, "mode"), paramStr(v, "session_cookie"), paramStr(v, "leak_encoding"), paramInt(v, "retries"))
	}
	cases := []struct {
		params map[string]any
		want   string
	}{
		{map[string]any{}, "plaintext"},
		{map[string]any{"mode": "nope"}, "未知模式"},
		{map[string]any{"mode": "flip"}, "plaintext"},
		{map[string]any{"mode": "flip", "plaintext": "x", "orig": "ab", "target": "c"}, "必须等长"},
		{map[string]any{"mode": "flip", "plaintext": "x", "orig": "a", "target": "b", "input_encoding": "rot13"}, "编码必须"},
		{map[string]any{"mode": "attack", "plaintext": "x", "orig": "a", "target": "b", "source_type": "manual"}, "manual_cipher"},
		{map[string]any{"mode": "attack", "plaintext": "x", "orig": "a", "target": "b"}, "login_url"},
		{map[string]any{"mode": "oracle"}, "oracle_url"},
		{map[string]any{"mode": "oracle", "oracle_url": "http://x/"}, "oracle_ok"},
	}
	for _, c := range cases {
		if _, err := tool.ValidateParams(c.params); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("params=%v 期望错误含 %q, 实际 %v", c.params, c.want, err)
		}
	}
}

// run() 错误分支：[错误]（ValueError 类）与 [中止]（网络错误，含 Python 类名）。
func TestCbcRunErrors(t *testing.T) {
	// 找不到原片段 → "[错误]" + stderr
	res := runCbc(t, map[string]any{
		"mode": "flip", "plaintext": "hello", "orig": "zz", "target": "yy", "encoding": "ascii",
	})
	if res.Status != core.StatusError {
		t.Fatalf("status = %s", res.Status)
	}
	if !strings.Contains(res.Stdout, "[错误] 在已知明文里找不到原片段 b'zz'") {
		t.Errorf("缺 [错误] 行:\n%s", res.Stdout)
	}
	if res.Stderr == "" {
		t.Errorf("stderr 为空")
	}
	// 网络拒绝 → "[中止] URLError"（urllib 对连接拒绝的异常名）
	res = runCbc(t, map[string]any{
		"mode": "attack", "plaintext": cbcTestPlain, "orig": "aaaaa", "target": "admin",
		"encoding": "ascii", "login_url": "http://127.0.0.1:9/", "source_type": "cookie",
		"retries": 1,
	})
	if res.Status != core.StatusError {
		t.Fatalf("status = %s", res.Status)
	}
	if !strings.Contains(res.Stdout, "[中止] URLError:") {
		t.Errorf("缺 [中止] URLError 行:\n%s", res.Stdout)
	}
}

// oracle 模式：httptest 内嵌严格 PKCS7 oracle，恢复 2 块明文（含块 0）。
func TestCbcOracleRecover(t *testing.T) {
	plain2 := "flag{oracle_works_123}" // 22B，PKCS7 填充 10B → 32B = 2 块
	block, _ := aes.NewCipher([]byte(cbcTestKey))
	padded := append([]byte(plain2), 10, 10, 10, 10, 10, 10, 10, 10, 10, 10)
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, []byte(cbcTestIV)).CryptBlocks(ct, padded)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc, err := base64.StdEncoding.DecodeString(r.URL.Query().Get("payload"))
		if err != nil || len(enc) < 32 {
			w.WriteHeader(400)
			w.Write([]byte("INVALID PADDING"))
			return
		}
		prev, cur := enc[:16], enc[16:32]
		dec := make([]byte, 16)
		cipher.NewCBCDecrypter(block, prev).CryptBlocks(dec, cur)
		pad := int(dec[15])
		if pad >= 1 && pad <= 16 {
			valid := true
			for i := 16 - pad; i < 16; i++ {
				if int(dec[i]) != pad {
					valid = false
					break
				}
			}
			if valid {
				w.WriteHeader(200)
				w.Write([]byte("VALID PADDING"))
				return
			}
		}
		w.WriteHeader(400)
		w.Write([]byte("INVALID PADDING"))
	}))
	defer srv.Close()

	res := runCbc(t, map[string]any{
		"mode":             "oracle",
		"oracle_url":       srv.URL,
		"oracle_method":    "GET",
		"oracle_params":    "payload={payload}",
		"oracle_ok_status": 200,
		"target_cipher":    base64.StdEncoding.EncodeToString(ct),
		"iv_b64":           base64.StdEncoding.EncodeToString([]byte(cbcTestIV)),
	})
	if res.Status != core.StatusSuccess {
		t.Fatalf("oracle 失败: %s\n%s", res.Stderr, res.Stdout)
	}
	if !strings.Contains(res.Stdout, "[自检] Oracle 判定可区分（原密文=true，随机前缀=false）") {
		t.Errorf("缺自检行:\n%s", res.Stdout)
	}
	for _, want := range []string{"[恢复] 块 1:", "[恢复] 块 0:", "[结果] 去除 PKCS7 填充（10B）"} {
		if !strings.Contains(res.Stdout, want) {
			t.Errorf("缺 %q:\n%s", want, res.Stdout)
		}
	}
	m := regexp.MustCompile(`\[结果\] 明文文本（utf-8）: (.+)`).FindStringSubmatch(res.Stdout)
	if m == nil || m[1] != plain2 {
		t.Fatalf("恢复明文 = %q, 期望 %q\n%s", m, plain2, res.Stdout)
	}
}

// 注册表：元数据与默认参数。
func TestCbcToolRegistered(t *testing.T) {
	tool, ok := core.DefaultRegistry.Get("crypto.aes_cbc_bitflip")
	if !ok {
		t.Fatalf("未注册")
	}
	if tool.Version() != "1.1.0" || tool.Timeout() != 900 {
		t.Errorf("version=%s timeout=%d", tool.Version(), tool.Timeout())
	}
	names := map[string]bool{}
	for _, ps := range tool.Params() {
		names[ps.Name] = true
	}
	for _, k := range []string{"mode", "plaintext", "cipher_b64", "leak_regex", "success_regex",
		"oracle_url", "oracle_ok_status", "target_cipher", "iv_in_cipher"} {
		if !names[k] {
			t.Errorf("缺参数 %s", k)
		}
	}
}
