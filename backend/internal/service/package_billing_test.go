//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUsageBillingTypeFor(t *testing.T) {
	ctx := context.Background()
	pkgCtx := WithPackageBilling(ctx, &PackageBilling{GroupID: 7, Concurrency: 5})

	require.Equal(t, BillingTypeBalance, usageBillingTypeFor(ctx, false))
	require.Equal(t, BillingTypePackage, usageBillingTypeFor(pkgCtx, false))
	require.Equal(t, BillingTypeSubscription, usageBillingTypeFor(pkgCtx, true), "订阅优先于套餐")
}

func TestBalanceDeductedAfterPackages(t *testing.T) {
	require.Equal(t, 1.5, balanceDeductedAfterPackages(1.5, nil))
	require.Equal(t, 1.5, balanceDeductedAfterPackages(1.5, &UsageBillingApplyResult{}))
	require.InDelta(t, 0.5, balanceDeductedAfterPackages(1.5, &UsageBillingApplyResult{PackageCost: 1}), 1e-12)
	require.Zero(t, balanceDeductedAfterPackages(1.5, &UsageBillingApplyResult{PackageCost: 1.5}), "套餐全覆盖时余额缓存不变")
}

func TestBuildUsageBillingCommand_PackageBillingCarriesGroup(t *testing.T) {
	groupID := int64(7)
	p := &postUsageBillingParams{
		Cost:    &CostBreakdown{TotalCost: 3, ActualCost: 0.9},
		User:    &User{ID: 10},
		APIKey:  &APIKey{ID: 20, GroupID: &groupID},
		Account: &Account{ID: 30},
	}

	cmd := buildUsageBillingCommand("req-1", &UsageLog{BillingType: BillingTypePackage}, p)
	require.NotNil(t, cmd.PackageGroupID)
	require.Equal(t, groupID, *cmd.PackageGroupID)
	require.InDelta(t, 0.9, cmd.BalanceCost, 1e-12, "费用仍放在 BalanceCost，由扣费事务先扣套餐")

	balanceCmd := buildUsageBillingCommand("req-2", &UsageLog{BillingType: BillingTypeBalance}, p)
	require.Nil(t, balanceCmd.PackageGroupID)
}

func TestResolvePackageBilling_Branches(t *testing.T) {
	repo := newPackageRepoFake()
	svc := newPackageServiceForTest(repo, time.Now())
	ctx := context.Background()
	user := &User{ID: 1, PackageConcurrency: 0}
	standard := &Group{ID: 7, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}

	repo.state = &PackageGroupState{Usable: 2}
	pb, err := svc.ResolvePackageBilling(ctx, user, standard, true)
	require.NoError(t, err)
	require.Equal(t, DefaultPackageConcurrency, pb.Concurrency, "未设置套餐并发时取默认值")

	svc.InvalidateGroupState(1, 7)
	repo.state = &PackageGroupState{Frozen: 1}
	pb, err = svc.ResolvePackageBilling(ctx, user, standard, false)
	require.NoError(t, err)
	require.Nil(t, pb, "全部冻结但余额充足时按余额计费")
	_, err = svc.ResolvePackageBilling(ctx, user, standard, true)
	require.ErrorIs(t, err, ErrPackageFrozenNoBalance)

	free := &Group{ID: 9, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, IsFree: true}
	sub := &Group{ID: 8, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription}
	for _, g := range []*Group{free, sub, nil} {
		pb, err = svc.ResolvePackageBilling(ctx, user, g, true)
		require.NoError(t, err)
		require.Nil(t, pb, "免费分组、订阅分组不参与套餐计费")
	}
}
