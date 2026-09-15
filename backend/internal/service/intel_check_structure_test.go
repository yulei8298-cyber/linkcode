//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func readIntelCheckStructureSample(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(intelCheckFixtureDir, "structure_v2", name))
	require.NoError(t, err, "七份标定样本必须随代码存在，不允许跳过")
	return string(source)
}

func TestIntelCheckStructure七样本标定(t *testing.T) {
	withIntelCheckHTTPClient(t, &http.Client{Transport: intelCheckRoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("结构判定不能请求模型")
		return nil, fmt.Errorf("禁止模型评审")
	})})
	standards := []string{readIntelCheckStructureSample(t, "pass_a.html"), readIntelCheckStructureSample(t, "pass_b.html"), readIntelCheckStructureSample(t, "pass_c.html")}
	question := &IntelCheckQuestion{
		Kind: IntelCheckKindDrawing, ReferenceHTML: standards[0],
		DrawingRules: map[string]any{"standard_sources": standards[1:]},
	}
	svc := &IntelCheckService{motion: intelCheckPassingMotionEvaluator()}
	for _, name := range []string{"pass_a.html", "pass_b.html", "pass_c.html", "fail_a.html", "fail_b.html", "fail_c.html", "fail_d.html"} {
		t.Run(name, func(t *testing.T) {
			source := readIntelCheckStructureSample(t, name)
			outcome := svc.judgeIntelCheckDrawing(context.Background(), &IntelCheckSettings{}, question, nil, source)
			want := IntelCheckStatusFail
			if strings.HasPrefix(name, "pass") {
				want = IntelCheckStatusPass
			}
			require.Equal(t, want, outcome.Status, "%#v", outcome.JudgeDetail)
			require.Equal(t, intelCheckDrawingJudgeVersion, outcome.JudgeDetail["judge_method"])
			require.Equal(t, 3, outcome.JudgeDetail["reference_count"])
			t.Logf("%s：%s，结构分=%v，指标=%+v", name, outcome.Status, outcome.JudgeDetail["structure_score"], outcome.JudgeDetail["candidate_structure"])
		})
	}
}

func TestIntelCheckStructure格式精度与复用等价(t *testing.T) {
	sources := []string{
		`<svg><path d="M0 0L10 10"/><path d="M0 0L10 10"/></svg>`,
		`<svg><!-- 注释不参与计分 --><path d="M 0.000,0.0 L 1e1,10.000"/><path d="M0 0 L10 10"/></svg>`,
		`<svg><defs><path id="p" d="M0 0L10 10"/><circle r="999"/></defs><use href="#p"/><use href="#p"/></svg>`,
		`<svg><defs><symbol id="s"><path d="M0 0L10 10"/></symbol></defs><use href="#s"/><use href="#s"/></svg>`,
	}
	for _, source := range sources {
		got, err := ComputeIntelCheckStructureMetrics(source)
		require.NoError(t, err)
		require.Equal(t, IntelCheckStructureMetrics{Shapes: 2, GeometryValues: 8}, got)
	}
}

func TestIntelCheckStructure引用边界(t *testing.T) {
	for _, source := range []string{
		`<svg><use href="https://example.test/p.svg"/></svg>`,
		`<svg><use href="#missing"/></svg>`,
		`<svg><defs><g id="a"><use href="#b"/></g><g id="b"><use href="#a"/></g></defs><use href="#a"/></svg>`,
	} {
		_, err := ComputeIntelCheckStructureMetrics(source)
		require.Error(t, err)
	}
	_, _, err := intelCheckStructureBaseline("", nil)
	require.Error(t, err)
	_, _, err = intelCheckStructureBaseline("<svg/>", make([]string, 8))
	require.Error(t, err)
}

