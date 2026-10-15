//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var errProxyAuth407 = errors.New(`Post "https://api.x.ai/v1/responses": Proxy Authentication Required`)

func withFastProxyAuthBackoff(t *testing.T) {
	t.Helper()
	prev := proxyAuthRetryBackoffs
	proxyAuthRetryBackoffs = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { proxyAuthRetryBackoffs = prev })
}

func newReplayableTestRequest(t *testing.T, payload string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.x.ai/v1/responses", bytes.NewReader([]byte(payload)))
	require.NoError(t, err)
	return req
}

func TestDoWithProxyAuthRetry_RecoversAfterTransient407(t *testing.T) {
	withFastProxyAuthBackoff(t)
	req := newReplayableTestRequest(t, `{"model":"grok-4.7"}`)

	var bodies []string
	calls := 0
	resp, err := doWithProxyAuthRetry(req, 612, func(r *http.Request) (*http.Response, error) {
		calls++
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if calls == 1 {
			return nil, errProxyAuth407
		}
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 2, calls)
	require.Equal(t, []string{`{"model":"grok-4.7"}`, `{"model":"grok-4.7"}`}, bodies, "重试必须重放完整请求体")
}

func TestDoWithProxyAuthRetry_GivesUpAfterConfiguredAttempts(t *testing.T) {
	withFastProxyAuthBackoff(t)
	req := newReplayableTestRequest(t, `{}`)

	calls := 0
	_, err := doWithProxyAuthRetry(req, 612, func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errProxyAuth407
	})

	require.Error(t, err)
	require.Equal(t, 1+proxyAuthRetryAttempts, calls)
}

func TestDoWithProxyAuthRetry_DoesNotRetryOtherErrors(t *testing.T) {
	withFastProxyAuthBackoff(t)
	req := newReplayableTestRequest(t, `{}`)

	calls := 0
	_, err := doWithProxyAuthRetry(req, 612, func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("dial tcp 1.2.3.4:443: connect: connection refused")
	})

	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func TestDoWithProxyAuthRetry_SkipsRetryWhenBodyNotReplayable(t *testing.T) {
	withFastProxyAuthBackoff(t)
	req, err := http.NewRequest(http.MethodPost, "https://api.x.ai/v1/responses", io.NopCloser(bytes.NewReader([]byte(`{}`))))
	require.NoError(t, err)
	require.Nil(t, req.GetBody)

	calls := 0
	_, err = doWithProxyAuthRetry(req, 612, func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errProxyAuth407
	})

	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func TestDoWithProxyAuthRetry_StopsWhenContextCanceled(t *testing.T) {
	prev := proxyAuthRetryBackoffs
	proxyAuthRetryBackoffs = []time.Duration{time.Hour, time.Hour}
	t.Cleanup(func() { proxyAuthRetryBackoffs = prev })

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.x.ai/v1/responses", bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)

	calls := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = doWithProxyAuthRetry(req, 612, func(*http.Request) (*http.Response, error) {
			calls++
			return nil, errProxyAuth407
		})
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("上下文取消后重试仍在等待")
	}
	require.Equal(t, 1, calls)
}

func TestTransportErrorTempUnschedDuration(t *testing.T) {
	require.Equal(t, proxyAuthTempUnschedDuration,
		transportErrorTempUnschedDuration(`upstream transport error (proxy/network): Post "https://api.x.ai/v1/responses": Proxy Authentication Required`, 10*time.Minute))
	require.Equal(t, 10*time.Minute,
		transportErrorTempUnschedDuration("socks connect: username/password authentication failed", 10*time.Minute))
}

// 407 经完整处理链：账号只冷却 1 分钟（而非 10 分钟），且仍然切换到其他账号。
func TestHandleOpenAIUpstreamTransportError_ProxyAuth407UsesShortCooldown(t *testing.T) {
	repo := &openaiTransportAccountRepoStub{}
	svc := &OpenAIGatewayService{accountRepo: repo}
	account := &Account{ID: 612, Name: "grok-612", Platform: PlatformGrok}
	c, _ := newOpenAITransportErrTestContext()

	before := time.Now()
	retErr := svc.handleOpenAIUpstreamTransportError(context.Background(), c, account, errProxyAuth407, true)
	after := time.Now()

	var fo *UpstreamFailoverError
	require.True(t, errors.As(retErr, &fo))
	require.Len(t, repo.tempUnschedCalls, 1)
	until := repo.tempUnschedCalls[0].until
	require.True(t, until.After(before.Add(proxyAuthTempUnschedDuration-time.Second)))
	require.True(t, until.Before(after.Add(proxyAuthTempUnschedDuration+time.Second)))
	require.True(t, until.Before(before.Add(openAITransportErrorTempUnschedDuration/2)), "不应沿用 10 分钟冷却")
}
