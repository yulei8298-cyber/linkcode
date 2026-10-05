package repository

import (
	"context"
	"fmt"
)

// 套餐请求的用户级并发槽位，与余额请求的 concurrency:user:{id} 互不占用。
// 格式: concurrency:package_user:{userID}（有序集合，成员为 requestID）
const (
	packageUserSlotKeyPrefix     = "concurrency:package_user:"
	livePackageUserSlotKeyPrefix = "concurrency:live:package_user:"
)

func packageUserSlotKey(userID int64) string {
	return fmt.Sprintf("%s%d", packageUserSlotKeyPrefix, userID)
}

func livePackageUserSlotKey(userID int64) string {
	return fmt.Sprintf("%s%d", livePackageUserSlotKeyPrefix, userID)
}

// AcquirePackageUserSlot 复用通用占槽脚本：过期槽位在脚本内按 TTL 清理，不依赖启动清理。
func (c *concurrencyCache) AcquirePackageUserSlot(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
	result, _, err := runScriptInt64Pair(ctx, c.rdb, acquireScript,
		[]string{packageUserSlotKey(userID), livePackageUserSlotKey(userID)}, maxConcurrency, c.slotTTLSeconds, requestID)
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (c *concurrencyCache) ReleasePackageUserSlot(ctx context.Context, userID int64, requestID string) error {
	return c.rdb.ZRem(ctx, packageUserSlotKey(userID), requestID).Err()
}
