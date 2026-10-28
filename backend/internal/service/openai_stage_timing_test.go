package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// stageFieldMap 把 zap 字段还原成 map，便于断言。
func stageFieldMap(fields []zap.Field) map[string]interface{} {
	enc := zapcore.NewMapObjectEncoder()
	for _, field := range fields {
		field.AddTo(enc)
	}
	return enc.Fields
}

// stageTimingUpstream 用真实 http.Client 实现 HTTPUpstream，让 httptrace 在测试里真正触发。
type stageTimingUpstream struct {
	client *http.Client
}

func (u *stageTimingUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return u.client.Do(req)
}

func (u *stageTimingUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.client.Do(req)
}

func newStageTimingTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c
}

func doStageTimingRequest(t *testing.T, ctx context.Context, client *http.Client, url string) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
}

func TestOpenAIStageTimingNilReceiverIsSafe(t *testing.T) {
	var timing *OpenAIStageTiming
	timing.Set(StageBodyRead, 1)
	timing.RecordSince(StagePrepare, time.Now())
	timing.BeginForward(time.Now())
	timing.RecordSinceForward(StageForwardEntry, time.Now())
	timing.UpstreamDispatched(time.Now())
	timing.UpstreamHeaders(time.Now())
	ctx := context.Background()
	require.Equal(t, ctx, timing.AttachUpstreamTrace(ctx))
	require.Nil(t, timing.Fields())
	_, ok := timing.Get(StageBodyRead)
	require.False(t, ok)
	require.Nil(t, OpenAIStageTimingFrom(nil))
}

func TestOpenAIStageTimingSetKeepsFirstWriteOrder(t *testing.T) {
	timing := NewOpenAIStageTiming(nil)
	timing.Set(StageBodyRead, 10)
	timing.Set(StagePrepare, 20)
	timing.Set(StageBodyRead, 15)
	timing.Set(StageUserSlot, -1)

	fields := timing.Fields()
	require.Len(t, fields, 2)
	require.Equal(t, StageBodyRead, fields[0].Key)
	require.Equal(t, int64(15), fields[0].Integer)
	require.Equal(t, StagePrepare, fields[1].Key)
	_, ok := timing.Get(StageUserSlot)
	require.False(t, ok, "负值应被忽略")
}

func TestOpenAIStageTimingBeginForwardClearsAttemptStages(t *testing.T) {
	timing := NewOpenAIStageTiming(nil)
	timing.Set(StagePreForward, 100)
	timing.BeginForward(time.Now().Add(-50 * time.Millisecond))
	timing.UpstreamDispatched(time.Now())
	timing.Set(StageUpstreamHeaders, 30)
	timing.Set(StageFirstToken, 200)
	prep, ok := timing.Get(StageForwardPrep)
	require.True(t, ok)
	require.GreaterOrEqual(t, prep, int64(50))

	// 换号重试：转发阶段与网络时序清空，请求前阶段保留。
	timing.BeginForward(time.Now())
	for _, name := range attemptScopedStages {
		_, exists := timing.Get(name)
		require.False(t, exists, name)
	}
	pre, ok := timing.Get(StagePreForward)
	require.True(t, ok)
	require.Equal(t, int64(100), pre)
}

func TestOpenAIStageTimingUpstreamDispatchedRecordsFirstAttemptOnly(t *testing.T) {
	timing := NewOpenAIStageTiming(nil)
	start := time.Now().Add(-80 * time.Millisecond)
	timing.BeginForward(start)
	timing.UpstreamDispatched(start.Add(80 * time.Millisecond))
	timing.UpstreamDispatched(start.Add(900 * time.Millisecond))
	prep, ok := timing.Get(StageForwardPrep)
	require.True(t, ok)
	require.Equal(t, int64(80), prep, "重试不应覆盖首次尝试的前处理耗时")
}

func TestOpenAIStageTimingDerivedFields(t *testing.T) {
	timing := NewOpenAIStageTiming(nil)
	timing.Set(StagePreForward, 100)
	timing.Set(StageForwardEntry, 20)
	timing.Set(StageForwardPrep, 50)
	timing.Set(StageUpstreamHeaders, 300)
	timing.Set(StageForward, 1000)
	timing.Set(StageFirstToken, 500)

	got := stageFieldMap(timing.Fields())
	require.Equal(t, int64(650), got[stageAfterHeaders])
	require.Equal(t, int64(620), got[stageTTFTFromReq])

	// 多次上游尝试时无法把响应头之后的耗时归属到某次尝试，应省略。
	timing.AttachUpstreamTrace(context.Background())
	timing.AttachUpstreamTrace(context.Background())
	got = stageFieldMap(timing.Fields())
	require.NotContains(t, got, stageAfterHeaders)
	require.Equal(t, int64(2), got[stageUpstreamAttempts])
	require.Equal(t, int64(620), got[stageTTFTFromReq])
}

