package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 套餐（周卡 / 月卡）：绑定普通（余额）分组的预付额度，扣费时按到期先后优先于余额。
// 额度与余额同口径（分组倍率折算后的实际扣费金额）。

const (
	PackageCycleWeek  = "week"
	PackageCycleMonth = "month"

	PackageStatusActive    = "active"
	PackageStatusFrozen    = "frozen"
	PackageStatusExhausted = "exhausted"
	PackageStatusExpired   = "expired"
	PackageStatusVoided    = "voided"

	PackageDayKindOff  = "off"  // 放假日，可冻结
	PackageDayKindWork = "work" // 调休补班日，仅用于日历标注

	PackageDaySourceAuto   = "auto"   // 官方节假日同步
	PackageDaySourceManual = "manual" // 后台手动添加

	PackageUnfreezeManual   = "manual"   // 用户手动解冻
	PackageUnfreezeCap      = "cap"      // 累计冻结达到上限自动解冻
	PackageUnfreezeAdmin    = "admin"    // 管理员解冻
	PackageUnfreezeDisabled = "disabled" // 后台关闭冻结功能

	DefaultPackageConcurrency = 5
	MaxPackageConcurrency     = 50
	// 单张套餐累计冻结上限（天），周卡与月卡分开设置。
	DefaultPackageMaxFreezeDayWeek  = 7
	DefaultPackageMaxFreezeDayMonth = 15
	MaxPackageMaxFreezeDay          = 60

	// PackageHistoryRetention 「我的套餐」展示已结束套餐的时间范围。
	PackageHistoryRetention = 30 * 24 * time.Hour
)

// PackageValidityDays 按周期返回有效天数。
func PackageValidityDays(cycle string) int {
	if cycle == PackageCycleMonth {
		return 30
	}
	return 7
}

// IsValidPackageCycle 校验周期取值。
func IsValidPackageCycle(cycle string) bool {
	return cycle == PackageCycleWeek || cycle == PackageCycleMonth
}

// IsValidPackageTier 校验额度档位取值（1x / 2x）。
func IsValidPackageTier(tier int) bool {
	return tier == 1 || tier == 2
}

var (
	ErrPackagePlanNotFound    = infraerrors.NotFound("PACKAGE_PLAN_NOT_FOUND", "套餐不存在或已下架")
	ErrPackageNotFound        = infraerrors.NotFound("PACKAGE_NOT_FOUND", "套餐不存在")
	ErrPackageInvalidPlan     = infraerrors.BadRequest("PACKAGE_INVALID_PLAN", "套餐配置不合法")
	ErrPackageGroupInvalid    = infraerrors.BadRequest("PACKAGE_GROUP_INVALID", "套餐只能绑定可用的普通（余额）分组")
	ErrPackageBaseTierMissing = infraerrors.BadRequest("PACKAGE_BASE_TIER_MISSING", "请先配置同周期的 1x 套餐，2x 额度按 1x 的两倍计算")
	ErrPackageFreezeDisabled  = infraerrors.Forbidden("PACKAGE_FREEZE_DISABLED", "冻结功能暂未开放")
	ErrPackageFreezeNotToday  = infraerrors.BadRequest("PACKAGE_FREEZE_NOT_ALLOWED_TODAY", "只有周末和节假日可以冻结")
	ErrPackageFreezeCapUsed   = infraerrors.BadRequest("PACKAGE_FREEZE_CAP_REACHED", "这张套餐的冻结额度已用完")
	ErrPackageNotFreezable    = infraerrors.BadRequest("PACKAGE_NOT_FREEZABLE", "只有使用中的套餐可以冻结")
	ErrPackageNotFrozen       = infraerrors.BadRequest("PACKAGE_NOT_FROZEN", "套餐未处于冻结状态")
	ErrPackageNoticeOutdated  = infraerrors.BadRequest("PACKAGE_NOTICE_OUTDATED", "购买须知已更新，请重新阅读并同意")
	ErrPackageHolidayInvalid  = infraerrors.BadRequest("PACKAGE_HOLIDAY_INVALID", "名称或日期范围不合法（单次最多 60 天）")
	ErrPackageHolidayNotFound = infraerrors.NotFound("PACKAGE_HOLIDAY_NOT_FOUND", "没有找到这段手动添加的日期")
	ErrPackageMonthInvalid    = infraerrors.BadRequest("PACKAGE_MONTH_INVALID", "月份格式应为 YYYY-MM")
	ErrPackageFrozenNoBalance = infraerrors.Forbidden("PACKAGE_FROZEN", "套餐已冻结，且余额不足。请在「我的套餐」中解冻，或充值后再试。")
)

