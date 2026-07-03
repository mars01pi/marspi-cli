package ui

// Hooks 让 Printer 将输出转发到 TUI，并支持自定义确认对话框。
type Hooks struct {
	OnEvent func(Event)
	Silent  bool
	Confirm func(message string) bool
}
