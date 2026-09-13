//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// referenceHTMLFixture 一份最小但完整的参考稿：带 title/desc、defs 符号、
// 路径数据与一个 SMIL 动画，足以让 ComputeDrawingMetrics 的各项指标都非零。
const referenceHTMLFixture = `<!DOCTYPE html><html><body>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <title>参考稿</title>
  <desc>用于结构门禁的基准图</desc>
  <defs><symbol id="dot"><circle cx="5" cy="5" r="3"/></symbol></defs>
  <path d="M10 10 L90 90 L10 90 Z"/>
  <rect x="20" y="20" width="30" height="30"/>
  <circle cx="50" cy="50" r="10">
    <animate attributeName="r" dur="1s" values="5;15;5" repeatCount="indefinite"/>
  </circle>
</svg></body></html>`

func TestNormalizeIntelCheckQuestionKind(t *testing.T) {
	t.Run("大小写与空白容错", func(t *testing.T) {
		for _, in := range []string{"logic", "LOGIC", "  Logic  "} {
			got, err := normalizeIntelCheckQuestionKind(in)
			require.NoError(t, err)
			require.Equal(t, IntelCheckKindLogic, got)
		}
		got, err := normalizeIntelCheckQuestionKind(" DRAWING ")
		require.NoError(t, err)
		require.Equal(t, IntelCheckKindDrawing, got)
	})

	t.Run("无法识别的题型必须报错而非回落", func(t *testing.T) {
		// 题型决定走哪条判定链路，回落会让绘图题被当成逻辑题比字符串，
		// 结果恒为 fail 且页面上看不出配置问题。
		for _, in := range []string{"", "   ", "image", "draw"} {
			_, err := normalizeIntelCheckQuestionKind(in)
			require.Error(t, err, "输入[%s]", in)
		}
	})
}

func TestNormalizeIntelCheckMatchMode(t *testing.T) {
	t.Run("留空取 exact", func(t *testing.T) {
		// 与迁移 238 里该列的 DEFAULT 'exact' 保持一致。
		for _, in := range []string{"", "   "} {
			got, err := normalizeIntelCheckMatchMode(in)
			require.NoError(t, err)
			require.Equal(t, IntelCheckMatchExact, got)
		}
	})

	t.Run("四种合法值", func(t *testing.T) {
		for _, want := range []string{
			IntelCheckMatchExact, IntelCheckMatchNumeric,
			IntelCheckMatchContains, IntelCheckMatchRegex,
		} {
			got, err := normalizeIntelCheckMatchMode(strings.ToUpper(want))
			require.NoError(t, err)
			require.Equal(t, want, got)
		}
	})

	t.Run("非法值报错而非回落 exact", func(t *testing.T) {
		// 静默回落会把 contains 的拼写错误变成严格全等，
		// 判定从「通过」翻成「失败」，而配置看上去毫无问题。
		_, err := normalizeIntelCheckMatchMode("contain")
		require.Error(t, err)
	})
}

func TestBuildIntelCheckQuestion_公共字段校验(t *testing.T) {
	base := IntelCheckQuestionParams{
		Kind:           IntelCheckKindLogic,
		Title:          "三段论",
		Prompt:         "所有 A 都是 B……",
		ExpectedAnswer: "42",
	}

	t.Run("标题与题面去首尾空白", func(t *testing.T) {
		p := base
		p.Title = "  三段论  "
		p.Prompt = "\n题面\n"
		q := &IntelCheckQuestion{}
		require.NoError(t, buildIntelCheckQuestion(q, p))
		require.Equal(t, "三段论", q.Title)
		require.Equal(t, "题面", q.Prompt)
	})

	cases := []struct {
		name  string
		mutit func(*IntelCheckQuestionParams)
	}{
		{"标题为空", func(p *IntelCheckQuestionParams) { p.Title = "   " }},
		{"题面为空", func(p *IntelCheckQuestionParams) { p.Prompt = "  " }},
		{"标题超长", func(p *IntelCheckQuestionParams) {
			p.Title = strings.Repeat("标", maxIntelCheckQuestionTitleRunes+1)
		}},
		{"题面超长", func(p *IntelCheckQuestionParams) {
			p.Prompt = strings.Repeat("字", maxIntelCheckQuestionPromptRunes+1)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.mutit(&p)
			require.Error(t, buildIntelCheckQuestion(&IntelCheckQuestion{}, p))
		})
	}

	t.Run("标题恰好等于上限通过", func(t *testing.T) {
		p := base
		p.Title = strings.Repeat("标", maxIntelCheckQuestionTitleRunes)
		require.NoError(t, buildIntelCheckQuestion(&IntelCheckQuestion{}, p))
	})
}

