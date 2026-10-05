package handler

import (
	"strconv"
	"strings"
)

// parseWeChatPaymentNoticeVersion 解析微信授权跳转携带的套餐购买须知版本，非法值视为 0（未同意）。
func parseWeChatPaymentNoticeVersion(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v < 0 {
		return 0
	}
	return v
}
