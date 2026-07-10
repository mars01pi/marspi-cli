// Package i18n 提供中英双语文案，对齐 mangopi-cli 的 I18N/HELP_COMMANDS。
package i18n

// pair 是一个 {zh,en} 文案对。
type pair struct{ zh, en string }

var dict = map[string]pair{
	"tool.call":                     {"工具调用", "Tool call"},
	"tool.result.ok":                {"成功应用", "Applied successfully"},
	"tool.result.fail":              {"执行失败", "Execution failed"},
	"tool.denied":                   {"已拒绝", "Denied by user"},
	"llm.thinking":                  {"思考中", "Thinking"},
	"llm.output":                    {"输出", "Output"},
	"context.compact":               {"上下文压缩", "Context compact"},
	"context.compact.strategy":      {"策略", "Strategy"},
	"context.round":                 {"轮次", "round"},
	"context.tokens_in_out":         {"tokens 输入/输出", "tokens in/out"},
	"cli.welcome":                   {"Marspi CLI — 基于大模型的命令行编程助手", "Marspi CLI — Large Model CLI Assistant"},
	"cli.help_intro":                {"内置命令:", "Built-in commands:"},
	"safety.warn.dangerous_command": {"检测到危险命令", "Dangerous command detected"},
	"safety.danger.rm":              {"文件删除", "File deletion"},
	"safety.danger.mkfs":            {"磁盘格式化或分区", "Disk formatting or partition"},
	"safety.danger.chmod":           {"危险权限修改", "Dangerous permission change"},
	"safety.danger.sudo":            {"提权操作", "Privilege escalation"},
	"safety.danger.kill":            {"危险进程操作", "Dangerous process operation"},
	"safety.danger.env":             {"环境变量或系统配置修改", "Environment or system config change"},
	"safety.danger.history":         {"清理历史/日志", "History/log clearing"},
}

// HelpCommand 是一条内置命令说明。
type HelpCommand struct {
	Cmd    string
	zh, en string
}

// Desc 返回当前语言的说明。
func (h HelpCommand) Desc() string {
	if lang == "zh" {
		return h.zh
	}
	return h.en
}

// HelpCommands 是内置命令的说明，按显示顺序排列。
var HelpCommands = []HelpCommand{
	{"/q or /quit", "退出程序", "Quit"},
	{"/c or /compact", "手动压缩当前会话（释放上下文空间）", "Manually compact current session"},
	{"/n or /new", "结束当前会话并创建一个全新的会话", "End current session and start a new one"},
	{"/h or /help", "显示本帮助信息", "Show this help info"},
	{"/g or /goal", "[已废弃] 请改用 /loop", "[deprecated] Use /loop instead"},
	{"/l or /loop", "启动循环工程完成你的目标", "Start Loop Engineering to complete your goal"},
	{"/lg or /loopg", "用 marspi-graph 跑循环工程（实验）", "Run Loop Engineering via marspi-graph (experimental)"},
	{"/sv or /supervise", "Supervisor 动态调度（实验；派 coder 前会确认）", "Supervisor multi-agent routing (experimental; confirms before coder)"},
}

// lang 是进程级语言设置，默认英文。由 SetLang 初始化。
var lang = "en"

// SetLang 设置全局语言（"zh" 或 "en"）。
func SetLang(l string) {
	if l == "zh" {
		lang = "zh"
	} else {
		lang = "en"
	}
}

// T 返回 key 对应的当前语言文案，缺失返回空串。
func T(key string) string {
	p, ok := dict[key]
	if !ok {
		return ""
	}
	if lang == "zh" {
		return p.zh
	}
	return p.en
}
