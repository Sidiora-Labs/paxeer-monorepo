package types_test

import (
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

type QuoteLedgerKind = types.QuoteLedgerKind

func TestParseQuoteLedgerKind(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  QuoteLedgerKind
	}{
		{"held", types.QuoteLedgerKind_QUOTE_LEDGER_KIND_HELD},
		{"fee-held", types.QuoteLedgerKind_QUOTE_LEDGER_KIND_FEE_HELD},
		{"treasury-held", types.QuoteLedgerKind_QUOTE_LEDGER_KIND_TREASURY_HELD},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := types.ParseQuoteLedgerKind(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}

	for _, input := range []string{
		"", "unknown", "unspecified", "HELD", "Fee-held", "TREASURY-HELD",
		" held", "held ", "fee-held\n", "treasury-held\t", "fee_held",
		"treasury_held", "fee", "treasury", "1", "2", "3",
		"QUOTE_LEDGER_KIND_HELD", "held\x00",
	} {
		t.Run("invalid/"+input, func(t *testing.T) {
			got, err := types.ParseQuoteLedgerKind(input)
			require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
			require.Equal(t, types.QuoteLedgerKind_QUOTE_LEDGER_KIND_UNSPECIFIED, got)
		})
	}
}

func TestFeeAccountingInfoOf(t *testing.T) {
	pending, ok := sdk.NewIntFromString("1234567890123456789012345678901234567890")
	require.True(t, ok)
	liabilities, ok := sdk.NewIntFromString("9876543210987654321098765432109876543210")
	require.True(t, ok)

	for _, tc := range []struct {
		name string
		in   types.FeeAccounting
		want types.FeeAccountingInfo
	}{
		{
			name: "zero value with nil collections",
			in:   types.FeeAccounting{},
			want: types.FeeAccountingInfo{},
		},
		{
			name: "initialized with empty collections",
			in: types.FeeAccounting{
				PendingProtocolFees:   sdk.ZeroInt(),
				TotalLiabilities:      sdk.ZeroInt(),
				AccountingInitialized: true,
				Pools:                 []types.PoolFeeAccount{},
				Epochs:                []types.PoolAirdropEpoch{},
				Claims:                []types.PoolAirdropClaim{},
			},
			want: types.FeeAccountingInfo{
				PendingProtocolFees:   sdk.ZeroInt(),
				TotalLiabilities:      sdk.ZeroInt(),
				AccountingInitialized: true,
			},
		},
		{
			name: "populated initialized accounting",
			in: types.FeeAccounting{
				PendingProtocolFees:       pending,
				TotalLiabilities:          liabilities,
				AccountingInitialized:     true,
				AccountingMigrationCursor: math.MaxUint64,
				Pools:                     make([]types.PoolFeeAccount, 2, 5),
				Epochs:                    make([]types.PoolAirdropEpoch, 3, 6),
				Claims:                    make([]types.PoolAirdropClaim, 4, 7),
			},
			want: types.FeeAccountingInfo{
				PendingProtocolFees:       pending,
				TotalLiabilities:          liabilities,
				AccountingInitialized:     true,
				AccountingMigrationCursor: math.MaxUint64,
				PoolCount:                 2,
				EpochCount:                3,
				ClaimCount:                4,
			},
		},
		{
			name: "migration in progress",
			in: types.FeeAccounting{
				PendingProtocolFees:       sdk.NewInt(17),
				TotalLiabilities:          sdk.NewInt(29),
				AccountingInitialized:     false,
				AccountingMigrationCursor: 11,
				Pools:                     make([]types.PoolFeeAccount, 1),
			},
			want: types.FeeAccountingInfo{
				PendingProtocolFees:       sdk.NewInt(17),
				TotalLiabilities:          sdk.NewInt(29),
				AccountingInitialized:     false,
				AccountingMigrationCursor: 11,
				PoolCount:                 1,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, types.FeeAccountingInfoOf(tc.in))
		})
	}
}
