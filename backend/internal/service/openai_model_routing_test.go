package service

import (
	"context"
	"testing"

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
	require.False(t, matched.allows(103))
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
			matchedAccounts: map[int64]struct{}{101: {}},
		},
	}

	compatible, reason := scheduler.isAccountRequestCompatibleReason(context.Background(), &Account{ID: 102}, req)
	require.False(t, compatible)
	require.Equal(t, "model_routing_target_only", reason)
}
