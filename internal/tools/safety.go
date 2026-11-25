package tools

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// filteredDirs 是目录遍历输出时要过滤的大型/无意义目录，对齐 FILTERED_DIRS。
var filteredDirs = []string{
	".git", "node_modules", "__pycache__", ".venv", "venv", "dist", "build", ".next", ".turbo", ".idea",
	".vscode", ".mypy_cache", ".pytest_cache", ".cache", "target", "vendor",
}

// imageExts 是被视为图片的扩展名，对齐 IMAGE_EXTS。
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
}

// dangerRule 是一条危险命令规则。
type dangerRule struct {
	re     *regexp.Regexp
	reason string
}

var dangerRules = func() []dangerRule {
	specs := []struct {
		pat    string
		reason string
	}{
		{`\brm\s+.*-[rf]`, "File deletion"}, {`\brm\s+-[rf]`, "File deletion"},
		{`\bunlink\b`, "File deletion"}, {`\brm\s+(-[rf]+\s+)?.*`, "File deletion"},
		{`\bmkfs\b`, "Disk formatting or partition"}, {`\bfdisk\b`, "Disk formatting or partition"},
		{`\bparted\b`, "Disk formatting or partition"}, {`\bdd\s+.*if=.*of=`, "Disk formatting or partition"},
		{`\bchmod\s+(?:-[a-zA-Z]+\s+)*\d*7\d*7\b`, "Dangerous permission change"}, {`\bchmod\s+777\b`, "Dangerous permission change"},
		{`\bchmod\s+\d*7\d*7\b`, "Dangerous permission change"}, {`\bchown\s+.*root\b`, "Dangerous permission change"},
		{`\bsudo\s+.*rm\b`, "Privilege escalation"}, {`\bsu\s+-\b`, "Privilege escalation"},
		{`\bsu\s+root\b`, "Privilege escalation"},
		{`\bkill\s+-9\s+1\b`, "Dangerous process operation"}, {`\bkillall\s+-9\b`, "Dangerous process operation"},
		{`\bpkill\s+-9\b`, "Dangerous process operation"}, {`\bkill\s+-9\s+-\d+\b`, "Dangerous process operation"},
		{`\bexport\s+PATH=`, "Environment or system config change"}, {`\bunset\s+PATH\b`, "Environment or system config change"},
		{`>>?\s*/etc/`, "Environment or system config change"}, {`\becho\s+.*>\s*/etc/`, "Environment or system config change"},
		{`\bhistory\s+-c\b`, "History/log clearing"}, {`>\s*/dev/null\s+2>&1`, "History/log clearing"},
	}
	rules := make([]dangerRule, 0, len(specs))
	for _, s := range specs {
		rules = append(rules, dangerRule{re: regexp.MustCompile(`(?i)` + s.pat), reason: s.reason})
	}
	return rules
}()

// checkCommandSafety 判断命令是否危险，返回 (dangerous, 原因文案)。
func checkCommandSafety(command string) (bool, string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return false, ""
	}
	for _, r := range dangerRules {
		if r.re.MatchString(command) {
			return true, r.reason
		}
	}
	return false, ""
}

// validateFilePath 校验路径必须位于项目根目录内，且不是目录，对齐 _validate_file_path。
func validateFilePath(root, path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "path '" + path + "' invalid"
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		real = abs // 文件可能尚不存在（write 新文件）
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}
	if real != realRoot && !strings.HasPrefix(real, realRoot+string(os.PathSeparator)) {
		return "path '" + path + "' is outside project root"
	}
	if fi, err := os.Stat(real); err == nil && fi.IsDir() {
		return "path '" + path + "' is a directory, not a file"
	}
	return ""
}

// isDirectoryHeavy 判断是否目录遍历类命令，对齐 _is_directory_heavy。
func isDirectoryHeavy(command string) bool {
	for _, k := range []string{"find ", "tree", "ls -R", "du ", "fd ", "rg "} {
		if strings.Contains(command, k) {
			return true
		}
	}
	return false
}

func dirMatches(line, d string) bool {
	return strings.Contains(line, "/"+d+"/") || strings.Contains(line, "/"+d+":") ||
		strings.HasPrefix(line, d+"/") || strings.HasPrefix(line, "./"+d+"/") ||
		strings.HasPrefix(line, "./"+d+":") || line == d || line == "./"+d ||
		strings.HasSuffix(line, "/"+d)
}

func filterDirectoryOutput(lines []string) []string {
	out := lines[:0:0]
	for _, line := range lines {
		skip := false
		for _, d := range filteredDirs {
			if dirMatches(line, d) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, line)
		}
	}
	return out
}

func limitOutputLines(lines []string, maxLines int) []string {
	if len(lines) <= maxLines {
		return lines
	}
	trimmed := append([]string(nil), lines[:maxLines]...)
	return append(trimmed, "", "... truncated "+itoa(len(lines)-maxLines)+" lines ...")
}

// processBashOutput 对 bash 输出做目录过滤与行数限制，对齐 _process_bash_output。
func processBashOutput(command string, lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	if isDirectoryHeavy(command) {
		lines = filterDirectoryOutput(lines)
	}
	return limitOutputLines(lines, 1000)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
