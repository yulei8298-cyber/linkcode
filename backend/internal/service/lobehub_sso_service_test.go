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
	created    []APIKey
	nextID     int64
	dailyUsage float64
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

func (s *lobeHubSSOAPIKeyRepoStub) FindActiveChatStationKey(_ context.Context, userID, groupID int64) (*APIKey, error) {
	for i := range s.created {
		key := s.created[i]
		if key.UserID == userID && key.GroupID != nil && *key.GroupID == groupID &&
			key.Name == ChatStationAPIKeyName && key.IsActive() {
			return &key, nil
		}
	}
	return nil, nil
}

func (s *lobeHubSSOAPIKeyRepoStub) GetDailyFreeUsage(context.Context, int64, int64, time.Time) (float64, error) {
	return s.dailyUsage, nil
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

type lobeHubSSOCodeStoreStub struct {
	payload *LobeHubSSOCodePayload
}

func (s *lobeHubSSOCodeStoreStub) Store(context.Context, string, LobeHubSSOCodePayload, time.Duration) error {
	panic("unexpected Store call")
}

func (s *lobeHubSSOCodeStoreStub) Take(context.Context, string) (*LobeHubSSOCodePayload, error) {
	return s.payload, nil
}

func TestAPIKeyServiceResolveChatStationAPIKeyCreatesAndReusesSortedCandidate(t *testing.T) {
	limit := 2.5
	selectedGroupID := int64(88)
	groupRepo := &lobeHubSSOGroupRepoStub{
		groups: []Group{
			{
				ID: selectedGroupID, Name: "free-chat", Platform: PlatformOpenAI,
				Status: StatusActive, SubscriptionType: SubscriptionTypeStandard,
				IsHidden: true, IsFree: true, DailyFreeLimitUSD: &limit, ChatStationOnly: true, SortOrder: 5,
			},
			{
				ID: 89, Name: "later-free-chat", Platform: PlatformOpenAI,
				Status: StatusActive, SubscriptionType: SubscriptionTypeStandard,
				IsHidden: true, IsFree: true, DailyFreeLimitUSD: &limit, ChatStationOnly: true, SortOrder: 10,
			},
			{
				ID: 70, Name: "not-free", Platform: PlatformOpenAI,
				Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, SortOrder: 1,
			},
		},
	}
	userRepo := &lobeHubSSOUserRepoStub{
		user: &User{ID: 1001, Email: "new@example.com", Status: StatusActive},
	}
	apiKeyRepo := &lobeHubSSOAPIKeyRepoStub{}
	cfg := &config.Config{}
	cfg.Default.APIKeyPrefix = "sk-test-"

	apiKeyService := NewAPIKeyService(apiKeyRepo, userRepo, groupRepo, nil, nil, nil, cfg)
	key, err := apiKeyService.ResolveChatStationAPIKey(context.Background(), userRepo.user.ID, PlatformOpenAI)
	require.NoError(t, err)
	require.NotEmpty(t, key)
	require.Len(t, apiKeyRepo.created, 1)
	require.Equal(t, ChatStationAPIKeyName, apiKeyRepo.created[0].Name)
	require.NotNil(t, apiKeyRepo.created[0].GroupID)
	require.Equal(t, selectedGroupID, *apiKeyRepo.created[0].GroupID)
	require.False(t, userRepo.addGroupCalled)

	reused, err := apiKeyService.ResolveChatStationAPIKey(context.Background(), userRepo.user.ID, PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, key, reused)
	require.Len(t, apiKeyRepo.created, 1)
}

func TestAPIKeyServiceResolveChatStationAPIKeySkipsVisibleCandidate(t *testing.T) {
	limit := 2.5
	groupRepo := &lobeHubSSOGroupRepoStub{
		groups: []Group{
			{
				ID: 87, Name: "visible-free-chat", Platform: PlatformOpenAI,
				Status: StatusActive, SubscriptionType: SubscriptionTypeStandard,
				IsFree: true, DailyFreeLimitUSD: &limit, ChatStationOnly: true, SortOrder: 1,
			},
			{
				ID: 88, Name: "hidden-free-chat", Platform: PlatformOpenAI,
				Status: StatusActive, SubscriptionType: SubscriptionTypeStandard,
				IsHidden: true, IsFree: true, DailyFreeLimitUSD: &limit, ChatStationOnly: true, SortOrder: 2,
			},
		},
	}
	userRepo := &lobeHubSSOUserRepoStub{user: &User{ID: 1001, Status: StatusActive}}
	apiKeyRepo := &lobeHubSSOAPIKeyRepoStub{}
	cfg := &config.Config{}
	cfg.Default.APIKeyPrefix = "sk-test-"

	svc := NewAPIKeyService(apiKeyRepo, userRepo, groupRepo, nil, nil, nil, cfg)
	_, err := svc.ResolveChatStationAPIKey(context.Background(), userRepo.user.ID, PlatformOpenAI)
	require.NoError(t, err)
	require.Len(t, apiKeyRepo.created, 1)
	require.Equal(t, int64(88), *apiKeyRepo.created[0].GroupID)
}

func TestAPIKeyServiceResolveChatStationAPIKeyDoesNotReuseOrdinaryKey(t *testing.T) {
	limit := 2.5
	groupID := int64(88)
	groupRepo := &lobeHubSSOGroupRepoStub{groups: []Group{{
		ID: groupID, Name: "hidden-free-chat", Platform: PlatformOpenAI,
		Status: StatusActive, SubscriptionType: SubscriptionTypeStandard,
		IsHidden: true, IsFree: true, DailyFreeLimitUSD: &limit, ChatStationOnly: true,
	}}}
	userRepo := &lobeHubSSOUserRepoStub{user: &User{ID: 1001, Status: StatusActive}}
	apiKeyRepo := &lobeHubSSOAPIKeyRepoStub{created: []APIKey{{
		ID: 1, UserID: userRepo.user.ID, Name: "ordinary-key", GroupID: &groupID,
		Key: "sk-ordinary", Status: StatusAPIKeyActive,
	}}}
	cfg := &config.Config{}
	cfg.Default.APIKeyPrefix = "sk-test-"

	svc := NewAPIKeyService(apiKeyRepo, userRepo, groupRepo, nil, nil, nil, cfg)
	key, err := svc.ResolveChatStationAPIKey(context.Background(), userRepo.user.ID, PlatformOpenAI)
	require.NoError(t, err)
	require.NotEqual(t, "sk-ordinary", key)
	require.Len(t, apiKeyRepo.created, 2)
	require.Equal(t, ChatStationAPIKeyName, apiKeyRepo.created[1].Name)
}

func TestAPIKeyServiceResolveChatStationAPIKeyReplacesInactiveAutomaticKey(t *testing.T) {
	limit := 2.5
	groupID := int64(88)
	groupRepo := &lobeHubSSOGroupRepoStub{groups: []Group{{
		ID: groupID, Name: "hidden-free-chat", Platform: PlatformOpenAI,
		Status: StatusActive, SubscriptionType: SubscriptionTypeStandard,
		IsHidden: true, IsFree: true, DailyFreeLimitUSD: &limit, ChatStationOnly: true,
	}}}
	userRepo := &lobeHubSSOUserRepoStub{user: &User{ID: 1001, Status: StatusActive}}
	apiKeyRepo := &lobeHubSSOAPIKeyRepoStub{created: []APIKey{{
		ID: 1, UserID: userRepo.user.ID, Name: ChatStationAPIKeyName, GroupID: &groupID,
		Key: "sk-disabled", Status: StatusAPIKeyDisabled,
	}}}
	cfg := &config.Config{}
	cfg.Default.APIKeyPrefix = "sk-test-"

	svc := NewAPIKeyService(apiKeyRepo, userRepo, groupRepo, nil, nil, nil, cfg)
	key, err := svc.ResolveChatStationAPIKey(context.Background(), userRepo.user.ID, PlatformOpenAI)
	require.NoError(t, err)
	require.NotEqual(t, "sk-disabled", key)
	require.Len(t, apiKeyRepo.created, 2)
	require.Equal(t, StatusAPIKeyActive, apiKeyRepo.created[1].Status)
}

func TestAPIKeyServiceValidateDailyFreeAllowance(t *testing.T) {
	limit := 1.0
	repo := &lobeHubSSOAPIKeyRepoStub{dailyUsage: limit}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})

	err := svc.ValidateDailyFreeAllowance(context.Background(), &APIKey{
		User:  &User{ID: 1001},
		Group: &Group{ID: 88, IsFree: true, SubscriptionType: SubscriptionTypeStandard, DailyFreeLimitUSD: &limit},
	})
	require.ErrorIs(t, err, ErrDailyFreeLimitExceeded)
}

