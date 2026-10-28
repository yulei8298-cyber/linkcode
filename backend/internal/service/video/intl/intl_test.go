//go:build unit

package intl

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIVideosRelayCompat(t *testing.T) {
	call, err := OpenAIVideos{}.BuildCreate(video.Request{Model: "sora-2", Prompt: "p", DurationSeconds: 8, Resolution: video.Resolution720P, AspectRatio: "9:16"})
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", call.Path)
	body := gjson.ParseBytes(call.Body)
	require.Equal(t, "8", body.Get("seconds").String())
	require.Equal(t, "720p", body.Get("resolution").String())
	require.Equal(t, "9:16", body.Get("aspect_ratio").String())
	require.False(t, body.Get("size").Exists(), "未指定像素尺寸时不发送 size")

	// 带首帧图：画幅跟随原图，不发送 aspect_ratio。
	call, err = OpenAIVideos{}.BuildCreate(video.Request{Model: "kling-v3", Prompt: "p", Resolution: video.Resolution1080P, AspectRatio: "16:9",
		Images: []video.Image{{URL: "https://img.example/a.png", Role: video.ImageRoleFirstFrame}}})
	require.NoError(t, err)
	body = gjson.ParseBytes(call.Body)
	require.Equal(t, "https://img.example/a.png", body.Get("input_reference").String())
	require.Equal(t, "1080p", body.Get("resolution").String())
	require.False(t, body.Get("aspect_ratio").Exists())

	// 客户端显式给了像素尺寸：原样转发。
	call, err = OpenAIVideos{}.BuildCreate(video.Request{Model: "m", Prompt: "p", Size: "1280x720", Resolution: video.Resolution720P})
	require.NoError(t, err)
	require.Equal(t, "1280x720", gjson.GetBytes(call.Body, "size").String())
	require.False(t, gjson.GetBytes(call.Body, "resolution").Exists())

	snap, err := OpenAIVideos{}.ParseQuery([]byte(`{"id":"video_1","status":"completed","seconds":"8"}`))
	require.NoError(t, err)
	require.Equal(t, video.StatusSucceeded, snap.Status)
	require.Equal(t, "/v1/videos/video_1/content", snap.ContentPath)

	// 第三方中转常直接返回 video_url，并可能包一层 data。
	snap, err = OpenAIVideos{}.ParseQuery([]byte(`{"data":{"id":"x","status":"SUCCESS","video_url":"https://cdn/x.mp4"}}`))
	require.NoError(t, err)
	require.Equal(t, "https://cdn/x.mp4", snap.VideoURL)
	require.Empty(t, snap.ContentPath)

	snap, err = OpenAIVideos{}.ParseQuery([]byte(`{"id":"x","status":"failed","error":{"code":"moderation","message":"blocked"}}`))
	require.NoError(t, err)
	require.Equal(t, video.StatusFailed, snap.Status)
	require.Equal(t, "blocked", snap.ErrorMessage)
}

func TestNewAPIMapsMetadata(t *testing.T) {
	call, err := NewAPI{}.BuildCreate(video.Request{Model: "m", Prompt: "p", Resolution: video.Resolution1080P, Size: "1920x1080",
		Images: []video.Image{{URL: "u", Role: video.ImageRoleFirstFrame}}})
	require.NoError(t, err)
	body := gjson.ParseBytes(call.Body)
	require.Equal(t, "u", body.Get("image").String())
	require.EqualValues(t, 1920, body.Get("width").Int())
	require.Equal(t, "1080p", body.Get("metadata.resolution").String())
	created, err := NewAPI{}.ParseCreate([]byte(`{"task_id":"t1","status":"queued"}`))
	require.NoError(t, err)
	require.Equal(t, "t1", created.TaskID)
	snap, err := NewAPI{}.ParseQuery([]byte(`{"task_id":"t1","status":"in_progress","progress":40}`))
	require.NoError(t, err)
	require.Equal(t, video.StatusRunning, snap.Status)
	require.Equal(t, 40, snap.Progress)
}

