package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// PackageBilling 本次请求按套餐计费的上下文：扣费时先从该分组的套餐扣，
// 并发占用套餐专属槽位（上限为用户的套餐并发）。
type PackageBilling struct {
	GroupID     int64
	Concurrency int
}

// WithPackageBilling 把套餐计费标记写入上下文。
func WithPackageBilling(ctx context.Context, pb *PackageBilling) context.Context {
	if pb == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.PackageBilling, pb)
}

// PackageBillingFromContext 读取套餐计费标记；不是套餐计费时返回 nil。
func PackageBillingFromContext(ctx context.Context) *PackageBilling {
	if ctx == nil {
		return nil
	}
	pb, _ := ctx.Value(ctxkey.PackageBilling).(*PackageBilling)
	return pb
}

// ResolvePackageBilling 鉴权时判断请求是否按套餐计费。
//
// 返回值：
//   - (*PackageBilling, nil)：有可用套餐，按套餐计费；
//   - (nil, nil)：没有可用套餐，走原有余额逻辑；
//   - (nil, ErrPackageFrozenNoBalance)：只有冻结的套餐且余额不足，应拒绝请求。
//
// lowBalance 由调用方按原有余额阈值判断后传入，避免这里重复实现阈值规则。
// 查询失败时降级为余额逻辑，不因套餐模块故障阻断请求。
func (s *PackageService) ResolvePackageBilling(ctx context.Context, user *User, group *Group, lowBalance bool) (*PackageBilling, error) {
	if s == nil || user == nil || !isPackageGroup(group) {
		return nil, nil
	}
	state, err := s.ResolveGroupState(ctx, user.ID, group.ID)
	if err != nil || state == nil {
		return nil, nil
	}
	if state.Usable > 0 {
		concurrency := user.PackageConcurrency
		if concurrency <= 0 {
			concurrency = DefaultPackageConcurrency
		}
		return &PackageBilling{GroupID: group.ID, Concurrency: concurrency}, nil
	}
	if state.Frozen > 0 && lowBalance {
		return nil, ErrPackageFrozenNoBalance
	}
	return nil, nil
}

// usageBillingTypeFor 决定使用记录的计费类型：订阅 > 套餐 > 余额。
func usageBillingTypeFor(ctx context.Context, isSubscriptionBilling bool) int8 {
	if isSubscriptionBilling {
		return BillingTypeSubscription
	}
	if PackageBillingFromContext(ctx) != nil {
		return BillingTypePackage
	}
	return BillingTypeBalance
}

// SetPackageStateInvalidator 注入套餐概况缓存失效函数，扣费让套餐用完时调用。
func (s *BillingCacheService) SetPackageStateInvalidator(fn func(userID, groupID int64)) {
	if s != nil {
		s.packageStateInvalidator = fn
	}
}

// balanceDeductedAfterPackages 本次实际从余额扣掉的金额（总费用减去套餐覆盖部分）。
func balanceDeductedAfterPackages(actualCost float64, result *UsageBillingApplyResult) float64 {
	if result == nil || result.PackageCost <= 0 {
		return actualCost
	}
	return QuantizeUsageBillingAmount(actualCost - result.PackageCost)
}

// notifyPackageExhausted 扣费让套餐用完时失效该用户该分组的套餐概况缓存，
// 让后续请求尽快回到余额准入判断。
func notifyPackageExhausted(ctx context.Context, p *postUsageBillingParams, deps *billingDeps, result *UsageBillingApplyResult) {
	if result == nil || !result.PackageExhausted || p == nil || p.User == nil || deps == nil || deps.billingCacheService == nil {
		return
	}
	pb := PackageBillingFromContext(ctx)
	if pb == nil || deps.billingCacheService.packageStateInvalidator == nil {
		return
	}
	deps.billingCacheService.packageStateInvalidator(p.User.ID, pb.GroupID)
}
