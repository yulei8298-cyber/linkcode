package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/Wei-Shaw/sub2api/internal/service/video/registry"
	"github.com/gin-gonic/gin"
)

// Native 处理厂商原生协议透传：/video/{protocol}/{原生路径}。各厂商 SDK 只需把
// Base URL 改为 https://网关/video/{protocol} 即可使用。
func (h *VideoHandler) Native(c *gin.Context) {
	provider, ok := registry.LookupNative(c.Param("protocol"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Unknown video protocol"}})
		return
	}
	h.native(c, string(provider.Info().Protocol), c.Param("path"))
}

// ArkTasks 兼容把方舟 SDK Base URL 直接指向网关根地址（/api/v3 等）的调用方式。
func (h *VideoHandler) ArkTasks(c *gin.Context) {
	h.native(c, string(video.ProtocolArk), c.Request.URL.Path)
}

func (h *VideoHandler) native(c *gin.Context, protocol, path string) {
	provider, ok := registry.Lookup(protocol)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Unknown video protocol"}})
		return
	}
	op := provider.Classify(c.Request.Method, path, c.Request.URL.Query())
	switch op.Kind {
	case video.NativeCreate:
		body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil || len(body) == 0 {
			h.openai.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is required")
			return
		}
		req, err := provider.ParseNativeCreate(path, body)
		if err != nil {
			h.openai.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		h.create(c, videoCreateInput{Request: req, Protocol: protocol, NativePath: path, NativeBody: body, Native: provider})
	case video.NativeQuery, video.NativeCancel, video.NativeFile, video.NativeContent:
		h.nativeLookup(c, protocol, provider, op)
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Unsupported native video endpoint"}})
	}
}

// nativeLookup 处理原生查询 / 取消 / 取文件：只允许提交者访问，并固定回到提交账号。
func (h *VideoHandler) nativeLookup(c *gin.Context, protocol string, provider video.Provider, op video.NativeOp) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.openai.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if strings.TrimSpace(op.TaskID) == "" && strings.TrimSpace(op.FileID) == "" {
		h.openai.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "task id is required")
		return
	}
	task, err := h.videos.FindNativeTaskForOwner(c.Request.Context(), protocol, op, apiKey.UserID, apiKey.ID)
	if err != nil {
		h.openai.errorResponse(c, http.StatusNotFound, "not_found_error", "Video task not found")
		return
	}
	c.Header("Cache-Control", "no-store")
	switch op.Kind {
	case video.NativeQuery:
		fresh, resp, err := h.videos.Refresh(c.Request.Context(), task)
		h.writeNative(c, h.hideUpstreamContentURL(c, provider, fresh, resp), err)
	case video.NativeContent:
		h.stream(c, task)
	case video.NativeCancel:
		resp, err := h.videos.Cancel(c.Request.Context(), task)
		if resp == nil && err == nil {
			c.JSON(http.StatusOK, gin.H{"id": task.UpstreamTaskID, "status": string(task.Status)})
			return
		}
		h.writeNative(c, resp, err)
	case video.NativeFile:
		if !h.videos.HasContent(task) {
			// 先刷新一次拿到成品，才能把下载地址换成网关签名链接。
			if fresh, _, err := h.videos.Refresh(c.Request.Context(), task); err == nil {
				task = fresh
			}
		}
		resp, err := h.videos.ForwardNativeFile(c.Request.Context(), task, op.FileID)
		h.writeNative(c, h.hideUpstreamContentURL(c, provider, task, resp), err)
	}
}

// hideUpstreamContentURL 把原生响应里的上游成品地址换成网关签名链接；
// 成品尚未就绪时原样返回（此时响应里也不会有成品地址）。
func (h *VideoHandler) hideUpstreamContentURL(c *gin.Context, provider video.Provider, task *service.VideoTask, resp *service.VideoUpstreamResponse) *service.VideoUpstreamResponse {
	if resp == nil || resp.StatusCode >= 300 || !h.videos.HasContent(task) {
		return resp
	}
	link := requestPublicBase(c) + h.videos.SignedContentPath(task.TaskID, time.Now())
	out := *resp
	out.Body = video.RewriteContentURLs(provider, resp.Body, link)
	return &out
}

func (h *VideoHandler) writeNative(c *gin.Context, resp *service.VideoUpstreamResponse, err error) {
	if resp != nil {
		service.WriteVideoUpstream(c, resp)
		return
	}
	status := http.StatusBadGateway
	if errors.Is(err, video.ErrUnsupported) {
		status = http.StatusBadRequest
	}
	h.openai.errorResponse(c, status, "upstream_error", "Video upstream request failed")
}

// forwardedNativeHeaders 只透传协议声明需要的客户端请求头。
func forwardedNativeHeaders(c *gin.Context, provider video.Provider) http.Header {
	forwarder, ok := provider.(video.NativeHeaderForwarder)
	if !ok {
		return nil
	}
	out := http.Header{}
	for _, name := range forwarder.ForwardHeaders() {
		if v := strings.TrimSpace(c.GetHeader(name)); v != "" {
			out.Set(name, v)
		}
	}
	return out
}

// NormalizeVideoNativeAuth 把厂商 SDK 的鉴权头（Vidu 的 "Token xxx"、PixVerse 的 API-KEY）
// 转为网关识别的 Bearer，放在 API Key 鉴权中间件之前。
func NormalizeVideoNativeAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := strings.TrimSpace(c.GetHeader("Authorization"))
		if scheme, token, ok := strings.Cut(auth, " "); ok && strings.EqualFold(scheme, "Token") {
			c.Request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
		}
		if auth == "" && c.GetHeader("x-api-key") == "" {
			if key := strings.TrimSpace(c.GetHeader("API-KEY")); key != "" {
				c.Request.Header.Set("Authorization", "Bearer "+key)
			}
		}
		c.Next()
	}
}
