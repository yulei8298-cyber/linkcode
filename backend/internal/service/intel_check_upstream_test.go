//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func withIntelCheckHTTPClient(t *testing.T, client *http.Client) {
	t.Helper()
	original := intelCheckHTTPClient
	intelCheckHTTPClient = client
	t.Cleanup(func() { intelCheckHTTPClient = original })
}

type intelCheckRoundTripFunc func(*http.Request) (*http.Response, error)

func (f intelCheckRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func intelCheckResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestBuildIntelCheckUpstreamBody_按协议生成请求体(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		wantPath  string
		checkBody func(t *testing.T, body map[string]any)
	}{
		{
			name:     "responses",
			mode:     IntelCheckAPIModeResponses,
			wantPath: intelCheckPathResponses,
			checkBody: func(t *testing.T, body map[string]any) {
				require.Equal(t, "gpt-test", body["model"])
				require.Equal(t, "请回答", body["input"])
				require.Equal(t, false, body["stream"])
				reasoning, ok := body["reasoning"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "xhigh", reasoning["effort"])
				require.NotContains(t, body, "messages")
			},
		},
		{
			name:     "chat completions",
			mode:     IntelCheckAPIModeChatCompletions,
			wantPath: intelCheckPathChatCompletions,
			checkBody: func(t *testing.T, body map[string]any) {
				require.Equal(t, "gpt-test", body["model"])
				require.Equal(t, "xhigh", body["reasoning_effort"])
				require.Equal(t, false, body["stream"])
				messages, ok := body["messages"].([]any)
				require.True(t, ok)
				require.Len(t, messages, 1)
				message, ok := messages[0].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "user", message["role"])
				require.Equal(t, "请回答", message["content"])
				require.NotContains(t, body, "input")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, path, err := buildIntelCheckUpstreamBody(intelCheckUpstreamRequest{
				APIMode:         tt.mode,
				Model:           " gpt-test ",
				ReasoningEffort: "xhigh",
				Prompt:          "请回答",
			})
			require.NoError(t, err)
			require.Equal(t, tt.wantPath, path)

			var body map[string]any
			require.NoError(t, json.Unmarshal(payload, &body))
			tt.checkBody(t, body)
		})
	}
}

func TestCallIntelCheckUpstream_非2xx只保留截断预览(t *testing.T) {
	body := strings.Repeat("x", intelCheckErrorBodyPreview+100)
	withIntelCheckHTTPClient(t, &http.Client{Transport: intelCheckRoundTripFunc(
		func(_ *http.Request) (*http.Response, error) {
			return intelCheckResponse(http.StatusBadGateway, body), nil
		},
	)})

	_, err := callIntelCheckUpstream(context.Background(), intelCheckUpstreamRequest{
		BaseURL: "https://example.test/v1",
		APIMode: IntelCheckAPIModeResponses,
		Model:   "gpt-test",
		Prompt:  "请回答",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "上游返回 502")
	require.Contains(t, err.Error(), strings.Repeat("x", intelCheckErrorBodyPreview)+"…")
	require.NotContains(t, err.Error(), strings.Repeat("x", intelCheckErrorBodyPreview+1))
}

func TestCallIntelCheckUpstream_响应体超过2MiB时拒绝解析(t *testing.T) {
	withIntelCheckHTTPClient(t, &http.Client{Transport: intelCheckRoundTripFunc(
		func(_ *http.Request) (*http.Response, error) {
			return intelCheckResponse(http.StatusOK, strings.Repeat("a", intelCheckResponseMaxBytes+1)), nil
		},
	)})

	_, err := callIntelCheckUpstream(context.Background(), intelCheckUpstreamRequest{
		BaseURL: "https://example.test",
		APIMode: IntelCheckAPIModeResponses,
		Model:   "gpt-test",
		Prompt:  "请回答",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "响应体超过 2097152 字节上限")
}

func TestCallIntelCheckUpstream_成功响应回填文本耗时与usage(t *testing.T) {
	withIntelCheckHTTPClient(t, &http.Client{Transport: intelCheckRoundTripFunc(
		func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "/v1/responses", r.URL.Path)
			require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
			response := intelCheckResponse(http.StatusOK, `{"output":[{"type":"message","content":[{"type":"output_text","text":"答案是 42"}]}],"usage":{"input_tokens":12,"output_tokens":34}}`)
			response.Header.Set("Content-Type", "application/json")
			return response, nil
		},
	)})

	reply, err := callIntelCheckUpstream(context.Background(), intelCheckUpstreamRequest{
		BaseURL: "https://example.test/v1/",
		APIKey:  "secret",
		APIMode: IntelCheckAPIModeResponses,
		Model:   "gpt-test",
		Prompt:  "请回答",
	})
	require.NoError(t, err)
	require.Equal(t, "答案是 42", reply.Text)
	require.NotNil(t, reply.InputTokens)
	require.NotNil(t, reply.OutputTokens)
	require.Equal(t, 12, *reply.InputTokens)
	require.Equal(t, 34, *reply.OutputTokens)
	require.GreaterOrEqual(t, reply.LatencyMs, 0)
}

