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
}

// Console 是进程级默认 Printer。
var Console = NewPrinter()

// NewPrinter 创建一个 Printer。
func NewPrinter() *Printer {
	return &Printer{stdin: bufio.NewReader(os.Stdin)}
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
	if p.running {
		clearSpinnerLine()
	}
	fmt.Fprintln(os.Stdout, text)
	if p.running {
		p.renderFrame("⠋")
	}
}

// Section 打印一个小节标题。
func (p *Printer) Section(title string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writeLineLocked("")
	p.writeLineLocked(C("• "+title, Orange))
}

// ToolCall 打印工具调用抬头。
func (p *Printer) ToolCall(name, desc string) {
	p.Section(i18n.T("tool.call"))
	p.writeLine(C("› ", Grey) + C(name, Cyan) + "  " + C(desc, Grey))
}

// ToolResult 打印工具执行结果标记。
func (p *Printer) ToolResult(ok bool) {
	icon, color, key := "✓", Green, "tool.result.ok"
	if !ok {
		icon, color, key = "✗", Red, "tool.result.fail"
	}
	p.writeLine("  " + C(icon, color) + C(i18n.T(key), Grey))
}

// Success 打印成功消息。
func (p *Printer) Success(msg string) { p.writeLine(C("✓ ", Green) + C(msg, Grey)) }

// Error 打印错误消息。
func (p *Printer) Error(msg string) { p.writeLine(C("✗ ", Red) + C(msg, Grey)) }

// Warning 打印警告消息。
func (p *Printer) Warning(msg string) { p.writeLine(C("! ", Yellow) + C(msg, Grey)) }

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
		p.writeLine("  " + C(line, Grey))
	}
}

// Output 打印模型输出内容。
func (p *Printer) Output(content string) {
	p.Section(i18n.T("llm.output"))
	for _, line := range strings.Split(content, "\n") {
		p.writeLine("  " + C(line, Soft))
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
