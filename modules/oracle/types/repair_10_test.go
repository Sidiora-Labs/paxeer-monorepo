package types_test

import (
	"math"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper/testutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
)

func TestRepair_10_WeightedMedian(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rates  []int64
		powers []int64
		want   int64
	}{
		{name: "odd_total_three", rates: []int64{1, 100}, powers: []int64{1, 2}, want: 100},
		{name: "odd_total_one_with_abstentions", rates: []int64{-1, 0, 100}, powers: []int64{0, 0, 1}, want: 100},
		{name: "odd_total_one_with_zero_power_price", rates: []int64{1, 100}, powers: []int64{0, 1}, want: 100},
		{name: "odd_lower_majority", rates: []int64{1, 100}, powers: []int64{2, 1}, want: 1},
		{name: "even_tie", rates: []int64{1, 100}, powers: []int64{1, 1}, want: 1},
		{name: "even_tie_with_abstention", rates: []int64{0, 1, 100}, powers: []int64{0, 2, 2}, want: 1},
		{name: "all_zero_power", rates: []int64{1, 100}, powers: []int64{0, 0}, want: 0},
		{name: "empty", want: 0},
		{name: "max_odd_total", rates: []int64{1, 100}, powers: []int64{math.MaxInt64 / 2, math.MaxInt64/2 + 1}, want: 100},
		{name: "max_even_tie", rates: []int64{1, 100}, powers: []int64{(math.MaxInt64 - 1) / 2, (math.MaxInt64 - 1) / 2}, want: 1},
		{name: "max_single_vote", rates: []int64{100}, powers: []int64{math.MaxInt64}, want: 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ballot := make(types.ExchangeRateBallot, 0, len(tc.rates))
			for i, rate := range tc.rates {
				ballot = append(ballot, types.NewVoteForTally(sdk.NewDec(rate), utils.MicroAtomDenom, nil, tc.powers[i]))
			}
			require.True(t, sort.IsSorted(ballot))
			require.Equal(t, sdk.NewDec(tc.want), ballot.WeightedMedian())
			require.Equal(t, sdk.NewDec(tc.want), ballot.WeightedMedianWithAssertion())
		})
	}
}

func TestRepair_10_AssertionRejectsUnsorted(t *testing.T) {
	for _, power := range []int64{0, 1} {
		ballot := types.ExchangeRateBallot{
			types.NewVoteForTally(sdk.NewDec(100), utils.MicroAtomDenom, nil, power),
			types.NewVoteForTally(sdk.OneDec(), utils.MicroAtomDenom, nil, power),
		}
		require.PanicsWithValue(t, "ballot must be sorted", func() { ballot.WeightedMedianWithAssertion() })
	}
}

func TestRepair_10_MidBlockOddTotalOne(t *testing.T) {
	repair10MidBlock(t, []int64{1, 1}, []int64{0, 100}, []int64{0, 50}, 1, 100, 50)
}

func TestRepair_10_MidBlockOddTotalThree(t *testing.T) {
	repair10MidBlock(t, []int64{1, 2}, []int64{1, 100}, []int64{1, 50}, 3, 100, 50)
}

func TestRepair_10_MidBlockEvenTie(t *testing.T) {
	repair10MidBlock(t, []int64{1, 1}, []int64{2, 100}, []int64{2, 50}, 2, 2, 2)
}

