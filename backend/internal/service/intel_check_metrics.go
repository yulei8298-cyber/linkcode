package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// 绘图题结构指标的元素分类表。
var (
	// 计入造型数量的 SVG 图元。
	drawingShapeTags = map[string]bool{
		"path": true, "circle": true, "ellipse": true, "rect": true,
		"line": true, "polygon": true, "polyline": true, "use": true, "text": true,
	}
	// SMIL 动画元素，其父节点视为被驱动的动画目标。
	drawingSMILTags = map[string]bool{
		"animate": true, "animatetransform": true, "animatemotion": true,
		"animatecolor": true, "set": true,
	}
	// <defs> 内可被复用的符号类元素，需带 id 才计数。
	drawingDefsTags = map[string]bool{
		"g": true, "symbol": true, "pattern": true, "marker": true,
		"lineargradient": true, "radialgradient": true, "filter": true,
		"mask": true, "clippath": true,
	}
)

// 动画机制标识，用于 DrawingMetrics.Mechanisms。
const (
	DrawingMechanismSMIL   = "smil"
	DrawingMechanismCSS    = "css"
	DrawingMechanismScript = "script"
)

var (
	drawingCSSCommentRe  = regexp.MustCompile(`/\*[\s\S]*?\*/`)
	drawingKeyframesRe   = regexp.MustCompile(`(?i)@(?:-webkit-|-moz-)?keyframes\s+("[^"]+"|'[^']+'|[A-Za-z_][\w-]*)`)
	drawingSelectorRe    = regexp.MustCompile(`([.#])([A-Za-z_][\w-]*)`)
	drawingJSDriverRe    = regexp.MustCompile(`requestAnimationFrame|setInterval\s*\(|\.animate\s*\(`)
	drawingJSStringRe    = regexp.MustCompile(`"([^"\\\n]{1,64})"|'([^'\\\n]{1,64})'`)
	drawingJSMemberRe    = regexp.MustCompile(`\.([A-Za-z_][\w$]{0,63})`)
	errDrawingNoSVGFound = errors.New("intel check: 绘图结果中未找到可解析的 <svg> 元素")
)

// DrawingMetrics 绘图题的结构指标。
//
// 参考稿与候选稿使用同一套算法计算，判定时比较两者比值，
// 因此各项指标只需保证「同算法可比」，不追求与浏览器渲染结果绝对一致。
type DrawingMetrics struct {
	ShapeCount      int      `json:"shape_count"`
	AnimatedTargets int      `json:"animated_targets"`
	DefsSymbols     int      `json:"defs_symbols"`
	PathDataBytes   int      `json:"path_data_bytes"`
	HTMLBytes       int      `json:"html_bytes"`
	Mechanisms      []string `json:"mechanisms"`
	HasSVG          bool     `json:"has_svg"`
	HasTitle        bool     `json:"has_title"`
	HasDesc         bool     `json:"has_desc"`
}

// HasMechanism 判断是否存在指定动画机制。
func (m DrawingMetrics) HasMechanism(name string) bool {
	for _, item := range m.Mechanisms {
		if item == name {
			return true
		}
	}
	return false
}

// EncodeDrawingMetrics 把结构指标转成可写入 JSONB 列的 map。
//
// ent schema 无法 import service 包（会形成循环依赖），因此 reference_metrics
// 列只能声明为 map[string]any，存取两端靠这对函数做转换。走 JSON 往返而非
// 手写字段拷贝，是为了让 map 的键名始终等于 DrawingMetrics 的 json tag——
// 手写拷贝会在新增指标时被漏掉，且漏掉时没有任何编译期提示。
func EncodeDrawingMetrics(metrics DrawingMetrics) (map[string]any, error) {
	raw, err := json.Marshal(metrics)
	if err != nil {
		return nil, fmt.Errorf("encode drawing metrics: %w", err)
	}
	out := make(map[string]any)
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("encode drawing metrics: %w", err)
	}
	return out, nil
}

