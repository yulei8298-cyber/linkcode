package intl

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	pixverseTextPath   = "/openapi/v2/video/text/generate"
	pixverseImagePath  = "/openapi/v2/video/img/generate"
	pixverseResultPath = "/openapi/v2/video/result"
)

// PixVerse 的状态是数字：1 成功、5 生成中、6 已删除、7 审核未通过、8 失败。
var pixverseStatus = map[string]video.Status{
	"1": video.StatusSucceeded,
	"5": video.StatusRunning,
	"6": video.StatusFailed,
	"7": video.StatusFailed,
	"8": video.StatusFailed,
}

// PixVerse 实现 PixVerse 开放平台（鉴权头为 API-KEY，并要求 Ai-trace-id）。
type PixVerse struct{}

func (PixVerse) Info() video.Info {
	return video.Info{
		Protocol:       video.ProtocolPixVerse,
		DisplayName:    "PixVerse",
		Vendor:         "pixverse",
		DefaultBaseURL: "https://app-api.pixverse.ai",
		NativePrefix:   "/video/pixverse",
		ExampleModels:  []string{"v5.5", "v5"},
	}
}

func (PixVerse) BuildCreate(req video.Request) (video.Call, error) {
	path := pixverseTextPath
	fields := map[string]any{
		"model":           req.Model,
		"prompt":          req.Prompt,
		"negative_prompt": req.NegativePrompt,
		"duration":        req.DurationSeconds,
		"quality":         pixverseQuality(req.Resolution),
		"aspect_ratio":    req.AspectRatio,
		"seed":            req.Seed,
	}
	if req.GenerateAudio != nil {
		fields["generate_audio_switch"] = *req.GenerateAudio
	}
	// 图生视频需要先调用 PixVerse 上传接口取得 img_id，统一入口通过 metadata.img_id 传入。
	if _, ok := req.Extra["img_id"]; ok {
		path = pixverseImagePath
		delete(fields, "aspect_ratio")
	} else if len(req.Images) > 0 {
		return video.Call{}, fmt.Errorf("%w: pixverse image-to-video requires metadata.img_id from the upload API", video.ErrInvalidRequest)
	}
	body, err := video.BuildBody(fields, req.Extra)
	if err != nil {
		return video.Call{}, err
	}
	return video.Call{Method: http.MethodPost, Path: path, Body: body}, nil
}

func pixverseQuality(resolution string) string {
	if resolution == video.Resolution480P {
		return "540p"
	}
	if resolution == video.Resolution4K {
		return "1080p"
	}
	return resolution
}

func (PixVerse) Classify(method, path string, _ url.Values) video.NativeOp {
	if method == http.MethodPost && (video.HasSuffixPath(path, "text", "generate") || video.HasSuffixPath(path, "img", "generate")) {
		return video.NativeOp{Kind: video.NativeCreate}
	}
	rest, ok := video.AfterSegments(path, "video", "result")
	if method == http.MethodGet && ok && len(rest) == 1 {
		return video.NativeOp{Kind: video.NativeQuery, TaskID: rest[0]}
	}
	return video.NativeOp{}
}

func (PixVerse) ParseNativeCreate(_ string, body []byte) (video.Request, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Request{}, err
	}
	req := video.Request{
		Model:           strings.TrimSpace(root.Get("model").String()),
		Prompt:          root.Get("prompt").String(),
		DurationSeconds: video.ParseDurationValue(root.Get("duration")),
		Resolution:      video.NormalizeResolution(root.Get("quality").String()),
		AspectRatio:     root.Get("aspect_ratio").String(),
	}
	if req.DurationSeconds == 0 {
		req.DurationSeconds = 5
	}
	if req.Resolution == "" {
		req.Resolution = video.Resolution720P
	}
	return req, nil
}

func (PixVerse) RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error) {
	out, err := sjson.SetBytes(body, "model", model)
	return out, path, err
}

func pixverseError(root gjson.Result) error {
	if code := root.Get("ErrCode"); code.Exists() && code.Int() != 0 {
		return fmt.Errorf("pixverse error %d: %s", code.Int(), root.Get("ErrMsg").String())
	}
	return nil
}

func (PixVerse) ParseCreate(body []byte) (video.CreateResult, error) {
	root := gjson.ParseBytes(body)
	if err := pixverseError(root); err != nil {
		return video.CreateResult{}, err
	}
	id, err := video.RequireTaskID(root.Get("Resp.video_id"), video.ProtocolPixVerse)
	return video.CreateResult{TaskID: id}, err
}

func (PixVerse) BuildQuery(taskID, _ string) video.Call {
	return video.Call{Method: http.MethodGet, Path: pixverseResultPath + "/" + taskID}
}

func (PixVerse) ParseQuery(body []byte) (video.Snapshot, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Snapshot{}, err
	}
	if apiErr := pixverseError(root); apiErr != nil {
		return video.Snapshot{Status: video.StatusFailed, ErrorMessage: apiErr.Error()}, nil
	}
	resp := root.Get("Resp")
	snap := video.Snapshot{
		Status:   video.MapStatus(resp.Get("status").String(), pixverseStatus),
		VideoURL: resp.Get("url").String(),
	}
	if h := resp.Get("outputHeight").Int(); h > 0 {
		snap.Resolution = video.NormalizeResolution(fmt.Sprintf("%dx%d", resp.Get("outputWidth").Int(), h))
	}
	if snap.Status == video.StatusFailed {
		snap.ErrorCode = resp.Get("status").String()
	}
	if snap.Status == video.StatusSucceeded {
		snap.Progress = 100
	}
	return snap, nil
}

func (PixVerse) BuildCancel(string, string) (video.Call, bool) { return video.Call{}, false }

func (PixVerse) Authorize(h http.Header, cred video.Credentials) error {
	key := strings.TrimSpace(cred.APIKey)
	if key == "" {
		return fmt.Errorf("video account is missing api_key")
	}
	h.Set("API-KEY", key)
	if h.Get("Ai-trace-id") == "" {
		h.Set("Ai-trace-id", newTraceID())
	}
	return nil
}

func newTraceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// ContentURLPaths 列出原生响应中的成品地址字段，透传前替换为网关签名链接。
func (PixVerse) ContentURLPaths() []string {
	return []string{"Resp.url"}
}
