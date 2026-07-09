// Package flashext 实现 OpenAI 兼容代理服务器，注入结构化思考框架。
// 对齐 mangopi 的 FlashExtServer。
package flashext

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/mars/marspi-core/agentctx"
	"github.com/mars/marspi-core/config"
	"github.com/mars/marspi-core/flash"
	"github.com/mars/marspi-core/llm"
	"github.com/mars/marspi-core/memory"
)

// Server 是 Flash-ext 代理服务器。
type Server struct {
	host         string
	port         int
	token        string
	provider     llm.Provider
	enableMemory bool
	enableSearch bool
	searchKey    string
	mem          *memory.Manager
	maxContext   int
	lastDeepTS   int64
	debug        bool
}

// Options 是 Server 的构造参数。
type Options struct {
	Host         string
	Port         int
	Token        string
	Provider     llm.Provider
	EnableMemory bool
	EnableSearch bool
	Debug        bool
}

// New 依据配置与选项构建 Server。
func New(cfg *config.Config, opts Options) *Server {
	s := &Server{
		host:         opts.Host,
		port:         opts.Port,
		token:        opts.Token,
		provider:     opts.Provider,
		enableMemory: opts.EnableMemory,
		enableSearch: opts.EnableSearch && cfg.SearchKey != "",
		searchKey:    cfg.SearchKey,
		mem:          memory.New(cfg.MemoryDir),
		maxContext:   cfg.MaxContext,
		debug:        opts.Debug,
	}
	return s
}

var flashExtRe = regexp.MustCompile(`(?s)\n*<flash_ext>.*?</flash_ext>\n*`)

func (s *Server) debugf(format string, args ...any) {
	if s.debug {
		log.Printf("[flash-ext] "+format, args...)
	}
}

// augment 依据会话状态向最后一条 user 消息注入 <flash_ext> 增强块。
func (s *Server) augment(messages []llm.Message) []llm.Message {
	ctx := agentctx.New(s.maxContext, s.provider, nil, nil)
	ctx.Messages = messages
	agentctx.BackfillToolNames(ctx.Messages)

	query := lastUser(messages)
	toolPattern := ctx.ToolPattern(10)
	toolCtx := ctx.ToolContext(10, 800)
	var elems []string

	complexity := ctx.AssessComplexity()
	now := time.Now().Unix()
	if complexity == "deep" && now-s.lastDeepTS < 60 {
		complexity = "fast"
	} else if complexity == "deep" {
		s.lastDeepTS = now
	}
	s.debugf("complexity=%s", complexity)

	if complexity == "deep" {
		if analysis := s.analyzeDeep(ctx, query, toolPattern); analysis != nil {
			if fw, _ := analysis["framework"].(string); fw != "" && flash.FormatFramework(fw) != "" {
				elems = append(elems, "<framework name=\""+fw+"\">\n"+flash.FormatFramework(fw)+"\n</framework>")
			}
			if ins, _ := analysis["insight"].(string); ins != "" {
				elems = append(elems, "<insight>"+ins+"</insight>")
			}
			if al, _ := analysis["anti_loop"].(string); al != "" {
				elems = append(elems, "<anti_loop>"+al+"</anti_loop>")
			}
			if ts, _ := analysis["tool_summary"].(string); ts != "" && len(toolCtx) > 2000 {
				elems = append(elems, "<tool_context>"+ts+"</tool_context>")
			} else if toolCtx != "" {
				elems = append(elems, "<tool_context>"+toolCtx+"</tool_context>")
			}
		} else {
			complexity = "fast"
		}
	}
	if complexity != "deep" {
		fw := flash.Match(query, toolPattern)
		if fw != "" && flash.FormatFramework(fw) != "" {
			elems = append(elems, "<framework name=\""+fw+"\">\n"+flash.FormatFramework(fw)+"\n</framework>")
		}
		if toolCtx != "" && len(toolCtx) < 2000 {
			elems = append(elems, "<tool_context>"+toolCtx+"</tool_context>")
		} else if toolCtx != "" {
			elems = append(elems, "<tool_context>"+toolCtx[:2000]+"...(truncated)</tool_context>")
		}
	}

	if s.enableMemory {
		if mem := s.mem.Search(query, 10); mem != "" && !strings.Contains(mem, "No memory") {
			elems = append(elems, "<memory>"+mem+"</memory>")
		}
	}
	if s.enableSearch {
		if sr := bochaBrief(query, s.searchKey); sr != "" {
			elems = append(elems, "<web_search>"+sr+"</web_search>")
		}
	}

	if len(elems) > 0 {
		prefix := "<flash_ext>\n" + strings.Join(elems, "\n") + "\n</flash_ext>"
		for i := len(messages) - 1; i >= 0; i-- {
			if r, _ := messages[i]["role"].(string); r == "user" {
				clean := strings.TrimSpace(flashExtRe.ReplaceAllString(strGet(messages[i], "content"), ""))
				nm := cloneMsg(messages[i])
				nm["content"] = prefix + "\n\n" + clean
				messages[i] = nm
				break
			}
		}
	}
	return messages
}