func TestVeoOperationsAndAuth(t *testing.T) {
	call, err := Veo{}.BuildCreate(video.Request{Model: "veo-3.1-generate-preview", Prompt: "p", DurationSeconds: 8, Resolution: video.Resolution1080P,
		Images: []video.Image{{URL: "data:image/png;base64,QUJD", Role: video.ImageRoleFirstFrame}}})
	require.NoError(t, err)
	require.Equal(t, "/v1beta/models/veo-3.1-generate-preview:predictLongRunning", call.Path)
	require.Equal(t, "QUJD", gjson.GetBytes(call.Body, "instances.0.image.bytesBase64Encoded").String())

	_, err = Veo{}.BuildCreate(video.Request{Model: "veo", Images: []video.Image{{URL: "https://x/a.png", Role: video.ImageRoleFirstFrame}}})
	require.ErrorIs(t, err, video.ErrInvalidRequest)

	created, err := Veo{}.ParseCreate([]byte(`{"name":"models/veo-3.1/operations/op1"}`))
	require.NoError(t, err)
	require.Equal(t, "/v1beta/models/veo-3.1/operations/op1", Veo{}.BuildQuery(created.TaskID, "").Path)

	snap, err := Veo{}.ParseQuery([]byte(`{"done":true,"response":{"generateVideoResponse":{"generatedSamples":[{"video":{"uri":"https://generativelanguage.googleapis.com/v1beta/files/f:download?alt=media"}}]}}}`))
	require.NoError(t, err)
	require.Equal(t, video.StatusSucceeded, snap.Status)
	require.True(t, snap.ContentAuth)

	snap, err = Veo{}.ParseQuery([]byte(`{"done":true,"response":{"generateVideoResponse":{"raiMediaFilteredReasons":["unsafe"]}}}`))
	require.NoError(t, err)
	require.Equal(t, video.StatusFailed, snap.Status)

	op := Veo{}.Classify(http.MethodGet, "/v1beta/models/veo-3.1/operations/op1", nil)
	require.Equal(t, "models/veo-3.1/operations/op1", op.TaskID)
	_, path, err := Veo{}.RewriteNativeModel("/v1beta/models/alias:predictLongRunning", nil, "veo-3.1-generate-preview")
	require.NoError(t, err)
	require.Equal(t, "/v1beta/models/veo-3.1-generate-preview:predictLongRunning", path)

	official, relay := http.Header{}, http.Header{}
	require.NoError(t, Veo{}.Authorize(official, video.Credentials{BaseURL: "https://generativelanguage.googleapis.com", APIKey: "k"}))
	require.NoError(t, Veo{}.Authorize(relay, video.Credentials{BaseURL: "https://relay.example.com", APIKey: "k"}))
	require.Empty(t, official.Get("Authorization"))
	require.Equal(t, "Bearer k", relay.Get("Authorization"))
}

func TestRunwayLumaPixVerse(t *testing.T) {
	call, err := Runway{}.BuildCreate(video.Request{Model: "gen4_turbo", Prompt: "p", Resolution: video.Resolution720P, AspectRatio: "16:9",
		Images: []video.Image{{URL: "u", Role: video.ImageRoleFirstFrame}}})
	require.NoError(t, err)
	require.Equal(t, "/v1/image_to_video", call.Path)
	require.Equal(t, "1280:720", gjson.GetBytes(call.Body, "ratio").String())
	h := http.Header{}
	require.NoError(t, Runway{}.Authorize(h, video.Credentials{APIKey: "k"}))
	require.Equal(t, runwayAPIVersion, h.Get("X-Runway-Version"))
	snap, err := Runway{}.ParseQuery([]byte(`{"status":"RUNNING","progress":0.5}`))
	require.NoError(t, err)
	require.Equal(t, 50, snap.Progress)

	call, err = Luma{}.BuildCreate(video.Request{Model: "ray-2", Prompt: "p", DurationSeconds: 5})
	require.NoError(t, err)
	require.Equal(t, "5s", gjson.GetBytes(call.Body, "duration").String())
	snap, err = Luma{}.ParseQuery([]byte(`{"state":"completed","assets":{"video":"https://l/v.mp4"},"request":{"duration":"9s","resolution":"1080p"}}`))
	require.NoError(t, err)
	require.Equal(t, video.StatusSucceeded, snap.Status)
	require.InDelta(t, 9, snap.DurationSeconds, 0.001)

	_, err = PixVerse{}.BuildCreate(video.Request{Model: "v5", Images: []video.Image{{URL: "u"}}})
	require.ErrorIs(t, err, video.ErrInvalidRequest, "PixVerse 图生视频必须先上传取得 img_id")
	snap, err = PixVerse{}.ParseQuery([]byte(`{"ErrCode":0,"Resp":{"status":1,"url":"https://p/v.mp4","outputWidth":1280,"outputHeight":720}}`))
	require.NoError(t, err)
	require.Equal(t, video.StatusSucceeded, snap.Status)
	require.Equal(t, video.Resolution720P, snap.Resolution)
	ph := http.Header{}
	require.NoError(t, PixVerse{}.Authorize(ph, video.Credentials{APIKey: "k"}))
	require.Equal(t, "k", ph.Get("API-KEY"))
	require.NotEmpty(t, ph.Get("Ai-trace-id"))
}
