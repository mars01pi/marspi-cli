package tool

import (
	"strings"
	"testing"

	"github.com/mars/marspi-cli/internal/config"
	"github.com/mars/marspi-cli/internal/ui"
)

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
func (s stubTool) PreviewLines() int  { return s.lines }
func (s stubTool) PreviewWidth() int  { return s.width }
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
