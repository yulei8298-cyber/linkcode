package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"go.uber.org/zap"
)

const (
	videoFirstPollDelay    = 10 * time.Second
	videoMaxPollInterval   = time.Minute
	videoPollTick          = 5 * time.Second
	videoPollLease         = 2 * time.Minute
	videoPollBatch         = 20
	videoPollWorkers       = 4
	videoSettleTick        = time.Minute
	videoSettleGrace       = 30 * time.Second
	videoMaxBillingRetries = 10
	// videoTaskTimeout 后仍未终态的任务判定失败且不计费（上游任务记录通常 24 小时过期）。
	videoTaskTimeout = 6 * time.Hour
)

// videoNextPollDelay 随轮询次数线性退避，10 秒起步、最多 1 分钟一次。
func videoNextPollDelay(pollCount int) time.Duration {
	d := videoFirstPollDelay + time.Duration(pollCount)*5*time.Second
	if d > videoMaxPollInterval {
		return videoMaxPollInterval
	}
	return d
}

// Refresh 向上游查询一次任务状态并写回；终态成功时立即结算。返回最新任务与上游原始响应。
func (s *VideoGatewayService) Refresh(ctx context.Context, task *VideoTask) (*VideoTask, *VideoUpstreamResponse, error) {
	account, provider, err := s.LoadTaskAccount(ctx, task)
	if err != nil {
		return task, nil, err
	}
	resp, err := s.doVideoCall(ctx, account, provider, provider.BuildQuery(task.UpstreamTaskID, task.UpstreamMeta))
	if err != nil {
		s.deferPoll(ctx, task)
		return task, nil, err
	}
	if resp.StatusCode >= 300 {
		s.deferPoll(ctx, task)
		return task, resp, &VideoUpstreamError{StatusCode: resp.StatusCode, Body: resp.Body, Header: resp.Header}
	}
	snap, err := provider.ParseQuery(resp.Body)
	if err != nil {
		s.deferPoll(ctx, task)
		return task, resp, err
	}
	if snap.Status == video.StatusSucceeded && snap.FileID != "" && snap.VideoURL == "" {
		if resolver, ok := provider.(video.FileResolver); ok {
			if fileResp, ferr := s.doVideoCall(ctx, account, provider, resolver.BuildFileQuery(snap.FileID)); ferr == nil && fileResp.StatusCode < 300 {
				snap.VideoURL, _ = resolver.ParseFileQuery(fileResp.Body)
			}
		}
	}
	if snap.ContentPath != "" && snap.VideoURL == "" {
		// 需带鉴权下载的成品（如 OpenAI /content）保存为账号同源绝对地址，下载时补鉴权头。
		snap.VideoURL = video.JoinURL(videoAccountBaseURL(account, provider), snap.ContentPath)
	}
	if !snap.Status.IsTerminal() && time.Since(task.CreatedAt) > videoTaskTimeout {
		snap.Status, snap.ErrorCode, snap.ErrorMessage = video.StatusFailed, "timeout", "video task did not finish in time"
	}
	if err := s.applySnapshot(ctx, task, snap); err != nil {
		return task, resp, err
	}
	if fresh, gerr := s.repo.GetByTaskID(ctx, task.TaskID); gerr == nil {
		task = fresh
	}
	if task.Status == video.StatusSucceeded && task.BillingStatus == VideoBillingPending {
		if serr := s.Settle(ctx, task); serr != nil {
			logger.L().Warn("video_task.settle_failed", zap.String("task_id", task.TaskID), zap.Error(serr))
		} else if fresh, gerr := s.repo.GetByTaskID(ctx, task.TaskID); gerr == nil {
			task = fresh
		}
	}
	return task, resp, nil
}

