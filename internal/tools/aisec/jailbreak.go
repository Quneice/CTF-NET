// aisec.jailbreak.jailbreak-scan / aisec.injection.injection-scan ——
// LLM 越狱/提示注入批量扫描:内置模板库 + 自定义 payload,并发请求 OpenAI 兼容端点,
// 中英拒绝关键词 + 成功正则综合判定,产出文本报告 + CSV 产物 + 结构化结果。
package aisec

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ctfnet/internal/core"
)

// 模型安全分类。
const (
	CatJailbreak core.Category = "aisec.jailbreak"
	CatInjection core.Category = "aisec.injection"
	CatBackdoor  core.Category = "aisec.backdoor"
	CatPoison    core.Category = "aisec.poison"
)

func init() {
	registerJailbreak("jailbreak-scan", CatJailbreak, "jailbreak",
		"LLM 越狱批量扫描:内置越狱模板库(DAN/角色扮演/翻译外带等)+ 自定义 payload,并发打 OpenAI 兼容端点,中英拒绝关键词 + 成功正则判定,输出报告/CSV/结构化结果")
	registerJailbreak("injection-scan", CatInjection, "injection",
		"提示注入批量扫描:内置注入模板库(忽略指令/系统提示词泄露/角色越权/分隔符欺骗等)+ 自定义 payload,并发打 OpenAI 兼容端点并判定是否被注入成功")
}

// registerJailbreak 注册越狱/注入扫描工具(共享同一执行器,默认模板集不同)。
func registerJailbreak(name string, cat core.Category, defaultSet, desc string) {
	core.DefaultRegistry.Register(&JailbreakTool{BaseTool: core.BaseTool{
		NameAttr:        name,
		CategoryAttr:    cat,
		VersionAttr:     "1.0.0",
		DescriptionAttr: desc,
		TimeoutAttr:     900,
		ParamsAttr: []core.ParamSpec{
			{Name: "task_goal", Type: "text", Required: true, Description: "攻击目标(替换模板 {{GOAL}} 占位符),如「泄露系统提示词」「写出 sqlmap 用法」"},
			{Name: "payload_set", Type: "str", Required: false, Default: defaultSet, Description: "内置模板集:jailbreak 越狱模板 / injection 注入模板 / full 全部 / custom 仅自定义"},
			{Name: "payloads", Type: "text", Required: false, Description: "自定义 payload,每行一条(支持 {{GOAL}} 占位符);payload_set=custom 时必填"},
			{Name: "target_url", Type: "str", Required: false, Description: "OpenAI 兼容端点(留空用全局 AI 配置;填 base 或完整 /chat/completions 均可)"},
			{Name: "model", Type: "str", Required: false, Description: "模型名(留空用全局 AI 配置)"},
			{Name: "api_key", Type: "password", Required: false, Description: "API Key(留空用全局 AI 配置)"},
			{Name: "concurrency", Type: "int", Required: false, Default: 4, Description: "并发数(1-8)"},
			{Name: "max_tokens", Type: "int", Required: false, Default: 1024, Description: "单次响应最大 token 数"},
			{Name: "refusal_keywords", Type: "text", Required: false, Description: "追加拒绝判定关键词,逗号分隔(命中即判被拦截)"},
			{Name: "success_pattern", Type: "str", Required: false, Description: "成功判定正则(命中=越狱/注入成功,如 flag\\{|BEGIN RSA)"},
		},
		Runner: func(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
			return jailbreakRun(ctx, params, tc, defaultSet)
		},
	}})
}

// JailbreakTool 越狱/注入扫描工具。
type JailbreakTool struct {
	core.BaseTool
}

