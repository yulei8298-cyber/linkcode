package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// channelMonitorAvgFirstTokenSQL 用探测 Key 反查所属分组，再统计该分组 + 主模型
// 在 since 之后真实用户调用的平均首字延迟。
//   - 排除探测 Key 自身的请求，只统计用户调用
//   - 模型按客户端请求名匹配；历史行 requested_model 为空时回退 model
//   - usage_logs 走 (group_id, created_at) 索引，窗口很短，扫描量可控
const channelMonitorAvgFirstTokenSQL = `
	WITH targets AS (
	    SELECT unnest($1::bigint[]) AS monitor_id,
	           unnest($2::text[])   AS api_key,
	           unnest($3::text[])   AS model
	),
	probe_keys AS (
	    SELECT t.monitor_id, t.model, k.id AS key_id, k.group_id
	    FROM targets t
	    JOIN api_keys k
	      ON k.key = t.api_key AND k.deleted_at IS NULL AND k.group_id IS NOT NULL
	)
	SELECT p.monitor_id, ROUND(AVG(ul.first_token_ms))::int AS avg_first_token_ms
	FROM probe_keys p
	JOIN usage_logs ul
	  ON ul.group_id = p.group_id
	 AND ul.created_at >= $4
	 AND ul.api_key_id <> p.key_id
	 AND ul.first_token_ms > 0
	 AND COALESCE(ul.requested_model, ul.model) = p.model
	GROUP BY p.monitor_id
`

// AvgFirstTokenForMonitors 见 service.ChannelMonitorRepository 同名方法说明。
func (r *channelMonitorRepository) AvgFirstTokenForMonitors(
	ctx context.Context,
	targets []service.MonitorFirstTokenTarget,
	since time.Time,
) (map[int64]int, error) {
	out := make(map[int64]int, len(targets))
	if len(targets) == 0 {
		return out, nil
	}
	ids := make([]int64, 0, len(targets))
	keys := make([]string, 0, len(targets))
	models := make([]string, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, t.MonitorID)
		keys = append(keys, t.APIKey)
		models = append(models, t.Model)
	}

	rows, err := r.db.QueryContext(ctx, channelMonitorAvgFirstTokenSQL,
		pq.Array(ids), pq.Array(keys), pq.Array(models), since)
	if err != nil {
		return nil, fmt.Errorf("query monitor avg first token: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var monitorID int64
		var avgMs int
		if err := rows.Scan(&monitorID, &avgMs); err != nil {
			return nil, fmt.Errorf("scan monitor avg first token: %w", err)
		}
		out[monitorID] = avgMs
	}
	return out, rows.Err()
}
