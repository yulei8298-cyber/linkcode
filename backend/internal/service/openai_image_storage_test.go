package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newImageStorageTestContext() *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "https://api.example.test/v1/images/generations", nil)
	return c
}

func TestRewriteImageResponseForDeliveryOffloadsBase64(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := &fakeImageStorage{}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	svc := &OpenAIGatewayService{
		imageStorageResolver: func() (*ImageResultUploader, bool) {
			return uploader, true
		},
	}
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	body := []byte(`{"created":1,"data":[{"b64_json":"` + b64 + `","revised_prompt":"kept"}]}`)

	out := svc.rewriteImageResponseForDelivery(context.Background(), newImageStorageTestContext(), body)

	require.Len(t, storage.saved, 1)
	require.Contains(t, gjson.GetBytes(out, "data.0.url").String(), "https://cdn.test/images/sync_")
	require.False(t, gjson.GetBytes(out, "data.0.b64_json").Exists())
	require.Equal(t, "kept", gjson.GetBytes(out, "data.0.revised_prompt").String())
}

func TestRewriteImageResponseForDeliveryFallsBackWhenUploadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := &fakeImageStorage{err: errors.New("storage unavailable")}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	svc := &OpenAIGatewayService{
		imageStorageResolver: func() (*ImageResultUploader, bool) {
			return uploader, true
		},
	}
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	body := []byte(`{"data":[{"b64_json":"` + b64 + `"}]}`)

	out := svc.rewriteImageResponseForDelivery(context.Background(), newImageStorageTestContext(), body)

	require.JSONEq(t, string(body), string(out))
}

func TestRewriteBuiltImageStreamPayloadForDeliveryOffloadsCompletedOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := &fakeImageStorage{}
	uploader := NewImageResultUploader(storage, "images/", 0, nil)
	svc := &OpenAIGatewayService{
		imageStorageResolver: func() (*ImageResultUploader, bool) {
			return uploader, true
		},
	}
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	partial := []byte(`{"type":"image_generation.partial_image","b64_json":"` + b64 + `"}`)
	completed := []byte(`{"type":"image_generation.completed","b64_json":"` + b64 + `","output_format":"png"}`)
	c := newImageStorageTestContext()

	require.Equal(t, partial, svc.rewriteBuiltImageStreamPayloadForDelivery(context.Background(), c, "image_generation.partial_image", partial))
	require.Empty(t, storage.saved)

	out := svc.rewriteBuiltImageStreamPayloadForDelivery(context.Background(), c, "image_generation.completed", completed)
	require.Len(t, storage.saved, 1)
	require.Contains(t, gjson.GetBytes(out, "url").String(), "https://cdn.test/images/sync_")
	require.False(t, gjson.GetBytes(out, "b64_json").Exists())
	require.Equal(t, "png", gjson.GetBytes(out, "output_format").String())
}

func TestImageStorageResolverDisabledKeepsExistingResponse(t *testing.T) {
	svc := &OpenAIGatewayService{
		imageStorageResolver: func() (*ImageResultUploader, bool) {
			return nil, false
		},
	}
	body, err := json.Marshal(map[string]any{
		"data": []map[string]any{{"b64_json": "YWJj"}},
	})
	require.NoError(t, err)

	out := svc.rewriteImageResponseForDelivery(context.Background(), newImageStorageTestContext(), body)
	require.JSONEq(t, string(body), string(out))
}
