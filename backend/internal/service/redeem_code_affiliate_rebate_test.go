package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type redeemCodeAffiliateCaptureRepo struct {
	redeemRejectRepo
	created []RedeemCode
	batched []RedeemCode
}

func (r *redeemCodeAffiliateCaptureRepo) Create(_ context.Context, code *RedeemCode) error {
	clone := *code
	r.created = append(r.created, clone)
	return nil
}

func (r *redeemCodeAffiliateCaptureRepo) CreateBatch(_ context.Context, codes []RedeemCode) error {
	r.batched = append(r.batched, codes...)
	return nil
}

func float64Pointer(value float64) *float64 {
	return &value
}

func TestNormalizeAffiliateRebateBaseAmount(t *testing.T) {
	tests := []struct {
		name   string
		code   RedeemCode
		want   *float64
		badReq bool
	}{
		{
			name: "positive balance defaults to face value",
			code: RedeemCode{Type: RedeemTypeBalance, Value: 278},
			want: float64Pointer(278),
		},
		{
			name: "positive balance accepts paid amount",
			code: RedeemCode{
				Type:                      RedeemTypeBalance,
				Value:                     278,
				AffiliateRebateBaseAmount: float64Pointer(200),
			},
			want: float64Pointer(200),
		},
		{
			name: "zero disables rebate for a gifted balance code",
			code: RedeemCode{
				Type:                      RedeemTypeBalance,
				Value:                     278,
				AffiliateRebateBaseAmount: float64Pointer(0),
			},
			want: float64Pointer(0),
		},
		{
			name: "rebate amount cannot exceed face value",
			code: RedeemCode{
				Type:                      RedeemTypeBalance,
				Value:                     278,
				AffiliateRebateBaseAmount: float64Pointer(278.01),
			},
			badReq: true,
		},
		{
			name: "non balance code cannot set a rebate amount",
			code: RedeemCode{
				Type:                      RedeemTypeConcurrency,
				Value:                     10,
				AffiliateRebateBaseAmount: float64Pointer(10),
			},
			badReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := normalizeAffiliateRebateBaseAmount(&tt.code)
			if tt.badReq {
				require.Error(t, err)
				require.True(t, infraerrors.IsBadRequest(err))
				require.Equal(t, "REDEEM_CODE_AFFILIATE_REBATE_BASE_INVALID", infraerrors.Reason(err))
				return
			}

			require.NoError(t, err)
			require.NotNil(t, tt.code.AffiliateRebateBaseAmount)
			require.InDelta(t, *tt.want, *tt.code.AffiliateRebateBaseAmount, 1e-9)
		})
	}
}

func TestRedeemAffiliateRebateBaseAmountFallsBackForLegacyCode(t *testing.T) {
	require.InDelta(t, 278, redeemAffiliateRebateBaseAmount(&RedeemCode{Value: 278}), 1e-9)
	require.InDelta(t, 200, redeemAffiliateRebateBaseAmount(&RedeemCode{
		Value:                     278,
		AffiliateRebateBaseAmount: float64Pointer(200),
	}), 1e-9)
	require.InDelta(t, 0, redeemAffiliateRebateBaseAmount(&RedeemCode{
		Value:                     278,
		AffiliateRebateBaseAmount: float64Pointer(0),
	}), 1e-9)
}

func TestGenerateRedeemCodesPersistsAffiliateRebateBaseAmount(t *testing.T) {
	repo := &redeemCodeAffiliateCaptureRepo{}
	service := &adminServiceImpl{redeemCodeRepo: repo}
	base := 200.0

	codes, err := service.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:                     2,
		Type:                      RedeemTypeBalance,
		Value:                     278,
		AffiliateRebateBaseAmount: &base,
	})

	require.NoError(t, err)
	require.Len(t, codes, 2)
	require.Len(t, repo.created, 2)
	for _, code := range codes {
		require.NotNil(t, code.AffiliateRebateBaseAmount)
		require.InDelta(t, 200, *code.AffiliateRebateBaseAmount, 1e-9)
	}
}

func TestCreateCodeDefaultsAffiliateRebateBaseAmount(t *testing.T) {
	repo := &redeemCodeAffiliateCaptureRepo{}
	service := NewRedeemService(repo, nil, nil, nil, nil, nil, nil, nil)

	err := service.CreateCode(context.Background(), &RedeemCode{
		Code:   "COST-DEFAULT-001",
		Type:   RedeemTypeBalance,
		Value:  278,
		Status: StatusUnused,
	})

	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	require.NotNil(t, repo.created[0].AffiliateRebateBaseAmount)
	require.InDelta(t, 278, *repo.created[0].AffiliateRebateBaseAmount, 1e-9)
}
