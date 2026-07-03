package llm

import (
	"strings"
	"testing"
)

func TestNormalizeToolCalls(t *testing.T) {
	msg := Message{
		"tool_calls": []any{
			map[string]any{
				"id": "c1", "type": "function",
				"function": map[string]any{"name": "read", "arguments": `{"path":"a.go"}`},
			},
		},
	}
	calls := normalizeToolCalls(msg)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "read" {
		t.Errorf("expected read, got %s", calls[0].Name)
	}
	if calls[0].Arguments["path"] != "a.go" {
		t.Errorf("expected path a.go, got %v", calls[0].Arguments["path"])
	}
}

func TestNormalizeFunctionCallFallback(t *testing.T) {
	msg := Message{
		"function_call": map[string]any{"name": "edit", "arguments": `{}`},
	}
	calls := normalizeToolCalls(msg)
	if len(calls) != 1 || calls[0].Name != "edit" {
		t.Errorf("function_call fallback failed: %v", calls)
	}
}

func TestExtractReasoning(t *testing.T) {
	if got := extractReasoning(Message{"reasoning_content": "rc"}); got != "rc" {
		t.Errorf("deepseek reasoning: got %q", got)
	}
	if got := extractReasoning(Message{"reasoning": "r"}); got != "r" {
		t.Errorf("qwen reasoning: got %q", got)
	}
	details := Message{"reasoning_details": []any{
		map[string]any{"text": "part1"}, map[string]any{"text": "part2"},
	}}
	if got := extractReasoning(details); got != "part1\npart2" {
		t.Errorf("minimax reasoning: got %q", got)
	}
}

func TestParseOpenAIResponse(t *testing.T) {
	resp := map[string]any{
		"model": "gpt-4",
		"choices": []any{map[string]any{
			"finish_reason": "stop",
			"message":       map[string]any{"role": "assistant", "content": "hi"},
		}},
		"usage": map[string]any{"prompt_tokens": float64(10), "completion_tokens": float64(5)},
	}
	r := parseOpenAIResponse(resp)
	if r.FinishReason != "stop" || r.Content != "hi" {
		t.Errorf("parse failed: %+v", r)
	}
	if r.Usage.PromptTokens != 10 || r.Usage.CompletionTokens != 5 {
		t.Errorf("usage parse failed: %+v", r.Usage)
	}
	if r.HasToolCalls {
		t.Error("should have no tool calls")
	}
}

func TestParseAPIErrorInBody(t *testing.T) {
	resp := map[string]any{
		"error": map[string]any{"message": "Invalid API key", "type": "authentication_error"},
	}
	r := parseOpenAIResponse(resp)
	if r.FinishReason != "error" || !strings.Contains(r.Content, "Invalid API key") {
		t.Errorf("expected api error, got %+v", r)
	}
}

func TestMessageContentNull(t *testing.T) {
	if got := messageContent(Message{"content": nil}); got != "" {
		t.Errorf("nil content: got %q", got)
	}
}

func TestMessageContentBlocks(t *testing.T) {
	msg := Message{"content": []any{
		map[string]any{"type": "text", "text": "hello"},
	}}
	if got := messageContent(msg); got != "hello" {
		t.Errorf("block content: got %q", got)
	}
}

func TestNewProviderSelection(t *testing.T) {
	if _, ok := NewProvider("deepseek-v4", "https://x.com", "k").(*DeepSeekProvider); !ok {
		t.Error("expected DeepSeekProvider")
	}
	if _, ok := NewProvider("minimax-01", "https://x.com", "k").(*MiniMaxProvider); !ok {
		t.Error("expected MiniMaxProvider")
	}
	if _, ok := NewProvider("gpt-4o", "https://x.com", "k").(*OpenAIProvider); !ok {
		t.Error("expected OpenAIProvider")
	}
}

func TestNormalizeURL(t *testing.T) {
	if got := normalizeURL("https://api.deepseek.com"); got != "https://api.deepseek.com/chat/completions" {
		t.Errorf("got %q", got)
	}
	if got := normalizeURL("https://x.com/chat/completions"); got != "https://x.com/chat/completions" {
		t.Errorf("should not double-append: %q", got)
	}
}

func TestKeywordScore(t *testing.T) {
	if s := keywordScore("设计一个分布式系统"); s != 9 {
		t.Errorf("design should score 9, got %d", s)
	}
	if s := keywordScore("卧槽这什么玩意"); s != 10 {
		t.Errorf("anger should score 10, got %d", s)
	}
	if s := keywordScore("随便说点什么"); s != 4 {
		t.Errorf("neutral should score 4, got %d", s)
	}
	if s := keywordScore("implement a new feature"); s != 5 {
		t.Errorf("implement should score 5, got %d", s)
	}
	if s := keywordScore("fix the login bug"); s != 3 {
		t.Errorf("debug should score 3, got %d", s)
	}
}

func TestRoutedProviderInit(t *testing.T) {
	rp, err := NewRoutedProviderFromList([]providerCfg{
		{Name: "lo", URL: "https://lo.com", Model: "lo", Tier: "low", APIKey: "k-lo"},
		{Name: "md", URL: "https://md.com", Model: "md", Tier: "medium", APIKey: "k-md"},
		{Name: "hi", URL: "https://hi.com", Model: "hi", Tier: "high", APIKey: "k-hi"},
	}, "medium", nil)
	if err != nil {
		t.Fatal(err)
	}
	if rp.Model() != "md" {
		t.Errorf("default tier medium, got model %s", rp.Model())
	}
	if rp.TotalProviders() != 3 {
		t.Errorf("expected 3 providers, got %d", rp.TotalProviders())
	}
}

func TestRoutedProviderRouteKeywordLow(t *testing.T) {
	rp, err := NewRoutedProviderFromList([]providerCfg{
		{Name: "lo", URL: "https://lo.com", Model: "lo-model", Tier: "low", APIKey: "k"},
		{Name: "md", URL: "https://md.com", Model: "md-model", Tier: "medium", APIKey: "k"},
		{Name: "hi", URL: "https://hi.com", Model: "hi-model", Tier: "high", APIKey: "k"},
	}, "medium", nil)
	if err != nil {
		t.Fatal(err)
	}
	rp.Route("read the file", "[]")
	if rp.Model() != "lo-model" {
		t.Errorf("expected low tier, got %s", rp.Model())
	}
}

func TestRoutedProviderRouteKeywordHigh(t *testing.T) {
	rp, err := NewRoutedProviderFromList([]providerCfg{
		{Name: "lo", URL: "https://lo.com", Model: "lo-model", Tier: "low", APIKey: "k"},
		{Name: "md", URL: "https://md.com", Model: "md-model", Tier: "medium", APIKey: "k"},
		{Name: "hi", URL: "https://hi.com", Model: "hi-model", Tier: "high", APIKey: "k"},
	}, "medium", nil)
	if err != nil {
		t.Fatal(err)
	}
	rp.Route("design a distributed system", "[]")
	if rp.Model() != "hi-model" {
		t.Errorf("expected high tier, got %s", rp.Model())
	}
}

func TestRoutedProviderInvalidTier(t *testing.T) {
	_, err := NewRoutedProviderFromList([]providerCfg{
		{Name: "x", URL: "https://x.com", Model: "x", Tier: "super_fast", APIKey: "k"},
	}, "medium", nil)
	if err == nil {
		t.Error("expected error for invalid tier")
	}
}
