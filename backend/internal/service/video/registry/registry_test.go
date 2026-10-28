//go:build unit

package registry

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const gatewayLink = "https://gw.example/video-files/vt_1?expires=1&signature=s"

// 每个协议的原生成功响应样例与成品地址所在字段：透传前必须替换为网关链接。
var nativeSuccessSamples = map[video.Protocol]struct {
	body string
	path string
}{
	video.ProtocolArk:          {`{"status":"succeeded","content":{"video_url":"https://ark-cdn.example/v.mp4"}}`, "content.video_url"},
	video.ProtocolKling:        {`{"data":{"task_status":"succeed","task_result":{"videos":[{"url":"https://kling-cdn.example/v.mp4"}]}}}`, "data.task_result.videos.0.url"},
	video.ProtocolMiniMax:      {`{"file":{"download_url":"https://minimax-cdn.example/v.mp4"}}`, "file.download_url"},
	video.ProtocolDashScope:    {`{"output":{"task_status":"SUCCEEDED","video_url":"https://oss.example/v.mp4"}}`, "output.video_url"},
	video.ProtocolVidu:         {`{"state":"success","creations":[{"url":"https://vidu-cdn.example/v.mp4","watermarked_url":"https://vidu-cdn.example/w.mp4"}]}`, "creations.0.url"},
	video.ProtocolVeo:          {`{"done":true,"response":{"generateVideoResponse":{"generatedSamples":[{"video":{"uri":"https://generativelanguage.googleapis.com/f"}}]}}}`, "response.generateVideoResponse.generatedSamples.0.video.uri"},
	video.ProtocolRunway:       {`{"status":"SUCCEEDED","output":["https://runway-cdn.example/v.mp4"]}`, "output.0"},
	video.ProtocolLuma:         {`{"state":"completed","assets":{"video":"https://luma-cdn.example/v.mp4"}}`, "assets.video"},
	video.ProtocolPixVerse:     {`{"ErrCode":0,"Resp":{"status":1,"url":"https://pix-cdn.example/v.mp4"}}`, "Resp.url"},
	video.ProtocolOpenAIVideos: {`{"id":"v1","status":"completed","video_url":"https://relay.example/v.mp4"}`, "video_url"},
	video.ProtocolNewAPI:       {`{"data":{"task_id":"t1","status":"SUCCESS","url":"https://relay.example/v.mp4"}}`, "data.url"},
}

func TestEveryProtocolHidesUpstreamContentURL(t *testing.T) {
	for _, info := range Infos() {
		provider, ok := Lookup(string(info.Protocol))
		require.True(t, ok)
		sample, ok := nativeSuccessSamples[info.Protocol]
		require.True(t, ok, "协议 %s 缺少原生成品地址样例", info.Protocol)
		out := video.RewriteContentURLs(provider, []byte(sample.body), gatewayLink)
		require.Equal(t, gatewayLink, gjson.GetBytes(out, sample.path).String(), info.Protocol)
		require.NotContains(t, string(out), "cdn.example", info.Protocol)
		require.False(t, strings.Contains(string(out), "relay.example"), info.Protocol)
	}
}

func TestRewriteContentURLsLeavesOtherResponsesUntouched(t *testing.T) {
	provider, _ := Lookup(string(video.ProtocolArk))
	running := `{"status":"running"}`
	require.JSONEq(t, running, string(video.RewriteContentURLs(provider, []byte(running), gatewayLink)))
	require.Equal(t, "not json", string(video.RewriteContentURLs(provider, []byte("not json"), gatewayLink)))
}

func TestOpenAINativeContentRoute(t *testing.T) {
	provider, _ := Lookup(string(video.ProtocolOpenAIVideos))
	op := provider.Classify(http.MethodGet, "/v1/videos/video_1/content", nil)
	require.Equal(t, video.NativeOp{Kind: video.NativeContent, TaskID: "video_1"}, op)
	require.Equal(t, video.NativeQuery, provider.Classify(http.MethodGet, "/v1/videos/video_1", nil).Kind)
}

func TestEveryNativePrefixResolvesToItsProtocol(t *testing.T) {
	for _, info := range Infos() {
		require.True(t, strings.HasPrefix(info.NativePrefix, "/video/"), info.Protocol)
		provider, ok := LookupNative(strings.TrimPrefix(info.NativePrefix, "/video/"))
		require.True(t, ok, "原生前缀 %s 找不到协议", info.NativePrefix)
		require.Equal(t, info.Protocol, provider.Info().Protocol)
	}
	_, ok := LookupNative("openai_videos")
	require.False(t, ok, "原生路由只认对外前缀，不认协议 ID")
}
