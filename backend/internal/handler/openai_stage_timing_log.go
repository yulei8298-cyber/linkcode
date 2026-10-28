package handler

import (
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

const (
	stageTimingLogMessage = "openai.stage_timing"

	stageOutcomeOK       = "ok"
	stageOutcomeFailover = "failover"
	stageOutcomeError    = "error"
)

// logOpenAIStageTiming 在一次 Forward 尝试结束后输出一条聚合的分阶段耗时日志。
//
// 只对 accounts.extra.openai_stage_timing_log=true 的账号输出，其余账号直接返回。
// 日志只含阶段耗时、账号标识、结果分类与上游状态码，不含请求体、密钥或上游响应内容。
// 调用时机必须在 Forward 返回之后、失败处理之前，这样 failover 的每次尝试都有记录。
func logOpenAIStageTiming(
	reqLog *zap.Logger,
	timing *service.OpenAIStageTiming,
	account *service.Account,
	accountSwitches int,
	result *service.OpenAIForwardResult,
	forwardErr error,
) {
	if reqLog == nil || timing == nil || !account.IsOpenAIStageTimingLogEnabled() {
		return
	}
	if result != nil && result.FirstTokenMs != nil {
		timing.Set(service.StageFirstToken, int64(*result.FirstTokenMs))
	}
	fields := []zap.Field{
		zap.Int64("account_id", account.ID),
		zap.String("account_name", account.Name),
		zap.Int("account_switches", accountSwitches),
	}
	fields = append(fields, classifyStageTimingOutcome(forwardErr)...)
	reqLog.Info(stageTimingLogMessage, append(fields, timing.Fields()...)...)
}

// classifyStageTimingOutcome 把 Forward 的返回归类为 ok / failover / error。
// failover 额外带上游状态码，便于区分限流、5xx 与传输层失败。
func classifyStageTimingOutcome(forwardErr error) []zap.Field {
	if forwardErr == nil {
		return []zap.Field{zap.String("outcome", stageOutcomeOK)}
	}
	var failoverErr *service.UpstreamFailoverError
	if errors.As(forwardErr, &failoverErr) {
		return []zap.Field{
			zap.String("outcome", stageOutcomeFailover),
			zap.Int("upstream_status", failoverErr.StatusCode),
		}
	}
	return []zap.Field{zap.String("outcome", stageOutcomeError)}
}
