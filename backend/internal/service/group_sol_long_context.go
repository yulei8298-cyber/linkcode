package service

import (
	"encoding/json"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

// GPT-6.1 Sol 长上下文的分组策略（LinkCode 二开）。
//
// 官方 0.2.16 对订阅账号发出的 Sol 请求全局免收长上下文附加费，并把 Codex 目录的
// 默认上下文窗口提到 500k。这里改为按分组决定两件事：
//   - 是否向用户免收长上下文附加费（SolLongContextExempt，默认不免，保持既有计费）；
//   - 下发给 Codex 客户端的上下文窗口（SolContextWindow，0 表示沿用目录原值）。
//
// 账号统计成本仍按官方口径（订阅账号上游不收附加费）计算，见 openAISubscriptionLongContextExempt。

const (
	// SolContextWindowMin 防止误填过小的窗口，让 Codex 每轮都触发压缩。
	SolContextWindowMin = 16_000
	// SolContextWindowMax 取 GPT-6.1 Sol 官方 max_context_window。
	SolContextWindowMax = 872_000
)

// ErrInvalidSolContextWindow 后台填写的 Sol 上下文窗口超出允许范围。
var ErrInvalidSolContextWindow = infraerrors.BadRequest(
	"INVALID_SOL_CONTEXT_WINDOW",
	"sol_context_window must be 0 (use catalog default) or between 16000 and 872000",
)

// normalizeSolContextWindow 校验后台填写的窗口；0 表示不改写。
func normalizeSolContextWindow(window int) (int, error) {
	if window == 0 {
		return 0, nil
	}
	if window < SolContextWindowMin || window > SolContextWindowMax {
		return 0, ErrInvalidSolContextWindow
	}
	return window, nil
}

// groupSolLongContextExempt 判断本次请求是否按分组配置免收 Sol 长上下文附加费。
// 与官方订阅豁免一样按最终发送给上游的模型判断，避免客户别名或计费模型改变结论。
func groupSolLongContextExempt(group *Group, result *OpenAIForwardResult) bool {
	if group == nil || !group.SolLongContextExempt || result == nil {
		return false
	}
	model := strings.TrimSpace(result.UpstreamModel)
	if model == "" {
		model = result.Model
	}
	return openai.IsGPT61SolModelSpelling(model)
}

// applyGroupSolContextWindow 按分组配置改写目录中 GPT-6.1 Sol 条目的 context_window。
// 条目带 max_context_window 时不超过该上限；未配置或目录里没有 Sol 时原样返回。
func applyGroupSolContextWindow(body []byte, group *Group) ([]byte, bool, error) {
	if group == nil || group.SolContextWindow <= 0 || len(body) == 0 {
		return body, false, nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, false, err
	}
	var models []json.RawMessage
	if err := json.Unmarshal(envelope["models"], &models); err != nil {
		return nil, false, err
	}
	changed := false
	for i, rawModel := range models {
		updated, modelChanged, err := codexModelWithSolContextWindow(rawModel, int64(group.SolContextWindow))
		if err != nil {
			return nil, false, err
		}
		if modelChanged {
			models[i] = updated
			changed = true
		}
	}
	if !changed {
		return body, false, nil
	}
	rawModels, err := json.Marshal(models)
	if err != nil {
		return nil, false, err
	}
	envelope["models"] = rawModels
	updatedBody, err := json.Marshal(envelope)
	if err != nil {
		return nil, false, err
	}
	return updatedBody, true, nil
}

func codexModelWithSolContextWindow(rawModel json.RawMessage, window int64) (json.RawMessage, bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawModel, &fields); err != nil {
		// 无法解析的条目保持原样，由后续合并逻辑决定去留。
		return rawModel, false, nil
	}
	var slug string
	if rawSlug, ok := fields["slug"]; !ok || json.Unmarshal(rawSlug, &slug) != nil || !openai.IsGPT61SolModelSpelling(slug) {
		return rawModel, false, nil
	}
	var maxWindow int64
	if rawMax, ok := fields["max_context_window"]; ok {
		_ = json.Unmarshal(rawMax, &maxWindow)
	}
	if maxWindow > 0 && window > maxWindow {
		window = maxWindow
	}
	var current int64
	if rawCurrent, ok := fields["context_window"]; ok && json.Unmarshal(rawCurrent, &current) == nil && current == window {
		return rawModel, false, nil
	}
	rawWindow, err := json.Marshal(window)
	if err != nil {
		return nil, false, err
	}
	fields["context_window"] = rawWindow
	updated, err := json.Marshal(fields)
	if err != nil {
		return nil, false, err
	}
	return updated, true, nil
}
