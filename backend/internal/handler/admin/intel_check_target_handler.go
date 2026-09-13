package admin

import (
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// --- Request / Response ---

// intelCheckTargetRequest 受检分组的创建与更新共用入参。
//
// 创建与更新共用一个结构体（不同于 ChannelMonitor 的双结构体写法），
// 因为 service 层的 IntelCheckTargetParams 本就是整块覆盖语义：
// 管理端表单一次提交分组全部字段，没有「只改一个字段」的合并需求。
// 唯一的例外是 APIKey，见下。
type intelCheckTargetRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description" binding:"omitempty,max=500"`
	BaseURL     string `json:"base_url" binding:"required,max=500"`

	// APIKey 创建时必填（由 service 校验），更新时留空表示保留原凭据。
	// 必须留这条语义：管理端拿到的是掩码（sk-1***），把它原样回传提交
	// 会把掩码当成新 key 写进库，那把真凭据就永久丢了。
	APIKey string `json:"api_key" binding:"omitempty,max=2000"`

	APIMode         string `json:"api_mode" binding:"omitempty,oneof=responses chat_completions"`
	Model           string `json:"model" binding:"required,max=200"`
	ReasoningEffort string `json:"reasoning_effort" binding:"omitempty,oneof=low medium high xhigh"`
	RateLabel       string `json:"rate_label" binding:"omitempty,max=20"`

	// Enabled 省略时按 true 处理。由于整个请求是整块覆盖语义，
	// 更新时省略它会让一个已停用的分组重新启用——管理端表单必须始终带上该字段。
	Enabled   *bool `json:"enabled"`
	SortOrder int   `json:"sort_order"`
}

// intelCheckTargetResponse 受检分组的管理端视图。
//
// 逐字段列出而非嵌入 service.IntelCheckTarget：那个结构体带明文 APIKey
// （service 层已解密），直接序列化就是把上游凭据发给浏览器。
// 这里只给掩码，且没有任何字段承载完整 key。
type intelCheckTargetResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	BaseURL     string `json:"base_url"`

	APIKeyMasked        string `json:"api_key_masked"`
	APIKeyDecryptFailed bool   `json:"api_key_decrypt_failed"`

	APIMode         string `json:"api_mode"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	RateLabel       string `json:"rate_label"`

	Enabled   bool   `json:"enabled"`
	SortOrder int    `json:"sort_order"`
	CreatedBy int64  `json:"created_by"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func intelCheckTargetToResponse(t *service.IntelCheckTarget) *intelCheckTargetResponse {
	if t == nil {
		return nil
	}
	return &intelCheckTargetResponse{
		ID:                  t.ID,
		Name:                t.Name,
		Description:         t.Description,
		BaseURL:             t.BaseURL,
		APIKeyMasked:        maskAPIKey(t.APIKey),
		APIKeyDecryptFailed: t.APIKeyDecryptFailed,
		APIMode:             t.APIMode,
		Model:               t.Model,
		ReasoningEffort:     t.ReasoningEffort,
		RateLabel:           t.RateLabel,
		Enabled:             t.Enabled,
		SortOrder:           t.SortOrder,
		CreatedBy:           t.CreatedBy,
		CreatedAt:           t.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           t.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// toTargetParams 把请求映射为 service 入参。createdBy 仅创建时有意义。
func (r *intelCheckTargetRequest) toTargetParams(createdBy int64) service.IntelCheckTargetParams {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return service.IntelCheckTargetParams{
		Name:            r.Name,
		Description:     r.Description,
		BaseURL:         r.BaseURL,
		APIKey:          r.APIKey,
		APIMode:         r.APIMode,
		Model:           r.Model,
		ReasoningEffort: r.ReasoningEffort,
		RateLabel:       r.RateLabel,
		Enabled:         enabled,
		SortOrder:       r.SortOrder,
		CreatedBy:       createdBy,
	}
}

// --- Handlers ---

// ListTargets GET /api/v1/admin/intel-check/targets
func (h *IntelCheckHandler) ListTargets(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > service.IntelCheckMaxPageSize {
		pageSize = service.IntelCheckMaxPageSize
	}

	items, total, err := h.intelCheckService.ListTargets(c.Request.Context(),
		service.IntelCheckTargetListParams{
			Page:     page,
			PageSize: pageSize,
			Enabled:  parseListEnabled(c.Query("enabled")),
			Search:   strings.TrimSpace(c.Query("search")),
		})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]*intelCheckTargetResponse, 0, len(items))
	for _, t := range items {
		out = append(out, intelCheckTargetToResponse(t))
	}
	response.Paginated(c, out, total, page, pageSize)
}

// GetTarget GET /api/v1/admin/intel-check/targets/:id
func (h *IntelCheckHandler) GetTarget(c *gin.Context) {
	id, ok := parseIntelCheckID(c)
	if !ok {
		return
	}
	target, err := h.intelCheckService.GetTarget(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, intelCheckTargetToResponse(target))
}

// CreateTarget POST /api/v1/admin/intel-check/targets
func (h *IntelCheckHandler) CreateTarget(c *gin.Context) {
	var req intelCheckTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)

	target, err := h.intelCheckService.CreateTarget(
		c.Request.Context(), req.toTargetParams(subject.UserID))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, intelCheckTargetToResponse(target))
}

// UpdateTarget PUT /api/v1/admin/intel-check/targets/:id
func (h *IntelCheckHandler) UpdateTarget(c *gin.Context) {
	id, ok := parseIntelCheckID(c)
	if !ok {
		return
	}
	var req intelCheckTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	// CreatedBy 传 0：service 的 UpdateTarget 不读该字段（创建者不随编辑变更）。
	target, err := h.intelCheckService.UpdateTarget(c.Request.Context(), id, req.toTargetParams(0))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, intelCheckTargetToResponse(target))
}

// DeleteTarget DELETE /api/v1/admin/intel-check/targets/:id
//
// 关联明细由外键 ON DELETE CASCADE 一并删除（见 service.DeleteTarget 的说明）：
// 分组配置已不存在时，留着它的历史色块只会在公开页上变成点不开的孤立数据。
func (h *IntelCheckHandler) DeleteTarget(c *gin.Context) {
	id, ok := parseIntelCheckID(c)
	if !ok {
		return
	}
	if err := h.intelCheckService.DeleteTarget(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}
