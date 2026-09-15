//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 这两个常量刻意长得像真凭据：泄露断言靠在序列化结果里搜它们，
// 用「apikey」之类的泛化词搜不出「把整个 target 结构塞进响应」这类错误。
const (
	intelCheckStubBaseURL = "https://api.codexpro.example.com/v1"
	intelCheckStubAPIKey  = "sk-super-secret-token"
)

func intelCheckPublicFixture(t *testing.T, enabled bool) (*IntelCheckService, *intelCheckRepoStub) {
	t.Helper()

	cfg := DefaultIntelCheckSettings()
	cfg.Enabled = enabled
	cfg.DegradedRule = IntelCheckDegradedRule{FailStreak: 3, RecoverStreak: 2}
	raw, err := json.Marshal(&cfg)
	require.NoError(t, err)

	repo := &intelCheckRepoStub{
		enabledTargets: []*IntelCheckTarget{
			{
				ID: 1, Name: "满血组", Description: "官方直连", Model: "gpt-6-astra",
				ReasoningEffort: IntelCheckEffortHigh, RateLabel: "×1.0",
				BaseURL: intelCheckStubBaseURL, APIKey: intelCheckStubAPIKey,
				APIMode: IntelCheckAPIModeResponses, Enabled: true, CreatedBy: 9,
			},
			{
				ID: 2, Name: "疑似降智组", Model: "gpt-6-astra",
				ReasoningEffort: IntelCheckEffortHigh, RateLabel: "×0.3",
				BaseURL: intelCheckStubBaseURL, APIKey: intelCheckStubAPIKey,
				APIMode: IntelCheckAPIModeChatCompletions, Enabled: true, CreatedBy: 9,
			},
		},
		timelines: map[int64]map[string][]*IntelCheckTimelinePoint{
			1: {
				IntelCheckKindLogic:   intelCheckPoints(IntelCheckStatusPass, IntelCheckStatusPass, IntelCheckStatusPass),
				IntelCheckKindDrawing: intelCheckPoints(IntelCheckStatusPass, IntelCheckStatusFail),
			},
			// 升序（旧→新）三连失败：按 fail_streak=3 应判降智。
			2: {
				IntelCheckKindLogic: intelCheckPoints(
					IntelCheckStatusPass, IntelCheckStatusFail, IntelCheckStatusFail, IntelCheckStatusFail),
			},
		},
		counts: map[string]map[int64]map[string]int64{
			IntelCheckKindLogic: {
				1: {IntelCheckStatusPass: 10, IntelCheckStatusRequestError: 2},
				2: {IntelCheckStatusPass: 1, IntelCheckStatusFail: 9},
			},
			IntelCheckKindDrawing: {
				1: {IntelCheckStatusPass: 4, IntelCheckStatusFail: 4, IntelCheckStatusUnverified: 3},
				2: {IntelCheckStatusFail: 2, IntelCheckStatusRunning: 1},
			},
		},
		artworks: map[int64]int64{1: 501},
		rounds: []*IntelCheckRound{
			{ID: 77, Seq: 1284, StartedAt: time.Now().Add(-10 * time.Minute), TriggerSource: IntelCheckTriggerCron},
		},
	}

	return NewIntelCheckService(repo, &intelCheckSettingStoreStub{value: string(raw)}, nil), repo
}

func intelCheckPoints(statuses ...string) []*IntelCheckTimelinePoint {
	points := make([]*IntelCheckTimelinePoint, 0, len(statuses))
	for i, status := range statuses {
		points = append(points, &IntelCheckTimelinePoint{
			ResultID:  int64(i + 1),
			Status:    status,
			CheckedAt: time.Now().Add(time.Duration(i-len(statuses)) * time.Minute),
		})
	}
	return points
}

func TestIntelCheckPublicOverview开关关闭时不给数据(t *testing.T) {
	svc, _ := intelCheckPublicFixture(t, false)

	_, err := svc.PublicOverview(context.Background())
	require.ErrorIs(t, err, ErrIntelCheckDisabled)
}

func TestIntelCheckPublicOverview未配置分组时返回空卡片(t *testing.T) {
	// 管理员开了开关但还没建分组，页面应正常显示说明文案。
	svc, repo := intelCheckPublicFixture(t, true)
	repo.enabledTargets = nil

	out, err := svc.PublicOverview(context.Background())
	require.NoError(t, err)
	require.NotNil(t, out.Groups, "必须是空切片，序列化成 [] 而非 null")
	require.Empty(t, out.Groups)
	require.Equal(t, IntelCheckDefaultIntroTitle, out.IntroTitle)
	// 没有分组也应告知最近一轮的信息（此时轮次可能来自已被删除的分组）。
	require.EqualValues(t, 1284, out.Summary.LatestRoundSeq)
}

