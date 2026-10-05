package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/shopspring/decimal"
)

// usageBillingPackageRow 扣费事务内锁定的一张可用套餐。
type usageBillingPackageRow struct {
	id     int64
	remain decimal.Decimal
}

// deductUsageBillingPackages 在计费事务内按到期先后扣减套餐额度，返回实际从套餐扣掉的金额。
//
// 只扣 active、未到期、未用完的套餐；冻结的套餐 status 为 frozen，天然被跳过。
// 行锁（FOR UPDATE）只覆盖该用户在该分组的套餐，与冻结 / 解冻操作互斥。
// 金额用 decimal 计算并以 NUMERIC 文本写回，避免 float 累加误差让 used_usd 越过 quota_usd。
func deductUsageBillingPackages(ctx context.Context, tx *sql.Tx, userID, groupID int64, amount float64) (float64, bool, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, (quota_usd - used_usd)::text
		FROM user_packages
		WHERE user_id = $1 AND group_id = $2 AND status = 'active'
		  AND expires_at > NOW() AND used_usd < quota_usd
		ORDER BY expires_at, id
		FOR UPDATE
	`, userID, groupID)
	if err != nil {
		return 0, false, fmt.Errorf("lock usage billing packages: %w", err)
	}
	var candidates []usageBillingPackageRow
	for rows.Next() {
		var row usageBillingPackageRow
		var remain string
		if err := rows.Scan(&row.id, &remain); err != nil {
			_ = rows.Close()
			return 0, false, err
		}
		if row.remain, err = decimal.NewFromString(remain); err != nil {
			_ = rows.Close()
			return 0, false, err
		}
		candidates = append(candidates, row)
	}
	if err := rows.Close(); err != nil {
		return 0, false, err
	}

	pending := decimal.NewFromFloat(amount)
	covered := decimal.Zero
	exhausted := false
	for _, row := range candidates {
		if !pending.IsPositive() {
			break
		}
		take := decimal.Min(pending, row.remain)
		usedUp := take.Equal(row.remain)
		if _, err := tx.ExecContext(ctx, `
			UPDATE user_packages
			SET used_usd = used_usd + $1::numeric,
			    status = CASE WHEN $2 THEN 'exhausted' ELSE status END,
			    updated_at = NOW()
			WHERE id = $3
		`, take.String(), usedUp, row.id); err != nil {
			return 0, false, fmt.Errorf("deduct usage billing package %d: %w", row.id, err)
		}
		pending = pending.Sub(take)
		covered = covered.Add(take)
		exhausted = exhausted || usedUp
	}
	value, _ := covered.Float64()
	return value, exhausted, nil
}
