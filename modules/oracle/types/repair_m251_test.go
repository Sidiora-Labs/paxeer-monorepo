package types_test

import (
	"math"
	"math/big"
	"sort"
	"strings"
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

func TestRepairM_251_WeightedMedian(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rates  []int64
		powers []int64
		want   int64
	}{
		{"odd_three", []int64{1, 100}, []int64{1, 2}, 100},
		{"odd_one_leading_abstention", []int64{0, 100}, []int64{0, 1}, 100},
		{"both_ends_zero_power", []int64{0, 1, 100, 1000}, []int64{0, 1, 2, 0}, 100},
		{"all_zero_power", []int64{0, 100}, []int64{0, 0}, 0},
		{"even_tie", []int64{1, 100}, []int64{1, 1}, 1},
		{"majority", []int64{1, 100}, []int64{2, 1}, 1},
		{"max_odd_power", []int64{1, 100}, []int64{math.MaxInt64 / 2, math.MaxInt64/2 + 1}, 100},
		{"empty", nil, nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ballot := types.ExchangeRateBallot{}
			for i, rate := range tc.rates {
				ballot = append(ballot, types.NewVoteForTally(sdk.NewDec(rate), utils.MicroAtomDenom, nil, tc.powers[i]))
			}
			require.Equal(t, sdk.NewDec(tc.want), ballot.WeightedMedian())
			require.Equal(t, sdk.NewDec(tc.want), ballot.WeightedMedianWithAssertion())
		})
	}
}

func TestRepairM_251_DeterministicDeviation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rates  []string
		powers []int64
		want   string
	}{
		{"power_weighting", []string{"1", "5"}, []int64{3, 1}, "2"},
		{"irrational_root", []string{"1", "2", "3", "4"}, []int64{1, 100, 100, 1}, "0.720972939990135692"},
		{"full_precision", []string{"1", "1.000000000000000002"}, []int64{3, 1}, "0.000000000000000001"},
		{"no_intermediate_rounding", []string{"0.000000000000000001", "0.000000000000000003"}, []int64{1, 1}, "0.000000000000000001"},
		{"large_power", []string{"1", "5"}, []int64{3 * (math.MaxInt64 / 4), math.MaxInt64 / 4}, "2"},
		{"large_variance", []string{"0", "100000000000000000000000000000000000000000000000000000000"}, []int64{2, 1}, "57735026918962576450914878050195745564760175127012687601.860232648397767230"},
		{"all_zero", []string{"1", "5"}, []int64{0, 0}, "0"},
		{"empty", nil, nil, "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ballot := types.ExchangeRateBallot{}
			for i, rate := range tc.rates {
				ballot = append(ballot, types.NewVoteForTally(sdk.MustNewDecFromStr(rate), utils.MicroAtomDenom, nil, tc.powers[i]))
			}
			median := ballot.WeightedMedianWithAssertion()
			got := ballot.StandardDeviation(median)
			require.Equal(t, sdk.MustNewDecFromStr(tc.want), got)
			withZero := append(types.ExchangeRateBallot{}, ballot...)
			withZero = append(withZero,
				types.NewVoteForTally(sdk.NewDec(-999999999), utils.MicroAtomDenom, nil, 0),
				types.NewVoteForTally(sdk.MustNewDecFromStr("100000000000000000000000000000000000000000000000000000000"), utils.MicroAtomDenom, nil, 0))
			sort.Sort(withZero)
			require.Equal(t, median, withZero.WeightedMedianWithAssertion())
			require.Equal(t, got, withZero.StandardDeviation(median))
			// Ignored observations need not have a representable rate.
			withZero = append(withZero, types.NewVoteForTally(sdk.Dec{}, utils.MicroAtomDenom, nil, 0))
			require.Equal(t, got, withZero.StandardDeviation(median))
		})
	}
	// The largest representable decimal must not overflow while squaring.
	limit := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255+sdk.DecimalPrecisionBits), big.NewInt(1))
	maxRate := sdk.NewDecFromBigIntWithPrec(limit, sdk.Precision)
	ballot := types.ExchangeRateBallot{types.NewVoteForTally(sdk.ZeroDec(), utils.MicroAtomDenom, nil, 3), types.NewVoteForTally(maxRate, utils.MicroAtomDenom, nil, 1)}
	want := sdk.NewDecFromBigIntWithPrec(new(big.Int).Quo(limit, big.NewInt(2)), sdk.Precision)
	require.Equal(t, want, ballot.StandardDeviation(sdk.ZeroDec()))
}

