package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/packageplan"
	"github.com/Wei-Shaw/sub2api/ent/userpackage"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// packageRepository 实现 service.PackageRepository。
//
// 配置与单行状态变更走 ent（复用事务上下文、FOR UPDATE 行锁）；
// 「批量过期并返回受影响行」「冻结超限扫描」「按日期读写可冻结日」走原生 SQL：
// 前者需要 UPDATE ... RETURNING，后者要以 DATE 文本收发，避免 time.Time 跨时区被截到前一天。
// 扣费时的套餐扣减在计费事务里完成，见 usage_billing_package.go。
type packageRepository struct {
	client *dbent.Client
	db     *sql.DB
}

// NewPackageRepository 创建套餐仓储。
func NewPackageRepository(client *dbent.Client, db *sql.DB) service.PackageRepository {
	return &packageRepository{client: client, db: db}
}

// ---------- 套餐配置 ----------

func (r *packageRepository) ListPlans(ctx context.Context, groupID *int64, onlyForSale bool) ([]service.PackagePlan, error) {
	q := clientFromContext(ctx, r.client).PackagePlan.Query()
	if groupID != nil {
		q = q.Where(packageplan.GroupIDEQ(*groupID))
	}
	if onlyForSale {
		q = q.Where(packageplan.ForSaleEQ(true))
	}
	rows, err := q.Order(dbent.Asc(packageplan.FieldGroupID), dbent.Desc(packageplan.FieldCycle), dbent.Asc(packageplan.FieldTier)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.PackagePlan, 0, len(rows))
	for _, row := range rows {
		out = append(out, *packagePlanToService(row))
	}
	return out, nil
}

func (r *packageRepository) GetPlan(ctx context.Context, id int64) (*service.PackagePlan, error) {
	row, err := clientFromContext(ctx, r.client).PackagePlan.Get(ctx, id)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPackagePlanNotFound, nil)
	}
	return packagePlanToService(row), nil
}

func (r *packageRepository) GetPlanByCombo(ctx context.Context, groupID int64, cycle string, tier int) (*service.PackagePlan, error) {
	row, err := clientFromContext(ctx, r.client).PackagePlan.Query().
		Where(packageplan.GroupIDEQ(groupID), packageplan.CycleEQ(cycle), packageplan.TierEQ(int8(tier))).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return packagePlanToService(row), nil
}

// SavePlan 按 ID 更新；ID 为 0 时新建。唯一索引保证同分组同组合只有一个套餐。
func (r *packageRepository) SavePlan(ctx context.Context, plan *service.PackagePlan) error {
	client := clientFromContext(ctx, r.client)
	if plan.ID == 0 {
		row, err := client.PackagePlan.Create().
			SetGroupID(plan.GroupID).
			SetName(plan.Name).
			SetCycle(plan.Cycle).
			SetTier(int8(plan.Tier)).
			SetPrice(plan.Price).
			SetQuotaUsd(plan.QuotaUSD).
			SetValidityDays(plan.ValidityDays).
			SetForSale(plan.ForSale).
			Save(ctx)
		if err != nil {
			return err
		}
		*plan = *packagePlanToService(row)
		return nil
	}
	row, err := client.PackagePlan.UpdateOneID(plan.ID).
		SetName(plan.Name).
		SetPrice(plan.Price).
		SetQuotaUsd(plan.QuotaUSD).
		SetValidityDays(plan.ValidityDays).
		SetForSale(plan.ForSale).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrPackagePlanNotFound, nil)
	}
	*plan = *packagePlanToService(row)
	return nil
}

func (r *packageRepository) SetPlanQuota(ctx context.Context, id int64, quotaUSD float64) error {
	_, err := clientFromContext(ctx, r.client).PackagePlan.UpdateOneID(id).SetQuotaUsd(quotaUSD).Save(ctx)
	return translatePersistenceError(err, service.ErrPackagePlanNotFound, nil)
}

func (r *packageRepository) DeletePlan(ctx context.Context, id int64) error {
	err := clientFromContext(ctx, r.client).PackagePlan.DeleteOneID(id).Exec(ctx)
	return translatePersistenceError(err, service.ErrPackagePlanNotFound, nil)
}

// ---------- 用户套餐 ----------

// CreateUserPackage 新建套餐；带 order_id 时按订单幂等，重复发货返回已存在的那一张。
func (r *packageRepository) CreateUserPackage(ctx context.Context, pkg *service.UserPackage) (*service.UserPackage, error) {
	if pkg.OrderID != nil {
		existing, err := r.GetUserPackageByOrderID(ctx, *pkg.OrderID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}
	row, err := clientFromContext(ctx, r.client).UserPackage.Create().
		SetUserID(pkg.UserID).
		SetGroupID(pkg.GroupID).
		SetPlanID(pkg.PlanID).
		SetNillableOrderID(pkg.OrderID).
		SetName(pkg.Name).
		SetCycle(pkg.Cycle).
		SetTier(int8(pkg.Tier)).
		SetQuotaUsd(pkg.QuotaUSD).
		SetStartsAt(pkg.StartsAt).
		SetExpiresAt(pkg.ExpiresAt).
		SetStatus(service.PackageStatusActive).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return userPackageToService(row), nil
}

func (r *packageRepository) GetUserPackage(ctx context.Context, id int64) (*service.UserPackage, error) {
	row, err := clientFromContext(ctx, r.client).UserPackage.Get(ctx, id)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPackageNotFound, nil)
	}
	return userPackageToService(row), nil
}

