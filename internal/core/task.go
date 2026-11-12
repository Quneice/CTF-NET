// 任务管理：goroutine 任务队列 + 状态追踪。对应 legacy/app/core/task.py。
package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Job 单次任务。JSON 结构与 Python Job.to_dict() 完全一致（error 空时为 null）。
type Job struct {
	ID         string
	ToolID     string
	Params     map[string]any
	Status     Status
	Result     *ToolResult
	Error      string
	CreatedAt  float64
	StartedAt  *float64
	FinishedAt *float64

	// 运行期进度（工具通过 ToolContext.ReportProgress 上报，前端轮询展示；
	// 未上报的工具为零值，前端回退显示"任务执行中"）。
	ProgressPercent int
	ProgressMessage string

	mu sync.RWMutex
}

// ToDict 对应 Python Job.to_dict()。
func (j *Job) ToDict() map[string]any {
	j.mu.RLock()
	defer j.mu.RUnlock()
	var result any
	if j.Result != nil {
		result = j.Result.ToDict()
	}
	var errVal any
	if j.Error != "" {
		errVal = j.Error
	}
	var startedAt, finishedAt any
	if j.StartedAt != nil {
		startedAt = *j.StartedAt
	}
	if j.FinishedAt != nil {
		finishedAt = *j.FinishedAt
	}
	return map[string]any{
		"id":               j.ID,
		"tool_id":          j.ToolID,
		"params":           j.Params,
		"status":           string(j.Status),
		"result":           result,
		"error":            errVal,
		"created_at":       j.CreatedAt,
		"started_at":       startedAt,
		"finished_at":      finishedAt,
		"progress_percent": j.ProgressPercent,
		"progress_message": j.ProgressMessage,
	}
}

