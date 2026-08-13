package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type imageProxyStoreStub struct {
	mu     sync.Mutex
	values map[string]string
	ttls   map[string]time.Duration
	setErr error
}

func (s *imageProxyStoreStub) SetImageProxyURL(_ context.Context, token, upstreamURL string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.setErr != nil {
		return s.setErr
	}
	if s.values == nil {
		s.values = make(map[string]string)
		s.ttls = make(map[string]time.Duration)
	}
	s.values[token] = upstreamURL
	s.ttls[token] = ttl
	return nil
}

func TestRewriteImageResponseURLsDoesNotLeakUpstreamWhenRedisFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &imageProxyStoreStub{setErr: errors.New("redis unavailable")}
	svc := &OpenAIGatewayService{cache: &imageProxyGatewayCacheStub{imageProxyStoreStub: store}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.linkcode.site/v1/images/generations", nil)
	body := []byte(`{"data":[{"url":"https://private-upstream.example/image.png"}]}`)

	rewritten := svc.rewriteImageResponseURLsFailOpen(context.Background(), c, body)

	require.Equal(t, "https://api.linkcode.site/v1/images/proxy/unavailable", gjson.GetBytes(rewritten, "data.0.url").String())
	require.NotContains(t, string(rewritten), "private-upstream.example")
}

func TestRewriteImageResponseURLsDoesNotLeakUpstreamWithoutStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.linkcode.site/v1/images/generations", nil)
	body := []byte(`{"data":[{"url":"https://private-upstream.example/image.png"}]}`)

	rewritten := svc.rewriteImageResponseURLsFailOpen(context.Background(), c, body)

	require.Equal(t, "https://api.linkcode.site/v1/images/proxy/unavailable", gjson.GetBytes(rewritten, "data.0.url").String())
	require.NotContains(t, string(rewritten), "private-upstream.example")
}

func (s *imageProxyStoreStub) GetImageProxyURL(_ context.Context, token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[token]
	if !ok {
		return "", ErrImageProxyURLNotFound
	}
	return value, nil
}

type imageProxyGatewayCacheStub struct {
	GatewayCache
	*imageProxyStoreStub
}

type imageProxyRoundTripper func(*http.Request) (*http.Response, error)

func (f imageProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRewriteImageResponseURLsUsesOpaqueGatewayURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &imageProxyStoreStub{}
	svc := &OpenAIGatewayService{cache: &imageProxyGatewayCacheStub{imageProxyStoreStub: store}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "http://internal/v1/images/generations", nil)
	c.Request.Host = "api.linkcode.site"
	c.Request.Header.Set("X-Forwarded-Proto", "https")

	upstreamURL := "https://usawest.up.railway.app/v1/images/proxy?source=inline&token=secret"
	body := []byte(`{"created":1,"data":[{"url":"` + upstreamURL + `","revised_prompt":"kept"}]}`)
	rewritten, err := svc.rewriteImageResponseURLs(context.Background(), c, body)

	require.NoError(t, err)
	proxyURL := gjson.GetBytes(rewritten, "data.0.url").String()
	require.True(t, strings.HasPrefix(proxyURL, "https://api.linkcode.site/v1/images/proxy/"), proxyURL)
	require.NotContains(t, string(rewritten), "railway.app")
	require.NotContains(t, string(rewritten), "token=secret")
	require.Equal(t, "kept", gjson.GetBytes(rewritten, "data.0.revised_prompt").String())
	token := strings.TrimPrefix(proxyURL, "https://api.linkcode.site/v1/images/proxy/")
	require.Len(t, token, 32)
	require.Equal(t, upstreamURL, store.values[token])
	require.Equal(t, 24*time.Hour, store.ttls[token])
}

