package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/alitto/pond/v2"
)

// intelCheckSaveTimeout 落库操作的超时。
//
// 写库用的是 context.WithoutCancel 派生的 ctx：整轮超时或进程收到停止信号时，
// 已经拿到的检测结果仍必须写回，否则那些 running 行会永远停在浅蓝色。
const intelCheckSaveTimeout = 10 * time.Second

// RunOnce 执行一轮检测：选题 → 建轮次 → 落 running 占位行 → 并发检测 → 收尾。
//
// trigger 取 IntelCheckTriggerCron（自动）或 IntelCheckTriggerManual（管理端手动）。
// 返回的 round 即便在部分分组失败时也是有效的——单个分组的失败已写进各自明细，
// 不应把整轮拖成错误。
func (s *IntelCheckService) RunOnce(ctx context.Context, trigger string) (*IntelCheckRound, error) {
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

	logicQ, err := s.pickIntelCheckQuestion(ctx, IntelCheckKindLogic)
	if err != nil {
		return nil, err
	}
	drawingQ, err := s.pickIntelCheckQuestion(ctx, IntelCheckKindDrawing)
	if err != nil {
		return nil, err
	}
	// 两类题都没有就不建轮次：空轮次会让公开页的轮次号凭空前进，
	// 看起来像检测跑过但全员没数据。
	if logicQ == nil && drawingQ == nil {
		return nil, ErrIntelCheckNoQuestion
	}

	targets, err := s.ListEnabledTargets(ctx)
	if err != nil {
		return nil, err
	}
	targets = filterIntelCheckRunnableTargets(targets)

	round, err := s.createIntelCheckRound(ctx, trigger, logicQ, drawingQ)
	if err != nil {
		return nil, err
	}
	// 没有可跑的分组时仍然建并结束轮次：公开页的「下次检测时刻」是按最近一轮
	// 的开始时间推算的，跳过建轮会让页面看起来调度已经停摆。
	if len(targets) == 0 {
		slog.Warn("intel_check: 本轮没有可用的受检分组", "round_id", round.ID, "round_seq", round.Seq)
		s.finishIntelCheckRound(ctx, round.ID)
		return round, nil
	}

	drafts := buildIntelCheckDrafts(round, targets, logicQ, drawingQ)
	if err := s.repo.CreateRunningResults(ctx, drafts); err != nil {
		s.finishIntelCheckRound(ctx, round.ID)
		return nil, fmt.Errorf("create intel check running results: %w", err)
	}

	s.runIntelCheckTargets(ctx, cfg, round, targets, logicQ, drawingQ, nil)
	s.finishIntelCheckRound(ctx, round.ID)
	return round, nil
}

// pickIntelCheckQuestion 取一道启用的题目；该题型无题时返回 (nil, nil)。
//
// 只把「没有题目」当作可容忍的情况，其余错误（如数据库不可用）原样抛出：
// 把连接故障静默当成无题，会让整轮悄无声息地什么都不测。
func (s *IntelCheckService) pickIntelCheckQuestion(ctx context.Context, kind string) (*IntelCheckQuestion, error) {
	question, err := s.PickEnabledQuestion(ctx, kind)
	if err != nil {
		if errors.Is(err, ErrIntelCheckNoQuestion) {
			return nil, nil
		}
		return nil, fmt.Errorf("pick intel check question(%s): %w", kind, err)
	}
	return question, nil
}

// createIntelCheckRound 建轮次。
// 不设置 Seq：编号由仓储取序列值，重复赋值会撞上唯一索引让整轮失败。
func (s *IntelCheckService) createIntelCheckRound(
	ctx context.Context, trigger string, logicQ, drawingQ *IntelCheckQuestion,
) (*IntelCheckRound, error) {
	round := &IntelCheckRound{StartedAt: time.Now(), TriggerSource: trigger}
	if logicQ != nil {
		id := logicQ.ID
		round.LogicQuestionID = &id
	}
	if drawingQ != nil {
		id := drawingQ.ID
		round.DrawingQuestionID = &id
	}
	if err := s.repo.CreateRound(ctx, round); err != nil {
		return nil, fmt.Errorf("create intel check round: %w", err)
	}
	return round, nil
}

