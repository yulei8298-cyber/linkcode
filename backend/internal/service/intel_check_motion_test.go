//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type intelCheckMotionEvaluatorStub struct {
	result *IntelCheckMotionEvaluation
	err    error
}

type intelCheckMotionRoundTripFunc func(*http.Request) (*http.Response, error)

func (f intelCheckMotionRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func intelCheckMotionHTTPForTest(t *testing.T, status int, body string) IntelCheckMotionEvaluator {
	t.Helper()
	evaluator := NewIntelCheckMotionEvaluator(config.IntelCheckMotionConfig{
		Enabled: true, EvaluatorURL: "http://motion.test", TimeoutSeconds: 2,
	}).(*httpIntelCheckMotionEvaluator)
	evaluator.client = &http.Client{Transport: intelCheckMotionRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "/evaluate", request.URL.Path)
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
	return evaluator
}

func (s intelCheckMotionEvaluatorStub) Evaluate(context.Context, string) (*IntelCheckMotionEvaluation, error) {
	return s.result, s.err
}

func intelCheckPassingMotionEvaluator() IntelCheckMotionEvaluator {
	return intelCheckMotionEvaluatorStub{result: &IntelCheckMotionEvaluation{
		Version: "browser_motion_v1", Verifiable: true, Pass: true,
		Reason: "轨迹通过", Checks: []IntelCheckMotionCheck{},
	}}
}

func TestIntelCheckEffectivePrompt只给绘图题追加动作协议(t *testing.T) {
	logic := &IntelCheckQuestion{Kind: IntelCheckKindLogic, Prompt: "回答问题"}
	drawing := &IntelCheckQuestion{Kind: IntelCheckKindDrawing, Prompt: "画一辆自行车"}
	require.Equal(t, logic.Prompt, intelCheckEffectivePrompt(logic))
	require.Contains(t, intelCheckEffectivePrompt(drawing), drawing.Prompt)
	for _, part := range []string{"wheel-rear", "crank-center", "pedal-left", "foot-right", "leg-right"} {
		require.Contains(t, intelCheckEffectivePrompt(drawing), part)
	}
}

func TestIntelCheckMotionHTTPClient(t *testing.T) {
	t.Run("关闭配置不创建客户端", func(t *testing.T) {
		require.Nil(t, NewIntelCheckMotionEvaluator(config.IntelCheckMotionConfig{}))
	})

	t.Run("解析公开安全结果", func(t *testing.T) {
		evaluator := intelCheckMotionHTTPForTest(t, http.StatusOK,
			`{"version":"browser_motion_v1","verifiable":true,"pass":true,"reason":"通过","checks":[]}`)
		result, err := evaluator.Evaluate(context.Background(), "<svg/>")
		require.NoError(t, err)
		require.True(t, result.Verifiable)
		require.True(t, result.Pass)
		require.Empty(t, result.Checks)
	})

	t.Run("非成功状态只作为内部错误", func(t *testing.T) {
		evaluator := intelCheckMotionHTTPForTest(t, http.StatusServiceUnavailable, "internal detail")
		_, err := evaluator.Evaluate(context.Background(), "<svg/>")
		require.ErrorContains(t, err, "HTTP 503")
		require.NotContains(t, err.Error(), "internal detail")
	})
}

func TestJudgeIntelCheckDrawing动作验收状态分支(t *testing.T) {
	question := intelCheckDrawingQuestion(nil)
	cfg := DefaultIntelCheckSettings()

	t.Run("未配置验收器为未验证", func(t *testing.T) {
		outcome := (&IntelCheckService{}).judgeIntelCheckDrawing(context.Background(), &cfg, question, nil, intelCheckPassingDrawing)
		require.Equal(t, IntelCheckStatusUnverified, outcome.Status)
		require.Equal(t, false, outcome.JudgeDetail["kinematics_verified"])
	})

	t.Run("缺少标记为未验证", func(t *testing.T) {
		svc := &IntelCheckService{motion: intelCheckMotionEvaluatorStub{result: &IntelCheckMotionEvaluation{
			Version: "browser_motion_v1", Verifiable: false, Pass: false,
			Reason: "缺少 pedal-left", Checks: []IntelCheckMotionCheck{},
		}}}
		outcome := svc.judgeIntelCheckDrawing(context.Background(), &cfg, question, nil, intelCheckPassingDrawing)
		require.Equal(t, IntelCheckStatusUnverified, outcome.Status)
		require.Contains(t, outcome.JudgeDetail["reason"], "动作未验证")
	})

	t.Run("轨迹不合格为失败", func(t *testing.T) {
		svc := &IntelCheckService{motion: intelCheckMotionEvaluatorStub{result: &IntelCheckMotionEvaluation{
			Version: "browser_motion_v1", Verifiable: true, Pass: false,
			Reason: "脚踏脱节", Checks: []IntelCheckMotionCheck{{Item: "脚跟随踏板", Pass: false, Detail: "距离超限"}},
		}}}
		outcome := svc.judgeIntelCheckDrawing(context.Background(), &cfg, question, nil, intelCheckPassingDrawing)
		require.Equal(t, IntelCheckStatusFail, outcome.Status)
		require.Equal(t, true, outcome.JudgeDetail["kinematics_verified"])
	})

	t.Run("验收器故障为请求失败且不公开原文", func(t *testing.T) {
		svc := &IntelCheckService{motion: intelCheckMotionEvaluatorStub{err: errors.New("http://internal-motion:8081 secret")}}
		outcome := svc.judgeIntelCheckDrawing(context.Background(), &cfg, question, nil, intelCheckPassingDrawing)
		require.Equal(t, IntelCheckStatusRequestError, outcome.Status)
		require.Contains(t, outcome.ErrorMessage, "internal-motion")
		require.NotContains(t, intelCheckDetailJSON(t, outcome), "internal-motion")
	})
}
