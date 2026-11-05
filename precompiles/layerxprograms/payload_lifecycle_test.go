package layerxprograms_test

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func lifecycleIDs() (program, authority, oldHash, newHash [32]byte) {
	for i := range program {
		program[i] = byte(i)
		authority[i] = byte(i + 32)
		oldHash[i] = byte(i + 64)
		newHash[i] = byte(i + 96)
	}
	return
}

func requireLifecycleGolden(t *testing.T, payload []byte, err error, golden string) {
	t.Helper()
	require.NoError(t, err)
	want, decodeErr := hex.DecodeString(golden)
	require.NoError(t, decodeErr)
	require.Equal(t, want, payload)
}

func requireLifecycleRefusal(t *testing.T, payload []byte, err error, code string) {
	t.Helper()
	require.Error(t, err)
	require.Nil(t, payload)
	reason, decodeErr := abi.UnpackRevert(pcommon.LXPReason(err))
	require.NoError(t, decodeErr)
	require.True(t, strings.HasPrefix(reason, code+": "), "unexpected revert: %s", reason)
}

func TestEncodeDeployPayloadLayout(t *testing.T) {
	program, authority, codeHash, _ := lifecycleIDs()
	wasm := []byte{0, 0x61, 0x73, 0x6d, 1, 0, 0, 0}
	for _, tc := range []struct {
		name      string
		policy    uint8
		authority [32]byte
		iface     []byte
		golden    string
	}{
		{
			name: "legacy authority", policy: 1, authority: authority,
			golden: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
				"12340100" +
				"202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f" +
				"404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f" +
				"000000080061736d01000000",
		},
		{
			name: "legacy immutable empty interface", iface: []byte{},
			golden: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
				"12340000" +
				"0000000000000000000000000000000000000000000000000000000000000000" +
				"404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f" +
				"000000080061736d01000000",
		},
		{
			name: "framed authority", policy: 1, authority: authority, iface: []byte{0xfe, 0, 0xff},
			golden: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
				"12340100" +
				"202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f" +
				"404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f" +
				"0000000800000003fe00ff0061736d01000000",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := layerxprograms.EncodeDeployPayload(program, 0x1234, tc.policy, tc.authority, codeHash, tc.iface, wasm)
			requireLifecycleGolden(t, payload, err, tc.golden)
			payload[len(payload)-1] = 0xff
			require.Equal(t, byte(0), wasm[len(wasm)-1])
			if len(tc.iface) != 0 {
				payload[108] = 0
				require.Equal(t, byte(0xfe), tc.iface[0])
			}
		})
	}
	for _, tc := range []struct {
		name        string
		program     [32]byte
		policy      uint8
		authority   [32]byte
		iface, wasm []byte
		code        string
	}{
		{"zero program", [32]byte{}, 1, authority, nil, wasm, "LXP_NON_CANONICAL"},
		{"unknown policy", program, 2, authority, nil, wasm, "LXP_NON_CANONICAL"},
		{"immutable with authority", program, 0, authority, nil, wasm, "LXP_NON_CANONICAL"},
		{"authority missing", program, 1, [32]byte{}, nil, wasm, "LXP_NON_CANONICAL"},
		{"empty wasm", program, 1, authority, nil, nil, "LXP_NON_CANONICAL"},
		{"framed empty wasm", program, 1, authority, []byte{1}, nil, "LXP_NON_CANONICAL"},
		{"oversize interface", program, 1, authority, make([]byte, 953), wasm, "LXP_LENGTH_LIMIT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := layerxprograms.EncodeDeployPayload(tc.program, 1, tc.policy, tc.authority, codeHash, tc.iface, tc.wasm)
			requireLifecycleRefusal(t, payload, err, tc.code)
		})
	}
	t.Run("maximum interface", func(t *testing.T) {
		iface := make([]byte, 952)
		payload, err := layerxprograms.EncodeDeployPayload(program, 0xffff, 1, authority, codeHash, iface, wasm)
		require.NoError(t, err)
		require.Len(t, payload, 108+952+len(wasm))
		require.Equal(t, uint16(0xffff), binary.BigEndian.Uint16(payload[32:34]))
		require.Equal(t, uint32(952), binary.BigEndian.Uint32(payload[104:108]))
		require.Equal(t, iface, payload[108:1060])
		require.Equal(t, wasm, payload[1060:])
	})
}

