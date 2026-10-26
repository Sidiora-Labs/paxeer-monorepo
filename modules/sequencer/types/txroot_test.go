package types_test

import (
	"crypto/sha256"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

func TestTxRootEmptyEqualsEmptyTxRoot(t *testing.T) {
	require.Equal(t, types.EmptyTxRoot, types.TxRoot(nil))
	require.Equal(t, types.EmptyTxRoot, types.TxRoot([][32]byte{}))
}

func TestTxRootOrderSensitive(t *testing.T) {
	first := sha256.Sum256([]byte("first activity"))
	second := sha256.Sum256([]byte("second activity"))
	ids := [][32]byte{first, second}

	var concatenated [64]byte
	copy(concatenated[:32], first[:])
	copy(concatenated[32:], second[:])
	root := types.TxRoot(ids)
	require.Equal(t, sha256.Sum256(concatenated[:]), root)

	copy(concatenated[:32], second[:])
	copy(concatenated[32:], first[:])
	reversedRoot := types.TxRoot([][32]byte{second, first})
	require.Equal(t, sha256.Sum256(concatenated[:]), reversedRoot)
	require.NotEqual(t, root, reversedRoot)
}
