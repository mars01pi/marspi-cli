// Package config 承载运行时配置：环境变量、持久化路径与版本信息。
// 通过 MARS_* 环境变量加载运行时配置。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	Version = "0.1.0"
	Author  = "mars"
	License = "Apache License 2.0"
)

// Config 保存一次运行所需的全部静态配置。
type Config struct {
	APIKey     string // MARS_KEY
	APIURL     string // MARS_API_URL
	Model      string // MARS_MODEL
	MaxContext int    // MARS_MAX_CONTEXT，单位 token
	MaxIter    int    // MARS_MAX_ITER，单轮 agent_loop 的最大迭代
	Lang       string // MARS_LANG: en | zh
	Routing    string // MARS_ROUTING: on | off
	Stream     bool   // MARS_STREAM: 1/on 启用 SSE 流式（默认开）
	SearchKey  string // MARS_SEARCH_API_KEY，web_search 用

	ProjectRoot   string // 当前工作目录
	BasePersist   string // <root>/.marspicli
	SessionDir    string // <root>/.marspicli/session
	MemoryDir     string // <root>/.marspicli/memory
	LoopsDir      string // <root>/.marspicli/loops
	ProvidersFile string // <root>/.marspicli/providers.json
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

// Load 从环境变量与当前工作目录构建 Config。
func Load() *Config {
	root, err := os.Getwd()
	if err != nil {
		root = "."
	}
	base := filepath.Join(root, ".marspicli")
	c := &Config{
		APIKey:     os.Getenv("MARS_KEY"),
		APIURL:     env("MARS_API_URL", "https://api.deepseek.com"),
		Model:      env("MARS_MODEL", "deepseek-v4-flash"),
		MaxContext: envInt("MARS_MAX_CONTEXT", 1_000_000),
		MaxIter:    envInt("MARS_MAX_ITER", 100),
		Lang:       normLang(env("MARS_LANG", "en")),
		Routing:    lower(env("MARS_ROUTING", "off")),
		Stream:     envBool("MARS_STREAM", true),
		SearchKey:  os.Getenv("MARS_SEARCH_API_KEY"),

		ProjectRoot:   root,
		BasePersist:   base,
		SessionDir:    filepath.Join(base, "session"),
		MemoryDir:     filepath.Join(base, "memory"),
		LoopsDir:      filepath.Join(base, "loops"),
		ProvidersFile: filepath.Join(base, "providers.json"),
	}
	return c
}

// ProviderReady 检查 LLM 调用所需的凭据是否就绪。
func (c *Config) ProviderReady() (bool, string) {
	if c.Routing == "on" {
		if _, err := os.Stat(c.ProvidersFile); err != nil {
			return false, fmt.Sprintf(
				"MARS_ROUTING=on but %s not found\n  cp providers.json.example %s",
				c.ProvidersFile, c.ProvidersFile,
			)
		}
		return true, ""
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return false, "MARS_KEY is not set (required)\n  export MARS_KEY=sk-your-key"
	}
	return true, ""
}

// Initialize 创建持久化所需的目录结构，等价于 mangopi 的 initialize_system。
func (c *Config) Initialize() error {
	for _, d := range []string{c.BasePersist, c.SessionDir, c.MemoryDir, c.LoopsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func lower(s string) string {
	b := []byte(s)
	for i, ch := range b {
		if ch >= 'A' && ch <= 'Z' {
			b[i] = ch + 32
		}
	}
	return string(b)
}

func normLang(s string) string {
	s = lower(s)
	if s == "zh" {
		return "zh"
	}
	return "en"
}
