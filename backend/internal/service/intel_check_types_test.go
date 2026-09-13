//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDefaultIntelCheckSettings 默认配置本身必须是合法配置。
// Normalize 后若发生任何变化，说明默认值与允许区间脱节了。
func TestDefaultIntelCheckSettings(t *testing.T) {
	settings := DefaultIntelCheckSettings()

	// 功能默认关闭：配置未完成前不应自动对外开放公开页。
	require.False(t, settings.Enabled, "智力检测默认必须关闭")

	normalized := settings
	normalized.Normalize()
	require.Equal(t, settings, normalized, "默认配置经 Normalize 后不应发生变化")
}

func TestIntelCheckSettingsNormalize(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*IntelCheckSettings)
		assert func(*testing.T, IntelCheckSettings)
	}{
		{
			name:   "零值回落默认值",
			mutate: func(s *IntelCheckSettings) { *s = IntelCheckSettings{} },
			assert: func(t *testing.T, s IntelCheckSettings) {
				require.Equal(t, intelCheckDefaultIntervalMinutes, s.IntervalMinutes)
				require.Equal(t, intelCheckDefaultTimeoutSeconds, s.RequestTimeoutSeconds)
				require.Equal(t, intelCheckDefaultConcurrency, s.Concurrency)
				require.Equal(t, intelCheckDefaultFailStreak, s.DegradedRule.FailStreak)
				require.Equal(t, intelCheckDefaultRecoverStreak, s.DegradedRule.RecoverStreak)
				require.Equal(t, intelCheckDefaultPassScore, s.DrawingJudge.PassScore)
				require.Equal(t, intelCheckDefaultTimelinePoints, s.TimelinePoints)
				require.Equal(t, intelCheckDefaultRetentionDays, s.RetentionDays)
			},
		},
		{
			name: "负值回落默认值",
			mutate: func(s *IntelCheckSettings) {
				s.IntervalMinutes = -10
				s.Concurrency = -1
				s.DegradedRule.FailStreak = -3
			},
			assert: func(t *testing.T, s IntelCheckSettings) {
				require.Equal(t, intelCheckDefaultIntervalMinutes, s.IntervalMinutes)
				require.Equal(t, intelCheckDefaultConcurrency, s.Concurrency)
				require.Equal(t, intelCheckDefaultFailStreak, s.DegradedRule.FailStreak)
			},
		},
		{
			name: "低于下限收敛到下限",
			mutate: func(s *IntelCheckSettings) {
				// 检测间隔过短会持续占用上游配额，必须抬到下限。
				s.IntervalMinutes = 1
				s.RequestTimeoutSeconds = 5
				s.TimelinePoints = 3
			},
			assert: func(t *testing.T, s IntelCheckSettings) {
				require.Equal(t, intelCheckMinIntervalMinutes, s.IntervalMinutes)
				require.Equal(t, intelCheckMinTimeoutSeconds, s.RequestTimeoutSeconds)
				require.Equal(t, intelCheckMinTimelinePoints, s.TimelinePoints)
			},
		},
		{
			name: "超过上限收敛到上限",
			mutate: func(s *IntelCheckSettings) {
				s.IntervalMinutes = 99999
				s.RequestTimeoutSeconds = 99999
				s.Concurrency = 9999
				s.DegradedRule.FailStreak = 9999
				s.DegradedRule.RecoverStreak = 9999
				s.DrawingJudge.PassScore = 500
				s.TimelinePoints = 99999
				s.RetentionDays = 99999
			},
			assert: func(t *testing.T, s IntelCheckSettings) {
				require.Equal(t, intelCheckMaxIntervalMinutes, s.IntervalMinutes)
				require.Equal(t, intelCheckMaxTimeoutSeconds, s.RequestTimeoutSeconds)
				require.Equal(t, intelCheckMaxConcurrency, s.Concurrency)
				require.Equal(t, intelCheckMaxStreak, s.DegradedRule.FailStreak)
				require.Equal(t, intelCheckMaxStreak, s.DegradedRule.RecoverStreak)
				require.Equal(t, 100, s.DrawingJudge.PassScore)
				require.Equal(t, intelCheckMaxTimelinePoints, s.TimelinePoints)
				require.Equal(t, intelCheckMaxRetentionDays, s.RetentionDays)
			},
		},
		{
			name: "合法配置保持不变",
			mutate: func(s *IntelCheckSettings) {
				s.IntervalMinutes = 15
				s.RequestTimeoutSeconds = 120
				s.Concurrency = 8
				s.DegradedRule = IntelCheckDegradedRule{FailStreak: 5, RecoverStreak: 3}
				s.DrawingJudge.PassScore = 90
				s.TimelinePoints = 96
				s.RetentionDays = 7
			},
			assert: func(t *testing.T, s IntelCheckSettings) {
				require.Equal(t, 15, s.IntervalMinutes)
				require.Equal(t, 120, s.RequestTimeoutSeconds)
				require.Equal(t, 8, s.Concurrency)
				require.Equal(t, 5, s.DegradedRule.FailStreak)
				require.Equal(t, 3, s.DegradedRule.RecoverStreak)
				require.Equal(t, 90, s.DrawingJudge.PassScore)
				require.Equal(t, 96, s.TimelinePoints)
				require.Equal(t, 7, s.RetentionDays)
			},
		},
		{
			name: "Normalize 不得篡改开关与文案",
			mutate: func(s *IntelCheckSettings) {
				s.Enabled = true
				s.IntroTitle = "模型真的是满血在跑吗?"
				s.IntroText = "每 30 分钟自动检测一次"
				s.DrawingJudge.Model = "gpt-6-astra"
				s.DrawingJudge.TargetID = 7
			},
			assert: func(t *testing.T, s IntelCheckSettings) {
				require.True(t, s.Enabled)
				require.Equal(t, "模型真的是满血在跑吗?", s.IntroTitle)
				require.Equal(t, "每 30 分钟自动检测一次", s.IntroText)
				require.Equal(t, "gpt-6-astra", s.DrawingJudge.Model)
				require.EqualValues(t, 7, s.DrawingJudge.TargetID)
			},
		},
		{
			// bool 零值即 false，若归一化套用「零值回落默认」，
			// 管理员关掉的开关会在下一次读取时自己打开。
			// 这是本功能最不能出的错，与上一条的「开启态不被关掉」互为两个方向。
			name:   "关闭态不得被回落成开启",
			mutate: func(s *IntelCheckSettings) { s.Enabled = false },
			assert: func(t *testing.T, s IntelCheckSettings) {
				require.False(t, s.Enabled)
			},
		},
		{
			name: "自定义文案只去首尾空白，内容不动",
			mutate: func(s *IntelCheckSettings) {
				s.IntroTitle = "  自定义标题  "
				s.IntroText = "\n自定义说明\n"
			},
			assert: func(t *testing.T, s IntelCheckSettings) {
				require.Equal(t, "自定义标题", s.IntroTitle)
				require.Equal(t, "自定义说明", s.IntroText)
			},
		},
		{
			// 为空表示「未配置」，绝不能回落成某个猜测的模型名——
			// 那会让管理员以为配好了，而实际调用的是一个不存在的模型。
			name: "评审模型名为空时不猜测",
			mutate: func(s *IntelCheckSettings) {
				s.DrawingJudge.Model = "   "
				s.DrawingJudge.TargetID = -7
			},
			assert: func(t *testing.T, s IntelCheckSettings) {
				require.Equal(t, "", s.DrawingJudge.Model)
				require.EqualValues(t, 0, s.DrawingJudge.TargetID, "负的分组 id 归零表示未配置")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := DefaultIntelCheckSettings()
			tt.mutate(&settings)
			settings.Normalize()
			tt.assert(t, settings)
		})
	}
}

