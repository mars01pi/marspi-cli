package tool

// consoleIface 是工具需要的输出能力子集，便于测试替换。
// *ui.Printer 天然满足该接口。
type consoleIface interface {
	Diff(old, new, filename string)
	PromptApply(message string) bool
}
