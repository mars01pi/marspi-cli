package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderInlineMarkdown 渲染行内 **bold** 与 `code`，其余交给 base 样式。
// 未闭合标记按纯文本输出（流式半截时不会死循环）。
func RenderInlineMarkdown(text string, base, bold, code lipgloss.Style) string {
	if text == "" || (!strings.Contains(text, "**") && !strings.Contains(text, "`")) {
		return base.Render(text)
	}

	var b strings.Builder
	i := 0
	for i < len(text) {
		if strings.HasPrefix(text[i:], "**") {
			if end := strings.Index(text[i+2:], "**"); end >= 0 {
				b.WriteString(bold.Render(text[i+2 : i+2+end]))
				i += end + 4
				continue
			}
			// 未闭合：** 起直到结尾当纯文本（避免 next==i 死循环）
			b.WriteString(base.Render(text[i:]))
			break
		}
		if text[i] == '`' {
			if end := strings.IndexByte(text[i+1:], '`'); end >= 0 {
				b.WriteString(code.Render(text[i+1 : i+1+end]))
				i += end + 2
				continue
			}
			b.WriteString(base.Render(text[i:]))
			break
		}

		next := len(text)
		if j := strings.Index(text[i:], "**"); j >= 0 {
			next = i + j
		}
		if j := strings.IndexByte(text[i:], '`'); j >= 0 && i+j < next {
			next = i + j
		}
		if next <= i {
			next = i + 1
		}
		b.WriteString(base.Render(text[i:next]))
		i = next
	}
	return b.String()
}
