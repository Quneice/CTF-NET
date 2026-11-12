package aiagent

import (
	"context"
	"fmt"
)

// planRound 执行一轮规划：planner 会话读态势、派意图或判达成。
// 返回：派发意图数、会话总结、错误。
func planRound(ctx context.Context, llm *LLM, task *Task, round int, maxTurns int, maxTokens int) (int, string, error) {
	reg := plannerTools()
	tc := &RunContext{Task: task, WorkDir: task.dir, Agent: "planner"}
	before := intentCount(task)
	task.Log("planner", "plan", fmt.Sprintf("第 %d 轮规划开始", round))
	system := plannerSystemPrompt()
	if cb := constraintBlock(task); cb != "" {
		system += "\n\n" + cb
	}
	res, err := runSession(ctx, llm, reg, tc, system, situationBlock(task, round), maxTurns, maxTokens)
	if err != nil {
		return 0, "", err
	}
	added := intentCount(task) - before
	task.Log("planner", "plan", fmt.Sprintf("第 %d 轮规划结束：新派 %d 条意图，工具调用 %d 次", round, added, res.ToolCalls))
	return added, res.Text, nil
}

func intentCount(t *Task) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.Intents)
}
