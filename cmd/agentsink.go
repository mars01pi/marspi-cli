package cmd

import (
	"github.com/mars/marspi-cli/internal/ui"
	"github.com/mars/marspi-core/agent"
)

// agentTUIHandler 订阅 agent.Events 并转发到 Bubble Tea event channel。
func agentTUIHandler(ch chan<- ui.Event) agent.Handler {
	return func(ev agent.Event) {
		ui.RenderAgentEvent(ch, mapAgentEvent(ev))
	}
}

func mapAgentEvent(ev agent.Event) ui.AgentEvent {
	out := ui.AgentEvent{
		Type:            ui.AgentEventKind(ev.Type),
		UserInput:       ev.UserInput,
		Iteration:       ev.Iteration,
		PromptTokens:    ev.Usage.PromptTokens,
		OutputTokens:    ev.Usage.CompletionTokens,
		ContextTokens:   ev.ContextTokens,
		MaxContext:      ev.MaxContext,
		Content:         ev.Content,
		Reasoning:       ev.Reasoning,
		HasToolCalls:    ev.HasToolCalls,
		Streamed:        ev.Streamed,
		Delta:           ev.Delta,
		Text:            ev.Text,
		ToolName:        ev.ToolName,
		ToolCallID:      ev.ToolCallID,
		ToolPreview:     ev.ToolPreview,
		ToolResultLines: ev.ToolResultLines,
		ToolOK:          ev.ToolOK,
		ToolDenied:      ev.ToolDenied,
	}
	switch ev.DeltaField {
	case agent.DeltaReasoning:
		out.DeltaField = "reasoning"
	case agent.DeltaContent:
		out.DeltaField = "content"
	}
	return out
}
