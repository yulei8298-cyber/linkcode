package service

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
)

// 视频计费单位。
const (
	VideoBillingUnitPerSecond = "per_second"
	VideoBillingUnitPerVideo  = "per_video"
	VideoBillingUnitToken     = "token"
)

// videoPriceWildcard 表示任意分辨率 / 任意时长。
const videoPriceWildcard = "*"

// VideoTierPrice 是某个分辨率档的价格（USD）。
type VideoTierPrice struct {
	PerSecond          *float64 `json:"per_second,omitempty"`
	PerSecondWithAudio *float64 `json:"per_second_with_audio,omitempty"`
	// PerVideo 是按次计费的单条价格：键为时长秒数（"5"、"10"），"*" 表示任意时长。
	PerVideo map[string]float64 `json:"per_video,omitempty"`
}

// VideoPricingSpec 是一个视频模型的完整计费规则，模型定价覆盖与内置默认价共用。
type VideoPricingSpec struct {
	Unit string `json:"unit"`
	// Prices 的键为分辨率档（480p / 720p / 1080p / 4k），"*" 为任意档。
	Prices map[string]VideoTierPrice `json:"prices,omitempty"`
	// TokenPricePerMillion 是按 token 计费时每百万输出 token 的价格。
	TokenPricePerMillion   *float64 `json:"token_price_per_million,omitempty"`
	DefaultResolution      string   `json:"default_resolution,omitempty"`
	DefaultDurationSeconds int      `json:"default_duration_seconds,omitempty"`
	MinDurationSeconds     int      `json:"min_duration_seconds,omitempty"`
	MaxDurationSeconds     int      `json:"max_duration_seconds,omitempty"`
}

// VideoUsage 是一次视频生成的计费用量。
type VideoUsage struct {
	Resolution       string
	DurationSeconds  int
	Count            int
	WithAudio        bool
	CompletionTokens int64
}

func validPrice(p *float64) bool {
	return p == nil || (!math.IsNaN(*p) && !math.IsInf(*p, 0) && *p >= 0)
}

// Validate 校验规则完整且价格合法。
func (s VideoPricingSpec) Validate() error {
	switch s.Unit {
	case VideoBillingUnitPerSecond, VideoBillingUnitPerVideo:
		if len(s.Prices) == 0 {
			return fmt.Errorf("video pricing requires at least one resolution price")
		}
	case VideoBillingUnitToken:
		if s.TokenPricePerMillion == nil {
			return fmt.Errorf("token video pricing requires token_price_per_million")
		}
	default:
		return fmt.Errorf("unsupported video billing unit %q", s.Unit)
	}
	if !validPrice(s.TokenPricePerMillion) {
		return fmt.Errorf("prices must be finite and non-negative")
	}
	for tier, p := range s.Prices {
		if tier != videoPriceWildcard && video.NormalizeResolution(tier) != tier {
			return fmt.Errorf("unknown resolution tier %q", tier)
		}
		if !validPrice(p.PerSecond) || !validPrice(p.PerSecondWithAudio) {
			return fmt.Errorf("prices must be finite and non-negative")
		}
		if s.Unit == VideoBillingUnitPerSecond && p.PerSecond == nil {
			return fmt.Errorf("tier %s is missing per_second price", tier)
		}
		if s.Unit == VideoBillingUnitPerVideo && len(p.PerVideo) == 0 {
			return fmt.Errorf("tier %s is missing per_video prices", tier)
		}
		for k, v := range p.PerVideo {
			if _, err := strconv.Atoi(k); err != nil && k != videoPriceWildcard {
				return fmt.Errorf("per_video key %q must be seconds or *", k)
			}
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return fmt.Errorf("prices must be finite and non-negative")
			}
		}
	}
	if s.MinDurationSeconds < 0 || s.MaxDurationSeconds < 0 || (s.MaxDurationSeconds > 0 && s.MinDurationSeconds > s.MaxDurationSeconds) {
		return fmt.Errorf("invalid duration bounds")
	}
	return nil
}

// NormalizeUsage 补全默认分辨率 / 时长并把时长收敛到模型允许区间。
func (s VideoPricingSpec) NormalizeUsage(u VideoUsage) VideoUsage {
	if u.Resolution = video.NormalizeResolution(u.Resolution); u.Resolution == "" {
		u.Resolution = video.FirstNonEmpty(s.DefaultResolution, video.Resolution720P)
	}
	if u.DurationSeconds <= 0 {
		u.DurationSeconds = s.DefaultDurationSeconds
		if u.DurationSeconds <= 0 {
			u.DurationSeconds = 5
		}
	}
	if s.MinDurationSeconds > 0 && u.DurationSeconds < s.MinDurationSeconds {
		u.DurationSeconds = s.MinDurationSeconds
	}
	if s.MaxDurationSeconds > 0 && u.DurationSeconds > s.MaxDurationSeconds {
		u.DurationSeconds = s.MaxDurationSeconds
	}
	if u.Count <= 0 {
		u.Count = 1
	}
	return u
}

