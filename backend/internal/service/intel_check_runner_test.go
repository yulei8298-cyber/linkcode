//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// intelCheckRunnerSvcStub 只桩接调度器真正依赖的两个方法。
//
// 这里刻意不复用 IntelCheckService：本组用例要验的是「何时该跑、何时该躲」，
// 接上真实 service 就得连带准备题库、分组、加解密，一旦那边出问题，
// 失败会指向调度逻辑，排查方向整个跑偏。
type intelCheckRunnerSvcStub struct {
	mu       sync.Mutex
	settings IntelCheckSettings
	runCalls int
	triggers []string

	// block 非 nil 时 RunOnce 会阻塞在此，用于稳定构造「一轮仍在飞」的状态。
	block chan struct{}
}

func newIntelCheckRunnerSvcStub(enabled bool) *intelCheckRunnerSvcStub {
	cfg := DefaultIntelCheckSettings()
	cfg.Enabled = enabled
	return &intelCheckRunnerSvcStub{settings: cfg}
}

func (s *intelCheckRunnerSvcStub) GetSettings(context.Context) (*IntelCheckSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.settings
	return &cfg, nil
}

func (s *intelCheckRunnerSvcStub) RunOnce(_ context.Context, trigger string) (*IntelCheckRound, error) {
	s.mu.Lock()
	s.runCalls++
	s.triggers = append(s.triggers, trigger)
	block := s.block
	s.mu.Unlock()

	if block != nil {
		<-block
	}
	return &IntelCheckRound{ID: 1, Seq: 1}, nil
}

func (s *intelCheckRunnerSvcStub) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runCalls
}

func (s *intelCheckRunnerSvcStub) triggerList() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.triggers...)
}

func TestIntelCheckRunner_未抢到锁时整轮跳过(t *testing.T) {
	svc := newIntelCheckRunnerSvcStub(true)
	cache := &fakeLeaderLockCache{}

	held, err := cache.TryAcquireLeaderLock(context.Background(), intelCheckLeaderLockKey, "peer-instance", time.Minute)
	require.NoError(t, err)
	require.True(t, held)

	runner := newIntelCheckRunner(svc, cache, nil)
	runner.runRound(IntelCheckTriggerCron)

	// 多实例部署下每个实例都有自己的 ticker，不设单飞锁就会同一时刻各跑一轮，
	// 公开页会出现两条时间贴得极近的轮次，上游也被白白多打一遍。
	require.Zero(t, svc.calls(), "同伴持锁时必须整轮跳过")
	require.Equal(t, "peer-instance", cache.heldBy(intelCheckLeaderLockKey), "不得夺走同伴的锁")
}

func TestIntelCheckRunner_抢到锁执行完即释放(t *testing.T) {
	svc := newIntelCheckRunnerSvcStub(true)
	cache := &fakeLeaderLockCache{}

	runner := newIntelCheckRunner(svc, cache, nil)
	runner.runRound(IntelCheckTriggerCron)

	require.Equal(t, 1, svc.calls())
	// 锁按轮释放而非续期：leadership 每轮重新竞争，不会被某个实例长期霸占。
	require.Empty(t, cache.heldBy(intelCheckLeaderLockKey))
}

func TestIntelCheckRunner_无锁后端时不被静默饿死(t *testing.T) {
	svc := newIntelCheckRunnerSvcStub(true)

	// 单实例部署既没 Redis 也没 db 句柄，此时必须直接放行；
	// 若把「拿不到锁后端」当成「没抢到锁」，检测会一轮都跑不起来。
	runner := newIntelCheckRunner(svc, nil, nil)
	runner.runRound(IntelCheckTriggerManual)

	require.Equal(t, 1, svc.calls())
	require.Equal(t, []string{IntelCheckTriggerManual}, svc.triggerList())
}

func TestIntelCheckRunner_开关关闭时到点不触发(t *testing.T) {
	svc := newIntelCheckRunnerSvcStub(false)
	runner := newIntelCheckRunner(svc, nil, nil)

	runner.fire()
	runner.wg.Wait()

	// 关闭开关是运营动作，ticker 仍在走，靠 fire 这一层拦住。
	require.Zero(t, svc.calls())
}

