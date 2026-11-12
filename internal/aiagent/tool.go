package aiagent

import (
	"context"
	"fmt"
	"sort"
)

// Tool agent 可调用的工具接口。工具实现必须：
//   - 幂等地把副作用写进 Task 黑板上（由 Orchestrator 统一持久化）
//   - 返回给 LLM 的人类可读文本结果（进入下一轮上下文）
//   - 参数非法/执行失败时返回 error，错误文本会作为工具结果回传给 LLM
type Tool interface {
	// Name LLM 侧工具名（如 http_request）。
	Name() string
	// Description 用途说明（进入提示词，影响模型选择工具的准确率）。
	Description() string
	// InputSchema JSON Schema（OpenAI function.parameters 风格）。
	InputSchema() map[string]any
	// Call 执行工具。tc 携带当前任务黑板与运行环境。
	Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error)
}

// RunContext 工具运行环境（Orchestrator 注入）。
type RunContext struct {
	Task    *Task    // 当前任务黑板（意图/资产/事实/漏洞均写在这里）
	WorkDir string   // 任务工作目录（配方与证据文件落这里）
	Intent  *Intent  // 当前 worker 绑定的意图（可为 nil，如 planner 会话）
	Agent   string   // 当前会话角色：planner | worker
	Exec    ExecFunc // 本地命令执行（nil = 未启用 exec_command）
}

// ExecFunc 执行本地命令：dir 为工作目录，timeout 为秒。返回 stdout/stderr。
type ExecFunc func(ctx context.Context, command string, dir string, timeoutSec int) (string, string, error)

// intentIDOf 当前运行上下文绑定的意图 ID（planner 会话等无意图时为 0）。
func intentIDOf(tc *RunContext) int {
	if tc != nil && tc.Intent != nil {
		return tc.Intent.ID
	}
	return 0
}

// registry 工具注册表。
type registry struct {
	tools map[string]Tool
}

func newRegistry() *registry { return &registry{tools: map[string]Tool{}} }

func (r *registry) register(t Tool) { r.tools[t.Name()] = t }

// schemas 输出全部工具的 JSON Schema 声明（名称排序，稳定输出便于测试）。
func (r *registry) schemas() []ToolSchema {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]ToolSchema, 0, len(names))
	for _, n := range names {
		t := r.tools[n]
		out = append(out, ToolSchema{Name: t.Name(), Description: t.Description(), Parameters: t.InputSchema()})
	}
	return out
}

// call 查工具并执行；未知工具返回明确错误（回传 LLM 让它换工具）。
func (r *registry) call(ctx context.Context, name string, rawArgs string, tc *RunContext) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("未知工具 %q", name)
	}
	args, err := parseArgs(rawArgs)
	if err != nil {
		return "", err
	}
	return t.Call(ctx, args, tc)
}

// objSchema 便捷构造：object 类型的 JSON Schema。
// 注意：required/properties 为空时必须省略而非输出 null —— DeepSeek 等严格
// 校验的 OpenAI 兼容服务会以 400 拒绝 "null is not of type array/object"
// （2026-11-12 实测：list_assets 的 required:null 整请求被拒）。
func objSchema(props map[string]any, required []string) map[string]any {
	m := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
	}
	if len(props) > 0 {
		m["properties"] = props
	} else {
		m["properties"] = map[string]any{}
	}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

// prop 便捷构造：单属性定义。
func prop(typ string, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}

func propEnum(typ string, desc string, values []string) map[string]any {
	return map[string]any{"type": typ, "description": desc, "enum": values}
}

// argString 从参数 map 取字符串（缺失/类型不符返回空）。
func argString(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

// argInt 从参数 map 取整数（缺失/类型不符返回 0）。JSON 数字经解析为 float64。
func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return 0
}

// argBool 从参数 map 取布尔。
func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	}
	return false
}
