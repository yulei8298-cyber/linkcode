package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// videoMultipartMaxBytes 是统一入口 multipart 上传参考图的大小上限。
const videoMultipartMaxBytes = 20 << 20

// VideoHandler 处理视频平台的统一任务接口与厂商原生协议透传。
// 并发槽、内容审核、计费准入复用 OpenAIGatewayHandler 的实现。
type VideoHandler struct {
	openai *OpenAIGatewayHandler
	videos *service.VideoGatewayService
}

// NewVideoHandler 创建视频处理器。
func NewVideoHandler(openai *OpenAIGatewayHandler, videos *service.VideoGatewayService) *VideoHandler {
	return &VideoHandler{openai: openai, videos: videos}
}

// videoCreateInput 是一次创建请求；Protocol 非空表示原生透传。
type videoCreateInput struct {
	Request    video.Request
	Protocol   string
	NativePath string
	NativeBody []byte
	Native     video.Provider
}

// Create 处理统一入口创建（POST /v1/videos、/v1/video/generations）。
func (h *VideoHandler) Create(c *gin.Context) {
	var req video.Request
	var err error
	if strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data") {
		req, err = video.ParseUnifiedMultipart(c.Request, videoMultipartMaxBytes)
	} else {
		var body []byte
		body, err = pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err == nil {
			req, err = video.ParseUnifiedJSON(body)
		}
	}
	if err != nil {
		h.openai.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	h.create(c, videoCreateInput{Request: req})
}

func (h *VideoHandler) create(c *gin.Context, in videoCreateInput) {
	streamStarted := false
	defer h.openai.recoverResponsesPanic(c, &streamStarted)
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	subject, ok2 := middleware2.GetAuthSubjectFromContext(c)
	if !ok || !ok2 {
		h.openai.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	model := strings.TrimSpace(in.Request.Model)
	if model == "" {
		h.openai.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	// 原生协议的模型字段名各异（model_name、路径中的模型），分组白名单在此按统一模型名复核。
	if apiKey.Group != nil && apiKey.Group.ModelAllowlistEnabled() && !apiKey.Group.ModelAllowlist.Allows(model) {
		h.openai.errorResponse(c, http.StatusNotFound, "not_found_error", "Model \""+model+"\" is not available for this group")
		return
	}
	reqLog := requestLogger(c, "handler.video.create",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID),
		zap.String("model", model), zap.String("protocol", in.Protocol))
	setOpsRequestContext(c, model, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))

	if prompt := strings.TrimSpace(in.Request.Prompt); prompt != "" {
		moderation, _ := json.Marshal(map[string]string{"prompt": prompt})
		if decision := h.openai.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, model, moderation); decision != nil && !decision.AllowNextStage {
			h.openai.openAISecurityAuditError(c, decision)
			return
		}
	}
	userRelease, acquired := h.openai.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userRelease != nil {
		defer userRelease()
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.openai.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.PlatformVideo); err != nil {
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.openai.errorResponse(c, status, code, message)
		return
	}
	h.submitWithFailover(c, in, apiKey, subscription, reqLog, &streamStarted)
}

