package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mars/marspi-cli/internal/i18n"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Printer 负责所有终端输出，内含并发安全的 spinner。
type Printer struct {
	mu          sync.Mutex
	running     bool
	message     string
	stopCh      chan struct{}
	doneCh      chan struct{}
	stdin       *bufio.Reader
	hooks       *Hooks
}

// Console 是进程级默认 Printer。
var Console = NewPrinter()

// NewPrinter 创建一个 Printer。
func NewPrinter() *Printer {
	return &Printer{stdin: bufio.NewReader(os.Stdin)}
}

// SetHooks 配置 TUI 转发；hooks 为 nil 时恢复标准输出模式。
func (p *Printer) SetHooks(hooks *Hooks) { p.hooks = hooks }

func (p *Printer) emit(ev Event) {
	if p.hooks != nil && p.hooks.OnEvent != nil {
		p.hooks.OnEvent(ev)
	}
}

func (p *Printer) stdoutEnabled() bool {
	return p.hooks == nil || !p.hooks.Silent
}

func clearSpinnerLine() { fmt.Print("\r\033[K") }

func (p *Printer) renderFrame(frame string) {
	fmt.Print("\r" + C(frame, Orange) + " " + C(p.message, Orange))
}

// writeLine 输出一行，spinner 运行时先清行再恢复。
func (p *Printer) writeLine(text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writeLineLocked(text)
}

func (p *Printer) writeLineLocked(text string) {
	if p.running && p.stdoutEnabled() {
		clearSpinnerLine()
	}
	if p.stdoutEnabled() {
		fmt.Fprintln(os.Stdout, text)
	}
	p.emit(Event{Kind: EvLine, Text: text, Style: "dim"})
	if p.running && p.stdoutEnabled() {
		p.renderFrame("⠋")
	}
}

