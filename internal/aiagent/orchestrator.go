package aiagent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// —— 任务编排器：状态机 + 意图队列 + 并发调度 + 再规划触发 + 终态判定 ——
//
// 生命周期：
//
//	created → running
//	  ├─ 初轮规划派发意图
//	  └─ 事件循环（黑板变更唤醒）：
//	       ├─ 空闲 worker 槽位 → 领取 open 意图并发执行
//	       ├─ 全部空闲且无 open 意图 → 还有规划预算？再规划一轮 : 意图耗尽
//	       └─ 墙钟超时 / 上下文取消 → timeout / stopped
//	终态：done（prove_goal/goal_met）| exhausted | failed | timeout | stopped

// Orchestrator 编排器。
type Orchestrator struct {
	cfg       Config
	llm       *LLM
	task      *Task
	workerReg *registry
	execFunc  ExecFunc
	progress  func(percent int, message string)
	maxTokens int

	runCtx context.Context // 任务级上下文（worker 会话继承，取消时终止在途 LLM 调用）

	mu            sync.Mutex
	intentCancels map[int]context.CancelFunc // 意图级取消（暂停/人工舵向中止在途回合）
}

// NewOrchestrator 构建编排器。recipesDir 为用户配方目录（可为空）；exec 为本地命令
// 执行函数（nil 时 worker 不注册 exec_command/配方执行能力）。
func NewOrchestrator(cfg Config, proxyURL, recipesDir string, exec ExecFunc) (*Orchestrator, error) {
	cfg = cfg.normalize()
	llm := NewLLM(cfg, proxyURL)
	reg := workerTools(proxyURL, cfg.EnableExec)
	recipeTools, err := LoadRecipes(recipesDir)
	if err != nil {
		return nil, err
	}
	if err := registerRecipes(reg, recipeTools); err != nil {
		return nil, err
	}
	return &Orchestrator{cfg: cfg, llm: llm, workerReg: reg, execFunc: exec,
		intentCancels: map[int]context.CancelFunc{}}, nil
}

// SetProgress 注入进度回调（percent 0-100）。
func (o *Orchestrator) SetProgress(fn func(percent int, message string)) { o.progress = fn }

func (o *Orchestrator) report() {
	if o.progress == nil {
		return
	}
	t := o.task
	t.mu.Lock()
	done, total := 0, 0
	for _, it := range t.Intents {
		if it.State != IntentOpen && it.State != IntentRunning {
			done++
		}
		total++
	}
	status := t.Status
	t.mu.Unlock()
	pct := 0
	if total > 0 {
		pct = done * 90 / total
	}
	msg := fmt.Sprintf("意图进度 %d/%d", done, total)
	switch status {
	case StatusDone, StatusExhausted, StatusFailed, StatusTimeout, StatusStopped:
		pct, msg = 100, "任务结束: "+status
	case StatusCreated, StatusRunning:
	}
	o.progress(pct, msg)
}

