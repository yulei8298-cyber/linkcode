package admin

import (
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// IntelCheckHandler 模型智力检测的管理后台 handler。
//
// 分三个文件：本文件放设置、手动触发与两个只读列表，
// 受检分组与题库的 CRUD 分别在 intel_check_target_handler.go 与
// intel_check_question_handler.go（方法挂在同一个结构体上，
// 沿用 admin.SettingHandler 跨文件拆分的既有做法）。
type IntelCheckHandler struct {
	intelCheckService *service.IntelCheckService
	// runner 用于手动触发。不直接调 service.RunOnce：那会让 HTTP 请求
	// 同步等到整轮跑完（最长一个检测周期），反向代理超时后页面只会拿到
	// 一个与实际执行无关的 504。runner.RunNow 立即返回并自带单飞保护。
	runner *service.IntelCheckRunner
}

// NewIntelCheckHandler 创建管理端智力检测 handler。
func NewIntelCheckHandler(
	intelCheckService *service.IntelCheckService,
	runner *service.IntelCheckRunner,
) *IntelCheckHandler {
	return &IntelCheckHandler{intelCheckService: intelCheckService, runner: runner}
}

// --- Response ---

type intelCheckRoundResponse struct {
	ID                int64   `json:"id"`
	Seq               int64   `json:"seq"`
	StartedAt         string  `json:"started_at"`
	FinishedAt        *string `json:"finished_at"`
	LogicQuestionID   *int64  `json:"logic_question_id"`
	DrawingQuestionID *int64  `json:"drawing_question_id"`
	TriggerSource     string  `json:"trigger_source"`
}

// intelCheckResultListItem 明细列表的一行。
//
// 刻意不带 raw_reply 与 html_output，只给出字节数：绘图产物动辄几百 KB，
// 列表里带上它们会让一次翻页传好几 MB，而管理员在列表阶段要看的是
// 「哪一条失败了、错在哪」。要看产物本身可点进公开详情接口，
// 那里返回的是清洗后的 html_output，本就是给人看的那一份。
type intelCheckResultListItem struct {
	ID       int64  `json:"id"`
	RoundID  int64  `json:"round_id"`
	TargetID int64  `json:"target_id"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`

	LatencyMs       *int   `json:"latency_ms"`
	ExtractedAnswer string `json:"extracted_answer"`
	// ErrorMessage 是管理端相对公开详情多出来的那一项：上游状态码与报错片段。
	// 公开接口会把它换成中性的 status_note，这里照实给出，排障就靠它。
	ErrorMessage string `json:"error_message"`

	RawReplyBytes   int `json:"raw_reply_bytes"`
	HTMLOutputBytes int `json:"html_output_bytes"`

	InputTokens  *int   `json:"input_tokens"`
	OutputTokens *int   `json:"output_tokens"`
	CheckedAt    string `json:"checked_at"`
}

func intelCheckRoundToResponse(r *service.IntelCheckRound) *intelCheckRoundResponse {
	if r == nil {
		return nil
	}
	resp := &intelCheckRoundResponse{
		ID:                r.ID,
		Seq:               r.Seq,
		StartedAt:         r.StartedAt.UTC().Format(time.RFC3339),
		LogicQuestionID:   r.LogicQuestionID,
		DrawingQuestionID: r.DrawingQuestionID,
		TriggerSource:     r.TriggerSource,
	}
	if r.FinishedAt != nil {
		s := r.FinishedAt.UTC().Format(time.RFC3339)
		resp.FinishedAt = &s
	}
	return resp
}

func intelCheckResultToListItem(r *service.IntelCheckResult) intelCheckResultListItem {
	return intelCheckResultListItem{
		ID:              r.ID,
		RoundID:         r.RoundID,
		TargetID:        r.TargetID,
		Kind:            r.Kind,
		Status:          r.Status,
		LatencyMs:       r.LatencyMs,
		ExtractedAnswer: r.ExtractedAnswer,
		ErrorMessage:    r.ErrorMessage,
		RawReplyBytes:   len(r.RawReply),
		HTMLOutputBytes: len(r.HTMLOutput),
		InputTokens:     r.InputTokens,
		OutputTokens:    r.OutputTokens,
		CheckedAt:       r.CheckedAt.UTC().Format(time.RFC3339),
	}
}

// parseIntelCheckID 提取并校验路径参数 :id。
// 校验失败时已写入 4xx 响应，调用方只需 return。
func parseIntelCheckID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_INTEL_CHECK_ID", "无效的 id"))
		return 0, false
	}
	return id, true
}

