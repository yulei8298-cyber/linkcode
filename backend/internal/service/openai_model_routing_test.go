package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestOpenAIModelRoutingReservesAccountsWithinGroup(t *testing.T) {
	groupID := int64(42)
	ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{
		ID:                  groupID,
		Platform:            PlatformOpenAI,
		ModelRoutingEnabled: true,
		ModelRouting: map[string][]int64{
			"gpt-5.6-sol":  {101},
			"gpt-5.6-luna": {102},
		},
	})
	svc := &OpenAIGatewayService{}

	matched := svc.openAIModelRoutingForRequest(ctx, &groupID, PlatformOpenAI, "gpt-5.6-sol")
	require.True(t, matched.allows(101))
	require.False(t, matched.allows(102))
	require.True(t, matched.allows(103), "unreserved accounts remain available as fallback")
	require.Equal(t, "model_routing_target_only", matched.rejectionReason())

	unmatched := svc.openAIModelRoutingForRequest(ctx, &groupID, PlatformOpenAI, "gpt-5.6-terra")
	require.False(t, unmatched.allows(101))
	require.False(t, unmatched.allows(102))
	require.True(t, unmatched.allows(103))
	require.Equal(t, "model_routing_reserved", unmatched.rejectionReason())
}

func TestOpenAIModelRoutingDoesNotChangeOtherPlatforms(t *testing.T) {
	groupID := int64(42)
	ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{
		ID:                  groupID,
		Platform:            PlatformAnthropic,
		ModelRoutingEnabled: true,
		ModelRouting:        map[string][]int64{"claude-*": {101}},
	})

	policy := (&OpenAIGatewayService{}).openAIModelRoutingForRequest(ctx, &groupID, PlatformOpenAI, "gpt-5.6-sol")
	require.True(t, policy.allows(101))
	require.True(t, policy.allows(102))
}

func TestOpenAIAccountSchedulerModelRoutingFilter(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{}
	req := OpenAIAccountScheduleRequest{
		ModelRouting: openAIModelRoutingPolicy{
			matchedAccounts:  map[int64]struct{}{101: {}},
			reservedAccounts: map[int64]struct{}{101: {}, 102: {}},
		},
	}

	compatible, reason := scheduler.isAccountRequestCompatibleReason(context.Background(), &Account{ID: 102}, req)
	require.False(t, compatible)
	require.Equal(t, "model_routing_target_only", reason)
}

func TestOpenAIModelRoutingPrefersTargetAndAllowsFallback(t *testing.T) {
	policy := openAIModelRoutingPolicy{
		matchedAccounts:  map[int64]struct{}{101: {}},
		reservedAccounts: map[int64]struct{}{101: {}, 102: {}},
	}

	require.True(t, policy.isPreferred(101))
	require.True(t, policy.allows(101))
	require.False(t, policy.allows(102), "another model's routed account stays reserved")
	require.True(t, policy.allows(103), "ordinary accounts are the fallback pool")
}

func TestOpenAISelectionOrderPrefersRoutedAccount(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{}
	req := OpenAIAccountScheduleRequest{
		ModelRouting: openAIModelRoutingPolicy{
			matchedAccounts:  map[int64]struct{}{101: {}},
			reservedAccounts: map[int64]struct{}{101: {}, 102: {}},
		},
	}
	plan := openAIAccountLoadPlan{
		topK: 1,
		candidates: []openAIAccountCandidateScore{
			{account: &Account{ID: 102}, loadInfo: &AccountLoadInfo{AccountID: 102}},
			{account: &Account{ID: 101}, loadInfo: &AccountLoadInfo{AccountID: 101}},
		},
	}

	order := scheduler.buildOpenAISelectionOrder(req, plan)
	require.Len(t, order, 2)
	require.Equal(t, int64(101), order[0].account.ID)
	require.Equal(t, int64(102), order[1].account.ID)
}

func TestOpenAISelectAccountWithLoadAwareness_ModelRoutingPrefersTarget(t *testing.T) {
	const (
		targetAccount  = int64(101)
		regularAccount = int64(103)
		requestedModel = "gpt-5.6-luna"
	)
	groupID := int64(42)
	ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{
		ID:                  groupID,
		Platform:            PlatformOpenAI,
		ModelRoutingEnabled: true,
		ModelRouting: map[string][]int64{
			requestedModel: {targetAccount},
			"gpt-5.6-sol":  {102},
		},
	})
	accounts := []Account{
		{ID: targetAccount, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
		{ID: regularAccount, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0},
	}

	tests := []struct {
		name  string
		cache stubConcurrencyCache
		want  int64
	}{
		{
			name: "load aware selection keeps routed target ahead of lower load regular account",
			cache: stubConcurrencyCache{loadMap: map[int64]*AccountLoadInfo{
				targetAccount:  {AccountID: targetAccount, LoadRate: 80},
				regularAccount: {AccountID: regularAccount, LoadRate: 0},
			}},
			want: targetAccount,
		},
		{
			name:  "load lookup error keeps routed target ahead of higher priority regular account",
			cache: stubConcurrencyCache{loadBatchErr: errors.New("load lookup failed")},
			want:  targetAccount,
		},
		{
			name: "regular account remains fallback when routed target is full",
			cache: stubConcurrencyCache{loadMap: map[int64]*AccountLoadInfo{
				targetAccount:  {AccountID: targetAccount, LoadRate: 100},
				regularAccount: {AccountID: regularAccount, LoadRate: 0},
			}},
			want: regularAccount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.LoadBatchEnabled = true
			svc := &OpenAIGatewayService{
				accountRepo:        stubOpenAIAccountRepo{accounts: accounts},
				cfg:                cfg,
				concurrencyService: NewConcurrencyService(tt.cache),
			}

			selection, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, "", requestedModel, nil)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.NotNil(t, selection.Account)
			require.Equal(t, tt.want, selection.Account.ID)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
		})
	}
}

func TestOpenAISelectAccountWithLoadAwareness_ModelRoutingClearsRegularStickyAccount(t *testing.T) {
	const (
		targetAccount  = int64(201)
		regularAccount = int64(203)
		sessionHash    = "routed-sticky"
		requestedModel = "gpt-5.6-luna"
	)
	groupID := int64(43)
	ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{
		ID:                  groupID,
		Platform:            PlatformOpenAI,
		ModelRoutingEnabled: true,
		ModelRouting:        map[string][]int64{requestedModel: {targetAccount}},
	})
	cache := &stubGatewayCache{sessionBindings: map[string]int64{"openai:" + sessionHash: regularAccount}}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	svc := &OpenAIGatewayService{
		accountRepo: stubOpenAIAccountRepo{accounts: []Account{
			{ID: targetAccount, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10},
			{ID: regularAccount, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0},
		}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, sessionHash, requestedModel, nil)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, targetAccount, selection.Account.ID)
	require.Equal(t, 1, cache.deletedSessions["openai:"+sessionHash])
	require.Equal(t, targetAccount, cache.sessionBindings["openai:"+sessionHash])
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}
