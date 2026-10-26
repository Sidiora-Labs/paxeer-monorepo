package keeper_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestPausedVenueSettlesFunding(t *testing.T) {
	e := newEndblockEnv(t, 0)
	e.setMark(usdAmount(60_000))
	e.pair(7, e.address(1), 100_000_000_000, baseAmount(100_000), usdAmount(60_000))
	e.maker(2, usdAmount(60_060), baseAmount(100_000))
	state := e.marketState(0)
	state.LastIndexHour = endblockHour
	e.k.SetMarketState(e.ctx, state)
	market, err := e.k.GetMarket(e.ctx, 0)
	require.NoError(t, err)

	response, err := keeper.NewMsgServerImpl(e.k).SetPaused(sdk.WrapSDKContext(e.ctx), &types.MsgSetPaused{
		Authority: e.k.Authority(), Paused: true,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	params, err := e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.True(t, params.Paused)
	require.ErrorIs(t, e.k.CheckPause(e.ctx, "openPosition"), types.ErrPaused)

	nextHour := time.Unix(int64((endblockHour+1)*keeper.SecondsPerHour), 0).UTC()
	interval := time.Duration(params.PremiumIntervalSeconds) * time.Second
	for i, at := range []time.Time{nextHour.Add(-interval - time.Second), nextHour.Add(-time.Second)} {
		e.ctx = e.ctx.WithBlockTime(at)
		sampled, err := e.k.SamplePremium(e.ctx, market)
		require.NoError(t, err)
		require.True(t, sampled, "premium sampling continues while paused")
		state = e.marketState(0)
		require.Equal(t, uint64(i+1), state.PremiumCount)
		require.Equal(t, big.NewInt(int64(i+1)*1_000_000_000_000_000), state.PremiumSum.ToBig())
		require.Equal(t, usdAmount(int64(i+1)*60_000), state.OracleSum)
		require.Equal(t, uint64(at.Unix()), state.LastSampleTime)
		require.Equal(t, endblockHour, state.LastIndexHour)
	}

	e.setAggregate(0, 60_000, e.ctx.BlockTime().UnixMilli())
	records := e.k.EndBlock(e.ctx)
	require.Empty(t, e.errorEvents())
	require.Empty(t, eventsNamed(records, types.EventFundingSettled), "funding waits for the hour boundary")
	state = e.marketState(0)
	require.Equal(t, uint64(2), state.PremiumCount)
	require.Equal(t, endblockHour, state.LastIndexHour)
	require.True(t, state.FundingIndex.IsZero())

	e.ctx = e.ctx.WithBlockHeight(e.ctx.BlockHeight() + 1).
		WithBlockTime(nextHour.Add(time.Second)).WithEventManager(sdk.NewEventManager())
	e.setAggregate(0, 60_000, e.ctx.BlockTime().UnixMilli())
	records = e.k.EndBlock(e.ctx)
	require.Empty(t, e.errorEvents())
	require.Equal(t, []types.EventRecord{{
		Name: types.EventFundingSettled,
		Args: []any{big.NewInt(0), big.NewInt(62_500_000_000_000),
			big.NewInt(-3_750_000_000_000_000_000), big.NewInt(3_750_000_000_000_000_000)},
	}}, eventsNamed(records, types.EventFundingSettled))
	require.Len(t, eventsNamed(records, types.EventFundingRateUpdated), 1)
	state = e.marketState(0)
	require.Equal(t, endblockHour+1, state.LastIndexHour)
	require.Equal(t, *uint256.NewInt(62_500_000_000_000), state.HourlyRate)
	require.Equal(t, *uint256.NewInt(3_750_000_000_000_000_000), state.FundingIndex)
	require.Zero(t, state.PremiumCount)
	require.True(t, state.PremiumSum.IsZero())
	require.True(t, state.OracleSum.IsZero())
	params, err = e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.True(t, params.Paused, "settlement leaves the governance pause in effect")
}
