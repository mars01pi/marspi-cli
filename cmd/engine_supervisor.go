package cmd

// Supervisor engine for star-topology multi-agent orchestration with HITL approval.
import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mars/marspi-graph/checkpoint"
	"github.com/mars/marspi-graph/orchestrator"
)

// superviseRequest is a parsed /sv or /supervise command.
type superviseRequest struct {
	Goal           string
	ResumeThreadID string
	List           bool
}

// runSupervisorEngine runs a star-topology supervisor workflow via marspi-graph.
// 用 marspi-graph Supervisor 星型编排跑动态多 Agent。
// 实验命令 /supervise /sv，与 /loop /loopg 并存。
// 派发 coder 前会经 Confirm 审批（HITL）。
// Checkpoints 落在 .marspicli/checkpoints.db，可用 /sv resume <threadID> 跨进程续跑。
func (a *App) runSupervisorEngine(ctx context.Context, req superviseRequest, maxSteps int) {
	if ctx == nil {
		ctx = context.Background()
	}

	cp, dbPath, err := openSupervisorCheckpointer()
	if err != nil {
		a.console.Error("Checkpoint DB: " + err.Error())
		return
	}
	defer cp.Close()

	if req.List {
		a.listSupervisorCheckpoints(ctx, cp, dbPath)
		return
	}

	threadID := req.ResumeThreadID
	resume := threadID != ""
	if !resume {
		threadID = fmt.Sprintf("supervisor-%d", time.Now().UnixNano())
	}

	a.console.Text(fmt.Sprintf("thread=%s  db=%s", threadID, dbPath))

	cfg := orchestrator.SupervisorConfig{
		Goal:                 req.Goal,
		MaxSteps:             maxSteps,
		SystemPrompt:         a.prompt.Assemble(),
		Provider:             a.provider,
		Registry:             a.registry,
		Reporter:             a.console,
		Events:               a.runner.Events,
		MaxContext:           a.cfg.MaxContext,
		MaxIterAgent:         a.cfg.MaxIter,
		Stream:               a.cfg.Stream,
		ThreadID:             threadID,
		Checkpointer:         cp,
		ResumeFromCheckpoint: resume,
		RequireApprovalFor:   []string{"coder"},
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
		Workers: supervisorDemoWorkers(a),
	}

	res, err := orchestrator.RunSupervisor(ctx, cfg)
	if err != nil {
		if ctx.Err() != nil {
			a.console.Warning(fmt.Sprintf("Supervisor stopped (thread=%s). Resume with: /sv resume %s", threadID, threadID))
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

func (a *App) listSupervisorCheckpoints(ctx context.Context, cp *checkpoint.SQLite, dbPath string) {
	list, err := cp.ListInterrupted(ctx)
	if err != nil {
		a.console.Error(err.Error())
		return
	}
	a.console.Text("db=" + dbPath)
	if len(list) == 0 {
		a.console.Text("No interrupted supervisor threads.")
		return
	}
	for _, m := range list {
		a.console.Text(fmt.Sprintf("- %s  node=%s step=%d  (/sv resume %s)", m.ThreadID, m.Node, m.Step, m.ThreadID))
	}
}

func openSupervisorCheckpointer() (*checkpoint.SQLite, string, error) {
	dir := ".marspicli"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, "checkpoints.db")
	if env := strings.TrimSpace(os.Getenv("MARSPI_CHECKPOINT_DB")); env != "" {
		path = env
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, "", err
		}
	}
	cp, err := checkpoint.OpenSQLite(path)
	if err != nil {
		return nil, path, err
	}
	return cp, path, nil
}

func supervisorDemoWorkers(a *App) []orchestrator.WorkerSpec {
	return []orchestrator.WorkerSpec{
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

// parseSuperviseRequest 解析 /supervise 或 /sv 命令。
// 支持: /sv <goal> | /sv resume <threadID> | /sv --resume <threadID> | /sv list
func parseSuperviseRequest(userInput string) (superviseRequest, bool) {
	var rest string
	switch {
	case strings.HasPrefix(userInput, "/supervise "):
		rest = strings.TrimSpace(userInput[len("/supervise "):])
	case userInput == "/supervise":
		return superviseRequest{}, false
	case strings.HasPrefix(userInput, "/sv "):
		rest = strings.TrimSpace(userInput[len("/sv "):])
	case userInput == "/sv":
		return superviseRequest{}, false
	default:
		return superviseRequest{}, false
	}
	if rest == "" {
		return superviseRequest{}, false
	}

	fields := strings.Fields(rest)
	switch strings.ToLower(fields[0]) {
	case "list":
		return superviseRequest{List: true}, true
	case "resume", "--resume", "-r":
		if len(fields) < 2 {
			return superviseRequest{}, false
		}
		return superviseRequest{ResumeThreadID: fields[1]}, true
	default:
		return superviseRequest{Goal: rest}, true
	}
}
