package layerxprograms_test

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/stretchr/testify/require"
)

func TestEncodeTransferPayloadLegs(t *testing.T) {
	var programID, from, asset, to [32]byte
	for i := range programID {
		programID[i] = byte(i + 1)
		from[i] = byte(i + 33)
		asset[i] = byte(i + 65)
		to[i] = byte(i + 97)
	}
	amountBytes := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	maxAmount := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	legs := []layerxprograms.TransferLeg{
		{From: from, Asset: asset, To: to, Amount: new(big.Int).SetBytes(amountBytes)},
		{From: to, Asset: from, To: asset, Amount: maxAmount},
		{From: asset, Asset: to, To: from, Amount: big.NewInt(1)},
		{From: from, Asset: to, To: asset, Amount: new(big.Int)},
	}
	amounts := make([]*big.Int, len(legs))
	for i := range legs {
		amounts[i] = new(big.Int).Set(legs[i].Amount)
	}
	payload, err := layerxprograms.EncodeTransferPayload(programID, legs)
	require.NoError(t, err)
	require.Len(t, payload, 34+112*len(legs))
	want := append([]byte{}, programID[:]...)
	want = append(want, 0, 4)
	encodedAmounts := [][]byte{
		amountBytes,
		bytes.Repeat([]byte{0xff}, 16),
		append(make([]byte, 15), 1),
		make([]byte, 16),
	}
	for i, leg := range legs {
		want = append(want, leg.From[:]...)
		want = append(want, leg.Asset[:]...)
		want = append(want, leg.To[:]...)
		want = append(want, encodedAmounts[i]...)
		require.Zero(t, leg.Amount.Cmp(amounts[i]), "amount %d changed", i)
	}
	require.Equal(t, want, payload)

	t.Run("two leg acceptance vector", func(t *testing.T) {
		pair := []layerxprograms.TransferLeg{legs[2], legs[1]}
		payload, err := layerxprograms.EncodeTransferPayload(programID, pair)
		require.NoError(t, err)
		require.Len(t, payload, 258)
		require.Equal(t, programID[:], payload[:32])
		require.Equal(t, []byte{0, 2}, payload[32:34])
		require.Equal(t, want[258:370], payload[34:146])
		require.Equal(t, want[146:258], payload[146:258])
		require.Equal(t, encodedAmounts[2], payload[130:146])
		require.Equal(t, encodedAmounts[1], payload[242:258])
	})

	t.Run("single leg", func(t *testing.T) {
		payload, err := layerxprograms.EncodeTransferPayload(programID, legs[:1])
		require.NoError(t, err)
		require.Len(t, payload, 146)
		require.Equal(t, programID[:], payload[:32])
		require.Equal(t, []byte{0, 1}, payload[32:34])
		require.Equal(t, want[34:146], payload[34:])
	})

	t.Run("maximum leg count", func(t *testing.T) {
		many := make([]layerxprograms.TransferLeg, 256)
		for i := range many {
			many[i] = legs[0]
		}
		payload, err := layerxprograms.EncodeTransferPayload(programID, many)
		require.NoError(t, err)
		require.Len(t, payload, 34+112*256)
		require.Equal(t, programID[:], payload[:32])
		require.Equal(t, []byte{1, 0}, payload[32:34])
		require.Equal(t, bytes.Repeat(want[34:146], 256), payload[34:])
	})

	for _, count := range []int{257, 65536} {
		t.Run(fmt.Sprintf("invalid leg count %d", count), func(t *testing.T) {
			many := make([]layerxprograms.TransferLeg, count)
			for i := range many {
				many[i] = legs[0]
			}
			payload, err := layerxprograms.EncodeTransferPayload(programID, many)
			require.EqualError(t, err, "LXP_LENGTH_LIMIT")
			require.Nil(t, payload)
		})
	}
	payload, err = layerxprograms.EncodeTransferPayload(programID, nil)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	require.Nil(t, payload)
	payload, err = layerxprograms.EncodeTransferPayload(programID, []layerxprograms.TransferLeg{})
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	require.Nil(t, payload)

	for _, tc := range []struct {
		name   string
		amount *big.Int
		code   string
	}{
		{"nil amount", nil, "LXP_INVALID_AMOUNT"},
		{"negative amount", big.NewInt(-1), "LXP_INVALID_AMOUNT"},
		{"u128 overflow", new(big.Int).Lsh(big.NewInt(1), 128), "LXP_OVERFLOW"},
		{"u256 amount", new(big.Int).Lsh(big.NewInt(1), 255), "LXP_OVERFLOW"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := append([]layerxprograms.TransferLeg(nil), legs...)
			invalid[len(invalid)-1].Amount = tc.amount
			payload, err := layerxprograms.EncodeTransferPayload(programID, invalid)
			require.EqualError(t, err, tc.code)
			require.Nil(t, payload)
		})
	}
}

func TestEncodeAccountPayloadProfiles(t *testing.T) {
	var programID, assetID [32]byte
	for i := range programID {
		programID[i] = byte(i + 1)
		assetID[i] = byte(255 - i)
	}
	for _, profile := range []uint8{1, 2} {
		for _, seedLength := range []int{0, 1, 4, 17, 128} {
			t.Run(fmt.Sprintf("profile %d seed %d", profile, seedLength), func(t *testing.T) {
				seed := make([]byte, seedLength)
				for i := range seed {
					seed[i] = byte(i * 7)
				}
				originalSeed := append([]byte{}, seed...)
				payload, err := layerxprograms.EncodeAccountPayload(programID, profile, assetID, seed)
				require.NoError(t, err)
				require.Len(t, payload, 73+seedLength)
				magic := "LXPA1"
				if profile == 2 {
					magic = "LXPA2"
				}
				want := append([]byte{}, programID[:]...)
				want = append(want, []byte(magic)...)
				want = append(want, assetID[:]...)
				want = append(want, 0, 0, 0, byte(seedLength))
				want = append(want, seed...)
				require.Equal(t, want, payload)
				require.Equal(t, originalSeed, seed)
				if len(seed) > 0 {
					seed[0] ^= 0xff
					require.Equal(t, want, payload, "payload must own the seed bytes")
				}
			})
		}
		t.Run(fmt.Sprintf("profile %d nil seed", profile), func(t *testing.T) {
			payload, err := layerxprograms.EncodeAccountPayload(programID, profile, assetID, nil)
			require.NoError(t, err)
			require.Len(t, payload, 73)
			require.Equal(t, []byte{0, 0, 0, 0}, payload[69:73])
		})
		t.Run(fmt.Sprintf("profile %d oversized seed", profile), func(t *testing.T) {
			payload, err := layerxprograms.EncodeAccountPayload(programID, profile, assetID, make([]byte, 129))
			require.EqualError(t, err, "LXP_NON_CANONICAL")
			require.Nil(t, payload)
		})
	}
	for profile := 0; profile <= 255; profile++ {
		if profile == 1 || profile == 2 {
			continue
		}
		payload, err := layerxprograms.EncodeAccountPayload(programID, uint8(profile), assetID, nil)
		require.EqualError(t, err, "LXP_NON_CANONICAL", "profile %d", profile)
		require.Nil(t, payload, "profile %d", profile)
	}
}
