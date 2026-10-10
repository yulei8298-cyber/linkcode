//go:build integration

package repository

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *GroupRepoSuite) createRealtimeFixtureGroup(name string) *service.Group {
	group := &service.Group{
		Name:             name,
		Platform:         service.PlatformOpenAI,
		RateMultiplier:   1.0,
		Status:           service.StatusActive,
		SubscriptionType: service.SubscriptionTypeStandard,
	}
	s.Require().NoError(s.repo.Create(s.ctx, group), "create group")
	return group
}

func (s *GroupRepoSuite) TestListGroupRealtimeRPM_CountsOnlyLogsInsideTheWindowPerGroup() {
	client := s.tx.Client()
	user := mustCreateUser(s.T(), client, &service.User{Email: "realtime-rpm@test.com"})
	account := mustCreateAccount(s.T(), client, &service.Account{Name: "realtime-rpm-acc"})
	busy := s.createRealtimeFixtureGroup("realtime-rpm-busy")
	quiet := s.createRealtimeFixtureGroup("realtime-rpm-quiet")
	idle := s.createRealtimeFixtureGroup("realtime-rpm-idle")
	busyKey := mustCreateApiKey(s.T(), client, &service.APIKey{UserID: user.ID, Key: "sk-rt-busy", GroupID: &busy.ID})
	quietKey := mustCreateApiKey(s.T(), client, &service.APIKey{UserID: user.ID, Key: "sk-rt-quiet", GroupID: &quiet.ID})

	now := time.Now()
	add := func(requestID string, keyID, groupID int64, age time.Duration) {
		_, err := client.UsageLog.Create().
			SetUserID(user.ID).SetAPIKeyID(keyID).SetAccountID(account.ID).SetGroupID(groupID).
			SetRequestID(requestID).SetModel("gpt-5").SetCreatedAt(now.Add(-age)).
			Save(s.ctx)
		s.Require().NoError(err, requestID)
	}
	add("rt-busy-1", busyKey.ID, busy.ID, 5*time.Second)
	add("rt-busy-2", busyKey.ID, busy.ID, 30*time.Second)
	add("rt-busy-3", busyKey.ID, busy.ID, 55*time.Second)
	add("rt-busy-old", busyKey.ID, busy.ID, 3*time.Minute) // 窗口外
	add("rt-quiet-1", quietKey.ID, quiet.ID, 10*time.Second)
	add("rt-quiet-old", quietKey.ID, quiet.ID, 10*time.Minute) // 窗口外

	items, err := s.repo.ListGroupRealtimeRPM(s.ctx, 60*time.Second)
	s.Require().NoError(err)

	byGroup := make(map[int64]int, len(items))
	for _, item := range items {
		byGroup[item.GroupID] = item.RPM
	}
	s.Require().Equal(3, byGroup[busy.ID], "only logs inside the 60s window count")
	s.Require().Equal(1, byGroup[quiet.ID])
	_, listed := byGroup[idle.ID]
	s.Require().False(listed, "groups without recent requests are omitted")

	wide, err := s.repo.ListGroupRealtimeRPM(s.ctx, 5*time.Minute)
	s.Require().NoError(err)
	for _, item := range wide {
		if item.GroupID == busy.ID {
			s.Require().Equal(4, item.RPM, "a wider window includes the older log")
		}
	}
}

func (s *GroupRepoSuite) TestListActiveAPIKeyOwnersByGroup_ReturnsLiveKeysWithOwners() {
	client := s.tx.Client()
	alice := mustCreateUser(s.T(), client, &service.User{Email: "owners-alice@test.com", Username: "alice"})
	bob := mustCreateUser(s.T(), client, &service.User{Email: "owners-bob@test.com"})
	target := s.createRealtimeFixtureGroup("owners-target")
	other := s.createRealtimeFixtureGroup("owners-other")

	aliceKey := mustCreateApiKey(s.T(), client, &service.APIKey{UserID: alice.ID, Key: "sk-owners-a", Name: "alice-main", GroupID: &target.ID})
	bobKey := mustCreateApiKey(s.T(), client, &service.APIKey{UserID: bob.ID, Key: "sk-owners-b", Name: "bob-key", GroupID: &target.ID})
	mustCreateApiKey(s.T(), client, &service.APIKey{UserID: alice.ID, Key: "sk-owners-other", Name: "elsewhere", GroupID: &other.ID})
	removed := mustCreateApiKey(s.T(), client, &service.APIKey{UserID: bob.ID, Key: "sk-owners-removed", Name: "removed", GroupID: &target.ID})
	s.Require().NoError(client.APIKey.DeleteOneID(removed.ID).Exec(s.ctx), "soft delete key")

	owners, err := s.repo.ListActiveAPIKeyOwnersByGroup(s.ctx, target.ID)
	s.Require().NoError(err)

	s.Require().Equal([]service.GroupAPIKeyOwner{
		{APIKeyID: aliceKey.ID, APIKeyName: "alice-main", UserID: alice.ID, Email: "owners-alice@test.com", Username: "alice"},
		{APIKeyID: bobKey.ID, APIKeyName: "bob-key", UserID: bob.ID, Email: "owners-bob@test.com", Username: ""},
	}, owners)

	none, err := s.repo.ListActiveAPIKeyOwnersByGroup(s.ctx, 999999999)
	s.Require().NoError(err)
	s.Require().Empty(none)
}
