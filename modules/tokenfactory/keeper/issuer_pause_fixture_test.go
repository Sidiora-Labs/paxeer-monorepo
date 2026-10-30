package keeper_test

import (
	"testing"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestIssuerPauseFixtureParity(t *testing.T) {
	pause := denomPauseReadReceipt(t, "supply-pause.receipt", denomPauseKernelPause)
	unpause := denomPauseReadReceipt(t, "supply-unpause.receipt", denomPauseKernelUnpause)
	require.Equal(t, byte(2), pause.operation)
	require.Equal(t, byte(3), unpause.operation)
	for _, receipt := range []denomPauseReceipt{pause, unpause} {
		require.Zero(t, receipt.amount.Sign())
		require.True(t, receipt.supplyStart.IsInt64())
		require.True(t, receipt.supplyEnd.IsInt64())
		require.Equal(t, int64(1_000_000), receipt.supplyStart.Int64())
		require.Zero(t, receipt.supplyStart.Cmp(receipt.supplyEnd))
	}

	s := denomPauseSuite(t)
	k := s.App.TokenFactoryKeeper
	issuerDid := adminsIssuerDid(0xab)
	recipient := s.TestAccs[1]
	require.NoError(t, k.SetIssuerDid(s.Ctx, s.defaultDenom, issuerDid))
	require.NoError(t, k.MintByIssuer(s.Ctx, issuerDid, sdk.NewInt64Coin(s.defaultDenom, 1_000_000), recipient.String()))
	require.False(t, k.IsDenomPaused(s.Ctx, s.defaultDenom))

	for _, receipt := range []denomPauseReceipt{pause, unpause} {
		paused := receipt.operation == denomPauseKernelPause
		require.Equal(t, receipt.supplyStart.Int64(), denomPauseSupply(s))
		require.NoError(t, k.SetDenomPausedByIssuer(s.Ctx, s.defaultDenom, issuerDid, paused))
		require.Equal(t, paused, k.IsDenomPaused(s.Ctx, s.defaultDenom))
		require.Equal(t, receipt.supplyEnd.Int64(), denomPauseSupply(s))
		require.Equal(t, receipt.supplyEnd.Int64(), s.App.BankKeeper.GetBalance(s.Ctx, recipient, s.defaultDenom).Amount.Int64())
	}
}
