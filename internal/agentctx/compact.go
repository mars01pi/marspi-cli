package agentctx

import (
	"encoding/json"
	"strings"

	"github.com/mars/marspi-cli/internal/llm"
)

// compactRule 是某类消息的压缩规则，对齐 COMPACT_RULES。
type compactRule struct {
	maxTokens int
	keepHead  int
	keepTail  int
	maxAge    int64
}

var compactRules = map[string]compactRule{
	"tool":              {maxTokens: 800, keepHead: 200, keepTail: 200, maxAge: 21600},
	"reasoning_content": {maxTokens: 500, keepHead: 125, keepTail: 125, maxAge: 7200},
	"assistant":         {maxTokens: 1500, keepHead: 350, keepTail: 350, maxAge: 10800},
}

// estimatedTokens 估算单条消息 token：len(json)/4 + 4，对齐 estimated_tokens。
func estimatedTokens(msg any) int {
	b, _ := json.Marshal(msg)
	return len(b)/4 + 4
}

// TotalTokens 估算全部消息 token。
func (m *Manager) TotalTokens() int {
	total := 0
	for _, msg := range m.Messages {
		total += estimatedTokens(msg)
	}
	return total
}

func (m *Manager) underThreshold() bool { return m.TotalTokens() < m.autoThreshold }

// compactText 截断保留头尾，对齐 compact_text。
func compactText(text string, head, tail int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	r := []rune(text)
	if len(r) <= head+tail {
		return text
	}
	return string(r[:head]) + "\n...\n" + string(r[len(r)-tail:]) + "\n<compacted>"
}

// splitTurns 将消息按 user 边界拆分为轮次，忽略 system，对齐 split_turns。
func (m *Manager) splitTurns() [][]llm.Message {
	var turns [][]llm.Message
	var current []llm.Message
	for _, msg := range m.Messages {
		role, _ := msg["role"].(string)
		if role == "system" {
			continue
		}
		if role == "user" && len(current) > 0 {
			turns = append(turns, current)
			current = nil
		}
		if role == "user" {
			current = []llm.Message{msg}
		} else {
			current = append(current, msg)
		}
	}
	if len(current) > 0 {
		turns = append(turns, current)
	}
	return turns
}

func (m *Manager) systemMessages() []llm.Message {
	var out []llm.Message
	for _, msg := range m.Messages {
		if r, _ := msg["role"].(string); r == "system" {
			out = append(out, cloneMsg(msg))
		}
	}
	return out
}

func cloneMsg(msg llm.Message) llm.Message {
	out := make(llm.Message, len(msg))
	for k, v := range msg {
		out[k] = v
	}
	return out
}

func msgString(msg llm.Message, key string) (string, bool) {
	s, ok := msg[key].(string)
	return s, ok
}

// microCompact 就地压缩过旧的大 tool 结果，对齐 micro_compact。
func (m *Manager) microCompact() {
	now := m.now()
	rule := compactRules["tool"]
	for _, msg := range m.Messages {
		if r, _ := msg["role"].(string); r != "tool" {
			continue
		}
		if tn, _ := msg["tool_name"].(string); m.whiteToolList[tn] {
			continue
		}
		content, ok := msgString(msg, "content")
		if !ok || strings.HasSuffix(content, "<compacted>") {
			continue
		}
		ts, _ := toInt64(msg["ts"])
		if now-ts < rule.maxAge {
			continue
		}
		if estimatedTokens(map[string]any{"content": content}) <= rule.maxTokens {
			continue
		}
		msg["content"] = compactText(content, rule.keepHead, rule.keepTail)
	}
}

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	}
	return 0, false
}

// autoCompactIfNeeded 依次尝试三级压缩直到低于阈值，对齐 auto_compact_if_needed。
func (m *Manager) autoCompactIfNeeded() {
	if m.autoDisabled || m.underThreshold() {
		return
	}
	m.sessionMemoryCompact(10, 200)
	if m.underThreshold() {
		return
	}
	m.compactConversation(8)
	if m.underThreshold() {
		return
	}
	if m.continuousFails >= m.maxFailures {
		return
	}
	if err := m.fullCompact(); err != nil {
		m.continuousFails++
	} else {
		m.continuousFails = 0
	}
}

