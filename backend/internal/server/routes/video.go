package routes

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// videoOnlyAllowedPaths 是视频专用分组允许访问的非视频端点（模型列表、用量、余额）。
var videoOnlyAllowedPaths = []string{"/models", "/usage", "/sub2api/billing"}

// isVideoRoutePath 判断路由模板是否属于视频端点。
func isVideoRoutePath(fullPath string) bool {
	return strings.Contains(fullPath, "/videos") ||
		strings.Contains(fullPath, "/video/") ||
		strings.Contains(fullPath, "/contents/generations/tasks")
}

// videoOnlyGroupGate 拒绝视频专用分组访问文本、图片、语音等端点：视频账号复用 OpenAI
// 网关的调度，若放行会把对话请求转发到视频上游。
func videoOnlyGroupGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := middleware.GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.Group == nil || !domain.IsVideoPlatform(apiKey.Group.Platform) {
			c.Next()
			return
		}
		fullPath := c.FullPath()
		if isVideoRoutePath(fullPath) {
			c.Next()
			return
		}
		for _, allowed := range videoOnlyAllowedPaths {
			if strings.HasSuffix(fullPath, allowed) || strings.Contains(fullPath, allowed+"/") {
				c.Next()
				return
			}
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": gin.H{
			"type":    "not_found_error",
			"message": "This group only supports video generation endpoints",
		}})
	}
}

// videosUnsupported 是统一视频入口对不支持平台的响应。
func videosUnsupported(c *gin.Context) {
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
	c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
}

// routeVideoCreate 分派视频创建：视频平台分组（含组合分组解析到视频平台）走视频网关，
// Grok 分组与未解析出目标的组合分组走 Grok 媒体网关，其余平台不支持。
func routeVideoCreate(h *handler.Handlers) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch platform := getGroupPlatform(c); {
		case domain.IsVideoPlatform(platform):
			h.Video.Create(c)
		case platform == service.PlatformGrok || platform == service.PlatformComposite:
			h.OpenAIGateway.GrokVideoGeneration(c)
		default:
			videosUnsupported(c)
		}
	}
}

// routeVideoLookup 按任务 ID 前缀分派查询：vt_ 前缀属于视频平台任务，其余沿用 Grok。
func routeVideoLookup(videoHandler, grokHandler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service.IsVideoTaskID(c.Param("request_id")) {
			videoHandler(c)
			return
		}
		switch getGroupPlatform(c) {
		case service.PlatformGrok, service.PlatformComposite:
			grokHandler(c)
		default:
			videosUnsupported(c)
		}
	}
}

// registerVideoNativeRoutes 注册厂商原生协议透传与签名下载链接。
func registerVideoNativeRoutes(r *gin.Engine, h *handler.Handlers, chain []gin.HandlerFunc) {
	native := append([]gin.HandlerFunc{handler.NormalizeVideoNativeAuth()}, chain...)
	r.Any("/video/:protocol/*path", append(native, h.Video.Native)...)
	// 签名链接自带鉴权，供浏览器 <video> 直接播放与下载。
	r.GET("/video-files/:task_id", h.Video.PublicContent)
}
