package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mars/marspi-cli/internal/logx"
	"github.com/mars/marspi-cli/internal/ui"
)

// Request 发起带指数退避重试的 POST 请求，对齐 mangopi 的 _request。
func Request(url string, body map[string]any, headers map[string]string, timeout time.Duration, maxRetries int) (map[string]any, error) {
	return RequestContext(context.Background(), url, body, headers, timeout, maxRetries)
}

// RequestContext 与 Request 相同，但支持通过 ctx 取消进行中的 HTTP 请求。
func RequestContext(ctx context.Context, url string, body map[string]any, headers map[string]string, timeout time.Duration, maxRetries int) (map[string]any, error) {
	if headers == nil {
		headers = map[string]string{"Content-Type": "application/json"}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	logx.Debugf("POST %s (%d bytes)", url, len(payload))
	client := &http.Client{Timeout: timeout}
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			data, rerr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if rerr != nil {
				lastErr = rerr
			} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var out map[string]any
				if jerr := json.Unmarshal(data, &out); jerr != nil {
					lastErr = jerr
				} else {
					logx.Debugf("HTTP %d (%d bytes)", resp.StatusCode, len(data))
					if msg := apiErrorFromBody(out); msg != "" {
						return nil, fmt.Errorf("api error: %s", msg)
					}
					return out, nil
				}
			} else if resp.StatusCode >= 500 || resp.StatusCode == 429 {
				lastErr = fmt.Errorf("http %d: %s", resp.StatusCode, string(data))
			} else {
				return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(data))
			}
		}

		if attempt < maxRetries {
			delay := time.Duration(1<<attempt) * time.Second
			ui.Console.Warning(fmt.Sprintf("Request failed (attempt %d/%d), retrying in %.1fs",
				attempt+1, maxRetries+1, delay.Seconds()))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	if lastErr == nil {
		lastErr = errors.New("request failed")
	}
	return nil, lastErr
}
