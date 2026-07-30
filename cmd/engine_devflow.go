package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mars/marspi-graph/checkpoint"
	"github.com/mars/marspi-graph/workflow"
)

// devflowRequest is a parsed /devflow or /df command.
type devflowRequest struct {
	Goal           string
	ResumeThreadID string
	List           bool
	WorkflowPath   string
	AllowPush      bool
}

// runDevFlowEngine runs Design→Develop→Review→Test→Push via workflow Spec.
// Checkpoints share .marspicli/checkpoints.db with /sv.
func (a *App) runDevFlowEngine(ctx context.Context, req devflowRequest) {
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
		a.listDevflowCheckpoints(ctx, cp, dbPath)
		return
	}

	var spec workflow.Spec
	if req.WorkflowPath != "" {
		spec, err = workflow.LoadSpec(req.WorkflowPath)
		if err != nil {
			a.console.Error(err.Error())
			return
		}
	} else {
		spec, err = workflow.Builtin("devflow")
		if err != nil {
			a.console.Error(err.Error())
			return
		}
	}

	threadID := req.ResumeThreadID
	resume := threadID != ""
	if !resume {
		threadID = fmt.Sprintf("devflow-%d", time.Now().UnixNano())
	}

	a.console.Text(fmt.Sprintf("thread=%s  db=%s  workflow=%s  allow_push=%v",
		threadID, dbPath, spec.ID, req.AllowPush))

	b := workflow.Bindings{
		Goal:                 req.Goal,
		AllowPush:            req.AllowPush,
		MaxRework:            3,
		MaxSteps:             40,
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
		OnInterrupt: func(runCtx context.Context, info workflow.InterruptInfo) (bool, error) {
			if err := runCtx.Err(); err != nil {
				return false, err
			}
			msg := formatDevflowHITL(info)
			ok := a.console.PromptApply(msg)
			if err := runCtx.Err(); err != nil {
				return false, err
			}
			return ok, nil
		},
	}

	res, err := workflow.Run(ctx, spec, b)
	if err != nil {
		if ctx.Err() != nil {
			a.console.Warning(fmt.Sprintf("DevFlow stopped (thread=%s). Resume with: /df resume %s", threadID, threadID))
			return
		}
		if errors.Is(err, workflow.ErrApprovalDenied) {
			a.console.Warning("Push denied — DevFlow stopped.")
			return
		}
		a.console.Error("DevFlow error: " + err.Error())
		return
	}
	status := res.State.GetString("status")
	phase := res.State.GetString("phase")
	a.console.Success(fmt.Sprintf("DevFlow done (status=%s phase=%s)", status, phase))
	if res.Message != "" {
		a.console.Text(res.Message)
	}
	if pr := res.State.GetString("push_result"); pr != "" && pr != res.Message {
		a.console.Text(pr)
	}
}

func (a *App) listDevflowCheckpoints(ctx context.Context, cp *checkpoint.SQLite, dbPath string) {
	list, err := cp.ListResumable(ctx)
	if err != nil {
		a.console.Error(err.Error())
		return
	}
	a.console.Text("db=" + dbPath)
	shown := 0
	for _, m := range list {
		kind := "mid-run"
		if m.Interrupt {
			kind = "hitl"
		}
		if !strings.HasPrefix(m.ThreadID, "devflow-") {
			continue
		}
		shown++
		a.console.Text(fmt.Sprintf("- %s  node=%s step=%d (%s)  (/df resume %s)",
			m.ThreadID, m.Node, m.Step, kind, m.ThreadID))
	}
	if shown == 0 {
		a.console.Text("No resumable DevFlow threads.")
	}
}

func formatDevflowHITL(info workflow.InterruptInfo) string {
	goal, title, summary := "", "", ""
	if m, ok := info.Value.(map[string]any); ok {
		goal, _ = m["goal"].(string)
		title, _ = m["title"].(string)
		summary, _ = m["summary"].(string)
	}
	var b strings.Builder
	if title != "" {
		fmt.Fprintf(&b, "%s", title)
	} else {
		fmt.Fprintf(&b, "Approve push (gate=%s)?", info.Node)
	}
	if goal != "" && title == "" {
		fmt.Fprintf(&b, "\nGoal: %s", goal)
	}
	if summary != "" {
		fmt.Fprintf(&b, "\n%s", summary)
	}
	return b.String()
}

// parseDevflowRequest parses /devflow or /df.
// Supports: /df <goal> | /df resume <id> | /df list | /df --workflow path.yaml <goal>
// Optional: --no-push to skip git push phase.
func parseDevflowRequest(userInput string) (devflowRequest, bool) {
	var rest string
	switch {
	case strings.HasPrefix(userInput, "/devflow "):
		rest = strings.TrimSpace(userInput[len("/devflow "):])
	case userInput == "/devflow":
		return devflowRequest{}, false
	case strings.HasPrefix(userInput, "/df "):
		rest = strings.TrimSpace(userInput[len("/df "):])
	case userInput == "/df":
		return devflowRequest{}, false
	default:
		return devflowRequest{}, false
	}
	if rest == "" {
		return devflowRequest{}, false
	}

	req := devflowRequest{AllowPush: true}
	if v := strings.TrimSpace(os.Getenv("MARSPI_DEVFLOW_ALLOW_PUSH")); v == "0" || strings.EqualFold(v, "false") {
		req.AllowPush = false
	}

	fields := strings.Fields(rest)
	var goalParts []string
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		switch {
		case f == "--no-push":
			req.AllowPush = false
		case f == "--workflow" || f == "-w":
			if i+1 >= len(fields) {
				return devflowRequest{}, false
			}
			req.WorkflowPath = fields[i+1]
			i++
		case len(goalParts) == 0 && i == 0 && strings.EqualFold(f, "list"):
			req.List = true
			return req, true
		case len(goalParts) == 0 && i == 0 && (strings.EqualFold(f, "resume") || f == "--resume" || f == "-r"):
			if i+1 >= len(fields) {
				return devflowRequest{}, false
			}
			req.ResumeThreadID = fields[i+1]
			return req, true
		default:
			goalParts = append(goalParts, f)
		}
	}
	req.Goal = strings.TrimSpace(strings.Join(goalParts, " "))
	if req.Goal == "" {
		return devflowRequest{}, false
	}
	return req, true
}
