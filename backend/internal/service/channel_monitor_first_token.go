package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// 用户视图「首字延迟」：取号池对应分组 + 主模型近期真实用户调用的平均首字耗时。
// 探测请求本身是非流式的，测不出首字时间；真实调用的 first_token_ms 由网关写入 usage_logs。
// 号池配置里没有分组外键，这里用探测 Key 反查其所属分组（Key 必须是本站签发的才能命中）。

// monitorFirstTokenCache 缓存一次全量统计结果。用户视图总是覆盖全部启用的监控，
// 因此整份快照缓存即可；TTL 内新增的监控取不到值时前端会退回展示探测延迟。
type monitorFirstTokenCache struct {
	mu        sync.Mutex
	expiresAt time.Time
	values    map[int64]int
}

func (c *monitorFirstTokenCache) get(now time.Time) (map[int64]int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil || !now.Before(c.expiresAt) {
		return nil, false
	}
	return c.values, true
}

func (c *monitorFirstTokenCache) set(values map[int64]int, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = values
	c.expiresAt = now.Add(monitorFirstTokenCacheTTL)
}

// batchFirstToken 返回 monitorID -> 平均首字延迟（毫秒）。失败仅日志并返回空 map，
// 与其它聚合一致，不阻断列表渲染；失败结果不缓存，下次请求重试。
func (s *ChannelMonitorService) batchFirstToken(ctx context.Context, monitors []*ChannelMonitor) map[int64]int {
	now := time.Now()
	if cached, ok := s.firstToken.get(now); ok {
		return cached
	}
	targets := s.firstTokenTargets(monitors)
	if len(targets) == 0 {
		s.firstToken.set(map[int64]int{}, now)
		return map[int64]int{}
	}
	values, err := s.repo.AvgFirstTokenForMonitors(ctx, targets, now.Add(-monitorFirstTokenWindow))
	if err != nil {
		slog.Warn("channel_monitor: user view batch first token failed", "error", err)
		return map[int64]int{}
	}
	s.firstToken.set(values, now)
	return values
}

// firstTokenTargets 为带探测 Key 的监控解密出明文 Key。仅配额模式（无探测 Key）
// 或解密失败的监控直接跳过；解密在副本上进行，不改动传入的监控对象。
func (s *ChannelMonitorService) firstTokenTargets(monitors []*ChannelMonitor) []MonitorFirstTokenTarget {
	if s.encryptor == nil {
		return nil
	}
	targets := make([]MonitorFirstTokenTarget, 0, len(monitors))
	for _, m := range monitors {
		if m == nil || m.APIKey == "" || m.PrimaryModel == "" || m.CheckMode == MonitorCheckModeQuota {
			continue
		}
		plain, err := s.encryptor.Decrypt(m.APIKey)
		if err != nil || plain == "" {
			continue
		}
		targets = append(targets, MonitorFirstTokenTarget{MonitorID: m.ID, APIKey: plain, Model: m.PrimaryModel})
	}
	return targets
}
