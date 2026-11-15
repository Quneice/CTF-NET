package misc

// misc.zip-unfake:ZIP 伪加密修复(CTF 压缩包题最高频套路)。
// 伪加密 = local file header(PK0304 off6)与 central directory(PK0102 off8)的
// general purpose bit flag bit0 被置 1,但数据并未加密。修复 = 两处 bit0 清零,
// 输出修复后 zip。判定:存在任一置位且清零后 archive/zip 可正常读取 → 伪加密;
// 全程无置位 → 无需修复;置位但清零后仍读失败 → 真加密(提示走 ZIP 密码爆破)。

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&ZipUnfakeTool{BaseTool: core.BaseTool{
		NameAttr:        "zip-unfake",
		CategoryAttr:    core.CatMisc,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "ZIP 伪加密修复:local/central 目录的加密标志位被置 1 但数据未加密,清零两处 bit0 还原可直接打开的 zip",
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "提示加密但疑似伪加密的 zip"},
		},
		Runner: zipUnfakeRun,
	}})
}

// ZipUnfakeTool ZIP 伪加密修复工具。
type ZipUnfakeTool struct {
	core.BaseTool
}

func zipUnfakeRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	data, err := os.ReadFile(filepath.Join(tc.Workdir, paramStr(params, "file")))
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("读取文件失败: %v", err)}, nil
	}
	if len(data) < 4 || !bytes.Equal(data[:2], []byte("PK")) {
		return &core.ToolResult{Status: core.StatusError, Stderr: "不是 ZIP 文件(无 PK 签名)"}, nil
	}
	fixed := append([]byte{}, data...)
	localSet, centralSet := 0, 0
	for i := 0; i+4 <= len(fixed); i++ {
		if bytes.Equal(fixed[i:i+4], []byte("PK\x03\x04")) && i+8 <= len(fixed) {
			if fixed[i+6]&1 == 1 {
				fixed[i+6] &^= 1
				localSet++
			}
		}
		if bytes.Equal(fixed[i:i+4], []byte("PK\x01\x02")) && i+10 <= len(fixed) {
			if fixed[i+8]&1 == 1 {
				fixed[i+8] &^= 1
				centralSet++
			}
		}
	}
	if localSet == 0 && centralSet == 0 {
		// 标志未置位:若本身能打开则无需修复,打不开则是真加密或其他损坏
		if zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil {
			_ = zr
			rep := map[string]any{"fixed": false, "entries": len(zr.File),
				"summary": "加密标志未置位且 zip 可正常打开,无需修复"}
			out, _ := json.Marshal(rep)
			return &core.ToolResult{Status: core.StatusSuccess, Stdout: string(out)}, nil
		}
		return &core.ToolResult{Status: core.StatusError, Stderr: "加密标志未置位且无法解析:可能是真加密(走 ZIP 密码爆破)或文件损坏"}, nil
	}
	// 有置位:验证清零后可读 → 伪加密
	zr, err := zip.NewReader(bytes.NewReader(fixed), int64(len(fixed)))
	if err != nil {
		return &core.ToolResult{Status: core.StatusError,
			Stderr: "标志已清零仍无法解析:疑似真加密(请用「ZIP 密码爆破」)或文件损坏"}, nil
	}
	art := ""
	if tc.ArtifactsDir != "" {
		if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err == nil {
			if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "unfaked.zip"), fixed, 0o644); err == nil {
				art = "unfaked.zip"
			}
		}
	}
	rep := map[string]any{"fixed": true, "local_cleared": localSet, "central_cleared": centralSet,
		"entries": len(zr.File), "artifact": art,
		"summary": fmt.Sprintf("伪加密修复:local %d 处 + central %d 处标志清零,%d 个条目可读,产物 %s", localSet, centralSet, len(zr.File), art)}
	out, err := json.Marshal(rep)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: string(out)}, nil
}
