package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildArgs 参数构造:专用用户目录、代理指向、起始页置尾、空 URL 用默认页。
func TestBuildArgs(t *testing.T) {
	args := BuildArgs(`C:\prof`, "127.0.0.1:8080", "http://target.test/")
	if len(args) != 6 {
		t.Fatalf("args = %v", args)
	}
	if args[0] != `--user-data-dir=C:\prof` {
		t.Fatalf("arg0 = %q", args[0])
	}
	if args[1] != "--proxy-server=127.0.0.1:8080" {
		t.Fatalf("arg1 = %q", args[1])
	}
	if args[2] != "--ignore-certificate-errors" {
		t.Fatalf("arg2 = %q", args[2])
	}
	if args[5] != "http://target.test/" {
		t.Fatalf("url = %q", args[5])
	}

	def := BuildArgs(`C:\prof`, "127.0.0.1:8080", "  ")
	if def[5] != DefaultStartURL {
		t.Fatalf("default url = %q", def[5])
	}
}

// TestDetectFindsCandidate 命中候选路径(伪造目录 + 假 exe 文件)。
func TestDetectFindsCandidate(t *testing.T) {
	dir := t.TempDir()
	edge := filepath.Join(dir, "msedge.exe")
	if err := os.WriteFile(edge, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cands := []candidate{
		{name: "Edge", paths: []string{edge}},
		{name: "Chrome", paths: []string{filepath.Join(dir, "chrome.exe")}},
	}
	info, err := detectWith(cands, func(string) (string, error) { return "", os.ErrNotExist })
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if info.Name != "Edge" || info.Path != edge {
		t.Fatalf("info = %+v", info)
	}
}

// TestDetectEmpty 无候选且 PATH 无兜底时报错。
func TestDetectEmpty(t *testing.T) {
	_, err := detectWith(nil, func(string) (string, error) { return "", os.ErrNotExist })
	if err == nil || !strings.Contains(err.Error(), "Edge") {
		t.Fatalf("err = %v", err)
	}
}
