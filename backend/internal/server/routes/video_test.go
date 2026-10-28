//go:build unit

package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newVideoRoutesTestRouter(platform string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	openAI := &handler.OpenAIGatewayHandler{}
	RegisterGatewayRoutes(router,
		&handler.Handlers{
			Gateway:       &handler.GatewayHandler{},
			OpenAIGateway: openAI,
			AsyncImage:    handler.NewAsyncImageHandler(nil, nil),
			Video:         handler.NewVideoHandler(openAI, nil),
		},
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			groupID := int64(1)
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{Platform: platform}})
			c.Next()
		}),
		nil, nil, nil, nil, nil,
		&config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20, TextMaxBodySize: 1 << 20}},
	)
	return router
}

func TestVideoOnlyGroupRejectsTextEndpoints(t *testing.T) {
	router := newVideoRoutesTestRouter(service.PlatformVideo)
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/v1/chat/completions"},
		{http.MethodPost, "/v1/messages"},
		{http.MethodPost, "/v1/responses"},
		{http.MethodPost, "/v1/images/generations"},
		{http.MethodPost, "/chat/completions"},
		{http.MethodPost, "/v1/embeddings"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"model":"wan3.0-video","messages":[]}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code, tc.path)
		require.Contains(t, w.Body.String(), "only supports video", tc.path)
	}
}

func TestVideoRoutesRegisteredForNativeAndNewAPI(t *testing.T) {
	router := newVideoRoutesTestRouter(service.PlatformVideo)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/video/unknown/v1/videos", strings.NewReader(`{}`)))
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "Unknown video protocol")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/video/kling/v1/unsupported", nil))
	require.Contains(t, w.Body.String(), "Unsupported native video endpoint")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/video-files/vt_x?expires=1&signature=bad", nil))
	require.NotEqual(t, http.StatusUnauthorized, w.Code, "签名下载链接不经过 API Key 鉴权")
}

func TestVideoLookupNonVideoTaskOnOpenAIGroupStaysUnsupported(t *testing.T) {
	router := newVideoRoutesTestRouter(service.PlatformOpenAI)
	for _, path := range []string{"/v1/videos/request-1", "/v1/video/generations/request-1"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusNotFound, w.Code, path)
		require.Contains(t, w.Body.String(), "Videos API is not supported for this platform", path)
	}
}

func TestIsVideoRoutePath(t *testing.T) {
	require.True(t, isVideoRoutePath("/v1/videos/:request_id"))
	require.True(t, isVideoRoutePath("/video/:protocol/*path"))
	require.True(t, isVideoRoutePath("/api/v3/contents/generations/tasks"))
	require.False(t, isVideoRoutePath("/v1/chat/completions"))
}
