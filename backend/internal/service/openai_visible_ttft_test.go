package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIResponsesTTFTStartsAtStructuralProgress(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		name := "native"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			result := runSyntheticTTFTStream(t, passthrough, 250*time.Millisecond, 0,
				`{"type":"response.output_text.delta","delta":"test output"}`)
			require.NotNil(t, result.firstTokenMs)
			require.Less(t, *result.firstTokenMs, 200)
		})
	}
}

func TestOpenAIStreamStructuralProgressClassification(t *testing.T) {
	cases := []struct {
		name      string
		data      string
		eventType string
		want      bool
	}{
		{name: "created preamble", data: `{"type":"response.created"}`, eventType: "response.created", want: false},
		{name: "in progress preamble", data: `{"type":"response.in_progress"}`, eventType: "response.in_progress", want: false},
		{name: "empty reasoning item is structural progress", data: `{"type":"response.output_item.added","item":{"type":"reasoning","summary":[]}}`, eventType: "response.output_item.added", want: true},
		{name: "empty content part is structural progress", data: `{"type":"response.content_part.added","part":{"type":"output_text","text":""}}`, eventType: "response.content_part.added", want: true},
		{name: "empty delta is structural progress", data: `{"type":"response.output_text.delta","delta":""}`, eventType: "response.output_text.delta", want: true},
		{name: "response failed is terminal", data: `{"type":"response.failed"}`, eventType: "response.failed", want: false},
		{name: "response incomplete is terminal", data: `{"type":"response.incomplete"}`, eventType: "response.incomplete", want: false},
		{name: "done marker is terminal", data: `[DONE]`, eventType: "", want: false},
		{name: "retryable error is not progress", data: `{"type":"error","error":{"code":"server_is_overloaded"}}`, eventType: "error", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, openAIStreamDataStartsStructuralProgress(tc.data, tc.eventType))
		})
	}
}

func TestOpenAINativeProgressDisarmsTimeoutAndStartsTTFT(t *testing.T) {
	result := runSyntheticTTFTStream(t, false, 1200*time.Millisecond, 1,
		`{"type":"response.output_text.delta","delta":"test output"}`)
	require.NotNil(t, result.firstTokenMs)
	require.Less(t, *result.firstTokenMs, 500)
}

func TestOpenAINativePreambleDoesNotDisarmFirstOutputTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{
		MaxLineSize:                     defaultMaxLineSize,
		OpenAIFirstOutputTimeoutSeconds: 1,
	}}}
	reader, writer := io.Pipe()
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer func() { _ = writer.Close() }()
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_test\"}}\n\n")
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_test\"}}\n\n")
		time.Sleep(1200 * time.Millisecond)
	}()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: reader}
	account := &Account{ID: 1, Name: "account_test", Platform: PlatformOpenAI}

	_, err := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "test-model", "test-model")
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.True(t, failoverErr.SafeToFailoverAfterWrite)
	require.Empty(t, recorder.Body.String())
	select {
	case <-writerDone:
	case <-time.After(time.Second):
		t.Fatal("synthetic upstream writer did not exit")
	}
}

func runSyntheticTTFTStream(t *testing.T, passthrough bool, visibleDelay time.Duration, timeoutSeconds int, visibleEvent string) *openaiStreamingResult {
	t.Helper()
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{
		MaxLineSize:                     defaultMaxLineSize,
		OpenAIFirstOutputTimeoutSeconds: timeoutSeconds,
	}}}
	reader, writer := io.Pipe()
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer func() { _ = writer.Close() }()
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_test\"}}\n\n")
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"item_test\",\"type\":\"reasoning\",\"summary\":[]}}\n\n")
		time.Sleep(visibleDelay)
		_, _ = io.WriteString(writer, "data: "+visibleEvent+"\n\n")
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n")
	}()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: reader}
	account := &Account{ID: 1, Name: "account_test", Platform: PlatformOpenAI}
	started := time.Now()

	var result *openaiStreamingResult
	var err error
	if passthrough {
		var passthroughResult *openaiStreamingResultPassthrough
		passthroughResult, err = svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, started, "test-model", "test-model")
		if passthroughResult != nil {
			result = &openaiStreamingResult{firstTokenMs: passthroughResult.firstTokenMs}
		}
	} else {
		result, err = svc.handleStreamingResponse(context.Background(), resp, c, account, started, "test-model", "test-model")
	}
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, recorder.Body.String(), `"type":"response.output_item.added"`)
	require.Contains(t, recorder.Body.String(), visibleEvent)
	select {
	case <-writerDone:
	case <-time.After(time.Second):
		t.Fatal("synthetic upstream writer did not exit")
	}
	return result
}
