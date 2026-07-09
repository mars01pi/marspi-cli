package tool

import (
	"context"
	"strings"
	"testing"

	"github.com/mars/marspi-cli/internal/config"
	"github.com/mars/marspi-cli/internal/ui"
)

type stubProvider struct {
	tools []Tool
}

func (s stubProvider) ID() string { return "stub" }
func (s stubProvider) Tools() []Tool {
	return s.tools
}
func (s stubProvider) Refresh(context.Context) error { return nil }
func (s stubProvider) Close() error                  { return nil }

type stubTool struct {
	Base
	name   string
	lines  int
	width  int
	output string
}

func (s stubTool) Name() string        { return s.name }
func (s stubTool) Description() string { return "stub" }
func (s stubTool) Params() []Param {
	return nil
}
func (s stubTool) PreviewLines() int         { return s.lines }
func (s stubTool) PreviewWidth() int         { return s.width }
func (s stubTool) Run(map[string]any) Result { return OK(s.output) }

func TestFormatResultPreviewTruncates(t *testing.T) {
	st := stubTool{name: "t", lines: 2, width: 10, output: "line1\nline2\nline3"}
	lines := FormatResultPreview(st, st.output)
	if len(lines) != 3 {
		t.Fatalf("lines: %v", lines)
	}
	if !strings.HasPrefix(lines[2], "... and 1 more line") {
		t.Fatalf("suffix: %q", lines[2])
	}
}

func TestExecuteQuietUnknownTool(t *testing.T) {
	cfg := &config.Config{ProjectRoot: t.TempDir()}
	reg := NewRegistry(cfg, ui.NewPrinter(), nil, nil)
	content, meta := reg.ExecuteQuiet("missing", nil)
	if meta.Success {
		t.Fatal("expected failure")
	}
	if _, ok := content.(string); !ok {
		t.Fatalf("content type: %T", content)
	}
}

func TestRegistryAddProviderAddsTools(t *testing.T) {
	cfg := &config.Config{ProjectRoot: t.TempDir()}
	reg := NewRegistry(cfg, ui.NewPrinter(), nil, nil)
	err := reg.AddProvider(stubProvider{tools: []Tool{stubTool{name: "prov", lines: 2, width: 20, output: "ok"}}})
	if err != nil {
		t.Fatalf("AddProvider error: %v", err)
	}
	if _, ok := reg.Get("prov"); !ok {
		t.Fatalf("provider tool not found")
	}
}