// Run 执行任务直到终态。返回错误（致命错误才返回；正常终态返回 nil，终态在 task.Status）。
func (o *Orchestrator) Run(ctx context.Context, task *Task) error {
	o.task = task
	o.runCtx = ctx
	if o.llm.APIKey() == "" {
		task.FinishFailed("未配置 LLM API Key：请在设置页 → AI 配置 填写")
		o.report()
		return fmt.Errorf("未配置 LLM API Key")
	}
	task.MarkRunning()
	task.Log("system", "terminal", fmt.Sprintf("任务开始：%s（目标：%s）", task.Description, task.GoalText))

	// goals 角色：目标分解 + 约束提取（失败保留原始目标，不阻塞任务）
	o.decomposeGoals(ctx, task)

	// 唤醒通道：黑板任何变更都会 ping（丢失可合并，事件循环自会重查状态）
	wake := make(chan struct{}, 1)
	task.setOnChange(func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	})
	defer task.setOnChange(nil)

	// 初轮规划
	if err := o.startPlanRound(ctx); err != nil {
		o.abort("初轮规划失败", err)
		o.report()
		return err
	}

	// 事件循环
	var wg sync.WaitGroup
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeoutAt := time.Time{}
	if task.TimeoutSec > 0 {
		timeoutAt = time.Now().Add(time.Duration(task.TimeoutSec) * time.Second)
	}
	pauseAt := time.Time{} // 人工暂停起点（恢复时顺延墙钟）

	loop := func() error {
		status := task.StatusNow()
		if isTerminal(status) {
			return errDone
		}
		// 人工暂停：中止在途 worker 回合（意图回 open 待恢复重领），恢复时墙钟顺延
		if task.Paused {
			if pauseAt.IsZero() {
				pauseAt = time.Now()
			}
			o.cancelInFlight()
			return nil
		}
		if !pauseAt.IsZero() {
			if !timeoutAt.IsZero() {
				timeoutAt = timeoutAt.Add(time.Since(pauseAt))
			}
			pauseAt = time.Time{}
		}
		// 人工舵向：中止对应运行中意图的回合，worker 重领后以舵向注入开场
		o.cancelSteered()
		// 墙钟超时
		if !timeoutAt.IsZero() && time.Now().After(timeoutAt) {
			task.FinishTimeout(fmt.Sprintf("超过任务时限 %d 秒", task.TimeoutSec))
			return errDone
		}
		// 调度：填满 worker 槽位
		o.dispatch(&wg)
		// 全部空闲且无 open 意图 → 决定下一步
		if task.busyCount() == 0 && task.openCount() == 0 {
			if task.allGoalsMet() {
				task.FinishDone("全部目标已达成")
				return errDone
			}
			if task.RoundsUsed >= task.PlanRounds {
				task.FinishExhausted("意图耗尽且规划轮次预算用完，目标未达成")
				return errDone
			}
			if err := o.startPlanRound(ctx); err != nil {
				o.abort("再规划失败", err)
				return err
			}
			// 新轮无产出且仍无工作可做 → 耗尽
			if task.busyCount() == 0 && task.openCount() == 0 {
				task.FinishExhausted("规划者没有派发新意图，目标未达成")
				return errDone
			}
			o.dispatch(&wg)
		}
		return nil
	}

	var runErr error
	for runErr == nil {
		if err := loop(); err != nil {
			runErr = err
			break
		}
		o.report()
		select {
		case <-ctx.Done():
			task.FinishStopped("用户取消任务")
			runErr = errDone
		case <-wake:
		case <-ticker.C:
		}
	}
	wg.Wait()
	// 兜底：仍非终态（例如 ctx 取消后）时按目标达成情况收尾
	if !isTerminal(task.StatusNow()) {
		if ctx.Err() != nil {
			task.FinishStopped("用户取消任务")
		} else if task.allGoalsMet() {
			task.FinishDone("全部目标已达成")
		} else {
			task.FinishExhausted("执行结束，目标未达成")
		}
	}
	task.Log("system", "terminal", "任务终态: "+task.StatusNow())
	task.SaveNow()
	o.report()
	return nil
}

var errDone = fmt.Errorf("任务已结束")

func isTerminal(s string) bool {
	switch s {
	case StatusDone, StatusExhausted, StatusFailed, StatusTimeout, StatusStopped:
		return true
	}
	return false
}

// abort 记录致命错误并落终态；上下文已取消时按用户取消落 stopped（而非 failed）。
func (o *Orchestrator) abort(stage string, err error) {
	if o.runCtx != nil && o.runCtx.Err() != nil {
		o.task.Log("system", "terminal", "任务取消: "+stage)
		o.task.FinishStopped("用户取消任务")
		return
	}
	o.task.Log("system", "terminal", stage+": "+err.Error())
	o.task.FinishFailed(stage + ": " + err.Error())
}

// decomposeGoals 任务开始时运行 goals 角色：分解子目标 + 提取约束（失败保留原始目标）。
func (o *Orchestrator) decomposeGoals(ctx context.Context, task *Task) {
	goals, constraints, scope, err := DecomposeGoals(ctx, o.llm, task.Description, task.GoalText, task.Hint)
	if err != nil || len(goals) == 0 {
		task.Log("system", "goals", "目标分解失败（保留原始目标继续）: "+errText(err))
		return
	}
	task.SetGoals(goals, constraints, scope)
	task.Log("system", "goals", fmt.Sprintf("目标分解为 %d 条子目标（约束 %d 字，范围 %d 字）",
		len(goals), len([]rune(constraints)), len([]rune(scope))))
}

func errText(err error) string {
	if err == nil {
		return "结果为空"
	}
	return err.Error()
}

// cancelIntent 取消指定意图的在途回合（幂等）。
func (o *Orchestrator) cancelIntent(id int, why string) {
	o.mu.Lock()
	c := o.intentCancels[id]
	o.mu.Unlock()
	if c != nil {
		c()
		o.task.Log("system", "interrupt", fmt.Sprintf("中止意图 #%d 的在途回合：%s", id, why))
	}
}

