package keeper_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func OpenInterestCap(t *testing.T, k *keeper.Keeper, ctx sdk.Context, marketID uint64, want sdk.Int) {
	t.Helper()
	capUsd, err := k.OpenInterestCap(ctx, marketID)
	require.NoError(t, err)
	require.Equal(t, want, capUsd)
}

func CheckOpenInterest(t *testing.T, k *keeper.Keeper, ctx sdk.Context, fill keeper.OpenInterestFill, wantErr error) *types.EventRecord {
	t.Helper()
	record, err := k.CheckOpenInterest(ctx, fill)
	if wantErr != nil {
		require.ErrorIs(t, err, wantErr)
		require.Nil(t, record)
	} else {
		require.NoError(t, err)
		require.NotNil(t, record)
	}
	return record
}

func TestGovernedOiCapAdmitsLargerOpen(t *testing.T) {
	k, ctx, key, _ := newMsgMarketKeeper(t)
	require.NoError(t, keeper.SeedV6120(ctx, *k))
	market, err := k.GetMarket(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "BTC", market.Symbol)
	oldCap := sdk.NewIntWithDecimal(500_000_000, types.USDDecimals)
	require.Equal(t, oldCap, market.OiCapUsd)
	OpenInterestCap(t, k, ctx, market.ID, oldCap)

	state := types.MarketState{
		MarketID:          market.ID,
		MarkPrice:         genesisE18(50_000),
		LongOpenInterest:  genesisE18(10_000),
		ShortOpenInterest: genesisE18(10_000),
	}
	k.SetMarketState(ctx, state)
	before := append([]byte(nil), ctx.KVStore(key).Get(types.MarketStateKey(market.ID))...)
	oiBefore, err := k.OpenInterest(ctx, market.ID)
	require.NoError(t, err)
	require.Equal(t, oldCap.BigInt(), oiBefore.LongUsd.ToBig())
	require.Equal(t, oldCap.BigInt(), oiBefore.ShortUsd.ToBig())
	fill := keeper.OpenInterestFill{
		MarketID:        market.ID,
		SizeBase:        genesisE18(1_000),
		PriceUsd:        genesisE18(49_000),
		BuyerOpensBase:  genesisE18(1_000),
		SellerOpensBase: genesisE18(1_000),
	}
	CheckOpenInterest(t, k, ctx, fill, types.ErrOpenInterestCap)
	require.Equal(t, before, ctx.KVStore(key).Get(types.MarketStateKey(market.ID)))

	raisedCap := sdk.NewIntWithDecimal(550_000_000, types.USDDecimals)
	market.OiCapUsd = raisedCap
	response, err := keeper.NewMsgServerImpl(k).SetMarket(sdk.WrapSDKContext(ctx), &types.MsgSetMarket{
		Authority: govAuthority(), Market: market,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := k.GetMarket(ctx, market.ID)
	require.NoError(t, err)
	require.Equal(t, market, stored)
	OpenInterestCap(t, k, ctx, market.ID, raisedCap)
	require.Equal(t, before, ctx.KVStore(key).Get(types.MarketStateKey(market.ID)))

	record := CheckOpenInterest(t, k, ctx, fill, nil)
	require.Equal(t, &types.EventRecord{
		Name: types.EventOpenInterestChanged,
		Args: []any{
			new(big.Int).SetUint64(market.ID), raisedCap.BigInt(), raisedCap.BigInt(),
			sdk.NewIntWithDecimal(49_000_000, types.USDDecimals).BigInt(), true,
		},
	}, record)
	state.LongOpenInterest = genesisE18(11_000)
	state.ShortOpenInterest = genesisE18(11_000)
	after, err := k.GetMarketState(ctx, market.ID)
	require.NoError(t, err)
	require.Equal(t, state, after)
	oiAfter, err := k.OpenInterest(ctx, market.ID)
	require.NoError(t, err)
	require.Equal(t, state.LongOpenInterest, oiAfter.LongBase)
	require.Equal(t, state.ShortOpenInterest, oiAfter.ShortBase)
	require.Equal(t, raisedCap.BigInt(), oiAfter.LongUsd.ToBig())
	require.Equal(t, raisedCap.BigInt(), oiAfter.ShortUsd.ToBig())
}
