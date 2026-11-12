package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// —— 角色分配：goals 角色（对齐 ARTEX 的 goals 角色 / set_goals / set_constraints）——
//
// 任务创建时把用户的一句话目标分解为 2~5 条可验证子目标，并从中提取操作约束
// （只允许/禁止、授权范围）——约束以"最高优先级约束块"注入后续 planner/worker
// 提示词。分解失败时调用方保留原始目标（单条）继续执行，不阻塞任务。

// goalsSystemPrompt goals 角色卡。注意：不得出现"规划者"/"执行者"字样
// （fake LLM 测试按其分类会话）。
func goalsSystemPrompt() string {
	return `你是自动化渗透测试任务的【目标分解者】(goals)。任务开始前，你把用户的一句话目标拆成可验证的子目标，并提取操作约束。

## 分解纪律
- 2~5 条子目标，每条一句话：对象 + 动作 + 可验证的成果判据（如"探测 http://10.0.0.5:8080 的登录接口并尝试弱口令，找到有效凭据即达成"）。
- 子目标必须具体可验证：目标地址写实值（IP/域名/端口），判据可被工具输出证明。禁止"全面评估""深入了解"之类不可验证的表述。
- 覆盖用户的原始目标即可，不要擅自扩大范围。

## 约束提取纪律
- 从目标文本与已知线索中提取操作边界：授权范围（哪些资产/网段属于目标）、允许的操作、禁止的操作。
- 必须自包含具体值：IP、域名、端口、路径要写实值；禁止"目标机器""该服务"之类的模糊指代（换一个人只看约束就能执行）。
- 没有明确信息时用常见渗透守则兜底：禁止破坏性操作、禁止 DDoS、保持低频请求、只测授权范围。

## 输出格式
只输出一个 JSON 对象（不要任何其它文字）：
{"goals":[{"text":"子目标一句话"}],"constraints":"允许的操作：…；禁止的操作：…","scope":"授权范围：…"}`
}

// goalsOutput goals 角色输出。
type goalsOutput struct {
	Goals       []struct{ Text string } `json:"goals"`
	Constraints string                  `json:"constraints"`
	Scope       string                  `json:"scope"`
}

// DecomposeGoals 调用 LLM 分解目标。失败返回 error（调用方保留原始目标兜底）。
func DecomposeGoals(ctx context.Context, llm *LLM, description, goalText, hint string) ([]Goal, string, string, error) {
	if strings.TrimSpace(goalText) == "" {
		return nil, "", "", fmt.Errorf("目标为空")
	}
	var sb strings.Builder
	sb.WriteString("任务描述: " + description + "\n")
	sb.WriteString("原始目标: " + goalText + "\n")
	if hint != "" {
		sb.WriteString("已知线索: " + hint + "\n")
	}
	sb.WriteString("\n请输出 JSON。")
	resp, err := llm.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: goalsSystemPrompt()},
			{Role: RoleUser, Content: sb.String()},
		},
		MaxTokens: 1200,
	})
	if err != nil {
		return nil, "", "", err
	}
	var out goalsOutput
	if err := parseJSONObject(resp.Content, &out); err != nil || len(out.Goals) == 0 {
		return nil, "", "", fmt.Errorf("解析分解结果失败: %w", err)
	}
	goals := make([]Goal, 0, len(out.Goals))
	for _, g := range out.Goals {
		if strings.TrimSpace(g.Text) == "" {
			continue
		}
		goals = append(goals, Goal{Text: strings.TrimSpace(g.Text)})
	}
	if len(goals) == 0 {
		return nil, "", "", fmt.Errorf("分解结果无有效子目标")
	}
	return goals, out.Constraints, out.Scope, nil
}

// parseJSONObject 从模型输出中提取首个 JSON 对象并解析（复用 llm.go 的围栏剥离）。
func parseJSONObject(s string, v any) error {
	text := extractJSON(s)
	if text == "" || text[0] != '{' {
		return fmt.Errorf("输出中未找到 JSON 对象")
	}
	return json.Unmarshal([]byte(text), v)
}
