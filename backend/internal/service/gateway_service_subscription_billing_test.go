//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier locks in the fix
// that subscription-mode billing honours the group (and any user-specific) rate
// multiplier — i.e. cmd.SubscriptionCost tracks ActualCost (= TotalCost *
// RateMultiplier), not raw TotalCost.
func TestBuildUsageBillingCommand_SubscriptionAppliesRateMultiplier(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	subID := int64(42)

	tests := []struct {
		name           string
		totalCost      float64
		actualCost     float64
		isSubscription bool
		wantSub        float64
		wantBalance    float64
	}{
		{
			name:           "subscription with 2x multiplier consumes 2x quota",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: true,
			wantSub:        2.0,
			wantBalance:    0,
		},
		{
			name:           "subscription with 0.5x multiplier consumes 0.5x quota",
			totalCost:      1.0,
			actualCost:     0.5,
			isSubscription: true,
			wantSub:        0.5,
			wantBalance:    0,
		},
		{
			name:           "free subscription (multiplier 0) consumes no quota",
			totalCost:      1.0,
			actualCost:     0,
			isSubscription: true,
			wantSub:        0,
			wantBalance:    0,
		},
		{
			name:           "balance billing keeps using ActualCost (regression)",
			totalCost:      1.0,
			actualCost:     2.0,
			isSubscription: false,
			wantSub:        0,
			wantBalance:    2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &postUsageBillingParams{
				Cost:               &CostBreakdown{TotalCost: tt.totalCost, ActualCost: tt.actualCost},
				User:               &User{ID: 1},
				APIKey:             &APIKey{ID: 2, GroupID: &groupID},
				Account:            &Account{ID: 3},
				Subscription:       &UserSubscription{ID: subID},
				IsSubscriptionBill: tt.isSubscription,
			}

			cmd := buildUsageBillingCommand("req-1", nil, p)
			if cmd == nil {
				t.Fatal("buildUsageBillingCommand returned nil")
			}
			if cmd.SubscriptionCost != tt.wantSub {
				t.Errorf("SubscriptionCost = %v, want %v", cmd.SubscriptionCost, tt.wantSub)
			}
			if cmd.BalanceCost != tt.wantBalance {
				t.Errorf("BalanceCost = %v, want %v", cmd.BalanceCost, tt.wantBalance)
			}
		})
	}
}

func TestBuildUsageBillingCommand_FreeGroupTracksDailyUsageWithoutBalanceCost(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	usageDate := time.Date(2026, time.July, 14, 23, 59, 0, 0, time.UTC)
	p := &postUsageBillingParams{
		Cost:          &CostBreakdown{TotalCost: 1.0, ActualCost: 1.25},
		User:          &User{ID: 1},
		APIKey:        &APIKey{ID: 2, GroupID: &groupID, Group: &Group{ID: groupID, IsFree: true}},
		Account:       &Account{ID: 3},
		IsFreeBill:    true,
		FreeUsageDate: usageDate,
	}

	cmd := buildUsageBillingCommand("req-free", nil, p)
	if cmd == nil {
		t.Fatal("buildUsageBillingCommand returned nil")
	}
	if cmd.FreeGroupID == nil || *cmd.FreeGroupID != groupID {
		t.Fatalf("FreeGroupID = %v, want %d", cmd.FreeGroupID, groupID)
	}
	if cmd.FreeUsageCost != 1.25 {
		t.Errorf("FreeUsageCost = %v, want 1.25", cmd.FreeUsageCost)
	}
	if !cmd.FreeUsageDate.Equal(usageDate) {
		t.Errorf("FreeUsageDate = %v, want %v", cmd.FreeUsageDate, usageDate)
	}
	if cmd.BalanceCost != 0 || cmd.SubscriptionCost != 0 {
		t.Errorf("free billing must not charge balance or subscription: balance=%v subscription=%v", cmd.BalanceCost, cmd.SubscriptionCost)
	}
}

func TestApplyUsageBilling_FreeGroupFailsClosedWithoutUnifiedRepository(t *testing.T) {
	t.Parallel()

	groupID := int64(7)
	p := &postUsageBillingParams{
		Cost:       &CostBreakdown{TotalCost: 1, ActualCost: 1},
		User:       &User{ID: 1},
		APIKey:     &APIKey{ID: 2, GroupID: &groupID, Group: &Group{ID: groupID, IsFree: true}},
		Account:    &Account{ID: 3},
		IsFreeBill: true,
	}

	applied, err := applyUsageBilling(context.Background(), "req-free", nil, p, &billingDeps{}, nil)
	require.ErrorContains(t, err, "daily free usage billing repository is unavailable")
	require.False(t, applied)
}

func TestApplySimpleModeDailyFreeUsage_OnlyPersistsFreeLedger(t *testing.T) {
	groupID := int64(88)
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	params := &postUsageBillingParams{
		Cost:          &CostBreakdown{ActualCost: 1.25, TotalCost: 1.0},
		User:          &User{ID: 1},
		APIKey:        &APIKey{ID: 2, Quota: 10, GroupID: &groupID, Group: &Group{ID: groupID, IsFree: true}},
		Account:       &Account{ID: 3, Type: AccountTypeAPIKey},
		IsFreeBill:    true,
		APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{},
	}

	err := applySimpleModeDailyFreeUsage(context.Background(), "simple-free-1", &UsageLog{Model: "gpt-5.1"}, params, billingRepo)
	require.NoError(t, err)
	require.Equal(t, 1, billingRepo.calls)
	require.NotNil(t, billingRepo.lastCmd.FreeGroupID)
	require.Equal(t, groupID, *billingRepo.lastCmd.FreeGroupID)
	require.InDelta(t, 1.25, billingRepo.lastCmd.FreeUsageCost, 1e-12)
	require.Zero(t, billingRepo.lastCmd.BalanceCost)
	require.Zero(t, billingRepo.lastCmd.SubscriptionCost)
	require.Zero(t, billingRepo.lastCmd.APIKeyQuotaCost)
	require.Zero(t, billingRepo.lastCmd.APIKeyRateLimitCost)
	require.Zero(t, billingRepo.lastCmd.AccountQuotaCost)
}
