package agentctx

import (
	"errors"
	"time"

	"github.com/mars/marspi-cli/internal/llm"
)

const fullCompactPrompt = `Create a detailed summary of the conversation so far.
Focus on: user's original intent, files modified with key code snippets, errors encountered and their fixes,
and the current work in progress.
Use this structure:
1. Primary Request and Intent
2. Key Technical Concepts
3. Files and Code Sections (most recent first)
4. Errors and fixes
5. Problem Solving
6. All user messages
7. Pending Tasks
8. Current Work

Output in <analysis>...</analysis><summary>...</summary> format.`

// FullCompact 调用模型生成整体摘要并以其替换历史，对齐 full_compact。
// 手动 /compact 命令与 auto 兜底都会调用。
func (m *Manager) FullCompact() error { return m.fullCompact() }

func (m *Manager) fullCompact() error {
	if m.provider == nil {
		return errors.New("full compact err: no provider")
	}
	m.AppendUser(fullCompactPrompt)
	raw, err := llm.Request(m.provider.APIURL(), m.provider.BuildBody(m.Messages, m.tools),
		m.provider.Headers(), 300*time.Second, 3)
	if err != nil {
		return errors.New("full compact err: " + err.Error())
	}
	resp := m.provider.ParseResponse(raw)
	if resp.Content == "" {
		return errors.New("full compact err: llm respon null")
	}
	systems := m.systemMessages()
	m.Messages = systems
	m.AppendUser(resp.Content)
	return nil
}
