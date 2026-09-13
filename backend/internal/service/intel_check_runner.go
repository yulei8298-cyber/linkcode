package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const (
	// intelCheckLeaderLockKey 多实例部署时的单飞锁键。
	intelCheckLeaderLockKey = "intel_check:round"

	// intelCheckRoundLockBuffer 锁 TTL 相对轮次硬上限的富余量。
	// TTL 只是崩溃兜底：正常路径跑完即释放，所以只需保证它不会在轮次执行中途过期。
	intelCheckRoundLockBuffer = 2 * time.Minute

	// intelCheckSettingsTimeout 调度器读取设置的超时。
	intelCheckSettingsTimeout = 5 * time.Second
)

// ErrIntelCheckRoundInFlight 已有一轮检测在执行中。
//
// 409 而非 500：这不是故障，管理员稍后再点一次即可；
// 自动调度撞上时直接跳过本周期，不排队——排队只会在上游恢复后堆出一串补跑。
var ErrIntelCheckRoundInFlight = infraerrors.Conflict(
	"INTEL_CHECK_ROUND_IN_FLIGHT", "已有一轮检测正在执行")

// intelCheckRunnerSvc 抽出调度器真正依赖的两个方法。
// 用窄接口而非 *IntelCheckService，是为了让单元测试能注入轻量 stub，
// 不必搭起完整的 repo + 加密器链路。
type intelCheckRunnerSvc interface {
	GetSettings(ctx context.Context) (*IntelCheckSettings, error)
	RunOnce(ctx context.Context, trigger string) (*IntelCheckRound, error)
}

// IntelCheckRunner 智力检测调度器。
//
// 与 ChannelMonitorRunner 的「每个监控一个 ticker」不同，这里全局只有一个周期：
// 所有分组必须在同一轮里用同一道题作答，否则时间线上的同一列就不再可比，
// 而「同题同刻横向对比」正是这张公开页的全部说服力来源。
//
// 设置变更由 IntelCheckService.UpdateSettings 通过 Reload 通知。Reload 走
// channel 信号而不是「停旧 goroutine 起新 goroutine」：后者要处理旧 goroutine
// 尚未退出时的重入，而周期变更本质上只需要 ticker.Reset 一次。
type IntelCheckRunner struct {
	svc        intelCheckRunnerSvc
	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	parentCtx    context.Context
	parentCancel context.CancelFunc

	// mu 保护 started/stopped。用布尔量而非 sync.Once：Once 一旦触发就无法
	// 再次执行，而 Stop 之后的 Start 必须保持为空操作（同 OpsCleanupService 的取舍）。
	mu       sync.Mutex
	started  bool
	stopped  bool
	wg       sync.WaitGroup
	reloadCh chan struct{}

	// inFlight 保证同一进程内至多一轮在跑。跨实例那层由 leader lock 负责。
	inFlightMu sync.Mutex
	inFlight   bool
}

// NewIntelCheckRunner 构造调度器。
//
// lockCache 与 db 都可为 nil：两者皆缺时 tryAcquireSingletonLeaderLock 会
// 直接放行（单实例部署或单元测试），不会让任务被静默饿死。
func NewIntelCheckRunner(svc *IntelCheckService, lockCache LeaderLockCache, db *sql.DB) *IntelCheckRunner {
	return newIntelCheckRunner(svc, lockCache, db)
}

// newIntelCheckRunner 内部构造，接受窄接口以便测试注入 stub。
func newIntelCheckRunner(svc intelCheckRunnerSvc, lockCache LeaderLockCache, db *sql.DB) *IntelCheckRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &IntelCheckRunner{
		svc:          svc,
		lockCache:    lockCache,
		db:           db,
		instanceID:   uuid.NewString(),
		parentCtx:    ctx,
		parentCancel: cancel,
		// 容量 1：多次 Reload 合并成一次即可，周期是幂等的。
		reloadCh: make(chan struct{}, 1),
	}
}

// Start 启动周期调度。重复调用与 Stop 之后调用均为空操作。
func (r *IntelCheckRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.mu.Lock()
	if r.started || r.stopped {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.wg.Add(1)
	r.mu.Unlock()

	go r.loop()
	slog.Info("intel_check: runner started", "instance_id", r.instanceID)
}

// Stop 优雅停止：取消上下文并等待在飞的轮次收尾。
// 已经拿到结果的检测仍会写回——落库用的是 WithoutCancel 派生的上下文。
func (r *IntelCheckRunner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.stopped = true
	r.parentCancel()
	r.mu.Unlock()

	r.wg.Wait()
}

// Reload 让设置变更即时生效（实现 intelCheckReloader）。
//
// 无参数无返回值是接口约定：它由 UpdateSettings 在持久化之后调用，
// 那条路径上没有可供调度器回报失败的地方，因此这里只发信号、不阻塞、不报错。
func (r *IntelCheckRunner) Reload() {
	if r == nil {
		return
	}
	r.mu.Lock()
	running := r.started && !r.stopped
	r.mu.Unlock()
	if !running {
		return
	}

	select {
	case r.reloadCh <- struct{}{}:
	default:
		// 已有待处理信号，合并即可。
	}
}

// RunNow 手动触发一轮，立即返回。
//
// 不同步等待：一轮可能跑上几分钟，让管理端的 HTTP 请求挂着等结果，
// 反向代理超时后页面只会得到一个与实际执行无关的 504。
func (r *IntelCheckRunner) RunNow() error {
	if r == nil || r.svc == nil {
		return fmt.Errorf("智力检测调度器未初始化")
	}
	r.mu.Lock()
	running := r.started && !r.stopped
	r.mu.Unlock()
	if !running {
		return fmt.Errorf("智力检测调度器未在运行")
	}

	if !r.tryAcquireInFlight() {
		return ErrIntelCheckRoundInFlight
	}
	r.launchRound(IntelCheckTriggerManual)
	return nil
}

// loop 周期主循环。
func (r *IntelCheckRunner) loop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.currentInterval())
	defer ticker.Stop()

	for {
		select {
		case <-r.parentCtx.Done():
			return
		case <-r.reloadCh:
			// Go 1.23 起 Reset 会丢弃尚未被接收的旧 tick，
			// 因此改完周期不会立刻多触发一次。
			ticker.Reset(r.currentInterval())
		case <-ticker.C:
			r.fire()
			// 每轮重读周期：设置也可能由另一个实例改动，那种改动传不到本进程的
			// Reload，靠这里最终收敛。
			ticker.Reset(r.currentInterval())
		}
	}
}

