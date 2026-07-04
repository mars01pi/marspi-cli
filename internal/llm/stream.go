package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mars/marspi-cli/internal/logx"
)

// StreamChunk 是单条 SSE data 行解析后的增量。
type StreamChunk struct {
	ContentDelta   string
	ReasoningDelta string
	ToolCallDeltas []ToolCallDelta
	FinishReason   string
	Usage          Usage
	HasUsage       bool
	Model          string
	Done           bool
}

// ToolCallDelta 是流式 tool_calls 的一个增量片段。
type ToolCallDelta struct {
	Index     int
	ID        string
	Type      string
	Name      string
	Arguments string
}

// StreamHandler 处理每个解析后的 chunk；返回 error 可中止读取。
type StreamHandler func(chunk StreamChunk) error

// StreamChat 发起 stream:true 的 chat completion，逐 chunk 回调并返回聚合响应。
func StreamChat(ctx context.Context, url string, body map[string]any, headers map[string]string, timeout time.Duration, onChunk StreamHandler) (Response, error) {
	if headers == nil {
		headers = map[string]string{"Content-Type": "application/json"}
	}
	bodyCopy := make(map[string]any, len(body)+1)
	for k, v := range body {
		bodyCopy[k] = v
	}
	bodyCopy["stream"] = true

	payload, err := json.Marshal(bodyCopy)
	if err != nil {
		return Response{}, err
	}
	logx.Debugf("POST stream %s (%d bytes)", url, len(payload))

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return Response{}, fmt.Errorf("http %d: %s", resp.StatusCode, string(data))
	}

	return ReadSSEStream(resp.Body, onChunk)
}
