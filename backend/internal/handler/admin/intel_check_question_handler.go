package admin

import (
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// --- Request / Response ---

// intelCheckQuestionRequest 题库条目的创建与更新共用入参（整块覆盖）。
//
// 与受检分组不同，题目的每个字段都能原样回填（没有掩码字段），
// 所以这里不需要「留空表示不改」那类语义。跨题型的字段由 service 层清空：
// 题型改成 logic 后留着旧参考稿，管理端会显示一堆不生效的配置。
type intelCheckQuestionRequest struct {
	Kind   string `json:"kind" binding:"required,oneof=logic drawing"`
	Title  string `json:"title" binding:"required,max=100"`
	Prompt string `json:"prompt" binding:"required"`

	// 逻辑题专用。是否必填、正则是否合法都由 service 层校验——
	// 正则要真编译一次才知道合不合法，binding 标签做不到。
	ExpectedAnswer string `json:"expected_answer"`
	MatchMode      string `json:"match_mode" binding:"omitempty,oneof=exact numeric contains regex"`

	// 绘图题专用。reference_metrics 不在入参里：它一律由服务端从
	// reference_html 现算，那是结构门禁全部相对项的分母，可写等于把判定阈值
	// 交给调用方（见 service.applyIntelCheckDrawingFields）。
	ReferenceHTML string         `json:"reference_html"`
	DrawingRules  map[string]any `json:"drawing_rules"`
	ReviewRubric  string         `json:"review_rubric"`

	// Enabled 省略时按 true 处理。
	Enabled *bool `json:"enabled"`
}

// intelCheckQuestionListItem 题库列表的一行。
//
// 参考稿与评审清单只给「有没有 / 多大」，不带正文：参考稿上限 512 KiB、
// 评审清单 20000 字，列表里带上它们会让题库页一次传好几 MB，
// 而列表上要看的只是「这道题配没配全」。点开编辑才需要正文。
type intelCheckQuestionListItem struct {
	ID     int64  `json:"id"`
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Prompt string `json:"prompt"`

	ExpectedAnswer string `json:"expected_answer"`
	MatchMode      string `json:"match_mode"`

	ReferenceHTMLBytes int            `json:"reference_html_bytes"`
	ReferenceMetrics   map[string]any `json:"reference_metrics"`
	DrawingRules       map[string]any `json:"drawing_rules"`
	HasReviewRubric    bool           `json:"has_review_rubric"`

	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// intelCheckEvaluateRequest 阈值标定入参：一份现成的 HTML / SVG 产物。
type intelCheckEvaluateRequest struct {
	Source string `json:"source" binding:"required"`
}

func intelCheckQuestionToListItem(q *service.IntelCheckQuestion) intelCheckQuestionListItem {
	return intelCheckQuestionListItem{
		ID:                 q.ID,
		Kind:               q.Kind,
		Title:              q.Title,
		Prompt:             q.Prompt,
		ExpectedAnswer:     q.ExpectedAnswer,
		MatchMode:          q.MatchMode,
		ReferenceHTMLBytes: len(q.ReferenceHTML),
		ReferenceMetrics:   q.ReferenceMetrics,
		DrawingRules:       q.DrawingRules,
		HasReviewRubric:    strings.TrimSpace(q.ReviewRubric) != "",
		Enabled:            q.Enabled,
		CreatedAt:          q.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          q.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// toQuestionParams 把请求映射为 service 入参。
func (r *intelCheckQuestionRequest) toQuestionParams() service.IntelCheckQuestionParams {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return service.IntelCheckQuestionParams{
		Kind:           r.Kind,
		Title:          r.Title,
		Prompt:         r.Prompt,
		ExpectedAnswer: r.ExpectedAnswer,
		MatchMode:      r.MatchMode,
		ReferenceHTML:  r.ReferenceHTML,
		DrawingRules:   r.DrawingRules,
		ReviewRubric:   r.ReviewRubric,
		Enabled:        enabled,
	}
}

// --- Handlers ---

// ListQuestions GET /api/v1/admin/intel-check/questions
func (h *IntelCheckHandler) ListQuestions(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > service.IntelCheckMaxPageSize {
		pageSize = service.IntelCheckMaxPageSize
	}

	items, total, err := h.intelCheckService.ListQuestions(c.Request.Context(),
		service.IntelCheckQuestionListParams{
			Page:     page,
			PageSize: pageSize,
			Kind:     strings.TrimSpace(c.Query("kind")),
			Enabled:  parseListEnabled(c.Query("enabled")),
		})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]intelCheckQuestionListItem, 0, len(items))
	for _, q := range items {
		out = append(out, intelCheckQuestionToListItem(q))
	}
	response.Paginated(c, out, total, page, pageSize)
}

// GetQuestion GET /api/v1/admin/intel-check/questions/:id
//
// 直接返回领域对象（含参考稿正文与评审清单）：题库条目里没有凭据类字段，
// 而编辑表单需要全部正文原样回填。
func (h *IntelCheckHandler) GetQuestion(c *gin.Context) {
	id, ok := parseIntelCheckID(c)
	if !ok {
		return
	}
	question, err := h.intelCheckService.GetQuestion(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, question)
}

// CreateQuestion POST /api/v1/admin/intel-check/questions
//
// 绘图题会在响应里带上服务端算出的 reference_metrics，供管理员核对
// 参考稿的造型数、动画目标数等指标是否符合预期（设计文档 §5.2）。
func (h *IntelCheckHandler) CreateQuestion(c *gin.Context) {
	var req intelCheckQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	question, err := h.intelCheckService.CreateQuestion(c.Request.Context(), req.toQuestionParams())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, question)
}

// UpdateQuestion PUT /api/v1/admin/intel-check/questions/:id
func (h *IntelCheckHandler) UpdateQuestion(c *gin.Context) {
	id, ok := parseIntelCheckID(c)
	if !ok {
		return
	}
	var req intelCheckQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	question, err := h.intelCheckService.UpdateQuestion(c.Request.Context(), id, req.toQuestionParams())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, question)
}