// TaskManager 内存任务队列（MVP），对应 Python TaskManager。
type TaskManager struct {
	Executor     Executor
	Workdir      string
	ArtifactsDir string
	FileStore    *FileStore
	Sessions     *SessionRegistry

	// OnJobDone 任务到达终态（success/error/timeout）后的回调，由桌面壳层注入
	// 用于事件推送（core 不依赖 wails）：前端由此即时收到结果，替代纯 REST 轮询。
	OnJobDone func(*Job)

	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewTaskManager 创建任务管理器并确保目录存在（对应 Python __init__）。
func NewTaskManager(executor Executor, workdir, artifactsDir string, fileStore *FileStore, sessions *SessionRegistry) *TaskManager {
	_ = os.MkdirAll(workdir, 0o755)
	_ = os.MkdirAll(artifactsDir, 0o755)
	return &TaskManager{
		Executor:     executor,
		Workdir:      workdir,
		ArtifactsDir: artifactsDir,
		FileStore:    fileStore,
		Sessions:     sessions,
		jobs:         map[string]*Job{},
	}
}

// Submit 提交任务并异步执行，返回 job id（12 位 hex）。
func (tm *TaskManager) Submit(tool Tool, params map[string]any) string {
	now := float64(time.Now().UnixNano()) / 1e9
	job := &Job{
		ID:        randomHex(6),
		ToolID:    tool.ID(),
		Params:    params,
		Status:    StatusPending,
		CreatedAt: now,
	}
	tm.mu.Lock()
	if tm.jobs == nil {
		tm.jobs = map[string]*Job{}
	}
	tm.jobs[job.ID] = job
	tm.mu.Unlock()
	go tm.run(job, tool)
	return job.ID
}

// Get 按 job_id 查找。
func (tm *TaskManager) Get(jobID string) (*Job, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	j, ok := tm.jobs[jobID]
	return j, ok
}

// ResultSnapshot 加锁返回当前结果快照（app 层读取产物列表用）。
func (j *Job) ResultSnapshot() (*ToolResult, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.Result, j.Result != nil
}

func nowFloat() float64 { return float64(time.Now().UnixNano()) / 1e9 }

func (tm *TaskManager) run(job *Job, tool Tool) {
	job.mu.Lock()
	job.Status = StatusRunning
	started := nowFloat()
	job.StartedAt = &started
	job.mu.Unlock()

	jobWorkdir := filepath.Join(tm.Workdir, job.ID)
	jobArtifacts := filepath.Join(tm.ArtifactsDir, job.ID)
	_ = os.MkdirAll(jobWorkdir, 0o755)
	_ = os.MkdirAll(jobArtifacts, 0o755)

	defer func() {
		if r := recover(); r != nil {
			job.mu.Lock()
			job.Status = StatusError
			job.Error = fmt.Sprintf("%T: %v", r, r)
			job.mu.Unlock()
		}
		finished := nowFloat()
		job.mu.Lock()
		job.FinishedAt = &finished
		job.mu.Unlock()
		// 终态事件：run() 的所有返回路径（panic/校验失败/正常结束）都在此汇合，
		// 此时 Status 已是终态，推送一次即可。
		if tm.OnJobDone != nil {
			tm.OnJobDone(job)
		}
	}()

	params, err := tm.resolveFileParams(tool, job.Params, jobWorkdir)
	if err != nil {
		job.mu.Lock()
		job.Status = StatusError
		job.Error = err.Error()
		job.mu.Unlock()
		return
	}

	ctx := context.Background()
	tc := &ToolContext{
		Executor:     tm.Executor,
		Workdir:      jobWorkdir,
		ArtifactsDir: jobArtifacts,
		Sessions:     tm.Sessions,
		ReportProgress: func(percent int, message string) {
			job.mu.Lock()
			job.ProgressPercent = percent
			job.ProgressMessage = message
			job.mu.Unlock()
		},
	}
	result, runErr := tool.Run(ctx, params, tc)

	job.mu.Lock()
	defer job.mu.Unlock()
	if runErr != nil {
		job.Status = StatusError
		job.Error = runErr.Error()
		return
	}
	collectArtifacts(result, jobArtifacts)
	job.Result = result
	job.Status = result.Status
}

// resolveFileParams 把 file 类型参数（file_id）落盘到工作目录，替换为相对文件名。
// 契约（与前端 uploadFileParams 对齐）：
//   - 单文件 → 字符串 file_id → 落盘为扁平安全文件名（历史行为不变）
//   - 多文件 → file_id 数组 → 落盘为相对路径数组；文件夹上传时保留子目录结构，
//     重名自动加 _2/_3 后缀去重
func (tm *TaskManager) resolveFileParams(tool Tool, params map[string]any, workdir string) (map[string]any, error) {
	if tm.FileStore == nil {
		return params, nil
	}
	resolved := map[string]any{}
	for k, v := range params {
		resolved[k] = v
	}
	for _, spec := range tool.Params() {
		if spec.Type != "file" {
			continue
		}
		fidAny, ok := resolved[spec.Name]
		if !ok || fidAny == nil {
			continue
		}
		if fid, isStr := fidAny.(string); isStr {
			if fid == "" {
				continue
			}
			name, err := tm.copyStoredFile(fid, false, workdir, nil)
			if err != nil {
				return nil, err
			}
			resolved[spec.Name] = name
			continue
		}
		ids, isArr := asStringSlice(fidAny)
		if !isArr || len(ids) == 0 {
			continue
		}
		used := map[string]bool{}
		names := make([]string, 0, len(ids))
		for _, id := range ids {
			if id == "" {
				continue
			}
			name, err := tm.copyStoredFile(id, true, workdir, used)
			if err != nil {
				return nil, err
			}
			names = append(names, name)
		}
		resolved[spec.Name] = names
	}
	return resolved, nil
}

// copyStoredFile 把 file_id 落盘到 workdir，返回相对路径。
// preserveDir=false 扁平化为安全文件名（历史单文件行为）；
// preserveDir=true 保留存储名中的相对目录结构（文件夹上传），
// used 非 nil 时对重名目标加 _2/_3 后缀（跨目录重名也去重）。
func (tm *TaskManager) copyStoredFile(fid string, preserveDir bool, workdir string, used map[string]bool) (string, error) {
	stored, err := tm.FileStore.Get(fid)
	if err != nil {
		return "", err
	}
	if stored == nil {
		return "", fmt.Errorf("文件不存在: %s", fid)
	}
	name := safeFilename(stored.Name)
	if preserveDir {
		name = sanitizeRelPath(stored.Name)
	}
	if name == "" || name == "." {
		name = "upload"
	}
	if used != nil {
		base, ext := splitExt(name)
		final := name
		for i := 2; used[final]; i++ {
			final = base + fmt.Sprintf("_%d", i) + ext
		}
		name = final
		used[name] = true
	}
	src, err := os.Open(stored.Path)
	if err != nil {
		return "", err
	}
	defer src.Close()
	dstPath := filepath.Join(workdir, filepath.FromSlash(name))
	if dir := filepath.Dir(dstPath); dir != workdir {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(dst, src)
	dst.Close()
	if copyErr != nil {
		return "", copyErr
	}
	return name, nil
}

// safeFilename 对应 Python _safe_filename：反斜杠归一后取最后一段。
func safeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return "upload"
	}
	return name
}

// sanitizeRelPath 清洗相对路径：\ → /，丢弃空段与 "."；".." 弹出上一段
// （越界时忽略），杜绝目录穿越。
func sanitizeRelPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	segs := strings.Split(p, "/")
	out := make([]string, 0, len(segs))
	for _, s := range segs {
		switch s {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		default:
			out = append(out, s)
		}
	}
	return strings.Join(out, "/")
}

// splitExt 拆分文件名与扩展名（含点）。
func splitExt(name string) (string, string) {
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext), ext
}

// asStringSlice 把 []string / []any 归一为 []string；其他类型返回 false。
func asStringSlice(v any) ([]string, bool) {
	switch arr := v.(type) {
	case []string:
		return arr, true
	case []any:
		out := make([]string, 0, len(arr))
		for _, it := range arr {
			s, ok := it.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	}
	return nil, false
}

// collectArtifacts 自动收集产物目录下工具写出的文件（对应 Python _collect_artifacts）。
func collectArtifacts(result *ToolResult, artifactsDir string) {
	existing := map[string]bool{}
	for _, a := range result.Artifacts {
		existing[a.Path] = true
	}
	entries, err := os.ReadDir(artifactsDir)
	if err != nil {
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		full := filepath.Join(artifactsDir, name)
		if existing[full] {
			continue
		}
		if st, err := os.Stat(full); err == nil {
			result.Artifacts = append(result.Artifacts, Artifact{Name: name, Path: full, Size: st.Size()})
		}
	}
}
