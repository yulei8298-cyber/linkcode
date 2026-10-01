package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 每日免费分组跳过余额在途预留，且须早于 fail_closed_on_unpriced 判断。
func TestReserveInflightBalance_SkipsDailyFreeGroup(t *testing.T) {
	cache := newHandlerInflightCache(0)
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, TTLSeconds: 60, FailClosedOnUnpriced: true}
	billing := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	apiKey := &service.APIKey{User: &service.User{ID: 1}, Group: &service.Group{ID: 66, IsFree: true}}

	for _, est := range []*countingEstimator{{cost: 1, priced: true}, {priced: false}} {
		done, err := reserveInflightBalance(newInflightTestGinContext(), billing, est, apiKey, nil, tokenInflightEstimate("m", []byte(`{}`)))
		require.NoError(t, err)
		done()
		require.Equal(t, 0, est.calls, "免费分组不应估算或预留")
	}
}
