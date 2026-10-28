package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/service/video"
)

type videoTaskRepository struct {
	// db 可以是 *sql.DB 或 *sql.Tx（集成测试用事务包裹并回滚）。
	db batchImageSQLExecutor
}

// NewVideoTaskRepository 创建视频任务仓储（原生 SQL，与 batch_image_jobs 一致）。
func NewVideoTaskRepository(db *sql.DB) service.VideoTaskRepository {
	return &videoTaskRepository{db: db}
}

const videoTaskColumns = `id, task_id, user_id, api_key_id, group_id, account_id, protocol, source, model,
 upstream_model, upstream_task_id, upstream_meta, status, progress, resolution, aspect_ratio,
 duration_seconds, actual_duration_seconds, video_count, generate_audio, estimated_cost, actual_cost,
 completion_tokens, video_url, file_id, error_code, error_message, poll_count, next_poll_at,
 billing_status, billing_attempts, billing_error, inbound_endpoint, client_ip, user_agent,
 created_at, updated_at, finished_at, billed_at`

const videoTaskSelectSQL = "SELECT " + videoTaskColumns + " FROM video_tasks"

// videoTaskTerminalSQL 是终态集合，供 SQL 条件复用。
var videoTaskTerminalSQL = fmt.Sprintf("('%s','%s','%s')", video.StatusSucceeded, video.StatusFailed, video.StatusCancelled)

func (r *videoTaskRepository) Create(ctx context.Context, t *service.VideoTask) error {
	row := r.db.QueryRowContext(ctx, `INSERT INTO video_tasks (
 task_id, user_id, api_key_id, group_id, account_id, protocol, source, model, upstream_model,
 upstream_task_id, upstream_meta, status, progress, resolution, aspect_ratio, duration_seconds,
 video_count, generate_audio, estimated_cost, next_poll_at, billing_status, inbound_endpoint,
 client_ip, user_agent)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)
 RETURNING id, created_at, updated_at`,
		t.TaskID, t.UserID, t.APIKeyID, t.GroupID, t.AccountID, t.Protocol, t.Source, t.Model, t.UpstreamModel,
		t.UpstreamTaskID, t.UpstreamMeta, string(t.Status), t.Progress, t.Resolution, t.AspectRatio, t.DurationSeconds,
		t.VideoCount, t.GenerateAudio, t.EstimatedCost, t.NextPollAt, t.BillingStatus, t.InboundEndpoint,
		truncateRunes(t.ClientIP, 64), truncateRunes(t.UserAgent, 512),
	)
	if err := row.Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt); err != nil {
		if isUniqueConstraintViolation(err) {
			return service.ErrVideoTaskExists
		}
		return err
	}
	return nil
}

func (r *videoTaskRepository) getOne(ctx context.Context, where string, args ...any) (*service.VideoTask, error) {
	task, err := scanVideoTask(r.db.QueryRowContext(ctx, videoTaskSelectSQL+" WHERE "+where, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrVideoTaskNotFound
	}
	return task, err
}

func (r *videoTaskRepository) GetByTaskID(ctx context.Context, taskID string) (*service.VideoTask, error) {
	return r.getOne(ctx, "task_id = $1", taskID)
}

func (r *videoTaskRepository) GetByUpstream(ctx context.Context, protocol, upstreamTaskID string, userID, apiKeyID int64) (*service.VideoTask, error) {
	return r.getOne(ctx, "protocol = $1 AND upstream_task_id = $2 AND user_id = $3 AND api_key_id = $4 ORDER BY id DESC LIMIT 1",
		protocol, upstreamTaskID, userID, apiKeyID)
}

func (r *videoTaskRepository) GetByFileID(ctx context.Context, protocol, fileID string, userID, apiKeyID int64) (*service.VideoTask, error) {
	return r.getOne(ctx, "protocol = $1 AND file_id = $2 AND user_id = $3 AND api_key_id = $4 ORDER BY id DESC LIMIT 1",
		protocol, fileID, userID, apiKeyID)
}

func (r *videoTaskRepository) UpdateSnapshot(ctx context.Context, id int64, u service.VideoTaskSnapshotUpdate) error {
	// 终态一旦写入不再被后续（可能乱序到达的）非终态快照覆盖。
	_, err := r.db.ExecContext(ctx, `UPDATE video_tasks SET
 status = CASE WHEN status IN `+videoTaskTerminalSQL+` THEN status ELSE $2 END,
 progress = GREATEST(progress, $3),
 video_url = CASE WHEN $4 <> '' THEN $4 ELSE video_url END,
 file_id = CASE WHEN $5 <> '' THEN $5 ELSE file_id END,
 actual_duration_seconds = COALESCE($6, actual_duration_seconds),
 resolution = CASE WHEN $7 <> '' AND resolution = '' THEN $7 ELSE resolution END,
 completion_tokens = GREATEST(completion_tokens, $8),
 error_code = CASE WHEN $9 <> '' THEN $9 ELSE error_code END,
 error_message = CASE WHEN $10 <> '' THEN $10 ELSE error_message END,
 next_poll_at = $11,
 finished_at = COALESCE(finished_at, $12),
 poll_count = poll_count + 1,
 updated_at = NOW()
 WHERE id = $1`,
		id, string(u.Status), u.Progress, u.VideoURL, u.FileID, u.ActualDurationSeconds, u.Resolution,
		u.CompletionTokens, truncateRunes(u.ErrorCode, 128), u.ErrorMessage, u.NextPollAt, u.FinishedAt)
	return err
}

func (r *videoTaskRepository) ClaimDuePolls(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]*service.VideoTask, error) {
	rows, err := r.db.QueryContext(ctx, `UPDATE video_tasks SET next_poll_at = $2, updated_at = NOW()
 WHERE id IN (
   SELECT id FROM video_tasks
   WHERE status IN ('`+string(video.StatusQueued)+`','`+string(video.StatusRunning)+`')
     AND (next_poll_at IS NULL OR next_poll_at <= $1)
   ORDER BY next_poll_at NULLS FIRST
   LIMIT $3
   FOR UPDATE SKIP LOCKED
 )
 RETURNING `+videoTaskColumns, now, now.Add(lease), limit)
	if err != nil {
		return nil, err
	}
	return scanVideoTasks(rows)
}

