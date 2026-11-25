package tools

import "strings"

// NewAttemptCompletionTool 提供通用的轮次结束工具，可返回答案或向用户追问。
func NewAttemptCompletionTool() Tool { return &attemptCompletionTool{} }

type attemptCompletionTool struct{ Base }

func (t *attemptCompletionTool) Name() string { return "attempt_completion" }
func (t *attemptCompletionTool) Description() string {
	return "End the current turn with a final answer or a question for the user"
}
func (t *attemptCompletionTool) PreviewLines() int { return 500 }
func (t *attemptCompletionTool) PreviewWidth() int { return 500 }
func (t *attemptCompletionTool) Params() []Param {
	return []Param{{Name: "result", Type: "string", Desc: "The answer or question to present to the user"}}
}
func (t *attemptCompletionTool) Run(args map[string]any) Result {
	text, ok := args["result"].(string)
	if !ok || strings.TrimSpace(text) == "" {
		return Fail("error: result must be a non-empty string")
	}
	return Result{Success: true, Content: text, EndTurn: true}
}
