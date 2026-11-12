// misc.file_checksum —— 上传文件哈希（对应 legacy/tools/misc/file_checksum.py）。
// file 类型参数由 TaskManager 解析为工作目录内的相对文件名。
package misc

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&FileChecksumTool{BaseTool: core.BaseTool{
		NameAttr:        "file_checksum",
		CategoryAttr:    core.CatMisc,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "计算上传文件的哈希（md5 / sha1 / sha256）",
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "上传文件的 file_id"},
			{Name: "algo", Type: "str", Required: false, Default: "sha256", Description: "md5 / sha1 / sha256"},
		},
		Runner: fileChecksumRun,
	}})
}

// FileChecksumTool 文件哈希工具。
type FileChecksumTool struct {
	core.BaseTool
}

func fileChecksumRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	algo := strings.ToLower(paramStr(params, "algo"))
	if algo == "" {
		algo = "sha256"
	}
	data, err := os.ReadFile(filepath.Join(tc.Workdir, paramStr(params, "file")))
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	var sum []byte
	switch algo {
	case "md5":
		h := md5.Sum(data)
		sum = h[:]
	case "sha1":
		h := sha1.Sum(data)
		sum = h[:]
	case "sha256":
		h := sha256.Sum256(data)
		sum = h[:]
	default:
		return &core.ToolResult{Status: core.StatusError, Stderr: "不支持的算法: " + algo}, nil
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: hex.EncodeToString(sum)}, nil
}