// Section 打印一个小节标题。
func (p *Printer) Section(title string) {
	p.emit(Event{Kind: EvSection, Title: title})
	if !p.stdoutEnabled() {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writeLineLocked("")
	p.writeLineLocked(C("• "+title, Orange))
}

// ToolCall 打印工具调用抬头。
func (p *Printer) ToolCall(name, desc string) {
	p.Section(i18n.T("tool.call"))
	line := C("› ", Grey) + C(name, Cyan) + "  " + C(desc, Grey)
	p.emit(Event{Kind: EvLine, Text: "› " + name + "  " + desc, Style: "tool"})
	if p.stdoutEnabled() {
		p.writeLine(line)
	}
}

// ToolResult 打印工具执行结果标记。
func (p *Printer) ToolResult(ok bool) {
	key := "tool.result.ok"
	icon, color := "✓", Green
	if !ok {
		key, icon, color = "tool.result.fail", "✗", Red
	}
	msg := i18n.T(key)
	if ok {
		p.emit(Event{Kind: EvSuccess, Text: msg})
	} else {
		p.emit(Event{Kind: EvError, Text: msg})
	}
	if !p.stdoutEnabled() {
		return
	}
	p.mu.Lock()
	p.writeLineLocked("  " + C(icon, color) + C(msg, Grey))
	p.mu.Unlock()
}

// ToolPreview 打印工具 stdout 预览（对齐 ⎿ 前缀格式）。
func (p *Printer) ToolPreview(lines []string) {
	if len(lines) == 0 {
		p.emit(Event{Kind: EvLine, Text: "⎿  (no output)", Style: "tool-result"})
		if p.stdoutEnabled() {
			p.mu.Lock()
			p.writeLineLocked("  " + C("⎿  (no output)", Dim))
			p.mu.Unlock()
		}
		return
	}
	for i, line := range lines {
		if i == 0 {
			p.emit(Event{Kind: EvLine, Text: "⎿  " + line, Style: "tool-result"})
		} else {
			p.emit(Event{Kind: EvLine, Text: "   " + line, Style: "tool-result"})
		}
		if !p.stdoutEnabled() {
			continue
		}
		p.mu.Lock()
		if i == 0 {
			p.writeLineLocked("  " + C("⎿  "+line, Dim))
		} else {
			p.writeLineLocked("     " + C(line, Dim))
		}
		p.mu.Unlock()
	}
}

// Error 打印错误消息。
func (p *Printer) Error(msg string) {
	p.emit(Event{Kind: EvError, Text: msg})
	if p.stdoutEnabled() {
		p.writeLine(C("✗ ", Red) + C(msg, Grey))
	}
}

// Warning 打印警告消息。
func (p *Printer) Warning(msg string) {
	p.emit(Event{Kind: EvWarning, Text: msg})
	if p.stdoutEnabled() {
		p.writeLine(C("! ", Yellow) + C(msg, Grey))
	}
}

// Success 打印成功消息。
func (p *Printer) Success(msg string) {
	p.emit(Event{Kind: EvSuccess, Text: msg})
	if p.stdoutEnabled() {
		p.writeLine(C("✓ ", Green) + C(msg, Grey))
	}
}

// Text 打印灰色文本。
func (p *Printer) Text(msg string) { p.writeLine(C(msg, Grey)) }

// Separator 打印分隔线。
func (p *Printer) Separator() {
	p.writeLine(Dim + strings.Repeat("─", 80) + Reset)
}

// Thinking 打印模型思考内容。
func (p *Printer) Thinking(content string) {
	p.Section(i18n.T("llm.thinking"))
	for _, line := range strings.Split(content, "\n") {
		p.emit(Event{Kind: EvLine, Text: line, Style: "thinking"})
		if p.stdoutEnabled() {
			p.writeLine("  " + C(line, Grey))
		}
	}
}

// Output 打印模型输出内容。
func (p *Printer) Output(content string) {
	p.Section(i18n.T("llm.output"))
	for _, line := range strings.Split(content, "\n") {
		p.emit(Event{Kind: EvLine, Text: line, Style: "output"})
		if p.stdoutEnabled() {
			p.writeLine("  " + C(line, Soft))
		}
	}
}

func fmtK(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func pctColor(percent int) string {
	switch {
	case percent < 50:
		return Green
	case percent < 70:
		return Yellow
	default:
		return Red
	}
}

// TokenUsage 打印一轮的 token 使用与上下文占用。
func (p *Printer) TokenUsage(iteration, inTok, outTok, ctxTok, maxCtx int) {
	percent := 0
	if maxCtx > 0 {
		percent = ctxTok * 100 / maxCtx
	}
	status := fmt.Sprintf("round %d | %s in / %s out | ctx %d%%",
		iteration, fmtK(inTok), fmtK(outTok), percent)
	p.emit(Event{Kind: EvStatus, Text: status})
	if !p.stdoutEnabled() {
		return
	}
	p.writeLine("")
	p.writeLine(
		C(fmt.Sprintf("%s: %d | %s: %s in / %s out |  ctx: ",
			i18n.T("context.round"), iteration, i18n.T("context.tokens_in_out"),
			fmtK(inTok), fmtK(outTok)), Grey) +
			C(fmt.Sprintf("%d%%", percent), pctColor(percent)))
}

// CompactStatus 打印上下文压缩的前后状态。
func (p *Printer) CompactStatus(before, after, maxCtx int, strategy string) {
	saved := before - after
	percent := 0
	if maxCtx > 0 {
		percent = after * 100 / maxCtx
	}
	p.Section(i18n.T("context.compact"))
	p.writeLine("  " + C(i18n.T("context.compact.strategy"), Grey) + " " + C(strategy, Orange))
	p.writeLine(fmt.Sprintf("  %s %s %s %s %s",
		C("tokens", Grey), C(fmt.Sprintf("%d", before), Red), C("→", Grey),
		C(fmt.Sprintf("%d", after), Green), C(fmt.Sprintf("(-%d)", saved), Orange)))
	p.writeLine("  " + C("context", Grey) + " " + C(fmt.Sprintf("%d%%", percent), pctColor(percent)))
}

// PromptApply 询问用户 y/n 确认，返回是否同意。
func (p *Printer) PromptApply(message string) bool {
	if p.hooks != nil && p.hooks.Confirm != nil {
		return p.hooks.Confirm(message)
	}
	for {
		fmt.Printf("%s%s [y/n]: %s", Yellow, message, Reset)
		line, err := p.stdin.ReadString('\n')
		if err != nil {
			return false
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			fmt.Println("input y or n")
		}
	}
}

// StartSpinner 启动后台 spinner。
func (p *Printer) StartSpinner(message string) {
	p.emit(Event{Kind: EvSpinner, Text: message, Style: "start"})
	if p.hooks != nil && p.hooks.Silent {
		return
	}
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	if message == "" {
		message = "Running..."
	}
	p.running = true
	p.message = message
	p.stopCh = make(chan struct{})
	p.doneCh = make(chan struct{})
	stop, done := p.stopCh, p.doneCh
	p.mu.Unlock()

	go func() {
		defer close(done)
		i := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				p.mu.Lock()
				if p.running {
					p.renderFrame(spinnerFrames[i%len(spinnerFrames)])
				}
				p.mu.Unlock()
				i++
			}
		}
	}()
}

// EndSpinner 停止 spinner 并清行。
func (p *Printer) EndSpinner() {
	p.emit(Event{Kind: EvSpinner, Style: "stop"})
	if p.hooks != nil && p.hooks.Silent {
		return
	}
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.stopCh)
	done := p.doneCh
	p.mu.Unlock()

	<-done
	p.mu.Lock()
	clearSpinnerLine()
	fmt.Fprintln(os.Stdout) // 换行，避免后续输出与 spinner 行重叠
	p.mu.Unlock()
}
