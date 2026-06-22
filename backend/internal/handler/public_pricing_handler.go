package handler

import (
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PublicPricingHandler 处理公开（无需认证）的定价方案查询。
//
// 与用户侧 AvailableChannelHandler 不同，公开定价页直接展示后台配置的
// active 公开分组（IsExclusive == false），不要求分组已经绑定到某个可用渠道。
type PublicPricingHandler struct {
	groupService *service.GroupService
}

// NewPublicPricingHandler 创建公开定价 handler。
func NewPublicPricingHandler(
	groupService *service.GroupService,
) *PublicPricingHandler {
	return &PublicPricingHandler{
		groupService: groupService,
	}
}

// List 列出所有公开分组的定价方案。
// GET /api/v1/public/pricing
func (h *PublicPricingHandler) List(c *gin.Context) {
	groups, err := h.groupService.ListActive(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	groupsByPlatform := make(map[string][]userAvailableGroup)
	for _, g := range groups {
		if g.IsExclusive || g.Platform == "" {
			continue
		}
		groupsByPlatform[g.Platform] = append(groupsByPlatform[g.Platform], userAvailableGroup{
			ID:               g.ID,
			Name:             g.Name,
			Platform:         g.Platform,
			SubscriptionType: g.SubscriptionType,
			RateMultiplier:   g.RateMultiplier,
			IsExclusive:      g.IsExclusive,
		})
	}

	platforms := make([]string, 0, len(groupsByPlatform))
	for platform := range groupsByPlatform {
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)

	sections := make([]userChannelPlatformSection, 0, len(platforms))
	for _, platform := range platforms {
		sections = append(sections, userChannelPlatformSection{
			Platform:        platform,
			Groups:          groupsByPlatform[platform],
			SupportedModels: []userSupportedModel{},
		})
	}
	if len(sections) == 0 {
		response.Success(c, []userAvailableChannel{})
		return
	}

	response.Success(c, []userAvailableChannel{{
		Name:        "public-groups",
		Description: "",
		Platforms:   sections,
	}})
}
