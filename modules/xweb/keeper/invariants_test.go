package keeper

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestProgramCountersInvariantHoldsAfterFulfilment(t *testing.T) {
	s := newProgramFulfilSuite(t, 4)
	invariant := ProgramCountersInvariant(s.k)
	assertHolds := func(pending, committed uint64) {
		t.Helper()
		before := genesisProgramStoreSnapshot(t, s.k, s.ctx)
		message, broken := invariant(s.ctx)
		require.False(t, broken, message)
		require.Contains(t, message, "xweb: program-counters invariant")
		require.Contains(t, message, fmt.Sprintf("pending: counted=%d stored=%d", pending, pending))
		require.Contains(t, message, fmt.Sprintf("committed: counted fulfilled=%d counted answer headers=%d stored=%d", committed, committed, committed))
		require.Equal(t, pending, s.k.ProgramPendingCount(s.ctx))
		require.Equal(t, committed, s.k.ProgramCommittedCount(s.ctx))
		require.Equal(t, before, genesisProgramStoreSnapshot(t, s.k, s.ctx))
	}
	assertHolds(0, 0)
	s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(75))
	s.record(programFulfilP, 5, types.KindSearch, programFulfilPayload, sidProgramAsset, sdk.NewInt(29))
	s.record(types.Hash32{0x71}, 3, types.KindFetch, []byte("pending fetch"), paxProgramAsset, sdk.NewInt(11))
	assertHolds(3, 0)

	response := bytes.Repeat([]byte{0xab}, 2*ProgramAnswerChunkBytes+17)
	observation := s.observation(programFulfilP, 3, response, uint32(len(response)+100))
	require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys[:3]...)))
	require.True(t, s.pending(programFulfilP, 3).Fulfilled)
	require.Len(t, s.k.ProgramAnswerEntries(s.ctx), 4)
	assertHolds(2, 1)

	observation = s.observation(programFulfilP, 5, nil, 0)
	require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys[:3]...)))
	require.True(t, s.pending(programFulfilP, 5).Fulfilled)
	require.Len(t, s.k.ProgramPendingEntries(s.ctx), 3)
	require.Len(t, s.k.ProgramAnswerEntries(s.ctx), 5)
	assertHolds(1, 2)
}

func TestProgramCountersInvariantBreaksOnADriftedCounter(t *testing.T) {
	for _, tc := range []struct {
		name      string
		mutate    func(*programFulfilSuite)
		pending   string
		committed string
	}{
		{"pending too low", func(s *programFulfilSuite) { s.k.setProgramPendingCount(s.ctx, 0) },
			"pending: counted=1 stored=0", "committed: counted fulfilled=1 counted answer headers=1 stored=1"},
		{"pending too high", func(s *programFulfilSuite) { s.k.setProgramPendingCount(s.ctx, 2) },
			"pending: counted=1 stored=2", "committed: counted fulfilled=1 counted answer headers=1 stored=1"},
		{"committed too low", func(s *programFulfilSuite) { s.k.setProgramCommittedCount(s.ctx, 0) },
			"pending: counted=1 stored=1", "committed: counted fulfilled=1 counted answer headers=1 stored=0"},
		{"committed too high", func(s *programFulfilSuite) { s.k.setProgramCommittedCount(s.ctx, 2) },
			"pending: counted=1 stored=1", "committed: counted fulfilled=1 counted answer headers=1 stored=2"},
		{"missing answer header", func(s *programFulfilSuite) {
			s.ctx.KVStore(s.k.storeKey).Delete(ProgramAnswerKey(programFulfilP, 5, 0))
		}, "pending: counted=1 stored=1", "committed: counted fulfilled=1 counted answer headers=0 stored=1"},
		{"missing fulfilled record", func(s *programFulfilSuite) {
			s.ctx.KVStore(s.k.storeKey).Delete(ProgramPendingKey(programFulfilP, 5))
		}, "pending: counted=1 stored=1", "committed: counted fulfilled=0 counted answer headers=1 stored=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newProgramFulfilSuite(t, 4)
			s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(75))
			s.record(programFulfilP, 5, types.KindSearch, programFulfilPayload, sidProgramAsset, sdk.NewInt(29))
			observation := s.observation(programFulfilP, 5, []byte("answer"), 6)
			require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys[:3]...)))
			invariant := ProgramCountersInvariant(s.k)
			message, broken := invariant(s.ctx)
			require.False(t, broken, message)
			tc.mutate(s)
			before := genesisProgramStoreSnapshot(t, s.k, s.ctx)
			message, broken = invariant(s.ctx)
			require.True(t, broken, message)
			require.Contains(t, message, "xweb: program-counters invariant")
			require.Contains(t, message, tc.pending)
			require.Contains(t, message, tc.committed)
			require.Equal(t, before, genesisProgramStoreSnapshot(t, s.k, s.ctx))
		})
	}
}
