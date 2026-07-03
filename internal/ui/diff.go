package ui

import "strings"

// Diff 打印一个简化的 unified diff（old→new），用于 edit 工具预览。
// Go 标准库无 difflib，这里用最小实现：整体删除旧块、整体加入新块。
func (p *Printer) Diff(old, new, filename string) {
	p.Section("Code Diff")
	p.writeLine(C("--- a/"+filename, Grey))
	p.writeLine(C("+++ b/"+filename, Grey))
	for _, line := range strings.Split(old, "\n") {
		p.writeLine(C("-"+line, Red))
	}
	for _, line := range strings.Split(new, "\n") {
		p.writeLine(C("+"+line, Green))
	}
}
