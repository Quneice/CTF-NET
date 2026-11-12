package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// —— 对话式渗透：会话持久化 + 后台任务启动/查询 ——
//
// 回合引擎在 chat_norma.go（原样照搬 ARTEX-1.0.4 的对话实现方式：norma
// agentcore.Session + transcript JSONL 续聊 + captureRun 事件合并）。
// 本文件保留会话基础设施：
//   - 会话 = session.json（消息历史，展示用）+ board/task.json（事实/资产/漏洞黑榜）
//   - 模型侧历史走 transcript JSONL（<SessionRoot>/<id>/<id>.jsonl，Resume 恢复）
//   - start_task 在后台 goroutine 跑完整编排器（与对话任务生命周期解耦），
//     会话记录 task 引用，task_status 实时查询
//   - busy 锁保证同一会话同时只有一个回合在跑

// ChatSeg 助手回复的轮次分段：一轮 = 思考/叙述 + 本轮工具动作。
// 由 chat_norma.go 的 captureChatRun 活动记录映射而来并随会话持久化，
// 前端按分段逐轮渲染（不再把所有轮次压进一个气泡）。
type ChatSeg struct {
	Kind    string `json:"kind"`               // text | thinking | tool | tool_result
	Text    string `json:"text,omitempty"`     // thinking/text 的内容
	Name    string `json:"name,omitempty"`     // tool/tool_result 的工具名
	Summary string `json:"summary,omitempty"`  // tool 参数摘要 / tool_result 输出节选
	IsError bool   `json:"is_error,omitempty"` // tool_result 是否报错
}

// ChatMsg 会话消息。
type ChatMsg struct {
	Role      string    `json:"role"` // user | assistant
	Content   string    `json:"content"`
	Reasoning string    `json:"reasoning,omitempty"` // 思考内容（DeepSeek 思考模式续传必须回传）
	Segments  []ChatSeg `json:"segments,omitempty"`  // 助手回复的轮次分段（仅展示；模型历史走 transcript JSONL）
	TS        string    `json:"ts"`
}

// ChatSession 对话会话（持久化于 <root>/<id>/session.json）。
type ChatSession struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
	Messages  []ChatMsg         `json:"messages"`
	TaskIDs   []string          `json:"task_ids,omitempty"`  // 本会话启动的后台任务
	TaskDirs  map[string]string `json:"task_dirs,omitempty"` // 任务 ID → 工作目录
}

// ChatSessionSummary 会话列表项。
type ChatSessionSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt string `json:"updated_at"`
	MsgCount  int    `json:"msg_count"`
}

func nowStr() string { return time.Now().Format("2006-01-02 15:04:05") }

// NewChatSessionID 生成新会话 ID（chat-<时间戳>）。
func NewChatSessionID() string { return "chat-" + time.Now().Format("20060102-150405") }

// OpenChatSession 读取或新建会话与其黑榜（chatRoot/<id>/session.json +
// board/task.json）。会话不存在时创建空会话；黑榜不存在时新建。
func OpenChatSession(chatRoot, sessionID string) (*ChatSession, *Task, error) {
	s, err := LoadChatSession(chatRoot, sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("读取会话失败: %w", err)
	}
	if s == nil {
		s = &ChatSession{ID: sessionID, CreatedAt: nowStr(), Messages: []ChatMsg{}}
	}
	boardDir := filepath.Join(chatRoot, sessionID, "board")
	board, err := LoadTask(boardDir)
	if err != nil || board == nil {
		board = NewTask(sessionID, "对话会话黑榜", "", boardDir)
	}
	return s, board, nil
}

