package tool

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// globRecursive 支持 ** 的 glob，对齐 Python glob(recursive=True)。
// 策略：将模式按 ** 拆分，遍历文件树并用 doublestar 语义匹配。
func globRecursive(pattern string) ([]string, error) {
	pattern = filepath.Clean(pattern)
	// 找到不含通配符的根前缀，作为遍历起点
	root := globRoot(pattern)
	re, err := globToRegexp(pattern)
	if err != nil {
		return nil, err
	}
	var matches []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		p := path
		if re.MatchString(p) {
			matches = append(matches, p)
		}
		return nil
	})
	return matches, nil
}

// globRoot 返回模式中第一个通配符之前的目录部分。
func globRoot(pattern string) string {
	idx := strings.IndexAny(pattern, "*?[")
	if idx < 0 {
		return pattern
	}
	prefix := pattern[:idx]
	if i := strings.LastIndex(prefix, "/"); i >= 0 {
		if prefix[:i] == "" {
			return "/"
		}
		return prefix[:i]
	}
	return "."
}

// globToRegexp 将 glob（支持 **）编译为正则。
func globToRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// ** 匹配任意层级（含 /）
				b.WriteString(".*")
				i++
				// 吞掉紧跟的 /
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// grepTool 递归正则搜索文件内容。
type grepTool struct {
	Base
	c *ctx
}

func (t *grepTool) Name() string        { return "grep" }
func (t *grepTool) Description() string { return "Search file contents recursively using a regular expression pattern" }
func (t *grepTool) UseSpinner() bool    { return true }
func (t *grepTool) Params() []Param {
	return []Param{
		{"pat", "string", "Regular expression pattern to search for (regex syntax)"},
		{"path", "string?", "Search directory to recursively (defaults to current working directory if omitted)"},
	}
}
func (t *grepTool) Run(args map[string]any) Result {
	pat, _ := argStr(args, "pat")
	re, err := regexp.Compile(pat)
	if err != nil {
		return Fail("grep error: invalid regex: " + err.Error())
	}
	base, ok := argStr(args, "path")
	if !ok || base == "" {
		base = "."
	}
	var hits []string
	_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for n, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				hits = append(hits, path+":"+itoa(n+1)+":"+strings.TrimRight(line, "\r"))
				if len(hits) >= 500 {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if len(hits) == 0 {
		return OK("none")
	}
	if len(hits) > 500 {
		hits = hits[:500]
	}
	return OK(strings.Join(hits, "\n"))
}
