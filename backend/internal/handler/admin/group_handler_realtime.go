package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// GetRealtimeRPM 返回每个分组最近 60 秒的请求数（实时 RPM），供分组页轮询。
// GET /api/v1/admin/groups/realtime-rpm
func (h *GroupHandler) GetRealtimeRPM(c *gin.Context) {
	if h.rejectUnsupportedSimpleModeOperation(c, "advanced") {
		return
	}
	summary, err := h.groupCapacityService.GetRealtimeRPM(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

// GetUserConcurrency 返回某分组内每个用户当前的并发请求数。
// GET /api/v1/admin/groups/:id/user-concurrency
func (h *GroupHandler) GetUserConcurrency(c *gin.Context) {
	if h.rejectUnsupportedSimpleModeOperation(c, "advanced") {
		return
	}
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	summary, err := h.groupCapacityService.GetGroupUserConcurrency(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}