// SaveChatSession 落盘会话（root/<id>/session.json）。
func SaveChatSession(root string, s *ChatSession) error {
	if err := os.MkdirAll(filepath.Join(root, s.ID), 0o755); err != nil {
		return err
	}
	s.UpdatedAt = nowStr()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(root, s.ID, "session.json.tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(root, s.ID, "session.json"))
}

// LoadChatSession 读取会话；不存在返回 (nil, nil)。
func LoadChatSession(root, id string) (*ChatSession, error) {
	data, err := os.ReadFile(filepath.Join(root, id, "session.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s ChatSession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ListChatSessions 列出全部会话摘要（按更新时间倒序）。
func ListChatSessions(root string) ([]ChatSessionSummary, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]ChatSessionSummary, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := LoadChatSession(root, e.Name())
		if err != nil || s == nil {
			continue
		}
		out = append(out, ChatSessionSummary{
			ID: s.ID, Title: s.Title, UpdatedAt: s.UpdatedAt, MsgCount: len(s.Messages),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out, nil
}

// —— 会话 busy 锁（对齐 ARTEX busyKey 防并发的意图）——

var chatBusy sync.Map // sessionID -> struct{}

// AcquireChatLock 抢占会话回合锁；false = 已有回合在跑。
func AcquireChatLock(sessionID string) bool {
	_, loaded := chatBusy.LoadOrStore(sessionID, struct{}{})
	return !loaded
}

// ReleaseChatLock 释放会话回合锁。
func ReleaseChatLock(sessionID string) { chatBusy.Delete(sessionID) }

// —— 后台任务注册（对话发起的自动化任务）——

var runningTasks sync.Map // taskID -> *Task

// StartBackgroundTask 在后台 goroutine 跑完整编排器。
// task.TimeoutSec 决定墙钟超时（0 = 不限时）；执行与调用方生命周期解耦。
// 任务对象注册进 runningTasks，可用 TaskStatusReport 实时查询。
func StartBackgroundTask(cfg Config, proxyURL, recipesDir string, exec ExecFunc, task *Task) error {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return fmt.Errorf("未配置 LLM API Key")
	}
	var ctx context.Context
	var cancel context.CancelFunc
	if task.TimeoutSec > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(task.TimeoutSec+120)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	RegisterRunningTask(task, cancel)
	go func() {
		defer cancel()
		defer UnregisterRunningTask(task.ID)
		orch, err := NewOrchestrator(cfg, proxyURL, recipesDir, exec)
		if err != nil {
			task.FinishFailed("初始化编排器失败: " + err.Error())
			return
		}
		_ = orch.Run(ctx, task)
	}()
	return nil
}

// TaskStatusReport 生成任务状态摘要（live 优先，否则读 task.json）。
func TaskStatusReport(taskID, taskDir string) string {
	if v, ok := runningTasks.Load(taskID); ok {
		return summarizeTask(v.(*Task), true)
	}
	if taskDir != "" {
		if t, err := LoadTask(taskDir); err == nil {
			return summarizeTask(t, false)
		}
	}
	return fmt.Sprintf("任务 %s：未找到运行实例，且 %s/task.json 不存在", taskID, taskDir)
}

// summarizeTask 任务状态 → 人类可读摘要。
func summarizeTask(t *Task, live bool) string {
	snap := t.Snapshot()
	alive := "已结束"
	if live {
		alive = "运行中"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "任务 %s [%s/%s] %s\n", snap.ID, snap.Status, alive, snap.GoalText)
	if snap.Summary != "" {
		sb.WriteString("结论: " + snap.Summary + "\n")
	}
	fmt.Fprintf(&sb, "意图: %d 条（open=%d running=%d done=%d exhausted=%d blocked=%d）\n",
		len(snap.Intents), countIntentState(snap.Intents, IntentOpen), countIntentState(snap.Intents, IntentRunning),
		countIntentState(snap.Intents, IntentDone), countIntentState(snap.Intents, IntentExhausted),
		countIntentState(snap.Intents, IntentBlocked))
	fmt.Fprintf(&sb, "漏洞: %d 个 | 事实: %d 条 | 资产: %d 个\n", len(snap.Findings), len(snap.Facts), len(snap.Assets))
	for _, f := range snap.Findings {
		fmt.Fprintf(&sb, "- [%s] %s (%s)\n", f.Severity, f.Title, f.VulnClass)
	}
	acts := snap.Activities
	if len(acts) > 5 {
		acts = acts[len(acts)-5:]
	}
	sb.WriteString("最近活动:\n")
	for _, a := range acts {
		fmt.Fprintf(&sb, "  [%s] %s\n", a.Agent, a.Summary)
	}
	return sb.String()
}

func countIntentState(intents []Intent, state string) int {
	n := 0
	for _, it := range intents {
		if it.State == state {
			n++
		}
	}
	return n
}

// —— ChatAgent 工具集 ——

// ChatEnv 对话回合的环境（由工具层构造）。
type ChatEnv struct {
	Cfg         Config
	ProxyURL    string
	RecipesDir  string
	Exec        ExecFunc
	TaskRoot    string // 后台任务工作目录根
	SessionRoot string // 会话持久化根目录
	Session     *ChatSession
	saveFn      func() error // start_task 后持久化会话
}

// chatRegistry 组装对话 agent 工具集：worker 侦察工具 + 任务启动/查询。
func chatRegistry(env *ChatEnv) (*registry, error) {
	reg := workerTools(env.ProxyURL, env.Cfg.EnableExec)
	recipeTools, err := LoadRecipes(env.RecipesDir)
	if err != nil {
		return nil, err
	}
	if err := registerRecipes(reg, recipeTools); err != nil {
		return nil, err
	}
	reg.register(&startTaskTool{env: env})
	reg.register(&taskStatusTool{env: env})
	return reg, nil
}

// startTaskTool 发起后台自动化渗透任务（对齐 ARTEX StartTriggeredRun）。
type startTaskTool struct{ env *ChatEnv }

func (t *startTaskTool) Name() string { return "start_task" }
func (t *startTaskTool) Description() string {
	return "发起一个后台自动化渗透任务（planner/worker 双角色引擎持续工作）。适合目标范围大、需要系统化推进时。返回任务 ID，之后可用 task_status 查询进展。参数：goal 必填；description/target_hint 可选；max_workers/plan_rounds/timeout_seconds 可选覆盖默认。"
}
func (t *startTaskTool) InputSchema() map[string]any {
	return objSchema(map[string]any{
		"goal":            prop("string", "渗透目标：目标地址 + 期望成果"),
		"description":     prop("string", "任务描述（可选）"),
		"target_hint":     prop("string", "已知线索（可选）"),
		"max_workers":     prop("integer", "并发 worker 数（可选，默认按配置）"),
		"plan_rounds":     prop("integer", "规划轮次（可选，默认按配置）"),
		"timeout_seconds": prop("integer", "任务时限秒（可选，默认按配置）"),
	}, []string{"goal"})
}

func (t *startTaskTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	goal := strings.TrimSpace(argString(args, "goal"))
	if goal == "" {
		return "", fmt.Errorf("缺少 goal")
	}
	desc := argString(args, "description")
	if desc == "" {
		desc = goal
	}
	id := "task-" + time.Now().Format("20060102-150405")
	dir := filepath.Join(t.env.TaskRoot, id)
	cfg2 := t.env.Cfg
	if v := argInt(args, "max_workers"); v > 0 {
		cfg2.MaxWorkers = v
	}
	if v := argInt(args, "plan_rounds"); v > 0 {
		cfg2.PlanRounds = v
	}
	if v := argInt(args, "timeout_seconds"); v > 0 {
		cfg2.Timeout = v
	}
	task := NewTask(id, desc, goal, dir)
	task.Description = desc
	task.Hint = argString(args, "target_hint")
	task.TimeoutSec = cfg2.Timeout
	task.MaxTurns = cfg2.MaxTurns
	task.MaxWorkers = cfg2.MaxWorkers
	task.PlanRounds = cfg2.PlanRounds
	task.EnableExec = cfg2.EnableExec
	if err := StartBackgroundTask(cfg2, t.env.ProxyURL, t.env.RecipesDir, t.env.Exec, task); err != nil {
		return "", err
	}
	t.env.Session.TaskIDs = append(t.env.Session.TaskIDs, id)
	if t.env.Session.TaskDirs == nil {
		t.env.Session.TaskDirs = map[string]string{}
	}
	t.env.Session.TaskDirs[id] = dir
	if t.env.saveFn != nil {
		_ = t.env.saveFn()
	}
	tc.Task.Log(tc.Agent, "task", fmt.Sprintf("发起后台任务 %s: %s", id, goal))
	return fmt.Sprintf("后台自动化渗透任务已启动，ID=%s。它会自行规划并执行，完成后可用 task_status 查询漏洞与结论。", id), nil
}

// taskStatusTool 查询本会话发起的任务状态。
type taskStatusTool struct{ env *ChatEnv }

func (t *taskStatusTool) Name() string { return "task_status" }
func (t *taskStatusTool) Description() string {
	return "查询后台自动化任务的状态（意图进度/漏洞/最近活动）。task_id 可选：省略时列出本会话启动的全部任务。"
}
func (t *taskStatusTool) InputSchema() map[string]any {
	return objSchema(map[string]any{"task_id": prop("string", "任务 ID（可选）")}, nil)
}

func (t *taskStatusTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	id := argString(args, "task_id")
	if id != "" {
		dir := t.env.Session.TaskDirs[id]
		return TaskStatusReport(id, dir), nil
	}
	var sb strings.Builder
	for _, tid := range t.env.Session.TaskIDs {
		sb.WriteString(TaskStatusReport(tid, t.env.Session.TaskDirs[tid]))
		sb.WriteString("\n")
	}
	if sb.Len() == 0 {
		return "(本会话尚未发起过后台任务；可用 start_task 发起)", nil
	}
	return sb.String(), nil
}

// —— 会话回合 ——
//
// 回合引擎已迁移到 chat_norma.go（原样照搬 ARTEX-1.0.4 的对话实现方式）：
// RunChatTurn / ChatEmitter / captureChatRun 均在那里定义。

// ChatTurnResult 一轮对话的结果。
type ChatTurnResult struct {
	Reply     string
	SessionID string
}

// titleOf 用首条消息生成会话标题。
func titleOf(msg string) string {
	m := strings.TrimSpace(msg)
	m = strings.ReplaceAll(m, "\n", " ")
	r := []rune(m)
	if len(r) > 30 {
		m = string(r[:30]) + "…"
	}
	if m == "" {
		m = "新会话"
	}
	return m
}

// chatSystemPrompt 对话式渗透助手角色卡。
func chatSystemPrompt() string {
	return `你是 CTF-NET 的【对话式渗透测试助手】。你与用户直接对话，讨论并推进渗透测试任务；你的每一条回复都会作为消息直接展示给用户。

## 你能做什么
- 直接侦察与验证：http_request、exec_command、配方工具（http_headers/dns_lookup 等），每一步用真实工具输出说话。
- 沉淀发现：新资产用 add_asset、服务指纹与线索用 record_fact；确认的漏洞用 report_finding 上报（必须带真实触发的证据节选，无证据会被拒绝）。
- 发起自动化任务：目标范围大、需要系统化推进时，用 start_task 把目标交给后台自动化引擎（它有自己的规划者/执行者，会持续工作并产出漏洞清单）。
- 查进展：用 task_status 查询已启动任务的实时状态；list_findings 查本会话已上报的漏洞。
- 讨论：分析用户给的线索、制定攻击思路、解释结果、回答技术问题。

## 对话纪律
- 回复简洁、结论先行，中文；不要复述思考过程，不编造工具输出。
- 引用证据时给出关键节选（响应头/状态码/输出片段），让用户能独立复核。
- 用户消息不明确时，先澄清目标与授权范围再动手，不要假设。
- 仅对用户授权的目标行动；破坏性命令会被工具拒绝；请求保持低频；禁止对目标发起 DDoS 类操作。
- 与用户对话时直接回答问题；需要动手时先说明要做什么，再调用工具。

` + winExecNote()
}

// chatSettlePrompt 步数/时间预算到点时注入的收尾提示：让模型用最后一步直接写结论，
// 而不是空手结束（norma 无 Settlement 时预算终止的终答文本为空，前端只能显示占位符）。
func chatSettlePrompt() string {
	return `（系统提示）你已触发本轮对话的步数上限，本轮即将结束。请不要再调用任何工具，直接用简洁中文整理结论回复用户，覆盖三部分：目标概况、已确认的关键发现（含证据节选/flag）、未完成事项与下一步建议。`
}