type intelCheckCRUDEncryptor struct{}

func (intelCheckCRUDEncryptor) Encrypt(plain string) (string, error) {
	return "enc:" + plain, nil
}

func (intelCheckCRUDEncryptor) Decrypt(ciphertext string) (string, error) {
	return strings.TrimPrefix(ciphertext, "enc:"), nil
}

func TestIntelCheckTargetService_CRUD与凭据边界(t *testing.T) {
	repo := &intelCheckRepoStub{}
	svc := NewIntelCheckService(repo, nil, intelCheckCRUDEncryptor{})
	params := IntelCheckTargetParams{
		Name:            "  测试分组  ",
		Description:     "说明",
		BaseURL:         "https://example.test/v1/",
		APIKey:          "secret",
		APIMode:         IntelCheckAPIModeResponses,
		Model:           " gpt-test ",
		ReasoningEffort: IntelCheckEffortHigh,
		Enabled:         true,
	}

	created, err := svc.CreateTarget(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, "secret", created.APIKey)
	require.Equal(t, "enc:secret", repo.targets[created.ID].APIKey)
	require.Equal(t, "测试分组", created.Name)
	require.Equal(t, "https://example.test/v1", created.BaseURL)

	fetched, err := svc.GetTarget(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "secret", fetched.APIKey)

	updated, err := svc.UpdateTarget(context.Background(), created.ID, IntelCheckTargetParams{
		Name:            "更新分组",
		BaseURL:         "https://example.test/v2",
		Model:           "gpt-updated",
		ReasoningEffort: IntelCheckEffortXHigh,
		Enabled:         false,
	})
	require.NoError(t, err)
	require.Equal(t, "secret", updated.APIKey)
	require.Equal(t, "enc:secret", repo.targets[created.ID].APIKey)
	require.False(t, updated.Enabled)

	updated, err = svc.UpdateTarget(context.Background(), created.ID, IntelCheckTargetParams{
		Name:    "再次更新",
		BaseURL: "https://example.test/v3",
		APIKey:  "new-secret",
		Model:   "gpt-new",
		Enabled: true,
	})
	require.NoError(t, err)
	require.Equal(t, "new-secret", updated.APIKey)
	require.Equal(t, "enc:new-secret", repo.targets[created.ID].APIKey)

	require.NoError(t, svc.DeleteTarget(context.Background(), created.ID))
	_, err = svc.GetTarget(context.Background(), created.ID)
	require.ErrorIs(t, err, ErrIntelCheckTargetNotFound)
}

func TestIntelCheckQuestionService_CRUD与题型字段覆盖(t *testing.T) {
	repo := &intelCheckRepoStub{}
	svc := NewIntelCheckService(repo, nil, nil)

	created, err := svc.CreateQuestion(context.Background(), IntelCheckQuestionParams{
		Kind:           IntelCheckKindLogic,
		Title:          "逻辑题",
		Prompt:         "答案是多少？",
		ExpectedAnswer: "42",
		MatchMode:      IntelCheckMatchNumeric,
		Enabled:        true,
	})
	require.NoError(t, err)
	require.Equal(t, IntelCheckKindLogic, created.Kind)
	require.Equal(t, IntelCheckMatchNumeric, created.MatchMode)

	fetched, err := svc.GetQuestion(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, "42", fetched.ExpectedAnswer)

	updated, err := svc.UpdateQuestion(context.Background(), created.ID, IntelCheckQuestionParams{
		Kind:          IntelCheckKindDrawing,
		Title:         "绘图题",
		Prompt:        "请画图",
		ReferenceHTML: referenceHTMLFixture,
		DrawingRules:  map[string]any{"min_ratio": 0.8},
		ReviewRubric:  "检查动画",
		Enabled:       true,
	})
	require.NoError(t, err)
	require.Equal(t, IntelCheckKindDrawing, updated.Kind)
	require.Empty(t, updated.ExpectedAnswer)
	require.Equal(t, IntelCheckMatchExact, updated.MatchMode)
	require.NotEmpty(t, updated.ReferenceMetrics)
	require.NotEmpty(t, updated.DrawingRules)

	require.NoError(t, svc.DeleteQuestion(context.Background(), created.ID))
	_, err = svc.GetQuestion(context.Background(), created.ID)
	require.ErrorIs(t, err, ErrIntelCheckQuestionNotFound)
}
