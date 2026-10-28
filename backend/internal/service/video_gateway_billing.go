package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
)

var (
	// ErrVideoModelNotPriced 表示模型没有任何可用的视频价格，网关拒绝创建以免出现未计价生成。
	ErrVideoModelNotPriced = errors.New("video model has no configured price")
	// ErrVideoInsufficientBalance 表示余额不足以覆盖本次与未结算任务的预估费用。
	ErrVideoInsufficientBalance = errors.New("insufficient balance for video generation")
)

// estimateVideoTokens 按像素数 × 24fps / 1024 估算方舟视频 token，仅用于预估。
func estimateVideoTokens(resolution, aspect string, seconds int) int64 {
	w, h, _ := video.ParseSize(video.SizeFor(resolution, aspect))
	return int64(w) * int64(h) * 24 * int64(seconds) / 1024
}

// videoBillingModel 选择计费模型名：公开模型名有价格时用公开名，否则回退上游模型名
// （管理员常把别名映射到带版本号的上游模型，价格只配在其中一个上）。
func (s *VideoGatewayService) videoBillingModel(ctx context.Context, apiKey *APIKey, requested, upstream string) string {
	if s.videoModelPriced(ctx, apiKey, requested) || strings.TrimSpace(upstream) == "" {
		return requested
	}
	if s.videoModelPriced(ctx, apiKey, upstream) {
		return upstream
	}
	return requested
}

// videoModelPriced 报告模型是否有渠道价、分组视频价或视频计费规则。
func (s *VideoGatewayService) videoModelPriced(ctx context.Context, apiKey *APIKey, model string) bool {
	if strings.TrimSpace(model) == "" {
		return false
	}
	if _, ok := s.gateway.videoPricingSpec(model); ok {
		return true
	}
	if resolved := s.gateway.resolveOpenAIChannelPricing(ctx, model, apiKey); resolved != nil {
		return true
	}
	for _, res := range []string{VideoBillingResolution480P, VideoBillingResolution720P, VideoBillingResolution1080P} {
		if apiKeyHasConfiguredVideoPrice(apiKey, model, res) {
			return true
		}
	}
	return false
}

// videoRateMultiplier 返回视频计费倍率（用户专属 / 分组倍率，含视频独立倍率）。
func (s *VideoGatewayService) videoRateMultiplier(ctx context.Context, apiKey *APIKey) float64 {
	multiplier := 1.0
	if s.cfg != nil {
		multiplier = s.cfg.Default.RateMultiplier
	}
	if apiKey != nil && apiKey.GroupID != nil && apiKey.Group != nil {
		multiplier = s.gateway.ResolveUserGroupRateMultiplier(ctx, apiKey.UserID, *apiKey.GroupID, apiKey.Group.RateMultiplier)
	}
	return resolveVideoRateMultiplier(apiKey, multiplier)
}

// Estimate 预估一次生成的实付费用；priced 为 false 表示模型未配置价格。
func (s *VideoGatewayService) Estimate(ctx context.Context, apiKey *APIKey, req video.Request, upstreamModel string) (cost float64, priced bool) {
	billingModel := s.videoBillingModel(ctx, apiKey, req.Model, upstreamModel)
	if !s.videoModelPriced(ctx, apiKey, billingModel) {
		return 0, false
	}
	result := &OpenAIForwardResult{
		Model:                billingModel,
		BillingModel:         billingModel,
		VideoCount:           1,
		VideoResolution:      req.Resolution,
		VideoDurationSeconds: req.DurationSeconds,
		VideoWithAudio:       req.GenerateAudio != nil && *req.GenerateAudio,
	}
	if spec, ok := s.gateway.videoPricingSpec(billingModel); ok && spec.Unit == VideoBillingUnitToken {
		usage := spec.NormalizeUsage(VideoUsage{Resolution: req.Resolution, DurationSeconds: req.DurationSeconds})
		result.Usage.OutputTokens = int(estimateVideoTokens(usage.Resolution, req.AspectRatio, usage.DurationSeconds))
	}
	breakdown := s.gateway.calculateOpenAIVideoCost(ctx, billingModel, apiKey, result, s.videoRateMultiplier(ctx, apiKey))
	if breakdown == nil {
		return 0, true
	}
	return breakdown.ActualCost, true
}

// CheckBalance 余额计费模式下要求余额覆盖本次预估与该用户全部未结算任务的预估之和；
// 订阅、套餐、免费分组不以余额为前置条件。
func (s *VideoGatewayService) CheckBalance(ctx context.Context, apiKey *APIKey, subscription *UserSubscription, estimate float64) error {
	if apiKey == nil || apiKey.User == nil || estimate <= 0 {
		return nil
	}
	if apiKey.Group != nil && (apiKey.Group.IsFree || (apiKey.Group.IsSubscriptionType() && subscription != nil)) {
		return nil
	}
	if PackageBillingFromContext(ctx) != nil {
		return nil
	}
	pending, err := s.repo.SumPendingEstimatedCost(ctx, apiKey.UserID)
	if err != nil {
		return err
	}
	if apiKey.User.Balance < pending+estimate {
		return ErrVideoInsufficientBalance
	}
	return nil
}

