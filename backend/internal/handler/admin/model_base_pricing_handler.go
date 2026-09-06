package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strings"
)

func (h *ChannelHandler) GetModelBasePricing(c *gin.Context) {
	model := strings.TrimSpace(c.Query("model"))
	if model == "" {
		response.Error(c, 400, "model is required")
		return
	}
	price, edited, catalog := h.pricingService.GetAdminModelPrice(model)
	response.Success(c, gin.H{"model": model, "override": price, "edited": edited, "catalog": catalog})
}

func (h *ChannelHandler) UpdateModelBasePricing(c *gin.Context) {
	var req struct {
		Model string                   `json:"model" binding:"required,max=256"`
		Price *service.AdminModelPrice `json:"price"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "invalid model pricing")
		return
	}
	if err := h.pricingService.SetAdminModelPrice(req.Model, req.Price); err != nil {
		response.Error(c, 400, "Unable to save model prices. Check the model name, non-negative prices and pricing storage permissions.")
		return
	}
	response.Success(c, gin.H{"saved": true})
}
