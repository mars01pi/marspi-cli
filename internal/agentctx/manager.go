// Package agentctx 管理对话上下文：消息、持久化与压缩。
// 命名为 agentctx 以避免与标准库 context 冲突。对齐 mangopi 的 ContextManager。
package agentctx

import (
	"encoding/json"
	"os"
	"time"

	"github.com/mars/marspi-cli/internal/llm"
	"github.com/mars/marspi-cli/internal/ui"
)

// Manager 保存并维护会话消息。
type Manager struct {
	Messages         []llm.Message
	whiteToolList    map[string]bool
	autoThreshold    int
	autoDisabled     bool
	continuousFails  int
	maxFailures      int
	maxContext       int
	runtimeInjection []llm.Message
	provider         llm.Provider              // 供 full_compact 调用
	tools            []map[string]any          // agent 工具 schema（full compact 不传 tools）
	console          *ui.Printer
	now              func() int64
}

// New 构建 Manager。provider/tools 可为 nil（此时 full_compact 不可用）。
func New(maxContext int, provider llm.Provider, tools []map[string]any, console *ui.Printer) *Manager {
	return &Manager{
		whiteToolList: map[string]bool{"attempt_completion": true},
		autoThreshold: int(float64(maxContext) * 0.8),
		maxFailures:   3,
		maxContext:    maxContext,
		provider:      provider,
		tools:         tools,
		console:       console,
		now:           func() int64 { return time.Now().Unix() },
	}
}

// Len 返回消息数量。
func (m *Manager) Len() int { return len(m.Messages) }

// BackfillToolNames 为标准 OpenAI 格式的 tool 消息补上 tool_name（依据 assistant 的 tool_calls）。
// 对齐 backfill_tool_names，供 flashext 处理外部 client 请求时使用。
func BackfillToolNames(messages []llm.Message) {
	idx := map[string]string{}
	for _, m := range messages {
		role, _ := m["role"].(string)
		if role == "assistant" {
			if tcs, ok := m["tool_calls"].([]any); ok {
				for _, tc := range tcs {
					cm, ok := tc.(map[string]any)
					if !ok {
						continue
					}
					id, _ := cm["id"].(string)
					fn, _ := cm["function"].(map[string]any)
					name, _ := fn["name"].(string)
					if id != "" && name != "" {
						idx[id] = name
					}
				}
			}
		} else if role == "tool" {
			if tn, _ := m["tool_name"].(string); tn == "" {
				if tcid, _ := m["tool_call_id"].(string); tcid != "" {
					if name, ok := idx[tcid]; ok {
						m["tool_name"] = name
					}
				}
			}
		}
	}
}

// Clear 清空消息。
func (m *Manager) Clear() { m.Messages = nil }

// AppendSystem 追加系统消息。
func (m *Manager) AppendSystem(content string) {
	m.Messages = append(m.Messages, llm.Message{"role": "system", "content": content})
}

// AppendUser 追加用户消息（带时间戳）。
func (m *Manager) AppendUser(content string) {
	m.Messages = append(m.Messages, llm.Message{"role": "user", "content": content, "ts": m.now()})
}

// InjectUser 追加一条临时运行时用户注入（不进入 session/compact/save）。
func (m *Manager) InjectUser(content string) {
	m.runtimeInjection = append(m.runtimeInjection, llm.Message{"role": "user", "content": content})
}

// ClearRuntimeInjections 清空运行时注入。
func (m *Manager) ClearRuntimeInjections() { m.runtimeInjection = nil }

// AppendAssistant 追加助手原始消息（带时间戳）。
func (m *Manager) AppendAssistant(msg llm.Message) {
	if msg == nil {
		msg = llm.Message{}
	}
	msg["ts"] = m.now()
	m.Messages = append(m.Messages, msg)
}

// AppendTool 追加工具结果消息，image 结果转多模态块，对齐 append_tool。
func (m *Manager) AppendTool(toolCallID, toolName string, content any) {
	msg := llm.Message{
		"role":         "tool",
		"tool_call_id": toolCallID,
		"tool_name":    toolName,
		"ts":           m.now(),
	}
	if d, ok := content.(map[string]any); ok && d["type"] == "image" {
		text, _ := d["text"].(string)
		if text == "" {
			text = "image"
		}
		url, _ := d["image_url"].(string)
		msg["content"] = []any{
			map[string]any{"type": "text", "text": text},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}},
		}
	} else if content == nil {
		msg["content"] = ""
	} else {
		msg["content"] = content
	}
	m.Messages = append(m.Messages, msg)
}

// Load 从持久化文件读取消息，损坏则备份并重置。
func (m *Manager) Load(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // 文件不存在视为空会话
	}
	var msgs []llm.Message
	if jerr := json.Unmarshal(data, &msgs); jerr != nil {
		m.Backup(path)
		m.Messages = nil
		if m.console != nil {
			m.console.Error("session.json file is corrupted. The corrupted file has been backed up and a new session.json has been generated.")
		}
		return
	}
	m.Messages = msgs
}

// Save 将消息写入持久化文件。
func (m *Manager) Save(path string) error {
	data, err := json.MarshalIndent(m.Messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Backup 备份持久化文件（追加时间戳后缀）。
func (m *Manager) Backup(path string) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	backup := path + "." + itoa64(m.now()) + ".backup"
	if err := os.Rename(path, backup); err != nil {
		if m.console != nil {
			m.console.Warning("Failed to backup corrupted session file: " + err.Error())
		}
		return
	}
	if m.console != nil {
		m.console.Warning("Session file backed up to " + backup)
	}
}

// PrepareForAPI 执行压缩并返回可发送的消息（含运行时注入），对齐 prepare_for_api。
func (m *Manager) PrepareForAPI() []llm.Message {
	m.microCompact()
	before := m.TotalTokens()
	m.autoCompactIfNeeded()
	after := m.TotalTokens()
	if before > after && m.console != nil {
		m.console.CompactStatus(before, after, m.maxContext, "auto")
	}
	out := make([]llm.Message, 0, len(m.Messages)+len(m.runtimeInjection))
	out = append(out, m.Messages...)
	out = append(out, m.runtimeInjection...)
	return out
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [24]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
