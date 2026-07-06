// Package agent 实现 ReAct 主循环与多智能体 loop-engine。
package agent

import (
	"context"
	"errors"
	"strings"

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
	Events     *Emitter
	MaxContext int
	MaxIter    int
	Stream     bool // MARS_STREAM
}

// Loop 运行一次完整的 agent_loop：追加用户输入，迭代调用模型与工具直至结束。
func (r *Runner) Loop(ctx *agentctx.Manager, ctxFilePath, userInput string) {
	r.LoopCtx(context.Background(), ctx, ctxFilePath, userInput)
}

// LoopCtx 与 Loop 相同，但可通过 ctx 在迭代间隙取消。
func (r *Runner) LoopCtx(runCtx context.Context, ctx *agentctx.Manager, ctxFilePath, userInput string) {
	ctx.AppendUser(userInput)
	tools := r.Registry.Schemas()
	logx.Debugf("agent loop start: model=%s tools=%d stream=%v", r.Provider.Model(), len(tools), r.Stream)

	r.emit(Event{Type: EventRunStart, UserInput: userInput})

	iteration := 0
	for {
		if err := runCtx.Err(); err != nil {
			r.emit(Event{Type: EventWarn, Text: "Stopped."})
			break
		}
		if strings.TrimSpace(r.Provider.APIKey()) == "" {
			r.emit(Event{Type: EventError, Text: "MARS_KEY is not set (required)\n  export MARS_KEY=sk-your-key"})
			break
		}

		iteration++
		r.emit(Event{Type: EventTurnStart, Iteration: iteration})
		r.emit(Event{Type: EventLLMStart, Text: llmSpinnerText()})

		msgs := ctx.PrepareForAPI()
		logx.Debugf("request iteration=%d messages=%d", iteration, len(msgs))

		resp, err := r.callLLM(runCtx, msgs, tools, iteration)
		r.emit(Event{Type: EventLLMEnd, Iteration: iteration, Usage: resp.Usage,
			ContextTokens: ctx.TotalTokens(), MaxContext: r.MaxContext})

		if err != nil {
			if errors.Is(err, context.Canceled) {
				r.emit(Event{Type: EventWarn, Text: "Stopped."})
			} else {
				r.emit(Event{Type: EventError, Text: "request failed: " + err.Error()})
			}
			break
		}

		if resp.FinishReason == "error" {
			r.emit(Event{Type: EventError, Text: resp.Content})
			break
		}

		ctx.AppendAssistant(resp.RawMessage)
		r.emit(Event{Type: EventTurnEnd, Iteration: iteration})

		if resp.Content == "" && resp.ReasoningContent == "" && !resp.HasToolCalls {
			r.emit(Event{Type: EventError, Text: formatEmptyResponse(resp, r.Provider.Model())})
			break
		}

		if resp.FinishReason == "stop" {
			break
		}
		if resp.HasToolCalls {
			completed := false
			for _, tc := range resp.ToolCalls {
				if err := runCtx.Err(); err != nil {
					r.emit(Event{Type: EventWarn, Text: "Stopped."})
					completed = true
					break
				}
				logx.Debugf("tool call: %s", tc.Name)
				result, done := r.executeTool(iteration, tc)
				ctx.AppendTool(tc.ID, tc.Name, result)
				if done {
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

	r.emit(Event{Type: EventRunEnd})
	if ctxFilePath != "" {
		_ = ctx.Save(ctxFilePath)
	}
}

func (r *Runner) emit(ev Event) {
	if r.Events != nil {
		r.Events.Emit(ev)
	}
}

func formatEmptyResponse(resp llm.Response, model string) string {
	return "empty model response (finish_reason=" + resp.FinishReason + ", model=" + model + ")"
}

func toolResultOK(result any) bool {
	if s, ok := result.(string); ok {
		return !strings.HasPrefix(s, "error:") && !strings.HasPrefix(s, "run tool ")
	}
	return true
}
