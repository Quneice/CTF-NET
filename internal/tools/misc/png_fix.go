package misc

// misc.png-fix:PNG 宽高修复(CTF 图片题最高频套路之一)。
// 原理:题目篡改 IHDR 宽/高后 CRC 不再匹配;爆破宽×高直到重算 CRC == 文件存储 CRC,
// 即为原始尺寸;输出修复后的 PNG 产物。策略:先固定宽爆破高、再固定高爆破宽(覆盖单维篡改,
// 绝大多数题目),再双维爆破(限 ≤4096,覆盖双改)。

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&PNGFixTool{BaseTool: core.BaseTool{
		NameAttr:        "png-fix",
		CategoryAttr:    core.CatMisc,
		VersionAttr:     "1.0.0",
		DescriptionAttr: "PNG 宽高修复:IHDR 宽/高被篡改后 CRC 失配,爆破宽×高匹配原始 CRC 还原正确尺寸,输出修复后 PNG",
		ParamsAttr: []core.ParamSpec{
			{Name: "file", Type: "file", Required: true, Description: "宽高异常的 PNG 文件"},
		},
		Runner: pngFixRun,
	}})
}

// PNGFixTool PNG 宽高修复工具。
type PNGFixTool struct {
	core.BaseTool
}

const pngMaxDim = 16384 // PNG 规范最大尺寸

// ihdrCRCMatch 校验 (w,h) 组合的 IHDR CRC 是否等于存储值。
func ihdrCRCMatch(ihdrData []byte, w, h uint32, stored uint32) bool {
	tmp := append([]byte{}, ihdrData...)
	binary.BigEndian.PutUint32(tmp[0:], w)
	binary.BigEndian.PutUint32(tmp[4:], h)
	return crc32.ChecksumIEEE(append([]byte("IHDR"), tmp...)) == stored
}

func pngFixRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	data, err := os.ReadFile(filepath.Join(tc.Workdir, paramStr(params, "file")))
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("读取文件失败: %v", err)}, nil
	}
	if len(data) < 41 || string(data[1:4]) != "PNG" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "不是 PNG 文件(签名不符)"}, nil
	}
	// IHDR:8..16 长度,12..16 类型,16..29 数据(宽4 高4 深1 色1 压1 滤1 隔1),29..33 CRC
	ihdr := append([]byte{}, data[16:29]...)
	stored := binary.BigEndian.Uint32(data[29:33])
	curW := binary.BigEndian.Uint32(ihdr[0:4])
	curH := binary.BigEndian.Uint32(ihdr[4:8])

	if ihdrCRCMatch(ihdr, curW, curH, stored) {
		rep := map[string]any{"fixed": false, "width": curW, "height": curH,
			"summary": fmt.Sprintf("IHDR CRC 校验通过(%d x %d),宽高未被篡改,无需修复", curW, curH)}
		out, _ := json.Marshal(rep)
		return &core.ToolResult{Status: core.StatusSuccess, Stdout: string(out)}, nil
	}

	// 阶段1:固定宽爆破高;阶段2:固定高爆破宽;阶段3:双爆破(≤4096)
	var fw, fh uint32
	found := false
	for h := uint32(1); h <= pngMaxDim; h++ {
		if ihdrCRCMatch(ihdr, curW, h, stored) {
			fw, fh, found = curW, h, true
			break
		}
	}
	if !found {
		for w := uint32(1); w <= pngMaxDim; w++ {
			if ihdrCRCMatch(ihdr, w, curH, stored) {
				fw, fh, found = w, curH, true
				break
			}
		}
	}
	if !found {
		const bothMax = 4096
		for w := uint32(1); w <= bothMax && !found; w++ {
			for h := uint32(1); h <= bothMax; h++ {
				if ihdrCRCMatch(ihdr, w, h, stored) {
					fw, fh, found = w, h, true
					break
				}
			}
		}
	}
	if !found {
		return &core.ToolResult{Status: core.StatusError,
			Stderr: fmt.Sprintf("爆破未命中:IHDR CRC 与任何宽高组合不匹配(当前 %d x %d)。可能 CRC 也被改或非宽高篡改", curW, curH)}, nil
	}

	// 修复:重写宽高(CRC 不变,因为现在匹配了)
	fixed := append([]byte{}, data...)
	binary.BigEndian.PutUint32(fixed[16:], fw)
	binary.BigEndian.PutUint32(fixed[20:], fh)
	art := ""
	if tc.ArtifactsDir != "" {
		if err := os.MkdirAll(tc.ArtifactsDir, 0o755); err == nil {
			if err := os.WriteFile(filepath.Join(tc.ArtifactsDir, "fixed.png"), fixed, 0o644); err == nil {
				art = "fixed.png"
			}
		}
	}
	rep := map[string]any{"fixed": true, "width": fw, "height": fh, "was_width": curW, "was_height": curH,
		"artifact": art,
		"summary":  fmt.Sprintf("宽高修复:%d x %d → %d x %d(IHDR CRC 匹配),产物 %s", curW, curH, fw, fh, art)}
	out, err := json.Marshal(rep)
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: err.Error()}, nil
	}
	return &core.ToolResult{Status: core.StatusSuccess, Stdout: string(out),
		Data: map[string]any{"width": fw, "height": fh}}, nil
}
