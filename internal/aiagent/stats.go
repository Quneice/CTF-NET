package aiagent

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

// —— 跨任务聚合统计层（AI 仪表盘 / 发现页 / 资产页数据源）——
//
// 扫描 <DataDir>/aiagent/tasks/*/task.json 聚合；运行中任务以内存态优先，
// 保证仪表盘实时。全部只读，不修改黑板。

// AITaskSummary 任务列表摘要。
type AITaskSummary struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	GoalText    string     `json:"goal_text"`
	Status      string     `json:"status"`
	Summary     string     `json:"summary,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Paused      bool       `json:"paused"`
	Intents     int        `json:"intents"`
	OpenIntents int        `json:"open_intents"`
	Findings    int        `json:"findings"`
	Assets      int        `json:"assets"`
	Facts       int        `json:"facts"`
	Usage       UsageTotal `json:"usage"`
}

// summarizeTaskDTO 快照 → 摘要。
func summarizeTaskDTO(s *Task) AITaskSummary {
	return AITaskSummary{
		ID:          s.ID,
		Description: s.Description,
		GoalText:    s.GoalText,
		Status:      s.Status,
		Summary:     s.Summary,
		CreatedAt:   s.CreatedAt,
		CompletedAt: s.CompletedAt,
		Paused:      s.Paused,
		Intents:     len(s.Intents),
		OpenIntents: countIntentState(s.Intents, IntentOpen),
		Findings:    len(s.Findings),
		Assets:      len(s.Assets),
		Facts:       len(s.Facts),
		Usage:       s.Usage,
	}
}

// ScanAITasks 列出全部任务摘要（按创建时间倒序）。
// 注意：任何空结果都返回非 nil 空切片——Wails 把 nil 序列化成 JSON null，前端会崩。
func ScanAITasks(taskRoot string) ([]AITaskSummary, error) {
	entries, err := os.ReadDir(taskRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []AITaskSummary{}, nil
		}
		return nil, err
	}
	out := make([]AITaskSummary, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := taskSnapshotOrLive(taskRoot, e.Name())
		if err != nil {
			continue
		}
		out = append(out, summarizeTaskDTO(s))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// taskSnapshotOrLive 运行中 → 内存快照；否则读 task.json。
func taskSnapshotOrLive(taskRoot, taskID string) (*Task, error) {
	if v, ok := runningTasks.Load(taskID); ok {
		return v.(*Task).Snapshot(), nil
	}
	return LoadTask(filepath.Join(taskRoot, taskID))
}

// FindingDTO 聚合发现（带任务上下文）。
type FindingDTO struct {
	TaskID   string  `json:"task_id"`
	TaskDesc string  `json:"task_desc"`
	Finding  Finding `json:"finding"`
}

// AggregateFindings 聚合全部任务的漏洞（按上报时间倒序）。
func AggregateFindings(taskRoot string) ([]FindingDTO, error) {
	entries, err := os.ReadDir(taskRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []FindingDTO{}, nil
		}
		return nil, err
	}
	out := []FindingDTO{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := taskSnapshotOrLive(taskRoot, e.Name())
		if err != nil {
			continue
		}
		for _, f := range s.Findings {
			out = append(out, FindingDTO{TaskID: s.ID, TaskDesc: s.Description, Finding: f})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Finding.CreatedAt > out[j].Finding.CreatedAt })
	return out, nil
}

// AssetDTO 聚合资产（带任务上下文）。
type AssetDTO struct {
	TaskID   string `json:"task_id"`
	TaskDesc string `json:"task_desc"`
	Asset    Asset  `json:"asset"`
}

// AggregateAssets 聚合全部任务的资产。
func AggregateAssets(taskRoot string) ([]AssetDTO, error) {
	entries, err := os.ReadDir(taskRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []AssetDTO{}, nil
		}
		return nil, err
	}
	out := []AssetDTO{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := taskSnapshotOrLive(taskRoot, e.Name())
		if err != nil {
			continue
		}
		for _, a := range s.Assets {
			out = append(out, AssetDTO{TaskID: s.ID, TaskDesc: s.Description, Asset: a})
		}
	}
	return out, nil
}

// ActivityDTO 聚合活动流（带任务上下文）。
type ActivityDTO struct {
	TaskID   string   `json:"task_id"`
	TaskDesc string   `json:"task_desc"`
	Activity Activity `json:"activity"`
}

// RecentActivities 跨任务最近 N 条活动（时间倒序）。
func RecentActivities(taskRoot string, n int) ([]ActivityDTO, error) {
	entries, err := os.ReadDir(taskRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []ActivityDTO{}, nil
		}
		return nil, err
	}
	out := []ActivityDTO{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := taskSnapshotOrLive(taskRoot, e.Name())
		if err != nil {
			continue
		}
		for _, act := range s.Activities {
			out = append(out, ActivityDTO{TaskID: s.ID, TaskDesc: s.Description, Activity: act})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Activity.TS.After(out[j].Activity.TS) })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out, nil
}

// StatsOverview AI 仪表盘总览。
type StatsOverview struct {
	Tasks     int        `json:"tasks"`
	Running   int        `json:"running"`
	Paused    int        `json:"paused"`
	Done      int        `json:"done"`
	Exhausted int        `json:"exhausted"`
	Failed    int        `json:"failed"`
	Findings  int        `json:"findings"`
	Assets    int        `json:"assets"`
	Facts     int        `json:"facts"`
	Intents   int        `json:"intents"`
	Usage     UsageTotal `json:"usage"`
	ByDay     []DayUsage `json:"by_day"` // 跨任务按天合并（升序，仅含用量非零的天）
}

// GlobalStats 跨任务聚合统计（单遍全量快照：总览 + 按天用量曲线）。
func GlobalStats(taskRoot string) (*StatsOverview, error) {
	entries, err := os.ReadDir(taskRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return &StatsOverview{}, nil
		}
		return nil, err
	}
	gs := &StatsOverview{ByDay: []DayUsage{}}
	dayIdx := map[string]int{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := taskSnapshotOrLive(taskRoot, e.Name())
		if err != nil {
			continue
		}
		gs.Tasks++
		switch s.Status {
		case StatusRunning, StatusCreated:
			gs.Running++
		case StatusDone:
			gs.Done++
		case StatusExhausted:
			gs.Exhausted++
		case StatusFailed, StatusTimeout, StatusStopped:
			gs.Failed++
		}
		if s.Paused {
			gs.Paused++
		}
		gs.Findings += len(s.Findings)
		gs.Assets += len(s.Assets)
		gs.Facts += len(s.Facts)
		gs.Intents += len(s.Intents)
		gs.Usage.In += s.Usage.In
		gs.Usage.Out += s.Usage.Out
		for _, d := range s.UsageByDay {
			if i, ok := dayIdx[d.Day]; ok {
				gs.ByDay[i].Usage.In += d.Usage.In
				gs.ByDay[i].Usage.Out += d.Usage.Out
			} else {
				dayIdx[d.Day] = len(gs.ByDay)
				gs.ByDay = append(gs.ByDay, DayUsage{Day: d.Day, Usage: d.Usage})
			}
		}
	}
	sort.Slice(gs.ByDay, func(i, j int) bool { return gs.ByDay[i].Day < gs.ByDay[j].Day })
	return gs, nil
}
