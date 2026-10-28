// Package intl 实现海外与通用视频协议：OpenAI /v1/videos、new-api、Veo、Runway、Luma、PixVerse。
package intl

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// 通用任务状态写法：OpenAI、new-api 与常见中转的取值合集。
var genericStatus = map[string]video.Status{
	"queued":      video.StatusQueued,
	"pending":     video.StatusQueued,
	"submitted":   video.StatusQueued,
	"not_start":   video.StatusQueued,
	"in_progress": video.StatusRunning,
	"processing":  video.StatusRunning,
	"running":     video.StatusRunning,
	"completed":   video.StatusSucceeded,
	"succeeded":   video.StatusSucceeded,
	"success":     video.StatusSucceeded,
	"failed":      video.StatusFailed,
	"failure":     video.StatusFailed,
	"error":       video.StatusFailed,
	"cancelled":   video.StatusCancelled,
	"canceled":    video.StatusCancelled,
}

// OpenAIVideos 实现 OpenAI /v1/videos 协议。第三方中转多数沿用此协议，因此除官方字段外
// 也识别查询响应里直接给出的 video_url / url。
type OpenAIVideos struct{}

func (OpenAIVideos) Info() video.Info {
	return video.Info{
		Protocol:       video.ProtocolOpenAIVideos,
		DisplayName:    "OpenAI 兼容 /v1/videos",
		Vendor:         "openai",
		DefaultBaseURL: "https://api.openai.com/v1",
		NativePrefix:   "/video/openai",
		ExampleModels:  []string{"sora-2", "sora-2-pro"},
	}
}

// BuildCreate：客户端给了像素尺寸就原样转发 size；否则发分辨率档 resolution，
// 画幅只在没有首帧图时发送（带首帧图时画幅应跟随原图，中转站会拒绝或冲突）。
func (OpenAIVideos) BuildCreate(req video.Request) (video.Call, error) {
	firstFrame := req.FirstImage(video.ImageRoleFirstFrame)
	fields := map[string]any{
		"model":           req.Model,
		"prompt":          req.Prompt,
		"seconds":         video.FormatDurationSeconds(req.DurationSeconds),
		"input_reference": firstFrame,
	}
	if req.Size != "" {
		fields["size"] = req.Size
	} else {
		fields["resolution"] = req.Resolution
		if firstFrame == "" {
			fields["aspect_ratio"] = req.AspectRatio
		}
	}
	body, err := video.BuildBody(fields, req.Extra)
	if err != nil {
		return video.Call{}, err
	}
	return video.Call{Method: http.MethodPost, Path: "/v1/videos", Body: body}, nil
}

func (OpenAIVideos) Classify(method, path string, _ url.Values) video.NativeOp {
	rest, ok := video.AfterSegments(path, "videos")
	if !ok {
		return video.NativeOp{}
	}
	switch {
	case method == http.MethodPost && len(rest) == 0:
		return video.NativeOp{Kind: video.NativeCreate}
	case method == http.MethodGet && len(rest) == 1:
		return video.NativeOp{Kind: video.NativeQuery, TaskID: rest[0]}
	case method == http.MethodGet && len(rest) == 2 && rest[1] == "content":
		return video.NativeOp{Kind: video.NativeContent, TaskID: rest[0]}
	case method == http.MethodDelete && len(rest) == 1:
		return video.NativeOp{Kind: video.NativeCancel, TaskID: rest[0]}
	}
	return video.NativeOp{}
}

func (OpenAIVideos) ParseNativeCreate(_ string, body []byte) (video.Request, error) {
	return video.ParseUnifiedJSON(body)
}

func (OpenAIVideos) RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error) {
	out, err := sjson.SetBytes(body, "model", model)
	return out, path, err
}

func (OpenAIVideos) ParseCreate(body []byte) (video.CreateResult, error) {
	root := unwrapData(body)
	id, err := video.RequireTaskID(video.FirstExisting(root, "id", "task_id"), video.ProtocolOpenAIVideos)
	return video.CreateResult{TaskID: id}, err
}

func (OpenAIVideos) BuildQuery(taskID, _ string) video.Call {
	return video.Call{Method: http.MethodGet, Path: "/v1/videos/" + taskID}
}

func (OpenAIVideos) ParseQuery(body []byte) (video.Snapshot, error) {
	if _, err := video.RequireJSONObject(body); err != nil {
		return video.Snapshot{}, err
	}
	root := unwrapData(body)
	snap := genericSnapshot(root)
	if snap.Status == video.StatusSucceeded && snap.VideoURL == "" {
		snap.ContentPath = "/v1/videos/" + strings.TrimSpace(root.Get("id").String()) + "/content"
	}
	return snap, nil
}

func (OpenAIVideos) BuildCancel(taskID, _ string) (video.Call, bool) {
	return video.Call{Method: http.MethodDelete, Path: "/v1/videos/" + taskID}, true
}

func (OpenAIVideos) Authorize(h http.Header, cred video.Credentials) error {
	return video.SetBearer(h, cred)
}

// unwrapData 兼容 {"data": {...}} 包裹的中转响应。
func unwrapData(body []byte) gjson.Result {
	root := gjson.ParseBytes(body)
	if data := root.Get("data"); data.IsObject() {
		return data
	}
	return root
}

// genericSnapshot 解析 OpenAI 与 new-api 风格的任务对象。
func genericSnapshot(root gjson.Result) video.Snapshot {
	snap := video.Snapshot{
		Status:          video.MapStatus(video.FirstNonEmpty(root.Get("status").String(), root.Get("state").String()), genericStatus),
		Progress:        int(root.Get("progress").Int()),
		VideoURL:        video.FirstNonEmpty(root.Get("video_url").String(), root.Get("url").String(), root.Get("result_url").String(), root.Get("output.video_url").String()),
		DurationSeconds: video.FirstExisting(root, "seconds", "duration", "metadata.duration").Float(),
		Resolution:      video.NormalizeResolution(video.FirstNonEmpty(root.Get("resolution").String(), root.Get("size").String())),
	}
	if errObj := root.Get("error"); errObj.IsObject() {
		snap.ErrorCode = errObj.Get("code").String()
		snap.ErrorMessage = errObj.Get("message").String()
	} else if errObj.Type == gjson.String {
		snap.ErrorMessage = errObj.String()
	}
	if snap.Status == video.StatusFailed && snap.ErrorMessage == "" {
		snap.ErrorMessage = root.Get("fail_reason").String()
	}
	if snap.Status == video.StatusSucceeded {
		snap.Progress = 100
	}
	return snap
}

// ContentURLPaths 列出原生响应中的成品地址字段，透传前替换为网关签名链接。
func (OpenAIVideos) ContentURLPaths() []string {
	return []string{"video_url", "url", "result_url", "output.video_url", "data.video_url", "data.url"}
}
