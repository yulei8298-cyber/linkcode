package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type packageHandlerRepoStub struct {
	service.PackageRepository
	frozenID        int64
	enterpriseSpent float64
	freezeDays      []service.PackageFreezeDay
}

func (s *packageHandlerRepoStub) ListPlans(context.Context, *int64, bool) ([]service.PackagePlan, error) {
	return []service.PackagePlan{{ID: 1, GroupID: 7, Name: "摸鱼周卡", Cycle: service.PackageCycleWeek, Tier: 1, Price: 95, QuotaUSD: 120, ForSale: true}}, nil
}

func (s *packageHandlerRepoStub) ListFreezeDays(context.Context, time.Time, time.Time) ([]service.PackageFreezeDay, error) {
	return s.freezeDays, nil
}

func (s *packageHandlerRepoStub) EnterpriseSpent(context.Context, int64) (float64, error) {
	return s.enterpriseSpent, nil
}

func (s *packageHandlerRepoStub) GetEnterpriseOverride(context.Context, int64) (string, error) {
	return "", nil
}

func (s *packageHandlerRepoStub) FreezePackage(_ context.Context, id, userID int64, now time.Time, _ service.PackageFreezeCaps) (*service.UserPackage, error) {
	s.frozenID = id
	return &service.UserPackage{ID: id, UserID: userID, GroupID: 7, Status: service.PackageStatusFrozen, FrozenAt: &now}, nil
}

type packageHandlerGroupStub struct{ service.GroupRepository }

func (packageHandlerGroupStub) GetByID(context.Context, int64) (*service.Group, error) {
	return &service.Group{ID: 7, Name: "Claude", Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 0.3}, nil
}

type packageHandlerUserStub struct{ service.UserRepository }

func (packageHandlerUserStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, PackageConcurrency: 6}, nil
}

type packageHandlerSettingStub struct{ service.SettingRepository }

func (packageHandlerSettingStub) GetValue(context.Context, string) (string, error) {
	return "", service.ErrSettingNotFound
}

func newPackageTestRouter(repo *packageHandlerRepoStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := service.NewPackageService(repo, packageHandlerSettingStub{}, packageHandlerGroupStub{}, packageHandlerUserStub{})
	h := NewPackageHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42, Concurrency: 3})
		c.Next()
	})
	r.GET("/user/enterprise", h.EnterpriseStatus)
	r.GET("/packages/shop", h.Shop)
	r.GET("/packages/calendar", h.Calendar)
	r.POST("/packages/:id/freeze", h.Freeze)
	return r
}

func servePackageRequest(r *gin.Engine, method, path string) (*httptest.ResponseRecorder, map[string]any) {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w, body
}

func TestPackageHandler_ShopReturnsPlansNoticeAndConcurrency(t *testing.T) {
	w, body := servePackageRequest(newPackageTestRouter(&packageHandlerRepoStub{}), http.MethodGet, "/packages/shop")
	require.Equal(t, http.StatusOK, w.Code)
	data := body["data"].(map[string]any)
	groups := data["groups"].([]any)
	require.Len(t, groups, 1)
	group := groups[0].(map[string]any)
	require.Equal(t, "Claude", group["group_name"])
	require.Len(t, group["plans"].([]any), 1)
	notice := data["notice"].(map[string]any)
	require.EqualValues(t, 1, notice["version"])
	require.Contains(t, notice["text"], "立即生效")
	require.EqualValues(t, 6, data["package_concurrency"], "返回当前用户的套餐并发")
	require.EqualValues(t, service.DefaultPackageMaxFreezeDayWeek, data["max_freeze_days_week"])
	require.EqualValues(t, service.DefaultPackageMaxFreezeDayMonth, data["max_freeze_days_month"])
}

func TestPackageHandler_CalendarRejectsBadMonth(t *testing.T) {
	w, body := servePackageRequest(newPackageTestRouter(&packageHandlerRepoStub{}), http.MethodGet, "/packages/calendar?month=2026/10")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "月份格式应为 YYYY-MM", body["message"])

	w, body = servePackageRequest(newPackageTestRouter(&packageHandlerRepoStub{}), http.MethodGet, "/packages/calendar?month=2026-10")
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, body["data"].([]any), 31)
}

func TestPackageHandler_FreezeValidatesIDAndCalendar(t *testing.T) {
	repo := &packageHandlerRepoStub{}
	w, body := servePackageRequest(newPackageTestRouter(repo), http.MethodPost, "/packages/abc/freeze")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "套餐 ID 不合法", body["message"])

	// 把今天设为后台自定义可冻结日，使用例与运行日期无关。
	repo.freezeDays = []service.PackageFreezeDay{{Day: timezone.Now(), Name: "测试日", Kind: service.PackageDayKindOff, Source: service.PackageDaySourceManual}}
	w, _ = servePackageRequest(newPackageTestRouter(repo), http.MethodPost, "/packages/11/freeze")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(11), repo.frozenID)
}

func TestPackageHandler_EnterpriseStatusUsesCurrentUser(t *testing.T) {
	repo := &packageHandlerRepoStub{enterpriseSpent: 3500}
	w, body := servePackageRequest(newPackageTestRouter(repo), http.MethodGet, "/user/enterprise")
	require.Equal(t, http.StatusOK, w.Code)
	data := body["data"].(map[string]any)
	require.Equal(t, true, data["enterprise"])
	require.Equal(t, "auto", data["mode"])
	require.EqualValues(t, 3500, data["total"])

	repo = &packageHandlerRepoStub{enterpriseSpent: 100}
	_, body = servePackageRequest(newPackageTestRouter(repo), http.MethodGet, "/user/enterprise")
	require.Equal(t, false, body["data"].(map[string]any)["enterprise"])
}
