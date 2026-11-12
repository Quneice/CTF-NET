package main

import (
	"ctfnet/internal/aiagent"
	"ctfnet/internal/core"
)

// —— AI 渗透工作台 Wails 绑定（P0：仪表盘 / 任务详情与控制 / 发现 / 资产 / 活动流）——
//
// 数据源：<DataDir>/aiagent/tasks/*/task.json（运行中任务取内存态，实时）。

// aiTaskRoot AI 任务统一落盘根目录。
func (a *App) aiTaskRoot() string {
	dataDir := core.GetDataDir()
	if a.Cfg != nil && a.Cfg.DataDir != "" {
		dataDir = a.Cfg.DataDir
	}
	return aiagent.DefaultTaskRoot(dataDir)
}

// ListAITasks AI 任务列表摘要（按创建时间倒序）。
func (a *App) ListAITasks() ([]aiagent.AITaskSummary, error) {
	return aiagent.ScanAITasks(a.aiTaskRoot())
}

// GetAITask 任务详情快照（运行中取内存态，否则读盘；不存在返回错误）。
func (a *App) GetAITask(taskID string) (*aiagent.Task, error) {
	return aiagent.LoadOrLiveTask(a.aiTaskRoot(), taskID)
}

// ControlAITask 控制运行中任务：stop / pause / resume。
func (a *App) ControlAITask(taskID, action string) error {
	return aiagent.ControlRunningTask(taskID, action)
}

// ControlAIIntent 控制运行中任务的单条意图：rerun（终态复活）/ steer（注入舵向，msg 必填）。
func (a *App) ControlAIIntent(taskID string, intentID int, action, msg string) error {
	return aiagent.ControlRunningIntent(taskID, intentID, action, msg)
}

// AIStats AI 仪表盘总览（任务/发现/资产/token 用量 + 按天曲线）。
func (a *App) AIStats() (*aiagent.StatsOverview, error) {
	return aiagent.GlobalStats(a.aiTaskRoot())
}

// AIFindings 全部任务的漏洞聚合（按上报时间倒序）。
func (a *App) AIFindings() ([]aiagent.FindingDTO, error) {
	return aiagent.AggregateFindings(a.aiTaskRoot())
}

// AIAssets 全部任务的资产聚合。
func (a *App) AIAssets() ([]aiagent.AssetDTO, error) {
	return aiagent.AggregateAssets(a.aiTaskRoot())
}

// AIActivities 跨任务最近活动流（limit 条，0 = 全部）。
func (a *App) AIActivities(limit int) ([]aiagent.ActivityDTO, error) {
	return aiagent.RecentActivities(a.aiTaskRoot(), limit)
}
