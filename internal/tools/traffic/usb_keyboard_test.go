package traffic

// traffic.usb-keyboard 测试:合成 pcap(LINKTYPE_USB_LINUX 189)还原 HID 键盘按键。

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ctfnet/internal/core"
)

// synthPcapUSB 构造 pcap:全局头(linktype 189)+ 每键一条 USB 记录(64B URB 头 + 8B HID)。
func synthPcapUSB(hidFrames [][]byte) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, uint32(0xa1b2c3d4))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(4))
	binary.Write(&b, binary.LittleEndian, int32(0))
	binary.Write(&b, binary.LittleEndian, uint32(0))
	binary.Write(&b, binary.LittleEndian, uint32(0xffff))
	binary.Write(&b, binary.LittleEndian, uint32(189))
	for i, fr := range hidFrames {
		rec := make([]byte, 0, 16+64+8)
		rh := make([]byte, 16)
		binary.LittleEndian.PutUint32(rh[0:], uint32(i))          // ts_sec
		binary.LittleEndian.PutUint32(rh[8:], uint32(64+len(fr))) // incl_len
		rec = append(rec, rh...)
		urb := make([]byte, 64)
		binary.LittleEndian.PutUint32(urb[36:], uint32(len(fr))) // data length
		rec = append(rec, urb...)
		rec = append(rec, fr...)
		b.Write(rec)
	}
	return b.Bytes()
}

// hidKey 生成 8B HID 报文:修饰键 + 6 键码(取一码)。
func hidKey(mod byte, code byte) []byte {
	f := make([]byte, 8)
	f[0] = mod
	f[2] = code
	return f
}

// keySeq 生成「按下-抬起」帧序列(真实键盘行为)。
func keySeq(mod byte, codes ...byte) [][]byte {
	var out [][]byte
	for _, c := range codes {
		out = append(out, hidKey(mod, c), make([]byte, 8)) // 按 + 抬
	}
	return out
}

func usbRunTool(t *testing.T, pcap []byte) (map[string]any, string) {
	t.Helper()
	tool, ok := core.DefaultRegistry.Get("traffic.usb-keyboard")
	if !ok {
		t.Fatal("traffic.usb-keyboard 未注册")
	}
	work := t.TempDir()
	arts := t.TempDir()
	os.WriteFile(filepath.Join(work, "k.pcap"), pcap, 0o644)
	res, err := tool.Run(context.Background(), map[string]any{"file": "k.pcap"}, &core.ToolContext{Workdir: work, ArtifactsDir: arts})
	if err != nil {
		t.Fatalf("运行出错: %v", err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("status %s: %s", res.Status, res.Stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &m); err != nil {
		t.Fatalf("stdout 非合法 JSON: %v\n%.200s", err, res.Stdout)
	}
	return m, arts
}

// 1. 基本还原:小写 "flag{usb}"(Shift+u 大写场景另测)。
func TestUSBKeyboardBasic(t *testing.T) {
	// f l a g Shift+{ u s b Shift+}
	frames := keySeq(0, 0x09, 0x0f, 0x04, 0x0a)             // f l a g
	frames = append(frames, keySeq(0x02, 0x2f)...)          // {
	frames = append(frames, keySeq(0, 0x18, 0x16, 0x05)...) // u s b
	frames = append(frames, keySeq(0x02, 0x30)...)          // }
	m, _ := usbRunTool(t, synthPcapUSB(frames))
	if m["text"] != "flag{usb}" {
		t.Fatalf("还原文本 = %v", m["text"])
	}
}

// 2. Shift 大写与符号:"F1" → Shift+f, 1。
func TestUSBKeyboardShift(t *testing.T) {
	frames := keySeq(0x02, 0x09)                   // Shift+f → F
	frames = append(frames, keySeq(0x02, 0x1e)...) // Shift+1 → !
	m, _ := usbRunTool(t, synthPcapUSB(frames))
	if m["text"] != "F!" {
		t.Fatalf("Shift 还原 = %v", m["text"])
	}
}

// 3. Enter/Backspace/空格。
func TestUSBKeyboardSpecial(t *testing.T) {
	frames := keySeq(0, 0x04, 0x2c, 0x05)             // a space b
	frames = append(frames, keySeq(0, 0x2a)...)       // Backspace
	frames = append(frames, keySeq(0, 0x2c, 0x28)...) // space Enter
	m, _ := usbRunTool(t, synthPcapUSB(frames))
	if m["text"] != "a  \n" { // a 空格 b → 退格删 b → 空格 → 回车
		t.Fatalf("特殊键还原 = %q", m["text"])
	}
}

// 4. 非 pcap / 无 USB 数据 → 友好错误或空结果。
func TestUSBKeyboardBad(t *testing.T) {
	if _, err := usbKeysFromPcap([]byte("not a pcap at all here")); err == nil {
		t.Fatal("非 pcap 应报错")
	}
}

// 5. flag 正则命中。
func TestUSBKeyboardFlag(t *testing.T) {
	frames := keySeq(0, 0x09, 0x0f, 0x04, 0x0a)       // f l a g
	frames = append(frames, keySeq(0x02, 0x2f)...)    // {
	frames = append(frames, keySeq(0, 0x18, 0x16)...) // u s
	frames = append(frames, keySeq(0x02, 0x30)...)    // }
	m, _ := usbRunTool(t, synthPcapUSB(frames))
	flags, _ := m["flags"].([]any)
	if len(flags) != 1 || flags[0] != "flag{us}" {
		t.Fatalf("flag 命中 = %v", flags)
	}
	if !strings.Contains(m["summary"].(string), "按键") {
		t.Fatalf("summary 应含统计: %v", m["summary"])
	}
}
