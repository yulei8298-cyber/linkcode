//go:build unit

package video

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeResolution(t *testing.T) {
	cases := map[string]string{
		"720P": Resolution720P, "hd": Resolution720P, "1280x720": Resolution720P, "720*1280": Resolution720P,
		"1080p": Resolution1080P, "1920x1080": Resolution1080P, "4K": Resolution4K, "3840x2160": Resolution4K,
		"540p": Resolution480P, "": "", "weird": "",
	}
	for in, want := range cases {
		require.Equal(t, want, NormalizeResolution(in), in)
	}
}

func TestAspectRatioAndSize(t *testing.T) {
	require.Equal(t, "16:9", AspectRatioFromSize("1280x720"))
	require.Equal(t, "9:16", AspectRatioFromSize("720x1280"))
	require.Equal(t, "1:1", AspectRatioFromSize("1024x1024"))
	require.Equal(t, "", AspectRatioFromSize("abc"))
	require.Equal(t, "1280x720", SizeFor(Resolution720P, "16:9"))
	require.Equal(t, "1080x1920", SizeFor(Resolution1080P, "9:16"))
	require.Equal(t, "1280x720", SizeFor("", ""))
}

func TestJoinURLDedupesVersionPrefix(t *testing.T) {
	cases := []struct{ base, path, want string }{
		{"https://ark.cn-beijing.volces.com/api/v3", "/api/v3/contents/generations/tasks", "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks"},
		{"https://ark.cn-beijing.volces.com", "/api/v3/contents/generations/tasks", "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks"},
		{"https://relay.example.com/v1/", "/v1/videos", "https://relay.example.com/v1/videos"},
		{"https://relay.example.com/kling", "/v1/videos/text2video", "https://relay.example.com/kling/v1/videos/text2video"},
		{"https://api.example.com", "/v1/videos/abc", "https://api.example.com/v1/videos/abc"},
		{"https://api.example.com/v1", "/v1", "https://api.example.com/v1"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, JoinURL(tc.base, tc.path), tc.base+" + "+tc.path)
	}
}

func TestParseUnifiedJSONOpenAIStyle(t *testing.T) {
	req, err := ParseUnifiedJSON([]byte(`{"model":"sora-2","prompt":"waves","seconds":"8","size":"1280x720","input_reference":"https://img/a.png"}`))
	require.NoError(t, err)
	require.Equal(t, "sora-2", req.Model)
	require.Equal(t, 8, req.DurationSeconds)
	require.Equal(t, Resolution720P, req.Resolution)
	require.Equal(t, "16:9", req.AspectRatio)
	require.Equal(t, "https://img/a.png", req.FirstImage(ImageRoleFirstFrame))
}

func TestParseUnifiedJSONNewAPIStyle(t *testing.T) {
	req, err := ParseUnifiedJSON([]byte(`{"model":"kling-v3","prompt":"cat","duration":10,"image":["a","b"],"metadata":{"mode":"pro","cfg_scale":0.5},"generate_audio":true,"seed":7}`))
	require.NoError(t, err)
	require.Equal(t, 10, req.DurationSeconds)
	require.Len(t, req.Images, 2)
	require.Equal(t, ImageRoleReference, req.Images[0].Role)
	require.JSONEq(t, `"pro"`, string(req.Extra["mode"]))
	require.True(t, *req.GenerateAudio)
	require.EqualValues(t, 7, *req.Seed)
}

func TestParseUnifiedJSONRejectsInvalid(t *testing.T) {
	for _, body := range []string{`[]`, `{"prompt":"x"}`, `{"model":"m"}`, `not json`} {
		_, err := ParseUnifiedJSON([]byte(body))
		require.ErrorIs(t, err, ErrInvalidRequest, body)
	}
}

func TestParseDurationValue(t *testing.T) {
	require.Equal(t, 5, ParseDurationValue(gjson.Parse(`5`)))
	require.Equal(t, 5, ParseDurationValue(gjson.Parse(`"5s"`)))
	require.Equal(t, 6, ParseDurationValue(gjson.Parse(`5.6`)))
	require.Equal(t, 0, ParseDurationValue(gjson.Parse(`-1`)))
	require.Equal(t, 0, ParseDurationValue(gjson.Result{}))
}

func TestPathHelpers(t *testing.T) {
	rest, ok := AfterSegments("/api/v3/contents/generations/tasks/abc", "contents", "generations", "tasks")
	require.True(t, ok)
	require.Equal(t, []string{"abc"}, rest)
	require.True(t, HasSuffixPath("/v1/query/video_generation", "query", "video_generation"))
	require.False(t, SafeSegment("../x"))
	require.True(t, SafeSegment("task-1"))
	require.Equal(t, StatusRunning, MapStatus("mystery", map[string]Status{}))
	require.True(t, StatusCancelled.IsTerminal())
}
