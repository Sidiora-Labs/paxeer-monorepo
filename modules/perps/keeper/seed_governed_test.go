package keeper_test

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestSeedV6120IdempotentAfterGovernance(t *testing.T) {
	k, ctx, _, _ := newMsgMarketKeeper(t)
	require.NoError(t, keeper.SeedV6120(ctx, *k))

	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, types.DefaultParams(), params)
	eth, err := k.GetMarket(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "ETH", eth.Symbol)
	require.True(t, eth.Enabled)

	server := keeper.NewMsgServerImpl(k)
	goCtx := sdk.WrapSDKContext(ctx)
	eth.Enabled = false
	marketResponse, err := server.SetMarket(goCtx, &types.MsgSetMarket{
		Authority: k.Authority(), Market: eth,
	})
	require.NoError(t, err)
	require.NotNil(t, marketResponse)

	seededCap := params.FundingCapPpmPerHour
	params.FundingCapPpmPerHour = 80_000
	require.Greater(t, params.FundingCapPpmPerHour, seededCap)
	paramsResponse, err := server.UpdateParams(goCtx, &types.MsgUpdateParams{
		Authority: k.Authority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, paramsResponse)

	governedParams, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, params, governedParams)
	governedETH, err := k.GetMarket(ctx, eth.ID)
	require.NoError(t, err)
	require.Equal(t, eth, governedETH)

	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithBlockTime(ctx.BlockTime().Add(time.Hour))
	require.NoError(t, keeper.SeedV6120(ctx, *k))

	reseededParams, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, governedParams, reseededParams)
	require.Equal(t, uint64(80_000), reseededParams.FundingCapPpmPerHour)
	reseededETH, err := k.GetMarket(ctx, eth.ID)
	require.NoError(t, err)
	require.Equal(t, governedETH, reseededETH)
	require.False(t, reseededETH.Enabled)
}