func TestBuildIntelCheckQuestion_逻辑题(t *testing.T) {
	base := IntelCheckQuestionParams{
		Kind:           IntelCheckKindLogic,
		Title:          "三段论",
		Prompt:         "题面",
		ExpectedAnswer: "42",
		MatchMode:      IntelCheckMatchNumeric,
	}

	t.Run("绘图题字段被清空", func(t *testing.T) {
		// 残留的参考稿会让管理端表单显示一堆不生效的配置，
		// 日后改回 drawing 又会悄悄套用过时的参考指标。
		q := &IntelCheckQuestion{
			ReferenceHTML:    referenceHTMLFixture,
			ReferenceMetrics: map[string]any{"shape_count": 9.0},
			DrawingRules:     map[string]any{"min_ratio": 0.9},
			ReviewRubric:     "旧清单",
		}
		require.NoError(t, buildIntelCheckQuestion(q, base))

		require.Equal(t, "42", q.ExpectedAnswer)
		require.Equal(t, IntelCheckMatchNumeric, q.MatchMode)
		require.Empty(t, q.ReferenceHTML)
		require.Nil(t, q.ReferenceMetrics)
		require.Nil(t, q.DrawingRules)
		require.Empty(t, q.ReviewRubric)
	})

	t.Run("标准答案必填", func(t *testing.T) {
		p := base
		p.ExpectedAnswer = "  "
		require.Error(t, buildIntelCheckQuestion(&IntelCheckQuestion{}, p))
	})

	t.Run("标准答案超长", func(t *testing.T) {
		p := base
		p.ExpectedAnswer = strings.Repeat("答", maxIntelCheckQuestionAnswerRunes+1)
		require.Error(t, buildIntelCheckQuestion(&IntelCheckQuestion{}, p))
	})

	t.Run("非法正则在写入时就被拦下", func(t *testing.T) {
		// 留到判定时才发现，那一轮所有分组都会记成 fail，
		// 而真正的原因只出现在日志里。
		p := base
		p.MatchMode = IntelCheckMatchRegex
		p.ExpectedAnswer = "([0-9]+"
		require.Error(t, buildIntelCheckQuestion(&IntelCheckQuestion{}, p))
	})

	t.Run("合法正则通过", func(t *testing.T) {
		p := base
		p.MatchMode = IntelCheckMatchRegex
		p.ExpectedAnswer = `^\d{2}$`
		require.NoError(t, buildIntelCheckQuestion(&IntelCheckQuestion{}, p))
	})
}

