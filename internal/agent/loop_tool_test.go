package agent

import (
	"testing"

	"github.com/mars/marspi-cli/internal/config"
	"github.com/mars/marspi-cli/internal/llm"
	"github.com/mars/marspi-cli/internal/tool"
	"github.com/mars/marspi-cli/internal/ui"
)

type echoTool struct{ tool.Base }

func (echoTool) Name() string        { return "echo_tool" }
func (echoTool) Description() string { return "echo" }
func (echoTool) Params() []tool.Param {
	return []tool.Param{{Name: "msg", Type: "string", Desc: "message"}}
}
func (echoTool) Preview(args map[string]any) string { return args["msg"].(string) }
func (echoTool) Run(args map[string]any) tool.Result {
	return tool.OK(args["msg"])
}

func newTestRegistry(t *testing.T) *tool.Registry {
	t.Helper()
	cfg := &config.Config{ProjectRoot: t.TempDir()}
	reg := tool.NewRegistry(cfg, ui.NewPrinter(), nil, nil)
	reg.Register(echoTool{})
	return reg
}

func TestValidateToolCall(t *testing.T) {
	if err := validateToolCall(llm.ToolCall{Name: "read", Arguments: map[string]any{"path": "a.go"}}); err != nil {
		t.Fatalf("valid call: %v", err)
	}
	if err := validateToolCall(llm.ToolCall{Name: "", Arguments: map[string]any{}}); err == nil {
		t.Fatal("expected error for empty name")
	}
	if err := validateToolCall(llm.ToolCall{Name: "read", Arguments: nil}); err != nil {
		t.Fatalf("nil args ok: %v", err)
	}
}

func TestExecuteToolEmitsEvents(t *testing.T) {
	reg := newTestRegistry(t)
	var events []Event
	e := NewEmitter()
	e.Subscribe(func(ev Event) { events = append(events, ev) })

	r := &Runner{Registry: reg, Events: e}
	result, done := r.executeTool(1, llm.ToolCall{
		ID: "c1", Name: "echo_tool", Arguments: map[string]any{"msg": "hi"},
	})
	if done {
		t.Fatal("unexpected completed")
	}
	if result != "hi" {
		t.Fatalf("result: %v", result)
	}
	if len(events) != 2 {
		t.Fatalf("expected start+end, got %d", len(events))
	}
	if events[0].Type != EventToolStart || events[0].ToolPreview == "" {
		t.Fatalf("start: %+v", events[0])
	}
	if events[1].Type != EventToolEnd || !events[1].ToolOK {
		t.Fatalf("end: %+v", events[1])
	}
	if len(events[1].ToolResultLines) == 0 {
		t.Fatal("expected preview lines")
	}
}

func TestExecuteToolInvalidArgs(t *testing.T) {
	reg := newTestRegistry(t)
	var events []Event
	e := NewEmitter()
	e.Subscribe(func(ev Event) { events = append(events, ev) })

	r := &Runner{Registry: reg, Events: e}
	result, _ := r.executeTool(1, llm.ToolCall{ID: "c1", Name: "", Arguments: map[string]any{}})
	if result == nil {
		t.Fatal("expected error result")
	}
	if len(events) != 2 || events[1].ToolOK {
		t.Fatalf("events: %+v", events)
	}
}
