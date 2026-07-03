package tool

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
	for _, tt := range tests {
		got, _ := checkCommandSafety(tt.cmd)
		if got != tt.dangerous {
			t.Errorf("checkCommandSafety(%q) = %v, want %v", tt.cmd, got, tt.dangerous)
		}
	}
}

func TestProcessBashOutput(t *testing.T) {
	// 目录遍历命令过滤 node_modules
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

	// 非目录命令不过滤
	out2 := processBashOutput("cat x", in)
	if len(out2) != 3 {
		t.Errorf("non-heavy cmd should not filter, got %d", len(out2))
	}
}

func TestLimitOutputLines(t *testing.T) {
	lines := make([]string, 1500)
	for i := range lines {
		lines[i] = "x"
	}
	out := limitOutputLines(lines, 1000)
	if len(out) != 1002 { // 1000 + "" + truncated msg
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
	for _, tt := range tests {
		re, err := globToRegexp(tt.pattern)
		if err != nil {
			t.Fatalf("globToRegexp(%q) error: %v", tt.pattern, err)
		}
		if re.MatchString(tt.path) != tt.match {
			t.Errorf("glob %q vs %q = %v, want %v", tt.pattern, tt.path, !tt.match, tt.match)
		}
	}
}

func TestSchemaGeneration(t *testing.T) {
	tl := &readTool{}
	sch := schemaOf(tl)
	fn := sch["function"].(map[string]any)
	if fn["name"] != "read" {
		t.Errorf("expected name read, got %v", fn["name"])
	}
	params := fn["parameters"].(map[string]any)
	required := params["required"].([]string)
	// path 必填，offset/limit 可选
	if len(required) != 1 || required[0] != "path" {
		t.Errorf("expected required=[path], got %v", required)
	}
}
