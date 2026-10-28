//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/stretchr/testify/require"
)

func newVideoTaskTestTask(taskID, upstreamID string, userID int64) *service.VideoTask {
	next := time.Now().Add(-time.Second)
	return &service.VideoTask{
		TaskID: taskID, UserID: userID, APIKeyID: 31, AccountID: 41, Protocol: "ark", Source: service.VideoTaskSourceUnified,
		Model: "seedance-2", UpstreamModel: "doubao-seedance-2-0", UpstreamTaskID: upstreamID, Status: video.StatusQueued,
		Resolution: "720p", DurationSeconds: 5, VideoCount: 1, EstimatedCost: 0.75, NextPollAt: &next,
		BillingStatus: service.VideoBillingPending, ClientIP: "127.0.0.1", UserAgent: "it",
	}
}

func TestVideoTaskRepository_Lifecycle(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	repo := &videoTaskRepository{db: tx}

	task := newVideoTaskTestTask("vt_it_lifecycle", "cgt-it-1", 9001)
	require.NoError(t, repo.Create(ctx, task))
	require.NotZero(t, task.ID)
	// 唯一冲突会让事务进入失败状态，用保存点隔离这次故意的重复插入。
	_, err := tx.ExecContext(ctx, "SAVEPOINT video_dup")
	require.NoError(t, err)
	require.ErrorIs(t, repo.Create(ctx, newVideoTaskTestTask("vt_it_dup", "cgt-it-1", 9001)), service.ErrVideoTaskExists,
		"同一账号的上游任务 ID 唯一")
	_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT video_dup")
	require.NoError(t, err)

	got, err := repo.GetByUpstream(ctx, "ark", "cgt-it-1", 9001, 31)
	require.NoError(t, err)
	require.Equal(t, task.TaskID, got.TaskID)
	_, err = repo.GetByUpstream(ctx, "ark", "cgt-it-1", 9002, 31)
	require.ErrorIs(t, err, service.ErrVideoTaskNotFound, "其他用户查不到")

	claimed, err := repo.ClaimDuePolls(ctx, time.Now(), time.Minute, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	again, err := repo.ClaimDuePolls(ctx, time.Now(), time.Minute, 10)
	require.NoError(t, err)
	require.Empty(t, again, "领取后 next_poll_at 推后，不会被重复领取")

	duration := 5.2
	finished := time.Now()
	require.NoError(t, repo.UpdateSnapshot(ctx, task.ID, service.VideoTaskSnapshotUpdate{
		Status: video.StatusSucceeded, Progress: 100, VideoURL: "https://v/x.mp4", FileID: "f-1",
		ActualDurationSeconds: &duration, CompletionTokens: 1000, FinishedAt: &finished,
	}))
	require.NoError(t, repo.UpdateSnapshot(ctx, task.ID, service.VideoTaskSnapshotUpdate{Status: video.StatusRunning, Progress: 10}))
	got, err = repo.GetByTaskID(ctx, task.TaskID)
	require.NoError(t, err)
	require.Equal(t, video.StatusSucceeded, got.Status, "终态不会被乱序到达的非终态快照覆盖")
	require.Equal(t, 100, got.Progress)
	require.Equal(t, "https://v/x.mp4", got.VideoURL)
	require.Equal(t, 5, got.BillingDurationSeconds())
	require.EqualValues(t, 1000, got.CompletionTokens)
	require.Equal(t, 2, got.PollCount)

	byFile, err := repo.GetByFileID(ctx, "ark", "f-1", 9001, 31)
	require.NoError(t, err)
	require.Equal(t, task.TaskID, byFile.TaskID)

	pending, err := repo.ListBillingPending(ctx, time.Now().Add(time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	sum, err := repo.SumPendingEstimatedCost(ctx, 9001)
	require.NoError(t, err)
	require.InDelta(t, 0.75, sum, 1e-9)

	require.NoError(t, repo.MarkBilling(ctx, task.ID, service.VideoBillingPending, nil, "transient"))
	cost := 0.78
	require.NoError(t, repo.MarkBilling(ctx, task.ID, service.VideoBillingBilled, &cost, ""))
	require.NoError(t, repo.MarkBilling(ctx, task.ID, service.VideoBillingFailed, nil, "should be ignored"))
	got, err = repo.GetByTaskID(ctx, task.TaskID)
	require.NoError(t, err)
	require.Equal(t, service.VideoBillingBilled, got.BillingStatus, "已扣费后不会再被改写")
	require.NotNil(t, got.BilledAt)
	require.InDelta(t, 0.78, *got.ActualCost, 1e-9)
	require.Equal(t, 2, got.BillingAttempts)
	sum, err = repo.SumPendingEstimatedCost(ctx, 9001)
	require.NoError(t, err)
	require.Zero(t, sum)
}

func TestVideoTaskRepository_ListFilters(t *testing.T) {
	ctx := context.Background()
	repo := &videoTaskRepository{db: testTx(t)}
	for i, id := range []string{"vt_it_list_a", "vt_it_list_b"} {
		task := newVideoTaskTestTask(id, "up-list-"+id, 9101)
		task.Model = []string{"kling-v3", "wan3.0-video"}[i]
		require.NoError(t, repo.Create(ctx, task))
	}
	tasks, total, err := repo.List(ctx, service.VideoTaskFilter{UserID: 9101, Model: "kling", Limit: 10})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "vt_it_list_a", tasks[0].TaskID)
	tasks, total, err = repo.List(ctx, service.VideoTaskFilter{Keyword: "up-list-vt_it_list_b", Limit: 10})
	require.NoError(t, err)
	require.EqualValues(t, 1, total, "关键字同时匹配任务 ID 与上游任务 ID")
	require.Equal(t, "vt_it_list_b", tasks[0].TaskID)
}
