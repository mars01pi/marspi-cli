package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mars/marspi-cli/internal/config"
	"github.com/mars/marspi-cli/internal/ui"
)

func nowStr() string { return time.Now().Format("2006-01-02 15:04:05") }

// Doctor 运行环境诊断，返回失败项数量，对齐 mangopi 的 doctor。
func Doctor(cfg *config.Config) int {
	console := ui.Console
	type check struct {
		ok  bool
		msg string
	}
	var results []check

	if cfg.APIKey != "" {
		results = append(results, check{true, "MARS_KEY is set"})
	} else {
		results = append(results, check{false, "MARS_KEY: not set (required)"})
	}
	if cfg.Routing == "on" {
		if _, err := os.Stat(cfg.ProvidersFile); err != nil {
			results = append(results, check{false, "providers.json not found (required when MARS_ROUTING=on)"})
		}
	}
	if fi, err := os.Stat(cfg.SessionDir); err != nil || !fi.IsDir() {
		results = append(results, check{false, "session directory not found"})
	} else {
		entries, _ := os.ReadDir(cfg.SessionDir)
		var files []string
		for _, e := range entries {
			n := e.Name()
			if strings.HasSuffix(n, ".json") && !strings.HasSuffix(n, ".backup") {
				files = append(files, n)
			}
		}
		results = append(results, check{true, "session: " + itoa(len(files)) + " file(s)"})
		for _, name := range files {
			data, err := os.ReadFile(filepath.Join(cfg.SessionDir, name))
			if err != nil {
				results = append(results, check{false, "  " + name + ": corrupted — " + err.Error()})
				continue
			}
			var msgs []map[string]any
			if json.Unmarshal(data, &msgs) != nil {
				results = append(results, check{false, "  " + name + ": invalid schema (expected list)"})
				continue
			}
			bad := 0
			for _, m := range msgs {
				if _, ok := m["role"]; !ok {
					bad++
				}
			}
			if bad == 0 {
				results = append(results, check{true, "  " + name + ": " + itoa(len(msgs)) + " message(s), valid"})
			} else {
				results = append(results, check{false, "  " + name + ": " + itoa(bad) + " malformed"})
			}
		}
	}

	fails := 0
	for _, r := range results {
		if r.ok {
			console.Success(r.msg)
		} else {
			console.Error(r.msg)
			fails++
		}
	}
	return fails
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
