package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/mars/marspi-cli/internal/i18n"
	"github.com/mars/marspi-cli/internal/ui"
	"github.com/mars/marspi-core/agent"
)

type consoleStreamState struct {
	spinnerStopped bool
	thinkingOpen   bool
	outputOpen     bool
	thinkingLen    int
	outputLen      int
}

func (s *consoleStreamState) reset() {
	*s = consoleStreamState{}
}

// ConsoleSink 将 agent 事件渲染到 Printer（plain REPL / 非 TUI 模式）。
func ConsoleSink(p *ui.Printer) agent.Handler {
	if p == nil {
		return func(agent.Event) {}
	}
	var stream consoleStreamState
	return func(ev agent.Event) {
		switch ev.Type {
		case agent.EventRunStart:
		case agent.EventRunEnd:
		case agent.EventTurnStart:
			if !p.TUIMode() {
				p.RoundMarker(ev.Iteration)
			}
		case agent.EventTurnEnd:
		case agent.EventLLMStart:
			p.StartSpinner(ev.Text)
		case agent.EventLLMEnd:
			p.EndSpinner()
			stream.spinnerStopped = true
			p.TokenUsage(ev.Iteration, ev.Usage.PromptTokens, ev.Usage.CompletionTokens,
				ev.ContextTokens, ev.MaxContext)
		case agent.EventMessageStart:
			stream.reset()
		case agent.EventMessageDelta:
			if p.TUIMode() {
				return
			}
			stream.onDelta(p, ev)
		case agent.EventMessageEnd:
			if p.TUIMode() {
				return
			}
			if ev.Streamed {
				stream.finishPlain(p)
				return
			}
			if ev.Reasoning != "" {
				p.Thinking(ev.Reasoning)
			}
			if ev.Content != "" && !ev.HasToolCalls {
				p.Output(ev.Content)
			}
		case agent.EventToolStart:
			if p.TUIMode() {
				return
			}
			renderToolStartPlain(p, ev)
		case agent.EventToolUpdate:
			if p.TUIMode() {
				return
			}
			if ev.Text != "" {
				p.StartSpinner(ev.Text)
			}
		case agent.EventToolEnd:
			if p.TUIMode() {
				return
			}
			renderToolEndPlain(p, ev)
		case agent.EventWarn:
			p.Warning(ev.Text)
		case agent.EventError:
			reportError(p, ev.Text)
		}
	}
}

func (s *consoleStreamState) onDelta(p *ui.Printer, ev agent.Event) {
	if !s.spinnerStopped {
		p.EndSpinner()
		s.spinnerStopped = true
	}
	switch ev.DeltaField {
	case agent.DeltaReasoning:
		if !s.thinkingOpen {
			s.thinkingOpen = true
			p.Section(i18n.T("llm.thinking"))
		}
		if n := len(ev.Delta); n > s.thinkingLen {
			streamPrint(p, "  "+ui.Dim+ev.Delta[s.thinkingLen:]+ui.Reset)
			s.thinkingLen = n
		}
	case agent.DeltaContent:
		if !s.outputOpen {
			s.outputOpen = true
			p.Section(i18n.T("llm.output"))
		}
		if n := len(ev.Delta); n > s.outputLen {
			streamPrint(p, "  "+ui.Soft+ev.Delta[s.outputLen:]+ui.Reset)
			s.outputLen = n
		}
	}
}

func (s *consoleStreamState) finishPlain(p *ui.Printer) {
	if s.thinkingOpen || s.outputOpen {
		streamPrint(p, "")
	}
	s.reset()
}

func streamPrint(p *ui.Printer, text string) {
	if !p.TUIMode() {
		fmt.Fprint(os.Stdout, text)
	}
}

func reportError(console *ui.Printer, msg string) {
	first, rest, _ := strings.Cut(msg, "\n")
	console.Error(first)
	if rest != "" {
		console.Text(rest)
	}
	fmt.Fprintln(os.Stderr, ui.Red+"✗ "+first+ui.Reset)
	if rest != "" {
		fmt.Fprintln(os.Stderr, ui.Dim+rest+ui.Reset)
	}
}

func llmSpinnerText() string { return "Request..." }

func renderToolStartPlain(p *ui.Printer, ev agent.Event) {
	p.Section(i18n.T("tool.call"))
	line := ui.C("› ", ui.Grey) + ui.C(ev.ToolName, ui.Cyan)
	if ev.ToolPreview != "" {
		line += "  " + ui.C(ev.ToolPreview, ui.Grey)
	}
	p.Text(line)
}

func renderToolEndPlain(p *ui.Printer, ev agent.Event) {
	renderToolPreviewPlain(p, ev.ToolResultLines)
	if ev.ToolDenied {
		p.Warning(i18n.T("tool.denied"))
		return
	}
	if ev.ToolOK {
		p.Success(i18n.T("tool.result.ok"))
	} else {
		p.Error(i18n.T("tool.result.fail"))
	}
}

func renderToolPreviewPlain(p *ui.Printer, lines []string) {
	if len(lines) == 0 {
		p.Text("  " + ui.C("⎿  (no output)", ui.Dim))
		return
	}
	for i, line := range lines {
		if i == 0 {
			p.Text("  " + ui.C("⎿  "+line, ui.Dim))
		} else {
			p.Text("     " + ui.C(line, ui.Dim))
		}
	}
}
