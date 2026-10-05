package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// PackageHandler 管理端套餐接口：套餐配置、全局设置、可冻结日期、用户套餐。
// 用户的套餐并发在「编辑用户」接口里修改（package_concurrency 字段）。
type PackageHandler struct {
	packageService *service.PackageService
}

// NewPackageHandler 创建管理端套餐处理器。
func NewPackageHandler(packageService *service.PackageService) *PackageHandler {
	return &PackageHandler{packageService: packageService}
}

// ---------- 套餐配置 ----------

// ListPlans GET /api/v1/admin/packages/plans?group_id=
func (h *PackageHandler) ListPlans(c *gin.Context) {
	var groupID *int64
	if raw := c.Query("group_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			response.ErrorFrom(c, infraerrors.BadRequest("INVALID_GROUP_ID", "分组 ID 不合法"))
			return
		}
		groupID = &id
	}
	plans, err := h.packageService.ListPlans(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plans)
}

// SavePlan POST /api/v1/admin/packages/plans
// 新建或覆盖（同分组同周期同档位只有一个套餐）；带 id 时更新该套餐。
func (h *PackageHandler) SavePlan(c *gin.Context) {
	var req service.PackagePlanInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	plan, err := h.packageService.SavePlan(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plan)
}

// DeletePlan DELETE /api/v1/admin/packages/plans/:id
func (h *PackageHandler) DeletePlan(c *gin.Context) {
	id, ok := packagePathID(c)
	if !ok {
		return
	}
	if err := h.packageService.DeletePlan(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// ---------- 全局设置 ----------

type packageSettingsResponse struct {
	service.PackageSettings
	HolidaySync service.PackageHolidaySyncState `json:"holiday_sync"`
}

// GetSettings GET /api/v1/admin/packages/settings
func (h *PackageHandler) GetSettings(c *gin.Context) {
	ctx := c.Request.Context()
	settings, err := h.packageService.GetSettings(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, packageSettingsResponse{PackageSettings: settings, HolidaySync: h.packageService.GetHolidaySyncState(ctx)})
}

// UpdateSettings PUT /api/v1/admin/packages/settings
// 整块覆盖；购买须知正文变化时版本号自动 +1，notice_version 字段以服务端为准。
func (h *PackageHandler) UpdateSettings(c *gin.Context) {
	var req service.PackageSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	settings, err := h.packageService.UpdateSettings(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// ---------- 可冻结日期 ----------

type packageHolidayRequest struct {
	Name  string `json:"name" binding:"required"`
	Start string `json:"start" binding:"required"`
	End   string `json:"end" binding:"required"`
}

// ListHolidays GET /api/v1/admin/packages/holidays?year=2026
func (h *PackageHandler) ListHolidays(c *gin.Context) {
	year := timezone.Now().Year()
	if raw := c.Query("year"); raw != "" {
		y, err := strconv.Atoi(raw)
		if err != nil || y < 2000 || y > 2100 {
			response.ErrorFrom(c, infraerrors.BadRequest("INVALID_YEAR", "年份不合法"))
			return
		}
		year = y
	}
	ranges, err := h.packageService.ListHolidayRanges(c.Request.Context(), year)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ranges)
}

// AddHoliday POST /api/v1/admin/packages/holidays
func (h *PackageHandler) AddHoliday(c *gin.Context) {
	var req packageHolidayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	if err := h.packageService.AddManualHoliday(c.Request.Context(), req.Name, req.Start, req.End); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"added": true})
}

// DeleteHoliday POST /api/v1/admin/packages/holidays/delete
// 用 POST 携带请求体：按名称 + 日期段删除，避免 DELETE 请求体在部分代理上被丢弃。
func (h *PackageHandler) DeleteHoliday(c *gin.Context) {
	var req packageHolidayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	if err := h.packageService.DeleteManualHoliday(c.Request.Context(), req.Name, req.Start, req.End); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// SyncHolidays POST /api/v1/admin/packages/holidays/sync
func (h *PackageHandler) SyncHolidays(c *gin.Context) {
	state, err := h.packageService.SyncHolidays(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("PACKAGE_HOLIDAY_SYNC_FAILED", "节假日同步失败："+err.Error()))
		return
	}
	response.Success(c, state)
}

// ---------- 用户套餐 ----------

// ListUserPackages GET /api/v1/admin/packages/user-packages?user_id=
func (h *PackageHandler) ListUserPackages(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_USER_ID", "用户 ID 不合法"))
		return
	}
	pkgs, err := h.packageService.AdminListUserPackages(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pkgs)
}

// UnfreezeUserPackage POST /api/v1/admin/packages/user-packages/:id/unfreeze
func (h *PackageHandler) UnfreezeUserPackage(c *gin.Context) {
	id, ok := packagePathID(c)
	if !ok {
		return
	}
	pkg, err := h.packageService.AdminUnfreeze(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pkg)
}

// VoidUserPackage POST /api/v1/admin/packages/user-packages/:id/void
func (h *PackageHandler) VoidUserPackage(c *gin.Context) {
	id, ok := packagePathID(c)
	if !ok {
		return
	}
	pkg, err := h.packageService.AdminVoid(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pkg)
}

func packagePathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "ID 不合法"))
		return 0, false
	}
	return id, true
}
