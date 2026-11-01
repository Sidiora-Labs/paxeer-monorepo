package keeper_test

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

type ClassEOiCaps []types.OiCapRow

func (caps ClassEOiCaps) OpenInterestCap(t *testing.T, k *keeper.Keeper, ctx sdk.Context, marketID uint64, row int) {
	t.Helper()
	capUsd, err := k.OpenInterestCap(ctx, marketID)
	require.NoError(t, err)
	require.Equal(t, caps[row].CapUsd, capUsd, "governed cap row %d at %s", row, ctx.BlockTime())
}

func TestGovernedClassEOiCapsApply(t *testing.T) {
	k, ctx, _, _ := newMsgMarketKeeper(t)
	server := keeper.NewMsgServerImpl(k)
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	caps := ClassEOiCaps{
		{MinAgeDays: 0, CapUsd: sdk.NewIntWithDecimal(1_000_000, types.USDDecimals)},
		{MinAgeDays: 2, CapUsd: sdk.NewIntWithDecimal(3_000_000, types.USDDecimals)},
		{MinAgeDays: 4, CapUsd: sdk.NewIntWithDecimal(6_000_000, types.USDDecimals)},
	}
	require.NotEqual(t, []types.OiCapRow(caps), params.ClassEOiCaps)
	params.ClassEOiCaps = []types.OiCapRow(caps)
	response, err := server.UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{
		Authority: k.Authority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	storedParams, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, params, storedParams)

	market := classEMarket(0, "BTC")
	market.OiCapUsd = sdk.ZeroInt()
	market.ListedAt = 1
	marketResponse, err := server.SetMarket(sdk.WrapSDKContext(ctx), &types.MsgSetMarket{
		Authority: k.Authority(), Market: market,
	})
	require.NoError(t, err)
	require.NotNil(t, marketResponse)
	storedMarket, err := k.GetMarket(ctx, market.ID)
	require.NoError(t, err)
	market.ListedAt = uint64(ctx.BlockTime().Unix())
	require.Equal(t, market, storedMarket)

	listedAt := ctx.BlockTime()
	day := time.Duration(types.SecondsPerDay) * time.Second
	for _, tc := range []struct {
		name string
		age  time.Duration
		row  int
	}{
		{name: "listing", age: 0, row: 0},
		{name: "before two days", age: 2*day - time.Second, row: 0},
		{name: "two days", age: 2 * day, row: 1},
		{name: "before four days", age: 4*day - time.Second, row: 1},
		{name: "four days", age: 4 * day, row: 2},
		{name: "past default final row", age: 180 * day, row: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx = ctx.WithBlockTime(listedAt.Add(tc.age))
			caps.OpenInterestCap(t, k, ctx, market.ID, tc.row)
			unchanged, err := k.GetMarket(ctx, market.ID)
			require.NoError(t, err)
			require.Equal(t, storedMarket, unchanged)
		})
	}
}
