package traffic

// traffic.usb-keyboard:PCAP 中 USB HID 键盘流量还原(CTF 流量题最高频套路)。
// 解析 pcap(LINKTYPE_USB_LINUX 189 / MMAPPED 220),提取 URB data 区 8 字节 HID 报文
// (byte0 修饰键 / byte1 保留 / byte2-7 键码),按 HID Usage 码表还原按键序列;
// 报文去重(长按只计一次)、Shift 修饰、Enter/Backspace/空格等特殊键、flag 正则。

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ctfnet/internal/core"
	"ctfnet/internal/stego"
)

func init() {
	core.DefaultRegistry.Register(&USBKeyboardTool{BaseTool: core.BaseTool{
		NameAttr:        "usb-keyboard",
		CategoryAttr:    core.CatTraffic,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "USB 键盘流量还原:解析 pcap 提取 HID 键盘报文,按 HID 码表还原按键序列(Shift 大写/符号、Enter/Backspace),flag 正则命中",
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "USB 键盘流量的 pcap 文件"},
		},
		Runner: usbKeyboardRun,
	}})
}

// USBKeyboardTool USB 键盘流量还原工具。
type USBKeyboardTool struct {
	core.BaseTool
}

// ---------- HID 码表 ----------

var hidBase = [0x40]byte{
	0x04: 'a', 0x05: 'b', 0x06: 'c', 0x07: 'd', 0x08: 'e', 0x09: 'f', 0x0a: 'g',
	0x0b: 'h', 0x0c: 'i', 0x0d: 'j', 0x0e: 'k', 0x0f: 'l', 0x10: 'm', 0x11: 'n',
	0x12: 'o', 0x13: 'p', 0x14: 'q', 0x15: 'r', 0x16: 's', 0x17: 't', 0x18: 'u',
	0x19: 'v', 0x1a: 'w', 0x1b: 'x', 0x1c: 'y', 0x1d: 'z',
	0x1e: '1', 0x1f: '2', 0x20: '3', 0x21: '4', 0x22: '5', 0x23: '6', 0x24: '7',
	0x25: '8', 0x26: '9', 0x27: '0',
	0x28: '\n', // Enter
	0x29: 0,    // Esc
	0x2a: '\b', // Backspace
	0x2c: ' ',
	0x2d: '-', 0x2e: '=', 0x2f: '[', 0x30: ']', 0x31: '\\',
	0x33: ';', 0x34: '\'', 0x36: ',', 0x37: '.', 0x38: '/',
}

var hidShift = map[byte]byte{
	0x04: 'A', 0x05: 'B', 0x06: 'C', 0x07: 'D', 0x08: 'E', 0x09: 'F', 0x0a: 'G',
	0x0b: 'H', 0x0c: 'I', 0x0d: 'J', 0x0e: 'K', 0x0f: 'L', 0x10: 'M', 0x11: 'N',
	0x12: 'O', 0x13: 'P', 0x14: 'Q', 0x15: 'R', 0x16: 'S', 0x17: 'T', 0x18: 'U',
	0x19: 'V', 0x1a: 'W', 0x1b: 'X', 0x1c: 'Y', 0x1d: 'Z',
	0x1e: '!', 0x1f: '@', 0x20: '#', 0x21: '$', 0x22: '%', 0x23: '^', 0x24: '&',
	0x25: '*', 0x26: '(', 0x27: ')',
	0x28: '\n', 0x2a: '\b', 0x2c: ' ',
	0x2d: '_', 0x2e: '+', 0x2f: '{', 0x30: '}', 0x31: '|',
	0x33: ':', 0x34: '"', 0x36: '<', 0x37: '>', 0x38: '?',
}

// ---------- pcap 解析 ----------

