package ai

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"ctfnet/internal/aiagent"
)

// —— aiagent.RunWindowsCmd 回归：cmd /S /C + 原生 CmdLine 的引号完整性 ——
// 背景：旧实现 cmd /C <argv> 被 Go 引号拼接 + cmd 引号剥离双层破坏，
// python -c "..." / findstr /C:"..." 全部失败（2026-11-12 实测）。

func TestRunWindowsCmdQuotes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 验证")
	}
	// 双引号必须原样到达 echo（旧实现会吃掉命令尾引号）
	out, errOut, err := aiagent.RunWindowsCmd(context.Background(), `echo "hello world"`, t.TempDir(), 10)
	if err != nil {
		t.Fatalf("echo 引号命令失败: %v / stderr=%s", err, errOut)
	}
	if !strings.Contains(out, `"hello world"`) {
		t.Fatalf("引号被破坏: %q", out)
	}
	// && 链与括号
	out, errOut, err = aiagent.RunWindowsCmd(context.Background(), `echo a && echo (b)`, t.TempDir(), 10)
	if err != nil || !strings.Contains(out, "a") || !strings.Contains(out, "(b)") {
		t.Fatalf("链式命令失败: %v / %q / %q", err, out, errOut)
	}
}

func TestRunWindowsCmdFindstr(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 验证")
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(f, []byte("needle line\nother line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// 命中：无错误
	out, errOut, err := aiagent.RunWindowsCmd(context.Background(), `findstr /I /C:"needle" f.txt`, dir, 10)
	if err != nil || !strings.Contains(out, "needle") {
		t.Fatalf("findstr 命中失败: %v / %q / %q", err, out, errOut)
	}
	// 未命中：exit 1 属正常（模型提示词已说明），不误报为致命错误
	_, _, err = aiagent.RunWindowsCmd(context.Background(), `findstr /I /C:"absent" f.txt`, dir, 10)
	if err == nil || !strings.Contains(err.Error(), "exit 1") {
		t.Fatalf("findstr 未命中应返回 exit 1: %v", err)
	}
}

func TestRunWindowsCmdTimeout(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 验证")
	}
	_, _, err := aiagent.RunWindowsCmd(context.Background(), `ping -n 10 127.0.0.1 >nul`, t.TempDir(), 2)
	if err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("应超时: %v", err)
	}
}
