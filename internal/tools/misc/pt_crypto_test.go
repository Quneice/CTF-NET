package misc

// PT(Packet Tracer)容器解密引擎测试:Twofish 官方向量、合成往返、坏输入。
// 金标准对拍(live 门控)见 pt_decode_test.go。

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
)

// 1. Twofish-128 官方向量(writeup 验证:t1 旋转方向是唯一易错点)。
func TestTwofishVector128(t *testing.T) {
	tf := newTwofish(make([]byte, 16))
	ct := tf.encryptBlock(make([]byte, 16))
	if got := hex.EncodeToString(ct); got != "9f589f5cf6122c32b6bfec2f2ae8c35a" {
		t.Fatalf("128 位向量不符: %s", got)
	}
	if pt := tf.decryptBlock(ct); !bytes.Equal(pt, make([]byte, 16)) {
		t.Fatalf("128 位解密不还原: %x", pt)
	}
}

// 2. Twofish-256 参考实现自带的断言向量(Gladman 移植源底部)。
func TestTwofishVector256(t *testing.T) {
	key, _ := hex.DecodeString("d43bb7556ea32e46f2a282b7d45b4e0d57ff739d4dc92c1bd7fc01700cc8216f")
	pt, _ := hex.DecodeString("90afe91bb288544f2c32dc239b2635e6")
	wantCt, _ := hex.DecodeString("6cb4561c40bf0a9705931cb6d408e7fa")
	tf := newTwofish(key)
	if ct := tf.encryptBlock(pt); !bytes.Equal(ct, wantCt) {
		t.Fatalf("256 位向量不符: %x", ct)
	}
	if back := tf.decryptBlock(wantCt); !bytes.Equal(back, pt) {
		t.Fatalf("256 位解密不还原: %x", back)
	}
}

// 3. 容器合成往返:encryptPT(decryptPT 的逆)∘decryptPT = identity。
func TestPTRoundtrip(t *testing.T) {
	plain := bytes.Repeat([]byte("<PACKETTRACER5>RoundTrip!"), 3000) // 69KB,多块
	enc := encryptPT(plain)
	got, err := decryptPT(enc)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("往返不一致: got %d B, want %d B", len(got), len(plain))
	}
}

// 4. 坏输入:纯文本 → 友好错误(不 panic);过短文件 → 错误。
func TestPTBadInput(t *testing.T) {
	if _, err := decryptPT([]byte("this is definitely not a packet tracer file")); err == nil {
		t.Fatal("纯文本应报错")
	}
	if _, err := decryptPT(make([]byte, 20)); err == nil {
		t.Fatal("过短文件应报错")
	}
	// 恰好 36 字节的垃圾(过门槛但 zlib 必失败)
	if _, err := decryptPT(bytes.Repeat([]byte{0xAB}, 36)); err == nil {
		t.Fatal("垃圾数据应报错")
	}
}

// 5. 解密统计可用:challenge.xml 前 4KB 也能独立往返(zlib 流截断场景之外的正道用法)。
func TestPTSmallRoundtrip(t *testing.T) {
	plain := []byte("<PACKETTRACER5>\n <VERSION>7.3.0</VERSION>\n</PACKETTRACER5>")
	got, err := decryptPT(encryptPT(plain))
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("小块往返失败: %v", err)
	}
	if _, err := os.Stat("."); err != nil {
		t.Fatal(err)
	}
}
