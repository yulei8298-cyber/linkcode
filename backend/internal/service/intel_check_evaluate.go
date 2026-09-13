package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// intelCheckJudgeOutcome 一次判定的产物，由 runner 搬进 IntelCheckResultOutcome。
//
// ErrorMessage 与 JudgeDetail 的分工是本文件最关键的约束：
// JudgeDetail 会原样出现在公开详情里，只能放中性、可复核的判定信息；
// 上游地址、HTTP 状态码、报错原文一律只进 ErrorMessage（不对外）。
type intelCheckJudgeOutcome struct {
	Status          string
	ExtractedAnswer string
	HTMLOutput      string
	JudgeDetail     map[string]any
	ErrorMessage    string
}

// judgeIntelCheckLogic 判定逻辑题。
//
// 匹配规则本身执行失败（例如管理端填了非法正则）记 request_error 而非 fail：
// 那是题目配置问题，把它算成模型降智会让公开页出现凭空的红块。
func judgeIntelCheckLogic(question *IntelCheckQuestion, reply string) intelCheckJudgeOutcome {
	extracted := ExtractIntelCheckAnswer(reply)
	matched, err := MatchIntelCheckAnswer(reply, extracted, question.ExpectedAnswer, question.MatchMode)
	if err != nil {
		return intelCheckJudgeOutcome{
			Status:          IntelCheckStatusRequestError,
			ExtractedAnswer: extracted,
			ErrorMessage:    fmt.Sprintf("题目匹配规则执行失败：%v", err),
			JudgeDetail: map[string]any{
				"match_mode": question.MatchMode,
				"reason":     "题目的匹配规则无法执行，本次不计入判定",
			},
		}
	}

	status := IntelCheckStatusFail
	if matched {
		status = IntelCheckStatusPass
	}
	return intelCheckJudgeOutcome{
		Status:          status,
		ExtractedAnswer: extracted,
		JudgeDetail: map[string]any{
			"match_mode":       question.MatchMode,
			"expected_answer":  question.ExpectedAnswer,
			"extracted_answer": extracted,
			"matched":          matched,
		},
	}
}

// judgeIntelCheckDrawing 判定绘图题：结构门禁 + 源码评审，两层都过才算通过。
//
// 分层的意义在于省钱也在于可解释：门禁只做相对参考稿的结构比对，不花 token；
// 只有结构达标的产物才值得送去评审模型打分。门禁没过就短路，
// DecideIntelCheckDrawing 接受 review 为 nil 正是为此。
func (s *IntelCheckService) judgeIntelCheckDrawing(
	ctx context.Context,
	cfg *IntelCheckSettings,
	question *IntelCheckQuestion,
	judge *IntelCheckTarget,
	reply string,
) intelCheckJudgeOutcome {
	rules, err := DecodeIntelCheckDrawingRules(question.DrawingRules)
	if err != nil {
		return intelCheckJudgeError("", "题目的绘图判定规则无法解析，本次不计入判定",
			fmt.Errorf("decode drawing rules: %w", err))
	}

	source, err := ExtractIntelCheckDrawing(reply)
	if err != nil {
		// 回复里根本没有产物属于模型没答到点上，是实打实的失败，不是链路故障。
		return intelCheckDrawingFail("", "回复中没有可提取的 HTML 或 SVG 产物")
	}

	html, err := SanitizeIntelCheckDrawing(source, rules.MaxBytes)
	if err != nil {
		reason := "产物清洗失败，无法安全展示"
		if errors.Is(err, errIntelCheckTooLarge) {
			reason = fmt.Sprintf("产物体积超过 %d 字节上限", rules.MaxBytes)
		}
		return intelCheckDrawingFail("", reason)
	}

	candidate, err := ComputeDrawingMetrics(html)
	if err != nil {
		return intelCheckDrawingFail(html, "产物中没有可解析的 SVG 内容")
	}
	reference, err := DecodeDrawingMetrics(question.ReferenceMetrics)
	if err != nil {
		return intelCheckJudgeError(html, "题目的参考稿指标无法解析，本次不计入判定",
			fmt.Errorf("decode reference metrics: %w", err))
	}

	gate := EvaluateIntelCheckGate(html, candidate, reference, rules)

	var review *IntelCheckReviewResult
	if gate.Pass {
		result, err := s.reviewIntelCheckDrawing(ctx, cfg, question, judge, html, reference, candidate)
		if err != nil {
			// 评审模型自己挂了，责任不在受检模型：记 request_error（黄色），
			// 不能让评审侧的抖动在受检分组的时间线上留下红块。
			return intelCheckJudgeError(html, "评审链路未能完成，本次不计入判定", err)
		}
		review = result
	}

	status, detail := DecideIntelCheckDrawing(gate, review, cfg.DrawingJudge.PassScore)
	return intelCheckJudgeOutcome{Status: status, HTMLOutput: html, JudgeDetail: detail}
}

// reviewIntelCheckDrawing 调用管理员指定的评审模型，对源码打分。
//
// 评审走的是「评审分组的地址与凭据 + 设置里指定的模型与推理等级」：
// 评审模型未必是受检模型，把它绑死在某个受检分组的模型上会让打分随受检对象漂移。
func (s *IntelCheckService) reviewIntelCheckDrawing(
	ctx context.Context,
	cfg *IntelCheckSettings,
	question *IntelCheckQuestion,
	judge *IntelCheckTarget,
	html string,
	reference, candidate DrawingMetrics,
) (*IntelCheckReviewResult, error) {
	if judge == nil {
		return nil, fmt.Errorf("评审分组不可用（未配置、已删除或凭据无法解密）")
	}

	prompt := BuildIntelCheckReviewPrompt(
		question.Prompt, question.ReferenceHTML, html, question.ReviewRubric, reference, candidate)

	callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.RequestTimeoutSeconds)*time.Second)
	defer cancel()

	reply, err := callIntelCheckUpstream(callCtx, intelCheckUpstreamRequest{
		BaseURL:         judge.BaseURL,
		APIKey:          judge.APIKey,
		APIMode:         judge.APIMode,
		Model:           cfg.DrawingJudge.Model,
		ReasoningEffort: cfg.DrawingJudge.ReasoningEffort,
		Prompt:          prompt,
	})
	if err != nil {
		return nil, fmt.Errorf("评审模型调用失败：%w", err)
	}

	result, err := ParseIntelCheckReviewResult(reply.Text)
	if err != nil {
		return nil, fmt.Errorf("评审模型输出无法解析：%w", err)
	}
	return &result, nil
}

// intelCheckDrawingFail 构造一条绘图题失败判定。
// reason 会被公开展示，必须是中性、面向读者的说明。
func intelCheckDrawingFail(html, reason string) intelCheckJudgeOutcome {
	return intelCheckJudgeOutcome{
		Status:      IntelCheckStatusFail,
		HTMLOutput:  html,
		JudgeDetail: map[string]any{"gate_pass": false, "reason": reason},
	}
}

// intelCheckJudgeError 构造一条「链路故障」判定（黄色块）。
// reason 对外，err 只进 error_message。
func intelCheckJudgeError(html, reason string, err error) intelCheckJudgeOutcome {
	return intelCheckJudgeOutcome{
		Status:       IntelCheckStatusRequestError,
		HTMLOutput:   html,
		ErrorMessage: fmt.Sprintf("%s：%v", reason, err),
		JudgeDetail:  map[string]any{"reason": reason},
	}
}
