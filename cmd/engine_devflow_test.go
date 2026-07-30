package cmd

import "testing"

func TestParseDevflowRequest(t *testing.T) {
	tests := []struct {
		in       string
		ok       bool
		goal     string
		resume   string
		list     bool
		workflow string
		noPush   bool
	}{
		{"/df", false, "", "", false, "", false},
		{"/df add history tabs", true, "add history tabs", "", false, "", false},
		{"/devflow fix bug", true, "fix bug", "", false, "", false},
		{"/df list", true, "", "", true, "", false},
		{"/df resume thr-1", true, "", "thr-1", false, "", false},
		{"/df --resume thr-2", true, "", "thr-2", false, "", false},
		{"/df -r thr-3", true, "", "thr-3", false, "", false},
		{"/df resume", false, "", "", false, "", false},
		{"/df --workflow ./w.yaml do it", true, "do it", "", false, "./w.yaml", false},
		{"/df --no-push ship feature", true, "ship feature", "", false, "", true},
		{"/df -w ./a.yaml --no-push x", true, "x", "", false, "./a.yaml", true},
		{"/other", false, "", "", false, "", false},
	}
	for _, tc := range tests {
		got, ok := parseDevflowRequest(tc.in)
		if ok != tc.ok {
			t.Fatalf("%q: ok=%v want %v", tc.in, ok, tc.ok)
		}
		if !ok {
			continue
		}
		wantPush := !tc.noPush
		if got.Goal != tc.goal || got.ResumeThreadID != tc.resume || got.List != tc.list ||
			got.WorkflowPath != tc.workflow || got.AllowPush != wantPush {
			t.Fatalf("%q: %+v", tc.in, got)
		}
	}
}
