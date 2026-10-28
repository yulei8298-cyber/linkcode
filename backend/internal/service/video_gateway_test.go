//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// videoUpstreamStub 按 "METHOD path" 返回预置响应序列，记录全部请求。
type videoUpstreamStub struct {
	mu        sync.Mutex
	responses map[string][]string
	statuses  map[string]int
	requests  []*http.Request
	bodies    []string
}

func (u *videoUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	body := ""
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		body = string(raw)
	}
	u.requests = append(u.requests, req)
	u.bodies = append(u.bodies, body)
	key := req.Method + " " + req.URL.Path
	queue := u.responses[key]
	payload := `{"error":"not found"}`
	status := http.StatusNotFound
	if len(queue) > 0 {
		payload, status = queue[0], http.StatusOK
		if len(queue) > 1 {
			u.responses[key] = queue[1:]
		}
		if s, ok := u.statuses[key]; ok {
			status = s
		}
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(payload))}, nil
}

func (u *videoUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

// memoryVideoTaskRepo 是 VideoTaskRepository 的内存实现，语义与 SQL 实现一致（终态不回退）。
type memoryVideoTaskRepo struct {
	mu    sync.Mutex
	tasks []*VideoTask
}

func (r *memoryVideoTaskRepo) Create(_ context.Context, t *VideoTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t.ID = int64(len(r.tasks) + 1)
	t.CreatedAt, t.UpdatedAt = time.Now(), time.Now()
	cp := *t
	r.tasks = append(r.tasks, &cp)
	return nil
}

func (r *memoryVideoTaskRepo) find(match func(*VideoTask) bool) (*VideoTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tasks {
		if match(t) {
			cp := *t
			return &cp, nil
		}
	}
	return nil, ErrVideoTaskNotFound
}

func (r *memoryVideoTaskRepo) GetByTaskID(_ context.Context, id string) (*VideoTask, error) {
	return r.find(func(t *VideoTask) bool { return t.TaskID == id })
}

func (r *memoryVideoTaskRepo) GetByUpstream(_ context.Context, protocol, upstream string, userID, keyID int64) (*VideoTask, error) {
	return r.find(func(t *VideoTask) bool {
		return t.Protocol == protocol && t.UpstreamTaskID == upstream && t.UserID == userID && t.APIKeyID == keyID
	})
}

func (r *memoryVideoTaskRepo) GetByFileID(_ context.Context, protocol, fileID string, userID, keyID int64) (*VideoTask, error) {
	return r.find(func(t *VideoTask) bool {
		return t.Protocol == protocol && t.FileID == fileID && t.UserID == userID && t.APIKeyID == keyID
	})
}

func (r *memoryVideoTaskRepo) UpdateSnapshot(_ context.Context, id int64, u VideoTaskSnapshotUpdate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.tasks[id-1]
	if !t.Status.IsTerminal() {
		t.Status = u.Status
	}
	t.Progress = max(t.Progress, u.Progress)
	if u.VideoURL != "" {
		t.VideoURL = u.VideoURL
	}
	if u.FileID != "" {
		t.FileID = u.FileID
	}
	if u.ActualDurationSeconds != nil {
		t.ActualDurationSeconds = u.ActualDurationSeconds
	}
	if u.Resolution != "" && t.Resolution == "" {
		t.Resolution = u.Resolution
	}
	if u.CompletionTokens > t.CompletionTokens {
		t.CompletionTokens = u.CompletionTokens
	}
	if u.ErrorMessage != "" {
		t.ErrorMessage = u.ErrorMessage
	}
	t.NextPollAt = u.NextPollAt
	if t.FinishedAt == nil {
		t.FinishedAt = u.FinishedAt
	}
	t.PollCount++
	return nil
}

func (r *memoryVideoTaskRepo) ClaimDuePolls(context.Context, time.Time, time.Duration, int) ([]*VideoTask, error) {
	return nil, nil
}

func (r *memoryVideoTaskRepo) ListBillingPending(context.Context, time.Time, int) ([]*VideoTask, error) {
	return nil, nil
}

func (r *memoryVideoTaskRepo) MarkBilling(_ context.Context, id int64, status string, cost *float64, billingErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.tasks[id-1]
	if t.BillingStatus != VideoBillingBilled {
		t.BillingStatus, t.BillingError = status, billingErr
		t.BillingAttempts++
		if cost != nil {
			t.ActualCost = cost
		}
	}
	return nil
}

func (r *memoryVideoTaskRepo) SumPendingEstimatedCost(_ context.Context, userID int64) (float64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sum := 0.0
	for _, t := range r.tasks {
		if t.UserID == userID && t.BillingStatus == VideoBillingPending && t.Status != video.StatusFailed && t.Status != video.StatusCancelled {
			sum += t.EstimatedCost
		}
	}
	return sum, nil
}

func (r *memoryVideoTaskRepo) List(context.Context, VideoTaskFilter) ([]*VideoTask, int64, error) {
	return r.tasks, int64(len(r.tasks)), nil
}

type videoAccountRepoStub struct {
	AccountRepository
	account *Account
}

func (s *videoAccountRepoStub) GetByID(context.Context, int64) (*Account, error) {
	return s.account, nil
}

func newVideoTestService(t *testing.T, protocol, baseURL string, upstream *videoUpstreamStub) (*VideoGatewayService, *memoryVideoTaskRepo, *Account) {
	t.Helper()
	account := &Account{ID: 7, Platform: PlatformVideo, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{
		video.CredentialKeyProtocol: protocol, "base_url": baseURL, "api_key": "upstream-key",
	}}
	repo := &memoryVideoTaskRepo{}
	gateway := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	svc := NewVideoGatewayService(gateway, repo, &videoAccountRepoStub{account: account}, nil, nil, &config.Config{JWT: config.JWTConfig{Secret: "s3cret"}})
	return svc, repo, account
}

func testVideoAPIKey() *APIKey {
	return &APIKey{ID: 3, UserID: 9, User: &User{ID: 9, Balance: 10}}
}

func TestVideoGatewaySubmitAndRefreshDashScope(t *testing.T) {
	upstream := &videoUpstreamStub{responses: map[string][]string{
		"POST /aliyun/api/v1/services/aigc/video-generation/video-synthesis": {`{"output":{"task_id":"ds-1","task_status":"PENDING"}}`},
		"GET /aliyun/api/v1/tasks/ds-1": {
			`{"output":{"task_status":"RUNNING"}}`,
			`{"output":{"task_status":"SUCCEEDED","video_url":"https://oss.example/v.mp4"},"usage":{"output_video_duration":20,"SR":1080}}`,
		},
	}}
	svc, repo, account := newVideoTestService(t, string(video.ProtocolDashScope), "https://relay.example/aliyun", upstream)
	req := video.Request{Model: "wan3", Prompt: "海浪", DurationSeconds: 20, Resolution: video.Resolution1080P, GenerateAudio: video.BoolPtr(true)}

	result, err := svc.Submit(context.Background(), VideoSubmitInput{APIKey: testVideoAPIKey(), Account: account, Request: req, RequestedModel: "wan3", UpstreamModel: "wan3.0-video", EstimatedCost: 3.4})
	require.NoError(t, err)
	require.True(t, IsVideoTaskID(result.Task.TaskID))
	require.Equal(t, "ds-1", result.Task.UpstreamTaskID)
	require.Equal(t, "https://relay.example/aliyun/api/v1/services/aigc/video-generation/video-synthesis", upstream.requests[0].URL.String(), "第三方中转地址按原样拼接协议路径")
	require.Equal(t, "Bearer upstream-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "enable", upstream.requests[0].Header.Get("X-DashScope-Async"))
	require.Equal(t, "wan3.0-video", gjson.Get(upstream.bodies[0], "model").String(), "提交时使用映射后的上游模型")

	task, _, err := svc.Refresh(context.Background(), result.Task)
	require.NoError(t, err)
	require.Equal(t, video.StatusRunning, task.Status)
	require.NotNil(t, task.NextPollAt)

	task, _, err = svc.Refresh(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, video.StatusSucceeded, task.Status)
	require.Equal(t, "https://oss.example/v.mp4", task.VideoURL)
	require.Equal(t, 20, task.BillingDurationSeconds())
	require.Equal(t, VideoBillingPending, task.BillingStatus, "结算依赖未装配时保持待结算，由补结算循环重试")
	require.NotEmpty(t, repo.tasks[0].BillingError)

	pending, err := repo.SumPendingEstimatedCost(context.Background(), 9)
	require.NoError(t, err)
	require.InDelta(t, 3.4, pending, 1e-9)
}

func TestVideoGatewayFailedTaskSkipsBillingAndUpstreamErrorsPassThrough(t *testing.T) {
	upstream := &videoUpstreamStub{
		responses: map[string][]string{
			"POST /api/v3/contents/generations/tasks":      {`{"id":"cgt-1"}`},
			"GET /api/v3/contents/generations/tasks/cgt-1": {`{"status":"failed","error":{"code":"OutputVideoSensitive","message":"blocked"}}`},
		},
	}
	svc, repo, account := newVideoTestService(t, string(video.ProtocolArk), "https://ark.cn-beijing.volces.com/api/v3", upstream)
	result, err := svc.Submit(context.Background(), VideoSubmitInput{APIKey: testVideoAPIKey(), Account: account, Request: video.Request{Model: "seedance", Prompt: "p"}, RequestedModel: "seedance", UpstreamModel: "doubao-seedance-2-0-260128"})
	require.NoError(t, err)
	task, _, err := svc.Refresh(context.Background(), result.Task)
	require.NoError(t, err)
	require.Equal(t, video.StatusFailed, task.Status)
	require.Equal(t, VideoBillingSkipped, repo.tasks[0].BillingStatus)
	require.Equal(t, "blocked", task.ErrorMessage)

	view := svc.VideoTaskView(task, "https://gw.example")
	require.Equal(t, "failed", view["status"])
	require.Nil(t, view["url"])

	rejecting := &videoUpstreamStub{responses: map[string][]string{"POST /api/v3/contents/generations/tasks": {`{"error":{"code":"InvalidParameter"}}`}},
		statuses: map[string]int{"POST /api/v3/contents/generations/tasks": http.StatusBadRequest}}
	svc2, repo2, account2 := newVideoTestService(t, string(video.ProtocolArk), "https://ark.example", rejecting)
	_, err = svc2.Submit(context.Background(), VideoSubmitInput{APIKey: testVideoAPIKey(), Account: account2, Request: video.Request{Model: "m", Prompt: "p"}, RequestedModel: "m", UpstreamModel: "m"})
	var upstreamErr *VideoUpstreamError
	require.ErrorAs(t, err, &upstreamErr)
	require.Equal(t, http.StatusBadRequest, upstreamErr.StatusCode)
	require.Empty(t, repo2.tasks, "上游拒绝的创建不落库")
}

func TestVideoGatewayKlingMetaAndMiniMaxFileResolve(t *testing.T) {
	kling := &videoUpstreamStub{responses: map[string][]string{
		"POST /v1/videos/image2video":   {`{"code":0,"data":{"task_id":"k1"}}`},
		"GET /v1/videos/image2video/k1": {`{"code":0,"data":{"task_status":"processing"}}`},
	}}
	svc, _, account := newVideoTestService(t, string(video.ProtocolKling), "https://kling.relay", kling)
	req := video.Request{Model: "kling-v3", Prompt: "p", Images: []video.Image{{URL: "https://i/a.png", Role: video.ImageRoleFirstFrame}}}
	result, err := svc.Submit(context.Background(), VideoSubmitInput{APIKey: testVideoAPIKey(), Account: account, Request: req, RequestedModel: "kling-v3", UpstreamModel: "kling-v3"})
	require.NoError(t, err)
	require.Equal(t, "image2video", result.Task.UpstreamMeta)
	_, _, err = svc.Refresh(context.Background(), result.Task)
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/image2video/k1", kling.requests[1].URL.Path)

	minimax := &videoUpstreamStub{responses: map[string][]string{
		"POST /v1/video_generation":      {`{"task_id":"mm1","base_resp":{"status_code":0}}`},
		"GET /v1/query/video_generation": {`{"status":"Success","file_id":"f-9","base_resp":{"status_code":0}}`},
		"GET /v1/files/retrieve":         {`{"file":{"download_url":"https://dl.example/x.mp4"},"base_resp":{"status_code":0}}`},
	}}
	svc, _, account = newVideoTestService(t, string(video.ProtocolMiniMax), "https://api.minimaxi.com", minimax)
	result, err = svc.Submit(context.Background(), VideoSubmitInput{APIKey: testVideoAPIKey(), Account: account, Request: video.Request{Model: "MiniMax-Hailuo-2.3", Prompt: "p"}, RequestedModel: "MiniMax-Hailuo-2.3", UpstreamModel: "MiniMax-Hailuo-2.3"})
	require.NoError(t, err)
	task, _, err := svc.Refresh(context.Background(), result.Task)
	require.NoError(t, err)
	require.Equal(t, "https://dl.example/x.mp4", task.VideoURL)
	require.Equal(t, "f-9", task.FileID)
}

func TestVideoGatewayOpenAIContentPathAndSignedLink(t *testing.T) {
	upstream := &videoUpstreamStub{responses: map[string][]string{
		"POST /v1/videos":                {`{"id":"video_1","status":"queued"}`},
		"GET /v1/videos/video_1":         {`{"id":"video_1","status":"completed","seconds":"8"}`},
		"GET /v1/videos/video_1/content": {`MP4DATA`},
	}}
	svc, _, account := newVideoTestService(t, string(video.ProtocolOpenAIVideos), "https://relay.example/v1", upstream)
	result, err := svc.Submit(context.Background(), VideoSubmitInput{APIKey: testVideoAPIKey(), Account: account, Request: video.Request{Model: "sora-2", Prompt: "p"}, RequestedModel: "sora-2", UpstreamModel: "sora-2"})
	require.NoError(t, err)
	task, _, err := svc.Refresh(context.Background(), result.Task)
	require.NoError(t, err)
	require.Equal(t, "https://relay.example/v1/videos/video_1/content", task.VideoURL)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/video-files/x", nil)
	require.NoError(t, svc.StreamContent(context.Background(), c, task))
	require.Equal(t, "MP4DATA", w.Body.String())
	last := upstream.requests[len(upstream.requests)-1]
	require.Equal(t, "Bearer upstream-key", last.Header.Get("Authorization"), "同源成品地址下载时携带上游鉴权")

	view := svc.VideoTaskView(task, "https://gw.example")
	link := view["url"].(string)
	require.True(t, strings.HasPrefix(link, "https://gw.example/video-files/"+task.TaskID+"?expires="))
	parts := strings.Split(strings.SplitN(link, "?", 2)[1], "&")
	expires := strings.TrimPrefix(parts[0], "expires=")
	signature := strings.TrimPrefix(parts[1], "signature=")
	require.True(t, svc.VerifyContentSignature(task.TaskID, expires, signature, time.Now()))
	require.False(t, svc.VerifyContentSignature("vt_other", expires, signature, time.Now()))
	require.False(t, svc.VerifyContentSignature(task.TaskID, expires, signature, time.Now().Add(48*time.Hour)))
}

func TestVideoGatewayCheckBalanceCountsPendingTasks(t *testing.T) {
	svc, repo, _ := newVideoTestService(t, string(video.ProtocolArk), "https://ark.example", &videoUpstreamStub{})
	require.NoError(t, repo.Create(context.Background(), &VideoTask{TaskID: "vt_a", UserID: 9, EstimatedCost: 8, BillingStatus: VideoBillingPending, Status: video.StatusRunning}))
	key := testVideoAPIKey()
	require.NoError(t, svc.CheckBalance(context.Background(), key, nil, 2))
	require.ErrorIs(t, svc.CheckBalance(context.Background(), key, nil, 2.5), ErrVideoInsufficientBalance)
	key.Group = &Group{IsFree: true}
	require.NoError(t, svc.CheckBalance(context.Background(), key, nil, 100), "免费分组不以余额为前置条件")
}

func TestVideoProtocolCapabilityFiltersAccounts(t *testing.T) {
	account := &Account{Platform: PlatformVideo, Credentials: map[string]any{video.CredentialKeyProtocol: "kling"}}
	require.True(t, account.SupportsOpenAIEndpointCapability(""))
	require.True(t, account.SupportsOpenAIEndpointCapability(VideoProtocolCapability("kling")))
	require.False(t, account.SupportsOpenAIEndpointCapability(VideoProtocolCapability("ark")))
	require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions), "视频账号不承接对话")
}
