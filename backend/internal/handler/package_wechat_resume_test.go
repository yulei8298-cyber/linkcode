package handler

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestParseWeChatPaymentNoticeVersion(t *testing.T) {
	require.Equal(t, 3, parseWeChatPaymentNoticeVersion(" 3 "))
	require.Zero(t, parseWeChatPaymentNoticeVersion(""))
	require.Zero(t, parseWeChatPaymentNoticeVersion("abc"))
	require.Zero(t, parseWeChatPaymentNoticeVersion("-2"))
}

func TestApplyWeChatPaymentResumeClaims_CarriesPackageNoticeVersion(t *testing.T) {
	req := CreateOrderRequest{PaymentType: "wxpay"}
	err := applyWeChatPaymentResumeClaims(&req, &service.WeChatPaymentResumeClaims{
		OpenID:               "openid-1",
		PaymentType:          "wxpay",
		OrderType:            "package",
		PlanID:               9,
		PackageNoticeVersion: 4,
	})
	require.NoError(t, err)
	require.Equal(t, "package", req.OrderType)
	require.Equal(t, int64(9), req.PlanID)
	require.Equal(t, 4, req.PackageNoticeVersion, "微信授权回跳后保留已同意的须知版本")
}