// submitWithFailover 选择账号并提交。只在上游明确拒绝（限流、鉴权、5xx）时换号重试；
// 网络错误可能已被上游受理，不重试以免重复生成计费任务。
func (h *VideoHandler) submitWithFailover(c *gin.Context, in videoCreateInput, apiKey *service.APIKey, subscription *service.UserSubscription, reqLog *zap.Logger, streamStarted *bool) {
	ctx := service.WithOpenAIProfitControlSuppressed(c.Request.Context())
	capability := service.OpenAIEndpointCapability("")
	if in.Protocol != "" {
		capability = service.VideoProtocolCapability(in.Protocol)
	}
	failed := map[int64]struct{}{}
	var lastUpstream *service.VideoUpstreamError
	for attempt := 0; attempt <= h.maxSwitches(); attempt++ {
		selection, _, err := h.openai.gatewayService.SelectAccountWithSchedulerForCapability(
			ctx, apiKey.GroupID, "", "", in.Request.Model, failed,
			service.OpenAIUpstreamTransportAny, capability, false, false, false, service.PlatformVideo)
		if err != nil || selection == nil || selection.Account == nil {
			if lastUpstream != nil {
				service.WriteVideoUpstreamError(c, lastUpstream)
				return
			}
			markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
			h.openai.errorResponse(c, http.StatusServiceUnavailable, "video_no_available_account", "No available video account for this model in the current group")
			return
		}
		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)
		upstreamModel := account.GetMappedModel(in.Request.Model)
		estimate, priced := h.videos.Estimate(ctx, apiKey, in.Request, upstreamModel)
		if !priced {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			h.openai.errorResponse(c, http.StatusServiceUnavailable, "video_model_not_priced", "This video model has no configured price; please contact the administrator")
			return
		}
		if err := h.videos.CheckBalance(ctx, apiKey, subscription, estimate); err != nil {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			h.openai.errorResponse(c, http.StatusPaymentRequired, "insufficient_balance", "Insufficient balance for this video generation and your pending video tasks")
			return
		}
		release, slot := h.openai.acquireResponsesAccountSlot(c, apiKey.GroupID, "", selection, false, streamStarted, reqLog)
		if slot != openAISlotAcquireOK {
			return
		}
		result, err := h.videos.Submit(ctx, h.submitInput(c, in, apiKey, account, upstreamModel, estimate))
		if release != nil {
			release()
		}
		var upstreamErr *service.VideoUpstreamError
		switch {
		case err == nil:
			h.openai.gatewayService.ReportOpenAIAccountScheduleResult(account, upstreamModel, true, nil)
			h.writeCreated(c, in, result)
			return
		case errors.As(err, &upstreamErr) && videoRetryableStatus(upstreamErr.StatusCode):
			h.openai.gatewayService.ReportOpenAIAccountScheduleResult(account, upstreamModel, false, nil)
			reqLog.Warn("video.create_upstream_rejected_switching", zap.Int64("account_id", account.ID), zap.Int("status", upstreamErr.StatusCode))
			failed[account.ID] = struct{}{}
			lastUpstream = upstreamErr
			continue
		case errors.As(err, &upstreamErr):
			service.WriteVideoUpstreamError(c, upstreamErr)
			return
		case errors.Is(err, video.ErrInvalidRequest):
			h.openai.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		default:
			reqLog.Warn("video.create_failed", zap.Int64("account_id", account.ID), zap.Error(err))
			h.openai.errorResponse(c, http.StatusBadGateway, "upstream_error", "Video upstream request failed")
			return
		}
	}
	if lastUpstream != nil {
		service.WriteVideoUpstreamError(c, lastUpstream)
	}
}

func (h *VideoHandler) maxSwitches() int {
	if h.openai.maxAccountSwitches > 0 {
		return h.openai.maxAccountSwitches
	}
	return 3
}

// videoRetryableStatus：限流、鉴权失败、上游故障说明任务未被受理，可以换号。
func videoRetryableStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusTooManyRequests || status >= 500
}

func (h *VideoHandler) submitInput(c *gin.Context, in videoCreateInput, apiKey *service.APIKey, account *service.Account, upstreamModel string, estimate float64) service.VideoSubmitInput {
	input := service.VideoSubmitInput{
		APIKey:          apiKey,
		Account:         account,
		Request:         in.Request,
		RequestedModel:  in.Request.Model,
		UpstreamModel:   upstreamModel,
		EstimatedCost:   estimate,
		InboundEndpoint: GetInboundEndpoint(c),
		ClientIP:        ip.GetClientIP(c),
		UserAgent:       c.GetHeader("User-Agent"),
	}
	if in.Native != nil {
		body, path, err := in.Native.RewriteNativeModel(in.NativePath, in.NativeBody, upstreamModel)
		if err != nil {
			body, path = in.NativeBody, in.NativePath
		}
		input.NativeCall = &video.Call{Method: http.MethodPost, Path: path, Body: body, ContentType: c.GetHeader("Content-Type"), Headers: forwardedNativeHeaders(c, in.Native)}
	}
	return input
}

