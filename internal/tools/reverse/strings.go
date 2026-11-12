// reverse.strings —— 提取二进制中的可打印字符串（对应 legacy/tools/reverse/strings.py）。
// Go 版为原生可打印串扫描，行为对齐 strings -n N（仅 ASCII 0x20~0x7E 连续串）。
package reverse

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&StringsTool{BaseTool: core.BaseTool{
		NameAttr:        "strings",
		CategoryAttr:    core.CatReverse,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "提取二进制文件中的可打印字符串（strings）",
		TimeoutAttr:     60,
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "上传文件的 file_id（见 POST /api/files）"},
			{Name: "min_len", Type: "int", Required: false, Default: 4, Description: "最小字符串长度"},
		},
		Runner: stringsRun,
	}})
}

// StringsTool strings 工具。
type StringsTool struct {
	core.BaseTool
}

func stringsRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	minLen := 4
	switch v := params["min_len"].(type) {
	case float64:
		minLen = int(v)
	case int:
		minLen = v
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			minLen = n
		}
	}
	if minLen < 1 {
		minLen = 1
	}

	data, err := os.ReadFile(filepath.Join(tc.Workdir, paramStr(params, "file")))
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}

	var out strings.Builder
	start := -1
	for i, b := range data {
		if b >= 0x20 && b <= 0x7E {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			if i-start >= minLen {
				out.Write(data[start:i])
				out.WriteByte('\n')
			}
			start = -1
		}
	}
	if start >= 0 && len(data)-start >= minLen {
		out.Write(data[start:])
		out.WriteByte('\n')
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: out.String()}, nil
}

func paramStr(params map[string]any, key string) string {
	s, _ := params[key].(string)
	return s
}
