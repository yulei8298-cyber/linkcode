package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// GetEnterpriseSettings GET /api/v1/admin/packages/enterprise/settings
func (h *PackageHandler) GetEnterpriseSettings(c *gin.Context) {
	settings, err := h.packageService.GetEnterpriseSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// UpdateEnterpriseSettings PUT /api/v1/admin/packages/enterprise/settings
func (h *PackageHandler) UpdateEnterpriseSettings(c *gin.Context) {
	var req service.EnterpriseSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "请求参数不合法"))
		return
	}
	settings, err := h.packageService.UpdateEnterpriseSettings(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

// GetUserEnterprise GET /api/v1/admin/packages/enterprise/users/:id
// 某用户的企业尊享状态：开通方式、当前累计消费与门槛。
func (h *PackageHandler) GetUserEnterprise(c *gin.Context) {
	userID, ok := enterpriseUserID(c)
	if !ok {
		return
	}
	status, err := h.packageService.EnterpriseStatus(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// SetUserEnterprise PUT /api/v1/admin/packages/enterprise/users/:id  body: {"mode":"auto|on|off"}
func (h *PackageHandler) SetUserEnterprise(c *gin.Context) {
	userID, ok := enterpriseUserID(c)
	if !ok {
		return
	}
	var req struct {
		Mode string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "请求参数不合法"))
		return
	}
	status, err := h.packageService.SetEnterpriseMode(c.Request.Context(), userID, req.Mode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func enterpriseUserID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_USER_ID", "用户 ID 不合法"))
		return 0, false
	}
	return id, true
}
