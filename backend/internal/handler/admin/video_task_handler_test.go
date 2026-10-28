//go:build unit

package admin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/stretchr/testify/require"
)

func TestVideoTaskAdminViewHidesUpstreamURL(t *testing.T) {
	videos := service.NewVideoGatewayService(nil, nil, nil, nil, nil, &config.Config{JWT: config.JWTConfig{Secret: "s"}})
	h := NewVideoTaskHandler(videos)
	done := &service.VideoTask{TaskID: "vt_done", Status: video.StatusSucceeded, VideoURL: "https://relay.example/v1/videos/x/content"}

	body, err := json.Marshal(h.view(done, time.Now()))
	require.NoError(t, err)
	require.NotContains(t, string(body), "relay.example", "管理端接口不能返回上游成品地址")
	require.NotContains(t, string(body), "video_url")

	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out))
	require.Contains(t, out["content_url"], "/video-files/vt_done?expires=")
	require.Equal(t, "vt_done", out["task_id"])

	running := &service.VideoTask{TaskID: "vt_run", Status: video.StatusRunning}
	body, err = json.Marshal(h.view(running, time.Now()))
	require.NoError(t, err)
	require.NotContains(t, string(body), "content_url", "未完成的任务没有成品链接")
}
