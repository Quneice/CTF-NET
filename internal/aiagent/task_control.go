package aiagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// —— 运行中任务的人工控制（暂停/恢复/停止/意图级舵向与重跑）——
//
// runningTasks（chat.go）保存运行中的 *Task 对象；本文件补充每个任务的
// 取消函数注册表，让 UI 能对两条运行路径（对话 start_task 后台任务、
// 工具 ai.pentest.task 内联运行）统一做 stop/pause/resume。

var runningTaskCancels sync.Map // taskID -> context.CancelFunc

// RegisterRunningTask 注册运行中任务及其取消函数（编排器启动前调用）。
// cancel 触发后编排器会以 FinishStopped 收尾；任务终态后由编排侧调用
// UnregisterRunningTask 清理。
func RegisterRunningTask(task *Task, cancel context.CancelFunc) {
	runningTasks.Store(task.ID, task)
	runningTaskCancels.Store(task.ID, cancel)
}

// UnregisterRunningTask 清理注册（任务终态或启动失败时调用）。
func UnregisterRunningTask(taskID string) {
	runningTasks.Delete(taskID)
	runningTaskCancels.Delete(taskID)
}

// ControlRunningTask 对运行中任务做人工控制。
// action: stop（取消编排 ctx → 任务以 stopped 收尾）/ pause / resume。
func ControlRunningTask(taskID, action string) error {
	switch action {
	case "pause":
		if v, ok := runningTasks.Load(taskID); ok {
			v.(*Task).Pause()
			return nil
		}
	case "resume":
		if v, ok := runningTasks.Load(taskID); ok {
			v.(*Task).Resume()
			return nil
		}
	case "stop":
		if c, ok := runningTaskCancels.Load(taskID); ok {
			c.(context.CancelFunc)()
			return nil
		}
	default:
		return fmt.Errorf("未知控制动作: %s", action)
	}
	return fmt.Errorf("任务 %s 不在运行中", taskID)
}

// ControlRunningIntent 对运行中任务的单条意图做人工控制。
// action: rerun（终态 → open 复活）/ steer（注入舵向，msg 必填）。
func ControlRunningIntent(taskID string, intentID int, action, msg string) error {
	v, ok := runningTasks.Load(taskID)
	if !ok {
		return fmt.Errorf("任务 %s 不在运行中", taskID)
	}
	t := v.(*Task)
	switch action {
	case "rerun":
		return t.RerunIntent(intentID)
	case "steer":
		if msg == "" {
			return fmt.Errorf("缺少舵向内容 msg")
		}
		return t.SteerIntent(intentID, msg)
	default:
		return fmt.Errorf("未知意图控制动作: %s", action)
	}
}

// LoadOrLiveTask 读取任务快照：运行中优先内存态，否则从 dir/task.json 恢复。
// taskRoot 为空时只查运行中实例。
func LoadOrLiveTask(taskRoot, taskID string) (*Task, error) {
	if v, ok := runningTasks.Load(taskID); ok {
		return v.(*Task).Snapshot(), nil
	}
	if taskRoot != "" {
		if t, err := LoadTask(filepath.Join(taskRoot, taskID)); err == nil {
			return t, nil
		}
	}
	return nil, fmt.Errorf("任务 %s 不存在（不在运行中，且 %s 下无落盘）", taskID, filepath.Join(taskRoot, taskID))
}

// DefaultTaskRoot AI 任务落盘根目录（与 chat.go start_task / tools/ai 一致：
// <DataDir>/aiagent/tasks）。
func DefaultTaskRoot(dataDir string) string { return filepath.Join(dataDir, "aiagent", "tasks") }

// MirrorTaskJSON 把任务黑板 task.json 从 srcDir 镜像到 dstDir（工具内联运行的
// 任务工作目录不在统一任务根下，跑完后镜像一份供仪表盘扫描/复盘）。
func MirrorTaskJSON(srcDir, dstDir string) error {
	data, err := os.ReadFile(filepath.Join(srcDir, "task.json"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dstDir, "task.json.tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dstDir, "task.json"))
}
