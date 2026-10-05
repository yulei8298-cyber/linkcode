package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/dgraph-io/ristretto"
	"golang.org/x/sync/singleflight"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	packageStateCacheSize = 20000
	// packageStateCacheTTL 鉴权用的套餐概况缓存时长。本实例的购买、冻结、解冻、用完、
	// 过期会主动失效；其他实例最多延迟一个 TTL，期间按套餐计费的请求在扣费时
	// 找不到可用套餐会自动转余额，不会少扣。
	packageStateCacheTTL = 30 * time.Second
)

// PackageService 套餐业务：配置、购买发货、冻结 / 解冻、请求鉴权概况、定时维护。
type PackageService struct {
	repo        PackageRepository
	settingRepo SettingRepository
	groupRepo   GroupRepository
	userRepo    UserRepository
	now         func() time.Time
	httpClient  *http.Client // 节假日同步用，为空时使用默认超时客户端

	stateCache *ristretto.Cache
	stateGroup singleflight.Group
}

// NewPackageService 创建套餐服务。
func NewPackageService(repo PackageRepository, settingRepo SettingRepository, groupRepo GroupRepository, userRepo UserRepository) *PackageService {
	s := &PackageService{repo: repo, settingRepo: settingRepo, groupRepo: groupRepo, userRepo: userRepo, now: timezone.Now}
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: packageStateCacheSize * 10,
		MaxCost:     packageStateCacheSize,
		BufferItems: 64,
	})
	if err != nil {
		log.Printf("Warning: failed to init package state cache: %v", err)
	} else {
		s.stateCache = cache
	}
	return s
}

// ---------- 设置 ----------

// GetSettings 读取套餐设置（不存在时返回默认值）。
func (s *PackageService) GetSettings(ctx context.Context) (PackageSettings, error) {
	return loadPackageSettings(ctx, s.settingRepo)
}

// UpdateSettings 保存设置。购买须知正文变化时版本号 +1，已同意旧版本的下单会被要求重新阅读；
// 关闭冻结功能时立即解冻所有冻结中的套餐。
func (s *PackageService) UpdateSettings(ctx context.Context, in PackageSettings) (PackageSettings, error) {
	if err := in.Validate(); err != nil {
		return PackageSettings{}, infraerrors.BadRequest("PACKAGE_SETTINGS_INVALID", err.Error())
	}
	current, err := s.GetSettings(ctx)
	if err != nil {
		return PackageSettings{}, err
	}
	in.Normalize()
	in.NoticeVersion = current.NoticeVersion
	if in.NoticeText != current.NoticeText {
		in.NoticeVersion = current.NoticeVersion + 1
	}
	if err := savePackageSettings(ctx, s.settingRepo, in); err != nil {
		return PackageSettings{}, fmt.Errorf("save package settings: %w", err)
	}
	if current.FreezeEnabled && !in.FreezeEnabled {
		s.unfreezeAll(ctx, PackageUnfreezeDisabled, in.FreezeCaps())
	}
	return in, nil
}

func (s *PackageService) unfreezeAll(ctx context.Context, reason string, caps PackageFreezeCaps) {
	ids, err := s.repo.ListFrozenIDs(ctx)
	if err != nil {
		log.Printf("[Package] list frozen packages failed: %v", err)
		return
	}
	s.unfreezeIDs(ctx, ids, reason, caps)
}

func (s *PackageService) unfreezeIDs(ctx context.Context, ids []int64, reason string, caps PackageFreezeCaps) int {
	done := 0
	for _, id := range ids {
		pkg, err := s.repo.UnfreezePackage(ctx, PackageUnfreezeInput{
			PackageID: id, Now: s.now(), Reason: reason, Caps: caps,
		})
		if err != nil {
			log.Printf("[Package] unfreeze package %d (%s) failed: %v", id, reason, err)
			continue
		}
		s.InvalidateGroupState(pkg.UserID, pkg.GroupID)
		done++
	}
	return done
}

// ---------- 请求鉴权概况 ----------

// ResolveGroupState 返回用户在分组上的套餐概况（带 L1 缓存与 singleflight）。
func (s *PackageService) ResolveGroupState(ctx context.Context, userID, groupID int64) (*PackageGroupState, error) {
	key := packageStateCacheKey(userID, groupID)
	if s.stateCache != nil {
		if v, ok := s.stateCache.Get(key); ok {
			if st, ok := v.(PackageGroupState); ok {
				return &st, nil
			}
		}
	}
	value, err, _ := s.stateGroup.Do(key, func() (any, error) {
		st, err := s.repo.GetGroupState(ctx, userID, groupID, s.now())
		if err != nil {
			return nil, err
		}
		if s.stateCache != nil {
			_ = s.stateCache.SetWithTTL(key, *st, 1, packageStateCacheTTL)
		}
		return *st, nil
	})
	if err != nil {
		return nil, err
	}
	st := value.(PackageGroupState)
	return &st, nil
}

// InvalidateGroupState 失效某用户在某分组的套餐概况缓存。
func (s *PackageService) InvalidateGroupState(userID, groupID int64) {
	if s == nil || s.stateCache == nil {
		return
	}
	s.stateCache.Del(packageStateCacheKey(userID, groupID))
}

func packageStateCacheKey(userID, groupID int64) string {
	return "pkg:" + strconv.FormatInt(userID, 10) + ":" + strconv.FormatInt(groupID, 10)
}

// ---------- 定时维护 ----------

// RunMaintenance 过期到期的套餐，并解冻累计冻结达到上限的套餐。返回（过期张数，解冻张数）。
func (s *PackageService) RunMaintenance(ctx context.Context) (int, int, error) {
	expired, err := s.repo.ExpirePackages(ctx, s.now())
	if err != nil {
		return 0, 0, err
	}
	for _, p := range expired {
		s.InvalidateGroupState(p.UserID, p.GroupID)
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return len(expired), 0, err
	}
	ids, err := s.repo.ListFrozenOverCap(ctx, s.now(), settings.FreezeCaps())
	if err != nil {
		return len(expired), 0, err
	}
	return len(expired), s.unfreezeIDs(ctx, ids, PackageUnfreezeCap, settings.FreezeCaps()), nil
}
