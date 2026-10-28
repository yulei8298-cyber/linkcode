//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/stretchr/testify/require"
)

func fp(v float64) *float64 { return &v }

func TestVideoPricingSpecPerSecondWithAudioAndTierFallback(t *testing.T) {
	spec := VideoPricingSpec{
		Unit: VideoBillingUnitPerSecond,
		Prices: map[string]VideoTierPrice{
			video.Resolution720P:  {PerSecond: fp(0.1), PerSecondWithAudio: fp(0.15)},
			video.Resolution1080P: {PerSecond: fp(0.2)},
		},
		MinDurationSeconds: 2, MaxDurationSeconds: 30,
	}
	require.NoError(t, spec.Validate())
	cost, err := spec.BaseCost(spec.NormalizeUsage(VideoUsage{Resolution: "720p", DurationSeconds: 10}), 0)
	require.NoError(t, err)
	require.InDelta(t, 1.0, cost, 1e-9)
	cost, err = spec.BaseCost(spec.NormalizeUsage(VideoUsage{Resolution: "720p", DurationSeconds: 10, WithAudio: true}), 0)
	require.NoError(t, err)
	require.InDelta(t, 1.5, cost, 1e-9)
	// 480p 未配置时取更高的最近档（720p），4k 未配置时取最高档（1080p）。
	cost, _ = spec.BaseCost(spec.NormalizeUsage(VideoUsage{Resolution: "480p", DurationSeconds: 1}), 0)
	require.InDelta(t, 0.2, cost, 1e-9, "低于下限的时长收敛到 2 秒")
	cost, _ = spec.BaseCost(spec.NormalizeUsage(VideoUsage{Resolution: "4k", DurationSeconds: 60}), 0)
	require.InDelta(t, 6.0, cost, 1e-9, "超过上限的时长收敛到 30 秒")
}

func TestVideoPricingSpecPerVideoDurationTiers(t *testing.T) {
	spec := VideoPricingSpec{Unit: VideoBillingUnitPerVideo, Prices: map[string]VideoTierPrice{
		video.Resolution720P: {PerVideo: map[string]float64{"6": 0.28, "10": 0.56}},
	}}
	require.NoError(t, spec.Validate())
	for duration, want := range map[int]float64{6: 0.28, 5: 0.28, 8: 0.56, 10: 0.56, 15: 0.56} {
		cost, err := spec.BaseCost(VideoUsage{Resolution: video.Resolution720P, DurationSeconds: duration, Count: 2}, 0)
		require.NoError(t, err)
		require.InDelta(t, want*2, cost, 1e-9, "duration %d", duration)
	}
}

func TestVideoPricingSpecTokenUnit(t *testing.T) {
	spec := VideoPricingSpec{Unit: VideoBillingUnitToken, TokenPricePerMillion: fp(10)}
	require.NoError(t, spec.Validate())
	cost, err := spec.BaseCost(VideoUsage{CompletionTokens: 200_000}, 0)
	require.NoError(t, err)
	require.InDelta(t, 2.0, cost, 1e-9)
	fallback := VideoPricingSpec{Unit: VideoBillingUnitToken}
	require.Error(t, fallback.Validate(), "按 token 计费必须给出单价")
	cost, err = fallback.BaseCost(VideoUsage{CompletionTokens: 1_000_000}, 0.000002)
	require.NoError(t, err)
	require.InDelta(t, 2.0, cost, 1e-9, "未填单价时回退目录输出价")
}

func TestVideoPricingSpecValidateRejects(t *testing.T) {
	for _, spec := range []VideoPricingSpec{
		{Unit: "per_minute"},
		{Unit: VideoBillingUnitPerSecond},
		{Unit: VideoBillingUnitPerSecond, Prices: map[string]VideoTierPrice{"8k": {PerSecond: fp(1)}}},
		{Unit: VideoBillingUnitPerSecond, Prices: map[string]VideoTierPrice{"720p": {PerSecond: fp(-1)}}},
		{Unit: VideoBillingUnitPerVideo, Prices: map[string]VideoTierPrice{"720p": {PerVideo: map[string]float64{"abc": 1}}}},
		{Unit: VideoBillingUnitPerSecond, Prices: map[string]VideoTierPrice{"720p": {PerSecond: fp(1)}}, MinDurationSeconds: 10, MaxDurationSeconds: 5},
	} {
		require.Error(t, spec.Validate(), "%+v", spec)
	}
}

func TestDefaultVideoPricingCatalog(t *testing.T) {
	spec, ok := LookupDefaultVideoPricingSpec("Wan3.0-Video-Prime")
	require.True(t, ok)
	require.Equal(t, 30, spec.MaxDurationSeconds)
	cost, err := spec.BaseCost(spec.NormalizeUsage(VideoUsage{Resolution: "1080p", DurationSeconds: 30}), 0)
	require.NoError(t, err)
	require.InDelta(t, 5.1, cost, 1e-9)
	spec, ok = LookupDefaultVideoPricingSpec("veo-3.1-fast-generate-preview")
	require.True(t, ok)
	require.InDelta(t, 0.10, *spec.Prices[video.Resolution720P].PerSecond, 1e-9, "最长前缀优先命中 fast 档")
	_, ok = LookupDefaultVideoPricingSpec("grok-imagine-video")
	require.False(t, ok, "Grok 保持独立默认价，不受视频规则影响")
	for prefix, s := range DefaultVideoPricingSpecs() {
		require.NoError(t, s.Validate(), prefix)
	}
}

func TestAdminModelPriceAcceptsVideoOnly(t *testing.T) {
	price := AdminModelPrice{Video: &VideoPricingSpec{Unit: VideoBillingUnitPerSecond, Prices: map[string]VideoTierPrice{"*": {PerSecond: fp(0.05)}}}}
	require.NoError(t, price.validate())
	price.Video.Unit = "bad"
	require.Error(t, price.validate())
}
