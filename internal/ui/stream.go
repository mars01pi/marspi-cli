package ui

import "sort"

// SortStreamIDs sorts LLM stream block IDs for display.
// IDs use "{iteration}-{rank}-{field}" where reasoning rank is 0 and content is 1.
func SortStreamIDs(ids []string) {
	sort.Strings(ids)
}