func (s *VideoGatewayService) applySnapshot(ctx context.Context, task *VideoTask, snap video.Snapshot) error {
	update := VideoTaskSnapshotUpdate{
		Status:           snap.Status,
		Progress:         snap.Progress,
		VideoURL:         snap.VideoURL,
		FileID:           snap.FileID,
		Resolution:       snap.Resolution,
		CompletionTokens: snap.CompletionTokens,
		ErrorCode:        snap.ErrorCode,
		ErrorMessage:     snap.ErrorMessage,
	}
	if snap.DurationSeconds > 0 {
		d := snap.DurationSeconds
		update.ActualDurationSeconds = &d
	}
	now := time.Now()
	if snap.Status.IsTerminal() {
		update.FinishedAt = &now
	} else {
		next := now.Add(videoNextPollDelay(task.PollCount + 1))
		update.NextPollAt = &next
	}
	if err := s.repo.UpdateSnapshot(ctx, task.ID, update); err != nil {
		return err
	}
	if snap.Status == video.StatusFailed || snap.Status == video.StatusCancelled {
		return s.repo.MarkBilling(ctx, task.ID, VideoBillingSkipped, nil, "")
	}
	return nil
}

// deferPoll 在查询失败时推迟下一次轮询，避免对故障上游高频重试。
func (s *VideoGatewayService) deferPoll(ctx context.Context, task *VideoTask) {
	next := time.Now().Add(videoMaxPollInterval)
	_ = s.repo.UpdateSnapshot(ctx, task.ID, VideoTaskSnapshotUpdate{Status: task.Status, NextPollAt: &next})
}

// Start 启动后台轮询与补结算协程。
func (s *VideoGatewayService) Start() {
	s.wg.Add(2)
	go s.loop(videoPollTick, s.pollDue)
	go s.loop(videoSettleTick, s.settlePending)
}

// Stop 停止后台协程并等待退出。
func (s *VideoGatewayService) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

func (s *VideoGatewayService) loop(interval time.Duration, fn func(context.Context)) {
	defer s.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			fn(ctx)
			cancel()
		}
	}
}

func (s *VideoGatewayService) pollDue(ctx context.Context) {
	tasks, err := s.repo.ClaimDuePolls(ctx, time.Now(), videoPollLease, videoPollBatch)
	if err != nil {
		logger.L().Warn("video_task.claim_polls_failed", zap.Error(err))
		return
	}
	sem := make(chan struct{}, videoPollWorkers)
	var wg sync.WaitGroup
	for _, task := range tasks {
		sem <- struct{}{}
		wg.Add(1)
		go func(t *VideoTask) {
			defer func() { <-sem; wg.Done() }()
			if _, _, err := s.Refresh(ctx, t); err != nil {
				logger.L().Debug("video_task.poll_failed", zap.String("task_id", t.TaskID), zap.Error(err))
			}
		}(task)
	}
	wg.Wait()
}

// settlePending 补结算已成功但扣费失败的任务（如数据库瞬时故障）。
func (s *VideoGatewayService) settlePending(ctx context.Context) {
	tasks, err := s.repo.ListBillingPending(ctx, time.Now().Add(-videoSettleGrace), videoPollBatch)
	if err != nil {
		logger.L().Warn("video_task.list_billing_pending_failed", zap.Error(err))
		return
	}
	for _, task := range tasks {
		if task.BillingAttempts >= videoMaxBillingRetries {
			_ = s.repo.MarkBilling(ctx, task.ID, VideoBillingFailed, nil, "settlement retry limit reached: "+task.BillingError)
			continue
		}
		if err := s.Settle(ctx, task); err != nil && !errors.Is(err, context.Canceled) {
			logger.L().Warn("video_task.resettle_failed", zap.String("task_id", task.TaskID), zap.Error(err))
		}
	}
}

// RetryBilling 供管理端手动重试结算失败的任务。
func (s *VideoGatewayService) RetryBilling(ctx context.Context, taskID string) (*VideoTask, error) {
	task, err := s.repo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status != video.StatusSucceeded {
		return task, fmt.Errorf("only completed tasks can be billed")
	}
	if task.BillingStatus == VideoBillingBilled {
		return task, nil
	}
	if err := s.Settle(ctx, task); err != nil {
		return task, err
	}
	return s.repo.GetByTaskID(ctx, taskID)
}
