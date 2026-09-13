package service

import "strings"

// 模型智力检测：题目类型。
const (
	IntelCheckKindLogic   = "logic"
	IntelCheckKindDrawing = "drawing"
)

// 模型智力检测：单次检测结果状态。
// 与前端色块一一对应：通过（绿）/ 失败（红）/ 请求失败（黄）/ 检测中（浅蓝）。
const (
	IntelCheckStatusPass         = "pass"
	IntelCheckStatusFail         = "fail"
	IntelCheckStatusRequestError = "request_error"
	IntelCheckStatusRunning      = "running"
)

// 模型智力检测：逻辑题答案匹配模式。
const (
	IntelCheckMatchExact    = "exact"
	IntelCheckMatchNumeric  = "numeric"
	IntelCheckMatchContains = "contains"
	IntelCheckMatchRegex    = "regex"
)

// 模型智力检测：上游请求协议。
const (
	IntelCheckAPIModeResponses       = "responses"
	IntelCheckAPIModeChatCompletions = "chat_completions"
)

// 模型智力检测：推理等级。
//
// 与 api_mode、kind、status 不同，intel_check_targets.reasoning_effort 在迁移里
// 只是一个不带 CHECK 约束的 VARCHAR(32)，合法值完全靠 NormalizeIntelCheckReasoningEffort
// 把关——写库前必须过那个函数，否则非法等级会一路存进去。
const (
	IntelCheckEffortLow    = "low"
	IntelCheckEffortMedium = "medium"
	IntelCheckEffortHigh   = "high"
	IntelCheckEffortXHigh  = "xhigh"
)

// 模型智力检测：轮次触发来源。
// 取值保留 cron 字面量（读作「自动定时触发」），尽管调度已改用 time.Ticker：
// 该值写在迁移的 CHECK 约束里，迁移落库后受 SHA256 校验保护、不可编辑。
const (
	IntelCheckTriggerCron   = "cron"
	IntelCheckTriggerManual = "manual"
)

// 模型智力检测：对外展示的分组智力状态。
const (
	IntelCheckStateNormal   = "normal"
	IntelCheckStateDegraded = "degraded"
	IntelCheckStateUnknown  = "unknown"
)

// SettingKeyIntelCheckSettings 智力检测配置在 settings 表中的 key，整块 JSON 存储。
const SettingKeyIntelCheckSettings = "intel_check_settings"

// IntelCheckDegradedRule 降智判定规则。
type IntelCheckDegradedRule struct {
	// FailStreak 连续失败多少次判为疑似降智。
	FailStreak int `json:"fail_streak"`
	// RecoverStreak 连续通过多少次恢复为正常。
	RecoverStreak int `json:"recover_streak"`
}

