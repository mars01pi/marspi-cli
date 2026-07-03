package tool

import (
	"fmt"
	"strings"

	"github.com/mars/marspi-cli/internal/ui"
)

// Execute 执行一次工具调用并回显结果，返回给模型的内容（string 或 image map）。
// 对齐 mangopi 的 run_tool。
func (r *Registry) Execute(name string, args map[string]any) any {
	t, ok := r.Get(name)
	if !ok {
		return "run tool " + name + " error: unknown tool"
	}

	defer func() {
		if rec := recover(); rec != nil {
			r.console.EndSpinner()
		}
	}()

	r.console.ToolCall(t.Name(), t.Preview(args))
	t.Before(args)
	if !t.Confirm(args) {
		return "error: User denied action"
	}
	if t.UseSpinner() {
		r.console.StartSpinner("Running...")
	}
	result := t.Run(args)
	if t.UseSpinner() {
		r.console.EndSpinner()
	}

	content := result.Content
	display := ""
	switch v := content.(type) {
	case map[string]any:
		if v["type"] == "image" {
			if s, ok := v["text"].(string); ok {
				display = s
			} else {
				display = "[image]"
			}
		} else {
			display = fmt.Sprintf("%v", v)
		}
	case nil:
		display = ""
	case string:
		display = v
	default:
		display = fmt.Sprintf("%v", v)
	}

	r.echoResult(t, display)
	r.console.ToolResult(result.Success)
	return content
}

// echoResult 按 PreviewLines/PreviewWidth 截断回显工具结果。
func (r *Registry) echoResult(t Tool, display string) {
	if display == "" {
		fmt.Printf("  %s⎿  (no output)%s\n", ui.Dim, ui.Reset)
		return
	}
	lines := strings.Split(display, "\n")
	limit := t.PreviewLines()
	width := t.PreviewWidth()
	show := lines
	if len(lines) > limit {
		show = lines[:limit]
	}
	preview := make([]string, 0, len(show)+1)
	for _, line := range show {
		if len(line) > width {
			line = line[:width-3] + "..."
		}
		preview = append(preview, line)
	}
	if len(lines) > limit {
		more := len(lines) - limit
		suffix := ""
		if more > 1 {
			suffix = "s"
		}
		preview = append(preview, fmt.Sprintf("... and %d more line%s", more, suffix))
	}
	for i, line := range preview {
		if i == 0 {
			fmt.Printf("  %s⎿  %s%s\n", ui.Dim, line, ui.Reset)
		} else {
			fmt.Printf("     %s%s%s\n", ui.Dim, line, ui.Reset)
		}
	}
}
