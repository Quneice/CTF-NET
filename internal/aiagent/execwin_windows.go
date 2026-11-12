//go:build windows

package aiagent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

// RunWindowsCmd 在 cmd /S /C 下执行 shell 命令（Windows 专用，所有入口共用：
// tools/ai 的 task/chat 工具与 chat_api.go 的对话页都经此执行，避免各写各的
// 引号处理再踩同一个坑）。
//
// 关键（2026-11-12 实测踩坑）：不能走 exec.Command("cmd", "/C", command)——
// Go 会为含空格的 argv 自动加外层引号，cmd 的引号剥离规则再把命令里最后一个
// 引号吃掉，导致 python -c "..."、findstr /C:"..." 等引号命令被破坏
// （python 收到缺括号的语句 → File "<string>", line 1）。这里用 SysProcAttr.CmdLine
// 原样传命令（外层手工包一对引号），/S 下 cmd 只剥离这对外层引号，内部引号原样
// 保留；命令走 UTF-16 命令行，中文不受批处理 GBK 编码影响。
//
// 已知限制：超时只杀 cmd 本身，不递归杀子进程树（Windows 无进程组概念，
// 与 core.Executor 旧实现同病）；模型提示词（winExecNote）已引导少用长驻命令。
func RunWindowsCmd(ctx context.Context, command, dir string, timeoutSec int) (string, string, error) {
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	tctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	c := exec.CommandContext(tctx, "cmd")
	c.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd /S /C "` + command + `"`}
	if dir != "" {
		c.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	out, errOut := stdout.String(), stderr.String()
	if errors.Is(tctx.Err(), context.DeadlineExceeded) {
		return out, errOut, fmt.Errorf("命令超时（>%ds）", timeoutSec)
	}
	if err != nil {
		return out, errOut, fmt.Errorf("exit %d: %s", exitCodeOf(err), errOut)
	}
	return out, errOut, nil
}

// exitCodeOf 提取子进程退出码（非 ExitError 返回 -1）。
func exitCodeOf(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
