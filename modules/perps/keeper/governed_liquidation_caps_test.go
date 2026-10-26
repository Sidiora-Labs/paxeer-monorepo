package keeper_test

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestGovernedLiquidationCapsBoundScan(t *testing.T) {
	mark := usdAmount(60_000)
	e := newLiquidationEnv(t, mark)
	params, err := e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, params.LiquidationScanPerBlock, uint64(3))
	require.GreaterOrEqual(t, params.SlicesPerBlock, uint64(3))
	require.Greater(t, params.LiquidationsPerBlock, uint64(1))
	params.LiquidationsPerBlock = 1

	response, err := keeper.NewMsgServerImpl(e.k).UpdateParams(sdk.WrapSDKContext(e.ctx), &types.MsgUpdateParams{
		Authority: e.k.Authority(), Params: params,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.Equal(t, params, stored)
	require.Equal(t, uint64(1), stored.LiquidationsPerBlock)

	type candidate struct {
		holder     common.Address
		positionID uint256.Int
		collateral uint64
	}
	var candidates []candidate
	size := baseAmount(100_000)
	for i := uint64(0); i < 3; i++ {
		holder := e.address(100 + i)
		collateral := uint64(28_000_000) + i*500_000
		positionID := e.pair(1_000+i, holder, collateral, size, mark)
		candidates = append(candidates, candidate{holder, positionID, collateral})

		summary, err := e.k.AccountMargin(e.ctx, holder)
		require.NoError(t, err)
		require.Equal(t, sdk.NewIntWithDecimal(30, types.USDDecimals), summary.MaintenanceMargin)
		require.True(t, summary.Equity.LT(summary.MaintenanceMargin), "account %s must be below maintenance", holder)
		require.True(t, summary.Equity.MulRaw(int64(params.BackstopRatioDenominator)).GT(
			summary.MaintenanceMargin.MulRaw(int64(params.BackstopRatioNumerator))),
			"account %s must liquidate through the book above the backstop line", holder)
		account := e.readAccount(holder)
		require.True(t, e.ctx.KVStore(e.key).Has(types.MarginRatioIndexKey(account.MarginRatioBucket, holder)))
		if i > 0 {
			require.Greater(t, account.MarginRatioBucket, e.readAccount(candidates[i-1].holder).MarginRatioBucket)
		}
	}
	e.maker(2, mark, baseAmount(300_000))

	for block, expected := range candidates {
		e.ctx = e.ctx.WithBlockHeight(int64(block + 1))
		e.at(time.Duration(block) * time.Second)
		result, err := e.k.LiquidationScan(e.ctx)
		require.NoError(t, err)
		require.Empty(t, result.Failures)
		require.Equal(t, uint64(1), result.Liquidated, "block %d obeys the governed account cap", block+1)
		require.Equal(t, uint64(1), result.Visited, "the scan stops before visiting another account after reaching the cap")
		require.Equal(t, uint64(1), result.Slices)
		require.Equal(t, []types.EventRecord{liquidationEvent(expected.positionID, expected.holder, mark, common.Address{})},
			eventsNamed(result.Events, types.EventLiquidation))
		require.Len(t, eventsNamed(result.Events, types.EventFill), 1)

		for i, entry := range candidates {
			remaining, open := e.positionSize(entry.positionID)
			account := e.readAccount(entry.holder)
			if i <= block {
				require.False(t, open, "account %s has been liquidated", entry.holder)
				require.Zero(t, account.PositionCount)
				require.False(t, e.ctx.KVStore(e.key).Has(types.AccountMarketPositionKey(entry.holder, 0)))
				require.False(t, e.ctx.KVStore(e.key).Has(types.MarginRatioIndexKey(account.MarginRatioBucket, entry.holder)))
			} else {
				require.True(t, open, "account %s carries to a later block", entry.holder)
				require.Equal(t, size, remaining)
				require.Equal(t, uint64(1), account.PositionCount)
				require.Equal(t, *uint256.NewInt(entry.collateral), account.Collateral)
				require.True(t, e.ctx.KVStore(e.key).Has(types.MarginRatioIndexKey(account.MarginRatioBucket, entry.holder)))
			}
		}
	}
}
