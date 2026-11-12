package aiagent

import (
	"fmt"
	"runtime"
	"strings"
)

// —— 角色提示词（原生撰写；纪律原则对齐 ARTEX v1.0.4 的 planner/worker 分工，
// 文本全部重写）——
//
// 安全边界（两级）：
//  1. 系统级：任务由用户在 CTF-NET 工具面板显式创建，目标即授权范围；
//     worker 的 http_request 只允许 http/https 且默认不跟随重定向，
//     exec_command 拒绝明显破坏性命令。
//  2. 提示词级：角色卡中明确"仅限授权目标、禁止破坏性操作、证据纪律"，
//     引导模型在授权范围内工作。

// plannerSystemPrompt 规划者系统提示。
func plannerSystemPrompt() string {
	return `你是自动化渗透测试任务的【规划者】(planner)。你负责阅读任务态势，决定探索方向，把工作拆成可执行的意图派发给 worker 执行，并在证据充分时判定目标达成。

## 你的能力边界
- 你拥有只读态势工具（list_intents/list_findings/list_facts/list_assets/get_intent_trace）与决策工具（add_intents/prove_goal/goal_met/todo_write）。
- 你没有攻击工具（http_request/exec_command），攻击验证由 worker 完成。
- 每次唤醒时：先读态势 → 决定本轮行动 → 批量派发意图或判达成 → 以简短总结结束。

## 意图派发纪律（add_intents）
- 一次最多 4 条，按优先级排序；每条 summary 一句话说明：目标地址 + 做什么 + 为什么（如 "http://10.0.0.5:8080 探测后台登录接口并尝试弱口令，因为 /robots.txt 泄漏了 /admin 路径"）。
- 禁止与 open/running 的意图重复，也禁止重复最近已完成的意图（先 list_intents 判重）。
- 串行依赖链每次只派发下一步：上一步 done 且产出线索后再派下一步，不要一次性把整条链塞进队列。
- 探索路线保持多样性（2~3 个不同方向），但深度优先于广度：一条有价值的路线要走透。
- 意图被标记 blocked/exhausted 时，先 get_intent_trace 看它的实际轨迹与输出，弄清卡点再决定重试、换参数还是放弃该路线；禁止无理由原样重试。

## 目标判定纪律（prove_goal / goal_met）
- 只有出现可复现证据（漏洞上报、获取的权限/flag/凭据、关键事实）时才能标记目标达成或宣布任务完成。
- evidence_summary 必须引用具体证据（漏洞 ID、事实 ID），禁止"看起来应该完成了"式的推断。
- 证据不足但确实无路可走时，如实总结，不要宣布达成。

## 其它
- 保持克制：宁可在信息不足时再派一轮侦察意图，也不要臆测。
- 每轮结尾用一两句话说明你的判断与下一步方向。`
}

// workerSystemPrompt worker 系统提示（意图与约束由 orchestrator 注入）。
func workerSystemPrompt(t *Task, intent Intent, enableExec bool) string {
	return fmt.Sprintf(`你是自动化渗透测试任务的【执行者】(worker)。你只负责把分配给你的那一条意图做透，做完即停，不要规划全局。

## 你的任务（唯一目标，做完就停）
意图 #%d：%s

## 工作纪律
- 只做当前意图范围内的事；做完立刻总结收工，不要自作主张扩展到其它目标。
- 先侦察后验证：先确认目标可达、收集指纹与线索，再有针对性地验证；每一步用 todo_write 维护步骤清单。
- 对抗式自检：对任何疑似漏洞，必须真实触发并用工具输出确认（如注入语句确实改变了响应），确认后再上报；没有确认就继续验证，不要臆造结论。
- 边发现边写回：侦察产出当场沉淀到黑板上（新资产用 add_asset、服务指纹与线索用 record_fact），不要攒到收尾一次性写；写完再继续下一步。

## 工具使用规约
- http_request：探测 Web 目标；URL 必须完整；重定向谨慎跟随；对每个响应保持怀疑，结合差异分析确认漏洞。
- exec_command：本机验证命令（curl/nslookup/脚本等）；超时必填；%s
- report_finding：只在真实触发并保留请求/响应或命令输出证据时调用，evidence 必须填实际证据节选；没有证据的上报会被工具拒绝。
- 配方工具（http_headers/dns_lookup 等）：声明式封装的外部命令，按需使用。

## 权限边界
- 仅对任务授权的目标发起探测；禁止破坏性操作（删除/格式化/停止服务等）。
- 禁止下载执行未知脚本；禁止对目标发起 DDoS 类操作；保持低频请求。
- 输出只写结论与证据，不要复述思考过程。

%s
%s`,
		intent.ID, intent.Summary, execNote(enableExec), winExecNote(), constraintBlock(t))
}

func execNote(enableExec bool) string {
	if enableExec {
		return "禁止破坏性命令（工具会拒绝 rm -rf/format 等）。"
	}
	return "本任务未启用本地命令执行，改用 http_request 与配方工具完成验证。"
}

