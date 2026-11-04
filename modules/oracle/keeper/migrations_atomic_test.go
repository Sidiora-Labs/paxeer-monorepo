package keeper_test

import (
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper/testutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestMigrate5To6RejectsInconsistentCountersAtomically(t *testing.T) {
	assertMigrate5To6AtomicRejection(t)
}

func assertMigrate5To6AtomicRejection(t *testing.T) {
	t.Helper()
	cases := []struct {
		name      string
		counter   types.VotePenaltyCounter
		malformed bool
	}{
		{name: "25 penalties exceed completed periods", counter: types.VotePenaltyCounter{MissCount: 12, AbstainCount: 13}},
		{name: "one beyond completed periods", counter: types.VotePenaltyCounter{MissCount: 13}},
		{name: "miss exceeds elapsed", counter: types.VotePenaltyCounter{MissCount: 26}},
		{name: "combined exceeds elapsed", counter: types.VotePenaltyCounter{MissCount: 12, AbstainCount: 14}},
		{name: "addition overflow", counter: types.VotePenaltyCounter{MissCount: math.MaxUint64, AbstainCount: 1}},
		{name: "abstain overflow", counter: types.VotePenaltyCounter{AbstainCount: math.MaxUint64}},
		{name: "invalid encoding", malformed: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := testutils.CreateTestInput(t)
			ctx := input.Ctx.WithBlockHeight(25)
			store := ctx.KVStore(input.OracleKeeper.GetStoreKey())
			first := sdk.ValAddress(make([]byte, 20))
			second := sdk.ValAddress(append(make([]byte, 19), 1))
			firstKey := types.GetVotePenaltyCounterKey(first)
			secondKey := types.GetVotePenaltyCounterKey(second)
			firstBytes := input.OracleKeeper.GetCdc().MustMarshal(&types.VotePenaltyCounter{MissCount: 1, AbstainCount: 2})
			secondBytes := input.OracleKeeper.GetCdc().MustMarshal(&tc.counter)
			if tc.malformed {
				secondBytes = []byte{0xff}
			}
			store.Set(firstKey, firstBytes)
			store.Set(secondKey, secondBytes)
			require.Error(t, keeper.NewMigrator(input.OracleKeeper, keeper.VotePeriodHistory{
				StartHeight: 1, EndHeight: 25,
				VotePeriod:  input.OracleKeeper.VotePeriod(ctx),
				SlashWindow: input.OracleKeeper.SlashWindow(ctx),
			}).Migrate5To6(ctx))
			require.Equal(t, firstBytes, store.Get(firstKey))
			require.Equal(t, secondBytes, store.Get(secondKey))
		})
	}
}

func TestMigrate5To6AcceptsZeroSuccessBoundary(t *testing.T) {
	assertMigrate5To6ZeroSuccessBoundary(t)
}

func assertMigrate5To6ZeroSuccessBoundary(t *testing.T) {
	t.Helper()
	for _, height := range []int64{25, 50} {
		input := testutils.CreateTestInput(t)
		ctx := input.Ctx.WithBlockHeight(height)
		period := input.OracleKeeper.VotePeriod(ctx)
		window := input.OracleKeeper.SlashWindow(ctx)
		observations := migrationHookObservations(ctx, period, window)
		address := testutils.ValAddrs[0]
		misses := observations / 2
		abstentions := observations - misses
		input.OracleKeeper.SetVotePenaltyCounter(ctx, address, misses, abstentions, 0)

		require.NoError(t, keeper.NewMigrator(input.OracleKeeper, keeper.VotePeriodHistory{
			StartHeight: 1, EndHeight: uint64(height),
			VotePeriod: period, SlashWindow: window,
		}).Migrate5To6(ctx))

		require.Equal(t, types.VotePenaltyCounter{MissCount: misses, AbstainCount: abstentions, SuccessCount: 0}, input.OracleKeeper.GetVotePenaltyCounter(ctx, address))
	}
}
