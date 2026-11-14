// 通用 LSB 隐写引擎单元测试:位平面打包、魔数识别、智能截断、flag 正则。
// 这些向量同时是 WAV/图像两个工具共享行为的规格。
package stego

import (
	"bytes"
	"strings"
	"testing"
)

func TestPackKnownVector(t *testing.T) {
	// 采样值 = 0..7;bit0 平面位流 = 0,1,0,1,1,0,1,1 → MSB-first 首字节 0b01011011=0x5B
	samples := []uint64{0, 1, 0, 1, 1, 0, 1, 1, 1, 0, 1, 0, 1, 0, 1, 0}
	get := func(i int) uint64 { return samples[i] }
	all := func(int) bool { return true }

	out, ones, total := Pack(len(samples), get, 0, 1, true, all)
	if len(out) != 2 || out[0] != 0x5B {
		t.Fatalf("MSB-first 打包错误: % X", out)
	}
	if ones != 9 || total != 16 {
		t.Fatalf("ones/total 错误: %d/%d", ones, total)
	}

	// LSB-first 同位流首字节 = 0b11011010=0xDA
	out, _, _ = Pack(len(samples), get, 0, 1, false, all)
	if out[0] != 0xDA {
		t.Fatalf("LSB-first 打包错误: % X", out)
	}

	// bits=2(plane=1):采样 0..7 的 (bit1,bit2) 升序:
	// i=0→00 1→00 2→10 3→10 4→01 5→01 6→11 7→11 → 位流 00001010 01011111 = 0x0A,0x5F
	out, _, _ = Pack(8, func(i int) uint64 { return uint64(i) }, 1, 2, true, all)
	if len(out) != 2 || out[0] != 0x0A || out[1] != 0x5F {
		t.Fatalf("2bit 打包错误: % X (期望 0A 5F)", out)
	}

	// include 过滤(如双声道只取声道 1)
	out, _, total = Pack(8, func(i int) uint64 { return samples[i] }, 0, 1, true, func(i int) bool { return i%2 == 1 })
	if total != 4 || len(out) != 0 { // 4 bit 不足 1 字节 → 空输出
		t.Fatalf("过滤后打包错误: %v total=%d", out, total)
	}
}

func TestDetect(t *testing.T) {
	jpg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{7}, 64)...)
	name, ext, _, off := Detect(jpg)
	if name != "JPEG" || ext != "jpg" || off != 0 {
		t.Fatalf("JPEG 偏移 0 识别错误: %s/%s/%d", name, ext, off)
	}

	// PNG 藏在流中间(前 32 字节噪声)
	stream := append(bytes.Repeat([]byte{0xAA}, 32), append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}, make([]byte, 64)...)...)
	name, _, _, off = Detect(stream)
	if name != "PNG" || off != 32 {
		t.Fatalf("PNG 窗口内搜索错误: %s off=%d", name, off)
	}

	// 2 字节魔数(BM/MZ)不参与窗口内搜索,只认偏移 0
	stream = append(bytes.Repeat([]byte{0x55}, 16), append([]byte("BMxx"), make([]byte, 64)...)...)
	if name, _, _, off = Detect(stream); name != "" {
		t.Fatalf("BM 不应在窗口中部命中: %s off=%d", name, off)
	}
	if name, ext, _, _ = Detect(append([]byte("MZ\x90\x00"), make([]byte, 64)...)); name != "EXE" || ext != "exe" {
		t.Fatalf("MZ 偏移 0 应识别 EXE: %s", name)
	}

	// WEBP 特判:RIFF 但偏移 8 非 WEBP → 不命中;是 WEBP → 命中
	riff := append([]byte("RIFF\x00\x00\x00\x00AVI "), make([]byte, 64)...)
	if name, _, _, _ = Detect(riff); name != "" {
		t.Fatalf("RIFF/AVI 不应识别为 WEBP: %s", name)
	}
	riff[8], riff[9], riff[10], riff[11] = 'W', 'E', 'B', 'P'
	if name, _, _, _ = Detect(riff); name != "WEBP" {
		t.Fatalf("WEBP 识别失败: %s", name)
	}

	// 文本:可打印比例 ≥0.9 且字节多样性足够
	if name, ext, _, _ = Detect([]byte("flag{h3ll0_w0rld} this is a hidden text message!!")); name != "TEXT" || ext != "txt" {
		t.Fatalf("文本识别失败: %s", name)
	}
	// 重复单字节流('UUUU…',方波载波打包产物)不算文本
	if name, _, _, _ = Detect(bytes.Repeat([]byte("U"), 128)); name != "" {
		t.Fatalf("重复流误报文本: %s", name)
	}
	// 噪声不命中
	noise := []byte{0x8F, 0x13, 0xC7, 0x00, 0xFE, 0x31, 0x9A, 0x02, 0x77, 0xB4, 0x15, 0xE9, 0x60, 0x3C, 0xAD, 0x91,
		0x2E, 0xF7, 0x48, 0x0B, 0xD6, 0x23, 0x9E, 0x71, 0x4A, 0xC5, 0x18, 0x83, 0x36, 0xAF, 0x62, 0x2D}
	noise = append(noise, noise...)
	if name, _, _, _ = Detect(noise); name != "" {
		t.Fatalf("噪声误报: %s", name)
	}
}