// cancelSteered 中止所有带待注入舵向的运行中意图（worker 重领后以舵向开场）。
func (o *Orchestrator) cancelSteered() {
	for _, id := range o.task.runningWithSteer() {
		o.cancelIntent(id, "人工舵向")
	}
}

// cancelInFlight 中止全部在途 worker 回合（暂停语义）。
func (o *Orchestrator) cancelInFlight() {
	o.mu.Lock()
	ids := make([]int, 0, len(o.intentCancels))
	for id := range o.intentCancels {
		ids = append(ids, id)
	}
	o.mu.Unlock()
	for _, id := range ids {
		o.cancelIntent(id, "任务暂停")
	}
}

// startPlanRound 占用一个规划轮次并运行 planner。
func (o *Orchestrator) startPlanRound(ctx context.Context) error {
	round := o.task.bumpRound()
	added, summary, err := planRound(ctx, o.llm, o.task, round, o.cfg.MaxTurns, o.maxTokens)
	if err != nil {
		return err
	}
	o.task.Log("planner", "plan", "规划总结: "+truncateRune(summary, 200))
	_ = added
	return nil
}

// dispatch 把 open 意图填满空闲 worker 槽位（并发上限 MaxWorkers）。
func (o *Orchestrator) dispatch(wg *sync.WaitGroup) {
	for o.task.busyCount() < o.cfg.MaxWorkers {
		intent := o.task.claimIntent(fmt.Sprintf("worker-%d", o.task.busyCount()+1))
		if intent == nil {
			return
		}
		it := *intent
		o.task.Log("system", "dispatch", fmt.Sprintf("worker 领取意图 #%d: %s", it.ID, it.Summary))
		wg.Add(1)
		go func() {
			defer wg.Done()
			o.runWorker(it)
		}()
	}
}

// runWorker 执行单条意图并结算。
func (o *Orchestrator) runWorker(intent Intent) {
	// 意图级上下文：暂停/人工舵向只中止本意图的回合（ARTEX 命名 AbortCause 语义）
	ctx, cancel := context.WithCancel(o.runCtx)
	o.mu.Lock()
	o.intentCancels[intent.ID] = cancel
	o.mu.Unlock()
	defer func() {
		o.mu.Lock()
		delete(o.intentCancels, intent.ID)
		o.mu.Unlock()
		cancel()
	}()
	tc := &RunContext{
		Task:    o.task,
		WorkDir: o.task.dir,
		Intent:  &intent,
		Agent:   "worker",
		Exec:    o.execFunc,
	}
	snapshot := o.task.Snapshot()
	user := fmt.Sprintf("开始执行意图 #%d：%s\n完成任务后请总结：验证了什么、发现了什么、结论是什么。", intent.ID, intent.Summary)
	if steer := o.task.takeSteer(intent.ID); steer != "" {
		user += fmt.Sprintf("\n\n【人工舵向（最高优先级，必须遵循）】%s", steer)
	}
	res, err := runSession(ctx, o.llm, o.workerReg, tc,
		workerSystemPrompt(o.task, intent, o.cfg.EnableExec), user, o.cfg.MaxTurns, o.maxTokens)
	if err != nil {
		// 中止（暂停/舵向）：意图回 open 待重领，不算失败（ARTEX paused 状态映射）
		if errors.Is(err, context.Canceled) && o.task.StatusNow() == StatusRunning {
			o.task.settleIntent(intent.ID, IntentOpen, "")
			o.task.Log("worker", "settle", fmt.Sprintf("意图 #%d 回合被中止（暂停/舵向），回到待执行", intent.ID))
			o.report()
			return
		}
		o.task.settleIntent(intent.ID, IntentBlocked, "执行出错: "+err.Error())
		o.task.Log("worker", "settle", fmt.Sprintf("意图 #%d 阻塞: %v", intent.ID, err))
		o.report()
		return
	}
	// 结算：会话期间是否有黑板上写（漏洞/事实/资产）→ done；否则 exhausted
	wrote := len(o.task.Snapshot().Findings)+len(o.task.Snapshot().Facts)+len(o.task.Snapshot().Assets) >
		len(snapshot.Findings)+len(snapshot.Facts)+len(snapshot.Assets)
	state := IntentDone
	if !wrote && res.TurnLimit {
		state = IntentExhausted
	}
	o.task.settleIntent(intent.ID, state, workerResultSummary(res.Text))
	o.task.Log("worker", "settle", fmt.Sprintf("意图 #%d 完成[%s]（工具调用 %d 次）", intent.ID, state, res.ToolCalls))
	o.report()
}