func repair10MidBlock(t *testing.T, powers, referenceRates, quoteRates []int64, ballotPower, wantReference, wantQuote int64) {
	t.Helper()
	previousWhitelist := types.DefaultWhitelist
	types.DefaultWhitelist = types.DenomList{{Name: utils.MicroAtomDenom}, {Name: utils.MicroEthDenom}}
	t.Cleanup(func() { types.DefaultWhitelist = previousWhitelist })
	input := testutils.CreateTestInput(t)
	input.Ctx = input.Ctx.WithBlockTime(time.Unix(1700000000, 0).UTC())
	stakingHandler := staking.NewHandler(input.StakingKeeper)
	for i, power := range powers {
		amount := sdk.TokensFromConsensusPower(power, sdk.DefaultPowerReduction)
		_, err := stakingHandler(input.Ctx, testutils.NewTestMsgCreateValidator(testutils.ValAddrs[i], testutils.ValPubKeys[i], amount))
		require.NoError(t, err)
	}
	_, err := input.StakingKeeper.ApplyAndReturnValidatorSetUpdates(input.Ctx)
	require.NoError(t, err)

	claims := make(map[string]types.Claim, len(powers))
	for i, power := range powers {
		validator, found := input.StakingKeeper.GetValidator(input.Ctx, testutils.ValAddrs[i])
		require.True(t, found)
		require.True(t, validator.IsBonded())
		require.Equal(t, power, validator.GetConsensusPower(sdk.DefaultPowerReduction))
		claims[testutils.ValAddrs[i].String()] = types.NewClaim(power, 0, 0, testutils.ValAddrs[i], false)
	}

	denoms := []string{utils.MicroAtomDenom, utils.MicroEthDenom}
	wants := []int64{wantReference, wantQuote}
	for _, denom := range denoms {
		require.True(t, input.OracleKeeper.IsVoteTarget(input.Ctx, denom))
		input.OracleKeeper.SetBaseExchangeRate(input.Ctx, denom, sdk.NewDec(777))
	}
	params := input.OracleKeeper.GetParams(input.Ctx)
	require.Equal(t, uint64(2), params.VotePeriod)
	ctx := input.Ctx.WithBlockHeight(1).WithBlockTime(input.Ctx.BlockTime().Add(time.Second))
	handler := oracle.NewHandler(input.OracleKeeper)
	for i := range powers {
		exchangeRates := sdk.NewDec(referenceRates[i]).String() + denoms[0] + "," + sdk.NewDec(quoteRates[i]).String() + denoms[1]
		msg := types.NewMsgAggregateExchangeRateVote(exchangeRates, testutils.Addrs[i], testutils.ValAddrs[i])
		require.NoError(t, msg.ValidateBasic())
		_, err := handler(ctx, msg)
		require.NoError(t, err)
		vote, err := input.OracleKeeper.GetAggregateExchangeRateVote(ctx, testutils.ValAddrs[i])
		require.NoError(t, err)
		require.Equal(t, testutils.ValAddrs[i].String(), vote.Voter)
		require.Equal(t, types.ExchangeRateTuples{
			types.NewExchangeRateTuple(denoms[0], sdk.NewDec(referenceRates[i])),
			types.NewExchangeRateTuple(denoms[1], sdk.NewDec(quoteRates[i])),
		}, vote.ExchangeRateTuples)
	}
	ballots := input.OracleKeeper.OrganizeBallotByDenom(ctx, claims)
	for _, denom := range denoms {
		require.Len(t, ballots[denom], len(powers))
		require.True(t, sort.IsSorted(ballots[denom]))
		require.Equal(t, ballotPower, ballots[denom].Power())
	}

	oracle.MidBlocker(ctx, input.OracleKeeper)
	query := keeper.NewQuerier(input.OracleKeeper)
	for i, denom := range denoms {
		response, err := query.ExchangeRate(sdk.WrapSDKContext(ctx), &types.QueryExchangeRateRequest{Denom: denom})
		require.NoError(t, err)
		require.Equal(t, sdk.NewDec(wants[i]), response.OracleExchangeRate.ExchangeRate)
		require.Equal(t, sdk.NewInt(ctx.BlockHeight()), response.OracleExchangeRate.LastUpdate)
		require.Equal(t, ctx.BlockTime().UnixMilli(), response.OracleExchangeRate.LastUpdateTimestamp)
	}
	for i := range powers {
		_, err := input.OracleKeeper.GetAggregateExchangeRateVote(ctx, testutils.ValAddrs[i])
		require.ErrorIs(t, err, types.ErrNoAggregateVote)
	}
}
