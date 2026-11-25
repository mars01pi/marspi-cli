package tools

import "testing"

func TestCheckCommandSafety(t *testing.T) {
	tests := []struct {
		cmd       string
		dangerous bool
	}{
		{"rm -rf /", true},
		{"rm -f foo", true},
		{"unlink bar", true},
		{"mkfs.ext4 /dev/sda", true},
		{"dd if=/dev/zero of=/dev/sda", true},
		{"chmod 777 secret", true},
		{"sudo rm foo", true},
		{"kill -9 1", true},
		{"export PATH=/evil", true},
		{"history -c", true},
		{"ls -la", false},
		{"git status", false},
		{"echo hello", false},
		{"", false},
	}
	for _, test := range tests {
		got, _ := checkCommandSafety(test.cmd)
		if got != test.dangerous {
			t.Errorf("checkCommandSafety(%q) = %v, want %v", test.cmd, got, test.dangerous)
		}
	}
}

func TestProcessBashOutput(t *testing.T) {
	in := []string{"src/a.go", "node_modules/pkg/index.js", "README.md"}
	out := processBashOutput("find . -type f", in)
	for _, line := range out {
		if line == "node_modules/pkg/index.js" {
			t.Errorf("node_modules line should be filtered: %v", out)
		}
	}
	if len(out) != 2 {
		t.Errorf("expected 2 lines after filter, got %d: %v", len(out), out)
	}
	if out := processBashOutput("cat x", in); len(out) != 3 {
		t.Errorf("non-heavy cmd should not filter, got %d", len(out))
	}
}

func TestLimitOutputLines(t *testing.T) {
	lines := make([]string, 1500)
	out := limitOutputLines(lines, 1000)
	if len(out) != 1002 {
		t.Errorf("expected 1002 lines, got %d", len(out))
	}
}

func TestGlobToRegexp(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
	}{
		{"./**/*.go", "./internal/tool/tool.go", true},
		{"./**/*.go", "./main.go", true},
		{"./**/*.py", "./main.go", false},
		{"./*.go", "./main.go", true},
		{"./*.go", "./sub/main.go", false},
	}
	for _, test := range tests {
		re, err := globToRegexp(test.pattern)
		if err != nil {
			t.Fatalf("globToRegexp(%q) error: %v", test.pattern, err)
		}
		if re.MatchString(test.path) != test.match {
			t.Errorf("glob %q vs %q = %v, want %v", test.pattern, test.path, re.MatchString(test.path), test.match)
		}
	}
}

func TestReadToolSchema(t *testing.T) {
	registry := NewRegistry(Options{})
	schemas, err := registry.SchemasChecked()
	if err != nil {
		t.Fatal(err)
	}
	var read map[string]any
	for _, schema := range schemas {
		function := schema["function"].(map[string]any)
		if function["name"] == "read" {
			read = function
		}
	}
	if read == nil {
		t.Fatal("read schema missing")
	}
	parameters := read["parameters"].(map[string]any)
	required := parameters["required"].([]string)
	if len(required) != 1 || required[0] != "path" {
		t.Errorf("expected required=[path], got %v", required)
	}
}
