package cmd

import (
	"context"
	"strings"

	"github.com/mars/marspi-graph/orchestrator"
)

// runGraphLoopEngine 用 marspi-graph CodingLoop 跑 Implementer→Verifier→Updater。
// 与 runLoopEngine（命令式旧实现）并存，供 /loopg 试用。
func (a *App) runGraphLoopEngine(goal string, maxIter int) {
	res, err := orchestrator.RunCodingLoop(context.Background(), orchestrator.CodingLoopConfig{
		Goal:         goal,
		MaxIter:      maxIter,
		SystemPrompt: a.prompt.Assemble(),
		Provider:     a.provider,
		Registry:     a.registry,
		Reporter:     a.console,
		Events:       a.runner.Events,
		MaxContext:   a.cfg.MaxContext,
		MaxIterAgent: a.cfg.MaxIter,
		Stream:       a.cfg.Stream,
	})
	if err != nil {
		a.console.Error("Graph loop error: " + err.Error())
		return
	}
	if res.Success {
		a.console.Success(res.Message)
		return
	}
	a.console.Error(res.Message)
}

// parseGraphLoopGoal 解析 /loopg 或 /lg 命令。
func parseGraphLoopGoal(userInput string) (goal string, ok bool) {
	var rest string
	switch {
	case strings.HasPrefix(userInput, "/loopg "):
		rest = userInput[len("/loopg "):]
	case userInput == "/loopg":
		return "", false
	case strings.HasPrefix(userInput, "/lg "):
		rest = userInput[len("/lg "):]
	case userInput == "/lg":
		return "", false
	default:
		return "", false
	}
	goal = strings.TrimSpace(rest)
	return goal, goal != ""
}
