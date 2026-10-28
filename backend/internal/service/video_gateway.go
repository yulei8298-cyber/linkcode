package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/Wei-Shaw/sub2api/internal/service/video/registry"
)

// videoUpstreamBodyLimit 限制读取的上游 JSON 响应大小，视频成品走流式下载不受此限制。
const videoUpstreamBodyLimit = 8 << 20

// ErrVideoAccountMisconfigured 表示视频账号缺少协议或密钥。
var ErrVideoAccountMisconfigured = errors.New("video account is misconfigured")

// VideoProtocolCapability 是调度器按视频协议筛选账号使用的能力标识（原生协议透传必须
// 落到同协议账号；统一入口不限协议）。
func VideoProtocolCapability(protocol string) OpenAIEndpointCapability {
	return OpenAIEndpointCapability("video_protocol:" + strings.TrimSpace(protocol))
}

// VideoGatewayService 负责视频任务的提交、轮询、结算与成品下载。
// 调度、并发与扣费复用 OpenAIGatewayService，视频平台账号按协议实现转换。
type VideoGatewayService struct {
	gateway       *OpenAIGatewayService
	repo          VideoTaskRepository
	accountRepo   AccountRepository
	apiKeyService *APIKeyService
	subscriptions *SubscriptionService
	cfg           *config.Config

	stopOnce sync.Once
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewVideoGatewayService 创建视频网关服务（不启动后台轮询，见 Start）。
func NewVideoGatewayService(
	gateway *OpenAIGatewayService,
	repo VideoTaskRepository,
	accountRepo AccountRepository,
	apiKeyService *APIKeyService,
	subscriptions *SubscriptionService,
	cfg *config.Config,
) *VideoGatewayService {
	return &VideoGatewayService{
		gateway:       gateway,
		repo:          repo,
		accountRepo:   accountRepo,
		apiKeyService: apiKeyService,
		subscriptions: subscriptions,
		cfg:           cfg,
		stopCh:        make(chan struct{}),
	}
}

// VideoProviderForAccount 返回账号配置的协议实现。
func VideoProviderForAccount(account *Account) (video.Provider, error) {
	if account == nil || account.Platform != PlatformVideo {
		return nil, fmt.Errorf("%w: not a video account", ErrVideoAccountMisconfigured)
	}
	protocol := account.GetCredential(video.CredentialKeyProtocol)
	provider, ok := registry.Lookup(protocol)
	if !ok {
		return nil, fmt.Errorf("%w: unknown video protocol %q", ErrVideoAccountMisconfigured, protocol)
	}
	return provider, nil
}

func videoAccountBaseURL(account *Account, provider video.Provider) string {
	return video.FirstNonEmpty(account.GetCredential("base_url"), provider.Info().DefaultBaseURL)
}

func videoAccountCredentials(account *Account, provider video.Provider) video.Credentials {
	return video.Credentials{
		BaseURL:   videoAccountBaseURL(account, provider),
		APIKey:    account.GetCredential("api_key"),
		SecretKey: account.GetCredential("secret_key"),
	}
}

// VideoUpstreamResponse 是读取完毕的上游 JSON 响应。
type VideoUpstreamResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// buildVideoRequest 组装上游请求：拼接 URL、写入协议鉴权与账号自定义请求头。
func (s *VideoGatewayService) buildVideoRequest(ctx context.Context, account *Account, provider video.Provider, call video.Call) (*http.Request, error) {
	base, err := s.gateway.validateUpstreamBaseURL(videoAccountBaseURL(account, provider))
	if err != nil {
		return nil, err
	}
	target := video.JoinURL(base, call.Path)
	if len(call.Query) > 0 {
		target += "?" + call.Query.Encode()
	}
	var body io.Reader
	if len(call.Body) > 0 {
		body = bytes.NewReader(call.Body)
	}
	req, err := http.NewRequestWithContext(ctx, call.Method, target, body)
	if err != nil {
		return nil, err
	}
	if len(call.Body) > 0 {
		req.Header.Set("Content-Type", video.FirstNonEmpty(call.ContentType, "application/json"))
	}
	req.Header.Set("Accept", "application/json")
	for k, values := range call.Headers {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}
	if err := provider.Authorize(req.Header, videoAccountCredentials(account, provider)); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVideoAccountMisconfigured, err)
	}
	account.ApplyHeaderOverrides(req.Header)
	return req, nil
}

func videoAccountProxy(account *Account) string {
	if account.ProxyID != nil && account.Proxy != nil {
		return account.Proxy.URL()
	}
	return ""
}

