//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ExtendPackage 替身：按 Extendable 判断后顺延到期时间，与真实仓储同口径。
func (f *packageRepoFake) ExtendPackage(_ context.Context, id int64, days int, now time.Time) (*UserPackage, error) {
	f.ensureUserPackages()
	p, ok := f.userPackages[id]
	if !ok {
		return nil, ErrPackageNotFound
	}
	if !p.Extendable(now) {
		return nil, ErrPackageNotExtendable
	}
	p.ExpiresAt = p.ExpiresAt.AddDate(0, 0, days)
	cp := *p
	return &cp, nil
}

func TestUserPackageExtendable(t *testing.T) {
	now := time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC)
	future, past := now.Add(time.Hour), now.Add(-time.Hour)
	cases := []struct {
		name string
		pkg  *UserPackage
		want bool
	}{
		{"使用中", &UserPackage{Status: PackageStatusActive, ExpiresAt: future, QuotaUSD: 10, UsedUSD: 3}, true},
		{"冻结中（到期时间已过也可，冻结期间暂停计时）", &UserPackage{Status: PackageStatusFrozen, ExpiresAt: past, QuotaUSD: 10}, true},
		{"active 但已到期待定时任务处理", &UserPackage{Status: PackageStatusActive, ExpiresAt: past, QuotaUSD: 10}, false},
		{"active 但额度已用完", &UserPackage{Status: PackageStatusActive, ExpiresAt: future, QuotaUSD: 10, UsedUSD: 10}, false},
		{"已过期", &UserPackage{Status: PackageStatusExpired, ExpiresAt: past, QuotaUSD: 10}, false},
		{"已用完", &UserPackage{Status: PackageStatusExhausted, ExpiresAt: future, QuotaUSD: 10, UsedUSD: 10}, false},
		{"已作废", &UserPackage{Status: PackageStatusVoided, ExpiresAt: future, QuotaUSD: 10}, false},
		{"空指针", nil, false},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, tc.pkg.Extendable(now), tc.name)
	}
}

func TestPackageAdminExtend(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC)
	expires := now.AddDate(0, 0, 3)
	repo := newPackageRepoFake()
	repo.userPackages = map[int64]*UserPackage{
		1: {ID: 1, UserID: 9, GroupID: 7, Status: PackageStatusActive, ExpiresAt: expires, QuotaUSD: 10, UsedUSD: 4},
		2: {ID: 2, UserID: 9, GroupID: 7, Status: PackageStatusExpired, ExpiresAt: now.Add(-time.Hour), QuotaUSD: 10},
	}
	svc := newPackageServiceForTest(repo, now)

	for _, days := range []int{0, -3, MaxPackageExtendDays + 1} {
		_, err := svc.AdminExtend(ctx, 1, days)
		require.ErrorIs(t, err, ErrPackageExtendInvalid, "天数 %d 应被拒绝", days)
	}
	require.Equal(t, expires, repo.userPackages[1].ExpiresAt, "非法天数不改动套餐")

	pkg, err := svc.AdminExtend(ctx, 1, 5)
	require.NoError(t, err)
	require.Equal(t, expires.AddDate(0, 0, 5), pkg.ExpiresAt, "到期时间顺延 5 天")
	require.Equal(t, 10.0, pkg.QuotaUSD, "额度不变")
	require.Equal(t, 4.0, pkg.UsedUSD, "已用额度不变")

	_, err = svc.AdminExtend(ctx, 1, MaxPackageExtendDays)
	require.NoError(t, err, "上限天数允许")

	_, err = svc.AdminExtend(ctx, 2, 5)
	require.ErrorIs(t, err, ErrPackageNotExtendable, "已结束的套餐不能延长")

	_, err = svc.AdminExtend(ctx, 404, 5)
	require.ErrorIs(t, err, ErrPackageNotFound)
}

// 延长后立即失效鉴权概况缓存：同实例下一次请求按新到期时间重新判定。
func TestPackageAdminExtend_InvalidatesGroupState(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC)
	repo := newPackageRepoFake()
	repo.userPackages = map[int64]*UserPackage{
		1: {ID: 1, UserID: 9, GroupID: 7, Status: PackageStatusActive, ExpiresAt: now.Add(time.Hour), QuotaUSD: 10},
	}
	repo.state = &PackageGroupState{Usable: 0}
	svc := newPackageServiceForTest(repo, now)

	st, err := svc.ResolveGroupState(ctx, 9, 7)
	require.NoError(t, err)
	require.Equal(t, 0, st.Usable)
	svc.stateCache.Wait()

	repo.state = &PackageGroupState{Usable: 1}
	_, err = svc.AdminExtend(ctx, 1, 2)
	require.NoError(t, err)

	st, err = svc.ResolveGroupState(ctx, 9, 7)
	require.NoError(t, err)
	require.Equal(t, 1, st.Usable, "延长后不应读到旧缓存")
}
