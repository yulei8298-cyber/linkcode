//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// ---------- 用户套餐替身（扩展 packageRepoFake） ----------

func (f *packageRepoFake) ensureUserPackages() {
	if f.userPackages == nil {
		f.userPackages = map[int64]*UserPackage{}
	}
}

func (f *packageRepoFake) CreateUserPackage(_ context.Context, pkg *UserPackage) (*UserPackage, error) {
	f.ensureUserPackages()
	for _, p := range f.userPackages {
		if p.OrderID != nil && pkg.OrderID != nil && *p.OrderID == *pkg.OrderID {
			cp := *p
			return &cp, nil
		}
	}
	f.createCalls++
	cp := *pkg
	cp.ID = int64(len(f.userPackages) + 1)
	cp.Status = PackageStatusActive
	f.userPackages[cp.ID] = &cp
	out := cp
	return &out, nil
}

func (f *packageRepoFake) GetUserPackage(_ context.Context, id int64) (*UserPackage, error) {
	f.ensureUserPackages()
	p, ok := f.userPackages[id]
	if !ok {
		return nil, ErrPackageNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *packageRepoFake) GetUserPackageByOrderID(_ context.Context, orderID int64) (*UserPackage, error) {
	f.ensureUserPackages()
	for _, p := range f.userPackages {
		if p.OrderID != nil && *p.OrderID == orderID {
			cp := *p
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *packageRepoFake) VoidPackage(_ context.Context, id int64) (*UserPackage, error) {
	p := f.userPackages[id]
	p.Status = PackageStatusVoided
	cp := *p
	return &cp, nil
}

func (f *packageRepoFake) RestorePackageStatus(_ context.Context, id int64, status string) (*UserPackage, error) {
	p := f.userPackages[id]
	if p.Status != PackageStatusVoided {
		return nil, ErrPackageNotFound
	}
	p.Status = status
	cp := *p
	return &cp, nil
}

// ---------- 下单校验 ----------

func newPackagePaymentFixture(t *testing.T) (*PaymentService, *packageRepoFake, *PackageService) {
	t.Helper()
	repo := newPackageRepoFake()
	pkgSvc := newPackageServiceForTest(repo, time.Now())
	_, err := pkgSvc.SavePlan(context.Background(), PackagePlanInput{GroupID: 7, Cycle: PackageCycleWeek, Tier: 1, Name: "摸鱼周卡", Price: 95, QuotaUSD: 120, ForSale: true})
	require.NoError(t, err)
	svc := &PaymentService{}
	svc.SetPackageService(pkgSvc)
	return svc, repo, pkgSvc
}

func TestValidatePackageOrder(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newPackagePaymentFixture(t)
	planID := int64(1)

	plan, err := svc.validatePackageOrder(ctx, CreateOrderRequest{OrderType: payment.OrderTypeBalance})
	require.NoError(t, err)
	require.Nil(t, plan, "非套餐订单不做套餐校验")

	_, err = svc.validatePackageOrder(ctx, CreateOrderRequest{OrderType: payment.OrderTypePackage, PlanID: planID})
	require.ErrorIs(t, err, ErrPackageNoticeOutdated, "未同意须知（版本 0）不能下单")

	plan, err = svc.validatePackageOrder(ctx, CreateOrderRequest{OrderType: payment.OrderTypePackage, PlanID: planID, PackageNoticeVersion: 1})
	require.NoError(t, err)
	require.Equal(t, 95.0, plan.Price)

	repo.plans[planID].ForSale = false
	_, err = svc.validatePackageOrder(ctx, CreateOrderRequest{OrderType: payment.OrderTypePackage, PlanID: planID, PackageNoticeVersion: 1})
	require.ErrorIs(t, err, ErrPackagePlanNotFound, "下架的套餐不能购买")

	_, err = (&PaymentService{}).validatePackageOrder(ctx, CreateOrderRequest{OrderType: payment.OrderTypePackage, PlanID: planID})
	require.Error(t, err, "未注入套餐服务时拒绝")
}

func TestValidateOrderInput_PackageSkipsAmountCheck(t *testing.T) {
	svc := &PaymentService{}
	plan, err := svc.validateOrderInput(context.Background(), CreateOrderRequest{OrderType: payment.OrderTypePackage}, &PaymentConfig{MinAmount: 10})
	require.NoError(t, err, "套餐订单金额取套餐价，不受充值金额范围限制")
	require.Nil(t, plan)
}

// ---------- 发货 ----------

func TestExecutePackageFulfillment_IssuesOnceAndCompletes(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	svc, repo, _ := newPackagePaymentFixture(t)
	svc.entClient = client

	user, err := client.User.Create().SetEmail("package-buyer@example.com").SetPasswordHash("hash").SetUsername("package-buyer").Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
		SetAmount(95).SetPayAmount(95).SetFeeRate(0).
		SetRechargeCode("PAY-PKG-1").SetOutTradeNo("sub2_package_1").
		SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("trade-pkg-1").
		SetOrderType(payment.OrderTypePackage).SetPlanID(1).
		SetStatus(OrderStatusPaid).SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	// 走总分派入口，确认套餐订单不会被当成余额充值发货。
	require.NoError(t, svc.executeFulfillment(ctx, order.ID))
	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.Equal(t, 1, repo.createCalls)

	pkg, err := repo.GetUserPackageByOrderID(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, user.ID, pkg.UserID)
	require.Equal(t, int64(7), pkg.GroupID)
	require.Equal(t, "摸鱼周卡", pkg.Name)
	require.Equal(t, 120.0, pkg.QuotaUSD)
	require.WithinDuration(t, pkg.StartsAt.AddDate(0, 0, 7), pkg.ExpiresAt, time.Second)

	// 回调重放：已完成的订单不重复发货。
	require.NoError(t, svc.ExecutePackageFulfillment(ctx, order.ID))
	require.Equal(t, 1, repo.createCalls)
}

// ---------- 退款 ----------

func TestPackageRefundVoidsAndRollbackRestores(t *testing.T) {
	ctx := context.Background()
	svc, repo, pkgSvc := newPackagePaymentFixture(t)
	orderID := int64(55)
	issued, err := pkgSvc.FulfillOrder(ctx, PackageFulfillInput{OrderID: orderID, UserID: 3, PlanID: 1})
	require.NoError(t, err)
	repo.userPackages[issued.ID].Status = PackageStatusFrozen

	plan := &RefundPlan{OrderID: orderID}
	require.Nil(t, svc.prepPackageDeduct(ctx, &dbent.PaymentOrder{ID: orderID}, plan, false))
	require.Equal(t, payment.DeductionTypePackage, plan.DeductionType)
	require.Equal(t, issued.ID, plan.PackageID)

	require.NoError(t, svc.applyPackageRefundDeduction(ctx, plan))
	require.Equal(t, PackageStatusVoided, repo.userPackages[issued.ID].Status)
	require.Equal(t, PackageStatusFrozen, plan.PackagePrevStatus)

	require.True(t, svc.rollbackPackageRefundDeduction(ctx, plan, nil))
	require.Equal(t, PackageStatusFrozen, repo.userPackages[issued.ID].Status, "网关失败时恢复为作废前的冻结状态")

	missing := &RefundPlan{OrderID: 99}
	res := svc.prepPackageDeduct(ctx, &dbent.PaymentOrder{ID: 99}, missing, false)
	require.NotNil(t, res)
	require.True(t, res.RequireForce, "找不到套餐时需要强制退款")
	require.Nil(t, svc.prepPackageDeduct(ctx, &dbent.PaymentOrder{ID: 99}, missing, true))
}
