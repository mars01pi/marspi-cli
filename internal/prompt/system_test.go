package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuidanceFollowsRegisteredTools(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		names := []string{"read", "bash", "attempt_completion"}
		if enabled {
			names = append(names, "search_knowledge", "use_skill")
		}
		text := New(Options{ToolNames: names, SkillDescriptions: "example-skill"}).Assemble()
		for _, term := range []string{"search_knowledge", "Knowledge Base (RAG)", "example-skill", "use_skill"} {
			if strings.Contains(text, term) != enabled {
				t.Errorf("enabled=%v: unexpected presence of %q", enabled, term)
			}
		}
		for _, name := range []string{"web_search", "edit", "search_memory", "append_memory", "view_image"} {
			if strings.Contains(text, name) {
				t.Errorf("prompt recommends unavailable tool %s", name)
			}
		}
	}
}

func TestPromptIncludesHostRulesAndEnvironment(t *testing.T) {
	root := t.TempDir()
	rulesPath := filepath.Join(root, "MARS.md")
	if err := os.WriteFile(rulesPath, []byte("Always explain the tradeoffs."), 0o644); err != nil {
		t.Fatal(err)
	}
	text := New(Options{ProjectRoot: root, RulesPath: rulesPath}).Assemble()
	for _, want := range []string{root, "Always explain the tradeoffs.", "## Safety", "## Built-in Rules"} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}
