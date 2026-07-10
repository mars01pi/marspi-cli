package cmd

// Supervisor engine for star-topology multi-agent orchestration with HITL approval.
import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mars/marspi-graph/orchestrator"
)

// runSupervisorEngine runs a star-topology supervisor workflow via marspi-graph.
// 用 marspi-graph Supervisor 星型编排跑动态多 Agent。
// 实验命令 /supervise /sv，与 /loop /loopg 并存。
// 派发 coder 前会经 Confirm 审批（HITL）。
func (a *App) runSupervisorEngine(ctx context.Context, goal string, maxSteps int) {
	if ctx == nil {
		ctx = context.Background()
	}
	threadID := fmt.Sprintf("supervisor-%d", time.Now().UnixNano())
	res, err := orchestrator.RunSupervisor(ctx, orchestrator.SupervisorConfig{
		Goal:               goal,
		MaxSteps:           maxSteps,
		SystemPrompt:       a.prompt.Assemble(),
		Provider:           a.provider,
		Registry:           a.registry,
		Reporter:           a.console,
		Events:             a.runner.Events,
		MaxContext:         a.cfg.MaxContext,
		MaxIterAgent:       a.cfg.MaxIter,
		Stream:             a.cfg.Stream,
		ThreadID:           threadID,
		RequireApprovalFor: []string{"coder"},
		OnInterrupt: func(runCtx context.Context, info orchestrator.InterruptInfo) (bool, error) {
			if err := runCtx.Err(); err != nil {
				return false, err
			}
			msg := formatHITLConfirm(info)
			ok := a.console.PromptApply(msg)
			if err := runCtx.Err(); err != nil {
				return false, err
			}
			return ok, nil
		},
		Workers: []orchestrator.WorkerSpec{
			{
				ID:           "researcher",
				Description:  "Read/search the codebase and gather facts; do not edit files",
				SystemPrompt: a.prompt.Assemble() + "\nYou are the RESEARCHER. Prefer read/grep. Do not edit unless essential.",
				AllowTools:   []string{"read", "grep", "search", "bash", "attempt_completion"},
			},
			{
				ID:           "coder",
				Description:  "Implement code changes with edit/write tools",
				SystemPrompt: a.prompt.Assemble() + "\nYou are the CODER. Implement the assigned task. Call attempt_completion when done.",
			},
			{
				ID:           "writer",
				Description:  "Draft summaries, docs, or final answers without heavy coding",
				SystemPrompt: a.prompt.Assemble() + "\nYou are the WRITER. Produce a clear final answer or doc. Prefer read-only tools.",
				AllowTools:   []string{"read", "grep", "search", "attempt_completion"},
			},
		},
	})
	if err != nil {
		if ctx.Err() != nil {
			a.console.Warning("Supervisor stopped.")
			return
		}
		if errors.Is(err, orchestrator.ErrApprovalDenied) {
			a.console.Warning("Handoff denied — supervisor stopped.")
			return
		}
		a.console.Error("Supervisor error: " + err.Error())
		return
	}
	a.console.Success(fmt.Sprintf("Supervisor done (last=%s)", res.State.GetString("last_agent")))
	if res.Message != "" {
		a.console.Text(res.Message)
	}
}

func formatHITLConfirm(info orchestrator.InterruptInfo) string {
	worker := info.Node
	reason, task := "", ""
	if m, ok := info.Value.(map[string]any); ok {
		if w, _ := m["worker"].(string); w != "" {
			worker = w
		}
		reason, _ = m["reason"].(string)
		task, _ = m["task"].(string)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Approve handoff to %s?", worker)
	if reason != "" {
		fmt.Fprintf(&b, "\nReason: %s", reason)
	}
	if task != "" {
		fmt.Fprintf(&b, "\nTask: %s", task)
	}
	return b.String()
}

// parseSuperviseGoal 解析 /supervise 或 /sv 命令。
func parseSuperviseGoal(userInput string) (goal string, ok bool) {
	var rest string
	switch {
	case strings.HasPrefix(userInput, "/supervise "):
		rest = userInput[len("/supervise "):]
	case userInput == "/supervise":
		return "", false
	case strings.HasPrefix(userInput, "/sv "):
		rest = userInput[len("/sv "):]
	case userInput == "/sv":
		return "", false
	default:
		return "", false
	}
	goal = strings.TrimSpace(rest)
	return goal, goal != ""
}
