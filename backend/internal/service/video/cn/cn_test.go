//go:build unit

package cn

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func i2v() video.Request {
	return video.Request{
		Model: "m", Prompt: "p", DurationSeconds: 5, Resolution: video.Resolution1080P, AspectRatio: "16:9",
		Images: []video.Image{{URL: "https://img/first.png", Role: video.ImageRoleFirstFrame}},
	}
}

func TestArkBuildAndParse(t *testing.T) {
	call, err := Ark{}.BuildCreate(i2v())
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, call.Method)
	require.Equal(t, "/api/v3/contents/generations/tasks", call.Path)
	body := gjson.ParseBytes(call.Body)
	require.Equal(t, "m", body.Get("model").String())
	require.Equal(t, "first_frame", body.Get("content.1.role").String())
	require.Equal(t, "1080p", body.Get("resolution").String())

	created, err := Ark{}.ParseCreate([]byte(`{"id":"cgt-1"}`))
	require.NoError(t, err)
	require.Equal(t, "cgt-1", created.TaskID)

	snap, err := Ark{}.ParseQuery([]byte(`{"status":"succeeded","content":{"video_url":"https://v/x.mp4"},"duration":5,"resolution":"720p","usage":{"completion_tokens":108900}}`))
	require.NoError(t, err)
	require.Equal(t, video.StatusSucceeded, snap.Status)
	require.Equal(t, "https://v/x.mp4", snap.VideoURL)
	require.EqualValues(t, 108900, snap.CompletionTokens)

	require.Equal(t, video.NativeOp{Kind: video.NativeQuery, TaskID: "cgt-1"}, Ark{}.Classify(http.MethodGet, "/api/v3/contents/generations/tasks/cgt-1", nil))
	require.Equal(t, video.NativeCreate, Ark{}.Classify(http.MethodPost, "/v3/contents/generations/tasks", nil).Kind)
}

func TestArkParseNativeCreateReadsTextFlags(t *testing.T) {
	req, err := Ark{}.ParseNativeCreate("", []byte(`{"model":"seedance","content":[{"type":"text","text":"waves --duration 10 --resolution 1080p --ratio 9:16"}]}`))
	require.NoError(t, err)
	require.Equal(t, 10, req.DurationSeconds)
	require.Equal(t, video.Resolution1080P, req.Resolution)
	require.Equal(t, "9:16", req.AspectRatio)
}

func TestKlingRoutesByKindAndSignsJWT(t *testing.T) {
	call, err := Kling{}.BuildCreate(i2v())
	require.NoError(t, err)
	require.Equal(t, "/v1/videos/image2video", call.Path)
	require.Equal(t, "pro", gjson.GetBytes(call.Body, "mode").String())
	require.Equal(t, "image2video", Kling{}.CreateMeta(call.Path))
	require.Equal(t, "/v1/videos/image2video/t1", Kling{}.BuildQuery("t1", "image2video").Path)

	snap, err := Kling{}.ParseQuery([]byte(`{"code":0,"data":{"task_status":"succeed","task_result":{"videos":[{"url":"https://v/k.mp4","duration":"5.1"}]}}}`))
	require.NoError(t, err)
	require.Equal(t, video.StatusSucceeded, snap.Status)
	require.InDelta(t, 5.1, snap.DurationSeconds, 0.001)

	_, err = Kling{}.ParseCreate([]byte(`{"code":1001,"message":"bad"}`))
	require.Error(t, err)

	h := http.Header{}
	require.NoError(t, Kling{}.Authorize(h, video.Credentials{APIKey: "relay-key"}))
	require.Equal(t, "Bearer relay-key", h.Get("Authorization"), "第三方中转只填 Key 时按 Bearer 发送")

	token, err := KlingJWT("ak", "sk", time.Unix(1_800_000_000, 0))
	require.NoError(t, err)
	parsed, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return []byte("sk"), nil }, jwt.WithTimeFunc(func() time.Time { return time.Unix(1_800_000_000, 0) }))
	require.NoError(t, err)
	iss, _ := parsed.Claims.GetIssuer()
	require.Equal(t, "ak", iss)
}

