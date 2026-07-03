package flash

import "testing"

func TestMatchKeyword(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"帮我 debug 这个报错", "debug"},
		{"设计一个分布式架构", "design"},
		{"解释一下这个原理", "explain"},
		{"优化性能", "optimize"},
		{"实现一个新功能", "implement"},
		{"随便聊聊天气", ""},
	}
	for _, tt := range tests {
		if got := Match(tt.query, nil); got != tt.want {
			t.Errorf("Match(%q) = %q, want %q", tt.query, got, tt.want)
		}
	}
}

func TestMatchPhasePriority(t *testing.T) {
	// tool pattern 优先于 query 关键词
	if got := Match("解释原理", []string{"edit", "write"}); got != "implement" {
		t.Errorf("expected phase executing→implement, got %q", got)
	}
	if got := Match("解释原理", []string{"read", "grep"}); got != "investigate" {
		t.Errorf("expected phase exploring→investigate, got %q", got)
	}
}

func TestFormatFramework(t *testing.T) {
	out := FormatFramework("debug")
	if out == "" {
		t.Fatal("debug framework should not be empty")
	}
	if FormatFramework("nonexistent") != "" {
		t.Error("nonexistent framework should be empty")
	}
}
