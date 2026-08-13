package service

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/gin-gonic/gin"
)

const (
	imageProxyURLTTL           = 24 * time.Hour
	imageProxyRequestTimeout   = 2 * time.Minute
	imageProxyMaxResponseBytes = 50 << 20
	imageProxyTokenBytes       = 24
)

var imageProxyHTTPClient = newImageProxyHTTPClient()

func newImageProxyHTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext:           safeDialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	client := &http.Client{
		Timeout:   imageProxyRequestTimeout,
		Transport: servertiming.WrapRoundTripper(transport),
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many image proxy redirects")
		}
		return validateImageProxyTargetURL(req.URL.String())
	}
	return client
}

func (s *OpenAIGatewayService) imageProxyStore() ImageProxyURLStore {
	if s == nil || s.cache == nil {
		return nil
	}
	store, _ := s.cache.(ImageProxyURLStore)
	return store
}

func validateImageProxyTargetURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return errors.New("invalid image proxy target url")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return errors.New("unsupported image proxy target scheme")
	}
	host := parsed.Hostname()
	if parsed.User != nil || isBlockedHostname(host) {
		return errors.New("image proxy target is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && isPrivateIP(ip) {
		return errors.New("image proxy target is not allowed")
	}
	return nil
}

func (s *OpenAIGatewayService) rewriteImageResponseURLsFailOpen(ctx context.Context, c *gin.Context, body []byte) []byte {
	rewritten, err := s.rewriteImageResponseURLs(ctx, c, body)
	if err != nil {
		logger.LegacyPrintf("service.openai_image_proxy", "image url rewrite degraded: %v", err)
		if len(rewritten) > 0 {
			return rewritten
		}
		return body
	}
	return rewritten
}

func (s *OpenAIGatewayService) rewriteImageSSELineFailOpen(ctx context.Context, c *gin.Context, line []byte) []byte {
	lineEnding := ""
	trimmed := strings.TrimRight(string(line), "\r\n")
	lineEnding = string(line[len(trimmed):])
	data, ok := extractOpenAISSEDataLine(trimmed)
	if !ok || strings.TrimSpace(data) == "" || strings.TrimSpace(data) == "[DONE]" {
		return line
	}
	rewritten := s.rewriteImageResponseURLsFailOpen(ctx, c, []byte(data))
	if string(rewritten) == data {
		return line
	}
	return []byte("data: " + string(rewritten) + lineEnding)
}

func newImageProxyToken() (string, error) {
	raw := make([]byte, imageProxyTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func isValidImageProxyToken(token string) bool {
	if len(token) != base64.RawURLEncoding.EncodedLen(imageProxyTokenBytes) {
		return false
	}
	for _, char := range token {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func imageProxyRequestBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	host := strings.TrimSpace(c.Request.Host)
	if host == "" || strings.ContainsAny(host, "/\\\r\n") {
		return ""
	}
	scheme := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")))
	if comma := strings.IndexByte(scheme, ','); comma >= 0 {
		scheme = strings.TrimSpace(scheme[:comma])
	}
	if scheme != "http" && scheme != "https" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + host
}

func isAllowedImageProxyContentType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png", "image/jpeg", "image/webp", "image/gif", "image/avif", "image/bmp":
		return true
	default:
		return false
	}
}

func (s *OpenAIGatewayService) rewriteImageResponseURLs(ctx context.Context, c *gin.Context, body []byte) ([]byte, error) {
	store := s.imageProxyStore()
	if len(body) == 0 {
		return body, nil
	}
	baseURL := imageProxyRequestBaseURL(c)
	if baseURL == "" {
		return nil, errors.New("image proxy public url is unavailable")
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return body, nil
	}
	changed := false
	var firstErr error
	registered := make(map[string]string)
	var rewrite func(any, string) error
	rewrite = func(value any, field string) error {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if text, ok := child.(string); ok && (key == "url" || key == "image_url") {
					trimmed := strings.TrimSpace(text)
					if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
						unavailableURL := baseURL + "/v1/images/proxy/unavailable"
						if store == nil {
							typed[key] = unavailableURL
							changed = true
							if firstErr == nil {
								firstErr = errors.New("image proxy store is unavailable")
							}
							continue
						}
						proxyURL, ok := registered[trimmed]
						if !ok {
							if err := validateImageProxyTargetURL(trimmed); err != nil {
								typed[key] = unavailableURL
								changed = true
								if firstErr == nil {
									firstErr = err
								}
								continue
							}
							token, err := newImageProxyToken()
							if err != nil {
								typed[key] = unavailableURL
								changed = true
								if firstErr == nil {
									firstErr = err
								}
								continue
							}
							if err := store.SetImageProxyURL(ctx, token, trimmed, imageProxyURLTTL); err != nil {
								typed[key] = unavailableURL
								changed = true
								if firstErr == nil {
									firstErr = fmt.Errorf("store image proxy url: %w", err)
								}
								continue
							}
							proxyURL = baseURL + "/v1/images/proxy/" + token
							registered[trimmed] = proxyURL
						}
						typed[key] = proxyURL
						changed = true
						continue
					}
				}
				if err := rewrite(child, key); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range typed {
				if err := rewrite(child, field); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := rewrite(payload, ""); err != nil {
		return nil, err
	}
	if !changed {
		return body, nil
	}
	rewritten, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return rewritten, firstErr
}

func (s *OpenAIGatewayService) ServeImageProxy(ctx context.Context, token string, w http.ResponseWriter) error {
	store := s.imageProxyStore()
	token = strings.TrimSpace(token)
	if store == nil || !isValidImageProxyToken(token) {
		return ErrImageProxyURLNotFound
	}
	upstreamURL, err := store.GetImageProxyURL(ctx, token)
	if err != nil {
		return err
	}
	if err := validateImageProxyTargetURL(upstreamURL); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/gif,image/*;q=0.8")
	req.Header.Set("User-Agent", "LinkCode-Image-Proxy/1.0")
	resp, err := imageProxyHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch proxied image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("proxied image upstream returned status %d", resp.StatusCode)
	}
	if resp.ContentLength > imageProxyMaxResponseBytes {
		return errors.New("proxied image is too large")
	}
	reader := bufio.NewReader(resp.Body)
	peek, _ := reader.Peek(512)
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if !isAllowedImageProxyContentType(contentType) {
		contentType = strings.ToLower(http.DetectContentType(peek))
	}
	if !isAllowedImageProxyContentType(contentType) {
		return errors.New("proxied content is not an image")
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	limited := io.LimitReader(reader, imageProxyMaxResponseBytes+1)
	written, copyErr := io.Copy(w, limited)
	if written > imageProxyMaxResponseBytes {
		logger.LegacyPrintf("service.openai_image_proxy", "proxied image exceeded size limit")
		return errors.New("proxied image is too large")
	}
	return copyErr
}
