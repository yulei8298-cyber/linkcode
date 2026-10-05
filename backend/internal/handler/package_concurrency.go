package handler

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// PackageConcurrencyError 套餐并发已满（套餐请求不排队）。
type PackageConcurrencyError struct {
	Limit int
}

func (e *PackageConcurrencyError) Error() string {
	return fmt.Sprintf("套餐并发已达上限（%d 个），请等待进行中的请求完成后重试。如需更高并发请联系客服。", e.Limit)
}

func (h *ConcurrencyHelper) tryAcquirePackageSlot(ctx context.Context, userID int64, packageBilling *service.PackageBilling) (func(), bool, error) {
	result, err := h.concurrencyService.AcquirePackageUserSlot(ctx, userID, packageBilling.Concurrency)
	if err != nil {
		return nil, false, err
	}
	if !result.Acquired {
		return nil, false, nil
	}
	return result.ReleaseFunc, true, nil
}
