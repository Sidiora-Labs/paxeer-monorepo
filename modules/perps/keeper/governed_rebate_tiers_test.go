package keeper

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

type RebateTiers []types.RebateTier

func TestGovernedRebateTiersChangeMakerRebate(t *testing.T) {
	e := newMatchEnv(t, matchFixed(t, "60000"))
	server := NewMsgServerImpl(e.k)
	params := types.DefaultParams()
	response, err := server.UpdateParams(sdk.WrapSDKContext(e.ctx), &types.MsgUpdateParams{
		Authority: e.k.Authority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, response)

	e.mustPlace(t, limitOrder(t, matchB, false, types.TimeInForceGTC, "60000", "0.01"))
	seed := e.mustPlace(t, marketOrder(t, matchA, true, "60000", "0.01"))
	require.Len(t, seed.Fills, 1)
	require.Equal(t, matchB, seed.Fills[0].Maker)
	e.mustPlace(t, limitOrder(t, matchC, false, types.TimeInForceGTC, "60000", "0.03"))
	other := e.mustPlace(t, marketOrder(t, matchD, true, "60000", "0.03"))
	require.Len(t, other.Fills, 1)
	require.Equal(t, matchC, other.Fills[0].Maker)

	resting := e.mustPlace(t, limitOrder(t, matchB, false, types.TimeInForceGTC, "60000", "0.02"))
	require.True(t, resting.Rested)
	require.Empty(t, resting.Fills)
	require.Equal(t, types.OrderStatusResting, resting.Taker.Status)

	oldTier, err := e.k.FeeTier(e.ctx, matchB)
	require.NoError(t, err)
	require.Equal(t, int64(-30), oldTier.MakerFeePpm)
	beforeOldFill := e.account(t, e.ctx, matchB).Collateral
	oldFill := e.mustPlace(t, marketOrder(t, matchA, true, "60000", "0.01"))
	require.Len(t, oldFill.Fills, 1)
	require.Equal(t, resting.Taker.ID, oldFill.Fills[0].MakerOrderID)
	require.Equal(t, matchFixed(t, "600"), oldFill.Fills[0].SizeUsd)
	require.Equal(t, "-18000000000000000", oldFill.Fills[0].MakerFeeUsd.String())
	afterOldFill := e.account(t, e.ctx, matchB).Collateral
	require.Equal(t, big.NewInt(18_000), new(big.Int).Sub(afterOldFill.ToBig(), beforeOldFill.ToBig()))
	partiallyFilled := e.order(t, resting.Taker.ID)
	require.Equal(t, types.OrderStatusResting, partiallyFilled.Status)
	require.Equal(t, matchFixed(t, "0.01"), partiallyFilled.FilledBase)

	oldTier, err = e.k.FeeTier(e.ctx, matchB)
	require.NoError(t, err)
	require.Equal(t, int64(-30), oldTier.MakerFeePpm)
	require.Equal(t, matchFixed(t, "1200"), oldTier.MakerVolume)
	require.Equal(t, matchFixed(t, "3000"), oldTier.VenueVolume)
	params.RebateTiers = RebateTiers{
		{MinMakerSharePpm: 100_000, MakerFeePpm: -40},
		{MinMakerSharePpm: 300_000, MakerFeePpm: -75},
		{MinMakerSharePpm: 500_000, MakerFeePpm: -100},
	}
	beforeUpdate := e.account(t, e.ctx, matchB).Collateral
	response, err = server.UpdateParams(sdk.WrapSDKContext(e.ctx), &types.MsgUpdateParams{
		Authority: e.k.Authority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.Equal(t, params, stored)
	require.Equal(t, beforeUpdate, e.account(t, e.ctx, matchB).Collateral)
	require.Equal(t, partiallyFilled, e.order(t, resting.Taker.ID))

	newTier, err := e.k.FeeTier(e.ctx, matchB)
	require.NoError(t, err)
	require.Equal(t, int64(-75), newTier.MakerFeePpm)
	require.Equal(t, oldTier.MakerVolume, newTier.MakerVolume)
	require.Equal(t, oldTier.VenueVolume, newTier.VenueVolume)
	require.Equal(t, oldTier.TakerFeePpm, newTier.TakerFeePpm)
	feesBefore := e.account(t, e.ctx, feesAccountAddress()).Collateral
	result := e.mustPlace(t, marketOrder(t, matchA, true, "60000", "0.01"))
	require.Len(t, result.Fills, 1)
	require.Equal(t, types.OrderStatusFilled, result.Taker.Status)
	fill := result.Fills[0]
	require.Equal(t, resting.Taker.ID, fill.MakerOrderID)
	require.Equal(t, matchB, fill.Maker)
	require.Equal(t, matchA, fill.Taker)
	require.Equal(t, matchFixed(t, "60000"), fill.Price)
	require.Equal(t, matchFixed(t, "0.01"), fill.SizeBase)
	require.Equal(t, matchFixed(t, "600"), fill.SizeUsd)
	require.Equal(t, "-45000000000000000", fill.MakerFeeUsd.String())
	require.Equal(t, matchFixed(t, "0.27"), fill.TakerFeeUsd)
	afterNewFill := e.account(t, e.ctx, matchB).Collateral
	require.Equal(t, big.NewInt(45_000), new(big.Int).Sub(afterNewFill.ToBig(), beforeUpdate.ToBig()))
	feesAfter := e.account(t, e.ctx, feesAccountAddress()).Collateral
	require.Equal(t, big.NewInt(225_000), new(big.Int).Sub(feesAfter.ToBig(), feesBefore.ToBig()))
	requireReleased(t, e, resting.Taker.ID, types.OrderStatusFilled, "0.02")
	require.Empty(t, e.bookIDs(t, types.SideSell))

	var rebates []types.EventRecord
	for _, event := range result.Events {
		if event.Name == types.EventFeeCollected && event.Args[3] == FeeTypeMakerRebate {
			rebates = append(rebates, event)
		}
	}
	require.Len(t, rebates, 1)
	require.Equal(t, feeCollectedArgs(fill.MakerPositionID.Uint64(), matchB, e.market.ID,
		FeeTypeMakerRebate, "45000000000000000", "45000"), argStrings(rebates[0].Args))
}
