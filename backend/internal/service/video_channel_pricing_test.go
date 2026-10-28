//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelVideoTierAlignsResolution(t *testing.T) {
	resolved := &ResolvedPricing{RequestTiers: []PricingInterval{
		{TierLabel: "720p", PerRequestPrice: fp(0.32)},
		{TierLabel: "1080P", PerRequestPrice: fp(0.416)},
		{TierLabel: "4K", PerRequestPrice: fp(0.96)},
	}}
	cases := map[string]string{"720p": "720p", "1080p": "1080P", "4k": "4K", "480p": "720p", "": "720p"}
	for in, want := range cases {
		got, ok := channelVideoTier(resolved, in)
		require.True(t, ok, in)
		require.Equal(t, want, got, "分辨率 %q 应对齐到渠道档 %q", in, want)
	}

	// 有默认价时，未命中的档位走默认价而不是借用其他档。
	resolved.DefaultPerRequestPrice = 0.5
	got, ok := channelVideoTier(resolved, "480p")
	require.True(t, ok)
	require.Empty(t, got)

	// 只有更低档时取最高档。
	got, ok = channelVideoTier(&ResolvedPricing{RequestTiers: []PricingInterval{{TierLabel: "720p", PerRequestPrice: fp(0.3)}}}, "4k")
	require.True(t, ok)
	require.Equal(t, "720p", got)

	_, ok = channelVideoTier(&ResolvedPricing{}, "720p")
	require.False(t, ok, "渠道既无档位也无默认价时不能按 0 元结算")
}

func TestCalculateOpenAIVideoCostUsesChannelTiers(t *testing.T) {
	groupID := int64(25)
	model := "kling-v3"
	resolver := newOpenAIImageChannelPricingResolverForTest(t, groupID, model, 0)
	cache := resolver.channelService.cache.Load().(*channelCache)
	pricing := cache.pricingByGroupModel[channelModelKey{groupID: groupID, model: model}]
	pricing.BillingMode = BillingModeVideo
	pricing.PerRequestPrice = nil
	pricing.Intervals = []PricingInterval{
		{TierLabel: "720p", PerRequestPrice: fp(0.32)},
		{TierLabel: "1080p", PerRequestPrice: fp(0.416)},
		{TierLabel: "4k", PerRequestPrice: fp(0.96)},
	}
	svc := &OpenAIGatewayService{billingService: resolver.billingService, resolver: resolver}
	apiKey := &APIKey{GroupID: &groupID, Group: &Group{ID: groupID, Hydrated: true}}
	cost := func(resolution string, seconds int) *CostBreakdown {
		return svc.calculateOpenAIVideoCost(context.Background(), model, apiKey, &OpenAIForwardResult{
			VideoCount: 1, VideoResolution: resolution, VideoDurationSeconds: seconds,
		}, 2)
	}

	c := cost("4k", 5)
	require.InDelta(t, 4.8, c.TotalCost, 1e-9, "4K 按渠道 4k 档 × 5 秒")
	require.InDelta(t, 9.6, c.ActualCost, 1e-9)
	require.Equal(t, string(BillingModeVideo), c.BillingMode)
	require.InDelta(t, 3.2, cost("720p", 10).TotalCost, 1e-9)
	require.InDelta(t, 1.6, cost("480p", 5).TotalCost, 1e-9, "渠道未配 480p 时取更高的 720p 档，而不是 0 元")

	// 渠道档位全部无价时回落到模型计费规则（内置可灵参考价 720p 0.084/秒）。
	pricing.Intervals = []PricingInterval{{TierLabel: "720p"}}
	require.InDelta(t, 0.42, cost("720p", 5).TotalCost, 1e-9)
}

func TestListPlazaGroups_VideoModelShowsChannelPricing(t *testing.T) {
	channels := []Channel{{
		ID: 1, Name: "video-ch", Status: StatusActive, GroupIDs: []int64{25},
		ModelPricing: []ChannelModelPricing{{
			Platform: PlatformVideo, Models: []string{"kling-v3"}, BillingMode: BillingModeVideo,
			Intervals: []PricingInterval{
				{TierLabel: "720p", PerRequestPrice: fp(0.32)},
				{TierLabel: "4K", PerRequestPrice: fp(0.96)},
			},
		}, {
			Platform: PlatformVideo, Models: []string{"kling-v3-turbo"}, BillingMode: BillingModeVideo,
			PerRequestPrice: fp(0.4),
		}},
	}}
	groups := []Group{{ID: 25, Name: "视频生成", Platform: PlatformVideo, RateMultiplier: 1}}
	out, err := newPlazaService(channels, groups, nil).ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	byName := map[string]PlazaModel{}
	for _, m := range out[0].Models {
		byName[m.Name] = m
	}

	spec := byName["kling-v3"].VideoPricing
	require.NotNil(t, spec)
	require.Equal(t, VideoBillingUnitPerSecond, spec.Unit)
	require.Len(t, spec.Prices, 2)
	require.InDelta(t, 0.32, *spec.Prices["720p"].PerSecond, 1e-9)
	require.InDelta(t, 0.96, *spec.Prices["4k"].PerSecond, 1e-9)
	require.Equal(t, 5, spec.MinDurationSeconds, "时长区间沿用模型计费规则")

	turbo := byName["kling-v3-turbo"].VideoPricing
	require.NotNil(t, turbo)
	require.InDelta(t, 0.4, *turbo.Prices[videoPriceWildcard].PerSecond, 1e-9, "渠道默认价展示为任意分辨率")
}

func TestVideoChannelUsageFieldsRecordsGroupChannel(t *testing.T) {
	groupID := int64(25)
	resolver := newOpenAIImageChannelPricingResolverForTest(t, groupID, "kling-v3", 0.32)
	svc := &VideoGatewayService{gateway: &OpenAIGatewayService{channelService: resolver.channelService}}

	fields := svc.videoChannelUsageFields(context.Background(), &APIKey{GroupID: &groupID}, "kling-v3")
	require.Equal(t, groupID, fields.ChannelID, "测试解析器以分组 ID 作为渠道 ID")
	require.Equal(t, "kling-v3", fields.ChannelMappedModel)

	require.Zero(t, svc.videoChannelUsageFields(context.Background(), &APIKey{}, "kling-v3").ChannelID)
}
