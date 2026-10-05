package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/userpackage"
	"github.com/Wei-Shaw/sub2api/ent/userpackagefreeze"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const packageDayLayout = "2006-01-02"

// FreezePackage 在行锁下校验并冻结一张套餐，同时写入冻结记录。
// 「今天能否冻结」属于业务规则，由 service 层在调用前判断。
func (r *packageRepository) FreezePackage(ctx context.Context, packageID, userID int64, now time.Time, caps service.PackageFreezeCaps) (*service.UserPackage, error) {
	var out *service.UserPackage
	err := r.withTx(ctx, func(txCtx context.Context, tx *dbent.Client) error {
		row, err := tx.UserPackage.Query().
			Where(userpackage.IDEQ(packageID), userpackage.UserIDEQ(userID)).
			ForUpdate().
			Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrPackageNotFound, nil)
		}
		if row.Status != service.PackageStatusActive || !row.ExpiresAt.After(now) || row.UsedUsd >= row.QuotaUsd {
			return service.ErrPackageNotFreezable
		}
		if capSeconds := caps.For(row.Cycle); capSeconds > 0 && row.FrozenSecondsTotal >= capSeconds {
			return service.ErrPackageFreezeCapUsed
		}
		updated, err := row.Update().
			SetStatus(service.PackageStatusFrozen).
			SetFrozenAt(now).
			Save(txCtx)
		if err != nil {
			return err
		}
		if _, err := tx.UserPackageFreeze.Create().
			SetPackageID(packageID).
			SetUserID(userID).
			SetFrozenAt(now).
			Save(txCtx); err != nil {
			return err
		}
		out = userPackageToService(updated)
		return nil
	})
	return out, err
}

// UnfreezePackage 解冻并按冻结时长顺延到期时间。
//
// 冻结时长超过上限时按上限截断：自动解冻任务存在扫描间隔，截断保证
// 不会因任务晚跑而多送冻结时间。
func (r *packageRepository) UnfreezePackage(ctx context.Context, in service.PackageUnfreezeInput) (*service.UserPackage, error) {
	var out *service.UserPackage
	err := r.withTx(ctx, func(txCtx context.Context, tx *dbent.Client) error {
		q := tx.UserPackage.Query().Where(userpackage.IDEQ(in.PackageID))
		if in.UserID != nil {
			q = q.Where(userpackage.UserIDEQ(*in.UserID))
		}
		row, err := q.ForUpdate().Only(txCtx)
		if err != nil {
			return translatePersistenceError(err, service.ErrPackageNotFound, nil)
		}
		if row.Status != service.PackageStatusFrozen || row.FrozenAt == nil {
			return service.ErrPackageNotFrozen
		}
		elapsed := int64(0)
		if in.Now.After(*row.FrozenAt) {
			elapsed = int64(in.Now.Sub(*row.FrozenAt) / time.Second)
		}
		if capSeconds := in.Caps.For(row.Cycle); capSeconds > 0 && row.FrozenSecondsTotal+elapsed > capSeconds {
			elapsed = max(capSeconds-row.FrozenSecondsTotal, 0)
		}
		shift := time.Duration(elapsed) * time.Second
		status := service.PackageStatusActive
		if row.UsedUsd >= row.QuotaUsd {
			status = service.PackageStatusExhausted
		}
		updated, err := row.Update().
			SetStatus(status).
			ClearFrozenAt().
			SetFrozenSecondsTotal(row.FrozenSecondsTotal + elapsed).
			SetExpiresAt(row.ExpiresAt.Add(shift)).
			Save(txCtx)
		if err != nil {
			return err
		}
		if _, err := tx.UserPackageFreeze.Update().
			Where(userpackagefreeze.PackageIDEQ(in.PackageID), userpackagefreeze.UnfrozenAtIsNil()).
			SetUnfrozenAt(row.FrozenAt.Add(shift)).
			SetUnfreezeReason(in.Reason).
			SetDurationSeconds(elapsed).
			Save(txCtx); err != nil {
			return err
		}
		out = userPackageToService(updated)
		return nil
	})
	return out, err
}

