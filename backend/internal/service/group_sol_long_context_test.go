//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSolContextWindow(t *testing.T) {
	for _, tt := range []struct {
		name    string
		window  int
		want    int
		wantErr bool
	}{
		{name: "0 表示沿用目录值", window: 0, want: 0},
		{name: "下限", window: SolContextWindowMin, want: SolContextWindowMin},
		{name: "常用 272k", window: 272_000, want: 272_000},
		{name: "常用 500k", window: 500_000, want: 500_000},
		{name: "上限", window: SolContextWindowMax, want: SolContextWindowMax},
		{name: "低于下限", window: SolContextWindowMin - 1, wantErr: true},
		{name: "超过上限", window: SolContextWindowMax + 1, wantErr: true},
		{name: "负数", window: -1, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeSolContextWindow(tt.window)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidSolContextWindow)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGroupSolLongContextExempt(t *testing.T) {
	exempt := &Group{SolLongContextExempt: true}
	for _, tt := range []struct {
		name   string
		group  *Group
		result *OpenAIForwardResult
		want   bool
	}{
		{name: "无分组", result: &OpenAIForwardResult{Model: "gpt-6.1-sol"}},
		{name: "分组未开启", group: &Group{}, result: &OpenAIForwardResult{Model: "gpt-6.1-sol"}},
		{name: "空结果", group: exempt},
		{name: "分组开启", group: exempt, result: &OpenAIForwardResult{Model: "gpt-6.1-sol"}, want: true},
		{name: "推理强度后缀", group: exempt, result: &OpenAIForwardResult{Model: "gpt-6.1-sol-xhigh"}, want: true},
		{name: "客户别名映射到 Sol", group: exempt, result: &OpenAIForwardResult{Model: "custom", UpstreamModel: "gpt-6.1-sol"}, want: true},
		{name: "映射离开 Sol", group: exempt, result: &OpenAIForwardResult{Model: "gpt-6.1-sol", UpstreamModel: "gpt-6-sol"}},
		{name: "其他模型", group: exempt, result: &OpenAIForwardResult{Model: "gpt-6-astra"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, groupSolLongContextExempt(tt.group, tt.result))
		})
	}
}

const solContextManifestBody = `{"models":[` +
	`{"slug":"gpt-6.1-sol","context_window":272000,"max_context_window":872000,"display_name":"GPT-6.1 Sol"},` +
	`{"slug":"gpt-6.1-sol-high","context_window":500000,"max_context_window":600000},` +
	`{"slug":"gpt-5.6","context_window":272000,"max_context_window":1000000}` +
	`],"etag_hint":"kept"}`

func TestApplyGroupSolContextWindow(t *testing.T) {
	t.Run("未配置时原样返回", func(t *testing.T) {
		body, changed, err := applyGroupSolContextWindow([]byte(solContextManifestBody), &Group{})
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, solContextManifestBody, string(body))
	})

	t.Run("只改 Sol 条目且不超过 max_context_window", func(t *testing.T) {
		body, changed, err := applyGroupSolContextWindow([]byte(solContextManifestBody), &Group{SolContextWindow: 800_000})
		require.NoError(t, err)
		require.True(t, changed)
		models := decodeCodexManifestModels(t, body)
		require.Len(t, models, 3)
		require.EqualValues(t, 800_000, models[0]["context_window"])
		require.EqualValues(t, 872_000, models[0]["max_context_window"])
		require.Equal(t, "GPT-6.1 Sol", models[0]["display_name"])
		require.EqualValues(t, 600_000, models[1]["context_window"], "超过条目上限时取上限")
		require.EqualValues(t, 272_000, models[2]["context_window"], "非 Sol 模型不受影响")
		require.Contains(t, string(body), `"etag_hint":"kept"`)
	})

	t.Run("企业分组 500k、普通分组 272k", func(t *testing.T) {
		enterprise, _, err := applyGroupSolContextWindow([]byte(solContextManifestBody), &Group{SolContextWindow: 500_000})
		require.NoError(t, err)
		require.EqualValues(t, 500_000, decodeCodexManifestModels(t, enterprise)[0]["context_window"])

		pro, _, err := applyGroupSolContextWindow([]byte(solContextManifestBody), &Group{SolContextWindow: 272_000})
		require.NoError(t, err)
		models := decodeCodexManifestModels(t, pro)
		require.EqualValues(t, 272_000, models[0]["context_window"])
		require.EqualValues(t, 272_000, models[1]["context_window"])
	})

	t.Run("值已一致时不改动", func(t *testing.T) {
		body := `{"models":[{"slug":"gpt-6.1-sol","context_window":500000}]}`
		got, changed, err := applyGroupSolContextWindow([]byte(body), &Group{SolContextWindow: 500_000})
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, body, string(got))
	})
}

func TestMergeGroupConfiguredCodexModelsAppliesSolContextWindow(t *testing.T) {
	const groupID int64 = 81
	svc := &OpenAIGatewayService{accountRepo: codexModelsVisibilityAccountRepo{
		byGroup: map[int64][]Account{groupID: {{Platform: PlatformOpenAI, Type: AccountTypeOAuth}}},
	}}
	newManifest := func() *OpenAIModelsResponse {
		body := []byte(`{"models":[{"slug":"gpt-6.1-sol","context_window":272000,"max_context_window":872000}]}`)
		return &OpenAIModelsResponse{Body: body, ETag: codexModelsManifestBodyETag(body)}
	}

	plain := newManifest()
	require.NoError(t, svc.MergeGroupConfiguredCodexModels(context.Background(), &Group{ID: groupID, Platform: PlatformOpenAI}, plain, ""))
	require.EqualValues(t, 272_000, decodeCodexManifestModels(t, plain.Body)[0]["context_window"])

	enterprise := newManifest()
	group := &Group{ID: groupID, Platform: PlatformOpenAI, SolContextWindow: 500_000}
	require.NoError(t, svc.MergeGroupConfiguredCodexModels(context.Background(), group, enterprise, ""))
	require.EqualValues(t, 500_000, decodeCodexManifestModels(t, enterprise.Body)[0]["context_window"])
	require.NotEqual(t, plain.ETag, enterprise.ETag, "改写后的目录必须换 ETag，客户端才会重新拉取")

	// 客户端带着改写后目录的 ETag 再来，应得到 304。
	again := newManifest()
	require.NoError(t, svc.MergeGroupConfiguredCodexModels(context.Background(), group, again, enterprise.ETag))
	require.True(t, again.NotModified)
}