// doVideoCall 执行一次上游 JSON 调用并读取响应体（非 2xx 也返回响应，由调用方处理）。
func (s *VideoGatewayService) doVideoCall(ctx context.Context, account *Account, provider video.Provider, call video.Call) (*VideoUpstreamResponse, error) {
	req, err := s.buildVideoRequest(ctx, account, provider, call)
	if err != nil {
		return nil, err
	}
	resp, err := s.gateway.httpUpstream.Do(req, videoAccountProxy(account), account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, videoUpstreamBodyLimit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > videoUpstreamBodyLimit {
		return nil, fmt.Errorf("video upstream response exceeds %d bytes", videoUpstreamBodyLimit)
	}
	return &VideoUpstreamResponse{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: body}, nil
}

// VideoSubmitInput 描述一次提交：统一入口时 Call 为空，由协议从 Request 构建；
// 原生透传时 Call 携带客户端原始路径与（已改写模型的）请求体。
type VideoSubmitInput struct {
	APIKey          *APIKey
	Account         *Account
	Request         video.Request
	RequestedModel  string
	UpstreamModel   string
	NativeCall      *video.Call
	EstimatedCost   float64
	InboundEndpoint string
	ClientIP        string
	UserAgent       string
}

// VideoSubmitResult 是提交结果；Upstream 为上游原始响应（原生透传直接回写给客户端）。
type VideoSubmitResult struct {
	Task     *VideoTask
	Upstream *VideoUpstreamResponse
}

// VideoUpstreamError 表示上游拒绝了请求；StatusCode 与 Body 原样回传给客户端。
type VideoUpstreamError struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

func (e *VideoUpstreamError) Error() string {
	return fmt.Sprintf("video upstream status %d", e.StatusCode)
}

// Submit 提交创建请求并落库。异步创建不做跨账号重试：上游可能已受理计费任务。
func (s *VideoGatewayService) Submit(ctx context.Context, in VideoSubmitInput) (*VideoSubmitResult, error) {
	provider, err := VideoProviderForAccount(in.Account)
	if err != nil {
		return nil, err
	}
	call := video.Call{}
	if in.NativeCall != nil {
		call = *in.NativeCall
	} else {
		upstreamReq := in.Request
		upstreamReq.Model = in.UpstreamModel
		if call, err = provider.BuildCreate(upstreamReq); err != nil {
			return nil, err
		}
	}
	resp, err := s.doVideoCall(ctx, in.Account, provider, call)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, &VideoUpstreamError{StatusCode: resp.StatusCode, Body: resp.Body, Header: resp.Header}
	}
	created, err := provider.ParseCreate(resp.Body)
	if err != nil {
		return nil, &VideoUpstreamError{StatusCode: http.StatusBadGateway, Body: resp.Body, Header: resp.Header}
	}
	if metaProvider, ok := provider.(video.CreateMetaProvider); ok && created.Meta == "" {
		created.Meta = metaProvider.CreateMeta(call.Path)
	}
	taskID, err := NewVideoTaskID()
	if err != nil {
		return nil, err
	}
	source := VideoTaskSourceUnified
	if in.NativeCall != nil {
		source = VideoTaskSourceNative
	}
	nextPoll := time.Now().Add(videoFirstPollDelay)
	generateAudio := in.Request.GenerateAudio != nil && *in.Request.GenerateAudio
	task := &VideoTask{
		TaskID:          taskID,
		UserID:          in.APIKey.UserID,
		APIKeyID:        in.APIKey.ID,
		GroupID:         in.APIKey.GroupID,
		AccountID:       in.Account.ID,
		Protocol:        string(provider.Info().Protocol),
		Source:          source,
		Model:           in.RequestedModel,
		UpstreamModel:   in.UpstreamModel,
		UpstreamTaskID:  created.TaskID,
		UpstreamMeta:    created.Meta,
		Status:          video.StatusQueued,
		Resolution:      in.Request.Resolution,
		AspectRatio:     in.Request.AspectRatio,
		DurationSeconds: in.Request.DurationSeconds,
		VideoCount:      1,
		GenerateAudio:   generateAudio,
		EstimatedCost:   in.EstimatedCost,
		NextPollAt:      &nextPoll,
		BillingStatus:   VideoBillingPending,
		InboundEndpoint: in.InboundEndpoint,
		ClientIP:        in.ClientIP,
		UserAgent:       in.UserAgent,
	}
	if err := s.repo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("persist video task: %w", err)
	}
	return &VideoSubmitResult{Task: task, Upstream: resp}, nil
}

// FindTask 按任务 ID 查找（签名下载链接与管理端使用，不校验归属）。
func (s *VideoGatewayService) FindTask(ctx context.Context, taskID string) (*VideoTask, error) {
	return s.repo.GetByTaskID(ctx, strings.TrimSpace(taskID))
}

// ListTasks 供管理端分页查询任务。
func (s *VideoGatewayService) ListTasks(ctx context.Context, filter VideoTaskFilter) ([]*VideoTask, int64, error) {
	return s.repo.List(ctx, filter)
}

// FindTaskForOwner 按统一任务 ID 查找，只允许提交者本人（同用户同 Key）访问。
func (s *VideoGatewayService) FindTaskForOwner(ctx context.Context, taskID string, userID, apiKeyID int64) (*VideoTask, error) {
	task, err := s.repo.GetByTaskID(ctx, strings.TrimSpace(taskID))
	if err != nil {
		return nil, err
	}
	if task.UserID != userID || task.APIKeyID != apiKeyID {
		return nil, ErrVideoTaskNotFound
	}
	return task, nil
}

// FindNativeTaskForOwner 按原生协议的上游任务 ID 或文件 ID 查找提交者的任务。
func (s *VideoGatewayService) FindNativeTaskForOwner(ctx context.Context, protocol string, op video.NativeOp, userID, apiKeyID int64) (*VideoTask, error) {
	if op.Kind == video.NativeFile {
		return s.repo.GetByFileID(ctx, protocol, op.FileID, userID, apiKeyID)
	}
	return s.repo.GetByUpstream(ctx, protocol, op.TaskID, userID, apiKeyID)
}

// LoadTaskAccount 加载任务所属账号（查询、下载、取消必须回到提交账号）。
func (s *VideoGatewayService) LoadTaskAccount(ctx context.Context, task *VideoTask) (*Account, video.Provider, error) {
	account, err := s.accountRepo.GetByID(ctx, task.AccountID)
	if err != nil {
		return nil, nil, err
	}
	provider, err := VideoProviderForAccount(account)
	if err != nil {
		return nil, nil, err
	}
	return account, provider, nil
}