// currentInterval 读取当前检测周期。
func (r *IntelCheckRunner) currentInterval() time.Duration {
	return time.Duration(r.loadSettings().IntervalMinutes) * time.Minute
}

// loadSettings 读取设置，失败时回落默认值。
//
// 回落而非停摆：周期只决定「多久看一眼」，真要不要跑由 fire 再判一次，
// 让一次数据库抖动永久停掉调度是更糟的失败方式。
func (r *IntelCheckRunner) loadSettings() *IntelCheckSettings {
	ctx, cancel := context.WithTimeout(r.parentCtx, intelCheckSettingsTimeout)
	defer cancel()

	cfg, err := r.svc.GetSettings(ctx)
	if err != nil || cfg == nil {
		fallback := DefaultIntelCheckSettings()
		slog.Warn("intel_check: 读取设置失败，本次按默认周期处理", "error", err)
		return &fallback
	}
	return cfg
}

// fire 触发一轮。功能关闭、上一轮未完成时静默跳过。
func (r *IntelCheckRunner) fire() {
	if !r.loadSettings().Enabled {
		return
	}
	if !r.tryAcquireInFlight() {
		slog.Warn("intel_check: 上一轮仍在执行，跳过本次触发")
		return
	}
	r.launchRound(IntelCheckTriggerCron)
}

// launchRound 异步执行一轮。调用方必须已持有 in-flight 标记。
func (r *IntelCheckRunner) launchRound(trigger string) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer r.releaseInFlight()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("intel_check: 轮次执行 panic", "trigger", trigger, "panic", rec)
			}
		}()
		r.runRound(trigger)
	}()
}

// runRound 抢锁并执行一轮。
func (r *IntelCheckRunner) runRound(trigger string) {
	deadline := intelCheckRoundDeadline(r.loadSettings())

	// 给整轮设硬上限：跑过头的轮次会与下一轮抢同一批分组的配额，
	// 与其让两轮互相拖慢，不如截断——未完成的分组会落成 request_error（黄色），
	// 这本身就是需要被看见的信号。
	ctx, cancel := context.WithTimeout(r.parentCtx, deadline)
	defer cancel()

	release, ok := tryAcquireSingletonLeaderLock(
		ctx, r.lockCache, r.db, intelCheckLeaderLockKey, r.instanceID, deadline+intelCheckRoundLockBuffer)
	if !ok {
		slog.Info("intel_check: 另一实例正在执行本轮，跳过", "trigger", trigger)
		return
	}
	defer release()

	round, err := r.svc.RunOnce(ctx, trigger)
	if err != nil {
		slog.Warn("intel_check: 本轮检测未能完成", "trigger", trigger, "error", err)
		return
	}
	slog.Info("intel_check: 本轮检测完成",
		"trigger", trigger, "round_id", round.ID, "round_seq", round.Seq)
}

// intelCheckRoundDeadline 一轮的硬上限。
//
// 取一个检测周期：一轮跑满一个周期还没完，说明配置（并发、超时、分组数）
// 已经失衡，继续跑只会与下一轮叠在一起。下限保底为「单次请求超时 + 1 分钟」，
// 否则周期小于单次超时的配置会让每一轮都必然被截断。
func intelCheckRoundDeadline(cfg *IntelCheckSettings) time.Duration {
	deadline := time.Duration(cfg.IntervalMinutes) * time.Minute
	if floor := time.Duration(cfg.RequestTimeoutSeconds)*time.Second + time.Minute; deadline < floor {
		deadline = floor
	}
	return deadline
}

// tryAcquireInFlight 原子地占用「本进程唯一一轮」的名额。
func (r *IntelCheckRunner) tryAcquireInFlight() bool {
	r.inFlightMu.Lock()
	defer r.inFlightMu.Unlock()
	if r.inFlight {
		return false
	}
	r.inFlight = true
	return true
}

// releaseInFlight 释放名额。轮次结束（含 panic recover）后必须调用。
func (r *IntelCheckRunner) releaseInFlight() {
	r.inFlightMu.Lock()
	r.inFlight = false
	r.inFlightMu.Unlock()
}
