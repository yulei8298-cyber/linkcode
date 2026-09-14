package service

import (
	"context"
	"errors"
	"fmt"
	"math"
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

// judgeIntelCheckDrawing 执行确定性结构验收。保留调用签名，但不使用评审配置或凭据。
// 新结果标记算法版本和覆盖范围，不能把结构通过解释成运动学或视觉质量通过。
func (s *IntelCheckService) judgeIntelCheckDrawing(
	_ context.Context,
	_ *IntelCheckSettings,
	question *IntelCheckQuestion,
	_ *IntelCheckTarget,
	reply string,
) intelCheckJudgeOutcome {
	rules, err := DecodeIntelCheckDrawingRules(question.DrawingRules)
	if err != nil {
		return intelCheckJudgeError("", "题目的绘图判定规则无法解析，本次不计入判定",
			fmt.Errorf("decode drawing rules: %w", err))
	}
	rules.Normalize()

	source, err := ExtractIntelCheckDrawing(reply)
	if err != nil {
		// 回复里根本没有产物属于模型没答到点上，是实打实的失败，不是链路故障。
		return intelCheckDrawingFail("", "回复中没有可提取的 HTML 或 SVG 产物")
	}

	html, err := SanitizeIntelCheckDrawing(source, rules.MaxBytes)
	if err != nil {
		reason := "产物无法展示"
		if errors.Is(err, errIntelCheckTooLarge) {
			reason = fmt.Sprintf("产物体积超过 %d 字节上限", rules.MaxBytes)
		}
		return intelCheckDrawingFail("", reason)
	}

	candidate, err := ComputeDrawingMetrics(html)
	if err != nil {
		return intelCheckDrawingFail(html, "产物中没有可解析的 SVG 内容")
	}
	baseline, count, err := intelCheckStructureBaseline(question.ReferenceHTML, rules.StandardSources)
	if err != nil {
		return intelCheckJudgeError(html, "题目的标准样本尚未配置或无法解析，本次不计入判定", err)
	}
	structure, err := ComputeIntelCheckStructureMetrics(html)
	if err != nil {
		return intelCheckDrawingFail(html, "产物的本地 SVG 引用无效或结构展开超过上限")
	}
	if structure.Shapes == 0 && candidate.HasMechanism(DrawingMechanismScript) {
		return intelCheckJudgeError(html, "未检测到可统计的静态图元，需人工核验动态绘图",
			fmt.Errorf("静态结构验收无法验证脚本生成的几何"))
	}
	gate := evaluateIntelCheckFixedGate(html, candidate, rules)
	score := math.Round(intelCheckStructureScore(structure, baseline)*100) / 100
	threshold := rules.MinRatio * 100
	gate.Items = append(gate.Items, IntelCheckGateItem{
		Item: "结构综合分", Pass: score >= threshold,
		Detail: fmt.Sprintf("%.2f / 100（要求 ≥ %.2f）；图元 %d / 基准 %d，几何参数 %d / 基准 %d；%d 份标准取中位数",
			score, threshold, structure.Shapes, baseline.Shapes, structure.GeometryValues, baseline.GeometryValues, count),
	})
	gate.Pass = gate.Pass && score >= threshold
	status, reason := IntelCheckStatusPass, "结构基准通过"
	if !gate.Pass {
		status, reason = IntelCheckStatusFail, "结构基准未通过"
	}
	return intelCheckJudgeOutcome{
		Status: status, HTMLOutput: html,
		JudgeDetail: map[string]any{
			"judge_method": intelCheckStructureVersion, "gate_pass": gate.Pass, "gate_items": gate.Items,
			"structure_score": score, "structure_threshold": threshold,
			"reference_count": count, "candidate_structure": structure, "baseline_structure": baseline,
			"standard_digest": intelCheckStandardDigest(question.ReferenceHTML, rules.StandardSources),
			"review_skipped":  true, "kinematics_verified": false, "reason": reason,
			"scope_note": "仅检查静态结构和动画声明，未验证视觉质量、轮心稳定或脚踏联动；结构达标不等于画作质量达标。",
		},
	}
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
