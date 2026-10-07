package service

import (
	"context"
	"fmt"
)

// GetEnterpriseSettings 读取企业尊享配置，未配置时返回默认值。
func (s *PackageService) GetEnterpriseSettings(ctx context.Context) (EnterpriseSettings, error) {
	return loadEnterpriseSettings(ctx, s.settingRepo)
}

// UpdateEnterpriseSettings 保存企业尊享配置。
func (s *PackageService) UpdateEnterpriseSettings(ctx context.Context, in EnterpriseSettings) (EnterpriseSettings, error) {
	if err := in.Validate(); err != nil {
		return EnterpriseSettings{}, ErrEnterpriseInvalid.WithMetadata(map[string]string{"reason": err.Error()})
	}
	if err := saveEnterpriseSettings(ctx, s.settingRepo, in); err != nil {
		return EnterpriseSettings{}, fmt.Errorf("save enterprise settings: %w", err)
	}
	return in, nil
}

// EnterpriseStatus 计算某用户是否是企业尊享用户。
// 总开关关闭时一律不是；手动开通 / 关闭优先；否则按累计消费是否达到门槛自动判定。
func (s *PackageService) EnterpriseStatus(ctx context.Context, userID int64) (*EnterpriseStatus, error) {
	settings, err := s.GetEnterpriseSettings(ctx)
	if err != nil {
		return nil, err
	}
	mode, err := s.repo.GetEnterpriseOverride(ctx, userID)
	if err != nil {
		return nil, err
	}
	if mode == "" {
		mode = EnterpriseModeAuto
	}
	total, err := s.repo.EnterpriseSpent(ctx, userID)
	if err != nil {
		return nil, err
	}
	status := &EnterpriseStatus{Mode: mode, Total: total, Threshold: settings.Threshold, Enabled: settings.Enabled}
	if !settings.Enabled {
		return status, nil
	}
	switch mode {
	case EnterpriseModeOn:
		status.Enterprise = true
	case EnterpriseModeOff:
		status.Enterprise = false
	default:
		status.Enterprise = total >= settings.Threshold
	}
	return status, nil
}

// SetEnterpriseMode 管理员手动设置某用户的开通方式；auto 表示去掉手动覆盖。
func (s *PackageService) SetEnterpriseMode(ctx context.Context, userID int64, mode string) (*EnterpriseStatus, error) {
	if !IsValidEnterpriseMode(mode) {
		return nil, ErrEnterpriseInvalid
	}
	stored := mode
	if mode == EnterpriseModeAuto {
		stored = ""
	}
	if err := s.repo.SetEnterpriseOverride(ctx, userID, stored); err != nil {
		return nil, err
	}
	return s.EnterpriseStatus(ctx, userID)
}
