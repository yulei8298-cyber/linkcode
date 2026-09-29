//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type grokVideoMultipartPart struct {
	name  string
	value string
	file  []byte
}

func grokVideoMultipartBody(t *testing.T, parts ...grokVideoMultipartPart) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, part := range parts {
		if part.file == nil {
			require.NoError(t, w.WriteField(part.name, part.value))
			continue
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="`+part.name+`"; filename="reference.png"`)
		header.Set("Content-Type", "image/png")
		field, err := w.CreatePart(header)
		require.NoError(t, err)
		_, err = field.Write(part.file)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return body.Bytes(), w.FormDataContentType()
}

func TestForwardGrokMediaConvertsVideoMultipartAndPreservesBilling(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		fields                  []grokVideoMultipartPart
		duration                int64
		resolution, aspectRatio string
	}{
		{
			name:     "画布字段",
			fields:   []grokVideoMultipartPart{{name: "seconds", value: "6"}, {name: "resolution_name", value: "720p"}, {name: "size", value: "1280x720"}},
			duration: 6, resolution: "720p", aspectRatio: "16:9",
		},
		{
			name:     "正规字段在别名前",
			fields:   []grokVideoMultipartPart{{name: "duration", value: "12"}, {name: "seconds", value: "6"}, {name: "resolution", value: "hd"}, {name: "resolution_name", value: "480p"}, {name: "aspect_ratio", value: "3:4"}, {name: "size", value: "1280x720"}},
			duration: 12, resolution: "720p", aspectRatio: "3:4",
		},
		{
			name:     "正规字段在别名后",
			fields:   []grokVideoMultipartPart{{name: "seconds", value: "6"}, {name: "duration", value: "10"}, {name: "resolution_name", value: "720p"}, {name: "resolution", value: "1080p"}, {name: "size", value: "720x1280"}},
			duration: 10, resolution: "1080p", aspectRatio: "9:16",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parts := append([]grokVideoMultipartPart{{name: "model", value: "grok-imagine-video"}, {name: "prompt", value: "waves"}, {name: "preset", value: "normal"}}, tt.fields...)
			body, contentType := grokVideoMultipartBody(t, parts...)
			prepared, preparedType, err := PrepareGrokVideoGenerationRequest(body, contentType)
			require.NoError(t, err)
			info := ParseGrokMediaRequest(preparedType, prepared)
			require.Equal(t, int(tt.duration), info.DurationSeconds)
			require.Equal(t, tt.resolution, info.Resolution)
			require.Equal(t, "waves", gjson.GetBytes(info.ModerationBody(), "prompt").String())
			upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"request_id":"video-task"}`)}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			c, _ := grokMediaContentTestContext(http.MethodPost, "https://api.example/v1/videos", nil)
			result, err := svc.ForwardGrokMedia(context.Background(), c, grokMediaContentTestAccount(), GrokMediaEndpointVideosGenerations, "", body, contentType)
			require.NoError(t, err)
			require.Equal(t, "application/json", upstream.request.Header.Get("Content-Type"))
			forwarded, err := io.ReadAll(upstream.request.Body)
			require.NoError(t, err)
			require.JSONEq(t, string(prepared), string(forwarded))
			require.Equal(t, int64(len(forwarded)), upstream.request.ContentLength)
			require.Equal(t, "grok-imagine-video", gjson.GetBytes(forwarded, "model").String())
			require.Equal(t, tt.duration, gjson.GetBytes(forwarded, "duration").Int())
			require.Equal(t, tt.resolution, gjson.GetBytes(forwarded, "resolution").String())
			require.Equal(t, tt.aspectRatio, gjson.GetBytes(forwarded, "aspect_ratio").String())
			for _, alias := range []string{"seconds", "resolution_name", "size", "preset"} {
				require.False(t, gjson.GetBytes(forwarded, alias).Exists(), alias)
			}
			require.Equal(t, info.DurationSeconds, result.VideoDurationSeconds)
			require.Equal(t, info.Resolution, result.VideoResolution)
		})
	}
}

func TestGrokMediaVideoMultipartReferencesAreVisibleToModeration(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aDc8AAAAASUVORK5CYII=")
	require.NoError(t, err)
	wantURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	for _, count := range []int{1, 2, 7} {
		parts := []grokVideoMultipartPart{{name: "model", value: "grok-imagine-video"}}
		for range count {
			parts = append(parts, grokVideoMultipartPart{name: "input_reference[]", file: png})
		}
		body, contentType := grokVideoMultipartBody(t, parts...)
		prepared, preparedType, err := PrepareGrokVideoGenerationRequest(body, contentType)
		require.NoError(t, err)
		info := ParseGrokMediaRequest(preparedType, prepared)
		require.Len(t, info.InputImageURLs, count)
		require.True(t, info.HasInputImage())
		require.Len(t, gjson.GetBytes(info.ModerationBody(), "images").Array(), count)
		require.Equal(t, wantURL, gjson.GetBytes(info.ModerationBody(), "images.0.image_url").String())
		if count == 1 {
			require.Equal(t, wantURL, gjson.GetBytes(prepared, "image.url").String())
			require.False(t, gjson.GetBytes(prepared, "reference_images").Exists())
		} else {
			require.False(t, gjson.GetBytes(prepared, "image").Exists())
			require.Len(t, gjson.GetBytes(prepared, "reference_images").Array(), count)
			require.Equal(t, wantURL, gjson.GetBytes(prepared, "reference_images.0.url").String())
		}
	}
	for _, canonical := range []string{"image", "reference_images[]"} {
		body, contentType := grokVideoMultipartBody(t,
			grokVideoMultipartPart{name: "input_reference[]", file: png},
			grokVideoMultipartPart{name: canonical, value: "https://example.com/preferred.png"},
		)
		prepared, preparedType, err := PrepareGrokVideoGenerationRequest(body, contentType)
		require.NoError(t, err)
		info := ParseGrokMediaRequest(preparedType, prepared)
		require.Equal(t, []string{"https://example.com/preferred.png"}, info.InputImageURLs)
	}
	body, contentType := grokVideoMultipartBody(t, grokVideoMultipartPart{name: "input_reference[]", file: png})
	body = bytes.Replace(body, []byte(`filename="reference.png"`), []byte(`filename=""`), 1)
	prepared, _, err := PrepareGrokVideoGenerationRequest(body, contentType)
	require.NoError(t, err)
	require.Equal(t, wantURL, gjson.GetBytes(prepared, "image.url").String())
}