func TestRewriteImageResponseURLsLeavesInlineImagesUntouched(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &imageProxyStoreStub{}
	svc := &OpenAIGatewayService{cache: &imageProxyGatewayCacheStub{imageProxyStoreStub: store}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.linkcode.site/v1/images/generations", nil)
	body := []byte(`{"data":[{"url":"data:image/png;base64,aGVsbG8=","b64_json":"aGVsbG8="}]}`)

	rewritten, err := svc.rewriteImageResponseURLs(context.Background(), c, body)

	require.NoError(t, err)
	require.Equal(t, body, rewritten)
	require.Empty(t, store.values)
}

func TestRewriteImageSSELineUsesOpaqueGatewayURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &imageProxyStoreStub{}
	svc := &OpenAIGatewayService{cache: &imageProxyGatewayCacheStub{imageProxyStoreStub: store}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "https://api.linkcode.site/v1/images/generations", nil)
	line := []byte("data: {\"type\":\"image_generation.completed\",\"url\":\"https://usawest.up.railway.app/image.png\"}\n\n")

	rewritten := svc.rewriteImageSSELineFailOpen(context.Background(), c, line)

	require.Contains(t, string(rewritten), "https://api.linkcode.site/v1/images/proxy/")
	require.NotContains(t, string(rewritten), "railway.app")
	require.True(t, strings.HasSuffix(string(rewritten), "\n\n"))
}

func TestServeImageProxyStreamsImageWithoutRedirect(t *testing.T) {
	token := strings.Repeat("a", 32)
	store := &imageProxyStoreStub{values: map[string]string{token: "https://images.example.com/result.png"}}
	svc := &OpenAIGatewayService{cache: &imageProxyGatewayCacheStub{imageProxyStoreStub: store}}
	originalClient := imageProxyHTTPClient
	imageProxyHTTPClient = &http.Client{Transport: imageProxyRoundTripper(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "https://images.example.com/result.png", req.URL.String())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader([]byte("\x89PNG\r\n\x1a\nimage"))),
		}, nil
	})}
	t.Cleanup(func() { imageProxyHTTPClient = originalClient })
	recorder := httptest.NewRecorder()

	err := svc.ServeImageProxy(context.Background(), token, recorder)

	require.NoError(t, err)
	require.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	require.Equal(t, "public, max-age=3600", recorder.Header().Get("Cache-Control"))
	require.Empty(t, recorder.Header().Get("Location"))
	require.Equal(t, "\x89PNG\r\n\x1a\nimage", recorder.Body.String())
}

func TestServeImageProxyRejectsNonImageAndMissingToken(t *testing.T) {
	htmlToken := strings.Repeat("h", 32)
	missingToken := strings.Repeat("m", 32)
	store := &imageProxyStoreStub{values: map[string]string{htmlToken: "https://images.example.com/error"}}
	svc := &OpenAIGatewayService{cache: &imageProxyGatewayCacheStub{imageProxyStoreStub: store}}
	originalClient := imageProxyHTTPClient
	imageProxyHTTPClient = &http.Client{Transport: imageProxyRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
			Body:       io.NopCloser(strings.NewReader("<!doctype html>")),
		}, nil
	})}
	t.Cleanup(func() { imageProxyHTTPClient = originalClient })

	err := svc.ServeImageProxy(context.Background(), htmlToken, httptest.NewRecorder())
	require.ErrorContains(t, err, "not an image")
	require.True(t, errors.Is(svc.ServeImageProxy(context.Background(), missingToken, httptest.NewRecorder()), ErrImageProxyURLNotFound))
	require.True(t, errors.Is(svc.ServeImageProxy(context.Background(), "invalid", httptest.NewRecorder()), ErrImageProxyURLNotFound))
}

func TestValidateImageProxyTargetURLRejectsPrivateTargets(t *testing.T) {
	for _, rawURL := range []string{
		"http://127.0.0.1/image.png",
		"http://10.0.0.1/image.png",
		"http://169.254.169.254/latest/meta-data",
		"file:///etc/passwd",
		"https://user:pass@example.com/image.png",
	} {
		require.Error(t, validateImageProxyTargetURL(rawURL), rawURL)
	}
	require.NoError(t, validateImageProxyTargetURL("https://images.example.com/image.png"))
}

func TestAllowedImageProxyContentTypeRejectsSVG(t *testing.T) {
	require.True(t, isAllowedImageProxyContentType("image/png"))
	require.True(t, isAllowedImageProxyContentType("image/webp"))
	require.False(t, isAllowedImageProxyContentType("image/svg+xml"))
}
