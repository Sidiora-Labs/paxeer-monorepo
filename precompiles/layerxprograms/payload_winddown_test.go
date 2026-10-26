package layerxprograms_test

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/stretchr/testify/require"
)

func TestEncodeWindDownRouteAndDeprecate(t *testing.T) {
	programId := [32]byte(bytes.Repeat([]byte{7}, 32))
	account := [32]byte(bytes.Repeat([]byte{8}, 32))
	asset := [32]byte(bytes.Repeat([]byte{9}, 32))
	destination := [32]byte(bytes.Repeat([]byte{10}, 32))
	routePrefix := strings.Repeat("07", 32) + "01" + strings.Repeat("08", 32) +
		strings.Repeat("09", 32) + strings.Repeat("0a", 32)

	t.Run("route golden", func(t *testing.T) {
		seed := []byte{0xaa, 0xbb, 0xcc}
		payload, err := layerxprograms.EncodeSetExitRoutePayload(programId, account, asset, destination, seed)
		require.NoError(t, err)
		require.Len(t, payload, 134)
		require.Equal(t, routePrefix+"0003aabbcc", hex.EncodeToString(payload))
		seed[0] = 0xff
		require.Equal(t, byte(0xaa), payload[131])
	})

	t.Run("seed bounds", func(t *testing.T) {
		for _, seed := range [][]byte{nil, {}, bytes.Repeat([]byte{0xab}, 128)} {
			payload, err := layerxprograms.EncodeSetExitRoutePayload(programId, account, asset, destination, seed)
			require.NoError(t, err)
			lengthHex := "0000"
			if len(seed) == 128 {
				lengthHex = "0080"
			}
			require.Equal(t, routePrefix+lengthHex+hex.EncodeToString(seed), hex.EncodeToString(payload))
		}
		payload, err := layerxprograms.EncodeSetExitRoutePayload(programId, account, asset, destination, make([]byte, 129))
		require.ErrorContains(t, err, "LXP_LENGTH_LIMIT:")
		require.Nil(t, payload)
	})

	t.Run("deprecate golden and u64 deadlines", func(t *testing.T) {
		for _, tc := range []struct {
			deadline uint64
			hex      string
		}{
			{77, "000000000000004d"},
			{0, "0000000000000000"},
			{0x0102030405060708, "0102030405060708"},
			{^uint64(0), "ffffffffffffffff"},
		} {
			payload, err := layerxprograms.EncodeDeprecatePayload(programId, programId, tc.deadline)
			require.NoError(t, err)
			require.Len(t, payload, 73)
			require.Equal(t, strings.Repeat("07", 32)+"02"+strings.Repeat("07", 32)+tc.hex, hex.EncodeToString(payload))
		}
	})

	t.Run("foreign exit program refused", func(t *testing.T) {
		for _, exitProgram := range [][32]byte{account, {}} {
			payload, err := layerxprograms.EncodeDeprecatePayload(programId, exitProgram, 77)
			require.ErrorContains(t, err, "LXP_PROGRAM_REFUSED:")
			require.Nil(t, payload)
		}
	})

	t.Run("zero program refused", func(t *testing.T) {
		payload, err := layerxprograms.EncodeSetExitRoutePayload([32]byte{}, account, asset, destination, nil)
		require.ErrorContains(t, err, "LXP_NON_CANONICAL:")
		require.Nil(t, payload)
		payload, err = layerxprograms.EncodeDeprecatePayload([32]byte{}, [32]byte{}, 77)
		require.ErrorContains(t, err, "LXP_NON_CANONICAL:")
		require.Nil(t, payload)
	})
}

func TestEncodeWindDownTombstoneAndExits(t *testing.T) {
	programId := [32]byte(bytes.Repeat([]byte{7}, 32))
	account := [32]byte(bytes.Repeat([]byte{8}, 32))
	programHex := strings.Repeat("07", 32)
	accountHex := strings.Repeat("08", 32)

	t.Run("tombstone golden", func(t *testing.T) {
		payload, err := layerxprograms.EncodeTombstonePayload(programId)
		require.NoError(t, err)
		require.Len(t, payload, 33)
		require.Equal(t, programHex+"03", hex.EncodeToString(payload))
	})

	t.Run("exit golden", func(t *testing.T) {
		payload, err := layerxprograms.EncodeExitValuePayload(programId, account)
		require.NoError(t, err)
		require.Len(t, payload, 65)
		require.Equal(t, programHex+"04"+accountHex, hex.EncodeToString(payload))
	})

	t.Run("bounded exit big endian u128", func(t *testing.T) {
		for _, amountHex := range []string{
			"00000000000000000000000000000009",
			"0102030405060708090a0b0c0d0e0f10",
			"ffffffffffffffffffffffffffffffff",
		} {
			amount, ok := new(big.Int).SetString(amountHex, 16)
			require.True(t, ok)
			original := new(big.Int).Set(amount)
			payload, err := layerxprograms.EncodeExitValueBoundedPayload(programId, account, amount)
			require.NoError(t, err)
			require.Len(t, payload, 81)
			require.Equal(t, programHex+"05"+accountHex+amountHex, hex.EncodeToString(payload))
			require.Zero(t, original.Cmp(amount))
		}
	})

	t.Run("invalid maximum refused", func(t *testing.T) {
		for _, amount := range []*big.Int{nil, new(big.Int), big.NewInt(-1)} {
			payload, err := layerxprograms.EncodeExitValueBoundedPayload(programId, account, amount)
			require.ErrorContains(t, err, "LXP_NON_CANONICAL:")
			require.Nil(t, payload)
		}
		for _, bits := range []uint{128, 256} {
			payload, err := layerxprograms.EncodeExitValueBoundedPayload(programId, account, new(big.Int).Lsh(big.NewInt(1), bits))
			require.ErrorContains(t, err, "LXP_OVERFLOW:")
			require.Nil(t, payload)
		}
	})

	t.Run("zero program refused", func(t *testing.T) {
		payload, err := layerxprograms.EncodeTombstonePayload([32]byte{})
		require.ErrorContains(t, err, "LXP_NON_CANONICAL:")
		require.Nil(t, payload)
		payload, err = layerxprograms.EncodeExitValuePayload([32]byte{}, account)
		require.ErrorContains(t, err, "LXP_NON_CANONICAL:")
		require.Nil(t, payload)
		payload, err = layerxprograms.EncodeExitValueBoundedPayload([32]byte{}, account, big.NewInt(9))
		require.ErrorContains(t, err, "LXP_NON_CANONICAL:")
		require.Nil(t, payload)
	})
}
