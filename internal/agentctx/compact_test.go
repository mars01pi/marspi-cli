package agentctx

import (
	"strings"
	"testing"

	"github.com/mars/marspi-cli/internal/llm"
)

func TestCompactText(t *testing.T) {
	short := "hello world"
	if got := compactText(short, 100, 100); got != short {
		t.Errorf("short text should be unchanged, got %q", got)
	}
	long := strings.Repeat("a", 500)
	got := compactText(long, 100, 100)
	if !strings.HasSuffix(got, "<compacted>") {
		t.Errorf("long text should end with <compacted>, got %q", got[len(got)-20:])
	}
	if !strings.Contains(got, "\n...\n") {
		t.Error("compacted text should contain ellipsis")
	}
}

func TestSplitTurns(t *testing.T) {
	m := &Manager{Messages: []llm.Message{
		{"role": "system", "content": "sys"},
		{"role": "user", "content": "q1"},
		{"role": "assistant", "content": "a1"},
		{"role": "tool", "content": "t1"},
		{"role": "user", "content": "q2"},
		{"role": "assistant", "content": "a2"},
	}}
	turns := m.splitTurns()
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}
	if len(turns[0]) != 3 { // user + assistant + tool
		t.Errorf("turn 0 should have 3 msgs, got %d", len(turns[0]))
	}
	if len(turns[1]) != 2 {
		t.Errorf("turn 1 should have 2 msgs, got %d", len(turns[1]))
	}
}

func TestEstimatedTokens(t *testing.T) {
	msg := llm.Message{"role": "user", "content": "hello"}
	tk := estimatedTokens(msg)
	if tk <= 0 {
		t.Errorf("tokens should be positive, got %d", tk)
	}
}

func TestSessionMemoryCompact(t *testing.T) {
	var msgs []llm.Message
	msgs = append(msgs, llm.Message{"role": "system", "content": "sys"})
	// 制造 15 轮
	for i := 0; i < 15; i++ {
		msgs = append(msgs, llm.Message{"role": "user", "content": "q"})
		msgs = append(msgs, llm.Message{"role": "assistant", "content": "a"})
		msgs = append(msgs, llm.Message{"role": "tool", "tool_name": "read", "tool_call_id": "c", "content": strings.Repeat("x", 100)})
	}
	m := &Manager{Messages: msgs, whiteToolList: map[string]bool{}, now: func() int64 { return 0 }}
	ok := m.sessionMemoryCompact(10, 200)
	if !ok {
		t.Fatal("expected compaction to run")
	}
	// 旧 tool 应被强制压缩标记
	found := false
	for _, msg := range m.Messages {
		if c, _ := msg["content"].(string); strings.Contains(c, "force compacted") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected old tool results to be force-compacted")
	}
}
