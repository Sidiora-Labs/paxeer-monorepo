package keeper

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestDecodeProgramRequestMatchesTheKernelRecordRows(t *testing.T) {
	const requestID uint64 = 0x0102030405060708
	payload := []byte("https://paxeer.app/")
	record := binary.BigEndian.AppendUint64(nil, requestID)
	record = append(record, types.KindFetch)
	record = binary.BigEndian.AppendUint32(record, uint32(len(payload)))
	record = append(record, payload...)
	invalidKind := bytes.Clone(record)
	invalidKind[8] = 3

	for _, row := range []struct {
		name string
		data []byte
		code string
	}{
		{name: "fetch", data: record},
		{name: "one_byte_short", data: record[:len(record)-1], code: "LXP_NON_CANONICAL"},
		{name: "kind_3", data: invalidKind, code: "LXP_NON_CANONICAL"},
	} {
		t.Run(row.name, func(t *testing.T) {
			id, kind, decoded, err := decodeProgramRequest(row.data)
			if row.code != "" {
				require.EqualError(t, err, row.code)
				return
			}
			require.NoError(t, err)
			require.Equal(t, requestID, id)
			require.Equal(t, uint8(types.KindFetch), kind)
			require.Len(t, decoded, len(payload))
			require.Equal(t, payload, decoded)
		})
	}
}

func TestFulfilProgramTwiceMatchesTheKernelReuseRefusal(t *testing.T) {
	s := newProgramFulfilSuite(t, 3)
	require.True(t, s.k.ProgramOriginActive(s.ctx))
	const requestID uint64 = 0x0102030405060708
	var programID types.Hash32
	for i := range programID {
		programID[i] = byte(0xa0 + i)
	}
	payload := []byte("https://paxeer.app/")
	response := []byte("Paxeer X Network")
	s.record(programID, requestID, types.KindFetch, payload, paxProgramAsset, sdk.NewInt(40))
	require.False(t, s.pending(programID, requestID).Fulfilled)
	require.Equal(t, uint64(1), s.k.ProgramPendingCount(s.ctx))
	require.Zero(t, s.k.ProgramCommittedCount(s.ctx))

	observation := s.observation(programID, requestID, response, uint32(len(response)))
	observation.Kind = types.KindFetch
	observation.PayloadHash = types.Keccak(payload)
	raw := s.signed(observation, s.keys...)
	require.NoError(t, s.k.FulfilProgram(s.ctx, raw))
	require.True(t, s.pending(programID, requestID).Fulfilled)
	require.Equal(t, byte(1), s.fulfilledByte(programID, requestID))
	require.Zero(t, s.k.ProgramPendingCount(s.ctx))
	require.Equal(t, uint64(1), s.k.ProgramCommittedCount(s.ctx))
	require.Equal(t, response, s.answerPart(programID, requestID, 1))

	before := s.state()
	err := s.k.FulfilProgram(s.ctx, raw)
	require.ErrorIs(t, err, errProgramSequenceReused)
	require.EqualError(t, err, "LXP_SEQUENCE_REUSED")
	require.Equal(t, before, s.state())
}
