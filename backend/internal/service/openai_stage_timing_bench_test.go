package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// BenchmarkOpenAIStageTimingRequestPathDisabled 模拟账号未开启开关时，一个请求在 handler 与
// service 里多做的全部计时动作（不含日志输出，因为开关关闭时直接返回）。
func BenchmarkOpenAIStageTimingRequestPathDisabled(b *testing.B) {
	account := &Account{ID: 1, Extra: map[string]any{}}
	c := newStageTimingTestContext()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		timing := NewOpenAIStageTiming(c)
		timing.RecordSince(StageBodyRead, start)
		timing.Set(StageBodyBytes, 52000)
		timing.RecordSince(StagePrepare, start)
		timing.RecordSince(StageUserSlot, start)
		timing.RecordSince(StageBilling, start)
		timing.RecordSince(StageSelectAccount, start)
		timing.RecordSince(StageAccountSlot, start)
		timing.RecordSince(StagePreForward, start)
		timing.BeginForward(start)
		timing.RecordSinceForward(StageForwardEntry, start)
		_ = attachOpenAIUpstreamTrace(ctx, c, account)
		timing.UpstreamDispatched(start)
		timing.UpstreamHeaders(start)
		timing.Set(StageForward, 1)
	}
}

func benchmarkLoopbackUpstreamRequest(b *testing.B, withTrace bool) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	client := server.Client()
	defer client.CloseIdleConnections()
	timing := NewOpenAIStageTiming(nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		if withTrace {
			ctx = timing.AttachUpstreamTrace(ctx)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, nil)
		if err != nil {
			b.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

// 回环 HTTP 请求本身约几十微秒，两者之差即 httptrace 回调的额外开销。
func BenchmarkLoopbackUpstreamRequestWithoutTrace(b *testing.B) {
	benchmarkLoopbackUpstreamRequest(b, false)
}
func BenchmarkLoopbackUpstreamRequestWithTrace(b *testing.B) {
	benchmarkLoopbackUpstreamRequest(b, true)
}
