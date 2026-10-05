package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// SettingKeyPackageSettings 套餐配置在 settings 表中的 key，整块 JSON 存储。
const SettingKeyPackageSettings = "package_settings"

const (
	// DefaultPackageHolidaySourceURL 官方节假日数据（NateScarlet/holiday-cn，按国务院通知整理），{year} 会被替换为年份。
	DefaultPackageHolidaySourceURL = "https://cdn.jsdelivr.net/gh/NateScarlet/holiday-cn@master/{year}.json"
	PackageNoticeMaxRunes          = 5000
	packageHolidaySourceMaxLen     = 500
)

// DefaultPackageNotice 默认购买须知。每行一条「标题：内容」，〔〕内文字前端高亮，
// {并发} 由前端替换为用户的套餐并发，{冻结上限} 替换为单张累计冻结天数上限。
const DefaultPackageNotice = `立即生效：付款成功后套餐立即生效，可在「我的套餐」查看剩余额度和到期时间。
适用范围：套餐仅限购买时选择的分组使用，不能跨分组、不能转给其他账号。
计费说明：套餐额度按所选分组的倍率扣减，与余额扣费口径一致。〔分组倍率会随上游官方价格和风控政策调整〕，调整后按新倍率扣减，已购额度金额不变。
扣费顺序：同一分组有多张套餐时，先到期的先扣；有套餐时优先扣套餐，套餐用完、到期或冻结时按账户余额扣费。
重复购买：重复购买只叠加额度，每张套餐独立计时，不会延长原有套餐的时间。
有效期：〔到期或额度用完即作废〕，剩余额度不退回余额，也不转到下一张套餐。
冻结规则：仅周末和法定节假日可以冻结，以「我的套餐」中的冻结日历为准；冻结期间暂停计时，可随时手动解冻，解冻后到期时间按冻结时长顺延。〔每张套餐累计最多冻结 {冻结上限} 天〕，满了自动解冻。
并发说明：套餐不限 RPM，〔同一账号所有套餐的请求合计同时不超过 {并发} 个〕，超出的请求会被拒绝并提示「套餐并发已达上限」。如需更高并发请联系客服。
可用性：上游服务受官方风控和维护影响，可能出现短时不可用或模型调整，平台会尽快恢复。
使用规则：仅限本人在 Claude Code、Codex 等工具中使用，〔禁止转售、分发、共享、破限及 NSFW 等违规用途〕，一经发现封号处理，不予退款。
退款政策：套餐属于虚拟商品，付款后不支持退款；如因平台原因长期无法供应，按剩余额度比例原路退回。`

// PackageSettings 套餐全局设置。
type PackageSettings struct {
	FreezeEnabled      bool   `json:"freeze_enabled"`
	MaxFreezeDays      int    `json:"max_freeze_days"`
	HolidaySyncEnabled bool   `json:"holiday_sync_enabled"`
	HolidaySourceURL   string `json:"holiday_source_url"`
	NoticeText         string `json:"notice_text"`
	NoticeVersion      int    `json:"notice_version"`
}

// DefaultPackageSettings 返回默认设置。
func DefaultPackageSettings() PackageSettings {
	return PackageSettings{
		FreezeEnabled:      true,
		MaxFreezeDays:      DefaultPackageMaxFreezeDay,
		HolidaySyncEnabled: true,
		HolidaySourceURL:   DefaultPackageHolidaySourceURL,
		NoticeText:         DefaultPackageNotice,
		NoticeVersion:      1,
	}
}

// Normalize 把越界值收敛到合法范围。
func (s *PackageSettings) Normalize() {
	if s.MaxFreezeDays <= 0 {
		s.MaxFreezeDays = DefaultPackageMaxFreezeDay
	}
	if s.MaxFreezeDays > MaxPackageMaxFreezeDay {
		s.MaxFreezeDays = MaxPackageMaxFreezeDay
	}
	s.HolidaySourceURL = strings.TrimSpace(s.HolidaySourceURL)
	if s.HolidaySourceURL == "" {
		s.HolidaySourceURL = DefaultPackageHolidaySourceURL
	}
	s.NoticeText = strings.TrimSpace(s.NoticeText)
	if s.NoticeText == "" {
		s.NoticeText = DefaultPackageNotice
	}
	if s.NoticeVersion <= 0 {
		s.NoticeVersion = 1
	}
}

// Validate 校验管理端提交的设置；数值越界交给 Normalize 收敛。
func (s *PackageSettings) Validate() error {
	if utf8.RuneCountInString(s.NoticeText) > PackageNoticeMaxRunes {
		return fmt.Errorf("购买须知不能超过 %d 字", PackageNoticeMaxRunes)
	}
	url := strings.TrimSpace(s.HolidaySourceURL)
	if len(url) > packageHolidaySourceMaxLen {
		return errors.New("节假日数据地址过长")
	}
	if url != "" && !strings.HasPrefix(url, "https://") {
		return errors.New("节假日数据地址必须以 https:// 开头")
	}
	if url != "" && !strings.Contains(url, "{year}") {
		return errors.New("节假日数据地址必须包含 {year} 占位符")
	}
	return nil
}

// FreezeCapSeconds 单张套餐累计冻结上限（秒）。
func (s PackageSettings) FreezeCapSeconds() int64 {
	return int64(s.MaxFreezeDays) * 86400
}

func loadPackageSettings(ctx context.Context, repo SettingRepository) (PackageSettings, error) {
	settings := DefaultPackageSettings()
	if repo == nil {
		return settings, nil
	}
	raw, err := repo.GetValue(ctx, SettingKeyPackageSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return settings, nil
		}
		return settings, err
	}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &settings); err != nil {
			return DefaultPackageSettings(), nil
		}
	}
	settings.Normalize()
	return settings, nil
}

func savePackageSettings(ctx context.Context, repo SettingRepository, settings PackageSettings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return repo.Set(ctx, SettingKeyPackageSettings, string(raw))
}