// usbKeysFromPcap 解析 pcap,返回按序的 8 字节 HID 键盘报文。
func usbKeysFromPcap(data []byte) ([][]byte, error) {
	if len(data) < 24 {
		return nil, fmt.Errorf("文件过小,不是 pcap")
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	var le bool
	switch magic {
	case 0xa1b2c3d4, 0xa1b23c4d: // LE 标准/纳秒
		le = true
	case 0xd4c3b2a1, 0x4d3cb2a1: // BE
		le = false
	default:
		return nil, fmt.Errorf("pcap 魔数不符(不是 pcap 文件;pcapng 暂不支持,请先转 pcap)")
	}
	rd := func(b []byte) uint32 {
		if le {
			return binary.LittleEndian.Uint32(b)
		}
		return binary.BigEndian.Uint32(b)
	}
	linktype := rd(data[20:24]) & 0xffff
	urbHeader := 64
	switch linktype {
	case 189: // LINKTYPE_USB_LINUX
	case 220: // LINKTYPE_USB_LINUX_MMAPPED
		urbHeader = 64
	default:
		return nil, fmt.Errorf("linktype %d 非 USB(需要 189/220;先用 Wireshark 过滤 usb.capdata 导出)", linktype)
	}
	var frames [][]byte
	off := 24
	for off+16 <= len(data) {
		inclLen := int(rd(data[off+8 : off+12]))
		if inclLen < 0 || off+16+inclLen > len(data) {
			break
		}
		rec := data[off+16 : off+16+inclLen]
		off += 16 + inclLen
		if len(rec) <= urbHeader {
			continue
		}
		payload := rec[urbHeader:]
		// HID 键盘报文特征:8 字节,b1==0,键码 <0x40(0x2x/0x3x/0x4x…上限 0x65 取保守 0x40 内的常规键)
		if len(payload) >= 8 {
			f := payload[:8]
			if f[1] != 0 {
				continue
			}
			ok := true
			for _, c := range f[2:8] {
				if c >= 0x40 {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			frames = append(frames, append([]byte{}, f...))
		}
	}
	return frames, nil
}

// hidFramesToText 报文序列 → 按键文本(去重长按、修饰键、退格)。
func hidFramesToText(frames [][]byte) string {
	var sb strings.Builder
	var last [8]byte
	first := true
	for _, f := range frames {
		if first || f[0] != last[0] || f[2] != last[2] || f[3] != last[3] {
			shift := f[0]&0x22 != 0
			for _, c := range f[2:8] {
				if c == 0 {
					continue
				}
				var ch byte
				if shift {
					ch = hidShift[c]
				} else {
					if c < 0x40 {
						ch = hidBase[c]
					}
				}
				switch ch {
				case 0: // 未知键码(Esc 等)
				case '\b':
					s := sb.String()
					if len(s) > 0 {
						sb.Reset()
						sb.WriteString(s[:len(s)-1])
					}
				default:
					sb.WriteByte(ch)
				}
			}
		}
		last = *(*[8]byte)(f)
		first = false
	}
	return sb.String()
}

func usbKeyboardRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	data, err := os.ReadFile(filepath.Join(tc.Workdir, paramStr2(params, "file")))
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("读取文件失败: %v", err)}, nil
	}
	frames, err := usbKeysFromPcap(data)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	if len(frames) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "未发现 HID 键盘报文:请确认 pcap 为 USB 总线抓包(linktype 189/220)"}, nil
	}
	text := hidFramesToText(frames)
	flags := stego.Flags([]byte(text))
	if flags == nil {
		flags = []string{}
	}
	rep := map[string]any{
		"file":    paramStr2(params, "file"),
		"frames":  len(frames),
		"text":    text,
		"flags":   flags,
		"summary": fmt.Sprintf("USB 键盘报文 %d 帧,还原按键 %d 字符;flag 命中 %d 条", len(frames), len(text), len(flags)),
	}
	out, err := json.Marshal(rep)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: string(out),
		Data: map[string]any{"text": text, "flags": flags}}, nil
}

func paramStr2(params map[string]any, key string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
