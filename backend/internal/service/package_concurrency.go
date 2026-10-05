package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// PackageConcurrencyCache 套餐并发槽位（可选能力，未实现时不限制套餐并发）。
type PackageConcurrencyCache interface {
	AcquirePackageUserSlot(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error)
	ReleasePackageUserSlot(ctx context.Context, userID int64, requestID string) error
}

// AcquirePackageUserSlot 立即尝试占用套餐并发槽位，不排队：套餐并发满了直接拒绝。
func (s *ConcurrencyService) AcquirePackageUserSlot(ctx context.Context, userID int64, maxConcurrency int) (*AcquireResult, error) {
	cache, ok := s.cache.(PackageConcurrencyCache)
	if !ok || cache == nil || maxConcurrency <= 0 {
		return &AcquireResult{Acquired: true, ReleaseFunc: func() {}}, nil
	}
	requestID := generateRequestID()
	acquired, err := cache.AcquirePackageUserSlot(ctx, userID, maxConcurrency, requestID)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return &AcquireResult{Acquired: false}, nil
	}
	return &AcquireResult{
		Acquired: true,
		ReleaseFunc: func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := cache.ReleasePackageUserSlot(bgCtx, userID, requestID); err != nil {
				logger.LegacyPrintf("service.concurrency", "Warning: failed to release package slot for %d (req=%s): %v", userID, requestID, err)
			}
		},
	}, nil
}
