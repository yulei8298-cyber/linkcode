//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetPeakValleyMultiplier_NilConfig(t *testing.T) {
	multiplier := GetPeakValleyMultiplier(nil, time.Now())
	require.Equal(t, 1.0, multiplier)
}

func TestGetPeakValleyMultiplier_DisabledConfig(t *testing.T) {
	config := &PeakValleyPricingConfig{
		Enabled: false,
		Schedules: []PeakValleySchedule{
			{Name: "高峰", StartHour: 8, EndHour: 20, Multiplier: 2.0},
		},
	}
	multiplier := GetPeakValleyMultiplier(config, time.Now())
	require.Equal(t, 1.0, multiplier)
}

func TestGetPeakValleyMultiplier_EmptySchedules(t *testing.T) {
	config := &PeakValleyPricingConfig{
		Enabled:   true,
		Schedules: []PeakValleySchedule{},
	}
	multiplier := GetPeakValleyMultiplier(config, time.Now())
	require.Equal(t, 1.0, multiplier)
}

func TestGetPeakValleyMultiplier_BasicSchedule(t *testing.T) {
	config := &PeakValleyPricingConfig{
		Enabled:  true,
		Timezone: "UTC",
		Schedules: []PeakValleySchedule{
			{
				Name:        "高峰时段",
				StartHour:   8,
				StartMinute: 0,
				EndHour:     20,
				EndMinute:   0,
				Multiplier:  2.0,
				Weekdays:    []int{}, // 全周生效
				Priority:    1,
			},
		},
	}

	tests := []struct {
		name     string
		hour     int
		minute   int
		expected float64
	}{
		{"08:00 高峰开始", 8, 0, 2.0},
		{"12:00 高峰期间", 12, 0, 2.0},
		{"19:59 高峰结束前", 19, 59, 2.0},
		{"20:00 高峰结束", 20, 0, 1.0},
		{"07:59 高峰开始前", 7, 59, 1.0},
		{"00:00 低谷时段", 0, 0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := time.Date(2026, 9, 8, tt.hour, tt.minute, 0, 0, time.UTC)
			multiplier := GetPeakValleyMultiplier(config, testTime)
			require.Equal(t, tt.expected, multiplier)
		})
	}
}

func TestGetPeakValleyMultiplier_WeekdayFilter(t *testing.T) {
	config := &PeakValleyPricingConfig{
		Enabled:  true,
		Timezone: "UTC",
		Schedules: []PeakValleySchedule{
			{
				Name:       "工作日高峰",
				StartHour:  8,
				EndHour:    20,
				Multiplier: 2.0,
				Weekdays:   []int{1, 2, 3, 4, 5}, // 周一到周五
				Priority:   1,
			},
		},
	}

	tests := []struct {
		name     string
		date     time.Time
		hour     int
		expected float64
	}{
		{"周一 12:00 工作日高峰", time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC), 12, 2.0},   // 2026-09-07 是周一
		{"周五 12:00 工作日高峰", time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC), 12, 2.0},  // 2026-09-11 是周五
		{"周六 12:00 周末低谷", time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC), 12, 1.0},    // 2026-09-12 是周六
		{"周日 12:00 周末低谷", time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), 12, 1.0},    // 2026-09-13 是周日
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			multiplier := GetPeakValleyMultiplier(config, tt.date)
			require.Equal(t, tt.expected, multiplier)
		})
	}
}

func TestGetPeakValleyMultiplier_CrossMidnight(t *testing.T) {
	config := &PeakValleyPricingConfig{
		Enabled:  true,
		Timezone: "UTC",
		Schedules: []PeakValleySchedule{
			{
				Name:       "夜间低谷",
				StartHour:  22,
				EndHour:    2,
				Multiplier: 0.5,
				Priority:   1,
			},
		},
	}

	tests := []struct {
		name     string
		hour     int
		expected float64
	}{
		{"22:00 跨日时段开始", 22, 0.5},
		{"23:30 跨日时段中", 23, 0.5},
		{"00:30 跨日时段中（次日）", 0, 0.5},
		{"01:59 跨日时段结束前", 1, 0.5},
		{"02:00 跨日时段结束", 2, 1.0},
		{"21:59 跨日时段开始前", 21, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := time.Date(2026, 9, 8, tt.hour, 0, 0, 0, time.UTC)
			multiplier := GetPeakValleyMultiplier(config, testTime)
			require.Equal(t, tt.expected, multiplier)
		})
	}
}