func TestForwardGrokMediaRejectsInvalidVideoMultipartWithoutUpstream(t *testing.T) {
	validBody, validType := grokVideoMultipartBody(t, grokVideoMultipartPart{name: "model", value: "grok-imagine-video"}, grokVideoMultipartPart{name: "prompt", value: "waves"})
	tests := []struct {
		name        string
		body        []byte
		contentType string
	}{
		{"缺少boundary", validBody, "multipart/form-data"},
		{"非法Content-Type", validBody, "multipart/form-data; boundary=\""},
		{"截断表单", validBody[:len(validBody)-10], validType},
		{"错误boundary", validBody, "multipart/form-data; boundary=wrong"},
	}
	for _, invalid := range []struct {
		name   string
		fields []grokVideoMultipartPart
	}{
		{"空文件", []grokVideoMultipartPart{{name: "input_reference[]", file: []byte{}}}},
		{"伪造图片类型", []grokVideoMultipartPart{{name: "input_reference[]", file: []byte("not an image")}}},
		{"不支持的上传字段", []grokVideoMultipartPart{{name: "attachment", file: []byte("unrelated")}}},
		{"上传过大", []grokVideoMultipartPart{{name: "input_reference[]", file: bytes.Repeat([]byte("x"), openAIImageMaxUploadPartSize+1)}}},
		{"非法时长", []grokVideoMultipartPart{{name: "seconds", value: "invalid"}}},
		{"超出计费支持时长", []grokVideoMultipartPart{{name: "seconds", value: "20"}}},
		{"正规时长不回退", []grokVideoMultipartPart{{name: "duration", value: ""}, {name: "seconds", value: "6"}}},
		{"正规清晰度不回退", []grokVideoMultipartPart{{name: "resolution", value: "4k"}, {name: "resolution_name", value: "720p"}}},
		{"非法尺寸", []grokVideoMultipartPart{{name: "size", value: "0x720"}}},
		{"参考图过多", []grokVideoMultipartPart{{name: "input_reference[]", value: "https://example.com/1.png"}, {name: "input_reference[]", value: "https://example.com/2.png"}, {name: "input_reference[]", value: "https://example.com/3.png"}, {name: "input_reference[]", value: "https://example.com/4.png"}, {name: "input_reference[]", value: "https://example.com/5.png"}, {name: "input_reference[]", value: "https://example.com/6.png"}, {name: "input_reference[]", value: "https://example.com/7.png"}, {name: "input_reference[]", value: "https://example.com/8.png"}}},
	} {
		body, contentType := grokVideoMultipartBody(t, append([]grokVideoMultipartPart{{name: "model", value: "grok-imagine-video"}}, invalid.fields...)...)
		tests = append(tests, struct {
			name        string
			body        []byte
			contentType string
		}{invalid.name, body, contentType})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &grokMediaContentUpstreamStub{}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			c, _ := grokMediaContentTestContext(http.MethodPost, "https://api.example/v1/videos", nil)
			_, err := svc.ForwardGrokMedia(context.Background(), c, grokMediaContentTestAccount(), GrokMediaEndpointVideosGenerations, "", tt.body, tt.contentType)
			require.Error(t, err)
			require.Contains(t, err.Error(), "video")
			require.Empty(t, upstream.requests)
		})
	}
}

func TestGrokMediaVideoMultipartKeepsNativeJSONAndImageEditBehavior(t *testing.T) {
	body := []byte(`{"model":"grok-imagine-video-1.5","duration":20,"resolution":"future","keyframes":[{"image":{"url":"https://example.com/a.png"},"timestamp_s":3}]}`)
	out, contentType, err := PrepareGrokVideoGenerationRequest(body, "application/json; charset=utf-8")
	require.NoError(t, err)
	require.Equal(t, body, out)
	require.Equal(t, "application/json; charset=utf-8", contentType)

	imageBody, imageType := grokVideoMultipartBody(t, grokVideoMultipartPart{name: "model", value: "grok-imagine-image"}, grokVideoMultipartPart{name: "image", value: "https://example.com/image.png"})
	out, contentType, err = prepareGrokMediaForwardBody(GrokMediaEndpointImagesEdits, imageBody, imageType)
	require.NoError(t, err)
	require.Equal(t, "application/json", contentType)
	require.Equal(t, "https://example.com/image.png", gjson.GetBytes(out, "image.url").String())
	require.Equal(t, "image_url", gjson.GetBytes(out, "image.type").String())
}

func TestGrokMediaVideoAspectRatioStaysWithinVideoOptions(t *testing.T) {
	for size, want := range map[string]string{
		"1280x720": "16:9", "720x1280": "9:16", "1024x1024": "1:1",
		"1792x1024": "16:9", "1024x1792": "9:16", "2000x1000": "16:9",
		"1200x800": "3:2", "800x1200": "2:3", "1200x900": "4:3", "900x1200": "3:4",
	} {
		got, err := grokVideoAspectRatioFromSize(size)
		require.NoError(t, err)
		require.Equal(t, want, got, size)
	}
}
