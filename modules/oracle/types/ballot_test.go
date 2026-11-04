package types

import (
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/utils"
)

func TestToMap(t *testing.T) {
	tests := struct {
		votes   []VoteForTally
		isValid []bool
	}{
		[]VoteForTally{
			{
				Voter:        sdk.ValAddress(secp256k1.GenPrivKey().PubKey().Address()),
				Denom:        utils.MicroAtomDenom,
				ExchangeRate: sdk.NewDec(1600),
				Power:        100,
			},
		},
		[]bool{true},
	}

	pb := ExchangeRateBallot(tests.votes)
	mapData := pb.ToMap()
	for i, vote := range tests.votes {
		exchangeRate, ok := mapData[string(vote.Voter)]
		if tests.isValid[i] {
			require.True(t, ok)
			require.Equal(t, exchangeRate, vote.ExchangeRate)
		} else {
			require.False(t, ok)
		}
	}
}

func TestToCrossRate(t *testing.T) {
	data := []struct {
		base     sdk.Dec
		quote    sdk.Dec
		expected sdk.Dec
	}{
		{
			base:     sdk.NewDec(1600),
			quote:    sdk.NewDec(100),
			expected: sdk.NewDec(16),
		},
		{
			base:     sdk.NewDec(0),
			quote:    sdk.NewDec(100),
			expected: sdk.NewDec(16),
		},
		{
			base:     sdk.NewDec(1600),
			quote:    sdk.NewDec(0),
			expected: sdk.NewDec(16),
		},
	}

	pbBase := ExchangeRateBallot{}
	pbQuote := ExchangeRateBallot{}
	cb := ExchangeRateBallot{}
	for _, data := range data {
		valAddr := sdk.ValAddress(secp256k1.GenPrivKey().PubKey().Address())
		if !data.base.IsZero() {
			pbBase = append(pbBase, NewVoteForTally(data.base, utils.MicroAtomDenom, valAddr, 100))
		}

		pbQuote = append(pbQuote, NewVoteForTally(data.quote, utils.MicroAtomDenom, valAddr, 100))

		if !data.base.IsZero() && !data.quote.IsZero() {
			cb = append(cb, NewVoteForTally(data.base.Quo(data.quote), utils.MicroAtomDenom, valAddr, 100))
		} else {
			cb = append(cb, NewVoteForTally(sdk.ZeroDec(), utils.MicroAtomDenom, valAddr, 0))
		}
	}

	sort.Sort(cb)

	baseMapBallot := pbBase.ToMap()
	require.Equal(t, cb, pbQuote.ToCrossRateWithSort(baseMapBallot))
}

func TestSqrt(t *testing.T) {
	pb := ExchangeRateBallot{
		NewVoteForTally(sdk.ZeroDec(), utils.MicroAtomDenom, nil, 3),
		NewVoteForTally(sdk.NewDecWithPrec(24, 2), utils.MicroAtomDenom, nil, 1),
	}
	require.Equal(t, sdk.NewDecWithPrec(12, 2), pb.StandardDeviation(sdk.ZeroDec()))
}

func TestPBPower(t *testing.T) {
	ctx := sdk.NewContext(nil, tmproto.Header{}, false)
	_, valAccAddrs, sk := GenerateRandomTestCase()
	pb := ExchangeRateBallot{}
	ballotPower := int64(0)

	for i := 0; i < len(sk.Validators()); i++ {
		power := sk.Validator(ctx, valAccAddrs[i]).GetConsensusPower(sdk.DefaultPowerReduction)
		vote := NewVoteForTally(
			sdk.ZeroDec(),
			utils.MicroAtomDenom,
			valAccAddrs[i],
			power,
		)

		pb = append(pb, vote)

		require.NotEqual(t, int64(0), vote.Power)

		ballotPower += vote.Power
	}

	require.Equal(t, ballotPower, pb.Power())

	// Mix in a fake validator, the total power should not have changed.
	pubKey := secp256k1.GenPrivKey().PubKey()
	faceValAddr := sdk.ValAddress(pubKey.Address())
	fakeVote := NewVoteForTally(
		sdk.OneDec(),
		utils.MicroAtomDenom,
		faceValAddr,
		0,
	)

	pb = append(pb, fakeVote)
	require.Equal(t, ballotPower, pb.Power())
}

