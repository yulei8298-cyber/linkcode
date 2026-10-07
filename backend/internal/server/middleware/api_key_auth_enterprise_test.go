//go:build unit

package middleware

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type enterpriseAuthRepoStub struct {
	service.PackageRepository
	state service.PackageGroupState
	spent float64
}

func (s *enterpriseAuthRepoStub) GetGroupState(context.Context, int64, int64, time.Time) (*service.PackageGroupState, error) {
	st := s.state
	return &st, nil
}
func (s *enterpriseAuthRepoStub) EnterpriseSpent(context.Context, int64) (float64, error) {
	return s.spent, nil
}
func (s *enterpriseAuthRepoStub) GetEnterpriseOverride(context.Context, int64) (string, error) {
	return "", nil
}

type enterpriseAuthSettingStub struct{ service.SettingRepository }

func (enterpriseAuthSettingStub) GetValue(_ context.Context, key string) (string, error) {
	if key == service.SettingKeyEnterpriseSettings {
		return `{"enabled":true,"threshold":3000,"group_rates":[{"group_id":7,"multiplier":0.28}]}`, nil
	}
	return "", service.ErrSettingNotFound
}

type enterpriseSeen struct {
	pkg  *service.PackageBilling
	rate *service.EnterpriseRate
}

// newEnterpriseAuthRouter 余额充足的用户 + 普通分组 7（配了企业倍率 0.28）。
func newEnterpriseAuthRouter(repo *enterpriseAuthRepoStub, seen *enterpriseSeen) *gin.Engine {
	gin.SetMode(gin.TestMode)
	groupID := int64(7)
	user := &service.User{ID: 10, Role: service.RoleUser, Status: service.StatusActive, Balance: 100, Concurrency: 3, PackageConcurrency: 4}
	group := &service.Group{ID: groupID, Name: "Claude", Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 0.3, Hydrated: true}
	apiKey := &service.APIKey{ID: 105, UserID: user.ID, Key: "pkg-key", Status: service.StatusActive, User: user, GroupID: &groupID, Group: group}
	apiKeyRepo := &stubApiKeyRepo{
		getByKey: func(ctx context.Context, key string) (*service.APIKey, error) {
			if key != apiKey.Key {
				return nil, service.ErrAPIKeyNotFound
			}
			clone := *apiKey
			userClone := *user
			clone.User = &userClone
			return &clone, nil
		},
	}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	apiKeyService := service.NewAPIKeyService(apiKeyRepo, nil, nil, nil, nil, nil, cfg)
	apiKeyService.SetPackageService(service.NewPackageService(repo, enterpriseAuthSettingStub{}, nil, nil))

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg)))
	router.GET("/t", func(c *gin.Context) {
		seen.pkg = service.PackageBillingFromContext(c.Request.Context())
		seen.rate = service.EnterpriseRateFromContext(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func TestAPIKeyAuthEnterpriseRate_OnlyForBalanceRequests(t *testing.T) {
	// 企业用户、没有套餐：按量请求带上企业倍率。
	var seen enterpriseSeen
	w := servePackageAuth(newEnterpriseAuthRouter(&enterpriseAuthRepoStub{spent: 5000}, &seen))
	require.Equal(t, http.StatusOK, w.Code)
	require.Nil(t, seen.pkg)
	require.Equal(t, &service.EnterpriseRate{GroupID: 7, Multiplier: 0.28}, seen.rate)

	// 企业用户、有可用套餐：走套餐，不带企业倍率（套餐按原倍率）。
	seen = enterpriseSeen{}
	w = servePackageAuth(newEnterpriseAuthRouter(&enterpriseAuthRepoStub{spent: 5000, state: service.PackageGroupState{Usable: 1}}, &seen))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, seen.pkg)
	require.Nil(t, seen.rate)

	// 不是企业用户：不带企业倍率。
	seen = enterpriseSeen{}
	w = servePackageAuth(newEnterpriseAuthRouter(&enterpriseAuthRepoStub{spent: 100}, &seen))
	require.Equal(t, http.StatusOK, w.Code)
	require.Nil(t, seen.rate)
}
