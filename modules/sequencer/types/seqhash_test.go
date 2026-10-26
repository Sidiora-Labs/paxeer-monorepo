package types_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

func TestSeqHashGoldenVector(t *testing.T) {
	require.Equal(t, "PAXEER/v1/sequence\x00", types.SeqHashDomain)

	var prev, activityID [32]byte
	for i := range prev {
		prev[i] = byte(i)
		activityID[i] = byte(i + 32)
	}

	got := types.SeqHash(prev, 0x0102030405060708, activityID)
	require.Equal(t, mustHex(t, "7a5ead0b5ee92e71a50501e928b3dd33b02abd4e9f3b78638d0fa9e31df9ce69"), got[:])
}

func TestSeqHashChainOrderSensitive(t *testing.T) {
	activities := [3][32]byte{{0x11}, {0x22}, {0x33}}
	swapped := [3][32]byte{activities[1], activities[0], activities[2]}
	var originalHash, swappedHash [32]byte
	for i := range activities {
		seq := uint64(i + 1)
		originalHash = types.SeqHash(originalHash, seq, activities[i])
		swappedHash = types.SeqHash(swappedHash, seq, swapped[i])
	}

	require.NotEqual(t, originalHash, swappedHash)
}
