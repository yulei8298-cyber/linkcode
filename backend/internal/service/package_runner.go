package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	packageMaintenanceInterval = time.Minute
	packageMaintenanceLockKey  = "package:maintenance"
	packageMaintenanceLockTTL  = 2 * time.Minute
	packageHolidaySyncLockKey  = "package:holiday_sync"
	packageHolidaySyncLockTTL  = 2 * time.Minute
	// packageHolidaySyncHour 每天这个整点之后做当天的节假日同步。
	packageHolidaySyncHour = 3
)

// PackageRunner 套餐后台任务：每分钟过期到期套餐、解冻累计冻结满上限的套餐；
// 每天凌晨同步一次官方节假日。多实例部署时用领导锁保证同一时刻只有一个实例执行。
type PackageRunner struct {
	svc        *PackageService
	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
	wg     sync.WaitGroup
}

// NewPackageRunner 创建后台任务（需调用 Start 启动）。
func NewPackageRunner(svc *PackageService, lockCache LeaderLockCache, db *sql.DB) *PackageRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &PackageRunner{svc: svc, lockCache: lockCache, db: db, instanceID: uuid.NewString(), ctx: ctx, cancel: cancel}
}

// Start 启动后台循环。
func (r *PackageRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.wg.Add(1)
	go r.loop()
	slog.Info("package: runner started", "instance_id", r.instanceID)
}

// Stop 停止后台循环并等待当前任务结束。
func (r *PackageRunner) Stop() {
	if r == nil {
		return
	}
	r.once.Do(r.cancel)
	r.wg.Wait()
}

func (r *PackageRunner) loop() {
	defer r.wg.Done()
	ticker := time.NewTicker(packageMaintenanceInterval)
	defer ticker.Stop()
	r.tick()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.tick()
		}
	}
}

func (r *PackageRunner) tick() {
	r.runMaintenance()
	r.maybeSyncHolidays()
}

func (r *PackageRunner) runMaintenance() {
	release, ok := tryAcquireSingletonLeaderLock(r.ctx, r.lockCache, r.db, packageMaintenanceLockKey, r.instanceID, packageMaintenanceLockTTL)
	if !ok {
		return
	}
	defer release()
	expired, unfrozen, err := r.svc.RunMaintenance(r.ctx)
	if err != nil {
		slog.Warn("package: maintenance failed", "error", err)
		return
	}
	if expired > 0 || unfrozen > 0 {
		slog.Info("package: maintenance done", "expired", expired, "auto_unfrozen", unfrozen)
	}
}

// maybeSyncHolidays 当天还没同步过、且已过凌晨同步时刻时执行一次。
// 是否同步过以 settings 中的同步状态为准，因此多实例之间不会重复拉取。
func (r *PackageRunner) maybeSyncHolidays() {
	settings, err := r.svc.GetSettings(r.ctx)
	if err != nil || !settings.HolidaySyncEnabled {
		return
	}
	now := r.svc.now()
	state := r.svc.GetHolidaySyncState(r.ctx)
	if !packageHolidaySyncDue(state.LastSyncedAt, now) {
		return
	}
	release, ok := tryAcquireSingletonLeaderLock(r.ctx, r.lockCache, r.db, packageHolidaySyncLockKey, r.instanceID, packageHolidaySyncLockTTL)
	if !ok {
		return
	}
	defer release()
	if _, err := r.svc.SyncHolidays(r.ctx); err != nil {
		slog.Warn("package: holiday sync failed", "error", err)
	}
}

// packageHolidaySyncDue 从未同步过立即同步；否则每天过了同步时刻且当天未同步时再同步一次。
func packageHolidaySyncDue(last *time.Time, now time.Time) bool {
	if last == nil {
		return true
	}
	if now.Hour() < packageHolidaySyncHour {
		return false
	}
	ly, lm, ld := last.In(now.Location()).Date()
	ny, nm, nd := now.Date()
	return ly != ny || lm != nm || ld != nd
}
