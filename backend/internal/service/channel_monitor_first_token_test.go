//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type firstTokenMonitorRepoStub struct {
	ChannelMonitorRepository
	monitors    []*ChannelMonitor
	firstToken  map[int64]int
	firstErr    error
	calls       int
	lastTargets []MonitorFirstTokenTarget
	lastSince   time.Time
}

func (r *firstTokenMonitorRepoStub) ListEnabled(context.Context) ([]*ChannelMonitor, error) {
	return r.monitors, nil
}

func (r *firstTokenMonitorRepoStub) ListLatestForMonitorIDs(context.Context, []int64) (map[int64][]*ChannelMonitorLatest, error) {
	return map[int64][]*ChannelMonitorLatest{}, nil
}

func (r *firstTokenMonitorRepoStub) ComputeAvailabilityForMonitors(context.Context, []int64, int) (map[int64][]*ChannelMonitorAvailability, error) {
	return map[int64][]*ChannelMonitorAvailability{}, nil
}

func (r *firstTokenMonitorRepoStub) ListRecentHistoryForMonitors(context.Context, []int64, map[int64]string, int) (map[int64][]*ChannelMonitorHistoryEntry, error) {
	return map[int64][]*ChannelMonitorHistoryEntry{}, nil
}

func (r *firstTokenMonitorRepoStub) AvgFirstTokenForMonitors(_ context.Context, targets []MonitorFirstTokenTarget, since time.Time) (map[int64]int, error) {
	r.calls++
	r.lastTargets = targets
	r.lastSince = since
	if r.firstErr != nil {
		return nil, r.firstErr
	}
	return r.firstToken, nil
}

func newFirstTokenTestMonitors() []*ChannelMonitor {
	return []*ChannelMonitor{
		{ID: 1, Name: "有调用", PrimaryModel: "claude-opus-5", APIKey: "OLD:sk-probe-1", CheckMode: MonitorCheckModeProbe},
		{ID: 2, Name: "无调用", PrimaryModel: "gpt-5", APIKey: "OLD:sk-probe-2"},
		{ID: 3, Name: "仅配额", PrimaryModel: "glm-5", APIKey: "OLD:sk-quota", CheckMode: MonitorCheckModeQuota},
		{ID: 4, Name: "密文损坏", PrimaryModel: "grok-4", APIKey: "broken"},
	}
}

func TestListUserView_FillsFirstTokenFromRealUsage(t *testing.T) {
	repo := &firstTokenMonitorRepoStub{monitors: newFirstTokenTestMonitors(), firstToken: map[int64]int{1: 1830}}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	before := time.Now()
	views, err := svc.ListUserView(context.Background())
	require.NoError(t, err)
	require.Len(t, views, 4)

	require.NotNil(t, views[0].PrimaryFirstTokenMs)
	require.Equal(t, 1830, *views[0].PrimaryFirstTokenMs)
	for _, v := range views[1:] {
		require.Nil(t, v.PrimaryFirstTokenMs, "无真实调用数据的监控应留空，由前端退回探测延迟")
	}

	// 仅配额模式与解密失败的监控不参与统计；明文 Key 只传给仓储，不改动监控对象。
	require.Equal(t, []MonitorFirstTokenTarget{
		{MonitorID: 1, APIKey: "sk-probe-1", Model: "claude-opus-5"},
		{MonitorID: 2, APIKey: "sk-probe-2", Model: "gpt-5"},
	}, repo.lastTargets)
	require.Equal(t, "OLD:sk-probe-1", repo.monitors[0].APIKey)
	require.WithinDuration(t, before.Add(-monitorFirstTokenWindow), repo.lastSince, time.Second)
}

func TestListUserView_FirstTokenCachedWithinTTL(t *testing.T) {
	repo := &firstTokenMonitorRepoStub{monitors: newFirstTokenTestMonitors(), firstToken: map[int64]int{1: 900}}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	for i := 0; i < 3; i++ {
		_, err := svc.ListUserView(context.Background())
		require.NoError(t, err)
	}
	require.Equal(t, 1, repo.calls, "TTL 内重复请求应命中缓存")

	svc.firstToken.expiresAt = time.Now().Add(-time.Second)
	_, err := svc.ListUserView(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, repo.calls, "缓存过期后应重新统计")
}

func TestListUserView_FirstTokenErrorDegradesAndRetries(t *testing.T) {
	repo := &firstTokenMonitorRepoStub{monitors: newFirstTokenTestMonitors(), firstErr: errors.New("db down")}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	views, err := svc.ListUserView(context.Background())
	require.NoError(t, err, "首字延迟统计失败不应阻断列表")
	for _, v := range views {
		require.Nil(t, v.PrimaryFirstTokenMs)
	}

	repo.firstErr = nil
	repo.firstToken = map[int64]int{2: 640}
	views, err = svc.ListUserView(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, repo.calls, "失败结果不应被缓存")
	require.NotNil(t, views[1].PrimaryFirstTokenMs)
	require.Equal(t, 640, *views[1].PrimaryFirstTokenMs)
}

func TestListUserView_NoProbeKeysSkipsQuery(t *testing.T) {
	repo := &firstTokenMonitorRepoStub{monitors: []*ChannelMonitor{
		{ID: 9, PrimaryModel: "glm-5", CheckMode: MonitorCheckModeQuota},
	}}
	svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})

	views, err := svc.ListUserView(context.Background())
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.Nil(t, views[0].PrimaryFirstTokenMs)
	require.Zero(t, repo.calls, "没有可统计的探测 Key 时不应查询 usage_logs")
}
