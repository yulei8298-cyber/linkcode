package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
)

// 视频任务来源：统一入口（网关生成 vt_ 任务 ID）或厂商原生协议透传（沿用上游任务 ID）。
const (
	VideoTaskSourceUnified = "unified"
	VideoTaskSourceNative  = "native"
)

// 视频任务的账务状态。
const (
	VideoBillingPending = "pending"
	VideoBillingBilled  = "billed"
	VideoBillingSkipped = "skipped"
	VideoBillingFailed  = "failed"
)

// videoTaskIDPrefix 是统一入口任务 ID 前缀，路由据此把查询分派给视频网关。
const videoTaskIDPrefix = "vt_"

var (
	ErrVideoTaskNotFound = errors.New("video task not found")
	ErrVideoTaskExists   = errors.New("video task already exists")
)

// VideoTask 是 video_tasks 表的领域对象。
type VideoTask struct {
	ID                    int64        `json:"id"`
	TaskID                string       `json:"task_id"`
	UserID                int64        `json:"user_id"`
	APIKeyID              int64        `json:"api_key_id"`
	GroupID               *int64       `json:"group_id"`
	AccountID             int64        `json:"account_id"`
	Protocol              string       `json:"protocol"`
	Source                string       `json:"source"`
	Model                 string       `json:"model"`
	UpstreamModel         string       `json:"upstream_model"`
	UpstreamTaskID        string       `json:"upstream_task_id"`
	UpstreamMeta          string       `json:"upstream_meta"`
	Status                video.Status `json:"status"`
	Progress              int          `json:"progress"`
	Resolution            string       `json:"resolution"`
	AspectRatio           string       `json:"aspect_ratio"`
	DurationSeconds       int          `json:"duration_seconds"`
	ActualDurationSeconds *float64     `json:"actual_duration_seconds"`
	VideoCount            int          `json:"video_count"`
	GenerateAudio         bool         `json:"generate_audio"`
	EstimatedCost         float64      `json:"estimated_cost"`
	ActualCost            *float64     `json:"actual_cost"`
	CompletionTokens      int64        `json:"completion_tokens"`
	// VideoURL 是上游成品地址（可能需要上游鉴权、暴露上游域名），只供网关内部下载，
	// 不序列化给任何接口；对外一律使用 SignedContentPath 生成的网关签名链接。
	VideoURL        string     `json:"-"`
	FileID          string     `json:"file_id"`
	ErrorCode       string     `json:"error_code"`
	ErrorMessage    string     `json:"error_message"`
	PollCount       int        `json:"poll_count"`
	NextPollAt      *time.Time `json:"next_poll_at"`
	BillingStatus   string     `json:"billing_status"`
	BillingAttempts int        `json:"billing_attempts"`
	BillingError    string     `json:"billing_error"`
	InboundEndpoint string     `json:"inbound_endpoint"`
	ClientIP        string     `json:"client_ip"`
	UserAgent       string     `json:"user_agent"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	BilledAt        *time.Time `json:"billed_at"`
}

// BillingDurationSeconds 返回结算时长：优先上游实际时长，其次请求时长。
func (t *VideoTask) BillingDurationSeconds() int {
	if t.ActualDurationSeconds != nil && *t.ActualDurationSeconds > 0 {
		return int(*t.ActualDurationSeconds + 0.5)
	}
	return t.DurationSeconds
}

// VideoTaskSnapshotUpdate 是一次轮询后写回的状态。
type VideoTaskSnapshotUpdate struct {
	Status                video.Status
	Progress              int
	VideoURL              string
	FileID                string
	ActualDurationSeconds *float64
	Resolution            string
	CompletionTokens      int64
	ErrorCode             string
	ErrorMessage          string
	NextPollAt            *time.Time
	FinishedAt            *time.Time
}

// VideoTaskFilter 是管理端列表筛选条件。
type VideoTaskFilter struct {
	UserID        int64
	Status        string
	BillingStatus string
	Protocol      string
	Model         string
	Keyword       string
	Limit         int
	Offset        int
}

// VideoTaskRepository 持久化视频任务。实现须保证 Create 对 (account_id, upstream_task_id) 唯一。
type VideoTaskRepository interface {
	Create(ctx context.Context, task *VideoTask) error
	GetByTaskID(ctx context.Context, taskID string) (*VideoTask, error)
	GetByUpstream(ctx context.Context, protocol, upstreamTaskID string, userID, apiKeyID int64) (*VideoTask, error)
	GetByFileID(ctx context.Context, protocol, fileID string, userID, apiKeyID int64) (*VideoTask, error)
	UpdateSnapshot(ctx context.Context, id int64, update VideoTaskSnapshotUpdate) error
	// ClaimDuePolls 原子地领取到期的非终态任务并把 next_poll_at 推后 lease，防止多实例重复轮询。
	ClaimDuePolls(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]*VideoTask, error)
	// ListBillingPending 返回已成功但尚未结算的任务。
	ListBillingPending(ctx context.Context, olderThan time.Time, limit int) ([]*VideoTask, error)
	// MarkBilling 写入结算结果；status 为 pending 时累加尝试次数。
	MarkBilling(ctx context.Context, id int64, status string, actualCost *float64, billingErr string) error
	// SumPendingEstimatedCost 返回用户未结算任务的预估费用之和，用于创建前的余额校验。
	SumPendingEstimatedCost(ctx context.Context, userID int64) (float64, error)
	List(ctx context.Context, filter VideoTaskFilter) ([]*VideoTask, int64, error)
}

// NewVideoTaskID 生成统一入口的任务 ID。
func NewVideoTaskID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return videoTaskIDPrefix + hex.EncodeToString(b[:]), nil
}

// IsVideoTaskID 报告是否为视频网关生成的任务 ID。
func IsVideoTaskID(id string) bool {
	return len(id) > len(videoTaskIDPrefix) && id[:len(videoTaskIDPrefix)] == videoTaskIDPrefix
}

// videoTaskBillingRequestID 是结算写 usage_logs 的幂等请求 ID。
func videoTaskBillingRequestID(task *VideoTask) string {
	return "video-task:" + task.TaskID
}