// parseOptionalInt64Query 解析可选的 int64 query 参数；缺省或非法返回 nil。
func parseOptionalInt64Query(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return nil
	}
	return &v
}

// --- Handlers ---

// GetSettings GET /api/v1/admin/intel-check/settings
func (h *IntelCheckHandler) GetSettings(c *gin.Context) {
	cfg, err := h.intelCheckService.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateSettings PUT /api/v1/admin/intel-check/settings
//
// 直接绑定 service.IntelCheckSettings 而不另造一份管理端 DTO：这个结构体的
// json tag 本身就是接口契约（它整块序列化后存进 settings 表，部分字段又由公开页
// 原样回显），再复制一份平行结构只会多一处会漂移的映射。
//
// 语义是整块覆盖：管理端设置页一次提交全部字段。缺失字段会被 Normalize
// 补成默认值——对 enabled 而言缺失即 false，这与「关掉开关」是同一件事，
// 符合覆盖语义（见 IntelCheckSettings.Normalize 的注释）。
func (h *IntelCheckHandler) UpdateSettings(c *gin.Context) {
	var req service.IntelCheckSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	cfg, err := h.intelCheckService.UpdateSettings(c.Request.Context(), &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// RunNow POST /api/v1/admin/intel-check/run-now
//
// 立即返回，不等轮次跑完。已有一轮在飞时返回 409（ErrIntelCheckRoundInFlight），
// 不排队：排队只会在上游恢复后堆出一串补跑，把公开页刷成一片时刻贴得极近的轮次。
func (h *IntelCheckHandler) RunNow(c *gin.Context) {
	if h.runner == nil {
		response.InternalError(c, "智力检测调度器未初始化")
		return
	}
	if err := h.runner.RunNow(); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"started": true})
}

// ListRounds GET /api/v1/admin/intel-check/rounds
func (h *IntelCheckHandler) ListRounds(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > service.IntelCheckMaxPageSize {
		pageSize = service.IntelCheckMaxPageSize
	}

	items, total, err := h.intelCheckService.ListRounds(c.Request.Context(),
		service.IntelCheckRoundListParams{Page: page, PageSize: pageSize})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]*intelCheckRoundResponse, 0, len(items))
	for _, r := range items {
		out = append(out, intelCheckRoundToResponse(r))
	}
	response.Paginated(c, out, total, page, pageSize)
}

// ListResults GET /api/v1/admin/intel-check/results
//
// 支持按轮次、分组、题型、状态过滤。
func (h *IntelCheckHandler) ListResults(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	// 先按 service 的上限收敛，再把同一个值交给 Paginated：明细行带着大字段，
	// 单页拉多了是给自己制造慢查询；而回报值必须是实际生效的那个，
	// 否则前端按请求值算总页数，翻到后面全是空页。
	if pageSize > service.IntelCheckResultMaxPageSize {
		pageSize = service.IntelCheckResultMaxPageSize
	}

	params := service.IntelCheckResultListParams{
		Page:     page,
		PageSize: pageSize,
		RoundID:  parseOptionalInt64Query(c.Query("round_id")),
		TargetID: parseOptionalInt64Query(c.Query("target_id")),
		Kind:     strings.TrimSpace(c.Query("kind")),
		Status:   strings.TrimSpace(c.Query("status")),
	}

	items, total, err := h.intelCheckService.ListResults(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]intelCheckResultListItem, 0, len(items))
	for _, r := range items {
		out = append(out, intelCheckResultToListItem(r))
	}
	response.Paginated(c, out, total, page, pageSize)
}
