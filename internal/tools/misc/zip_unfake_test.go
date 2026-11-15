package misc

// misc.zip-unfake 测试:正常 zip 手动置位加密标志(模拟伪加密),修复后 archive/zip 可直接打开。

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ctfnet/internal/core"
)

// synthZip 用 archive/zip 造正常 zip。
func synthZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("secret.txt")
	f.Write([]byte("flag{fake_zip_enc}"))
	w.Close()
	return buf.Bytes()
}

// fakeEncrypt 把所有 local(PK0304 off6)与 central(PK0102 off8)的 flag bit0 置 1。
func fakeEncrypt(data []byte) []byte {
	out := append([]byte{}, data...)
	for i := 0; i+4 <= len(out); i++ {
		if bytes.Equal(out[i:i+4], []byte("PK\x03\x04")) && i+8 <= len(out) {
			out[i+6] |= 1
		}
		if bytes.Equal(out[i:i+4], []byte("PK\x01\x02")) && i+10 <= len(out) {
			out[i+8] |= 1
		}
	}
	return out
}

func runZipUnfake(t *testing.T, data []byte) (map[string]any, string) {
	t.Helper()
	tool, ok := core.DefaultRegistry.Get("misc.zip-unfake")
	if !ok {
		t.Fatal("misc.zip-unfake 未注册")
	}
	work := t.TempDir()
	arts := t.TempDir()
	os.WriteFile(filepath.Join(work, "fake.zip"), data, 0o644)
	res, err := tool.Run(context.Background(), map[string]any{"file": "fake.zip"}, &core.ToolContext{Workdir: work, ArtifactsDir: arts})
	if err != nil {
		t.Fatalf("运行出错: %v", err)
	}
	if res.Status != core.StatusSuccess {
		t.Fatalf("status %s: %s", res.Status, res.Stderr)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &m); err != nil {
		t.Fatalf("stdout 非合法 JSON")
	}
	return m, arts
}

// 1. 伪加密 → 修复,产物可被 archive/zip 直接打开且内容正确。
func TestZipUnfakeFix(t *testing.T) {
	m, arts := runZipUnfake(t, fakeEncrypt(synthZip(t)))
	if m["fixed"] != true {
		t.Fatalf("应判定为伪加密并修复: %v", m)
	}
	zr, err := zip.OpenReader(filepath.Join(arts, "unfaked.zip"))
	if err != nil {
		t.Fatalf("修复后 zip 打不开: %v", err)
	}
	defer zr.Close()
	if len(zr.File) != 1 || zr.File[0].Name != "secret.txt" {
		t.Fatalf("条目不符")
	}
	rc, _ := zr.File[0].Open()
	buf := new(bytes.Buffer)
	buf.ReadFrom(rc)
	if buf.String() != "flag{fake_zip_enc}" {
		t.Fatalf("内容不符: %q", buf.String())
	}
}

// 2. 正常 zip(未置位)→ 报告无需修复。
func TestZipUnfakeIntact(t *testing.T) {
	m, _ := runZipUnfake(t, synthZip(t))
	if m["fixed"] != false {
		t.Fatalf("正常 zip 应报无需修复: %v", m)
	}
}

// 3. 非 zip → 友好错误。
func TestZipUnfakeBad(t *testing.T) {
	tool, _ := core.DefaultRegistry.Get("misc.zip-unfake")
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "x.zip"), []byte("not a zip"), 0o644)
	res, _ := tool.Run(context.Background(), map[string]any{"file": "x.zip"}, &core.ToolContext{Workdir: work, ArtifactsDir: t.TempDir()})
	if res.Status != core.StatusError {
		t.Fatalf("非 zip 应报错")
	}
}
