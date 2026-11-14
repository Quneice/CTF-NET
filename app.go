package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"ctfnet/internal/audit"
	"ctfnet/internal/core"
	"ctfnet/internal/projects"
	"ctfnet/internal/wiki"
)

// —— 与旧 REST API 一一对应的哨兵错误（前端按 message 前缀映射旧 404 行为）——

var (
	ErrToolNotFound     = errors.New("工具不存在")
	ErrJobNotFound      = errors.New("任务不存在")
	ErrSessionNotFound  = errors.New("会话不存在或已关闭")
	ErrFileNotFound     = errors.New("文件不存在")
	ErrArtifactNotFound = errors.New("产物不存在")
)

func notFound(base error, target string) error {
	return fmt.Errorf("%w: %s", base, target)
}

// —— 前端契约结构体（JSON 键与旧 REST 响应逐字一致）——

// CategoryInfo GET /api/categories 元素。
type CategoryInfo struct {
	Name      string `json:"name"`
	ToolCount int    `json:"tool_count"`
}

// ToolSummary GET /api/tools 元素。
type ToolSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Timeout     int    `json:"timeout"`
}

// ParamSchema GET /api/tools/{id} 参数声明。
type ParamSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     any    `json:"default"`
	Description string `json:"description"`
}

// ToolDetail GET /api/tools/{id} 响应。
type ToolDetail struct {
	ToolSummary
	Params []ParamSchema `json:"params"`
}

// RunResponse POST /api/tools/{id}/run 响应。
type RunResponse struct {
	JobID string `json:"job_id"`
}

// ExecResponse POST /api/sessions/{sid}/exec 响应。
type ExecResponse struct {
	Output   string `json:"output"`
	TimedOut bool   `json:"timed_out"`
	Closed   bool   `json:"closed"`
}

