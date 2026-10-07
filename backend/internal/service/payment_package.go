package service

import (
	"context"
	"fmt"
	"log/slog"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 二开：套餐（周卡 / 月卡）订单的下单校验、发货与退款作废。
// 订单复用 payment_orders：order_type=package，plan_id 指向 package_plans。

// SetPackageService 注入套餐服务（未注入时套餐订单下单直接拒绝）。
func (s *PaymentService) SetPackageService(packageService *PackageService) {
	s.packageService = packageService
}

// validatePackageOrder 套餐订单：套餐在售、分组可用、须知版本为当前版本。非套餐订单返回 nil。
func (s *PaymentService) validatePackageOrder(ctx context.Context, req CreateOrderRequest) (*PackagePlan, error) {
	if req.OrderType != payment.OrderTypePackage {
		return nil, nil
	}
	if s.packageService == nil {
		return nil, infraerrors.Forbidden("PACKAGE_UNAVAILABLE", "套餐功能暂不可用")
	}
	if req.PlanID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "请选择要购买的套餐")
	}
	plan, err := s.packageService.GetPlanForPurchase(ctx, req.PlanID)
	if err != nil {
		return nil, err
	}
	settings, err := s.packageService.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if req.PackageNoticeVersion != settings.NoticeVersion {
		return nil, ErrPackageNoticeOutdated
	}
	return plan, nil
}

// ExecutePackageFulfillment 支付成功后发放套餐，状态与租约处理与订阅发货一致。
func (s *PaymentService) ExecutePackageFulfillment(ctx context.Context, oid int64) error {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.Status == OrderStatusCompleted {
		return nil
	}
	if psIsRefundStatus(o.Status) {
		return infraerrors.BadRequest("INVALID_STATUS", "refund-related order cannot fulfill")
	}
	if o.Status != OrderStatusPaid && o.Status != OrderStatusFailed && o.Status != OrderStatusRecharging {
		return infraerrors.BadRequest("INVALID_STATUS", "order cannot fulfill in status "+o.Status)
	}
	if o.PlanID == nil {
		return infraerrors.BadRequest("INVALID_STATUS", "missing package plan")
	}
	lease, err := s.acquirePaymentFulfillmentLease(ctx, o)
	if err != nil {
		return err
	}
	if lease == nil {
		return nil
	}
	if err := s.doPackage(ctx, o, lease); err != nil {
		s.markFailed(ctx, oid, lease, err)
		return err
	}
	return nil
}

func (s *PaymentService) doPackage(ctx context.Context, o *dbent.PaymentOrder, lease *paymentFulfillmentLease) error {
	if s.packageService == nil {
		return fmt.Errorf("package service is unavailable")
	}
	// 按 order_id 幂等：重放的支付回调不会重复发套餐。
	if _, err := s.packageService.FulfillOrder(ctx, PackageFulfillInput{OrderID: o.ID, UserID: o.UserID, PlanID: *o.PlanID}); err != nil {
		return fmt.Errorf("fulfill package: %w", err)
	}
	if err := s.applyAffiliateRebateForOrder(ctx, o); err != nil {
		return err
	}
	return s.markCompleted(ctx, o, lease, "PACKAGE_SUCCESS")
}

// prepPackageDeduct 退款准备：定位订单发出的套餐，退款时作废。
func (s *PaymentService) prepPackageDeduct(ctx context.Context, o *dbent.PaymentOrder, p *RefundPlan, force bool) *RefundResult {
	p.DeductionType = payment.DeductionTypePackage
	var pkg *UserPackage
	var err error
	if s.packageService != nil {
		pkg, err = s.packageService.PackageByOrder(ctx, o.ID)
	}
	if err != nil || pkg == nil {
		if !force {
			return &RefundResult{Success: false, Warning: "cannot find the package issued by this order, use force", RequireForce: true}
		}
		return nil
	}
	if pkg.Status != PackageStatusVoided {
		p.PackageID = pkg.ID
	}
	return nil
}

// applyPackageRefundDeduction 作废退款订单对应的套餐，记录作废前状态以便网关失败时恢复。
func (s *PaymentService) applyPackageRefundDeduction(ctx context.Context, p *RefundPlan) error {
	if p == nil || p.DeductionType != payment.DeductionTypePackage || p.PackageID <= 0 || s.packageService == nil {
		return nil
	}
	prev, err := s.packageService.VoidForRefund(ctx, p.PackageID)
	if err != nil {
		return fmt.Errorf("void package: %w", err)
	}
	p.PackagePrevStatus = prev
	return nil
}

// rollbackPackageRefundDeduction 网关退款失败或转为待确认时恢复已作废的套餐。
func (s *PaymentService) rollbackPackageRefundDeduction(ctx context.Context, p *RefundPlan, gErr error) bool {
	if p == nil || p.DeductionType != payment.DeductionTypePackage || p.PackageID <= 0 || s.packageService == nil {
		return true
	}
	if p.PackagePrevStatus == "" || p.PackagePrevStatus == PackageStatusVoided {
		return true
	}
	if err := s.packageService.RestoreAfterRefund(ctx, p.PackageID, p.PackagePrevStatus); err != nil {
		slog.Error("[CRITICAL] package refund rollback failed", "orderID", p.OrderID, "packageID", p.PackageID, "error", err)
		s.writeAuditLog(ctx, p.OrderID, "REFUND_ROLLBACK_FAILED", "admin", map[string]any{
			"gatewayError": psErrMsg(gErr), "rollbackError": psErrMsg(err), "packageID": p.PackageID,
		})
		return false
	}
	p.PackagePrevStatus = ""
	return true
}

// refreshEnterpriseAfterOrderChange 订单完成或退款完成后，立即让该用户的企业尊享身份缓存失效，
// 下一次请求按新的累计消费重新判定（达标立即享受，退款后不再达标立即取消）。
func (s *PaymentService) refreshEnterpriseAfterOrderChange(userID int64) {
	if s == nil || s.packageService == nil || userID <= 0 {
		return
	}
	s.packageService.invalidateEnterprise(userID)
}
