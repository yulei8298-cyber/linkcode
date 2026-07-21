//go:build unit

package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupFromServiceAdminMapsChatStationFreeConfig(t *testing.T) {
	limit := 0.75
	out := GroupFromServiceAdmin(&service.Group{
		ID:                42,
		IsHidden:          true,
		IsFree:            true,
		DailyFreeLimitUSD: &limit,
		ChatStationOnly:   true,
		IPWhitelist:       []string{"203.0.113.0/24"},
		IPBlacklist:       []string{"198.51.100.10"},
	})

	require.NotNil(t, out)
	require.True(t, out.IsHidden)
	require.True(t, out.IsFree)
	require.NotNil(t, out.DailyFreeLimitUSD)
	require.InDelta(t, limit, *out.DailyFreeLimitUSD, 1e-12)
	require.True(t, out.ChatStationOnly)
	require.Equal(t, []string{"203.0.113.0/24"}, out.IPWhitelist)
	require.Equal(t, []string{"198.51.100.10"}, out.IPBlacklist)
}
