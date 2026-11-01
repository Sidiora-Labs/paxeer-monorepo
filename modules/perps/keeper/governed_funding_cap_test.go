package keeper

import (
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestGovernedFundingCapBoundsHourlyRate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		marketCap uint64
		wantPpm   int64
	}{
		{name: "venue cap", marketCap: 40_000, wantPpm: 100},
		{name: "market cap", marketCap: 50, wantPpm: 50},
		{name: "equal caps", marketCap: 100, wantPpm: 100},
	} {
		for _, direction := range []struct {
			name   string
			sign   int64
			isLong bool
			price  int64
		}{
			{name: "positive premium", sign: 1, isLong: true, price: 60_600},
			{name: "negative premium", sign: -1, isLong: false, price: 59_400},
		} {
			t.Run(tc.name+"/"+direction.name, func(t *testing.T) {
				at := fundingHour*secondsPerHour + 10
				k, ctx := fundingKeeper(t, at)
				market := fundingMarket()
				market.FundingCapPpmPerHour = tc.marketCap
				require.NoError(t, k.SetMarket(ctx, k.Authority(), market))
				k.setFundingMarketState(ctx, types.MarketState{MarketID: market.ID, IndexPrice: usd18(60_000)})
				events, err := k.SettleFunding(ctx, market)
				require.NoError(t, err)
				require.Empty(t, events, "initialize the funding hour before collecting samples")

				params, err := k.GetParams(ctx)
				require.NoError(t, err)
				require.Greater(t, params.FundingCapPpmPerHour, uint64(100))
				params.FundingCapPpmPerHour = 100
				response, err := NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{
					Authority: k.Authority(), Params: params,
				})
				require.NoError(t, err)
				require.NotNil(t, response)
				stored, err := k.GetParams(ctx)
				require.NoError(t, err)
				require.Equal(t, params, stored, "governance persists the lower venue funding cap")

				require.NoError(t, k.BookInsert(ctx, restingOrder(1, direction.isLong, usd18(direction.price), usd18(1))))
				sampled, err := k.SamplePremium(ctx, market)
				require.NoError(t, err)
				require.True(t, sampled)
				state, err := k.fundingMarketState(ctx, market.ID)
				require.NoError(t, err)
				require.Equal(t, uint64(1), state.PremiumCount)
				require.Equal(t, big.NewInt(direction.sign*10_000_000_000_000_000), signedOf(state.PremiumSum), "the book produces a signed one-percent premium")

				ctx = ctx.WithBlockTime(time.Unix(int64((fundingHour+1)*secondsPerHour), 0).UTC())
				events, err = k.SettleFunding(ctx, market)
				require.NoError(t, err)
				require.Len(t, events, 2)
				wantRate := big.NewInt(direction.sign * tc.wantPpm * 1_000_000_000_000)
				require.Equal(t, types.EventFundingRateUpdated, events[0].Name)
				require.Equal(t, []any{
					new(big.Int).SetUint64(market.ID),
					new(big.Int).Quo(new(big.Int).Set(wantRate), big.NewInt(3600)),
					new(big.Int).Mul(new(big.Int).Set(wantRate), big.NewInt(24)),
				}, events[0].Args)
				require.Equal(t, types.EventFundingSettled, events[1].Name)
				require.Equal(t, new(big.Int).SetUint64(market.ID), events[1].Args[0])
				require.Equal(t, wantRate, events[1].Args[1], "the next settled rate equals the smaller governed or market cap with the premium's sign")

				state, err = k.fundingMarketState(ctx, market.ID)
				require.NoError(t, err)
				require.Equal(t, wantRate, signedOf(state.HourlyRate))
				require.Equal(t, new(big.Int).Mul(big.NewInt(60_000), wantRate), signedOf(state.FundingIndex), "the cumulative index uses the capped rate for one hour")
				require.Equal(t, fundingHour+1, state.LastIndexHour)
				require.Zero(t, state.PremiumCount)
				require.True(t, state.PremiumSum.IsZero())
				require.True(t, state.OracleSum.IsZero())
			})
		}
	}
}
