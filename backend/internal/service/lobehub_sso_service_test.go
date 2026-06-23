//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type lobeHubSSOGroupRepoStub struct {
	groupRepoNoop
	groups []Group
}

func (s *lobeHubSSOGroupRepoStub) ListActiveByPlatform(_ context.Context, platform string) ([]Group, error) {
	out := make([]Group, 0, len(s.groups))
	for _, g := range s.groups {
		if g.Status == StatusActive && g.Platform == platform {
			out = append(out, g)
		}
	}
	return out, nil
}

func (s *lobeHubSSOGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	for _, g := range s.groups {
		if g.ID == id {
			clone := g
			return &clone, nil
		}
	}
	return nil, ErrGroupNotFound
}

type lobeHubSSOUserRepoStub struct {
	userRepoStubForGroupUpdate
	user *User
}

func (s *lobeHubSSOUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	if s.user == nil || s.user.ID != id {
		return nil, ErrUserNotFound
	}
	clone := *s.user
	clone.AllowedGroups = append([]int64(nil), s.user.AllowedGroups...)
	return &clone, nil
}

func (s *lobeHubSSOUserRepoStub) AddGroupToAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	if err := s.userRepoStubForGroupUpdate.AddGroupToAllowedGroups(ctx, userID, groupID); err != nil {
		return err
	}
	if s.user != nil && s.user.ID == userID {
		for _, existing := range s.user.AllowedGroups {
			if existing == groupID {
				return nil
			}
		}
		s.user.AllowedGroups = append(s.user.AllowedGroups, groupID)
	}
	return nil
}

type lobeHubSSOAPIKeyRepoStub struct {
	created []APIKey
	nextID  int64
}

func (s *lobeHubSSOAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	if s.nextID == 0 {
		s.nextID = 1
	}
	key.ID = s.nextID
	s.nextID++
	clone := *key
	s.created = append(s.created, clone)
	return nil
}

func (s *lobeHubSSOAPIKeyRepoStub) SearchAPIKeys(_ context.Context, userID int64, keyword string, limit int) ([]APIKey, error) {
	out := make([]APIKey, 0, len(s.created))
	for _, key := range s.created {
		if key.UserID == userID && key.Name == keyword {
			out = append(out, key)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (s *lobeHubSSOAPIKeyRepoStub) ExistsByKey(_ context.Context, key string) (bool, error) {
	for _, existing := range s.created {
		if existing.Key == key {
			return true, nil
		}
	}
	return false, nil
}

func (s *lobeHubSSOAPIKeyRepoStub) GetByID(context.Context, int64) (*APIKey, error) {
	panic("unexpected GetByID call")
}
func (s *lobeHubSSOAPIKeyRepoStub) GetKeyAndOwnerID(context.Context, int64) (string, int64, error) {
	panic("unexpected GetKeyAndOwnerID call")
}
func (s *lobeHubSSOAPIKeyRepoStub) GetByKey(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKey call")
}
func (s *lobeHubSSOAPIKeyRepoStub) GetByKeyForAuth(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKeyForAuth call")
}
func (s *lobeHubSSOAPIKeyRepoStub) Update(context.Context, *APIKey) error {
	panic("unexpected Update call")
}
func (s *lobeHubSSOAPIKeyRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *lobeHubSSOAPIKeyRepoStub) DeleteWithAudit(context.Context, int64) error {
	panic("unexpected DeleteWithAudit call")
}
func (s *lobeHubSSOAPIKeyRepoStub) ListByUserID(context.Context, int64, pagination.PaginationParams, APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserID call")
}
func (s *lobeHubSSOAPIKeyRepoStub) VerifyOwnership(context.Context, int64, []int64) ([]int64, error) {
	panic("unexpected VerifyOwnership call")
}
func (s *lobeHubSSOAPIKeyRepoStub) CountByUserID(context.Context, int64) (int64, error) {
	panic("unexpected CountByUserID call")
}
func (s *lobeHubSSOAPIKeyRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}
func (s *lobeHubSSOAPIKeyRepoStub) ClearGroupIDByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected ClearGroupIDByGroupID call")
}
func (s *lobeHubSSOAPIKeyRepoStub) UpdateGroupIDByUserAndGroup(context.Context, int64, int64, int64) (int64, error) {
	panic("unexpected UpdateGroupIDByUserAndGroup call")
}
func (s *lobeHubSSOAPIKeyRepoStub) CountByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected CountByGroupID call")
}
func (s *lobeHubSSOAPIKeyRepoStub) ListKeysByUserID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByUserID call")
}
func (s *lobeHubSSOAPIKeyRepoStub) ListKeysByGroupID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByGroupID call")
}
func (s *lobeHubSSOAPIKeyRepoStub) IncrementQuotaUsed(context.Context, int64, float64) (float64, error) {
	panic("unexpected IncrementQuotaUsed call")
}
func (s *lobeHubSSOAPIKeyRepoStub) UpdateLastUsed(context.Context, int64, time.Time) error {
	panic("unexpected UpdateLastUsed call")
}
func (s *lobeHubSSOAPIKeyRepoStub) IncrementRateLimitUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementRateLimitUsage call")
}
func (s *lobeHubSSOAPIKeyRepoStub) ResetRateLimitWindows(context.Context, int64) error {
	panic("unexpected ResetRateLimitWindows call")
}
func (s *lobeHubSSOAPIKeyRepoStub) GetRateLimitData(context.Context, int64) (*APIKeyRateLimitData, error) {
	panic("unexpected GetRateLimitData call")
}

func TestLobeHubSSOPrepareProviderKeysCreatesKeyForExclusiveOpenAIChatGroupWithoutChannel(t *testing.T) {
	groupID := int64(88)
	groupRepo := &lobeHubSSOGroupRepoStub{
		groups: []Group{{
			ID:               groupID,
			Name:             lobeHubGroupOpenAI,
			Platform:         PlatformOpenAI,
			Status:           StatusActive,
			IsExclusive:      true,
			SubscriptionType: SubscriptionTypeStandard,
		}},
	}
	userRepo := &lobeHubSSOUserRepoStub{
		user: &User{ID: 1001, Email: "new@example.com", Status: StatusActive},
	}
	apiKeyRepo := &lobeHubSSOAPIKeyRepoStub{}
	cfg := &config.Config{}
	cfg.Default.APIKeyPrefix = "sk-test-"
	cfg.LobeHubSSO.APIKeyNamePrefix = "Link AI"

	apiKeyService := NewAPIKeyService(apiKeyRepo, userRepo, groupRepo, nil, nil, nil, cfg)
	svc := NewLobeHubSSOService(cfg, nil, userRepo, groupRepo, nil, apiKeyService, nil, nil)

	keys, err := svc.prepareProviderKeys(context.Background(), userRepo.user.ID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, lobeHubProviderGPT, keys[0].Provider)
	require.Equal(t, PlatformOpenAI, keys[0].Platform)
	require.Equal(t, groupID, keys[0].GroupID)
	require.NotEmpty(t, keys[0].Key)

	require.True(t, userRepo.addGroupCalled)
	require.Equal(t, userRepo.user.ID, userRepo.addedUserID)
	require.Equal(t, groupID, userRepo.addedGroupID)

	require.Len(t, apiKeyRepo.created, 1)
	require.Equal(t, "Link AI GPT", apiKeyRepo.created[0].Name)
	require.NotNil(t, apiKeyRepo.created[0].GroupID)
	require.Equal(t, groupID, *apiKeyRepo.created[0].GroupID)
}
