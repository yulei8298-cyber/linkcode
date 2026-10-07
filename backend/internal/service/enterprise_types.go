package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 企业尊享：累计消费达到阈值的用户自动获得，管理员可对单个用户手动开通或关闭。
// 逻辑挂在套餐服务上（共用设置仓储与套餐订单数据），不单独增加一套依赖注入。

// SettingKeyEnterpriseSettings 企业尊享配置在 settings 表中的 key，整块 JSON 存储。
const SettingKeyEnterpriseSettings = "enterprise_settings"

const (
	// DefaultEnterpriseThreshold 自动获得企业尊享所需的累计消费。
	DefaultEnterpriseThreshold = 3000.0
	enterpriseThresholdMax     = 100_000_000.0

	// EnterpriseModeAuto / On / Off 单个用户的开通方式。auto 即没有手动覆盖。
	EnterpriseModeAuto = "auto"
	EnterpriseModeOn   = "on"
	EnterpriseModeOff  = "off"
)

var ErrEnterpriseInvalid = infraerrors.BadRequest("ENTERPRISE_INVALID", "企业尊享设置不合法")

// EnterpriseSettings 企业尊享的全局配置。
type EnterpriseSettings struct {
	// Enabled 总开关；关闭后所有用户（包括被手动开通的）都不显示企业尊享。
	Enabled bool `json:"enabled"`
	// Threshold 自动获得所需的累计消费：已完成订单的实付人民币（套餐、订阅、余额充值）
	// 加兑换码入账的余额（美元额度），数值直接相加。
	Threshold float64 `json:"threshold"`
}

func DefaultEnterpriseSettings() EnterpriseSettings {
	return EnterpriseSettings{Enabled: true, Threshold: DefaultEnterpriseThreshold}
}

// Validate 校验管理端提交的配置。
func (s EnterpriseSettings) Validate() error {
	if math.IsNaN(s.Threshold) || math.IsInf(s.Threshold, 0) || s.Threshold <= 0 || s.Threshold > enterpriseThresholdMax {
		return errors.New("累计消费门槛必须大于 0")
	}
	return nil
}

// EnterpriseStatus 某用户的企业尊享状态。
type EnterpriseStatus struct {
	Enterprise bool `json:"enterprise"`
	// Mode 该用户的开通方式：auto 按累计自动，on 强制开通，off 强制关闭。
	Mode string `json:"mode"`
	// Total 当前累计消费，Threshold 自动获得的门槛。
	Total     float64 `json:"total"`
	Threshold float64 `json:"threshold"`
	// Enabled 全局总开关是否打开。
	Enabled bool `json:"enabled"`
}

// IsValidEnterpriseMode 是否合法的开通方式。
func IsValidEnterpriseMode(mode string) bool {
	return mode == EnterpriseModeAuto || mode == EnterpriseModeOn || mode == EnterpriseModeOff
}

func loadEnterpriseSettings(ctx context.Context, repo SettingRepository) (EnterpriseSettings, error) {
	settings := DefaultEnterpriseSettings()
	if repo == nil {
		return settings, nil
	}
	raw, err := repo.GetValue(ctx, SettingKeyEnterpriseSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return settings, nil
		}
		return settings, err
	}
	if strings.TrimSpace(raw) == "" {
		return settings, nil
	}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil || settings.Validate() != nil {
		return DefaultEnterpriseSettings(), nil
	}
	return settings, nil
}

func saveEnterpriseSettings(ctx context.Context, repo SettingRepository, settings EnterpriseSettings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal enterprise settings: %w", err)
	}
	return repo.Set(ctx, SettingKeyEnterpriseSettings, string(raw))
}
