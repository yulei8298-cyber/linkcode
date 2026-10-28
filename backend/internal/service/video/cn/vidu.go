package cn

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var viduKinds = map[string]bool{"text2video": true, "img2video": true, "start-end2video": true, "reference2video": true}

var viduStatus = map[string]video.Status{
	"created":    video.StatusQueued,
	"queueing":   video.StatusQueued,
	"processing": video.StatusRunning,
	"success":    video.StatusSucceeded,
	"failed":     video.StatusFailed,
}

// Vidu 实现生数科技 Vidu 企业 API（鉴权头为 "Token <key>"）。
type Vidu struct{}

func (Vidu) Info() video.Info {
	return video.Info{
		Protocol:       video.ProtocolVidu,
		DisplayName:    "Vidu（生数科技）",
		Vendor:         "vidu",
		DefaultBaseURL: "https://api.vidu.cn",
		NativePrefix:   "/video/vidu",
		ExampleModels:  []string{"viduq2-pro", "viduq2-turbo", "viduq1"},
	}
}

func (Vidu) BuildCreate(req video.Request) (video.Call, error) {
	kind := "text2video"
	fields := map[string]any{
		"model":        req.Model,
		"prompt":       req.Prompt,
		"duration":     req.DurationSeconds,
		"resolution":   req.Resolution,
		"aspect_ratio": req.AspectRatio,
		"seed":         req.Seed,
	}
	first, last := req.FirstImage(video.ImageRoleFirstFrame), req.FirstImage(video.ImageRoleLastFrame)
	refs := []string{}
	for _, img := range req.Images {
		if img.Role == video.ImageRoleReference {
			refs = append(refs, img.URL)
		}
	}
	switch {
	case first != "" && last != "":
		kind = "start-end2video"
		fields["images"] = []string{first, last}
	case first != "":
		kind = "img2video"
		fields["images"] = []string{first}
	case len(refs) > 0:
		kind = "reference2video"
		fields["images"] = refs
	}
	if kind != "text2video" && kind != "reference2video" {
		delete(fields, "aspect_ratio")
	}
	if req.GenerateAudio != nil {
		fields["audio"] = *req.GenerateAudio
	}
	body, err := video.BuildBody(fields, req.Extra)
	if err != nil {
		return video.Call{}, err
	}
	return video.Call{Method: http.MethodPost, Path: "/ent/v2/" + kind, Body: body}, nil
}

func (Vidu) Classify(method, path string, _ url.Values) video.NativeOp {
	segs := video.PathSegments(path)
	if len(segs) == 0 {
		return video.NativeOp{}
	}
	if method == http.MethodPost && viduKinds[segs[len(segs)-1]] {
		return video.NativeOp{Kind: video.NativeCreate}
	}
	rest, ok := video.AfterSegments(path, "tasks")
	if !ok || len(rest) != 2 {
		return video.NativeOp{}
	}
	switch {
	case method == http.MethodGet && rest[1] == "creations":
		return video.NativeOp{Kind: video.NativeQuery, TaskID: rest[0]}
	case method == http.MethodPost && rest[1] == "cancel":
		return video.NativeOp{Kind: video.NativeCancel, TaskID: rest[0]}
	}
	return video.NativeOp{}
}

func (Vidu) ParseNativeCreate(_ string, body []byte) (video.Request, error) {
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
	for i, img := range video.StringList(root.Get("images")) {
		role := video.ImageRoleReference
		if i == 0 {
			role = video.ImageRoleFirstFrame
		}
		req.Images = append(req.Images, video.Image{URL: img, Role: role})
	}
	if req.DurationSeconds == 0 {
		req.DurationSeconds = 5
	}
	if req.Resolution == "" {
		req.Resolution = video.Resolution720P
	}
	return req, nil
}

func (Vidu) RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error) {
	out, err := sjson.SetBytes(body, "model", model)
	return out, path, err
}

func (Vidu) ParseCreate(body []byte) (video.CreateResult, error) {
	root := gjson.ParseBytes(body)
	if root.Get("state").String() == "failed" {
		return video.CreateResult{}, fmt.Errorf("vidu create failed: %s", root.Get("err_code").String())
	}
	id, err := video.RequireTaskID(root.Get("task_id"), video.ProtocolVidu)
	return video.CreateResult{TaskID: id}, err
}

func (Vidu) BuildQuery(taskID, _ string) video.Call {
	return video.Call{Method: http.MethodGet, Path: "/ent/v2/tasks/" + taskID + "/creations"}
}

func (Vidu) ParseQuery(body []byte) (video.Snapshot, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Snapshot{}, err
	}
	snap := video.Snapshot{
		Status:    video.MapStatus(root.Get("state").String(), viduStatus),
		VideoURL:  root.Get("creations.0.url").String(),
		ErrorCode: root.Get("err_code").String(),
	}
	if snap.Status == video.StatusSucceeded {
		snap.Progress = 100
	}
	return snap, nil
}

func (Vidu) BuildCancel(taskID, _ string) (video.Call, bool) {
	body, _ := sjson.SetBytes([]byte("{}"), "id", taskID)
	return video.Call{Method: http.MethodPost, Path: "/ent/v2/tasks/" + taskID + "/cancel", Body: body}, true
}

func (Vidu) Authorize(h http.Header, cred video.Credentials) error {
	key := strings.TrimSpace(cred.APIKey)
	if key == "" {
		return fmt.Errorf("video account is missing api_key")
	}
	h.Set("Authorization", "Token "+key)
	return nil
}

// ContentURLPaths 列出原生响应中的成品地址字段，透传前替换为网关签名链接。
func (Vidu) ContentURLPaths() []string {
	return []string{"creations.0.url", "creations.0.watermarked_url"}
}
