package tools

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mars/marspi-core/console"
)

func TestFileToolsExecuteThroughCoreRegistry(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(Options{Root: root, Host: console.Nop{}})
	path := filepath.Join(root, "notes.txt")

	content, meta := registry.ExecuteQuiet("write", map[string]any{
		"path": path, "content": "hello from CLI tools",
	})
	if !meta.Success || content == nil {
		t.Fatalf("write content=%v meta=%+v", content, meta)
	}

	content, meta = registry.ExecuteQuiet("read", map[string]any{"path": path})
	if !meta.Success || !strings.Contains(content.(string), "hello from CLI tools") {
		t.Fatalf("read content=%v meta=%+v", content, meta)
	}

	content, meta = registry.ExecuteQuiet("write", map[string]any{
		"path": filepath.Join(root, "..", "outside.txt"), "content": "blocked",
	})
	if meta.Success || !strings.Contains(content.(string), "outside project root") {
		t.Fatalf("outside write content=%v meta=%+v", content, meta)
	}
}

func TestCompletionToolEndsTurn(t *testing.T) {
	registry := NewRegistry(Options{})
	content, meta := registry.ExecuteQuiet("attempt_completion", map[string]any{"result": "done"})
	if !meta.Success || !meta.EndTurn || content != "done" {
		t.Fatalf("content=%v meta=%+v", content, meta)
	}
}
