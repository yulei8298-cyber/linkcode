package service

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// OpenAIStageTimingExtraKey 是 accounts.extra 中开启分阶段耗时日志的账号级开关（bool，缺省关闭）。
	OpenAIStageTimingExtraKey = "openai_stage_timing_log"

	openAIStageTimingContextKey = "openai_stage_timing"
)

// 分阶段耗时日志的字段名。除 body_bytes 与 upstream_attempts 外单位均为毫秒。
const (
	StageBodyBytes       = "body_bytes"
	StageBodyRead        = "body_read_ms"
	StagePrepare         = "prepare_ms"
	StageUserSlot        = "user_slot_ms"
	StageBilling         = "billing_ms"
	StageSelectAccount   = "select_account_ms"
	StageAccountSlot     = "account_slot_ms"
	StagePreForward      = "pre_forward_ms"
	StageForwardEntry    = "forward_entry_ms"
	StageForwardPrep     = "forward_prep_ms"
	StageUpstreamHeaders = "upstream_headers_ms"
	StageForward         = "forward_ms"
	StageFirstToken      = "first_token_ms"

	stageAfterHeaders     = "after_headers_ms"
	stageTTFTFromReq      = "ttft_from_request_ms"
	stageUpstreamAttempts = "upstream_attempts"
)

// attemptScopedStages 是随「一次 Forward 尝试」变化的阶段，每次 BeginForward 都要清空，
// 避免 failover 换号后沿用上一个账号的数值。
var attemptScopedStages = []string{
	StageForwardEntry, StageForwardPrep, StageUpstreamHeaders, StageForward, StageFirstToken,
}

// OpenAIStageTiming 记录一次 /v1/responses 请求在网关内各阶段的耗时，用于排查延迟。
//
// 并发模型：handler 主流程串行写入，httptrace 回调在传输层 goroutine 触发，因此内部加锁。
// 所有方法对 nil 接收者安全，调用方无需判空；未开启开关的请求只承担几次 time.Now() 的开销。
type OpenAIStageTiming struct {
	mu           sync.Mutex
	forwardStart time.Time
	names        []string
	values       map[string]int64
	up           upstreamTrace
}

// upstreamTrace 保存最近一次上游尝试的 httptrace 时间点。
type upstreamTrace struct {
	attempts     int
	getConn      time.Time
	gotConn      time.Time
	dnsStart     time.Time
	dnsDone      time.Time
	connStart    time.Time
	connDone     time.Time
	tlsStart     time.Time
	tlsDone      time.Time
	wroteRequest time.Time
	firstByte    time.Time
	reused       bool
	idle         time.Duration
}

// NewOpenAIStageTiming 创建收集器并挂到 gin Context，供 service 层按需取用。
func NewOpenAIStageTiming(c *gin.Context) *OpenAIStageTiming {
	t := &OpenAIStageTiming{values: make(map[string]int64, 24)}
	if c != nil {
		c.Set(openAIStageTimingContextKey, t)
	}
	return t
}

// OpenAIStageTimingFrom 取出请求级收集器；未创建时返回 nil（nil 安全，可直接调用方法）。
func OpenAIStageTimingFrom(c *gin.Context) *OpenAIStageTiming {
	if c == nil {
		return nil
	}
	value, _ := c.Get(openAIStageTimingContextKey)
	t, _ := value.(*OpenAIStageTiming)
	return t
}

// IsOpenAIStageTimingLogEnabled 返回账号是否开启分阶段耗时日志。
// 字段：accounts.extra.openai_stage_timing_log。
func (a *Account) IsOpenAIStageTimingLogEnabled() bool {
	if a == nil || a.Extra == nil {
		return false
	}
	enabled, ok := a.Extra[OpenAIStageTimingExtraKey].(bool)
	return ok && enabled
}

// Set 写入一个阶段值；同名覆盖并保持首次写入的顺序，负值忽略。
func (t *OpenAIStageTiming) Set(name string, value int64) {
	if t == nil || value < 0 {
		return
	}
	t.mu.Lock()
	t.setLocked(name, value)
	t.mu.Unlock()
}

func (t *OpenAIStageTiming) setLocked(name string, value int64) {
	if _, exists := t.values[name]; !exists {
		t.names = append(t.names, name)
	}
	t.values[name] = value
}

// RecordSince 记录从 from 到现在经过的毫秒数。
func (t *OpenAIStageTiming) RecordSince(name string, from time.Time) {
	t.Set(name, time.Since(from).Milliseconds())
}

