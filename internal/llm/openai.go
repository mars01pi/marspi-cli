package llm

import "strings"

// OpenAIProvider 是 OpenAI 兼容实现。
type OpenAIProvider struct{ BaseProvider }

func (p *OpenAIProvider) BuildBody(messages []Message, tools []map[string]any) map[string]any {
	return map[string]any{
		"model":    p.model,
		"messages": p.sanitizeMessages(messages),
		"tools":    tools,
		"stream":   false,
	}
}

func (p *OpenAIProvider) ParseResponse(resp map[string]any) Response {
	return parseOpenAIResponse(resp)
}

// DeepSeekProvider 在 OpenAI 基础上追加 thinking/reasoning_effort。
type DeepSeekProvider struct{ OpenAIProvider }

func (p *DeepSeekProvider) BuildBody(messages []Message, tools []map[string]any) map[string]any {
	body := p.OpenAIProvider.BuildBody(messages, tools)
	body["thinking"] = map[string]any{"type": "enabled"}
	body["reasoning_effort"] = "max"
	return body
}

// MiniMaxProvider 使用 reasoning_details 字段并开启 reasoning_split。
type MiniMaxProvider struct{ OpenAIProvider }

func (p *MiniMaxProvider) BuildBody(messages []Message, tools []map[string]any) map[string]any {
	body := p.OpenAIProvider.BuildBody(messages, tools)
	body["reasoning_split"] = true
	return body
}

// normalizeURL 补全 /chat/completions 后缀，对齐 _new_provider。
func normalizeURL(apiURL string) string {
	if strings.HasSuffix(apiURL, "/chat/completions") {
		return apiURL
	}
	return strings.TrimRight(apiURL, "/") + "/chat/completions"
}

// NewProvider 依据模型名选择合适的 provider，对齐 _new_provider。
func NewProvider(model, apiURL, apiKey string) Provider {
	url := normalizeURL(apiURL)
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "deepseek"):
		return &DeepSeekProvider{OpenAIProvider{BaseProvider{apiURL: url, apiKey: apiKey, model: model}}}
	case strings.Contains(lower, "minimax"):
		return &MiniMaxProvider{OpenAIProvider{BaseProvider{apiURL: url, apiKey: apiKey, model: model, reasoningField: "reasoning_details"}}}
	default:
		return &OpenAIProvider{BaseProvider{apiURL: url, apiKey: apiKey, model: model}}
	}
}
