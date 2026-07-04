package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mars/marspi-cli/internal/llm"
	"github.com/mars/marspi-cli/internal/logx"
)

const llmTimeout = 300 * time.Second

// callLLM 发起一次模型请求（流式或非流式），并 emit 对应 message 事件。
func (r *Runner) callLLM(runCtx context.Context, msgs []llm.Message, tools []map[string]any, iteration int) (llm.Response, error) {
	r.emit(Event{Type: EventMessageStart, Iteration: iteration})

	var resp llm.Response
	var err error
	streamed := false

	if r.Stream {
		resp, streamed, err = r.requestStream(runCtx, msgs, tools, iteration)
		if err != nil && !errors.Is(err, context.Canceled) {
			logx.Debugf("stream failed, fallback blocking: %v", err)
			resp, err = r.requestBlocking(runCtx, msgs, tools)
			streamed = false
		}
	} else {
		resp, err = r.requestBlocking(runCtx, msgs, tools)
	}

	if err != nil {
		return resp, err
	}
	if resp.Model == "" {
		resp.Model = r.Provider.Model()
	}

	r.emit(Event{
		Type:         EventMessageEnd,
		Iteration:    iteration,
		Content:      resp.Content,
		Reasoning:    resp.ReasoningContent,
		FinishReason: resp.FinishReason,
		HasToolCalls: resp.HasToolCalls,
		Streamed:     streamed,
	})
	return resp, nil
}

func (r *Runner) requestBlocking(runCtx context.Context, msgs []llm.Message, tools []map[string]any) (llm.Response, error) {
	raw, err := llm.RequestContext(runCtx, r.Provider.APIURL(), r.Provider.BuildBody(msgs, tools),
		r.Provider.Headers(), llmTimeout, 3)
	if err != nil {
		return llm.Response{}, err
	}
	return r.Provider.ParseResponse(raw), nil
}

func (r *Runner) requestStream(runCtx context.Context, msgs []llm.Message, tools []map[string]any, iteration int) (llm.Response, bool, error) {
	streamed := false
	var reasoning, content strings.Builder
	resp, err := llm.StreamChat(runCtx, r.Provider.APIURL(), r.Provider.BuildBody(msgs, tools),
		r.Provider.Headers(), llmTimeout, func(ch llm.StreamChunk) error {
			if ch.Done {
				return nil
			}
			if ch.ReasoningDelta != "" {
				streamed = true
				reasoning.WriteString(ch.ReasoningDelta)
				r.emit(Event{Type: EventMessageDelta, Iteration: iteration, DeltaField: DeltaReasoning, Delta: reasoning.String()})
			}
			if ch.ContentDelta != "" {
				streamed = true
				content.WriteString(ch.ContentDelta)
				r.emit(Event{Type: EventMessageDelta, Iteration: iteration, DeltaField: DeltaContent, Delta: content.String()})
			}
			return runCtx.Err()
		})
	return resp, streamed, err
}