func TestRepairM_251_ArithmeticFailureIsVisible(t *testing.T) {
	oversized := sdk.NewDecFromInt(sdk.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))))
	for _, tc := range []struct {
		name    string
		ballot  types.ExchangeRateBallot
		median  sdk.Dec
		message string
	}{
		{"oversized_median", nil, oversized, "oracle deviation: invalid median"},
		{"oversized_rate", types.ExchangeRateBallot{types.NewVoteForTally(oversized, utils.MicroAtomDenom, nil, 1)}, sdk.ZeroDec(), "oracle deviation: invalid exchange rate"},
		{"nil_median", nil, sdk.Dec{}, "oracle deviation: invalid median"},
		{"nil_rate", types.ExchangeRateBallot{types.NewVoteForTally(sdk.Dec{}, utils.MicroAtomDenom, nil, 1)}, sdk.ZeroDec(), "oracle deviation: invalid exchange rate"},
		{"negative_power", types.ExchangeRateBallot{types.NewVoteForTally(sdk.OneDec(), utils.MicroAtomDenom, nil, -1)}, sdk.OneDec(), "oracle deviation: invalid or overflowing voting power"},
		{"power_overflow", types.ExchangeRateBallot{types.NewVoteForTally(sdk.OneDec(), utils.MicroAtomDenom, nil, math.MaxInt64), types.NewVoteForTally(sdk.OneDec(), utils.MicroAtomDenom, nil, 1)}, sdk.OneDec(), "oracle deviation: invalid or overflowing voting power"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.PanicsWithValue(t, tc.message, func() { tc.ballot.StandardDeviation(tc.median) })
		})
	}
	limit := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255+sdk.DecimalPrecisionBits), big.NewInt(1))
	maxRate := sdk.NewDecFromBigIntWithPrec(limit, sdk.Precision)
	ballot := types.ExchangeRateBallot{types.NewVoteForTally(maxRate, utils.MicroAtomDenom, nil, 1)}
	require.PanicsWithValue(t, "decimal out of range", func() { ballot.StandardDeviation(maxRate.Neg()) })
	claims := map[string]types.Claim{}
	ballot = types.ExchangeRateBallot{
		types.NewVoteForTally(sdk.OneDec(), utils.MicroAtomDenom, testutils.ValAddrs[0], math.MaxInt64),
		types.NewVoteForTally(sdk.NewDec(2), utils.MicroAtomDenom, testutils.ValAddrs[1], 1),
	}
	require.PanicsWithValue(t, "oracle deviation: invalid or overflowing voting power", func() {
		oracle.Tally(sdk.Context{}, ballot, sdk.ZeroDec(), claims)
	})
	require.Empty(t, claims, "arithmetic failure must occur before any reward claims change")
}

func repairM251Input(t *testing.T, powers []int64) testutils.TestInput {
	t.Helper()
	oldWhitelist := types.DefaultWhitelist
	types.DefaultWhitelist = types.DenomList{{Name: utils.MicroAtomDenom}}
	t.Cleanup(func() { types.DefaultWhitelist = oldWhitelist })
	input := testutils.CreateTestInput(t)
	input.Ctx = input.Ctx.WithBlockHeight(1).WithBlockTime(time.Unix(1700000000, 0).UTC())
	handler := staking.NewHandler(input.StakingKeeper)
	for i, power := range powers {
		amount := sdk.TokensFromConsensusPower(power, sdk.DefaultPowerReduction)
		_, err := handler(input.Ctx, testutils.NewTestMsgCreateValidator(testutils.ValAddrs[i], testutils.ValPubKeys[i], amount))
		require.NoError(t, err)
	}
	_, err := input.StakingKeeper.ApplyAndReturnValidatorSetUpdates(input.Ctx)
	require.NoError(t, err)
	for i, power := range powers {
		validator, found := input.StakingKeeper.GetValidator(input.Ctx, testutils.ValAddrs[i])
		require.True(t, found)
		require.True(t, validator.IsBonded())
		require.Equal(t, power, validator.GetConsensusPower(sdk.DefaultPowerReduction))
	}
	return input
}

