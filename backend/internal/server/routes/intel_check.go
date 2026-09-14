package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterIntelCheckPublicRoutes 注册模型智力检测的公开路由。
//
// 与 /public 下其他端点一样不挂 JWT，可见性由功能开关而非登录态决定：
// 这张页面的用途就是让未登录的人也能核验我们的模型有没有降智，
// 要求登录等于把证据锁在门后。开关关闭时 handler 返回 404（见 IntelCheckHandler）。
//
// 挂 PublicIP 限流（设计文档 §7）：overview 一次请求要做多分组时间线聚合
// 加两次按题型的状态计数，比 /public 下那几个端点重得多，
// 不限流很容易被刷成数据库压力源。
func RegisterIntelCheckPublicRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	intelCheck := v1.Group("/public/intel-check")
	intelCheck.Use(panelRateLimiter.PublicIP())
	{
		// 无数据的静态预览外壳也供开关关闭时的管理端标定使用。
		intelCheck.GET("/preview", h.IntelCheck.Preview)
		intelCheck.GET("/overview", h.IntelCheck.Overview)
		intelCheck.GET("/results/:id", h.IntelCheck.ResultDetail)
	}
}

// registerIntelCheckRoutes 注册模型智力检测的管理端路由。
//
// 刻意不挂功能开关守卫（不同于 registerChannelMonitorRoutes）：
// 配置受检分组、录题、标定阈值全都发生在开启之前，而 ValidateIntelCheckSettings
// 又要求开启前必须已配好评审模型。用开关守卫这些页面会形成
// 「想开启必须先开启」的死锁。
func registerIntelCheckRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	intelCheck := admin.Group("/intel-check")
	{
		intelCheck.GET("/settings", h.Admin.IntelCheck.GetSettings)
		intelCheck.PUT("/settings", h.Admin.IntelCheck.UpdateSettings)
		intelCheck.POST("/run-now", h.Admin.IntelCheck.RunNow)

		intelCheck.GET("/rounds", h.Admin.IntelCheck.ListRounds)
		intelCheck.GET("/results", h.Admin.IntelCheck.ListResults)

		targets := intelCheck.Group("/targets")
		{
			targets.GET("", h.Admin.IntelCheck.ListTargets)
			targets.POST("", h.Admin.IntelCheck.CreateTarget)
			targets.GET("/:id", h.Admin.IntelCheck.GetTarget)
			targets.PUT("/:id", h.Admin.IntelCheck.UpdateTarget)
			targets.DELETE("/:id", h.Admin.IntelCheck.DeleteTarget)
		}

		questions := intelCheck.Group("/questions")
		{
			questions.GET("", h.Admin.IntelCheck.ListQuestions)
			questions.POST("", h.Admin.IntelCheck.CreateQuestion)
			questions.GET("/:id", h.Admin.IntelCheck.GetQuestion)
			questions.PUT("/:id", h.Admin.IntelCheck.UpdateQuestion)
			questions.DELETE("/:id", h.Admin.IntelCheck.DeleteQuestion)
			// 标定：只判定粘贴进来的产物，不发绘图请求
			questions.POST("/:id/evaluate", h.Admin.IntelCheck.EvaluateQuestion)
			// 试跑：对单组真发一次请求，结果不落库
			questions.POST("/:id/dry-run", h.Admin.IntelCheck.DryRunQuestion)
		}
	}
}