// DecodeDrawingMetrics 从 JSONB 列还原结构指标。
//
// 入参为 nil 或空 map 时返回零值且不报错：绘图题尚未上传参考稿是正常状态，
// 此时门禁会跳过全部相对指标项（见 EvaluateIntelCheckGate），
// 不能因为「没有参考稿」就把所有分组判成失败。
func DecodeDrawingMetrics(raw map[string]any) (DrawingMetrics, error) {
	if len(raw) == 0 {
		return DrawingMetrics{}, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return DrawingMetrics{}, fmt.Errorf("decode drawing metrics: %w", err)
	}
	var metrics DrawingMetrics
	if err := json.Unmarshal(encoded, &metrics); err != nil {
		return DrawingMetrics{}, fmt.Errorf("decode drawing metrics: %w", err)
	}
	return metrics, nil
}

// drawingScanner 单次 DOM 遍历收集全部指标所需的中间状态。
type drawingScanner struct {
	metrics    DrawingMetrics
	idIndex    map[string]*html.Node
	classIndex map[string][]*html.Node
	animated   map[*html.Node]struct{}
	style      strings.Builder
	script     strings.Builder
}

// ComputeDrawingMetrics 解析一份 HTML（内嵌 SVG）并计算结构指标。
//
// 入参应为已清洗的 HTML 原文；解析失败或不含 <svg> 时返回错误，
// 由调用方按「未产出有效画作」处理。
func ComputeDrawingMetrics(source string) (DrawingMetrics, error) {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return DrawingMetrics{}, err
	}

	scanner := &drawingScanner{
		idIndex:    make(map[string]*html.Node),
		classIndex: make(map[string][]*html.Node),
		animated:   make(map[*html.Node]struct{}),
	}
	scanner.walk(doc, false)
	if !scanner.metrics.HasSVG {
		return DrawingMetrics{}, errDrawingNoSVGFound
	}

	if len(scanner.animated) > 0 {
		scanner.metrics.Mechanisms = append(scanner.metrics.Mechanisms, DrawingMechanismSMIL)
	}
	if scanner.applyCSSAnimations() {
		scanner.metrics.Mechanisms = append(scanner.metrics.Mechanisms, DrawingMechanismCSS)
	}
	if scanner.applyScriptAnimations() {
		scanner.metrics.Mechanisms = append(scanner.metrics.Mechanisms, DrawingMechanismScript)
	}

	scanner.metrics.AnimatedTargets = len(scanner.animated)
	scanner.metrics.HTMLBytes = len(source)
	return scanner.metrics, nil
}

// walk 深度遍历文档；inSVG 标记当前是否位于 <svg> 子树内，
// 用于把 <head><title> 与页面控件排除在画作指标之外。
func (s *drawingScanner) walk(node *html.Node, inSVG bool) {
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		switch tag {
		case "svg":
			inSVG = true
			s.metrics.HasSVG = true
		case "style":
			s.style.WriteString(drawingNodeText(node))
		case "script":
			s.script.WriteString(drawingNodeText(node))
		}
		if inSVG {
			s.indexElement(node, tag)
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		s.walk(child, inSVG)
	}
}

// indexElement 累计单个 SVG 内元素的各项指标，并建立 id/class 索引。
func (s *drawingScanner) indexElement(node *html.Node, tag string) {
	if drawingShapeTags[tag] {
		s.metrics.ShapeCount++
	}
	switch tag {
	case "title":
		s.metrics.HasTitle = true
	case "desc":
		s.metrics.HasDesc = true
	}
	if drawingSMILTags[tag] {
		if parent := node.Parent; parent != nil && parent.Type == html.ElementNode {
			s.animated[parent] = struct{}{}
		}
	}

	hasID := false
	for _, attr := range node.Attr {
		switch strings.ToLower(attr.Key) {
		case "id":
			if attr.Val == "" {
				continue
			}
			hasID = true
			if _, exists := s.idIndex[attr.Val]; !exists {
				s.idIndex[attr.Val] = node
			}
		case "class":
			for _, name := range strings.Fields(attr.Val) {
				s.classIndex[name] = append(s.classIndex[name], node)
			}
		case "d":
			s.metrics.PathDataBytes += len(attr.Val)
		}
	}
	if hasID && drawingDefsTags[tag] && drawingInsideDefs(node) {
		s.metrics.DefsSymbols++
	}
}

