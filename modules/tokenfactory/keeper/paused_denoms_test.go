package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestGetPausedDenomsSorted(t *testing.T) {
	s := denomPauseSuite(t)
	k := s.App.TokenFactoryKeeper
	admin := s.TestAccs[0].String()
	ctx := sdk.WrapSDKContext(s.Ctx)

	require.Empty(t, k.GetPausedDenoms(s.Ctx))

	denoms := make(map[string]string)
	for _, subdenom := range []string{"zeta", "a0", "Zeta", "a"} {
		response, err := s.msgServer.CreateDenom(ctx, types.NewMsgCreateDenom(admin, subdenom))
		require.NoError(t, err)
		denom := response.GetNewTokenDenom()
		denoms[subdenom] = denom
		_, err = s.msgServer.SetDenomPaused(ctx, types.NewMsgSetDenomPaused(admin, denom, true))
		require.NoError(t, err)
		require.True(t, k.IsDenomPaused(s.Ctx, denom))
	}

	require.Equal(t, []string{denoms["Zeta"], denoms["a"], denoms["a0"], denoms["zeta"]}, k.GetPausedDenoms(s.Ctx))
	require.False(t, k.IsDenomPaused(s.Ctx, s.defaultDenom))

	_, err := s.msgServer.SetDenomPaused(ctx, types.NewMsgSetDenomPaused(admin, denoms["a"], false))
	require.NoError(t, err)
	require.False(t, k.IsDenomPaused(s.Ctx, denoms["a"]))
	require.Equal(t, []string{denoms["Zeta"], denoms["a0"], denoms["zeta"]}, k.GetPausedDenoms(s.Ctx))

	for _, subdenom := range []string{"Zeta", "a0", "zeta"} {
		_, err := s.msgServer.SetDenomPaused(ctx, types.NewMsgSetDenomPaused(admin, denoms[subdenom], false))
		require.NoError(t, err)
	}
	require.Empty(t, k.GetPausedDenoms(s.Ctx))
}
