package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// intelCheckSettingStore 是本服务对 settings 表的最小依赖面。
//
// 只声明用得到的两个方法而非直接依赖 SettingRepository，理由与
// channelMonitorRuntimeReader 相同：单测里给个十几行的 stub 就够，
// 不必为了一个 GetValue 去实现几十个无关方法。
type intelCheckSettingStore interface {
	GetValue(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
}

// intelCheckReloader 由 runner 实现，用于在设置变更后重建 ticker。
type intelCheckReloader interface {
	Reload()
}

// IntelCheckService 智力检测管理服务：受检分组与题库的 CRUD、设置读写、公开视图聚合。
//
// APIKey 的加解密只发生在本层：repository 存取的恒为密文，handler 拿到的是明文
// 但必须脱敏后再输出，公开接口则完全不输出该字段。
type IntelCheckService struct {
	repo        IntelCheckRepository
	settingRepo intelCheckSettingStore
	encryptor   SecretEncryptor
	motion      IntelCheckMotionEvaluator

	// reloader 由 wire 在 runner 构造完成后通过 SetReloader 注入
	// （runner 依赖本服务读设置，构造参数注入会形成循环）。
	// 未注入时所有调用变为 no-op，单测无需关心。
	reloader intelCheckReloader
}

// NewIntelCheckService 创建智力检测服务实例。
func NewIntelCheckService(
	repo IntelCheckRepository,
	settingRepo intelCheckSettingStore,
	encryptor SecretEncryptor,
) *IntelCheckService {
	return &IntelCheckService{repo: repo, settingRepo: settingRepo, encryptor: encryptor}
}

// SetReloader 注入 runner 的重载回调。
func (s *IntelCheckService) SetReloader(r intelCheckReloader) {
	if s == nil {
		return
	}
	s.reloader = r
}

// SetMotionEvaluator 注入独立动作验收器。nil 表示当前环境未配置，绘图结构
// 达标后会记为 unverified，不能因为基础设施缺失而显示绿色通过。
func (s *IntelCheckService) SetMotionEvaluator(evaluator IntelCheckMotionEvaluator) {
	if s != nil {
		s.motion = evaluator
	}
}

// ---------- 设置读写 ----------

// GetSettings 读取设置，缺失或损坏时回落默认值。
//
// 这里刻意不返回错误给调用方：本方法同时服务于公开页与 runner，
// 一条脏 JSON 不应该让公开页整页 500——回落默认值 + 告警是更合适的降级。
// 真正的存储故障（非「key 不存在」）仍然向上抛。
func (s *IntelCheckService) GetSettings(ctx context.Context) (*IntelCheckSettings, error) {
	def := DefaultIntelCheckSettings()
	d := &def
	if s == nil || s.settingRepo == nil {
		return d, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyIntelCheckSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			// 首次读取时把默认值落库，让管理端设置页一开始就有完整表单。
			// 尽力而为：写失败不影响本次读取，下次读还会再试。
			if b, mErr := json.Marshal(d); mErr == nil {
				_ = s.settingRepo.Set(ctx, SettingKeyIntelCheckSettings, string(b))
			}
			return d, nil
		}
		return nil, fmt.Errorf("load intel check settings: %w", err)
	}

	cfg := &IntelCheckSettings{}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		slog.Warn("intel_check: settings json is corrupted, falling back to defaults", "error", err)
		return d, nil
	}
	cfg.Normalize()
	return cfg, nil
}

// UpdateSettings 整块覆盖设置。
//
// 覆盖而非逐字段合并：管理端设置页一次提交全部字段，合并语义反而会让
// 「把某项清空」变得无法表达（清空与未提交在 JSON 里长得一样）。
func (s *IntelCheckService) UpdateSettings(
	ctx context.Context, in *IntelCheckSettings,
) (*IntelCheckSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("设置存储未初始化")
	}
	if in == nil {
		return nil, fmt.Errorf("设置内容为空")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cfg := *in
	// 先归一化再校验：归一化负责把越界数值收敛到合法区间，
	// 校验只针对收敛不了的问题（文案超长、开启但评审未配），
	// 顺序颠倒会让「interval 填 2000」这种可自动修正的输入变成报错。
	cfg.Normalize()
	if err := ValidateIntelCheckSettings(&cfg); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal intel check settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyIntelCheckSettings, string(raw)); err != nil {
		return nil, fmt.Errorf("save intel check settings: %w", err)
	}

	// interval_minutes 或总开关可能已变，通知 runner 重建 ticker。
	// 放在持久化之后：重载读的是库里的值，先重载会读到旧配置。
	if s.reloader != nil {
		s.reloader.Reload()
	}
	return &cfg, nil
}

// IsEnabled 供公开接口做开关判断，任何异常都按「未开启」处理。
//
// fail-closed 而非 fail-open：读设置失败时放行，等于在故障时把一个
// 本该受开关控制的公开页暴露出去，比短暂 404 严重得多。
func (s *IntelCheckService) IsEnabled(ctx context.Context) bool {
	cfg, err := s.GetSettings(ctx)
	if err != nil || cfg == nil {
		return false
	}
	return cfg.Enabled
}

// ---------- API Key 加解密 ----------

// encryptAPIKey 加密上游凭据。空串也照常加密：空密文与「字段没填」在
// 存储层长得一样会导致 runner 无法区分，索性让空串也走一遍加密。
func (s *IntelCheckService) encryptAPIKey(plain string) (string, error) {
	if s == nil || s.encryptor == nil {
		return "", fmt.Errorf("密钥加密组件未初始化")
	}
	encrypted, err := s.encryptor.Encrypt(strings.TrimSpace(plain))
	if err != nil {
		return "", fmt.Errorf("encrypt intel check api key: %w", err)
	}
	return encrypted, nil
}

// decryptTargetInPlace 就地把密文换成明文，失败时置位标记而不报错。
//
// 沿用 ChannelMonitor 的处理方式：一把坏密钥不应该让整个列表接口 500。
// 置位后管理端显示「需重新填写」，runner 则跳过该分组。
func (s *IntelCheckService) decryptTargetInPlace(t *IntelCheckTarget) {
	if t == nil {
		return
	}
	if s == nil || s.encryptor == nil {
		t.APIKey = ""
		t.APIKeyDecryptFailed = true
		return
	}
	plain, err := s.encryptor.Decrypt(t.APIKey)
	if err != nil {
		slog.Warn("intel_check: decrypt target api key failed", "target_id", t.ID, "error", err)
		t.APIKey = ""
		t.APIKeyDecryptFailed = true
		return
	}
	t.APIKey = plain
}
