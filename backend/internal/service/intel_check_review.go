package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// intelCheckDefaultRubric 绘图题源码评审的内置检查清单。
// 题目未单独配置 review_rubric 时使用。清单按「掉智力最先掉的能力」排序：
// 降智模型通常先丢细节与动画联动，最后才丢整体构图。
const intelCheckDefaultRubric = `1. 主体完整度（25 分）：题面要求的主体是否齐全，各部件是否分离建模（而非一团色块）。
2. 结构合理度（20 分）：部件的比例、连接关系、遮挡层次是否正确。
3. 动画质量（25 分）：是否有持续动画，多个部件是否联动（而非整体平移），运动是否符合物理直觉。
4. 细节丰富度（20 分）：背景、光影、渐变、点缀元素是否到位。
5. 代码工程性（10 分）：是否使用 defs/symbol 复用、是否有 title/desc 说明、结构是否清晰。`

// intelCheckReviewOutputSpec 评审输出格式约束。
// 单独抽出是因为它必须和 IntelCheckReviewResult 的 json tag 严格对应，
// 改字段时两处要一起改。
const intelCheckReviewOutputSpec = `只输出一个 JSON 对象，不要输出任何解释文字，不要使用 Markdown 代码块。格式：
{"score": 0-100 的整数, "summary": "一句话总评", "items": [{"item": "检查项名称", "score": 该项得分, "max_score": 该项满分, "comment": "扣分理由，不扣分则写通过"}]}`

// IntelCheckReviewItem 评审的单项打分。
type IntelCheckReviewItem struct {
	Item     string `json:"item"`
	Score    int    `json:"score"`
	MaxScore int    `json:"max_score"`
	Comment  string `json:"comment"`
}

// IntelCheckReviewResult 评审模型的输出。
type IntelCheckReviewResult struct {
	Score   int                    `json:"score"`
	Summary string                 `json:"summary"`
	Items   []IntelCheckReviewItem `json:"items"`
}

// ErrIntelCheckReviewUnparsable 评审输出无法解析为约定 JSON。
// 属于评审链路自身故障，调用方应记为 request_error 而非判定失败。
var ErrIntelCheckReviewUnparsable = errors.New("intel check: 评审输出无法解析为 JSON")

// BuildIntelCheckReviewPrompt 拼装第二层源码评审的提示词。
//
// 参考稿必须一并提供：没有「满血产出长什么样」的标尺，模型对孤立样本的打分
// 会随机漂移，无法稳定区分满血稿与降智稿。参考稿为空时降级为绝对打分，
// 并在提示词中明示，避免模型自行脑补标尺。
func BuildIntelCheckReviewPrompt(prompt, referenceHTML, candidateHTML, rubric string, reference, candidate DrawingMetrics) string {
	rubric = strings.TrimSpace(rubric)
	if rubric == "" {
		rubric = intelCheckDefaultRubric
	}

	var b strings.Builder
	b.WriteString("你是严格的前端图形代码评审员。下面是一道绘图题，以及某个模型的作答源码。\n")
	b.WriteString("请阅读源码本身（不要脑补渲染效果），按检查清单打分。\n\n")

	b.WriteString("## 题目\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\n\n")

	referenceHTML = strings.TrimSpace(referenceHTML)
	if referenceHTML != "" {
		b.WriteString("## 参考稿（满血模型产出，视为 90 分基准）\n")
		b.WriteString("```html\n")
		b.WriteString(referenceHTML)
		b.WriteString("\n```\n\n")
	} else {
		b.WriteString("## 参考稿\n本题暂无参考稿，请按检查清单绝对打分。\n\n")
	}

	b.WriteString("## 待评审作答\n")
	b.WriteString("```html\n")
	b.WriteString(strings.TrimSpace(candidateHTML))
	b.WriteString("\n```\n\n")

	b.WriteString("## 结构指标对比（程序静态统计，供参考）\n")
	b.WriteString(intelCheckFormatMetricsTable(candidate, reference, referenceHTML != ""))
	b.WriteString("\n")

	b.WriteString("## 检查清单\n")
	b.WriteString(rubric)
	b.WriteString("\n\n")

	b.WriteString("## 评分要求\n")
	if referenceHTML != "" {
		b.WriteString("- 以参考稿为 90 分基准：作答明显更简陋（部件更少、动画更单调、细节缺失）必须显著低于 90 分。\n")
		b.WriteString("- 不要因为作答「能跑」「没报错」就给高分，本次评审的目的是识别偷工减料。\n")
	}
	b.WriteString("- 各项得分之和必须等于总分 score。\n")
	b.WriteString("- 扣分项必须写明具体扣在哪里（指出源码中的具体缺失），不得笼统评价。\n\n")

	b.WriteString("## 输出格式\n")
	b.WriteString(intelCheckReviewOutputSpec)
	b.WriteString("\n")

	return b.String()
}

