package video

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// 统一入口的扩展参数容器：其中的键原样合并进上游请求体，用来传厂商专有参数。
var unifiedExtraContainers = []string{"metadata", "extra_body", "parameters"}

// ErrInvalidRequest 表示统一请求不合法。
var ErrInvalidRequest = errors.New("invalid video request")

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRequest, fmt.Sprintf(format, args...))
}

// ParseUnifiedJSON 解析统一入口的 JSON 请求体，兼容 OpenAI /v1/videos 与
// new-api /v1/video/generations 两种字段写法。
func ParseUnifiedJSON(body []byte) (Request, error) {
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return Request{}, invalidf("request body must be a JSON object")
	}
	root := gjson.ParseBytes(body)
	req := Request{
		Model:          strings.TrimSpace(root.Get("model").String()),
		Prompt:         root.Get("prompt").String(),
		NegativePrompt: root.Get("negative_prompt").String(),
		Size:           strings.TrimSpace(root.Get("size").String()),
	}
	if req.Model == "" {
		return Request{}, invalidf("model is required")
	}
	req.DurationSeconds = ParseDurationValue(FirstExisting(root, "seconds", "duration", "duration_seconds"))
	req.Resolution = NormalizeResolution(FirstNonEmpty(root.Get("resolution").String(), root.Get("quality").String()))
	if req.Resolution == "" {
		req.Resolution = NormalizeResolution(req.Size)
	}
	req.AspectRatio = strings.TrimSpace(FirstNonEmpty(root.Get("aspect_ratio").String(), root.Get("ratio").String()))
	if req.AspectRatio == "" {
		req.AspectRatio = AspectRatioFromSize(req.Size)
	}
	if v := FirstExisting(root, "generate_audio", "with_audio", "audio"); v.Exists() && (v.Type == gjson.True || v.Type == gjson.False) {
		req.GenerateAudio = BoolPtr(v.Bool())
	}
	if v := root.Get("seed"); v.Exists() && v.Type == gjson.Number {
		seed := v.Int()
		req.Seed = &seed
	}
	collectImages(root, &req)
	req.Videos = append(StringList(root.Get("video")), StringList(root.Get("videos"))...)
	req.Videos = append(req.Videos, StringList(root.Get("reference_videos"))...)
	req.Audios = append(StringList(root.Get("audio_url")), StringList(root.Get("reference_audios"))...)
	req.Extra = map[string]json.RawMessage{}
	for _, key := range unifiedExtraContainers {
		obj := root.Get(key)
		if !obj.IsObject() {
			continue
		}
		obj.ForEach(func(k, v gjson.Result) bool {
			req.Extra[k.String()] = json.RawMessage(v.Raw)
			return true
		})
	}
	if req.Prompt == "" && len(req.Images) == 0 && len(req.Videos) == 0 {
		return Request{}, invalidf("prompt or input media is required")
	}
	return req, nil
}

// ParseUnifiedMultipart 解析 OpenAI SDK 以 multipart 提交的 /v1/videos 请求，
// input_reference 文件转为 data URI 作为首帧。
func ParseUnifiedMultipart(r *http.Request, maxFileBytes int64) (Request, error) {
	if err := r.ParseMultipartForm(maxFileBytes); err != nil {
		return Request{}, invalidf("invalid multipart form: %v", err)
	}
	form := r.MultipartForm
	payload := map[string]any{}
	for key, values := range form.Value {
		if len(values) == 0 {
			continue
		}
		payload[key] = values[0]
	}
	for _, field := range []string{"input_reference", "image", "first_frame"} {
		files := form.File[field]
		if len(files) == 0 {
			continue
		}
		uri, err := fileToDataURI(files[0], maxFileBytes)
		if err != nil {
			return Request{}, err
		}
		payload[field] = uri
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Request{}, err
	}
	return ParseUnifiedJSON(body)
}

func fileToDataURI(fh *multipart.FileHeader, maxBytes int64) (string, error) {
	if maxBytes > 0 && fh.Size > maxBytes {
		return "", invalidf("input file is too large")
	}
	f, err := fh.Open()
	if err != nil {
		return "", invalidf("read input file: %v", err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", invalidf("read input file: %v", err)
	}
	contentType := fh.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func collectImages(root gjson.Result, req *Request) {
	add := func(url, role string) {
		if url = strings.TrimSpace(url); url != "" {
			req.Images = append(req.Images, Image{URL: url, Role: role})
		}
	}
	add(ImageURLValue(FirstExisting(root, "first_frame", "first_frame_image", "input_reference")), ImageRoleFirstFrame)
	add(ImageURLValue(FirstExisting(root, "last_frame", "last_frame_image", "tail_image", "image_tail")), ImageRoleLastFrame)
	images := append(StringList(root.Get("image")), StringList(root.Get("images"))...)
	if len(images) == 1 && req.FirstImage(ImageRoleFirstFrame) == "" {
		add(images[0], ImageRoleFirstFrame)
		images = nil
	}
	for _, url := range append(images, StringList(root.Get("reference_images"))...) {
		add(url, ImageRoleReference)
	}
}

// ImageURLValue 兼容 "url" 字符串与 {"url":...} / {"image_url":...} 对象写法。
func ImageURLValue(v gjson.Result) string {
	if !v.Exists() {
		return ""
	}
	if v.IsObject() {
		return FirstNonEmpty(v.Get("url").String(), v.Get("image_url").String(), v.Get("image_url.url").String())
	}
	return v.String()
}

// StringList 把字符串或字符串数组（含 {url} 对象）统一为字符串切片。
func StringList(v gjson.Result) []string {
	if !v.Exists() {
		return nil
	}
	if v.IsArray() {
		var out []string
		for _, item := range v.Array() {
			if s := strings.TrimSpace(ImageURLValue(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	if s := strings.TrimSpace(ImageURLValue(v)); s != "" {
		return []string{s}
	}
	return nil
}

// FirstExisting 返回第一个存在且非 null 的字段。
func FirstExisting(root gjson.Result, keys ...string) gjson.Result {
	for _, k := range keys {
		if v := root.Get(k); v.Exists() && v.Type != gjson.Null {
			return v
		}
	}
	return gjson.Result{}
}

// FirstNonEmpty 返回第一个非空白字符串。
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// ParseDurationValue 解析 5、"5"、"5s"、5.0 形式的时长；无法解析返回 0。
func ParseDurationValue(v gjson.Result) int {
	if !v.Exists() {
		return 0
	}
	if v.Type == gjson.Number {
		return int(v.Float() + 0.5)
	}
	s := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(v.String())), "s")
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f < 0 {
		return 0
	}
	return int(f + 0.5)
}
