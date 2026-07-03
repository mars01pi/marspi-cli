package agentctx

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/mars/marspi-cli/internal/flash"
	"github.com/mars/marspi-cli/internal/llm"
)

// lastUserContent 返回消息列表中最后一条 user 的 content。
func lastUserContent(msgs []llm.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if r, _ := msgs[i]["role"].(string); r == "user" {
			if c, ok := msgs[i]["content"].(string); ok {
				return c
			}
		}
	}
	return ""
}

// toolNames 提取消息中的 tool_name（去空）。
func toolNames(msgs []llm.Message) []string {
	var out []string
	for _, m := range msgs {
		if r, _ := m["role"].(string); r != "tool" {
			continue
		}
		if tn, _ := m["tool_name"].(string); tn != "" {
			out = append(out, tn)
		}
	}
	return out
}

// ToolFingerprint 返回最近 nTurns 轮的 (user_query, [tools]) 指纹，对齐 tool_fingerprint。
func (m *Manager) ToolFingerprint(nTurns int) string {
	turns := m.splitTurns()
	if len(turns) > nTurns {
		turns = turns[len(turns)-nTurns:]
	}
	var fp [][2]any
	for _, t := range turns {
		tools := toolNames(t)
		if len(tools) == 0 {
			continue
		}
		fp = append(fp, [2]any{lastUserContent(t), tools})
	}
	if len(fp) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(fp)
	return string(b)
}

// ToolPattern 返回最近 n 条消息的 tool 模式，无则 nil。
func (m *Manager) ToolPattern(n int) []string {
	msgs := m.Messages
	if len(msgs) > n {
		msgs = msgs[len(msgs)-n:]
	}
	tools := toolNames(msgs)
	if len(tools) == 0 {
		return nil
	}
	return tools
}

// LastUserContent 返回全局最后一条 user 的 content。
func (m *Manager) LastUserContent() string { return lastUserContent(m.Messages) }

// roleMsgs 返回最近 n 条中指定 role 的消息（n<=0 表示全部）。
func (m *Manager) roleMsgs(role string, n int) []llm.Message {
	src := m.Messages
	if n > 0 && len(src) > n {
		src = src[len(src)-n:]
	}
	var out []llm.Message
	for _, msg := range src {
		if r, _ := msg["role"].(string); r == role {
			out = append(out, msg)
		}
	}
	return out
}

// ToolContext 提取最近 tool 结果的关键内容，注入为上下文，对齐 tool_context。
func (m *Manager) ToolContext(n, cap int) string {
	var tc []string
	for _, msg := range m.roleMsgs("tool", n) {
		content := contentToString(msg["content"])
		tn, _ := msg["tool_name"].(string)
		if len(content) > cap {
			content = compactText(content, 200, 200)
		}
		tc = append(tc, "["+tn+" tool] "+content)
	}
	return strings.Join(tc, "\n\n")
}

// DetectPhase 推断当前对话阶段，对齐 detect_phase。
func (m *Manager) DetectPhase() string {
	if looping, _ := m.detectLoop(3); looping {
		return "stuck"
	}
	all := toolNames(m.Messages)
	if len(all) == 0 {
		return "start"
	}
	recent := all
	if len(recent) > 5 {
		recent = recent[len(recent)-5:]
	}
	bashCount := 0
	allExplore := true
	for _, t := range recent {
		if t == "edit" || t == "write" {
			return "executing"
		}
		if t != "read" && t != "grep" && t != "search" {
			allExplore = false
		}
		if t == "bash" {
			bashCount++
		}
	}
	if allExplore {
		return "exploring"
	}
	if bashCount >= 2 {
		return "verifying"
	}
	return "executing"
}

// detectLoop 检测是否陷入迭代死循环，对齐 detect_loop。
func (m *Manager) detectLoop(threshold int) (bool, string) {
	recent := m.roleMsgs("tool", 20)
	if len(recent) < 10 {
		return false, ""
	}
	lastTool, failStreak := "", 0
	for _, msg := range recent {
		tool, _ := msg["tool_name"].(string)
		content := strings.ToLower(contentToString(msg["content"]))
		failing := strings.Contains(content, "fail") || strings.Contains(content, "error")
		if tool == lastTool && failing {
			failStreak++
		} else if tool != lastTool {
			failStreak = 0
		}
		lastTool = tool
		if failStreak >= threshold {
			return true, tool
		}
	}
	failSet := map[string]bool{}
	failCount := 0
	tail := recent
	if len(tail) > 12 {
		tail = tail[len(tail)-12:]
	}
	for _, msg := range tail {
		content := strings.ToLower(contentToString(msg["content"]))
		if strings.Contains(content, "fail") || strings.Contains(content, "error") {
			tn, _ := msg["tool_name"].(string)
			failSet[tn] = true
			failCount++
		}
	}
	if len(failSet) >= 2 && failCount >= threshold*2 {
		var names []string
		for k := range failSet {
			names = append(names, k)
		}
		sort.Strings(names)
		return true, strings.Join(names, ",")
	}
	return false, ""
}

// SummarizeRecentTurns 压缩最近 nTurns 轮为轻量文本，对齐 summarize_recent_turns。
func (m *Manager) SummarizeRecentTurns(nTurns int) string {
	var lines []string
	turnCount := 0
	src := m.Messages
	if len(src) > 50 {
		src = src[len(src)-50:]
	}
	for _, msg := range src {
		role, _ := msg["role"].(string)
		if role == "user" && turnCount >= nTurns {
			break
		}
		switch role {
		case "user":
			turnCount++
			lines = append(lines, "[USER] "+truncate(contentToString(msg["content"]), 300))
		case "tool":
			tn, _ := msg["tool_name"].(string)
			lines = append(lines, "["+tn+"] "+truncate(contentToString(msg["content"]), 300))
		case "assistant":
			if tc, ok := msg["tool_calls"].([]any); ok && len(tc) > 0 {
				var names []string
				for _, c := range tc {
					if cm, ok := c.(map[string]any); ok {
						if fn, ok := cm["function"].(map[string]any); ok {
							n, _ := fn["name"].(string)
							names = append(names, n)
						}
					}
				}
				lines = append(lines, "[ASSISTANT → tool_calls: "+strings.Join(names, ",")+"]")
			} else {
				lines = append(lines, "[ASSISTANT] "+truncate(contentToString(msg["content"]), 300))
			}
		}
	}
	return strings.Join(lines, "\n")
}

// DetectLoop 是 detectLoop 的导出封装（供 flashext 使用）。
func (m *Manager) DetectLoop(threshold int) (bool, string) { return m.detectLoop(threshold) }

// AssessComplexity 判断是否需要深思路径，返回 "deep" 或 "fast"，对齐 assess_complexity。
func (m *Manager) AssessComplexity() string {
	toolPattern := m.ToolPattern(10)
	toolCtx := m.ToolContext(10, 800)
	if len(toolCtx) > 2000 {
		return "deep"
	}
	if len(toolPattern) > 0 {
		nonRead := map[string]bool{}
		for _, t := range toolPattern {
			if t != "read" {
				nonRead[t] = true
			}
		}
		if len(nonRead) >= 4 {
			return "deep"
		}
		uniq := map[string]bool{}
		for _, t := range toolPattern {
			uniq[t] = true
		}
		if len(toolPattern) >= 5 && len(uniq) == 1 {
			return "deep"
		}
	}
	fw := flash.Match(m.LastUserContent(), nil)
	if fw == "design" || fw == "optimize" || fw == "reevaluate" {
		return "deep"
	}
	return "fast"
}

func contentToString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
