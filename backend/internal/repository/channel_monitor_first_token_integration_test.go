//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// AvgFirstTokenForMonitors 集成测试：验证探测 Key → 分组反查，以及只统计
// 「同分组 + 同模型 + 窗口内 + 非探测 Key + 有首字时间」的真实调用。
func TestChannelMonitorAvgFirstTokenForMonitors(t *testing.T) {
	ctx := context.Background()
	repo := NewChannelMonitorRepository(integrationEntClient, integrationDB)
	suffix := time.Now().UnixNano()

	group := mustCreateGroup(t, integrationEntClient, &service.Group{
		Name: fmt.Sprintf("monitor-ttft-group-%d", suffix), Platform: service.PlatformAnthropic, RateMultiplier: 1,
	})
	otherGroup := mustCreateGroup(t, integrationEntClient, &service.Group{
		Name: fmt.Sprintf("monitor-ttft-other-%d", suffix), Platform: service.PlatformAnthropic, RateMultiplier: 1,
	})
	user := mustCreateUser(t, integrationEntClient, &service.User{Email: fmt.Sprintf("monitor-ttft-%d@example.com", suffix)})
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("monitor-ttft-acc-%d", suffix)})

	groupID, otherGroupID := group.ID, otherGroup.ID
	probeKey := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: user.ID, GroupID: &groupID, Key: fmt.Sprintf("sk-monitor-probe-%d", suffix)})
	userKey := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: user.ID, GroupID: &groupID, Key: fmt.Sprintf("sk-monitor-user-%d", suffix)})
	otherKey := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: user.ID, GroupID: &otherGroupID, Key: fmt.Sprintf("sk-monitor-other-%d", suffix)})

	const model = "claude-opus-5"
	now := time.Now()
	seq := 0
	insert := func(keyID, gID int64, requestedModel string, firstTokenMs *int, createdAt time.Time) {
		seq++
		b := integrationEntClient.UsageLog.Create().
			SetUserID(user.ID).
			SetAPIKeyID(keyID).
			SetAccountID(account.ID).
			SetGroupID(gID).
			SetRequestID(fmt.Sprintf("monitor-ttft-%d-%d", suffix, seq)).
			SetModel(requestedModel).
			SetRequestedModel(requestedModel).
			SetCreatedAt(createdAt)
		if firstTokenMs != nil {
			b.SetFirstTokenMs(*firstTokenMs)
		}
		_, err := b.Save(ctx)
		require.NoError(t, err)
	}
	ms := func(v int) *int { return &v }

	insert(userKey.ID, groupID, model, ms(1000), now.Add(-10*time.Minute))      // 计入
	insert(userKey.ID, groupID, model, ms(2000), now.Add(-20*time.Minute))      // 计入
	insert(userKey.ID, groupID, model, nil, now.Add(-5*time.Minute))            // 无首字时间（非流式）：排除
	insert(probeKey.ID, groupID, model, ms(9000), now.Add(-5*time.Minute))      // 探测 Key 自身：排除
	insert(userKey.ID, groupID, "gpt-5", ms(9000), now.Add(-5*time.Minute))     // 其他模型：排除
	insert(otherKey.ID, otherGroupID, model, ms(9000), now.Add(-5*time.Minute)) // 其他分组：排除
	insert(userKey.ID, groupID, model, ms(9000), now.Add(-3*time.Hour))         // 窗口外：排除

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE request_id LIKE $1", fmt.Sprintf("monitor-ttft-%d-%%", suffix))
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM api_keys WHERE id = ANY(ARRAY[$1,$2,$3]::bigint[])", probeKey.ID, userKey.ID, otherKey.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id = $1", account.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id = ANY(ARRAY[$1,$2]::bigint[])", groupID, otherGroupID)
	})

	got, err := repo.AvgFirstTokenForMonitors(ctx, []service.MonitorFirstTokenTarget{
		{MonitorID: 1, APIKey: probeKey.Key, Model: model},
		{MonitorID: 2, APIKey: "sk-not-issued-here", Model: model},    // Key 不是本站签发：无结果
		{MonitorID: 3, APIKey: probeKey.Key, Model: "claude-haiku-5"}, // 该模型无调用：无结果
	}, now.Add(-time.Hour))
	require.NoError(t, err)
	require.Equal(t, map[int64]int{1: 1500}, got)
}
