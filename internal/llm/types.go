// Package llm 封装大模型交互：消息类型、Provider 抽象与 HTTP 请求。
// 对齐 mangopi-cli 的 BaseProvider/OpenAIProvider/DeepSeek/MiniMax/RoutedProvider。
package llm

// Message 是对话消息。为兼容多家 provider 的非标准字段，
// 采用 map 承载，键与 OpenAI/DeepSeek/MiniMax 的 JSON 字段一致。
// 常见键：role, content, tool_calls, tool_call_id, tool_name, reasoning_content, ts。
type Message = map[string]any

// ToolCall 是规范化后的工具调用。
type ToolCall struct {
	ID        string
	Type      string
	Name      string
	Arguments map[string]any
}

// Response 是解析后的模型响应，对齐 parse_response 的返回结构。
type Response struct {
	FinishReason     string
	RawMessage       Message
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
	HasToolCalls     bool
	Model            string
	Usage            Usage
}

// Usage 是 token 用量。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Provider 是一个可发起 chat completion 的模型端点。
type Provider interface {
	APIURL() string
	APIKey() string
	Model() string
	Headers() map[string]string
	BuildBody(messages []Message, tools []map[string]any) map[string]any
	ParseResponse(resp map[string]any) Response
}
