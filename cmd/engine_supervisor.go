package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/mars/marspi-graph/orchestrator"
)

// runSupervisorEngine 用 marspi-graph Supervisor 星型编排跑动态多 Agent。
// 实验命令 /supervise /sv，与 /loop /loopg 并存。
func (a *App) runSupervisorEngine(goal string, maxSteps int) {
	res, err := orchestrator.RunSupervisor(context.Background(), orchestrator.SupervisorConfig{
		Goal:         goal,
		MaxSteps:     maxSteps,
		SystemPrompt: a.prompt.Assemble(),
		Provider:     a.provider,
		Registry:     a.registry,
		Reporter:     a.console,
		Events:       a.runner.Events,
		MaxContext:   a.cfg.MaxContext,
		MaxIterAgent: a.cfg.MaxIter,
		Stream:       a.cfg.Stream,
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
		a.console.Error("Supervisor error: " + err.Error())
		return
	}
	if e := res.State.GetString("error"); e != "" {
		a.console.Warning("Supervisor note: " + e)
	}
	a.console.Success(fmt.Sprintf("Supervisor done (last=%s)", res.State.GetString("last_agent")))
	if res.Message != "" {
		a.console.Text(res.Message)
	}
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
