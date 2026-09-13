package admin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
)

// 本文件验的是管理端响应结构的**边界**，而不是业务逻辑（那些在 service 层已覆盖）。
//
// handler 这一层唯一无可替代的职责是决定「什么东西离开进程」。service 交上来的
// IntelCheckTarget 带着解密后的明文 API Key，IntelCheckResult 带着几百 KB 的绘图
// 产物——这两样都不该原样上线。下面的断言全部围绕这一点：不是检查字段值算得对不对，
// 而是检查某些字段压根不存在于序列化结果里。
//
// 断言方式刻意用「反序列化成 map 后查 key」而非字符串包含：`api_key` 是
// `api_key_masked` 的前缀，用 strings.Contains 判断会永远通过，等于没测。

// intelCheckJSONMap 把响应对象序列化再解回 map，用于逐个 key 断言。
func intelCheckJSONMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func TestIntelCheckTargetToResponse_响应里不存在承载明文凭据的字段(t *testing.T) {
	fields := intelCheckJSONMap(t, intelCheckTargetToResponse(&service.IntelCheckTarget{
		ID: 1, Name: "满血组", APIKey: "sk-live-SECRET-abcdef",
	}))

	// 有掩码、有解密失败标记，但没有任何字段叫 api_key。
	// 这条断言的价值在于：日后若有人为了「方便前端回填」加回 APIKey 字段，
	// 这里会立刻失败——而那个改动本身看起来完全无害。
	_, hasPlain := fields["api_key"]
	require.False(t, hasPlain, "受检分组的管理端响应不得包含 api_key 字段")
	require.Equal(t, "sk-l***", fields["api_key_masked"])
	require.Contains(t, fields, "api_key_decrypt_failed")
}

func TestIntelCheckTargetToResponse_完整凭据不出现在序列化结果任何位置(t *testing.T) {
	const plain = "sk-live-SECRET-abcdef"

	raw, err := json.Marshal(intelCheckTargetToResponse(&service.IntelCheckTarget{
		ID: 1, Name: "满血组", BaseURL: "https://upstream.example.com/v1", APIKey: plain,
	}))
	require.NoError(t, err)

	// 不只查 api_key 那一格：description、name 等字段也可能被人无意中拼进凭据。
	require.NotContains(t, string(raw), plain)
}

func TestIntelCheckTargetToResponse_短凭据不泄露任何字符(t *testing.T) {
	fields := intelCheckJSONMap(t, intelCheckTargetToResponse(&service.IntelCheckTarget{
		ID: 1, APIKey: "sk",
	}))

	// 长度不足时整串换成 ***，而不是把这两个字符原样露出来：
	// 短 key 往往是测试或占位凭据，但露出前缀同样是在泄露。
	require.Equal(t, "***", fields["api_key_masked"])
}

func TestIntelCheckTargetToResponse_空指针返回空(t *testing.T) {
	require.Nil(t, intelCheckTargetToResponse(nil))
}

func TestIntelCheckResultToListItem_只给大字段的字节数而不带正文(t *testing.T) {
	fields := intelCheckJSONMap(t, intelCheckResultToListItem(&service.IntelCheckResult{
		ID: 7, RoundID: 3, TargetID: 1, Kind: service.IntelCheckKindDrawing,
		Status:     service.IntelCheckStatusFail,
		RawReply:   "12345",
		HTMLOutput: "<svg></svg>",
	}))

	// 绘图产物动辄几百 KB，列表按 20 条一页也会传好几 MB。
	// 列表只需回答「哪条失败了」，正文留给详情接口。
	require.NotContains(t, fields, "raw_reply")
	require.NotContains(t, fields, "html_output")
	require.EqualValues(t, 5, fields["raw_reply_bytes"])
	require.EqualValues(t, 11, fields["html_output_bytes"])
}

func TestIntelCheckResultToListItem_保留错误原文供排障(t *testing.T) {
	fields := intelCheckJSONMap(t, intelCheckResultToListItem(&service.IntelCheckResult{
		ID: 7, Status: service.IntelCheckStatusRequestError,
		ErrorMessage: "上游请求失败：上游返回 429",
	}))

	// 与公开详情相反：公开侧把 error_message 换成中性的 status_note，
	// 管理端必须照实给出，否则排障时只能看到一句「本次请求未能完成」。
	require.Equal(t, "上游请求失败：上游返回 429", fields["error_message"])
}

func TestIntelCheckRoundToResponse_未结束的轮次给null而非零时间(t *testing.T) {
	fields := intelCheckJSONMap(t, intelCheckRoundToResponse(&service.IntelCheckRound{
		ID: 3, Seq: 1284, StartedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}))

	// 正在跑的一轮 finished_at 必须是 null。若回落成零值时间，
	// 管理端会显示「0001-01-01」，看起来像数据损坏而不是「还没跑完」。
	require.Nil(t, fields["finished_at"])
	require.Equal(t, "2026-01-02T03:04:05Z", fields["started_at"])
	require.EqualValues(t, 1284, fields["seq"])
}

func TestIntelCheckQuestionToListItem_不带参考稿正文(t *testing.T) {
	fields := intelCheckJSONMap(t, intelCheckQuestionToListItem(&service.IntelCheckQuestion{
		ID: 22, Kind: service.IntelCheckKindDrawing, Title: "鹈鹕骑车",
		ReferenceHTML: "<svg></svg>",
		ReviewRubric:  "1. 主体完整度",
		DrawingRules:  map[string]any{"min_ratio": 0.7},
	}))

	// 参考稿上限 512 KiB，清单上限 20000 字，题库列表带上正文就是几 MB 一页。
	require.NotContains(t, fields, "reference_html")
	require.NotContains(t, fields, "review_rubric")
	require.EqualValues(t, 11, fields["reference_html_bytes"])
	require.Equal(t, true, fields["has_review_rubric"])
	// 门禁规则要带上：管理员在列表页就要能看出这道题的阈值配没配。
	require.NotNil(t, fields["drawing_rules"])
}

func TestIntelCheckQuestionToListItem_空评审清单标记为false(t *testing.T) {
	fields := intelCheckJSONMap(t, intelCheckQuestionToListItem(&service.IntelCheckQuestion{
		ID: 11, Kind: service.IntelCheckKindLogic, ReviewRubric: "   ",
	}))

	// 只有空白字符等同于没配：否则列表会显示「已配置清单」，
	// 而实际判定时会回落到内置默认清单。
	require.Equal(t, false, fields["has_review_rubric"])
}

func TestParseOptionalInt64Query_非法值一律按不过滤处理(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want *int64
	}{
		{"空串不过滤", "", nil},
		{"纯空白不过滤", "   ", nil},
		{"非数字不过滤", "abc", nil},
		{"零不过滤", "0", nil},
		{"负数不过滤", "-3", nil},
		{"正常值", "42", func() *int64 { v := int64(42); return &v }()},
		{"两侧空白被去掉", " 42 ", func() *int64 { v := int64(42); return &v }()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseOptionalInt64Query(tc.raw)
			if tc.want == nil {
				// 回 nil 而不是 0：过滤条件用的是 *int64，
				// 传 0 会变成「筛选 id=0 的记录」，结果恒为空集。
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, *tc.want, *got)
		})
	}
}
