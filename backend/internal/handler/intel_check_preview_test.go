package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIntelCheckPreview_独立CSP且不读取配置或数据(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.SecurityHeaders(config.CSPConfig{
		Enabled: true, Policy: "default-src 'self'; script-src 'self' __CSP_NONCE__; frame-src 'self'",
	}, nil))
	// service 为 nil，任何数据库或配置查询都会使本用例失败。
	h := NewIntelCheckHandler(nil)
	router.GET("/api/v1/public/intel-check/preview", h.Preview)
	router.GET("/portal/intel-check", func(c *gin.Context) { c.String(http.StatusOK, "控制台") })

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/public/intel-check/preview?html=不得反射的内容", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, string(intelCheckPreviewHTML), rec.Body.String())
	require.NotContains(t, rec.Body.String(), "不得反射的内容")
	require.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Equal(t, intelCheckPreviewCSP, rec.Header().Get("Content-Security-Policy"))
	require.Contains(t, intelCheckPreviewCSP, "sandbox allow-scripts")
	require.Contains(t, intelCheckPreviewCSP, "connect-src 'none'")
	require.NotContains(t, intelCheckPreviewCSP, "allow-same-origin")
	require.NotContains(t, intelCheckPreviewCSP, "nonce-")
	require.Equal(t, "SAMEORIGIN", rec.Header().Get("X-Frame-Options"))
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))

	parent := httptest.NewRecorder()
	router.ServeHTTP(parent, httptest.NewRequest(http.MethodGet, "/portal/intel-check", nil))
	require.Contains(t, parent.Header().Get("Content-Security-Policy"), "script-src 'self' 'nonce-")
	require.NotContains(t, parent.Header().Get("Content-Security-Policy"), "script-src 'unsafe-inline'")
	require.Equal(t, "DENY", parent.Header().Get("X-Frame-Options"))
}
