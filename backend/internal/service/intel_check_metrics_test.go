//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const drawingSampleSMIL = `<!DOCTYPE html>
<html><head><title>页面标题</title></head><body>
<svg viewBox="0 0 10 10">
  <title>骑行的鹈鹕</title>
  <desc>鹈鹕骑自行车的循环动画</desc>
  <defs>
    <linearGradient id="sky"><stop offset="0"/></linearGradient>
    <symbol id="wheel"><circle r="3"/></symbol>
  </defs>
  <circle id="body" cx="5" cy="5" r="2">
    <animate attributeName="r" dur="1s" repeatCount="indefinite"/>
  </circle>
  <path id="road" d="M0 0 L10 10"/>
</svg>
</body></html>`

const drawingSampleCSS = `<html><head><style>
/* 注释应被忽略 */
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
#wheel { animation: spin 2s linear infinite; }
.cloud { animation: spin 6s linear infinite; }
#static { fill: red; }
</style></head><body>
<svg>
  <title>标题</title><desc>说明</desc>
  <circle id="wheel" r="1"/>
  <rect class="cloud"/>
  <rect class="cloud"/>
  <rect id="static"/>
</svg>
</body></html>`

const drawingSampleScript = `<html><head></head><body>
<svg>
  <title>标题</title><desc>说明</desc>
  <g id="rider"></g>
  <g id="wheel"></g>
  <circle id="sun" r="1"/>
