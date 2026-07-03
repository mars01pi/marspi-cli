// Package flash 实现思考框架增强系统（Flash-ext 思维引导）。
// 根据 query 关键词和 tool call 模式选择结构化思考框架。
// 对齐 mangopi 的 FlashThinking。
package flash

import "strings"

// keywords 是各框架的触发关键词，对齐 FlashThinking.KEYWORDS。
var keywords = map[string][]string{
	"debug": {"报错", "bug", "error", "失败", "fail", "慢", "slow", "崩溃", "crash", "排查", "debug",
		"修复", "fix", "test", "修改", "modif", "update", "chang", "issue", "adjust",
		"patch", "correct", "错误", "问题", "调整", "更正", "改动", "alter"},
	"design": {"设计", "design", "架构", "architect", "选型", "规划",
		"distribut", "microservic", "scalab", "infrastructur",
		"overall", "可扩展", "高可用", "容灾", "分布式", "framework", "platform",
		"重构", "refactor", "migrat", "死锁", "deadlock", "并发", "concurren",
		"async", "multithread", "异步", "迁移"},
	"explain": {"什么是", "解释", "explain", "区别", "原理", "怎么理解", "what is",
		"read", "查看", "show", "find", "search", "搜索", "查询", "query",
		"display", "获取", "了解", "描述", "describe"},
	"optimize": {"优化", "optimize", "性能", "performance", "加速", "提升"},
	"implement": {"实现", "implement", "写", "create", "build", "开发", "生成",
		"integrat", "multi", "feature", "api", "interfac", "modul",
		"component", "databas", "config", "集成", "接口", "模块", "组件", "数据库", "存储", "stor"},
}

// keywordOrder 固定框架匹配顺序（Go map 无序，需显式），对齐 Python dict 插入序。
var keywordOrder = []string{"debug", "design", "explain", "optimize", "implement"}

// phaseMap 将阶段映射到框架名。
var phaseMap = map[string]string{
	"exploring": "investigate", "executing": "implement", "verifying": "verify", "stuck": "reevaluate",
}

var frameworks = map[string][]string{
	"debug": {
		"Reproduce the issue and confirm trigger conditions",
		"List all possible causes, ordered by likelihood",
		"Design verification method for each cause",
		"Eliminate causes one by one to find root cause",
		"Propose fix and verification steps"},
	"design": {
		"Clarify requirements and constraints",
		"Propose 2-3 viable approaches",
		"Compare pros and cons of each approach",
		"Choose one approach and justify the choice",
		"Outline key implementation steps"},
	"explain": {
		"Give a one-sentence summary first",
		"Expand on key details",
		"Provide a concrete example"},
	"optimize": {
		"Measure current performance baseline",
		"Locate bottleneck with data, not guesses",
		"Propose optimization for the bottleneck",
		"Estimate expected improvement",
		"Define verification method"},
	"implement": {
		"Understand requirements: confirm inputs and outputs",
		"Design data structures and core logic",
		"Write main logic first, then handle edge cases",
		"Verify: manually trace the happy path",
		"List possible failure scenarios"},
	"investigate": {
		"Summarize what is already known",
		"Identify missing critical information",
		"Plan information-gathering order",
		"Gather first, decide later — don't rush into action"},
	"verify": {
		"Check each expected result one by one",
		"For each: pass = record, fail = fix",
		"List all remaining issues",
		"Confirm no new problems introduced"},
	"reevaluate": {
		"Stop and re-examine current assumptions",
		"Could previous conclusions be wrong?",
		"Are there completely different approaches?",
		"List alternatives and evaluate feasibility one by one"},
}

// phaseOf 根据 tool 模式判断阶段，对齐 FlashThinking.PHASES。
func phaseOf(tools []string) string {
	if len(tools) == 0 {
		return ""
	}
	allExplore := true
	anyEdit := false
	bashCount := 0
	for _, t := range tools {
		switch t {
		case "read", "grep", "search", "search_memory":
		default:
			allExplore = false
		}
		if t == "edit" || t == "write" {
			anyEdit = true
		}
		if t == "bash" {
			bashCount++
		}
	}
	// 顺序对齐 Python dict: exploring, executing, verifying
	if allExplore {
		return "exploring"
	}
	if anyEdit {
		return "executing"
	}
	if bashCount >= 2 {
		return "verifying"
	}
	return ""
}

// Match 匹配思考框架：优先 tool pattern 阶段，其次 query 关键词。
// 返回框架名，未命中返回 ""。
func Match(query string, toolPattern []string) string {
	q := strings.ToLower(query)
	if len(toolPattern) > 0 {
		if phase := phaseOf(toolPattern); phase != "" {
			return phaseMap[phase]
		}
	}
	for _, fw := range keywordOrder {
		for _, kw := range keywords[fw] {
			if strings.Contains(q, kw) {
				return fw
			}
		}
	}
	return ""
}

// FormatFramework 返回框架的编号步骤文本，用于注入 user content。
func FormatFramework(name string) string {
	steps, ok := frameworks[name]
	if !ok {
		return ""
	}
	var b strings.Builder
	for i, s := range steps {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(itoa(i + 1))
		b.WriteString(". ")
		b.WriteString(s)
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
