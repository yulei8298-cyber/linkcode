package cn

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// 可灵按任务类型区分端点，查询时必须回到同一类型的路径。
const (
	klingKindText2Video  = "text2video"
	klingKindImage2Video = "image2video"
	klingKindMultiImage  = "multi-image2video"
)

var klingKinds = map[string]bool{klingKindText2Video: true, klingKindImage2Video: true, klingKindMultiImage: true, "omni-video": true}

var klingStatus = map[string]video.Status{
	"submitted":  video.StatusQueued,
	"processing": video.StatusRunning,
	"succeed":    video.StatusSucceeded,
	"succeeded":  video.StatusSucceeded,
	"failed":     video.StatusFailed,
}

// Kling 实现快手可灵开放平台视频接口。官方账号使用 AccessKey + SecretKey 签发 JWT，
// 第三方中转只填一个 Key 时按 Bearer 直接发送。
type Kling struct{}

func (Kling) Info() video.Info {
	return video.Info{
		Protocol:          video.ProtocolKling,
		DisplayName:       "可灵 Kling",
		Vendor:            "kling",
		DefaultBaseURL:    "https://api-beijing.klingai.com",
		SupportsSecretKey: true,
		NativePrefix:      "/video/kling",
		ExampleModels:     []string{"kling-v3", "kling-v2-5-turbo", "kling-v2-1-master"},
	}
}

func (Kling) BuildCreate(req video.Request) (video.Call, error) {
	kind := klingKindText2Video
	fields := map[string]any{
		"model_name":      req.Model,
		"prompt":          req.Prompt,
		"negative_prompt": req.NegativePrompt,
		"duration":        video.FormatDurationSeconds(req.DurationSeconds),
		"aspect_ratio":    req.AspectRatio,
		"mode":            klingModeForResolution(req.Resolution),
	}
	if req.GenerateAudio != nil {
		fields["sound"] = map[bool]string{true: "on", false: "off"}[*req.GenerateAudio]
	}
	refs := []map[string]any{}
	for _, img := range req.Images {
		if img.Role == video.ImageRoleReference {
			refs = append(refs, map[string]any{"image": img.URL})
		}
	}
	switch {
	case len(refs) > 0:
		kind = klingKindMultiImage
		fields["image_list"] = refs
	case req.FirstImage(video.ImageRoleFirstFrame) != "":
		kind = klingKindImage2Video
		fields["image"] = req.FirstImage(video.ImageRoleFirstFrame)
		fields["image_tail"] = req.FirstImage(video.ImageRoleLastFrame)
		delete(fields, "aspect_ratio")
	}
	body, err := video.BuildBody(fields, req.Extra)
	if err != nil {
		return video.Call{}, err
	}
	return video.Call{Method: http.MethodPost, Path: "/v1/videos/" + kind, Body: body}, nil
}

// klingModeForResolution：std 输出 720p，pro 输出 1080p。
func klingModeForResolution(resolution string) string {
	switch resolution {
	case video.Resolution1080P, video.Resolution4K:
		return "pro"
	case "":
		return ""
	default:
		return "std"
	}
}

func (Kling) Classify(method, path string, _ url.Values) video.NativeOp {
	rest, ok := video.AfterSegments(path, "videos")
	if !ok || len(rest) == 0 || !klingKinds[rest[0]] {
		return video.NativeOp{}
	}
	switch {
	case method == http.MethodPost && len(rest) == 1:
		return video.NativeOp{Kind: video.NativeCreate}
	case method == http.MethodGet && len(rest) == 2:
		return video.NativeOp{Kind: video.NativeQuery, TaskID: rest[1]}
	}
	return video.NativeOp{}
}

