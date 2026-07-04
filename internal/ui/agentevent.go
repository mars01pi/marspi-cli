package ui

import (
	"fmt"
	"strings"

	"github.com/mars/marspi-cli/internal/i18n"
)

// AgentEventKind 与 agent.EventType 对应，避免 ui 依赖 agent 包。
type AgentEventKind string

const (
	AgentRunStart     AgentEventKind = "run_start"
	AgentRunEnd       AgentEventKind = "run_end"
	AgentTurnStart    AgentEventKind = "turn_start"
	AgentTurnEnd      AgentEventKind = "turn_end"
	AgentLLMStart     AgentEventKind = "llm_start"
	AgentLLMEnd       AgentEventKind = "llm_end"
	AgentMessageStart AgentEventKind = "message_start"
	AgentMessageDelta AgentEventKind = "message_delta"
	AgentMessageEnd   AgentEventKind = "message_end"
	AgentToolStart    AgentEventKind = "tool_start"
	AgentToolUpdate   AgentEventKind = "tool_update"
	AgentToolEnd      AgentEventKind = "tool_end"
	AgentWarn         AgentEventKind = "warn"
	AgentError        AgentEventKind = "error"
)

// AgentEvent 是 agent runtime 事件的 UI 侧镜像（由 cmd 从 agent.Event 转换）。
type AgentEvent struct {
	Type AgentEventKind

	UserInput     string
	Iteration     int
	PromptTokens  int
	OutputTokens  int
	ContextTokens int
	MaxContext    int

	Content      string
	Reasoning    string
	HasToolCalls bool
	Streamed     bool
	DeltaField   string // "content" | "reasoning"
	Delta        string

	Text string
}

func streamID(iteration int, field string) string {
	// reasoning 固定排在 content 前（显示顺序），与字典序无关
	rank := "1"
	if field == "reasoning" {
		rank = "0"
	}
	return fmt.Sprintf("%d-%s-%s", iteration, rank, field)
}

// RenderAgentEvent 将 agent 镜像事件转为 TUI ui.Event 并写入 channel。
func RenderAgentEvent(ch chan<- Event, ev AgentEvent) {
	if ch == nil {
		return
	}
	send := func(e Event) {
		ch <- e
	}
	switch ev.Type {
	case AgentRunStart, AgentRunEnd:
	case AgentTurnStart:
		send(Event{Kind: EvLine, Text: fmt.Sprintf("── round %d ──", ev.Iteration), Style: "round"})
	case AgentTurnEnd:
	case AgentLLMStart:
		send(Event{Kind: EvSpinner, Text: ev.Text, Style: "start"})
	case AgentLLMEnd:
		send(Event{Kind: EvSpinner, Style: "stop"})
		if ev.MaxContext > 0 {
			pct := ev.ContextTokens * 100 / ev.MaxContext
			send(Event{Kind: EvStatus, Text: fmt.Sprintf("round %d | %s in / %s out | ctx %d%%",
				ev.Iteration, fmtK(ev.PromptTokens), fmtK(ev.OutputTokens), pct)})
		}
	case AgentMessageStart:
	case AgentMessageDelta:
		style := "output"
		title := i18n.T("llm.output")
		if ev.DeltaField == "reasoning" {
			style = "thinking"
			title = i18n.T("llm.thinking")
		}
		id := streamID(ev.Iteration, ev.DeltaField)
		send(Event{Kind: EvStreamDelta, StreamID: id, Text: ev.Delta, Style: style, Title: title})
	case AgentMessageEnd:
		if ev.Streamed {
			if ev.Reasoning != "" {
				send(Event{Kind: EvStreamEnd, StreamID: streamID(ev.Iteration, "reasoning"), Style: "thinking", Title: i18n.T("llm.thinking")})
			}
			if ev.Content != "" && !ev.HasToolCalls {
				send(Event{Kind: EvStreamEnd, StreamID: streamID(ev.Iteration, "content"), Style: "output", Title: i18n.T("llm.output")})
			}
			return
		}
		if ev.Reasoning != "" {
			send(Event{Kind: EvSection, Title: i18n.T("llm.thinking")})
			for _, line := range truncateLines(strings.Split(ev.Reasoning, "\n"), 30) {
				send(Event{Kind: EvLine, Text: line, Style: "thinking"})
			}
		}
		if ev.Content != "" && !ev.HasToolCalls {
			send(Event{Kind: EvSection, Title: i18n.T("llm.output")})
			for _, line := range strings.Split(ev.Content, "\n") {
				send(Event{Kind: EvLine, Text: line, Style: "output"})
			}
		}
	case AgentToolStart, AgentToolUpdate, AgentToolEnd:
	case AgentWarn:
		send(Event{Kind: EvWarning, Text: ev.Text})
	case AgentError:
		send(Event{Kind: EvError, Text: ev.Text})
	}
}

func truncateLines(lines []string, max int) []string {
	if len(lines) <= max {
		return lines
	}
	return append(lines[:max], fmt.Sprintf("… (%d more lines)", len(lines)-max))
}
