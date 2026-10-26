package types_test

import (
	"math"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	"github.com/stretchr/testify/require"
)

func TestActiveIndexKeysAreDisjoint(t *testing.T) {
	prefixes := [][]byte{
		types.ParamsKey,
		types.BookPrefix,
		types.EscrowPrefix,
		types.FeeAccrualPrefix,
		types.NextOrderIDKey,
		types.LimitOrderPrefix,
		types.StopLimitOrderPrefix,
		types.UserOrderPrefix,
		types.ActiveLimitPrefix,
		types.ActiveStopLimitPrefix,
		types.ActiveStopLossPrefix,
		types.TwapSamplePrefix,
		types.TwapRingPrefix,
		types.VolumePrefix,
		types.StopLossOrderPrefix,
		types.OrderMinOutputPrefix,
		types.ExpiryIndexPrefix,
		types.CursorPrefix,
		types.FillBackoffPrefix,
		types.ActiveCountPrefix,
		types.ActivePositionPrefix,
		types.ActiveLengthPrefix,
	}
	seenPrefixes := make(map[byte]bool)
	for i, prefix := range prefixes {
		require.Equal(t, []byte{byte(i + 1)}, prefix)
		require.False(t, seenPrefixes[prefix[0]], "duplicate prefix %x", prefix)
		seenPrefixes[prefix[0]] = true
	}

	require.Equal(t, []byte{0x16, 0x09}, types.ActiveLengthKey(types.ActiveLimitPrefix))

	positions := []struct {
		position uint64
		encoded  [8]byte
	}{
		{0, [8]byte{}},
		{1, [8]byte{0, 0, 0, 0, 0, 0, 0, 1}},
		{math.MaxUint64, [8]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{0x0102030405060708, [8]byte{1, 2, 3, 4, 5, 6, 7, 8}},
	}
	seenKeys := make(map[string]bool)
	for _, list := range [][]byte{types.ActiveLimitPrefix, types.ActiveStopLimitPrefix, types.ActiveStopLossPrefix} {
		lengthKey := types.ActiveLengthKey(list)
		require.Equal(t, []byte{0x16, list[0]}, lengthKey)
		require.False(t, seenKeys[string(lengthKey)], "duplicate key %x", lengthKey)
		seenKeys[string(lengthKey)] = true
		for _, tc := range positions {
			key := types.ActivePositionKey(list, tc.position)
			want := append([]byte{0x15, list[0]}, tc.encoded[:]...)
			require.Equal(t, want, key)
			require.False(t, seenKeys[string(key)], "duplicate key %x", key)
			seenKeys[string(key)] = true
		}
	}
}
