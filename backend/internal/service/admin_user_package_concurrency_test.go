//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminService_UpdateUser_PackageConcurrency(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", Role: RoleUser, Concurrency: 3, PackageConcurrency: 5}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{userRepo: repo, redeemCodeRepo: &redeemRepoStub{}, authCacheInvalidator: invalidator}

	value := 10
	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{PackageConcurrency: &value})
	require.NoError(t, err)
	require.Equal(t, 10, updated.PackageConcurrency)
	require.Equal(t, 3, updated.Concurrency, "余额并发不受影响")
	require.Equal(t, []int64{42}, invalidator.userIDs, "套餐并发变更应失效认证缓存，鉴权立即按新上限占槽")

	for _, bad := range []int{0, MaxPackageConcurrency + 1} {
		v := bad
		_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{PackageConcurrency: &v})
		require.Error(t, err, "套餐并发越界应拒绝：%d", bad)
	}
}

func TestGroupPackageHolidayRanges(t *testing.T) {
	day := func(s string) PackageFreezeDay {
		return PackageFreezeDay{Day: packageDate(t, s+" 00:00"), Name: "国庆节", Kind: PackageDayKindOff, Source: PackageDaySourceAuto}
	}
	days := []PackageFreezeDay{day("2026-10-01"), day("2026-10-02"), day("2026-10-03")}
	work := day("2026-10-10")
	work.Kind = PackageDayKindWork
	days = append(days, work)
	manual := day("2026-11-18")
	manual.Name, manual.Source = "平台周年庆", PackageDaySourceManual
	days = append(days, manual)

	ranges := groupPackageHolidayRanges(days)
	require.Len(t, ranges, 3)
	require.Equal(t, PackageHolidayRange{Name: "国庆节", Start: "2026-10-01", End: "2026-10-03", Days: 3, Kind: PackageDayKindOff, Source: PackageDaySourceAuto}, ranges[0])
	require.Equal(t, PackageDayKindWork, ranges[1].Kind, "调休补班单独成段")
	require.Equal(t, PackageDaySourceManual, ranges[2].Source)
}

func TestValidatePackageHolidayInput(t *testing.T) {
	_, _, _, err := validatePackageHolidayInput("平台活动日", "2026-11-18", "2026-11-18")
	require.NoError(t, err)
	for _, tc := range [][3]string{
		{" ", "2026-11-18", "2026-11-18"},
		{"活动", "2026-11-19", "2026-11-18"},
		{"活动", "2026-01-01", "2026-12-31"},
		{"活动", "2026/11/18", "2026-11-18"},
	} {
		_, _, _, err := validatePackageHolidayInput(tc[0], tc[1], tc[2])
		require.ErrorIs(t, err, ErrPackageHolidayInvalid, "%v", tc)
	}
}
