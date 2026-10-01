//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 简单模式开启 Key 限速窗口后，免费分组的每日免费账本必须与限速窗口在同一条命令内落库，
// 否则同 request_id 的第二次 Apply 会被幂等去重吞掉，导致每日免费额度失效。
func TestBuildUsageBillingCommandSimpleModeKeepsDailyFreeLedger(t *testing.T) {
	groupID := int64(66)
	p := &postUsageBillingParams{
		Cost:                       &CostBreakdown{ActualCost: 2.5, TotalCost: 2},
		User:                       &User{ID: 7},
		APIKey:                     &APIKey{ID: 13, Quota: 100, RateLimit5h: 10, GroupID: &groupID, Group: &Group{ID: groupID, IsFree: true}},
		Account:                    &Account{ID: 9, Type: AccountTypeAPIKey},
		IsFreeBill:                 true,
		APIKeyService:              &apiKeyQuotaUpdaterStub{},
		SimpleModeKeyRateLimitOnly: true,
	}

	cmd := buildUsageBillingCommand("simple-free-req", nil, p)
	require.NotNil(t, cmd)
	require.InDelta(t, 2.5, cmd.APIKeyRateLimitCost, 1e-12)
	require.NotNil(t, cmd.FreeGroupID)
	require.Equal(t, groupID, *cmd.FreeGroupID)
	require.InDelta(t, 2.5, cmd.FreeUsageCost, 1e-12)
	require.False(t, cmd.FreeUsageDate.IsZero())
	require.Zero(t, cmd.BalanceCost)
	require.Zero(t, cmd.SubscriptionCost)
	require.Zero(t, cmd.APIKeyQuotaCost)
	require.Zero(t, cmd.AccountQuotaCost)
}

func TestSimpleModeRecordUsageFreeGroupLedgerWithKeyWindows(t *testing.T) {
	for _, openAI := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("openai=%v/key_windows=%v", openAI, enabled), func(t *testing.T) {
				groupID := int64(66)
				logs := &openAIRecordUsageLogRepoStub{inserted: true}
				billing := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
				users := &openAIRecordUsageUserRepoStub{}
				subs := &openAIRecordUsageSubRepoStub{}
				key := &APIKey{ID: 1, Quota: 100, RateLimit5h: 30, GroupID: &groupID, Group: &Group{ID: groupID, IsFree: true, RateMultiplier: 1}}
				user := &User{ID: 2}
				account := &Account{ID: 3, Type: AccountTypeAPIKey}
				if openAI {
					svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billing, users, subs, nil)
					svc.cfg.RunMode = config.RunModeSimple
					svc.cfg.SimpleModeKeyRateLimitEnabled = enabled
					require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
						Result: &OpenAIForwardResult{RequestID: "simple-free", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 20}},
						APIKey: key, User: user, Account: account,
					}))
				} else {
					svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, users, subs)
					svc.cfg.RunMode = config.RunModeSimple
					svc.cfg.SimpleModeKeyRateLimitEnabled = enabled
					require.NoError(t, svc.RecordUsage(context.Background(), &RecordUsageInput{
						Result: &ForwardResult{RequestID: "simple-free", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 20}},
						APIKey: key, User: user, Account: account,
					}))
				}

				require.Equal(t, 1, logs.calls)
				require.Positive(t, logs.lastLog.ActualCost)
				require.Equal(t, 1, billing.calls, "免费账本与限速窗口只能合并为一次 Apply")
				require.NotNil(t, billing.lastCmd.FreeGroupID)
				require.Equal(t, groupID, *billing.lastCmd.FreeGroupID)
				require.InDelta(t, logs.lastLog.ActualCost, billing.lastCmd.FreeUsageCost, 1e-12)
				if enabled {
					require.InDelta(t, logs.lastLog.ActualCost, billing.lastCmd.APIKeyRateLimitCost, 1e-12)
				} else {
					require.Zero(t, billing.lastCmd.APIKeyRateLimitCost)
				}
				require.Zero(t, billing.lastCmd.BalanceCost)
				require.Zero(t, billing.lastCmd.SubscriptionCost)
				require.Zero(t, billing.lastCmd.APIKeyQuotaCost)
				require.Zero(t, billing.lastCmd.AccountQuotaCost)
				require.Zero(t, users.deductCalls)
				require.Zero(t, subs.incrementCalls)
			})
		}
	}
}
