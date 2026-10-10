package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	gocache "github.com/patrickmn/go-cache"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// 企业倍率：企业尊享用户按量（扣余额）使用配置了企业倍率的分组时，
// 实付倍率取「分组 / 个人专属倍率」与企业倍率中更低的；套餐请求仍按原倍率。
//
// 判定在 API Key 鉴权中间件里做（那里已经知道本次请求走不走套餐），结果写入上下文，
// 计费、在途预留、利润保护都通过同一个倍率解析函数读取，口径一致。

// enterpriseCacheTTL 企业配置与用户企业身份的缓存时长。本实例改配置、手动开通会主动失效；
// 用户刚充值达标或其他实例改了配置，最多延迟一个 TTL 生效。
const enterpriseCacheTTL = time.Minute

const enterpriseSettingsCacheKey = "settings"

// EnterpriseRate 本次请求可用的企业倍率。
type EnterpriseRate struct {
	GroupID    int64
	Multiplier float64
}

// WithEnterpriseRate 把企业倍率写入上下文。
func WithEnterpriseRate(ctx context.Context, rate *EnterpriseRate) context.Context {
	if rate == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.EnterpriseRate, rate)
}

// EnterpriseRateFromContext 读取企业倍率；没有时返回 nil。
func EnterpriseRateFromContext(ctx context.Context) *EnterpriseRate {
	if ctx == nil {
		return nil
	}
	rate, _ := ctx.Value(ctxkey.EnterpriseRate).(*EnterpriseRate)
	return rate
}

// ApplyEnterpriseRate 本次请求带企业倍率且分组一致时，返回它与 resolved 中更低的；否则原样返回。
func ApplyEnterpriseRate(ctx context.Context, groupID int64, resolved float64) float64 {
	rate := EnterpriseRateFromContext(ctx)
	if rate == nil || rate.GroupID != groupID || rate.Multiplier >= resolved {
		return resolved
	}
	return rate.Multiplier
}

func (s *PackageService) enterpriseCacheStore() *gocache.Cache {
	s.enterpriseCacheOnce.Do(func() {
		s.enterpriseCache = gocache.New(enterpriseCacheTTL, 2*enterpriseCacheTTL)
	})
	return s.enterpriseCache
}

// invalidateEnterprise 清掉企业配置缓存；userID > 0 时同时清掉该用户的身份缓存。
func (s *PackageService) invalidateEnterprise(userID int64) {
	cache := s.enterpriseCacheStore()
	cache.Delete(enterpriseSettingsCacheKey)
	if userID > 0 {
		cache.Delete(enterpriseUserCacheKey(userID))
	}
}

func enterpriseUserCacheKey(userID int64) string {
	return fmt.Sprintf("user:%d", userID)
}

func (s *PackageService) cachedEnterpriseSettings(ctx context.Context) (EnterpriseSettings, error) {
	cache := s.enterpriseCacheStore()
	if v, ok := cache.Get(enterpriseSettingsCacheKey); ok {
		if settings, ok := v.(EnterpriseSettings); ok {
			return settings, nil
		}
	}
	settings, err := s.GetEnterpriseSettings(ctx)
	if err != nil {
		return settings, err
	}
	cache.SetDefault(enterpriseSettingsCacheKey, settings)
	return settings, nil
}

func (s *PackageService) cachedIsEnterprise(ctx context.Context, userID int64) (bool, error) {
	cache := s.enterpriseCacheStore()
	key := enterpriseUserCacheKey(userID)
	if v, ok := cache.Get(key); ok {
		if enterprise, ok := v.(bool); ok {
			return enterprise, nil
		}
	}
	status, err := s.EnterpriseStatus(ctx, userID)
	if err != nil {
		return false, err
	}
	cache.SetDefault(key, status.Enterprise)
	return status.Enterprise, nil
}

// EnterpriseGroupRates 企业尊享用户可享受的各分组企业倍率；不是企业用户或没有配置时返回 nil。
// 查询失败按「没有企业倍率」处理并记录日志：企业倍率只是优惠，失败时按原价计费不会少收。
func (s *PackageService) EnterpriseGroupRates(ctx context.Context, userID int64) map[int64]float64 {
	if s == nil || userID <= 0 {
		return nil
	}
	settings, err := s.cachedEnterpriseSettings(ctx)
	if err != nil {
		slog.Warn("enterprise_settings_load_failed", "error", err)
		return nil
	}
	if !settings.Enabled || len(settings.GroupRates) == 0 {
		return nil
	}
	enterprise, err := s.cachedIsEnterprise(ctx, userID)
	if err != nil {
		slog.Warn("enterprise_status_load_failed", "error", err, "user_id", userID)
		return nil
	}
	if !enterprise {
		return nil
	}
	rates := make(map[int64]float64, len(settings.GroupRates))
	for _, r := range settings.GroupRates {
		rates[r.GroupID] = r.Multiplier
	}
	return rates
}

// ResolveEnterpriseRate 本次按量请求可用的企业倍率；没有时返回 nil。
func (s *PackageService) ResolveEnterpriseRate(ctx context.Context, userID, groupID int64) *EnterpriseRate {
	if groupID <= 0 {
		return nil
	}
	settings, err := s.cachedEnterpriseSettings(ctx)
	if err != nil || !settings.Enabled {
		return nil
	}
	// 先看分组有没有配置，大多数请求在这里就返回，不必查用户身份。
	if _, ok := settings.GroupRate(groupID); !ok {
		return nil
	}
	rate, ok := s.EnterpriseGroupRates(ctx, userID)[groupID]
	if !ok {
		return nil
	}
	return &EnterpriseRate{GroupID: groupID, Multiplier: rate}
}

// MergeEnterpriseGroupRates 把企业倍率并入用户专属倍率，供前端展示「我的倍率」：
// 某分组的企业倍率低于该用户在此分组的现行倍率（个人专属倍率，没有则分组默认倍率）时，
// 用企业倍率，并记下这些分组。与按量计费的取值口径一致；套餐请求仍按原倍率，不在展示范围内。
//
// groupRates 是用户可见分组的默认倍率；企业倍率里不在其中的分组没有对照基准，不处理。
// 不修改传入的 map，没有企业倍率时原样返回 userRates。
func MergeEnterpriseGroupRates(userRates, enterpriseRates, groupRates map[int64]float64) (map[int64]float64, map[int64]bool) {
	if len(enterpriseRates) == 0 {
		return userRates, nil
	}
	merged := make(map[int64]float64, len(userRates)+len(enterpriseRates))
	for id, rate := range userRates {
		merged[id] = rate
	}
	marked := make(map[int64]bool, len(enterpriseRates))
	for groupID, enterprise := range enterpriseRates {
		current, hasPersonal := merged[groupID]
		if !hasPersonal {
			base, known := groupRates[groupID]
			if !known {
				continue
			}
			current = base
		}
		if enterprise < current {
			merged[groupID] = enterprise
			marked[groupID] = true
		}
	}
	return merged, marked
}
