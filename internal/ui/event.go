package ui

// EventKind 是 TUI 渲染事件类型。
type EventKind int

const (
	EvSection EventKind = iota
	EvLine
	EvError
	EvSuccess
	EvWarning
	EvStatus
	EvSpinner
	EvStreamDelta
	EvStreamEnd
)

// Event 供 Printer 向 Bubble Tea 等上层 UI 推送结构化输出。
type Event struct {
	Kind  EventKind
	Title string
	Text  string
	Style string // thinking | output | tool | dim | user | ...

	StreamID string // 流式块 ID（如 "2-reasoning"）
}
