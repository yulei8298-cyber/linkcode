package service

import (
	"time"
)

// PeakValleyPricingConfig 峰谷定价配置
type PeakValleyPricingConfig struct {
	Enabled   bool                   `json:"enabled"`    // 是否启用峰谷定价
	Timezone  string                 `json:"timezone"`   // 时区（UTC/Asia/Shanghai）
	Schedules []PeakValleySchedule   `json:"schedules"`  // 时段配置列表
}

// PeakValleySchedule 单个时段配置
type PeakValleySchedule struct {
	Name         string    `json:"name"`          // 时段名称（如"高峰时段"）
	StartHour    int       `json:"start_hour"`    // 开始小时（0-23）
	StartMinute  int       `json:"start_minute"`  // 开始分钟（0-59，默认0）
	EndHour      int       `json:"end_hour"`      // 结束小时（0-23）
	EndMinute    int       `json:"end_minute"`    // 结束分钟（0-59，默认0）
	Multiplier   float64   `json:"multiplier"`    // 价格倍率
	Weekdays     []int     `json:"weekdays"`      // 生效星期（0=周日,1=周一,...,6=周六；空数组表示全周生效）
	Priority     int       `json:"priority"`      // 优先级（数字越大优先级越高，用于时段重叠时的选择）
}

// GetPeakValleyMultiplier 获取指定时刻的峰谷倍率
// 返回匹配时段的倍率，未匹配任何时段时返回 1.0
func GetPeakValleyMultiplier(config *PeakValleyPricingConfig, at time.Time) float64 {
	if config == nil || !config.Enabled || len(config.Schedules) == 0 {
		return 1.0
	}

	// 转换到配置的时区
	tz, err := time.LoadLocation(config.Timezone)
	if err != nil {
		// 时区解析失败，回退到 UTC
		tz = time.UTC
	}
	localTime := at.In(tz)

	// 查找所有匹配的时段
	var matched *PeakValleySchedule
	maxPriority := -1

	for i := range config.Schedules {
		schedule := &config.Schedules[i]

		// 检查星期是否匹配
		if !isWeekdayMatch(localTime.Weekday(), schedule.Weekdays) {
			continue
		}

		// 检查时间是否在区间内
		if !isTimeInRange(localTime, schedule) {
			continue
		}

		// 选择优先级最高的时段
		if schedule.Priority > maxPriority {
			matched = schedule
			maxPriority = schedule.Priority
		}
	}

	if matched != nil {
		return matched.Multiplier
	}

	return 1.0 // 默认倍率
}

// GetPeakValleyPeriodName 获取指定时刻所属的峰谷时段名称
// 返回匹配时段的名称，未匹配时返回空字符串
func GetPeakValleyPeriodName(config *PeakValleyPricingConfig, at time.Time) string {
	if config == nil || !config.Enabled || len(config.Schedules) == 0 {
		return ""
	}

	// 转换到配置的时区
	tz, err := time.LoadLocation(config.Timezone)
	if err != nil {
		tz = time.UTC
	}
	localTime := at.In(tz)

	// 查找所有匹配的时段
	var matched *PeakValleySchedule
	maxPriority := -1

	for i := range config.Schedules {
		schedule := &config.Schedules[i]

		if !isWeekdayMatch(localTime.Weekday(), schedule.Weekdays) {
			continue
		}

		if !isTimeInRange(localTime, schedule) {
			continue
		}

		if schedule.Priority > maxPriority {
			matched = schedule
			maxPriority = schedule.Priority
		}
	}

	if matched != nil {
		return matched.Name
	}

	return ""
}

// isWeekdayMatch 检查星期是否匹配
func isWeekdayMatch(weekday time.Weekday, allowedDays []int) bool {
	if len(allowedDays) == 0 {
		return true // 空数组表示全周生效
	}

	dayNum := int(weekday)
	for _, d := range allowedDays {
		if d == dayNum {
			return true
		}
	}
	return false
}

// isTimeInRange 检查时间是否在时段范围内（半开区间 [start, end)）
func isTimeInRange(t time.Time, schedule *PeakValleySchedule) bool {
	currentMinutes := t.Hour()*60 + t.Minute()
	startMinutes := schedule.StartHour*60 + schedule.StartMinute
	endMinutes := schedule.EndHour*60 + schedule.EndMinute

	// 处理跨日情况（如 22:00 - 02:00）
	if endMinutes <= startMinutes {
		// 跨日：当前时间在 start 之后或 end 之前
		return currentMinutes >= startMinutes || currentMinutes < endMinutes
	}

	// 同日：半开区间 [start, end)
	return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

// ValidatePeakValleyConfig 验证峰谷定价配置的合法性
func ValidatePeakValleyConfig(config *PeakValleyPricingConfig) error {
	if config == nil {
		return nil
	}

	if !config.Enabled {
		return nil
	}

	// 验证时区
	if config.Timezone != "" {
		if _, err := time.LoadLocation(config.Timezone); err != nil {
			return err
		}
	}

	// 验证每个时段
	for i, schedule := range config.Schedules {
		if schedule.Name == "" {
			return newValidationError("时段名称不能为空", i)
		}

		if schedule.StartHour < 0 || schedule.StartHour > 23 {
			return newValidationError("开始小时必须在 0-23 之间", i)
		}

		if schedule.EndHour < 0 || schedule.EndHour > 23 {
			return newValidationError("结束小时必须在 0-23 之间", i)
		}

		if schedule.StartMinute < 0 || schedule.StartMinute > 59 {
			return newValidationError("开始分钟必须在 0-59 之间", i)
		}

		if schedule.EndMinute < 0 || schedule.EndMinute > 59 {
			return newValidationError("结束分钟必须在 0-59 之间", i)
		}

		if schedule.Multiplier <= 0 {
			return newValidationError("价格倍率必须大于 0", i)
		}

		if schedule.Multiplier < 0.1 || schedule.Multiplier > 10.0 {
			return newValidationError("价格倍率建议在 0.1-10.0 之间", i)
		}

		// 验证星期（0-6）
		for _, day := range schedule.Weekdays {
			if day < 0 || day > 6 {
				return newValidationError("星期必须在 0-6 之间", i)
			}
		}
	}

	return nil
}

func newValidationError(msg string, scheduleIndex int) error {
	return &ValidationError{
		Message:       msg,
		ScheduleIndex: scheduleIndex,
	}
}

// ValidationError 峰谷配置验证错误
type ValidationError struct {
	Message       string
	ScheduleIndex int
}

func (e *ValidationError) Error() string {
	return e.Message
}
