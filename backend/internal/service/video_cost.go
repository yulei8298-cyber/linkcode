package service

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"go.uber.org/zap"
)

// videoPricingSpec 返回模型的视频计费规则（管理员覆盖 > 内置默认价）。
func (s *OpenAIGatewayService) videoPricingSpec(model string) (VideoPricingSpec, bool) {
	var pricing *PricingService
	if s != nil && s.billingService != nil {
		pricing = s.billingService.pricingService
	}
	if pricing != nil {
		spec, _, ok := pricing.GetVideoPricingSpec(model)
		return spec, ok
	}
	return LookupDefaultVideoPricingSpec(model)
}

// videoBillingDurationSeconds 归一化计费时长：有计费规则的模型按其时长区间收敛，
// 其余（Grok Imagine）沿用 1-15 秒、默认 8 秒的既有口径。
func (s *OpenAIGatewayService) videoBillingDurationSeconds(model string, durationSeconds int) int {
	if spec, ok := s.videoPricingSpec(model); ok {
		return spec.NormalizeUsage(VideoUsage{DurationSeconds: durationSeconds}).DurationSeconds
	}
	return NormalizeVideoBillingDurationSecondsOrDefault(durationSeconds)
}

// videoBillingResolution 归一化计费分辨率：有计费规则的模型支持 4k 档并以规则默认档兜底。
func (s *OpenAIGatewayService) videoBillingResolution(model, resolution string) string {
	if spec, ok := s.videoPricingSpec(model); ok {
		return spec.NormalizeUsage(VideoUsage{Resolution: resolution}).Resolution
	}
	return NormalizeVideoBillingResolutionOrDefault(resolution)
}

// calculateVideoSpecCost 按模型视频计费规则计算费用；没有规则或规则无法定价时返回 false。
func (s *OpenAIGatewayService) calculateVideoSpecCost(model string, usage VideoUsage, multiplier float64) (*CostBreakdown, bool) {
	spec, ok := s.videoPricingSpec(model)
	if !ok {
		return nil, false
	}
	base, err := spec.BaseCost(spec.NormalizeUsage(usage), s.videoTokenFallbackPrice(model))
	if err != nil {
		logger.L().Warn("video_pricing.spec_cost_failed", zap.String("model", model), zap.Error(err))
		return nil, false
	}
	if multiplier < 0 {
		multiplier = 0
	}
	return &CostBreakdown{TotalCost: base, ActualCost: base * multiplier, BillingMode: string(BillingModeVideo)}, true
}

// videoTokenFallbackPrice 是按 token 计费规则未填价格时使用的目录输出单价。
func (s *OpenAIGatewayService) videoTokenFallbackPrice(model string) float64 {
	if s == nil || s.billingService == nil || s.billingService.pricingService == nil {
		return 0
	}
	if p := s.billingService.pricingService.GetModelPricing(model); p != nil {
		return p.OutputCostPerToken
	}
	return 0
}

// channelVideoTier 把计费分辨率对齐到渠道视频价的档位，口径与模型计费规则一致：
// 精确档 → 渠道默认价 → 更高档 → 最高档。渠道既无可用档位也无默认价时返回 false，
// 交给后续计价来源，避免未配置的分辨率按 0 元结算。
func channelVideoTier(resolved *ResolvedPricing, resolution string) (string, bool) {
	labels := make(map[string]string, len(resolved.RequestTiers))
	for _, tier := range resolved.RequestTiers {
		if tier.PerRequestPrice != nil {
			labels[video.NormalizeResolution(tier.TierLabel)] = tier.TierLabel
		}
	}
	if label, ok := labels[resolution]; ok {
		return label, true
	}
	if resolved.DefaultPerRequestPrice > 0 {
		return "", true
	}
	order := []string{video.Resolution480P, video.Resolution720P, video.Resolution1080P, video.Resolution4K}
	start := 0
	for i, r := range order {
		if r == resolution {
			start = i
		}
	}
	for i := start; i < len(order); i++ {
		if label, ok := labels[order[i]]; ok {
			return label, true
		}
	}
	for i := len(order) - 1; i >= 0; i-- {
		if label, ok := labels[order[i]]; ok {
			return label, true
		}
	}
	return "", false
}
