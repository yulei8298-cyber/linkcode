//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// ---------- 测试替身 ----------

type packageRepoFake struct {
	PackageRepository
	plans      map[int64]*PackagePlan
	nextPlanID int64
	days       []PackageFreezeDay
	frozenCall *struct {
		id, userID int64
		cap        int64
	}
	state *PackageGroupState

	userPackages map[int64]*UserPackage
	createCalls  int
}

func newPackageRepoFake() *packageRepoFake {
	return &packageRepoFake{plans: map[int64]*PackagePlan{}}
}

func (f *packageRepoFake) GetPlan(_ context.Context, id int64) (*PackagePlan, error) {
	p, ok := f.plans[id]
	if !ok {
		return nil, ErrPackagePlanNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *packageRepoFake) GetPlanByCombo(_ context.Context, groupID int64, cycle string, tier int) (*PackagePlan, error) {
	for _, p := range f.plans {
		if p.GroupID == groupID && p.Cycle == cycle && p.Tier == tier {
			cp := *p
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *packageRepoFake) SavePlan(_ context.Context, plan *PackagePlan) error {
	if plan.ID == 0 {
		f.nextPlanID++
		plan.ID = f.nextPlanID
	}
	cp := *plan
	f.plans[plan.ID] = &cp
	return nil
}

func (f *packageRepoFake) SetPlanQuota(_ context.Context, id int64, quota float64) error {
	f.plans[id].QuotaUSD = quota
	return nil
}

func (f *packageRepoFake) ListFreezeDays(_ context.Context, _, _ time.Time) ([]PackageFreezeDay, error) {
	return f.days, nil
}

func (f *packageRepoFake) FreezePackage(_ context.Context, id, userID int64, now time.Time, capSeconds int64) (*UserPackage, error) {
	f.frozenCall = &struct {
		id, userID int64
		cap        int64
	}{id, userID, capSeconds}
	return &UserPackage{ID: id, UserID: userID, GroupID: 7, Status: PackageStatusFrozen, FrozenAt: &now}, nil
}

func (f *packageRepoFake) ListFrozenIDs(_ context.Context) ([]int64, error) {
	return nil, nil
}

func (f *packageRepoFake) GetGroupState(_ context.Context, _, _ int64, _ time.Time) (*PackageGroupState, error) {
	cp := *f.state
	return &cp, nil
}

type packageGroupRepoFake struct {
	GroupRepository
	groups map[int64]*Group
}

func (f *packageGroupRepoFake) GetByID(_ context.Context, id int64) (*Group, error) {
	if g, ok := f.groups[id]; ok {
		return g, nil
	}
	return nil, ErrGroupNotFound
}

type packageSettingRepoFake struct {
	SettingRepository
	values map[string]string
}

func (f *packageSettingRepoFake) GetValue(_ context.Context, key string) (string, error) {
	v, ok := f.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}

func (f *packageSettingRepoFake) Set(_ context.Context, key, value string) error {
	f.values[key] = value
	return nil
}

func newPackageServiceForTest(repo *packageRepoFake, now time.Time) *PackageService {
	groups := &packageGroupRepoFake{groups: map[int64]*Group{
		7: {ID: 7, Name: "Claude", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, RateMultiplier: 0.3},
		8: {ID: 8, Name: "Sub", Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
		9: {ID: 9, Name: "Free", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, IsFree: true},
	}}
	svc := NewPackageService(repo, &packageSettingRepoFake{values: map[string]string{}}, groups, nil)
	svc.now = func() time.Time { return now }
	return svc
}

func packageDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := timezone.ParseInLocation("2006-01-02 15:04", s)
	require.NoError(t, err)
	return d
}

// ---------- 套餐配置 ----------

func TestPackageSavePlan_DoubleTierFollowsBaseQuota(t *testing.T) {
	repo := newPackageRepoFake()
	svc := newPackageServiceForTest(repo, time.Now())
	ctx := context.Background()

	_, err := svc.SavePlan(ctx, PackagePlanInput{GroupID: 7, Cycle: PackageCycleWeek, Tier: 2, Name: "爆肝周卡", Price: 190, ForSale: true})
	require.ErrorIs(t, err, ErrPackageBaseTierMissing, "没有 1x 时不能建 2x")

	base, err := svc.SavePlan(ctx, PackagePlanInput{GroupID: 7, Cycle: PackageCycleWeek, Tier: 1, Name: "摸鱼周卡", Price: 95, QuotaUSD: 120, ForSale: true})
	require.NoError(t, err)
	require.Equal(t, 7, base.ValidityDays)

	double, err := svc.SavePlan(ctx, PackagePlanInput{GroupID: 7, Cycle: PackageCycleWeek, Tier: 2, Name: "爆肝周卡", Price: 190, QuotaUSD: 999, ForSale: true})
	require.NoError(t, err)
	require.Equal(t, 240.0, double.QuotaUSD, "2x 额度忽略传入值，固定为 1x 的两倍")

	// 修改 1x 额度后，2x 自动同步。
	_, err = svc.SavePlan(ctx, PackagePlanInput{ID: base.ID, Name: "摸鱼周卡", Price: 95, QuotaUSD: 150, ForSale: true})
	require.NoError(t, err)
	require.Equal(t, 300.0, repo.plans[double.ID].QuotaUSD)
}

func TestPackageSavePlan_SameComboOverwrites(t *testing.T) {
	repo := newPackageRepoFake()
	svc := newPackageServiceForTest(repo, time.Now())
	ctx := context.Background()

	first, err := svc.SavePlan(ctx, PackagePlanInput{GroupID: 7, Cycle: PackageCycleMonth, Tier: 1, Name: "摸鱼月卡", Price: 385, QuotaUSD: 520})
	require.NoError(t, err)
	second, err := svc.SavePlan(ctx, PackagePlanInput{GroupID: 7, Cycle: PackageCycleMonth, Tier: 1, Name: "单核月卡", Price: 399, QuotaUSD: 520})
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Len(t, repo.plans, 1)
	require.Equal(t, "单核月卡", repo.plans[first.ID].Name)
}

func TestPackageSavePlan_RejectsInvalidInputAndGroups(t *testing.T) {
	svc := newPackageServiceForTest(newPackageRepoFake(), time.Now())
	ctx := context.Background()
	cases := []struct {
		name string
		in   PackagePlanInput
		want error
	}{
		{"周期非法", PackagePlanInput{GroupID: 7, Cycle: "day", Tier: 1, Name: "x", Price: 1, QuotaUSD: 1}, ErrPackageInvalidPlan},
		{"档位非法", PackagePlanInput{GroupID: 7, Cycle: PackageCycleWeek, Tier: 3, Name: "x", Price: 1, QuotaUSD: 1}, ErrPackageInvalidPlan},
		{"名称为空", PackagePlanInput{GroupID: 7, Cycle: PackageCycleWeek, Tier: 1, Name: " ", Price: 1, QuotaUSD: 1}, ErrPackageInvalidPlan},
		{"价格为零", PackagePlanInput{GroupID: 7, Cycle: PackageCycleWeek, Tier: 1, Name: "x", Price: 0, QuotaUSD: 1}, ErrPackageInvalidPlan},
		{"额度为零", PackagePlanInput{GroupID: 7, Cycle: PackageCycleWeek, Tier: 1, Name: "x", Price: 1, QuotaUSD: 0}, ErrPackageInvalidPlan},
		{"订阅分组", PackagePlanInput{GroupID: 8, Cycle: PackageCycleWeek, Tier: 1, Name: "x", Price: 1, QuotaUSD: 1}, ErrPackageGroupInvalid},
		{"免费分组", PackagePlanInput{GroupID: 9, Cycle: PackageCycleWeek, Tier: 1, Name: "x", Price: 1, QuotaUSD: 1}, ErrPackageGroupInvalid},
		{"分组不存在", PackagePlanInput{GroupID: 99, Cycle: PackageCycleWeek, Tier: 1, Name: "x", Price: 1, QuotaUSD: 1}, ErrPackageGroupInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.SavePlan(ctx, tc.in)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

// ---------- 设置 ----------

func TestPackageUpdateSettings_NoticeVersionBumpsOnlyWhenTextChanges(t *testing.T) {
	svc := newPackageServiceForTest(newPackageRepoFake(), time.Now())
	ctx := context.Background()

	cur, err := svc.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, cur.NoticeVersion)
	require.Equal(t, DefaultPackageMaxFreezeDay, cur.MaxFreezeDays)

	cur.MaxFreezeDays = 99
	saved, err := svc.UpdateSettings(ctx, cur)
	require.NoError(t, err)
	require.Equal(t, 1, saved.NoticeVersion, "只改冻结上限不应提升须知版本")
	require.Equal(t, MaxPackageMaxFreezeDay, saved.MaxFreezeDays, "越界值收敛到上限")

	saved.NoticeText = "立即生效：测试"
	saved, err = svc.UpdateSettings(ctx, saved)
	require.NoError(t, err)
	require.Equal(t, 2, saved.NoticeVersion)

	saved.HolidaySourceURL = "http://example.com/{year}.json"
	_, err = svc.UpdateSettings(ctx, saved)
	require.Error(t, err, "节假日地址必须是 https")
}

// ---------- 冻结 ----------

func TestPackageFreeze_RespectsCalendarAndSwitch(t *testing.T) {
	ctx := context.Background()
	repo := newPackageRepoFake()

	// 2026-10-14 周三：工作日不可冻结。
	svc := newPackageServiceForTest(repo, packageDate(t, "2026-10-14 12:00"))
	_, err := svc.Freeze(ctx, 1, 11)
	require.ErrorIs(t, err, ErrPackageFreezeNotToday)
	require.Nil(t, repo.frozenCall)

	// 同一天被后台设为自定义可冻结日后可以冻结，并带上 7 天上限。
	repo.days = []PackageFreezeDay{{Day: packageDate(t, "2026-10-14 00:00"), Name: "平台活动日", Kind: PackageDayKindOff, Source: PackageDaySourceManual}}
	_, err = svc.Freeze(ctx, 1, 11)
	require.NoError(t, err)
	require.Equal(t, int64(7*86400), repo.frozenCall.cap)

	// 关闭总开关后任何日子都不能冻结。
	settings, _ := svc.GetSettings(ctx)
	settings.FreezeEnabled = false
	_, err = svc.UpdateSettings(ctx, settings)
	require.NoError(t, err)
	_, err = svc.Freeze(ctx, 1, 11)
	require.ErrorIs(t, err, ErrPackageFreezeDisabled)
}

func TestPackageCalendar_ClassifiesWeekendHolidayAndMakeup(t *testing.T) {
	days := []PackageFreezeDay{
		{Day: packageDate(t, "2026-10-01 00:00"), Name: "国庆节", Kind: PackageDayKindOff, Source: PackageDaySourceAuto},
		{Day: packageDate(t, "2026-10-10 00:00"), Name: "国庆节", Kind: PackageDayKindWork, Source: PackageDaySourceAuto},
	}
	cal := buildPackageCalendar(packageDate(t, "2026-10-01 00:00"), packageDate(t, "2026-10-14 00:00"), days, true)
	byDate := map[string]PackageCalendarDay{}
	for _, d := range cal {
		byDate[d.Date] = d
	}
	require.Len(t, cal, 14)
	require.Equal(t, PackageCalendarHoliday, byDate["2026-10-01"].Kind)
	require.Equal(t, "国庆节", byDate["2026-10-01"].Label)
	require.True(t, byDate["2026-10-01"].Freezable)
	require.Equal(t, PackageCalendarMakeup, byDate["2026-10-10"].Kind, "调休补班的周六仍可冻结")
	require.True(t, byDate["2026-10-10"].Freezable)
	require.Equal(t, PackageCalendarWeekend, byDate["2026-10-11"].Kind)
	require.Equal(t, PackageCalendarNone, byDate["2026-10-14"].Kind)
	require.False(t, byDate["2026-10-14"].Freezable)

	disabled := buildPackageCalendar(packageDate(t, "2026-10-01 00:00"), packageDate(t, "2026-10-01 00:00"), days, false)
	require.False(t, disabled[0].Freezable, "总开关关闭时节假日也不可冻结")
}

func TestUserPackage_FrozenSecondsAndRemaining(t *testing.T) {
	frozenAt := packageDate(t, "2026-10-03 08:00")
	p := &UserPackage{QuotaUSD: 10, UsedUSD: 12, Status: PackageStatusFrozen, FrozenAt: &frozenAt, FrozenSecondsTotal: 3600}
	require.Zero(t, p.RemainingUSD(), "超额使用时剩余额度不为负")
	require.Equal(t, int64(3600+2*3600), p.FrozenSecondsAt(frozenAt.Add(2*time.Hour)))
	p.Status = PackageStatusActive
	require.Equal(t, int64(3600), p.FrozenSecondsAt(frozenAt.Add(2*time.Hour)), "未冻结时只计已结束的冻结段")
}

// ---------- 鉴权概况缓存 ----------

func TestPackageResolveGroupState_CachesAndInvalidates(t *testing.T) {
	repo := newPackageRepoFake()
	repo.state = &PackageGroupState{Usable: 1}
	svc := newPackageServiceForTest(repo, time.Now())
	ctx := context.Background()

	st, err := svc.ResolveGroupState(ctx, 1, 7)
	require.NoError(t, err)
	require.Equal(t, 1, st.Usable)
	svc.stateCache.Wait()

	repo.state = &PackageGroupState{Frozen: 1}
	st, _ = svc.ResolveGroupState(ctx, 1, 7)
	require.Equal(t, 1, st.Usable, "缓存期内返回旧值")

	svc.InvalidateGroupState(1, 7)
	st, _ = svc.ResolveGroupState(ctx, 1, 7)
	require.Equal(t, 0, st.Usable)
	require.Equal(t, 1, st.Frozen)
}

// ---------- 节假日同步 ----------

func TestParseHolidayCN_KeepsOffAndMakeupDaysOfYear(t *testing.T) {
	body := []byte(`{"year":2026,"days":[
		{"name":"国庆节","date":"2026-10-01","isOffDay":true},
		{"name":"国庆节","date":"2026-10-10","isOffDay":false},
		{"name":"元旦","date":"2027-01-01","isOffDay":true}]}`)
	days, err := parseHolidayCN(2026, body)
	require.NoError(t, err)
	require.Len(t, days, 2, "跨年的日期不属于该年文件")
	require.Equal(t, PackageDayKindOff, days[0].Kind)
	require.Equal(t, PackageDayKindWork, days[1].Kind)
	require.Equal(t, PackageDaySourceAuto, days[1].Source)

	_, err = parseHolidayCN(2027, body)
	require.Error(t, err, "年份不符时拒绝")
}

func TestPackageHolidaySyncDue(t *testing.T) {
	now := packageDate(t, "2026-10-04 03:30")
	require.True(t, packageHolidaySyncDue(nil, now), "从未同步立即执行")
	yesterday := packageDate(t, "2026-10-03 03:10")
	require.True(t, packageHolidaySyncDue(&yesterday, now))
	today := packageDate(t, "2026-10-04 03:05")
	require.False(t, packageHolidaySyncDue(&today, now))
	early := packageDate(t, "2026-10-04 02:00")
	require.False(t, packageHolidaySyncDue(&yesterday, early), "凌晨同步时刻之前不执行")
}