// tier 返回分辨率档价格：精确档优先，其次通配档，再次最接近的更高档、最高档。
func (s VideoPricingSpec) tier(resolution string) (VideoTierPrice, bool) {
	if p, ok := s.Prices[resolution]; ok {
		return p, true
	}
	if p, ok := s.Prices[videoPriceWildcard]; ok {
		return p, true
	}
	order := []string{video.Resolution480P, video.Resolution720P, video.Resolution1080P, video.Resolution4K}
	idx := 0
	for i, r := range order {
		if r == resolution {
			idx = i
		}
	}
	for i := idx; i < len(order); i++ {
		if p, ok := s.Prices[order[i]]; ok {
			return p, true
		}
	}
	for i := len(order) - 1; i >= 0; i-- {
		if p, ok := s.Prices[order[i]]; ok {
			return p, true
		}
	}
	return VideoTierPrice{}, false
}

// perVideoPrice 选择不小于实际时长的最短时长档；都更短时取最长档。
func perVideoPrice(prices map[string]float64, duration int) (float64, bool) {
	if p, ok := prices[strconv.Itoa(duration)]; ok {
		return p, true
	}
	keys := make([]int, 0, len(prices))
	for k := range prices {
		if n, err := strconv.Atoi(k); err == nil {
			keys = append(keys, n)
		}
	}
	sort.Ints(keys)
	for _, k := range keys {
		if k >= duration {
			return prices[strconv.Itoa(k)], true
		}
	}
	if p, ok := prices[videoPriceWildcard]; ok {
		return p, true
	}
	if len(keys) > 0 {
		return prices[strconv.Itoa(keys[len(keys)-1])], true
	}
	return 0, false
}

// BaseCost 计算未乘倍率的费用；usage 需先经 NormalizeUsage。
func (s VideoPricingSpec) BaseCost(u VideoUsage, fallbackTokenPricePerToken float64) (float64, error) {
	switch s.Unit {
	case VideoBillingUnitToken:
		perToken := fallbackTokenPricePerToken
		if s.TokenPricePerMillion != nil {
			perToken = *s.TokenPricePerMillion / 1e6
		}
		return float64(u.CompletionTokens) * perToken, nil
	case VideoBillingUnitPerSecond:
		p, ok := s.tier(u.Resolution)
		if !ok || p.PerSecond == nil {
			return 0, fmt.Errorf("no per-second price for %s", u.Resolution)
		}
		unit := *p.PerSecond
		if u.WithAudio && p.PerSecondWithAudio != nil {
			unit = *p.PerSecondWithAudio
		}
		return unit * float64(u.DurationSeconds) * float64(u.Count), nil
	case VideoBillingUnitPerVideo:
		p, ok := s.tier(u.Resolution)
		if !ok {
			return 0, fmt.Errorf("no per-video price for %s", u.Resolution)
		}
		unit, ok := perVideoPrice(p.PerVideo, u.DurationSeconds)
		if !ok {
			return 0, fmt.Errorf("no per-video price for %ds", u.DurationSeconds)
		}
		return unit * float64(u.Count), nil
	}
	return 0, fmt.Errorf("unsupported video billing unit %q", s.Unit)
}

// LookupDefaultVideoPricingSpec 按模型名匹配内置默认价（最长前缀优先，忽略大小写）。
func LookupDefaultVideoPricingSpec(model string) (VideoPricingSpec, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return VideoPricingSpec{}, false
	}
	best, bestLen := VideoPricingSpec{}, 0
	for prefix, spec := range defaultVideoPricingSpecs {
		if strings.HasPrefix(model, prefix) && len(prefix) > bestLen {
			best, bestLen = spec, len(prefix)
		}
	}
	return best, bestLen > 0
}

// DefaultVideoPricingSpecs 返回内置默认价目录的副本（模型前缀 → 规则），供管理端展示。
func DefaultVideoPricingSpecs() map[string]VideoPricingSpec {
	out := make(map[string]VideoPricingSpec, len(defaultVideoPricingSpecs))
	for k, v := range defaultVideoPricingSpecs {
		out[k] = v
	}
	return out
}
