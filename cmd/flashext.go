package cmd

import (
	"errors"
	"flag"

	"github.com/mars/marspi-cli/internal/flashext"
	"github.com/mars/marspi-core/config"
	"github.com/mars/marspi-core/llm"
)

// RunFlashExt 启动 OpenAI 兼容代理服务器（thinking framework 注入）。
// 对齐 mangopi 的 flash-ext 子命令。
func RunFlashExt(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("flash-ext", flag.ContinueOnError)
	host := fs.String("host", "127.0.0.1", "server bind host")
	port := fs.Int("port", 8080, "server port")
	token := fs.String("token", "", "bearer token for client auth")
	mem := fs.Bool("memory", false, "enable auto memory write")
	search := fs.Bool("web-search", false, "enable web search augmentation")
	debug := fs.Bool("debug", false, "enable debug logging")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if cfg.APIKey == "" {
		return errors.New("MARS_KEY env var is required for Flash-ext mode")
	}

	provider := llm.NewProvider(cfg.Model, cfg.APIURL, cfg.APIKey)
	server := flashext.New(cfg, flashext.Options{
		Host:         *host,
		Port:         *port,
		Token:        *token,
		Provider:     provider,
		EnableMemory: *mem,
		EnableSearch: *search,
		Debug:        *debug,
	})
	return server.Start()
}