func (Kling) ParseNativeCreate(_ string, body []byte) (video.Request, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Request{}, err
	}
	req := video.Request{
		Model:           strings.TrimSpace(video.FirstNonEmpty(root.Get("model_name").String(), root.Get("model").String())),
		Prompt:          root.Get("prompt").String(),
		DurationSeconds: video.ParseDurationValue(root.Get("duration")),
		AspectRatio:     root.Get("aspect_ratio").String(),
		Resolution:      video.Resolution720P,
	}
	if root.Get("mode").String() == "pro" {
		req.Resolution = video.Resolution1080P
	}
	if root.Get("sound").String() == "on" {
		req.GenerateAudio = video.BoolPtr(true)
	}
	if img := root.Get("image").String(); img != "" {
		req.Images = append(req.Images, video.Image{URL: img, Role: video.ImageRoleFirstFrame})
	}
	if tail := root.Get("image_tail").String(); tail != "" {
		req.Images = append(req.Images, video.Image{URL: tail, Role: video.ImageRoleLastFrame})
	}
	if req.DurationSeconds == 0 {
		req.DurationSeconds = 5
	}
	return req, nil
}

func (Kling) RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error) {
	key := "model_name"
	if !gjson.GetBytes(body, "model_name").Exists() && gjson.GetBytes(body, "model").Exists() {
		key = "model"
	}
	out, err := sjson.SetBytes(body, key, model)
	return out, path, err
}

func (Kling) ParseCreate(body []byte) (video.CreateResult, error) {
	if code := gjson.GetBytes(body, "code"); code.Exists() && code.Int() != 0 {
		return video.CreateResult{}, fmt.Errorf("kling create failed: %s", gjson.GetBytes(body, "message").String())
	}
	id, err := video.RequireTaskID(gjson.GetBytes(body, "data.task_id"), video.ProtocolKling)
	return video.CreateResult{TaskID: id}, err
}

// CreateMeta 从创建路径取出任务类型，查询时据此回到同类端点。
func (Kling) CreateMeta(createPath string) string {
	rest, ok := video.AfterSegments(createPath, "videos")
	if ok && len(rest) > 0 && klingKinds[rest[0]] {
		return rest[0]
	}
	return klingKindText2Video
}

func (Kling) BuildQuery(taskID, meta string) video.Call {
	kind := meta
	if !klingKinds[kind] {
		kind = klingKindText2Video
	}
	return video.Call{Method: http.MethodGet, Path: "/v1/videos/" + kind + "/" + taskID}
}

func (Kling) ParseQuery(body []byte) (video.Snapshot, error) {
	root, err := video.RequireJSONObject(body)
	if err != nil {
		return video.Snapshot{}, err
	}
	data := root.Get("data")
	snap := video.Snapshot{
		Status:          video.MapStatus(data.Get("task_status").String(), klingStatus),
		VideoURL:        data.Get("task_result.videos.0.url").String(),
		DurationSeconds: data.Get("task_result.videos.0.duration").Float(),
	}
	if snap.Status == video.StatusFailed {
		snap.ErrorMessage = video.FirstNonEmpty(data.Get("task_status_msg").String(), root.Get("message").String())
	}
	if snap.Status == video.StatusSucceeded {
		snap.Progress = 100
	}
	return snap, nil
}

func (Kling) BuildCancel(string, string) (video.Call, bool) { return video.Call{}, false }

func (Kling) Authorize(h http.Header, cred video.Credentials) error {
	if strings.TrimSpace(cred.SecretKey) == "" {
		return video.SetBearer(h, cred)
	}
	token, err := KlingJWT(cred.APIKey, cred.SecretKey, time.Now())
	if err != nil {
		return err
	}
	h.Set("Authorization", "Bearer "+token)
	return nil
}

// KlingJWT 按可灵官方规则签发 30 分钟有效的 HS256 令牌。
func KlingJWT(accessKey, secretKey string, now time.Time) (string, error) {
	accessKey, secretKey = strings.TrimSpace(accessKey), strings.TrimSpace(secretKey)
	if accessKey == "" || secretKey == "" {
		return "", fmt.Errorf("kling access key and secret key are required")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": accessKey,
		"exp": now.Add(30 * time.Minute).Unix(),
		"nbf": now.Add(-5 * time.Second).Unix(),
	})
	return token.SignedString([]byte(secretKey))
}

// ContentURLPaths 列出原生响应中的成品地址字段，透传前替换为网关签名链接。
func (Kling) ContentURLPaths() []string {
	return []string{"data.task_result.videos.0.url"}
}
