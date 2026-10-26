package keeper_test

import (
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestSetPausedRequiresAuthority(t *testing.T) {
	k, ctx, key := newKeeper(t)
	require.Equal(t, govAuthority(), k.Authority())
	store := ctx.KVStore(key)
	signers := []string{
		"",
		"invalid-authority",
		sdk.AccAddress(common.HexToAddress("0x00000000000000000000000000000000000b0313").Bytes()).String(),
		k.ModuleAddress().String(),
	}
	for _, signer := range signers {
		requireUnauthorized(t, k.SetPaused(ctx, signer, true))
		require.Nil(t, store.Get(types.ParamsKey))
	}
	for _, paused := range []bool{true, false} {
		require.NoError(t, k.SetPaused(ctx, govAuthority(), paused))
		params, err := k.GetParams(ctx)
		require.NoError(t, err)
		require.Equal(t, paused, params.Paused)
		before := append([]byte(nil), store.Get(types.ParamsKey)...)
		for _, signer := range signers {
			requireUnauthorized(t, k.SetPaused(ctx, signer, !paused))
			require.Equal(t, before, store.Get(types.ParamsKey))
		}
	}

	t.Run("corrupt_params", func(t *testing.T) {
		corrupt := []byte("invalid JSON")
		store.Set(types.ParamsKey, corrupt)
		requireUnauthorized(t, k.SetPaused(ctx, signers[0], true))
		require.ErrorIs(t, k.SetPaused(ctx, govAuthority(), true), types.ErrInvalidParams)
		require.Equal(t, corrupt, store.Get(types.ParamsKey))
	})
}

func TestSetPausedKeepsOtherParams(t *testing.T) {
	k, ctx, key := newKeeper(t)
	params := types.DefaultParams()
	params.CollateralToken = common.HexToAddress("0x00000000000000000000000000000000000c0313")
	params.FeeTiers[0].TakerFeePpm = 600
	params.FeeTiers[0].MakerFeePpm = 200
	params.RebateTiers[0].MakerFeePpm = -11
	params.InsuranceSharePpm = 100_000
	params.FundingInterestPpm8h = 101
	params.FundingPremiumClampPpm = 501
	params.FundingCapPpmPerHour = 40_001
	params.PremiumIntervalSeconds = 6
	params.MarkEmaSeconds = 151
	params.ClassEOiCaps[0].CapUsd = sdk.NewIntWithDecimal(3_000_000, types.USDDecimals)
	params.WithdrawFloorPpm = 100_001
	params.LiquidationSliceThresholdUsd = sdk.NewIntWithDecimal(100_001, types.USDDecimals)
	params.LiquidationSlicePpm = 200_001
	params.LiquidationCooldownSeconds = 31
	params.BackstopRatioNumerator = 3
	params.BackstopRatioDenominator = 4
	params.SweepPositionsPerBlock = 513
	params.LiquidationScanPerBlock = 1_025
	params.LiquidationsPerBlock = 65
	params.SlicesPerBlock = 257
	params.AdlPositionsPerBlock = 65
	params.TriggersPerBlock = 257
	params.ExpiriesPerBlock = 257
	params.MinOrderValueUsd = sdk.NewIntWithDecimal(11, types.USDDecimals)
	params.MarketOrderMaxValues[0].MaxValueUsd = sdk.NewIntWithDecimal(500_001, types.USDDecimals)
	params.LimitOrderValueMultiple = 11
	params.DefaultSlippagePpm = 5_001
	params.MaxOpenOrdersPerAccount = 1_001
	params.MaxFillsPerOrder = 33
	require.NoError(t, k.UpdateParams(ctx, govAuthority(), params))
	store := ctx.KVStore(key)
	var before map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(store.Get(types.ParamsKey), &before))
	delete(before, "paused")

	for _, paused := range []bool{true, true, false, false, true} {
		require.NoError(t, k.SetPaused(ctx, govAuthority(), paused))
		stored, err := k.GetParams(ctx)
		require.NoError(t, err)
		require.Equal(t, paused, stored.Paused)
		params.Paused = paused
		expected, err := json.Marshal(params)
		require.NoError(t, err)
		require.Equal(t, expected, store.Get(types.ParamsKey))

		var after map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(store.Get(types.ParamsKey), &after))
		delete(after, "paused")
		require.Equal(t, before, after)

		for _, method := range []string{
			"closePosition", "partialClose", "closePositionWithParams", "cancelOrder",
			"cancelByCloid", "deposit", "addCollateral", "topUpPosition",
		} {
			require.NoError(t, k.CheckPause(ctx, method), method)
		}
		if paused {
			require.ErrorIs(t, k.CheckPause(ctx, "placeOrder"), types.ErrPaused)
		} else {
			require.NoError(t, k.CheckPause(ctx, "placeOrder"))
		}
	}
}
