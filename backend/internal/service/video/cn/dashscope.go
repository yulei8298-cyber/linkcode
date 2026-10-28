package cn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	dashScopeCreatePath = "/api/v1/services/aigc/video-generation/video-synthesis"
	dashScopeTasksPath  = "/api/v1/tasks"
)

var dashScopeStatus = map[string]video.Status{
	"pending":   video.StatusQueued,
	"running":   video.StatusRunning,
	"succeeded": video.StatusSucceeded,
	"failed":    video.StatusFailed,
	"canceled":  video.StatusCancelled,
	"cancelled": video.StatusCancelled,
	"unknown":   video.StatusFailed,
}

// DashScope 实现阿里云百炼的通义万相视频接口（含 Wan3.0 的 media 素材写法与 Wan2.x 的旧写法）。
type DashScope struct{}

func (DashScope) Info() video.Info {
	return video.Info{
		Protocol:       video.ProtocolDashScope,
		DisplayName:    "通义万相 Wan（阿里云百炼）",
		Vendor:         "wan",
		DefaultBaseURL: "https://dashscope.aliyuncs.com",
		NativePrefix:   "/video/dashscope",
		ExampleModels:  []string{"wan3.0-video", "wan3.0-video-prime", "wan2.7-video", "wan2.5-t2v-preview"},
	}
}

// usesMediaInput 报告模型是否使用 input.media 素材数组（Wan2.7 起的统一写法）。
func usesMediaInput(model string) bool {
	m := strings.ToLower(model)
	return strings.HasPrefix(m, "wan3") || strings.HasPrefix(m, "wan2.7")
}

func dashScopeResolution(resolution string) string {
	if resolution == video.Resolution4K {
		return "1080P"
	}
	return strings.ToUpper(resolution)
}

func (DashScope) BuildCreate(req video.Request) (video.Call, error) {
	input := map[string]any{}
	if req.Prompt != "" {
		input["prompt"] = req.Prompt
	}
	if req.NegativePrompt != "" {
		input["negative_prompt"] = req.NegativePrompt
	}
	if usesMediaInput(req.Model) {
		media := []map[string]any{}
		for _, img := range req.Images {
			kind := "reference_image"
			switch img.Role {
			case video.ImageRoleFirstFrame:
				kind = "first_frame"
			case video.ImageRoleLastFrame:
				kind = "last_frame"
			}
			media = append(media, map[string]any{"type": kind, "url": img.URL})
		}
		for _, v := range req.Videos {
			media = append(media, map[string]any{"type": "reference_video", "url": v})
		}
		for _, a := range req.Audios {
			media = append(media, map[string]any{"type": "reference_audio", "url": a})
		}
		if len(media) > 0 {
			input["media"] = media
		}
	} else {
		if first := req.FirstImage(video.ImageRoleFirstFrame); first != "" {
			input["img_url"] = first
		}
		if last := req.FirstImage(video.ImageRoleLastFrame); last != "" {
			input["first_frame_url"] = req.FirstImage(video.ImageRoleFirstFrame)
			input["last_frame_url"] = last
		}
		if len(req.Audios) > 0 {
			input["audio_url"] = req.Audios[0]
		}
	}
	params := map[string]any{}
	if req.Resolution != "" {
		params["resolution"] = dashScopeResolution(req.Resolution)
	}
	if req.AspectRatio != "" && usesMediaInput(req.Model) {
		params["ratio"] = req.AspectRatio
	}
	if req.DurationSeconds > 0 {
		params["duration"] = req.DurationSeconds
	}
	if req.GenerateAudio != nil {
		params["audio"] = *req.GenerateAudio
	}
	if req.Seed != nil {
		params["seed"] = *req.Seed
	}
	video.MergeObject(params, req.Extra, "parameters")
	video.MergeObject(input, req.Extra, "input")
	extra := video.WithoutKeys(req.Extra, "parameters", "input")
	body, err := video.BuildBody(map[string]any{"model": req.Model, "input": input, "parameters": params}, extra)
	if err != nil {
		return video.Call{}, err
	}
	return video.Call{
		Method:  http.MethodPost,
		Path:    dashScopeCreatePath,
		Body:    body,
		Headers: http.Header{"X-Dashscope-Async": {"enable"}},
	}, nil
}

