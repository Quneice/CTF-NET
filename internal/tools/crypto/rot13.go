// crypto.rot13 —— ROT13 凯撒移位（对应 legacy/tools/crypto/rot13.py）。
package crypto

import (
	"context"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&Rot13Tool{BaseTool: core.BaseTool{
		NameAttr:        "rot13",
		CategoryAttr:    core.CatCrypto,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "ROT13 凯撒移位（位移 13）",
		ParamsAttr: []core.ParamSpec{
			{Name: "data", Type: "text", Required: true, Description: "要处理的数据"},
		},
		Runner: func(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
			data, _ := params["data"].(string)
			return &core.ToolResult{Status: core.StatusSuccess, Stdout: rot13(data)}, nil
		},
	}})
}

// Rot13Tool ROT13 工具。
type Rot13Tool struct {
	core.BaseTool
}

// rot13 与 Python codecs.encode(..., "rot_13") 等价：仅字母移位，其余原样。
func rot13(s string) string {
	b := []byte(s)
	for i, c := range b {
		switch {
		case 'a' <= c && c <= 'z':
			b[i] = 'a' + (c-'a'+13)%26
		case 'A' <= c && c <= 'Z':
			b[i] = 'A' + (c-'A'+13)%26
		}
	}
	return string(b)
}