func repairM251Submit(t *testing.T, input testutils.TestInput, rates []string) {
	t.Helper()
	handler := oracle.NewHandler(input.OracleKeeper)
	for i, rate := range rates {
		if rate == "" {
			continue
		}
		msg := types.NewMsgAggregateExchangeRateVote(rate+utils.MicroAtomDenom, testutils.Addrs[i], testutils.ValAddrs[i])
		require.NoError(t, msg.ValidateBasic())
		_, err := handler(input.Ctx, msg)
		require.NoError(t, err)
		stored, err := input.OracleKeeper.GetAggregateExchangeRateVote(input.Ctx, testutils.ValAddrs[i])
		require.NoError(t, err)
		require.Equal(t, types.NewAggregateExchangeRateVote(types.ExchangeRateTuples{types.NewExchangeRateTuple(utils.MicroAtomDenom, sdk.MustNewDecFromStr(rate))}, testutils.ValAddrs[i]), stored)
	}
}

func TestRepairM_251_TallyAndPenaltyOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		powers    []int64
		rates     []string
		band      string
		deviation string
		wins      []int64
	}{
		{"weighted_boundary", []int64{99, 1, 1, 1, 1}, []string{"100", "110", "200", "0", ""}, "0", "10", []int64{1, 1, 0, 0, 0}},
		{"zero_abstention_cannot_widen_band", []int64{99, 1, 1, 1}, []string{"100", "150", "0", ""}, "0.02", "5", []int64{1, 0, 0, 0}},
		{"configured_band_boundary", []int64{99, 1, 1, 1}, []string{"100", "125", "0", ""}, "0.5", "2.5", []int64{1, 1, 0, 0}},
		{"zero_power_never_wins_wide_band", []int64{99, 1, 1, 1}, []string{"100", "1200", "0", ""}, "0", "110", []int64{1, 0, 0, 0}},
		{"eighteen_decimal_band", []int64{3, 1}, []string{"100", "100.000000000000000002"}, "0", "0.000000000000000001", []int64{1, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := repairM251Input(t, tc.powers)
			params := input.OracleKeeper.GetParams(input.Ctx)
			params.RewardBand = sdk.MustNewDecFromStr(tc.band)
			input.OracleKeeper.SetParams(input.Ctx, params)
			repairM251Submit(t, input, tc.rates)
			claims := map[string]types.Claim{}
			for i, power := range tc.powers {
				claims[testutils.ValAddrs[i].String()] = types.NewClaim(power, 0, 0, testutils.ValAddrs[i], false)
			}
			ballot := input.OracleKeeper.OrganizeBallotByDenom(input.Ctx, claims)[utils.MicroAtomDenom]
			require.True(t, sort.IsSorted(ballot))
			require.Equal(t, sdk.MustNewDecFromStr(tc.deviation), ballot.StandardDeviation(sdk.NewDec(100)))
			withoutZero := types.ExchangeRateBallot{}
			for _, vote := range ballot {
				if vote.Power > 0 {
					withoutZero = append(withoutZero, vote)
				}
			}
			require.Equal(t, withoutZero.StandardDeviation(sdk.NewDec(100)), ballot.StandardDeviation(sdk.NewDec(100)))
			positiveClaims := map[string]types.Claim{}
			for key, claim := range claims {
				positiveClaims[key] = claim
			}
			require.Equal(t, sdk.NewDec(100), oracle.Tally(input.Ctx, withoutZero, params.RewardBand, positiveClaims))
			require.Equal(t, sdk.NewDec(100), oracle.Tally(input.Ctx, ballot, params.RewardBand, claims))
			for _, vote := range withoutZero {
				require.Equal(t, positiveClaims[vote.Voter.String()], claims[vote.Voter.String()])
			}
			for i, power := range tc.powers {
				claim := claims[testutils.ValAddrs[i].String()]
				require.Equal(t, tc.wins[i], claim.WinCount)
				require.Equal(t, power*tc.wins[i], claim.Weight)
				require.Equal(t, tc.rates[i] != "", claim.DidVote)
			}
			oracle.MidBlocker(input.Ctx, input.OracleKeeper)
			query := keeper.NewQuerier(input.OracleKeeper)
			response, err := query.ExchangeRate(sdk.WrapSDKContext(input.Ctx), &types.QueryExchangeRateRequest{Denom: utils.MicroAtomDenom})
			require.NoError(t, err)
			require.Equal(t, sdk.NewDec(100), response.OracleExchangeRate.ExchangeRate)
			require.Equal(t, sdk.NewInt(1), response.OracleExchangeRate.LastUpdate)
			require.Equal(t, input.Ctx.BlockTime().UnixMilli(), response.OracleExchangeRate.LastUpdateTimestamp)
			for i := range tc.powers {
				want := types.VotePenaltyCounter{}
				switch {
				case tc.wins[i] == 1:
					want.SuccessCount = 1
				case tc.rates[i] == "":
					want.AbstainCount = 1
				default:
					want.MissCount = 1
				}
				require.Equal(t, want, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[i]))
				_, err := input.OracleKeeper.GetAggregateExchangeRateVote(input.Ctx, testutils.ValAddrs[i])
				require.ErrorIs(t, err, types.ErrNoAggregateVote)
			}
		})
	}
}

