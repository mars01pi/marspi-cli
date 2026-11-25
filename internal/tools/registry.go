package tools

import (
	"github.com/mars/marspi-core/console"
	"github.com/mars/marspi-core/memory"
	"github.com/mars/marspi-core/rag"
	"github.com/mars/marspi-core/skill"
	coretool "github.com/mars/marspi-core/tool"
)

type Options struct {
	Root      string
	Host      Host
	Memory    *memory.Manager
	Skills    *skill.Manager
	SearchKey string
	Retriever rag.Retriever
	RAGTopK   int
}

func NewRegistry(options Options) *coretool.Registry {
	registry := coretool.NewEmptyRegistry(options.Host)
	Register(registry, options)
	return registry
}

func Register(registry *coretool.Registry, options Options) {
	host := options.Host
	if host == nil {
		host = console.Nop{}
	}
	scope := &ctx{root: options.Root, console: host}
	image := &viewImageTool{c: scope}

	registry.Register(&readTool{c: scope, image: image})
	registry.Register(&writeTool{c: scope})
	registry.Register(&editTool{c: scope})
	registry.Register(&searchTool{c: scope})
	registry.Register(&grepTool{c: scope})
	registry.Register(&bashTool{c: scope})
	registry.Register(&useSkillTool{skills: options.Skills})
	registry.Register(&searchMemoryTool{mem: options.Memory})
	registry.Register(&appendMemoryTool{mem: options.Memory})
	registry.Register(&webSearchTool{apiKey: options.SearchKey})
	registry.Register(image)
	registry.Register(NewAttemptCompletionTool())

	if options.Retriever != nil {
		registry.Register(NewKnowledgeTool(options.Retriever, options.RAGTopK))
	}
}
