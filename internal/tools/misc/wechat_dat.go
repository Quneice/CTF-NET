// misc.wechat_dat —— 微信 FileStorage .dat 还原（对应 legacy/tools/misc/wechat_dat.py）。
// 单字节循环异或，文件头魔数逆推密钥；解密后落盘为产物。
package misc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ctfnet/internal/core"
)

// magic -> ext（可靠度排序：图片优先，与 Python MAGICS 一致）。
var wechatMagics = []struct {
	Magic []byte
	Ext   string
}{
	{[]byte("\x89PNG\r\n\x1a\n"), "png"},
	{[]byte("\xff\xd8\xff"), "jpg"},
	{[]byte("GIF87a"), "gif"},
	{[]byte("GIF89a"), "gif"},
	{[]byte("BM"), "bmp"},
	{[]byte("PK\x03\x04"), "zip"},
	{[]byte("Rar!\x1a\x07"), "rar"},
	{[]byte("RIFF"), "webp"},
}

func init() {
	core.DefaultRegistry.Register(&WechatDatTool{BaseTool: core.BaseTool{
		NameAttr:        "wechat_dat",
		CategoryAttr:    core.CatMisc,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "还原微信 FileStorage 加密 .dat 缓存（单字节循环异或，文件头逆推密钥）",
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "上传的 .dat 文件 file_id（见 POST /api/files）"},
			{Name: "key", Type: "str", Required: false, Default: "",
				Description: "可选：手动指定异或密钥（十六进制，如 9E）；留空则按文件头自动探测"},
		},
		Runner: wechatDatRun,
	}})
}

// WechatDatTool 微信 dat 工具。
type WechatDatTool struct {
	core.BaseTool
}

// wechatDetectKey 用常见文件头逆推单字节异或密钥，返回 (key, ext) 或 nil。
func wechatDetectKey(data []byte) (int, string, bool) {
	for _, m := range wechatMagics {
		if len(data) < len(m.Magic) {
			continue
		}
		key := int(data[0]) ^ int(m.Magic[0])
		ok := true
		for i := range m.Magic {
			if int(data[i])^key != int(m.Magic[i]) {
				ok = false
				break
			}
		}
		if ok {
			return key, m.Ext, true
		}
	}
	return 0, "", false
}

// wechatDetectFormat 解密后识别真实类型。
func wechatDetectFormat(data []byte) string {
	for _, m := range wechatMagics {
		if len(data) >= len(m.Magic) && string(data[:len(m.Magic)]) == string(m.Magic) {
			return m.Ext
		}
	}
	return "bin"
}

func wechatDatRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	filePath := filepath.Join(tc.Workdir, paramStr(params, "file"))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("读取文件失败: %v", err)}, nil
	}
	if len(data) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "文件为空"}, nil
	}

	manual := strings.TrimSpace(paramStr(params, "key"))
	var key int
	if manual != "" {
		v, err := strconv.ParseInt(manual, 16, 64)
		if err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("非法密钥（应为十六进制）: %s", manual)}, nil
		}
		if v < 0 || v > 255 {
			return &core.ToolResult{Status: core.StatusError, Stderr: "密钥需为单字节（0x00~0xFF）"}, nil
		}
		key = int(v)
	} else {
		hit, _, ok := wechatDetectKey(data)
		if !ok {
			return &core.ToolResult{
				Status: core.StatusError,
				Stderr: "未匹配到常见文件头，无法自动推断密钥。请提供 key 参数手动指定异或密钥。",
			}, nil
		}
		key = hit
	}

	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ byte(key)
	}
	ext := wechatDetectFormat(out)
	stem := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	outName := fmt.Sprintf("%s_decrypted.%s", stem, ext)
	if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, outName), out, 0o644); err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}

	return &core.ToolResult{
		Status: core.StatusSuccess,
		Stdout: fmt.Sprintf("[+] 密钥=0x%02X  类型=*.%s  大小=%dB -> %s\n", key, ext, len(out), outName),
		Data:   map[string]any{"key_hex": fmt.Sprintf("%02X", key), "format": ext, "size": len(out)},
	}, nil
}