// intelCheckFormatMetricsTable 渲染结构指标对比表。
func intelCheckFormatMetricsTable(candidate, reference DrawingMetrics, withReference bool) string {
	rows := []struct {
		name      string
		got, want int
	}{
		{"造型数量", candidate.ShapeCount, reference.ShapeCount},
		{"动画目标数", candidate.AnimatedTargets, reference.AnimatedTargets},
		{"可复用符号数", candidate.DefsSymbols, reference.DefsSymbols},
		{"路径数据量(字节)", candidate.PathDataBytes, reference.PathDataBytes},
		{"产物体积(字节)", candidate.HTMLBytes, reference.HTMLBytes},
	}

	var b strings.Builder
	if withReference {
		b.WriteString("| 指标 | 作答 | 参考稿 |\n| --- | --- | --- |\n")
		for _, row := range rows {
			fmt.Fprintf(&b, "| %s | %d | %d |\n", row.name, row.got, row.want)
		}
	} else {
		b.WriteString("| 指标 | 作答 |\n| --- | --- |\n")
		for _, row := range rows {
			fmt.Fprintf(&b, "| %s | %d |\n", row.name, row.got)
		}
	}
	mechanisms := intelCheckJoinOrNone(candidate.Mechanisms)
	fmt.Fprintf(&b, "\n作答使用的动画机制：%s\n", mechanisms)
	return b.String()
}

// ParseIntelCheckReviewResult 解析评审模型的输出。
//
// 三级兜底：原文直接解码 → 剥掉 Markdown 代码围栏 → 截取第一个 `{` 到最后一个 `}`。
// 三级都失败返回 ErrIntelCheckReviewUnparsable。
func ParseIntelCheckReviewResult(reply string) (IntelCheckReviewResult, error) {
	for _, candidate := range intelCheckJSONCandidates(reply) {
		// 每级使用全新变量：解析失败时 json.Unmarshal 可能已写入部分字段，
		// 复用同一变量会让上一级的残留数据混进最终结果。
		var result IntelCheckReviewResult
		if err := json.Unmarshal([]byte(candidate), &result); err != nil {
			continue
		}
		result.Normalize()
		return result, nil
	}
	return IntelCheckReviewResult{}, ErrIntelCheckReviewUnparsable
}

// intelCheckJSONCandidates 依次给出三级兜底的候选 JSON 文本。
func intelCheckJSONCandidates(reply string) []string {
	trimmed := strings.TrimSpace(reply)
	candidates := make([]string, 0, 3)
	if trimmed != "" {
		candidates = append(candidates, trimmed)
	}
	if stripped := intelCheckStripCodeFence(trimmed); stripped != "" && stripped != trimmed {
		candidates = append(candidates, stripped)
	}
	if braced := intelCheckSliceBraces(trimmed); braced != "" {
		candidates = append(candidates, braced)
	}
	return candidates
}

// intelCheckStripCodeFence 剥掉包裹整段输出的 Markdown 代码围栏。
func intelCheckStripCodeFence(text string) string {
	if !strings.HasPrefix(text, "```") {
		return ""
	}
	newline := strings.Index(text, "\n")
	if newline < 0 {
		return ""
	}
	body := text[newline+1:]
	if end := strings.LastIndex(body, "```"); end >= 0 {
		body = body[:end]
	}
	return strings.TrimSpace(body)
}

// intelCheckSliceBraces 截取最外层花括号之间的内容，处理模型在 JSON 前后加废话的情况。
func intelCheckSliceBraces(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return ""
	}
	return text[start : end+1]
}

// DecideIntelCheckDrawing 合并两层判定，得出绘图题的最终状态与明细。
//
// 两层都必须通过：结构门禁拦截「明显偷工减料」，源码评审拦截「结构指标凑数
// 但实际粗糙」。门禁不通过时直接判失败，不再调用评审模型（省调用成本，
// 也避免评审模型给残缺产物打出高分）。
//
// 返回的 detail 直接写入 results.judge_detail，前端据此逐条展示判定依据。
func DecideIntelCheckDrawing(gate IntelCheckGateResult, review *IntelCheckReviewResult, passScore int) (string, map[string]any) {
	if passScore <= 0 || passScore > 100 {
		passScore = intelCheckDefaultPassScore
	}

	detail := map[string]any{
		"gate_pass":  gate.Pass,
		"gate_items": gate.Items,
		"pass_score": passScore,
	}

	if !gate.Pass {
		detail["reason"] = "结构门禁未通过，未进入源码评审"
		return IntelCheckStatusFail, detail
	}

	if review == nil {
		detail["reason"] = "结构门禁通过，但未获得评审结果"
		return IntelCheckStatusFail, detail
	}

	detail["review_score"] = review.Score
	detail["review_summary"] = review.Summary
	detail["review_items"] = review.Items

	if review.Score < passScore {
		detail["reason"] = fmt.Sprintf("源码评审 %d 分，低于及格线 %d 分", review.Score, passScore)
		return IntelCheckStatusFail, detail
	}

	detail["reason"] = fmt.Sprintf("结构门禁通过，源码评审 %d 分（及格线 %d 分）", review.Score, passScore)
	return IntelCheckStatusPass, detail
}

// Normalize 收敛越界分数，避免模型输出 120 分或负分污染判定。
func (r *IntelCheckReviewResult) Normalize() {
	if r.Score < 0 {
		r.Score = 0
	}
	if r.Score > 100 {
		r.Score = 100
	}
	for i := range r.Items {
		if r.Items[i].Score < 0 {
			r.Items[i].Score = 0
		}
		if r.Items[i].MaxScore > 0 && r.Items[i].Score > r.Items[i].MaxScore {
			r.Items[i].Score = r.Items[i].MaxScore
		}
	}
	r.Summary = strings.TrimSpace(r.Summary)
}
