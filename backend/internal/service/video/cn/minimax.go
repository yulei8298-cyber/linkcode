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

const (
	minimaxCreatePath = "/v1/video_generation"
	minimaxQueryPath  = "/v1/query/video_generation"
	minimaxFilePath   = "/v1/files/retrieve"
)

var minimaxStatus = map[string]video.Status{
	"preparing":  video.StatusQueued,
	"queueing":   video.StatusQueued,
	"processing": video.StatusRunning,
	"success":    video.StatusSucceeded,
	"fail":       video.StatusFailed,
}

// MiniMax 实现海螺视频接口：成功后返回 file_id，需再调用文件接口换取下载地址。
type MiniMax struct{}

func (MiniMax) Info() video.Info {
	return video.Info{
		Protocol:       video.ProtocolMiniMax,
		DisplayName:    "海螺 Hailuo（MiniMax）",
		Vendor:         "hailuo",
		DefaultBaseURL: "https://api.minimaxi.com",
		NativePrefix:   "/video/minimax",
		ExampleModels:  []string{"MiniMax-Hailuo-2.3", "MiniMax-Hailuo-2.3-Fast", "MiniMax-Hailuo-02"},
	}
}

// minimaxResolution 把档位转换为海螺的 512P / 768P / 1080P。
func minimaxResolution(resolution string) string {
	switch resolution {
	case video.Resolution480P:
		return "512P"
	case video.Resolution720P:
		return "768P"
	case video.Resolution1080P, video.Resolution4K:
		return "1080P"
	}
	return ""
}

func (MiniMax) BuildCreate(req video.Request) (video.Call, error) {
	fields := map[string]any{
		"model":             req.Model,
		"prompt":            req.Prompt,
		"duration":          req.DurationSeconds,
		"resolution":        minimaxResolution(req.Resolution),
		"first_frame_image": req.FirstImage(video.ImageRoleFirstFrame),
		"last_frame_image":  req.FirstImage(video.ImageRoleLastFrame),
	}
	refs := []map[string]any{}
	for _, img := range req.Images {
		if img.Role == video.ImageRoleReference {
			refs = append(refs, map[string]any{"type": "character", "image": []string{img.URL}})
		}
	}
	fields["subject_reference"] = refs
	body, err := video.BuildBody(fields, req.Extra)
	if err != nil {
		return video.Call{}, err
	}
	return video.Call{Method: http.MethodPost, Path: minimaxCreatePath, Body: body}, nil
}

func (MiniMax) Classify(method, path string, query url.Values) video.NativeOp {
	switch {
	case method == http.MethodPost && video.HasSuffixPath(path, "video_generation"):
		return video.NativeOp{Kind: video.NativeCreate}
	case method == http.MethodGet && video.HasSuffixPath(path, "query", "video_generation"):
		return video.NativeOp{Kind: video.NativeQuery, TaskID: query.Get("task_id")}
	case method == http.MethodGet && video.HasSuffixPath(path, "files", "retrieve"):
		return video.NativeOp{Kind: video.NativeFile, FileID: query.Get("file_id")}
	}
	return video.NativeOp{}
}

func (MiniMax) ParseNativeCreate(_ string, body []byte) (video.Request, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Request{}, err
	}
	req := video.Request{
		Model:           strings.TrimSpace(root.Get("model").String()),
		Prompt:          root.Get("prompt").String(),
		DurationSeconds: video.ParseDurationValue(root.Get("duration")),
		Resolution:      minimaxTier(root.Get("resolution").String()),
	}
	if img := root.Get("first_frame_image").String(); img != "" {
		req.Images = append(req.Images, video.Image{URL: img, Role: video.ImageRoleFirstFrame})
	}
	if req.DurationSeconds == 0 {
		req.DurationSeconds = 6
	}
	if req.Resolution == "" {
		req.Resolution = video.Resolution720P
	}
	return req, nil
}

func minimaxTier(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "512P":
		return video.Resolution480P
	case "768P", "720P":
		return video.Resolution720P
	}
	return video.NormalizeResolution(raw)
}

func (MiniMax) RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error) {
	out, err := sjson.SetBytes(body, "model", model)
	return out, path, err
}

func minimaxError(root gjson.Result) error {
	if code := root.Get("base_resp.status_code"); code.Exists() && code.Int() != 0 {
		return fmt.Errorf("minimax error %d: %s", code.Int(), root.Get("base_resp.status_msg").String())
	}
	return nil
}

func (MiniMax) ParseCreate(body []byte) (video.CreateResult, error) {
	root := gjson.ParseBytes(body)
	if err := minimaxError(root); err != nil {
		return video.CreateResult{}, err
	}
	id, err := video.RequireTaskID(root.Get("task_id"), video.ProtocolMiniMax)
	return video.CreateResult{TaskID: id}, err
}

func (MiniMax) BuildQuery(taskID, _ string) video.Call {
	return video.Call{Method: http.MethodGet, Path: minimaxQueryPath, Query: url.Values{"task_id": {taskID}}}
}

func (MiniMax) ParseQuery(body []byte) (video.Snapshot, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Snapshot{}, err
	}
	snap := video.Snapshot{
		Status: video.MapStatus(root.Get("status").String(), minimaxStatus),
		FileID: root.Get("file_id").String(),
	}
	if h := root.Get("video_height").Int(); h > 0 {
		snap.Resolution = video.NormalizeResolution(fmt.Sprintf("%dx%d", root.Get("video_width").Int(), h))
	}
	if baseErr := minimaxError(root); baseErr != nil && snap.Status != video.StatusSucceeded {
		snap.Status = video.StatusFailed
		snap.ErrorMessage = baseErr.Error()
	}
	if snap.Status == video.StatusSucceeded {
		snap.Progress = 100
	}
	return snap, nil
}

func (MiniMax) BuildCancel(string, string) (video.Call, bool) { return video.Call{}, false }

func (MiniMax) Authorize(h http.Header, cred video.Credentials) error {
	return video.SetBearer(h, cred)
}

func (MiniMax) BuildFileQuery(fileID string) video.Call {
	return video.Call{Method: http.MethodGet, Path: minimaxFilePath, Query: url.Values{"file_id": {fileID}}}
}

func (MiniMax) ParseFileQuery(body []byte) (string, error) {
	root := gjson.ParseBytes(body)
	if err := minimaxError(root); err != nil {
		return "", err
	}
	link := root.Get("file.download_url").String()
	if link == "" {
		return "", fmt.Errorf("minimax file response is missing download_url")
	}
	return link, nil
}

// ContentURLPaths 列出原生响应中的成品地址字段，透传前替换为网关签名链接。
func (MiniMax) ContentURLPaths() []string {
	return []string{"file.download_url"}
}
