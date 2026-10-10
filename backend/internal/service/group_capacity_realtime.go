package service

import (
	"context"
	"sort"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 管理端分组页的实时指标：每个分组的 RPM、分组内每个用户的当前并发。
// 与容量汇总（账号侧上限的占用）互补，这里反映的是用户侧真实流量。

// GroupRealtimeRPMWindow 实时 RPM 的统计窗口：最近 60 秒（滚动），数值即每分钟请求数。
const GroupRealtimeRPMWindow = 60 * time.Second

var ErrGroupRealtimeUnsupported = infraerrors.ServiceUnavailable(
	"GROUP_REALTIME_UNSUPPORTED", "当前部署不支持分组实时指标",
)

// GroupRealtimeRPM 某分组最近一个窗口内完成并记账的请求数。
type GroupRealtimeRPM struct {
	GroupID int64 `json:"group_id"`
	RPM     int   `json:"rpm"`
}

// GroupRealtimeRPMSummary 全部分组的实时 RPM。没有请求的分组不出现在 Items 中，前端按 0 展示。
type GroupRealtimeRPMSummary struct {
	WindowSeconds int                `json:"window_seconds"`
	Total         int                `json:"total"`
	Items         []GroupRealtimeRPM `json:"items"`
}

// GroupAPIKeyOwner 绑定在某分组上的 API Key 及其所属用户。
type GroupAPIKeyOwner struct {
	APIKeyID   int64
	APIKeyName string
	UserID     int64
	Email      string
	Username   string
}

// GroupUserConcurrencyKey 某用户在该分组内、正在处理请求的一个 API Key。
type GroupUserConcurrencyKey struct {
	APIKeyID    int64  `json:"api_key_id"`
	APIKeyName  string `json:"api_key_name"`
	Concurrency int    `json:"concurrency"`
}

// GroupUserConcurrency 某用户在该分组内的当前并发（各 API Key 并发之和）。
type GroupUserConcurrency struct {
	UserID      int64                     `json:"user_id"`
	Email       string                    `json:"email"`
	Username    string                    `json:"username"`
	Concurrency int                       `json:"concurrency"`
	APIKeys     []GroupUserConcurrencyKey `json:"api_keys"`
}

// GroupUserConcurrencySummary 分组内所有「当前有请求在处理」的用户。
type GroupUserConcurrencySummary struct {
	GroupID  int64                  `json:"group_id"`
	Total    int                    `json:"total"`
	Users    []GroupUserConcurrency `json:"users"`
	KeyCount int                    `json:"api_key_count"`
}

type groupRealtimeRPMReader interface {
	ListGroupRealtimeRPM(ctx context.Context, window time.Duration) ([]GroupRealtimeRPM, error)
}

type groupAPIKeyOwnerLister interface {
	ListActiveAPIKeyOwnersByGroup(ctx context.Context, groupID int64) ([]GroupAPIKeyOwner, error)
}

// GetRealtimeRPM 返回所有分组最近 60 秒的请求数。
func (s *GroupCapacityService) GetRealtimeRPM(ctx context.Context) (*GroupRealtimeRPMSummary, error) {
	reader, ok := s.groupRepo.(groupRealtimeRPMReader)
	if !ok {
		return nil, ErrGroupRealtimeUnsupported
	}
	items, err := reader.ListGroupRealtimeRPM(ctx, GroupRealtimeRPMWindow)
	if err != nil {
		return nil, err
	}
	summary := &GroupRealtimeRPMSummary{
		WindowSeconds: int(GroupRealtimeRPMWindow / time.Second),
		Items:         items,
	}
	if summary.Items == nil {
		summary.Items = []GroupRealtimeRPM{}
	}
	for _, item := range items {
		summary.Total += item.RPM
	}
	return summary, nil
}

// GetGroupUserConcurrency 汇总分组内每个用户当前的并发请求数。
// 并发按 API Key 槽位统计（网关每个请求都会登记），再按 Key 所属用户累加；
// 只返回当前并发大于 0 的用户，按并发从高到低排序。
func (s *GroupCapacityService) GetGroupUserConcurrency(ctx context.Context, groupID int64) (*GroupUserConcurrencySummary, error) {
	lister, ok := s.groupRepo.(groupAPIKeyOwnerLister)
	if !ok {
		return nil, ErrGroupRealtimeUnsupported
	}
	if _, err := s.groupRepo.GetByID(ctx, groupID); err != nil {
		return nil, err
	}
	owners, err := lister.ListActiveAPIKeyOwnersByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}

	summary := &GroupUserConcurrencySummary{GroupID: groupID, Users: []GroupUserConcurrency{}, KeyCount: len(owners)}
	if len(owners) == 0 {
		return summary, nil
	}

	keyIDs := make([]int64, 0, len(owners))
	for _, owner := range owners {
		keyIDs = append(keyIDs, owner.APIKeyID)
	}
	counts, err := s.concurrencyService.GetAPIKeyConcurrencyBatch(ctx, keyIDs)
	if err != nil {
		return nil, err
	}

	byUser := make(map[int64]*GroupUserConcurrency)
	for _, owner := range owners {
		current := counts[owner.APIKeyID]
		if current <= 0 {
			continue
		}
		user, exists := byUser[owner.UserID]
		if !exists {
			user = &GroupUserConcurrency{UserID: owner.UserID, Email: owner.Email, Username: owner.Username}
			byUser[owner.UserID] = user
		}
		user.Concurrency += current
		user.APIKeys = append(user.APIKeys, GroupUserConcurrencyKey{
			APIKeyID: owner.APIKeyID, APIKeyName: owner.APIKeyName, Concurrency: current,
		})
		summary.Total += current
	}

	for _, user := range byUser {
		sort.SliceStable(user.APIKeys, func(i, j int) bool {
			if user.APIKeys[i].Concurrency != user.APIKeys[j].Concurrency {
				return user.APIKeys[i].Concurrency > user.APIKeys[j].Concurrency
			}
			return user.APIKeys[i].APIKeyID < user.APIKeys[j].APIKeyID
		})
		summary.Users = append(summary.Users, *user)
	}
	sort.SliceStable(summary.Users, func(i, j int) bool {
		if summary.Users[i].Concurrency != summary.Users[j].Concurrency {
			return summary.Users[i].Concurrency > summary.Users[j].Concurrency
		}
		return summary.Users[i].UserID < summary.Users[j].UserID
	})
	return summary, nil
}
