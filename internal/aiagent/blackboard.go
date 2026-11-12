package aiagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// —— 任务黑板：规划器与 worker 之间唯一的共享状态，全部经 Orchestrator 持久化 ——

// 任务状态机：created → running → done / exhausted / failed / timeout / stopped
const (
	StatusCreated   = "created"
	StatusRunning   = "running"
	StatusDone      = "done"      // 目标已证明达成（prove_goal / goal_met）
	StatusExhausted = "exhausted" // 意图耗尽且规划预算用完，目标未达成
	StatusFailed    = "failed"    // 致命错误（LLM 不可恢复失败等）
	StatusTimeout   = "timeout"   // 墙钟超时
	StatusStopped   = "stopped"   // 用户取消（上下文取消）
)

// 意图状态机：open → running → done / exhausted / blocked
const (
	IntentOpen      = "open"
	IntentRunning   = "running"
	IntentDone      = "done"
	IntentExhausted = "exhausted" // worker 做完了但没产出
	IntentBlocked   = "blocked"   // 执行报错（planner 会看 trace 决定重试或换路）
)

// Severity 漏洞严重度。
type Severity string

const (
	SevCritical Severity = "critical"
	SevHigh     Severity = "high"
	SevMedium   Severity = "medium"
	SevLow      Severity = "low"
	SevInfo     Severity = "info"
)

// validSeverity 校验严重度取值。
func validSeverity(s string) (Severity, bool) {
	for _, v := range []Severity{SevCritical, SevHigh, SevMedium, SevLow, SevInfo} {
		if string(v) == s {
			return v, true
		}
	}
	return SevInfo, false
}

// Goal 渗透目标（任务开始时创建）。
type Goal struct {
	ID     int    `json:"id"`
	Text   string `json:"text"`
	Met    bool   `json:"met"`
	MetBy  string `json:"met_by,omitempty"`  // 达成方式简述（prove_goal 的 reason）
	EvdSum string `json:"evd_sum,omitempty"` // 证据要点
	When   string `json:"when,omitempty"`
}

// Intent 规划器派发的执行意图（worker 领取单元）。
type Intent struct {
	ID        int      `json:"id"`
	Summary   string   `json:"summary"` // 一句话方向：目标地址 + 做什么 + 为什么
	State     string   `json:"state"`   // open | running | done | exhausted | blocked
	ParentIDs []int    `json:"parent_ids,omitempty"`
	AssetIDs  []int    `json:"asset_ids,omitempty"`
	Result    string   `json:"result,omitempty"` // worker 总结
	Steer     string   `json:"steer,omitempty"`  // 人工舵向：下一次 worker 回合注入后清空
	Trace     []string `json:"trace,omitempty"`  // worker 关键步骤摘要（≤30 条）
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at,omitempty"`
	Round     int      `json:"round"` // 第几轮规划产出
	Worker    string   `json:"worker,omitempty"`
}

// Asset 资产（URL/域名/IP 等）。
type Asset struct {
	ID      int               `json:"id"`
	Type    string            `json:"type"` // url | domain | ip | service | other
	URL     string            `json:"url,omitempty"`
	Domain  string            `json:"domain,omitempty"`
	IP      string            `json:"ip,omitempty"`
	Props   map[string]string `json:"props,omitempty"`
	AddedBy string            `json:"added_by,omitempty"`
}

// Fact 事实（侦察产出，供 planner 与后续 worker 参考）。
type Fact struct {
	ID     int    `json:"id"`
	Text   string `json:"text"`
	Kind   string `json:"kind,omitempty"` // service | vuln | credential | info | other
	Source string `json:"source,omitempty"`
}

// Finding 漏洞上报（必须有可复现证据，见 report_finding 工具）。
type Finding struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	VulnClass string   `json:"vuln_class"`
	Severity  Severity `json:"severity"`
	Summary   string   `json:"summary"`
	Evidence  string   `json:"evidence"`
	PoC       string   `json:"poc,omitempty"`
	AssetIDs  []int    `json:"asset_ids,omitempty"`
	IntentID  int      `json:"intent_id,omitempty"`
	Confirmed bool     `json:"confirmed"`
	CreatedAt string   `json:"created_at"`
}

