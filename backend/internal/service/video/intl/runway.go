package intl

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// runwayAPIVersion 是 Runway 要求每个请求携带的 API 版本。
const runwayAPIVersion = "2024-11-06"

var runwayStatus = map[string]video.Status{
	"pending":   video.StatusQueued,
	"throttled": video.StatusQueued,
	"running":   video.StatusRunning,
	"succeeded": video.StatusSucceeded,
	"failed":    video.StatusFailed,
	"cancelled": video.StatusCancelled,
}

var runwayCreateKinds = map[string]bool{"image_to_video": true, "text_to_video": true, "video_to_video": true}

// Runway 实现 Runway API 的视频任务协议。
type Runway struct{}

func (Runway) Info() video.Info {
	return video.Info{
		Protocol:       video.ProtocolRunway,
		DisplayName:    "Runway",
		Vendor:         "runway",
		DefaultBaseURL: "https://api.dev.runwayml.com",
		NativePrefix:   "/video/runway",
		ExampleModels:  []string{"gen4.5", "gen4_turbo", "veo3.1"},
	}
}

// runwayRatio 由档位与画幅生成 Runway 的像素比例写法（如 1280:720）。
func runwayRatio(resolution, aspect string) string {
	if resolution == "" && aspect == "" {
		return ""
	}
	return strings.Replace(video.SizeFor(video.FirstNonEmpty(resolution, video.Resolution720P), aspect), "x", ":", 1)
}

func (Runway) BuildCreate(req video.Request) (video.Call, error) {
	kind := "text_to_video"
	fields := map[string]any{
		"model":      req.Model,
		"promptText": req.Prompt,
		"ratio":      runwayRatio(req.Resolution, req.AspectRatio),
		"duration":   req.DurationSeconds,
		"seed":       req.Seed,
	}
	if first := req.FirstImage(video.ImageRoleFirstFrame); first != "" {
		kind = "image_to_video"
		frames := []map[string]any{{"uri": first, "position": "first"}}
		if last := req.FirstImage(video.ImageRoleLastFrame); last != "" {
			frames = append(frames, map[string]any{"uri": last, "position": "last"})
		}
		fields["promptImage"] = frames
	}
	if len(req.Videos) > 0 {
		kind = "video_to_video"
		fields["videoUri"] = req.Videos[0]
	}
	body, err := video.BuildBody(fields, req.Extra)
	if err != nil {
		return video.Call{}, err
	}
	return video.Call{Method: http.MethodPost, Path: "/v1/" + kind, Body: body}, nil
}

func (Runway) Classify(method, path string, _ url.Values) video.NativeOp {
	segs := video.PathSegments(path)
	if len(segs) == 0 {
		return video.NativeOp{}
	}
	if method == http.MethodPost && runwayCreateKinds[segs[len(segs)-1]] {
		return video.NativeOp{Kind: video.NativeCreate}
	}
	rest, ok := video.AfterSegments(path, "tasks")
	if !ok || len(rest) != 1 {
		return video.NativeOp{}
	}
	switch method {
	case http.MethodGet:
		return video.NativeOp{Kind: video.NativeQuery, TaskID: rest[0]}
	case http.MethodDelete:
		return video.NativeOp{Kind: video.NativeCancel, TaskID: rest[0]}
	}
	return video.NativeOp{}
}

func (Runway) ParseNativeCreate(_ string, body []byte) (video.Request, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Request{}, err
	}
	req := video.Request{
		Model:           strings.TrimSpace(root.Get("model").String()),
		Prompt:          root.Get("promptText").String(),
		DurationSeconds: video.ParseDurationValue(root.Get("duration")),
		Resolution:      video.NormalizeResolution(root.Get("ratio").String()),
	}
	if req.DurationSeconds == 0 {
		req.DurationSeconds = 5
	}
	if req.Resolution == "" {
		req.Resolution = video.Resolution720P
	}
	return req, nil
}

func (Runway) RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error) {
	out, err := sjson.SetBytes(body, "model", model)
	return out, path, err
}

func (Runway) ParseCreate(body []byte) (video.CreateResult, error) {
	id, err := video.RequireTaskID(gjson.GetBytes(body, "id"), video.ProtocolRunway)
	return video.CreateResult{TaskID: id}, err
}

func (Runway) BuildQuery(taskID, _ string) video.Call {
	return video.Call{Method: http.MethodGet, Path: "/v1/tasks/" + taskID}
}

func (Runway) ParseQuery(body []byte) (video.Snapshot, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Snapshot{}, err
	}
	snap := video.Snapshot{
		Status:       video.MapStatus(root.Get("status").String(), runwayStatus),
		Progress:     int(root.Get("progress").Float() * 100),
		VideoURL:     root.Get("output.0").String(),
		ErrorCode:    root.Get("failureCode").String(),
		ErrorMessage: root.Get("failure").String(),
	}
	if snap.Status == video.StatusSucceeded {
		snap.Progress = 100
	}
	return snap, nil
}

func (Runway) BuildCancel(taskID, _ string) (video.Call, bool) {
	return video.Call{Method: http.MethodDelete, Path: "/v1/tasks/" + taskID}, true
}

func (Runway) Authorize(h http.Header, cred video.Credentials) error {
	if h.Get("X-Runway-Version") == "" {
		h.Set("X-Runway-Version", runwayAPIVersion)
	}
	return video.SetBearer(h, cred)
}

// ForwardHeaders 允许客户端指定 Runway API 版本。
func (Runway) ForwardHeaders() []string { return []string{"X-Runway-Version"} }

// ContentURLPaths 列出原生响应中的成品地址字段，透传前替换为网关签名链接。
func (Runway) ContentURLPaths() []string {
	return []string{"output.0"}
}
