package service

import (
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	// proxyAuthRetryAttempts 是代理偶发 407 时在同一账号上的额外重试次数。
	// 407 发生在 CONNECT 隧道建立阶段，请求体尚未发给上游，重试不会造成重复执行或重复计费。
	proxyAuthRetryAttempts = 2

	// proxyAuthTempUnschedDuration 是 407 触发的临时不可调度时长。
	// 代理供应商侧的偶发 407 通常很快恢复，不应沿用 10 分钟的持久性故障冷却；
	// 凭据真的过期时账号仍会被反复下线，由告警与运维处理。
	proxyAuthTempUnschedDuration = time.Minute
)

// proxyAuthRetryBackoffs 与 proxyAuthRetryAttempts 一一对应。
var proxyAuthRetryBackoffs = []time.Duration{200 * time.Millisecond, 600 * time.Millisecond}

const proxyAuthRequiredMarker = "proxy authentication required"

// isProxyAuthRequiredMessage 判断传输错误文本是否为 HTTP 代理 407。
func isProxyAuthRequiredMessage(msg string) bool {
	return strings.Contains(strings.ToLower(msg), proxyAuthRequiredMarker)
}

// transportErrorTempUnschedDuration 按故障信息选择临时不可调度时长：
// 代理 407 使用短冷却，其余持久性传输故障沿用调用方给出的默认时长。
func transportErrorTempUnschedDuration(safeErr string, defaultDuration time.Duration) time.Duration {
	if isProxyAuthRequiredMessage(safeErr) {
		return proxyAuthTempUnschedDuration
	}
	return defaultDuration
}

// doWithProxyAuthRetry 执行一次上游请求，遇到代理 407 时在同一账号上短暂退避重试。
// 仅当请求体可重放（GetBody 可用或无请求体）时才重试，否则原样返回首次错误。
func doWithProxyAuthRetry(
	req *http.Request,
	accountID int64,
	do func(*http.Request) (*http.Response, error),
) (*http.Response, error) {
	resp, err := do(req)
	for attempt := 0; err != nil && attempt < proxyAuthRetryAttempts && isProxyAuthRequiredMessage(err.Error()); attempt++ {
		retryReq, ok := cloneRequestForRetry(req)
		if !ok {
			break
		}
		if sleepWithContext(req.Context(), proxyAuthRetryBackoffs[attempt]) != nil {
			break
		}
		logger.L().With(zap.String("component", "service.openai_gateway")).Warn(
			"upstream.proxy_auth_required_retry",
			zap.Int64("account_id", accountID),
			zap.Int("attempt", attempt+1),
		)
		resp, err = do(retryReq)
	}
	return resp, err
}

// cloneRequestForRetry 复制请求并重新取得请求体；请求体不可重放时返回 false。
func cloneRequestForRetry(req *http.Request) (*http.Request, bool) {
	clone := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return clone, true
	}
	if req.GetBody == nil {
		return nil, false
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, false
	}
	clone.Body = body
	return clone, true
}
