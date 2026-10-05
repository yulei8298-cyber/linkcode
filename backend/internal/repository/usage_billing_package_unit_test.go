//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	lockUsageBillingPackagesSQL  = `(?s)SELECT id, \(quota_usd - used_usd\)::text\s+FROM user_packages\s+WHERE user_id = \$1 AND group_id = \$2 AND status = 'active'.*ORDER BY expires_at, id\s+FOR UPDATE`
	deductUsageBillingPackageSQL = `(?s)UPDATE user_packages\s+SET used_usd = used_usd \+ \$1::numeric,\s+status = CASE WHEN \$2 THEN 'exhausted' ELSE status END`
	packageLockColumns           = "remain"
)

func TestDeductUsageBillingPackages_SpansPackagesInExpiryOrder(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(lockUsageBillingPackagesSQL).
		WithArgs(int64(42), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", packageLockColumns}).
			AddRow(int64(11), "0.30000000").
			AddRow(int64(12), "5.00000000"))
	// 先到期的一张被扣满并标记用完，剩余 0.2 由下一张承担。
	mock.ExpectExec(deductUsageBillingPackageSQL).
		WithArgs("0.3", true, int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(deductUsageBillingPackageSQL).
		WithArgs("0.2", false, int64(12)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	covered, exhausted, err := deductUsageBillingPackages(ctx, tx, 42, 7, 0.5)
	require.NoError(t, err)
	require.InDelta(t, 0.5, covered, 1e-12)
	require.True(t, exhausted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingPackages_NoUsablePackageCoversNothing(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(lockUsageBillingPackagesSQL).
		WithArgs(int64(42), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", packageLockColumns}))
	mock.ExpectCommit()

	covered, exhausted, err := deductUsageBillingPackages(ctx, tx, 42, 7, 1.25)
	require.NoError(t, err)
	require.Zero(t, covered)
	require.False(t, exhausted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_PackageCoversPartThenBalance(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(lockUsageBillingPackagesSQL).
		WithArgs(int64(42), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", packageLockColumns}).AddRow(int64(11), "1.00000000"))
	mock.ExpectExec(deductUsageBillingPackageSQL).
		WithArgs("1", true, int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// 套餐只覆盖 1.0，剩余 0.5 走原有余额扣减。
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(0.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.5))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:         42,
		PackageGroupID: pointerToInt64(7),
		BalanceCost:    1.5,
	}, result)
	require.NoError(t, err)
	require.InDelta(t, 1.0, result.PackageCost, 1e-12)
	require.True(t, result.PackageExhausted)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, 9.5, *result.NewBalance, 1e-12)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_PackageCoversAllSkipsBalance(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(lockUsageBillingPackagesSQL).
		WithArgs(int64(42), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", packageLockColumns}).AddRow(int64(11), "10.00000000"))
	mock.ExpectExec(deductUsageBillingPackageSQL).
		WithArgs("1.5", false, int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:         42,
		PackageGroupID: pointerToInt64(7),
		BalanceCost:    1.5,
	}, result)
	require.NoError(t, err)
	require.InDelta(t, 1.5, result.PackageCost, 1e-12)
	require.False(t, result.PackageExhausted)
	require.Nil(t, result.NewBalance, "套餐覆盖全部费用时不应扣余额")
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