// Get 读取已记录的阶段值。
func (t *OpenAIStageTiming) Get(name string) (int64, bool) {
	if t == nil {
		return 0, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	value, ok := t.values[name]
	return value, ok
}

// BeginForward 标记 handler 调用 Forward 的起点，后续 forward_entry/forward_prep 都以它为基准，
// 并清空上一次尝试遗留的转发阶段与上游网络时序。
func (t *OpenAIStageTiming) BeginForward(at time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.forwardStart = at
	t.up = upstreamTrace{}
	for _, name := range attemptScopedStages {
		t.deleteLocked(name)
	}
	t.mu.Unlock()
}

func (t *OpenAIStageTiming) deleteLocked(name string) {
	if _, exists := t.values[name]; !exists {
		return
	}
	delete(t.values, name)
	for i, existing := range t.names {
		if existing == name {
			t.names = append(t.names[:i], t.names[i+1:]...)
			break
		}
	}
}

// RecordSinceForward 记录从 BeginForward 到 at 的毫秒数；未标记起点时忽略。
func (t *OpenAIStageTiming) RecordSinceForward(name string, at time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	if !t.forwardStart.IsZero() {
		t.setLocked(name, nonNegativeMs(at.Sub(t.forwardStart).Milliseconds()))
	}
	t.mu.Unlock()
}

// nonNegativeMs 把毫秒值下限钳到 0（时钟回拨或乱序时避免出现负数）。
func nonNegativeMs(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

// UpstreamDispatched 在上游请求发出前调用，仅首次尝试记录 forward_prep_ms，
// 即 Forward 入口到首次发出上游请求之间的处理耗时（含请求构造）。
func (t *OpenAIStageTiming) UpstreamDispatched(at time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	if _, exists := t.values[StageForwardPrep]; !exists && !t.forwardStart.IsZero() {
		t.setLocked(StageForwardPrep, nonNegativeMs(at.Sub(t.forwardStart).Milliseconds()))
	}
	t.mu.Unlock()
}

// UpstreamHeaders 记录上游响应头到达耗时（与 Ops 的上游耗时同口径）。
func (t *OpenAIStageTiming) UpstreamHeaders(startedAt time.Time) {
	t.RecordSince(StageUpstreamHeaders, startedAt)
}

// AttachUpstreamTrace 在一次上游尝试开始前调用：清空上一次尝试的网络时序，
// 并返回携带 httptrace 的 ctx，用于拆分 DNS、建连、TLS、发送与服务端等待。
func (t *OpenAIStageTiming) AttachUpstreamTrace(ctx context.Context) context.Context {
	if t == nil {
		return ctx
	}
	t.mu.Lock()
	t.up = upstreamTrace{attempts: t.up.attempts + 1}
	t.mu.Unlock()
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GetConn:  func(string) { t.traceAt(func(u *upstreamTrace, now time.Time) { u.getConn = now }) },
		GotConn:  t.onGotConn,
		DNSStart: func(httptrace.DNSStartInfo) { t.traceAt(func(u *upstreamTrace, now time.Time) { u.dnsStart = now }) },
		DNSDone:  func(httptrace.DNSDoneInfo) { t.traceAt(func(u *upstreamTrace, now time.Time) { u.dnsDone = now }) },
		ConnectStart: func(string, string) {
			// 双栈并发拨号会多次触发，取最早的起点。
			t.traceAt(func(u *upstreamTrace, now time.Time) {
				if u.connStart.IsZero() {
					u.connStart = now
				}
			})
		},
		ConnectDone:       func(string, string, error) { t.traceAt(func(u *upstreamTrace, now time.Time) { u.connDone = now }) },
		TLSHandshakeStart: func() { t.traceAt(func(u *upstreamTrace, now time.Time) { u.tlsStart = now }) },
		TLSHandshakeDone:  func(tls.ConnectionState, error) { t.traceAt(func(u *upstreamTrace, now time.Time) { u.tlsDone = now }) },
		WroteRequest: func(httptrace.WroteRequestInfo) {
			t.traceAt(func(u *upstreamTrace, now time.Time) { u.wroteRequest = now })
		},
		GotFirstResponseByte: func() { t.traceAt(func(u *upstreamTrace, now time.Time) { u.firstByte = now }) },
	})
}

func (t *OpenAIStageTiming) onGotConn(info httptrace.GotConnInfo) {
	t.traceAt(func(u *upstreamTrace, now time.Time) {
		u.gotConn = now
		u.reused = info.Reused
		if info.WasIdle {
			u.idle = info.IdleTime
		}
	})
}

func (t *OpenAIStageTiming) traceAt(apply func(u *upstreamTrace, now time.Time)) {
	now := time.Now()
	t.mu.Lock()
	apply(&t.up, now)
	t.mu.Unlock()
}

