//go:build !windows

package aiagent

import (
	"context"
	"fmt"
)

// RunWindowsCmd 非 Windows 平台占位（调用方已按 runtime.GOOS 分流，不会走到这里）。
func RunWindowsCmd(ctx context.Context, command, dir string, timeoutSec int) (string, string, error) {
	return "", "", fmt.Errorf("RunWindowsCmd 仅支持 Windows")
}
