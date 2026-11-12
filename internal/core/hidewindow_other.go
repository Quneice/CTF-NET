//go:build !windows

package core

import "os/exec"

// HideSubprocessWindow 非 Windows 平台无控制台弹窗问题，空实现。
func HideSubprocessWindow(c *exec.Cmd) {}