func TestGetPeakValleyMultiplier_OverlappingSchedules(t *testing.T) {
	config := &PeakValleyPricingConfig{
		Enabled:  true,
		Timezone: "UTC",
		Schedules: []PeakValleySchedule{
			{
				Name:       "基础高峰",
				StartHour:  8,
				EndHour:    20,
				Multiplier: 1.5,
				Priority:   1,
			},
			{
				Name:       "超级高峰",
				StartHour:  12,
				EndHour:    14,
				Multiplier: 3.0,
				Priority:   2, // 优先级更高
			},
		},
	}

	tests := []struct {
		name     string
		hour     int
		expected float64
	}{
		{"09:00 基础高峰", 9, 1.5},
		{"12:00 超级高峰开始", 12, 3.0},
		{"13:30 超级高峰中", 13, 3.0},
		{"14:00 超级高峰结束", 14, 1.5},
		{"19:00 基础高峰", 19, 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := time.Date(2026, 9, 8, tt.hour, 0, 0, 0, time.UTC)
			multiplier := GetPeakValleyMultiplier(config, testTime)
			require.Equal(t, tt.expected, multiplier)
		})
	}
}

func TestGetPeakValleyMultiplier_TimezoneConversion(t *testing.T) {
	config := &PeakValleyPricingConfig{
		Enabled:  true,
		Timezone: "Asia/Shanghai", // 北京时间 UTC+8
		Schedules: []PeakValleySchedule{
			{
				Name:       "北京时间高峰",
				StartHour:  18, // 北京时间 18:00
				EndHour:    22, // 北京时间 22:00
				Multiplier: 2.0,
				Priority:   1,
			},
		},
	}

	// UTC 10:00 = 北京时间 18:00（高峰开始）
	utc10 := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	require.Equal(t, 2.0, GetPeakValleyMultiplier(config, utc10))

	// UTC 14:00 = 北京时间 22:00（高峰结束）
	utc14 := time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC)
	require.Equal(t, 1.0, GetPeakValleyMultiplier(config, utc14))

	// UTC 09:59 = 北京时间 17:59（高峰开始前）
	utc09 := time.Date(2026, 9, 8, 9, 59, 0, 0, time.UTC)
	require.Equal(t, 1.0, GetPeakValleyMultiplier(config, utc09))
}

func TestGetPeakValleyMultiplier_DeepseekCompatible(t *testing.T) {
	// 复刻 DeepSeek 官方峰谷规则
	config := &PeakValleyPricingConfig{
		Enabled:  true,
		Timezone: "UTC",
		Schedules: []PeakValleySchedule{
			{
				Name:       "高峰时段1",
				StartHour:  1,
				EndHour:    4,
				Multiplier: 2.0,
				Weekdays:   []int{1, 2, 3, 4, 5}, // 工作日
				Priority:   1,
			},
			{
				Name:       "高峰时段2",
				StartHour:  6,
				EndHour:    10,
				Multiplier: 2.0,
				Weekdays:   []int{1, 2, 3, 4, 5}, // 工作日
				Priority:   1,
			},
		},
	}

	// 2026-09-08 是周一（工作日）
	monday := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		hour     int
		expected float64
	}{
		{0, 1.0},   // 低谷
		{1, 2.0},   // 高峰1开始
		{3, 2.0},   // 高峰1中
		{4, 1.0},   // 高峰1结束
		{5, 1.0},   // 低谷
		{6, 2.0},   // 高峰2开始
		{9, 2.0},   // 高峰2中
		{10, 1.0},  // 高峰2结束
		{12, 1.0},  // 低谷
	}

	for _, tt := range tests {
		testTime := monday.Add(time.Duration(tt.hour) * time.Hour)
		multiplier := GetPeakValleyMultiplier(config, testTime)
		require.Equal(t, tt.expected, multiplier, "hour=%d", tt.hour)
	}

	// 2026-09-13 是周日（周末）
	sunday := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	for hour := 0; hour < 24; hour++ {
		testTime := sunday.Add(time.Duration(hour) * time.Hour)
		multiplier := GetPeakValleyMultiplier(config, testTime)
		require.Equal(t, 1.0, multiplier, "周末全天应为低谷 hour=%d", hour)
	}
}

func TestGetPeakValleyPeriodName(t *testing.T) {
	config := &PeakValleyPricingConfig{
		Enabled:  true,
		Timezone: "UTC",
		Schedules: []PeakValleySchedule{
			{
				Name:       "高峰时段",
				StartHour:  8,
				EndHour:    20,
				Multiplier: 2.0,
				Priority:   1,
			},
		},
	}

	peakTime := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	require.Equal(t, "高峰时段", GetPeakValleyPeriodName(config, peakTime))

	offPeakTime := time.Date(2026, 9, 8, 6, 0, 0, 0, time.UTC)
	require.Equal(t, "", GetPeakValleyPeriodName(config, offPeakTime))
}

func TestValidatePeakValleyConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *PeakValleyPricingConfig
		wantError bool
	}{
		{
			name:      "nil config",
			config:    nil,
			wantError: false,
		},
		{
			name: "disabled config",
			config: &PeakValleyPricingConfig{
				Enabled: false,
			},
			wantError: false,
		},
		{
			name: "valid config",
			config: &PeakValleyPricingConfig{
				Enabled:  true,
				Timezone: "UTC",
				Schedules: []PeakValleySchedule{
					{
						Name:       "高峰",
						StartHour:  8,
						EndHour:    20,
						Multiplier: 2.0,
						Priority:   1,
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid timezone",
			config: &PeakValleyPricingConfig{
				Enabled:  true,
				Timezone: "Invalid/Timezone",
				Schedules: []PeakValleySchedule{
					{Name: "高峰", StartHour: 8, EndHour: 20, Multiplier: 2.0},
				},
			},
			wantError: true,
		},
		{
			name: "empty name",
			config: &PeakValleyPricingConfig{
				Enabled:  true,
				Timezone: "UTC",
				Schedules: []PeakValleySchedule{
					{Name: "", StartHour: 8, EndHour: 20, Multiplier: 2.0},
				},
			},
			wantError: true,
		},
		{
			name: "invalid start hour",
			config: &PeakValleyPricingConfig{
				Enabled:  true,
				Timezone: "UTC",
				Schedules: []PeakValleySchedule{
					{Name: "高峰", StartHour: 25, EndHour: 20, Multiplier: 2.0},
				},
			},
			wantError: true,
		},
		{
			name: "invalid multiplier zero",
			config: &PeakValleyPricingConfig{
				Enabled:  true,
				Timezone: "UTC",
				Schedules: []PeakValleySchedule{
					{Name: "高峰", StartHour: 8, EndHour: 20, Multiplier: 0},
				},
			},
			wantError: true,
		},
		{
			name: "invalid multiplier too high",
			config: &PeakValleyPricingConfig{
				Enabled:  true,
				Timezone: "UTC",
				Schedules: []PeakValleySchedule{
					{Name: "高峰", StartHour: 8, EndHour: 20, Multiplier: 15.0},
				},
			},
			wantError: true,
		},
		{
			name: "invalid weekday",
			config: &PeakValleyPricingConfig{
				Enabled:  true,
				Timezone: "UTC",
				Schedules: []PeakValleySchedule{
					{Name: "高峰", StartHour: 8, EndHour: 20, Multiplier: 2.0, Weekdays: []int{1, 7}},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePeakValleyConfig(tt.config)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsWeekdayMatch(t *testing.T) {
	tests := []struct {
		name        string
		weekday     time.Weekday
		allowedDays []int
		expected    bool
	}{
		{"空数组全匹配", time.Monday, []int{}, true},
		{"周一匹配工作日", time.Monday, []int{1, 2, 3, 4, 5}, true},
		{"周六不匹配工作日", time.Saturday, []int{1, 2, 3, 4, 5}, false},
		{"周日匹配周末", time.Sunday, []int{0, 6}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWeekdayMatch(tt.weekday, tt.allowedDays)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTimeInRange(t *testing.T) {
	tests := []struct {
		name     string
		hour     int
		minute   int
		schedule PeakValleySchedule
		expected bool
	}{
		{
			name:   "正常时段开始",
			hour:   8,
			minute: 0,
			schedule: PeakValleySchedule{
				StartHour: 8, StartMinute: 0,
				EndHour: 20, EndMinute: 0,
			},
			expected: true,
		},
		{
			name:   "正常时段结束",
			hour:   20,
			minute: 0,
			schedule: PeakValleySchedule{
				StartHour: 8, StartMinute: 0,
				EndHour: 20, EndMinute: 0,
			},
			expected: false,
		},
		{
			name:   "跨日时段前半部分",
			hour:   23,
			minute: 0,
			schedule: PeakValleySchedule{
				StartHour: 22, StartMinute: 0,
				EndHour: 2, EndMinute: 0,
			},
			expected: true,
		},
		{
			name:   "跨日时段后半部分",
			hour:   1,
			minute: 0,
			schedule: PeakValleySchedule{
				StartHour: 22, StartMinute: 0,
				EndHour: 2, EndMinute: 0,
			},
			expected: true,
		},
		{
			name:   "跨日时段结束",
			hour:   2,
			minute: 0,
			schedule: PeakValleySchedule{
				StartHour: 22, StartMinute: 0,
				EndHour: 2, EndMinute: 0,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := time.Date(2026, 9, 8, tt.hour, tt.minute, 0, 0, time.UTC)
			result := isTimeInRange(testTime, &tt.schedule)
			require.Equal(t, tt.expected, result)
		})
	}
}
