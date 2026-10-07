//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestEnterpriseSpent_CompletedOrdersPlusUserRedeemedBalanceCodes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// 订单只算已完成；兑换码只算用户自己兑换的余额码，且排除订单入账生成的 PAY- 码与管理员赠送。
	mock.ExpectQuery(`(?s)FROM payment_orders WHERE user_id = \$1 AND status = \$2.*type = 'balance' AND status = 'used' AND code NOT LIKE 'PAY-%'`).
		WithArgs(int64(7), payment.OrderStatusCompleted).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(3120.5))

	total, err := (&packageRepository{db: db}).EnterpriseSpent(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, 3120.5, total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseOverride_GetSetAndClear(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &packageRepository{db: db}
	ctx := context.Background()

	mock.ExpectQuery(`SELECT mode FROM user_enterprise_overrides WHERE user_id = \$1`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"mode"}))
	mode, err := repo.GetEnterpriseOverride(ctx, 7)
	require.NoError(t, err)
	require.Empty(t, mode, "没有记录即自动判定")

	mock.ExpectExec(`INSERT INTO user_enterprise_overrides .* ON CONFLICT \(user_id\) DO UPDATE`).WithArgs(int64(7), "on").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.SetEnterpriseOverride(ctx, 7, "on"))

	mock.ExpectExec(`DELETE FROM user_enterprise_overrides WHERE user_id = \$1`).WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.SetEnterpriseOverride(ctx, 7, ""))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSetEnterpriseOverride_UnknownUserBecomesUserNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectExec(`INSERT INTO user_enterprise_overrides`).WithArgs(int64(999), "off").
		WillReturnError(&pq.Error{Code: "23503"})

	err = (&packageRepository{db: db}).SetEnterpriseOverride(context.Background(), 999, "off")
	require.ErrorIs(t, err, service.ErrUserNotFound)
}
