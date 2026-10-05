package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const packageNextFreezableLookahead = 60 // 查找下一个可冻结日的天数上限

// UserPackageView 「我的套餐」中的一张套餐。
type UserPackageView struct {
	UserPackage
	GroupName         string  `json:"group_name"`
	RemainingUSD      float64 `json:"remaining_usd"`
	FrozenSeconds     int64   `json:"frozen_seconds"`      // 截至现在的累计冻结秒数
	FreezeLeftSeconds int64   `json:"freeze_left_seconds"` // 还可冻结的秒数
	DeductOrder       int     `json:"deduct_order"`        // 同分组内的扣费顺序，冻结或已结束为 0
}

// PackageMine 「我的套餐」汇总。
type PackageMine struct {
	Active             []UserPackageView   `json:"active"`
	Ended              []UserPackageView   `json:"ended"`
	PackageConcurrency int                 `json:"package_concurrency"`
	FreezeEnabled      bool                `json:"freeze_enabled"`
	MaxFreezeDays      int                 `json:"max_freeze_days"`
	Today              PackageCalendarDay  `json:"today"`
	NextFreezable      *PackageCalendarDay `json:"next_freezable,omitempty"`
}

// PackageFulfillInput 支付成功后的发货参数。
type PackageFulfillInput struct {
	OrderID int64
	UserID  int64
	PlanID  int64
}

// GetMine 返回用户的套餐列表与冻结相关的当日信息。
func (s *PackageService) GetMine(ctx context.Context, userID int64) (*PackageMine, error) {
	now := s.now()
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	pkgs, err := s.repo.ListUserPackages(ctx, userID, now.Add(-PackageHistoryRetention))
	if err != nil {
		return nil, err
	}
	out := &PackageMine{
		Active:             []UserPackageView{},
		Ended:              []UserPackageView{},
		PackageConcurrency: s.UserPackageConcurrency(ctx, userID),
		FreezeEnabled:      settings.FreezeEnabled,
		MaxFreezeDays:      settings.MaxFreezeDays,
	}

	groupNames := map[int64]string{}
	order := map[int64]int{}
	capSeconds := settings.FreezeCapSeconds()
	for _, p := range pkgs {
		view := UserPackageView{UserPackage: p, RemainingUSD: p.RemainingUSD(), FrozenSeconds: p.FrozenSecondsAt(now)}
		if left := capSeconds - view.FrozenSeconds; left > 0 {
			view.FreezeLeftSeconds = left
		}
		view.GroupName = s.groupName(ctx, groupNames, p.GroupID)
		switch {
		case p.Status == PackageStatusActive && p.ExpiresAt.After(now) && p.UsedUSD < p.QuotaUSD:
			order[p.GroupID]++
			view.DeductOrder = order[p.GroupID]
			out.Active = append(out.Active, view)
		case p.Status == PackageStatusFrozen:
			out.Active = append(out.Active, view)
		default:
			out.Ended = append(out.Ended, view)
		}
	}

	days, err := s.repo.ListFreezeDays(ctx, now, now.AddDate(0, 0, packageNextFreezableLookahead))
	if err != nil {
		return nil, err
	}
	idx := newPackageDayIndex(days)
	out.Today = idx.classify(now, settings.FreezeEnabled)
	for i := 1; settings.FreezeEnabled && i <= packageNextFreezableLookahead; i++ {
		day := idx.classify(now.AddDate(0, 0, i), true)
		if day.Freezable {
			out.NextFreezable = &day
			break
		}
	}
	return out, nil
}

// UserPackageConcurrency 用户的套餐并发；读取失败或未设置时返回默认值。
func (s *PackageService) UserPackageConcurrency(ctx context.Context, userID int64) int {
	if s.userRepo != nil {
		if user, err := s.userRepo.GetByID(ctx, userID); err == nil && user != nil && user.PackageConcurrency > 0 {
			return user.PackageConcurrency
		}
	}
	return DefaultPackageConcurrency
}

// GetCalendar 返回某月（YYYY-MM）每一天能否冻结。
func (s *PackageService) GetCalendar(ctx context.Context, month string) ([]PackageCalendarDay, error) {
	first, err := timezone.ParseInLocation("2006-01", month)
	if err != nil {
		return nil, ErrPackageMonthInvalid
	}
	last := first.AddDate(0, 1, -1)
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	days, err := s.repo.ListFreezeDays(ctx, first, last)
	if err != nil {
		return nil, err
	}
	return buildPackageCalendar(first, last, days, settings.FreezeEnabled), nil
}

// Freeze 冻结一张套餐：功能开启、今天可冻结、套餐使用中且冻结额度未用完。
func (s *PackageService) Freeze(ctx context.Context, userID, packageID int64) (*UserPackage, error) {
	now := s.now()
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.FreezeEnabled {
		return nil, ErrPackageFreezeDisabled
	}
	days, err := s.repo.ListFreezeDays(ctx, now, now)
	if err != nil {
		return nil, err
	}
	if !newPackageDayIndex(days).classify(now, true).Freezable {
		return nil, ErrPackageFreezeNotToday
	}
	pkg, err := s.repo.FreezePackage(ctx, packageID, userID, now, settings.FreezeCapSeconds())
	if err != nil {
		return nil, err
	}
	s.InvalidateGroupState(pkg.UserID, pkg.GroupID)
	return pkg, nil
}

