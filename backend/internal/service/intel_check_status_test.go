//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeriveIntelCheckState(t *testing.T) {
	rule := IntelCheckDegradedRule{FailStreak: 3, RecoverStreak: 2}

	tests := []struct {
		name     string
		statuses []string
		rule     IntelCheckDegradedRule
		previous string
		expected string
	}{
		{
			name:     "连续三次失败判为降智",
			statuses: []string{"fail", "fail", "fail"},
			rule:     rule,
			previous: "",
			expected: IntelCheckStateDegraded,
		},
		{
			name:     "仅两次失败未达阈值仍为正常",
			statuses: []string{"fail", "fail", "pass"},
			rule:     rule,
			previous: "",
			expected: IntelCheckStateNormal,
		},
		{
			name:     "request_error 不打断失败连续段",
			statuses: []string{"fail", "request_error", "fail", "fail"},
			rule:     rule,
			previous: "",
			expected: IntelCheckStateDegraded,
		},
		{
			name:     "running 同样不参与计数",
			statuses: []string{"running", "fail", "fail", "fail"},
			rule:     rule,
			previous: "",
			expected: IntelCheckStateDegraded,
		},
		{
			name:     "全为非决定性状态时未知",
			statuses: []string{"running", "request_error"},
			rule:     rule,
			previous: "",
			expected: IntelCheckStateUnknown,
		},
		{
			name:     "无样本时未知",
			statuses: nil,
			rule:     rule,
			previous: "",
			expected: IntelCheckStateUnknown,
		},
		{
			name:     "已降智时单次通过不足以恢复",
			statuses: []string{"pass", "fail", "fail", "fail"},
			rule:     rule,
			previous: IntelCheckStateDegraded,
			expected: IntelCheckStateDegraded,
		},
		{
			name:     "已降智且连续两次通过才恢复",
			statuses: []string{"pass", "pass", "fail", "fail", "fail"},
			rule:     rule,
			previous: IntelCheckStateDegraded,
			expected: IntelCheckStateNormal,
		},
		{
			name:     "阈值非法时回落到默认值而非恒判降智",
			statuses: []string{"fail", "fail"},
			rule:     IntelCheckDegradedRule{},
			previous: "",
			expected: IntelCheckStateNormal,
		},
		{
			name:     "阈值为一时单次失败即判降智",
			statuses: []string{"fail", "pass", "pass"},
			rule:     IntelCheckDegradedRule{FailStreak: 1, RecoverStreak: 1},
			previous: "",
			expected: IntelCheckStateDegraded,
		},
		{
			name:     "全部通过时为正常",
			statuses: []string{"pass", "pass", "pass"},
			rule:     rule,
			previous: "",
			expected: IntelCheckStateNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, DeriveIntelCheckState(tt.statuses, tt.rule, tt.previous))
		})
	}
}

// TestDeriveIntelCheckStateHysteresis 验证滞回：在阈值边界反复切换时状态不应抖动。
// 这是公开页最怕的现象——分组状态一会儿红一会儿绿，页面本身的可信度就没了。
func TestDeriveIntelCheckStateHysteresis(t *testing.T) {
	rule := IntelCheckDegradedRule{FailStreak: 2, RecoverStreak: 3}

	state := DeriveIntelCheckState([]string{"fail", "fail"}, rule, "")
	require.Equal(t, IntelCheckStateDegraded, state)

	// 通过次数未达恢复阈值前，始终维持降智。
	state = DeriveIntelCheckState([]string{"pass", "fail", "fail"}, rule, state)
	require.Equal(t, IntelCheckStateDegraded, state)

	state = DeriveIntelCheckState([]string{"pass", "pass", "fail", "fail"}, rule, state)
	require.Equal(t, IntelCheckStateDegraded, state)

	state = DeriveIntelCheckState([]string{"pass", "pass", "pass", "fail", "fail"}, rule, state)
	require.Equal(t, IntelCheckStateNormal, state)
}

