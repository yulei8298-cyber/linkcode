package service

import (
	"context"
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

const packagePlanNameMaxRunes = 50

// PackagePlanInput 管理端保存套餐的参数。group / cycle / tier 决定唯一组合；
// tier=2 时 QuotaUSD 被忽略，固定为同组同周期 1x 的两倍。
type PackagePlanInput struct {
	ID       int64   `json:"id"`
	GroupID  int64   `json:"group_id"`
	Cycle    string  `json:"cycle"`
	Tier     int     `json:"tier"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	QuotaUSD float64 `json:"quota_usd"`
	ForSale  bool    `json:"for_sale"`
}

// PackageShopGroup 套餐商店里的一个分组及其在售套餐。
type PackageShopGroup struct {
	GroupID        int64         `json:"group_id"`
	GroupName      string        `json:"group_name"`
	RateMultiplier float64       `json:"rate_multiplier"`
	Plans          []PackagePlan `json:"plans"`
}

// ListPlans 管理端列出套餐配置（groupID 为空时列出全部）。
func (s *PackageService) ListPlans(ctx context.Context, groupID *int64) ([]PackagePlan, error) {
	return s.repo.ListPlans(ctx, groupID, false)
}

// SavePlan 新建或更新套餐。同一组合已存在时覆盖原套餐（与后台「保存会覆盖」一致）。
func (s *PackageService) SavePlan(ctx context.Context, in PackagePlanInput) (*PackagePlan, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.ID > 0 {
		existing, err := s.repo.GetPlan(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		// 组合不可修改：换周期或档位等于另一个套餐，应新建。
		in.GroupID, in.Cycle, in.Tier = existing.GroupID, existing.Cycle, existing.Tier
	}
	if !IsValidPackageCycle(in.Cycle) || !IsValidPackageTier(in.Tier) || in.Name == "" ||
		utf8.RuneCountInString(in.Name) > packagePlanNameMaxRunes || !isPositiveAmount(in.Price) {
		return nil, ErrPackageInvalidPlan
	}
	if err := s.ensurePackageGroup(ctx, in.GroupID); err != nil {
		return nil, err
	}
	if in.ID == 0 {
		existing, err := s.repo.GetPlanByCombo(ctx, in.GroupID, in.Cycle, in.Tier)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			in.ID = existing.ID
		}
	}

	quota, err := s.resolvePlanQuota(ctx, in)
	if err != nil {
		return nil, err
	}
	plan := &PackagePlan{
		ID:           in.ID,
		GroupID:      in.GroupID,
		Name:         in.Name,
		Cycle:        in.Cycle,
		Tier:         in.Tier,
		Price:        roundCents(in.Price),
		QuotaUSD:     quota,
		ValidityDays: PackageValidityDays(in.Cycle),
		ForSale:      in.ForSale,
	}
	if err := s.repo.SavePlan(ctx, plan); err != nil {
		return nil, err
	}
	if in.Tier == 1 {
		if err := s.syncDoubleTierQuota(ctx, plan); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

// DeletePlan 删除套餐配置；已购套餐保存了快照，不受影响。
func (s *PackageService) DeletePlan(ctx context.Context, id int64) error {
	return s.repo.DeletePlan(ctx, id)
}

// ListShop 列出可购买的分组与在售套餐，跳过已停用或不再是普通分组的。
func (s *PackageService) ListShop(ctx context.Context) ([]PackageShopGroup, error) {
	plans, err := s.repo.ListPlans(ctx, nil, true)
	if err != nil {
		return nil, err
	}
	byGroup := map[int64]*PackageShopGroup{}
	var order []int64
	for _, p := range plans {
		g, ok := byGroup[p.GroupID]
		if !ok {
			group, err := s.groupRepo.GetByID(ctx, p.GroupID)
			if err != nil || !isPackageGroup(group) {
				byGroup[p.GroupID] = nil
				continue
			}
			g = &PackageShopGroup{GroupID: group.ID, GroupName: group.Name, RateMultiplier: group.RateMultiplier}
			byGroup[p.GroupID] = g
			order = append(order, p.GroupID)
		}
		if g != nil {
			g.Plans = append(g.Plans, p)
		}
	}
	out := make([]PackageShopGroup, 0, len(order))
	for _, id := range order {
		g := byGroup[id]
		sort.SliceStable(g.Plans, func(i, j int) bool { return packagePlanRank(g.Plans[i]) < packagePlanRank(g.Plans[j]) })
		out = append(out, *g)
	}
	return out, nil
}

// GetPlanForPurchase 下单校验：套餐在售且分组仍是可用的普通分组。
func (s *PackageService) GetPlanForPurchase(ctx context.Context, planID int64) (*PackagePlan, error) {
	plan, err := s.repo.GetPlan(ctx, planID)
	if err != nil || plan == nil || !plan.ForSale {
		return nil, ErrPackagePlanNotFound
	}
	if err := s.ensurePackageGroup(ctx, plan.GroupID); err != nil {
		return nil, err
	}
	return plan, nil
}

func (s *PackageService) resolvePlanQuota(ctx context.Context, in PackagePlanInput) (float64, error) {
	if in.Tier == 1 {
		if !isPositiveAmount(in.QuotaUSD) {
			return 0, ErrPackageInvalidPlan
		}
		return QuantizeUsageBillingAmount(in.QuotaUSD), nil
	}
	base, err := s.repo.GetPlanByCombo(ctx, in.GroupID, in.Cycle, 1)
	if err != nil {
		return 0, err
	}
	if base == nil {
		return 0, ErrPackageBaseTierMissing
	}
	return QuantizeUsageBillingAmount(base.QuotaUSD * 2), nil
}

// syncDoubleTierQuota 1x 额度变化后，把同组同周期 2x 的额度同步为两倍。
func (s *PackageService) syncDoubleTierQuota(ctx context.Context, base *PackagePlan) error {
	twin, err := s.repo.GetPlanByCombo(ctx, base.GroupID, base.Cycle, 2)
	if err != nil || twin == nil {
		return err
	}
	want := QuantizeUsageBillingAmount(base.QuotaUSD * 2)
	if twin.QuotaUSD == want {
		return nil
	}
	return s.repo.SetPlanQuota(ctx, twin.ID, want)
}

func (s *PackageService) ensurePackageGroup(ctx context.Context, groupID int64) error {
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil || !isPackageGroup(group) {
		return ErrPackageGroupInvalid
	}
	return nil
}

// isPackageGroup 套餐只能绑定启用中的普通（余额）分组：订阅分组与每日免费分组不按余额计费。
func isPackageGroup(g *Group) bool {
	return g != nil && g.IsActive() && !g.IsSubscriptionType() && !g.IsFree
}

// packagePlanRank 商店展示顺序：周卡在前，同周期 1x 在前。
func packagePlanRank(p PackagePlan) int {
	rank := p.Tier
	if p.Cycle == PackageCycleMonth {
		rank += 10
	}
	return rank
}

func isPositiveAmount(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func roundCents(v float64) float64 {
	return math.Round(v*100) / 100
}
