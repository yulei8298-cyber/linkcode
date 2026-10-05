//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestListFrozenOverCap_UsesPerCycleCaps(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, time.October, 5, 12, 0, 0, 0, time.UTC)
	caps := service.PackageFreezeCaps{WeekSeconds: 7 * 86400, MonthSeconds: 15 * 86400}
	// SQL 里按 cycle 选上限：month 用 $3，其余用 $2。
	mock.ExpectQuery(`(?s)SELECT id FROM user_packages.*CASE WHEN cycle = 'month' THEN \$3::bigint ELSE \$2::bigint END`).
		WithArgs(now, caps.WeekSeconds, caps.MonthSeconds).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(5)).AddRow(int64(9)))

	ids, err := (&packageRepository{db: db}).ListFrozenOverCap(context.Background(), now, caps)
	require.NoError(t, err)
	require.Equal(t, []int64{5, 9}, ids)
	require.NoError(t, mock.ExpectationsWereMet())
}
