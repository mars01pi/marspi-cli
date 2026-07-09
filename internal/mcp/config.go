package mcp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config 对齐 Cursor mcp.json 顶层结构。
type Config struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

// ServerConfig 描述一个 MCP server（stdio 或远程 URL）。
type ServerConfig struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	Trust string `json:"trust,omitempty"` // prompt|always|never（预留）
}

// LoadMergedConfig 读取并合并 mcp 配置（低优先级 -> 高优先级）。
// 当前顺序：~/.marspicli/mcp.json < .cursor/mcp.json < .marspicli/mcp.json
func LoadMergedConfig(projectRoot string) (Config, error) {
	cfg := Config{MCPServers: map[string]ServerConfig{}}
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}
	paths := []string{
		filepath.Join(home, ".marspicli", "mcp.json"),
		filepath.Join(projectRoot, ".cursor", "mcp.json"),
		filepath.Join(projectRoot, ".marspicli", "mcp.json"),
	}
	var loadedAny bool
	for _, p := range paths {
		part, ok, err := loadSingleConfig(p)
		if err != nil {
			return cfg, err
		}
		if !ok {
			continue
		}
		loadedAny = true
		for name, server := range part.MCPServers {
			cfg.MCPServers[name] = server
		}
	}
	if !loadedAny {
		return cfg, nil
	}
	return cfg, nil
}

func loadSingleConfig(path string) (Config, bool, error) {
	out := Config{MCPServers: map[string]ServerConfig{}}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return out, false, nil
		}
		return out, false, err
	}
	if len(b) == 0 {
		return out, true, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, false, err
	}
	if out.MCPServers == nil {
		out.MCPServers = map[string]ServerConfig{}
	}
	return out, true, nil
}
