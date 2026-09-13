package service

import (
	"fmt"
	"strings"
)

// 文案与模型名长度上限。超限时报错而非截断——截断会静默改写管理员填的内容。
const (
	IntelCheckMaxIntroTitleRunes = 100
	IntelCheckMaxIntroTextRunes  = 2000
	IntelCheckMaxJudgeModelRunes = 200
)

// NormalizeIntelCheckReasoningEffort 归一化推理等级，无法识别时回落 high。
//
// 回落到 high 而不是 medium：检测的前提是「让模型尽可能发挥」，
// 等级填错时降低它会把配置失误伪装成模型降智。
func NormalizeIntelCheckReasoningEffort(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case IntelCheckEffortLow:
		return IntelCheckEffortLow
	case IntelCheckEffortMedium:
		return IntelCheckEffortMedium
	case IntelCheckEffortXHigh:
		return IntelCheckEffortXHigh
	default:
		return IntelCheckEffortHigh
	}
}

// ValidateIntelCheckSettings 校验管理端提交的设置。
//
// 只查归一化无法安抚的问题——长度超限与逻辑矛盾。数值越界交给
// IntelCheckSettings.Normalize 收敛，因为把 interval=2000 直接报错给管理员，
// 不如静默收到 1440 更有用。
func ValidateIntelCheckSettings(s *IntelCheckSettings) error {
	if s == nil {
		return fmt.Errorf("设置内容为空")
	}
	if n := len([]rune(strings.TrimSpace(s.IntroTitle))); n > IntelCheckMaxIntroTitleRunes {
		return fmt.Errorf("页面标题过长：%d 字，上限 %d 字", n, IntelCheckMaxIntroTitleRunes)
	}
	if n := len([]rune(strings.TrimSpace(s.IntroText))); n > IntelCheckMaxIntroTextRunes {
		return fmt.Errorf("页面说明过长：%d 字，上限 %d 字", n, IntelCheckMaxIntroTextRunes)
	}
	if n := len([]rune(strings.TrimSpace(s.DrawingJudge.Model))); n > IntelCheckMaxJudgeModelRunes {
		return fmt.Errorf("评审模型名过长：%d 字，上限 %d 字", n, IntelCheckMaxJudgeModelRunes)
	}
	// 开启功能时必须已经配好评审模型，否则绘图题只会持续产出 request_error，
	// 公开页会挂满黄块——那比不开启更糟，用户会以为是上游在抖。
	if s.Enabled {
		if s.DrawingJudge.TargetID <= 0 {
			return fmt.Errorf("开启前请先指定源码评审所使用的受检分组")
		}
		if strings.TrimSpace(s.DrawingJudge.Model) == "" {
			return fmt.Errorf("开启前请先填写源码评审所使用的模型名")
		}
	}
	return nil
}