func (r *videoTaskRepository) ListBillingPending(ctx context.Context, olderThan time.Time, limit int) ([]*service.VideoTask, error) {
	rows, err := r.db.QueryContext(ctx, videoTaskSelectSQL+`
 WHERE status = $1 AND billing_status = $2 AND updated_at <= $3
 ORDER BY updated_at LIMIT $4`, string(video.StatusSucceeded), service.VideoBillingPending, olderThan, limit)
	if err != nil {
		return nil, err
	}
	return scanVideoTasks(rows)
}

func (r *videoTaskRepository) MarkBilling(ctx context.Context, id int64, status string, actualCost *float64, billingErr string) error {
	// 是否写入 billed_at 单独传布尔参数：同一个参数既赋给 varchar 列又与字符串常量比较时，
	// PostgreSQL 会报 "inconsistent types deduced for parameter"。
	_, err := r.db.ExecContext(ctx, `UPDATE video_tasks SET
 billing_status = $2,
 actual_cost = COALESCE($3, actual_cost),
 billing_error = $4,
 billing_attempts = billing_attempts + 1,
 billed_at = CASE WHEN $5 THEN NOW() ELSE billed_at END,
 updated_at = NOW()
 WHERE id = $1 AND billing_status <> 'billed'`, id, status, actualCost, billingErr, status == service.VideoBillingBilled)
	return err
}

func (r *videoTaskRepository) SumPendingEstimatedCost(ctx context.Context, userID int64) (float64, error) {
	var sum sql.NullFloat64
	err := r.db.QueryRowContext(ctx, `SELECT SUM(estimated_cost) FROM video_tasks
 WHERE user_id = $1 AND billing_status = $2 AND status NOT IN ('`+string(video.StatusFailed)+`','`+string(video.StatusCancelled)+`')`,
		userID, service.VideoBillingPending).Scan(&sum)
	return sum.Float64, err
}

func (r *videoTaskRepository) List(ctx context.Context, f service.VideoTaskFilter) ([]*service.VideoTask, int64, error) {
	where := []string{"1=1"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if f.UserID > 0 {
		add("user_id = ?", f.UserID)
	}
	if f.Status != "" {
		add("status = ?", f.Status)
	}
	if f.BillingStatus != "" {
		add("billing_status = ?", f.BillingStatus)
	}
	if f.Protocol != "" {
		add("protocol = ?", f.Protocol)
	}
	if f.Model != "" {
		add("model ILIKE ?", "%"+f.Model+"%")
	}
	if f.Keyword != "" {
		// 同一占位符在条件中出现两次，共用一个参数。
		add("(task_id = ? OR upstream_task_id = ?)", f.Keyword)
	}
	cond := " WHERE " + strings.Join(where, " AND ")
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM video_tasks"+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	args = append(args, limit, max(f.Offset, 0))
	rows, err := r.db.QueryContext(ctx, videoTaskSelectSQL+cond+" ORDER BY id DESC LIMIT $"+strconv.Itoa(len(args)-1)+" OFFSET $"+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	tasks, err := scanVideoTasks(rows)
	return tasks, total, err
}

func scanVideoTask(row rowScanner) (*service.VideoTask, error) {
	var t service.VideoTask
	var groupID sql.NullInt64
	var actualDuration, actualCost sql.NullFloat64
	var nextPollAt, finishedAt, billedAt sql.NullTime
	var status string
	err := row.Scan(
		&t.ID, &t.TaskID, &t.UserID, &t.APIKeyID, &groupID, &t.AccountID, &t.Protocol, &t.Source, &t.Model,
		&t.UpstreamModel, &t.UpstreamTaskID, &t.UpstreamMeta, &status, &t.Progress, &t.Resolution, &t.AspectRatio,
		&t.DurationSeconds, &actualDuration, &t.VideoCount, &t.GenerateAudio, &t.EstimatedCost, &actualCost,
		&t.CompletionTokens, &t.VideoURL, &t.FileID, &t.ErrorCode, &t.ErrorMessage, &t.PollCount, &nextPollAt,
		&t.BillingStatus, &t.BillingAttempts, &t.BillingError, &t.InboundEndpoint, &t.ClientIP, &t.UserAgent,
		&t.CreatedAt, &t.UpdatedAt, &finishedAt, &billedAt,
	)
	if err != nil {
		return nil, err
	}
	t.Status = video.Status(status)
	if groupID.Valid {
		t.GroupID = &groupID.Int64
	}
	if actualDuration.Valid {
		t.ActualDurationSeconds = &actualDuration.Float64
	}
	if actualCost.Valid {
		t.ActualCost = &actualCost.Float64
	}
	if nextPollAt.Valid {
		t.NextPollAt = &nextPollAt.Time
	}
	if finishedAt.Valid {
		t.FinishedAt = &finishedAt.Time
	}
	if billedAt.Valid {
		t.BilledAt = &billedAt.Time
	}
	return &t, nil
}

func scanVideoTasks(rows *sql.Rows) ([]*service.VideoTask, error) {
	defer func() { _ = rows.Close() }()
	var out []*service.VideoTask
	for rows.Next() {
		t, err := scanVideoTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// truncateRunes 按字符截断，避免超长 UA 等写库失败。
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
