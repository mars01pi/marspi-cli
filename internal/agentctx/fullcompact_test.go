package agentctx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mars/marspi-cli/internal/llm"
)

func TestCompactSummaryText(t *testing.T) {
	if got := compactSummaryText(llm.Response{Content: "hello"}); got != "hello" {
		t.Errorf("content: got %q", got)
	}
	if got := compactSummaryText(llm.Response{ReasoningContent: "<summary>x</summary>"}); got != "<summary>x</summary>" {
		t.Errorf("reasoning fallback: got %q", got)
	}
	if got := compactSummaryText(llm.Response{}); got != "" {
		t.Errorf("empty: got %q", got)
	}
}

func TestFullCompactNoToolsInRequest(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"finish_reason": "stop",
				"message":       map[string]any{"role": "assistant", "content": "<summary>ok</summary>"},
			}},
		})
	}))
	defer srv.Close()

	p := llm.NewProvider("gpt-4", srv.URL, "k")
	m := &Manager{
		Messages: []llm.Message{
			{"role": "system", "content": "sys"},
			{"role": "user", "content": "q"},
		},
		provider: p,
		tools:    []map[string]any{{"type": "function", "function": map[string]any{"name": "read"}}},
		now:      func() int64 { return 1 },
	}
	if err := m.fullCompact(); err != nil {
		t.Fatal(err)
	}
	if _, ok := gotBody["tools"]; ok {
		t.Error("compact request should not include tools")
	}
	if len(m.Messages) != 2 || m.Messages[1]["content"] != "<summary>ok</summary>" {
		t.Fatalf("unexpected messages: %+v", m.Messages)
	}
}

func TestFullCompactReasoningFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"finish_reason": "stop",
				"message": map[string]any{
					"role":              "assistant",
					"content":           "",
					"reasoning_content": "<summary>from reasoning</summary>",
				},
			}},
		})
	}))
	defer srv.Close()

	p := llm.NewProvider("deepseek-chat", srv.URL, "k")
	m := &Manager{
		Messages: []llm.Message{{"role": "user", "content": "q"}},
		provider: p,
		now:      func() int64 { return 1 },
	}
	if err := m.fullCompact(); err != nil {
		t.Fatal(err)
	}
	content, _ := m.Messages[len(m.Messages)-1]["content"].(string)
	if !strings.Contains(content, "from reasoning") {
		t.Fatalf("expected reasoning summary, got %q", content)
	}
}

func TestFullCompactRollbackOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"finish_reason": "stop",
				"message":       map[string]any{"role": "assistant", "content": ""},
			}},
		})
	}))
	defer srv.Close()

	p := llm.NewProvider("gpt-4", srv.URL, "k")
	orig := []llm.Message{{"role": "user", "content": "q"}}
	m := &Manager{Messages: append([]llm.Message(nil), orig...), provider: p, now: func() int64 { return 1 }}
	err := m.fullCompact()
	if err == nil || !strings.Contains(err.Error(), "llm respon null") {
		t.Fatalf("expected null error, got %v", err)
	}
	if len(m.Messages) != 1 || m.Messages[0]["content"] != "q" {
		t.Fatalf("prompt should be rolled back, got %+v", m.Messages)
	}
}
