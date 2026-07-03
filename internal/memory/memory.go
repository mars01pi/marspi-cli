// Package memory 实现基于 Markdown 的长期记忆：按日追加，关键词打分检索。
// 对齐 mangopi-cli 的 MemoryManager。
package memory

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Manager 管理记忆目录下的 Markdown 文件。
type Manager struct {
	dir string
	now func() time.Time // 便于测试注入时间
}

// New 构建 Manager。
func New(dir string) *Manager {
	return &Manager{dir: dir, now: time.Now}
}

func (m *Manager) todayPath() string {
	return filepath.Join(m.dir, m.now().Format("2006-01-02")+".md")
}

// Append 追加一条记忆到当日文件。
func (m *Manager) Append(content string) error {
	f, err := os.OpenFile(m.todayPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(strings.TrimSpace(content) + "\n\n")
	return err
}

func tokenize(text string) []string {
	var out []string
	for _, x := range strings.Fields(text) {
		x = strings.ToLower(strings.TrimSpace(x))
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

// splitChunks 按空行切分文本块，对齐 _split_chunks。
func splitChunks(text string) []string {
	var chunks []string
	for _, c := range strings.Split(text, "\n\n") {
		c = strings.TrimSpace(c)
		if c != "" {
			chunks = append(chunks, c)
		}
	}
	return chunks
}

type scored struct {
	score   int
	file    string
	content string
}

// Search 在全部记忆中按关键词打分检索，返回渲染文本。
func (m *Manager) Search(query string, topK int) string {
	keywords := tokenize(query)
	if len(keywords) == 0 {
		return "empty query"
	}
	files, _ := filepath.Glob(filepath.Join(m.dir, "*.md"))
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	var results []scored
	nowUnix := m.now().Unix()
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var mtimeUnix int64
		if fi, e := os.Stat(path); e == nil {
			mtimeUnix = fi.ModTime().Unix()
		}
		for _, chunk := range splitChunks(string(data)) {
			lower := strings.ToLower(chunk)
			score := 0
			for _, kw := range keywords {
				if strings.Contains(lower, kw) {
					score += strings.Count(lower, kw) * 10
				}
			}
			if score <= 0 {
				continue
			}
			if bonus := len(chunk) / 200; bonus < 5 {
				score += bonus
			} else {
				score += 5
			}
			if mb := 30 - int((nowUnix-mtimeUnix)/86400); mb > 0 {
				score += mb
			}
			c := chunk
			if len(c) > 2000 {
				c = c[:2000]
			}
			results = append(results, scored{score: score, file: filepath.Base(path), content: c})
		}
	}
	if len(results) == 0 {
		return "No memory found. Tip: append important user preferences, decisions, " +
			"and non-obvious fixes so future sessions can recall them."
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > topK {
		results = results[:topK]
	}
	var out []string
	for _, it := range results {
		out = append(out, "# "+it.file+" (score="+itoa(it.score)+")\n"+it.content)
	}
	return strings.Join(out, "\n\n---\n\n")
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
