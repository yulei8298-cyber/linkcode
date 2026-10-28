package handler

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func newStageTimingObserver() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zap.InfoLevel)
	return zap.New(core), logs
}

func stageTimingAccount(enabled bool) *service.Account {
	extra := map[string]any{}
	if enabled {
		extra[service.OpenAIStageTimingExtraKey] = true
	}
	return &service.Account{ID: 659, Name: "krill-svip", Extra: extra}
}

func newPopulatedStageTiming() *service.OpenAIStageTiming {
	timing := service.NewOpenAIStageTiming(nil)
	timing.Set(service.StageBodyBytes, 2048)
	timing.Set(service.StageBodyRead, 12)
	timing.Set(service.StagePreForward, 30)
	timing.Set(service.StageForward, 900)
	return timing
}

func TestLogOpenAIStageTimingSkipsAccountWithoutSwitch(t *testing.T) {
	logger, logs := newStageTimingObserver()
	logOpenAIStageTiming(logger, newPopulatedStageTiming(), stageTimingAccount(false), 0, nil, nil)
	require.Equal(t, 0, logs.Len())
}

func TestLogOpenAIStageTimingToleratesNilInputs(t *testing.T) {
	logger, logs := newStageTimingObserver()
	require.NotPanics(t, func() {
		logOpenAIStageTiming(nil, newPopulatedStageTiming(), stageTimingAccount(true), 0, nil, nil)
		logOpenAIStageTiming(logger, nil, stageTimingAccount(true), 0, nil, nil)
		logOpenAIStageTiming(logger, newPopulatedStageTiming(), nil, 0, nil, nil)
	})
	require.Equal(t, 0, logs.Len())
}

func TestLogOpenAIStageTimingSuccessIncludesStagesAndFirstToken(t *testing.T) {
	logger, logs := newStageTimingObserver()
	firstTokenMs := 500
	timing := newPopulatedStageTiming()
	timing.Set(service.StageForwardEntry, 5)

	logOpenAIStageTiming(logger, timing, stageTimingAccount(true), 1, &service.OpenAIForwardResult{FirstTokenMs: &firstTokenMs}, nil)

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	require.Equal(t, "openai.stage_timing", entry.Message)
	fields := entry.ContextMap()
	require.Equal(t, int64(659), fields["account_id"])
	require.Equal(t, "krill-svip", fields["account_name"])
	require.Equal(t, int64(1), fields["account_switches"])
	require.Equal(t, "ok", fields["outcome"])
	require.NotContains(t, fields, "upstream_status")
	require.Equal(t, int64(2048), fields["body_bytes"])
	require.Equal(t, int64(500), fields["first_token_ms"])
	require.Equal(t, int64(535), fields["ttft_from_request_ms"], "pre_forward 30 + forward_entry 5 + first_token 500")
}

func TestLogOpenAIStageTimingClassifiesFailures(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantResult string
		wantStatus interface{}
	}{
		{name: "failover 带状态码", err: fmt.Errorf("wrapped: %w", &service.UpstreamFailoverError{StatusCode: 429}), wantResult: "failover", wantStatus: int64(429)},
		{name: "普通错误", err: errors.New("boom"), wantResult: "error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, logs := newStageTimingObserver()
			logOpenAIStageTiming(logger, newPopulatedStageTiming(), stageTimingAccount(true), 0, nil, tt.err)

			require.Equal(t, 1, logs.Len())
			fields := logs.All()[0].ContextMap()
			require.Equal(t, tt.wantResult, fields["outcome"])
			if tt.wantStatus == nil {
				require.NotContains(t, fields, "upstream_status")
			} else {
				require.Equal(t, tt.wantStatus, fields["upstream_status"])
			}
			require.NotContains(t, fields, "first_token_ms", "没有 result 时不应凭空出现首字字段")
		})
	}
}
