package service

import "database/sql"

// ProvidePackageService 创建套餐服务，并把套餐判定接入 API Key 鉴权与计费缓存：
//   - APIKeyService：鉴权中间件据此判定请求是否按套餐计费；
//   - BillingCacheService：扣费让套餐用完时失效套餐概况缓存；
//   - PaymentService：套餐订单的下单校验、发货与退款作废。
//
// 用 setter 注入而不改两者的构造参数，避免上游同步时构造函数冲突。
func ProvidePackageService(
	repo PackageRepository,
	settingRepo SettingRepository,
	groupRepo GroupRepository,
	userRepo UserRepository,
	apiKeyService *APIKeyService,
	billingCacheService *BillingCacheService,
	paymentService *PaymentService,
) *PackageService {
	svc := NewPackageService(repo, settingRepo, groupRepo, userRepo)
	apiKeyService.SetPackageService(svc)
	paymentService.SetPackageService(svc)
	billingCacheService.SetPackageStateInvalidator(svc.InvalidateGroupState)
	return svc
}

// ProvidePackageRunner 创建并启动套餐后台任务（过期、自动解冻、节假日同步），Stop 由 cleanup 调用。
func ProvidePackageRunner(svc *PackageService, lockCache LeaderLockCache, db *sql.DB) *PackageRunner {
	r := NewPackageRunner(svc, lockCache, db)
	r.Start()
	return r
}
