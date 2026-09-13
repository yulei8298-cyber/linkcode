package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 公开接口在开关关闭时必须是 404，而不是 200 加空数据（设计文档 §4.5）。
//
// 这条不能只在 service 层验：service 返回的是 ErrIntelCheckDisabled，
// 它经 infraerrors.NotFound 构造、由 response.ErrorFrom 翻成 HTTP 状态码，
// 中间这段映射断了的话，service 测试全绿而前端路由守卫会收到 500 并把
// 「功能没开」显示成「服务故障」。

// intelCheckPanicRepo 一个「碰即炸」的仓储。
//
// 嵌入接口而不逐个实现 25 个方法：这里要证明的恰恰是「关闭状态下一次库都不查」，
// 而嵌入的 nil 接口在任何方法被调用时都会 panic——比写 25 个返回零值的桩
// 更精确地表达了这个断言，桩返回零值反而会让误查静默通过。
type intelCheckPanicRepo struct {
	service.IntelCheckRepository
}

// intelCheckDisabledSettingStore 恒返回「开关关闭」的设置。
//
// 返回空串即可：service 读到空串解析失败后回落默认设置，而默认 Enabled 为 false
// （DefaultIntelCheckSettings 刻意如此——功能对外可见，配置完成前不应自动开放）。
type intelCheckDisabledSettingStore struct{}

func (intelCheckDisabledSettingStore) GetValue(context.Context, string) (string, error) {
	return "", service.ErrSettingNotFound
}

func (intelCheckDisabledSettingStore) Set(context.Context, string, string) error { return nil }

// newDisabledIntelCheckRouter 装一个「功能未开启」的公开路由。
func newDisabledIntelCheckRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	svc := service.NewIntelCheckService(
		&intelCheckPanicRepo{}, intelCheckDisabledSettingStore{}, nil)
	h := NewIntelCheckHandler(svc)

	router := gin.New()
	router.GET("/api/v1/public/intel-check/overview", h.Overview)
	router.GET("/api/v1/public/intel-check/results/:id", h.ResultDetail)
	return router
}

func doIntelCheckGet(router *gin.Engine, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestIntelCheckOverview_开关关闭返回404且不查库(t *testing.T) {
	router := newDisabledIntelCheckRouter()

	// 不 panic 本身就是断言：仓储是碰即炸的，走到任何一次查询都会在这里炸掉。
	require.NotPanics(t, func() {
		rec := doIntelCheckGet(router, "/api/v1/public/intel-check/overview")
		// 404 而非 403：未开启时不应让外部探知该功能是否存在，
		// 403 等于承认「有但不给你看」。
		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestIntelCheckResultDetail_开关关闭返回404且不查库(t *testing.T) {
	router := newDisabledIntelCheckRouter()

	require.NotPanics(t, func() {
		rec := doIntelCheckGet(router, "/api/v1/public/intel-check/results/42")
		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestIntelCheckResultDetail_开关关闭时响应不泄露任何内部信息(t *testing.T) {
	rec := doIntelCheckGet(newDisabledIntelCheckRouter(), "/api/v1/public/intel-check/results/42")

	body := rec.Body.String()
	// 关闭状态下的 404 是未登录访客最容易拿到的一个响应，
	// 它不该透露功能内部的任何命名细节。
	require.NotContains(t, body, "base_url")
	require.NotContains(t, body, "api_key")
	require.NotContains(t, body, "judge_detail")
	require.NotContains(t, body, "prompt_snapshot")

	// 响应是合法 JSON 而非半截内容：错误路径同样会被前端 parse。
	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
}

func TestIntelCheckResultDetail_非法id返回400而非404(t *testing.T) {
	router := newDisabledIntelCheckRouter()

	for _, id := range []string{"abc", "0", "-1", "1.5"} {
		t.Run(id, func(t *testing.T) {
			rec := doIntelCheckGet(router, "/api/v1/public/intel-check/results/"+id)
			// 参数格式错误是调用方的问题，与功能开关无关，必须能区分开：
			// 一律回 404 会让前端无法判断是「记录不存在」还是「id 拼错了」。
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}
