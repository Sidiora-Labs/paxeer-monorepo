package layerxprograms_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/stretchr/testify/require"
)

func TestEncodeCallPayloadMatchesKernelDecoder(t *testing.T) {
	t.Run("kernel_call_activity_golden", func(t *testing.T) {
		var programID [32]byte
		copy(programID[:], bytes.Repeat([]byte{0x31}, 32))
		budget := [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096}
		payload, err := layerxprograms.EncodeCallPayload(programID, 1, "layerx_call", nil, []byte{0, 0}, []byte("LayerX/programs/access-declaration/v1\x00\x00"), 16, budget)
		require.NoError(t, err)
		golden, err := hex.DecodeString(
			"3131313131313131313131313131313131313131313131313131313131313131" +
				"0001000b0000000000020000002700000010" +
				"00000000000f4240000000000100000000000000001000000000000000100000" +
				"000000000000004000000000001000000000000000001000" +
				"6c61796572785f63616c6c0000" +
				"4c61796572582f70726f6772616d732f6163636573732d6465636c61726174696f6e2f76310000")
		require.NoError(t, err)
		require.Equal(t, golden, payload)
	})

	t.Run("all_fields", func(t *testing.T) {
		var programID [32]byte
		for i := range programID {
			programID[i] = byte(i + 1)
		}
		entrypoint := "Az09_.call"
		calldata := bytes.Repeat([]byte{0x12, 0x34, 0x00}, 87)
		capabilities := bytes.Repeat([]byte{0x56, 0x78}, 130)
		access := bytes.Repeat([]byte{0x9a, 0xbc, 0xff}, 89)
		budget := [7]uint64{0x0102030405060708, 0x1122334455667788, math.MaxUint64, 0, 256, 0x8070605040302010, 0xfedcba9876543210}
		payload, err := layerxprograms.EncodeCallPayload(programID, 0x1234, entrypoint, calldata, capabilities, access, 0x87654321, budget)
		require.NoError(t, err)
		require.Len(t, payload, 106+len(entrypoint)+len(calldata)+len(capabilities)+len(access))
		require.Equal(t, programID[:], payload[:32])
		require.Equal(t, uint16(0x1234), binary.BigEndian.Uint16(payload[32:34]))
		require.Equal(t, uint16(len(entrypoint)), binary.BigEndian.Uint16(payload[34:36]))
		require.Equal(t, uint32(len(calldata)), binary.BigEndian.Uint32(payload[36:40]))
		require.Equal(t, uint16(len(capabilities)), binary.BigEndian.Uint16(payload[40:42]))
		require.Equal(t, uint32(len(access)), binary.BigEndian.Uint32(payload[42:46]))
		require.Equal(t, uint32(0x87654321), binary.BigEndian.Uint32(payload[46:50]))
		for i, word := range budget {
			require.Equal(t, word, binary.BigEndian.Uint64(payload[50+i*8:58+i*8]), "budget word %d", i)
		}
		cursor := 106
		require.Equal(t, []byte(entrypoint), payload[cursor:cursor+len(entrypoint)])
		cursor += len(entrypoint)
		require.Equal(t, calldata, payload[cursor:cursor+len(calldata)])
		cursor += len(calldata)
		require.Equal(t, capabilities, payload[cursor:cursor+len(capabilities)])
		cursor += len(capabilities)
		require.Equal(t, access, payload[cursor:])
	})

	t.Run("entrypoint_and_transport_boundaries", func(t *testing.T) {
		for _, entrypoint := range []string{"a", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_.", strings.Repeat("a", 128)} {
			payload, err := layerxprograms.EncodeCallPayload([32]byte{1}, 1, entrypoint, nil, nil, nil, 0, [7]uint64{})
			require.NoError(t, err)
			require.Len(t, payload, 106+len(entrypoint))
			require.Equal(t, uint16(len(entrypoint)), binary.BigEndian.Uint16(payload[34:36]))
			require.Equal(t, make([]byte, 70), payload[36:106])
			require.Equal(t, entrypoint, string(payload[106:]))
		}
		capabilities := bytes.Repeat([]byte{0xa5}, math.MaxUint16)
		payload, err := layerxprograms.EncodeCallPayload([32]byte{1}, 1, "a", nil, capabilities, nil, 0, [7]uint64{})
		require.NoError(t, err)
		require.Equal(t, uint16(math.MaxUint16), binary.BigEndian.Uint16(payload[40:42]))
		require.Equal(t, capabilities, payload[107:])
		payload, err = layerxprograms.EncodeCallPayload([32]byte{1}, 1, "a", nil, append(capabilities, 0xa5), nil, 0, [7]uint64{})
		require.EqualError(t, err, "LXP_LENGTH_LIMIT")
		require.Nil(t, payload)
	})
}

func TestEncodeCallPayloadRefusesInvalidEntrypoint(t *testing.T) {
	for _, entrypoint := range []string{"", strings.Repeat("a", 129), strings.Repeat("a", 65537), "caf\u00e9", "call\x00suffix", "call-name", "call/name", "call name"} {
		t.Run(fmt.Sprintf("entrypoint_%q", entrypoint[:min(len(entrypoint), 16)]), func(t *testing.T) {
			payload, err := layerxprograms.EncodeCallPayload([32]byte{1}, 1, entrypoint, nil, nil, nil, 16, [7]uint64{})
			require.EqualError(t, err, "LXP_NON_CANONICAL")
			require.Nil(t, payload)
		})
	}
	for b := 0; b < 256; b++ {
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '.' {
			continue
		}
		t.Run(fmt.Sprintf("invalid_byte_%02x", b), func(t *testing.T) {
			payload, err := layerxprograms.EncodeCallPayload([32]byte{1}, 1, "call"+string([]byte{byte(b)})+"suffix", nil, nil, nil, 16, [7]uint64{})
			require.EqualError(t, err, "LXP_NON_CANONICAL")
			require.Nil(t, payload)
		})
	}
}