func TestIntelCheckPublicOverview聚合(t *testing.T) {
	svc, _ := intelCheckPublicFixture(t, true)

	out, err := svc.PublicOverview(context.Background())
	require.NoError(t, err)

	require.Len(t, out.Groups, 2)
	require.Equal(t, 30, out.IntervalMinutes)
	require.Equal(t, 48, out.TimelinePoints)
	// 判定规则随数据一起公开，否则色块只是颜色，用户无从复核。
	require.Equal(t, 3, out.DegradedRule.FailStreak)
	require.Equal(t, 2, out.DegradedRule.RecoverStreak)

	first, second := out.Groups[0], out.Groups[1]
	require.EqualValues(t, 1, first.ID, "分组顺序应沿用仓储的 sort_order 排序")
	require.Equal(t, "满血组", first.Name)
	require.Equal(t, "gpt-6-astra", first.Model)
	require.Equal(t, "×1.0", first.RateLabel)

	require.Equal(t, IntelCheckStateNormal, first.State)
	require.Equal(t, IntelCheckStateDegraded, second.State, "连续三次失败应判疑似降智")

	require.Len(t, first.LogicTimeline, 3)
	require.Len(t, first.DrawingTimeline, 2)
	require.EqualValues(t, 501, first.LatestDrawingResultID)

	// 没有绘图时间线的分组要拿到空切片，而不是 nil。
	require.NotNil(t, second.DrawingTimeline)
	require.Empty(t, second.DrawingTimeline)
	require.Zero(t, second.LatestDrawingResultID, "没有画作产出时为 0")

	require.EqualValues(t, 10, first.LogicStats24h.Pass)
	require.EqualValues(t, 2, first.LogicStats24h.Error)
	require.EqualValues(t, 3, first.DrawingStats24h.Unverified)
	require.InDelta(t, 1.0, first.LogicStats24h.PassRate, 1e-9)
	require.True(t, first.LogicStats24h.HasData)

	require.Equal(t, 2, out.Summary.TotalGroups)
	require.Equal(t, 1, out.Summary.NormalGroups)
	require.Equal(t, 1, out.Summary.DegradedGroups)
	require.Zero(t, out.Summary.UnknownGroups)
}

func TestIntelCheckPublicOverview汇总口径(t *testing.T) {
	svc, _ := intelCheckPublicFixture(t, true)

	out, err := svc.PublicOverview(context.Background())
	require.NoError(t, err)

	// running 不计入任何一项：它只是尚未出结论的占位行（桩数据里给了 1 条）。
	require.EqualValues(t, 15, out.Summary.Stats24h.Pass)
	require.EqualValues(t, 15, out.Summary.Stats24h.Fail)
	require.EqualValues(t, 2, out.Summary.Stats24h.Error, "request_error 单独计，不进通过率分母")
	require.EqualValues(t, 3, out.Summary.Stats24h.Unverified, "unverified 单独计，不进通过率分母")

	// 先累加分子分母再相除得 0.5；若改成逐组求平均会得 0.4，
	// 样本只有 2 条的分组会和样本 12 条的分组等权，这正是要避免的口径。
	require.InDelta(t, 0.5, out.Summary.Stats24h.PassRate, 1e-9)
	require.True(t, out.Summary.Stats24h.HasData)
}

func TestIntelCheckPublicOverview统计按题型分别查询(t *testing.T) {
	// CountByStatusSince 的 kind 会直接落到 SQL 的枚举列上，
	// 传空串不是「不过滤」而是一个非法枚举值，Postgres 会直接报错。
	svc, repo := intelCheckPublicFixture(t, true)

	_, err := svc.PublicOverview(context.Background())
	require.NoError(t, err)

	require.Equal(t, []string{IntelCheckKindLogic, IntelCheckKindDrawing}, repo.countKinds)
	require.WithinDuration(t, time.Now().Add(-24*time.Hour), repo.countSince, time.Minute,
		"统计窗口固定 24 小时，不随 timeline_points 变化")
}

func TestIntelCheckPublicOverview下次检测时刻(t *testing.T) {
	svc, repo := intelCheckPublicFixture(t, true)

	out, err := svc.PublicOverview(context.Background())
	require.NoError(t, err)

	started := repo.rounds[0].StartedAt
	require.NotNil(t, out.Summary.LastCheckedAt)
	require.WithinDuration(t, started, *out.Summary.LastCheckedAt, time.Second)
	require.NotNil(t, out.Summary.NextCheckAt)
	// 锚在上一轮的开始时刻而非结束时刻：结束时刻随上游耗时波动，用它推算会逐轮漂移。
	require.WithinDuration(t, started.Add(30*time.Minute), *out.Summary.NextCheckAt, time.Second)
	require.False(t, out.GeneratedAt.IsZero(), "前端以服务端时间为基准做倒计时")
}

