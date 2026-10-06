package service

// 管理端「用户套餐」总览：跨用户的分页列表与汇总统计。

const (
	// AdminPackagePageSizeDefault / Max 管理端套餐列表的分页大小。
	AdminPackagePageSizeDefault = 20
	AdminPackagePageSizeMax     = 100
	// AdminPackageSalesWindowDays 汇总里「近期售出」的统计窗口（天）。
	AdminPackageSalesWindowDays = 30
)

// IsValidPackageStatus 是否合法的套餐状态。
func IsValidPackageStatus(status string) bool {
	switch status {
	case PackageStatusActive, PackageStatusFrozen, PackageStatusExhausted, PackageStatusExpired, PackageStatusVoided:
		return true
	}
	return false
}

// AdminPackageFilter 管理端套餐列表的筛选条件，零值表示不筛选。
type AdminPackageFilter struct {
	// Keyword 匹配用户邮箱、用户名；纯数字时同时匹配用户 ID。
	Keyword string
	Status  string
	Cycle   string
	GroupID int64
	Page    int
	// PageSize 由 service 层收敛到 [1, AdminPackagePageSizeMax]。
	PageSize int
}

// AdminPackageRow 仓储层返回的一行：套餐本身加上用户、分组名与实付金额。
type AdminPackageRow struct {
	UserPackage
	UserEmail  string
	Username   string
	GroupName  string
	PaidAmount float64
}

// AdminPackageItem 管理端列表项。
type AdminPackageItem struct {
	UserPackage
	UserEmail     string  `json:"user_email"`
	Username      string  `json:"username"`
	GroupName     string  `json:"group_name"`
	PaidAmount    float64 `json:"paid_amount"`
	RemainingUSD  float64 `json:"remaining_usd"`
	FrozenSeconds int64   `json:"frozen_seconds"`
	MaxFreezeDays int     `json:"max_freeze_days"`
}

// AdminPackagePage 分页结果。
type AdminPackagePage struct {
	Items    []AdminPackageItem `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Pages    int                `json:"pages"`
}

// AdminPackageStats 管理端汇总：各状态张数、在途额度与近期售出。
type AdminPackageStats struct {
	Total     int64 `json:"total"`
	Active    int64 `json:"active"`
	Frozen    int64 `json:"frozen"`
	Exhausted int64 `json:"exhausted"`
	Expired   int64 `json:"expired"`
	Voided    int64 `json:"voided"`
	// LiveQuotaUSD / LiveUsedUSD 生效中（active + frozen）套餐的总额度与已用额度。
	LiveQuotaUSD float64 `json:"live_quota_usd"`
	LiveUsedUSD  float64 `json:"live_used_usd"`
	// RecentSold / RecentRevenue 最近 WindowDays 天售出的张数与已完成订单的实收金额。
	RecentSold    int64   `json:"recent_sold"`
	RecentRevenue float64 `json:"recent_revenue"`
	WindowDays    int     `json:"window_days"`
}
