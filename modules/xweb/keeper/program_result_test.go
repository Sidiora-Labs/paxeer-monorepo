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

func TestGetProgramResultReassemblesTheChunks(t *testing.T) {
	var s *programFulfilSuite
	for size := 0; size <= types.MaxResponseBytes; size++ {
		if uint64(size)%ProgramCommittedCapacity == 0 {
			s = newProgramFulfilSuite(t, 1)
		}
		requestID := uint64(size) + 1
		response := make([]byte, size)
		for i := range response {
			response[i] = byte(i*31 + i/ProgramAnswerChunkBytes + size)
		}
		fullLength := uint32(size)
		if size%2 != 0 {
			fullLength += 0x01020304
		}
		s.record(programFulfilP, requestID, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(1))
		observation := s.observation(programFulfilP, requestID, response, fullLength)
		observation.ContentDigest = types.Keccak(response)
		require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys...)), "size %d", size)

		result, err := s.k.GetProgramResult(s.ctx, programFulfilP, requestID)
		require.NoError(t, err, "size %d", size)
		require.Equal(t, observation.ContentDigest, result.ContentDigest, "size %d", size)
		require.Equal(t, fullLength, result.FullLength, "size %d", size)
		require.Equal(t, uint32(size), result.ResponseLength, "size %d", size)
		require.Equal(t, response, result.Response, "size %d", size)
		if size == types.MaxResponseBytes {
			result.Response[0] ^= 0xff
			again, err := s.k.GetProgramResult(s.ctx, programFulfilP, requestID)
			require.NoError(t, err)
			require.Equal(t, response, again.Response)
		}
	}
}

func TestGetProgramResultAbsentIsUnknownField(t *testing.T) {
	for _, state := range []string{"never_requested", "pending", "missing_header"} {
		t.Run(state, func(t *testing.T) {
			s := newProgramFulfilSuite(t, 1)
			if state != "never_requested" {
				s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(1))
			}
			if state == "missing_header" {
				observation := s.observation(programFulfilP, 3, []byte("answer"), 6)
				require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys...)))
				s.ctx.KVStore(s.k.storeKey).Delete(ProgramAnswerKey(programFulfilP, 3, 0))
			}
			before := s.state()
			_, err := s.k.GetProgramResult(s.ctx, programFulfilP, 3)
			require.ErrorIs(t, err, errProgramUnknownField)
			require.EqualError(t, err, "LXP_UNKNOWN_FIELD")
			require.Equal(t, before, s.state())
		})
	}
}

func TestGetProgramResultMalformedPartsAreNonCanonical(t *testing.T) {
	commit := func(t *testing.T, size int) *programFulfilSuite {
		t.Helper()
		s := newProgramFulfilSuite(t, 1)
		s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(1))
		observation := s.observation(programFulfilP, 3, bytes.Repeat([]byte{0xa5}, size), uint32(size))
		require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys...)))
		return s
	}
	refuse := func(t *testing.T, s *programFulfilSuite) {
		t.Helper()
		before := s.state()
		_, err := s.k.GetProgramResult(s.ctx, programFulfilP, 3)
		require.ErrorIs(t, err, errProgramNonCanonical)
		require.EqualError(t, err, "LXP_NON_CANONICAL")
		require.Equal(t, before, s.state())
	}
	for _, size := range []int{0, 1, 32, 39, 41, 80} {
		t.Run(fmt.Sprintf("header_length_%d", size), func(t *testing.T) {
			s := commit(t, 2500)
			header := make([]byte, size)
			copy(header, s.answerPart(programFulfilP, 3, 0))
			s.ctx.KVStore(s.k.storeKey).Set(ProgramAnswerKey(programFulfilP, 3, 0), header)
			refuse(t, s)
		})
	}
	for _, test := range []struct {
		name                       string
		fullLength, responseLength uint32
	}{
		{"above_response_limit", 4097, 4097},
		{"above_full_length", 2499, 2500},
		{"nonempty_with_zero_full_length", 0, 1},
		{"maximum_response_length", ^uint32(0), ^uint32(0)},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := commit(t, 2500)
			header := bytes.Clone(s.answerPart(programFulfilP, 3, 0))
			binary.BigEndian.PutUint32(header[32:36], test.fullLength)
			binary.BigEndian.PutUint32(header[36:40], test.responseLength)
			s.ctx.KVStore(s.k.storeKey).Set(ProgramAnswerKey(programFulfilP, 3, 0), header)
			refuse(t, s)
		})
	}
	for _, size := range []int{1024, 2500, 4096} {
		chunks := (size + ProgramAnswerChunkBytes - 1) / ProgramAnswerChunkBytes
		for part := 1; part <= chunks; part++ {
			for _, corruption := range []string{"missing", "short", "long"} {
				t.Run(fmt.Sprintf("size_%d_part_%d_%s", size, part, corruption), func(t *testing.T) {
					s := commit(t, size)
					store := s.ctx.KVStore(s.k.storeKey)
					key := ProgramAnswerKey(programFulfilP, 3, uint8(part))
					chunk := bytes.Clone(store.Get(key))
					switch corruption {
					case "missing":
						store.Delete(key)
					case "short":
						store.Set(key, chunk[:len(chunk)-1])
					case "long":
						store.Set(key, append(chunk, 0))
					}
					refuse(t, s)
				})
			}
		}
	}
}

func TestGetProgramResultIsScopedToTheProgram(t *testing.T) {
	s := newProgramFulfilSuite(t, 1)
	otherProgram := programFulfilP
	otherProgram[31] ^= 1
	const requestID uint64 = 0x0102030405060708
	s.record(programFulfilP, requestID, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(1))
	first := s.observation(programFulfilP, requestID, []byte("first program answer"), 100)
	require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(first, s.keys...)))
	_, err := s.k.GetProgramResult(s.ctx, otherProgram, requestID)
	require.ErrorIs(t, err, errProgramUnknownField)
	_, err = s.k.GetProgramResult(s.ctx, programFulfilP, requestID+1)
	require.ErrorIs(t, err, errProgramUnknownField)

	s.record(otherProgram, requestID, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(1))
	_, err = s.k.GetProgramResult(s.ctx, otherProgram, requestID)
	require.ErrorIs(t, err, errProgramUnknownField)
	second := s.observation(otherProgram, requestID, bytes.Repeat([]byte{0x42}, 1500), 2000)
	second.ContentDigest = types.Keccak(second.Response)
	require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(second, s.keys...)))
	before := s.state()
	for _, observation := range []types.Observation{first, second} {
		result, err := s.k.GetProgramResult(s.ctx, observation.ProgramID, requestID)
		require.NoError(t, err)
		require.Equal(t, observation.ContentDigest, result.ContentDigest)
		require.Equal(t, observation.FullLength, result.FullLength)
		require.Equal(t, uint32(len(observation.Response)), result.ResponseLength)
		require.Equal(t, observation.Response, result.Response)
	}
	require.Equal(t, before, s.state())
}