// DeleteQuestion DELETE /api/v1/admin/intel-check/questions/:id
//
// 历史轮次不受影响：rounds 的两个 question_id 是 ON DELETE SET NULL，
// 且题面已快照在 results.prompt_snapshot 里，删题不会让旧详情失去上下文。
func (h *IntelCheckHandler) DeleteQuestion(c *gin.Context) {
	id, ok := parseIntelCheckID(c)
	if !ok {
		return
	}
	if err := h.intelCheckService.DeleteQuestion(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// EvaluateQuestion POST /api/v1/admin/intel-check/questions/:id/evaluate
//
// 只对粘贴进来的产物跑判定（结构门禁 + 源码评审），不向受检分组发绘图请求。
// 用于标定 min_ratio 与 pass_score：拿几份已知好坏的样例反复跑，
// 直到门禁与评审的结论与人的判断一致。
func (h *IntelCheckHandler) EvaluateQuestion(c *gin.Context) {
	id, ok := parseIntelCheckID(c)
	if !ok {
		return
	}
	var req intelCheckEvaluateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	result, err := h.intelCheckService.EvaluateDrawingSource(c.Request.Context(), id, req.Source)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// DryRunQuestion POST /api/v1/admin/intel-check/questions/:id/dry-run?target_id=
//
// 对指定分组真发一次请求并判定，结果不落库、不占轮次号。
// 与真实检测共用同一条判定路径（service.probeIntelCheckOnce），
// 所以这里看到的结论就是线上会得出的结论。
func (h *IntelCheckHandler) DryRunQuestion(c *gin.Context) {
	id, ok := parseIntelCheckID(c)
	if !ok {
		return
	}
	targetID := parseOptionalInt64Query(c.Query("target_id"))
	if targetID == nil {
		response.ErrorFrom(c, infraerrors.BadRequest(
			"INVALID_INTEL_CHECK_TARGET_ID", "缺少或无效的 target_id"))
		return
	}

	result, err := h.intelCheckService.DryRunTarget(c.Request.Context(), id, *targetID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
