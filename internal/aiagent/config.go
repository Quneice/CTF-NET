// Package aiagent 是 CTF-NET 原生的 AI 自动化渗透引擎：
//
//   - AI 自动化渗透：planner/worker 双角色 LLM agent 循环（规划 → 意图队列 →
//     worker 带工具执行 → 写回事实/资产/漏洞 → 触发再规划 → 终态判定）
//   - 编排：任务状态机（created→running→done/exhausted/failed/timeout/stopped）、
//     意图队列与并发调度、规划轮次预算、墙钟超时、上下文取消
//   - 工具调用：OpenAI 兼容 function-calling 循环 + 内置工具集（HTTP 请求/本地
//     命令/漏洞上报/资产与事实写回/意图管理）+ YAML 配方工具（command/args/flag
//     模型，见 recipe.go）
//
// 能力设计参考 ARTEX v1.0.4（jaykwok/ARTEX，AGPL-3.0）的角色划分、工具目录与
// 提示词纪律，但为本项目全新实现，不包含其任何源码。
package aiagent

import (
	"strings"
	"time"
)

// Config 一次 AI 渗透任务的运行配置。
// 来源优先级：工具参数（显式传值）> CTF-NET 设置页 ai 组 > 内置默认值。
type Config struct {
	// LLM 接入（OpenAI 兼容协议：智谱 GLM / DeepSeek / OpenAI / Ollama / 各类中转均可用）。
	APIKey  string // API Key（必填，无 Key 无法运行）
	BaseURL string // 如 https://open.bigmodel.cn/api/paas/v4（智谱）；留空用默认
	Model   string // 如 glm-5.3-flash / glm-4.5-air；留空用默认

	// ReasoningEffort 思考强度（智谱 glm-5.x 始终思考模型：low/high/max）。
	// 留空 = 不传该字段（服务端默认 max）；glm-5.3-flash 建议 low 省 token。
	ReasoningEffort string

	MaxTurns   int // 单个 worker/planner 会话的最大工具轮数（0=不限）
	MaxWorkers int // 并发 worker 数（意图并行执行上限）
	PlanRounds int // 规划轮次预算（超出后不再唤醒 planner）
	Timeout    int // 任务墙钟超时（秒，0=不限时）

	EnableExec bool // 是否给 worker 开放 exec_command（本地命令执行）
}

// DefaultConfig 内置默认值。
func DefaultConfig() Config {
	return Config{
		BaseURL:    "https://open.bigmodel.cn/api/paas/v4",
		Model:      "glm-5.3-flash",
		MaxTurns:   48,
		MaxWorkers: 2,
		PlanRounds: 6,
		Timeout:    3600,
		EnableExec: true,
	}
}

// normalize 补默认值并做边界收敛。
func (c Config) normalize() Config {
	d := DefaultConfig()
	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = d.BaseURL
	}
	if strings.TrimSpace(c.Model) == "" {
		c.Model = d.Model
	}
	if c.MaxTurns <= 0 {
		c.MaxTurns = d.MaxTurns
	}
	if c.MaxWorkers <= 0 {
		c.MaxWorkers = d.MaxWorkers
	} else if c.MaxWorkers > 8 {
		c.MaxWorkers = 8
	}
	if c.PlanRounds <= 0 {
		c.PlanRounds = d.PlanRounds
	}
	if c.Timeout < 0 {
		c.Timeout = d.Timeout
	}
	return c
}

// callTimeout 单次 LLM 调用的 HTTP 超时（秒）。
func (c Config) callTimeout() time.Duration {
	t := time.Duration(180) * time.Second
	if c.Timeout > 0 {
		if v := time.Duration(c.Timeout/4) * time.Second; v < t && v > 30*time.Second {
			t = v
		}
	}
	return t
}
