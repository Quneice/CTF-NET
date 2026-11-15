package misc

// misc.png-fix 测试:合成真 CRC 的 PNG,改坏宽高,修复应还原正确尺寸并输出修复产物。

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"

	"ctfnet/internal/core"
)

// pngChunkRealCRC 写一个带正确 CRC 的 chunk。
func pngChunkRealCRC(b *bytes.Buffer, typ string, data []byte) {
	var l [4]byte
	binary.BigEndian.PutUint32(l[:], uint32(len(data)))
	b.Write(l[:])
	b.WriteString(typ)
	b.Write(data)
	var c [4]byte
	binary.BigEndian.PutUint32(c[:], crc32.ChecksumIEEE(append([]byte(typ), data...)))
	b.Write(c[:])
}

// synthPNGRealCRC 合成真 CRC 的最小 PNG。
func synthPNGRealCRC(w, h uint32) []byte {
	var b bytes.Buffer
	b.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8] = 8
	ihdr[9] = 2
	pngChunkRealCRC(&b, "IHDR", ihdr)
	pngChunkRealCRC(&b, "IDAT", bytes.Repeat([]byte{1, 2, 3}, 40))
	pngChunkRealCRC(&b, "IEND", nil)
	return b.Bytes()
}

// corruptIHDRWidth 改坏宽字段(不动 CRC)。
func corruptIHDRWidth(data []byte, w uint32) []byte {
	out := append([]byte{}, data...)
	binary.BigEndian.PutUint32(out[16:], w)
	return out
}

func corruptIHDRHeight(data []byte, h uint32) []byte {
	out := append([]byte{}, data...)
	binary.BigEndian.PutUint32(out[20:], h)
	return out
}

func runPNGFix(t *testing.T, data []byte) (map[string]any, string) {
	t.Helper()
	tool, ok := core.DefaultRegistry.Get("misc.png-fix")
	if !ok {
		t.Fatal("misc.png-fix 未注册")
	}
	work := t.TempDir()
	arts := t.TempDir()
	os.WriteFile(filepath.Join(work, "bad.png"), data, 0o644)
	res, err := tool.Run(context.Background(), map[string]any{"file": "bad.png"}, &core.ToolContext{Workdir: work, ArtifactsDir: arts})
	if err != nil {
		t.Fatalf("运行出错: %v", err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("status %s: %s", res.Status, res.Stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &m); err != nil {
		t.Fatalf("stdout 非合法 JSON: %v", res.Stdout[:200])
	}
	return m, arts
}

// 1. 宽被改坏 → 修复还原宽。
func TestPNGFixWidth(t *testing.T) {
	orig := synthPNGRealCRC(640, 480)
	m, arts := runPNGFix(t, corruptIHDRWidth(orig, 100))
	if m["width"] != float64(640) || m["height"] != float64(480) {
		t.Fatalf("修复尺寸 = %v x %v", m["width"], m["height"])
	}
	// 产物应为原始正确 PNG
	got, err := os.ReadFile(filepath.Join(arts, "fixed.png"))
	if err != nil || !bytes.Equal(got, orig) {
		t.Fatalf("fixed.png 应等于原始正确 PNG")
	}
}

// 2. 高被改坏 → 修复还原高。
func TestPNGFixHeight(t *testing.T) {
	orig := synthPNGRealCRC(1920, 1080)
	m, _ := runPNGFix(t, corruptIHDRHeight(orig, 7))
	if m["height"] != float64(1080) {
		t.Fatalf("修复高 = %v", m["height"])
	}
}

// 3. 宽高都被改坏 → 双爆破。
func TestPNGFixBoth(t *testing.T) {
	orig := synthPNGRealCRC(800, 600)
	bad := corruptIHDRWidth(orig, 99)
	bad = corruptIHDRHeight(bad, 1)
	m, _ := runPNGFix(t, bad)
	if m["width"] != float64(800) || m["height"] != float64(600) {
		t.Fatalf("双修复 = %v x %v", m["width"], m["height"])
	}
}

// 4. 完好 PNG → 报告无需修复。
func TestPNGFixIntact(t *testing.T) {
	orig := synthPNGRealCRC(64, 64)
	m, _ := runPNGFix(t, orig)
	if m["fixed"] != false {
		t.Fatalf("完好 PNG 应报无需修复: %v", m)
	}
}

// 5. 非 PNG → 友好错误。
func TestPNGFixBadFile(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("misc.png-fix")
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "x.png"), []byte("not png"), 0o644)
	res, err := tool.Run(context.Background(), map[string]any{"file": "x.png"}, &core.ToolContext{Workdir: work, ArtifactsDir: t.TempDir()})
	if err != nil || res.Status != core.StatusError {
		t.Fatalf("非 PNG 应报错: %v %s", err, res.Status)
	}
}
