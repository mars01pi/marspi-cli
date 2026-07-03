package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mars/marspi-cli/internal/agentctx"
	"github.com/mars/marspi-cli/internal/llm"
)

// runLoopEngine 实现 3-agent 协作：Implementer + Verifier + Updater。
// 对齐 mangopi 的 loop_engine。
func (a *App) runLoopEngine(goal string, maxIter int) {
	systemPrompt := a.prompt.Assemble()
	var loopFiles []string

	newCtx := func(loopType string) (*agentctx.Manager, string) {
		id := fmt.Sprintf("loop_%d_%d", time.Now().UnixNano(), os.Getpid())
		file := filepath.Join(a.cfg.LoopsDir, loopType+"_"+id+".json")
		c := agentctx.New(a.cfg.MaxContext, a.provider, a.registry.Schemas(), a.console)
		c.Load(file)
		return c, file
	}

	defer func() {
		for _, f := range loopFiles {
			_ = os.Remove(f)
		}
	}()

	implCtx, implFile := newCtx("implementer")
	implCtx.AppendSystem(systemPrompt)
	loopFiles = append(loopFiles, implFile)

	for iteration := 1; iteration <= maxIter; iteration++ {
		a.runner.Loop(implCtx, implFile, fmt.Sprintf(
			"[Loop iter %d/%d]\n"+
				"GOAL: %s\n\n"+
				"You are the IMPLEMENTER. Design + write code.\n"+
				"1. Read relevant code (read/grep).\n"+
				"2. Plan your design.\n"+
				"3. Implement progressively: smallest working change first.\n"+
				"4. Self-review: read back your changes, verify no logic errors.\n"+
				"5. Design for extensibility: clear abstractions, hooks, avoid hard-coding.\n"+
				"6. When calling edit/write, briefly explain WHY in your thinking.\n"+
				"7. Call `attempt_completion` tool when done.\n\n"+
				"DO NOT run tests or verify your own code. That's the Verifier's job.",
			iteration, maxIter, goal))
		implFiles := changedFiles(implCtx)

		verCtx, verFile := newCtx("verifier")
		verCtx.AppendSystem(systemPrompt)
		loopFiles = append(loopFiles, verFile)
		a.runner.Loop(verCtx, verFile, fmt.Sprintf(
			"[Verify iter %d]\n"+
				"GOAL: %s\n"+
				"Files changed by implementer: \n%s\n\n"+
				"You are an OBJECTIVE Verifier. Independent judgment required.\n"+
				"1. Inspect the changed files (read tool).\n"+
				"2. Determine the right test command(inspect project:package.json/pyproject.toml/go.mod).\n"+
				"3. Run tests with bash tool.\n"+
				"4. Judge PASS/FAIL based on exit code AND tests actually verify the goal.\n"+
				"5. If FAIL: explain which test, why, and suggested fix.\n"+
				"6. Call `attempt_completion` tool with one line:\n"+
				"   - 'VERIFY: PASS'\n"+
				"   - 'VERIFY: FAIL: <reason>'\n"+
				"   - 'VERIFY: PASS, NO_ISSUES'\n"+
				"   - 'VERIFY: PASS, ISSUES: <list>'\n\n"+
				"Explain at architecture-level (module/flow), not function-level.",
			iteration, goal, implFiles))

		verifyResult := completionResult(verCtx)
		if verifyResult != "" && strings.Contains(verifyResult, "VERIFY: PASS") {
			a.console.Success(fmt.Sprintf("Loop succeeded at iter %d", iteration))
			return
		}

		updCtx, updFile := newCtx("updater")
		updCtx.AppendSystem(systemPrompt)
		loopFiles = append(loopFiles, updFile)
		a.runner.Loop(updCtx, updFile, fmt.Sprintf(
			"[Updater iter %d]\n"+
				"GOAL: %s\n"+
				"Verifier FAILED: %s\n\n"+
				"You are an UPDATER.\n"+
				"Refine the user's prompt for the next implementer iteration.\n"+
				"You MUST NOT write code or call write/edit/bash. Only read/grep for context.\n\n"+
				"Read the verifier's failure analysis above.\n"+
				"Identify what's missing or unclear in the original goal.\n\n"+
				"Output: a single prompt, 50-200 words, specific constraints.\n"+
				"Call `attempt_completion` tool to return the refined prompt.",
			iteration, goal, verifyResult))
		refined := completionResult(updCtx)
		if refined != "" {
			implCtx.AppendUser(fmt.Sprintf("[Refined prompt from updater iter %d]\n%s", iteration, refined))
		}
	}
	a.console.Error(fmt.Sprintf("Loop failed after %d iterations", maxIter))
}

// changedFiles 从 edit/write 的 tool 消息中提取被修改的文件，对齐 _extract_changed_files。
func changedFiles(ctx *agentctx.Manager) string {
	seen := map[string]bool{}
	var files []string
	for _, m := range ctx.Messages {
		if r, _ := m["role"].(string); r != "tool" {
			continue
		}
		tn, _ := m["tool_name"].(string)
		if tn != "edit" && tn != "write" {
			continue
		}
		content := contentStr(m["content"])
		if !seen[content] {
			seen[content] = true
			files = append(files, content)
		}
	}
	if len(files) == 0 {
		return "(unknown — Verifier inspect project to find)"
	}
	return strings.Join(files, ",\n ")
}

// completionResult 提取最后一条 attempt_completion 的结果，对齐 _get_completion_result。
func completionResult(ctx *agentctx.Manager) string {
	msgs := ctx.Messages
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if r, _ := m["role"].(string); r != "tool" {
			continue
		}
		if tn, _ := m["tool_name"].(string); tn == "attempt_completion" {
			return contentStr(m["content"])
		}
	}
	return ""
}

func contentStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

var _ = llm.ToolCall{} // 保持 llm 依赖（Messages 类型来自 llm）
