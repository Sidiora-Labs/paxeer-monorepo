package keeper

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/stretchr/testify/require"
)

func TestGetProgramResultMatchesTheKernelCommittedRead(t *testing.T) {
	fixture := newKernelProgramFixture(t)
	s, observation := fixture.suite, fixture.observation
	raw, err := types.EncodeObservation(observation)
	require.NoError(t, err)
	require.NoError(t, s.k.FulfilProgram(s.ctx, raw))

	result, err := s.k.GetProgramResult(s.ctx, observation.ProgramID, observation.RequestID)
	require.NoError(t, err)
	require.Equal(t, observation.FullLength, result.FullLength)
	require.Equal(t, uint32(16), result.FullLength)
	require.Equal(t, uint32(16), result.ResponseLength)
	require.Len(t, result.Response, 16)
	require.Equal(t, []byte("Paxeer X Network"), result.Response)
	require.Equal(t, observation.ContentDigest, result.ContentDigest)

	t.Run("request_id_plus_one", func(t *testing.T) {
		_, err := s.k.GetProgramResult(s.ctx, observation.ProgramID, observation.RequestID+1)
		require.ErrorIs(t, err, errProgramUnknownField)
		require.EqualError(t, err, "LXP_UNKNOWN_FIELD")
	})
	t.Run("flipped_program_id", func(t *testing.T) {
		programID := observation.ProgramID
		programID[0] ^= 1
		_, err := s.k.GetProgramResult(s.ctx, programID, observation.RequestID)
		require.ErrorIs(t, err, errProgramUnknownField)
		require.EqualError(t, err, "LXP_UNKNOWN_FIELD")
	})
}
