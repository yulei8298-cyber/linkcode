//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// intelCheckGateReference 构造一份「满血参考稿」的结构指标。
func intelCheckGateReference() DrawingMetrics {
	return DrawingMetrics{
		ShapeCount:      100,
		AnimatedTargets: 20,
		DefsSymbols:     8,
		PathDataBytes:   10000,
		HTMLBytes:       40000,
		Mechanisms:      []string{DrawingMechanismScript},
		HasSVG:          true,
		HasTitle:        true,
		HasDesc:         true,
	}
}

// intelCheckGateItemByName 取出指定门禁项，便于断言具体哪一项挂了。
func intelCheckGateItemByName(t *testing.T, result IntelCheckGateResult, name string) IntelCheckGateItem {
	t.Helper()
	for _, item := range result.Items {
		if item.Item == name {
			return item
		}
	}
	require.Failf(t, "门禁项缺失", "未找到门禁项 %q", name)
	return IntelCheckGateItem{}
}

func TestEvaluateIntelCheckGatePass(t *testing.T) {
	reference := intelCheckGateReference()
	candidate := reference

	result := EvaluateIntelCheckGate("<svg></svg>", candidate, reference, IntelCheckDrawingRules{})
	require.True(t, result.Pass)
	for _, item := range result.Items {
		require.Truef(t, item.Pass, "门禁项 %q 不应失败：%s", item.Item, item.Detail)
	}
}

func TestEvaluateIntelCheckGateRatioThreshold(t *testing.T) {
	reference := intelCheckGateReference()
	rules := IntelCheckDrawingRules{MinRatio: 0.7}

	tests := []struct {
		name       string
		shapeCount int
		wantPass   bool
	}{
		{"恰好达到七成比例通过", 70, true},
		{"略低于七成比例失败", 69, false},
		{"高于参考稿通过", 120, true},
		{"造型数量腰斩失败", 50, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := reference
			candidate.ShapeCount = tt.shapeCount

			result := EvaluateIntelCheckGate("<svg></svg>", candidate, reference, rules)
			require.Equal(t, tt.wantPass, result.Pass)
			require.Equal(t, tt.wantPass, intelCheckGateItemByName(t, result, "造型数量").Pass)
		})
	}
}

func TestEvaluateIntelCheckGateFixedChecks(t *testing.T) {
	reference := intelCheckGateReference()

	t.Run("缺少 title 或 desc 失败", func(t *testing.T) {
		candidate := reference
		candidate.HasDesc = false

		result := EvaluateIntelCheckGate("<svg></svg>", candidate, reference, IntelCheckDrawingRules{})
		require.False(t, result.Pass)
		require.False(t, intelCheckGateItemByName(t, result, "包含 title 与 desc 说明").Pass)
	})

	t.Run("没有任何动画机制失败", func(t *testing.T) {
		candidate := reference
		candidate.Mechanisms = nil

		result := EvaluateIntelCheckGate("<svg></svg>", candidate, reference, IntelCheckDrawingRules{})
		require.False(t, result.Pass)
		require.False(t, intelCheckGateItemByName(t, result, "存在动画机制").Pass)
	})

	t.Run("缺失必需关键词失败", func(t *testing.T) {
		rules := IntelCheckDrawingRules{RequiredKeywords: []string{"鹈鹕", "自行车"}}

		result := EvaluateIntelCheckGate("<svg><title>鹈鹕</title></svg>", reference, reference, rules)
		require.False(t, result.Pass)

		item := intelCheckGateItemByName(t, result, "命中必需关键词")
		require.False(t, item.Pass)
		require.Contains(t, item.Detail, "自行车")
	})

	t.Run("体积超过上限失败", func(t *testing.T) {
		candidate := reference
		candidate.HTMLBytes = 2048

		rules := IntelCheckDrawingRules{MaxBytes: 1024}
		result := EvaluateIntelCheckGate("<svg></svg>", candidate, reference, rules)
		require.False(t, result.Pass)
		require.False(t, intelCheckGateItemByName(t, result, "体积未超上限").Pass)
	})
}

// TestEvaluateIntelCheckGateWithoutReference 参考稿尚未上传时，
// 相对指标项必须跳过而非判失败——否则新建题目会把所有分组冤判成降智。
func TestEvaluateIntelCheckGateWithoutReference(t *testing.T) {
	candidate := intelCheckGateReference()

	result := EvaluateIntelCheckGate("<svg></svg>", candidate, DrawingMetrics{}, IntelCheckDrawingRules{})
	require.True(t, result.Pass)
	require.Contains(t, intelCheckGateItemByName(t, result, "造型数量").Detail, "跳过")
}

func TestIntelCheckDrawingRulesNormalize(t *testing.T) {
	tests := []struct {
		name         string
		input        IntelCheckDrawingRules
		wantMinRatio float64
		wantMaxBytes int
	}{
		{"空配置回落默认值", IntelCheckDrawingRules{}, intelCheckDefaultMinRatio, intelCheckDefaultMaxBytes},
		{"比例为负回落默认值", IntelCheckDrawingRules{MinRatio: -1}, intelCheckDefaultMinRatio, intelCheckDefaultMaxBytes},
		{"比例超过一回落默认值", IntelCheckDrawingRules{MinRatio: 1.5}, intelCheckDefaultMinRatio, intelCheckDefaultMaxBytes},
		{"合法配置保持不变", IntelCheckDrawingRules{MinRatio: 0.8, MaxBytes: 1024}, 0.8, 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := tt.input
			rules.Normalize()
			require.InDelta(t, tt.wantMinRatio, rules.MinRatio, 1e-9)
			require.Equal(t, tt.wantMaxBytes, rules.MaxBytes)
		})
	}
}
