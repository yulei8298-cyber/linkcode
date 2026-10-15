//go:build unit

package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 回归：后台保存分组「模型列表 / 展示到模型广场」时，请求里的 models_list_config
// 必须传到服务层。0.2.4 同步后这个字段被静默忽略，广场清单一直无法更新。

func newStandardModeGroupRouter(svc *stubAdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewGroupHandlerWithConfig(svc, nil, nil, &config.Config{RunMode: config.RunModeStandard})
	r := gin.New()
	r.POST("/groups", h.Create)
	r.PUT("/groups/:id", h.Update)
	return r
}

func performGroupJSON(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	return rec
}

func TestGroupHandler_Update_PassesModelsListConfigToService(t *testing.T) {
	svc := newStubAdminService()
	r := newStandardModeGroupRouter(svc)

	rec := performGroupJSON(t, r, http.MethodPut, "/groups/36",
		`{"models_list_config":{"enabled":false,"models":["grok-4.7","grok-4.6"],"plaza_enabled":true}}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, svc.updatedGroups, 1)
	cfg := svc.updatedGroups[0].ModelsListConfig
	require.NotNil(t, cfg, "models_list_config 必须传到服务层")
	require.False(t, cfg.Enabled)
	require.Equal(t, []string{"grok-4.7", "grok-4.6"}, cfg.Models)
	require.NotNil(t, cfg.PlazaEnabled)
	require.True(t, *cfg.PlazaEnabled)
}

func TestGroupHandler_Update_OmittedModelsListConfigStaysNil(t *testing.T) {
	svc := newStubAdminService()
	r := newStandardModeGroupRouter(svc)

	rec := performGroupJSON(t, r, http.MethodPut, "/groups/36", `{"name":"Grok"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, svc.updatedGroups, 1)
	require.Nil(t, svc.updatedGroups[0].ModelsListConfig, "未携带时为 nil，服务层据此不改动")
}

func TestGroupHandler_Create_PassesModelsListConfigToService(t *testing.T) {
	svc := newStubAdminService()
	r := newStandardModeGroupRouter(svc)

	rec := performGroupJSON(t, r, http.MethodPost, "/groups",
		`{"name":"g","platform":"grok","models_list_config":{"enabled":true,"models":["grok-4.7"]}}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, svc.createdGroups, 1)
	require.True(t, svc.createdGroups[0].ModelsListConfig.Enabled)
	require.Equal(t, []string{"grok-4.7"}, svc.createdGroups[0].ModelsListConfig.Models)
}
