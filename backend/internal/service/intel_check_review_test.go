//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseIntelCheckReviewResult(t *testing.T) {
	tests := []struct {
		name      string
		reply     string
		wantScore int
	}{
		{
			name:      "纯 JSON 直接解析",
			reply:     `{"score": 85, "summary": "整体完整", "items": [{"item": "主体完整度", "score": 22, "max_score": 25, "comment": "缺少尾羽"}]}`,
			wantScore: 85,
		},
		{
			name:      "剥离 Markdown 代码围栏",
			reply:     "```json\n{\"score\": 72, \"summary\": \"细节不足\", \"items\": []}\n```",
			wantScore: 72,
		},
		{
			name:      "剥离无语言标记的围栏",
			reply:     "```\n{\"score\": 60, \"summary\": \"偏简陋\", \"items\": []}\n```",
			wantScore: 60,
		},
		{
			name:      "截取夹在废话中间的 JSON",
			reply:     "好的，我的评审结论如下：\n{\"score\": 91, \"summary\": \"接近参考稿\", \"items\": []}\n以上。",
			wantScore: 91,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseIntelCheckReviewResult(tt.reply)
			require.NoError(t, err)
			require.Equal(t, tt.wantScore, result.Score)
		})
	}
}

// TestParseIntelCheckReviewResultUnparsable 解析失败必须能被调用方识别出来。
// 这类故障属于评审链路自身出问题，应记为 request_error，
// 不能当成受检分组答错——否则评审模型一抽风，所有分组集体被冤判降智。
func TestParseIntelCheckReviewResultUnparsable(t *testing.T) {
	tests := []struct {
		name  string
		reply string
	}{
		{"纯文本拒答", "抱歉，我无法评审这段代码。"},
		{"空回复", "   "},
		{"残缺 JSON", `{"score": 85, "summary":`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseIntelCheckReviewResult(tt.reply)
			require.ErrorIs(t, err, ErrIntelCheckReviewUnparsable)
		})
	}
}

func TestIntelCheckReviewResultNormalize(t *testing.T) {
	result := IntelCheckReviewResult{
		Score:   120,
		Summary: "  评分偏高  ",
		Items: []IntelCheckReviewItem{
			{Item: "动画质量", Score: 40, MaxScore: 25},
			{Item: "细节丰富度", Score: -5, MaxScore: 20},
		},
	}
	result.Normalize()

	require.Equal(t, 100, result.Score)
	require.Equal(t, "评分偏高", result.Summary)
	require.Equal(t, 25, result.Items[0].Score)
	require.Equal(t, 0, result.Items[1].Score)
}

func TestDecideIntelCheckDrawing(t *testing.T) {
	passedGate := IntelCheckGateResult{
		Pass:  true,
		Items: []IntelCheckGateItem{{Item: "造型数量", Pass: true, Detail: "100 / 参考 100 = 100%"}},
	}
	failedGate := IntelCheckGateResult{
		Pass:  false,
		Items: []IntelCheckGateItem{{Item: "造型数量", Pass: false, Detail: "30 / 参考 100 = 30%"}},
	}

	t.Run("门禁未过直接判失败且不消耗评审调用", func(t *testing.T) {
		status, detail := DecideIntelCheckDrawing(failedGate, nil, 80)
		require.Equal(t, IntelCheckStatusFail, status)
		require.Equal(t, false, detail["gate_pass"])
		require.NotContains(t, detail, "review_score")
	})

	t.Run("门禁通过且评审达标判通过", func(t *testing.T) {
		review := &IntelCheckReviewResult{Score: 85, Summary: "接近参考稿"}
		status, detail := DecideIntelCheckDrawing(passedGate, review, 80)
		require.Equal(t, IntelCheckStatusPass, status)
		require.Equal(t, 85, detail["review_score"])
	})

	t.Run("评分恰好等于及格线判通过", func(t *testing.T) {
		review := &IntelCheckReviewResult{Score: 80}
		status, _ := DecideIntelCheckDrawing(passedGate, review, 80)
		require.Equal(t, IntelCheckStatusPass, status)
	})

	t.Run("门禁通过但评审不达标判失败", func(t *testing.T) {
		review := &IntelCheckReviewResult{Score: 79}
		status, detail := DecideIntelCheckDrawing(passedGate, review, 80)
		require.Equal(t, IntelCheckStatusFail, status)
		require.Contains(t, detail["reason"], "低于及格线")
	})

	t.Run("缺失评审结果判失败", func(t *testing.T) {
		status, _ := DecideIntelCheckDrawing(passedGate, nil, 80)
		require.Equal(t, IntelCheckStatusFail, status)
	})

	t.Run("及格线非法时回落默认值", func(t *testing.T) {
		review := &IntelCheckReviewResult{Score: 85}
		status, detail := DecideIntelCheckDrawing(passedGate, review, 0)
		require.Equal(t, IntelCheckStatusPass, status)
		require.Equal(t, intelCheckDefaultPassScore, detail["pass_score"])
	})
}

func TestBuildIntelCheckReviewPrompt(t *testing.T) {
	candidate := DrawingMetrics{ShapeCount: 40, AnimatedTargets: 5, Mechanisms: []string{DrawingMechanismCSS}}
	reference := DrawingMetrics{ShapeCount: 100, AnimatedTargets: 20}

	t.Run("含参考稿时给出对照基准", func(t *testing.T) {
		prompt := BuildIntelCheckReviewPrompt(
			"画一只骑自行车的鹈鹕",
			"<svg id=\"reference\"></svg>",
			"<svg id=\"candidate\"></svg>",
			"",
			reference, candidate,
		)

		require.Contains(t, prompt, "画一只骑自行车的鹈鹕")
		require.Contains(t, prompt, "reference")
		require.Contains(t, prompt, "candidate")
		// 必须明确告知参考稿为 90 分基准，否则模型打分会脱锚。
		require.Contains(t, prompt, "90 分基准")
		// 未配置清单时应回落到内置清单。
		require.Contains(t, prompt, "动画质量")
	})

	t.Run("无参考稿时降级为绝对打分", func(t *testing.T) {
		prompt := BuildIntelCheckReviewPrompt("题面", "", "<svg></svg>", "自定义清单项", DrawingMetrics{}, candidate)

		require.Contains(t, prompt, "暂无参考稿")
		require.Contains(t, prompt, "自定义清单项")
		require.NotContains(t, prompt, "动画质量")
	})
}
