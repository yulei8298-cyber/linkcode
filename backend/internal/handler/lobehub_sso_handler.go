package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type LobeHubSSOHandler struct {
	ssoService *service.LobeHubSSOService
}

func NewLobeHubSSOHandler(ssoService *service.LobeHubSSOService) *LobeHubSSOHandler {
	return &LobeHubSSOHandler{ssoService: ssoService}
}

type LobeHubSSOAuthorizeRequest struct {
	ReturnTo string `json:"return_to"`
}

type LobeHubSSOExchangeRequest struct {
	Code string `json:"code" binding:"required"`
}

// Authorize creates a short-lived one-time code and returns the LobeHub callback URL.
// POST /api/v1/lobehub-sso/authorize
func (h *LobeHubSSOHandler) Authorize(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req LobeHubSSOAuthorizeRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
	}

	result, err := h.ssoService.Authorize(c.Request.Context(), service.LobeHubSSOAuthorizeInput{
		UserID:   subject.UserID,
		ReturnTo: req.ReturnTo,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// Exchange is called by LobeHub backend to exchange a one-time code for user info and provider keys.
// POST /api/v1/lobehub-sso/exchange
func (h *LobeHubSSOHandler) Exchange(c *gin.Context) {
	var req LobeHubSSOExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	secret := strings.TrimSpace(c.GetHeader("X-LobeHub-SSO-Secret"))
	result, err := h.ssoService.Exchange(c.Request.Context(), service.LobeHubSSOExchangeInput{
		Code:         req.Code,
		SharedSecret: secret,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
