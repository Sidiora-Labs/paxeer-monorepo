package types_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/stretchr/testify/require"
)

func TestStoredVotesCodecRoundTrip(t *testing.T) {
	votes := []abci.VoteInfo{
		{Validator: abci.Validator{Address: bytes.Repeat([]byte{0x11}, 20), Power: 1}, SignedLastBlock: true},
		{Validator: abci.Validator{Address: bytes.Repeat([]byte{0x22}, 20), Power: 10}, SignedLastBlock: false},
		{Validator: abci.Validator{Address: bytes.Repeat([]byte{0x33}, 20), Power: 100}, SignedLastBlock: true},
	}
	bz, err := types.EncodeStoredVotes(votes)
	require.NoError(t, err)
	expected := mustHex(t, "00000003"+
		"14"+"1111111111111111111111111111111111111111"+"0000000000000001"+"01"+
		"14"+"2222222222222222222222222222222222222222"+"000000000000000a"+"00"+
		"14"+"3333333333333333333333333333333333333333"+"0000000000000064"+"01")
	require.Equal(t, expected, bz)
	decoded, err := types.DecodeStoredVotes(bz)
	require.NoError(t, err)
	require.Equal(t, votes, decoded)
	bz[5] ^= 0xff
	require.Equal(t, votes, decoded, "decoded addresses must own their bytes")

	for _, empty := range [][]abci.VoteInfo{nil, {}} {
		bz, err := types.EncodeStoredVotes(empty)
		require.NoError(t, err)
		require.Equal(t, []byte{0, 0, 0, 0}, bz)
		decoded, err := types.DecodeStoredVotes(bz)
		require.NoError(t, err)
		require.Empty(t, decoded)
	}

	for _, power := range []int64{math.MinInt64, -1, 0, math.MaxInt64} {
		for _, addressLen := range []int{0, 1, 255} {
			t.Run(fmt.Sprintf("power=%d/address=%d", power, addressLen), func(t *testing.T) {
				vote := abci.VoteInfo{
					Validator:       abci.Validator{Address: bytes.Repeat([]byte{0xa5}, addressLen), Power: power},
					SignedLastBlock: true,
				}
				bz, err := types.EncodeStoredVotes([]abci.VoteInfo{vote})
				require.NoError(t, err)
				require.Len(t, bz, 4+1+addressLen+8+1)
				require.Equal(t, []byte{0, 0, 0, 1, byte(addressLen)}, bz[:5])
				require.Equal(t, uint64(power), binary.BigEndian.Uint64(bz[5+addressLen:]))
				decoded, err := types.DecodeStoredVotes(bz)
				require.NoError(t, err)
				require.Equal(t, []abci.VoteInfo{vote}, decoded)
			})
		}
	}
}

func TestStoredVotesCodecRefusesMalformed(t *testing.T) {
	votes := []abci.VoteInfo{
		{Validator: abci.Validator{Address: bytes.Repeat([]byte{0x11}, 20), Power: 1}, SignedLastBlock: true},
		{Validator: abci.Validator{Address: bytes.Repeat([]byte{0x22}, 20), Power: 10}, SignedLastBlock: false},
	}
	bz, err := types.EncodeStoredVotes(votes)
	require.NoError(t, err)
	for n := 0; n < len(bz); n++ {
		t.Run(fmt.Sprintf("truncated-at-%d", n), func(t *testing.T) {
			decoded, err := types.DecodeStoredVotes(bz[:n])
			require.Error(t, err)
			require.Nil(t, decoded)
		})
	}
	for _, flagOffset := range []int{33, 63} {
		for flag := 2; flag <= 255; flag++ {
			invalid := bytes.Clone(bz)
			invalid[flagOffset] = byte(flag)
			decoded, err := types.DecodeStoredVotes(invalid)
			require.Error(t, err, "flag %d at offset %d", flag, flagOffset)
			require.Nil(t, decoded)
		}
	}

	for name, malformed := range map[string][]byte{
		"trailing byte":       append(bytes.Clone(bz), 0),
		"empty with trailing": {0, 0, 0, 0, 0},
		"excessive count":     {0xff, 0xff, 0xff, 0xff},
		"count too small":     append([]byte{0, 0, 0, 1}, bz[4:]...),
		"count too large":     append([]byte{0, 0, 0, 3}, bz[4:]...),
		"address truncated":   append([]byte{0, 0, 0, 1, 255}, make([]byte, 9)...),
		"missing next entry":  append([]byte{0, 0, 0, 2, 50}, make([]byte, 59)...),
	} {
		t.Run(name, func(t *testing.T) {
			decoded, err := types.DecodeStoredVotes(malformed)
			require.Error(t, err)
			require.Nil(t, decoded)
		})
	}

	encoded, err := types.EncodeStoredVotes([]abci.VoteInfo{
		votes[0],
		{Validator: abci.Validator{Address: make([]byte, 256), Power: 1}},
	})
	require.Error(t, err)
	require.Nil(t, encoded)
}
