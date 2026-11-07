package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

var _ types.IssuerKeeper = keeper.Keeper{}

func TestKeeperImplementsIssuerKeeper(t *testing.T) {
	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()

	var issuer types.IssuerKeeper = s.App.TokenFactoryKeeper
	ctx := s.Ctx
	creator := s.TestAccs[0]
	recipient := s.TestAccs[1]
	issuerDid := adminsIssuerDid(0xab)

	denom, err := issuer.CreateDenom(ctx, creator.String(), "issuer-interface")
	require.NoError(t, err)
	require.Equal(t, "factory/"+creator.String()+"/issuer-interface", denom)
	require.False(t, issuer.IsDenomPaused(ctx, denom))

	require.NoError(t, issuer.SetIssuerDid(ctx, denom, issuerDid))
	storedDid, err := issuer.IssuerDid(ctx, denom)
	require.NoError(t, err)
	require.Equal(t, issuerDid, storedDid)

	amount := sdk.NewInt64Coin(denom, 10)
	require.NoError(t, issuer.MintByIssuer(ctx, issuerDid, amount, recipient.String()))
	require.Equal(t, amount, s.App.BankKeeper.GetBalance(ctx, recipient, denom))
	require.Equal(t, amount, s.App.BankKeeper.GetSupply(ctx, denom))

	require.NoError(t, issuer.SetDenomPausedByIssuer(ctx, denom, issuerDid, true))
	require.True(t, issuer.IsDenomPaused(ctx, denom))
	require.ErrorIs(t, issuer.MintByIssuer(ctx, issuerDid, amount, recipient.String()), types.ErrDenomPaused)
	require.ErrorIs(t, issuer.BurnByIssuer(ctx, issuerDid, amount, recipient.String()), types.ErrDenomPaused)
	require.Equal(t, amount, s.App.BankKeeper.GetBalance(ctx, recipient, denom))
	require.Equal(t, amount, s.App.BankKeeper.GetSupply(ctx, denom))

	require.NoError(t, issuer.SetDenomPausedByIssuer(ctx, denom, issuerDid, false))
	require.False(t, issuer.IsDenomPaused(ctx, denom))
	require.NoError(t, issuer.BurnByIssuer(ctx, issuerDid, amount, recipient.String()))
	require.True(t, s.App.BankKeeper.GetBalance(ctx, recipient, denom).IsZero())
	require.True(t, s.App.BankKeeper.GetSupply(ctx, denom).IsZero())
}
