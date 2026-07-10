package cmd

import "testing"

func TestParseSuperviseRequest(t *testing.T) {
	tests := []struct {
		in      string
		ok      bool
		goal    string
		resume  string
		list    bool
	}{
		{"/sv", false, "", "", false},
		{"/sv fix the bug", true, "fix the bug", "", false},
		{"/supervise do stuff", true, "do stuff", "", false},
		{"/sv list", true, "", "", true},
		{"/sv resume thr-1", true, "", "thr-1", false},
		{"/sv --resume thr-2", true, "", "thr-2", false},
		{"/sv -r thr-3", true, "", "thr-3", false},
		{"/sv resume", false, "", "", false},
		{"/other", false, "", "", false},
	}
	for _, tc := range tests {
		got, ok := parseSuperviseRequest(tc.in)
		if ok != tc.ok {
			t.Fatalf("%q: ok=%v want %v", tc.in, ok, tc.ok)
		}
		if !ok {
			continue
		}
		if got.Goal != tc.goal || got.ResumeThreadID != tc.resume || got.List != tc.list {
			t.Fatalf("%q: %+v", tc.in, got)
		}
	}
}