func TestIntelCheckPublicOverview尚未跑过任何轮次(t *testing.T) {
	svc, repo := intelCheckPublicFixture(t, true)
	repo.rounds = nil

	out, err := svc.PublicOverview(context.Background())
	require.NoError(t, err)

	// 留 nil 让前端显示「尚未开始检测」；塞当前时间会让页面看起来刚检测过。
	require.Nil(t, out.Summary.LastCheckedAt)
	require.Nil(t, out.Summary.NextCheckAt)
	require.Zero(t, out.Summary.LatestRoundSeq)
}

func TestIntelCheckPublicOverview存储故障不得被吞成空页面(t *testing.T) {
	boom := errors.New("connection refused")

	cases := map[string]func(*intelCheckRepoStub){
		"分组查询失败":  func(r *intelCheckRepoStub) { r.listEnabledErr = boom },
		"时间线查询失败": func(r *intelCheckRepoStub) { r.timelineErr = boom },
		"统计查询失败":  func(r *intelCheckRepoStub) { r.countErr = boom },
		"轮次查询失败":  func(r *intelCheckRepoStub) { r.roundsErr = boom },
	}
	for name, inject := range cases {
		t.Run(name, func(t *testing.T) {
			svc, repo := intelCheckPublicFixture(t, true)
			inject(repo)

			_, err := svc.PublicOverview(context.Background())
			require.ErrorIs(t, err, boom)
		})
	}
}

// TestIntelCheckPublicOverview不得泄露上游信息 是本功能的硬约束：
// 公开页承诺不暴露上游地址与凭据，这里直接在序列化结果上搜。
// 断言放在 JSON 而不是逐字段检查，是因为真正的事故形态是「有人把领域模型
// 整个塞进了响应」，那种改动逐字段断言恰好检查不到。
func TestIntelCheckPublicOverview不得泄露上游信息(t *testing.T) {
	svc, _ := intelCheckPublicFixture(t, true)

	out, err := svc.PublicOverview(context.Background())
	require.NoError(t, err)

	encoded, err := json.Marshal(out)
	require.NoError(t, err)
	payload := string(encoded)

	for _, forbidden := range []string{
		intelCheckStubBaseURL, intelCheckStubAPIKey,
		"codexpro", "base_url", "api_key", "created_by",
		IntelCheckAPIModeResponses, IntelCheckAPIModeChatCompletions,
	} {
		require.NotContains(t, payload, forbidden)
	}
	// 该公开的必须在，否则「什么都不返回」也能让上面的断言通过。
	require.Contains(t, payload, "满血组")
	require.Contains(t, payload, "gpt-6-astra")
}

func intelCheckDetailFixture(t *testing.T) (*IntelCheckService, *intelCheckRepoStub) {
	t.Helper()

	svc, repo := intelCheckPublicFixture(t, true)
	repo.targets = map[int64]*IntelCheckTarget{1: repo.enabledTargets[0]}
	logicID, drawingID := int64(10), int64(20)
	repo.roundsByID = map[int64]*IntelCheckRound{
		77: {ID: 77, Seq: 1284, LogicQuestionID: &logicID, DrawingQuestionID: &drawingID},
		// 题目已被删除，外键置空（ON DELETE SET NULL）。
		78: {ID: 78, Seq: 1285},
	}
	repo.questions = map[int64]*IntelCheckQuestion{
		10: {ID: 10, Kind: IntelCheckKindLogic, Title: "三段论", ExpectedAnswer: "42", MatchMode: IntelCheckMatchNumeric},
		20: {ID: 20, Kind: IntelCheckKindDrawing, Title: "画一只骑车的鹈鹕", MatchMode: IntelCheckMatchExact},
	}
	latency := 4200
	repo.results = map[int64]*IntelCheckResult{
		100: {
			ID: 100, RoundID: 77, TargetID: 1, Kind: IntelCheckKindLogic, Status: IntelCheckStatusFail,
			PromptSnapshot: "题面", RawReply: "推理过程……答案是 41", ExtractedAnswer: "41",
			LatencyMs: &latency, CheckedAt: time.Now(),
			// 管理端才看得到的排障信息，绝不能出现在公开详情里。
			ErrorMessage: "upstream rate limited from api.codexpro.example.com",
		},
		200: {
			ID: 200, RoundID: 77, TargetID: 1, Kind: IntelCheckKindDrawing, Status: IntelCheckStatusPass,
			PromptSnapshot: "画图题面", RawReply: "```html…```", HTMLOutput: "<svg><title>鹈鹕</title></svg>",
			JudgeDetail: map[string]any{"gate": "passed", "score": 88.0}, CheckedAt: time.Now(),
		},
		300: {ID: 300, RoundID: 78, TargetID: 1, Kind: IntelCheckKindLogic, Status: IntelCheckStatusPass},
		400: {ID: 400, RoundID: 999, TargetID: 1, Kind: IntelCheckKindLogic, Status: IntelCheckStatusRequestError},
	}
	return svc, repo
}

