package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/tidwall/gjson"
)

// 智力检测的上游调用参数。
const (
	// intelCheckResponseMaxBytes 单次响应体上限。
	//
	// 取 2 MiB 而非复用 monitorResponseMaxBytes（64 KB）：绘图题的产物是完整
	// HTML，正常就有几百 KB，用监控那档上限会把合格答卷截成半截 JSON，
	// 最终以「解析失败」的形式误判成降智。
	intelCheckResponseMaxBytes = 2 << 20

	// intelCheckErrorBodyPreview 非 2xx 时保留的响应体片段长度。
	// 只进 error_message（不对外），供管理端排障。
	intelCheckErrorBodyPreview = 512

	intelCheckPathResponses       = "/responses"
	intelCheckPathChatCompletions = "/chat/completions"
)

// intelCheckTransport 智力检测专用 transport。
//
// 复用 safeDialContext 是必需的：它在 dial 时再次校验解析出的 IP，
// 能拦住 DNS rebinding —— 这是任何静态 URL 校验都做不到的。
//
// 但不能直接复用 monitorHTTPClient：那个 client 的 ResponseHeaderTimeout 是
// 30 秒，而本功能刻意要求高推理等级（xhigh 下首字节等上几分钟属正常），
// 30 秒会把满血模型判成请求失败，恰好与本功能要证明的事情相反。
// 这里把 ResponseHeaderTimeout 与 Client.Timeout 都留空，
// 超时统一由调用方的 context 控制——这样管理员改了超时设置即刻生效，
// 也不必为每种超时各建一个 client。
var intelCheckHTTPClient = &http.Client{
	Transport: servertiming.WrapRoundTripper(&http.Transport{
		DialContext:         safeDialContext,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        16,
		IdleConnTimeout:     monitorIdleConnTimeout,
		TLSHandshakeTimeout: monitorTLSHandshakeTimeout,
	}),
}

// intelCheckUpstreamRequest 一次上游调用的入参。
//
// 不直接收 *IntelCheckTarget：绘图题的评审调用会借用评审分组的地址与凭据，
// 但模型名与推理等级来自设置里的 DrawingJudge，两者字段来源不同。
type intelCheckUpstreamRequest struct {
	// BaseURL 需包含版本前缀（如 https://example.com/v1），与 Codex CLI 的
	// OPENAI_BASE_URL 约定一致；此处只做拼接，不替管理员补 /v1。
	BaseURL         string
	APIKey          string
	APIMode         string
	Model           string
	ReasoningEffort string
	Prompt          string
	// Stream 只在受检模型作答时开启；源码评审返回短 JSON，继续走同步响应。
	Stream bool
}

// intelCheckUpstreamReply 一次上游调用的产物。
type intelCheckUpstreamReply struct {
	Text      string
	LatencyMs int
	// InputTokens/OutputTokens 上游未返回 usage 时为 nil，区别于「返回了 0」。
	InputTokens  *int
	OutputTokens *int
}

// callIntelCheckUpstream 向受检分组发起请求并取回完整文本。
//
// 超时完全由 ctx 决定；返回的 error 一律视为 request_error（黄色块），
// 其文本只应写入 error_message，不得进入对外公开的 judge_detail。
func callIntelCheckUpstream(ctx context.Context, req intelCheckUpstreamRequest) (*intelCheckUpstreamReply, error) {
	payload, path, err := buildIntelCheckUpstreamBody(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, joinURL(req.BaseURL, path), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("构造请求失败：%w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}
	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(req.APIKey))

	startedAt := time.Now()
	resp, err := intelCheckHTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发起请求失败：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, overLimit, err := readIntelCheckBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}
	latencyMs := int(time.Since(startedAt).Milliseconds())

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("上游返回 %d：%s", resp.StatusCode, intelCheckBodyPreview(body))
	}
	// 超限单独报错而不是硬着头皮解析：截断后的 JSON 会以「解析失败」的面目出现，
	// 那条错误信息会把排障引向完全错误的方向。
	if overLimit {
		return nil, fmt.Errorf("响应体超过 %d 字节上限，无法完整解析", intelCheckResponseMaxBytes)
	}

	var (
		text         string
		inputTokens  *int
		outputTokens *int
	)
	if req.Stream {
		streamed, err := parseIntelCheckUpstreamStream(req.APIMode, body)
		if err != nil {
			return nil, err
		}
		text = streamed.Text
		inputTokens = streamed.InputTokens
		outputTokens = streamed.OutputTokens
	} else {
		text = extractIntelCheckReplyText(req.APIMode, body)
		if usage, ok := extractOpenAIUsageFromJSONBytes(body); ok {
			input, output := usage.InputTokens, usage.OutputTokens
			inputTokens, outputTokens = &input, &output
		}
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("上游响应中没有可用的文本内容：%s", intelCheckBodyPreview(body))
	}

	reply := &intelCheckUpstreamReply{
		Text: text, LatencyMs: latencyMs,
		InputTokens: inputTokens, OutputTokens: outputTokens,
	}
	return reply, nil
}

