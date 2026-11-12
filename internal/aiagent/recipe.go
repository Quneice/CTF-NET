package aiagent

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// —— YAML 配方工具：把外部命令（curl/nslookup 等）声明成 agent 可调用的工具 ——
//
// 配方格式（与 ARTEX v1.0.4 的工具配方模型一致，本实现为原生重写）：
//
//	version: 1
//	name: http_headers
//	enabled: true
//	command: curl
//	args: ["--head", "--silent", "--max-time", "30"]
//	short_description: 查看响应头
//	description: 详细说明（进入提示词）
//	agents: [worker]
//	timeout_seconds: 30
//	parameters:
//	  - name: url
//	    type: string      # string | integer | float | boolean
//	    flag: --url
//	    required: true
//	    description: 目标 URL
//
// 调用语义：command + args（静态） + 每个已提供参数的 <flag> <value>（boolean 仅当
// true 时附加 flag 本身）。执行走 RunContext.Exec（与 exec_command 同一授权开关）。

//go:embed recipes/*.yaml
var recipeFS embed.FS

// recipeDef YAML 配方定义。
type recipeDef struct {
	Version     int              `yaml:"version"`
	Name        string           `yaml:"name"`
	Enabled     *bool            `yaml:"enabled"`
	Command     string           `yaml:"command"`
	Args        []string         `yaml:"args"`
	ShortDesc   string           `yaml:"short_description"`
	Description string           `yaml:"description"`
	Agents      []string         `yaml:"agents"`
	TimeoutSec  int              `yaml:"timeout_seconds"`
	Parameters  []recipeParamDef `yaml:"parameters"`
}

type recipeParamDef struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Flag        string `yaml:"flag"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description"`
}

// LoadRecipes 从 YAML 目录加载配方（目录不存在时忽略）；嵌入的内置配方始终加载。
func LoadRecipes(dir string) ([]Tool, error) {
	var tools []Tool
	// 内置配方
	entries, err := recipeFS.ReadDir("recipes")
	if err != nil {
		return nil, fmt.Errorf("读取内置配方失败: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		data, err := recipeFS.ReadFile("recipes/" + n)
		if err != nil {
			continue
		}
		rt, err := parseRecipe(data, "内置:"+n)
		if err != nil {
			return nil, err
		}
		tools = append(tools, rt)
	}
	// 用户配方（覆盖内置同名配方）
	if dir != "" {
		userNames, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
		if err == nil {
			sort.Strings(userNames)
			for _, p := range userNames {
				data, err := os.ReadFile(p)
				if err != nil {
					continue
				}
				rt, err := parseRecipe(data, filepath.Base(p))
				if err != nil {
					return nil, fmt.Errorf("配方 %s 解析失败: %w", filepath.Base(p), err)
				}
				// 同名替换
				replaced := false
				for i, t := range tools {
					if t.Name() == rt.Name() {
						tools[i] = rt
						replaced = true
						break
					}
				}
				if !replaced {
					tools = append(tools, rt)
				}
			}
		}
	}
	return tools, nil
}

// parseRecipe 解析单个配方 YAML。
func parseRecipe(data []byte, label string) (*recipeTool, error) {
	var d recipeDef
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("%s: YAML 解析失败: %w", label, err)
	}
	if d.Name == "" || d.Command == "" {
		return nil, fmt.Errorf("%s: name/command 必填", label)
	}
	if d.Enabled != nil && !*d.Enabled {
		return nil, fmt.Errorf("%s: 已禁用", label)
	}
	if d.Description == "" {
		d.Description = d.ShortDesc
	}
	rt := &recipeTool{def: d}
	return rt, nil
}

// recipeTool 配方工具。
type recipeTool struct {
	def recipeDef
}

func (r *recipeTool) Name() string        { return r.def.Name }
func (r *recipeTool) Description() string { return r.def.Description }

func (r *recipeTool) InputSchema() map[string]any {
	props := map[string]any{}
	var required []string
	for _, p := range r.def.Parameters {
		t := p.Type
		if t == "" {
			t = "string"
		}
		props[p.Name] = map[string]any{"type": t, "description": p.Description}
		if p.Required {
			required = append(required, p.Name)
		}
	}
	return objSchema(props, required)
}

func (r *recipeTool) Call(ctx context.Context, args map[string]any, tc *RunContext) (string, error) {
	if tc.Exec == nil {
		return "", fmt.Errorf("配方工具 %s 需要本地命令执行能力（任务未启用）", r.def.Name)
	}
	for _, p := range r.def.Parameters {
		if p.Required && isEmptyVal(args[p.Name]) {
			return "", fmt.Errorf("缺少必需参数: %s", p.Name)
		}
	}
	cmdArgs := append([]string(nil), r.def.Args...)
	for _, p := range r.def.Parameters {
		v, ok := args[p.Name]
		if !ok || isEmptyVal(v) {
			continue
		}
		if p.Flag != "" {
			if strings.Contains(p.Flag, "{value}") {
				// 单 token 形式：flag 内嵌值（如 -type={value}）
				cmdArgs = append(cmdArgs, strings.ReplaceAll(p.Flag, "{value}", fmt.Sprint(v)))
				continue
			}
			cmdArgs = append(cmdArgs, p.Flag)
		}
		switch p.Type {
		case "boolean":
			// flag 已附加，不带值
		default:
			cmdArgs = append(cmdArgs, fmt.Sprint(v))
		}
	}
	timeout := r.def.TimeoutSec
	if timeout <= 0 || timeout > 300 {
		timeout = 60
	}
	command := r.def.Command + " " + strings.Join(cmdArgs, " ")
	stdout, stderr, err := tc.Exec(ctx, command, tc.WorkDir, timeout)
	if err != nil {
		return "", fmt.Errorf("命令失败: %v\n%s%s", err, stdout, stderr)
	}
	var sb strings.Builder
	if stdout != "" {
		sb.WriteString(stdout)
	}
	if stderr != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("[stderr]\n")
		sb.WriteString(stderr)
	}
	if sb.Len() == 0 {
		sb.WriteString("(无输出，命令执行成功)")
	}
	return truncateRune(sb.String(), maxExecOutBytes), nil
}

func isEmptyVal(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok && s == "" {
		return true
	}
	return false
}

// registerRecipes 把配方注册进 registry（与内置工具同名冲突时报错）。
func registerRecipes(r *registry, tools []Tool) error {
	for _, t := range tools {
		if _, exists := r.tools[t.Name()]; exists {
			return fmt.Errorf("配方工具名 %q 与内置工具冲突", t.Name())
		}
		r.register(t)
	}
	return nil
}

// recipeCallLog 配方调用摘要（工具侧日志用）。
func recipeCallLog(name string, args map[string]any) string {
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+strconv.Quote(fmt.Sprint(args[k])))
	}
	return name + "(" + strings.Join(parts, ", ") + ")"
}
