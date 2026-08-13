package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const synchronousImageStorageTimeout = 30 * time.Second

func (s *OpenAIGatewayService) resolveImageResultUploader() (*ImageResultUploader, bool) {
	if s == nil || s.imageStorageResolver == nil {
		return nil, false
	}
	return s.imageStorageResolver()
}

func (s *OpenAIGatewayService) rewriteImageResponseForDelivery(
	ctx context.Context,
	c *gin.Context,
	body []byte,
) []byte {
	uploader, enabled := s.resolveImageResultUploader()
	if !enabled || uploader == nil || len(body) == 0 {
		return s.rewriteImageResponseURLsFailOpen(ctx, c, body)
	}

	token, err := newImageProxyToken()
	if err != nil {
		logger.FromContext(ctx).Warn("openai.image_storage.token_failed", zap.Error(err))
		return s.rewriteImageResponseURLsFailOpen(ctx, c, body)
	}

	uploadCtx, cancel := context.WithTimeout(ctx, synchronousImageStorageTimeout)
	defer cancel()
	rewritten, err := uploader.Rewrite(uploadCtx, "sync_"+token, body)
	if err != nil {
		logger.FromContext(ctx).Warn("openai.image_storage.offload_failed", zap.Error(err))
		return s.rewriteImageResponseURLsFailOpen(ctx, c, body)
	}
	if bytes.Equal(rewritten, body) {
		return s.rewriteImageResponseURLsFailOpen(ctx, c, body)
	}
	return rewritten
}

func (s *OpenAIGatewayService) rewriteImageSSELineForDelivery(
	ctx context.Context,
	c *gin.Context,
	line []byte,
) []byte {
	lineEnding := ""
	trimmed := strings.TrimRight(string(line), "\r\n")
	lineEnding = string(line[len(trimmed):])
	data, ok := extractOpenAISSEDataLine(trimmed)
	if !ok || strings.TrimSpace(data) == "" || strings.TrimSpace(data) == "[DONE]" {
		return line
	}

	payload := []byte(data)
	if isCompletedImageSSEPayload(payload) {
		payload = s.rewriteCompletedImagePayloadForDelivery(ctx, c, payload)
	} else {
		payload = s.rewriteImageResponseURLsFailOpen(ctx, c, payload)
	}
	if string(payload) == data {
		return line
	}
	return []byte("data: " + string(payload) + lineEnding)
}

func (s *OpenAIGatewayService) rewriteCompletedImagePayloadForDelivery(
	ctx context.Context,
	c *gin.Context,
	payload []byte,
) []byte {
	uploader, enabled := s.resolveImageResultUploader()
	if !enabled || uploader == nil {
		return s.rewriteImageResponseURLsFailOpen(ctx, c, payload)
	}

	token, err := newImageProxyToken()
	if err != nil {
		logger.FromContext(ctx).Warn("openai.image_storage.token_failed", zap.Error(err))
		return s.rewriteImageResponseURLsFailOpen(ctx, c, payload)
	}

	uploadCtx, cancel := context.WithTimeout(ctx, synchronousImageStorageTimeout)
	defer cancel()
	rewritten, err := uploader.RewriteItem(uploadCtx, "sync_"+token, payload)
	if err != nil {
		logger.FromContext(ctx).Warn("openai.image_storage.stream_offload_failed", zap.Error(err))
		return s.rewriteImageResponseURLsFailOpen(ctx, c, payload)
	}
	return rewritten
}

func isCompletedImageSSEPayload(payload []byte) bool {
	eventType := strings.ToLower(strings.TrimSpace(gjson.GetBytes(payload, "type").String()))
	if strings.Contains(eventType, "partial") {
		return false
	}
	if strings.Contains(eventType, "completed") || strings.Contains(eventType, "done") {
		return gjson.GetBytes(payload, "b64_json").Exists() ||
			gjson.GetBytes(payload, "url").Exists() ||
			gjson.GetBytes(payload, "image_url").Exists()
	}
	return false
}

func (s *OpenAIGatewayService) rewriteBuiltImageStreamPayloadForDelivery(
	ctx context.Context,
	c *gin.Context,
	eventName string,
	payload []byte,
) []byte {
	if !strings.Contains(strings.ToLower(strings.TrimSpace(eventName)), "completed") {
		return payload
	}
	rewritten := s.rewriteCompletedImagePayloadForDelivery(ctx, c, payload)
	if len(rewritten) == 0 {
		logger.FromContext(ctx).Warn("openai.image_storage.empty_rewrite",
			zap.Error(fmt.Errorf("empty rewritten image payload")))
		return payload
	}
	return rewritten
}
