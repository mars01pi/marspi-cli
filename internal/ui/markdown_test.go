package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mars/marspi-cli/internal/i18n"
)

func TestRenderInlineMarkdown(t *testing.T) {
	base := lipgloss.NewStyle()
	bold := lipgloss.NewStyle().Bold(true)
	code := lipgloss.NewStyle().Foreground(lipgloss.Color("81"))

	out := RenderInlineMarkdown("see `main.go` and **PASS** ok", base, bold, code)
	if !strings.Contains(out, "main.go") || !strings.Contains(out, "PASS") {
		t.Fatalf("missing content: %q", out)
	}
	// 标记本身应被吃掉
	if strings.Contains(out, "**") || strings.Contains(out, "`") {
		t.Fatalf("raw markers left: %q", out)
	}
}

func TestRenderInlineMarkdown_plain(t *testing.T) {
	base := lipgloss.NewStyle()
	out := RenderInlineMarkdown("no markers here", base, base, base)
	if out == "" {
		t.Fatal("empty")
	}
}

func TestRenderInlineMarkdown_unclosedDoesNotHang(t *testing.T) {
	base := lipgloss.NewStyle()
	bold := lipgloss.NewStyle().Bold(true)
	code := lipgloss.NewStyle()

	done := make(chan string, 1)
	go func() {
		done <- RenderInlineMarkdown("当前进行中的**任务", base, bold, code)
	}()
	select {
	case out := <-done:
		if !strings.Contains(out, "当前进行中的") || !strings.Contains(out, "**") {
			t.Fatalf("unexpected: %q", out)
		}
	case <-time.After(time.Second):
		t.Fatal("RenderInlineMarkdown hung on unclosed **")
	}

	go func() {
		done <- RenderInlineMarkdown("code `partial", base, bold, code)
	}()
	select {
	case out := <-done:
		if !strings.Contains(out, "`") {
			t.Fatalf("unexpected: %q", out)
		}
	case <-time.After(time.Second):
		t.Fatal("RenderInlineMarkdown hung on unclosed backtick")
	}
}

func TestCollapseThinking(t *testing.T) {
	i18n.SetLang("en")
	got := CollapseThinking("line1\nline2\nline3")
	if got != "Thought · 3 lines" {
		t.Fatalf("got %q", got)
	}
}
