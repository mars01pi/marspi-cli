// Package tool 定义 Agent 可调用的工具体系：Tool 接口、Registry 与内置工具。
// 对齐 mangopi-cli 的 ToolBase 与 TOOLS 注册表。
package tool

import (
	"context"

	"github.com/mars/marspi-cli/internal/config"
	"github.com/mars/marspi-cli/internal/memory"
	"github.com/mars/marspi-cli/internal/skill"
	"github.com/mars/marspi-cli/internal/ui"
)

// Result 是工具执行结果，对齐 ToolBase.ok/fail。
type Result struct {
	Success bool
	Content any // 通常是 string；view_image 返回 image 结构
	Extra   map[string]any
}

// OK 构造成功结果。
func OK(content any) Result { return Result{Success: true, Content: content} }

// Fail 构造失败结果。
func Fail(content any) Result { return Result{Success: false, Content: content} }

// Param 描述一个工具参数。类型以字符串表达，"?" 后缀表示可选。
type Param struct {
	Name string
	Type string // "string" | "number" | "boolean"，可加 "?" 后缀表示可选
	Desc string
}

// Tool 是所有工具的统一接口，对齐 Python ToolBase。
type Tool interface {
	Name() string
	Description() string
	Params() []Param
	Run(args map[string]any) Result

	// Before 在确认前执行副作用（如 diff 预览）。
	Before(args map[string]any)
	// Confirm 返回是否需要用户放行（true 表示继续）。
	Confirm(args map[string]any) bool
	// Preview 返回工具调用的单行预览文本。
	Preview(args map[string]any) string
	// UseSpinner 表示执行期间是否显示 spinner。
	UseSpinner() bool
	// PreviewLines/PreviewWidth 控制结果回显的截断。
	PreviewLines() int
	PreviewWidth() int
}

// Base 提供 Tool 接口的默认实现，内置工具通过嵌入复用。
type Base struct{}

func (Base) Before(map[string]any)       {}
func (Base) Confirm(map[string]any) bool { return true }
func (Base) UseSpinner() bool            { return false }
func (Base) PreviewLines() int           { return 20 }
func (Base) PreviewWidth() int           { return 100 }
func (Base) Preview(args map[string]any) string {
	for _, v := range args {
		return truncate(toStr(v), 100)
	}
	return ""
}

// Registry 保存工具集合并生成 OpenAI tools schema。
type Registry struct {
	console      *ui.Printer
	order        []string
	tools        map[string]Tool
	builtinOrder []string
	builtinTools map[string]Tool
	providers    []Provider
}

// NewRegistry 构建注册表并注册全部内置工具。
func NewRegistry(cfg *config.Config, console *ui.Printer, mem *memory.Manager, skills *skill.Manager) *Registry {
	r := &Registry{
		console:      console,
		tools:        map[string]Tool{},
		builtinTools: map[string]Tool{},
	}
	c := &ctx{root: cfg.ProjectRoot, console: console}
	image := &viewImageTool{c: c}
	// 注册顺序对齐 mangopi 的 TOOLS 列表
	r.Register(&readTool{c: c, image: image})
	r.Register(&writeTool{c: c})
	r.Register(&editTool{c: c})
	r.Register(&searchTool{c: c})
	r.Register(&grepTool{c: c})
	r.Register(&bashTool{c: c})
	r.Register(&useSkillTool{skills: skills})
	r.Register(&searchMemoryTool{mem: mem})
	r.Register(&appendMemoryTool{mem: mem})
	r.Register(&webSearchTool{})
	r.Register(image)
	r.Register(&attemptCompletionTool{})
	return r
}

// Register 追加一个工具，保持注册顺序（顺序影响 schema 顺序）。
func (r *Registry) Register(t Tool) {
	if _, ok := r.builtinTools[t.Name()]; !ok {
		r.builtinOrder = append(r.builtinOrder, t.Name())
	}
	r.builtinTools[t.Name()] = t
	r.rebuildIndex()
}

// AddProvider 增加一个工具提供方（MCP 等），并立即刷新一次索引。
func (r *Registry) AddProvider(p Provider) error {
	if err := p.Refresh(context.Background()); err != nil {
		return err
	}
	r.providers = append(r.providers, p)
	r.rebuildIndex()
	return nil
}

// RefreshProviders 刷新所有 provider 的工具定义并重建索引。
func (r *Registry) RefreshProviders(ctx context.Context) error {
	for _, p := range r.providers {
		if err := p.Refresh(ctx); err != nil {
			return err
		}
	}
	r.rebuildIndex()
	return nil
}

// Close 关闭所有 provider 资源（例如 MCP 子进程连接）。
func (r *Registry) Close() error {
	var firstErr error
	for _, p := range r.providers {
		if err := p.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Get 按名称取工具。
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Names 返回注册顺序下的工具名。
func (r *Registry) Names() []string { return append([]string(nil), r.order...) }

// Schemas 生成 OpenAI function-calling 的 tools 数组。
func (r *Registry) Schemas() []map[string]any {
	out := make([]map[string]any, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, schemaOf(r.tools[name]))
	}
	return out
}

func (r *Registry) rebuildIndex() {
	order := make([]string, 0, len(r.builtinOrder))
	tools := make(map[string]Tool, len(r.builtinTools))
	for _, name := range r.builtinOrder {
		t := r.builtinTools[name]
		order = append(order, name)
		tools[name] = t
	}
	for _, p := range r.providers {
		for _, t := range p.Tools() {
			name := t.Name()
			if _, exists := tools[name]; exists {
				continue
			}
			order = append(order, name)
			tools[name] = t
		}
	}
	r.order = order
	r.tools = tools
}

// schemaOf 依据 Tool.Params 生成单个 function schema，对齐 ToolBase.schema。
func schemaOf(t Tool) map[string]any {
	properties := map[string]any{}
	required := []string{}
	for _, p := range t.Params() {
		optional := len(p.Type) > 0 && p.Type[len(p.Type)-1] == '?'
		base := p.Type
		if optional {
			base = base[:len(base)-1]
		}
		jsonType := base
		if base == "number" {
			jsonType = "integer"
		}
		properties[p.Name] = map[string]any{"type": jsonType, "description": p.Desc}
		if !optional {
			required = append(required, p.Name)
		}
	}
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        t.Name(),
			"description": t.Description(),
			"parameters": map[string]any{
				"type":       "object",
				"properties": properties,
				"required":   required,
			},
		},
	}
}
