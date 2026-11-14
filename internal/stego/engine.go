// Package stego 通用 LSB 位平面隐写引擎:位平面打包、隐藏内容类型识别、
// 智能截断、flag 正则、可打印串提取。WAV(audio_lsb)与图像(image_lsb)工具共享。
package stego

import "regexp"

// ---------- 位平面打包 ----------

// Pack 把采样位平面打包成字节流。
// count/sample:采样总数与取值(任意整数语义,位运算无需区分符号);
// plane..plane+bits-1 位按升序每采样贡献 bits 位;msbFirst 决定首位进字节最高位还是最低位;
// include 过滤采样(如只取某声道;nil=全部)。返回 字节流/1 的个数/总位数。
func Pack(count int, sample func(int) uint64, plane, bits int, msbFirst bool, include func(int) bool) (out []byte, ones, total int) {
	if include == nil {
		include = func(int) bool { return true }
	}
	// 先数一遍有效采样,精确分配
	n := 0
	for i := 0; i < count; i++ {
		if include(i) {
			n++
		}
	}
	out = make([]byte, n*bits/8+1)
	bitIdx := 0
	for i := 0; i < count; i++ {
		if !include(i) {
			continue
		}
		v := sample(i)
		for k := 0; k < bits; k++ {
			bit := int((v >> uint(plane+k)) & 1)
			ones += bit
			total++
			if msbFirst {
				out[bitIdx>>3] |= byte(bit << (7 - uint(bitIdx&7)))
			} else {
				out[bitIdx>>3] |= byte(bit << uint(bitIdx&7))
			}
			bitIdx++
		}
	}
	return out[:bitIdx/8], ones, total
}

// ---------- 类型识别 ----------

type Magic struct {
	Name, Ext, Mime string
	Prefix          []byte
	Anywhere        bool // 允许窗口内搜索(≥3 字节魔数);2 字节魔数仅在偏移 0 判定
}

var Magics = []Magic{
	{"JPEG", "jpg", "image/jpeg", []byte{0xFF, 0xD8, 0xFF}, true},
	{"PNG", "png", "image/png", []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}, true},
	{"GIF", "gif", "image/gif", []byte("GIF8"), true},
	{"BMP", "bmp", "image/bmp", []byte("BM"), false},
	{"ZIP", "zip", "application/zip", []byte("PK\x03\x04"), true},
	{"ZIP(空)", "zip", "application/zip", []byte("PK\x05\x06"), true},
	{"GZIP", "gz", "application/gzip", []byte{0x1F, 0x8B}, true},
	{"RAR", "rar", "application/vnd.rar", []byte("Rar!\x1A\x07"), true},
	{"7-Zip", "7z", "application/x-7z-compressed", []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}, true},
	{"PDF", "pdf", "application/pdf", []byte("%PDF-"), true},
	{"SQLite", "db", "application/x-sqlite3", []byte("SQLite format 3\x00"), true},
	{"ELF", "elf", "application/x-elf", []byte{0x7F, 'E', 'L', 'F'}, true},
	{"EXE", "exe", "application/x-msdownload", []byte("MZ"), false},
	{"WEBP", "webp", "image/webp", []byte("RIFF"), false},
}

var FlagRe = regexp.MustCompile(`(?i)\b(?:flag|ctf|key)\{[!-~]{2,200}\}`)

const (
	ScanWindow = 64 * 1024 // 魔数/flag 扫描窗口
	MaxSave    = 4 << 20   // 未知类型产物上限
	MaxPreview = 4 << 20   // base64 预览上限
	MaxText    = 4096      // 文本产物上限
)