func TestEncodeUpgradePayloadLayout(t *testing.T) {
	program, _, oldHash, newHash := lifecycleIDs()
	wasm := []byte{0, 0x61, 0x73, 0x6d, 1, 0, 0, 0}
	const id = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
	const hashes = "404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f" +
		"606162636465666768696a6b6c6d6e6f707172737475767778797a7b7c7d7e7f"
	for _, tc := range []struct {
		name        string
		flags       uint8
		hook, iface []byte
		golden      string
	}{
		{"legacy no hook", 0, nil, nil, id + "12340000" + hashes + "0000000000080061736d01000000"},
		{"legacy hook", 1, []byte{0xaa, 0xbb}, nil, id + "12340100" + hashes + "000200000008aabb0061736d01000000"},
		{"framed hook", 1, []byte{0xaa, 0xbb}, []byte{0xfe, 0, 0xff}, id + "12340100" + hashes + "00020000000800000003aabbfe00ff0061736d01000000"},
		{"framed no hook", 0, nil, []byte{0xfe, 0, 0xff}, id + "12340000" + hashes + "00000000000800000003fe00ff0061736d01000000"},
		{"clear interface", 2, nil, nil, id + "12340200" + hashes + "000000000008000000000061736d01000000"},
		{"clear interface with hook", 3, []byte{0xaa, 0xbb}, nil, id + "12340300" + hashes + "00020000000800000000aabb0061736d01000000"},
		{"flag two with interface", 2, nil, []byte{0xfe, 0, 0xff}, id + "12340200" + hashes + "00000000000800000003fe00ff0061736d01000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := layerxprograms.EncodeUpgradePayload(program, 0x1234, tc.flags, oldHash, newHash, tc.hook, tc.iface, wasm)
			requireLifecycleGolden(t, payload, err, tc.golden)
			payload[len(payload)-1] = 0xff
			require.Equal(t, byte(0), wasm[len(wasm)-1])
			if len(tc.hook) != 0 {
				offset := 106
				if len(tc.iface) != 0 || tc.flags&2 != 0 {
					offset = 110
				}
				payload[offset] = 0
				require.Equal(t, byte(0xaa), tc.hook[0])
			}
		})
	}
	for _, tc := range []struct {
		name              string
		program           [32]byte
		flags             uint8
		hook, iface, wasm []byte
		code              string
	}{
		{"zero program", [32]byte{}, 0, nil, nil, wasm, "LXP_NON_CANONICAL"},
		{"hook flag without hook", program, 1, nil, nil, wasm, "LXP_NON_CANONICAL"},
		{"hook without flag", program, 0, []byte{1}, nil, wasm, "LXP_NON_CANONICAL"},
		{"framed hook flag without hook", program, 3, nil, []byte{1}, wasm, "LXP_NON_CANONICAL"},
		{"framed hook without flag", program, 2, []byte{1}, []byte{1}, wasm, "LXP_NON_CANONICAL"},
		{"oversize hook", program, 1, make([]byte, 65536), nil, wasm, "LXP_LENGTH_LIMIT"},
		{"oversize interface", program, 0, nil, make([]byte, 953), wasm, "LXP_LENGTH_LIMIT"},
		{"empty wasm", program, 0, nil, nil, nil, "LXP_NON_CANONICAL"},
		{"framed empty wasm", program, 2, nil, []byte{1}, nil, "LXP_NON_CANONICAL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := layerxprograms.EncodeUpgradePayload(tc.program, 1, tc.flags, oldHash, newHash, tc.hook, tc.iface, tc.wasm)
			requireLifecycleRefusal(t, payload, err, tc.code)
		})
	}
	t.Run("reserved flag bits", func(t *testing.T) {
		for bit := uint8(2); bit < 8; bit++ {
			payload, err := layerxprograms.EncodeUpgradePayload(program, 1, 1<<bit, oldHash, newHash, nil, nil, wasm)
			requireLifecycleRefusal(t, payload, err, "LXP_NON_CANONICAL")
		}
	})
	t.Run("maximum hook and interface", func(t *testing.T) {
		hook, iface := make([]byte, 65535), make([]byte, 952)
		hook[65534], iface[951] = 0xaa, 0xbb
		payload, err := layerxprograms.EncodeUpgradePayload(program, 0xffff, 1, oldHash, newHash, hook, iface, wasm)
		require.NoError(t, err)
		require.Len(t, payload, 110+65535+952+len(wasm))
		require.Equal(t, uint16(65535), binary.BigEndian.Uint16(payload[100:102]))
		require.Equal(t, uint32(952), binary.BigEndian.Uint32(payload[106:110]))
		require.Equal(t, hook, payload[110:65645])
		require.Equal(t, iface, payload[65645:66597])
		require.Equal(t, wasm, payload[66597:])
	})
}

func TestEncodeRegistryReadPayload(t *testing.T) {
	program, _, _, _ := lifecycleIDs()
	for _, tc := range []struct {
		name   string
		body   []byte
		golden string
	}{
		{"nil body", nil, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"},
		{"empty body", []byte{}, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"},
		{"opaque body", []byte{0, 0xff, 0x80, 1, 0}, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f00ff800100"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := layerxprograms.EncodeRegistryReadPayload(program, tc.body)
			requireLifecycleGolden(t, payload, err, tc.golden)
			if len(tc.body) != 0 {
				payload[32] = 1
				require.Equal(t, byte(0), tc.body[0])
			}
		})
	}
	for _, body := range [][]byte{nil, {}, {0, 0xff}} {
		payload, err := layerxprograms.EncodeRegistryReadPayload([32]byte{}, body)
		requireLifecycleRefusal(t, payload, err, "LXP_NON_CANONICAL")
	}
}
