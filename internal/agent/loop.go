// Package agent 实现 ReAct 主循环与多智能体 loop-engine。
// 对齐 mangopi 的 agent_loop 与 loop_engine。
package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mars/marspi-cli/internal/agentctx"
	"github.com/mars/marspi-cli/internal/llm"
	"github.com/mars/marspi-cli/internal/logx"
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
func (r *Runner) Loop(ctx *agentctx.Manager, ctxFilePath, userInput string) {
	r.LoopCtx(context.Background(), ctx, ctxFilePath, userInput)
}

// LoopCtx 与 Loop 相同，但可通过 ctx 在迭代间隙取消。
func (r *Runner) LoopCtx(runCtx context.Context, ctx *agentctx.Manager, ctxFilePath, userInput string) {
	ctx.AppendUser(userInput)
	tools := r.Registry.Schemas()
	logx.Debugf("agent loop start: model=%s tools=%d", r.Provider.Model(), len(tools))

	iteration := 0
	for {
		if err := runCtx.Err(); err != nil {
			r.Console.Warning("Stopped.")
			break
		}
		if strings.TrimSpace(r.Provider.APIKey()) == "" {
			reportError(r.Console, "MARS_KEY is not set (required)\n  export MARS_KEY=sk-your-key")
			break
		}
		r.Console.StartSpinner("Request...")
		msgs := ctx.PrepareForAPI()
		logx.Debugf("request iteration=%d messages=%d", iteration+1, len(msgs))
		raw, err := llm.RequestContext(runCtx, r.Provider.APIURL(), r.Provider.BuildBody(msgs, tools),
			r.Provider.Headers(), 300*time.Second, 3)
		r.Console.EndSpinner()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				r.Console.Warning("Stopped.")
			} else {
				reportError(r.Console, "request failed: "+err.Error())
			}
			break
		}
		resp := r.Provider.ParseResponse(raw)
		if resp.FinishReason == "error" {
			reportError(r.Console, resp.Content)
			break
		}
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

		if resp.Content == "" && resp.ReasoningContent == "" && !resp.HasToolCalls {
			reportError(r.Console, fmt.Sprintf(
				"empty model response (finish_reason=%q, model=%s)",
				resp.FinishReason, r.Provider.Model(),
			))
			break
		}

		if resp.FinishReason == "stop" {
			break
		}
		if resp.HasToolCalls {
			completed := false
			for _, tc := range resp.ToolCalls {
				if err := runCtx.Err(); err != nil {
					r.Console.Warning("Stopped.")
					completed = true
					break
				}
				logx.Debugf("tool call: %s", tc.Name)
				result := r.Registry.Execute(tc.Name, tc.Arguments)
				ctx.AppendTool(tc.ID, tc.Name, result)
				if tc.Name == "attempt_completion" {
					if s, ok := result.(string); ok && s != "" {
						r.Console.Output(s)
					}
					completed = true
					break
				}
			}
			if completed {
				break
			}
		} else {
			logx.Debugf("loop exit: finish_reason=%q no tool calls", resp.FinishReason)
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
