package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type groupRealtimeGroupRepoStub struct {
	GroupRepository
	rpm        []GroupRealtimeRPM
	owners     []GroupAPIKeyOwner
	missing    bool
	gotWindow  time.Duration
	gotGroupID int64
}

func (s *groupRealtimeGroupRepoStub) ListGroupRealtimeRPM(_ context.Context, window time.Duration) ([]GroupRealtimeRPM, error) {
	s.gotWindow = window
	return s.rpm, nil
}

func (s *groupRealtimeGroupRepoStub) ListActiveAPIKeyOwnersByGroup(_ context.Context, groupID int64) ([]GroupAPIKeyOwner, error) {
	s.gotGroupID = groupID
	return s.owners, nil
}

func (s *groupRealtimeGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	if s.missing {
		return nil, ErrGroupNotFound
	}
	return &Group{ID: id}, nil
}

// 只有基础 GroupRepository 的实现（没有实时指标能力）。
type groupRealtimePlainRepoStub struct{ GroupRepository }

type groupRealtimeAPIKeyCacheStub struct {
	ConcurrencyCache
	counts    map[int64]int
	requested []int64
}

func (s *groupRealtimeAPIKeyCacheStub) TrackAPIKeySlot(context.Context, int64, string) error {
	return nil
}
func (s *groupRealtimeAPIKeyCacheStub) ReleaseAPIKeySlot(context.Context, int64, string) error {
	return nil
}
func (s *groupRealtimeAPIKeyCacheStub) GetAPIKeyConcurrencyBatch(_ context.Context, ids []int64) (map[int64]int, error) {
	s.requested = append([]int64(nil), ids...)
	return s.counts, nil
}

func TestGetRealtimeRPM_SumsGroupsAndUsesSixtySecondWindow(t *testing.T) {
	repo := &groupRealtimeGroupRepoStub{rpm: []GroupRealtimeRPM{{GroupID: 2, RPM: 30}, {GroupID: 38, RPM: 12}}}
	svc := NewGroupCapacityService(nil, repo, nil, nil, nil)

	summary, err := svc.GetRealtimeRPM(context.Background())
	require.NoError(t, err)

	require.Equal(t, 60*time.Second, repo.gotWindow)
	require.Equal(t, 60, summary.WindowSeconds)
	require.Equal(t, 42, summary.Total)
	require.Equal(t, repo.rpm, summary.Items)
}

func TestGetRealtimeRPM_EmptyIsNotNil(t *testing.T) {
	svc := NewGroupCapacityService(nil, &groupRealtimeGroupRepoStub{}, nil, nil, nil)

	summary, err := svc.GetRealtimeRPM(context.Background())
	require.NoError(t, err)
	require.NotNil(t, summary.Items)
	require.Empty(t, summary.Items)
	require.Zero(t, summary.Total)
}

func TestGetGroupUserConcurrency_AggregatesKeysPerUserAndSorts(t *testing.T) {
	repo := &groupRealtimeGroupRepoStub{owners: []GroupAPIKeyOwner{
		{APIKeyID: 1, APIKeyName: "a", UserID: 10, Email: "u10@x.com", Username: "ten"},
		{APIKeyID: 2, APIKeyName: "b", UserID: 10, Email: "u10@x.com", Username: "ten"},
		{APIKeyID: 3, APIKeyName: "c", UserID: 20, Email: "u20@x.com"},
		{APIKeyID: 4, APIKeyName: "idle", UserID: 30, Email: "u30@x.com"},
		{APIKeyID: 5, APIKeyName: "d", UserID: 40, Email: "u40@x.com"},
	}}
	cache := &groupRealtimeAPIKeyCacheStub{counts: map[int64]int{1: 1, 2: 3, 3: 4, 4: 0, 5: 4}}
	svc := NewGroupCapacityService(nil, repo, NewConcurrencyService(cache), nil, nil)

	summary, err := svc.GetGroupUserConcurrency(context.Background(), 38)
	require.NoError(t, err)

	require.Equal(t, int64(38), repo.gotGroupID)
	require.Equal(t, []int64{1, 2, 3, 4, 5}, cache.requested)
	require.Equal(t, 5, summary.KeyCount)
	require.Equal(t, 12, summary.Total)
	// 并发相同的用户按 user_id 升序；没有在途请求的用户（30）不出现。
	require.Len(t, summary.Users, 3)
	require.Equal(t, int64(10), summary.Users[0].UserID)
	require.Equal(t, 4, summary.Users[0].Concurrency)
	require.Equal(t, int64(20), summary.Users[1].UserID)
	require.Equal(t, 4, summary.Users[1].Concurrency)
	require.Equal(t, int64(40), summary.Users[2].UserID)
	// 同一用户的 Key 明细按并发降序。
	require.Equal(t, []GroupUserConcurrencyKey{
		{APIKeyID: 2, APIKeyName: "b", Concurrency: 3},
		{APIKeyID: 1, APIKeyName: "a", Concurrency: 1},
	}, summary.Users[0].APIKeys)
}

func TestGetGroupUserConcurrency_NoKeysOrNoRedisReturnsEmpty(t *testing.T) {
	empty := NewGroupCapacityService(nil, &groupRealtimeGroupRepoStub{}, nil, nil, nil)
	summary, err := empty.GetGroupUserConcurrency(context.Background(), 1)
	require.NoError(t, err)
	require.Empty(t, summary.Users)
	require.Zero(t, summary.Total)

	// 有 Key 但没有并发服务（Redis 不可用）：统计是尽力而为，全部按 0 处理。
	repo := &groupRealtimeGroupRepoStub{owners: []GroupAPIKeyOwner{{APIKeyID: 1, UserID: 10}}}
	noRedis := NewGroupCapacityService(nil, repo, nil, nil, nil)
	summary, err = noRedis.GetGroupUserConcurrency(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, summary.KeyCount)
	require.Empty(t, summary.Users)
}

func TestGetGroupUserConcurrency_UnknownGroupAndUnsupportedRepo(t *testing.T) {
	missing := NewGroupCapacityService(nil, &groupRealtimeGroupRepoStub{missing: true}, nil, nil, nil)
	_, err := missing.GetGroupUserConcurrency(context.Background(), 999)
	require.ErrorIs(t, err, ErrGroupNotFound)

	plain := NewGroupCapacityService(nil, &groupRealtimePlainRepoStub{}, nil, nil, nil)
	_, err = plain.GetGroupUserConcurrency(context.Background(), 1)
	require.ErrorIs(t, err, ErrGroupRealtimeUnsupported)
	_, err = plain.GetRealtimeRPM(context.Background())
	require.ErrorIs(t, err, ErrGroupRealtimeUnsupported)
}