// sessionMemoryCompact 压缩旧轮次的 tool/assistant/reasoning，对齐 session_memory_compact。
func (m *Manager) sessionMemoryCompact(retainTurns, minTokens int) bool {
	_ = minTokens
	systems := m.systemMessages()
	turns := m.splitTurns()
	if len(turns) <= retainTurns {
		return false
	}
	oldTurns := turns[:len(turns)-retainTurns]
	recentTurns := turns[len(turns)-retainTurns:]

	var compacted []llm.Message
	for _, turn := range oldTurns {
		for _, msg := range turn {
			cm := cloneMsg(msg)
			role, _ := cm["role"].(string)
			switch role {
			case "tool":
				tn, _ := cm["tool_name"].(string)
				tcid, _ := cm["tool_call_id"].(string)
				cm["content"] = "<Old tool(" + tn + ":" + tcid + ") result force compacted>"
			case "assistant":
				if content, ok := msgString(cm, "content"); ok && content != "" {
					rule := compactRules["assistant"]
					if !strings.HasSuffix(content, "<compacted>") &&
						estimatedTokens(map[string]any{"content": content}) > rule.maxTokens {
						cm["content"] = compactText(content, rule.keepHead, rule.keepTail)
					}
				}
				reasoning := reasoningStr(cm)
				if reasoning != "" {
					rule := compactRules["reasoning_content"]
					if !strings.HasSuffix(reasoning, "<compacted>") &&
						estimatedTokens(map[string]any{"content": reasoning}) > rule.maxTokens {
						ct := compactText(reasoning, rule.keepHead, rule.keepTail)
						if _, ok := cm["reasoning_content"]; ok {
							cm["reasoning_content"] = ct
						}
						if _, ok := cm["reasoning"]; ok {
							cm["reasoning"] = ct
						}
						if _, ok := cm["reasoning_details"]; ok {
							cm["reasoning_details"] = ct
						}
					}
				}
			}
			compacted = append(compacted, cm)
		}
	}
	for _, turn := range recentTurns {
		for _, msg := range turn {
			compacted = append(compacted, cloneMsg(msg))
		}
	}
	m.Messages = append(systems, compacted...)
	return true
}

func reasoningStr(msg llm.Message) string {
	for _, k := range []string{"reasoning_content", "reasoning", "reasoning_details"} {
		if s, ok := msg[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func flattenTurns(systems []llm.Message, turns [][]llm.Message) []llm.Message {
	out := append([]llm.Message(nil), systems...)
	for _, turn := range turns {
		for _, msg := range turn {
			out = append(out, cloneMsg(msg))
		}
	}
	return out
}

func sumTokens(msgs []llm.Message) int {
	total := 0
	for _, msg := range msgs {
		total += estimatedTokens(msg)
	}
	return total
}

// compactConversation 通过丢弃旧轮次把总量压到阈值内，对齐 compact_conversation。
func (m *Manager) compactConversation(retainTurns int) {
	systems := m.systemMessages()
	turns := m.splitTurns()
	if len(turns) == 0 {
		return
	}
	split := len(turns) - retainTurns
	if split < 0 {
		split = 0
	}
	oldTurns := turns[:split]
	recentTurns := turns[split:]

	rebuilt := flattenTurns(systems, append(append([][]llm.Message(nil), oldTurns...), recentTurns...))
	if sumTokens(rebuilt) <= m.autoThreshold {
		m.Messages = rebuilt
		return
	}

	trimmedOld := append([][]llm.Message(nil), oldTurns...)
	for len(trimmedOld) > 0 {
		candidate := flattenTurns(systems, append(append([][]llm.Message(nil), trimmedOld...), recentTurns...))
		if sumTokens(candidate) <= m.autoThreshold {
			m.Messages = candidate
			return
		}
		trimmedOld = trimmedOld[1:]
	}

	trimmedRecent := append([][]llm.Message(nil), recentTurns...)
	for len(trimmedRecent) > 1 {
		candidate := flattenTurns(systems, trimmedRecent)
		if sumTokens(candidate) <= m.autoThreshold {
			m.Messages = candidate
			return
		}
		trimmedRecent = trimmedRecent[1:]
	}
	m.Messages = flattenTurns(systems, trimmedRecent)
}
