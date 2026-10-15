package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// doOpenAIImageTestRequest 向 OpenAI 兼容的 /images/generations 发送一次测试请求，
// 返回状态码与完整响应体。错误文案沿用 testOpenAIImageAPIKey 的既有口径。
func (s *AccountTestService) doOpenAIImageTestRequest(
	ctx context.Context,
	account *Account,
	apiURL, authToken, proxyURL string,
	payload map[string]any,
) (int, []byte, error) {
	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("Failed to create request")
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)
	// 账号级请求头覆写：测试请求与真实转发保持一致的最终头
	account.ApplyHeaderOverrides(req.Header)

	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		return 0, nil, fmt.Errorf("Request failed: %s", err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("Failed to read response: %s", err.Error())
	}
	return resp.StatusCode, body, nil
}

// isRejectedResponseFormatParam 判断 400 响应是否明确指出 response_format 参数不被接受。
//
// OpenAI 官方 gpt-image 系列固定返回 b64_json，传 response_format 会直接 400
// （unknown_parameter），而不少兼容中转站要求显式指定；测试请求因此先带参数发送，
// 仅在被明确拒绝时才去掉重试，避免改变对中转站的既有行为。
func isRejectedResponseFormatParam(body []byte) bool {
	var parsed struct {
		Error struct {
			Param   string `json:"param"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(parsed.Error.Param), "response_format") {
		return true
	}
	return strings.Contains(strings.ToLower(parsed.Error.Message), "response_format")
}
