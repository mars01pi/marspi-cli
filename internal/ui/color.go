// Package ui 提供终端渲染：ANSI 颜色、Printer（section/tool/thinking/diff/spinner 等）。
// 对齐 mangopi-cli 的 Printer，并发安全的 spinner 与行输出。
package ui

// ANSI 颜色，对齐 mangopi 的定义。
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Soft   = "\033[37m"
	Dim    = "\033[2m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Red    = "\033[31m"
	Grey   = "\033[90m"
	Orange = "\033[38;2;245;78;0m"
)

// C 用给定颜色包裹文本。
func C(text, color string) string { return color + text + Reset }
