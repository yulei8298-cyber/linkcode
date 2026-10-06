package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type adminPackageRepoStub struct {
	service.PackageRepository
	filter service.AdminPackageFilter
}

func (s *adminPackageRepoStub) AdminListPackages(_ context.Context, f service.AdminPackageFilter) ([]service.AdminPackageRow, int64, error) {
	s.filter = f
	return []service.AdminPackageRow{{
		UserPackage: service.UserPackage{ID: 3, UserID: 9, Cycle: service.PackageCycleWeek, QuotaUSD: 10, UsedUSD: 4, Status: service.PackageStatusActive},
		UserEmail:   "a@example.com", GroupName: "GPT-Pro",
	}}, 1, nil
}

func (s *adminPackageRepoStub) AdminPackageStats(context.Context, time.Time) (*service.AdminPackageStats, error) {
	return &service.AdminPackageStats{Total: 5, Active: 3, WindowDays: service.AdminPackageSalesWindowDays}, nil
}

type adminPackageSettingStub struct{ service.SettingRepository }

func (adminPackageSettingStub) GetValue(context.Context, string) (string, error) {
	return "", service.ErrSettingNotFound
}

func newAdminPackageRouter(repo *adminPackageRepoStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewPackageHandler(service.NewPackageService(repo, adminPackageSettingStub{}, nil, nil))
	r := gin.New()
	r.GET("/user-packages", h.ListUserPackages)
	r.GET("/user-packages/stats", h.UserPackageStats)
	return r
}

func serveAdminPackage(r *gin.Engine, path string) (*httptest.ResponseRecorder, map[string]any) {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w, body
}

func TestAdminPackageHandler_ListPassesFiltersAndPaginates(t *testing.T) {
	repo := &adminPackageRepoStub{}
	w, body := serveAdminPackage(newAdminPackageRouter(repo), "/user-packages?keyword=alice&status=active&cycle=week&group_id=2&page=2&page_size=10")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, service.AdminPackageFilter{Keyword: "alice", Status: "active", Cycle: "week", GroupID: 2, Page: 2, PageSize: 10}, repo.filter)

	data := body["data"].(map[string]any)
	require.EqualValues(t, 1, data["total"])
	item := data["items"].([]any)[0].(map[string]any)
	require.Equal(t, "a@example.com", item["user_email"])
	require.Equal(t, "GPT-Pro", item["group_name"])
	require.EqualValues(t, 6, item["remaining_usd"])
}

func TestAdminPackageHandler_RejectsBadFilters(t *testing.T) {
	r := newAdminPackageRouter(&adminPackageRepoStub{})
	for _, path := range []string{"/user-packages?status=bogus", "/user-packages?cycle=year", "/user-packages?group_id=abc"} {
		w, _ := serveAdminPackage(r, path)
		require.Equal(t, http.StatusBadRequest, w.Code, path)
	}
}

func TestAdminPackageHandler_Stats(t *testing.T) {
	w, body := serveAdminPackage(newAdminPackageRouter(&adminPackageRepoStub{}), "/user-packages/stats")
	require.Equal(t, http.StatusOK, w.Code)
	data := body["data"].(map[string]any)
	require.EqualValues(t, 5, data["total"])
	require.EqualValues(t, 3, data["active"])
	require.EqualValues(t, service.AdminPackageSalesWindowDays, data["window_days"])
}
