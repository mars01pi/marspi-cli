package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMergedConfig_Precedence(t *testing.T) {
	base := mustTempDirInWorkspace(t, "mcp-test-*")
	home := filepath.Join(base, "home")
	t.Setenv("HOME", home)
	project := filepath.Join(base, "project")

	mustMkdir(t, home)
	mustMkdir(t, project)
	mustMkdir(t, filepath.Join(home, ".marspicli"))
	mustMkdir(t, filepath.Join(project, ".marspicli"))

	writeFile(t, filepath.Join(home, ".marspicli", "mcp.json"), `{"mcpServers":{"shared":{"command":"home"},"homeOnly":{"command":"h"}}}`)
	writeFile(t, filepath.Join(project, ".marspicli", "mcp.json"), `{"mcpServers":{"shared":{"command":"project"},"projectOnly":{"command":"p"}}}`)

	cfg, err := LoadMergedConfig(project)
	if err != nil {
		t.Fatalf("LoadMergedConfig error: %v", err)
	}
	if got := cfg.MCPServers["shared"].Command; got != "project" {
		t.Fatalf("shared precedence mismatch: %q", got)
	}
	if _, ok := cfg.MCPServers["homeOnly"]; !ok {
		t.Fatalf("homeOnly server missing")
	}
	if _, ok := cfg.MCPServers["projectOnly"]; !ok {
		t.Fatalf("projectOnly server missing")
	}
}

func mustTempDirInWorkspace(t *testing.T, pattern string) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", pattern)
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", p, err)
	}
}

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", p, err)
	}
}
