package tools

import (
	"context"
	"strings"

	"github.com/mars/marspi-core/rag"
)

// searchKnowledgeTool 检索治理知识库（SOP/规则/卷宗等），与 search_memory 分离。
type searchKnowledgeTool struct {
	Base
	ret         rag.Retriever
	defaultTopK int
}

func (t *searchKnowledgeTool) Name() string { return "search_knowledge" }
func (t *searchKnowledgeTool) Description() string {
	return "Search the project/governance knowledge base (SOP, rules, case dossiers, policies). " +
		"CALL THIS WHEN: (1) answering needs citations from internal docs, " +
		"(2) judgment/fee/cancel/service rules are involved, " +
		"(3) user asks for policy/SOP grounds. " +
		"DO NOT use for personal preferences or past-session notes — use search_memory instead. " +
		"Always cite returned [id|source] markers in the final answer."
}
func (t *searchKnowledgeTool) UseSpinner() bool  { return true }
func (t *searchKnowledgeTool) PreviewLines() int { return 40 }
func (t *searchKnowledgeTool) PreviewWidth() int { return 160 }
func (t *searchKnowledgeTool) Params() []Param {
	return []Param{
		{Name: "query", Type: "string", Desc: "Natural-language retrieval query."},
		{Name: "kb", Type: "string?", Desc: "Optional knowledge-base name/id to scope search."},
		{Name: "top_k", Type: "number?", Desc: "Max hits to return (1-50, default from config)."},
	}
}
func (t *searchKnowledgeTool) Preview(args map[string]any) string {
	s, _ := argStr(args, "query")
	return truncate(s, t.PreviewWidth())
}

func (t *searchKnowledgeTool) Run(args map[string]any) Result {
	return t.RunCtx(context.Background(), ExecutionMeta{}, args)
}

func (t *searchKnowledgeTool) RunCtx(ctx context.Context, _ ExecutionMeta, args map[string]any) Result {
	if t.ret == nil {
		return Fail("search_knowledge error: no retriever configured (set MARS_RAG_URL or RegisterKnowledge)")
	}
	query, _ := argStr(args, "query")
	query = strings.TrimSpace(query)
	if query == "" {
		return Fail("search_knowledge error: 'query' is required")
	}
	topK := t.defaultTopK
	if topK <= 0 {
		topK = 5
	}
	if v, ok := argInt(args, "top_k"); ok {
		topK = v
	}
	kb, _ := argStr(args, "kb")
	hits, err := t.ret.Search(ctx, rag.Query{Text: query, TopK: topK, KB: strings.TrimSpace(kb)})
	if err != nil {
		return Fail("search_knowledge error: " + err.Error())
	}
	if len(hits) == 0 {
		return OK("No knowledge found for query: " + query)
	}
	return OK(rag.FormatHits(hits))
}

// NewKnowledgeTool creates the optional knowledge-base tool.
func NewKnowledgeTool(ret rag.Retriever, defaultTopK int) Tool {
	if ret == nil {
		return nil
	}
	if defaultTopK <= 0 {
		defaultTopK = 5
	}
	return &searchKnowledgeTool{ret: ret, defaultTopK: defaultTopK}
}