// Detect 在打包流上识别隐藏内容:返回 (类型名, 扩展名, mime, 数据偏移)。类型名为空 = 未命中。
func Detect(bs []byte) (name, ext, mime string, offset int) {
	win := bs
	if len(win) > ScanWindow {
		win = win[:ScanWindow]
	}
	// 1) 偏移 0:全部魔数;WEBP 特判(RIFF + 偏移 8 处 WEBP)
	for _, m := range Magics {
		if len(win) >= len(m.Prefix) && string(win[:len(m.Prefix)]) == string(m.Prefix) {
			if m.Name == "WEBP" && !(len(win) >= 12 && string(win[8:12]) == "WEBP") {
				continue
			}
			return m.Name, m.Ext, m.Mime, 0
		}
	}
	// 2) 窗口内搜索:≥3 字节魔数(降低噪声误报)
	for _, m := range Magics {
		if !m.Anywhere || m.Name == "WEBP" {
			continue
		}
		if idx := indexOf(win, m.Prefix); idx > 0 {
			return m.Name, m.Ext, m.Mime, idx
		}
	}
	// 3) 文本启发式
	if isTextStream(win) {
		return "TEXT", "txt", "text/plain", 0
	}
	return "", "", "", -1
}

// TypeOf type 字段语义:二进制取扩展名(jpg/png/zip…),文本固定 text。
func TypeOf(name, ext string) string {
	if name == "TEXT" {
		return "text"
	}
	return ext
}

func indexOf(hay, needle []byte) int {
	n := len(hay) - len(needle)
	for i := 0; i <= n; i++ {
		match := true
		for j := range needle {
			if hay[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func IsPrintableByte(b byte) bool {
	return b >= 0x20 && b <= 0x7E || b == '\t' || b == '\n' || b == '\r'
}

func isTextStream(win []byte) bool {
	n := len(win)
	if n < 32 {
		return false
	}
	if n > 256 {
		n = 256
	}
	p := 0
	distinct := map[byte]bool{}
	dn := n
	if dn > 64 {
		dn = 64
	}
	for i, b := range win[:n] {
		if IsPrintableByte(b) {
			p++
		}
		if i < dn {
			distinct[b] = true
		}
	}
	// 可打印比例 ≥0.9,且前 64 字节至少 5 种不同字节
	// (单字节重复流——如方波/静音载体的位平面打包后是 'UUUU…'——不算文本)
	return float64(p)/float64(n) >= 0.9 && len(distinct) >= 5
}

// ---------- 智能截断 ----------

// Cut 按类型截取有效载荷(JPEG→EOI、PNG→IEND、文本→可打印前缀,其余上限 4MB)。
func Cut(bs []byte, offset int, typ string) []byte {
	seg := bs[offset:]
	if len(seg) > MaxSave {
		seg = seg[:MaxSave]
	}
	switch typ {
	case "JPEG":
		if idx := indexOf(seg[4:], []byte{0xFF, 0xD9}); idx >= 0 {
			seg = seg[:idx+4+2]
		}
	case "PNG":
		if idx := indexOf(seg, []byte{0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D', 0xAE, 0x42, 0x60, 0x82}); idx >= 0 {
			seg = seg[:idx+12]
		}
	case "TEXT":
		end := 0
		for end < len(seg) && end < MaxText && IsPrintableByte(seg[end]) {
			end++
		}
		seg = seg[:end]
	}
	return seg
}

// ---------- flag 与可打印串 ----------

// Flags 提取窗口内全部 flag 形态串(去重保序)。无命中返回空切片(非 nil)——
// JSON 序列化给前端时,nil 会变 null,读 .length 直接崩溃。
func Flags(win []byte) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, f := range FlagRe.FindAllString(string(win), -1) {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// PrintableStrings 提取 ≥minLen 的可打印串(用于原始字节 strings 扫描),最多 max 条。
// 同 Flags:永远返回非 nil 切片。
func PrintableStrings(data []byte, minLen, max int) []string {
	out := []string{}
	start := -1
	for i := 0; i <= len(data); i++ {
		if i < len(data) && IsPrintableByte(data[i]) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			if i-start >= minLen {
				out = append(out, string(data[start:i]))
				if len(out) >= max {
					break
				}
			}
			start = -1
		}
	}
	return out
}
