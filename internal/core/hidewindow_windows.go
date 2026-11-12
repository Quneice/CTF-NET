//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

// HideSubprocessWindow 阻止子进程弹出控制台窗口。
// GUI 应用（无控制台）中，每 spawn 一个子进程 Windows 都会新建一个黑色
// 控制台窗口（如批量 OCR 时每张图弹一次）。所有工具的输出都经管道捕获，
// 不需要窗口，统一在创建 *exec.Cmd 后调用本函数。
func HideSubprocessWindow(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
