package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/mars/marspi-cli/internal/agent"
	"github.com/mars/marspi-cli/internal/agentctx"
	"github.com/mars/marspi-cli/internal/config"
	"github.com/mars/marspi-cli/internal/i18n"
	"github.com/mars/marspi-cli/internal/logx"
	"github.com/mars/marspi-cli/internal/llm"
	"github.com/mars/marspi-cli/internal/memory"
	"github.com/mars/marspi-cli/internal/prompt"
	"github.com/mars/marspi-cli/internal/skill"
	"github.com/mars/marspi-cli/internal/tool"
	"github.com/mars/marspi-cli/internal/ui"
)

// ErrConfig 表示启动前配置校验未通过（错误信息已输出到终端）。
var ErrConfig = errors.New("configuration incomplete")

// App 聚合一次 CLI 运行的全部组件。
type App struct {
	cfg      *config.Config
	console  *ui.Printer
	provider llm.Provider
	routed   *llm.RoutedProvider
	registry *tool.Registry
	mem      *memory.Manager
	skills   *skill.Manager
	prompt   *prompt.System
	runner   *agent.Runner
}

// NewApp 依据配置装配 App。
func NewApp(cfg *config.Config) *App {
	i18n.SetLang(cfg.Lang)
	console := ui.Console

	mem := memory.New(cfg.MemoryDir)
	skills := skill.New(nil)
	skills.AddBasePath(filepath.Join(cfg.BasePersist, "skills"))

	registry := tool.NewRegistry(cfg, console, mem, skills)

	var provider llm.Provider = llm.NewProvider(cfg.Model, cfg.APIURL, cfg.APIKey)
	var routed *llm.RoutedProvider
	if cfg.Routing == "on" {
		if rp, err := llm.NewRoutedProviderFromFile(cfg.ProvidersFile); err == nil {
			routed = rp
			provider = rp
		} else {
			console.Warning("Failed to load " + cfg.ProvidersFile + ", falling back to single provider")
		}
	}
	sys := prompt.NewSystem(cfg, skills)

	runner := &agent.Runner{
		Provider:   provider,
		Registry:   registry,
		Console:    console,
		MaxContext: cfg.MaxContext,
		MaxIter:    cfg.MaxIter,
	}

	return &App{
		cfg: cfg, console: console, provider: provider, routed: routed, registry: registry,
		mem: mem, skills: skills, prompt: sys, runner: runner,
	}
}

// Run 启动交互式 REPL
func (a *App) Run() error {
	if err := a.cfg.Initialize(); err != nil {
		return err
	}
	if ok, msg := a.cfg.ProviderReady(); !ok {
		a.console.Error(strings.SplitN(msg, "\n", 2)[0])
		for _, line := range strings.Split(msg, "\n")[1:] {
			if line != "" {
				a.console.Text(line)
			}
		}
		return ErrConfig
	}

	ctxFile := filepath.Join(a.cfg.SessionDir, "session.json")
	ctx := agentctx.New(a.cfg.MaxContext, a.provider, a.registry.Schemas(), a.console)
	ctx.Load(ctxFile)

	systemPrompt := a.prompt.Assemble()
	if ctx.Len() == 0 {
		ctx.AppendSystem(systemPrompt)
	}

	if os.Getenv("MARS_PLAIN") == "1" || !isatty.IsTerminal(os.Stdin.Fd()) {
		return a.runPlain(ctx, ctxFile, systemPrompt)
	}
	return a.runTUI(ctx, ctxFile, systemPrompt)
}

// runPlain 使用单行 bufio REPL（管道或非 TTY 时）。
func (a *App) runPlain(ctx *agentctx.Manager, ctxFile, systemPrompt string) error {
	mode := a.provider.Model()
	if a.routed != nil {
		mode = fmt.Sprintf("smart-routing[%d]", a.routed.TotalProviders())
	}
	fmt.Printf("%sMarspi Cli v%s%s | %s%s | %s%s\n\n",
		ui.Bold, config.Version, ui.Reset, ui.Dim, mode, a.cfg.ProjectRoot, ui.Reset)
	if logx.Enabled() {
		a.console.Text("debug logging enabled (MARS_DEBUG=1)")
	}
	logx.Debugf("provider model=%s url=%s routing=%s", a.provider.Model(), a.provider.APIURL(), a.cfg.Routing)

	reader := bufio.NewReader(os.Stdin)
	for {
		a.console.Separator()
		fmt.Printf("%s%s❯%s ", ui.Bold, ui.Blue, ui.Reset)
		line, err := reader.ReadString('\n')
		if err != nil {
			break // EOF / Ctrl-D
		}
		userInput := strings.TrimSpace(line)
		if userInput == "" {
			continue
		}

		if handled, quit := a.handleCommand(userInput, ctx, ctxFile, systemPrompt); quit {
			return nil
		} else if handled {
			continue
		}

		logx.Debugf("user input: %q", userInput)
		normal := userInput + ", Current date: " + nowStr()
		if a.routed != nil {
			a.routed.Route(userInput, ctx.ToolFingerprint(10))
		}
		a.runner.Loop(ctx, ctxFile, normal)
		ctx.ClearRuntimeInjections()
		_ = ctx.Save(ctxFile)
	}
	return nil
}

// handleCommand 处理内置斜杠命令。
// 返回 (handled, quit)：handled=true 表示斜杠命令已消费；quit=true 表示退出 REPL。
// 普通用户输入返回 (false, false)，由调用方进入 agent loop。
func (a *App) handleCommand(userInput string, ctx *agentctx.Manager, ctxFile, systemPrompt string) (handled, quit bool) {
	switch {
	case userInput == "/q" || userInput == "/quit":
		return true, true
	case userInput == "/stop" || userInput == "/s":
		a.console.Warning("No task running (use Esc during a task in TUI mode).")
		return true, false
	case userInput == "/c" || userInput == "/compact":
		if err := ctx.FullCompact(); err != nil {
			a.console.Error(err.Error())
		} else {
			a.console.Success("Full compact success.")
		}
		return true, false
	case userInput == "/n" || userInput == "/new":
		ctx.Backup(ctxFile)
		ctx.Clear()
		ctx.AppendSystem(systemPrompt)
		a.console.Success("New session created.")
		return true, false
	case userInput == "/h" || userInput == "/help":
		a.helper()
		return true, false
	case strings.HasPrefix(userInput, "/g") || strings.HasPrefix(userInput, "/goal"):
		a.console.Warning("/goal is deprecated. Use /loop <goal> instead.")
		return true, false
	case strings.HasPrefix(userInput, "/l") || strings.HasPrefix(userInput, "/loop"):
		parts := strings.SplitN(userInput, " ", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			a.console.Error("Please input '/l or /loop <query>'")
			return true, false
		}
		goal := strings.TrimSpace(parts[1])
		a.console.Success("🎯 Loop: " + goal)
		a.runLoopEngine(goal, 5)
		return true, false
	}
	return false, false
}

func (a *App) helper() {
	a.console.Text(i18n.T("cli.welcome"))
	a.console.Text(i18n.T("cli.help_intro"))
	for _, h := range i18n.HelpCommands {
		a.console.Text(fmt.Sprintf("  %-16s -- %s", h.Cmd, h.Desc()))
	}
}
