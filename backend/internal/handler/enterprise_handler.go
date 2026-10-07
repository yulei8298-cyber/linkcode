package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
)

// EnterpriseStatus GET /api/v1/user/enterprise
// 当前用户是否是企业尊享用户，前端据此显示 Logo 旁的标识与进入网站时的欢迎提示。
func (h *PackageHandler) EnterpriseStatus(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	status, err := h.packageService.EnterpriseStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}
