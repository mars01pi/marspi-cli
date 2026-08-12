package main

import "strings"

// stringListFlag 收集可重复的 --add-dir 参数。
type stringListFlag []string

func (s *stringListFlag) String() string { return strings.Join(*s, ",") }
func (s *stringListFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}