func TestMiniMaxResolvesFile(t *testing.T) {
	call, err := MiniMax{}.BuildCreate(i2v())
	require.NoError(t, err)
	require.Equal(t, "1080P", gjson.GetBytes(call.Body, "resolution").String())
	snap, err := MiniMax{}.ParseQuery([]byte(`{"status":"Success","file_id":"f1","video_width":1920,"video_height":1080,"base_resp":{"status_code":0}}`))
	require.NoError(t, err)
	require.Equal(t, video.StatusSucceeded, snap.Status)
	require.Equal(t, "f1", snap.FileID)
	require.Equal(t, video.Resolution1080P, snap.Resolution)
	link, err := MiniMax{}.ParseFileQuery([]byte(`{"file":{"download_url":"https://d/x.mp4"},"base_resp":{"status_code":0}}`))
	require.NoError(t, err)
	require.Equal(t, "https://d/x.mp4", link)
	op := MiniMax{}.Classify(http.MethodGet, "/v1/files/retrieve", url.Values{"file_id": {"f1"}})
	require.Equal(t, video.NativeOp{Kind: video.NativeFile, FileID: "f1"}, op)
	_, err = MiniMax{}.ParseCreate([]byte(`{"task_id":"","base_resp":{"status_code":1002,"status_msg":"rate limit"}}`))
	require.Error(t, err)
}

func TestDashScopeWan3MediaAndUsage(t *testing.T) {
	req := i2v()
	req.Model = "wan3.0-video"
	req.DurationSeconds = 20
	call, err := DashScope{}.BuildCreate(req)
	require.NoError(t, err)
	require.Equal(t, "enable", call.Headers.Get("X-DashScope-Async"))
	body := gjson.ParseBytes(call.Body)
	require.Equal(t, "first_frame", body.Get("input.media.0.type").String())
	require.Equal(t, "1080P", body.Get("parameters.resolution").String())
	require.EqualValues(t, 20, body.Get("parameters.duration").Int())

	legacy := i2v()
	legacy.Model = "wan2.5-i2v-preview"
	call, err = DashScope{}.BuildCreate(legacy)
	require.NoError(t, err)
	require.Equal(t, "https://img/first.png", gjson.GetBytes(call.Body, "input.img_url").String())

	snap, err := DashScope{}.ParseQuery([]byte(`{"output":{"task_status":"SUCCEEDED","video_url":"https://o/v.mp4"},"usage":{"output_video_duration":20.0,"SR":720}}`))
	require.NoError(t, err)
	require.Equal(t, video.StatusSucceeded, snap.Status)
	require.InDelta(t, 20, snap.DurationSeconds, 0.001)
	require.Equal(t, video.Resolution720P, snap.Resolution)

	native, err := DashScope{}.ParseNativeCreate("", []byte(`{"model":"wan3.0-video","input":{"prompt":"x"},"parameters":{"duration":12}}`))
	require.NoError(t, err)
	require.Equal(t, video.Resolution1080P, native.Resolution, "Wan3.0 未指定分辨率默认 1080P")
	require.Equal(t, video.NativeOp{Kind: video.NativeCancel, TaskID: "t9"}, DashScope{}.Classify(http.MethodPost, "/api/v1/tasks/t9/cancel", nil))
}

func TestViduKindsAndTokenAuth(t *testing.T) {
	req := i2v()
	req.Images = append(req.Images, video.Image{URL: "https://img/last.png", Role: video.ImageRoleLastFrame})
	call, err := Vidu{}.BuildCreate(req)
	require.NoError(t, err)
	require.Equal(t, "/ent/v2/start-end2video", call.Path)
	h := http.Header{}
	require.NoError(t, Vidu{}.Authorize(h, video.Credentials{APIKey: "k"}))
	require.Equal(t, "Token k", h.Get("Authorization"))
	snap, err := Vidu{}.ParseQuery([]byte(`{"state":"success","creations":[{"url":"https://v/vidu.mp4"}]}`))
	require.NoError(t, err)
	require.Equal(t, "https://v/vidu.mp4", snap.VideoURL)
	require.Equal(t, video.NativeQuery, Vidu{}.Classify(http.MethodGet, "/ent/v2/tasks/1/creations", nil).Kind)
}
