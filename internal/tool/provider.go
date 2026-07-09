package tool

import "context"

// Provider 抽象工具来源（内置、MCP、未来插件等）。
type Provider interface {
	ID() string
	Tools() []Tool
	Refresh(ctx context.Context) error
	Close() error
}