func (DashScope) Classify(method, path string, _ url.Values) video.NativeOp {
	if method == http.MethodPost && video.HasSuffixPath(path, "video-generation", "video-synthesis") {
		return video.NativeOp{Kind: video.NativeCreate}
	}
	rest, ok := video.AfterSegments(path, "tasks")
	if !ok {
		return video.NativeOp{}
	}
	switch {
	case method == http.MethodGet && len(rest) == 1:
		return video.NativeOp{Kind: video.NativeQuery, TaskID: rest[0]}
	case method == http.MethodPost && len(rest) == 2 && rest[1] == "cancel":
		return video.NativeOp{Kind: video.NativeCancel, TaskID: rest[0]}
	}
	return video.NativeOp{}
}

func (DashScope) ParseNativeCreate(_ string, body []byte) (video.Request, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Request{}, err
	}
	params := root.Get("parameters")
	req := video.Request{
		Model:           strings.TrimSpace(root.Get("model").String()),
		Prompt:          root.Get("input.prompt").String(),
		DurationSeconds: video.ParseDurationValue(params.Get("duration")),
		Resolution:      video.NormalizeResolution(video.FirstNonEmpty(params.Get("resolution").String(), params.Get("size").String())),
		AspectRatio:     params.Get("ratio").String(),
	}
	if req.Resolution == "" {
		// Wan3.0 未指定分辨率时默认 1080P，估价按默认档。
		req.Resolution = video.Resolution1080P
	}
	if req.DurationSeconds == 0 {
		req.DurationSeconds = 5
	}
	for _, item := range root.Get("input.media").Array() {
		role := video.ImageRoleReference
		switch item.Get("type").String() {
		case "first_frame":
			role = video.ImageRoleFirstFrame
		case "last_frame":
			role = video.ImageRoleLastFrame
		case "reference_video":
			req.Videos = append(req.Videos, item.Get("url").String())
			continue
		}
		req.Images = append(req.Images, video.Image{URL: item.Get("url").String(), Role: role})
	}
	if img := root.Get("input.img_url").String(); img != "" {
		req.Images = append(req.Images, video.Image{URL: img, Role: video.ImageRoleFirstFrame})
	}
	return req, nil
}

func (DashScope) RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error) {
	out, err := sjson.SetBytes(body, "model", model)
	return out, path, err
}

func (DashScope) ParseCreate(body []byte) (video.CreateResult, error) {
	root := gjson.ParseBytes(body)
	if code := root.Get("code").String(); code != "" {
		return video.CreateResult{}, fmt.Errorf("dashscope error %s: %s", code, root.Get("message").String())
	}
	id, err := video.RequireTaskID(root.Get("output.task_id"), video.ProtocolDashScope)
	return video.CreateResult{TaskID: id}, err
}

func (DashScope) BuildQuery(taskID, _ string) video.Call {
	return video.Call{Method: http.MethodGet, Path: dashScopeTasksPath + "/" + taskID}
}

func (DashScope) ParseQuery(body []byte) (video.Snapshot, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Snapshot{}, err
	}
	output := root.Get("output")
	snap := video.Snapshot{
		Status:       video.MapStatus(output.Get("task_status").String(), dashScopeStatus),
		VideoURL:     output.Get("video_url").String(),
		ErrorCode:    output.Get("code").String(),
		ErrorMessage: output.Get("message").String(),
	}
	usage := root.Get("usage")
	snap.DurationSeconds = usage.Get("output_video_duration").Float()
	if snap.DurationSeconds == 0 {
		snap.DurationSeconds = usage.Get("duration").Float()
	}
	if sr := usage.Get("SR"); sr.Exists() {
		snap.Resolution = video.NormalizeResolution(sr.String())
	}
	if snap.Status == video.StatusSucceeded {
		snap.Progress = 100
	}
	return snap, nil
}

func (DashScope) BuildCancel(taskID, _ string) (video.Call, bool) {
	return video.Call{Method: http.MethodPost, Path: dashScopeTasksPath + "/" + taskID + "/cancel", Body: json.RawMessage("{}")}, true
}

func (DashScope) Authorize(h http.Header, cred video.Credentials) error {
	return video.SetBearer(h, cred)
}

// ForwardHeaders 透传异步开关与 OSS 临时地址解析开关。
func (DashScope) ForwardHeaders() []string {
	return []string{"X-DashScope-Async", "X-DashScope-OssResourceResolve"}
}

// ContentURLPaths 列出原生响应中的成品地址字段，透传前替换为网关签名链接。
func (DashScope) ContentURLPaths() []string {
	return []string{"output.video_url"}
}