// PackagePlan 套餐配置。样式（颜色、角标）不入库，由前端按 cycle + tier 映射。
type PackagePlan struct {
	ID           int64     `json:"id"`
	GroupID      int64     `json:"group_id"`
	Name         string    `json:"name"`
	Cycle        string    `json:"cycle"`
	Tier         int       `json:"tier"`
	Price        float64   `json:"price"`
	QuotaUSD     float64   `json:"quota_usd"`
	ValidityDays int       `json:"validity_days"`
	ForSale      bool      `json:"for_sale"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserPackage 用户已购套餐。
type UserPackage struct {
	ID                 int64      `json:"id"`
	UserID             int64      `json:"user_id"`
	GroupID            int64      `json:"group_id"`
	PlanID             int64      `json:"plan_id"`
	OrderID            *int64     `json:"order_id,omitempty"`
	Name               string     `json:"name"`
	Cycle              string     `json:"cycle"`
	Tier               int        `json:"tier"`
	QuotaUSD           float64    `json:"quota_usd"`
	UsedUSD            float64    `json:"used_usd"`
	StartsAt           time.Time  `json:"starts_at"`
	ExpiresAt          time.Time  `json:"expires_at"`
	Status             string     `json:"status"`
	FrozenAt           *time.Time `json:"frozen_at,omitempty"`
	FrozenSecondsTotal int64      `json:"frozen_seconds_total"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// RemainingUSD 剩余额度，不会为负。
func (p *UserPackage) RemainingUSD() float64 {
	if p == nil || p.UsedUSD >= p.QuotaUSD {
		return 0
	}
	return p.QuotaUSD - p.UsedUSD
}

// FrozenSecondsAt 截至 now 的累计冻结秒数（含进行中的一段）。
func (p *UserPackage) FrozenSecondsAt(now time.Time) int64 {
	if p == nil {
		return 0
	}
	total := p.FrozenSecondsTotal
	if p.Status == PackageStatusFrozen && p.FrozenAt != nil && now.After(*p.FrozenAt) {
		total += int64(now.Sub(*p.FrozenAt) / time.Second)
	}
	return total
}

// PackageFreezeDay 可冻结日期（按服务时区的自然日）。
type PackageFreezeDay struct {
	ID     int64     `json:"id"`
	Day    time.Time `json:"day"`
	Name   string    `json:"name"`
	Kind   string    `json:"kind"`
	Source string    `json:"source"`
}

// PackageUnfreezeInput 解冻参数。UserID 非空时只允许解冻自己的套餐。
type PackageUnfreezeInput struct {
	PackageID int64
	UserID    *int64
	Now       time.Time
	Reason    string
	Caps      PackageFreezeCaps
}

// PackageFreezeCaps 单张套餐累计冻结上限（秒），按周期取值。
type PackageFreezeCaps struct {
	WeekSeconds  int64
	MonthSeconds int64
}

// For 返回某周期的上限秒数；周期未知时按周卡处理。
func (c PackageFreezeCaps) For(cycle string) int64 {
	if cycle == PackageCycleMonth {
		return c.MonthSeconds
	}
	return c.WeekSeconds
}

// PackageGroupState 某用户在某分组的套餐概况，用于请求鉴权。
type PackageGroupState struct {
	Usable int // 可扣费（active、未过期、未用完）的张数
	Frozen int // 冻结中的张数
}

// PackageRepository 套餐持久化接口。
type PackageRepository interface {
	// 套餐配置
	ListPlans(ctx context.Context, groupID *int64, onlyForSale bool) ([]PackagePlan, error)
	GetPlan(ctx context.Context, id int64) (*PackagePlan, error)
	GetPlanByCombo(ctx context.Context, groupID int64, cycle string, tier int) (*PackagePlan, error)
	SavePlan(ctx context.Context, plan *PackagePlan) error
	SetPlanQuota(ctx context.Context, id int64, quotaUSD float64) error
	DeletePlan(ctx context.Context, id int64) error

	// 用户套餐
	CreateUserPackage(ctx context.Context, pkg *UserPackage) (*UserPackage, error)
	GetUserPackage(ctx context.Context, id int64) (*UserPackage, error)
	GetUserPackageByOrderID(ctx context.Context, orderID int64) (*UserPackage, error)
	ListUserPackages(ctx context.Context, userID int64, endedSince time.Time) ([]UserPackage, error)
	GetGroupState(ctx context.Context, userID, groupID int64, now time.Time) (*PackageGroupState, error)
	FreezePackage(ctx context.Context, packageID, userID int64, now time.Time, caps PackageFreezeCaps) (*UserPackage, error)
	UnfreezePackage(ctx context.Context, in PackageUnfreezeInput) (*UserPackage, error)
	ListFrozenOverCap(ctx context.Context, now time.Time, caps PackageFreezeCaps) ([]int64, error)
	ListFrozenIDs(ctx context.Context) ([]int64, error)
	ExpirePackages(ctx context.Context, now time.Time) ([]UserPackage, error)
	VoidPackage(ctx context.Context, packageID int64) (*UserPackage, error)
	RestorePackageStatus(ctx context.Context, packageID int64, status string) (*UserPackage, error)

	// 可冻结日期
	ListFreezeDays(ctx context.Context, from, to time.Time) ([]PackageFreezeDay, error)
	ReplaceAutoFreezeDays(ctx context.Context, year int, days []PackageFreezeDay) error
	AddManualFreezeDays(ctx context.Context, name string, from, to time.Time) error
	DeleteManualFreezeDays(ctx context.Context, name string, from, to time.Time) (int, error)
}