func TestIntelCheckRunner_开关打开时到点触发(t *testing.T) {
	svc := newIntelCheckRunnerSvcStub(true)
	runner := newIntelCheckRunner(svc, nil, nil)

	runner.fire()
	runner.wg.Wait()

	require.Equal(t, 1, svc.calls())
	require.Equal(t, []string{IntelCheckTriggerCron}, svc.triggerList())
}

func TestIntelCheckRunner_一轮在飞时手动触发被拒(t *testing.T) {
	svc := newIntelCheckRunnerSvcStub(true)
	svc.block = make(chan struct{})

	runner := newIntelCheckRunner(svc, nil, nil)
	runner.Start()
	t.Cleanup(runner.Stop)

	require.NoError(t, runner.RunNow())
	// 不排队、直接拒：上游长时间不可用时，排队重跑只会在恢复后堆出一串补跑，
	// 把公开页刷成一片密集的同刻轮次。
	require.ErrorIs(t, runner.RunNow(), ErrIntelCheckRoundInFlight)

	close(svc.block)
}

func TestIntelCheckRunner_停止后不再接受触发(t *testing.T) {
	svc := newIntelCheckRunnerSvcStub(true)
	runner := newIntelCheckRunner(svc, nil, nil)
	runner.Start()
	runner.Stop()

	require.Error(t, runner.RunNow())

	// Stop 之后 Start 必须是空操作，否则重复启停会留下悬空的 loop goroutine。
	runner.Start()
	require.Error(t, runner.RunNow())
	require.Zero(t, svc.calls())
}

func TestIntelCheckRunner_重复启停不panic(t *testing.T) {
	runner := newIntelCheckRunner(newIntelCheckRunnerSvcStub(true), nil, nil)

	require.NotPanics(t, func() {
		runner.Start()
		runner.Start()
		runner.Stop()
		runner.Stop()
	})
}

func TestIntelCheckRunner_未启动时Reload不阻塞(t *testing.T) {
	runner := newIntelCheckRunner(newIntelCheckRunnerSvcStub(true), nil, nil)

	// Reload 由 UpdateSettings 同步调用。调度器没跑起来时无人消费 reloadCh，
	// 这里若不走 default 分支，管理端保存设置就会直接卡死。
	require.NotPanics(t, func() {
		for i := 0; i < 5; i++ {
			runner.Reload()
		}
	})
	require.Empty(t, runner.reloadCh)
}

func TestIntelCheckRunner_连续Reload合并为一次(t *testing.T) {
	runner := newIntelCheckRunner(newIntelCheckRunnerSvcStub(true), nil, nil)
	runner.mu.Lock()
	runner.started = true
	runner.mu.Unlock()

	for i := 0; i < 5; i++ {
		runner.Reload()
	}

	// 管理端连点保存会连发多次 Reload，信号必须合并，
	// 否则每一次都触发一轮 ticker 重建，周期被反复清零。
	require.Len(t, runner.reloadCh, 1)
}

func TestIntelCheckRoundDeadline_不短于单次请求超时(t *testing.T) {
	cases := []struct {
		name     string
		interval int
		timeout  int
		want     time.Duration
	}{
		{"周期远大于单次超时时取周期", 30, 120, 30 * time.Minute},
		{"周期小于单次超时时抬到超时加一分钟", 1, 600, 11 * time.Minute},
		{"两者相等时仍抬高，给收尾留余量", 5, 300, 6 * time.Minute},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &IntelCheckSettings{
				IntervalMinutes:       tc.interval,
				RequestTimeoutSeconds: tc.timeout,
			}
			// 硬截止短于单次请求超时，会在最慢的那个分组刚要出结果时掐断，
			// 该格永远停在「检测中」，看起来像分组挂了。
			require.Equal(t, tc.want, intelCheckRoundDeadline(cfg))
		})
	}
}
