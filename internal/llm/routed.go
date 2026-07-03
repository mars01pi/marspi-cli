package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mars/marspi-cli/internal/flash"
	"github.com/mars/marspi-cli/internal/ui"
)

// RoutedProvider 根据任务复杂度评分，委托到 low/medium/high 分层 provider。
// 对齐 mangopi 的 RoutedProvider。
type RoutedProvider struct {
	tiers       map[string][]Provider
	current     Provider
	thresholds  map[string]int
	defaultTier string
}

type providerCfg struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Model  string `json:"model"`
	Tier   string `json:"tier"`
	APIKey string `json:"api_key"`
}

type routedFileCfg struct {
	Providers []providerCfg `json:"providers"`
	Routing   struct {
		DefaultTier     string         `json:"default_tier"`
		ScoreThresholds map[string]int `json:"score_thresholds"`
	} `json:"routing"`
}

// NewRoutedProviderFromFile 从 providers.json 构建 RoutedProvider。
func NewRoutedProviderFromFile(path string) (*RoutedProvider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg routedFileCfg
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return NewRoutedProvider(cfg)
}

// NewRoutedProviderFromList 用一组内联 provider 配置构建（用于兜底）。
func NewRoutedProviderFromList(providers []providerCfg, defaultTier string, thresholds map[string]int) (*RoutedProvider, error) {
	var cfg routedFileCfg
	cfg.Providers = providers
	cfg.Routing.DefaultTier = defaultTier
	cfg.Routing.ScoreThresholds = thresholds
	return NewRoutedProvider(cfg)
}

// NewRoutedProvider 从解析后的配置构建。
func NewRoutedProvider(cfg routedFileCfg) (*RoutedProvider, error) {
	r := &RoutedProvider{
		tiers:      map[string][]Provider{"low": {}, "medium": {}, "high": {}},
		thresholds: map[string]int{"low_max": 3, "medium_max": 7},
	}
	for _, p := range cfg.Providers {
		if _, ok := r.tiers[p.Tier]; !ok {
			return nil, fmt.Errorf("invalid provider tier '%s'. Must be low/medium/high", p.Tier)
		}
		r.tiers[p.Tier] = append(r.tiers[p.Tier], NewProvider(p.Model, p.URL, p.APIKey))
	}
	if len(r.tiers["low"])+len(r.tiers["medium"])+len(r.tiers["high"]) == 0 {
		return nil, fmt.Errorf("no providers defined in config")
	}
	for k, v := range cfg.Routing.ScoreThresholds {
		r.thresholds[k] = v
	}
	dt := cfg.Routing.DefaultTier
	if dt == "" {
		dt = "medium"
	}
	if len(r.tiers[dt]) == 0 {
		for _, t := range []string{"medium", "low", "high"} {
			if len(r.tiers[t]) > 0 {
				dt = t
				break
			}
		}
	}
	r.defaultTier = dt
	def := r.tiers[dt]
	if len(def) == 0 {
		for _, v := range r.tiers {
			if len(v) > 0 {
				def = v
				break
			}
		}
	}
	r.current = def[0]
	return r, nil
}

// —— Provider 接口：委托到 current ——

func (r *RoutedProvider) APIURL() string            { return r.current.APIURL() }
func (r *RoutedProvider) APIKey() string            { return r.current.APIKey() }
func (r *RoutedProvider) Model() string             { return r.current.Model() }
func (r *RoutedProvider) Headers() map[string]string { return r.current.Headers() }
func (r *RoutedProvider) BuildBody(m []Message, t []map[string]any) map[string]any {
	return r.current.BuildBody(m, t)
}
func (r *RoutedProvider) ParseResponse(resp map[string]any) Response { return r.current.ParseResponse(resp) }

// TotalProviders 返回 provider 总数。
func (r *RoutedProvider) TotalProviders() int {
	return len(r.tiers["low"]) + len(r.tiers["medium"]) + len(r.tiers["high"])
}

var angerWords = []string{
	"fuck", "fuxx", "f**k", "shit", "damn", "asshole", "bastard", "傻子", "笨蛋", "蠢货", "白痴", "脑残", "sb", "废物",
	"垃圾", "特么", "卧槽", "我操", "cnm", "tmd", "傻x",
}

var frameworkScore = map[string]int{
	"design": 9, "reevaluate": 8,
	"implement": 5, "optimize": 5,
	"debug": 3, "investigate": 3, "verify": 3, "explain": 1,
}

func keywordScore(query string) int {
	q := strings.ToLower(query)
	for _, kw := range angerWords {
		if strings.Contains(q, kw) {
			return 10
		}
	}
	fw := flash.Match(query, nil)
	if s, ok := frameworkScore[fw]; ok {
		return s
	}
	return 4
}

const scoringPrompt = `Rate this coding task complexity from 1-10 (1=trivial, 10=architectural/system design).
Consider: scope of changes, reasoning depth, debugging difficulty, components involved.

Tool call history (each segment = one user turn):
%s

Current request:
%s

Rubric: 1-3=read/search, 4-6=multi-file/edit/debug, 7-10=design/refactor/complex

Respond with ONLY a single integer.`

var digitRe = regexp.MustCompile(`\d+`)

func llmScore(userQuery, fingerprint string, high Provider) int {
	prompt := fmt.Sprintf(scoringPrompt, fingerprint, userQuery)
	body := high.BuildBody([]Message{{"role": "user", "content": prompt}}, nil)
	ui.Console.StartSpinner("Smart Routing...")
	raw, err := Request(high.APIURL(), body, high.Headers(), 15*time.Second, 0)
	ui.Console.EndSpinner()
	if err != nil {
		return 5
	}
	parsed := high.ParseResponse(raw)
	match := digitRe.FindString(strings.TrimSpace(parsed.Content))
	if match == "" {
		return 5
	}
	var val int
	fmt.Sscanf(match, "%d", &val)
	if val < 1 {
		val = 1
	}
	if val > 10 {
		val = 10
	}
	return val
}

// Route 依据任务复杂度评分切换到对应 tier，对齐 route。
// fingerprint 由调用方从上下文生成（ctx.ToolFingerprint）。
func (r *RoutedProvider) Route(userQuery, fingerprint string) {
	kw := keywordScore(userQuery)
	var tier string
	switch {
	case kw <= r.thresholds["low_max"]:
		tier = "low"
	case kw > r.thresholds["medium_max"]:
		tier = "high"
	default:
		if high := r.tiers["high"]; len(high) > 0 {
			ls := llmScore(userQuery, fingerprint, high[0])
			final := int(float64(kw)*0.3 + float64(ls)*0.7 + 0.5)
			switch {
			case final <= r.thresholds["low_max"]:
				tier = "low"
			case final <= r.thresholds["medium_max"]:
				tier = "medium"
			default:
				tier = "high"
			}
		} else {
			tier = r.defaultTier
		}
	}
	providers := r.tiers[tier]
	if len(providers) == 0 {
		providers = r.tiers[r.defaultTier]
	}
	r.current = providers[0]
	fmt.Printf("%s→ %s: %s%s\n", ui.Dim, tier, r.current.Model(), ui.Reset)
}