func TestLobeHubSSOExchangeDoesNotCreateLegacySubscriptionOrAPIKey(t *testing.T) {
	userRepo := &lobeHubSSOUserRepoStub{
		user: &User{ID: 1001, Email: "new@example.com", Status: StatusActive},
	}
	apiKeyRepo := &lobeHubSSOAPIKeyRepoStub{}
	cfg := &config.Config{}
	cfg.LobeHubSSO.Enabled = true
	cfg.LobeHubSSO.SharedSecret = "0123456789abcdef0123456789abcdef"
	cfg.LobeHubSSO.AutoCreateAPIKeys = true
	cfg.LobeHubSSO.APIBaseURL = "https://api.example.com"
	store := &lobeHubSSOCodeStoreStub{payload: &LobeHubSSOCodePayload{
		UserID: userRepo.user.ID, ExpiresAt: time.Now().Add(time.Minute),
	}}
	apiKeyService := NewAPIKeyService(apiKeyRepo, userRepo, nil, nil, nil, nil, cfg)
	svc := NewLobeHubSSOService(cfg, store, userRepo, nil, nil, apiKeyService, nil, nil)

	result, err := svc.Exchange(context.Background(), LobeHubSSOExchangeInput{
		Code: "one-time-code", SharedSecret: cfg.LobeHubSSO.SharedSecret,
	})
	require.NoError(t, err)
	require.Equal(t, userRepo.user.ID, result.User.ID)
	require.Equal(t, "https://api.example.com", result.APIBaseURL)
	require.Empty(t, result.Keys)
	require.Empty(t, apiKeyRepo.created)
	require.False(t, userRepo.addGroupCalled)
}
