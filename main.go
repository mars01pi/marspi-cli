// marspi-cli：终端 AI 编程助手。提供交互式 REPL、flash-ext 代理子命令与 -doctor 环境诊断。
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/mars/marspi-cli/cmd"
	"github.com/mars/marspi-cli/internal/config"
)

func main() {
	cfg := config.Load()

	versionFlag := flag.Bool("version", false, "print version and exit")
	doctorFlag := flag.Bool("doctor", false, "run environment diagnostics and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("marspi-cli v%s\n", config.Version)
		return
	}
	if *doctorFlag {
		os.Exit(cmd.Doctor(cfg))
	}

	// 子命令：flash-ext（OpenAI 兼容代理），其余进入交互 REPL
	if args := flag.Args(); len(args) > 0 && args[0] == "flash-ext" {
		if err := cmd.RunFlashExt(cfg, args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "flash-ext error:", err)
			os.Exit(1)
		}
		return
	}

	app := cmd.NewApp(cfg)
	if err := app.Run(); err != nil {
		if !errors.Is(err, cmd.ErrConfig) {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(1)
	}
}