// filterIntelCheckRunnableTargets 滤掉凭据无法解密的分组。
// 沿用 ChannelMonitor 的取舍：一把坏密钥只让该分组缺席本轮，不牵连其他分组。
func filterIntelCheckRunnableTargets(targets []*IntelCheckTarget) []*IntelCheckTarget {
	runnable := make([]*IntelCheckTarget, 0, len(targets))
	for _, target := range targets {
		if target == nil {
			continue
		}
		if target.APIKeyDecryptFailed {
			slog.Error("intel_check: 分组凭据解密失败，本轮跳过",
				"target_id", target.ID, "name", target.Name)
			continue
		}
		runnable = append(runnable, target)
	}
	return runnable
}

// buildIntelCheckDrafts 为每个（分组 × 题型）建一条 running 占位行。
// 先落占位行再发请求，前端在一轮开始的瞬间就能看到浅蓝色的「检测中」。
func buildIntelCheckDrafts(
	round *IntelCheckRound, targets []*IntelCheckTarget, logicQ, drawingQ *IntelCheckQuestion,
) []*IntelCheckResultDraft {
	now := time.Now()
	drafts := make([]*IntelCheckResultDraft, 0, len(targets)*2)
	for _, target := range targets {
		for _, question := range []*IntelCheckQuestion{logicQ, drawingQ} {
			if question == nil {
				continue
			}
			drafts = append(drafts, &IntelCheckResultDraft{
				RoundID:  round.ID,
				TargetID: target.ID,
				Kind:     question.Kind,
				// 快照题面：题目日后被改或被删，历史详情仍能还原当时问了什么。
				PromptSnapshot: intelCheckEffectivePrompt(question),
				CheckedAt:      now,
			})
		}
	}
	return drafts
}

// runIntelCheckTargets 并发跑各分组。
//
// 并发粒度是「分组」而非「单题」：同一分组的两道题共用一把上游凭据，
// 并行发只会给自己制造限流，而限流会被记成 request_error——
// 那等于用检测结果污染检测结论。
func (s *IntelCheckService) runIntelCheckTargets(
	ctx context.Context,
	cfg *IntelCheckSettings,
	round *IntelCheckRound,
	targets []*IntelCheckTarget,
	logicQ, drawingQ *IntelCheckQuestion,
	judge *IntelCheckTarget,
) {
	pool := pond.NewPool(cfg.Concurrency)
	for _, target := range targets {
		target := target
		pool.Submit(func() {
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("intel_check: 分组检测 panic",
						"round_id", round.ID, "target_id", target.ID, "panic", rec)
				}
			}()
			for _, question := range []*IntelCheckQuestion{logicQ, drawingQ} {
				if question == nil {
					continue
				}
				s.runIntelCheckOne(ctx, cfg, round, target, question, judge)
			}
		})
	}
	// StopAndWait 会等所有已提交任务跑完，不需要额外的 WaitGroup。
	pool.StopAndWait()
}

// runIntelCheckOne 跑一道题并写回结果。
//
// 无论走哪条分支（含 panic），defer 都保证恰好写一次结果：
// 提前 return 而漏写，那一格就会永远停在「检测中」。
func (s *IntelCheckService) runIntelCheckOne(
	ctx context.Context,
	cfg *IntelCheckSettings,
	round *IntelCheckRound,
	target *IntelCheckTarget,
	question *IntelCheckQuestion,
	judge *IntelCheckTarget,
) {
	outcome := &IntelCheckResultOutcome{
		RoundID:  round.ID,
		TargetID: target.ID,
		Kind:     question.Kind,
		// 初值即兜底：任何未覆盖到的退出路径都落成 request_error，
		// 不会把「没跑完」记成模型不合格。
		Status:       IntelCheckStatusRequestError,
		ErrorMessage: "检测未产出结论",
		CheckedAt:    time.Now(),
	}
	defer func() {
		if rec := recover(); rec != nil {
			outcome.Status = IntelCheckStatusRequestError
			outcome.ErrorMessage = fmt.Sprintf("检测过程发生 panic：%v", rec)
			outcome.JudgeDetail = map[string]any{"reason": "检测过程异常中断，本次不计入判定"}
			slog.Error("intel_check: 单题检测 panic",
				"round_id", round.ID, "target_id", target.ID, "kind", question.Kind, "panic", rec)
		}
		s.saveIntelCheckOutcome(ctx, outcome)
	}()

	probe := s.probeIntelCheckOnce(ctx, cfg, target, question, judge)
	outcome.Status = probe.Judged.Status
	outcome.ExtractedAnswer = probe.Judged.ExtractedAnswer
	outcome.HTMLOutput = probe.Judged.HTMLOutput
	outcome.JudgeDetail = probe.Judged.JudgeDetail
	outcome.ErrorMessage = probe.Judged.ErrorMessage
	if probe.Reply != nil {
		outcome.LatencyMs = &probe.Reply.LatencyMs
		outcome.InputTokens = probe.Reply.InputTokens
		outcome.OutputTokens = probe.Reply.OutputTokens
		outcome.RawReply = probe.Reply.Text
	}
}

