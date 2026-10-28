// Package cn 实现国内视频厂商的上游协议：火山方舟、可灵、海螺、通义万相、Vidu。
package cn

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const arkTasksPath = "/api/v3/contents/generations/tasks"

var arkStatus = map[string]video.Status{
	"queued":    video.StatusQueued,
	"running":   video.StatusRunning,
	"succeeded": video.StatusSucceeded,
	"failed":    video.StatusFailed,
	"cancelled": video.StatusCancelled,
	"expired":   video.StatusFailed,
}

// Ark 实现火山方舟 / BytePlus ModelArk 的 Seedance 异步视频任务协议。
type Ark struct{}

func (Ark) Info() video.Info {
	return video.Info{
		Protocol:       video.ProtocolArk,
		DisplayName:    "Seedance / 豆包（火山方舟）",
		Vendor:         "seedance",
		DefaultBaseURL: "https://ark.cn-beijing.volces.com/api/v3",
		NativePrefix:   "/video/ark",
		ExampleModels:  []string{"doubao-seedance-2-0-260128", "doubao-seedance-2-0-fast-260128", "doubao-seedance-1-5-pro-251215"},
	}
}

func (Ark) BuildCreate(req video.Request) (video.Call, error) {
	content := []map[string]any{}
	if req.Prompt != "" {
		content = append(content, map[string]any{"type": "text", "text": req.Prompt})
	}
	for _, img := range req.Images {
		role := "reference_image"
		switch img.Role {
		case video.ImageRoleFirstFrame:
			role = "first_frame"
		case video.ImageRoleLastFrame:
			role = "last_frame"
		}
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": img.URL}, "role": role})
	}
	for _, v := range req.Videos {
		content = append(content, map[string]any{"type": "video_url", "video_url": map[string]any{"url": v}, "role": "reference_video"})
	}
	for _, a := range req.Audios {
		content = append(content, map[string]any{"type": "audio_url", "audio_url": map[string]any{"url": a}, "role": "reference_audio"})
	}
	body, err := video.BuildBody(map[string]any{
		"model":          req.Model,
		"content":        content,
		"duration":       req.DurationSeconds,
		"resolution":     req.Resolution,
		"ratio":          req.AspectRatio,
		"generate_audio": req.GenerateAudio,
		"seed":           req.Seed,
	}, req.Extra)
	if err != nil {
		return video.Call{}, err
	}
	return video.Call{Method: http.MethodPost, Path: arkTasksPath, Body: body}, nil
}

func (Ark) Classify(method, path string, _ url.Values) video.NativeOp {
	rest, ok := video.AfterSegments(path, "contents", "generations", "tasks")
	if !ok {
		return video.NativeOp{}
	}
	switch {
	case method == http.MethodPost && len(rest) == 0:
		return video.NativeOp{Kind: video.NativeCreate}
	case method == http.MethodGet && len(rest) == 1:
		return video.NativeOp{Kind: video.NativeQuery, TaskID: rest[0]}
	case method == http.MethodDelete && len(rest) == 1:
		return video.NativeOp{Kind: video.NativeCancel, TaskID: rest[0]}
	}
	return video.NativeOp{}
}

func (Ark) ParseNativeCreate(_ string, body []byte) (video.Request, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Request{}, err
	}
	req := video.Request{
		Model:           strings.TrimSpace(root.Get("model").String()),
		DurationSeconds: video.ParseDurationValue(root.Get("duration")),
		Resolution:      video.NormalizeResolution(root.Get("resolution").String()),
		AspectRatio:     root.Get("ratio").String(),
	}
	if v := root.Get("generate_audio"); v.Exists() {
		req.GenerateAudio = video.BoolPtr(v.Bool())
	}
	var texts []string
	for _, item := range root.Get("content").Array() {
		switch item.Get("type").String() {
		case "text":
			text := item.Get("text").String()
			applyArkTextFlags(text, &req)
			texts = append(texts, text)
		case "image_url":
			role := video.ImageRoleReference
			switch item.Get("role").String() {
			case "first_frame", "":
				role = video.ImageRoleFirstFrame
			case "last_frame":
				role = video.ImageRoleLastFrame
			}
			req.Images = append(req.Images, video.Image{URL: item.Get("image_url.url").String(), Role: role})
		case "video_url":
			req.Videos = append(req.Videos, item.Get("video_url.url").String())
		}
	}
	req.Prompt = strings.Join(texts, "\n")
	return req, nil
}

// applyArkTextFlags 解析旧版写在文本里的 --duration / --resolution / --ratio 参数。
func applyArkTextFlags(text string, req *video.Request) {
	fields := strings.Fields(text)
	for i := 0; i+1 < len(fields); i++ {
		value := fields[i+1]
		switch fields[i] {
		case "--duration", "--dur":
			if req.DurationSeconds == 0 {
				req.DurationSeconds = video.ParseDurationValue(gjson.Parse(`"` + value + `"`))
			}
		case "--resolution", "--rs":
			if req.Resolution == "" {
				req.Resolution = video.NormalizeResolution(value)
			}
		case "--ratio", "--rt":
			if req.AspectRatio == "" {
				req.AspectRatio = value
			}
		}
	}
}

func (Ark) RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error) {
	out, err := sjson.SetBytes(body, "model", model)
	return out, path, err
}

func (Ark) ParseCreate(body []byte) (video.CreateResult, error) {
	id, err := video.RequireTaskID(gjson.GetBytes(body, "id"), video.ProtocolArk)
	return video.CreateResult{TaskID: id}, err
}

func (Ark) BuildQuery(taskID, _ string) video.Call {
	return video.Call{Method: http.MethodGet, Path: arkTasksPath + "/" + taskID}
}

func (Ark) ParseQuery(body []byte) (video.Snapshot, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Snapshot{}, err
	}
	snap := video.Snapshot{
		Status:           video.MapStatus(root.Get("status").String(), arkStatus),
		VideoURL:         root.Get("content.video_url").String(),
		DurationSeconds:  root.Get("duration").Float(),
		Resolution:       video.NormalizeResolution(root.Get("resolution").String()),
		CompletionTokens: root.Get("usage.completion_tokens").Int(),
		ErrorCode:        root.Get("error.code").String(),
		ErrorMessage:     root.Get("error.message").String(),
	}
	if snap.Status == video.StatusSucceeded {
		snap.Progress = 100
	}
	return snap, nil
}

func (Ark) BuildCancel(taskID, _ string) (video.Call, bool) {
	return video.Call{Method: http.MethodDelete, Path: arkTasksPath + "/" + taskID}, true
}

func (Ark) Authorize(h http.Header, cred video.Credentials) error {
	return video.SetBearer(h, cred)
}

// ContentURLPaths 列出原生响应中的成品地址字段，透传前替换为网关签名链接。
func (Ark) ContentURLPaths() []string {
	return []string{"content.video_url"}
}
