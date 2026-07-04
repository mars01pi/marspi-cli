package agent

import (
	"fmt"
	"os"
	"strings"

	"github.com/mars/marspi-cli/internal/ui"
)

// ConsoleSink 将 agent 事件渲染到 Printer（plain REPL / 非 TUI 模式）。
func ConsoleSink(p *ui.Printer) Handler {
	if p == nil {
		return func(Event) {}
	}
	return func(ev Event) {
		switch ev.Type {
		case EventRunStart:
			// 用户输入由 REPL 自行打印
		case EventRunEnd:
			// no-op
		case EventTurnStart:
			if !p.TUIMode() {
				p.RoundMarker(ev.Iteration)
			}
		case EventTurnEnd:
		case EventLLMStart:
			p.StartSpinner(ev.Text)
		case EventLLMEnd:
			p.EndSpinner()
			p.TokenUsage(ev.Iteration, ev.Usage.PromptTokens, ev.Usage.CompletionTokens,
				ev.ContextTokens, ev.MaxContext)
		case EventMessageStart:
			// 流式阶段：此处可显示占位；非流式跳过
		case EventMessageDelta:
			// 流式阶段：ConsoleSink 可逐行刷新；当前 loop 不 emit
		case EventMessageEnd:
			if ev.Reasoning != "" {
				p.Thinking(ev.Reasoning)
			}
			if ev.Content != "" && !ev.HasToolCalls {
				p.Output(ev.Content)
			}
		case EventToolStart, EventToolUpdate, EventToolEnd:
			// 工具细节仍由 tool.Registry → Printer 输出
		case EventWarn:
			p.Warning(ev.Text)
		case EventError:
			reportError(p, ev.Text)
		}
	}
}

// reportError 输出错误；同时写 stderr，避免 spinner 在非 TTY 下吞掉输出。
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

// llmSpinnerText 返回 LLM 请求 spinner 文案。
func llmSpinnerText() string { return "Request..." }
