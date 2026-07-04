package agent

import (
	"sync"

	"github.com/mars/marspi-cli/internal/llm"
)

// EventType 是 agent runtime 事件类型。命名对齐 pi agent-loop，便于后续接流式。
type EventType string

const (
	EventRunStart  EventType = "run_start"
	EventRunEnd    EventType = "run_end"
	EventTurnStart EventType = "turn_start"
	EventTurnEnd   EventType = "turn_end"

	EventLLMStart EventType = "llm_start"
	EventLLMEnd   EventType = "llm_end"

	EventMessageStart EventType = "message_start"
	EventMessageDelta EventType = "message_delta"
	EventMessageEnd   EventType = "message_end"

	EventToolStart  EventType = "tool_start"
	EventToolUpdate EventType = "tool_update"
	EventToolEnd    EventType = "tool_end"

	EventWarn  EventType = "warn"
	EventError EventType = "error"
)

// DeltaField 标识 message_delta 追加的目标字段。
type DeltaField string

const (
	DeltaContent   DeltaField = "content"
	DeltaReasoning DeltaField = "reasoning"
)

// Event 是 loop 对外广播的统一事件载体。
type Event struct {
	Type EventType

	UserInput string
	Iteration int

	Usage         llm.Usage
	ContextTokens int
	MaxContext    int

	Content      string
	Reasoning    string
	FinishReason string
	HasToolCalls bool
	DeltaField   DeltaField
	Delta        string // message_delta：截至当前的累积全文（非增量片段）

	ToolName   string
	ToolCallID string
	ToolArgs   map[string]any
	ToolOK     bool

	Text string

	// message_end：正文已通过 delta 输出，订阅方勿重复渲染全文
	Streamed bool
}

// Handler 处理单条 agent 事件。
type Handler func(Event)

type handlerEntry struct {
	id int
	fn Handler
}

// Emitter 是进程内 pub/sub 总线；loop 只 emit，UI/日志各自 subscribe。
type Emitter struct {
	mu       sync.RWMutex
	handlers []handlerEntry
	nextID   int
}

// NewEmitter 创建事件总线。
func NewEmitter() *Emitter { return &Emitter{} }

// Subscribe 注册 handler，返回取消函数。
func (e *Emitter) Subscribe(h Handler) (unsubscribe func()) {
	if e == nil || h == nil {
		return func() {}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	id := e.nextID
	e.nextID++
	e.handlers = append(e.handlers, handlerEntry{id: id, fn: h})
	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		for i, entry := range e.handlers {
			if entry.id == id {
				e.handlers = append(e.handlers[:i], e.handlers[i+1:]...)
				return
			}
		}
	}
}

// Emit 依次调用所有订阅者（同步、按注册顺序）。
func (e *Emitter) Emit(ev Event) {
	if e == nil {
		return
	}
	e.mu.RLock()
	entries := append([]handlerEntry(nil), e.handlers...)
	e.mu.RUnlock()
	for _, entry := range entries {
		if entry.fn != nil {
			entry.fn(ev)
		}
	}
}
