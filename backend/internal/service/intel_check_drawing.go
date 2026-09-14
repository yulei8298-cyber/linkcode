package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	// Markdown 代码块，模型通常把 HTML 放在 ```html 里。
	intelCheckCodeBlockRe = regexp.MustCompile("(?is)```[a-z]*\\s*?\\n(.*?)```")
	// 裸 HTML 文档。
	intelCheckHTMLDocRe = regexp.MustCompile(`(?is)<!doctype\s+html.*?</html\s*>|<html[\s>].*?</html\s*>`)
	// 裸 SVG 片段，作为最后的兜底。
	intelCheckSVGRe = regexp.MustCompile(`(?is)<svg[\s>].*?</svg\s*>`)

	errIntelCheckNoDrawing = errors.New("intel check: 回复中未找到 HTML 或 SVG 产物")
	errIntelCheckTooLarge  = errors.New("intel check: 画作体积超过上限")
)

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

// SanitizeIntelCheckDrawing 原样返回模型产出，只做体积检查。
//
// ## 为什么不再清洗
//
// 早期版本会用 golang.org/x/net/html 解析产物、剔除危险标签与脚本、再序列化回来。
// 这条路在本功能上是错的，原因不是安全考量不成立，而是**它破坏了本功能唯一要做的事**：
//
//   - 这张页面的全部价值在于「我们如实展示模型画出了什么」。任何改写都让展示物
//     不再等于产出物，判定依据也跟着失真——门禁数的是清洗后的指标，
//     评审模型读的是清洗后的源码，而用户看到的动画也是清洗后的效果。
//     三者一起偏离，却没有任何一环会报错。
//   - HTML 解析器按 HTML 规则处理 SVG，往返一趟就可能动到自闭合写法、
//     属性大小写（viewBox / attributeName / repeatCount 这些 SVG 必须保留驼峰）
//     与外来元素的序列化形式。这类改动不会报错，只会让动画悄悄不动。
//   - 脚本按关键词整段剔除的规则尤其粗暴：动画代码里出现一个名为 location
//     的局部变量就会让整段动画消失，而表现出来只是「这个模型画不出动画」。
//
// ## 那安全靠什么
//
// 靠渲染侧的沙箱，而且它本来就是三层里真正起作用的那层：
// 预览用 <iframe sandbox="allow-scripts"> 且**不给** allow-same-origin，
// 产物因此运行在不透明源里——拿不到父页面 DOM、cookie、storage，
// 也不能做顶层导航或弹窗。清洗只是锦上添花的第二层，
// 而它的代价是让本功能失去可信度，这笔交易不划算。
//
// **改动 SvgArtworkPreview.vue 的 sandbox 属性前请先回到这里。**
// 一旦那里补上 allow-same-origin，沙箱即告失效，而此处已无清洗兜底。
//
// 体积上限保留：它与保真无关，是防止一份几十 MB 的产物拖垮页面与数据库。
func SanitizeIntelCheckDrawing(source string, maxBytes int) (string, error) {
	if maxBytes > 0 && len(source) > maxBytes {
		return "", fmt.Errorf("%w: %d > %d", errIntelCheckTooLarge, len(source), maxBytes)
	}
	return source, nil
}
