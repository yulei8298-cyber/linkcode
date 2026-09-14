package handler

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 独立 HTTP 文档不继承控制台的 nonce CSP，允许沙箱内的原始动画脚本运行。
// 权限只作用于这个无数据外壳，绝不能给主站放开 unsafe-inline。
// CSP sandbox 同时保护直接打开的外壳；其中也不能出现 allow-same-origin。
const intelCheckPreviewCSP = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:; frame-src 'self'; connect-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'; sandbox allow-scripts"

//go:embed templates/intel_check_preview.html
var intelCheckPreviewHTML []byte

// Preview 提供静态预览外壳，不接收 URL 中的 HTML，也不读取数据库或凭据。
// 刻意不检查功能开关：管理端在开启功能之前也需要预览标定产物。
// 两个公开数据接口仍受开关保护，此端点只返回固定的空白加载器。
func (h *IntelCheckHandler) Preview(c *gin.Context) {
	c.Header("Content-Security-Policy", intelCheckPreviewCSP)
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", intelCheckPreviewHTML)
}
