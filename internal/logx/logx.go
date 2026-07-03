// Package logx 提供可选的调试日志，通过 MARS_DEBUG=1 开启。
package logx

import (
	"log"
	"os"
)

var debug = os.Getenv("MARS_DEBUG") != ""

func init() {
	log.SetPrefix("[marspi] ")
	log.SetFlags(log.Ltime)
}

// Enabled 是否开启调试日志。
func Enabled() bool { return debug }

// Debugf 写调试日志到 stderr。
func Debugf(format string, args ...any) {
	if debug {
		log.Printf(format, args...)
	}
}
