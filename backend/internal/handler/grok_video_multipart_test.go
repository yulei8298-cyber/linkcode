//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGrokVideoMultipartPreservesPendingBilling(t *testing.T) {
	h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, field := range [][2]string{{"model", "grok-imagine-video"}, {"prompt", "waves"}, {"seconds", "10"}, {"size", "1280x720"}, {"resolution_name", "720p"}} {
		require.NoError(t, w.WriteField(field[0], field[1]))
	}
	require.NoError(t, w.Close())
	original := upstream.call
	upstream.call = func(req *http.Request, accountID int64) (*http.Response, error) {
		require.Equal(t, "application/json", req.Header.Get("Content-Type"))
		forwarded, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, int64(10), gjson.GetBytes(forwarded, "duration").Int())
		require.Equal(t, "720p", gjson.GetBytes(forwarded, "resolution").String())
		require.Equal(t, "16:9", gjson.GetBytes(forwarded, "aspect_ratio").String())
		return original(req, accountID)
	}
	c, recorder := grokMediaSlotContext(context.Background(), true)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())
	h.GrokVideoGeneration(c)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 1, upstream.calls)
	require.Len(t, bindings.pending, 1)
	for _, data := range bindings.pending {
		var pending service.GrokVideoPendingBilling
		require.NoError(t, json.Unmarshal(data, &pending))
		require.Equal(t, "grok-imagine-video", pending.Model)
		require.Equal(t, 10, pending.VideoDurationSeconds)
		require.Equal(t, "720p", pending.VideoResolution)
	}
	slots.assertReleased(t)
}

func TestGrokVideoMalformedMultipartRejectedBeforeScheduling(t *testing.T) {
	for _, contentType := range []string{"multipart/form-data", "multipart/form-data; boundary=missing"} {
		t.Run(contentType, func(t *testing.T) {
			h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false)
			c, recorder := grokMediaSlotContext(context.Background(), true)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader("truncated form"))
			c.Request.Header.Set("Content-Type", contentType)
			h.GrokVideoGeneration(c)
			require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			require.Equal(t, "invalid_request_error", gjson.Get(recorder.Body.String(), "error.type").String())
			require.Zero(t, upstream.calls)
			require.Zero(t, slots.acquired)
			require.Zero(t, bindings.writes)
			require.Empty(t, bindings.pending)
		})
	}
}