func TestCut(t *testing.T) {
	// JPEG:FFD9 截断含 EOI
	jpg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, append(bytes.Repeat([]byte{1}, 100), 0xFF, 0xD9, 0xAA, 0xBB)...)
	got := Cut(jpg, 0, "JPEG")
	if !bytes.Equal(got, jpg[:106]) {
		t.Fatalf("JPEG EOI 截断错误: %d 字节", len(got))
	}
	// PNG:IEND 截断
	png := append([]byte{0x89, 'P', 'N', 'G'}, append(bytes.Repeat([]byte{2}, 50),
		0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xAE, 0x42, 0x60, 0x82, 0xCC, 0xCC)...)
	got = Cut(png, 0, "PNG")
	if !bytes.Equal(got, png[:4+50+12]) {
		t.Fatalf("PNG IEND 截断错误: %d 字节", len(got))
	}
	// TEXT:可打印前缀截止
	got = Cut([]byte("flag{ok}\x00\xFFgarbage"), 0, "TEXT")
	if string(got) != "flag{ok}" {
		t.Fatalf("TEXT 截断错误: %q", got)
	}
	// 未知类型:上限截断
	big := bytes.Repeat([]byte{9}, 5<<20)
	if got = Cut(big, 0, ""); len(got) != 4<<20 {
		t.Fatalf("未知类型上限截断错误: %d", len(got))
	}
}

func TestFlagsAndTypeOf(t *testing.T) {
	fs := Flags([]byte("xx flag{a1b2} yy KEY{s3cret} zz ctf{Ch4ll} flag{a1b2}"))
	if len(fs) != 3 || fs[0] != "flag{a1b2}" || fs[1] != "KEY{s3cret}" || fs[2] != "ctf{Ch4ll}" {
		t.Fatalf("flag 提取错误: %v", fs)
	}
	if len(Flags([]byte("nothing here {braces} flg{nope}"))) != 0 {
		t.Fatal("不应误报")
	}
	if TypeOf("TEXT", "txt") != "text" || TypeOf("JPEG", "jpg") != "jpg" {
		t.Fatal("TypeOf 语义错误")
	}
}

func TestPrintableStrings(t *testing.T) {
	out := PrintableStrings([]byte("\x00\x01hi there\x02flag{in_raw}\x03\x00trailing"), 6, 10)
	// "hi there\x02..." — \x02 终止;flag{in_raw} 应整体提取
	joined := strings.Join(out, "|")
	if !strings.Contains(joined, "flag{in_raw}") || !strings.Contains(joined, "hi there") {
		t.Fatalf("可打印串提取错误: %v", out)
	}
}
