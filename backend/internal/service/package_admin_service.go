package service

import (
	"context"
	"strings"
)

// normalize 收敛分页与筛选参数；非法的状态、周期直接返回 400，避免静默忽略筛选。
func (f AdminPackageFilter) normalize() (AdminPackageFilter, error) {
	f.Keyword = strings.TrimSpace(f.Keyword)
	if f.Status != "" && !IsValidPackageStatus(f.Status) {
		return f, ErrPackageInvalidFilter
	}
	if f.Cycle != "" && !IsValidPackageCycle(f.Cycle) {
		return f, ErrPackageInvalidFilter
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = AdminPackagePageSizeDefault
	}
	if f.PageSize > AdminPackagePageSizeMax {
		f.PageSize = AdminPackagePageSizeMax
	}
	return f, nil
}

// AdminListPackages 管理端分页查看所有用户的套餐，最新购买的在前。
func (s *PackageService) AdminListPackages(ctx context.Context, filter AdminPackageFilter) (*AdminPackagePage, error) {
	f, err := filter.normalize()
	if err != nil {
		return nil, err
	}
	rows, total, err := s.repo.AdminListPackages(ctx, f)
	if err != nil {
		return nil, err
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	items := make([]AdminPackageItem, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		items = append(items, AdminPackageItem{
			UserPackage:   row.UserPackage,
			UserEmail:     row.UserEmail,
			Username:      row.Username,
			GroupName:     row.GroupName,
			PaidAmount:    row.PaidAmount,
			RemainingUSD:  row.UserPackage.RemainingUSD(),
			FrozenSeconds: row.UserPackage.FrozenSecondsAt(now),
			MaxFreezeDays: settings.MaxFreezeDaysFor(row.Cycle),
		})
	}
	pages := int((total + int64(f.PageSize) - 1) / int64(f.PageSize))
	if pages < 1 {
		pages = 1
	}
	return &AdminPackagePage{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize, Pages: pages}, nil
}

// AdminPackageStats 管理端汇总统计。
func (s *PackageService) AdminPackageStats(ctx context.Context) (*AdminPackageStats, error) {
	return s.repo.AdminPackageStats(ctx, s.now().AddDate(0, 0, -AdminPackageSalesWindowDays))
}
