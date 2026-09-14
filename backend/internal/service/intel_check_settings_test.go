//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 本文件只覆盖 intel_check_settings.go 里的两个函数：
// NormalizeIntelCheckReasoningEffort 与 ValidateIntelCheckSettings。
// 设置结构的归一化、默认值与 JSON 往返属于 intel_check_types.go，
// 相应用例在 intel_check_types_test.go——两边都写会让改一次区间要改两处，
// 漏一处就是一边红一边绿。

func TestNormalizeIntelCheckReasoningEffort(t *testing.T) {
	cases := map[string]string{
		"low":       IntelCheckEffortLow,
		"MEDIUM":    IntelCheckEffortMedium,
		"  xhigh  ": IntelCheckEffortXHigh,
		"high":      IntelCheckEffortHigh,
		// 留空与非法值都回落 high：迁移里该列没有 CHECK 约束，
		// 合法性完全靠这个函数把关，回落到最高档也符合「检测要用满血配置」的语义。
		"":             IntelCheckEffortHigh,
		"not-an-level": IntelCheckEffortHigh,
	}
	for in, want := range cases {
		t.Run("输入["+in+"]", func(t *testing.T) {
			require.Equal(t, want, NormalizeIntelCheckReasoningEffort(in))
		})
	}
}

func TestValidateIntelCheckSettings(t *testing.T) {
	valid := func() *IntelCheckSettings {
		s := DefaultIntelCheckSettings()
		s.Enabled = true
		s.DrawingJudge.TargetID = 1
		s.DrawingJudge.Model = "gpt-6-astra"
		return &s
	}

	t.Run("完整配置通过", func(t *testing.T) {
		require.NoError(t, ValidateIntelCheckSettings(valid()))
	})

	t.Run("空指针报错", func(t *testing.T) {
		require.Error(t, ValidateIntelCheckSettings(nil))
	})

	t.Run("开启不再要求评审分组", func(t *testing.T) {
		s := valid()
		s.DrawingJudge.TargetID = 0
		require.NoError(t, ValidateIntelCheckSettings(s))
	})

	t.Run("开启不再要求评审模型", func(t *testing.T) {
		s := valid()
		s.DrawingJudge.Model = "   "
		require.NoError(t, ValidateIntelCheckSettings(s))
	})

	t.Run("未开启时允许评审配置缺失", func(t *testing.T) {
		// 管理员通常先建分组、再配评审、最后开开关，中途保存不该被拦。
		s := DefaultIntelCheckSettings()
		require.NoError(t, ValidateIntelCheckSettings(&s))
	})

	t.Run("标题超长", func(t *testing.T) {
		s := valid()
		s.IntroTitle = strings.Repeat("标", IntelCheckMaxIntroTitleRunes+1)
		require.Error(t, ValidateIntelCheckSettings(s))
	})

	t.Run("说明超长", func(t *testing.T) {
		s := valid()
		s.IntroText = strings.Repeat("字", IntelCheckMaxIntroTextRunes+1)
		require.Error(t, ValidateIntelCheckSettings(s))
	})

	t.Run("评审模型名超长", func(t *testing.T) {
		s := valid()
		s.DrawingJudge.Model = strings.Repeat("m", IntelCheckMaxJudgeModelRunes+1)
		require.Error(t, ValidateIntelCheckSettings(s))
	})

	t.Run("标题恰好等于上限通过", func(t *testing.T) {
		s := valid()
		s.IntroTitle = strings.Repeat("标", IntelCheckMaxIntroTitleRunes)
		require.NoError(t, ValidateIntelCheckSettings(s))
	})
}