func TestRepairM_251_MidBlockMedianRegressions(t *testing.T) {
	for _, tc := range []struct {
		name                                  string
		powers, reference, quote              []int64
		ballotPower, wantReference, wantQuote int64
	}{
		{"odd_one", []int64{1, 1}, []int64{0, 100}, []int64{0, 50}, 1, 100, 50},
		{"odd_three", []int64{1, 2}, []int64{1, 100}, []int64{1, 50}, 3, 100, 50},
		{"even_tie", []int64{1, 1}, []int64{2, 100}, []int64{2, 50}, 2, 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repair10MidBlock(t, tc.powers, tc.reference, tc.quote, tc.ballotPower, tc.wantReference, tc.wantQuote)
		})
	}
	input := repairM251Input(t, []int64{1, 2})
	repairM251Submit(t, input, []string{"0", "0"})
	oracle.MidBlocker(input.Ctx, input.OracleKeeper)
	_, _, _, err := input.OracleKeeper.GetBaseExchangeRate(input.Ctx, utils.MicroAtomDenom)
	require.ErrorIs(t, err, types.ErrUnknownDenom)
	for i := 0; i < 2; i++ {
		require.Equal(t, types.VotePenaltyCounter{SuccessCount: 1}, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[i]), "below-threshold ballots retain the existing participation policy")
	}
}

func repairM251Genesis(t *testing.T) *types.GenesisState {
	t.Helper()
	input := repairM251Input(t, []int64{1, 2})
	repairM251Submit(t, input, []string{"100", "0"})
	msg := types.NewMsgDelegateFeedConsent(testutils.ValAddrs[0], testutils.Addrs[1])
	require.NoError(t, msg.ValidateBasic())
	_, err := oracle.NewHandler(input.OracleKeeper)(input.Ctx, msg)
	require.NoError(t, err)
	input.OracleKeeper.IncrementMissCount(input.Ctx, testutils.ValAddrs[0])
	input.OracleKeeper.IncrementAbstainCount(input.Ctx, testutils.ValAddrs[0])
	input.OracleKeeper.IncrementSuccessCount(input.Ctx, testutils.ValAddrs[1])
	for i, delta := range []int64{-20, -10} {
		ctx := input.Ctx.WithBlockTime(input.Ctx.BlockTime().Add(time.Duration(delta) * time.Second))
		input.OracleKeeper.SetBaseExchangeRate(ctx, utils.MicroAtomDenom, sdk.NewDec(100+int64(i)))
		rate, height, timestamp, err := input.OracleKeeper.GetBaseExchangeRate(ctx, utils.MicroAtomDenom)
		require.NoError(t, err)
		snapshot := types.NewPriceSnapshot(types.PriceSnapshotItems{types.NewPriceSnapshotItem(utils.MicroAtomDenom, types.OracleExchangeRate{ExchangeRate: rate, LastUpdate: height, LastUpdateTimestamp: timestamp})}, ctx.BlockTime().Unix())
		input.OracleKeeper.AddPriceSnapshot(input.Ctx, snapshot)
	}
	return oracle.ExportGenesis(input.Ctx, input.OracleKeeper)
}

