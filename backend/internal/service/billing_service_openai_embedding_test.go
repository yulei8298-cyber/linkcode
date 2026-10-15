//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 回归：OpenAI embedding 模型（text-embedding-3-small/large、ada-002）此前在内置价格表和
// 在线价格表里都没有价格，网关按「缺价放行并记 0」处理，调用方等于免费使用。

var openAIEmbeddingPricesPerMTok = []struct {
	model string
	price float64 // 官方价，每百万输入 Token（美元）
}{
	{"text-embedding-3-small", 0.02},
	{"text-embedding-3-large", 0.13},
	{"text-embedding-ada-002", 0.10},
}

func TestOpenAIEmbeddingFallbackPricing(t *testing.T) {
	svc := newTestBillingService()

	for _, tc := range openAIEmbeddingPricesPerMTok {
		t.Run(tc.model, func(t *testing.T) {
			require.True(t, svc.HasIdentifiedTokenPricing(tc.model), "应能识别价格，在途预留才能估价")

			pricing, err := svc.GetModelPricing(tc.model)
			require.NoError(t, err)
			require.InDelta(t, tc.price/1e6, pricing.InputPricePerToken, 1e-15)
			require.Zero(t, pricing.OutputPricePerToken, "embedding 没有输出 Token")

			// 100 万输入 Token，倍率 6.8（官key 分组）：标准价 = 官方价，实扣 = 官方价 × 6.8。
			cost, err := svc.CalculateCost(tc.model, UsageTokens{InputTokens: 1_000_000}, 6.8)
			require.NoError(t, err)
			require.InDelta(t, tc.price, cost.InputCost, 1e-9)
			require.InDelta(t, tc.price, cost.TotalCost, 1e-9)
			require.InDelta(t, tc.price*6.8, cost.ActualCost, 1e-9)
			require.Zero(t, cost.OutputCost)
		})
	}
}

func TestOpenAIEmbeddingFallbackPricing_MatchesAliases(t *testing.T) {
	svc := newTestBillingService()

	for alias, want := range map[string]float64{
		"OpenAI/Text-Embedding-3-Large": 0.13,
		"text-embedding-3-small":        0.02,
		"openai/text-embedding-ada-002": 0.10,
	} {
		pricing, err := svc.GetModelPricing(alias)
		require.NoError(t, err, alias)
		require.InDelta(t, want/1e6, pricing.InputPricePerToken, 1e-15, alias)
	}

	// 未登记的新型号不会被误计价，仍然是缺价（由渠道定价补）。
	_, err := svc.GetModelPricing("text-embedding-4-xl")
	require.Error(t, err)
}

func TestOpenAIEmbedding_ChannelPricingOverridesFallback(t *testing.T) {
	r := newResolverWithChannel(t, []ChannelModelPricing{{
		Platform:    "anthropic",
		Models:      []string{"text-embedding-3-large"},
		BillingMode: BillingModeToken,
		InputPrice:  testPtrFloat64(0.5e-6), // 运营给这个分组单独定价
	}})
	// 解析器测试用的计费服务只有少量内置价，换成完整的，才能验证「渠道价压过内置价」。
	r = NewModelPricingResolver(r.channelService, newTestBillingService())

	cost, err := newTestBillingService().CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "text-embedding-3-large",
		GroupID:        groupIDPtr(),
		Tokens:         UsageTokens{InputTokens: 2_000_000},
		RequestCount:   1,
		RateMultiplier: 2,
		Resolver:       r,
	})
	require.NoError(t, err)
	require.InDelta(t, 1.0, cost.TotalCost, 1e-9, "2M × $0.5/M")
	require.InDelta(t, 2.0, cost.ActualCost, 1e-9)
}

func TestOpenAIEmbedding_ChannelOnlyPricingWorksWithoutBasePrice(t *testing.T) {
	// 内置价里没有的 embedding 新型号：只在渠道里配了价格也必须能计费，不能再记 0。
	r := newResolverWithChannel(t, []ChannelModelPricing{{
		Platform:    "anthropic",
		Models:      []string{"text-embedding-4-xl"},
		BillingMode: BillingModeToken,
		InputPrice:  testPtrFloat64(0.13e-6),
		OutputPrice: testPtrFloat64(0),
	}})

	cost, err := newTestBillingService().CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "text-embedding-4-xl",
		GroupID:        groupIDPtr(),
		Tokens:         UsageTokens{InputTokens: 1_000_000},
		RequestCount:   1,
		RateMultiplier: 6.8,
		Resolver:       r,
	})
	require.NoError(t, err)
	require.InDelta(t, 0.13, cost.TotalCost, 1e-9)
	require.InDelta(t, 0.13*6.8, cost.ActualCost, 1e-9)
}
