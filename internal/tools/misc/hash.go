// misc.hash —— 常见哈希（md5/sha1/sha256）（对应 legacy/tools/misc/hash.py）。
package misc

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&HashTool{BaseTool: core.BaseTool{
		NameAttr:        "hash",
		CategoryAttr:    core.CatMisc,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "计算常见哈希（md5/sha1/sha256）",
		ParamsAttr: []core.ParamSpec{
			{Name: "algo", Type: "str", Required: true, Description: "md5 / sha1 / sha256"},
			{Name: "data", Type: "text", Required: true, Description: "要哈希的数据"},
		},
		Runner: hashRun,
	}})
}

// HashTool 哈希工具。
type HashTool struct {
	core.BaseTool
}

func hashRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	algo := strings.ToLower(paramStr(params, "algo"))
	data := paramStr(params, "data")
	var sum []byte
	switch algo {
	case "md5":
		h := md5.Sum([]byte(data))
		sum = h[:]
	case "sha1":
		h := sha1.Sum([]byte(data))
		sum = h[:]
	case "sha256":
		h := sha256.Sum256([]byte(data))
		sum = h[:]
	default:
		return &core.ToolResult{Status: core.StatusError, Stderr: "不支持的算法: " + algo}, nil
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: hex.EncodeToString(sum)}, nil
}

func paramStr(params map[string]any, key string) string {
	s, _ := params[key].(string)
	return s
}