func TestPBWeightedMedian(t *testing.T) {
	tests := []struct {
		inputs      []int64
		weights     []int64
		isValidator []bool
		median      sdk.Dec
	}{
		{
			// Supermajority one number
			[]int64{1, 2, 10, 100000},
			[]int64{1, 1, 100, 1},
			[]bool{true, true, true, true},
			sdk.NewDec(10),
		},
		{
			// Adding fake validator doesn't change outcome
			[]int64{1, 2, 10, 100000, 10000000000},
			[]int64{1, 1, 100, 1, 10000},
			[]bool{true, true, true, true, false},
			sdk.NewDec(10),
		},
		{
			// Tie votes
			[]int64{1, 2, 3, 4},
			[]int64{1, 100, 100, 1},
			[]bool{true, true, true, true},
			sdk.NewDec(2),
		},
		{[]int64{1, 100}, []int64{1, 2}, []bool{true, true}, sdk.NewDec(100)},
		{[]int64{0, 100}, []int64{0, 1}, []bool{false, true}, sdk.NewDec(100)},
		{[]int64{0, 1, 100, 1000}, []int64{0, 1, 2, 0}, []bool{false, true, true, false}, sdk.NewDec(100)},
		{[]int64{0, 100}, []int64{0, 0}, []bool{false, false}, sdk.ZeroDec()},
		{
			// No votes
			[]int64{},
			[]int64{},
			[]bool{true, true, true, true},
			sdk.NewDec(0),
		},
	}

	for _, tc := range tests {
		pb := ExchangeRateBallot{}
		for i, input := range tc.inputs {
			valAddr := sdk.ValAddress(secp256k1.GenPrivKey().PubKey().Address())

			power := tc.weights[i]
			if !tc.isValidator[i] {
				power = 0
			}

			vote := NewVoteForTally(
				sdk.NewDec(int64(input)),
				utils.MicroAtomDenom,
				valAddr,
				power,
			)

			pb = append(pb, vote)
		}

		require.Equal(t, tc.median, pb.WeightedMedian())
		require.Equal(t, tc.median, pb.WeightedMedianWithAssertion())
	}
}

func TestPBStandardDeviation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rates  []string
		powers []int64
		want   string
	}{
		{"supermajority", []string{"1", "2", "10", "100000"}, []int64{1, 1, 100, 1}, "9852.307523808283989385"},
		{"zero_power_invariance", []string{"1", "2", "10", "100000", "10000000000"}, []int64{1, 1, 100, 1, 0}, "9852.307523808283989385"},
		{"tie", []string{"1", "2", "3", "4"}, []int64{1, 100, 100, 1}, "0.720972939990135692"},
		{"nonuniform", []string{"1", "5"}, []int64{3, 1}, "2"},
		{"decimal_precision", []string{"1", "1.000000000000000002"}, []int64{3, 1}, "0.000000000000000001"},
		{"zero_power", []string{"0", "100"}, []int64{0, 0}, "0"},
		{"empty", nil, nil, "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pb := ExchangeRateBallot{}
			for i, rate := range tc.rates {
				pb = append(pb, NewVoteForTally(sdk.MustNewDecFromStr(rate), utils.MicroAtomDenom, nil, tc.powers[i]))
			}
			got := pb.StandardDeviation(pb.WeightedMedianWithAssertion())
			require.Equal(t, sdk.MustNewDecFromStr(tc.want), got)
			withZero := append(ExchangeRateBallot{NewVoteForTally(sdk.ZeroDec(), utils.MicroAtomDenom, nil, 0)}, pb...)
			withZero = append(withZero, NewVoteForTally(sdk.NewDec(100000000000), utils.MicroAtomDenom, nil, 0))
			sort.Sort(withZero)
			require.Equal(t, got, withZero.StandardDeviation(withZero.WeightedMedianWithAssertion()))
		})
	}
}

func TestPBStandardDeviationOverflow(t *testing.T) {
	exchangeRate := sdk.MustNewDecFromStr("100000000000000000000000000000000000000000000000000000000.0")
	pb := ExchangeRateBallot{
		NewVoteForTally(sdk.ZeroDec(), utils.MicroAtomDenom, nil, 2),
		NewVoteForTally(exchangeRate, utils.MicroAtomDenom, nil, 1),
	}
	require.Equal(t, sdk.MustNewDecFromStr("57735026918962576450914878050195745564760175127012687601.860232648397767230"),
		pb.StandardDeviation(pb.WeightedMedianWithAssertion()))
	pb[0].Power = math.MaxInt64
	require.PanicsWithValue(t, "oracle deviation: invalid or overflowing voting power", func() {
		pb.StandardDeviation(sdk.ZeroDec())
	})
}
