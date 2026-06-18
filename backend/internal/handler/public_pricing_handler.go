package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PublicPricingHandler 处理公开（无需认证）的定价方案查询。
//
// 与用户侧 AvailableChannelHandler 的区别仅在过滤口径：
//   - 用户侧按「当前用户可访问分组」过滤；
//   - 公开侧按「公开分组」过滤（IsExclusive == false），不依赖用户身份。
//
// 复用 available_channel_handler.go 的包级 DTO 转换函数
// （filterUserVisibleGroups / buildPlatformSections），输出结构与
// /channels/available 完全一致，前端可共用同一渲染组件。
type PublicPricingHandler struct {
	channelService *service.ChannelService
	settingService *service.SettingService
}

// NewPublicPricingHandler 创建公开定价 handler。
func NewPublicPricingHandler(
	channelService *service.ChannelService,
	settingService *service.SettingService,
) *PublicPricingHandler {
	return &PublicPricingHandler{
		channelService: channelService,
		settingService: settingService,
	}
}

// featureEnabled 复用 available-channels 功能开关（默认关闭，opt-in）。
func (h *PublicPricingHandler) featureEnabled(c *gin.Context) bool {
	if h.settingService == nil {
		return false
	}
	return h.settingService.GetAvailableChannelsRuntime(c.Request.Context()).Enabled
}

// List 列出所有公开分组的定价方案。
// GET /api/v1/public/pricing
func (h *PublicPricingHandler) List(c *gin.Context) {
	// Feature 未启用时返回空数组（不暴露渠道信息）。
	if !h.featureEnabled(c) {
		response.Success(c, []userAvailableChannel{})
		return
	}

	channels, err := h.channelService.ListAvailable(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 收集所有公开分组（IsExclusive == false）的 ID，作为可见分组白名单。
	allowedGroupIDs := make(map[int64]struct{})
	for _, ch := range channels {
		for _, g := range ch.Groups {
			if !g.IsExclusive {
				allowedGroupIDs[g.ID] = struct{}{}
			}
		}
	}

	out := make([]userAvailableChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.Status != service.StatusActive {
			continue
		}
		visibleGroups := filterUserVisibleGroups(ch.Groups, allowedGroupIDs)
		if len(visibleGroups) == 0 {
			continue
		}
		sections := buildPlatformSections(ch, visibleGroups)
		if len(sections) == 0 {
			continue
		}
		out = append(out, userAvailableChannel{
			Name:        ch.Name,
			Description: ch.Description,
			Platforms:   sections,
		})
	}

	response.Success(c, out)
}
