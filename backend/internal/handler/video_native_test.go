//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNormalizeVideoNativeAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(set func(h http.Header)) string {
		r := gin.New()
		got := ""
		r.GET("/x", NormalizeVideoNativeAuth(), func(c *gin.Context) { got = c.GetHeader("Authorization") })
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		set(req.Header)
		r.ServeHTTP(httptest.NewRecorder(), req)
		return got
	}
	require.Equal(t, "Bearer sk-1", run(func(h http.Header) { h.Set("Authorization", "Token sk-1") }), "Vidu 的 Token 鉴权")
	require.Equal(t, "Bearer sk-2", run(func(h http.Header) { h.Set("API-KEY", "sk-2") }), "PixVerse 的 API-KEY 鉴权")
	require.Equal(t, "Bearer sk-3", run(func(h http.Header) { h.Set("Authorization", "Bearer sk-3") }))
}

func TestRequestPublicBaseHonorsForwardedHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "http://internal:8080/v1/videos/x", nil)
	require.Equal(t, "http://internal:8080", requestPublicBase(c))
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	c.Request.Header.Set("X-Forwarded-Host", "api.linkcode.site")
	require.Equal(t, "https://api.linkcode.site", requestPublicBase(c))
}

func TestVideoRetryableStatus(t *testing.T) {
	for _, s := range []int{401, 403, 429, 500, 503} {
		require.True(t, videoRetryableStatus(s), s)
	}
	for _, s := range []int{400, 404, 422} {
		require.False(t, videoRetryableStatus(s), s)
	}
}
