package service

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

const intelCheckStructureVersion = "structure_v2"

// 只统计数值的个数；空白、逗号、小数位和科学计数法不改变贡献。
var intelCheckGeometryNumber = regexp.MustCompile(`[-+]?(?:[0-9]*\.[0-9]+|[0-9]+\.?[0-9]*)(?:[eE][-+]?[0-9]+)?`)

type IntelCheckStructureMetrics struct {
	Shapes         int `json:"shapes"`
	GeometryValues int `json:"geometry_values"`
}

// ComputeIntelCheckStructureMetrics 只读主 SVG 的静态结构，不执行产物脚本。
// use 在引用位置展开；未引用的定义和页面控件不贡献结构量。
// 这不是渲染或运动学验证，CSS 隐藏、遮挡及脚本动态生成的结构不在覆盖范围内。
func ComputeIntelCheckStructureMetrics(source string) (IntelCheckStructureMetrics, error) {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return IntelCheckStructureMetrics{}, err
	}
	// 选择图元最多的 SVG，避免页面前置的播放按钮或徽标抢占主画作。
	var root *html.Node
	best := -1
	var find func(*html.Node) int
	find = func(n *html.Node) int {
		count := 0
		if n.Type == html.ElementNode && drawingShapeTags[strings.ToLower(n.Data)] {
			count++
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			count += find(child)
		}
		if n.Type == html.ElementNode && n.Data == "svg" && count > best {
			root, best = n, count
		}
		return count
	}
	find(doc)
	if root == nil {
		return IntelCheckStructureMetrics{}, errDrawingNoSVGFound
	}
	ids := map[string]*html.Node{}
	var index func(*html.Node)
	index = func(n *html.Node) {
		if id := intelCheckNodeAttr(n, "id"); id != "" {
			ids[id] = n
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			index(child)
		}
	}
	index(root)
	var metrics IntelCheckStructureMetrics
	active := map[*html.Node]bool{}
	visited := 0
	var walk func(*html.Node, int) error
	walk = func(n *html.Node, depth int) error {
		if n.Type != html.ElementNode {
			return nil
		}
		visited++
		if depth > 128 || visited > 20000 || active[n] {
			return fmt.Errorf("SVG 引用循环或展开结构超过上限")
		}
		active[n] = true
		defer delete(active, n)
		tag := strings.ToLower(n.Data)
		switch tag {
		case "defs", "style", "script", "title", "desc", "pattern", "clippath", "mask", "filter", "lineargradient", "radialgradient":
			return nil
		case "use":
			href := intelCheckNodeAttr(n, "href")
			if !strings.HasPrefix(href, "#") || ids[strings.TrimPrefix(href, "#")] == nil {
				return fmt.Errorf("SVG use 未指向有效的本地定义")
			}
			return walk(ids[strings.TrimPrefix(href, "#")], depth+1)
		}
		if drawingShapeTags[tag] {
			metrics.Shapes++
			switch tag {
			case "path":
				metrics.GeometryValues += len(intelCheckGeometryNumber.FindAllStringIndex(intelCheckNodeAttr(n, "d"), -1))
			case "polygon", "polyline":
				metrics.GeometryValues += len(intelCheckGeometryNumber.FindAllStringIndex(intelCheckNodeAttr(n, "points"), -1))
			case "circle":
				metrics.GeometryValues += 3
			case "ellipse", "line":
				metrics.GeometryValues += 4
			case "rect":
				metrics.GeometryValues += 6
			case "text":
				metrics.GeometryValues += 2
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if err := walk(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	err = walk(root, 0)
	return metrics, err
}

func intelCheckNodeAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

// 多份标准的中位数降低单份堆料参考稿对阈值的影响；不读取调用方提供的指标。
func intelCheckStructureBaseline(primary string, standards []string) (IntelCheckStructureMetrics, int, error) {
	if len(standards) > 7 {
		return IntelCheckStructureMetrics{}, 0, fmt.Errorf("最多配置 8 份标准样本")
	}
	sources := append([]string{primary}, standards...)
	seen := map[string]bool{}
	var shapes, geometry []int
	for _, source := range sources {
		source = strings.TrimSpace(source)
		if source == "" || seen[source] {
			continue
		}
		if len(source) > maxIntelCheckReferenceHTMLBytes {
			return IntelCheckStructureMetrics{}, 0, fmt.Errorf("标准样本超过 %d 字节上限", maxIntelCheckReferenceHTMLBytes)
		}
		metrics, err := ComputeIntelCheckStructureMetrics(source)
		if err != nil || metrics.Shapes == 0 || metrics.GeometryValues == 0 {
			return IntelCheckStructureMetrics{}, 0, fmt.Errorf("标准样本无法提供有效的静态结构")
		}
		seen[source] = true
		shapes = append(shapes, metrics.Shapes)
		geometry = append(geometry, metrics.GeometryValues)
	}
	if len(shapes) == 0 {
		return IntelCheckStructureMetrics{}, 0, fmt.Errorf("请先配置至少一份标准样本")
	}
	median := func(values []int) int {
		sort.Ints(values)
		middle := len(values) / 2
		if len(values)%2 != 0 {
			return values[middle]
		}
		return (values[middle-1] + values[middle]) / 2
	}
	return IntelCheckStructureMetrics{Shapes: median(shapes), GeometryValues: median(geometry)}, len(shapes), nil
}

func intelCheckStructureScore(candidate, baseline IntelCheckStructureMetrics) float64 {
	// 两个相关但不同的代理指标等权，单项封顶，堆一类元素不能无限补偿另一类。
	return 50 * (min(1, float64(candidate.Shapes)/float64(baseline.Shapes)) +
		min(1, float64(candidate.GeometryValues)/float64(baseline.GeometryValues)))
}

func intelCheckStandardDigest(primary string, standards []string) string {
	seen := map[string]bool{}
	var sources []string
	for _, source := range append([]string{primary}, standards...) {
		source = strings.TrimSpace(source)
		if source != "" && !seen[source] {
			seen[source] = true
			sources = append(sources, source)
		}
	}
	sort.Strings(sources)
	hash := sha256.New()
	for _, source := range sources {
		_, _ = fmt.Fprintf(hash, "%d:%s", len(source), source)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
