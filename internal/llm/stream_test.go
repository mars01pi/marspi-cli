package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func loadSSEFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(testdataPath(name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func parseFixture(t *testing.T, name string) []StreamChunk {
	t.Helper()
	var chunks []StreamChunk
	err := ReadSSE(strings.NewReader(loadSSEFixture(t, name)), func(data string) error {
		c, err := ParseStreamData(data)
		if err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, c)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return chunks
}

func accumulateFixture(t *testing.T, name string) Response {
	t.Helper()
	resp, err := ReadSSEStream(strings.NewReader(loadSSEFixture(t, name)), nil)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestParseStreamDataDone(t *testing.T) {
	chunk, err := ParseStreamData("[DONE]")
	if err != nil {
		t.Fatal(err)
	}
	if !chunk.Done {
		t.Fatal("expected Done")
	}
}

func TestParseStreamDataInvalidJSON(t *testing.T) {
	_, err := ParseStreamData("{not json")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseStreamChunkContentDelta(t *testing.T) {
	chunk, err := ParseStreamData(`{"choices":[{"delta":{"content":"Hi"}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if chunk.ContentDelta != "Hi" {
		t.Fatalf("content delta: %q", chunk.ContentDelta)
	}
}

func TestParseStreamChunkReasoningDelta(t *testing.T) {
	chunk, err := ParseStreamData(`{"choices":[{"delta":{"reasoning_content":"think"}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if chunk.ReasoningDelta != "think" {
		t.Fatalf("reasoning delta: %q", chunk.ReasoningDelta)
	}
}

func TestParseStreamChunkToolCallDelta(t *testing.T) {
	chunk, err := ParseStreamData(`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"read","arguments":"{"}}]}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunk.ToolCallDeltas) != 1 {
		t.Fatalf("expected 1 tool delta, got %d", len(chunk.ToolCallDeltas))
	}
	d := chunk.ToolCallDeltas[0]
	if d.Index != 0 || d.ID != "c1" || d.Name != "read" || d.Arguments != "{" {
		t.Fatalf("unexpected delta: %+v", d)
	}
}

func TestReadSSEFixtureText(t *testing.T) {
	chunks := parseFixture(t, "stream_text.sse")
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	if !chunks[len(chunks)-1].Done {
		t.Fatal("last chunk should be Done")
	}
}

func TestAccumulatorTextFixture(t *testing.T) {
	resp := accumulateFixture(t, "stream_text.sse")
	if resp.Content != "Hello, world!" {
		t.Fatalf("content: %q", resp.Content)
	}
	if resp.FinishReason != "stop" {
		t.Fatalf("finish_reason: %q", resp.FinishReason)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Fatalf("usage total: %d", resp.Usage.TotalTokens)
	}
	if resp.Model != "gpt-4o" {
		t.Fatalf("model: %q", resp.Model)
	}
	if resp.HasToolCalls {
		t.Fatal("unexpected tool calls")
	}
}

func TestAccumulatorReasoningFixture(t *testing.T) {
	resp := accumulateFixture(t, "stream_reasoning.sse")
	if resp.ReasoningContent != "Let me think." {
		t.Fatalf("reasoning: %q", resp.ReasoningContent)
	}
	if resp.Content != "Done." {
		t.Fatalf("content: %q", resp.Content)
	}
}

func TestAccumulatorToolCallsFixture(t *testing.T) {
	resp := accumulateFixture(t, "stream_tool_calls.sse")
	if !resp.HasToolCalls {
		t.Fatal("expected tool calls")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls: %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "read" || tc.ID != "call_read_1" {
		t.Fatalf("unexpected tool call: %+v", tc)
	}
	if tc.Arguments["path"] != "README.md" {
		t.Fatalf("arguments: %v", tc.Arguments)
	}
	if resp.FinishReason != "tool_calls" {
		t.Fatalf("finish_reason: %q", resp.FinishReason)
	}

	acc := NewStreamAccumulator()
	for _, c := range parseFixture(t, "stream_tool_calls.sse") {
		acc.Apply(c)
	}
	if !acc.VerifyToolArgumentsJSON(0) {
		t.Fatalf("invalid args json: %q", acc.ToolCallArguments(0))
	}
}

func TestStreamHandlerAbort(t *testing.T) {
	called := 0
	_, err := ReadSSEStream(strings.NewReader(loadSSEFixture(t, "stream_text.sse")), func(c StreamChunk) error {
		called++
		if c.ContentDelta == "Hello" {
			return os.ErrClosed
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected abort error")
	}
	if called < 2 {
		t.Fatalf("expected at least 2 handler calls, got %d", called)
	}
}

func TestAccumulatorIncrementalApply(t *testing.T) {
	acc := NewStreamAccumulator()
	acc.Apply(StreamChunk{ContentDelta: "a"})
	acc.Apply(StreamChunk{ContentDelta: "b"})
	if acc.Content() != "ab" {
		t.Fatalf("content: %q", acc.Content())
	}
}

func TestStreamChatHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["stream"] != true {
			t.Fatalf("expected stream true, got %v", body["stream"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(loadSSEFixture(t, "stream_text.sse")))
	}))
	defer srv.Close()

	var deltas int
	resp, err := StreamChat(context.Background(), srv.URL, map[string]any{"model": "gpt-4o"},
		map[string]string{"Content-Type": "application/json"}, 5*time.Second,
		func(c StreamChunk) error {
			if c.ContentDelta != "" {
				deltas++
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if deltas != 3 {
		t.Fatalf("content deltas: %d", deltas)
	}
	if resp.Content != "Hello, world!" {
		t.Fatalf("content: %q", resp.Content)
	}
}
