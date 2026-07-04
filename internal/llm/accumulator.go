package llm

import (
	"encoding/json"
	"strings"
)

// StreamAccumulator 将流式 chunk 聚合为最终 Response。
type StreamAccumulator struct {
	content      strings.Builder
	reasoning    strings.Builder
	toolCalls    map[int]*streamToolCall
	finishReason string
	usage        Usage
	hasUsage     bool
	model        string
}

type streamToolCall struct {
	id        string
	typ       string
	name      string
	arguments strings.Builder
}

// NewStreamAccumulator 创建空 accumulator。
func NewStreamAccumulator() *StreamAccumulator {
	return &StreamAccumulator{toolCalls: map[int]*streamToolCall{}}
}

// Apply 合并一个 chunk。
func (a *StreamAccumulator) Apply(c StreamChunk) {
	if c.Done {
		return
	}
	if c.Model != "" {
		a.model = c.Model
	}
	if c.HasUsage {
		a.usage = c.Usage
		a.hasUsage = true
	}
	if c.FinishReason != "" {
		a.finishReason = c.FinishReason
	}
	a.content.WriteString(c.ContentDelta)
	a.reasoning.WriteString(c.ReasoningDelta)
	for _, d := range c.ToolCallDeltas {
		a.applyToolDelta(d)
	}
}

func (a *StreamAccumulator) applyToolDelta(d ToolCallDelta) {
	st, ok := a.toolCalls[d.Index]
	if !ok {
		st = &streamToolCall{}
		a.toolCalls[d.Index] = st
	}
	if d.ID != "" {
		st.id = d.ID
	}
	if d.Type != "" {
		st.typ = d.Type
	}
	if d.Name != "" {
		st.name = d.Name
	}
	st.arguments.WriteString(d.Arguments)
}

// BuildResponse 生成与非流式 ParseResponse 对齐的 Response。
func (a *StreamAccumulator) BuildResponse() Response {
	content := a.content.String()
	reasoning := a.reasoning.String()
	msg := Message{
		"role":    "assistant",
		"content": contentOrEmptyStr(content),
	}
	if reasoning != "" {
		msg["reasoning_content"] = reasoning
	}
	toolCalls := a.buildToolCallsMessage()
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
	}
	calls := normalizeToolCalls(msg)
	fr := a.finishReason
	if fr == "" && len(calls) > 0 {
		fr = "tool_calls"
	}
	if fr == "" {
		fr = "stop"
	}
	return Response{
		FinishReason:     fr,
		RawMessage:       msg,
		Content:          content,
		ReasoningContent: reasoning,
		ToolCalls:        calls,
		HasToolCalls:     len(calls) > 0,
		Model:            a.model,
		Usage:            a.usage,
	}
}

func (a *StreamAccumulator) buildToolCallsMessage() []any {
	if len(a.toolCalls) == 0 {
		return nil
	}
	maxIdx := -1
	for idx := range a.toolCalls {
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	out := make([]any, 0, len(a.toolCalls))
	for i := 0; i <= maxIdx; i++ {
		st, ok := a.toolCalls[i]
		if !ok {
			continue
		}
		typ := st.typ
		if typ == "" {
			typ = "function"
		}
		out = append(out, map[string]any{
			"id":   st.id,
			"type": typ,
			"function": map[string]any{
				"name":      st.name,
				"arguments": st.arguments.String(),
			},
		})
	}
	return out
}

func contentOrEmptyStr(s string) any {
	if s == "" {
		return ""
	}
	return s
}

// Content 返回当前聚合的正文（测试用）。
func (a *StreamAccumulator) Content() string { return a.content.String() }

// Reasoning 返回当前聚合的 reasoning（测试用）。
func (a *StreamAccumulator) Reasoning() string { return a.reasoning.String() }

// ToolCallNames 按 index 顺序返回工具名（测试用）。
func (a *StreamAccumulator) ToolCallNames() []string {
	resp := a.BuildResponse()
	names := make([]string, 0, len(resp.ToolCalls))
	for _, tc := range resp.ToolCalls {
		names = append(names, tc.Name)
	}
	return names
}

// ToolCallArguments 返回指定 index 的工具参数 JSON 字符串（测试用）。
func (a *StreamAccumulator) ToolCallArguments(index int) string {
	st, ok := a.toolCalls[index]
	if !ok {
		return ""
	}
	return st.arguments.String()
}

// VerifyToolArgumentsJSON 检查指定 index 的参数是否为合法 JSON object。
func (a *StreamAccumulator) VerifyToolArgumentsJSON(index int) bool {
	raw := a.ToolCallArguments(index)
	if raw == "" {
		return true
	}
	var obj map[string]any
	return json.Unmarshal([]byte(raw), &obj) == nil
}