// applyCSSAnimations 找出被 @keyframes 动画驱动的元素，返回是否命中 CSS 机制。
func (s *drawingScanner) applyCSSAnimations() bool {
	css := drawingCSSCommentRe.ReplaceAllString(s.style.String(), "")
	if strings.TrimSpace(css) == "" {
		return false
	}
	names := make(map[string]bool)
	for _, match := range drawingKeyframesRe.FindAllStringSubmatch(css, -1) {
		names[strings.Trim(match[1], `"'`)] = true
	}
	if len(names) == 0 {
		return false
	}

	hit := false
	drawingWalkCSSRules(css, func(selector, declarations string) {
		if !strings.Contains(strings.ToLower(declarations), "animation") {
			return
		}
		referenced := false
		for name := range names {
			if strings.Contains(declarations, name) {
				referenced = true
				break
			}
		}
		if !referenced {
			return
		}
		for _, token := range drawingSelectorRe.FindAllStringSubmatch(selector, -1) {
			if token[1] == "#" {
				if node, ok := s.idIndex[token[2]]; ok {
					s.animated[node] = struct{}{}
					hit = true
				}
				continue
			}
			for _, node := range s.classIndex[token[2]] {
				s.animated[node] = struct{}{}
				hit = true
			}
		}
	})
	return hit
}

// applyScriptAnimations 处理脚本驱动的动画。
//
// 脚本常把目标 id 收进数组再统一 getElementById（参考稿即如此），
// 逐调用点追踪数据流不可行；因此取「脚本中出现的字符串字面量与属性名」
// 与「SVG 内真实存在的 id / class」求交集作为代理指标。
func (s *drawingScanner) applyScriptAnimations() bool {
	script := s.script.String()
	if strings.TrimSpace(script) == "" || !drawingJSDriverRe.MatchString(script) {
		return false
	}

	tokens := make(map[string]bool)
	for _, match := range drawingJSStringRe.FindAllStringSubmatch(script, -1) {
		for _, group := range match[1:] {
			if group != "" {
				tokens[group] = true
			}
		}
	}
	for _, match := range drawingJSMemberRe.FindAllStringSubmatch(script, -1) {
		tokens[match[1]] = true
	}

	hit := false
	for token := range tokens {
		name := strings.TrimLeft(token, "#.")
		if node, ok := s.idIndex[name]; ok {
			s.animated[node] = struct{}{}
			hit = true
		}
		for _, node := range s.classIndex[name] {
			s.animated[node] = struct{}{}
			hit = true
		}
	}
	return hit
}

// drawingNodeText 取元素的直接文本内容，用于提取 <style> / <script> 正文。
func drawingNodeText(node *html.Node) string {
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			builder.WriteString(child.Data)
		}
	}
	return builder.String()
}

// drawingInsideDefs 判断元素是否位于 <defs> 内。
func drawingInsideDefs(node *html.Node) bool {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if parent.Type != html.ElementNode {
			continue
		}
		switch strings.ToLower(parent.Data) {
		case "defs":
			return true
		case "svg":
			return false
		}
	}
	return false
}

// drawingWalkCSSRules 遍历 CSS 规则：@keyframes 块跳过，
// @media/@supports 等条件块递归进入，其余按「选择器 + 声明块」回调。
func drawingWalkCSSRules(css string, fn func(selector, declarations string)) {
	for index := 0; index < len(css); {
		open := strings.IndexByte(css[index:], '{')
		if open < 0 {
			return
		}
		open += index
		prelude := strings.TrimSpace(css[index:open])

		depth := 1
		cursor := open + 1
		for ; cursor < len(css) && depth > 0; cursor++ {
			switch css[cursor] {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		end := cursor
		if depth == 0 {
			end = cursor - 1
		}
		body := css[min(open+1, len(css)):max(open+1, end)]

		switch {
		case strings.HasPrefix(prelude, "@"):
			if !strings.Contains(strings.ToLower(prelude), "keyframes") {
				drawingWalkCSSRules(body, fn)
			}
		case prelude != "":
			fn(prelude, body)
		}
		index = cursor
	}
}