func TestIntelCheckPublicResultDetail逻辑题(t *testing.T) {
	svc, _ := intelCheckDetailFixture(t)

	out, err := svc.PublicResultDetail(context.Background(), 100)
	require.NoError(t, err)

	require.EqualValues(t, 1284, out.RoundSeq)
	require.Equal(t, "满血组", out.TargetName)
	require.Equal(t, "gpt-6-astra", out.Model)
	require.Equal(t, IntelCheckEffortHigh, out.ReasoningEffort)
	require.Equal(t, "三段论", out.QuestionTitle)
	// 期望答案、匹配模式、提取答案三者齐全，用户才能自行复核这次判定。
	require.Equal(t, "42", out.ExpectedAnswer)
	require.Equal(t, IntelCheckMatchNumeric, out.MatchMode)
	require.Equal(t, "41", out.ExtractedAnswer)
	require.Equal(t, "本次检测未通过", out.StatusNote)
	require.EqualValues(t, 4200, *out.LatencyMs)
}

func TestIntelCheckPublicResultDetail绘图题(t *testing.T) {
	svc, _ := intelCheckDetailFixture(t)

	out, err := svc.PublicResultDetail(context.Background(), 200)
	require.NoError(t, err)

	require.Equal(t, "画一只骑车的鹈鹕", out.QuestionTitle)
	require.Contains(t, out.HTMLOutput, "<svg>")
	require.NotEmpty(t, out.JudgeDetail)
	// 绘图题带上期望答案会让弹窗多出一个永远空着的栏位，看起来像数据丢了。
	require.Empty(t, out.ExpectedAnswer)
	require.Empty(t, out.MatchMode)
	require.Equal(t, "本次检测通过", out.StatusNote)
}

func TestIntelCheckPublicResultDetail不得泄露上游报错原文(t *testing.T) {
	// 用户可见的详情里，请求类失败只说「未能完成」，
	// 上游域名与 HTTP 状态码留给管理端接口。
	svc, _ := intelCheckDetailFixture(t)

	out, err := svc.PublicResultDetail(context.Background(), 100)
	require.NoError(t, err)

	encoded, err := json.Marshal(out)
	require.NoError(t, err)
	payload := string(encoded)

	// 禁用词只取原文里的字母串，不拿状态码之类的纯数字：
	// 时间戳与自增 id 里随时可能凑出同样的数字，那种失败与泄露无关。
	for _, forbidden := range []string{
		"codexpro", "upstream", "rate limited", "error_message", intelCheckStubAPIKey,
	} {
		require.NotContains(t, strings.ToLower(payload), strings.ToLower(forbidden))
	}
	// 中性文案必须已经就位，否则「整个响应为空」也能让上面的断言通过。
	require.Contains(t, payload, "本次检测未通过")
}

func TestIntelCheckPublicResultDetail题目或轮次缺失仍可展示(t *testing.T) {
	svc, _ := intelCheckDetailFixture(t)

	t.Run("题目已被删除", func(t *testing.T) {
		out, err := svc.PublicResultDetail(context.Background(), 300)
		require.NoError(t, err)
		require.EqualValues(t, 1285, out.RoundSeq)
		require.Empty(t, out.QuestionTitle)
		require.Empty(t, out.ExpectedAnswer)
	})

	t.Run("轮次已被清理", func(t *testing.T) {
		// 保留期清理会物理删除过期轮次，届时明细本身仍在。
		out, err := svc.PublicResultDetail(context.Background(), 400)
		require.NoError(t, err)
		require.Zero(t, out.RoundSeq)
		require.Equal(t, "本次请求未能完成，未测出结论（不计入连续失败）", out.StatusNote)
	})
}

func TestIntelCheckPublicResultDetail边界(t *testing.T) {
	t.Run("开关关闭", func(t *testing.T) {
		svc, _ := intelCheckPublicFixture(t, false)
		_, err := svc.PublicResultDetail(context.Background(), 100)
		require.ErrorIs(t, err, ErrIntelCheckDisabled)
	})

	t.Run("记录不存在", func(t *testing.T) {
		svc, _ := intelCheckDetailFixture(t)
		_, err := svc.PublicResultDetail(context.Background(), 999999)
		require.ErrorIs(t, err, ErrIntelCheckResultNotFound)
	})
}