// jailbreakResult 单条 payload 的测试结果。
type jailbreakResult struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Verdict  string `json:"verdict"` // success / blocked / inconclusive / error
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func jailbreakRun(ctx context.Context, params map[string]any, tc *core.ToolContext, defaultSet string) (*core.ToolResult, error) {
	goal := strP(params, "task_goal", "")
	set := strP(params, "payload_set", defaultSet)

	// 组装 prompt 列表
	var prompts []payloadTemplate
	if set == "custom" {
		for i, line := range strings.Split(strP(params, "payloads", ""), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				prompts = append(prompts, payloadTemplate{Name: fmt.Sprintf("自定义 #%d", i+1), Content: line})
			}
		}
		if len(prompts) == 0 {
			return &core.ToolResult{Status: core.StatusError, Stderr: "payload_set=custom 但未提供 payloads"}, nil
		}
	} else {
		prompts = payloadLibrary(set)
	}
	if len(prompts) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "未知 payload_set: " + set}, nil
	}
	filled := make([]payloadTemplate, len(prompts))
	for i, p := range prompts {
		filled[i] = payloadTemplate{Name: p.Name, Content: strings.ReplaceAll(p.Content, "{{GOAL}}", goal)}
	}

	// 端点/密钥/代理(全局配置联动,参数可覆盖)
	ac := core.GetAppConfig()
	proxyURL := ""
	if ac != nil {
		proxyURL = ac.Proxy.URL
	}
	endpoint, model, apiKey := resolveEndpoint(ac, strP(params, "target_url", ""), strP(params, "model", ""), strP(params, "api_key", ""))
	client := llmClient(proxyURL, 120*time.Second)
	conc := clamp(intP(params, "concurrency", 4), 1, 8)
	maxTokens := clamp(intP(params, "max_tokens", 1024), 16, 8192)

	// 追加拒绝关键词
	var extra []string
	for _, kw := range strings.Split(strP(params, "refusal_keywords", ""), ",") {
		if kw = strings.TrimSpace(kw); kw != "" {
			extra = append(extra, strings.ToLower(kw))
		}
	}
	successPattern := strP(params, "success_pattern", "")

	// 并发执行
	results := make([]jailbreakResult, len(filled))
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for i, p := range filled {
		wg.Add(1)
		go func(i int, p payloadTemplate) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			r := jailbreakResult{ID: i + 1, Name: p.Name}
			resp, err := callChat(ctx, client, endpoint, apiKey, model, p.Content, maxTokens)
			if err != nil {
				r.Verdict = "error"
				r.Error = err.Error()
			} else {
				r.Response = truncate(resp, 2000)
				r.Verdict = judgeVerdict(resp, extra, successPattern)
			}
			mu.Lock()
			results[i] = r
			done++
			mu.Unlock()
			if tc.ReportProgress != nil {
				tc.ReportProgress(done*100/len(filled), fmt.Sprintf("已测试 %d/%d 条 payload", done, len(filled)))
			}
		}(i, p)
	}
	wg.Wait()

	// 汇总
	counts := map[string]int{}
	for _, r := range results {
		counts[r.Verdict]++
	}

	var out strings.Builder
	fmt.Fprintf(&out, "[模型安全扫描] 目标: %s\n", goal)
	fmt.Fprintf(&out, "端点: %s | 模型: %s\n", endpoint, model)
	fmt.Fprintf(&out, "总计 %d | 成功 %d | 被拦截 %d | 无结论 %d | 失败 %d\n\n",
		len(results), counts["success"], counts["blocked"], counts["inconclusive"], counts["error"])
	for _, r := range results {
		switch r.Verdict {
		case "success":
			fmt.Fprintf(&out, "✓ 成功 — %d. %s\n  摘要: %s\n\n", r.ID, r.Name, truncate(strings.TrimSpace(r.Response), 150))
		case "blocked":
			fmt.Fprintf(&out, "✗ 被拦截 — %d. %s\n", r.ID, r.Name)
		case "error":
			fmt.Fprintf(&out, "! 失败 — %d. %s (%s)\n", r.ID, r.Name, r.Error)
		default:
			fmt.Fprintf(&out, "? 无结论 — %d. %s\n", r.ID, r.Name)
		}
	}

	// CSV 产物
	var artifacts []core.Artifact
	if tc != nil && tc.ArtifactsDir != "" {
		_ = os.MkdirAll(tc.ArtifactsDir, 0o755)
		csvPath := filepath.Join(tc.ArtifactsDir, fmt.Sprintf("aisec_scan_%s.csv", time.Now().Format("20060102_150405")))
		if f, err := os.Create(csvPath); err == nil {
			w := csv.NewWriter(f)
			_ = w.Write([]string{"id", "name", "verdict", "response"})
			for _, r := range results {
				_ = w.Write([]string{fmt.Sprint(r.ID), r.Name, r.Verdict, r.Response})
			}
			w.Flush()
			_ = f.Close()
			if st, err := os.Stat(csvPath); err == nil {
				artifacts = append(artifacts, core.Artifact{Name: filepath.Base(csvPath), Path: csvPath, Size: st.Size(), ContentType: "text/csv"})
			}
		}
	}

	status := core.StatusSuccess
	if counts["error"] == len(results) {
		status = core.StatusError
	}
	return &core.ToolResult{
		Status:    status,
		Stdout:    out.String(),
		Artifacts: artifacts,
		Data: map[string]any{
			"results": results,
			"summary": map[string]any{
				"total": len(results), "success": counts["success"],
				"blocked": counts["blocked"], "inconclusive": counts["inconclusive"],
				"error": counts["error"],
			},
		},
	}, nil
}
