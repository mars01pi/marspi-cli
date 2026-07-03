package tool

import (
	"encoding/base64"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/mars/marspi-cli/internal/memory"
	"github.com/mars/marspi-cli/internal/skill"
)

// useSkillTool 加载一个已安装技能的说明、脚本与引用。
type useSkillTool struct {
	Base
	skills *skill.Manager
}

func (t *useSkillTool) Name() string        { return "use_skill" }
func (t *useSkillTool) Description() string { return "Load an installed skill with guidance, scripts and references" }
func (t *useSkillTool) Params() []Param {
	return []Param{{"name", "string", "Skill name"}}
}
func (t *useSkillTool) Run(args map[string]any) Result {
	name, _ := argStr(args, "name")
	s, ok := t.skills.Get(name)
	if !ok {
		return Fail("skill '" + name + "' not found")
	}
	var out []string
	out = append(out, "# Skill: "+name, s.Body)
	if len(s.Scripts) > 0 {
		out = append(out, "\n## Scripts\n")
		for path := range s.Scripts {
			out = append(out, path)
		}
	}
	if len(s.References) > 0 {
		out = append(out, "\n## References\n")
		for path := range s.References {
			out = append(out, path)
		}
	}
	return OK(strings.Join(out, "\n"))
}

// searchMemoryTool 检索长期记忆。
type searchMemoryTool struct {
	Base
	mem *memory.Manager
}

func (t *searchMemoryTool) Name() string { return "search_memory" }
func (t *searchMemoryTool) Description() string {
	return "Search YOUR long-term memory — notes YOU have saved in past sessions. CALL THIS WHEN: " +
		"(1) user references past work ('last time', 'as discussed'), " +
		"(2) before recommending architecture/patterns (check for prior decisions), " +
		"(3) user asks about their preferences or project conventions."
}
func (t *searchMemoryTool) UseSpinner() bool { return true }
func (t *searchMemoryTool) Params() []Param {
	return []Param{{"query", "string", "Search query. Supports multiple space-separated keywords in both English and Chinese."}}
}
func (t *searchMemoryTool) Preview(args map[string]any) string {
	s, _ := argStr(args, "query")
	return truncate(s, t.PreviewWidth())
}
func (t *searchMemoryTool) Run(args map[string]any) Result {
	q, _ := argStr(args, "query")
	return OK(t.mem.Search(q, 10))
}

// appendMemoryTool 追加一条长期记忆。
type appendMemoryTool struct {
	Base
	mem *memory.Manager
}

func (t *appendMemoryTool) Name() string { return "append_memory" }
func (t *appendMemoryTool) Description() string {
	return "Save a note to YOUR long-term memory. Persists across sessions. CALL THIS WHEN: " +
		"(1) user states a preference ('I always use X'), " +
		"(2) an architecture decision is made, (3) a non-obvious bug fix is found, " +
		"(4) a project convention is established. " +
		"DO NOT CALL for ephemeral session context, code already in the repo, or trivial facts."
}
func (t *appendMemoryTool) Params() []Param {
	return []Param{{"content", "string", "Concise 5-10 sentence note. Prefix tag: [PREFERENCE]/[DECISION]/[BUG-FIX]/[CONVENTION]."}}
}
func (t *appendMemoryTool) Run(args map[string]any) Result {
	content, _ := argStr(args, "content")
	if err := t.mem.Append(content); err != nil {
		return Fail("append_memory error: " + err.Error())
	}
	return OK("memory appended")
}

// viewImageTool 将本地图片载入视觉上下文。
type viewImageTool struct {
	Base
	c *ctx
}

const maxImageBytes = 5 * 1024 * 1024

func (t *viewImageTool) Name() string { return "view_image" }
func (t *viewImageTool) Description() string {
	return "Load a local image (screenshot, UI mockup, error screen, diagram) into the model's vision context. " +
		"Accepts an absolute path to a file on disk; URLs are not supported. " +
		"Supported formats: png, jpg, jpeg, gif, webp."
}
func (t *viewImageTool) UseSpinner() bool  { return true }
func (t *viewImageTool) PreviewLines() int { return 0 }
func (t *viewImageTool) PreviewWidth() int { return 200 }
func (t *viewImageTool) Params() []Param {
	return []Param{{"path", "string", "Absolute path to a local image file (png/jpg/jpeg/gif/webp). URL inputs are rejected."}}
}
func (t *viewImageTool) Preview(args map[string]any) string {
	s, _ := argStr(args, "path")
	return truncate(s, t.PreviewWidth())
}
func (t *viewImageTool) Run(args map[string]any) Result {
	path, _ := argStr(args, "path")
	path = strings.TrimSpace(path)
	if path == "" {
		return Fail("view_image error: 'path' is required")
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return Fail("view_image error: URL inputs are not supported. Download the image to a local file first, then pass the file path.")
	}
	if e := validateFilePath(t.c.root, path); e != "" {
		return Fail("view_image error: " + e)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return Fail("view_image error: cannot stat file: " + err.Error())
	}
	if fi.Size() == 0 {
		return Fail("view_image error: image file is empty")
	}
	if fi.Size() > maxImageBytes {
		return Fail("view_image error: image too large")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if !imageExts[ext] {
		return Fail("view_image error: unsupported image format '" + ext + "' (supported: png,jpg,jpeg,gif,webp)")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Fail("view_image error: cannot read file: " + err.Error())
	}
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "image/png"
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	return OK(map[string]any{
		"type":      "image",
		"text":      "Image: " + path + " (" + itoa(int(fi.Size())) + " bytes," + mimeType + ")",
		"image_url": "data:" + mimeType + ";base64," + b64,
	})
}
