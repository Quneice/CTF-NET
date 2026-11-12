// 执行引擎：统一 Executor 接口，支持本机子进程与 Docker 沙箱两种后端。
// 对应 legacy/app/core/executor.py。
package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Executor 执行器抽象接口。
type Executor interface {
	Run(ctx context.Context, cmd []string, cwd string, timeout time.Duration, env map[string]string) (*ToolResult, error)
}

// sanitizeUTF8 对应 Python decode(errors="replace")：非法 UTF-8 替换为 U+FFFD，
// 否则 json.Marshal 会整体失败。
func sanitizeUTF8(s string) string {
	return strings.ToValidUTF8(s, "�")
}

// SubprocessExecutor 本机子进程执行：无隔离，最快，适合可信工具。
type SubprocessExecutor struct{}

// Run 执行子进程并施加超时；语义与 Python asyncio 版逐条对应。
func (s *SubprocessExecutor) Run(ctx context.Context, cmd []string, cwd string, timeout time.Duration, env map[string]string) (*ToolResult, error) {
	start := time.Now()
	execCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	c := exec.CommandContext(execCtx, cmd[0], cmd[1:]...)
	HideSubprocessWindow(c)
	if cwd != "" {
		c.Dir = cwd
	}
	if env != nil {
		// Python create_subprocess_exec(env=dict) 语义：整体替换环境。
		c.Env = make([]string, 0, len(env))
		for k, v := range env {
			c.Env = append(c.Env, k+"="+v)
		}
	}
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	duration := time.Since(start).Seconds()

	if execCtx.Err() == context.DeadlineExceeded {
		return &ToolResult{
			Status:   StatusTimeout,
			ExitCode: -1,
			Stderr:   fmt.Sprintf("执行超时（>%ds）: %s", int(timeout.Seconds()), strings.Join(cmd, " ")),
			Duration: duration,
		}, nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return &ToolResult{
			Status:   StatusError,
			ExitCode: 127,
			Stderr:   fmt.Sprintf("命令不存在: %v", err),
			Duration: duration,
		}, nil
	}

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 127
		}
	}
	status := StatusSuccess
	if exitCode != 0 {
		status = StatusError
	}
	return &ToolResult{
		Status:   status,
		ExitCode: exitCode,
		Stdout:   sanitizeUTF8(stdout.String()),
		Stderr:   sanitizeUTF8(stderr.String()),
		Duration: duration,
	}, nil
}

// DockerExecutor Docker 沙箱执行：隔离文件系统与网络。
// 两种挂载模式（与 Python 版一致）：
//   - bind 模式（默认）：cwd 挂载到容器 /work；
//   - volume 模式：DataVolume + DataRoot 同时设置后改用命名卷挂载（DooD 场景）。
type DockerExecutor struct {
	Image       string
	Network     string
	ExtraMounts []string
	DataVolume  string
	DataRoot    string
}

// Run 组装 docker run 命令后交给 SubprocessExecutor。
func (d *DockerExecutor) Run(ctx context.Context, cmd []string, cwd string, timeout time.Duration, env map[string]string) (*ToolResult, error) {
	var dockerCmd []string
	if d.DataVolume != "" && d.DataRoot != "" {
		rel := ""
		if cwd != "" {
			if absCwd, err := filepath.Abs(cwd); err == nil {
				if absRoot, err2 := filepath.Abs(d.DataRoot); err2 == nil {
					if r, err3 := filepath.Rel(absRoot, absCwd); err3 == nil && r != "." {
						rel = filepath.ToSlash(r)
					}
				}
			}
		}
		containerCwd := "/data"
		if rel != "" {
			containerCwd = "/data/" + rel
		}
		dockerCmd = []string{
			"docker", "run", "--rm",
			"--network", d.Network,
			"-v", d.DataVolume + ":/data",
			"-w", containerCwd,
		}
	} else {
		hostSrc := cwd
		if hostSrc == "" {
			hostSrc, _ = os.Getwd()
		}
		if abs, err := filepath.Abs(hostSrc); err == nil {
			hostSrc = abs
		}
		hostSrc = filepath.ToSlash(hostSrc)
		dockerCmd = []string{
			"docker", "run", "--rm",
			"--network", d.Network,
			"-v", hostSrc + ":/work",
			"-w", "/work",
		}
	}
	for _, mount := range d.ExtraMounts {
		dockerCmd = append(dockerCmd, "-v", mount)
	}
	for key, value := range env {
		dockerCmd = append(dockerCmd, "-e", key+"="+value)
	}
	dockerCmd = append(dockerCmd, d.Image)
	dockerCmd = append(dockerCmd, cmd...)

	sub := &SubprocessExecutor{}
	return sub.Run(ctx, dockerCmd, "", timeout, nil)
}
