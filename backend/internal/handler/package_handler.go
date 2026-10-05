package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// PackageHandler 用户端套餐接口：商店、我的套餐、冻结日历、冻结 / 解冻。
// 下单沿用 POST /api/v1/payment/orders（order_type=package）。
type PackageHandler struct {
	packageService *service.PackageService
}

// NewPackageHandler 创建用户端套餐处理器。
func NewPackageHandler(packageService *service.PackageService) *PackageHandler {
	return &PackageHandler{packageService: packageService}
}

// packageNoticeResponse 购买须知：前端把 {并发}、{冻结上限} 替换为当前用户的值。
type packageNoticeResponse struct {
	Text    string `json:"text"`
	Version int    `json:"version"`
}

type packageShopResponse struct {
	Groups             []service.PackageShopGroup `json:"groups"`
	Notice             packageNoticeResponse      `json:"notice"`
	FreezeEnabled      bool                       `json:"freeze_enabled"`
	MaxFreezeDays      int                        `json:"max_freeze_days"`
	PackageConcurrency int                        `json:"package_concurrency"`
}

// Shop GET /api/v1/packages/shop
func (h *PackageHandler) Shop(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	groups, err := h.packageService.ListShop(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	settings, err := h.packageService.GetSettings(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, packageShopResponse{
		Groups:             groups,
		Notice:             packageNoticeResponse{Text: settings.NoticeText, Version: settings.NoticeVersion},
		FreezeEnabled:      settings.FreezeEnabled,
		MaxFreezeDays:      settings.MaxFreezeDays,
		PackageConcurrency: h.packageService.UserPackageConcurrency(ctx, subject.UserID),
	})
}

// Mine GET /api/v1/packages/mine
func (h *PackageHandler) Mine(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	mine, err := h.packageService.GetMine(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, mine)
}

// Calendar GET /api/v1/packages/calendar?month=2026-10
func (h *PackageHandler) Calendar(c *gin.Context) {
	if _, ok := requireAuth(c); !ok {
		return
	}
	days, err := h.packageService.GetCalendar(c.Request.Context(), c.Query("month"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, days)
}

// Freeze POST /api/v1/packages/:id/freeze
func (h *PackageHandler) Freeze(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	id, ok := packageIDParam(c)
	if !ok {
		return
	}
	pkg, err := h.packageService.Freeze(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pkg)
}

// Unfreeze POST /api/v1/packages/:id/unfreeze
func (h *PackageHandler) Unfreeze(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	id, ok := packageIDParam(c)
	if !ok {
		return
	}
	pkg, err := h.packageService.Unfreeze(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pkg)
}

func packageIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_PACKAGE_ID", "套餐 ID 不合法"))
		return 0, false
	}
	return id, true
}
