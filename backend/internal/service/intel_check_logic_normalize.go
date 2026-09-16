package service

import (
	"regexp"
	"strings"
)

var (
	// 只有整个候选都是「单一数值 + 受控单位/量词」时才去掉后缀。
	// 有限白名单避免把「21个苹果」「21颗或22颗」或任意英文单词折叠成 21。
	intelCheckNumberWithControlledUnitRe = regexp.MustCompile(`(?i)^(-?\d+(?:\.\d+)?)\s*(?:个|颗|只|名|人|位|件|张|条|本|枚|份|组|对|次|种|套|辆|台|盒|箱|瓶|杯|块|片|根|朵|棵|头|间|家|题|分|元|角|%|℃|度|毫米|厘米|千米|公里|米|毫克|千克|公斤|克|毫升|升|毫秒|秒|分钟|小时|天|周|个月|月|年|mm|cm|km|m|mg|kg|g|ml|l|ms|min|h|s|items?|pieces?|people|persons?|times?|points?|degrees?|seconds?|minutes?|hours?|days?|weeks?|months?|years?|dollars?|yuan)$`)
	intelCheckPlainNumberRe              = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
)

// 候选开头保留 ~、<、>、. 与 _：它们可能分别表示近似、比较、小数或标识符语义。
// 英文句号只从末尾剥掉，避免把 .5颗 错改成 5颗。
const (
	intelCheckCandidateLeadingCutset  = " \t\r\n*`\"'“”‘’()（）[]【】《》{}:：=。．,，、;；!！?？"
	intelCheckCandidateTrailingCutset = intelCheckCandidateLeadingCutset + "."
)

// normalizeIntelCheckAnswerCandidate 统一处理盒装、显式作答、加粗和末行四条提取路径。
func normalizeIntelCheckAnswerCandidate(candidate string, stripControlledUnit bool) string {
	candidate = strings.TrimRight(strings.TrimSpace(candidate), intelCheckCandidateTrailingCutset)
	for {
		unwrapped, ok := intelCheckStripPairedWrapper(candidate)
		if !ok {
			break
		}
		candidate = strings.TrimRight(strings.TrimSpace(unwrapped), intelCheckCandidateTrailingCutset)
	}
	candidate = strings.TrimLeft(candidate, intelCheckCandidateLeadingCutset)
	if candidate == "" {
		return ""
	}

	normalized := normalizeIntelCheckFullWidth(candidate)
	// 双下划线也可能是 __init__ 这类标识符，仅在内部明确为数值候选时剥掉。
	if strings.HasPrefix(normalized, "__") && strings.HasSuffix(normalized, "__") && len(normalized) > 4 {
		inner := strings.TrimSpace(normalized[2 : len(normalized)-2])
		if intelCheckPlainNumberRe.MatchString(inner) || intelCheckNumberWithControlledUnitRe.MatchString(inner) {
			candidate, normalized = inner, inner
		}
	}
	if stripControlledUnit {
		if match := intelCheckNumberWithControlledUnitRe.FindStringSubmatch(normalized); len(match) == 2 {
			return match[1]
		}
	}
	return candidate
}

func intelCheckStripPairedWrapper(candidate string) (string, bool) {
	for _, wrapper := range []string{"**", "*", "`"} {
		if strings.HasPrefix(candidate, wrapper) && strings.HasSuffix(candidate, wrapper) && len(candidate) > 2*len(wrapper) {
			return candidate[len(wrapper) : len(candidate)-len(wrapper)], true
		}
	}
	return candidate, false
}

func intelCheckShouldStripControlledUnit(expected, mode string) bool {
	if mode == IntelCheckMatchNumeric {
		return true
	}
	if mode != "" && mode != IntelCheckMatchExact {
		return false
	}
	normalized := normalizeIntelCheckFullWidth(strings.Trim(expected, intelCheckTrimCutset))
	return intelCheckPlainNumberRe.MatchString(normalized)
}
