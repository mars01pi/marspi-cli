// Package skill 加载基于 SKILL.md 的技能，对齐 mangopi-cli 的 SkillManager。
package skill

import (
	"os"
	"path/filepath"
	"strings"
)

// Skill 表示一个已加载的技能。
type Skill struct {
	Name        string
	Description string
	Body        string
	Meta        map[string]string
	Tags        []string
	Scripts     map[string]string // 路径 -> 内容
	References  map[string]string
}

// Manager 负责发现并加载技能。
type Manager struct {
	basePaths []string
	skills    map[string]*Skill
}

// New 构建 Manager 并加载技能。basePaths 为空时使用默认路径。
func New(basePaths []string) *Manager {
	if len(basePaths) == 0 {
		home, _ := os.UserHomeDir()
		basePaths = []string{filepath.Join(home, ".marspicli", "skills")}
	}
	m := &Manager{basePaths: basePaths, skills: map[string]*Skill{}}
	m.load()
	return m
}

// AddBasePath 追加一个技能搜索路径并重新加载。
func (m *Manager) AddBasePath(p string) {
	m.basePaths = append(m.basePaths, p)
	m.load()
}

func loadDirectory(skillDir, name string) map[string]string {
	dir := filepath.Join(skillDir, name)
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	out := map[string]string{}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if data, e := os.ReadFile(path); e == nil {
			out[path] = string(data)
		}
		return nil
	})
	return out
}

// parseFrontmatter 解析 SKILL.md 的 YAML frontmatter，返回 (meta, body)。
func parseFrontmatter(content string) (map[string]string, []string, string, bool) {
	if !strings.HasPrefix(content, "---") {
		return nil, nil, "", false
	}
	rest := content[3:]
	idx := strings.Index(rest, "---")
	if idx == -1 {
		return nil, nil, "", false
	}
	yamlText := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(rest[idx+3:])
	meta := map[string]string{}
	var tags []string
	for _, line := range strings.Split(yamlText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(v)
		meta[key] = val
		if key == "tags" {
			tags = parseTags(val)
		}
	}
	return meta, tags, body, true
}

// parseTags 解析形如 [a, b, c] 的简单列表。
func parseTags(v string) []string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	var out []string
	for _, t := range strings.Split(v, ",") {
		t = strings.Trim(strings.TrimSpace(t), `"'`)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (m *Manager) load() {
	m.skills = map[string]*Skill{}
	for _, base := range m.basePaths {
		matches, _ := filepath.Glob(filepath.Join(base, "*", "SKILL.md"))
		for _, skillMD := range matches {
			data, err := os.ReadFile(skillMD)
			if err != nil {
				continue
			}
			meta, tags, body, ok := parseFrontmatter(string(data))
			if !ok {
				continue
			}
			skillDir := filepath.Dir(skillMD)
			name := filepath.Base(skillDir)
			m.skills[name] = &Skill{
				Name:        name,
				Description: meta["description"],
				Body:        body,
				Meta:        meta,
				Tags:        tags,
				Scripts:     loadDirectory(skillDir, "scripts"),
				References:  loadDirectory(skillDir, "references"),
			}
		}
	}
}

// All 返回全部技能。
func (m *Manager) All() map[string]*Skill { return m.skills }

// Get 按名取技能。
func (m *Manager) Get(name string) (*Skill, bool) {
	s, ok := m.skills[name]
	return s, ok
}

// Descriptions 返回 "- name: description" 列表文本。
func (m *Manager) Descriptions() string {
	var lines []string
	for name, s := range m.skills {
		lines = append(lines, "- "+name+": "+s.Description)
	}
	return strings.Join(lines, "\n")
}
