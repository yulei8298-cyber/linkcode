//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestAdminPackageWhere_BuildsConditionsAndArgs(t *testing.T) {
	where, args := adminPackageWhere(service.AdminPackageFilter{})
	require.Empty(t, where)
	require.Empty(t, args)

	where, args = adminPackageWhere(service.AdminPackageFilter{Status: "frozen", Cycle: "month", GroupID: 2, Keyword: "100%_x"})
	require.Equal(t, " WHERE up.status = $1 AND up.cycle = $2 AND up.group_id = $3 AND (u.email ILIKE $4 OR u.username ILIKE $4)", where)
	require.Equal(t, []any{"frozen", "month", int64(2), `%100\%\_x%`}, args, "LIKE 通配符按字面匹配")

	// 纯数字关键词同时匹配用户 ID。
	where, args = adminPackageWhere(service.AdminPackageFilter{Keyword: "42"})
	require.Equal(t, " WHERE (u.email ILIKE $1 OR u.username ILIKE $1 OR up.user_id = $2)", where)
	require.Equal(t, []any{"%42%", int64(42)}, args)
}

func TestAdminListPackages_ScansRowsAndPaginates(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, time.October, 6, 12, 0, 0, 0, time.UTC)
	cols := []string{"id", "user_id", "group_id", "plan_id", "order_id", "name", "cycle", "tier", "quota_usd", "used_usd",
		"starts_at", "expires_at", "status", "frozen_at", "frozen_seconds_total", "created_at", "updated_at",
		"email", "username", "group_name", "amount"}
	mock.ExpectQuery(`SELECT COUNT\(\*\)\s+FROM user_packages up`).
		WithArgs("active").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(41)))
	mock.ExpectQuery(`(?s)SELECT up\.id.*ORDER BY up\.created_at DESC, up\.id DESC LIMIT \$2 OFFSET \$3`).
		WithArgs("active", 20, 20).
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow(int64(7), int64(9), int64(2), int64(1), int64(55), "摸鱼周卡", "week", int64(1), 10.0, 2.5,
				now, now.Add(7*24*time.Hour), "active", nil, int64(0), now, now, "a@example.com", "alice", "GPT-Pro", 1.0).
			AddRow(int64(6), int64(10), int64(2), int64(1), nil, "爆肝月卡", "month", int64(2), 80.0, 0.0,
				now, now.Add(30*24*time.Hour), "active", now, int64(3600), now, now, "", "", "", 0.0))

	rows, total, err := (&packageRepository{db: db}).AdminListPackages(context.Background(),
		service.AdminPackageFilter{Status: "active", Page: 2, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 41, total)
	require.Len(t, rows, 2)
	require.Equal(t, int64(55), *rows[0].OrderID)
	require.Equal(t, "alice", rows[0].Username)
	require.Equal(t, "GPT-Pro", rows[0].GroupName)
	require.Nil(t, rows[0].FrozenAt)
	require.Nil(t, rows[1].OrderID, "管理员手工发放的套餐没有订单")
	require.NotNil(t, rows[1].FrozenAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminListPackages_EmptyResultSkipsPageQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`SELECT COUNT\(\*\)`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	rows, total, err := (&packageRepository{db: db}).AdminListPackages(context.Background(), service.AdminPackageFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminPackageStats_QueriesWithCompletedOrdersOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	since := time.Date(2026, time.September, 6, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)COUNT\(\*\) FILTER.*po\.status = \$2`).
		WithArgs(since, payment.OrderStatusCompleted).
		WillReturnRows(sqlmock.NewRows([]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}).
			AddRow(int64(10), int64(4), int64(2), int64(1), int64(2), int64(1), 300.0, 120.5, int64(6), 18.0))

	s, err := (&packageRepository{db: db}).AdminPackageStats(context.Background(), since)
	require.NoError(t, err)
	require.Equal(t, int64(10), s.Total)
	require.Equal(t, int64(2), s.Frozen)
	require.Equal(t, 300.0, s.LiveQuotaUSD)
	require.Equal(t, int64(6), s.RecentSold)
	require.Equal(t, 18.0, s.RecentRevenue)
	require.Equal(t, service.AdminPackageSalesWindowDays, s.WindowDays)
	require.NoError(t, mock.ExpectationsWereMet())
}
