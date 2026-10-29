package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
	"golang.org/x/sync/singleflight"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"

	gocache "github.com/patrickmn/go-cache"
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
	// stateEpoch 每次失效加一，回源写缓存前比对，丢弃失效前已读库的结果。用全局计数而非
	// 按键计数，避免键集合无限增长；代价只是失效瞬间其他键的并发回源少缓存一次。
	stateMu    sync.RWMutex
	stateEpoch uint64

	// 企业尊享配置与用户身份缓存，首次使用时创建。
	enterpriseCache     *gocache.Cache
	enterpriseCacheOnce sync.Once
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
		epoch := s.currentStateEpoch()
		st, err := s.repo.GetGroupState(ctx, userID, groupID, s.now())
		if err != nil {
			return nil, err
		}
		s.cacheGroupState(key, *st, epoch)
		return *st, nil
	})
	if err != nil {
		return nil, err
	}
	st := value.(PackageGroupState)
	return &st, nil
}

func (s *PackageService) currentStateEpoch() uint64 {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.stateEpoch
}

// cacheGroupState 仅当回源期间没有发生失效时写缓存。比对与写入在读锁内完成，
// 失效方递增代际须拿写锁，因此写入要么排在失效的 Wait 之前被删掉，要么被拒绝。
func (s *PackageService) cacheGroupState(key string, st PackageGroupState, epoch uint64) {
	if s.stateCache == nil {
		return
	}
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	if s.stateEpoch == epoch {
		_ = s.stateCache.SetWithTTL(key, st, 1, packageStateCacheTTL)
	}
}

// InvalidateGroupState 失效某用户在某分组的套餐概况缓存。须在状态变更落库之后调用。
//
// 三处时序各自处理：
//   - 进行中的回源可能在变更前读库、在失效后写缓存：递增代际让它的写入被拒绝；
//   - 失效后到达的请求不能复用进行中的旧回源：Forget 让它们重新回源；
//   - ristretto 首次写入是异步缓冲的，Del 之前排队的旧写入会在之后才落地：先 Wait
//     冲刷缓冲区，再由 Del 同步删除。
//
// 失效是低频操作，读路径不加锁等待。
func (s *PackageService) InvalidateGroupState(userID, groupID int64) {
	if s == nil {
		return
	}
	key := packageStateCacheKey(userID, groupID)
	s.stateGroup.Forget(key)
	if s.stateCache == nil {
		return
	}
	s.stateMu.Lock()
	s.stateEpoch++
	s.stateMu.Unlock()
	s.stateCache.Wait()
	s.stateCache.Del(key)
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