// FileInfo POST /api/files 响应。
type FileInfo struct {
	FileID string `json:"file_id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
}

// App Wails 绑定面：等价旧 REST API（routes/categories/tools/tasks/files/sessions）。
//
// 注意：绑定方法一律不接收 context.Context 首参——本版本 Wails 运行时不注入 ctx，
// 前端实参会错位进 ctx 槽位导致 "cannot unmarshal ... into context.Context"。
type App struct {
	Reg      *core.Registry
	Tasks    *core.TaskManager
	Sessions *core.SessionRegistry
	Files    *core.FileStore
	Projects *projects.Store // 渗透测试项目管理（projects.go）
	Wiki     *wiki.Store     // 红队 Wiki（wiki.go）
	Cfg      *core.Config    // 统一应用配置（config_api.go）
	Audit    *audit.Store   // 代码审计工作区（audit_api.go）

	ctx context.Context // Wails 应用上下文（OnStartup 注入，流式事件推送用）
}

// —— 分类 / 工具 ——

// GetCategories 对应 GET /api/categories。
func (a *App) GetCategories() ([]CategoryInfo, error) {
	out := make([]CategoryInfo, 0)
	for _, c := range a.Reg.Categories() {
		out = append(out, CategoryInfo{Name: string(c), ToolCount: len(a.Reg.All(&c))})
	}
	return out, nil
}

// GetTools 对应 GET /api/tools[?category]；category 为空串表示全部。
func (a *App) GetTools(category string) ([]ToolSummary, error) {
	var cat *core.Category
	if category != "" {
		c := core.Category(category)
		cat = &c
	}
	out := make([]ToolSummary, 0)
	for _, t := range a.Reg.All(cat) {
		out = append(out, ToolSummary{
			ID:          t.ID(),
			Name:        t.Name(),
			Category:    string(t.Category()),
			Version:     t.Version(),
			Description: t.Description(),
			Timeout:     t.Timeout(),
		})
	}
	return out, nil
}

// GetTool 对应 GET /api/tools/{id}。
func (a *App) GetTool(toolID string) (*ToolDetail, error) {
	t, ok := a.Reg.Get(toolID)
	if !ok {
		return nil, notFound(ErrToolNotFound, toolID)
	}
	params := make([]ParamSchema, 0, len(t.Params()))
	for _, p := range t.Params() {
		params = append(params, ParamSchema{
			Name:        p.Name,
			Type:        p.Type,
			Required:    p.Required,
			Default:     p.Default,
			Description: p.Description,
		})
	}
	return &ToolDetail{
		ToolSummary: ToolSummary{
			ID:          t.ID(),
			Name:        t.Name(),
			Category:    string(t.Category()),
			Version:     t.Version(),
			Description: t.Description(),
			Timeout:     t.Timeout(),
		},
		Params: params,
	}, nil
}

// RunTool 对应 POST /api/tools/{id}/run：校验后提交任务，返回 job_id。
func (a *App) RunTool(toolID string, params map[string]any) (*RunResponse, error) {
	t, ok := a.Reg.Get(toolID)
	if !ok {
		return nil, notFound(ErrToolNotFound, toolID)
	}
	validated, err := t.ValidateParams(params)
	if err != nil {
		return nil, err // 对应 FastAPI 400，前端直接展示 message
	}
	core.ApplyDefaults(toolID, validated) // 空参数按统一配置（config.json）回填默认值
	jobID := a.Tasks.Submit(t, validated)
	return &RunResponse{JobID: jobID}, nil
}

// —— 任务 ——

// GetJob 对应 GET /api/tasks/{jobId}。
func (a *App) GetJob(jobID string) (map[string]any, error) {
	job, ok := a.Tasks.Get(jobID)
	if !ok {
		return nil, notFound(ErrJobNotFound, jobID)
	}
	return job.ToDict(), nil
}

// DownloadArtifact 对应 GET /api/tasks/{jobId}/artifacts/{name}：桌面版改为落盘到 destPath。
func (a *App) DownloadArtifact(jobID, name, destPath string) error {
	job, ok := a.Tasks.Get(jobID)
	if !ok {
		return notFound(ErrJobNotFound, jobID)
	}
	result, ok := job.ResultSnapshot()
	if !ok {
		return notFound(ErrJobNotFound, jobID)
	}
	for _, art := range result.Artifacts {
		if art.Name == name {
			src, err := os.Open(art.Path)
			if err != nil {
				return err
			}
			defer src.Close()
			dst, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer dst.Close()
			_, err = io.Copy(dst, src)
			return err
		}
	}
	return notFound(ErrArtifactNotFound, name)
}

// —— 交互会话 ——

// GetSession 对应 GET /api/sessions/{sid}。
func (a *App) GetSession(sid string) (map[string]any, error) {
	_, alive := a.Sessions.Get(sid)
	return map[string]any{"alive": alive}, nil
}

// ExecSessionCommand 对应 POST /api/sessions/{sid}/exec。
// 参数校验与 Pydantic 一致：command 1..4096，timeout 2..120。
func (a *App) ExecSessionCommand(sid, command string, timeout float64) (*ExecResponse, error) {
	session, ok := a.Sessions.Get(sid)
	if !ok {
		return nil, ErrSessionNotFound
	}
	if len(command) < 1 || len([]rune(command)) > 4096 {
		return nil, fmt.Errorf("command 长度需在 1~4096 之间")
	}
	if timeout < 2 || timeout > 120 {
		return nil, fmt.Errorf("timeout 需在 2~120 秒之间")
	}
	// 绑定方法不能带 ctx（Wails 不注入，见包注释）；会话命令本身自带超时控制
	ctx := context.Background()
	output, timedOut, closed := session.ExecCommand(ctx, command, time.Duration(timeout*float64(time.Second)))
	if closed {
		// 与 sessions.py 一致：先移除再清理（shell 已死，仅回收 fifo 保活进程与工作目录）
		a.Sessions.Remove(sid)
		_ = session.Close(ctx)
	}
	return &ExecResponse{Output: output, TimedOut: timedOut, Closed: closed}, nil
}

// CloseSession 对应 POST /api/sessions/{sid}/close。
func (a *App) CloseSession(sid string) (map[string]any, error) {
	session, ok := a.Sessions.Get(sid)
	if !ok {
		return nil, ErrSessionNotFound
	}
	a.Sessions.Remove(sid)
	if err := session.Close(context.Background()); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// —— 文件 ——

// UploadFile 对应 POST /api/files（multipart → 桌面版传 name + bytes）。
func (a *App) UploadFile(name string, data []byte) (*FileInfo, error) {
	stored, err := a.Files.Save(name, data)
	if err != nil {
		return nil, err
	}
	return &FileInfo{FileID: stored.ID, Name: stored.Name, Size: stored.Size}, nil
}

// UploadFileB64 与 UploadFile 等价，但数据为 base64 文本。
// 桌面绑定通道里 []byte 以 number[] 逐字节 JSON 序列化，大文件时 CPU/内存开销高；
// base64 文本传输体积约为其 1/3，序列化也快得多——前端上传统一走本方法。
func (a *App) UploadFileB64(name string, dataB64 string) (*FileInfo, error) {
	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return nil, fmt.Errorf("base64 解码失败: %w", err)
	}
	return a.UploadFile(name, data)
}

// GetArtifactData 小文件内联读取（预览用；大文件请用 DownloadArtifact）。
func (a *App) GetArtifactData(jobID, name string) ([]byte, error) {
	job, ok := a.Tasks.Get(jobID)
	if !ok {
		return nil, notFound(ErrJobNotFound, jobID)
	}
	result, ok := job.ResultSnapshot()
	if !ok {
		return nil, notFound(ErrJobNotFound, jobID)
	}
	for _, art := range result.Artifacts {
		if art.Name == name {
			return os.ReadFile(art.Path)
		}
	}
	return nil, notFound(ErrArtifactNotFound, name)
}

// —— 健康检查 ——

// Health 对应 GET /health。
func (a *App) Health() (map[string]any, error) {
	return map[string]any{"status": "ok", "tool_count": a.Reg.ToolCount()}, nil
}

// PathExists 便捷检查（前端保存对话框前置校验可用）。
func (a *App) PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
