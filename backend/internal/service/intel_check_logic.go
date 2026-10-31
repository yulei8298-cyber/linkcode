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
	// LaTeX 盒装结论中的常见文本/样式命令。真正的花括号配对由解析器处理，
	// 这里仅负责把已经取出的内容还原成便于比较的可见文本。
	intelCheckLatexTextCommandRe = regexp.MustCompile(`\\(?:text|textrm|textnormal|mathrm|mathbf|mathit|operatorname)\s*\{([^{}]*)\}`)
	intelCheckLatexSpacingRe     = regexp.MustCompile(`\\(?:,|;|:|!|quad|qquad)\s*`)
	// 「最终答案：X」这类显式作答句式，信号最强。
	intelCheckAnswerPhraseRe = regexp.MustCompile(`(?i)(?:最终答案|正确答案|答案是|答案为|答案|final answer|answer)[ 	]*(?:是|为|[:：=])?[ 	]*([^\n。！？；;]{1,200})`)
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
// 先区分本题结论与附带条件，再按更正、最终结论、排版信号和位置选择候选。
// 标准答案不参与候选选择，避免把中间推理或注释中的正确数字当作结论。
func ExtractIntelCheckAnswer(reply string) string {
	return extractIntelCheckAnswer(reply, true)
}

// extractIntelCheckAnswer 的 stripControlledUnit 由题目期望答案决定：
// 纯数值期望可忽略受控单位；期望本身带单位或是文本时必须保留原答案语义。
func extractIntelCheckAnswer(reply string, stripControlledUnit bool) string {
	text := strings.TrimSpace(intelCheckFenceRe.ReplaceAllString(reply, "\n"))
	if text == "" {
		return ""
	}

	candidates := intelCheckBoxedAnswers(text, stripControlledUnit)
	candidates = append(candidates, intelCheckCaptureAnswers(text, intelCheckAnswerPhraseRe, stripControlledUnit, 2)...)
	candidates = append(candidates, intelCheckCaptureAnswers(text, intelCheckBoldRe, stripControlledUnit, 1)...)
	offset := 0
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if answer := normalizeIntelCheckAnswerCandidate(line, stripControlledUnit); answer != "" {
			candidates = append(candidates, intelCheckAnswerCandidate{answer: answer, start: offset, contextEnd: offset + len(line)})
		}
		offset += len(line) + 1
	}
	return selectIntelCheckAnswer(text, candidates)
}

// intelCheckBoxedAnswers 收集有效的 \boxed{...} / \fbox{...} 及原文偏移。
//
// 不用正则直接匹配内容，因为模型常输出 \boxed{21\text{颗}} 这类嵌套花括号；
// Go 正则不支持递归匹配，简单的 `[^}]+` 会在 \text 的右括号处提前截断。
func intelCheckBoxedAnswers(text string, stripControlledUnit bool) []intelCheckAnswerCandidate {
	const maxBoxedAnswerBytes = 1000
	tokens := []string{`\boxed{`, `\fbox{`}
	var candidates []intelCheckAnswerCandidate

	for _, token := range tokens {
		for offset := 0; offset < len(text); {
			relative := strings.Index(text[offset:], token)
			if relative < 0 {
				break
			}
			start := offset + relative
			contentStart := start + len(token)
			content, end, ok := intelCheckBalancedBraceContent(text, contentStart, maxBoxedAnswerBytes)
			if ok {
				if answer := normalizeIntelCheckBoxedAnswer(content, stripControlledUnit); answer != "" {
					candidates = append(candidates, intelCheckAnswerCandidate{answer: answer, start: start, contextEnd: start, signal: 3})
				}
				offset = end
				continue
			}
			offset = contentStart
		}
	}

	return candidates
}

// intelCheckBalancedBraceContent 从起始花括号后一字节开始读取，直到与它配对
// 的右花括号。转义花括号不参与层级计数，避免 `\{` / `\}` 扰乱边界。
func intelCheckBalancedBraceContent(text string, contentStart, maxBytes int) (string, int, bool) {
	depth := 1
	for index := contentStart; index < len(text); index++ {
		if index-contentStart > maxBytes {
			return "", contentStart, false
		}
		switch text[index] {
		case '\\':
			if index+1 < len(text) && (text[index+1] == '{' || text[index+1] == '}') {
				index++
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[contentStart:index], index + 1, true
			}
		}
	}
	return "", contentStart, false
}

func normalizeIntelCheckBoxedAnswer(answer string, stripControlledUnit bool) string {
	answer = strings.TrimSpace(answer)
	// 连续处理几轮，使 \mathbf{\text{蓝色}} 这类简单嵌套样式也能展开。
	for range 8 {
		next := intelCheckLatexTextCommandRe.ReplaceAllString(answer, "$1")
		if next == answer {
			break
		}
		answer = next
	}
	answer = intelCheckLatexSpacingRe.ReplaceAllString(answer, " ")
	return normalizeIntelCheckAnswerCandidate(answer, stripControlledUnit)
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

// intelCheckCaptureAnswers 保留匹配位置，供上下文判断使用。
func intelCheckCaptureAnswers(text string, pattern *regexp.Regexp, stripControlledUnit bool, signal int) []intelCheckAnswerCandidate {
	var candidates []intelCheckAnswerCandidate
	for _, match := range pattern.FindAllStringSubmatchIndex(text, -1) {
		if answer := normalizeIntelCheckAnswerCandidate(text[match[2]:match[3]], stripControlledUnit); answer != "" {
			candidates = append(candidates, intelCheckAnswerCandidate{answer: answer, start: match[0], contextEnd: match[2], signal: signal})
		}
	}
	return candidates
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
