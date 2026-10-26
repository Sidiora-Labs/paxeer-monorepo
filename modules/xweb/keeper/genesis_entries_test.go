package keeper

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestProgramPendingEntriesIterateInKeyOrder(t *testing.T) {
	s := newProgramFulfilSuite(t, 1)
	require.Empty(t, s.k.ProgramPendingEntries(s.ctx))
	require.Empty(t, s.k.ProgramFeeDenomEntries(s.ctx))

	low, adjacent, high := types.Hash32{1}, types.Hash32{1}, types.Hash32{2}
	adjacent[31] = 1
	want := []types.ProgramPendingEntry{
		{ProgramID: low, RequestID: 0},
		{ProgramID: low, RequestID: 255},
		{ProgramID: low, RequestID: 256},
		{ProgramID: low, RequestID: ^uint64(0)},
		{ProgramID: adjacent, RequestID: 1},
		{ProgramID: high, RequestID: 0},
	}
	for _, index := range []int{5, 2, 4, 3, 0, 1} {
		entry := &want[index]
		s.record(entry.ProgramID, entry.RequestID, types.KindSearch, programFulfilPayload,
			paxProgramAsset, sdk.NewInt(int64(index+1)))
	}
	fulfilled := want[2]
	observation := s.observation(fulfilled.ProgramID, fulfilled.RequestID, []byte("answer"), 6)
	require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys...)))

	store := s.ctx.KVStore(s.k.storeKey)
	for i := range want {
		want[i].Record = bytes.Clone(store.Get(ProgramPendingKey(want[i].ProgramID, want[i].RequestID)))
	}
	require.Equal(t, byte(1), want[2].Record[122])
	entries := s.k.ProgramPendingEntries(s.ctx)
	require.Equal(t, want, entries)
	for i := range entries {
		entries[i].Record[0] ^= 0xff
		require.Equal(t, want[i].Record, store.Get(ProgramPendingKey(want[i].ProgramID, want[i].RequestID)))
	}
	require.Equal(t, want, s.k.ProgramPendingEntries(s.ctx))

	denoms := []types.ProgramFeeDenomEntry{
		{Asset: types.Hash32{}, Denom: types.ProgramFeeDenomPAX},
		{Asset: low, Denom: types.ProgramFeeDenomSID},
		{Asset: adjacent, Denom: types.ProgramFeeDenomUSDC},
		{Asset: high, Denom: types.ProgramFeeDenomUSDL},
	}
	for _, index := range []int{3, 1, 2, 0} {
		s.k.setProgramFeeDenom(s.ctx, denoms[index].Asset, denoms[index].Denom)
	}
	feeEntries := s.k.ProgramFeeDenomEntries(s.ctx)
	require.Equal(t, denoms, feeEntries)
	for _, entry := range feeEntries {
		raw := store.Get(ProgramFeeDenomKey(entry.Asset))
		require.Equal(t, []byte(entry.Denom), raw)
		raw[0] ^= 0xff
	}
	require.Equal(t, denoms, feeEntries)

	for _, family := range []struct {
		name   string
		prefix string
		length int
		visit  func(*programFulfilSuite)
	}{
		{"pending", programPendingPrefix, 40, func(s *programFulfilSuite) { s.k.ProgramPendingEntries(s.ctx) }},
		{"fee denom", programFeeDenomPrefix, 32, func(s *programFulfilSuite) { s.k.ProgramFeeDenomEntries(s.ctx) }},
	} {
		for _, length := range []int{0, family.length - 1, family.length + 1} {
			t.Run(fmt.Sprintf("%s invalid tail %d", family.name, length), func(t *testing.T) {
				s := newProgramFulfilSuite(t, 1)
				key := append([]byte(family.prefix), make([]byte, length)...)
				s.ctx.KVStore(s.k.storeKey).Set(key, []byte{1})
				requireProgramEntriesInvalidGenesis(t, func() { family.visit(s) })
			})
		}
	}
}

func TestProgramAnswerEntriesCarryEveryPart(t *testing.T) {
	s := newProgramFulfilSuite(t, 1)
	require.Empty(t, s.k.ProgramAnswerEntries(s.ctx))
	low, high := types.Hash32{1}, types.Hash32{2}
	observations := []types.Observation{
		s.observation(low, 255, bytes.Repeat([]byte{0x12, 0x34}, ProgramAnswerChunkBytes), 2*ProgramAnswerChunkBytes),
		s.observation(low, 256, bytes.Repeat([]byte{0x56}, 2*ProgramAnswerChunkBytes+17), 2*ProgramAnswerChunkBytes+17),
		s.observation(high, 0, nil, 0),
	}
	for _, index := range []int{2, 1, 0} {
		observation := observations[index]
		s.record(observation.ProgramID, observation.RequestID, types.KindSearch, programFulfilPayload,
			paxProgramAsset, sdk.NewInt(5))
		require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys...)))
	}
	require.Equal(t, uint64(len(observations)), s.k.ProgramCommittedCount(s.ctx))

	var want []types.ProgramAnswerEntry
	for _, observation := range observations {
		header := bytes.Clone(observation.ContentDigest[:])
		header = binary.BigEndian.AppendUint32(header, observation.FullLength)
		header = binary.BigEndian.AppendUint32(header, uint32(len(observation.Response)))
		want = append(want, types.ProgramAnswerEntry{
			ProgramID: observation.ProgramID, RequestID: observation.RequestID, Part: 0, Value: header,
		})
		for offset, part := 0, uint8(1); offset < len(observation.Response); offset, part = offset+ProgramAnswerChunkBytes, part+1 {
			end := min(offset+ProgramAnswerChunkBytes, len(observation.Response))
			want = append(want, types.ProgramAnswerEntry{
				ProgramID: observation.ProgramID, RequestID: observation.RequestID, Part: part,
				Value: bytes.Clone(observation.Response[offset:end]),
			})
		}
	}
	entries := s.k.ProgramAnswerEntries(s.ctx)
	require.Len(t, entries, 8)
	require.Equal(t, want, entries)
	for i := range entries {
		key := ProgramAnswerKey(entries[i].ProgramID, entries[i].RequestID, entries[i].Part)
		require.Equal(t, want[i].Value, s.ctx.KVStore(s.k.storeKey).Get(key))
		entries[i].Value[0] ^= 0xff
		require.Equal(t, want[i].Value, s.ctx.KVStore(s.k.storeKey).Get(key))
	}
	require.Equal(t, want, s.k.ProgramAnswerEntries(s.ctx))

	for _, length := range []int{0, 40, 42} {
		t.Run(fmt.Sprintf("invalid tail %d", length), func(t *testing.T) {
			s := newProgramFulfilSuite(t, 1)
			key := append([]byte(programAnswerPrefix), make([]byte, length)...)
			s.ctx.KVStore(s.k.storeKey).Set(key, []byte{1})
			requireProgramEntriesInvalidGenesis(t, func() { s.k.ProgramAnswerEntries(s.ctx) })
		})
	}
}

func requireProgramEntriesInvalidGenesis(t *testing.T, visit func()) {
	t.Helper()
	var recovered interface{}
	func() {
		defer func() { recovered = recover() }()
		visit()
	}()
	err, ok := recovered.(error)
	require.True(t, ok, "expected an ErrInvalidGenesis panic, got %v", recovered)
	require.ErrorIs(t, err, types.ErrInvalidGenesis)
}
