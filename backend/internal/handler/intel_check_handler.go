package handler

import (
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// IntelCheckHandler 模型智力检测的公开接口（无需登录）。
//
// 本 handler 只允许调用 service 上以 Public 开头的方法。这是一条硬约束而非风格
// 偏好：GetTarget / ListResults / DryRunTarget 这些方法返回的领域对象带着
// base_url、api_key 与上游报错原文，而这张页面对外承诺不暴露任何上游细节
// （设计文档 §4.5）。脱敏逻辑集中在 PublicOverview / PublicResultDetail 里，
// 这里的职责就是不要绕过它——一旦有人为了省事在这里直接序列化领域模型，
// 上游地址与凭据片段就随公开接口泄了。
//
// 开关关闭时两个数据接口都返回 404（ErrIntelCheckDisabled 是 infraerrors.NotFound），
// 不返回空数据：空数据会渲染出一个「一切正常但没有分组」的页面，比 404 更误导。
// Preview 是不读取数据的固定外壳，管理端在功能开启前也要使用它。
type IntelCheckHandler struct {
	intelCheckService *service.IntelCheckService
}

// NewIntelCheckHandler 创建公开智力检测 handler。
func NewIntelCheckHandler(intelCheckService *service.IntelCheckService) *IntelCheckHandler {
	return &IntelCheckHandler{intelCheckService: intelCheckService}
}

// Overview 返回公开页所需的全部数据。
// GET /api/v1/public/intel-check/overview
func (h *IntelCheckHandler) Overview(c *gin.Context) {
	overview, err := h.intelCheckService.PublicOverview(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}

// ResultDetail 返回单次检测的对外详情。
// GET /api/v1/public/intel-check/results/:id
func (h *IntelCheckHandler) ResultDetail(c *gin.Context) {
	id, ok := parseIntelCheckResultID(c)
	if !ok {
		return
	}
	result, err := h.intelCheckService.PublicResultDetail(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// parseIntelCheckResultID 提取并校验路径参数 :id。
// 校验失败时已写入 4xx 响应，调用方只需 return。
func parseIntelCheckResultID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest(
			"INVALID_INTEL_CHECK_RESULT_ID", "无效的检测记录 id"))
		return 0, false
	}
	return id, true
}