func TestRepairM_251_GenesisRoundTrip(t *testing.T) {
	genesis := repairM251Genesis(t)
	require.NoError(t, types.ValidateGenesis(genesis))
	cdc := testutils.MakeEncodingConfig(t).Marshaler
	encoded, err := cdc.MarshalAsJSON(genesis)
	require.NoError(t, err)
	var decoded types.GenesisState
	require.NoError(t, cdc.UnmarshalAsJSON(encoded, &decoded))
	require.NoError(t, types.ValidateGenesis(&decoded))
	input := repairM251Input(t, nil)
	require.NotPanics(t, func() { oracle.InitGenesis(input.Ctx, input.OracleKeeper, &decoded) })
	require.Equal(t, genesis, oracle.ExportGenesis(input.Ctx, input.OracleKeeper))
	require.Equal(t, testutils.Addrs[1], input.OracleKeeper.GetFeederDelegation(input.Ctx, testutils.ValAddrs[0]))
	rate, height, timestamp, err := input.OracleKeeper.GetBaseExchangeRate(input.Ctx, utils.MicroAtomDenom)
	require.NoError(t, err)
	require.Equal(t, sdk.NewDec(101), rate)
	require.Equal(t, sdk.NewInt(1), height)
	require.Equal(t, input.Ctx.BlockTime().Add(-10*time.Second).UnixMilli(), timestamp)
	require.Equal(t, types.VotePenaltyCounter{MissCount: 1, AbstainCount: 1}, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[0]))
	require.Equal(t, types.VotePenaltyCounter{SuccessCount: 1}, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[1]))
	legacy := types.DefaultGenesisState()
	legacy.ExchangeRateVersion = 0
	legacy.ExchangeRates = types.ExchangeRateTuples{types.NewExchangeRateTuple(utils.MicroAtomDenom, sdk.NewDec(99))}
	require.NoError(t, types.ValidateGenesis(legacy))
	oracle.InitGenesis(input.Ctx, input.OracleKeeper, legacy)
	_, _, _, err = input.OracleKeeper.GetBaseExchangeRate(input.Ctx, utils.MicroAtomDenom)
	require.ErrorIs(t, err, types.ErrUnknownDenom, "legacy imports cannot claim fresh observations")
}

