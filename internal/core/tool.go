// 核心抽象：赛题类型、工具接口、参数声明、结果模型。
// 与 legacy/app/core/tool.py 的契约一一对应，工具插件必须遵守此接口约定。
package core

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Category 赛题类型。
type Category string

const (
	CatWeb        Category = "web"
	CatPwn        Category = "pwn"
	CatReverse    Category = "reverse"
	CatCrypto     Category = "crypto"
	CatMisc       Category = "misc"
	CatForensics  Category = "forensics"
	CatMobile     Category = "mobile"
	CatBlockchain Category = "blockchain"
	CatOsint      Category = "osint"
	CatAI         Category = "ai"
)

// ParamSpec 工具参数的声明式描述，用于自动校验。
type ParamSpec struct {
	Name        string
	Type        string // str | int | float | bool | text | file | password（web.jndi 使用，按 str 处理）
	Required    bool
	Default     any
	Description string
}

// Status 工具执行状态。
type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusSuccess Status = "success"
	StatusError   Status = "error"
	StatusTimeout Status = "timeout"
)

// Artifact 工具产生的文件产物。
type Artifact struct {
	Name        string
	Path        string
	Size        int64
	ContentType string
}

// ToDict 对应 Python Artifact 的序列化（不含 path）。
func (a Artifact) ToDict() map[string]any {
	return map[string]any{
		"name":         a.Name,
		"size":         a.Size,
		"content_type": a.ContentType,
	}
}

// ToolResult 统一的工具执行结果。
type ToolResult struct {
	Status    Status
	ExitCode  int
	Stdout    string
	Stderr    string
	Duration  float64
	Artifacts []Artifact
	Data      map[string]any
}

// ToDict 对应 Python ToolResult.to_dict()。
func (r *ToolResult) ToDict() map[string]any {
	arts := make([]map[string]any, 0, len(r.Artifacts))
	for _, a := range r.Artifacts {
		arts = append(arts, a.ToDict())
	}
	return map[string]any{
		"status":    string(r.Status),
		"exit_code": r.ExitCode,
		"stdout":    r.Stdout,
		"stderr":    r.Stderr,
		"duration":  math.Round(r.Duration*1000) / 1000,
		"artifacts": arts,
		"data":      r.Data,
	}
}

// ToolContext 工具运行时上下文：执行器、工作目录、产物目录、交互会话注册表。
type ToolContext struct {
	Executor     Executor
	Workdir      string
	ArtifactsDir string
	Sessions     *SessionRegistry

	// ReportProgress 报告任务运行期进度（percent 0-100，message 人类可读描述）。
	// 由 TaskManager 注入并同步到 Job（前端轮询 GetJob 可见，长任务用于判断
	// 是否仍在运行）。工具侧调用前自行判 nil；未报告的工具进度字段为零值。
	ReportProgress func(percent int, message string)
}

// Tool 工具接口。命令型实现 BuildCommand；代码型通过 Runner 字段覆盖 run。
type Tool interface {
	ID() string
	Name() string
	Category() Category
	Version() string
	Description() string
	Params() []ParamSpec
	Timeout() int
	ValidateParams(params map[string]any) (map[string]any, error)
	BuildCommand(params map[string]any) ([]string, error)
	Run(ctx context.Context, params map[string]any, tc *ToolContext) (*ToolResult, error)
}

// BaseTool 工具基类：提供默认校验、默认执行流程。代码型工具设置 Runner。
type BaseTool struct {
	NameAttr        string
	CategoryAttr    Category
	VersionAttr     string
	DescriptionAttr string
	ParamsAttr      []ParamSpec
	TimeoutAttr     int
	// Runner 非 nil 时直接执行（代码型）；nil 时走 BuildCommand + Executor（命令型）。
	Runner func(ctx context.Context, params map[string]any, tc *ToolContext) (*ToolResult, error)
}

// ID 对应 Python tool_id：category.name。
func (b *BaseTool) ID() string { return string(b.CategoryAttr) + "." + b.NameAttr }

func (b *BaseTool) Name() string        { return b.NameAttr }
func (b *BaseTool) Category() Category  { return b.CategoryAttr }
func (b *BaseTool) Version() string     { return b.VersionAttr }
func (b *BaseTool) Description() string { return b.DescriptionAttr }
func (b *BaseTool) Params() []ParamSpec { return b.ParamsAttr }
func (b *BaseTool) Timeout() int {
	if b.TimeoutAttr <= 0 {
		return 60
	}
	return b.TimeoutAttr
}

// isEmptyValue 对应 Python `value in (None, "")` 的必填判定。
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok && s == "" {
		return true
	}
	return false
}

// ValidateParams 基于 ParamSpec 做必填校验与默认值填充。
// 与 Python 一致：缺失键取默认值；仅跳过 nil 值（空字符串保留）；错误文案逐字一致。
func (b *BaseTool) ValidateParams(params map[string]any) (map[string]any, error) {
	validated := map[string]any{}
	for _, spec := range b.ParamsAttr {
		value, ok := params[spec.Name]
		if !ok {
			value = spec.Default
		}
		if spec.Required && isEmptyValue(value) {
			return nil, fmt.Errorf("缺少必需参数: %s", spec.Name)
		}
		if value != nil {
			validated[spec.Name] = value
		}
	}
	return validated, nil
}

// BuildCommand 命令型工具重写；默认报错（文案与 Python NotImplementedError 一致）。
func (b *BaseTool) BuildCommand(params map[string]any) ([]string, error) {
	return nil, fmt.Errorf("工具 %s 既未实现 build_command 也未覆盖 run", b.ID())
}

// Run 默认执行流程：命令型（BuildCommand + Executor）；Runner 非 nil 时走代码型。
// ⚠️ Go 嵌入不晚绑定：命令型工具若只在外层类型覆盖 BuildCommand，本方法内部
// b.BuildCommand 仍命中 BaseTool 的默认实现（报「既未实现 build_command」）。
// 命令型工具必须像 FscanTool 一样在外层显式覆写 Run
// （见 internal/tools/pentest/fscan.go，含回归测试 TestFscanRunEndToEnd）。
func (b *BaseTool) Run(ctx context.Context, params map[string]any, tc *ToolContext) (*ToolResult, error) {
	if b.Runner != nil {
		return b.Runner(ctx, params, tc)
	}
	cmd, err := b.BuildCommand(params)
	if err != nil {
		return nil, err
	}
	return tc.Executor.Run(ctx, cmd, tc.Workdir, time.Duration(b.Timeout())*time.Second, nil)
}
