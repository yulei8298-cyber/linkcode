package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// intelCheckStatsWindow 汇总统计的时间窗，对应页面上的「24 小时通过率」。
//
// 不复用 timeline_points 反推窗口：48 格只在 interval=30 分钟时恰好等于 24 小时，
// 管理员把间隔改成 5 分钟后，同样的 48 格只覆盖 4 小时，而页面文案仍写着 24 小时。
const intelCheckStatsWindow = 24 * time.Hour

// PublicOverview 组装公开页（未登录可访问）所需的全部数据。
//
// 开关关闭时返回 ErrIntelCheckDisabled（404 语义），不返回空数据：
// 空数据会让前端渲染出一个「一切正常但没有分组」的页面，比 404 更容易误导。
func (s *IntelCheckService) PublicOverview(ctx context.Context) (*IntelCheckPublicOverview, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("智力检测服务未初始化")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, ErrIntelCheckDisabled
	}

	now := time.Now()
	out := &IntelCheckPublicOverview{
		IntroTitle:      cfg.IntroTitle,
		IntroText:       cfg.IntroText,
		IntervalMinutes: cfg.IntervalMinutes,
		TimelinePoints:  cfg.TimelinePoints,
		DegradedRule:    cfg.DegradedRule,
		// 初始化为空切片而非留 nil：序列化成 [] 而不是 null，
		// 前端就不必为「开关开了但还没配分组」单独写一条空值分支。
		Groups:      []*IntelCheckPublicGroup{},
		GeneratedAt: now,
	}

	targets, err := s.repo.ListEnabledTargets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled intel check targets: %w", err)
	}
	if err := s.fillIntelCheckLatestRound(ctx, out, cfg); err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return out, nil
	}

	targetIDs := make([]int64, 0, len(targets))
	for _, target := range targets {
		if target != nil {
			targetIDs = append(targetIDs, target.ID)
		}
	}

	timelines, err := s.repo.ListTimelineForTargets(ctx, targetIDs, cfg.TimelinePoints)
	if err != nil {
		return nil, fmt.Errorf("list intel check timeline: %w", err)
	}
	// 逻辑题与绘图题分两次统计：CountByStatusSince 的 kind 参数直接落到 SQL 的
	// 枚举列上，传空串不是「不过滤」而是一个非法枚举值。
	since := now.Add(-intelCheckStatsWindow)
	logicCounts, err := s.repo.CountByStatusSince(ctx, targetIDs, IntelCheckKindLogic, since)
	if err != nil {
		return nil, fmt.Errorf("count intel check logic results: %w", err)
	}
	drawingCounts, err := s.repo.CountByStatusSince(ctx, targetIDs, IntelCheckKindDrawing, since)
	if err != nil {
		return nil, fmt.Errorf("count intel check drawing results: %w", err)
	}
	artworks, err := s.repo.LatestDrawingResultIDs(ctx, targetIDs)
	if err != nil {
		return nil, fmt.Errorf("list intel check latest artworks: %w", err)
	}

	for _, target := range targets {
		if target == nil {
			continue
		}
		group := buildIntelCheckPublicGroup(target, timelines[target.ID], cfg.DegradedRule)
		group.LogicStats24h = intelCheckStatsFromCounts(logicCounts[target.ID])
		group.DrawingStats24h = intelCheckStatsFromCounts(drawingCounts[target.ID])
		group.LatestDrawingResultID = artworks[target.ID]

		out.Groups = append(out.Groups, group)
		accumulateIntelCheckSummary(&out.Summary, group)
	}
	out.Summary.TotalGroups = len(out.Groups)
	out.Summary.Stats24h.PassRate, out.Summary.Stats24h.HasData = intelCheckPassRate(
		out.Summary.Stats24h.Pass, out.Summary.Stats24h.Fail)
	return out, nil
}

// buildIntelCheckPublicGroup 按分组装配卡片，只搬运允许公开的字段。
//
// 逐字段赋值而不是结构体嵌入或反射拷贝：IntelCheckTarget 上多出一个字段时，
// 这里什么都不会变（最多少显示一项），而拷贝式写法会把新字段直接漏出去。
func buildIntelCheckPublicGroup(
	target *IntelCheckTarget,
	kinds map[string][]*IntelCheckTimelinePoint,
	rule IntelCheckDegradedRule,
) *IntelCheckPublicGroup {
	group := &IntelCheckPublicGroup{
		ID:              target.ID,
		Name:            target.Name,
		Description:     target.Description,
		Model:           target.Model,
		ReasoningEffort: target.ReasoningEffort,
		RateLabel:       target.RateLabel,
		LogicTimeline:   intelCheckTimelineOrEmpty(kinds[IntelCheckKindLogic]),
		DrawingTimeline: intelCheckTimelineOrEmpty(kinds[IntelCheckKindDrawing]),
	}
	// 状态只看逻辑题（设计文档 §4.4）：绘图题的判定含源码评审打分，
	// 天然比逻辑题波动大，拿它参与降智判定会频繁误报。
	group.State = ReplayIntelCheckState(intelCheckTimelineStatuses(group.LogicTimeline), rule)
	return group
}

