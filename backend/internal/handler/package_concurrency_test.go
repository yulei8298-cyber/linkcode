package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// packageConcurrencyCacheMock 在通用并发缓存替身上补充套餐槽位能力。
type packageConcurrencyCacheMock struct {
	concurrencyCacheMock
	packageAcquired   bool
	packageLimitSeen  int
	packageReleased   int32
	userSlotRequested int32
}

func (m *packageConcurrencyCacheMock) AcquirePackageUserSlot(_ context.Context, _ int64, maxConcurrency int, _ string) (bool, error) {
	m.packageLimitSeen = maxConcurrency
	return m.packageAcquired, nil
}

func (m *packageConcurrencyCacheMock) ReleasePackageUserSlot(context.Context, int64, string) error {
	atomic.AddInt32(&m.packageReleased, 1)
	return nil
}

func newPackageConcurrencyCacheMock(acquired bool) *packageConcurrencyCacheMock {
	m := &packageConcurrencyCacheMock{packageAcquired: acquired}
	m.acquireUserSlotFn = func(context.Context, int64, int, string) (bool, error) {
		atomic.AddInt32(&m.userSlotRequested, 1)
		return true, nil
	}
	return m
}

func newPackageGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	ctx := service.WithPackageBilling(req.Context(), &service.PackageBilling{GroupID: 7, Concurrency: 4})
	c.Request = req.WithContext(ctx)
	return c
}

func TestConcurrencyHelper_PackageRequestUsesPackageSlot(t *testing.T) {
	cache := newPackageConcurrencyCacheMock(true)
	helper := NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second)
	streamStarted := false

	release, err := helper.AcquireUserSlotWithWait(newPackageGinContext(), 10, 3, false, &streamStarted)
	require.NoError(t, err)
	require.Equal(t, 4, cache.packageLimitSeen, "上限取套餐并发而不是余额并发")
	require.Zero(t, atomic.LoadInt32(&cache.userSlotRequested), "套餐请求不占用余额并发槽位")

	release()
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.packageReleased))
}

func TestConcurrencyHelper_PackageSlotFullRejectsWithoutQueue(t *testing.T) {
	cache := newPackageConcurrencyCacheMock(false)
	helper := NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second)
	streamStarted := false

	start := time.Now()
	_, err := helper.AcquireUserSlotWithWait(newPackageGinContext(), 10, 3, false, &streamStarted)
	require.Less(t, time.Since(start), 500*time.Millisecond, "套餐并发满了不排队")

	var pkgErr *PackageConcurrencyError
	require.True(t, errors.As(err, &pkgErr))
	status, errType, code, message := concurrencyErrorResponse(err, "user")
	require.Equal(t, http.StatusTooManyRequests, status)
	require.Equal(t, "rate_limit_error", errType)
	require.Equal(t, packageConcurrencyLimitCode, code)
	require.Equal(t, "套餐并发已达上限（4 个），请等待进行中的请求完成后重试。如需更高并发请联系客服。", message)
}

func TestConcurrencyHelper_BalanceRequestKeepsUserSlot(t *testing.T) {
	cache := newPackageConcurrencyCacheMock(false)
	helper := NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second)

	release, acquired, err := helper.TryAcquireUserSlot(context.Background(), 10, 3)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NotNil(t, release)
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.userSlotRequested))
	require.Zero(t, cache.packageLimitSeen, "余额请求不碰套餐槽位")
}
