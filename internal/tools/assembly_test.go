package tools

import (
	"testing"

	"github.com/mars/marspi-core/console"
	"github.com/mars/marspi-core/memory"
	"github.com/mars/marspi-core/skill"
)

func TestRegisterUsesHostAndConfiguredDependencies(t *testing.T) {
	registry := NewRegistry(Options{
		Root:   t.TempDir(),
		Host:   console.Nop{},
		Memory: memory.New(t.TempDir()),
		Skills: skill.New([]string{t.TempDir()}),
	})

	for _, name := range []string{"read", "write", "edit", "bash", "search_memory", "use_skill"} {
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("expected explicitly registered tool %q", name)
		}
	}
	if _, ok := registry.Get("search_knowledge"); ok {
		t.Fatal("knowledge tool should require an injected retriever")
	}
}
