//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJudgeIntelCheckLogic_带量词的加粗数值答案(t *testing.T) {
	question := &IntelCheckQuestion{
		Kind: IntelCheckKindLogic, ExpectedAnswer: "21", MatchMode: IntelCheckMatchExact,
	}

	t.Run("截图等价回复记通过", func(t *testing.T) {
		reply := "经推导，20颗无法保证。\n\n答案：**21颗**"
		outcome := judgeIntelCheckLogic(question, reply)

		require.Equal(t, IntelCheckStatusPass, outcome.Status)
		require.Equal(t, "21", outcome.ExtractedAnswer)
		require.Equal(t, "21", outcome.JudgeDetail["extracted_answer"])
		require.Equal(t, true, outcome.JudgeDetail["matched"])
	})

	t.Run("错误数值仍记失败", func(t *testing.T) {
		outcome := judgeIntelCheckLogic(question, "答案：**22颗**")

		require.Equal(t, IntelCheckStatusFail, outcome.Status)
		require.Equal(t, "22", outcome.ExtractedAnswer)
		require.Equal(t, false, outcome.JudgeDetail["matched"])
	})
}

func TestJudgeIntelCheckLogic_带单位标准答案保留单位语义(t *testing.T) {
	question := &IntelCheckQuestion{
		Kind: IntelCheckKindLogic, ExpectedAnswer: "21秒", MatchMode: IntelCheckMatchExact,
	}

	t.Run("相同单位通过", func(t *testing.T) {
		outcome := judgeIntelCheckLogic(question, "答案：**21秒**")

		require.Equal(t, IntelCheckStatusPass, outcome.Status)
		require.Equal(t, "21秒", outcome.ExtractedAnswer)
	})

	t.Run("不同单位失败", func(t *testing.T) {
		outcome := judgeIntelCheckLogic(question, "答案：**21公斤**")

		require.Equal(t, IntelCheckStatusFail, outcome.Status)
		require.Equal(t, "21公斤", outcome.ExtractedAnswer)
	})
}

func TestJudgeIntelCheckLogic_歧义数值不折叠为纯数值(t *testing.T) {
	question := &IntelCheckQuestion{
		Kind: IntelCheckKindLogic, ExpectedAnswer: "21", MatchMode: IntelCheckMatchExact,
	}
	for _, candidate := range []string{
		"21个苹果", "大约21颗", "20~21颗", "21颗或22颗", "~21颗", "<21颗", ".5颗",
	} {
		t.Run(candidate, func(t *testing.T) {
			outcome := judgeIntelCheckLogic(question, "答案："+candidate)

			require.Equal(t, IntelCheckStatusFail, outcome.Status)
			require.Equal(t, candidate, outcome.ExtractedAnswer)
		})
	}
}

func TestJudgeIntelCheckLogic_文本标识符不被当作Markdown或单位(t *testing.T) {
	for _, expected := range []string{"__init__", "value_", "21S"} {
		t.Run(expected, func(t *testing.T) {
			question := &IntelCheckQuestion{
				Kind: IntelCheckKindLogic, ExpectedAnswer: expected, MatchMode: IntelCheckMatchExact,
			}
			outcome := judgeIntelCheckLogic(question, "答案："+expected)

			require.Equal(t, IntelCheckStatusPass, outcome.Status)
			require.Equal(t, expected, outcome.ExtractedAnswer)
		})
	}
}
