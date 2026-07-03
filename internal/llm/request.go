package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mars/marspi-cli/internal/ui"
)

// Request 发起带指数退避重试的 POST 请求，对齐 mangopi 的 _request。
// 5xx 与 429 重试；其他 4xx 直接返回错误。
func Request(url string, body map[string]any, headers map[string]string, timeout time.Duration, maxRetries int) (map[string]any, error) {
	if headers == nil {
		headers = map[string]string{"Content-Type": "application/json"}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: timeout}
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
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
			time.Sleep(delay)
		}
	}
	if lastErr == nil {
		lastErr = errors.New("request failed")
	}
	return nil, lastErr
}
