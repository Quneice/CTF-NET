// classical.go 的算法验证：教科书已知答案 + 加解密往返 + 注册表冒烟。
package crypto

import (
	"context"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

func TestCaesarKnownAndRoundTrip(t *testing.T) {
	if got := caesarShift("abc XYZ", 3); got != "def ABC" {
		t.Fatalf("caesar enc: %s", got)
	}
	if got := caesarShift(caesarShift("Attack at Dawn!", 7), -7); got != "Attack at Dawn!" {
		t.Fatalf("caesar roundtrip: %s", got)
	}
	// Runner 级穷举
	res, err := runCaesar(context.Background(), map[string]any{
		"data": "Khoor Zruog", "shift": 3, "mode": "decrypt",
	}, nil)
	if err != nil || res.Stdout != "Hello World" {
		t.Fatalf("caesar decrypt runner: %v %q", err, res.Stdout)
	}
}

func TestVigenereAlphaKnown(t *testing.T) {
	// 经典教科书向量：ATTACKATDAWN + LEMON → LXFOPVEFRNHR
	if got := vigenereAlphaRun("ATTACKATDAWN", "LEMON", 1); got != "LXFOPVEFRNHR" {
		t.Fatalf("vigenere alpha known: %s", got)
	}
	if got := vigenereAlphaRun("LXFOPVEFRNHR", "LEMON", -1); got != "ATTACKATDAWN" {
		t.Fatalf("vigenere alpha dec: %s", got)
	}
}

func TestVigenerePrintableRoundTrip(t *testing.T) {
	// printable 字母表（CryptoDen 兼容）：往返
	msg := "flag{vigenere_printable_123!}"
	key := "Key42"
	enc := vigenerePrintableRun(msg, key, 1)
	if got := vigenerePrintableRun(enc, key, -1); got != msg {
		t.Fatalf("vigenere printable roundtrip: %q -> %q -> %q", msg, enc, got)
	}
	// Python string.printable 长度 100
	if len(vigenerePrintable) != 100 {
		t.Fatalf("printable alphabet len = %d, want 100", len(vigenerePrintable))
	}
}

func TestAffineKnown(t *testing.T) {
	// 维基百科经典例：affine(5,8) "AFFINE" → "IHHWVC"
	if got := affineRun("AFFINE", 5, 8, 1); got != "IHHWVC" {
		t.Fatalf("affine known: %s", got)
	}
	if got := affineRun("IHHWVC", 5, 8, -1); got != "AFFINE" {
		t.Fatalf("affine dec: %s", got)
	}
}

func TestAtbashKnown(t *testing.T) {
	if got := runText(t, runAtbash, "data", "HELLO"); got != "SVOOL" {
		t.Fatalf("atbash: %s", got)
	}
}

func TestReverse(t *testing.T) {
	if got := runText(t, runReverse, "data", "abc 中文"); got != "文中 cba" {
		t.Fatalf("reverse: %s", got)
	}
}

func TestPolybiusKnown(t *testing.T) {
	if got := runText(t, runPolybius, "data", "hello", "mode", "encrypt"); got != "23 15 31 31 34" {
		t.Fatalf("polybius enc: %s", got)
	}
	if got := runText(t, runPolybius, "data", "23 15 31 31 34", "mode", "decrypt"); got != "hello" {
		t.Fatalf("polybius dec: %s", got)
	}
}

func TestXORKnown(t *testing.T) {
	// 'a'=0x61 ^1=0x60, 'b'=0x62 ^1=0x63, 'c'=0x63 ^1=0x62
	if got := runText(t, runXOR, "data", "abc", "key", float64(1), "out", "hex"); got != "60 63 62" {
		t.Fatalf("xor hex: %s", got)
	}
	if got := runText(t, runXOR, "data", "abc", "key", float64(1), "out", "dec"); got != "96 99 98" {
		t.Fatalf("xor dec: %s", got)
	}
}

func TestHillKnown(t *testing.T) {
	// 教科书经典：K=[[3,3],[2,5]] "help" → "hiat"
	if got := runText(t, runHill, "data", "help",
		"k1", float64(3), "k2", float64(3), "k3", float64(2), "k4", float64(5),
		"mode", "encrypt"); got != "hiat" {
		t.Fatalf("hill known: %s", got)
	}
	if got := runText(t, runHill, "data", "hiat",
		"k1", float64(3), "k2", float64(3), "k3", float64(2), "k4", float64(5),
		"mode", "decrypt"); got != "help" {
		t.Fatalf("hill dec: %s", got)
	}
	// 奇数长度补 x / 解密去填充
	if got := runText(t, runHill, "data", "cat",
		"k1", float64(3), "k2", float64(3), "k3", float64(2), "k4", float64(5),
		"mode", "encrypt"); len(got) != 4 {
		t.Fatalf("hill pad enc len: %d (%s)", len(got), got)
	}
	if got := runText(t, runHill, "data", runText(t, runHill, "data", "cat",
		"k1", float64(3), "k2", float64(3), "k3", float64(2), "k4", float64(5),
		"mode", "encrypt"),
		"k1", float64(3), "k2", float64(3), "k3", float64(2), "k4", float64(5),
		"mode", "decrypt"); got != "cat" {
		t.Fatalf("hill odd roundtrip: %s", got)
	}
	// 不可逆密钥报错
	if _, err := runHill(context.Background(), map[string]any{
		"data": "help", "k1": float64(2), "k2": float64(4), "k3": float64(6), "k4": float64(8),
	}, nil); err == nil {
		t.Fatal("hill even det should fail")
	}
}

func TestColumnarKnownAndRoundTrip(t *testing.T) {
	// CryptoDen 等宽模式：key="3" 时 "attackatdawn" → "aaaatctwtkdn"
	if got := runText(t, runColumnar, "data", "attackatdawn", "key", "3", "mode", "encrypt"); got != "aaaatctwtkdn" {
		t.Fatalf("columnar width3: %s", got)
	}
	if got := runText(t, runColumnar, "data", "aaaatctwtkdn", "key", "3", "mode", "decrypt"); got != "attackatdawn" {
		t.Fatalf("columnar width3 dec: %s", got)
	}
	// 关键词模式与数字列序模式往返
	for _, key := range []string{"HACK", "3512", "ZEBRA", "1234"} {
		enc := runText(t, runColumnar, "data", "the quick brown fox jumps", "key", key, "mode", "encrypt")
		dec := runText(t, runColumnar, "data", enc, "key", key, "mode", "decrypt")
		if dec != "thequickbrownfoxjumps" {
			t.Fatalf("columnar key=%s roundtrip: %q", key, dec)
		}
	}
}

func TestBifidRoundTrip(t *testing.T) {
	for _, msg := range []string{"ATTACKATDAWN", "DEFENDTHEEASTWALLOFTHECASTLE"} {
		enc := runText(t, runBifid, "data", msg, "key", "crypto", "mode", "encrypt")
		dec := runText(t, runBifid, "data", enc, "key", "crypto", "mode", "decrypt")
		if dec != strings.ReplaceAll(msg, "J", "I") {
			t.Fatalf("bifid roundtrip %q: %q", msg, dec)
		}
	}
}

func TestADFGXRoundTrip(t *testing.T) {
	enc := runText(t, runADFGX, "data", "ATTACKATDAWN", "key1", "ph0", "key2", "GERMAN", "mode", "encrypt")
	dec := runText(t, runADFGX, "data", enc, "key1", "ph0", "key2", "GERMAN", "mode", "decrypt")
	if dec != "ATTACKATDAWN" {
		t.Fatalf("adfgx roundtrip: %q -> %q", enc, dec)
	}
	if len(enc) != 24 { // 12 字母 × 2 坐标
		t.Fatalf("adfgx len: %d", len(enc))
	}
}

func TestADFGVXRoundTrip(t *testing.T) {
	// 6×6 含数字明文
	enc := runText(t, runADFGVX, "data", "ATTACK2026", "key1", "n0talg", "key2", "PRIVACY", "mode", "encrypt")
	dec := runText(t, runADFGVX, "data", enc, "key1", "n0talg", "key2", "PRIVACY", "mode", "decrypt")
	if dec != "ATTACK2026" {
		t.Fatalf("adfgvx roundtrip: %q -> %q", enc, dec)
	}
}

func TestFourSquareRoundTrip(t *testing.T) {
	// 无相邻重复字母 → 严格往返
	msg := "ATTACKATDAWN"
	enc := runText(t, runFourSquare, "data", msg, "key1", "example", "key2", "keyword", "mode", "encrypt")
	dec := runText(t, runFourSquare, "data", enc, "key1", "example", "key2", "keyword", "mode", "decrypt")
	if dec != msg {
		t.Fatalf("foursquare roundtrip: %q -> %q -> %q", msg, enc, dec)
	}
	// 相邻重复字母插 X：BALLOON 中 L,L 成对被替换为 L,X（O,O 落在奇偏移不构成双字母组），
	// 与 CryptoDen 逐对扫描行为一致 → BALXLOON
	enc2 := runText(t, runFourSquare, "data", "BALLOON", "key1", "example", "key2", "keyword", "mode", "encrypt")
	dec2 := runText(t, runFourSquare, "data", enc2, "key1", "example", "key2", "keyword", "mode", "decrypt")
	if dec2 != "BALXLOON" {
		t.Fatalf("foursquare insert X: BALLOON -> %q", dec2)
	}
}

// TestClassicalRegistryRegistered 注册表冒烟：13 个工具全部注册成功。
func TestClassicalRegistryRegistered(t *testing.T) {
	ids := []string{
		"crypto.caesar", "crypto.vigenere", "crypto.affine", "crypto.atbash", "crypto.reverse",
		"crypto.polybius", "crypto.xor", "crypto.hill", "crypto.columnar", "crypto.bifid",
		"crypto.adfgx", "crypto.adfgvx", "crypto.foursquare",
	}
	for _, id := range ids {
		if tool, ok := core.DefaultRegistry.Get(id); !ok || tool == nil {
			t.Errorf("tool %s not registered", id)
		}
	}
}

// runText 便捷封装：执行 runner 并断言成功，返回 Stdout。
func runText(t *testing.T, fn func(context.Context, map[string]any, *core.ToolContext) (*core.ToolResult, error),
	kv ...any) string {
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
