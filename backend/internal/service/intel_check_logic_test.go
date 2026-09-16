//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractIntelCheckAnswer(t *testing.T) {
	tests := []struct {
		name     string
		reply    string
		expected string
	}{
		{
			name:     "显式作答句式优先",
			reply:    "让我想想，先算 3 乘 7。\n最终答案：21",
			expected: "21",
		},
		{
			name:     "答案是的句式",
			reply:    "综合以上推理，答案是 蓝色",
			expected: "蓝色",
		},
		{
			name:     "显式作答句式去掉Markdown和受控量词",
			reply:    "综上，答案：**21颗**",
			expected: "21",
		},
		{
			name:     "英文作答句式去掉受控单位",
			reply:    "Therefore, the final answer: 21 items.",
			expected: "21",
		},
		{
			name:     "显式作答句式去掉中文复合单位",
			reply:    "换算后，最终答案：21分钟。",
			expected: "21",
		},
		{
			name:     "同时出现多处时取最后一处",
			reply:    "初步答案：18\n复核后修正。\n最终答案：21",
			expected: "21",
		},
		{
			name: "盒装结论优先于前面的加粗反证句",
			reply: "这颗圆形糖果总能与另一种口味配对，所以 21 颗足够。\n\n" +
				"**再证明20颗不能保证成功。**\n\n" +
				"因此，所求最小数目为\n\\[\n\\boxed{21\\text{颗}}。\n\\]",
			expected: "21",
		},
		{
			name:     "多个盒装结论取最后一个",
			reply:    `初算为 \boxed{20}，复核后为 \fbox{21}`,
			expected: "21",
		},
		{
			name:     "盒装文本答案展开嵌套LaTeX样式",
			reply:    `最终选择 \boxed{\mathbf{\text{蓝色}}}`,
			expected: "蓝色",
		},
		{
			name:     "代码块里的盒装内容不参与提取",
			reply:    "```latex\n\\boxed{99}\n```\n最终答案：21",
			expected: "21",
		},
		{
			name:     "未闭合盒装内容回退到既有规则",
			reply:    `推理中写了 \boxed{20，最终结论是 **21**`,
			expected: "21",
		},
		{
			name:     "无作答句式时取最后一处加粗",
			reply:    "推理过程略。\n结论如下：**42**",
			expected: "42",
		},
		{
			name:     "加粗路径去掉行内代码和受控量词",
			reply:    "推理过程略。\n结论如下：**`21 个`**",
			expected: "21",
		},
		{
			name:     "均无时取最后一个非空行",
			reply:    "第一步推理\n第二步推理\n\n21\n\n",
			expected: "21",
		},
		{
			name:     "末行路径兼容下划线全角数字量词和标点",
			reply:    "第一步推理\n第二步推理\n\n__２１颗__。\n",
			expected: "21",
		},
		{
			name:     "错误数值只去量词不改数值",
			reply:    "答案：**22颗**",
			expected: "22",
		},
		{
			name:     "忽略代码块内容避免误取中间变量",
			reply:    "```python\nanswer = 99\n```\n最终答案：21",
			expected: "21",
		},
		{
			name:     "空回复返回空",
			reply:    "   \n  ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, ExtractIntelCheckAnswer(tt.reply))
		})
	}
}

func TestExtractIntelCheckAnswer_歧义候选不折叠(t *testing.T) {
	tests := []string{
		"21个苹果",
		"大约21颗",
		"20~21颗",
		"21颗或22颗",
		"~21颗",
		"<21颗",
		".5颗",
	}

	for _, candidate := range tests {
		t.Run(candidate, func(t *testing.T) {
			require.Equal(t, candidate, ExtractIntelCheckAnswer("答案："+candidate))
		})
	}
}

func TestMatchIntelCheckAnswer(t *testing.T) {
	tests := []struct {
		name      string
		reply     string
		extracted string
		expected  string
		mode      string
		want      bool
	}{
		{
			name:      "精确匹配命中",
			extracted: "21",
			expected:  "21",
			mode:      IntelCheckMatchExact,
			want:      true,
		},
		{
			name:      "精确匹配忽略全角差异",
			extracted: "２１",
			expected:  "21",
			mode:      IntelCheckMatchExact,
			want:      true,
		},
		{
			name:      "精确匹配忽略两端标点",
			extracted: "蓝色。",
			expected:  "蓝色",
			mode:      IntelCheckMatchExact,
			want:      true,
		},
		{
			name:      "精确匹配拒绝多余内容",
			extracted: "21个苹果",
			expected:  "21",
			mode:      IntelCheckMatchExact,
			want:      false,
		},
		{
			name:      "模式为空时按精确匹配处理",
			extracted: "21",
			expected:  "21",
			mode:      "",
			want:      true,
		},
		{
			name:      "数值匹配容忍小数写法",
			extracted: "21.0",
			expected:  "21",
			mode:      IntelCheckMatchNumeric,
			want:      true,
		},
		{
			name:      "数值匹配容忍附加文字",
			extracted: "大约 21 个",
			expected:  "21",
			mode:      IntelCheckMatchNumeric,
			want:      true,
		},
		{
			name:      "数值匹配在提取失败时回退整段回复",
			reply:     "经计算共 21 个",
			extracted: "无法确定",
			expected:  "21",
			mode:      IntelCheckMatchNumeric,
			want:      true,
		},
		{
			name:      "数值匹配拒绝错误数值",
			extracted: "22",
			expected:  "21",
			mode:      IntelCheckMatchNumeric,
			want:      false,
		},
		{
			name:     "包含匹配针对整段回复",
			reply:    "推理后可知，它应该是蓝色的。",
			expected: "蓝色",
			mode:     IntelCheckMatchContains,
			want:     true,
		},
		{
			name:     "包含匹配忽略标点与空白",
			reply:    "答案：蓝 色",
			expected: "蓝色",
			mode:     IntelCheckMatchContains,
			want:     true,
		},
		{
			name:     "包含匹配未命中",
			reply:    "它应该是红色的。",
			expected: "蓝色",
			mode:     IntelCheckMatchContains,
			want:     false,
		},
		{
			name:     "正则匹配针对整段回复",
			reply:    "最终共 21 个",
			expected: `共\s*21\s*个`,
			mode:     IntelCheckMatchRegex,
			want:     true,
		},
		{
			name:     "正则匹配未命中",
			reply:    "最终共 22 个",
			expected: `共\s*21\s*个`,
			mode:     IntelCheckMatchRegex,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MatchIntelCheckAnswer(tt.reply, tt.extracted, tt.expected, tt.mode)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestMatchIntelCheckAnswerConfigError 覆盖「题目配置有问题」的分支。
// 这类错误必须与模型答错区分开：调用方据此记 request_error，
// 否则管理员填错一个正则就会把所有分组冤判成降智。
func TestMatchIntelCheckAnswerConfigError(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		mode     string
	}{
		{"期望答案为空", "", IntelCheckMatchExact},
		{"正则非法", "[unclosed", IntelCheckMatchRegex},
		{"数值模式的期望答案不含数字", "蓝色", IntelCheckMatchNumeric},
		{"未知匹配模式", "21", "fuzzy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MatchIntelCheckAnswer("任意回复", "21", tt.expected, tt.mode)
			require.Error(t, err)
			require.False(t, got)
		})
	}
}
