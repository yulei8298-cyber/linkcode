package service

import "github.com/Wei-Shaw/sub2api/internal/service/video"

// defaultVideoPricingSpecs 是内置的视频模型参考价（USD），按模型名前缀匹配。
//
// 数值取自各厂商公开价目与 2026 年第三方价格汇总，仅作开箱默认值：上线前应在
// 「模型定价」中按实际上游成本覆盖。未列出的模型必须由管理员配置价格，否则网关
// 拒绝创建任务，避免出现未计价的视频生成。Grok Imagine 沿用其独立的默认价逻辑。
var defaultVideoPricingSpecs = map[string]VideoPricingSpec{
	// 通义万相 Wan3.0（含 Prime）：官方 0.3 / 0.6 / 1.2 元每秒，2–30 秒，默认 1080P。
	"wan3.0-video": {
		Unit:                   VideoBillingUnitPerSecond,
		Prices:                 perSecondTiers(map[string]float64{video.Resolution480P: 0.0425, video.Resolution720P: 0.085, video.Resolution1080P: 0.17}),
		DefaultResolution:      video.Resolution1080P,
		DefaultDurationSeconds: 5,
		MinDurationSeconds:     2,
		MaxDurationSeconds:     30,
	},
	// 字节 Seedance 2.0：方舟按 token 结算，这里给出折算后的按秒参考价。
	"doubao-seedance-2-0": {
		Unit:                   VideoBillingUnitPerSecond,
		Prices:                 perSecondTiers(map[string]float64{video.Resolution480P: 0.08, video.Resolution720P: 0.15, video.Resolution1080P: 0.30}),
		DefaultResolution:      video.Resolution720P,
		DefaultDurationSeconds: 5,
		MinDurationSeconds:     4,
		MaxDurationSeconds:     15,
	},
	// 可灵：std（720p）与 pro（1080p）按秒。
	"kling-": {
		Unit:                   VideoBillingUnitPerSecond,
		Prices:                 perSecondTiers(map[string]float64{video.Resolution720P: 0.084, video.Resolution1080P: 0.112}),
		DefaultResolution:      video.Resolution720P,
		DefaultDurationSeconds: 5,
		MinDurationSeconds:     5,
		MaxDurationSeconds:     15,
	},
	// 海螺 2.3 / 02：按条计费，按分辨率与 6 秒 / 10 秒分档。
	"minimax-hailuo-": {
		Unit: VideoBillingUnitPerVideo,
		Prices: map[string]VideoTierPrice{
			video.Resolution480P:  {PerVideo: map[string]float64{"6": 0.10, "10": 0.15}},
			video.Resolution720P:  {PerVideo: map[string]float64{"6": 0.28, "10": 0.56}},
			video.Resolution1080P: {PerVideo: map[string]float64{"6": 0.49}},
		},
		DefaultResolution:      video.Resolution720P,
		DefaultDurationSeconds: 6,
		MinDurationSeconds:     6,
		MaxDurationSeconds:     10,
	},
	// Google Veo 3.1 标准 / Fast / Lite。
	"veo-3.1-generate": {
		Unit:                   VideoBillingUnitPerSecond,
		Prices:                 perSecondTiers(map[string]float64{video.Resolution720P: 0.40, video.Resolution1080P: 0.40, video.Resolution4K: 0.60}),
		DefaultResolution:      video.Resolution720P,
		DefaultDurationSeconds: 8,
		MinDurationSeconds:     4,
		MaxDurationSeconds:     8,
	},
	"veo-3.1-fast": {
		Unit:                   VideoBillingUnitPerSecond,
		Prices:                 perSecondTiers(map[string]float64{video.Resolution720P: 0.10, video.Resolution1080P: 0.12, video.Resolution4K: 0.30}),
		DefaultResolution:      video.Resolution720P,
		DefaultDurationSeconds: 8,
		MinDurationSeconds:     4,
		MaxDurationSeconds:     8,
	},
	"veo-3.1-lite": {
		Unit:                   VideoBillingUnitPerSecond,
		Prices:                 perSecondTiers(map[string]float64{video.Resolution720P: 0.05, video.Resolution1080P: 0.08}),
		DefaultResolution:      video.Resolution720P,
		DefaultDurationSeconds: 8,
		MinDurationSeconds:     4,
		MaxDurationSeconds:     8,
	},
	// Sora 2 系列（经 OpenAI 兼容中转时使用）。
	"sora-2-pro": {
		Unit:                   VideoBillingUnitPerSecond,
		Prices:                 perSecondTiers(map[string]float64{video.Resolution720P: 0.30, video.Resolution1080P: 0.50}),
		DefaultResolution:      video.Resolution720P,
		DefaultDurationSeconds: 4,
		MaxDurationSeconds:     25,
	},
	"sora-2": {
		Unit:                   VideoBillingUnitPerSecond,
		Prices:                 perSecondTiers(map[string]float64{video.Resolution720P: 0.10}),
		DefaultResolution:      video.Resolution720P,
		DefaultDurationSeconds: 4,
		MaxDurationSeconds:     25,
	},
	// Runway Gen-4 Turbo：5 credits/秒，1 credit = $0.01。
	"gen4_turbo": {
		Unit:                   VideoBillingUnitPerSecond,
		Prices:                 perSecondTiers(map[string]float64{videoPriceWildcard: 0.05}),
		DefaultResolution:      video.Resolution720P,
		DefaultDurationSeconds: 5,
		MinDurationSeconds:     5,
		MaxDurationSeconds:     10,
	},
}

func perSecondTiers(prices map[string]float64) map[string]VideoTierPrice {
	out := make(map[string]VideoTierPrice, len(prices))
	for tier, price := range prices {
		p := price
		out[tier] = VideoTierPrice{PerSecond: &p}
	}
	return out
}