// analyzeDeep 用模型分析会话状态，返回 JSON 对象，对齐 _analyze_deep。
func (s *Server) analyzeDeep(ctx *agentctx.Manager, query string, toolPattern []string) map[string]any {
	isLooping, loopTool := ctx.DetectLoop(3)
	phase := ctx.DetectPhase()
	recent := ctx.SummarizeRecentTurns(3)
	loopStr := "no"
	if isLooping {
		loopStr = "yes (" + loopTool + ")"
	}
	tp := "none"
	if len(toolPattern) > 0 {
		tp = strings.Join(toolPattern, ",")
	}
	q := query
	if q == "" {
		q = "(none)"
	}
	prompt := "Analyze this coding session and respond ONLY as JSON.\n\n" +
		"## User question (the goal agent is working toward)\n" + q + "\n\n" +
		"## Session state\n" +
		"Tool pattern: " + tp + "\nPhase: " + phase + "\n" +
		"Looping: " + loopStr + "\n\n" +
		recent + "\n\n" +
		`{"framework": "<one of: debug/design/explain/optimize/implement/investigate/verify/reevaluate>", ` +
		`"insight": "<optional key insight agent is missing>", ` +
		`"anti_loop": "<optional>", ` +
		`"tool_summary": "<optional>"}`

	body := s.provider.BuildBody([]llm.Message{{"role": "user", "content": prompt}}, nil)
	raw, err := llm.Request(s.provider.APIURL(), body, s.provider.Headers(), 15*time.Second, 3)
	if err != nil {
		return nil
	}
	parsed := s.provider.ParseResponse(raw)
	var out map[string]any
	if json.Unmarshal([]byte(parsed.Content), &out) != nil {
		return nil
	}
	return out
}

// handle 处理一次 chat/completions：增强消息 → 转发上游 → 可选记忆写入。
func (s *Server) handle(body map[string]any) map[string]any {
	if msgs, ok := body["messages"].([]any); ok {
		converted := make([]llm.Message, 0, len(msgs))
		for _, m := range msgs {
			if mm, ok := m.(map[string]any); ok {
				converted = append(converted, mm)
			}
		}
		augmented := s.augment(converted)
		asAny := make([]any, len(augmented))
		for i, m := range augmented {
			asAny[i] = m
		}
		body["messages"] = asAny
	}
	body["stream"] = false

	raw, err := llm.Request(s.provider.APIURL(), body, s.provider.Headers(), 300*time.Second, 3)
	if err != nil {
		return map[string]any{"error": map[string]any{
			"message": "Upstream model error: " + err.Error(), "type": "flash_ext_error", "code": 502}}
	}
	if s.enableMemory {
		parsed := s.provider.ParseResponse(raw)
		if len(parsed.Content) > 50 {
			q := ""
			if msgs, ok := body["messages"].([]any); ok {
				for i := len(msgs) - 1; i >= 0; i-- {
					if mm, ok := msgs[i].(map[string]any); ok {
						if r, _ := mm["role"].(string); r == "user" {
							q = truncate(strGet(mm, "content"), 200)
							break
						}
					}
				}
			}
			_ = s.mem.Append("[auto] Q: " + q + "\nA: " + truncate(parsed.Content, 500))
		}
	}
	return raw
}

func (s *Server) models() map[string]any {
	return map[string]any{
		"object": "list",
		"data":   []any{map[string]any{"id": s.provider.Model(), "object": "model"}},
	}
}

func lastUser(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if r, _ := messages[i]["role"].(string); r == "user" {
			return strGet(messages[i], "content")
		}
	}
	return ""
}

func strGet(m llm.Message, key string) string {
	s, _ := m[key].(string)
	return s
}

func cloneMsg(m llm.Message) llm.Message {
	out := make(llm.Message, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