// Fields 按写入顺序输出阶段值，并追加上游网络拆分与派生指标。
func (t *OpenAIStageTiming) Fields() []zap.Field {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	fields := make([]zap.Field, 0, len(t.names)+12)
	for _, name := range t.names {
		fields = append(fields, zap.Int64(name, t.values[name]))
	}
	fields = append(fields, t.up.fields()...)
	return append(fields, t.derivedFieldsLocked()...)
}

// derivedFieldsLocked 计算由多个阶段推导出的指标。
//   - after_headers_ms：响应头到达后到 Forward 返回（流式读取、回写客户端与收尾）。
//     仅单次上游尝试时才成立，多次尝试时无法归属，留空。
//   - ttft_from_request_ms：从 handler 入口到首个语义输出的网关侧首字延迟，
//     = pre_forward + forward_entry + first_token（first_token 自 Forward 内 startTime 起算）。
func (t *OpenAIStageTiming) derivedFieldsLocked() []zap.Field {
	var fields []zap.Field
	forward, hasForward := t.values[StageForward]
	prep, hasPrep := t.values[StageForwardPrep]
	headers, hasHeaders := t.values[StageUpstreamHeaders]
	if hasForward && hasPrep && hasHeaders && t.up.attempts <= 1 {
		fields = append(fields, zap.Int64(stageAfterHeaders, nonNegativeMs(forward-prep-headers)))
	}
	pre, hasPre := t.values[StagePreForward]
	entry, hasEntry := t.values[StageForwardEntry]
	firstToken, hasFirst := t.values[StageFirstToken]
	if hasPre && hasEntry && hasFirst {
		fields = append(fields, zap.Int64(stageTTFTFromReq, pre+entry+firstToken))
	}
	return fields
}

// fields 输出上游网络拆分。start 时间点缺失（例如复用连接没有 DNS/TLS）时对应字段不输出。
//   - upstream_conn_ms：获取连接耗时（连接池等待或新建，含 DNS/建连/TLS）。
//   - upstream_send_ms：连接就绪到请求（含请求体）写完。
//   - upstream_wait_ms：请求写完到收到首个响应字节，即上游处理等待。
func (u *upstreamTrace) fields() []zap.Field {
	if u.attempts == 0 {
		return nil
	}
	fields := []zap.Field{zap.Int(stageUpstreamAttempts, u.attempts)}
	if !u.gotConn.IsZero() {
		fields = append(fields, zap.Bool("upstream_conn_reused", u.reused))
		if u.idle > 0 {
			fields = append(fields, zap.Int64("upstream_conn_idle_ms", u.idle.Milliseconds()))
		}
	}
	fields = appendSpan(fields, "upstream_conn_ms", u.getConn, u.gotConn)
	fields = appendSpan(fields, "upstream_dns_ms", u.dnsStart, u.dnsDone)
	fields = appendSpan(fields, "upstream_tcp_ms", u.connStart, u.connDone)
	fields = appendSpan(fields, "upstream_tls_ms", u.tlsStart, u.tlsDone)
	fields = appendSpan(fields, "upstream_send_ms", u.gotConn, u.wroteRequest)
	fields = appendSpan(fields, "upstream_wait_ms", u.wroteRequest, u.firstByte)
	return appendSpan(fields, "upstream_ttfb_ms", u.getConn, u.firstByte)
}

func appendSpan(fields []zap.Field, name string, from, to time.Time) []zap.Field {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return fields
	}
	return append(fields, zap.Int64(name, to.Sub(from).Milliseconds()))
}

// attachOpenAIUpstreamTrace 仅在账号开启开关时挂 httptrace，其余账号零额外开销。
func attachOpenAIUpstreamTrace(ctx context.Context, c *gin.Context, account *Account) context.Context {
	if !account.IsOpenAIStageTimingLogEnabled() {
		return ctx
	}
	return OpenAIStageTimingFrom(c).AttachUpstreamTrace(ctx)
}

// doOpenAIUpstreamTimed 包装 doOpenAIUpstream：保持原有 Ops 上游耗时口径（响应头到达），
// 同时把上游调用的起止点记入分阶段计时。
func (s *OpenAIGatewayService) doOpenAIUpstreamTimed(c *gin.Context, request *http.Request, proxyURL string, account *Account) (*http.Response, error) {
	timing := OpenAIStageTimingFrom(c)
	startedAt := time.Now()
	timing.UpstreamDispatched(startedAt)
	response, err := s.doOpenAIUpstream(request, proxyURL, account)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(startedAt).Milliseconds())
	timing.UpstreamHeaders(startedAt)
	return response, err
}
