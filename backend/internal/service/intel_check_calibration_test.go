//go:build unit

package service

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// 校准样例目录。样例为真实模型产出的绘图作答，用于标定结构门禁阈值。
//
// 文件与来源的对应关系（放置时请按此重命名，避免中文文件名带来的跨平台差异）：
//
//	reference.html    自行车骑鹈鹕_海风骑行.html      —— 满血参考稿
//	pass_sample.html  鹈鹕骑自行车_晴日骑行.html      —— 期望判定为通过
//	fail_sample_a.html 鹈鹕骑自行车_2D_SVG_独立版.html —— 期望判定为不通过
//	fail_sample_b.html 自行车骑鹈鹕_SVG动画.html       —— 期望判定为不通过
const intelCheckFixtureDir = "testdata/intel_check"

// readIntelCheckFixture 读取校准样例；样例缺失时跳过测试而非失败，
// 使仓库在未放置样例时仍可跑通 make test。
func readIntelCheckFixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(intelCheckFixtureDir, name))
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("校准样例缺失，跳过：%s/%s", intelCheckFixtureDir, name)
	}
	require.NoError(t, err)
	return string(data)
}

// TestIntelCheckDrawingCalibration 用四份真实样例锁定结构门禁的判定结果。
//
// 实测结论（min_ratio = 0.7，各项为「候选 / 参考稿」比值）：
//
//	健康样例   造型 84%  动画 83%  符号 100%  路径 90%  体积 89%  → 全项通过
//	劣质样例甲 造型 54%  动画 57%  符号  86%  路径 55%  体积 51%  → 4 项不达标
//	劣质样例乙 造型 40%  动画 65%  符号  43%  路径 42%  体积 51%  → 5 项不达标，且缺 title/desc
//
// 门禁为「任一项不达标即失败」，因此可用阈值区间是 (51%, 83%]：
// 下界来自劣质样例甲的最低项，上界来自健康样例的最低项。默认值 0.7 接近该
// 区间中点，向上给健康样例留 13 个百分点容错，向下给劣质样例留 19 个百分点
// 拒绝余量。不要为了「更严格」把阈值提到 0.8——那只会把健康样例的容错压到
// 3 个百分点，而劣质样例本就各挂 4、5 项，收益为零。
//
// 另注：可复用符号数是弱判别项（劣质样例甲在该项拿到 86%，高于健康样例的
// 最低项），不可对其加权；三份样例分别使用 script / smil / css 三种动画机制，
// 因此门禁只能要求「至少存在一种机制」，指定具体机制必然误杀。
func TestIntelCheckDrawingCalibration(t *testing.T) {
	referenceSource := readIntelCheckFixture(t, "reference.html")

	reference, err := ComputeDrawingMetrics(referenceSource)
	require.NoError(t, err, "参考稿必须可解析")
	require.True(t, reference.HasSVG)

	t.Logf("参考稿指标：造型=%d 动画目标=%d 符号=%d 路径字节=%d 体积=%d 机制=%v 标题=%v 说明=%v",
		reference.ShapeCount, reference.AnimatedTargets, reference.DefsSymbols,
		reference.PathDataBytes, reference.HTMLBytes, reference.Mechanisms,
		reference.HasTitle, reference.HasDesc)

	rules := IntelCheckDrawingRules{MinRatio: intelCheckDefaultMinRatio}

	samples := []struct {
		name     string
		file     string
		wantPass bool
	}{
		{name: "健康样例", file: "pass_sample.html", wantPass: true},
		{name: "劣质样例甲", file: "fail_sample_a.html", wantPass: false},
		{name: "劣质样例乙", file: "fail_sample_b.html", wantPass: false},
	}

	for _, sample := range samples {
		t.Run(sample.name, func(t *testing.T) {
			source := readIntelCheckFixture(t, sample.file)

			candidate, err := ComputeDrawingMetrics(source)
			require.NoError(t, err, "样例必须可解析")

			t.Logf("%s 指标：造型=%d 动画目标=%d 符号=%d 路径字节=%d 体积=%d 机制=%v 标题=%v 说明=%v",
				sample.name, candidate.ShapeCount, candidate.AnimatedTargets, candidate.DefsSymbols,
				candidate.PathDataBytes, candidate.HTMLBytes, candidate.Mechanisms,
				candidate.HasTitle, candidate.HasDesc)

			result := EvaluateIntelCheckGate(source, candidate, reference, rules)
			for _, item := range result.Items {
				t.Logf("  门禁 [%s] %v — %s", item.Item, item.Pass, item.Detail)
			}

			// 三份样例的判定结果全部钉死。健康样例被拦下意味着阈值过严，
			// 劣质样例被放行意味着阈值过松——两个方向都必须失败报警，
			// 不能靠「交给第二层评审兜底」掩盖第一层的阈值退化。
			require.Equalf(t, sample.wantPass, result.Pass,
				"%s 的结构门禁判定与校准基准不符（阈值 %.2f）", sample.name, rules.MinRatio)
		})
	}
}

// TestIntelCheckDrawingCalibrationSanitize 确认真实样例经过处理后仍原样保留。
// 参考稿依赖脚本驱动动画，任何改写都可能让公开页展示内容偏离模型原产物。
func TestIntelCheckDrawingCalibrationSanitize(t *testing.T) {
	source := readIntelCheckFixture(t, "reference.html")

	before, err := ComputeDrawingMetrics(source)
	require.NoError(t, err)

	sanitized, err := SanitizeIntelCheckDrawing(source, 0)
	require.NoError(t, err)

	after, err := ComputeDrawingMetrics(sanitized)
	require.NoError(t, err)

	require.NotEmptyf(t, after.Mechanisms, "处理后参考稿失去全部动画机制（处理前为 %v）", before.Mechanisms)
	require.Equalf(t, before.AnimatedTargets, after.AnimatedTargets,
		"处理改变了动画目标数：%d -> %d", before.AnimatedTargets, after.AnimatedTargets)
	require.Equal(t, source, sanitized)
}
