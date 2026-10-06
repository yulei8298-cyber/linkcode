package service

import (
	"context"
	"log"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// PackageOrderDetail 订单列表里套餐订单的补充信息：买了什么，以及套餐现在的使用情况。
type PackageOrderDetail struct {
	PlanName  string  `json:"plan_name"`
	Cycle     string  `json:"cycle"`
	Tier      int     `json:"tier"`
	QuotaUSD  float64 `json:"quota_usd"`
	GroupName string  `json:"group_name"`
	// UserPackage 订单已发货时对应套餐的当前状态；未付款、已取消、发货失败时为空。
	UserPackage *UserPackageView `json:"user_package,omitempty"`
}

// PackageOrderRef 套餐订单的引用：订单 ID 与下单时的套餐配置 ID。
type PackageOrderRef struct {
	OrderID int64
	PlanID  int64
}

// OrderDetails 批量查询套餐订单的详情，返回 订单 ID → 详情。
// 已发货的订单取套餐本身的快照；没有发货的订单回落到下单时的套餐配置（配置被删除则没有详情）。
func (s *PackageService) OrderDetails(ctx context.Context, refs []PackageOrderRef) (map[int64]*PackageOrderDetail, error) {
	if len(refs) == 0 {
		return map[int64]*PackageOrderDetail{}, nil
	}
	ids := make([]int64, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.OrderID)
	}
	pkgs, err := s.repo.ListPackagesByOrderIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byOrder := make(map[int64]*UserPackage, len(pkgs))
	for i := range pkgs {
		if pkgs[i].OrderID != nil {
			byOrder[*pkgs[i].OrderID] = &pkgs[i]
		}
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	caps := settings.FreezeCaps()
	var plans map[int64]PackagePlan
	groupNames := map[int64]string{}
	now := s.now()
	out := make(map[int64]*PackageOrderDetail, len(refs))
	for _, ref := range refs {
		if pkg := byOrder[ref.OrderID]; pkg != nil {
			frozen := pkg.FrozenSecondsAt(now)
			left := caps.For(pkg.Cycle) - frozen
			if left < 0 {
				left = 0
			}
			out[ref.OrderID] = &PackageOrderDetail{
				PlanName: pkg.Name, Cycle: pkg.Cycle, Tier: pkg.Tier, QuotaUSD: pkg.QuotaUSD,
				GroupName: s.groupName(ctx, groupNames, pkg.GroupID),
				UserPackage: &UserPackageView{
					UserPackage:       *pkg,
					GroupName:         s.groupName(ctx, groupNames, pkg.GroupID),
					RemainingUSD:      pkg.RemainingUSD(),
					MaxFreezeDays:     settings.MaxFreezeDaysFor(pkg.Cycle),
					FrozenSeconds:     frozen,
					FreezeLeftSeconds: left,
				},
			}
			continue
		}
		if plans == nil {
			if plans, err = s.planIndex(ctx); err != nil {
				return nil, err
			}
		}
		if plan, ok := plans[ref.PlanID]; ok {
			out[ref.OrderID] = &PackageOrderDetail{
				PlanName: plan.Name, Cycle: plan.Cycle, Tier: plan.Tier, QuotaUSD: plan.QuotaUSD,
				GroupName: s.groupName(ctx, groupNames, plan.GroupID),
			}
		}
	}
	return out, nil
}

// planIndex 全部套餐配置按 ID 索引（配置总数很少，一次查出即可）。
func (s *PackageService) planIndex(ctx context.Context) (map[int64]PackagePlan, error) {
	plans, err := s.repo.ListPlans(ctx, nil, false)
	if err != nil {
		return nil, err
	}
	idx := make(map[int64]PackagePlan, len(plans))
	for _, p := range plans {
		idx[p.ID] = p
	}
	return idx, nil
}

// PackageOrderDetails 供订单列表使用：挑出套餐订单并补充详情。
// 详情只是辅助信息，查询失败只记录日志，不影响订单列表本身。
func (s *PaymentService) PackageOrderDetails(ctx context.Context, orders []*dbent.PaymentOrder) map[int64]*PackageOrderDetail {
	if s.packageService == nil {
		return nil
	}
	var refs []PackageOrderRef
	for _, o := range orders {
		if o == nil || o.OrderType != payment.OrderTypePackage {
			continue
		}
		ref := PackageOrderRef{OrderID: o.ID}
		if o.PlanID != nil {
			ref.PlanID = *o.PlanID
		}
		refs = append(refs, ref)
	}
	details, err := s.packageService.OrderDetails(ctx, refs)
	if err != nil {
		log.Printf("Warning: load package order details failed: %v", err)
		return nil
	}
	return details
}
