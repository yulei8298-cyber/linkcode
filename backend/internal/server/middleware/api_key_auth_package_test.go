//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type packageStateRepoStub struct {
	service.PackageRepository
	state service.PackageGroupState
}

func (s *packageStateRepoStub) GetGroupState(context.Context, int64, int64, time.Time) (*service.PackageGroupState, error) {
	st := s.state
	return &st, nil
}

// newPackageAuthRouter 余额为 0 的用户 + 普通分组，按 state 模拟其套餐概况。
func newPackageAuthRouter(t *testing.T, state service.PackageGroupState, seen **service.PackageBilling) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	groupID := int64(7)
	user := &service.User{ID: 10, Role: service.RoleUser, Status: service.StatusActive, Balance: 0, Concurrency: 3, PackageConcurrency: 4}
	group := &service.Group{ID: groupID, Name: "Claude", Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, Hydrated: true}
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
	apiKeyService.SetPackageService(service.NewPackageService(&packageStateRepoStub{state: state}, nil, nil, nil))

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(apiKeyService, nil, cfg)))
	router.GET("/t", func(c *gin.Context) {
		*seen = service.PackageBillingFromContext(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func servePackageAuth(router *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set("x-api-key", "pkg-key")
	router.ServeHTTP(w, req)
	return w
}

func TestAPIKeyAuthPackageAllowsZeroBalance(t *testing.T) {
	var seen *service.PackageBilling
	w := servePackageAuth(newPackageAuthRouter(t, service.PackageGroupState{Usable: 1}, &seen))

	require.Equal(t, http.StatusOK, w.Code, "有可用套餐时余额为 0 也放行")
	require.NotNil(t, seen)
	require.Equal(t, int64(7), seen.GroupID)
	require.Equal(t, 4, seen.Concurrency, "套餐并发取用户的套餐并发")
}

func TestAPIKeyAuthPackageFrozenWithoutBalanceRejectsInChinese(t *testing.T) {
	var seen *service.PackageBilling
	w := servePackageAuth(newPackageAuthRouter(t, service.PackageGroupState{Frozen: 1}, &seen))

	require.Equal(t, http.StatusForbidden, w.Code)
	requireAPIKeyAuthError(t, w, "PACKAGE_FROZEN", "套餐已冻结，且余额不足。请在「我的套餐」中解冻，或充值后再试。")
	require.Nil(t, seen)
}

func TestAPIKeyAuthPackageNoneFallsBackToBalance(t *testing.T) {
	var seen *service.PackageBilling
	w := servePackageAuth(newPackageAuthRouter(t, service.PackageGroupState{}, &seen))

	require.Equal(t, http.StatusForbidden, w.Code)
	requireAPIKeyAuthError(t, w, "INSUFFICIENT_BALANCE", "Insufficient account balance")
}
