package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mars/marspi-core/rag"
)

func TestKnowledgeToolReturnsCitations(t *testing.T) {
	registry := NewRegistry(Options{
		Retriever: &rag.MockRetriever{Docs: []rag.Hit{
			{ID: "1", Content: "取消费用规则细则", Source: "sop.md"},
		}},
	})
	content, meta := registry.ExecuteQuiet("search_knowledge", map[string]any{"query": "取消"})
	text, ok := content.(string)
	if !meta.Success || !ok || !strings.Contains(text, "id=1") || !strings.Contains(text, "取消费用") {
		t.Fatalf("content=%v meta=%+v", content, meta)
	}
}

type queryRetriever struct {
	query  rag.Query
	err    error
	cancel context.CancelFunc
}

func (retriever *queryRetriever) Search(ctx context.Context, query rag.Query) ([]rag.Hit, error) {
	retriever.query = query
	if retriever.cancel != nil {
		retriever.cancel()
		return nil, ctx.Err()
	}
	return nil, retriever.err
}

func TestKnowledgeToolReceivesConfiguredQuery(t *testing.T) {
	for _, test := range []struct {
		name string
		topK int
		args map[string]any
		want int
	}{
		{"default", 0, map[string]any{"query": "rules"}, 5},
		{"configured", 3, map[string]any{"query": "rules"}, 3},
		{"overridden", 3, map[string]any{"query": "rules", "top_k": 7, "kb": " policies "}, 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			retriever := &queryRetriever{}
			registry := NewRegistry(Options{Retriever: retriever, RAGTopK: test.topK})
			_, meta := registry.ExecuteQuiet("search_knowledge", test.args)
			if !meta.Success || retriever.query.Text != "rules" || retriever.query.TopK != test.want {
				t.Fatalf("query=%+v meta=%+v", retriever.query, meta)
			}
			if test.name == "overridden" && retriever.query.KB != "policies" {
				t.Fatalf("knowledge-base scope=%q", retriever.query.KB)
			}
		})
	}
}

func TestKnowledgeToolErrorsAndCancellation(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	retriever := &queryRetriever{cancel: cancel}
	registry := NewRegistry(Options{Retriever: retriever})
	_, meta := registry.ExecuteQuietCtx(runCtx, ExecutionMeta{}, "search_knowledge", map[string]any{"query": "rules"})
	if meta.Success || !meta.Canceled {
		t.Fatalf("cancellation not preserved: %+v", meta)
	}

	retriever = &queryRetriever{err: errors.New("retrieval unavailable")}
	registry = NewRegistry(Options{Retriever: retriever})
	content, meta := registry.ExecuteQuiet("search_knowledge", map[string]any{"query": "rules"})
	if meta.Success || !strings.Contains(content.(string), "retrieval unavailable") {
		t.Fatalf("content=%v meta=%+v", content, meta)
	}
}
