package tools

import (
	"context"
	"reflect"
	"testing"

	"github.com/mars/marspi-core/rag"
)

func TestNewRegistryRegistersDefaultTools(t *testing.T) {
	registry := NewRegistry(Options{})
	want := []string{
		"read", "write", "edit", "search", "grep", "bash",
		"use_skill", "search_memory", "append_memory", "web_search",
		"view_image", "attempt_completion",
	}
	if got := registry.Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("tool names = %v, want %v", got, want)
	}
}

func TestNewRegistryAddsKnowledgeOnlyWhenConfigured(t *testing.T) {
	registry := NewRegistry(Options{Retriever: &stubRetriever{}})
	if _, ok := registry.Get("search_knowledge"); !ok {
		t.Fatal("configured knowledge retriever was not registered")
	}
}

type stubRetriever struct{}

func (*stubRetriever) Search(context.Context, rag.Query) ([]rag.Hit, error) { return nil, nil }