// IntelCheckDrawingJudge 绘图题源码评审所用的模型配置。
// TargetID 复用受检分组的 base_url 与 api_key，避免再维护一套凭据。
type IntelCheckDrawingJudge struct {
	TargetID        int64  `json:"target_id"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	// PassScore 评审通过分数线（0-100）。
	PassScore int `json:"pass_score"`
}

// IntelCheckSettings 智力检测的全部可配置项。
type IntelCheckSettings struct {
	Enabled               bool                   `json:"enabled"`
	IntervalMinutes       int                    `json:"interval_minutes"`
	RequestTimeoutSeconds int                    `json:"request_timeout_seconds"`
	Concurrency           int                    `json:"concurrency"`
	DegradedRule          IntelCheckDegradedRule `json:"degraded_rule"`
	DrawingJudge          IntelCheckDrawingJudge `json:"drawing_judge"`
	TimelinePoints        int                    `json:"timeline_points"`
	RetentionDays         int                    `json:"retention_days"`
	IntroTitle            string                 `json:"intro_title"`
	IntroText             string                 `json:"intro_text"`
}

// 智力检测配置的默认值与取值区间。
const (
	intelCheckDefaultIntervalMinutes = 30
	intelCheckMinIntervalMinutes     = 5
	intelCheckMaxIntervalMinutes     = 24 * 60

	intelCheckDefaultTimeoutSeconds = 300
	intelCheckMinTimeoutSeconds     = 30
	intelCheckMaxTimeoutSeconds     = 900

	intelCheckDefaultConcurrency = 4
	intelCheckMaxConcurrency     = 32

	intelCheckDefaultFailStreak    = 3
	intelCheckDefaultRecoverStreak = 2
	intelCheckMaxStreak            = 20

	intelCheckDefaultPassScore = 80

	intelCheckDefaultTimelinePoints = 48
	intelCheckMinTimelinePoints     = 12
	intelCheckMaxTimelinePoints     = 200

	intelCheckDefaultRetentionDays = 30
	intelCheckMaxRetentionDays     = 365
)

// 公开页默认文案。管理员留空时回落到这里，保证页面不会出现空标题。
const (
	IntelCheckDefaultIntroTitle = "模型真的是满血在跑吗？"
	IntelCheckDefaultIntroText  = "每隔一段时间，我们会用与 Codex CLI 完全一致的请求方式，" +
		"向下列分组各发一道逻辑题和一道绘图题，并把每一次的原始回复与判定过程如实公开。" +
		"绿色代表通过，红色代表未通过，黄色代表请求失败。点击任意色块可查看那一次的完整细节。"
)

// DefaultIntelCheckSettings 返回默认配置。
// Enabled 默认为 false：功能对外可见，未配置完成前不应自动开放。
func DefaultIntelCheckSettings() IntelCheckSettings {
	return IntelCheckSettings{
		Enabled:               false,
		IntervalMinutes:       intelCheckDefaultIntervalMinutes,
		RequestTimeoutSeconds: intelCheckDefaultTimeoutSeconds,
		Concurrency:           intelCheckDefaultConcurrency,
		DegradedRule: IntelCheckDegradedRule{
			FailStreak:    intelCheckDefaultFailStreak,
			RecoverStreak: intelCheckDefaultRecoverStreak,
		},
		DrawingJudge: IntelCheckDrawingJudge{
			// TargetID 与 Model 留零值表示「未配置」，不猜测默认模型名：
			// 猜错会让评审悄悄用错模型，打出的分数无从解释。
			ReasoningEffort: IntelCheckEffortHigh,
			PassScore:       intelCheckDefaultPassScore,
		},
		TimelinePoints: intelCheckDefaultTimelinePoints,
		RetentionDays:  intelCheckDefaultRetentionDays,
		IntroTitle:     IntelCheckDefaultIntroTitle,
		IntroText:      IntelCheckDefaultIntroText,
	}
}

// Normalize 将越界或缺省的配置项收敛到合法区间，便于直接用于调度与判定。
//
// 该方法是幂等的（clamp 与 trim 都是幂等操作），只做「补齐与收敛」，不做「校验」：
// 它用在读路径上，读到脏数据时必须仍能渲染页面；需要让管理员看到报错的问题
// 由 ValidateIntelCheckSettings 负责。
//
// 两处刻意不动：
//   - Enabled 不参与任何回落。bool 的零值就是 false，若套用「零值回落默认」，
//     管理员关掉的开关会在下一次读取时自己打开。
//   - 非空文案原样保留，只去首尾空白；仅在为空时回落默认文案。
func (s *IntelCheckSettings) Normalize() {
	s.IntervalMinutes = clampIntelCheckInt(s.IntervalMinutes, intelCheckMinIntervalMinutes, intelCheckMaxIntervalMinutes, intelCheckDefaultIntervalMinutes)
	s.RequestTimeoutSeconds = clampIntelCheckInt(s.RequestTimeoutSeconds, intelCheckMinTimeoutSeconds, intelCheckMaxTimeoutSeconds, intelCheckDefaultTimeoutSeconds)
	s.Concurrency = clampIntelCheckInt(s.Concurrency, 1, intelCheckMaxConcurrency, intelCheckDefaultConcurrency)
	s.DegradedRule.FailStreak = clampIntelCheckInt(s.DegradedRule.FailStreak, 1, intelCheckMaxStreak, intelCheckDefaultFailStreak)
	s.DegradedRule.RecoverStreak = clampIntelCheckInt(s.DegradedRule.RecoverStreak, 1, intelCheckMaxStreak, intelCheckDefaultRecoverStreak)
	s.DrawingJudge.PassScore = clampIntelCheckInt(s.DrawingJudge.PassScore, 1, 100, intelCheckDefaultPassScore)
	s.TimelinePoints = clampIntelCheckInt(s.TimelinePoints, intelCheckMinTimelinePoints, intelCheckMaxTimelinePoints, intelCheckDefaultTimelinePoints)
	s.RetentionDays = clampIntelCheckInt(s.RetentionDays, 1, intelCheckMaxRetentionDays, intelCheckDefaultRetentionDays)

	if s.DrawingJudge.TargetID < 0 {
		s.DrawingJudge.TargetID = 0
	}
	// Model 为空保持为空（表示未配置），同 DefaultIntelCheckSettings 的理由。
	s.DrawingJudge.Model = strings.TrimSpace(s.DrawingJudge.Model)
	s.DrawingJudge.ReasoningEffort = NormalizeIntelCheckReasoningEffort(s.DrawingJudge.ReasoningEffort)

	if s.IntroTitle = strings.TrimSpace(s.IntroTitle); s.IntroTitle == "" {
		s.IntroTitle = IntelCheckDefaultIntroTitle
	}
	if s.IntroText = strings.TrimSpace(s.IntroText); s.IntroText == "" {
		s.IntroText = IntelCheckDefaultIntroText
	}
}

// clampIntelCheckInt 非正数取默认值，越界则收敛到边界。
func clampIntelCheckInt(value, minValue, maxValue, fallback int) int {
	if value <= 0 {
		value = fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
