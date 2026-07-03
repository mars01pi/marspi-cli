package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mars/marspi-cli/internal/agentctx"
	"github.com/mars/marspi-cli/internal/config"
	"github.com/mars/marspi-cli/internal/logx"
	"github.com/mars/marspi-cli/internal/ui"
)

const (
	replInputMinRows = 1
	replInputMaxRows = 6
	replStatusRows   = 1
)

const replHelpHint = "Enter send · Shift+Enter newline · Esc stop · /help"

var (
	replUserStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	replSecStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	replDimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	replOutStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	replThinkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	replToolStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	replErrStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	replOkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	replWarnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	replDebugStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
	replSpinStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	replBarStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Background(lipgloss.Color("235"))
	replBoxStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
)

var replSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type agentEventMsg struct{ ev ui.Event }
type agentDoneMsg struct{}
type confirmRequestMsg struct {
	message string
	resp    chan bool
}
type tickMsg time.Time

type replModel struct {
	app          *App
	program      *tea.Program
	ctx          *agentctx.Manager
	ctxFile      string
	systemPrompt string
	header       string

	vp           viewport.Model
	ta           textarea.Model
	history      strings.Builder
	width        int
	height       int

	running      bool
	spinIdx      int
	spinText     string
	agentCancel  context.CancelFunc
	confirmIn    chan confirmRequestMsg

	confirmMsg   string
	confirmResp  chan bool

	statusBar    string
	quitting     bool
}

func newReplModel(a *App, ctx *agentctx.Manager, ctxFile, systemPrompt, header string) *replModel {
	ta := textarea.New()
	ta.Placeholder = "Message…  Enter to send · Shift+Enter newline"
	ta.Focus()
	ta.CharLimit = 0
	ta.SetWidth(80)
	ta.SetHeight(replInputMinRows)
	ta.ShowLineNumbers = false
	ta.Prompt = "❯ "

	vp := viewport.New(80, 20)
	vp.SetContent("")

	return &replModel{
		app: a, ctx: ctx, ctxFile: ctxFile, systemPrompt: systemPrompt, header: header,
		vp: vp, ta: ta, confirmIn: make(chan confirmRequestMsg),
		statusBar: replHelpHint,
	}
}

func (m *replModel) inputLineCount() int {
	n := strings.Count(m.ta.Value(), "\n") + 1
	if n < replInputMinRows {
		return replInputMinRows
	}
	if n > replInputMaxRows {
		return replInputMaxRows
	}
	return n
}

func (m *replModel) adjustInputHeight() {
	rows := m.inputLineCount()
	if m.ta.Height() != rows {
		m.ta.SetHeight(rows)
	}
	m.resizeViewport()
}

func (m *replModel) resizeViewport() {
	if m.width == 0 || m.height == 0 {
		return
	}
	// header(1) + input border(2) + status(1) + gaps(2)
	footer := m.inputLineCount() + replStatusRows + 5
	vpH := m.height - footer
	if vpH < 4 {
		vpH = 4
	}
	m.vp.Width = m.width - 2
	m.vp.Height = vpH
}

func (a *App) runTUI(ctx *agentctx.Manager, ctxFile, systemPrompt string) error {
	mode := a.provider.Model()
	if a.routed != nil {
		mode = fmt.Sprintf("smart-routing[%d]", a.routed.TotalProviders())
	}
	header := fmt.Sprintf("Marspi Cli v%s · %s · %s", config.Version, mode, a.cfg.ProjectRoot)
	if logx.Enabled() {
		header += " · debug"
	}

	m := newReplModel(a, ctx, ctxFile, systemPrompt, header)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program = p
	m.installUIHooks()

	if logx.Enabled() {
		logx.SetSink(func(msg string) {
			p.Send(agentEventMsg{ev: ui.Event{Kind: ui.EvLine, Text: msg, Style: "debug"}})
		})
		defer logx.SetSink(nil)
	}

	// 将 agent 线程里的确认请求转发到 Bubble Tea 主循环。
	go func() {
		for req := range m.confirmIn {
			p.Send(req)
		}
	}()

	_, err := p.Run()
	a.console.SetHooks(nil)
	return err
}

func (m *replModel) installUIHooks() {
	m.app.console.SetHooks(&ui.Hooks{
		Silent: true,
		OnEvent: func(ev ui.Event) {
			if m.program != nil {
				m.program.Send(agentEventMsg{ev})
			}
		},
	})
}

func (m *replModel) agentHooks() *ui.Hooks {
	confirmIn := m.confirmIn
	program := m.program
	return &ui.Hooks{
		Silent: true,
		OnEvent: func(ev ui.Event) {
			if program != nil {
				program.Send(agentEventMsg{ev})
			}
		},
		Confirm: func(message string) bool {
			resp := make(chan bool, 1)
			confirmIn <- confirmRequestMsg{message: message, resp: resp}
			return <-resp
		},
	}
}

func (m *replModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.tickCmd())
}

func (m *replModel) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *replModel) appendRendered(line string) {
	if m.history.Len() > 0 {
		m.history.WriteByte('\n')
	}
	m.history.WriteString(line)
	m.vp.SetContent(m.history.String())
	m.vp.GotoBottom()
}