// Activity 活动流水（进度上报与前端展示）。
type Activity struct {
	Seq     int       `json:"seq"`
	TS      time.Time `json:"ts"`
	Agent   string    `json:"agent"` // planner | worker | system
	Kind    string    `json:"kind"`  // plan | dispatch | tool | finding | goal | terminal …
	Summary string    `json:"summary"`
}

// Edge 探索图边。From→To 方向 = 上游/父 → 下游/子（与 ARTEX 图约定一致）。
// 节点用 NodeKey(kind, id) 标识（intent/asset/fact/finding/goal 各自独立编号）。
type Edge struct {
	ID   int    `json:"id"`
	From string `json:"from"` // 节点键 "kind:id"
	To   string `json:"to"`
	Rel  string `json:"rel"` // yields | spawns | targets | proves | affects
}

// NodeKey 探索图节点键（kind 为节点类型，id 为该类型列表内的编号）。
func NodeKey(kind string, id int) string { return kind + ":" + itoa(id) }

// UsageTotal token 用量累计。
type UsageTotal struct {
	In  int64 `json:"in"`
	Out int64 `json:"out"`
}

// DayUsage 按天 token 用量（仪表盘趋势）。
type DayUsage struct {
	Day   string     `json:"day"` // 2006-01-02
	Usage UsageTotal `json:"usage"`
}

