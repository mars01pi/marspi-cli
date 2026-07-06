package tool

import (
	"fmt"
	"strings"
)

// RunMeta 是工具执行的展示元数据，供 agent 事件层使用。
type RunMeta struct {
	Success      bool
	Denied       bool
	PreviewLines []string
}

// Execute 执行一次工具调用并回显结果，返回给模型的内容（string 或 image map）。
// 对齐 mangopi 的 run_tool。Plain 非 agent 场景仍可使用。
func (r *Registry) Execute(name string, args map[string]any) any {
	content, meta := r.ExecuteQuiet(name, args)
	r.renderResult(name, args, content, meta)
	return content
}

// ExecuteQuiet 执行工具但不输出展示（由 agent 事件 / ConsoleSink 负责渲染）。
func (r *Registry) ExecuteQuiet(name string, args map[string]any) (content any, meta RunMeta) {
	t, ok := r.Get(name)
	if !ok {
		msg := "run tool " + name + " error: unknown tool"
		return msg, RunMeta{Success: false, PreviewLines: []string{msg}}
	}

	defer func() {
		if rec := recover(); rec != nil {
			r.console.EndSpinner()
		}
	}()

	t.Before(args)
	if !t.Confirm(args) {
		return "error: User denied action", RunMeta{Denied: true}
	}
	if t.UseSpinner() {
		r.console.StartSpinner("Running...")
	}
	result := t.Run(args)
	if t.UseSpinner() {
		r.console.EndSpinner()
	}

	display := formatResultDisplay(result.Content)
	return result.Content, RunMeta{
		Success:      result.Success,
		PreviewLines: FormatResultPreview(t, display),
	}
}

func (r *Registry) renderResult(name string, args map[string]any, content any, meta RunMeta) {
	t, ok := r.Get(name)
	if !ok {
		return
	}
	r.console.ToolCall(t.Name(), t.Preview(args))
	if meta.Denied {
		r.console.ToolResult(false)
		return
	}
	if len(meta.PreviewLines) == 0 {
		r.console.ToolPreview(nil)
	} else {
		r.console.ToolPreview(meta.PreviewLines)
	}
	r.console.ToolResult(meta.Success)
}

func formatResultDisplay(content any) string {
	switch v := content.(type) {
	case map[string]any:
		if v["type"] == "image" {
			if s, ok := v["text"].(string); ok {
				return s
			}
			return "[image]"
		}
		return fmt.Sprintf("%v", v)
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// FormatResultPreview 按工具截断规则生成结果预览行（对齐 echoResult / Printer.ToolPreview）。
func FormatResultPreview(t Tool, display string) []string {
	if display == "" {
		return nil
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
	return preview
}
