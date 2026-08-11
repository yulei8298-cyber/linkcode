package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIResponsesInputTokensUsesLocalEstimate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/input_tokens", bytes.NewReader([]byte(`{"model":"gpt-5","input":[{"role":"user","content":"hello"}]}`)))

	(&OpenAIGatewayHandler{}).ResponsesInputTokens(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"input_tokens":5}`, recorder.Body.String())
}
