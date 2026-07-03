package flashext

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Start 启动 HTTP 服务并阻塞。
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", s.handleChat)
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	log.Printf("[flash-ext] serving on %s (memory=%v search=%v auth=%v)",
		addr, s.enableMemory, s.enableSearch, s.token != "")
	srv := &http.Server{Addr: addr, Handler: mux}
	return srv.ListenAndServe()
}

func writeJSON(w http.ResponseWriter, code int, data any) {
	b, _ := json.Marshal(data)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(b)
}

func errResp(msg string, code int) map[string]any {
	return map[string]any{"error": map[string]any{"message": msg, "type": "flash_ext_error", "code": code}}
}

func (s *Server) auth(w http.ResponseWriter, r *http.Request) bool {
	if s.token == "" {
		return true
	}
	if r.Header.Get("Authorization") != "Bearer "+s.token {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{"message": "Invalid token", "code": 401}})
		return false
	}
	return true
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if !s.auth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusOK, errResp("Not found", 404))
		return
	}
	t0 := time.Now()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusOK, errResp("read error", 500))
		return
	}
	var body map[string]any
	if json.Unmarshal(data, &body) != nil {
		writeJSON(w, http.StatusOK, errResp("Invalid JSON", 400))
		return
	}
	writeJSON(w, http.StatusOK, s.handle(body))
	s.debugf("POST /v1/chat/completions 200 %dms", time.Since(t0).Milliseconds())
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if !s.auth(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, s.models())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// bochaBrief 调用博查搜索取前 3 条，返回 "- title: summary" 文本，供增强注入。
func bochaBrief(query, key string) string {
	if query == "" || key == "" {
		return ""
	}
	payload, _ := json.Marshal(map[string]any{
		"query": query, "freshness": "noLimit", "summary": true, "count": 3,
	})
	req, err := http.NewRequest("POST", "https://api.bocha.cn/v1/web-search", bytes.NewReader(payload))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var parsed struct {
		Data struct {
			WebPages struct {
				Value []struct {
					Name    string `json:"name"`
					Summary string `json:"summary"`
				} `json:"value"`
			} `json:"webPages"`
		} `json:"data"`
	}
	if json.NewDecoder(resp.Body).Decode(&parsed) != nil {
		return ""
	}
	var lines []string
	for i, v := range parsed.Data.WebPages.Value {
		if i >= 3 {
			break
		}
		lines = append(lines, "- "+v.Name+": "+v.Summary)
	}
	return strings.Join(lines, "\n")
}
