package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

// 结构门禁的默认阈值。
const (
	intelCheckDefaultMinRatio = 0.7
	intelCheckDefaultMaxBytes = 256 * 1024
)

// IntelCheckDrawingRules 绘图题第一层门禁的配置，存于题目的 drawing_rules 字段。
type IntelCheckDrawingRules struct {
	// MinRatio 各结构指标相对参考稿的最低比例。
	MinRatio float64 `json:"min_ratio"`
	// RequiredKeywords 产物中必须出现的关键词（取自题面要素）。
	RequiredKeywords []string `json:"required_keywords"`
	// MaxBytes 清洗后产物的体积上限。
	MaxBytes int `json:"max_bytes"`
}

// Normalize 兜底非法配置。
func (r *IntelCheckDrawingRules) Normalize() {
	if r.MinRatio <= 0 || r.MinRatio > 1 {
		r.MinRatio = intelCheckDefaultMinRatio
	}
	if r.MaxBytes <= 0 {
		r.MaxBytes = intelCheckDefaultMaxBytes
	}
}

// DecodeIntelCheckDrawingRules 把题目 drawing_rules 列的 JSONB 内容还原成门禁配置。
//
// 走 JSON 往返而非逐键类型断言：JSONB 读出来的 map 里数字一律是 float64，
// 手写断言得为 MinRatio 和 MaxBytes 各写一套转换，而 json.Unmarshal
// 按结构体字段类型直接转。这与 DecodeDrawingMetrics 的处理方式保持一致。
//
// 空输入返回零值且不报错：题目尚未配置门禁规则是合法状态，由调用方
// Normalize 补默认值。
func DecodeIntelCheckDrawingRules(raw map[string]any) (IntelCheckDrawingRules, error) {
	rules := IntelCheckDrawingRules{}
	if len(raw) == 0 {
		return rules, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return rules, fmt.Errorf("marshal intel check drawing rules: %w", err)
	}
	if err := json.Unmarshal(encoded, &rules); err != nil {
		return rules, fmt.Errorf("decode intel check drawing rules: %w", err)
	}
	return rules, nil
}

// EncodeIntelCheckDrawingRules 把门禁配置转成可写入 JSONB 列的 map。
func EncodeIntelCheckDrawingRules(rules IntelCheckDrawingRules) (map[string]any, error) {
	encoded, err := json.Marshal(rules)
	if err != nil {
		return nil, fmt.Errorf("marshal intel check drawing rules: %w", err)
	}
	out := make(map[string]any)
	if err := json.Unmarshal(encoded, &out); err != nil {
		return nil, fmt.Errorf("encode intel check drawing rules: %w", err)
	}
	return out, nil
}

// IntelCheckGateItem 单条门禁结果，写入 results.judge_detail 供前端逐条展示。
type IntelCheckGateItem struct {
	Item   string `json:"item"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

// IntelCheckGateResult 第一层门禁的汇总结果。
type IntelCheckGateResult struct {
	Pass  bool                 `json:"pass"`
	Items []IntelCheckGateItem `json:"items"`
}

// EvaluateIntelCheckGate 执行绘图题第一层结构门禁。
//
// 分两类检查：
//   - 固定项：可解析的 <svg>、同时具备 <title> 与 <desc>、命中必需关键词、
//     至少一种动画机制、体积不超上限；
//   - 相对项：造型数量 / 动画目标数 / 可复用符号数 / 路径数据量不低于参考稿的
//     MinRatio 倍。参考稿对应指标为 0 时（尚未上传参考稿）跳过该项，
//     避免无参考时误杀。产物体积**不在相对项之列**，理由见下方循环处的注释。
//
// 全部通过才进入第二层源码评审。
func EvaluateIntelCheckGate(source string, candidate, reference DrawingMetrics, rules IntelCheckDrawingRules) IntelCheckGateResult {
	rules.Normalize()
	result := IntelCheckGateResult{Pass: true, Items: make([]IntelCheckGateItem, 0, 12)}

	add := func(item string, pass bool, format string, args ...any) {
		result.Items = append(result.Items, IntelCheckGateItem{
			Item:   item,
			Pass:   pass,
			Detail: fmt.Sprintf(format, args...),
		})
		if !pass {
			result.Pass = false
		}
	}

	add("包含可解析的 SVG", candidate.HasSVG, "has_svg=%v", candidate.HasSVG)
	add("包含 title 与 desc 说明", candidate.HasTitle && candidate.HasDesc,
		"has_title=%v, has_desc=%v", candidate.HasTitle, candidate.HasDesc)

	missing := intelCheckMissingKeywords(source, rules.RequiredKeywords)
	add("命中必需关键词", len(missing) == 0, "缺失: %s", intelCheckJoinOrNone(missing))

	add("存在动画机制", len(candidate.Mechanisms) > 0,
		"mechanisms=%s", intelCheckJoinOrNone(candidate.Mechanisms))

	add("体积未超上限", candidate.HTMLBytes <= rules.MaxBytes,
		"html_bytes=%d, max=%d", candidate.HTMLBytes, rules.MaxBytes)

	// 相对项只取「画了多少东西」这类与表达方式无关的指标。
	//
	// 刻意**不含产物体积**：字节数量的是代码啰嗦程度，不是画作质量。
	// 一份压缩过的优秀产物（CSS 压成一行、path 紧凑无空格）体积可能只有
	// 格式宽松参考稿的三分之一，按比例一律判失败——而压缩恰恰是更熟练的写法。
	// 体积仍由上面的固定项「体积未超上限」把住，那一条防的是几十 MB 的失控产物，
	// 与保真无关。
	//
	// 路径数据量保留但需知其局限：`M0 0L10 10` 与 `M 0 0 L 10 10` 字节数不同，
	// 该项对空白写法有轻度敏感，只是远不如整体体积那么失真。
	for _, metric := range []struct {
		name      string
		got, want int
	}{
		{"造型数量", candidate.ShapeCount, reference.ShapeCount},
		{"动画目标数", candidate.AnimatedTargets, reference.AnimatedTargets},
		{"可复用符号数", candidate.DefsSymbols, reference.DefsSymbols},
		{"路径数据量", candidate.PathDataBytes, reference.PathDataBytes},
	} {
		if metric.want <= 0 {
			add(metric.name, true, "参考稿未提供该指标，跳过")
			continue
		}
		threshold := float64(metric.want) * rules.MinRatio
		ratio := float64(metric.got) / float64(metric.want)
		add(metric.name, float64(metric.got) >= threshold,
			"%d / 参考 %d = %.0f%%（要求 ≥ %.0f%%）",
			metric.got, metric.want, ratio*100, rules.MinRatio*100)
	}

	return result
}

// intelCheckMissingKeywords 返回产物中缺失的关键词。
func intelCheckMissingKeywords(source string, keywords []string) []string {
	missing := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if !strings.Contains(source, keyword) {
			missing = append(missing, keyword)
		}
	}
	return missing
}

// intelCheckJoinOrNone 拼接列表，空列表返回「无」，避免详情里出现空字符串。
func intelCheckJoinOrNone(items []string) string {
	if len(items) == 0 {
		return "无"
	}
	return strings.Join(items, ", ")
}