// intelCheckProbe 一次「发请求 + 判定」的产物，不含任何持久化字段。
type intelCheckProbe struct {
	// Reply 上游成功返回时的产物；请求本身失败时为 nil。
	Reply *intelCheckUpstreamReply
	// Judged 判定结论。请求失败时也已填成 request_error，调用方无需另判。
	Judged intelCheckJudgeOutcome
}

// probeIntelCheckOnce 对单个分组跑一道题并判定，全程不碰数据库。
//
// 从 runIntelCheckOne 里抽出来，是为了让管理端的试跑与真实轮次共用同一条判定路径。
// 这不只是去重：试跑的全部用途就是预测真实检测会怎么判，两边一旦漂移，
// 标定工具就会谎报 runner 的行为——而那种偏差没有任何断言能自动发现，
// 只会以「标定时说通过、线上却判失败」的形式暴露。
func (s *IntelCheckService) probeIntelCheckOnce(
	ctx context.Context,
	cfg *IntelCheckSettings,
	target *IntelCheckTarget,
	question *IntelCheckQuestion,
	judge *IntelCheckTarget,
) intelCheckProbe {
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.RequestTimeoutSeconds)*time.Second)
	defer cancel()

	reply, err := callIntelCheckUpstream(callCtx, intelCheckUpstreamRequest{
		BaseURL:         target.BaseURL,
		APIKey:          target.APIKey,
		APIMode:         target.APIMode,
		Model:           target.Model,
		ReasoningEffort: target.ReasoningEffort,
		Prompt:          intelCheckEffectivePrompt(question),
		// 逻辑题与绘图题都在验证受检模型，使用与 Codex CLI 一致的流式路径。
		// 必须等完整终止事件后再判题，解析器不会把半截输出当结论。
		Stream: true,
	})
	if err != nil {
		return intelCheckProbe{Judged: intelCheckJudgeOutcome{
			Status: IntelCheckStatusRequestError,
			// 报错原文只进 ErrorMessage：公开详情读的是 JudgeDetail 与状态文案。
			ErrorMessage: fmt.Sprintf("上游请求失败：%v", err),
			JudgeDetail:  map[string]any{"reason": "本次请求未能完成，未测出结论"},
		}}
	}

	judged := judgeIntelCheckLogic(question, reply.Text)
	if question.Kind == IntelCheckKindDrawing {
		judged = s.judgeIntelCheckDrawing(ctx, cfg, question, judge, reply.Text)
	}
	return intelCheckProbe{Reply: reply, Judged: judged}
}

// saveIntelCheckOutcome 写回一次检测结果。
func (s *IntelCheckService) saveIntelCheckOutcome(ctx context.Context, outcome *IntelCheckResultOutcome) {
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), intelCheckSaveTimeout)
	defer cancel()

	if err := s.repo.SaveResultOutcome(saveCtx, outcome); err != nil {
		slog.Error("intel_check: 检测结果写回失败",
			"round_id", outcome.RoundID, "target_id", outcome.TargetID,
			"kind", outcome.Kind, "status", outcome.Status, "error", err)
	}
}

// finishIntelCheckRound 标记轮次结束。失败只记日志：轮次的结束时间是展示信息，
// 缺一个时间戳不该让已经跑完的一轮对外表现为失败。
func (s *IntelCheckService) finishIntelCheckRound(ctx context.Context, roundID int64) {
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), intelCheckSaveTimeout)
	defer cancel()

	if err := s.repo.FinishRound(finishCtx, roundID, time.Now()); err != nil {
		slog.Error("intel_check: 轮次收尾失败", "round_id", roundID, "error", err)
	}
}
