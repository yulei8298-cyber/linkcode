//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminService_CreateGroup_SolLongContext(t *testing.T) {
	t.Run("保存豁免开关与上下文窗口", func(t *testing.T) {
		repo := &groupRepoStubForAdmin{}
		svc := &adminServiceImpl{groupRepo: repo}

		_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
			Name: "企业分组", Platform: PlatformOpenAI, RateMultiplier: 1,
			SolLongContextExempt: true, SolContextWindow: 500_000,
		})

		require.NoError(t, err)
		require.True(t, repo.created.SolLongContextExempt)
		require.Equal(t, 500_000, repo.created.SolContextWindow)
	})

	t.Run("默认不豁免、沿用目录窗口", func(t *testing.T) {
		repo := &groupRepoStubForAdmin{}
		svc := &adminServiceImpl{groupRepo: repo}

		_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "gpt-pro", Platform: PlatformOpenAI, RateMultiplier: 1})

		require.NoError(t, err)
		require.False(t, repo.created.SolLongContextExempt)
		require.Zero(t, repo.created.SolContextWindow)
	})

	t.Run("越界窗口被拒绝且不落库", func(t *testing.T) {
		repo := &groupRepoStubForAdmin{}
		svc := &adminServiceImpl{groupRepo: repo}

		_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
			Name: "g", Platform: PlatformOpenAI, RateMultiplier: 1, SolContextWindow: 1_000_000,
		})

		require.ErrorIs(t, err, ErrInvalidSolContextWindow)
		require.Nil(t, repo.created)
	})
}

func TestAdminService_UpdateGroup_SolLongContext(t *testing.T) {
	newExisting := func() *Group {
		return &Group{
			ID: 1, Name: "企业分组", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 1,
			SolLongContextExempt: true, SolContextWindow: 500_000,
		}
	}

	t.Run("未携带字段时保留原配置", func(t *testing.T) {
		repo := &groupRepoStubForAdmin{getByID: newExisting()}
		svc := &adminServiceImpl{groupRepo: repo}

		_, err := svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{Name: "企业首选"})

		require.NoError(t, err)
		require.True(t, repo.updated.SolLongContextExempt)
		require.Equal(t, 500_000, repo.updated.SolContextWindow)
	})

	t.Run("改为 272k 并关闭豁免", func(t *testing.T) {
		repo := &groupRepoStubForAdmin{getByID: newExisting()}
		svc := &adminServiceImpl{groupRepo: repo}
		exempt, window := false, 272_000

		_, err := svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{SolLongContextExempt: &exempt, SolContextWindow: &window})

		require.NoError(t, err)
		require.False(t, repo.updated.SolLongContextExempt)
		require.Equal(t, 272_000, repo.updated.SolContextWindow)
	})

	t.Run("0 表示恢复沿用目录窗口", func(t *testing.T) {
		repo := &groupRepoStubForAdmin{getByID: newExisting()}
		svc := &adminServiceImpl{groupRepo: repo}
		window := 0

		_, err := svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{SolContextWindow: &window})

		require.NoError(t, err)
		require.Zero(t, repo.updated.SolContextWindow)
	})

	t.Run("过小的窗口被拒绝", func(t *testing.T) {
		repo := &groupRepoStubForAdmin{getByID: newExisting()}
		svc := &adminServiceImpl{groupRepo: repo}
		window := 1000

		_, err := svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{SolContextWindow: &window})

		require.ErrorIs(t, err, ErrInvalidSolContextWindow)
		require.Nil(t, repo.updated)
	})
}

func TestAPIKeyAuthSnapshotCarriesSolLongContext(t *testing.T) {
	svc := &APIKeyService{}
	groupID := int64(9)
	apiKey := &APIKey{
		ID: 1, UserID: 2, GroupID: &groupID, Status: StatusActive,
		User:  &User{ID: 2, Status: StatusActive, Role: RoleUser},
		Group: &Group{ID: groupID, Platform: PlatformOpenAI, Status: StatusActive, SolLongContextExempt: true, SolContextWindow: 500_000},
	}

	restored := svc.snapshotToAPIKey("k", svc.snapshotFromAPIKey(context.Background(), apiKey))

	require.NotNil(t, restored.Group)
	require.True(t, restored.Group.SolLongContextExempt)
	require.Equal(t, 500_000, restored.Group.SolContextWindow)
}
