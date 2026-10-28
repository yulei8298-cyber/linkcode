package intl

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const lumaGenerationsPath = "/dream-machine/v1/generations"

var lumaStatus = map[string]video.Status{
	"queued":    video.StatusQueued,
	"dreaming":  video.StatusRunning,
	"completed": video.StatusSucceeded,
	"failed":    video.StatusFailed,
}

// Luma 实现 Luma Dream Machine API。
type Luma struct{}

func (Luma) Info() video.Info {
	return video.Info{
		Protocol:       video.ProtocolLuma,
		DisplayName:    "Luma Dream Machine",
		Vendor:         "luma",
		DefaultBaseURL: "https://api.lumalabs.ai",
		NativePrefix:   "/video/luma",
		ExampleModels:  []string{"ray-2", "ray-flash-2"},
	}
}

func (Luma) BuildCreate(req video.Request) (video.Call, error) {
	fields := map[string]any{
		"model":        req.Model,
		"prompt":       req.Prompt,
		"resolution":   req.Resolution,
		"aspect_ratio": req.AspectRatio,
	}
	if req.DurationSeconds > 0 {
		fields["duration"] = video.FormatDurationSeconds(req.DurationSeconds) + "s"
	}
	keyframes := map[string]any{}
	if first := req.FirstImage(video.ImageRoleFirstFrame); first != "" {
		keyframes["frame0"] = map[string]any{"type": "image", "url": first}
	}
	if last := req.FirstImage(video.ImageRoleLastFrame); last != "" {
		keyframes["frame1"] = map[string]any{"type": "image", "url": last}
	}
	fields["keyframes"] = keyframes
	body, err := video.BuildBody(fields, req.Extra)
	if err != nil {
		return video.Call{}, err
	}
	return video.Call{Method: http.MethodPost, Path: lumaGenerationsPath, Body: body}, nil
}

func (Luma) Classify(method, path string, _ url.Values) video.NativeOp {
	rest, ok := video.AfterSegments(path, "generations")
	if !ok {
		return video.NativeOp{}
	}
	switch {
	case method == http.MethodPost && (len(rest) == 0 || (len(rest) == 1 && rest[0] == "video")):
		return video.NativeOp{Kind: video.NativeCreate}
	case method == http.MethodGet && len(rest) == 1:
		return video.NativeOp{Kind: video.NativeQuery, TaskID: rest[0]}
	case method == http.MethodDelete && len(rest) == 1:
		return video.NativeOp{Kind: video.NativeCancel, TaskID: rest[0]}
	}
	return video.NativeOp{}
}

func (Luma) ParseNativeCreate(_ string, body []byte) (video.Request, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Request{}, err
	}
	req := video.Request{
		Model:           strings.TrimSpace(root.Get("model").String()),
		Prompt:          root.Get("prompt").String(),
		DurationSeconds: video.ParseDurationValue(root.Get("duration")),
		Resolution:      video.NormalizeResolution(root.Get("resolution").String()),
		AspectRatio:     root.Get("aspect_ratio").String(),
	}
	if img := root.Get("keyframes.frame0.url").String(); img != "" {
		req.Images = append(req.Images, video.Image{URL: img, Role: video.ImageRoleFirstFrame})
	}
	if req.DurationSeconds == 0 {
		req.DurationSeconds = 5
	}
	if req.Resolution == "" {
		req.Resolution = video.Resolution720P
	}
	return req, nil
}

func (Luma) RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error) {
	out, err := sjson.SetBytes(body, "model", model)
	return out, path, err
}

func (Luma) ParseCreate(body []byte) (video.CreateResult, error) {
	id, err := video.RequireTaskID(gjson.GetBytes(body, "id"), video.ProtocolLuma)
	return video.CreateResult{TaskID: id}, err
}

func (Luma) BuildQuery(taskID, _ string) video.Call {
	return video.Call{Method: http.MethodGet, Path: lumaGenerationsPath + "/" + taskID}
}

func (Luma) ParseQuery(body []byte) (video.Snapshot, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Snapshot{}, err
	}
	snap := video.Snapshot{
		Status:          video.MapStatus(root.Get("state").String(), lumaStatus),
		VideoURL:        root.Get("assets.video").String(),
		ErrorMessage:    root.Get("failure_reason").String(),
		DurationSeconds: float64(video.ParseDurationValue(root.Get("request.duration"))),
		Resolution:      video.NormalizeResolution(root.Get("request.resolution").String()),
	}
	if snap.Status == video.StatusSucceeded {
		snap.Progress = 100
	}
	return snap, nil
}

func (Luma) BuildCancel(taskID, _ string) (video.Call, bool) {
	return video.Call{Method: http.MethodDelete, Path: lumaGenerationsPath + "/" + taskID}, true
}

func (Luma) Authorize(h http.Header, cred video.Credentials) error {
	return video.SetBearer(h, cred)
}

// ContentURLPaths 列出原生响应中的成品地址字段，透传前替换为网关签名链接。
func (Luma) ContentURLPaths() []string {
	return []string{"assets.video"}
}