func TestOpenAIStageTimingUpstreamTraceOnRealConnection(t *testing.T) {
	const serverDelay = 120 * time.Millisecond
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(serverDelay)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	client := server.Client()
	defer client.CloseIdleConnections()
	timing := NewOpenAIStageTiming(nil)

	doStageTimingRequest(t, timing.AttachUpstreamTrace(context.Background()), client, server.URL)
	first := stageFieldMap(timing.Fields())
	require.Equal(t, int64(1), first[stageUpstreamAttempts])
	require.Equal(t, false, first["upstream_conn_reused"])
	for _, key := range []string{"upstream_conn_ms", "upstream_tcp_ms", "upstream_tls_ms", "upstream_send_ms", "upstream_wait_ms", "upstream_ttfb_ms"} {
		require.Contains(t, first, key)
	}
	require.GreaterOrEqual(t, first["upstream_wait_ms"].(int64), (serverDelay - 20*time.Millisecond).Milliseconds())
	require.GreaterOrEqual(t, first["upstream_ttfb_ms"].(int64), first["upstream_wait_ms"].(int64))

	// 第二次尝试复用同一连接：重置后不应再有建连与 TLS 字段。
	doStageTimingRequest(t, timing.AttachUpstreamTrace(context.Background()), client, server.URL)
	second := stageFieldMap(timing.Fields())
	require.Equal(t, int64(2), second[stageUpstreamAttempts])
	require.Equal(t, true, second["upstream_conn_reused"])
	require.NotContains(t, second, "upstream_tcp_ms")
	require.NotContains(t, second, "upstream_tls_ms")
}

func TestAttachOpenAIUpstreamTraceHonorsAccountSwitch(t *testing.T) {
	c := newStageTimingTestContext()
	NewOpenAIStageTiming(c)
	base := context.Background()

	off := &Account{ID: 1, Extra: map[string]any{}}
	require.Nil(t, httptrace.ContextClientTrace(attachOpenAIUpstreamTrace(base, c, off)))
	require.Nil(t, httptrace.ContextClientTrace(attachOpenAIUpstreamTrace(base, c, nil)))

	on := &Account{ID: 2, Extra: map[string]any{OpenAIStageTimingExtraKey: true}}
	require.NotNil(t, httptrace.ContextClientTrace(attachOpenAIUpstreamTrace(base, c, on)))

	// 开关打开但请求没有创建收集器（非 Responses 入口）时不挂 trace。
	require.Nil(t, httptrace.ContextClientTrace(attachOpenAIUpstreamTrace(base, newStageTimingTestContext(), on)))
}

func TestIsOpenAIStageTimingLogEnabled(t *testing.T) {
	var nilAccount *Account
	require.False(t, nilAccount.IsOpenAIStageTimingLogEnabled())
	require.False(t, (&Account{}).IsOpenAIStageTimingLogEnabled())
	require.False(t, (&Account{Extra: map[string]any{OpenAIStageTimingExtraKey: false}}).IsOpenAIStageTimingLogEnabled())
	require.False(t, (&Account{Extra: map[string]any{OpenAIStageTimingExtraKey: "true"}}).IsOpenAIStageTimingLogEnabled())
	require.True(t, (&Account{Extra: map[string]any{OpenAIStageTimingExtraKey: true}}).IsOpenAIStageTimingLogEnabled())
}

func TestDoOpenAIUpstreamTimedRecordsHeadersAndOpsLatency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(60 * time.Millisecond)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	svc := &OpenAIGatewayService{httpUpstream: &stageTimingUpstream{client: server.Client()}}
	account := &Account{ID: 7, Extra: map[string]any{OpenAIStageTimingExtraKey: true}}
	c := newStageTimingTestContext()
	timing := NewOpenAIStageTiming(c)
	timing.BeginForward(time.Now())

	ctx := attachOpenAIUpstreamTrace(context.Background(), c, account)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, nil)
	require.NoError(t, err)
	resp, err := svc.doOpenAIUpstreamTimed(c, req, "", account)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	headers, ok := timing.Get(StageUpstreamHeaders)
	require.True(t, ok)
	require.GreaterOrEqual(t, headers, int64(50))
	_, ok = timing.Get(StageForwardPrep)
	require.True(t, ok)
	opsLatency, exists := c.Get(OpsUpstreamLatencyMsKey)
	require.True(t, exists, "原有 Ops 上游耗时口径必须保留")
	require.GreaterOrEqual(t, opsLatency.(int64), int64(50))
	require.Contains(t, stageFieldMap(timing.Fields()), "upstream_wait_ms")
}