// fillIntelCheckLatestRound 填入最新轮次编号、上次检测时刻与下次预计时刻。
//
// 一轮都还没跑过时三项全部留零值/nil，而不是拿 now 顶替：nil 让前端显示
// 「尚未开始检测」，塞个当前时间会让页面看起来刚检测过。
func (s *IntelCheckService) fillIntelCheckLatestRound(
	ctx context.Context, out *IntelCheckPublicOverview, cfg *IntelCheckSettings,
) error {
	// ListRounds 按 seq 降序，取一条即最新一轮。
	rounds, _, err := s.repo.ListRounds(ctx, IntelCheckRoundListParams{Page: 1, PageSize: 1})
	if err != nil {
		return fmt.Errorf("list intel check latest round: %w", err)
	}
	if len(rounds) == 0 || rounds[0] == nil {
		return nil
	}

	last := rounds[0]
	startedAt := last.StartedAt
	out.Summary.LatestRoundSeq = last.Seq
	out.Summary.LastCheckedAt = &startedAt

	// 以上一轮的开始时刻推算：ticker 是按固定周期触发的，周期锚在触发时刻，
	// 而非上一轮跑完的时刻（跑完时间随上游耗时波动，用它推算会越来越漂）。
	nextAt := startedAt.Add(time.Duration(cfg.IntervalMinutes) * time.Minute)
	out.Summary.NextCheckAt = &nextAt
	return nil
}

// accumulateIntelCheckSummary 把单组结果累加进汇总。
// 通过率不在这里算——分子分母要先累完再除，逐组求平均会让样本少的分组被过度放大。
func accumulateIntelCheckSummary(summary *IntelCheckPublicSummary, group *IntelCheckPublicGroup) {
	switch group.State {
	case IntelCheckStateNormal:
		summary.NormalGroups++
	case IntelCheckStateDegraded:
		summary.DegradedGroups++
	default:
		summary.UnknownGroups++
	}

	for _, stats := range []IntelCheckPublicStats{group.LogicStats24h, group.DrawingStats24h} {
		summary.Stats24h.Pass += stats.Pass
		summary.Stats24h.Fail += stats.Fail
		summary.Stats24h.Error += stats.Error
		summary.Stats24h.Unverified += stats.Unverified
	}
}

// intelCheckStatsFromCounts 把 SQL 聚合出的状态计数换算成对外统计。
// running 不计入任何一项：它只是尚未出结论的占位行。
func intelCheckStatsFromCounts(counts map[string]int64) IntelCheckPublicStats {
	stats := IntelCheckPublicStats{
		Pass:       counts[IntelCheckStatusPass],
		Fail:       counts[IntelCheckStatusFail],
		Error:      counts[IntelCheckStatusRequestError],
		Unverified: counts[IntelCheckStatusUnverified],
	}
	stats.PassRate, stats.HasData = intelCheckPassRate(stats.Pass, stats.Fail)
	return stats
}

// intelCheckTimelineOrEmpty 保证时间线序列化为 [] 而非 null。
func intelCheckTimelineOrEmpty(points []*IntelCheckTimelinePoint) []*IntelCheckTimelinePoint {
	if points == nil {
		return []*IntelCheckTimelinePoint{}
	}
	return points
}

// intelCheckTimelineStatuses 抽出状态序列，顺序与入参一致（升序，旧→新）。
func intelCheckTimelineStatuses(points []*IntelCheckTimelinePoint) []string {
	statuses := make([]string, 0, len(points))
	for _, point := range points {
		if point != nil {
			statuses = append(statuses, point.Status)
		}
	}
	return statuses
}