func TestReplayIntelCheckState(t *testing.T) {
	rule := IntelCheckDegradedRule{FailStreak: 3, RecoverStreak: 2}

	tests := []struct {
		name        string
		statusesAsc []string
		rule        IntelCheckDegradedRule
		expected    string
	}{
		{"空输入为未知", nil, rule, IntelCheckStateUnknown},
		{"全为非决定性状态为未知", []string{"running", "request_error"}, rule, IntelCheckStateUnknown},
		{"全部通过为正常", []string{"pass", "pass", "pass"}, rule, IntelCheckStateNormal},
		{"连续三次失败判降智", []string{"pass", "fail", "fail", "fail"}, rule, IntelCheckStateDegraded},
		{
			// 回放与单次调用结论不同的那种输入，见下方 TestReplayIntelCheckState单次调用会漏掉滞回。
			name:        "降智后单次通过不恢复",
			statusesAsc: []string{"fail", "fail", "fail", "pass"},
			rule:        rule,
			expected:    IntelCheckStateDegraded,
		},
		{
			name:        "降智后连续两次通过才恢复",
			statusesAsc: []string{"fail", "fail", "fail", "pass", "pass"},
			rule:        rule,
			expected:    IntelCheckStateNormal,
		},
		{
			// 上游抖动不应把一段恢复中的连续通过洗断，否则永远恢复不了。
			name:        "恢复段中夹杂 request_error 不打断连续通过",
			statusesAsc: []string{"fail", "fail", "fail", "pass", "request_error", "pass"},
			rule:        rule,
			expected:    IntelCheckStateNormal,
		},
		{
			name:        "阈值非法时回落默认值而非恒判降智",
			statusesAsc: []string{"fail", "fail"},
			rule:        IntelCheckDegradedRule{},
			expected:    IntelCheckStateNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, ReplayIntelCheckState(tt.statusesAsc, tt.rule))
		})
	}
}

// TestReplayIntelCheckState入参方向 钉死升序语义。
// 时间线色块按旧→新排列，方向搞反会让状态由最旧一条主导，
// 页面上表现为「已经恢复好几轮了还挂着疑似降智」，而数据本身看不出错。
func TestReplayIntelCheckState入参方向(t *testing.T) {
	rule := IntelCheckDegradedRule{FailStreak: 3, RecoverStreak: 2}

	asc := []string{"fail", "fail", "fail", "pass", "pass"}
	require.Equal(t, IntelCheckStateNormal, ReplayIntelCheckState(asc, rule),
		"先三连失败再两连通过，最终应已恢复")

	reversed := []string{"pass", "pass", "fail", "fail", "fail"}
	require.Equal(t, IntelCheckStateDegraded, ReplayIntelCheckState(reversed, rule),
		"同一组样本反向输入必须得出不同结论，否则本用例无法证明方向")
}

// TestReplayIntelCheckState单次调用会漏掉滞回 固化「为什么公开页必须回放」。
//
// 这里刻意断言 DeriveIntelCheckState 的「错误」结论：它不是缺陷，而是该函数
// 依赖调用方传入 previous 的必然结果。若哪天这一断言变红，说明状态机语义
// 被改动过，ReplayIntelCheckState 的存在理由需要重新评估，而不是顺手改掉断言。
func TestReplayIntelCheckState单次调用会漏掉滞回(t *testing.T) {
	rule := IntelCheckDegradedRule{FailStreak: 3, RecoverStreak: 2}

	// 倒序：最新一条是 pass，其前三条为 fail。
	desc := []string{"pass", "fail", "fail", "fail"}
	require.Equal(t, IntelCheckStateNormal, DeriveIntelCheckState(desc, rule, ""),
		"previous 传空时，单次调用只看最新的连续段，会把尚未恢复的分组判成正常")

	asc := []string{"fail", "fail", "fail", "pass"}
	require.Equal(t, IntelCheckStateDegraded, ReplayIntelCheckState(asc, rule),
		"回放保留了中途判出的降智态，recover_streak 才真正生效")
}

func TestIntelCheckPassRate(t *testing.T) {
	tests := []struct {
		name     string
		statuses []string
		expected float64
		hasData  bool
	}{
		{"仅统计 pass 与 fail", []string{"pass", "fail", "pass", "request_error", "unverified"}, 2.0 / 3.0, true},
		{"全部通过", []string{"pass", "pass"}, 1.0, true},
		{"全部失败", []string{"fail", "fail"}, 0.0, true},
		{"无有效样本时无数据", []string{"running", "request_error"}, 0.0, false},
		{"空输入时无数据", nil, 0.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, ok := IntelCheckPassRate(tt.statuses)
			require.Equal(t, tt.hasData, ok)
			require.InDelta(t, tt.expected, rate, 1e-9)
		})
	}
}