// TestIntelCheckSettingsNormalizeIdempotent 归一化必须幂等。
// 配置每次读取都会过一遍 Normalize，不幂等会导致数值随读取次数漂移。
func TestIntelCheckSettingsNormalizeIdempotent(t *testing.T) {
	settings := IntelCheckSettings{
		IntervalMinutes:       99999,
		RequestTimeoutSeconds: 1,
		Concurrency:           0,
		DegradedRule:          IntelCheckDegradedRule{FailStreak: -1, RecoverStreak: 9999},
		DrawingJudge:          IntelCheckDrawingJudge{PassScore: 500},
		TimelinePoints:        3,
		RetentionDays:         -5,
	}

	settings.Normalize()
	once := settings

	settings.Normalize()
	require.Equal(t, once, settings, "二次 Normalize 不应再改变任何字段")
}

// TestIntelCheckSettingsJSONRoundTrip settings 表按整块 JSON 存取，
// 字段名一旦改动会让线上已存配置静默丢失，这里把 json tag 钉死。
func TestIntelCheckSettingsJSONRoundTrip(t *testing.T) {
	raw := `{
		"enabled": true,
		"interval_minutes": 30,
		"request_timeout_seconds": 300,
		"concurrency": 4,
		"degraded_rule": {"fail_streak": 3, "recover_streak": 2},
		"drawing_judge": {"target_id": 1, "model": "gpt-6-astra", "reasoning_effort": "high", "pass_score": 80},
		"timeline_points": 48,
		"retention_days": 30,
		"intro_title": "标题",
		"intro_text": "正文"
	}`

	var settings IntelCheckSettings
	require.NoError(t, json.Unmarshal([]byte(raw), &settings))

	require.True(t, settings.Enabled)
	require.Equal(t, 30, settings.IntervalMinutes)
	require.Equal(t, 300, settings.RequestTimeoutSeconds)
	require.Equal(t, 4, settings.Concurrency)
	require.Equal(t, 3, settings.DegradedRule.FailStreak)
	require.Equal(t, 2, settings.DegradedRule.RecoverStreak)
	require.EqualValues(t, 1, settings.DrawingJudge.TargetID)
	require.Equal(t, "gpt-6-astra", settings.DrawingJudge.Model)
	require.Equal(t, "high", settings.DrawingJudge.ReasoningEffort)
	require.Equal(t, 80, settings.DrawingJudge.PassScore)
	require.Equal(t, 48, settings.TimelinePoints)
	require.Equal(t, 30, settings.RetentionDays)
	require.Equal(t, "标题", settings.IntroTitle)
	require.Equal(t, "正文", settings.IntroText)

	// 再序列化回去必须能被自己读回，保证读改写链路无损。
	encoded, err := json.Marshal(settings)
	require.NoError(t, err)

	var again IntelCheckSettings
	require.NoError(t, json.Unmarshal(encoded, &again))
	require.Equal(t, settings, again)
}
