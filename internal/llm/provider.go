package llm

import (
	"encoding/json"
	"strings"
)

// BaseProvider 提供 Provider 的公共字段与共享逻辑。
type BaseProvider struct {
	apiURL         string
	apiKey         string
	model          string
	reasoningField string // 该 provider 使用的 reasoning 字段名（sanitize 时保留）
}

func (b *BaseProvider) APIURL() string { return b.apiURL }
func (b *BaseProvider) APIKey() string { return b.apiKey }
func (b *BaseProvider) Model() string  { return b.model }

func (b *BaseProvider) Headers() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + b.apiKey,
	}
}

// normalizeToolCalls 从消息中提取并规范化 tool_calls，对齐 normalize_tool_calls。
func normalizeToolCalls(message Message) []ToolCall {
	raw, _ := message["tool_calls"].([]any)
	if len(raw) == 0 {
		// 兼容旧版 function_call
		if fc, ok := message["function_call"].(map[string]any); ok {
			raw = []any{map[string]any{"id": "call_0", "type": "function", "function": fc}}
		}
	}
	if len(raw) == 0 {
		return nil
	}
	var calls []ToolCall
	for _, item := range raw {
		tc, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fn, _ := tc["function"].(map[string]any)
		argsStr, _ := fn["arguments"].(string)
		args := map[string]any{}
		if argsStr != "" {
			_ = json.Unmarshal([]byte(argsStr), &args)
		}
		name, _ := fn["name"].(string)
		id, _ := tc["id"].(string)
		typ, _ := tc["type"].(string)
		if typ == "" {
			typ = "function"
		}
		calls = append(calls, ToolCall{ID: id, Type: typ, Name: name, Arguments: args})
	}
	return calls
}

// extractReasoning 从已知字段提取 reasoning，对齐 extract_reasoning。
func extractReasoning(message Message) string {
	if v, ok := message["reasoning_content"].(string); ok && v != "" {
		return v
	}
	if v, ok := message["reasoning"].(string); ok && v != "" {
		return v
	}
	// reasoning_details 可能是 list
	if v, ok := message["reasoning_details"]; ok {
		return reasoningToText(v)
	}
	return ""
}

// reasoningToText 将任意 reasoning 值转纯文本，对齐 _extract_reasoning_text。
func reasoningToText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var parts []string
		for _, item := range x {
			if d, ok := item.(map[string]any); ok {
				if t, ok := d["text"].(string); ok && t != "" {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// textContent 提取消息 content 的纯文本，多模态时忽略图片，对齐 _text。
func textContent(content any) any {
	if arr, ok := content.([]any); ok {
		for _, b := range arr {
			if blk, ok := b.(map[string]any); ok && blk["type"] == "text" {
				return blk["text"]
			}
		}
		return "[image content omitted]"
	}
	return content
}

// sanitizeMessages 去除非标准字段，构建可发给 API 的干净消息，对齐 _sanitize_messages。
func (b *BaseProvider) sanitizeMessages(messages []Message) []Message {
	clean := make([]Message, 0, len(messages))
	for _, m := range messages {
		role, _ := m["role"].(string)
		switch role {
		case "system":
			clean = append(clean, Message{"role": "system", "content": strGet(m, "content")})
		case "user":
			clean = append(clean, Message{"role": "user", "content": textContent(m["content"])})
		case "assistant":
			msg := Message{"role": "assistant", "content": contentOrEmpty(m["content"])}
			if tc, ok := m["tool_calls"]; ok && tc != nil {
				msg["tool_calls"] = tc
			}
			if b.reasoningField != "" {
				if v, ok := m[b.reasoningField]; ok && notEmpty(v) {
					msg[b.reasoningField] = v
				} else if reasoning := extractReasoning(m); reasoning != "" {
					if b.reasoningField == "reasoning_details" {
						msg["reasoning_details"] = wrapReasoningDetail(reasoning)
					} else {
						msg["reasoning_content"] = reasoning
					}
				}
			} else if reasoning := extractReasoning(m); reasoning != "" {
				msg["reasoning_content"] = reasoning
			}
			clean = append(clean, msg)
		case "tool":
			clean = append(clean, Message{
				"role":         "tool",
				"tool_call_id": strGet(m, "tool_call_id"),
				"content":      textContent(m["content"]),
			})
		}
	}
	return clean
}

func wrapReasoningDetail(text string) []map[string]any {
	return []map[string]any{{
		"type": "reasoning.text", "id": "reasoning-text-1",
		"format": "MiniMax-response-v1", "index": 0, "text": text,
	}}
}

func strGet(m Message, key string) any {
	if v, ok := m[key]; ok {
		return v
	}
	return ""
}

func contentOrEmpty(v any) any {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return v
}

func notEmpty(v any) bool {
	if v == nil {
		return false
	}
	if s, ok := v.(string); ok {
		return s != ""
	}
	return true
}

// parseOpenAIResponse 是 OpenAI 兼容格式的响应解析，供各 provider 复用。
func parseOpenAIResponse(resp map[string]any) Response {
	var r Response
	choices, _ := resp["choices"].([]any)
	var message Message
	if len(choices) > 0 {
		if c, ok := choices[0].(map[string]any); ok {
			r.FinishReason, _ = c["finish_reason"].(string)
			message, _ = c["message"].(map[string]any)
		}
	}
	if message == nil {
		message = Message{}
	}
	r.RawMessage = message
	r.Content, _ = message["content"].(string)
	r.ReasoningContent = extractReasoning(message)
	r.ToolCalls = normalizeToolCalls(message)
	r.HasToolCalls = len(r.ToolCalls) > 0
	r.Model, _ = resp["model"].(string)
	if u, ok := resp["usage"].(map[string]any); ok {
		r.Usage = Usage{
			PromptTokens:     asInt(u["prompt_tokens"]),
			CompletionTokens: asInt(u["completion_tokens"]),
			TotalTokens:      asInt(u["total_tokens"]),
		}
	}
	return r
}

func asInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	}
	return 0
}
