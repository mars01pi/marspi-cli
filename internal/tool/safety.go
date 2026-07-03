package tool

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mars/marspi-cli/internal/i18n"
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
	reason string // i18n key
}

var dangerRules = func() []dangerRule {
	specs := []struct {
		pat    string
		reason string
	}{
		{`\brm\s+.*-[rf]`, "safety.danger.rm"}, {`\brm\s+-[rf]`, "safety.danger.rm"},
		{`\bunlink\b`, "safety.danger.rm"}, {`\brm\s+(-[rf]+\s+)?.*`, "safety.danger.rm"},
		{`\bmkfs\b`, "safety.danger.mkfs"}, {`\bfdisk\b`, "safety.danger.mkfs"},
		{`\bparted\b`, "safety.danger.mkfs"}, {`\bdd\s+.*if=.*of=`, "safety.danger.mkfs"},
		{`\bchmod\s+(?:-[a-zA-Z]+\s+)*\d*7\d*7\b`, "safety.danger.chmod"}, {`\bchmod\s+777\b`, "safety.danger.chmod"},
		{`\bchmod\s+\d*7\d*7\b`, "safety.danger.chmod"}, {`\bchown\s+.*root\b`, "safety.danger.chmod"},
		{`\bsudo\s+.*rm\b`, "safety.danger.sudo"}, {`\bsu\s+-\b`, "safety.danger.sudo"},
		{`\bsu\s+root\b`, "safety.danger.sudo"},
		{`\bkill\s+-9\s+1\b`, "safety.danger.kill"}, {`\bkillall\s+-9\b`, "safety.danger.kill"},
		{`\bpkill\s+-9\b`, "safety.danger.kill"}, {`\bkill\s+-9\s+-\d+\b`, "safety.danger.kill"},
		{`\bexport\s+PATH=`, "safety.danger.env"}, {`\bunset\s+PATH\b`, "safety.danger.env"},
		{`>>?\s*/etc/`, "safety.danger.env"}, {`\becho\s+.*>\s*/etc/`, "safety.danger.env"},
		{`\bhistory\s+-c\b`, "safety.danger.history"}, {`>\s*/dev/null\s+2>&1`, "safety.danger.history"},
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
			return true, i18n.T(r.reason)
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
