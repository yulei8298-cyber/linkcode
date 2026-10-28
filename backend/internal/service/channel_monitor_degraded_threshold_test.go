//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 绿色/黄色以 monitorDegradedThreshold 为界：低于阈值 operational，达到阈值即 degraded；
// 红色（error/failed）由超时、非 2xx 或答案错误触发，不在本函数内判定。
func TestFinalizeOperationalOrDegradedUsesTenSecondThreshold(t *testing.T) {
	require.Equal(t, 10*time.Second, monitorDegradedThreshold)
	require.Less(t, monitorDegradedThreshold, monitorRequestTimeout)

	cases := []struct {
		name    string
		latency time.Duration
		want    string
	}{
		{"快速返回", 800 * time.Millisecond, MonitorStatusOperational},
		{"原 6 秒阈值附近仍为绿色", 6 * time.Second, MonitorStatusOperational},
		{"9.999 秒仍为绿色", 9999 * time.Millisecond, MonitorStatusOperational},
		{"恰好 10 秒为黄色", 10 * time.Second, MonitorStatusDegraded},
		{"30 秒为黄色", 30 * time.Second, MonitorStatusDegraded},
		{"49 秒仍为黄色", 49 * time.Second, MonitorStatusDegraded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ms := int(tc.latency / time.Millisecond)
			res := finalizeOperationalOrDegraded(&CheckResult{Status: MonitorStatusError}, tc.latency, ms)
			require.Equal(t, tc.want, res.Status)
			if tc.want == MonitorStatusDegraded {
				require.Contains(t, res.Message, "slow response")
			} else {
				require.Empty(t, res.Message)
			}
		})
	}
}

// 红色（超时）的界线由请求总超时与响应头超时共同决定，二者保持一致且大于黄色阈值。
func TestMonitorTimeoutBoundariesAreFiftySeconds(t *testing.T) {
	require.Equal(t, 50*time.Second, monitorRequestTimeout)
	require.Equal(t, monitorRequestTimeout, monitorResponseHeaderTimeout)
	require.Less(t, monitorDegradedThreshold, monitorResponseHeaderTimeout)
	require.Equal(t, monitorRequestTimeout, newSSRFSafeHTTPClient(monitorRequestTimeout).Timeout)
}
