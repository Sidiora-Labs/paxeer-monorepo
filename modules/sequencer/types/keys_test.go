package types_test

import (
	"bytes"
	"encoding/hex"
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

func TestSequencerStoreKeysDistinct(t *testing.T) {
	keys := []struct {
		name string
		key  []byte
	}{
		{"LastSeqKey", types.LastSeqKey},
		{"LastSeqHashKey", types.LastSeqHashKey},
		{"ReceiptKeysKey", types.ReceiptKeysKey},
		{"LeaderListKey", types.LeaderListKey},
		{"StoredVotesKey", types.StoredVotesKey},
		// The keeper-private lastBatchHeightKey reserves 0x06.
		{"lastBatchHeightKey", []byte{0x06}},
		{"ActivationHeightKey", types.ActivationHeightKey},
	}
	seen := make(map[string]string)
	for i, key := range keys {
		require.Equal(t, []byte{byte(i + 1)}, key.key, key.name)
		require.NotContains(t, seen, string(key.key), key.name)
		seen[string(key.key)] = key.name
	}
}

func TestStoredVotesKeyAtOrdersByHeight(t *testing.T) {
	cases := []struct {
		height int64
		keyHex string
	}{
		{1, "050000000000000001"},
		{2, "050000000000000002"},
		{255, "0500000000000000ff"},
		{256, "050000000000000100"},
		{65535, "05000000000000ffff"},
		{65536, "050000000000010000"},
		{1 << 32, "050000000100000000"},
		{math.MaxInt64, "057fffffffffffffff"},
	}
	var previous []byte
	for _, tc := range cases {
		key := types.StoredVotesKeyAt(tc.height)
		require.Len(t, key, 9)
		require.Equal(t, tc.keyHex, hex.EncodeToString(key), "height %d", tc.height)
		require.True(t, bytes.HasPrefix(key, types.StoredVotesKey))
		require.NotEqual(t, types.StoredVotesKey, key)
		if previous != nil {
			require.Less(t, bytes.Compare(previous, key), 0, "height %d", tc.height)
		}
		previous = key
	}

	key := types.StoredVotesKeyAt(1)
	other := types.StoredVotesKeyAt(1)
	key[0] = 0xff
	key[8] = 0xff
	require.Equal(t, []byte{0x05}, types.StoredVotesKey)
	require.Equal(t, "050000000000000001", hex.EncodeToString(other))
	require.Equal(t, other, types.StoredVotesKeyAt(1))
}
