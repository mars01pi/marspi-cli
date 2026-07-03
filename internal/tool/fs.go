package tool

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ctx 是工具共享的运行环境（配置与输出器）。
type ctx struct {
	root    string
	console consoleIface
}

// readTool 读取文件（文本或图片；图片路由到 view_image）。
type readTool struct {
	Base
	c *ctx
	image *viewImageTool
}

func (t *readTool) Name() string        { return "read" }
func (t *readTool) Description() string { return "Read a file from the local filesystem (text or image; images are auto-routed to vision)" }
func (t *readTool) Params() []Param {
	return []Param{
		{"path", "string", "Path to the file to read (text or image: png/jpg/jpeg/gif/webp)"},
		{"offset", "number?", "Line number to start reading from (0-indexed, default 0)"},
		{"limit", "number?", "Maximum number of lines to read (default: all lines)"},
	}
}
func (t *readTool) Preview(args map[string]any) string {
	s, _ := argStr(args, "path")
	return truncate(s, t.PreviewWidth())
}
func (t *readTool) Run(args map[string]any) Result {
	path, _ := argStr(args, "path")
	ext := strings.ToLower(filepath.Ext(path))
	_, hasOff := args["offset"]
	_, hasLim := args["limit"]
	if imageExts[ext] && !hasOff && !hasLim {
		return t.image.Run(map[string]any{"path": path})
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Fail("read " + path + " error: " + err.Error())
	}
	lines := strings.Split(string(data), "\n")
	// 与 Python readlines 对齐：末尾换行会产生一个空串，去掉它
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	offset := 0
	if v, ok := argInt(args, "offset"); ok {
		offset = v
	}
	limit := len(lines)
	if v, ok := argInt(args, "limit"); ok {
		limit = v
	}
	if offset > len(lines) {
		offset = len(lines)
	}
	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}
	var b strings.Builder
	for i := offset; i < end; i++ {
		b.WriteString(padLineNo(i + 1))
		b.WriteString("| ")
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	return OK(b.String())
}

func padLineNo(n int) string {
	s := itoa(n)
	for len(s) < 4 {
		s = " " + s
	}
	return s
}

// writeTool 写文件（覆盖）。
type writeTool struct {
	Base
	c *ctx
}

func (t *writeTool) Name() string        { return "write" }
func (t *writeTool) Description() string { return "Write content to a file, overwriting if it exists" }
func (t *writeTool) Params() []Param {
	return []Param{
		{"path", "string", "Path to the file to write"},
		{"content", "string", "Content to write to the file"},
	}
}
func (t *writeTool) Preview(args map[string]any) string {
	s, _ := argStr(args, "path")
	return truncate(s, t.PreviewWidth())
}
func (t *writeTool) Run(args map[string]any) Result {
	path, _ := argStr(args, "path")
	content, _ := argStr(args, "content")
	if e := validateFilePath(t.c.root, path); e != "" {
		return Fail("write " + path + " error: " + e)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return Fail("write " + path + " error: " + err.Error())
	}
	return OK("write " + itoa(len(content)) + "byte to " + path + " ok")
}

// editTool 精确字符串替换。
type editTool struct {
	Base
	c *ctx
}

func (t *editTool) Name() string        { return "edit" }
func (t *editTool) Description() string { return "Edit a file by replacing an exact string with a new string" }
func (t *editTool) Params() []Param {
	return []Param{
		{"path", "string", "Path to the file to edit"},
		{"old", "string", "Exact string to be replaced"},
		{"new", "string", "String to replace it with"},
		{"all", "boolean?", "Replace all occurrences (default: false)"},
	}
}
func (t *editTool) Preview(args map[string]any) string {
	s, _ := argStr(args, "path")
	return truncate(s, t.PreviewWidth())
}
func (t *editTool) Before(args map[string]any) {
	old, _ := argStr(args, "old")
	nw, _ := argStr(args, "new")
	path, _ := argStr(args, "path")
	if old != "" && nw != "" {
		t.c.console.Diff(old, nw, path)
	}
}
func (t *editTool) Confirm(args map[string]any) bool {
	path, _ := argStr(args, "path")
	return t.c.console.PromptApply("Edit " + path + " (y or n)?")
}
func (t *editTool) Run(args map[string]any) Result {
	path, _ := argStr(args, "path")
	old, _ := argStr(args, "old")
	nw, _ := argStr(args, "new")
	if e := validateFilePath(t.c.root, path); e != "" {
		return Fail("edit error: " + e)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Fail("edit error: " + err.Error())
	}
	text := string(data)
	if !strings.Contains(text, old) {
		return Fail("edit error: old_string not found")
	}
	count := strings.Count(text, old)
	all := argBool(args, "all")
	if !all && count > 1 {
		return Fail("error: old_string appears " + itoa(count) + " times, must be unique (use all=true)")
	}
	var out string
	if all {
		out = strings.ReplaceAll(text, old, nw)
	} else {
		out = strings.Replace(text, old, nw, 1)
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return Fail("edit error: " + err.Error())
	}
	return OK("edit " + path + " ok")
}

// searchTool 用 glob 查找文件。
type searchTool struct {
	Base
	c *ctx
}

func (t *searchTool) Name() string        { return "search" }
func (t *searchTool) Description() string { return "Search for files using a glob pattern" }
func (t *searchTool) UseSpinner() bool    { return true }
func (t *searchTool) Params() []Param {
	return []Param{
		{"pat", "string", "Glob pattern to match file paths (e.g. '**/*.py')"},
		{"path", "string?", "Directory to start search from (default: current directory)"},
	}
}
func (t *searchTool) Run(args map[string]any) Result {
	pat, _ := argStr(args, "pat")
	base, ok := argStr(args, "path")
	if !ok || base == "" {
		base = "."
	}
	pattern := strings.ReplaceAll(base+"/"+pat, "//", "/")
	files, err := globRecursive(pattern)
	if err != nil {
		return Fail("search error: " + err.Error())
	}
	// 按修改时间降序
	sort.SliceStable(files, func(i, j int) bool {
		return mtime(files[i]) > mtime(files[j])
	})
	if len(files) == 0 {
		return OK("none")
	}
	return OK(strings.Join(files, "\n"))
}

func mtime(p string) int64 {
	fi, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return fi.ModTime().UnixNano()
}
