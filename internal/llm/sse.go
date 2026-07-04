package llm

import (
	"bufio"
	"io"
	"strings"
)

// ReadSSE 从 SSE 响应体读取 data 行并回调（不含 "data: " 前缀）。
func ReadSSE(r io.Reader, onData func(data string) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if err := onData(data); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// ReadSSEStream 读取 SSE、解析并聚合；可选 onChunk 旁路回调。
func ReadSSEStream(r io.Reader, onChunk StreamHandler) (Response, error) {
	acc := NewStreamAccumulator()
	err := ReadSSE(r, func(data string) error {
		chunk, err := ParseStreamData(data)
		if err != nil {
			return err
		}
		acc.Apply(chunk)
		if onChunk != nil {
			if err := onChunk(chunk); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Response{}, err
	}
	return acc.BuildResponse(), nil
}
