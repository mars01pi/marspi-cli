package tool

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// bashTool 执行 shell 命令，60s 超时。
type bashTool struct {
	Base
	c *ctx
}

func (t *bashTool) Name() string        { return "bash" }
func (t *bashTool) Description() string { return "Execute a shell command and return its stdout/stderr output (timeout after 60s)" }
func (t *bashTool) UseSpinner() bool    { return true }
func (t *bashTool) PreviewLines() int   { return 100 }
func (t *bashTool) PreviewWidth() int   { return 150 }
func (t *bashTool) Params() []Param {
	return []Param{
		{"cmd", "string", "The shell command to execute, e.g., 'ls -la' or 'git status'"},
	}
}
func (t *bashTool) Confirm(args map[string]any) bool {
	cmd, _ := argStr(args, "cmd")
	dangerous, reason := checkCommandSafety(cmd)
	if !dangerous {
		return true
	}
	return t.c.console.PromptApply("Execute dangerous cmd (" + reason + ")? " + cmd)
}
func (t *bashTool) Run(args map[string]any) Result {
	cmdStr, _ := argStr(args, "cmd")
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, "sh", "-c", cmdStr)
	out, err := cmd.CombinedOutput()
	if ctxTimeout.Err() == context.DeadlineExceeded {
		return Fail("exec " + cmdStr + " timed out after 60s")
	}
	_ = err // 退出码非零时输出仍返回，对齐 Python 的 STDOUT 合并

	lines := strings.Split(string(out), "\n")
	// 去掉末尾空行产生的空串
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	lines = processBashOutput(cmdStr, lines)
	res := strings.TrimSpace(strings.Join(lines, "\n"))
	if res == "" {
		res = "(empty)"
	}
	return OK(res)
}

// attemptCompletionTool 表示任务完成并给出最终结果。
type attemptCompletionTool struct {
	Base
}

func (t *attemptCompletionTool) Name() string { return "attempt_completion" }
func (t *attemptCompletionTool) Description() string {
	return "Indicate that the task is complete and provide the final result/answer to the user"
}
func (t *attemptCompletionTool) PreviewLines() int { return 500 }
func (t *attemptCompletionTool) PreviewWidth() int { return 500 }
func (t *attemptCompletionTool) Params() []Param {
	return []Param{
		{"result", "string", "The final result or summary of the completed task"},
	}
}
func (t *attemptCompletionTool) Run(args map[string]any) Result {
	r, _ := argStr(args, "result")
	return OK(r)
}
