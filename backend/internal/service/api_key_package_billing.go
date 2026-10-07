package service

import "context"

// SetPackageService 注入套餐服务，供 API Key 鉴权中间件判定套餐计费。
func (s *APIKeyService) SetPackageService(packageService *PackageService) {
	s.packageService = packageService
}

// ResolvePackageBilling 判定本次请求是否按套餐计费，语义见 PackageService.ResolvePackageBilling。
// 未注入套餐服务时恒走余额逻辑。
func (s *APIKeyService) ResolvePackageBilling(ctx context.Context, apiKey *APIKey, lowBalance bool) (*PackageBilling, error) {
	if s == nil || s.packageService == nil || apiKey == nil {
		return nil, nil
	}
	return s.packageService.ResolvePackageBilling(ctx, apiKey.User, apiKey.Group, lowBalance)
}

// ResolveEnterpriseRate 本次请求不走套餐时，判定能否享受企业倍率；不能时返回 nil。
func (s *APIKeyService) ResolveEnterpriseRate(ctx context.Context, apiKey *APIKey) *EnterpriseRate {
	if s == nil || s.packageService == nil || apiKey == nil || apiKey.GroupID == nil {
		return nil
	}
	return s.packageService.ResolveEnterpriseRate(ctx, apiKey.UserID, *apiKey.GroupID)
}

// EnterpriseGroupRates 用户可享受的各分组企业倍率，供模型广场展示专属价格；不是企业用户时返回 nil。
func (s *APIKeyService) EnterpriseGroupRates(ctx context.Context, userID int64) map[int64]float64 {
	if s == nil || s.packageService == nil {
		return nil
	}
	return s.packageService.EnterpriseGroupRates(ctx, userID)
}