func TestIntelCheckStructure标准去重且与顺序无关(t *testing.T) {
	a, b, c := `<svg><circle r="1"/></svg>`, `<svg><circle r="1"/><circle r="2"/></svg>`, `<svg><circle r="1"/><circle r="2"/><circle r="3"/></svg>`
	want, n, err := intelCheckStructureBaseline(a, []string{b, c, a})
	require.NoError(t, err)
	require.Equal(t, 3, n)
	got, n, err := intelCheckStructureBaseline(c, []string{a, b})
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.Equal(t, want, got)
	require.Equal(t, intelCheckStandardDigest(a, []string{b, c, a}), intelCheckStandardDigest(c, []string{b, a}))
	require.NotEqual(t, intelCheckStandardDigest(a, []string{b}), intelCheckStandardDigest(a, []string{c}))
}

func TestIntelCheckStructure主图排除独立控件(t *testing.T) {
	main := `<svg><circle r="1"/><path d="M0 0L10 10"/></svg>`
	want, err := ComputeIntelCheckStructureMetrics(main)
	require.NoError(t, err)
	for _, source := range []string{
		`<button><svg><path d="M0 0L1 1"/></svg></button>` + main,
		main + `<button><svg><path d="M0 0L1 1"/></svg></button>`,
	} {
		got, err := ComputeIntelCheckStructureMetrics(source)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}

func TestIntelCheckStructure标准集保存与JSON往返(t *testing.T) {
	a, b, c := `<svg><circle r="1"/></svg>`, `<svg><circle r="1"/><circle r="2"/></svg>`, `<svg><circle r="1"/><circle r="2"/><circle r="3"/></svg>`
	q := &IntelCheckQuestion{ReferenceMetrics: map[string]any{"structure_baseline": "不可相信旧缓存"}}
	p := IntelCheckQuestionParams{
		Kind: IntelCheckKindDrawing, Title: "结构标定", Prompt: "画出动画", ReferenceHTML: a,
		DrawingRules: map[string]any{"standard_sources": []any{b, c, a}, "min_ratio": 0.7},
	}
	require.NoError(t, buildIntelCheckQuestion(q, p))
	require.Equal(t, 3, q.ReferenceMetrics["standard_count"])
	require.Equal(t, IntelCheckStructureMetrics{Shapes: 2, GeometryValues: 6}, q.ReferenceMetrics["structure_baseline"])
	encoded, err := json.Marshal(q)
	require.NoError(t, err)
	var restored IntelCheckQuestion
	require.NoError(t, json.Unmarshal(encoded, &restored))
	rules, err := DecodeIntelCheckDrawingRules(restored.DrawingRules)
	require.NoError(t, err)
	require.Equal(t, []string{b, c, a}, rules.StandardSources)
	baseline, count, err := intelCheckStructureBaseline(restored.ReferenceHTML, rules.StandardSources)
	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.Equal(t, IntelCheckStructureMetrics{Shapes: 2, GeometryValues: 6}, baseline)

	for name, sources := range map[string][]string{
		"数量超限":  make([]string, 8),
		"单份超限":  {strings.Repeat("x", maxIntelCheckReferenceHTMLBytes+1)},
		"无法解析":  {"不是 SVG"},
		"空静态结构": {"<svg/>"},
	} {
		t.Run(name, func(t *testing.T) {
			p.DrawingRules = map[string]any{"standard_sources": sources}
			require.Error(t, buildIntelCheckQuestion(&IntelCheckQuestion{}, p))
		})
	}
}

func TestIntelCheckStructure综合分边界(t *testing.T) {
	baseline := IntelCheckStructureMetrics{Shapes: 100, GeometryValues: 100}
	require.Equal(t, 70.0, intelCheckStructureScore(IntelCheckStructureMetrics{Shapes: 70, GeometryValues: 70}, baseline))
	require.Equal(t, 69.0, intelCheckStructureScore(IntelCheckStructureMetrics{Shapes: 69, GeometryValues: 69}, baseline))
	require.Equal(t, 100.0, intelCheckStructureScore(IntelCheckStructureMetrics{Shapes: 1000, GeometryValues: 1000}, baseline))
	require.Equal(t, 50.0, intelCheckStructureScore(IntelCheckStructureMetrics{Shapes: 1000}, baseline))
}
