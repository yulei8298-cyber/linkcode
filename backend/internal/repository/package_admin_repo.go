package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// adminPackageWhere 按筛选条件拼接 WHERE 子句，返回子句与位置参数。
func adminPackageWhere(f service.AdminPackageFilter) (string, []any) {
	var conds []string
	var args []any
	add := func(cond string, arg any) {
		args = append(args, arg)
		conds = append(conds, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if f.Status != "" {
		add("up.status = ?", f.Status)
	}
	if f.Cycle != "" {
		add("up.cycle = ?", f.Cycle)
	}
	if f.GroupID > 0 {
		add("up.group_id = ?", f.GroupID)
	}
	if f.Keyword != "" {
		like := "%" + escapeLike(f.Keyword) + "%"
		args = append(args, like)
		n := strconv.Itoa(len(args))
		cond := "u.email ILIKE $" + n + " OR u.username ILIKE $" + n
		if id, err := strconv.ParseInt(f.Keyword, 10, 64); err == nil && id > 0 {
			args = append(args, id)
			cond += " OR up.user_id = $" + strconv.Itoa(len(args))
		}
		conds = append(conds, "("+cond+")")
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

const adminPackageFrom = `
FROM user_packages up
LEFT JOIN users u ON u.id = up.user_id
LEFT JOIN groups g ON g.id = up.group_id
LEFT JOIN payment_orders po ON po.id = up.order_id`

// AdminListPackages 跨用户分页列出套餐，最新购买的在前。
func (r *packageRepository) AdminListPackages(ctx context.Context, f service.AdminPackageFilter) ([]service.AdminPackageRow, int64, error) {
	where, args := adminPackageWhere(f)

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*)"+adminPackageFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count admin packages: %w", err)
	}
	if total == 0 {
		return []service.AdminPackageRow{}, 0, nil
	}

	limit, offset := len(args)+1, len(args)+2
	q := `SELECT up.id, up.user_id, up.group_id, up.plan_id, up.order_id, up.name, up.cycle, up.tier,
       up.quota_usd, up.used_usd, up.starts_at, up.expires_at, up.status, up.frozen_at,
       up.frozen_seconds_total, up.created_at, up.updated_at,
       COALESCE(u.email, ''), COALESCE(u.username, ''), COALESCE(g.name, ''), COALESCE(po.amount, 0)` +
		adminPackageFrom + where +
		fmt.Sprintf(" ORDER BY up.created_at DESC, up.id DESC LIMIT $%d OFFSET $%d", limit, offset)
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query admin packages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.AdminPackageRow, 0, f.PageSize)
	for rows.Next() {
		var (
			row      service.AdminPackageRow
			orderID  sql.NullInt64
			frozenAt sql.NullTime
		)
		p := &row.UserPackage
		if err := rows.Scan(&p.ID, &p.UserID, &p.GroupID, &p.PlanID, &orderID, &p.Name, &p.Cycle, &p.Tier,
			&p.QuotaUSD, &p.UsedUSD, &p.StartsAt, &p.ExpiresAt, &p.Status, &frozenAt,
			&p.FrozenSecondsTotal, &p.CreatedAt, &p.UpdatedAt,
			&row.UserEmail, &row.Username, &row.GroupName, &row.PaidAmount); err != nil {
			return nil, 0, fmt.Errorf("scan admin package: %w", err)
		}
		if orderID.Valid {
			p.OrderID = &orderID.Int64
		}
		if frozenAt.Valid {
			p.FrozenAt = &frozenAt.Time
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate admin packages: %w", err)
	}
	return out, total, nil
}

// AdminPackageStats 汇总各状态张数、生效中套餐的额度，以及 since 之后售出的张数与实收金额。
// 实收只统计已完成的订单，退款后订单状态改变，自然不再计入。
func (r *packageRepository) AdminPackageStats(ctx context.Context, since time.Time) (*service.AdminPackageStats, error) {
	const q = `
SELECT
    COUNT(*),
    COUNT(*) FILTER (WHERE up.status = 'active'),
    COUNT(*) FILTER (WHERE up.status = 'frozen'),
    COUNT(*) FILTER (WHERE up.status = 'exhausted'),
    COUNT(*) FILTER (WHERE up.status = 'expired'),
    COUNT(*) FILTER (WHERE up.status = 'voided'),
    COALESCE(SUM(up.quota_usd) FILTER (WHERE up.status IN ('active', 'frozen')), 0),
    COALESCE(SUM(up.used_usd) FILTER (WHERE up.status IN ('active', 'frozen')), 0),
    COUNT(*) FILTER (WHERE up.created_at >= $1),
    COALESCE(SUM(po.amount) FILTER (WHERE up.created_at >= $1 AND po.status = $2), 0)
FROM user_packages up
LEFT JOIN payment_orders po ON po.id = up.order_id`
	s := &service.AdminPackageStats{WindowDays: service.AdminPackageSalesWindowDays}
	err := r.db.QueryRowContext(ctx, q, since, payment.OrderStatusCompleted).Scan(
		&s.Total, &s.Active, &s.Frozen, &s.Exhausted, &s.Expired, &s.Voided,
		&s.LiveQuotaUSD, &s.LiveUsedUSD, &s.RecentSold, &s.RecentRevenue)
	if err != nil {
		return nil, fmt.Errorf("query admin package stats: %w", err)
	}
	return s, nil
}
