package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"

	"github.com/gin-gonic/gin"
)

// registerPackageUserRoutes 注册用户端套餐路由（挂在已鉴权的用户路由组下）。
// 下单走 /payment/orders（order_type=package），这里只有商店、我的套餐与冻结。
func registerPackageUserRoutes(authenticated *gin.RouterGroup, h *handler.Handlers) {
	authenticated.GET("/user/enterprise", h.Package.EnterpriseStatus)

	packages := authenticated.Group("/packages")
	{
		packages.GET("/shop", h.Package.Shop)
		packages.GET("/mine", h.Package.Mine)
		packages.GET("/calendar", h.Package.Calendar)
		packages.POST("/:id/freeze", h.Package.Freeze)
		packages.POST("/:id/unfreeze", h.Package.Unfreeze)
	}
}

// registerPackageAdminRoutes 注册管理端套餐路由。
func registerPackageAdminRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	packages := admin.Group("/packages")
	{
		packages.GET("/plans", h.Admin.Package.ListPlans)
		packages.POST("/plans", h.Admin.Package.SavePlan)
		packages.DELETE("/plans/:id", h.Admin.Package.DeletePlan)

		packages.GET("/settings", h.Admin.Package.GetSettings)
		packages.PUT("/settings", h.Admin.Package.UpdateSettings)

		packages.GET("/holidays", h.Admin.Package.ListHolidays)
		packages.POST("/holidays", h.Admin.Package.AddHoliday)
		packages.POST("/holidays/delete", h.Admin.Package.DeleteHoliday)
		packages.POST("/holidays/sync", h.Admin.Package.SyncHolidays)

		packages.GET("/enterprise/settings", h.Admin.Package.GetEnterpriseSettings)
		packages.PUT("/enterprise/settings", h.Admin.Package.UpdateEnterpriseSettings)
		packages.GET("/enterprise/users/:id", h.Admin.Package.GetUserEnterprise)
		packages.PUT("/enterprise/users/:id", h.Admin.Package.SetUserEnterprise)

		packages.GET("/user-packages", h.Admin.Package.ListUserPackages)
		packages.GET("/user-packages/stats", h.Admin.Package.UserPackageStats)
		packages.POST("/user-packages/:id/unfreeze", h.Admin.Package.UnfreezeUserPackage)
		packages.POST("/user-packages/:id/void", h.Admin.Package.VoidUserPackage)
	}
}
