// Package logx 提供可选的调试日志，通过 MARS_DEBUG=1 开启。
package logx

import (
	"fmt"
	"log"
	"os"
	"sync"
)

var (
	debug  = os.Getenv("MARS_DEBUG") != ""
	sinkMu sync.Mutex
	sink   func(string)
)

func init() {
	log.SetPrefix("[marspi] ")
	log.SetFlags(log.Ltime)
}

// Enabled 是否开启调试日志。
func Enabled() bool { return debug }

// SetSink 将调试日志转发到自定义接收器（如 TUI）；fn 为 nil 时恢复 stderr。
func SetSink(fn func(string)) {
	sinkMu.Lock()
	sink = fn
	sinkMu.Unlock()
}

// Debugf 写调试日志。TUI 模式下通过 SetSink 注入；否则输出到 stderr。
func Debugf(format string, args ...any) {
	if !debug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	sinkMu.Lock()
	fn := sink
	sinkMu.Unlock()
	if fn != nil {
		fn(msg)
		return
	}
	log.Print(msg)
}
