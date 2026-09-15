package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	intelCheckDrawingJudgeVersion   = "structure_motion_v3"
	intelCheckMotionResponseMaxSize = 64 * 1024
)

// intelCheckDrawingMotionContract 会追加到真实下发题面和题面快照。
// 标记必须挂在实际可见部件上；验收器不采信产物自行上报的结论。
const intelCheckDrawingMotionContract = `

【自动动作验收协议】
请在主 SVG 的实际可见部件上各添加且只添加一个 data-intel-part 属性，值必须完整包含：
wheel-rear、wheel-front、crank-center、pedal-left、pedal-right、foot-left、foot-right、leg-left、leg-right。
wheel-* 标记应挂在会旋转的完整车轮或辐条组上；crank-center 标记应挂在曲柄轴心的可见圆形部件上；pedal-*、foot-*、leg-* 必须挂在对应的真实可见部件上。
不得用隐藏、透明、移出画布或与画作脱离的占位元素伪造标记。动画必须持续运行，双脚应分别跟随对应踏板，两个踏板围绕同一曲柄轴心反相运动。请输出自包含的单文件 HTML。`

type IntelCheckMotionCheck struct {
	Item   string  `json:"item"`
	Pass   bool    `json:"pass"`
	Value  float64 `json:"value,omitempty"`
	Limit  float64 `json:"limit,omitempty"`
	Detail string  `json:"detail"`
}

// IntelCheckMotionEvaluation 是 sidecar 返回的公开安全测量结果。
// 不包含源码、浏览器日志、内部 URL 或堆栈，可直接写入 judge_detail。
type IntelCheckMotionEvaluation struct {
	Version    string                  `json:"version"`
	Verifiable bool                    `json:"verifiable"`
	Pass       bool                    `json:"pass"`
	Reason     string                  `json:"reason"`
	Checks     []IntelCheckMotionCheck `json:"checks"`
}

type IntelCheckMotionEvaluator interface {
	Evaluate(ctx context.Context, source string) (*IntelCheckMotionEvaluation, error)
}

type httpIntelCheckMotionEvaluator struct {
	endpoint string
	client   *http.Client
}

func NewIntelCheckMotionEvaluator(cfg config.IntelCheckMotionConfig) IntelCheckMotionEvaluator {
	endpoint := strings.TrimSpace(cfg.EvaluatorURL)
	if !cfg.Enabled || endpoint == "" {
		return nil
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 8 * time.Second
	}
	return &httpIntelCheckMotionEvaluator{
		endpoint: strings.TrimRight(endpoint, "/") + "/evaluate",
		client:   &http.Client{Timeout: timeout},
	}
}

func (e *httpIntelCheckMotionEvaluator) Evaluate(ctx context.Context, source string) (*IntelCheckMotionEvaluation, error) {
	body, err := json.Marshal(map[string]string{"html": source})
	if err != nil {
		return nil, fmt.Errorf("编码动作验收请求失败：%w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("构造动作验收请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("动作验收器请求失败：%w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, intelCheckMotionResponseMaxSize+1))
	if err != nil {
		return nil, fmt.Errorf("读取动作验收结果失败：%w", err)
	}
	if len(payload) > intelCheckMotionResponseMaxSize {
		return nil, fmt.Errorf("动作验收结果超过 %d 字节上限", intelCheckMotionResponseMaxSize)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("动作验收器返回 HTTP %d", resp.StatusCode)
	}
	var result IntelCheckMotionEvaluation
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("解析动作验收结果失败：%w", err)
	}
	if strings.TrimSpace(result.Version) == "" || strings.TrimSpace(result.Reason) == "" {
		return nil, fmt.Errorf("动作验收结果缺少版本或说明")
	}
	if result.Checks == nil {
		result.Checks = []IntelCheckMotionCheck{}
	}
	return &result, nil
}

func intelCheckEffectivePrompt(question *IntelCheckQuestion) string {
	if question == nil {
		return ""
	}
	if question.Kind != IntelCheckKindDrawing {
		return question.Prompt
	}
	return strings.TrimSpace(question.Prompt) + intelCheckDrawingMotionContract
}
