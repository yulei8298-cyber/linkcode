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

// enterprisePaidStatuses 已经收到钱的订单状态：已付款、发货中、已完成，以及退款流程中的各状态。
// 退款申请中 / 处理中 / 失败时钱还没退回，照常按全额计入；退款完成后（部分 / 全额）才扣掉退款部分。
var enterprisePaidStatuses = []string{
	payment.OrderStatusPaid,
	payment.OrderStatusRecharging,
	payment.OrderStatusCompleted,
	payment.OrderStatusRefundRequested,
	payment.OrderStatusRefunding,
	payment.OrderStatusRefundPending,
	payment.OrderStatusRefundFailed,
	payment.OrderStatusPartiallyRefunded,
	payment.OrderStatusRefunded,
}

// EnterpriseSpent 用户累计消费 =
//
//	订单的净实付（套餐、订阅、余额充值都算，扣掉已完成的订单退款）
//
// + 用户自己兑换的余额兑换码（卡密）
// + 管理员在后台的余额调整：「充值」为正数计入，「退款」为负数扣除（多为线下付款代充 / 线下退款）。
// 结果最低为 0。
//
// 订单退款金额按到账额度（amount）记录，实付（pay_amount）与到账额度成固定比例，
// 所以部分退款后的净实付 = pay_amount × (1 − refund_amount / amount)，全额退款为 0。
// 余额充值订单入账时会生成 PAY- 开头的兑换码记录，已经按订单计入，这里排除避免重复。
func (r *packageRepository) EnterpriseSpent(ctx context.Context, userID int64) (float64, error) {
	const q = `
SELECT GREATEST(0,
    COALESCE((
        SELECT SUM(CASE
            WHEN status IN ($3, $4) THEN
                CASE WHEN amount > 0 THEN pay_amount * GREATEST(0, 1 - refund_amount / amount) ELSE 0 END
            ELSE pay_amount
        END)
        FROM payment_orders
        WHERE user_id = $1 AND status = ANY($2)
    ), 0)
  + COALESCE((SELECT SUM(value) FROM redeem_codes
              WHERE used_by = $1 AND status = 'used'
                AND ((type = 'balance' AND code NOT LIKE 'PAY-%') OR type = 'admin_balance')), 0)
)`
	var total float64
	err := r.db.QueryRowContext(ctx, q, userID, pq.Array(enterprisePaidStatuses),
		payment.OrderStatusPartiallyRefunded, payment.OrderStatusRefunded).Scan(&total)
	if err != nil {
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
