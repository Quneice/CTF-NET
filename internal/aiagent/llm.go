package aiagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// —— OpenAI 兼容 chat/completions 客户端（DeepSeek / OpenAI / Ollama / 各类中转）——

// Role 消息角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall LLM 发起的工具调用。
type ToolCall struct {
	ID        string // 调用 ID（回传 tool 消息时用）
	Name      string // 工具名
	Arguments string // JSON 参数字符串
}

// Message 一轮对话消息。
type Message struct {
	Role       Role
	Content    string     // 文本内容（tool 角色为工具返回值）
	Reasoning  string     // 思考内容（DeepSeek 思考模式：下一轮请求必须原样回传，否则 400）
	ToolCalls  []ToolCall // assistant 角色可能带工具调用
	ToolCallID string     // tool 角色回填的调用 ID
}

// ToolSchema 工具声明（JSON Schema，OpenAI function 风格）。
type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ChatRequest 一次对话补全请求。
type ChatRequest struct {
	Messages []Message
	Tools    []ToolSchema
	// MaxTokens 回复 token 上限（0 = 服务端默认）。
	MaxTokens int
}

type chatHTTPMessage struct {
	Role             string         `json:"role"`
	Content          any            `json:"content,omitempty"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	ToolCalls        []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	Name             string         `json:"name,omitempty"`
}

type chatToolCall struct {
	Index    int    `json:"index"` // 流式 delta 中按 index 增量拼接
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatToolDecl struct {
	Type     string     `json:"type"`
	Function ToolSchema `json:"function"`
}

type chatRequest struct {
	Model           string            `json:"model"`
	Messages        []chatHTTPMessage `json:"messages"`
	Tools           []chatToolDecl    `json:"tools,omitempty"`
	ToolChoice      string            `json:"tool_choice,omitempty"`
	MaxTokens       int               `json:"max_tokens,omitempty"`
	Temperature     *float64          `json:"temperature,omitempty"`
	ReasoningEffort string            `json:"reasoning_effort,omitempty"` // 智谱 glm-5.x 始终思考模型：low/high/max；空=不传（服务端默认）
	Stream          bool              `json:"stream"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int             `json:"index"`
		Message      chatHTTPMessage `json:"message"`
		FinishReason string          `json:"finish_reason"`
		Delta        chatHTTPMessage `json:"delta"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// LLMResponse 一次成功调用的结果。
type LLMResponse struct {
	Content   string     // 最终文本
	Reasoning string     // 思考内容（DeepSeek 思考模式：下一轮请求必须原样回传）
	ToolCalls []ToolCall // 工具调用（按顺序）
	Finish    string     // stop | tool_calls | length …
	Usage     int        // 本次消耗 token（0 = 未知）
	UsageIn   int        // 输入 token（0 = 未知）
	UsageOut  int        // 输出 token（0 = 未知）
}

// LLM OpenAI 兼容客户端。
type LLM struct {
	cfg        Config
	httpClient *http.Client
	baseURL    string
	model      string
}

// NewLLM 构建客户端。baseURL 不带 /chat/completions 后缀，结尾斜杠会被规整。
func NewLLM(cfg Config, proxyURL string) *LLM {
	c := cfg.normalize()
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &LLM{
		cfg:     c,
		baseURL: base,
		model:   c.Model,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   c.callTimeout(),
		},
	}
}

// APIKey 供测试与诊断用。
func (l *LLM) APIKey() string { return l.cfg.APIKey }

func (l *LLM) endpoint() string {
	if strings.HasSuffix(l.baseURL, "/chat/completions") {
		return l.baseURL
	}
	if hasVersionSuffix(l.baseURL) {
		return l.baseURL + "/chat/completions"
	}
	return l.baseURL + "/v1/chat/completions"
}

// hasVersionSuffix 判断 baseURL 是否以 /v<数字> 结尾（DeepSeek /v1、Zhipu /v4 等）。
// 只认末尾纯数字版本段，避免误伤域名或路径里的 "/v" 子串。
func hasVersionSuffix(base string) bool {
	i := strings.LastIndex(base, "/v")
	if i < 0 {
		return false
	}
	rest := base[i+2:] // "/v" 之后的字符（如 "/v4" → "4"）
	if rest == "" {
		return false
	}
	for _, ch := range rest {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// Chat 发起一次对话补全；出错时按 429/5xx/网络错误重试（最多 3 次，指数退避）。
// 工具声明为空时仍按纯对话调用。
func (l *LLM) Chat(ctx context.Context, req ChatRequest) (*LLMResponse, error) {
	if strings.TrimSpace(l.cfg.APIKey) == "" {
		return nil, fmt.Errorf("未配置 LLM API Key（设置页 → AI 配置）")
	}
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := l.chatOnce(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		// 仅对可重试错误退避；业务错误（4xx 等）直接返回。
		if !retryable(err) || attempt == 3 {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * 2 * time.Second):
		}
	}
	return nil, lastErr
}

// retryable 判定：网络层错误、超时、429、5xx。
func retryable(err error) bool {
	var terr interface{ Timeout() bool }
	if ok := asErr(err, &terr); ok && terr.Timeout() {
		return true
	}
	msg := err.Error()
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "reset by peer") || strings.Contains(msg, "429") {
		return true
	}
	for _, code := range []string{"500", "502", "503", "504"} {
		if strings.Contains(msg, "HTTP "+code) {
			return true
		}
	}
	return false
}

func asErr[T any](err error, target *T) bool {
	for e := err; e != nil; {
		if v, ok := e.(T); ok {
			*target = v
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := e.(unwrapper)
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

func (l *LLM) chatOnce(ctx context.Context, req ChatRequest) (*LLMResponse, error) {
	payload, err := l.buildChatBody(req, false)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, l.endpoint(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+l.cfg.APIKey)
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := l.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("调用 LLM 失败: %w", err)
	}
	defer httpResp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(httpResp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 LLM 响应失败: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM HTTP %d: %s", httpResp.StatusCode, truncate(sanitizeJSON(data), 400))
	}
	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return nil, fmt.Errorf("解析 LLM 响应失败: %w", err)
	}
	if cr.Error != nil {
		return nil, fmt.Errorf("LLM 错误: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return nil, fmt.Errorf("LLM 响应无 choices")
	}
	msg := cr.Choices[0].Message
	out := &LLMResponse{Finish: cr.Choices[0].FinishReason, Usage: cr.Usage.TotalTokens,
		UsageIn: cr.Usage.PromptTokens, UsageOut: cr.Usage.CompletionTokens}
	if s, ok := msg.Content.(string); ok {
		out.Content = s
	}
	out.Reasoning = msg.ReasoningContent
	for _, tc := range msg.ToolCalls {
		args := tc.Function.Arguments
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		out.ToolCalls = append(out.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: args})
	}
	return out, nil
}

// buildChatBody 组装请求体（stream=true 时为 SSE 流式请求）。
func (l *LLM) buildChatBody(req ChatRequest, stream bool) ([]byte, error) {
	msgs := make([]chatHTTPMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		cm := chatHTTPMessage{Role: string(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		if m.Role == RoleAssistant && m.Reasoning != "" {
			// DeepSeek 思考模式：assistant 消息必须把上轮的 reasoning_content 原样带回
			cm.ReasoningContent = m.Reasoning
		}
		if m.Role == RoleAssistant && len(m.ToolCalls) > 0 {
			cm.Content = ""
			for _, tc := range m.ToolCalls {
				ct := chatToolCall{ID: tc.ID, Type: "function"}
				ct.Function.Name = tc.Name
				ct.Function.Arguments = tc.Arguments
				cm.ToolCalls = append(cm.ToolCalls, ct)
			}
		}
		if m.Role == RoleTool {
			cm.Content = m.Content // 工具结果
		}
		msgs = append(msgs, cm)
	}
	body := chatRequest{
		Model:      l.model,
		Messages:   msgs,
		ToolChoice: "",
		Stream:     stream,
	}
	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			body.Tools = append(body.Tools, chatToolDecl{Type: "function", Function: t})
		}
		body.ToolChoice = "auto"
	}
	if req.MaxTokens > 0 {
		body.MaxTokens = req.MaxTokens
	}
	if l.cfg.ReasoningEffort != "" {
		body.ReasoningEffort = l.cfg.ReasoningEffort
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("编码请求失败: %w", err)
	}
	return payload, nil
}

// streamToolAccum 流式工具调用按 index 增量拼接的累积器。
type streamToolAccum struct {
	id   string
	name string
	args strings.Builder
}

// ChatStream 流式对话补全（SSE）：增量文本经 onDelta 逐段回调（逐 token 级别的
// 打字机效果），返回完整文本与工具调用。出错且尚未收到任何内容时回退到非流式
// Chat（保留其重试语义）；已收到部分内容后中断则返回已收内容（尽力而为）。
func (l *LLM) ChatStream(ctx context.Context, req ChatRequest, onDelta func(delta string)) (*LLMResponse, error) {
	if strings.TrimSpace(l.cfg.APIKey) == "" {
		return nil, fmt.Errorf("未配置 LLM API Key（设置页 → AI 配置）")
	}
	resp, err := l.streamOnce(ctx, req, onDelta)
	if err != nil && resp != nil && (len(resp.Content) > 0 || len(resp.ToolCalls) > 0) {
		return resp, nil // 已收到部分内容：尽力返回
	}
	if err != nil {
		return l.Chat(ctx, req) // 回退非流式（自带重试）
	}
	return resp, nil
}

// streamOnce 单次 SSE 流式调用（无重试；重试由 ChatStream 回退路径承担）。
func (l *LLM) streamOnce(ctx context.Context, req ChatRequest, onDelta func(string)) (*LLMResponse, error) {
	payload, err := l.buildChatBody(req, true)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, l.endpoint(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+l.cfg.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	// 流式响应不能受 httpClient.Timeout 截断：只借 transport，超时交给 ctx。
	client := &http.Client{Transport: l.httpClient.Transport, Timeout: 0}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("调用 LLM 失败: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
		return nil, fmt.Errorf("LLM HTTP %d: %s", httpResp.StatusCode, truncate(sanitizeJSON(data), 400))
	}

	out := &LLMResponse{}
	byIndex := map[int]*streamToolAccum{}
	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	var chunkErr error
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var cr chatResponse
		if err := json.Unmarshal([]byte(sanitizeJSON([]byte(data))), &cr); err != nil {
			chunkErr = fmt.Errorf("解析流式响应失败: %w", err)
			break
		}
		if cr.Error != nil {
			chunkErr = fmt.Errorf("LLM 错误: %s", cr.Error.Message)
			break
		}
		if len(cr.Choices) == 0 {
			continue
		}
		delta := cr.Choices[0].Delta
		if s, ok := delta.Content.(string); ok && s != "" {
			out.Content += s
			if onDelta != nil {
				onDelta(s)
			}
		}
		// 思考内容只静默累积（不进打字机流），但要原样回传下一轮
		out.Reasoning += delta.ReasoningContent
		for _, tc := range delta.ToolCalls {
			acc := byIndex[tc.Index]
			if acc == nil {
				acc = &streamToolAccum{id: tc.ID, name: tc.Function.Name}
				byIndex[tc.Index] = acc
			}
			if acc.id == "" {
				acc.id = tc.ID
			}
			if acc.name == "" {
				acc.name = tc.Function.Name
			}
			acc.args.WriteString(tc.Function.Arguments)
		}
		out.Finish = cr.Choices[0].FinishReason
		// 部分供应商在最后一块带上 usage（其余为 null）；取最后非零值
		if cr.Usage.PromptTokens > 0 {
			out.UsageIn = cr.Usage.PromptTokens
		}
		if cr.Usage.CompletionTokens > 0 {
			out.UsageOut = cr.Usage.CompletionTokens
		}
		if cr.Usage.TotalTokens > 0 {
			out.Usage = cr.Usage.TotalTokens
		}
	}
	if out.Usage == 0 {
		out.Usage = out.UsageIn + out.UsageOut
	}
	if err := scanner.Err(); err != nil {
		chunkErr = fmt.Errorf("读取流式响应失败: %w", err)
	}
	// 按 index 排序输出工具调用
	for i := 0; i < len(byIndex); i++ {
		acc := byIndex[i]
		if acc == nil {
			continue
		}
		args := acc.args.String()
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		out.ToolCalls = append(out.ToolCalls, ToolCall{ID: acc.id, Name: acc.name, Arguments: args})
	}
	if chunkErr != nil {
		return out, chunkErr
	}
	return out, nil
}

// sanitizeJSON 剥离响应体里的敏感前缀（部分中转会在 JSON 前注入提示文本）。
func sanitizeJSON(data []byte) string {
	s := string(data)
	if i := strings.IndexByte(s, '{'); i > 0 {
		s = s[i:]
	}
	return s
}

// truncate 截断字符串（rune 安全）。
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// extractJSON 从可能带围栏的文本中提取首个 JSON 对象（工具参数解析用）。
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	if i := strings.IndexByte(s, '{'); i >= 0 {
		if j := strings.LastIndexByte(s, '}'); j > i {
			return s[i : j+1]
		}
	}
	return s
}

// parseArgs 把 LLM 返回的参数字符串解析成 map；失败时返回明确错误。
func parseArgs(raw string) (map[string]any, error) {
	s := extractJSON(raw)
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("工具参数 JSON 解析失败: %w", err)
	}
	return m, nil
}

// stripSSE 剥离 SSE 前缀行（部分服务在 JSON 行前加 data:）。
func stripSSE(r io.Reader) io.Reader {
	br := bufio.NewReader(r)
	peek, _ := br.Peek(8)
	if !bytes.HasPrefix(peek, []byte("data:")) {
		return br
	}
	return br
}
