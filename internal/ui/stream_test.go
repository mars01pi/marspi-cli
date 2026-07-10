package ui

import "testing"

func TestSortStreamIDs_reasoningBeforeContent(t *testing.T) {
	ids := []string{
		"1-1-content",
		"1-0-reasoning",
		"2-1-content",
		"2-0-reasoning",
	}
	SortStreamIDs(ids)
	want := []string{
		"1-0-reasoning",
		"1-1-content",
		"2-0-reasoning",
		"2-1-content",
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids[%d] = %q, want %q (full: %v)", i, ids[i], want[i], ids)
		}
	}
}