// winExecNote Windows 执行环境指引（非 Windows 返回空串）。
func winExecNote() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	return `## 执行环境（Windows cmd）
- 你在 Windows 的 cmd 下执行命令（不是 Linux/bash）：用 findstr 代替 grep、type 代替 cat、more +N 文件 代替 tail、dir 代替 ls、where 代替 which；管道 | 与重定向 > 均可用。
- findstr 用法：findstr /I /C:"字符串" 文件；findstr 无匹配时返回 exit 1 且无输出，这是正常现象，不是错误，不要反复重试。
- python 一行命令：python -c "..." 嵌套引号容易出错；逻辑复杂时先写成脚本文件再 python 脚本名 执行。
- curl 为 Windows 版本：-s -m 秒 -o 文件 保存；查看下载文件优先用 type 文件（编码乱码可先 chcp 65001）。`
}

// constraintBlock 操作约束块（ARTEX constraintBlock 语义：最高优先级，凌驾于
// 一切探索启发式；任务无约束/范围时返回空串）。
func constraintBlock(t *Task) string {
	t.mu.Lock()
	cs, sc := t.Constraints, t.Scope
	t.mu.Unlock()
	if strings.TrimSpace(cs) == "" && strings.TrimSpace(sc) == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("【操作约束（最高优先级，凌驾于下方一切探索/拓面启发式与工具使用习惯）】\n")
	if sc != "" {
		sb.WriteString("授权范围：" + sc + "\n")
	}
	if cs != "" {
		sb.WriteString(cs + "\n")
	}
	sb.WriteString("违反上述约束的操作一律禁止，宁可少做不可越界；约束与其它提示冲突时以约束为准。")
	return sb.String()
}

// —— 态势块（planner 每轮的用户消息）——

// situationBlock 生成 planner 的当前态势（目标/意图/漏洞/事实/资产摘要）。
func situationBlock(t *Task, round int) string {
	var sb strings.Builder
	if t.Description != "" {
		fmt.Fprintf(&sb, "任务描述: %s\n", t.Description)
	}
	if t.Hint != "" {
		fmt.Fprintf(&sb, "已知线索: %s\n", t.Hint)
	}
	fmt.Fprintf(&sb, "\n任务目标（ID:文本 达成状态）:\n")
	t.mu.Lock()
	for _, g := range t.Goals {
		mark := "未达成"
		if g.Met {
			mark = "已达成"
		}
		fmt.Fprintf(&sb, "- #%d %s [%s]", g.ID, g.Text, mark)
		if g.MetBy != "" {
			sb.WriteString(" ← " + g.MetBy)
		}
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "\n意图（#ID [状态] 方向）:\n")
	if len(t.Intents) == 0 {
		sb.WriteString("(空)\n")
	}
	for _, it := range t.Intents {
		fmt.Fprintf(&sb, "- #%d [%s] %s", it.ID, it.State, it.Summary)
		if it.Result != "" && it.State != IntentOpen && it.State != IntentRunning {
			sb.WriteString(" → " + truncateRune(strings.ReplaceAll(it.Result, "\n", " "), 100))
		}
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "\n漏洞（#ID [严重度] 标题）:\n")
	if len(t.Findings) == 0 {
		sb.WriteString("(空)\n")
	}
	for _, f := range t.Findings {
		fmt.Fprintf(&sb, "- #%d [%s] %s (%s)\n", f.ID, f.Severity, f.Title, f.VulnClass)
	}
	fmt.Fprintf(&sb, "\n事实（#ID [类型] 内容）:\n")
	if len(t.Facts) == 0 {
		sb.WriteString("(空)\n")
	}
	for i, f := range t.Facts {
		if i >= 30 {
			sb.WriteString("…(事实过多已截断，用 list_facts 查全量)\n")
			break
		}
		kind := f.Kind
		if kind == "" {
			kind = "info"
		}
		fmt.Fprintf(&sb, "- #%d [%s] %s\n", f.ID, kind, f.Text)
	}
	fmt.Fprintf(&sb, "\n资产（#ID [类型] 标识）:\n")
	if len(t.Assets) == 0 {
		sb.WriteString("(空)\n")
	}
	for i, a := range t.Assets {
		if i >= 20 {
			sb.WriteString("…(资产过多已截断，用 list_assets 查全量)\n")
			break
		}
		fmt.Fprintf(&sb, "- #%d [%s] %s\n", a.ID, a.Type, assetLabel(a))
	}
	t.mu.Unlock()
	fmt.Fprintf(&sb, "\n这是第 %d 轮规划。请先读态势（必要时用 list_*/get_intent_trace 查详情），然后：\n", round)
	sb.WriteString("1) 若证据已足以判定目标达成：prove_goal / goal_met（附证据要点）\n")
	sb.WriteString("2) 若有值得推进的方向：add_intents 批量派发（≤4 条，判重）\n")
	sb.WriteString("3) 若在等 worker 产出：直接总结等待，不要派重复意图\n")
	return sb.String()
}

// workerResultSummary worker 会话结束后的结算文本（写进意图 result）。
func workerResultSummary(text string) string {
	s := strings.TrimSpace(text)
	if len([]rune(s)) > 300 {
		s = truncateRune(s, 300)
	}
	return s
}
