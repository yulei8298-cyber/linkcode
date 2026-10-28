package service

import "github.com/Wei-Shaw/sub2api/internal/service/video"

// plazaVideoPricing 返回视频模型的展示计费规则，顺序与扣费（calculateOpenAIVideoCost）一致：
// 分组模型定价（视频模式）→ 分组按秒覆盖价 → 渠道视频价 → 模型定价（管理员覆盖 > 内置参考价）。
// 时长区间与默认分辨率始终沿用模型定价规则，渠道价只替换各档单价。
func (s *ModelPlazaService) plazaVideoPricing(model string, g *Group, channelPricing *ChannelModelPricing) *VideoPricingSpec {
	var spec VideoPricingSpec
	found := false
	if s.pricingService != nil {
		spec, _, found = s.pricingService.GetVideoPricingSpec(model)
	} else {
		spec, found = LookupDefaultVideoPricingSpec(model)
	}
	withRules := func(prices map[string]VideoTierPrice) *VideoPricingSpec {
		out := VideoPricingSpec{Unit: VideoBillingUnitPerSecond, Prices: prices}
		if found {
			out.DefaultResolution, out.DefaultDurationSeconds = spec.DefaultResolution, spec.DefaultDurationSeconds
			out.MinDurationSeconds, out.MaxDurationSeconds = spec.MinDurationSeconds, spec.MaxDurationSeconds
		}
		return &out
	}
	if prices := channelVideoDisplayPrices(matchGroupModelPricing(g, model)); len(prices) > 0 {
		return withRules(prices)
	}
	if g != nil {
		groupPrices := map[string]VideoTierPrice{}
		for _, tier := range []string{VideoBillingResolution480P, VideoBillingResolution720P, VideoBillingResolution1080P} {
			if price := g.GetVideoPriceForModel(model, tier); price != nil {
				p := *price
				groupPrices[tier] = VideoTierPrice{PerSecond: &p}
			}
		}
		if len(groupPrices) > 0 {
			return withRules(groupPrices)
		}
	}
	if prices := channelVideoDisplayPrices(channelPricing); len(prices) > 0 {
		return withRules(prices)
	}
	if !found {
		return nil
	}
	return &spec
}

// channelVideoDisplayPrices 把视频模式的渠道 / 分组定价转为按秒档位价；
// 渠道默认价（未命中档位时使用）展示为任意分辨率档。非视频模式返回 nil。
func channelVideoDisplayPrices(p *ChannelModelPricing) map[string]VideoTierPrice {
	if p == nil || p.BillingMode != BillingModeVideo {
		return nil
	}
	prices := map[string]VideoTierPrice{}
	for _, iv := range p.Intervals {
		tier := video.NormalizeResolution(iv.TierLabel)
		if iv.PerRequestPrice == nil || tier == "" {
			continue
		}
		v := *iv.PerRequestPrice
		prices[tier] = VideoTierPrice{PerSecond: &v}
	}
	if p.PerRequestPrice != nil {
		v := *p.PerRequestPrice
		prices[videoPriceWildcard] = VideoTierPrice{PerSecond: &v}
	}
	return prices
}
