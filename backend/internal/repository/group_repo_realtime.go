package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 分组实时指标所需的查询：滚动窗口 RPM、分组内 API Key 的归属用户。
// 都是只读的轻量查询，供管理端分组页高频轮询。

// groupAPIKeyOwnerLimit 单个分组最多读取的 API Key 数，防止异常数据拖垮 Redis 批量读取。
const groupAPIKeyOwnerLimit = 20000

var errGroupRepoSQLUnavailable = errors.New("group repository: sql executor unavailable")

// ListGroupRealtimeRPM 统计最近 window 内完成并记账的请求数，按分组汇总。
// 走 usage_logs(created_at) 索引，窗口只有几十秒，扫描行数很少；没有请求的分组不返回。
func (r *groupRepository) ListGroupRealtimeRPM(ctx context.Context, window time.Duration) ([]service.GroupRealtimeRPM, error) {
	if r.sql == nil {
		return nil, errGroupRepoSQLUnavailable
	}
	rows, err := r.sql.QueryContext(ctx, `
		SELECT group_id, COUNT(*)
		FROM usage_logs
		WHERE created_at >= NOW() - ($1::double precision * INTERVAL '1 second')
		  AND group_id IS NOT NULL
		GROUP BY group_id
		ORDER BY group_id
	`, window.Seconds())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.GroupRealtimeRPM, 0)
	for rows.Next() {
		var item service.GroupRealtimeRPM
		if err := rows.Scan(&item.GroupID, &item.RPM); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListActiveAPIKeyOwnersByGroup 列出绑定在该分组上的 API Key 及其所属用户。
// 已删除的 Key 和用户不返回；密钥明文不在查询范围内。
func (r *groupRepository) ListActiveAPIKeyOwnersByGroup(ctx context.Context, groupID int64) ([]service.GroupAPIKeyOwner, error) {
	if r.sql == nil {
		return nil, errGroupRepoSQLUnavailable
	}
	rows, err := r.sql.QueryContext(ctx, `
		SELECT k.id, k.name, k.user_id, u.email, u.username
		FROM api_keys k
		JOIN users u ON u.id = k.user_id AND u.deleted_at IS NULL
		WHERE k.group_id = $1
		  AND k.deleted_at IS NULL
		ORDER BY k.id
		LIMIT $2
	`, groupID, groupAPIKeyOwnerLimit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	owners := make([]service.GroupAPIKeyOwner, 0)
	for rows.Next() {
		var owner service.GroupAPIKeyOwner
		if err := rows.Scan(&owner.APIKeyID, &owner.APIKeyName, &owner.UserID, &owner.Email, &owner.Username); err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	return owners, rows.Err()
}
