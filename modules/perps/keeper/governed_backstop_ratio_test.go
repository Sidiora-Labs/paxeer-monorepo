package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestGovernedBackstopRatioMovesBackstopLine(t *testing.T) {
	mark := insuranceUsd(60_000)
	e := newInsuranceEnv(t, mark)
	holder := e.address(1)
	positionID := e.pair(7, holder, true, 18_000_000, insuranceBase(100_000), mark)

	margin, err := e.k.AccountMargin(e.ctx, holder)
	require.NoError(t, err)
	require.Equal(t, sdk.NewIntWithDecimal(18, types.USDDecimals), margin.Equity)
	require.Equal(t, sdk.NewIntWithDecimal(30, types.USDDecimals), margin.MaintenanceMargin)
	require.Equal(t, margin.MaintenanceMargin.MulRaw(3), margin.Equity.MulRaw(5))

	defaults, err := e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(2), defaults.BackstopRatioNumerator)
	require.Equal(t, uint64(3), defaults.BackstopRatioDenominator)
	updated := defaults
	updated.BackstopRatioNumerator = 1
	updated.BackstopRatioDenominator = 2
	server := keeper.NewMsgServerImpl(e.k)
	response, err := server.UpdateParams(sdk.WrapSDKContext(e.ctx), &types.MsgUpdateParams{
		Authority: e.k.Authority(), Params: updated,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.Equal(t, updated, stored)

	events, err := e.k.Backstop(e.ctx, holder)
	require.ErrorIs(t, err, types.ErrNotLiquidatable)
	require.Empty(t, events)
	account := e.readAccount(holder)
	require.Equal(t, *uint256.NewInt(18_000_000), account.Collateral)
	require.Equal(t, uint64(1), account.PositionCount)
	position := e.readPosition(positionID)
	require.Equal(t, holder, position.Account)
	require.Equal(t, insuranceBase(100_000), position.SizeBase)
	require.False(t, e.ctx.KVStore(e.key).Has(types.AccountKey(e.k.InsuranceAccountAddress())))
	unchanged, err := e.k.AccountMargin(e.ctx, holder)
	require.NoError(t, err)
	require.Equal(t, margin, unchanged)

	response, err = server.UpdateParams(sdk.WrapSDKContext(e.ctx), &types.MsgUpdateParams{
		Authority: e.k.Authority(), Params: defaults,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err = e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.Equal(t, defaults, stored)

	events, err = e.k.Backstop(e.ctx, holder)
	require.NoError(t, err)
	require.Equal(t, []types.EventRecord{
		backstopEvent(holder, insuranceUsd(60_000), mark, insuranceUsd(18)),
		contributionEvent(e.usdl, 18_000_000, keeper.InsuranceSourceBackstopMargin),
	}, events)
	account = e.readAccount(holder)
	require.True(t, account.Collateral.IsZero())
	require.Zero(t, account.PositionCount)
	require.False(t, e.ctx.KVStore(e.key).Has(types.AccountMarketPositionKey(holder, 0)))
	fund := e.readAccount(e.k.InsuranceAccountAddress())
	require.Equal(t, *uint256.NewInt(18_000_000), fund.Collateral)
	require.Equal(t, uint64(1), fund.PositionCount)
	position = e.readPosition(positionID)
	require.Equal(t, fund.Address, position.Account)
	require.Equal(t, insuranceBase(100_000), position.SizeBase)
	require.Equal(t, mark, position.EntryPrice)
}