// Unfreeze 用户随时可以解冻自己的套餐。
func (s *PackageService) Unfreeze(ctx context.Context, userID, packageID int64) (*UserPackage, error) {
	return s.unfreezeOne(ctx, packageID, &userID, PackageUnfreezeManual)
}

// AdminUnfreeze 管理员解冻任意套餐。
func (s *PackageService) AdminUnfreeze(ctx context.Context, packageID int64) (*UserPackage, error) {
	return s.unfreezeOne(ctx, packageID, nil, PackageUnfreezeAdmin)
}

func (s *PackageService) unfreezeOne(ctx context.Context, packageID int64, userID *int64, reason string) (*UserPackage, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	pkg, err := s.repo.UnfreezePackage(ctx, PackageUnfreezeInput{
		PackageID: packageID, UserID: userID, Now: s.now(), Reason: reason, CapSeconds: settings.FreezeCapSeconds(),
	})
	if err != nil {
		return nil, err
	}
	s.InvalidateGroupState(pkg.UserID, pkg.GroupID)
	return pkg, nil
}

// AdminListUserPackages 管理端查看某用户的套餐（含 30 天内已结束的）。
func (s *PackageService) AdminListUserPackages(ctx context.Context, userID int64) ([]UserPackage, error) {
	return s.repo.ListUserPackages(ctx, userID, s.now().Add(-PackageHistoryRetention))
}

// AdminVoid 管理员作废一张套餐（剩余额度作废，不退余额）。
func (s *PackageService) AdminVoid(ctx context.Context, packageID int64) (*UserPackage, error) {
	pkg, err := s.repo.VoidPackage(ctx, packageID)
	if err != nil {
		return nil, err
	}
	s.InvalidateGroupState(pkg.UserID, pkg.GroupID)
	return pkg, nil
}

// FulfillOrder 支付成功后发货：按订单幂等地创建一张套餐，额度与名称取当前配置快照。
func (s *PackageService) FulfillOrder(ctx context.Context, in PackageFulfillInput) (*UserPackage, error) {
	plan, err := s.repo.GetPlan(ctx, in.PlanID)
	if err != nil {
		return nil, err
	}
	if err := s.ensurePackageGroup(ctx, plan.GroupID); err != nil {
		return nil, err
	}
	now := s.now()
	orderID := in.OrderID
	pkg, err := s.repo.CreateUserPackage(ctx, &UserPackage{
		UserID:    in.UserID,
		GroupID:   plan.GroupID,
		PlanID:    plan.ID,
		OrderID:   &orderID,
		Name:      plan.Name,
		Cycle:     plan.Cycle,
		Tier:      plan.Tier,
		QuotaUSD:  plan.QuotaUSD,
		StartsAt:  now,
		ExpiresAt: now.AddDate(0, 0, plan.ValidityDays),
	})
	if err != nil {
		return nil, err
	}
	s.InvalidateGroupState(pkg.UserID, pkg.GroupID)
	return pkg, nil
}

// PackageByOrder 返回订单发出的套餐；订单未发货时返回 nil。
func (s *PackageService) PackageByOrder(ctx context.Context, orderID int64) (*UserPackage, error) {
	return s.repo.GetUserPackageByOrderID(ctx, orderID)
}

// VoidForRefund 退款时作废套餐，返回作废前的状态（已作废时原样返回 voided）。
func (s *PackageService) VoidForRefund(ctx context.Context, packageID int64) (string, error) {
	pkg, err := s.repo.GetUserPackage(ctx, packageID)
	if err != nil {
		return "", err
	}
	if pkg.Status == PackageStatusVoided {
		return PackageStatusVoided, nil
	}
	if _, err := s.AdminVoid(ctx, packageID); err != nil {
		return "", err
	}
	return pkg.Status, nil
}

// RestoreAfterRefund 退款网关失败时把作废的套餐恢复为作废前的状态。
func (s *PackageService) RestoreAfterRefund(ctx context.Context, packageID int64, prevStatus string) error {
	pkg, err := s.repo.RestorePackageStatus(ctx, packageID, prevStatus)
	if err != nil {
		return err
	}
	s.InvalidateGroupState(pkg.UserID, pkg.GroupID)
	return nil
}

func (s *PackageService) groupName(ctx context.Context, cache map[int64]string, groupID int64) string {
	if name, ok := cache[groupID]; ok {
		return name
	}
	name := ""
	if g, err := s.groupRepo.GetByIDLite(ctx, groupID); err == nil && g != nil {
		name = g.Name
	}
	cache[groupID] = name
	return name
}