func (r *packageRepository) GetUserPackageByOrderID(ctx context.Context, orderID int64) (*service.UserPackage, error) {
	row, err := clientFromContext(ctx, r.client).UserPackage.Query().Where(userpackage.OrderIDEQ(orderID)).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return userPackageToService(row), nil
}

// ListPackagesByOrderIDs 批量按订单查套餐。
func (r *packageRepository) ListPackagesByOrderIDs(ctx context.Context, orderIDs []int64) ([]service.UserPackage, error) {
	if len(orderIDs) == 0 {
		return []service.UserPackage{}, nil
	}
	rows, err := clientFromContext(ctx, r.client).UserPackage.Query().
		Where(userpackage.OrderIDIn(orderIDs...)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.UserPackage, 0, len(rows))
	for _, row := range rows {
		out = append(out, *userPackageToService(row))
	}
	return out, nil
}

// ListUserPackages 返回用户的全部套餐，已结束的历史记录也一并返回。
func (r *packageRepository) ListUserPackages(ctx context.Context, userID int64) ([]service.UserPackage, error) {
	rows, err := clientFromContext(ctx, r.client).UserPackage.Query().
		Where(userpackage.UserIDEQ(userID)).
		Order(dbent.Asc(userpackage.FieldExpiresAt), dbent.Asc(userpackage.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.UserPackage, 0, len(rows))
	for _, row := range rows {
		out = append(out, *userPackageToService(row))
	}
	return out, nil
}

func (r *packageRepository) GetGroupState(ctx context.Context, userID, groupID int64, now time.Time) (*service.PackageGroupState, error) {
	const q = `
SELECT
    COUNT(*) FILTER (WHERE status = 'active' AND expires_at > $3 AND used_usd < quota_usd),
    COUNT(*) FILTER (WHERE status = 'frozen')
FROM user_packages
WHERE user_id = $1 AND group_id = $2 AND status IN ('active', 'frozen')`
	state := &service.PackageGroupState{}
	if err := r.db.QueryRowContext(ctx, q, userID, groupID, now).Scan(&state.Usable, &state.Frozen); err != nil {
		return nil, fmt.Errorf("query package group state: %w", err)
	}
	return state, nil
}

// VoidPackage 作废套餐。保留 frozen_at：退款网关失败时 RestorePackageStatus 可原样恢复冻结状态；
// 作废后 status 不再是 frozen，不会被自动解冻任务扫到。
func (r *packageRepository) VoidPackage(ctx context.Context, packageID int64) (*service.UserPackage, error) {
	row, err := clientFromContext(ctx, r.client).UserPackage.UpdateOneID(packageID).
		SetStatus(service.PackageStatusVoided).
		Save(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrPackageNotFound, nil)
	}
	return userPackageToService(row), nil
}

// RestorePackageStatus 把已作废的套餐恢复为作废前的状态（仅用于退款回滚）。
func (r *packageRepository) RestorePackageStatus(ctx context.Context, packageID int64, status string) (*service.UserPackage, error) {
	n, err := clientFromContext(ctx, r.client).UserPackage.Update().
		Where(userpackage.IDEQ(packageID), userpackage.StatusEQ(service.PackageStatusVoided)).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, service.ErrPackageNotFound
	}
	return r.GetUserPackage(ctx, packageID)
}

// ExpirePackages 把已到期的 active 套餐标记为 expired，返回受影响的行（用于失效缓存）。
// 冻结中的套餐暂停计时，不参与过期。
func (r *packageRepository) ExpirePackages(ctx context.Context, now time.Time) ([]service.UserPackage, error) {
	const q = `
UPDATE user_packages SET status = 'expired', updated_at = $1
WHERE status = 'active' AND expires_at <= $1
RETURNING id, user_id, group_id`
	rows, err := r.db.QueryContext(ctx, q, now)
	if err != nil {
		return nil, fmt.Errorf("expire packages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []service.UserPackage
	for rows.Next() {
		var p service.UserPackage
		if err := rows.Scan(&p.ID, &p.UserID, &p.GroupID); err != nil {
			return nil, err
		}
		p.Status = service.PackageStatusExpired
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---------- 映射 ----------

func packagePlanToService(row *dbent.PackagePlan) *service.PackagePlan {
	return &service.PackagePlan{
		ID:           row.ID,
		GroupID:      row.GroupID,
		Name:         row.Name,
		Cycle:        row.Cycle,
		Tier:         int(row.Tier),
		Price:        row.Price,
		QuotaUSD:     row.QuotaUsd,
		ValidityDays: row.ValidityDays,
		ForSale:      row.ForSale,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func userPackageToService(row *dbent.UserPackage) *service.UserPackage {
	return &service.UserPackage{
		ID:                 row.ID,
		UserID:             row.UserID,
		GroupID:            row.GroupID,
		PlanID:             row.PlanID,
		OrderID:            row.OrderID,
		Name:               row.Name,
		Cycle:              row.Cycle,
		Tier:               int(row.Tier),
		QuotaUSD:           row.QuotaUsd,
		UsedUSD:            row.UsedUsd,
		StartsAt:           row.StartsAt,
		ExpiresAt:          row.ExpiresAt,
		Status:             row.Status,
		FrozenAt:           row.FrozenAt,
		FrozenSecondsTotal: row.FrozenSecondsTotal,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}