// settlementContext 重建与鉴权中间件一致的计费上下文（订阅 / 套餐 / 企业倍率），
// 让后台结算与同步请求按同一规则扣费。
func (s *VideoGatewayService) settlementContext(ctx context.Context, apiKey *APIKey) (context.Context, *UserSubscription) {
	ctx = WithResolvedTargetPlatform(ctx, PlatformVideo)
	if apiKey.Group != nil && apiKey.Group.IsSubscriptionType() && s.subscriptions != nil && apiKey.GroupID != nil {
		if sub, err := s.subscriptions.GetActiveSubscription(ctx, apiKey.UserID, *apiKey.GroupID); err == nil && sub != nil {
			return ctx, sub
		}
	}
	if s.apiKeyService == nil {
		return ctx, nil
	}
	lowBalance := (apiKey.Group == nil || !apiKey.Group.IsFree) && apiKey.User != nil && apiKey.User.Balance <= 0
	if pb, err := s.apiKeyService.ResolvePackageBilling(ctx, apiKey, lowBalance); err == nil && pb != nil {
		return WithPackageBilling(ctx, pb), nil
	}
	if rate := s.apiKeyService.ResolveEnterpriseRate(ctx, apiKey); rate != nil {
		ctx = WithEnterpriseRate(ctx, rate)
	}
	return ctx, nil
}

// Settle 按任务实际用量写 usage_logs 并扣费；request_id 固定为任务 ID，重复结算由用量去重保护。
func (s *VideoGatewayService) Settle(ctx context.Context, task *VideoTask) error {
	if task.Status != video.StatusSucceeded {
		return fmt.Errorf("video task %s is not completed", task.TaskID)
	}
	if s.apiKeyService == nil || s.accountRepo == nil {
		err := fmt.Errorf("video settlement dependencies are not configured")
		_ = s.repo.MarkBilling(ctx, task.ID, VideoBillingPending, nil, err.Error())
		return err
	}
	apiKey, err := s.apiKeyService.GetByID(ctx, task.APIKeyID)
	if err != nil {
		_ = s.repo.MarkBilling(ctx, task.ID, VideoBillingPending, nil, "load api key: "+err.Error())
		return err
	}
	account, err := s.accountRepo.GetByID(ctx, task.AccountID)
	if err != nil {
		_ = s.repo.MarkBilling(ctx, task.ID, VideoBillingPending, nil, "load account: "+err.Error())
		return err
	}
	billCtx, subscription := s.settlementContext(ctx, apiKey)
	billingModel := s.videoBillingModel(billCtx, apiKey, task.Model, task.UpstreamModel)
	finished := time.Now()
	if task.FinishedAt != nil {
		finished = *task.FinishedAt
	}
	result := &OpenAIForwardResult{
		RequestID:            videoTaskBillingRequestID(task),
		ResponseID:           task.TaskID,
		Model:                task.Model,
		BillingModel:         billingModel,
		UpstreamModel:        task.UpstreamModel,
		UpstreamEndpoint:     "video:" + task.Protocol,
		VideoCount:           max(task.VideoCount, 1),
		VideoResolution:      task.Resolution,
		VideoDurationSeconds: task.BillingDurationSeconds(),
		VideoWithAudio:       task.GenerateAudio,
		Duration:             finished.Sub(task.CreatedAt),
	}
	result.Usage.OutputTokens = int(task.CompletionTokens)
	err = s.gateway.RecordUsage(billCtx, &OpenAIRecordUsageInput{
		Result:             result,
		APIKey:             apiKey,
		User:               apiKey.User,
		Account:            account,
		Subscription:       subscription,
		InboundEndpoint:    task.InboundEndpoint,
		UpstreamEndpoint:   result.UpstreamEndpoint,
		UserAgent:          task.UserAgent,
		IPAddress:          task.ClientIP,
		RequestPayloadHash: HashUsageRequestPayload([]byte(task.TaskID)),
		APIKeyService:      s.apiKeyService,
		QuotaPlatform:      PlatformVideo,
		PricingAt:          task.CreatedAt,
		ChannelUsageFields: s.videoChannelUsageFields(billCtx, apiKey, task.Model),
	})
	if err != nil {
		_ = s.repo.MarkBilling(ctx, task.ID, VideoBillingPending, nil, err.Error())
		return err
	}
	actual := s.settledCost(billCtx, apiKey, billingModel, result)
	return s.repo.MarkBilling(ctx, task.ID, VideoBillingBilled, &actual, "")
}

// videoChannelUsageFields 视频不做渠道模型映射，只记录分组所属渠道，供用量按渠道统计。
func (s *VideoGatewayService) videoChannelUsageFields(ctx context.Context, apiKey *APIKey, model string) ChannelUsageFields {
	fields := ChannelUsageFields{OriginalModel: model, ChannelMappedModel: model}
	if apiKey != nil && apiKey.GroupID != nil {
		fields.ChannelID = s.gateway.ResolveChannelMapping(ctx, *apiKey.GroupID, model).ChannelID
	}
	return fields
}

// settledCost 重算本次实付费用，仅用于在任务上展示（扣费以 usage_logs 为准）。
func (s *VideoGatewayService) settledCost(ctx context.Context, apiKey *APIKey, billingModel string, result *OpenAIForwardResult) float64 {
	breakdown := s.gateway.calculateOpenAIVideoCost(ctx, billingModel, apiKey, result, s.videoRateMultiplier(ctx, apiKey))
	if breakdown == nil {
		return 0
	}
	return breakdown.ActualCost
}
