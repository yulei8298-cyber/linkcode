package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// videoTaskAdminView 是管理端任务视图：在任务字段之外附带网关签名的成品链接，
// 后台「查看成品」与用户拿到的链接一致，不暴露上游地址。
type videoTaskAdminView struct {
	*service.VideoTask
	ContentURL string `json:"content_url,omitempty"`
}

func (h *VideoTaskHandler) view(task *service.VideoTask, now time.Time) videoTaskAdminView {
	out := videoTaskAdminView{VideoTask: task}
	if h.videos.HasContent(task) {
		out.ContentURL = h.videos.SignedContentPath(task.TaskID, now)
	}
	return out
}

// VideoTaskHandler 是视频任务的管理端接口：查看任务与账务状态、手动刷新、重试结算。
type VideoTaskHandler struct {
	videos *service.VideoGatewayService
}

// NewVideoTaskHandler 创建视频任务管理处理器。
func NewVideoTaskHandler(videos *service.VideoGatewayService) *VideoTaskHandler {
	return &VideoTaskHandler{videos: videos}
}

// List 分页查询视频任务。
// GET /api/v1/admin/video-tasks
func (h *VideoTaskHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.VideoTaskFilter{
		Status:        strings.TrimSpace(c.Query("status")),
		BillingStatus: strings.TrimSpace(c.Query("billing_status")),
		Protocol:      strings.TrimSpace(c.Query("protocol")),
		Model:         strings.TrimSpace(c.Query("model")),
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		Limit:         pageSize,
		Offset:        (page - 1) * pageSize,
	}
	if v := strings.TrimSpace(c.Query("user_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		filter.UserID = id
	}
	tasks, total, err := h.videos.ListTasks(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, "Failed to list video tasks")
		return
	}
	now := time.Now()
	views := make([]videoTaskAdminView, 0, len(tasks))
	for _, task := range tasks {
		views = append(views, h.view(task, now))
	}
	response.Paginated(c, views, total, page, pageSize)
}

// Refresh 立即向上游查询一次任务状态（成功时同步结算）。
// POST /api/v1/admin/video-tasks/:task_id/refresh
func (h *VideoTaskHandler) Refresh(c *gin.Context) {
	task, err := h.videos.FindTask(c.Request.Context(), c.Param("task_id"))
	if err != nil {
		response.NotFound(c, "Video task not found")
		return
	}
	fresh, _, err := h.videos.Refresh(c.Request.Context(), task)
	if err != nil {
		response.BadRequest(c, "Refresh failed: "+err.Error())
		return
	}
	response.Success(c, h.view(fresh, time.Now()))
}

// RetryBilling 手动重试结算失败或未结算的成功任务。
// POST /api/v1/admin/video-tasks/:task_id/retry-billing
func (h *VideoTaskHandler) RetryBilling(c *gin.Context) {
	task, err := h.videos.RetryBilling(c.Request.Context(), c.Param("task_id"))
	if err != nil {
		response.BadRequest(c, "Retry billing failed: "+err.Error())
		return
	}
	response.Success(c, h.view(task, time.Now()))
}