</svg>
<script>
const ids = ["rider", "wheel", "sun"];
const nodes = Object.fromEntries(ids.map(id => [id, document.getElementById(id)]));
function tick() { requestAnimationFrame(tick); }
tick();
</script>
</body></html>`

func TestComputeDrawingMetricsSMIL(t *testing.T) {
	metrics, err := ComputeDrawingMetrics(drawingSampleSMIL)
	require.NoError(t, err)

	require.True(t, metrics.HasSVG)
	require.True(t, metrics.HasTitle)
	require.True(t, metrics.HasDesc)
	// symbol 内的 circle + 主体 circle + path；<stop> 不属于造型元素。
	require.Equal(t, 3, metrics.ShapeCount)
	// <animate> 的父节点 circle 被计为动画目标。
	require.Equal(t, 1, metrics.AnimatedTargets)
	// defs 内带 id 的 linearGradient 与 symbol。
	require.Equal(t, 2, metrics.DefsSymbols)
	require.Equal(t, len("M0 0 L10 10"), metrics.PathDataBytes)
	require.Equal(t, len(drawingSampleSMIL), metrics.HTMLBytes)
	require.True(t, metrics.HasMechanism(DrawingMechanismSMIL))
	require.False(t, metrics.HasMechanism(DrawingMechanismCSS))
}

func TestComputeDrawingMetricsCSS(t *testing.T) {
	metrics, err := ComputeDrawingMetrics(drawingSampleCSS)
	require.NoError(t, err)

	require.True(t, metrics.HasMechanism(DrawingMechanismCSS))
	require.False(t, metrics.HasMechanism(DrawingMechanismSMIL))
	// #wheel 与两个 .cloud；#static 未引用 keyframes 不计入。
	require.Equal(t, 3, metrics.AnimatedTargets)
	require.Equal(t, 4, metrics.ShapeCount)
}

// TestComputeDrawingMetricsScript 覆盖参考稿的真实写法：
// 把目标 id 收进数组后统一 getElementById。若按「扫描字面量 getElementById 调用」
// 统计，参考稿只会得到 0 个动画目标，门禁会把满血稿自己判死。
func TestComputeDrawingMetricsScript(t *testing.T) {
	metrics, err := ComputeDrawingMetrics(drawingSampleScript)
	require.NoError(t, err)

	require.True(t, metrics.HasMechanism(DrawingMechanismScript))
	require.Equal(t, 3, metrics.AnimatedTargets)
}

func TestComputeDrawingMetricsEdgeCases(t *testing.T) {
	t.Run("不含 svg 时报错", func(t *testing.T) {
		_, err := ComputeDrawingMetrics(`<html><body><p>抱歉，我无法完成该任务。</p></body></html>`)
		require.Error(t, err)
	})

	t.Run("head 中的 title 不计入画作说明", func(t *testing.T) {
		metrics, err := ComputeDrawingMetrics(`<html><head><title>页面标题</title></head><body><svg><circle r="1"/></svg></body></html>`)
		require.NoError(t, err)
		require.True(t, metrics.HasSVG)
		require.False(t, metrics.HasTitle)
		require.False(t, metrics.HasDesc)
	})

	t.Run("无动画驱动的脚本不计为动画机制", func(t *testing.T) {
		metrics, err := ComputeDrawingMetrics(`<html><body><svg><circle id="c" r="1"/></svg><script>const c = "c"; console.log(c);</script></body></html>`)
		require.NoError(t, err)
		require.False(t, metrics.HasMechanism(DrawingMechanismScript))
		require.Equal(t, 0, metrics.AnimatedTargets)
	})

	t.Run("defs 外的带 id 分组不计入可复用符号", func(t *testing.T) {
		metrics, err := ComputeDrawingMetrics(`<html><body><svg><g id="scene"><circle r="1"/></g></svg></body></html>`)
		require.NoError(t, err)
		require.Equal(t, 0, metrics.DefsSymbols)
	})
}

// TestDrawingMetricsCodec 参考稿指标要存进 JSONB 列再读回来判定，
// 这条往返链路一旦有损，门禁就会拿着错误的基准值判所有分组。
func TestDrawingMetricsCodec(t *testing.T) {
	t.Run("往返不丢字段", func(t *testing.T) {
		original := DrawingMetrics{
			ShapeCount:      116,
			AnimatedTargets: 23,
			DefsSymbols:     7,
			PathDataBytes:   3100,
			HTMLBytes:       19627,
			Mechanisms:      []string{DrawingMechanismScript, DrawingMechanismSMIL},
			HasSVG:          true,
			HasTitle:        true,
			HasDesc:         true,
		}

		encoded, err := EncodeDrawingMetrics(original)
		require.NoError(t, err)

		decoded, err := DecodeDrawingMetrics(encoded)
		require.NoError(t, err)
		require.Equal(t, original, decoded)
	})

	t.Run("map 的键名必须等于 json tag", func(t *testing.T) {
		encoded, err := EncodeDrawingMetrics(DrawingMetrics{ShapeCount: 1, HasSVG: true})
		require.NoError(t, err)

		// 键名写死在这里：改了 DrawingMetrics 的 json tag 而没做数据迁移，
		// 线上已存的参考稿指标会静默读成零值，这条断言把它挡在 CI。
		for _, key := range []string{
			"shape_count", "animated_targets", "defs_symbols", "path_data_bytes",
			"html_bytes", "mechanisms", "has_svg", "has_title", "has_desc",
		} {
			require.Containsf(t, encoded, key, "缺少键 %q", key)
		}
	})

	t.Run("参考稿未上传时解码为零值且不报错", func(t *testing.T) {
		// 绘图题刚建好、还没传参考稿是正常状态，此时门禁跳过全部相对指标项。
		// 若这里返回错误，新建题目会把所有分组直接判失败。
		metrics, err := DecodeDrawingMetrics(nil)
		require.NoError(t, err)
		require.Equal(t, DrawingMetrics{}, metrics)

		metrics, err = DecodeDrawingMetrics(map[string]any{})
		require.NoError(t, err)
		require.Equal(t, DrawingMetrics{}, metrics)
	})

	t.Run("脏数据解码报错而非静默取零值", func(t *testing.T) {
		// 指标是判定基准，类型错乱时必须显式失败；
		// 静默取零值会让门禁以为「参考稿为空」从而放行所有产物。
		_, err := DecodeDrawingMetrics(map[string]any{"shape_count": "一百一十六"})
		require.Error(t, err)
	})
}