func TestRepairM_251_GenesisRejectsInvalidRecords(t *testing.T) {
	base := repairM251Genesis(t)
	cdc := testutils.MakeEncodingConfig(t).Marshaler
	encoded, err := cdc.MarshalAsJSON(base)
	require.NoError(t, err)
	oversized := sdk.NewDecFromInt(sdk.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))))
	legacy := func(g *types.GenesisState) {
		g.ExchangeRateVersion = 0
		g.OracleExchangeRates = nil
		g.ExchangeRates = types.ExchangeRateTuples{types.NewExchangeRateTuple(utils.MicroAtomDenom, sdk.OneDec())}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*types.GenesisState)
	}{
		{"oversized_parameter", func(g *types.GenesisState) { g.Params.RewardBand = oversized }},
		{"oversized_rate", func(g *types.GenesisState) { g.OracleExchangeRates[0].OracleExchangeRate.ExchangeRate = oversized }},
		{"oversized_vote_rate", func(g *types.GenesisState) {
			g.AggregateExchangeRateVotes[0].ExchangeRateTuples[0].ExchangeRate = oversized
		}},
		{"oversized_legacy_rate", func(g *types.GenesisState) { legacy(g); g.ExchangeRates[0].ExchangeRate = oversized }},
		{"oversized_snapshot_rate", func(g *types.GenesisState) {
			g.PriceSnapshots[0].PriceSnapshotItems[0].OracleExchangeRate.ExchangeRate = oversized
		}},
		{"nil_vote_threshold", func(g *types.GenesisState) { g.Params.VoteThreshold = sdk.Dec{} }},
		{"nil_reward_band", func(g *types.GenesisState) { g.Params.RewardBand = sdk.Dec{} }},
		{"nil_slash_fraction", func(g *types.GenesisState) { g.Params.SlashFraction = sdk.Dec{} }},
		{"nil_min_valid", func(g *types.GenesisState) { g.Params.MinValidPerWindow = sdk.Dec{} }},
		{"excessive_vote_threshold", func(g *types.GenesisState) { g.Params.VoteThreshold = sdk.NewDec(2) }},
		{"lookback_overflow", func(g *types.GenesisState) { g.Params.LookbackDuration = uint64(math.MaxInt64) + 1 }},
		{"invalid_whitelist_denom", func(g *types.GenesisState) { g.Params.Whitelist[0].Name = "x" }},
		{"duplicate_whitelist", func(g *types.GenesisState) { g.Params.Whitelist = append(g.Params.Whitelist, g.Params.Whitelist[0]) }},
		{"invalid_operator", func(g *types.GenesisState) { g.FeederDelegations[0].ValidatorAddress = testutils.Addrs[0].String() }},
		{"invalid_feeder", func(g *types.GenesisState) { g.FeederDelegations[0].FeederAddress = testutils.ValAddrs[0].String() }},
		{"duplicate_delegation", func(g *types.GenesisState) { g.FeederDelegations = append(g.FeederDelegations, g.FeederDelegations[0]) }},
		{"nil_counter", func(g *types.GenesisState) { g.PenaltyCounters[0].VotePenaltyCounter = nil }},
		{"invalid_counter_operator", func(g *types.GenesisState) { g.PenaltyCounters[0].ValidatorAddress = "invalid" }},
		{"duplicate_counter", func(g *types.GenesisState) { g.PenaltyCounters = append(g.PenaltyCounters, g.PenaltyCounters[0]) }},
		{"overflowing_penalties", func(g *types.GenesisState) {
			g.PenaltyCounters[0].VotePenaltyCounter = &types.VotePenaltyCounter{MissCount: math.MaxUint64, AbstainCount: 1}
		}},
		{"overflowing_success_total", func(g *types.GenesisState) {
			g.PenaltyCounters[0].VotePenaltyCounter = &types.VotePenaltyCounter{MissCount: math.MaxUint64, SuccessCount: 1}
		}},
		{"invalid_voter", func(g *types.GenesisState) { g.AggregateExchangeRateVotes[0].Voter = "invalid" }},
		{"duplicate_voter", func(g *types.GenesisState) {
			g.AggregateExchangeRateVotes = append(g.AggregateExchangeRateVotes, g.AggregateExchangeRateVotes[0])
		}},
		{"empty_vote", func(g *types.GenesisState) { g.AggregateExchangeRateVotes[0].ExchangeRateTuples = nil }},
		{"nil_vote_rate", func(g *types.GenesisState) {
			g.AggregateExchangeRateVotes[0].ExchangeRateTuples[0].ExchangeRate = sdk.Dec{}
		}},
		{"negative_vote_rate", func(g *types.GenesisState) {
			g.AggregateExchangeRateVotes[0].ExchangeRateTuples[0].ExchangeRate = sdk.NewDec(-1)
		}},
		{"invalid_vote_denom", func(g *types.GenesisState) { g.AggregateExchangeRateVotes[0].ExchangeRateTuples[0].Denom = "bad denom" }},
		{"duplicate_vote_denom", func(g *types.GenesisState) {
			v := &g.AggregateExchangeRateVotes[0]
			v.ExchangeRateTuples = append(v.ExchangeRateTuples, v.ExchangeRateTuples[0])
		}},
		{"oversized_vote", func(g *types.GenesisState) {
			v := &g.AggregateExchangeRateVotes[0]
			for i := 0; i < 50; i++ {
				v.ExchangeRateTuples = append(v.ExchangeRateTuples, types.NewExchangeRateTuple("u"+strings.Repeat("a", 90)+big.NewInt(int64(i)).String(), sdk.OneDec()))
			}
		}},
		{"unknown_rate_version", func(g *types.GenesisState) { g.ExchangeRateVersion = 2 }},
		{"mixed_versioned_rates", func(g *types.GenesisState) {
			g.ExchangeRates = types.ExchangeRateTuples{types.NewExchangeRateTuple(utils.MicroAtomDenom, sdk.OneDec())}
		}},
		{"mixed_legacy_rates", func(g *types.GenesisState) { g.ExchangeRateVersion = 0 }},
		{"invalid_rate_denom", func(g *types.GenesisState) { g.OracleExchangeRates[0].Denom = "x" }},
		{"duplicate_rate", func(g *types.GenesisState) {
			g.OracleExchangeRates = append(g.OracleExchangeRates, g.OracleExchangeRates[0])
		}},
		{"nil_rate", func(g *types.GenesisState) { g.OracleExchangeRates[0].OracleExchangeRate.ExchangeRate = sdk.Dec{} }},
		{"zero_rate", func(g *types.GenesisState) { g.OracleExchangeRates[0].OracleExchangeRate.ExchangeRate = sdk.ZeroDec() }},
		{"negative_rate", func(g *types.GenesisState) { g.OracleExchangeRates[0].OracleExchangeRate.ExchangeRate = sdk.NewDec(-1) }},
		{"nil_source_height", func(g *types.GenesisState) { g.OracleExchangeRates[0].OracleExchangeRate.LastUpdate = sdk.Int{} }},
		{"negative_source_height", func(g *types.GenesisState) { g.OracleExchangeRates[0].OracleExchangeRate.LastUpdate = sdk.NewInt(-1) }},
		{"negative_observation_time", func(g *types.GenesisState) { g.OracleExchangeRates[0].OracleExchangeRate.LastUpdateTimestamp = -1 }},
		{"invalid_legacy_denom", func(g *types.GenesisState) { legacy(g); g.ExchangeRates[0].Denom = "x" }},
		{"nil_legacy_rate", func(g *types.GenesisState) { legacy(g); g.ExchangeRates[0].ExchangeRate = sdk.Dec{} }},
		{"zero_legacy_rate", func(g *types.GenesisState) { legacy(g); g.ExchangeRates[0].ExchangeRate = sdk.ZeroDec() }},
		{"negative_legacy_rate", func(g *types.GenesisState) { legacy(g); g.ExchangeRates[0].ExchangeRate = sdk.NewDec(-1) }},
		{"duplicate_legacy_rate", func(g *types.GenesisState) { legacy(g); g.ExchangeRates = append(g.ExchangeRates, g.ExchangeRates[0]) }},
		{"negative_snapshot_time", func(g *types.GenesisState) { g.PriceSnapshots[0].SnapshotTimestamp = -1 }},
		{"snapshot_time_overflow", func(g *types.GenesisState) { g.PriceSnapshots[1].SnapshotTimestamp = math.MaxInt64 }},
		{"duplicate_snapshot_time", func(g *types.GenesisState) {
			g.PriceSnapshots[1].SnapshotTimestamp = g.PriceSnapshots[0].SnapshotTimestamp
		}},
		{"out_of_order_snapshots", func(g *types.GenesisState) {
			g.PriceSnapshots[0], g.PriceSnapshots[1] = g.PriceSnapshots[1], g.PriceSnapshots[0]
		}},
		{"invalid_snapshot_denom", func(g *types.GenesisState) { g.PriceSnapshots[0].PriceSnapshotItems[0].Denom = "x" }},
		{"duplicate_snapshot_denom", func(g *types.GenesisState) {
			s := &g.PriceSnapshots[0]
			s.PriceSnapshotItems = append(s.PriceSnapshotItems, s.PriceSnapshotItems[0])
		}},
		{"nil_snapshot_rate", func(g *types.GenesisState) {
			g.PriceSnapshots[0].PriceSnapshotItems[0].OracleExchangeRate.ExchangeRate = sdk.Dec{}
		}},
		{"zero_snapshot_rate", func(g *types.GenesisState) {
			g.PriceSnapshots[0].PriceSnapshotItems[0].OracleExchangeRate.ExchangeRate = sdk.ZeroDec()
		}},
		{"negative_snapshot_rate", func(g *types.GenesisState) {
			g.PriceSnapshots[0].PriceSnapshotItems[0].OracleExchangeRate.ExchangeRate = sdk.NewDec(-1)
		}},
		{"nil_snapshot_height", func(g *types.GenesisState) {
			g.PriceSnapshots[0].PriceSnapshotItems[0].OracleExchangeRate.LastUpdate = sdk.Int{}
		}},
		{"negative_snapshot_height", func(g *types.GenesisState) {
			g.PriceSnapshots[0].PriceSnapshotItems[0].OracleExchangeRate.LastUpdate = sdk.NewInt(-1)
		}},
		{"negative_snapshot_observation_time", func(g *types.GenesisState) {
			g.PriceSnapshots[0].PriceSnapshotItems[0].OracleExchangeRate.LastUpdateTimestamp = -1
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var candidate types.GenesisState
			require.NoError(t, cdc.UnmarshalAsJSON(encoded, &candidate))
			tc.mutate(&candidate)
			input := repairM251Input(t, nil)
			before := oracle.ExportGenesis(input.Ctx, input.OracleKeeper)
			var validationErr error
			require.NotPanics(t, func() { validationErr = types.ValidateGenesis(&candidate) })
			require.Error(t, validationErr)
			require.NotEmpty(t, validationErr.Error())
			require.Equal(t, before, oracle.ExportGenesis(input.Ctx, input.OracleKeeper), "validation must reject before initialization changes the store")
		})
	}
	require.Error(t, types.ValidateGenesis(nil))
	require.Error(t, types.ValidateGenesisExchangeRates(nil))
}
