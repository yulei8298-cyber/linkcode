package intl

import (
	"net/http"
	"net/url"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/tidwall/sjson"
)

const newAPIPath = "/v1/video/generations"

// NewAPI 实现 new-api 系中转的统一视频任务接口 /v1/video/generations。
type NewAPI struct{}

func (NewAPI) Info() video.Info {
	return video.Info{
		Protocol:       video.ProtocolNewAPI,
		DisplayName:    "new-api 兼容 /v1/video/generations",
		Vendor:         "newapi",
		DefaultBaseURL: "https://api.example.com",
		NativePrefix:   "/video/newapi",
	}
}

func (NewAPI) BuildCreate(req video.Request) (video.Call, error) {
	fields := map[string]any{
		"model":    req.Model,
		"prompt":   req.Prompt,
		"duration": req.DurationSeconds,
		"seed":     req.Seed,
	}
	if images := req.ImageURLs(); len(images) == 1 {
		fields["image"] = images[0]
	} else if len(images) > 1 {
		fields["images"] = images
	}
	if w, h, ok := video.ParseSize(req.Size); ok {
		fields["width"], fields["height"] = w, h
	}
	metadata := map[string]any{}
	if req.Resolution != "" {
		metadata["resolution"] = req.Resolution
	}
	if req.AspectRatio != "" {
		metadata["aspect_ratio"] = req.AspectRatio
	}
	if req.GenerateAudio != nil {
		metadata["generate_audio"] = *req.GenerateAudio
	}
	for k, v := range req.Extra {
		metadata[k] = v
	}
	fields["metadata"] = metadata
	body, err := video.BuildBody(fields, nil)
	if err != nil {
		return video.Call{}, err
	}
	return video.Call{Method: http.MethodPost, Path: newAPIPath, Body: body}, nil
}

func (NewAPI) Classify(method, path string, _ url.Values) video.NativeOp {
	rest, ok := video.AfterSegments(path, "video", "generations")
	if !ok {
		return video.NativeOp{}
	}
	switch {
	case method == http.MethodPost && len(rest) == 0:
		return video.NativeOp{Kind: video.NativeCreate}
	case method == http.MethodGet && len(rest) == 1:
		return video.NativeOp{Kind: video.NativeQuery, TaskID: rest[0]}
	}
	return video.NativeOp{}
}

func (NewAPI) ParseNativeCreate(_ string, body []byte) (video.Request, error) {
	return video.ParseUnifiedJSON(body)
}

func (NewAPI) RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error) {
	out, err := sjson.SetBytes(body, "model", model)
	return out, path, err
}

func (NewAPI) ParseCreate(body []byte) (video.CreateResult, error) {
	root := unwrapData(body)
	id, err := video.RequireTaskID(video.FirstExisting(root, "task_id", "id"), video.ProtocolNewAPI)
	return video.CreateResult{TaskID: id}, err
}

func (NewAPI) BuildQuery(taskID, _ string) video.Call {
	return video.Call{Method: http.MethodGet, Path: newAPIPath + "/" + taskID}
}

func (NewAPI) ParseQuery(body []byte) (video.Snapshot, error) {
	if _, err := video.RequireJSONObject(body); err != nil {
		return video.Snapshot{}, err
	}
	return genericSnapshot(unwrapData(body)), nil
}

func (NewAPI) BuildCancel(string, string) (video.Call, bool) { return video.Call{}, false }

func (NewAPI) Authorize(h http.Header, cred video.Credentials) error {
	return video.SetBearer(h, cred)
}

// ContentURLPaths 列出原生响应中的成品地址字段，透传前替换为网关签名链接。
func (NewAPI) ContentURLPaths() []string {
	return []string{"video_url", "url", "result_url", "data.video_url", "data.url"}
}
