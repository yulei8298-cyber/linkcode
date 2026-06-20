package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterPublicRoutes 注册公开只读路由（无需认证）。
//
// 这些端点供「公开门户」使用，让访客无需登录即可查看渠道可用性、定价方案等只读信息。
// 数据可见性由各自的功能开关控制（渠道监控开关、可用渠道开关），关闭时返回空结果。
// 故意不挂 JWT 与 BackendModeAuthGuard：门户对外开放，由功能开关而非登录态决定可见性。
func RegisterPublicRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	settingService *service.SettingService,
) {
	public := v1.Group("/public")
	{
		// 渠道可用性检测（只读，复用用户视图，不依赖用户身份）
		public.GET("/channel-monitors", h.ChannelMonitor.PublicList)
		public.GET("/channel-monitors/:id/status", h.ChannelMonitor.PublicGetStatus)

		// 定价方案（只读，展示所有公开分组的定价）
		public.GET("/pricing", h.PublicPricing.List)
	}

	v1.POST("/lobehub-sso/exchange", h.LobeHubSSO.Exchange)
}
