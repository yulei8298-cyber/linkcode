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
		caps       PackageFreezeCaps
	}
	state *PackageGroupState

	userPackages map[int64]*UserPackage
	createCalls  int

	byOrder     []UserPackage
	allPackages []UserPackage

	enterpriseSpent    float64
	enterpriseOverride string
	spentCalls         int

	adminFilter AdminPackageFilter
	adminRows   []AdminPackageRow
	adminTotal  int64
	statsSince  time.Time
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

func (f *packageRepoFake) FreezePackage(_ context.Context, id, userID int64, now time.Time, caps PackageFreezeCaps) (*UserPackage, error) {
	f.frozenCall = &struct {
		id, userID int64
		caps       PackageFreezeCaps
	}{id, userID, caps}
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

func (f *packageGroupRepoFake) GetByIDLite(ctx context.Context, id int64) (*Group, error) {
	return f.GetByID(ctx, id)
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
	require.Equal(t, 7, cur.MaxFreezeDaysWeek, "周卡默认最多冻结 7 天")
	require.Equal(t, 15, cur.MaxFreezeDaysMonth, "月卡默认最多冻结 15 天")

	cur.MaxFreezeDaysWeek = 99
	cur.MaxFreezeDaysMonth = 20
	saved, err := svc.UpdateSettings(ctx, cur)
	require.NoError(t, err)
	require.Equal(t, 1, saved.NoticeVersion, "只改冻结上限不应提升须知版本")
	require.Equal(t, MaxPackageMaxFreezeDay, saved.MaxFreezeDaysWeek, "越界值收敛到上限")
	require.Equal(t, 20, saved.MaxFreezeDaysMonth, "周卡与月卡上限互不影响")

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
	require.Equal(t, PackageFreezeCaps{WeekSeconds: 7 * 86400, MonthSeconds: 15 * 86400}, repo.frozenCall.caps)

	// 关闭总开关后任何日子都不能冻结。
	settings, _ := svc.GetSettings(ctx)
	settings.FreezeEnabled = false
	_, err = svc.UpdateSettings(ctx, settings)
	require.NoError(t, err)
	_, err = svc.Freeze(ctx, 1, 11)
	require.ErrorIs(t, err, ErrPackageFreezeDisabled)
}

func TestPackageFreezeCaps_PerCycle(t *testing.T) {
	settings := DefaultPackageSettings()
	caps := settings.FreezeCaps()
	require.Equal(t, int64(7*86400), caps.For(PackageCycleWeek))
	require.Equal(t, int64(15*86400), caps.For(PackageCycleMonth))
	require.Equal(t, 7, settings.MaxFreezeDaysFor(PackageCycleWeek))
	require.Equal(t, 15, settings.MaxFreezeDaysFor(PackageCycleMonth))

	// 只配了其中一个时，另一个回落默认值；旧版单一字段不再生效。
	partial := PackageSettings{MaxFreezeDaysMonth: 10}
	partial.Normalize()
	require.Equal(t, 7, partial.MaxFreezeDaysWeek)
	require.Equal(t, 10, partial.MaxFreezeDaysMonth)
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

func (f *packageRepoFake) AdminListPackages(_ context.Context, filter AdminPackageFilter) ([]AdminPackageRow, int64, error) {
	f.adminFilter = filter
	return f.adminRows, f.adminTotal, nil
}

func (f *packageRepoFake) AdminPackageStats(_ context.Context, since time.Time) (*AdminPackageStats, error) {
	f.statsSince = since
	return &AdminPackageStats{Total: 3, WindowDays: AdminPackageSalesWindowDays}, nil
}

func TestPackageAdminList_NormalizesFilterAndPaging(t *testing.T) {
	repo := newPackageRepoFake()
	svc := newPackageServiceForTest(repo, time.Now())
	ctx := context.Background()

	_, err := svc.AdminListPackages(ctx, AdminPackageFilter{Keyword: "  alice  ", Page: 0, PageSize: 0})
	require.NoError(t, err)
	require.Equal(t, AdminPackageFilter{Keyword: "alice", Page: 1, PageSize: AdminPackagePageSizeDefault}, repo.adminFilter, "空白关键词去掉，分页取默认值")

	_, err = svc.AdminListPackages(ctx, AdminPackageFilter{Page: 3, PageSize: 5000, Status: PackageStatusFrozen, Cycle: PackageCycleMonth, GroupID: 7})
	require.NoError(t, err)
	require.Equal(t, AdminPackagePageSizeMax, repo.adminFilter.PageSize, "分页大小收敛到上限")
	require.Equal(t, 3, repo.adminFilter.Page)

	for _, bad := range []AdminPackageFilter{{Status: "bogus"}, {Cycle: "year"}} {
		_, err = svc.AdminListPackages(ctx, bad)
		require.ErrorIs(t, err, ErrPackageInvalidFilter, "非法筛选直接拒绝，不静默忽略")
	}
}

func TestPackageAdminList_BuildsItemsWithDerivedFields(t *testing.T) {
	now := packageDate(t, "2026-10-06 12:00")
	frozenAt := now.Add(-36 * time.Hour)
	repo := newPackageRepoFake()
	repo.adminTotal = 41
	repo.adminRows = []AdminPackageRow{
		{
			UserPackage: UserPackage{ID: 1, UserID: 9, Cycle: PackageCycleMonth, QuotaUSD: 40, UsedUSD: 15.5, Status: PackageStatusFrozen, FrozenAt: &frozenAt, FrozenSecondsTotal: 3600},
			UserEmail:   "a@example.com", Username: "alice", GroupName: "GPT-Pro", PaidAmount: 3,
		},
		{UserPackage: UserPackage{ID: 2, UserID: 10, Cycle: PackageCycleWeek, QuotaUSD: 10, UsedUSD: 12, Status: PackageStatusExhausted}},
	}
	svc := newPackageServiceForTest(repo, now)

	page, err := svc.AdminListPackages(context.Background(), AdminPackageFilter{Page: 2, PageSize: 20})
	require.NoError(t, err)
	require.EqualValues(t, 41, page.Total)
	require.Equal(t, 3, page.Pages, "41 条每页 20 条共 3 页")
	require.Len(t, page.Items, 2)

	frozen := page.Items[0]
	require.Equal(t, "a@example.com", frozen.UserEmail)
	require.InDelta(t, 24.5, frozen.RemainingUSD, 1e-9)
	require.Equal(t, int64(3600+36*3600), frozen.FrozenSeconds, "冻结中的套餐计入当前这一段")
	require.Equal(t, 15, frozen.MaxFreezeDays, "月卡取月卡的冻结上限")
	require.Equal(t, 3.0, frozen.PaidAmount)

	used := page.Items[1]
	require.Zero(t, used.RemainingUSD, "用超额度时剩余为 0，不为负")
	require.Equal(t, 7, used.MaxFreezeDays, "周卡取周卡的冻结上限")
}

func TestPackageAdminStats_UsesSalesWindow(t *testing.T) {
	now := packageDate(t, "2026-10-06 12:00")
	repo := newPackageRepoFake()
	svc := newPackageServiceForTest(repo, now)
	stats, err := svc.AdminPackageStats(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 3, stats.Total)
	require.Equal(t, now.AddDate(0, 0, -AdminPackageSalesWindowDays), repo.statsSince)
}

func (f *packageRepoFake) ListPlans(context.Context, *int64, bool) ([]PackagePlan, error) {
	out := make([]PackagePlan, 0, len(f.plans))
	for _, p := range f.plans {
		out = append(out, *p)
	}
	return out, nil
}

func (f *packageRepoFake) ListPackagesByOrderIDs(_ context.Context, orderIDs []int64) ([]UserPackage, error) {
	want := map[int64]bool{}
	for _, id := range orderIDs {
		want[id] = true
	}
	var out []UserPackage
	for _, p := range f.byOrder {
		if p.OrderID != nil && want[*p.OrderID] {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *packageRepoFake) ListUserPackages(context.Context, int64) ([]UserPackage, error) {
	return f.allPackages, nil
}

func TestPackageOrderDetails_ShippedUsesPackageSnapshotOthersFallBackToPlan(t *testing.T) {
	now := packageDate(t, "2026-10-06 12:00")
	frozenAt := now.Add(-48 * time.Hour)
	repo := newPackageRepoFake()
	repo.plans[1] = &PackagePlan{ID: 1, GroupID: 7, Name: "摸鱼周卡(新名)", Cycle: PackageCycleWeek, Tier: 1, QuotaUSD: 10}
	repo.byOrder = []UserPackage{{
		ID: 5, GroupID: 7, OrderID: ptrInt64(101), Name: "摸鱼周卡", Cycle: PackageCycleWeek, Tier: 1, QuotaUSD: 10, UsedUSD: 4,
		Status: PackageStatusFrozen, FrozenAt: &frozenAt, FrozenSecondsTotal: 3600,
	}}
	svc := newPackageServiceForTest(repo, now)

	details, err := svc.OrderDetails(context.Background(), []PackageOrderRef{
		{OrderID: 101, PlanID: 1},  // 已发货
		{OrderID: 102, PlanID: 1},  // 已取消，没有套餐
		{OrderID: 103, PlanID: 99}, // 套餐配置已被删除
	})
	require.NoError(t, err)

	shipped := details[101]
	require.Equal(t, "摸鱼周卡", shipped.PlanName, "已发货的订单取套餐快照里的名字，不受之后改名影响")
	require.Equal(t, "Claude", shipped.GroupName)
	require.NotNil(t, shipped.UserPackage)
	require.Equal(t, PackageStatusFrozen, shipped.UserPackage.Status)
	require.InDelta(t, 6, shipped.UserPackage.RemainingUSD, 1e-9)
	require.Equal(t, int64(3600+48*3600), shipped.UserPackage.FrozenSeconds)
	require.Equal(t, 7, shipped.UserPackage.MaxFreezeDays)

	pending := details[102]
	require.Equal(t, "摸鱼周卡(新名)", pending.PlanName, "没有发货时回落到下单时的套餐配置")
	require.Nil(t, pending.UserPackage)

	require.NotContains(t, details, int64(103), "套餐配置已删除且没有发货，没有详情可展示")

	empty, err := svc.OrderDetails(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestPackageGetMine_KeepsFullHistoryNewestFirst(t *testing.T) {
	now := packageDate(t, "2026-10-06 12:00")
	repo := newPackageRepoFake()
	repo.allPackages = []UserPackage{
		{ID: 1, GroupID: 7, Name: "半年前的卡", Cycle: PackageCycleWeek, QuotaUSD: 10, UsedUSD: 10, Status: PackageStatusExhausted, CreatedAt: now.AddDate(0, -6, 0), ExpiresAt: now.AddDate(0, -6, 7)},
		{ID: 2, GroupID: 7, Name: "上周的卡", Cycle: PackageCycleWeek, QuotaUSD: 10, Status: PackageStatusExpired, CreatedAt: now.AddDate(0, 0, -9), ExpiresAt: now.AddDate(0, 0, -2)},
		{ID: 3, GroupID: 7, Name: "生效中", Cycle: PackageCycleWeek, QuotaUSD: 10, Status: PackageStatusActive, CreatedAt: now.AddDate(0, 0, -1), ExpiresAt: now.AddDate(0, 0, 6)},
	}
	svc := newPackageServiceForTest(repo, now)

	mine, err := svc.GetMine(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, mine.Active, 1)
	require.Len(t, mine.Ended, 2, "半年前结束的套餐也保留，不再只显示 30 天内的")
	require.Equal(t, []string{"上周的卡", "半年前的卡"}, []string{mine.Ended[0].Name, mine.Ended[1].Name}, "历史按购买时间倒序")
}

func (f *packageRepoFake) EnterpriseSpent(context.Context, int64) (float64, error) {
	f.spentCalls++
	return f.enterpriseSpent, nil
}

func (f *packageRepoFake) GetEnterpriseOverride(context.Context, int64) (string, error) {
	return f.enterpriseOverride, nil
}

func (f *packageRepoFake) SetEnterpriseOverride(_ context.Context, _ int64, mode string) error {
	f.enterpriseOverride = mode
	return nil
}

func TestEnterpriseStatus_AutoByThresholdWithManualOverrides(t *testing.T) {
	ctx := context.Background()
	repo := newPackageRepoFake()
	svc := newPackageServiceForTest(repo, time.Now())

	// 默认门槛 3000：差一点不算，达到即算。
	repo.enterpriseSpent = 2999.99
	st, err := svc.EnterpriseStatus(ctx, 1)
	require.NoError(t, err)
	require.False(t, st.Enterprise)
	require.Equal(t, EnterpriseModeAuto, st.Mode)
	require.Equal(t, DefaultEnterpriseThreshold, st.Threshold)

	repo.enterpriseSpent = 3000
	st, _ = svc.EnterpriseStatus(ctx, 1)
	require.True(t, st.Enterprise, "恰好达到门槛就自动获得")

	// 手动关闭优先于累计；手动开通优先于累计。
	repo.enterpriseOverride = EnterpriseModeOff
	st, _ = svc.EnterpriseStatus(ctx, 1)
	require.False(t, st.Enterprise)
	repo.enterpriseSpent = 10
	repo.enterpriseOverride = EnterpriseModeOn
	st, _ = svc.EnterpriseStatus(ctx, 1)
	require.True(t, st.Enterprise)
	require.Equal(t, 10.0, st.Total, "手动开通时仍返回真实累计，便于后台核对")
}

func TestEnterpriseStatus_GlobalSwitchOffHidesEveryone(t *testing.T) {
	ctx := context.Background()
	repo := newPackageRepoFake()
	svc := newPackageServiceForTest(repo, time.Now())
	repo.enterpriseSpent = 99999
	repo.enterpriseOverride = EnterpriseModeOn

	_, err := svc.UpdateEnterpriseSettings(ctx, EnterpriseSettings{Enabled: false, Threshold: 3000})
	require.NoError(t, err)
	st, err := svc.EnterpriseStatus(ctx, 1)
	require.NoError(t, err)
	require.False(t, st.Enterprise, "总开关关闭后，连手动开通的用户也不显示")
	require.False(t, st.Enabled)
}

func TestEnterpriseSettings_ValidationAndThresholdChange(t *testing.T) {
	ctx := context.Background()
	repo := newPackageRepoFake()
	svc := newPackageServiceForTest(repo, time.Now())

	for _, bad := range []float64{0, -1, 1e12} {
		_, err := svc.UpdateEnterpriseSettings(ctx, EnterpriseSettings{Enabled: true, Threshold: bad})
		require.ErrorIs(t, err, ErrEnterpriseInvalid, "门槛 %v 不合法", bad)
	}

	_, err := svc.UpdateEnterpriseSettings(ctx, EnterpriseSettings{Enabled: true, Threshold: 500})
	require.NoError(t, err)
	repo.enterpriseSpent = 600
	st, _ := svc.EnterpriseStatus(ctx, 1)
	require.True(t, st.Enterprise, "门槛调低后立即生效")
	require.Equal(t, 500.0, st.Threshold)
}

func TestSetEnterpriseMode_StoresOnOffAndClearsOnAuto(t *testing.T) {
	ctx := context.Background()
	repo := newPackageRepoFake()
	svc := newPackageServiceForTest(repo, time.Now())

	st, err := svc.SetEnterpriseMode(ctx, 1, EnterpriseModeOn)
	require.NoError(t, err)
	require.True(t, st.Enterprise)
	require.Equal(t, EnterpriseModeOn, repo.enterpriseOverride)

	st, err = svc.SetEnterpriseMode(ctx, 1, EnterpriseModeAuto)
	require.NoError(t, err)
	require.Empty(t, repo.enterpriseOverride, "auto 表示去掉手动覆盖")
	require.Equal(t, EnterpriseModeAuto, st.Mode)

	_, err = svc.SetEnterpriseMode(ctx, 1, "forever")
	require.ErrorIs(t, err, ErrEnterpriseInvalid)
}

func TestApplyEnterpriseRate_TakesLowerOnlyForSameGroup(t *testing.T) {
	ctx := WithEnterpriseRate(context.Background(), &EnterpriseRate{GroupID: 7, Multiplier: 0.28})
	require.Equal(t, 0.28, ApplyEnterpriseRate(ctx, 7, 0.3), "企业倍率更低时用企业倍率")
	require.Equal(t, 0.2, ApplyEnterpriseRate(ctx, 7, 0.2), "个人专属倍率更低时保留个人专属倍率")
	require.Equal(t, 0.3, ApplyEnterpriseRate(ctx, 8, 0.3), "别的分组不受影响")
	require.Equal(t, 0.3, ApplyEnterpriseRate(context.Background(), 7, 0.3), "没有企业倍率（如套餐请求）原样返回")
}

func TestGatewayRateResolvers_ApplyEnterpriseRate(t *testing.T) {
	ctx := WithEnterpriseRate(context.Background(), &EnterpriseRate{GroupID: 7, Multiplier: 0.28})
	require.Equal(t, 0.28, (&GatewayService{}).ResolveUserGroupRateMultiplier(ctx, 1, 7, 0.3))
	require.Equal(t, 0.28, (&OpenAIGatewayService{}).ResolveUserGroupRateMultiplier(ctx, 1, 7, 0.3))
	require.Equal(t, 0.3, (&GatewayService{}).ResolveUserGroupRateMultiplier(context.Background(), 1, 7, 0.3), "套餐请求不带企业倍率，仍按原倍率")
}

func TestEnterpriseSettings_GroupRateValidation(t *testing.T) {
	base := EnterpriseSettings{Enabled: true, Threshold: 3000}
	ok := base
	ok.GroupRates = []EnterpriseGroupRate{{GroupID: 7, Multiplier: 0.28}, {GroupID: 8, Multiplier: 1.2}}
	require.NoError(t, ok.Validate())
	rate, found := ok.GroupRate(7)
	require.True(t, found)
	require.Equal(t, 0.28, rate)

	for name, rates := range map[string][]EnterpriseGroupRate{
		"重复分组":  {{GroupID: 7, Multiplier: 0.28}, {GroupID: 7, Multiplier: 0.2}},
		"倍率为 0": {{GroupID: 7, Multiplier: 0}},
		"倍率过大":  {{GroupID: 7, Multiplier: 101}},
		"分组不合法": {{GroupID: 0, Multiplier: 0.28}},
	} {
		bad := base
		bad.GroupRates = rates
		require.Error(t, bad.Validate(), name)
	}
}

func TestResolveEnterpriseRate_CachesAndInvalidates(t *testing.T) {
	ctx := context.Background()
	repo := newPackageRepoFake()
	svc := newPackageServiceForTest(repo, time.Now())
	repo.enterpriseSpent = 5000
	_, err := svc.UpdateEnterpriseSettings(ctx, EnterpriseSettings{Enabled: true, Threshold: 3000, GroupRates: []EnterpriseGroupRate{{GroupID: 7, Multiplier: 0.28}}})
	require.NoError(t, err)

	require.Nil(t, svc.ResolveEnterpriseRate(ctx, 1, 8), "没配企业倍率的分组直接返回")
	require.Zero(t, repo.spentCalls, "没配置的分组不查用户累计")

	rate := svc.ResolveEnterpriseRate(ctx, 1, 7)
	require.Equal(t, &EnterpriseRate{GroupID: 7, Multiplier: 0.28}, rate)
	svc.ResolveEnterpriseRate(ctx, 1, 7)
	require.Equal(t, 1, repo.spentCalls, "用户身份有缓存，不会每个请求都查库")

	// 管理员手动关闭后立即失效。
	_, err = svc.SetEnterpriseMode(ctx, 1, EnterpriseModeOff)
	require.NoError(t, err)
	require.Nil(t, svc.ResolveEnterpriseRate(ctx, 1, 7))

	// 改配置（去掉分组倍率）也立即生效。
	_, err = svc.SetEnterpriseMode(ctx, 1, EnterpriseModeAuto)
	require.NoError(t, err)
	_, err = svc.UpdateEnterpriseSettings(ctx, EnterpriseSettings{Enabled: true, Threshold: 3000})
	require.NoError(t, err)
	require.Nil(t, svc.ResolveEnterpriseRate(ctx, 1, 7))
	require.Nil(t, svc.EnterpriseGroupRates(ctx, 1))
}

func TestEnterpriseGroupRates_NotEnterpriseOrDisabled(t *testing.T) {
	ctx := context.Background()
	repo := newPackageRepoFake()
	svc := newPackageServiceForTest(repo, time.Now())
	repo.enterpriseSpent = 100
	_, err := svc.UpdateEnterpriseSettings(ctx, EnterpriseSettings{Enabled: true, Threshold: 3000, GroupRates: []EnterpriseGroupRate{{GroupID: 7, Multiplier: 0.28}}})
	require.NoError(t, err)
	require.Nil(t, svc.EnterpriseGroupRates(ctx, 1), "累计不够不享受")

	repo.enterpriseOverride = EnterpriseModeOn
	_, err = svc.UpdateEnterpriseSettings(ctx, EnterpriseSettings{Enabled: false, Threshold: 3000, GroupRates: []EnterpriseGroupRate{{GroupID: 7, Multiplier: 0.28}}})
	require.NoError(t, err)
	require.Nil(t, svc.EnterpriseGroupRates(ctx, 1), "总开关关闭后连手动开通的也不享受")
}

func TestRefreshEnterpriseAfterOrderChange_DropsCachedStatus(t *testing.T) {
	ctx := context.Background()
	repo := newPackageRepoFake()
	pkgSvc := newPackageServiceForTest(repo, time.Now())
	repo.enterpriseSpent = 5000
	_, err := pkgSvc.UpdateEnterpriseSettings(ctx, EnterpriseSettings{Enabled: true, Threshold: 3000, GroupRates: []EnterpriseGroupRate{{GroupID: 7, Multiplier: 0.28}}})
	require.NoError(t, err)
	require.NotNil(t, pkgSvc.ResolveEnterpriseRate(ctx, 1, 7))

	// 全额退款后累计不再达标：订单状态变化后立即失效缓存，不用等 TTL。
	repo.enterpriseSpent = 1000
	require.NotNil(t, pkgSvc.ResolveEnterpriseRate(ctx, 1, 7), "缓存未失效前仍按旧身份")
	(&PaymentService{packageService: pkgSvc}).refreshEnterpriseAfterOrderChange(1)
	require.Nil(t, pkgSvc.ResolveEnterpriseRate(ctx, 1, 7))

	// 没有注入套餐服务时不报错。
	(&PaymentService{}).refreshEnterpriseAfterOrderChange(1)
}
