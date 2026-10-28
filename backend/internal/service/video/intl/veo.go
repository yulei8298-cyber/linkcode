package intl

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/tidwall/gjson"
)

// Veo 实现 Gemini API 的 Veo 长任务协议（predictLongRunning + operations）。
// 任务 ID 是完整的 operation 名（models/{model}/operations/{id}）。
type Veo struct{}

func (Veo) Info() video.Info {
	return video.Info{
		Protocol:       video.ProtocolVeo,
		DisplayName:    "Google Veo（Gemini API）",
		Vendor:         "veo",
		DefaultBaseURL: "https://generativelanguage.googleapis.com",
		NativePrefix:   "/video/veo",
		ExampleModels:  []string{"veo-3.1-generate-preview", "veo-3.1-fast-generate-preview", "veo-3.1-lite-generate-preview"},
	}
}

// inlineImage 把 data URI 转为 Gemini 的 bytesBase64Encoded 结构；Gemini API 不接受外链图片。
func inlineImage(raw string) (map[string]any, error) {
	if !strings.HasPrefix(raw, "data:") {
		if _, err := base64.StdEncoding.DecodeString(raw); err == nil && len(raw) > 64 {
			return map[string]any{"bytesBase64Encoded": raw, "mimeType": "image/png"}, nil
		}
		return nil, fmt.Errorf("%w: veo only accepts base64 / data URI images", video.ErrInvalidRequest)
	}
	meta, data, ok := strings.Cut(strings.TrimPrefix(raw, "data:"), ",")
	if !ok {
		return nil, fmt.Errorf("%w: malformed data URI", video.ErrInvalidRequest)
	}
	return map[string]any{"bytesBase64Encoded": data, "mimeType": strings.TrimSuffix(meta, ";base64")}, nil
}

func (Veo) BuildCreate(req video.Request) (video.Call, error) {
	instance := map[string]any{"prompt": req.Prompt}
	for _, img := range req.Images {
		inline, err := inlineImage(img.URL)
		if err != nil {
			return video.Call{}, err
		}
		switch img.Role {
		case video.ImageRoleFirstFrame:
			instance["image"] = inline
		case video.ImageRoleLastFrame:
			instance["lastFrame"] = inline
		default:
			refs, _ := instance["referenceImages"].([]map[string]any)
			instance["referenceImages"] = append(refs, map[string]any{"image": inline, "referenceType": "asset"})
		}
	}
	params := map[string]any{}
	if req.AspectRatio != "" {
		params["aspectRatio"] = req.AspectRatio
	}
	if req.Resolution != "" {
		params["resolution"] = req.Resolution
	}
	if req.DurationSeconds > 0 {
		params["durationSeconds"] = req.DurationSeconds
	}
	if req.NegativePrompt != "" {
		params["negativePrompt"] = req.NegativePrompt
	}
	if req.Seed != nil {
		params["seed"] = *req.Seed
	}
	video.MergeObject(params, req.Extra, "parameters")
	body, err := video.BuildBody(map[string]any{
		"instances":  []map[string]any{instance},
		"parameters": params,
	}, video.WithoutKeys(req.Extra, "parameters"))
	if err != nil {
		return video.Call{}, err
	}
	return video.Call{Method: http.MethodPost, Path: "/v1beta/models/" + req.Model + ":predictLongRunning", Body: body}, nil
}

func (Veo) Classify(method, path string, _ url.Values) video.NativeOp {
	rest, ok := video.AfterSegments(path, "models")
	if !ok || len(rest) == 0 {
		return video.NativeOp{}
	}
	if method == http.MethodPost && len(rest) == 1 && strings.HasSuffix(rest[0], ":predictLongRunning") {
		return video.NativeOp{Kind: video.NativeCreate}
	}
	if method == http.MethodGet && len(rest) == 3 && rest[1] == "operations" {
		return video.NativeOp{Kind: video.NativeQuery, TaskID: "models/" + strings.Join(rest, "/")}
	}
	return video.NativeOp{}
}

func veoModelFromPath(path string) string {
	rest, _ := video.AfterSegments(path, "models")
	if len(rest) == 0 {
		return ""
	}
	return strings.TrimSuffix(rest[0], ":predictLongRunning")
}

func (Veo) ParseNativeCreate(path string, body []byte) (video.Request, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Request{}, err
	}
	params := root.Get("parameters")
	req := video.Request{
		Model:           veoModelFromPath(path),
		Prompt:          root.Get("instances.0.prompt").String(),
		DurationSeconds: video.ParseDurationValue(params.Get("durationSeconds")),
		Resolution:      video.NormalizeResolution(params.Get("resolution").String()),
		AspectRatio:     params.Get("aspectRatio").String(),
	}
	if req.Model == "" {
		return video.Request{}, fmt.Errorf("%w: model is required in path", video.ErrInvalidRequest)
	}
	if req.DurationSeconds == 0 {
		req.DurationSeconds = 8
	}
	if req.Resolution == "" {
		req.Resolution = video.Resolution720P
	}
	return req, nil
}

func (Veo) RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error) {
	current := veoModelFromPath(path)
	if current == "" {
		return body, path, nil
	}
	return body, strings.Replace(path, "models/"+current+":", "models/"+model+":", 1), nil
}

func (Veo) ParseCreate(body []byte) (video.CreateResult, error) {
	name := strings.TrimSpace(gjson.GetBytes(body, "name").String())
	if name == "" || strings.Contains(name, "..") {
		return video.CreateResult{}, fmt.Errorf("veo create response is missing operation name")
	}
	return video.CreateResult{TaskID: name}, nil
}

func (Veo) BuildQuery(taskID, _ string) video.Call {
	return video.Call{Method: http.MethodGet, Path: "/v1beta/" + strings.TrimPrefix(taskID, "/")}
}

func (Veo) ParseQuery(body []byte) (video.Snapshot, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Snapshot{}, err
	}
	snap := video.Snapshot{Status: video.StatusRunning}
	if errObj := root.Get("error"); errObj.Exists() {
		snap.Status = video.StatusFailed
		snap.ErrorCode = errObj.Get("code").String()
		snap.ErrorMessage = errObj.Get("message").String()
		return snap, nil
	}
	if !root.Get("done").Bool() {
		return snap, nil
	}
	resp := root.Get("response.generateVideoResponse")
	snap.VideoURL = resp.Get("generatedSamples.0.video.uri").String()
	if snap.VideoURL == "" {
		snap.Status = video.StatusFailed
		snap.ErrorMessage = video.FirstNonEmpty(resp.Get("raiMediaFilteredReasons.0").String(), "veo returned no video")
		return snap, nil
	}
	snap.Status, snap.Progress, snap.ContentAuth = video.StatusSucceeded, 100, true
	return snap, nil
}

func (Veo) BuildCancel(string, string) (video.Call, bool) { return video.Call{}, false }

// Authorize：官方 Gemini API 用 x-goog-api-key；第三方中转常要求 Bearer，两者同时携带。
func (Veo) Authorize(h http.Header, cred video.Credentials) error {
	key := strings.TrimSpace(cred.APIKey)
	if key == "" {
		return fmt.Errorf("video account is missing api_key")
	}
	h.Set("x-goog-api-key", key)
	if !strings.Contains(cred.BaseURL, "googleapis.com") {
		h.Set("Authorization", "Bearer "+key)
	}
	return nil
}

// ContentURLPaths 列出原生响应中的成品地址字段，透传前替换为网关签名链接。
func (Veo) ContentURLPaths() []string {
	return []string{"response.generateVideoResponse.generatedSamples.0.video.uri"}
}
