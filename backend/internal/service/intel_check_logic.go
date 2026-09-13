package service

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	// 去掉代码块，避免把推理过程里的片段当成答案。
	intelCheckFenceRe = regexp.MustCompile("(?s)```.*?```")
	// 「最终答案：X」这类显式作答句式，信号最强。
	intelCheckAnswerPhraseRe = regexp.MustCompile(`(?i)(?:最终答案|正确答案|答案是|答案为|答案|final answer|answer)\s*(?:是|为|[:：=])?\s*([^\n。！？；;]{1,200})`)
	// Markdown 加粗片段，模型常用来强调结论。
	intelCheckBoldRe   = regexp.MustCompile(`\*\*([^*\n]{1,200})\*\*`)
	intelCheckNumberRe = regexp.MustCompile(`-?\d+(?:\.\d+)?`)
)

// 答案两端需要剥掉的空白、标点与 Markdown 装饰字符。
const intelCheckTrimCutset = " \t\r\n*`~\"'“”‘’()（）[]【】<>《》{}:：=。．.,，、;；!！?？"

// 数值比较容差，覆盖 21 与 21.0 这类等价写法。
const intelCheckNumericEpsilon = 1e-9

// ExtractIntelCheckAnswer 从模型回复中提取答案。
//
// 按信号强度依次尝试：显式作答句式 → 最后一处加粗片段 → 最后一个非空行。
// 三者都取「最后一次出现」，因为模型常先给推理再复述结论。
func ExtractIntelCheckAnswer(reply string) string {
	text := strings.TrimSpace(intelCheckFenceRe.ReplaceAllString(reply, "\n"))
	if text == "" {
		return ""
	}

	if candidate := intelCheckLastCapture(intelCheckAnswerPhraseRe, text); candidate != "" {
		return candidate
	}
	if candidate := intelCheckLastCapture(intelCheckBoldRe, text); candidate != "" {
		return candidate
	}

	lines := strings.Split(text, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if trimmed := strings.Trim(lines[index], intelCheckTrimCutset); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// MatchIntelCheckAnswer 按匹配模式判定答案是否正确。
//
// reply 为模型原始回复，extracted 为 ExtractIntelCheckAnswer 的结果。
// regex 与 contains 针对整段回复匹配（答案可能出现在句中）；
// exact 与 numeric 针对提取出的答案匹配（要求模型给出明确结论）。
//
// 返回 error 仅代表**题目配置有问题**（如管理端填了非法正则），
// 调用方应按 request_error 记录，不可算作模型降智。
func MatchIntelCheckAnswer(reply, extracted, expected, mode string) (bool, error) {
	expected = strings.TrimSpace(expected)
	if expected == "" && mode != IntelCheckMatchRegex {
		return false, fmt.Errorf("intel check: 题目未配置期望答案")
	}

	switch mode {
	case IntelCheckMatchRegex:
		pattern, err := regexp.Compile(expected)
		if err != nil {
			return false, fmt.Errorf("intel check: 题目正则非法: %w", err)
		}
		return pattern.MatchString(reply), nil

	case IntelCheckMatchContains:
		needle := normalizeIntelCheckText(expected)
		if needle == "" {
			return false, fmt.Errorf("intel check: 题目期望答案归一化后为空")
		}
		return strings.Contains(normalizeIntelCheckText(reply), needle), nil

	case IntelCheckMatchNumeric:
		want, ok := intelCheckFirstNumber(expected)
		if !ok {
			return false, fmt.Errorf("intel check: 题目期望答案不含数值: %q", expected)
		}
		got, ok := intelCheckFirstNumber(extracted)
		if !ok {
			// 提取结果里没有数字时退回整段回复，容忍「答案就是那个数」这类表述。
			got, ok = intelCheckFirstNumber(reply)
		}
		if !ok {
			return false, nil
		}
		return math.Abs(got-want) <= intelCheckNumericEpsilon, nil

	case IntelCheckMatchExact, "":
		return normalizeIntelCheckText(extracted) == normalizeIntelCheckText(expected), nil

	default:
		return false, fmt.Errorf("intel check: 未知匹配模式 %q", mode)
	}
}

// intelCheckLastCapture 返回正则最后一次匹配的首个捕获组，并剥掉两端装饰字符。
func intelCheckLastCapture(pattern *regexp.Regexp, text string) string {
	matches := pattern.FindAllStringSubmatch(text, -1)
	for index := len(matches) - 1; index >= 0; index-- {
		if trimmed := strings.Trim(matches[index][1], intelCheckTrimCutset); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// intelCheckFirstNumber 取文本中第一个数值。
func intelCheckFirstNumber(text string) (float64, bool) {
	match := intelCheckNumberRe.FindString(normalizeIntelCheckFullWidth(text))
	if match == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// normalizeIntelCheckText 归一化用于比较的文本：
// 全角转半角、去空白与标点、统一小写。
func normalizeIntelCheckText(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	for _, char := range strings.ToLower(normalizeIntelCheckFullWidth(text)) {
		if strings.ContainsRune(intelCheckTrimCutset, char) {
			continue
		}
		builder.WriteRune(char)
	}
	return builder.String()
}

// normalizeIntelCheckFullWidth 把全角字符与全角空格转为半角，
// 使「２１」与「21」等价。
func normalizeIntelCheckFullWidth(text string) string {
	return strings.Map(func(char rune) rune {
		switch {
		case char == '　':
			return ' '
		case char >= '！' && char <= '～':
			return char - 0xfee0
		default:
			return char
		}
	}, text)
}