// Task 黑板。所有字段由持有 mu 的方法读写；任何修改都会触发 persist。
type Task struct {
	mu sync.Mutex

	ID          string `json:"id"`
	Description string `json:"description"`
	GoalText    string `json:"goal_text"`
	Hint        string `json:"hint,omitempty"` // 已知线索（注入首轮规划态势）
	// Constraints/Scope：任务创建时由 goals 角色从目标中提取（ARTEX set_constraints/
	// set_scope 等价物），以最高优先级约束块注入 planner/worker 提示词。
	Constraints string `json:"constraints,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Status      string `json:"status"`
	Summary     string `json:"summary,omitempty"` // 终态时的人工可读结论

	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	TimeoutSec int  `json:"timeout_sec"`
	MaxTurns   int  `json:"max_turns"`
	MaxWorkers int  `json:"max_workers"`
	PlanRounds int  `json:"plan_rounds"`
	RoundsUsed int  `json:"rounds_used"`
	EnableExec bool `json:"enable_exec"`

	Goals      []Goal     `json:"goals"`
	Intents    []Intent   `json:"intents"`
	Assets     []Asset    `json:"assets"`
	Facts      []Fact     `json:"facts"`
	Findings   []Finding  `json:"findings"`
	Activities []Activity `json:"activities"`
	Edges      []Edge     `json:"edges,omitempty"`

	Usage      UsageTotal `json:"usage"`
	UsageByDay []DayUsage `json:"usage_by_day,omitempty"`
	Paused     bool       `json:"paused"`

	dir  string // 持久化目录（空 = 不落盘）
	seq  int    // 活动序号
	onCh func() // 状态变更回调（Orchestrator 唤醒）
}

// NewTask 创建黑板。dir 非空时每次变更都会快照写 dir/task.json。
func NewTask(id, description, goal string, dir string) *Task {
	return &Task{
		ID:          id,
		Description: description,
		GoalText:    goal,
		Status:      StatusCreated,
		CreatedAt:   time.Now(),
		Goals:       []Goal{{ID: 1, Text: goal}},
		dir:         dir,
	}
}

// setOnChange 注册变更回调（内部用）。
func (t *Task) setOnChange(fn func()) {
	t.mu.Lock()
	t.onCh = fn
	t.mu.Unlock()
}

func (t *Task) now() string { return time.Now().Format("15:04:05") }

// —— 读取（每个方法独立加锁，返回快照副本）——

// Snapshot 返回数据副本（逐字段复制，避免整结构拷贝带锁的 sync.Mutex；
// mu/onCh 不复制，副本独立加锁安全）。
func (t *Task) Snapshot() *Task {
	t.mu.Lock()
	defer t.mu.Unlock()
	cp := &Task{
		ID:          t.ID,
		Description: t.Description,
		GoalText:    t.GoalText,
		Hint:        t.Hint,
		Constraints: t.Constraints,
		Scope:       t.Scope,
		Status:      t.Status,
		Summary:     t.Summary,
		CreatedAt:   t.CreatedAt,
		StartedAt:   t.StartedAt,
		CompletedAt: t.CompletedAt,
		TimeoutSec:  t.TimeoutSec,
		MaxTurns:    t.MaxTurns,
		MaxWorkers:  t.MaxWorkers,
		PlanRounds:  t.PlanRounds,
		RoundsUsed:  t.RoundsUsed,
		EnableExec:  t.EnableExec,
		Goals:       append([]Goal(nil), t.Goals...),
		Intents:     append([]Intent(nil), t.Intents...),
		Assets:      append([]Asset(nil), t.Assets...),
		Facts:       append([]Fact(nil), t.Facts...),
		Findings:    append([]Finding(nil), t.Findings...),
		Activities:  append([]Activity(nil), t.Activities...),
		Edges:       append([]Edge(nil), t.Edges...),
		Usage:       t.Usage,
		UsageByDay:  append([]DayUsage(nil), t.UsageByDay...),
		Paused:      t.Paused,
		dir:         t.dir,
		seq:         t.seq,
	}
	return cp
}

func (t *Task) StatusNow() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Status
}

// —— 写入 ——

// log 记活动流水（agent: planner/worker/system）。
func (t *Task) log(agent, kind, summary string) {
	t.mu.Lock()
	t.seq++
	t.Activities = append(t.Activities, Activity{
		Seq: t.seq, TS: time.Now(), Agent: agent, Kind: kind, Summary: summary,
	})
	if len(t.Activities) > 500 {
		t.Activities = t.Activities[len(t.Activities)-500:]
	}
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
}

// Log 导出给 orchestrator/工具。
func (t *Task) Log(agent, kind, summary string) { t.log(agent, kind, summary) }

// finish 写终态（幂等：已终态不再覆盖）。
func (t *Task) finish(status, summary string) {
	t.mu.Lock()
	if t.Status != StatusRunning && t.Status != StatusCreated {
		t.mu.Unlock()
		return
	}
	t.Status = status
	t.Summary = summary
	now := time.Now()
	t.CompletedAt = &now
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
}

// FinishDone 目标达成。
func (t *Task) FinishDone(summary string) { t.finish(StatusDone, summary) }

// FinishExhausted 意图耗尽。
func (t *Task) FinishExhausted(summary string) { t.finish(StatusExhausted, summary) }

// FinishFailed 致命失败。
func (t *Task) FinishFailed(summary string) { t.finish(StatusFailed, summary) }

// FinishTimeout 墙钟超时。
func (t *Task) FinishTimeout(summary string) { t.finish(StatusTimeout, summary) }

// FinishStopped 用户取消。
func (t *Task) FinishStopped(summary string) { t.finish(StatusStopped, summary) }

// MarkRunning 进入运行态。
func (t *Task) MarkRunning() {
	t.mu.Lock()
	if t.Status == StatusCreated {
		t.Status = StatusRunning
		t.StartedAt = time.Now()
	}
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
}

// AddIntent 批量派发意图，返回新意图 ID。
func (t *Task) AddIntents(summaries []string, parentIDs, assetIDs []int, round int) []int {
	t.mu.Lock()
	ids := make([]int, 0, len(summaries))
	for _, s := range summaries {
		n := Intent{
			ID:        len(t.Intents) + 1,
			Summary:   s,
			State:     IntentOpen,
			ParentIDs: parentIDs,
			AssetIDs:  assetIDs,
			CreatedAt: t.now(),
			Round:     round,
		}
		t.Intents = append(t.Intents, n)
		ids = append(ids, n.ID)
		// 探索图边：父意图 spawns 子意图；目标资产 targets 意图
		for _, pid := range parentIDs {
			t.addEdgeLocked(NodeKey("intent", pid), NodeKey("intent", n.ID), "spawns")
		}
		for _, aid := range assetIDs {
			t.addEdgeLocked(NodeKey("asset", aid), NodeKey("intent", n.ID), "targets")
		}
	}
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
	return ids
}

// claimIntent worker 领取一个 open 意图；无可用意图返回 nil。
func (t *Task) claimIntent(worker string) *Intent {
	t.mu.Lock()
	var claimed *Intent
	for i := range t.Intents {
		if t.Intents[i].State == IntentOpen {
			t.Intents[i].State = IntentRunning
			t.Intents[i].Worker = worker
			t.Intents[i].UpdatedAt = t.now()
			cp := t.Intents[i]
			claimed = &cp
			break
		}
	}
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
	return claimed
}

// settleIntent worker 结束后结算意图。
func (t *Task) settleIntent(id int, state, result string) {
	t.mu.Lock()
	for i := range t.Intents {
		if t.Intents[i].ID == id {
			t.Intents[i].State = state
			t.Intents[i].Result = result
			t.Intents[i].UpdatedAt = t.now()
			break
		}
	}
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
}

// appendIntentTrace 追加 worker 步骤摘要。
func (t *Task) appendIntentTrace(id int, line string) {
	t.mu.Lock()
	for i := range t.Intents {
		if t.Intents[i].ID == id {
			if len(t.Intents[i].Trace) >= 30 {
				t.Intents[i].Trace = t.Intents[i].Trace[1:]
			}
			t.Intents[i].Trace = append(t.Intents[i].Trace, line)
			break
		}
	}
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
}

// AddAsset 登记资产，返回资产 ID。intentID 为产生该资产的意图（0 = 无，无 yields 边）。
func (t *Task) AddAsset(a Asset, intentID int) int {
	t.mu.Lock()
	a.ID = len(t.Assets) + 1
	t.Assets = append(t.Assets, a)
	if intentID > 0 {
		t.addEdgeLocked(NodeKey("intent", intentID), NodeKey("asset", a.ID), "yields")
	}
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
	return a.ID
}

// AddFact 记录事实，返回 ID。intentID 为产生该事实的意图（0 = 无）。
func (t *Task) AddFact(text, kind, source string, intentID int) int {
	t.mu.Lock()
	f := Fact{ID: len(t.Facts) + 1, Text: text, Kind: kind, Source: source}
	t.Facts = append(t.Facts, f)
	if intentID > 0 {
		t.addEdgeLocked(NodeKey("intent", intentID), NodeKey("fact", f.ID), "yields")
	}
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
	return f.ID
}

// AddFinding 记录漏洞，返回 ID。
func (t *Task) AddFinding(f Finding) int {
	t.mu.Lock()
	f.ID = len(t.Findings) + 1
	if f.CreatedAt == "" {
		f.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	t.Findings = append(t.Findings, f)
	// 探索图边：意图 yields 漏洞；漏洞 affects 相关资产
	if f.IntentID > 0 {
		t.addEdgeLocked(NodeKey("intent", f.IntentID), NodeKey("finding", f.ID), "yields")
	}
	for _, aid := range f.AssetIDs {
		t.addEdgeLocked(NodeKey("finding", f.ID), NodeKey("asset", aid), "affects")
	}
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
	return f.ID
}

// ProveGoal 标记目标达成（planner 工具调用）。intentID 为证明目标的意图（0 = 无 proves 边）。
func (t *Task) ProveGoal(goalID int, reason, evdSum string, intentID int) error {
	t.mu.Lock()
	found := false
	for i := range t.Goals {
		if t.Goals[i].ID == goalID {
			t.Goals[i].Met = true
			t.Goals[i].MetBy = reason
			t.Goals[i].EvdSum = evdSum
			t.Goals[i].When = t.now()
			found = true
			break
		}
	}
	if found && intentID > 0 {
		t.addEdgeLocked(NodeKey("intent", intentID), NodeKey("goal", goalID), "proves")
	}
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
	if !found {
		return errUnknownGoal(goalID)
	}
	return nil
}

type unknownGoalErr struct{ id int }

func (e unknownGoalErr) Error() string { return "目标 ID 不存在: " + itoa(e.id) }

func errUnknownGoal(id int) error { return unknownGoalErr{id: id} }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// allGoalsMet 目标是否全部达成。
func (t *Task) allGoalsMet() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, g := range t.Goals {
		if !g.Met {
			return false
		}
	}
	return len(t.Goals) > 0
}

// SetGoals 用分解后的子目标替换原始目标（goals 角色产出），并落约束/范围。
// 子目标 ID 从 1 重新编号；持久化并唤醒编排器。
func (t *Task) SetGoals(goals []Goal, constraints, scope string) {
	t.mu.Lock()
	renumbered := make([]Goal, 0, len(goals))
	for i, g := range goals {
		g.ID = i + 1
		g.Met = false
		renumbered = append(renumbered, g)
	}
	t.Goals = renumbered
	t.Constraints = strings.TrimSpace(constraints)
	t.Scope = strings.TrimSpace(scope)
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
}

// runningWithSteer 返回运行中且有待注入人工舵向的意图 ID（编排器据此中止重跑）。
func (t *Task) runningWithSteer() []int {
	t.mu.Lock()
	defer t.mu.Unlock()
	var ids []int
	for i := range t.Intents {
		if t.Intents[i].State == IntentRunning && t.Intents[i].Steer != "" {
			ids = append(ids, t.Intents[i].ID)
		}
	}
	return ids
}

// roundsSnapshot 当前已用规划轮次。
func (t *Task) roundsSnapshot() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.RoundsUsed
}

// bumpRound 占用一个规划轮次，返回新轮次号。
func (t *Task) bumpRound() int {
	t.mu.Lock()
	t.RoundsUsed++
	r := t.RoundsUsed
	onCh := t.onCh
	t.mu.Unlock()
	t.persist()
	if onCh != nil {
		onCh()
	}
	return r
}

// busyCount 运行中意图数。
func (t *Task) busyCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, it := range t.Intents {
		if it.State == IntentRunning {
			n++
		}
	}
	return n
}

// openCount 待执行意图数。
func (t *Task) openCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, it := range t.Intents {
		if it.State == IntentOpen {
			n++
		}
	}
	return n
}

// —— 探索图边 / 用量 / 人工控制 ——

// addEdgeLocked 追加一条探索图边（已持有 mu；按 三元组 去重）。
func (t *Task) addEdgeLocked(from, to, rel string) {
	for _, e := range t.Edges {
		if e.From == from && e.To == to && e.Rel == rel {
			return
		}
	}
	t.Edges = append(t.Edges, Edge{ID: len(t.Edges) + 1, From: from, To: to, Rel: rel})
}

// AddUsage 累计 token 用量（按天聚合；只落盘不唤醒编排器，避免 LLM 调用节奏带起事件循环）。
func (t *Task) AddUsage(in, out int64) {
	t.mu.Lock()
	t.Usage.In += in
	t.Usage.Out += out
	if in+out > 0 {
		day := time.Now().Format("2006-01-02")
		for i := range t.UsageByDay {
			if t.UsageByDay[i].Day == day {
				t.UsageByDay[i].Usage.In += in
				t.UsageByDay[i].Usage.Out += out
				goto done
			}
		}
		t.UsageByDay = append(t.UsageByDay, DayUsage{Day: day, Usage: UsageTotal{In: in, Out: out}})
	}
done:
	t.mu.Unlock()
	t.persist()
}

// Pause 暂停任务（编排器事件循环在 dispatch 前检查）。
func (t *Task) Pause() {
	t.mu.Lock()
	if !t.Paused {
		t.Paused = true
		t.mu.Unlock()
		t.Log("system", "pause", "任务已暂停（人工）：worker 不再领取新意图，已在跑的回合会走完")
		return
	}
	t.mu.Unlock()
}

// Resume 恢复任务。
func (t *Task) Resume() {
	t.mu.Lock()
	if t.Paused {
		t.Paused = false
		t.mu.Unlock()
		t.Log("system", "resume", "任务已恢复（人工）")
		return
	}
	t.mu.Unlock()
}

// SteerIntent 向意图注入人工舵向：下一次 worker 回合开始时注入提示并清空。
func (t *Task) SteerIntent(id int, msg string) error {
	t.mu.Lock()
	found := false
	for i := range t.Intents {
		if t.Intents[i].ID == id {
			t.Intents[i].Steer = msg
			found = true
			break
		}
	}
	t.mu.Unlock()
	if !found {
		return errUnknownIntent(id)
	}
	t.appendIntentTrace(id, "[人工舵向] "+msg)
	t.Log("system", "steer", fmt.Sprintf("意图 #%d 已注入人工舵向: %s", id, truncateRune(msg, 80)))
	return nil
}

// takeSteer 取走意图的舵向（worker 回合开始时消费，一次性）。
func (t *Task) takeSteer(id int) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.Intents {
		if t.Intents[i].ID == id && t.Intents[i].Steer != "" {
			s := t.Intents[i].Steer
			t.Intents[i].Steer = ""
			return s
		}
	}
	return ""
}

// RerunIntent 重跑一条终态意图（blocked/exhausted/stopped → open，人工复活）。
func (t *Task) RerunIntent(id int) error {
	t.mu.Lock()
	found := false
	for i := range t.Intents {
		if t.Intents[i].ID == id {
			if t.Intents[i].State != IntentOpen && t.Intents[i].State != IntentRunning {
				t.Intents[i].State = IntentOpen
				t.Intents[i].Worker = ""
				t.Intents[i].UpdatedAt = t.now()
				found = true
			}
			break
		}
	}
	onCh := t.onCh
	t.mu.Unlock()
	if !found {
		return errUnknownIntent(id)
	}
	t.appendIntentTrace(id, "[人工重跑] 重置为 open")
	t.Log("system", "rerun", fmt.Sprintf("意图 #%d 已人工重跑", id))
	t.persist()
	if onCh != nil {
		onCh()
	}
	return nil
}

type unknownIntentErr struct{ id int }

func (e unknownIntentErr) Error() string { return "意图 ID 不存在: " + itoa(e.id) }

func errUnknownIntent(id int) error { return unknownIntentErr{id: id} }

// —— 持久化 ——

// persist 快照写盘（忽略错误：持久化是尽力而为，运行内存态才是权威）。
func (t *Task) persist() {
	if t.dir == "" {
		return
	}
	t.mu.Lock()
	data, err := json.MarshalIndent(t, "", "  ")
	t.mu.Unlock()
	if err != nil {
		return
	}
	_ = os.MkdirAll(t.dir, 0o755)
	tmp := filepath.Join(t.dir, "task.json.tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(t.dir, "task.json"))
}

// SaveNow 显式落盘（终态后保证最新快照在盘上）。
func (t *Task) SaveNow() { t.persist() }

// LoadTask 从 dir/task.json 恢复黑板（对话会话黑榜复用 / 任务复盘用）。
func LoadTask(dir string) (*Task, error) {
	data, err := os.ReadFile(filepath.Join(dir, "task.json"))
	if err != nil {
		return nil, err
	}
	var t Task
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	t.dir = dir
	return &t, nil
}
