package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mars/marspi-cli/internal/agentctx"
	"github.com/mars/marspi-cli/internal/config"
	"github.com/mars/marspi-cli/internal/llm"
	"github.com/mars/marspi-cli/internal/ui"
)

func writeJSONCompletion(w http.ResponseWriter, finish string, msg map[string]any) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"model": "mock",
		"choices": []any{map[string]any{
			"finish_reason": finish,
			"message":       msg,
		}},
		"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3},
	})
}

func newMockLLMServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, n int)) *httptest.Server {
	t.Helper()
	var n atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		handler(w, r, int(n.Add(1)))
	}))
}

func newTestRunner(t *testing.T, srvURL string) (*Runner, *agentctx.Manager, *Emitter) {
	t.Helper()
	prov := llm.NewProvider("gpt-4o", srvURL, "test-key")
	cfg := &config.Config{ProjectRoot: t.TempDir(), MaxContext: 100_000, MaxIter: 5}
	reg := newTestRegistry(t)
	emit := NewEmitter()
	runner := &Runner{
		Provider:   prov,
		Registry:   reg,
		Console:    ui.NewPrinter(),
		Events:     emit,
		MaxContext: cfg.MaxContext,
		MaxIter:    cfg.MaxIter,
		Stream:     false,
	}
	ctx := agentctx.New(cfg.MaxContext, prov, reg.Schemas(), nil)
	return runner, ctx, emit
}

func TestLoopStopResponse(t *testing.T) {
	srv := newMockLLMServer(t, func(w http.ResponseWriter, _ *http.Request, n int) {
		if n != 1 {
			t.Errorf("unexpected request %d", n)
		}
		writeJSONCompletion(w, "stop", map[string]any{"role": "assistant", "content": "hello"})
	})
	defer srv.Close()

	runner, mgr, emit := newTestRunner(t, srv.URL)
	var types []EventType
	emit.Subscribe(func(ev Event) { types = append(types, ev.Type) })

	runner.LoopCtx(context.Background(), mgr, "", "hi")

	if mgr.Len() < 2 {
		t.Fatalf("expected user+assistant messages, got %d", mgr.Len())
	}
	assertHasEvents(t, types, EventRunStart, EventTurnStart, EventLLMStart, EventMessageEnd, EventRunEnd)
}

func TestLoopAttemptCompletion(t *testing.T) {
	srv := newMockLLMServer(t, func(w http.ResponseWriter, _ *http.Request, n int) {
		switch n {
		case 1:
			writeJSONCompletion(w, "tool_calls", map[string]any{
				"role": "assistant",
				"content": "",
				"tool_calls": []any{map[string]any{
					"id": "call_done", "type": "function",
					"function": map[string]any{
						"name":      "attempt_completion",
						"arguments": `{"result":"all done"}`,
					},
				}},
			})
		default:
			t.Fatalf("unexpected extra request %d", n)
		}
	})
	defer srv.Close()

	runner, mgr, emit := newTestRunner(t, srv.URL)
	var types []EventType
	emit.Subscribe(func(ev Event) { types = append(types, ev.Type) })

	runner.LoopCtx(context.Background(), mgr, "", "finish task")

	assertHasEvents(t, types, EventToolStart, EventToolEnd, EventRunEnd)
	if mgr.Len() < 3 {
		t.Fatalf("expected user+assistant+tool messages, got %d", mgr.Len())
	}
}

func TestLoopContextCancel(t *testing.T) {
	block := make(chan struct{})
	srv := newMockLLMServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		<-block // 模拟长时间请求
	})
	defer srv.Close()
	defer close(block)

	runner, mgr, emit := newTestRunner(t, srv.URL)
	var types []EventType
	emit.Subscribe(func(ev Event) { types = append(types, ev.Type) })

	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runner.LoopCtx(runCtx, mgr, "", "slow")
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done

	assertHasEvents(t, types, EventWarn)
}

func assertHasEvents(t *testing.T, got []EventType, want ...EventType) {
	t.Helper()
	set := map[EventType]bool{}
	for _, ty := range got {
		set[ty] = true
	}
	for _, ty := range want {
		if !set[ty] {
			t.Errorf("missing event %q in %v", ty, got)
		}
	}
}
