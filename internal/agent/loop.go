// Package agent 实现 ReAct 主循环与多智能体 loop-engine。
// 对齐 mangopi 的 agent_loop 与 loop_engine。
package agent

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mars/marspi-cli/internal/agentctx"
	"github.com/mars/marspi-cli/internal/llm"
	"github.com/mars/marspi-cli/internal/tool"
	"github.com/mars/marspi-cli/internal/ui"
)

// Runner 承载运行一次 agent_loop 所需的依赖。
type Runner struct {
	Provider   llm.Provider
	Registry   *tool.Registry
	Console    *ui.Printer
	MaxContext int
	MaxIter    int
}

// Loop 运行一次完整的 agent_loop：追加用户输入，迭代调用模型与工具直至结束。
// 对齐 mangopi 的 agent_loop。
func (r *Runner) Loop(ctx *agentctx.Manager, ctxFilePath, userInput string) {
	ctx.AppendUser(userInput)
	tools := r.Registry.Schemas()

	iteration := 0
	for {
		if strings.TrimSpace(r.Provider.APIKey()) == "" {
			reportError(r.Console, "MARS_KEY is not set (required)\n  export MARS_KEY=sk-your-key")
			break
		}
		r.Console.StartSpinner("Request...")
		raw, err := llm.Request(r.Provider.APIURL(), r.Provider.BuildBody(ctx.PrepareForAPI(), tools),
			r.Provider.Headers(), 300*time.Second, 3)
		r.Console.EndSpinner()
		if err != nil {
			reportError(r.Console, "request failed: "+err.Error())
			break
		}
		resp := r.Provider.ParseResponse(raw)
		ctx.AppendAssistant(resp.RawMessage)

		iteration++
		r.Console.TokenUsage(iteration, resp.Usage.PromptTokens, resp.Usage.CompletionTokens,
			ctx.TotalTokens(), r.MaxContext)

		if resp.ReasoningContent != "" {
			r.Console.Thinking(resp.ReasoningContent)
		}
		if resp.Content != "" && !hasCompletion(resp.ToolCalls) {
			r.Console.Output(resp.Content)
		}

		if resp.FinishReason == "stop" {
			break
		}
		if resp.HasToolCalls {
			completed := false
			for _, tc := range resp.ToolCalls {
				result := r.Registry.Execute(tc.Name, tc.Arguments)
				ctx.AppendTool(tc.ID, tc.Name, result)
				if tc.Name == "attempt_completion" {
					completed = true
					break
				}
			}
			if completed {
				break
			}
		} else {
			break
		}
		if iteration == r.MaxIter {
			break
		}
	}
	if ctxFilePath != "" {
		_ = ctx.Save(ctxFilePath)
	}
}

func hasCompletion(calls []llm.ToolCall) bool {
	for _, tc := range calls {
		if tc.Name == "attempt_completion" {
			return true
		}
	}
	return false
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
