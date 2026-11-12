// crypto.base64 —— 代码型工具：Base64 编码/解码（对应 legacy/tools/crypto/base64.py）。
package crypto

import (
	"context"
	"encoding/base64"
	"strings"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&Base64Tool{BaseTool: core.BaseTool{
		NameAttr:        "base64",
		CategoryAttr:    core.CatCrypto,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "Base64 编码 / 解码",
		ParamsAttr: []core.ParamSpec{
			{Name: "mode", Type: "str", Required: true, Description: "encode 或 decode"},
			{Name: "data", Type: "text", Required: true, Description: "要处理的数据"},
		},
		Runner: base64Run,
	}})
}

// Base64Tool Base64 工具。
type Base64Tool struct {
	core.BaseTool
}

func base64Run(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	mode, _ := params["mode"].(string)
	data, _ := params["data"].(string)
	switch mode {
	case "encode":
		return &core.ToolResult{Status: core.StatusSuccess, Stdout: base64.StdEncoding.EncodeToString([]byte(data))}, nil
	case "decode":
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
		}
		// Python: decode(errors="replace") —— 非法 UTF-8 替换为 U+FFFD
		return &core.ToolResult{Status: core.StatusSuccess, Stdout: strings.ToValidUTF8(string(decoded), "�")}, nil
	default:
		return &core.ToolResult{Status: core.StatusError, Stderr: "未知模式: " + mode}, nil
	}
}