func (h *VideoHandler) writeCreated(c *gin.Context, in videoCreateInput, result *service.VideoSubmitResult) {
	c.Header("Cache-Control", "no-store")
	if in.Native != nil {
		service.WriteVideoUpstreamResponse(c, result)
		return
	}
	c.JSON(http.StatusOK, h.videos.VideoTaskView(result.Task, requestPublicBase(c)))
}

// Get 查询统一任务（GET /v1/videos/:id、/v1/video/generations/:id）。非终态时向上游刷新一次。
func (h *VideoHandler) Get(c *gin.Context) {
	task, ok := h.ownedTask(c)
	if !ok {
		return
	}
	if !task.Status.IsTerminal() {
		if fresh, _, err := h.videos.Refresh(c.Request.Context(), task); err == nil {
			task = fresh
		}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, h.videos.VideoTaskView(task, requestPublicBase(c)))
}

// Content 下载成品（GET /v1/videos/:id/content，需 API Key）。
func (h *VideoHandler) Content(c *gin.Context) {
	task, ok := h.ownedTask(c)
	if !ok {
		return
	}
	h.stream(c, task)
}

// PublicContent 通过签名链接下载成品（GET /video/files/:task_id，免鉴权）。
func (h *VideoHandler) PublicContent(c *gin.Context) {
	taskID := c.Param("task_id")
	if !h.videos.VerifyContentSignature(taskID, c.Query("expires"), c.Query("signature"), time.Now()) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Video link is invalid or expired"}})
		return
	}
	task, err := h.videos.FindTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Video not found"}})
		return
	}
	h.stream(c, task)
}

func (h *VideoHandler) stream(c *gin.Context, task *service.VideoTask) {
	if err := h.videos.StreamContent(c.Request.Context(), c, task); err != nil && !c.Writer.Written() {
		status := http.StatusBadGateway
		if errors.Is(err, service.ErrVideoContentUnavailable) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": gin.H{"type": "not_found_error", "message": "Video content is not available"}})
	}
}

// Cancel 取消统一任务（DELETE /v1/videos/:id）。
func (h *VideoHandler) Cancel(c *gin.Context) {
	task, ok := h.ownedTask(c)
	if !ok {
		return
	}
	if _, err := h.videos.Cancel(c.Request.Context(), task); err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, video.ErrUnsupported) {
			status = http.StatusBadRequest
		}
		h.openai.errorResponse(c, status, "invalid_request_error", "Video task cannot be cancelled: "+err.Error())
		return
	}
	if fresh, err := h.videos.FindTask(c.Request.Context(), task.TaskID); err == nil {
		task = fresh
	}
	c.JSON(http.StatusOK, h.videos.VideoTaskView(task, requestPublicBase(c)))
}

func (h *VideoHandler) ownedTask(c *gin.Context) (*service.VideoTask, bool) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.openai.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return nil, false
	}
	taskID := video.FirstNonEmpty(c.Param("request_id"), c.Param("task_id"))
	task, err := h.videos.FindTaskForOwner(c.Request.Context(), taskID, apiKey.UserID, apiKey.ID)
	if err != nil {
		h.openai.errorResponse(c, http.StatusNotFound, "not_found_error", "Video task not found")
		return nil, false
	}
	return task, true
}

// requestPublicBase 返回客户端访问网关所用的外部地址，用于拼接成品下载链接。
func requestPublicBase(c *gin.Context) string {
	scheme := "https"
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	} else if c.Request.TLS == nil {
		scheme = "http"
	}
	host := video.FirstNonEmpty(c.GetHeader("X-Forwarded-Host"), c.Request.Host)
	return scheme + "://" + host
}