// ListFrozenOverCap 找出累计冻结已达上限的冻结中套餐；周卡与月卡各用各的上限。
func (r *packageRepository) ListFrozenOverCap(ctx context.Context, now time.Time, caps service.PackageFreezeCaps) ([]int64, error) {
	const q = `
SELECT id FROM user_packages
WHERE status = 'frozen'
  AND frozen_seconds_total + EXTRACT(EPOCH FROM ($1 - frozen_at)) >=
      CASE WHEN cycle = 'month' THEN $3::bigint ELSE $2::bigint END`
	return r.queryIDs(ctx, q, now, caps.WeekSeconds, caps.MonthSeconds)
}

func (r *packageRepository) ListFrozenIDs(ctx context.Context) ([]int64, error) {
	return r.queryIDs(ctx, `SELECT id FROM user_packages WHERE status = 'frozen'`)
}

func (r *packageRepository) queryIDs(ctx context.Context, q string, args ...any) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query package ids: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ---------- 可冻结日期 ----------

func (r *packageRepository) ListFreezeDays(ctx context.Context, from, to time.Time) ([]service.PackageFreezeDay, error) {
	const q = `
SELECT id, day::text, name, kind, source FROM package_freeze_days
WHERE day BETWEEN $1::date AND $2::date
ORDER BY day, source`
	rows, err := r.db.QueryContext(ctx, q, packageDay(from), packageDay(to))
	if err != nil {
		return nil, fmt.Errorf("list package freeze days: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []service.PackageFreezeDay
	for rows.Next() {
		var d service.PackageFreezeDay
		var day string
		if err := rows.Scan(&d.ID, &day, &d.Name, &d.Kind, &d.Source); err != nil {
			return nil, err
		}
		if d.Day, err = timezone.ParseInLocation(packageDayLayout, day); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ReplaceAutoFreezeDays 用官方数据整体替换某一年的自动同步记录，手动添加的不受影响。
func (r *packageRepository) ReplaceAutoFreezeDays(ctx context.Context, year int, days []service.PackageFreezeDay) error {
	return r.withSQLTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM package_freeze_days WHERE source = 'auto' AND day BETWEEN $1::date AND $2::date`,
			fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year)); err != nil {
			return err
		}
		for _, d := range days {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO package_freeze_days (day, name, kind, source) VALUES ($1::date, $2, $3, 'auto')
				 ON CONFLICT (day, source) DO UPDATE SET name = EXCLUDED.name, kind = EXCLUDED.kind, updated_at = NOW()`,
				packageDay(d.Day), d.Name, d.Kind); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *packageRepository) AddManualFreezeDays(ctx context.Context, name string, from, to time.Time) error {
	return r.withSQLTx(ctx, func(tx *sql.Tx) error {
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO package_freeze_days (day, name, kind, source) VALUES ($1::date, $2, 'off', 'manual')
				 ON CONFLICT (day, source) DO UPDATE SET name = EXCLUDED.name, kind = 'off', updated_at = NOW()`,
				packageDay(d), name); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *packageRepository) DeleteManualFreezeDays(ctx context.Context, name string, from, to time.Time) (int, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM package_freeze_days WHERE source = 'manual' AND name = $1 AND day BETWEEN $2::date AND $3::date`,
		name, packageDay(from), packageDay(to))
	if err != nil {
		return 0, fmt.Errorf("delete manual freeze days: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ---------- 事务 ----------

func (r *packageRepository) withTx(ctx context.Context, fn func(txCtx context.Context, tx *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin package transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(dbent.NewTxContext(ctx, tx), tx.Client()); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *packageRepository) withSQLTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin package freeze day transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// packageDay 把时间按服务时区格式化为 DATE 文本。
func packageDay(t time.Time) string {
	return t.In(timezone.Location()).Format(packageDayLayout)
}