func TestBuildIntelCheckQuestion_绘图题(t *testing.T) {
	base := IntelCheckQuestionParams{
		Kind:          IntelCheckKindDrawing,
		Title:         "画一只会动的猫",
		Prompt:        "请输出内嵌动画 SVG 的单文件 HTML",
		ReferenceHTML: referenceHTMLFixture,
		ReviewRubric:  "看层次、看动画、看细节",
	}

	t.Run("服务端算出参考指标并清空逻辑题字段", func(t *testing.T) {
		q := &IntelCheckQuestion{
			ExpectedAnswer: "旧答案",
			MatchMode:      IntelCheckMatchRegex,
		}
		require.NoError(t, buildIntelCheckQuestion(q, base))

		require.NotEmpty(t, q.ReferenceMetrics, "参考指标必须由服务端算出")
		metrics, err := DecodeDrawingMetrics(q.ReferenceMetrics)
		require.NoError(t, err)
		require.True(t, metrics.HasSVG)
		require.True(t, metrics.HasTitle)
		require.True(t, metrics.HasDesc)
		require.Positive(t, metrics.ShapeCount)

		require.Empty(t, q.ExpectedAnswer)
		// 该列 NOT NULL 且带 CHECK 约束，绘图题也必须写一个合法值。
		require.Equal(t, IntelCheckMatchExact, q.MatchMode)
	})

	t.Run("参考稿解析不出 SVG 必须报错", func(t *testing.T) {
		// 门禁在参考值为 0 时会跳过相对项，存一份坏参考稿
		// 等于悄悄关掉整层结构门禁——这比直接报错危险得多。
		p := base
		p.ReferenceHTML = "<html><body><p>这里没有 svg</p></body></html>"
		require.Error(t, buildIntelCheckQuestion(&IntelCheckQuestion{}, p))
	})

	t.Run("未上传参考稿是合法状态", func(t *testing.T) {
		// 管理员通常先建题、再补参考稿；此时门禁只跑固定项。
		p := base
		p.ReferenceHTML = "   "
		q := &IntelCheckQuestion{}
		require.NoError(t, buildIntelCheckQuestion(q, p))
		require.Empty(t, q.ReferenceHTML)
		require.Nil(t, q.ReferenceMetrics)
	})

	t.Run("参考稿超过体积上限", func(t *testing.T) {
		p := base
		p.ReferenceHTML = strings.Repeat("x", maxIntelCheckReferenceHTMLBytes+1)
		require.Error(t, buildIntelCheckQuestion(&IntelCheckQuestion{}, p))
	})

	t.Run("评审清单超长", func(t *testing.T) {
		p := base
		p.ReviewRubric = strings.Repeat("条", maxIntelCheckReviewRubricRunes+1)
		require.Error(t, buildIntelCheckQuestion(&IntelCheckQuestion{}, p))
	})

	t.Run("门禁规则落库前已归一化", func(t *testing.T) {
		// 不归一化就落库，管理端读回的规则会与判定时实际生效的规则不一致。
		p := base
		p.DrawingRules = map[string]any{"min_ratio": 5.0, "max_bytes": -1}
		q := &IntelCheckQuestion{}
		require.NoError(t, buildIntelCheckQuestion(q, p))

		rules, err := DecodeIntelCheckDrawingRules(q.DrawingRules)
		require.NoError(t, err)
		require.InDelta(t, intelCheckDefaultMinRatio, rules.MinRatio, 1e-9)
		require.Equal(t, intelCheckDefaultMaxBytes, rules.MaxBytes)
	})

	t.Run("合法的自定义门禁规则原样保留", func(t *testing.T) {
		p := base
		p.DrawingRules = map[string]any{
			"min_ratio":         0.8,
			"max_bytes":         1024,
			"required_keywords": []any{"cat", "尾巴"},
		}
		q := &IntelCheckQuestion{}
		require.NoError(t, buildIntelCheckQuestion(q, p))

		rules, err := DecodeIntelCheckDrawingRules(q.DrawingRules)
		require.NoError(t, err)
		require.InDelta(t, 0.8, rules.MinRatio, 1e-9)
		require.Equal(t, 1024, rules.MaxBytes)
		require.Equal(t, []string{"cat", "尾巴"}, rules.RequiredKeywords)
	})
}

func TestIntelCheckDrawingRules编解码(t *testing.T) {
	t.Run("空输入返回零值且不报错", func(t *testing.T) {
		// 题目尚未配置门禁规则是合法状态，由调用方 Normalize 补默认值。
		for _, raw := range []map[string]any{nil, {}} {
			rules, err := DecodeIntelCheckDrawingRules(raw)
			require.NoError(t, err)
			require.Equal(t, IntelCheckDrawingRules{}, rules)
		}
	})

	t.Run("往返不丢字段", func(t *testing.T) {
		// JSONB 存的是整块 JSON，字段名写错就是静默丢配置。
		want := IntelCheckDrawingRules{
			MinRatio:         0.75,
			RequiredKeywords: []string{"轮廓", "阴影"},
			MaxBytes:         2048,
		}
		encoded, err := EncodeIntelCheckDrawingRules(want)
		require.NoError(t, err)
		require.Contains(t, encoded, "min_ratio")
		require.Contains(t, encoded, "required_keywords")
		require.Contains(t, encoded, "max_bytes")

		got, err := DecodeIntelCheckDrawingRules(encoded)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("JSONB 读出的数字是 float64 也能正确解码", func(t *testing.T) {
		// 这正是走 JSON 往返而非逐键类型断言的原因。
		rules, err := DecodeIntelCheckDrawingRules(map[string]any{
			"min_ratio": 0.9,
			"max_bytes": float64(4096),
		})
		require.NoError(t, err)
		require.InDelta(t, 0.9, rules.MinRatio, 1e-9)
		require.Equal(t, 4096, rules.MaxBytes)
	})
}
