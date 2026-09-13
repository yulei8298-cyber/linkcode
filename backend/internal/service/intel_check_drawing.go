package service

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var (
	// Markdown 代码块，模型通常把 HTML 放在 ```html 里。
	intelCheckCodeBlockRe = regexp.MustCompile("(?is)```[a-z]*\\s*?\\n(.*?)```")
	// 裸 HTML 文档。
	intelCheckHTMLDocRe = regexp.MustCompile(`(?is)<!doctype\s+html.*?</html\s*>|<html[\s>].*?</html\s*>`)
	// 裸 SVG 片段，作为最后的兜底。
	intelCheckSVGRe = regexp.MustCompile(`(?is)<svg[\s>].*?</svg\s*>`)

	// 脚本中一旦出现这些能力，即视为非「纯计算脚本」，整段剔除。
	intelCheckUnsafeScriptRe = regexp.MustCompile(`(?i)fetch\s*\(|XMLHttpRequest|WebSocket|EventSource|importScripts|\bimport\s*\(|sendBeacon|document\s*\.\s*cookie|localStorage|sessionStorage|indexedDB|postMessage|\bWorker\s*\(|\beval\s*\(|new\s+Function|\blocation\b|\btop\s*\.|\bparent\s*\.`)

	// 整体剔除的元素。
	intelCheckDropTags = map[string]bool{
		"iframe": true, "object": true, "embed": true, "applet": true,
		"frame": true, "frameset": true, "link": true, "base": true,
		"form": true, "noscript": true,
	}

	errIntelCheckNoDrawing  = errors.New("intel check: 回复中未找到 HTML 或 SVG 产物")
	errIntelCheckTooLarge   = errors.New("intel check: 画作体积超过上限")
	intelCheckExternalRefRe = regexp.MustCompile(`(?i)^\s*(?:https?:)?//|^\s*data:text/html`)
)

// 注入到产物 <head> 的内容安全策略：禁网络、禁外链，只放开内联样式与内联脚本。
const intelCheckCSP = "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data:"

// ExtractIntelCheckDrawing 从模型回复中抽取绘图产物。
//
// 依次尝试：Markdown 代码块中含 svg/html 的块 → 裸 HTML 文档 → 裸 SVG 片段；
// 多个候选时取最长的一个（通常是完整作品，而非片段示例）。
func ExtractIntelCheckDrawing(reply string) (string, error) {
	candidates := make([]string, 0, 4)

	for _, match := range intelCheckCodeBlockRe.FindAllStringSubmatch(reply, -1) {
		block := strings.TrimSpace(match[1])
		if intelCheckLooksLikeDrawing(block) {
			candidates = append(candidates, block)
		}
	}
	if len(candidates) == 0 {
		candidates = append(candidates, intelCheckHTMLDocRe.FindAllString(reply, -1)...)
	}
	if len(candidates) == 0 {
		candidates = append(candidates, intelCheckSVGRe.FindAllString(reply, -1)...)
	}

	best := ""
	for _, candidate := range candidates {
		if len(candidate) > len(best) {
			best = candidate
		}
	}
	if strings.TrimSpace(best) == "" {
		return "", errIntelCheckNoDrawing
	}
	return strings.TrimSpace(best), nil
}

// intelCheckLooksLikeDrawing 判断一段文本是否像绘图产物。
func intelCheckLooksLikeDrawing(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "<svg") || strings.Contains(lower, "<html") || strings.Contains(lower, "<!doctype html")
}

// SanitizeIntelCheckDrawing 清洗绘图产物，返回可安全渲染的 HTML。
//
// 清洗规则：
//   - 剔除 iframe/object/embed/link/base/form 等具备外联或嵌套文档能力的元素；
//   - 剔除所有内联事件属性（on*）与指向外部地址的 src/href/xlink:href；
//   - 脚本按内容取舍：含外联/逃逸 API 的整段剔除，纯计算脚本保留
//     （参考稿的动画即由 requestAnimationFrame 驱动，一律删脚本会让满血稿失去动画）；
//   - 在 <head> 注入 CSP meta，使产物无论在何处渲染都无法发起网络请求。
func SanitizeIntelCheckDrawing(source string, maxBytes int) (string, error) {
	if maxBytes > 0 && len(source) > maxBytes {
		return "", fmt.Errorf("%w: %d > %d", errIntelCheckTooLarge, len(source), maxBytes)
	}

	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", fmt.Errorf("intel check: 解析画作失败: %w", err)
	}

	var doomed []*html.Node
	var head *html.Node
	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			tag := strings.ToLower(node.Data)
			switch {
			case intelCheckDropTags[tag]:
				doomed = append(doomed, node)
				return
			case tag == "meta" && intelCheckHasHTTPEquiv(node):
				doomed = append(doomed, node)
				return
			case tag == "script":
				if intelCheckUnsafeScriptRe.MatchString(drawingNodeText(node)) {
					doomed = append(doomed, node)
					return
				}
			case tag == "head" && head == nil:
				head = node
			}
			intelCheckStripAttrs(node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	for _, node := range doomed {
		if node.Parent != nil {
			node.Parent.RemoveChild(node)
		}
	}
	if head != nil {
		head.InsertBefore(intelCheckCSPNode(), head.FirstChild)
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return "", fmt.Errorf("intel check: 序列化画作失败: %w", err)
	}
	if maxBytes > 0 && buf.Len() > maxBytes {
		return "", fmt.Errorf("%w: %d > %d", errIntelCheckTooLarge, buf.Len(), maxBytes)
	}
	return buf.String(), nil
}

// intelCheckStripAttrs 剔除内联事件属性与指向外部地址的引用属性。
func intelCheckStripAttrs(node *html.Node) {
	kept := node.Attr[:0]
	for _, attr := range node.Attr {
		key := strings.ToLower(attr.Key)
		if strings.HasPrefix(key, "on") {
			continue
		}
		switch key {
		case "src", "href", "xlink:href", "srcset", "poster", "formaction", "action":
			if intelCheckExternalRefRe.MatchString(attr.Val) {
				continue
			}
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(attr.Val)), "javascript:") {
				continue
			}
		}
		kept = append(kept, attr)
	}
	node.Attr = kept
}

// intelCheckHasHTTPEquiv 判断 meta 是否带 http-equiv（可用于跳转/刷新）。
func intelCheckHasHTTPEquiv(node *html.Node) bool {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, "http-equiv") {
			return true
		}
	}
	return false
}

// intelCheckCSPNode 构造注入用的 CSP meta 节点。
func intelCheckCSPNode() *html.Node {
	return &html.Node{
		Type: html.ElementNode,
		Data: "meta",
		Attr: []html.Attribute{
			{Key: "http-equiv", Val: "Content-Security-Policy"},
			{Key: "content", Val: intelCheckCSP},
		},
	}
}