// PublicResultDetail 返回单次检测的对外详情。
//
// 与管理端详情的差别不只是少几个字段：错误原文一律换成 StatusNote 中性文案。
// error_message 里可能带上游域名、HTTP 状态码乃至凭据片段，
// 而公开页承诺不暴露上游细节，这里是那道承诺唯一的落点。
func (s *IntelCheckService) PublicResultDetail(ctx context.Context, id int64) (*IntelCheckPublicResult, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("智力检测服务未初始化")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !s.IsEnabled(ctx) {
		return nil, ErrIntelCheckDisabled
	}

	result, err := s.repo.GetResultByID(ctx, id)
	if err != nil {
		return nil, err
	}

	out := &IntelCheckPublicResult{
		ID:              result.ID,
		Kind:            result.Kind,
		Status:          result.Status,
		TargetID:        result.TargetID,
		PromptSnapshot:  result.PromptSnapshot,
		ExtractedAnswer: result.ExtractedAnswer,
		RawReply:        result.RawReply,
		HTMLOutput:      result.HTMLOutput,
		// JudgeDetail 直接透出，因此 runner 写入时不得把上游报错原文放进去
		// （请求类失败应记进 error_message，那个字段不对外）。
		JudgeDetail: result.JudgeDetail,
		LatencyMs:   result.LatencyMs,
		StatusNote:  intelCheckStatusNote(result.Status),
		CheckedAt:   result.CheckedAt,
	}
	if result.Kind == IntelCheckKindDrawing && result.JudgeDetail["judge_method"] == intelCheckDrawingJudgeVersion {
		switch result.Status {
		case IntelCheckStatusPass:
			out.StatusNote = "结构与动作轨迹验收通过；视觉质量仍需自行查看"
		case IntelCheckStatusFail:
			out.StatusNote = "绘图验收未通过，不参与逻辑题降智判定"
		case IntelCheckStatusUnverified:
			out.StatusNote = "产物存在，但缺少足够动作证据，未标记为通过"
		}
	} else if result.Kind == IntelCheckKindDrawing && result.JudgeDetail["judge_method"] == intelCheckStructureVersion {
		switch result.Status {
		case IntelCheckStatusPass:
			out.StatusNote = "结构基准通过；动作轨迹未验证"
		case IntelCheckStatusFail:
			out.StatusNote = "结构基准未通过，不代表已证明模型降智"
		}
	}
	s.decoratePublicResult(ctx, out, result)
	return out, nil
}

// decoratePublicResult 补齐分组名与题目信息。
//
// 全程尽力而为：这些都是让详情更好读的装饰，取不到就留空。
// 让一次外键回溯失败把整个弹窗变成 500，是用可读性换了可用性。
func (s *IntelCheckService) decoratePublicResult(
	ctx context.Context, out *IntelCheckPublicResult, result *IntelCheckResult,
) {
	if target, err := s.repo.GetTargetByID(ctx, result.TargetID); err == nil && target != nil {
		out.TargetName = target.Name
		out.Model = target.Model
		out.ReasoningEffort = target.ReasoningEffort
	} else if err != nil && !errors.Is(err, ErrIntelCheckTargetNotFound) {
		slog.Warn("intel_check: load target for public result failed",
			"result_id", result.ID, "target_id", result.TargetID, "error", err)
	}

	round, err := s.repo.GetRoundByID(ctx, result.RoundID)
	if err != nil {
		if !errors.Is(err, ErrIntelCheckRoundNotFound) {
			slog.Warn("intel_check: load round for public result failed",
				"result_id", result.ID, "round_id", result.RoundID, "error", err)
		}
		return
	}
	out.RoundSeq = round.Seq

	questionID := round.LogicQuestionID
	if result.Kind == IntelCheckKindDrawing {
		questionID = round.DrawingQuestionID
	}
	// 题目被删除时外键置空（ON DELETE SET NULL），此时只是没有题目信息可显示，
	// 明细本身仍然有效，不应报错。
	if questionID == nil {
		return
	}
	question, err := s.repo.GetQuestionByID(ctx, *questionID)
	if err != nil {
		if !errors.Is(err, ErrIntelCheckQuestionNotFound) {
			slog.Warn("intel_check: load question for public result failed",
				"result_id", result.ID, "question_id", *questionID, "error", err)
		}
		return
	}

	out.QuestionTitle = question.Title
	// 期望答案与匹配模式只对逻辑题有意义；绘图题带上这两项会让弹窗显示
	// 一个空的「期望答案」栏位，看起来像数据丢了。
	if result.Kind == IntelCheckKindLogic {
		out.ExpectedAnswer = question.ExpectedAnswer
		out.MatchMode = question.MatchMode
	}
}

// intelCheckStatusNote 按状态给出中性说明，取代不可对外的 error_message。
func intelCheckStatusNote(status string) string {
	switch status {
	case IntelCheckStatusPass:
		return "本次检测通过"
	case IntelCheckStatusFail:
		return "本次检测未通过"
	case IntelCheckStatusRequestError:
		return "本次请求未能完成，未测出结论（不计入连续失败）"
	case IntelCheckStatusRunning:
		return "检测中"
	case IntelCheckStatusUnverified:
		return "未验证（不计入通过率或连续失败）"
	default:
		return ""
	}
}
