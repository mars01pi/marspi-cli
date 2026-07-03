package tool

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

var validFreshness = map[string]bool{
	"noLimit": true, "oneDay": true, "oneWeek": true, "oneMonth": true, "oneYear": true,
}

// webSearchTool 通过博查（Bocha）AI 搜索 API 检索实时网页。
type webSearchTool struct {
	Base
}

func (t *webSearchTool) Name() string { return "web_search" }
func (t *webSearchTool) Description() string {
	return "Search the live web via the Bocha (博查) AI Search API and return a list of results with " +
		"per-page AI summaries. Use this when the user asks for the latest docs, news, blog posts, " +
		"or any information that requires looking up something beyond the local filesystem. " +
		"Requires the MARS_SEARCH_API_KEY env var to be set; returns a clear error otherwise."
}
func (t *webSearchTool) UseSpinner() bool  { return true }
func (t *webSearchTool) PreviewLines() int { return 0 }
func (t *webSearchTool) PreviewWidth() int { return 200 }
func (t *webSearchTool) Params() []Param {
	return []Param{
		{"query", "string", "Natural-language search query, e.g. 'FastAPI vs Flask in 2026'."},
		{"top_k", "number?", "How many results to return (1-50, default 10)."},
		{"freshness", "string?", "Time filter: 'noLimit' (default), 'oneDay', 'oneWeek', 'oneMonth', 'oneYear'."},
	}
}
func (t *webSearchTool) Preview(args map[string]any) string {
	s, _ := argStr(args, "query")
	return truncate(s, t.PreviewWidth())
}

type bochaResult struct {
	date, title, link, summary, content string
}

func (t *webSearchTool) Run(args map[string]any) Result {
	query, _ := argStr(args, "query")
	query = strings.TrimSpace(query)
	if query == "" {
		return Fail("web_search error: 'query' is required")
	}
	apiKey := os.Getenv("MARS_SEARCH_API_KEY")
	if apiKey == "" {
		return Fail("web_search error: MARS_SEARCH_API_KEY env var is not set")
	}
	topK := 10
	if v, ok := argInt(args, "top_k"); ok {
		topK = v
	}
	if topK < 1 || topK > 50 {
		return Fail("web_search error: 'top_k' must be in [1, 50], got " + itoa(topK))
	}
	freshness := "noLimit"
	if f, ok := argStr(args, "freshness"); ok && strings.TrimSpace(f) != "" {
		freshness = strings.TrimSpace(f)
	}
	if !validFreshness[freshness] {
		return Fail("web_search error: 'freshness' must be one of noLimit/oneDay/oneWeek/oneMonth/oneYear, got " + freshness)
	}

	results, err := bochaSearch(query, freshness, topK, apiKey)
	if err != nil {
		return Fail("web_search error: Bocha API call failed: " + err.Error())
	}
	if len(results) == 0 {
		return OK("(no results for query: " + query + ")")
	}

	var lines []string
	lines = append(lines, "## Answer (Bocha · "+itoa(len(results))+" result(s) for: "+query+")", "")
	var sources []string
	for i, r := range results {
		n := itoa(i + 1)
		title := r.title
		if title == "" {
			title = "(untitled)"
		}
		if r.link != "" {
			lines = append(lines, "### "+n+". ["+title+"]("+r.link+")")
		} else {
			lines = append(lines, "### "+n+". "+title)
		}
		if r.date != "" {
			lines = append(lines, "*Date: "+r.date+"*")
		}
		lines = append(lines, "")
		if r.summary != "" {
			lines = append(lines, "> "+r.summary, "")
		}
		if r.content != "" && r.content != r.summary {
			snippet := r.content
			if len(snippet) > 500 {
				snippet = snippet[:500] + "..."
			}
			lines = append(lines, snippet, "")
		}
		if r.link != "" {
			sources = append(sources, n+". ["+title+"]("+r.link+")")
		} else {
			sources = append(sources, n+". "+title)
		}
	}
	lines = append(lines, "## Sources")
	lines = append(lines, sources...)
	return OK(strings.TrimRight(strings.Join(lines, "\n"), "\n"))
}

// bochaSearch 调用博查 web-search API。
func bochaSearch(query, freshness string, count int, key string) ([]bochaResult, error) {
	payload := map[string]any{
		"query": query, "freshness": freshness, "summary": true,
		"include": "", "exclude": "", "count": count,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.bocha.cn/v1/web-search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed struct {
		Data struct {
			WebPages struct {
				Value []struct {
					DateLastCrawled string `json:"dateLastCrawled"`
					Name            string `json:"name"`
					URL             string `json:"url"`
					Summary         string `json:"summary"`
					Content         string `json:"content"`
				} `json:"value"`
			} `json:"webPages"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	var out []bochaResult
	for _, v := range parsed.Data.WebPages.Value {
		out = append(out, bochaResult{
			date: v.DateLastCrawled, title: v.Name, link: v.URL,
			summary: v.Summary, content: v.Content,
		})
	}
	return out, nil
}
