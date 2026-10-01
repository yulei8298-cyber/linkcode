//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 每日免费分组不扣余额（余额常为 0），并发请求不得因余额在途预留被判余额不足。
func TestReserveInflight_DailyFreeGroupSkipsBalanceReservation(t *testing.T) {
	cache := newMemInflightCache(0)
	svc := newInflightSvc(t, cache, 60)
	user := &User{ID: 3}
	freeGroup := &Group{ID: 66, IsFree: true}

	for i := 0; i < 3; i++ {
		res, err := svc.ReserveInflight(context.Background(), user, freeGroup, nil, 0.5)
		require.NoError(t, err)
		require.Nil(t, res)
	}
	release, err := svc.ReserveInflightBalance(context.Background(), user, freeGroup, nil, 0.5)
	require.NoError(t, err)
	release()
	require.Equal(t, 0, cache.count(), "免费分组不应登记任何在途预留")

	// 普通余额分组仍按余额预留：首个请求放行，余额不足的并发请求被拒绝。
	normal := &Group{ID: 67}
	first, err := svc.ReserveInflight(context.Background(), user, normal, nil, 0.5)
	require.NoError(t, err)
	require.NotNil(t, first)
	_, err = svc.ReserveInflight(context.Background(), user, normal, nil, 0.5)
	require.ErrorIs(t, err, ErrInsufficientBalance)
	first.HandlerDone()
}
