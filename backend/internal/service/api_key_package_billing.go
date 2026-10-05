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