// buildIntelCheckUpstreamBody 按请求风格与调用用途组装请求体与路径。
//
// 刻意不带 max_tokens：绘图题的完整 HTML 动辄上万 token，
// 设上限等于把「答得太认真」变成失败。
// 受检模型作答开启 stream，以复现 Codex CLI 的实际传输路径；评审请求仍为同步。
func buildIntelCheckUpstreamBody(req intelCheckUpstreamRequest) ([]byte, string, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return nil, "", fmt.Errorf("模型名为空，无法发起检测")
	}
	// 推理等级原样透传（含 xhigh）：本功能要证明的正是「跑的是满血配置」，
	// 在这里悄悄降到 high，检测结论就失去了意义。
	// 上游若不支持该取值会回 400，那是 request_error，不会算到模型头上。
	effort := NormalizeIntelCheckReasoningEffort(req.ReasoningEffort)

	var (
		body map[string]any
		path string
	)
	if normalizeIntelCheckAPIMode(req.APIMode) == IntelCheckAPIModeChatCompletions {
		path = intelCheckPathChatCompletions
		body = map[string]any{
			"model":            model,
			"messages":         []map[string]string{{"role": "user", "content": req.Prompt}},
			"reasoning_effort": effort,
			"stream":           req.Stream,
		}
		if req.Stream {
			body["stream_options"] = map[string]bool{"include_usage": true}
		}
	} else {
		path = intelCheckPathResponses
		body = map[string]any{
			"model":     model,
			"input":     req.Prompt,
			"reasoning": map[string]string{"effort": effort},
			"stream":    req.Stream,
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("序列化请求体失败：%w", err)
	}
	return payload, path, nil
}

// readIntelCheckBody 读取响应体，返回内容与是否触顶。
// 多读一个字节来判断触顶：LimitReader 读满上限时无法区分「恰好这么长」与「被截断」。
func readIntelCheckBody(r io.Reader) ([]byte, bool, error) {
	body, err := io.ReadAll(io.LimitReader(r, intelCheckResponseMaxBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(body) > intelCheckResponseMaxBytes {
		return body[:intelCheckResponseMaxBytes], true, nil
	}
	return body, false, nil
}

// intelCheckBodyPreview 截取响应片段用于错误信息。
func intelCheckBodyPreview(body []byte) string {
	preview := strings.TrimSpace(string(body))
	if len(preview) > intelCheckErrorBodyPreview {
		return preview[:intelCheckErrorBodyPreview] + "…"
	}
	return preview
}

// extractIntelCheckReplyText 按请求风格取出最终文本。
func extractIntelCheckReplyText(apiMode string, body []byte) string {
	if normalizeIntelCheckAPIMode(apiMode) == IntelCheckAPIModeChatCompletions {
		return extractIntelCheckChatText(body)
	}
	// Responses 的 output 数组里 reasoning / tool-call item 可能排在 message 之前，
	// 这个函数已经处理了该顺序问题，不要换成固定下标取值。
	return extractOpenAIResponsesText(body)
}

// extractIntelCheckChatText 取 chat/completions 的回复文本。
// content 可能是字符串，也可能是 OpenAI 兼容层常见的分块数组。
func extractIntelCheckChatText(body []byte) string {
	content := gjson.GetBytes(body, "choices.0.message.content")
	if !content.IsArray() {
		return content.String()
	}

	var parts []string
	content.ForEach(func(_, block gjson.Result) bool {
		if text := block.Get("text").String(); strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
		return true
	})
	return strings.Join(parts, "")
}
