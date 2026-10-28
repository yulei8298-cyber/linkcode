package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/gin-gonic/gin"
)

// videoContentLinkTTL 是公开下载链接的有效期（上游成品地址多为 24 小时有效）。
const videoContentLinkTTL = 24 * time.Hour

// ErrVideoContentUnavailable 表示成品尚未生成或下载链接失效。
var ErrVideoContentUnavailable = errors.New("video content is not available")

func (s *VideoGatewayService) contentSecret() []byte {
	secret := ""
	if s.cfg != nil {
		secret = s.cfg.JWT.Secret
	}
	return []byte("video-content:" + secret)
}

func (s *VideoGatewayService) signContent(taskID string, expires int64) string {
	mac := hmac.New(sha256.New, s.contentSecret())
	_, _ = mac.Write([]byte(taskID + ":" + strconv.FormatInt(expires, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignedContentPath 返回免鉴权的成品下载路径（浏览器 <video> 可直接播放）。
func (s *VideoGatewayService) SignedContentPath(taskID string, now time.Time) string {
	expires := now.Add(videoContentLinkTTL).Unix()
	return "/video-files/" + url.PathEscape(taskID) + "?expires=" + strconv.FormatInt(expires, 10) + "&signature=" + s.signContent(taskID, expires)
}

// VerifyContentSignature 校验公开下载链接的签名与有效期。
func (s *VideoGatewayService) VerifyContentSignature(taskID, expiresRaw, signature string, now time.Time) bool {
	expires, err := strconv.ParseInt(expiresRaw, 10, 64)
	if err != nil || now.Unix() > expires {
		return false
	}
	return hmac.Equal([]byte(s.signContent(taskID, expires)), []byte(signature))
}

// HasContent 报告任务是否已有可下载的成品。
func (s *VideoGatewayService) HasContent(task *VideoTask) bool {
	return task != nil && task.Status == video.StatusSucceeded && strings.TrimSpace(task.VideoURL) != ""
}

// VideoTaskView 生成统一入口的任务对象，同时兼容 OpenAI（id / seconds / size）与
// new-api（task_id / url / metadata）两种客户端读取方式。
func (s *VideoGatewayService) VideoTaskView(task *VideoTask, publicBase string) gin.H {
	seconds := task.BillingDurationSeconds()
	view := gin.H{
		"id":         task.TaskID,
		"task_id":    task.TaskID,
		"object":     "video",
		"model":      task.Model,
		"status":     string(task.Status),
		"progress":   task.Progress,
		"created_at": task.CreatedAt.Unix(),
		"seconds":    strconv.Itoa(seconds),
		"duration":   seconds,
		"resolution": task.Resolution,
		"size":       video.SizeFor(task.Resolution, task.AspectRatio),
		"metadata":   gin.H{"duration": seconds, "resolution": task.Resolution, "aspect_ratio": task.AspectRatio},
	}
	if task.FinishedAt != nil {
		view["completed_at"] = task.FinishedAt.Unix()
	}
	if s.HasContent(task) {
		link := strings.TrimRight(publicBase, "/") + s.SignedContentPath(task.TaskID, time.Now())
		view["url"], view["video_url"] = link, link
	}
	if task.Status == video.StatusFailed || task.Status == video.StatusCancelled {
		message := video.FirstNonEmpty(task.ErrorMessage, "video generation "+string(task.Status))
		view["error"] = gin.H{"code": video.FirstNonEmpty(task.ErrorCode, string(task.Status)), "message": message}
		view["fail_reason"] = message
	}
	return view
}

// StreamContent 把成品视频流式回写给客户端；同源地址（如 OpenAI /content、Veo 文件）补带鉴权头。
func (s *VideoGatewayService) StreamContent(ctx context.Context, c *gin.Context, task *VideoTask) error {
	if task.Status != video.StatusSucceeded || strings.TrimSpace(task.VideoURL) == "" {
		return ErrVideoContentUnavailable
	}
	account, provider, err := s.LoadTaskAccount(ctx, task)
	if err != nil {
		return err
	}
	target, err := url.Parse(task.VideoURL)
	if err != nil || (target.Scheme != "https" && target.Scheme != "http") {
		return ErrVideoContentUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	if base, perr := url.Parse(videoAccountBaseURL(account, provider)); perr == nil && strings.EqualFold(base.Host, target.Host) {
		if err := provider.Authorize(req.Header, videoAccountCredentials(account, provider)); err != nil {
			return err
		}
		account.ApplyHeaderOverrides(req.Header)
	}
	resp, err := s.gateway.httpUpstream.Do(req, videoAccountProxy(account), account.ID, account.Concurrency)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: upstream status %d", ErrVideoContentUnavailable, resp.StatusCode)
	}
	if c.Writer.Header().Get("Content-Type") == "" && resp.Header.Get("Content-Type") == "" {
		c.Header("Content-Type", "video/mp4")
	}
	return writeGrokMediaContentResponse(c, resp)
}

// Cancel 请求上游取消任务；协议不支持取消时返回 video.ErrUnsupported。
func (s *VideoGatewayService) Cancel(ctx context.Context, task *VideoTask) (*VideoUpstreamResponse, error) {
	if task.Status.IsTerminal() {
		return nil, nil
	}
	account, provider, err := s.LoadTaskAccount(ctx, task)
	if err != nil {
		return nil, err
	}
	call, ok := provider.BuildCancel(task.UpstreamTaskID, task.UpstreamMeta)
	if !ok {
		return nil, video.ErrUnsupported
	}
	resp, err := s.doVideoCall(ctx, account, provider, call)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 300 {
		now := time.Now()
		_ = s.repo.UpdateSnapshot(ctx, task.ID, VideoTaskSnapshotUpdate{Status: video.StatusCancelled, FinishedAt: &now})
		_ = s.repo.MarkBilling(ctx, task.ID, VideoBillingSkipped, nil, "")
	}
	return resp, nil
}

// ForwardNativeFile 透传 MiniMax 按文件 ID 取下载地址的原生请求。
func (s *VideoGatewayService) ForwardNativeFile(ctx context.Context, task *VideoTask, fileID string) (*VideoUpstreamResponse, error) {
	account, provider, err := s.LoadTaskAccount(ctx, task)
	if err != nil {
		return nil, err
	}
	resolver, ok := provider.(video.FileResolver)
	if !ok {
		return nil, video.ErrUnsupported
	}
	return s.doVideoCall(ctx, account, provider, resolver.BuildFileQuery(fileID))
}

// WriteVideoUpstream 把上游原生响应回写给客户端（原生协议透传）。
func WriteVideoUpstream(c *gin.Context, resp *VideoUpstreamResponse) {
	contentType := video.FirstNonEmpty(resp.Header.Get("Content-Type"), "application/json")
	c.Data(resp.StatusCode, contentType, resp.Body)
}

// WriteVideoUpstreamResponse 回写提交时的上游原生响应（原生创建透传）。
func WriteVideoUpstreamResponse(c *gin.Context, result *VideoSubmitResult) {
	WriteVideoUpstream(c, result.Upstream)
}

// WriteVideoUpstreamError 回写上游错误响应。
func WriteVideoUpstreamError(c *gin.Context, upstreamErr *VideoUpstreamError) {
	WriteVideoUpstream(c, &VideoUpstreamResponse{StatusCode: upstreamErr.StatusCode, Header: upstreamErr.Header, Body: upstreamErr.Body})
}