func (m *replModel) renderEvent(ev ui.Event) {
	switch ev.Kind {
	case ui.EvSection:
		m.appendRendered(replSecStyle.Render("• " + ev.Title))
	case ui.EvLine:
		switch ev.Style {
		case "thinking":
			m.appendRendered("  " + replThinkStyle.Render(ev.Text))
		case "output":
			m.appendRendered("  " + replOutStyle.Render(ev.Text))
		case "tool":
			m.appendRendered(replToolStyle.Render(ev.Text))
		case "tool-result":
			m.appendRendered(replDimStyle.Render(ev.Text))
		case "debug":
			m.appendRendered(replDebugStyle.Render("◦ debug  " + ev.Text))
		default:
			m.appendRendered(replDimStyle.Render(ev.Text))
		}
	case ui.EvError:
		m.appendRendered(replErrStyle.Render("✗ " + ev.Text))
	case ui.EvSuccess:
		m.appendRendered(replOkStyle.Render("✓ " + ev.Text))
	case ui.EvWarning:
		m.appendRendered(replWarnStyle.Render("! " + ev.Text))
	case ui.EvStatus:
		m.statusBar = ev.Text
	case ui.EvSpinner:
		if ev.Style == "start" {
			m.spinText = ev.Text
		}
	}
}

func (m *replModel) startAgent(userInput string) tea.Cmd {
	m.running = true
	m.spinText = "Running…"
	m.statusBar = "Agent running — Esc or /stop to cancel"
	m.appendRendered("")
	m.appendRendered(replUserStyle.Render("You"))
	m.appendRendered(replOutStyle.Render(userInput))

	ctx, cancel := context.WithCancel(context.Background())
	m.agentCancel = cancel

	m.app.console.SetHooks(m.agentHooks())

	normal := userInput + ", Current date: " + nowStr()
	if m.app.routed != nil {
		m.app.routed.Route(userInput, m.ctx.ToolFingerprint(10))
	}

	go func() {
		m.app.runner.LoopCtx(ctx, m.ctx, m.ctxFile, normal)
		m.ctx.ClearRuntimeInjections()
		_ = m.ctx.Save(m.ctxFile)
		if m.program != nil {
			m.program.Send(agentDoneMsg{})
		}
	}()

	return nil
}

func (m *replModel) cancelAgent() {
	if m.agentCancel != nil {
		m.agentCancel()
	}
}

func (m *replModel) finishAgent() {
	m.running = false
	m.spinText = ""
	m.agentCancel = nil
	m.statusBar = replHelpHint
	m.installUIHooks()
}

func (m *replModel) submit() (tea.Model, tea.Cmd) {
	if m.confirmResp != nil {
		return m, nil
	}
	input := strings.TrimSpace(m.ta.Value())
	if input == "" {
		return m, nil
	}
	m.ta.Reset()
	m.ta.SetHeight(replInputMinRows)
	m.resizeViewport()

	if m.running && (input == "/stop" || input == "/s") {
		m.cancelAgent()
		m.appendRendered(replWarnStyle.Render("Stopping…"))
		return m, nil
	}
	if m.running {
		m.appendRendered(replWarnStyle.Render("Agent is running — Esc or /stop to cancel"))
		return m, nil
	}

	if input == "/stop" || input == "/s" {
		m.appendRendered(replWarnStyle.Render("No task running."))
		return m, nil
	}

	handled, quit := m.app.handleCommand(input, m.ctx, m.ctxFile, m.systemPrompt)
	if quit {
		m.quitting = true
		return m, tea.Quit
	}
	if handled {
		return m, nil
	}

	logx.Debugf("user input: %q", input)
	return m, m.startAgent(input)
}

func (m *replModel) answerConfirm(yes bool) (tea.Model, tea.Cmd) {
	if m.confirmResp != nil {
		m.confirmResp <- yes
		m.confirmResp = nil
		m.confirmMsg = ""
		m.ta.Focus()
	}
	return m, nil
}

func (m *replModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ta.SetWidth(msg.Width - 4)
		m.resizeViewport()
		return m, nil

	case tickMsg:
		if m.running && m.spinText != "" {
			m.spinIdx++
		}
		return m, m.tickCmd()

	case confirmRequestMsg:
		m.confirmMsg = msg.message
		m.confirmResp = msg.resp
		m.ta.Blur()
		return m, nil

	case agentEventMsg:
		m.renderEvent(msg.ev)
		return m, nil

	case agentDoneMsg:
		m.finishAgent()
		return m, nil

	case tea.KeyMsg:
		if m.confirmResp != nil {
			switch strings.ToLower(msg.String()) {
			case "y", "enter":
				return m.answerConfirm(true)
			case "n", "esc":
				return m.answerConfirm(false)
			}
			return m, nil
		}

		if m.running {
			switch msg.String() {
			case "esc", "ctrl+c":
				m.cancelAgent()
				m.appendRendered(replWarnStyle.Render("Stopping…"))
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "enter":
			return m.submit()
		case "shift+enter":
			var cmd tea.Cmd
			m.ta, cmd = m.ta.Update(msg)
			m.adjustInputHeight()
			return m, cmd
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	m.adjustInputHeight()
	return m, cmd
}

func (m *replModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(replDimStyle.Render(m.header))
	b.WriteString("\n")
	b.WriteString(m.vp.View())
	b.WriteString("\n")

	if m.confirmMsg != "" {
		b.WriteString(replWarnStyle.Render(m.confirmMsg + "  [y/n]"))
		b.WriteString("\n")
	} else {
		b.WriteString(replBoxStyle.Width(m.width - 2).Render(m.ta.View()))
		b.WriteString("\n")
	}

	status := m.statusBar
	if m.running && m.spinText != "" {
		frame := replSpinnerFrames[m.spinIdx%len(replSpinnerFrames)]
		status = replSpinStyle.Render(frame) + " " + replSpinStyle.Render(m.spinText) + "  ·  " + status
	}
	b.WriteString(replBarStyle.Width(m.width).Render(" " + status))
	return b.String()
}
