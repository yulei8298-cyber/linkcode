package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// EnterpriseSpent 用户累计消费 = 已完成订单的实付（套餐、订阅、余额充值都算）
// + 用户自己兑换的余额兑换码（卡密）。
// 余额充值订单入账时会生成 PAY- 开头的兑换码记录，已经按订单计入，这里排除避免重复；
// 管理员手动调整（admin_balance）是赠送，不算消费。已退款的订单状态不是 COMPLETED，自然不计。
func (r *packageRepository) EnterpriseSpent(ctx context.Context, userID int64) (float64, error) {
	const q = `
SELECT
    COALESCE((SELECT SUM(pay_amount) FROM payment_orders WHERE user_id = $1 AND status = $2), 0)
  + COALESCE((SELECT SUM(value) FROM redeem_codes
              WHERE used_by = $1 AND type = 'balance' AND status = 'used' AND code NOT LIKE 'PAY-%'), 0)`
	var total float64
	if err := r.db.QueryRowContext(ctx, q, userID, payment.OrderStatusCompleted).Scan(&total); err != nil {
		return 0, fmt.Errorf("query enterprise spent: %w", err)
	}
	return total, nil
}

// GetEnterpriseOverride 返回管理员对该用户的手动设置（on / off），没有则返回空串。
func (r *packageRepository) GetEnterpriseOverride(ctx context.Context, userID int64) (string, error) {
	var mode string
	err := r.db.QueryRowContext(ctx, `SELECT mode FROM user_enterprise_overrides WHERE user_id = $1`, userID).Scan(&mode)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query enterprise override: %w", err)
	}
	return mode, nil
}

// SetEnterpriseOverride 设置手动覆盖；mode 为空串表示去掉覆盖，回到自动判定。
func (r *packageRepository) SetEnterpriseOverride(ctx context.Context, userID int64, mode string) error {
	if mode == "" {
		if _, err := r.db.ExecContext(ctx, `DELETE FROM user_enterprise_overrides WHERE user_id = $1`, userID); err != nil {
			return fmt.Errorf("delete enterprise override: %w", err)
		}
		return nil
	}
	const q = `
INSERT INTO user_enterprise_overrides (user_id, mode, updated_at) VALUES ($1, $2, NOW())
ON CONFLICT (user_id) DO UPDATE SET mode = EXCLUDED.mode, updated_at = NOW()`
	if _, err := r.db.ExecContext(ctx, q, userID, mode); err != nil {
		return fmt.Errorf("set enterprise override: %w", translateEnterpriseFK(err))
	}
	return nil
}

// translateEnterpriseFK 用户不存在时（外键冲突，pq 错误码 23503）返回统一的「用户不存在」错误。
func translateEnterpriseFK(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr != nil && pqErr.Code == "23503" {
		return service.ErrUserNotFound
	}
	return err
}
