package types_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func observationRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	require.NoError(t, err)
	return root
}

func observationFixture(t *testing.T) []byte {
	t.Helper()
	encoded, err := os.ReadFile(filepath.Join(observationRoot(t), "tests/fixtures/web/observation-activity.hex"))
	require.NoError(t, err)
	activity, err := hex.DecodeString(strings.TrimSpace(string(encoded)))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(activity), 5)
	require.Equal(t, []byte{0, 2, 0x10, 1, 12}, activity[:5])
	remaining := activity[5:]
	var raw []byte
	for index, width := range []int{2, 4, 4, -1, -1, 8, 16, -1, 16, -1, -1, -1} {
		require.NotEmpty(t, remaining)
		tag := byte(index + 1)
		require.Equal(t, tag, remaining[0])
		remaining = remaining[1:]
		if width == -1 {
			require.GreaterOrEqual(t, len(remaining), 4)
			length := binary.BigEndian.Uint32(remaining[:4])
			remaining = remaining[4:]
			require.LessOrEqual(t, uint64(length), uint64(len(remaining)))
			width = int(length)
		}
		require.GreaterOrEqual(t, len(remaining), width)
		field := remaining[:width]
		switch tag {
		case 1:
			require.Equal(t, []byte{0, 2}, field)
		case 3:
			require.Equal(t, []byte{0, 0x0b, 0, 1}, field)
		case 11:
			raw = field
		case 12:
			require.Len(t, field, 64)
		}
		remaining = remaining[width:]
	}
	require.Empty(t, remaining)
	require.Len(t, raw, 358)
	return raw
}

func observationHex(t *testing.T, raw string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(raw)
	require.NoError(t, err)
	return decoded
}

func TestDecodeObservationFixture(t *testing.T) {
	raw := observationFixture(t)
	t.Run("5.12.1", func(t *testing.T) {
		const adapterNetwork uint32 = 9
		observation, err := types.DecodeObservation(raw)
		require.NoError(t, err)
		require.Equal(t, uint8(2), observation.Origin)
		assert.Equal(t, adapterNetwork, observation.NetworkID)
		var program types.Hash32
		for i := range program {
			program[i] = byte(0xa0 + i)
		}
		require.Equal(t, program, observation.ProgramID)
		require.Equal(t, uint64(0x0102030405060708), observation.RequestID)
		require.Equal(t, uint8(1), observation.Kind)
		require.Equal(t, observationHex(t, "62da65e513a2dc07e8c56bb6e148a96d6d259e496ec215d58c89522fa9649ce1"), observation.PayloadHash[:])
		require.Equal(t, observationHex(t, "2d823e82313101707a7081be2efb28edce966e3e6d1eaec9f7c1e48088f90781"), observation.ContentDigest[:])
		require.Equal(t, uint32(16), observation.FullLength)
		require.Len(t, observation.Response, 16)
		require.Equal(t, []byte("Paxeer X Network"), observation.Response)
		require.Equal(t, [][]byte{
			observationHex(t, "31947ab53187686fc5c3c21f0f78dcddc2a7ba4ae8fd7c47912f863614be8caf4bfc9541339a823e772aa657d00017a5ed0c36cc068b58e1020023cb2cab7fab1c"),
			observationHex(t, "207e4af3fb807260e2c4307b23313c31aae37b91ae2234f16d8523f4b575b3f24cdf153425e5ef708d4d7b3e8d4f270b0c5a9d7f43da00a3749a477a62cc145d1b"),
			observationHex(t, "2bd8755672a66bc20b1c68ac3ad4ea8f5eee7ca9fc41952de958527dce9bccec5150b88c5afcc010811a94e083fd8b0b38cfc9a6dba64f0a353aa3751b07a7cf1b"),
		}, observation.Signatures)
		canonical, err := types.EncodeObservation(observation)
		require.NoError(t, err)
		require.Equal(t, raw, canonical)
	})
	t.Run("owned_bytes", func(t *testing.T) {
		input := bytes.Clone(raw)
		observation, err := types.DecodeObservation(input)
		require.NoError(t, err)
		clear(input)
		canonical, err := types.EncodeObservation(observation)
		require.NoError(t, err)
		require.Equal(t, raw, canonical)
		observation.Response[0] ^= 1
		observation.Signatures[0][0] ^= 1
		require.Equal(t, raw, canonical)
	})
	t.Run("codec_bounds", func(t *testing.T) {
		require.Equal(t, 146, types.ObservationHeaderBytes)
		require.Equal(t, 16, types.ObservationMaxSignatures)
		require.Equal(t, 5283, types.ObservationMaxBytes)
		fixture, err := types.DecodeObservation(raw)
		require.NoError(t, err)
		cases := []struct {
			name   string
			change func(*types.Observation)
		}{
			{"empty_response", func(o *types.Observation) {
				o.Response = []byte{}
				o.FullLength = 0
				o.Signatures = o.Signatures[:1]
			}},
			{"search", func(o *types.Observation) { o.Kind = types.KindSearch }},
			{"maximum", func(o *types.Observation) {
				o.NetworkID = ^uint32(0)
				o.RequestID = ^uint64(0)
				o.Response = bytes.Repeat([]byte{0xab}, 4096)
				o.FullLength = ^uint32(0)
				o.Signatures = make([][]byte, 16)
				for i := range o.Signatures {
					o.Signatures[i] = bytes.Clone(fixture.Signatures[i%3])
				}
			}},
			{"zero_request_and_hashes", func(o *types.Observation) {
				o.RequestID = 0
				o.PayloadHash = types.Hash32{}
				o.ContentDigest = types.Hash32{}
			}},
			{"opaque_signature", func(o *types.Observation) { o.Signatures = [][]byte{make([]byte, 65)} }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				want := fixture
				tc.change(&want)
				encoded, err := types.EncodeObservation(want)
				require.NoError(t, err)
				got, err := types.DecodeObservation(encoded)
				require.NoError(t, err)
				require.Equal(t, want, got)
				if tc.name == "maximum" {
					require.Len(t, encoded, 5283)
				}
				if tc.name == "empty_response" {
					require.Len(t, encoded, 212)
				}
			})
		}
	})
}

func TestObservationFixtureRoundTripsInGo(t *testing.T) {
	raw := observationFixture(t)
	observation, err := types.DecodeObservation(raw)
	require.NoError(t, err)
	canonical, err := types.EncodeObservation(observation)
	require.NoError(t, err)
	require.Equal(t, raw, canonical)
}

func observationRefusal(t *testing.T, raw []byte) {
	t.Helper()
	observation, err := types.DecodeObservation(raw)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	require.Equal(t, types.Observation{}, observation)
}

func TestDecodeObservationRefusals(t *testing.T) {
	fixture := observationFixture(t)
	t.Run("5.12.2", func(t *testing.T) {
		cases := []struct {
			name   string
			change func([]byte) []byte
		}{
			{"origin", func(raw []byte) []byte { raw[0] = 1; return raw }},
			{"signature_count_above_cap", func(raw []byte) []byte { raw[162] = 17; return raw }},
			{"signature_count_zero", func(raw []byte) []byte { raw[162] = 0; return raw }},
			{"response_above_cap", func(raw []byte) []byte { binary.BigEndian.PutUint32(raw[142:146], 4097); return raw }},
			{"response_length_overflow", func(raw []byte) []byte { binary.BigEndian.PutUint32(raw[142:146], ^uint32(0)); return raw }},
			{"full_length_short", func(raw []byte) []byte { binary.BigEndian.PutUint32(raw[138:142], 15); return raw }},
			{"response_truncated", func(raw []byte) []byte { return raw[:161] }},
			{"response_count_missing", func(raw []byte) []byte { return raw[:162] }},
			{"signature_truncated", func(raw []byte) []byte { return raw[:len(raw)-1] }},
			{"trailing_byte", func(raw []byte) []byte { return append(raw, 0) }},
			{"extra_signature", func(raw []byte) []byte { return append(raw, raw[163:228]...) }},
			{"network_zero", func(raw []byte) []byte { clear(raw[29:33]); return raw }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) { observationRefusal(t, tc.change(bytes.Clone(fixture))) })
		}
		for i := 1; i <= 28; i++ {
			t.Run(fmt.Sprintf("padding_%d", i), func(t *testing.T) {
				raw := bytes.Clone(fixture)
				raw[i] = 1
				observationRefusal(t, raw)
			})
		}
		for length := 0; length <= 146; length++ {
			t.Run(fmt.Sprintf("short_%d", length), func(t *testing.T) { observationRefusal(t, fixture[:length]) })
		}
	})
	t.Run("5.12.3", func(t *testing.T) {
		t.Run("program_zero", func(t *testing.T) {
			raw := bytes.Clone(fixture)
			clear(raw[33:65])
			observationRefusal(t, raw)
		})
		for _, kind := range []byte{0, 3, 255} {
			t.Run(fmt.Sprintf("kind_%d", kind), func(t *testing.T) {
				raw := bytes.Clone(fixture)
				raw[73] = kind
				observationRefusal(t, raw)
			})
		}
	})
	t.Run("5.12.4", func(t *testing.T) {
		raw := make([]byte, 5284)
		copy(raw, fixture)
		observationRefusal(t, raw)
	})
	t.Run("encoder_signature_width", func(t *testing.T) {
		observation, err := types.DecodeObservation(fixture)
		require.NoError(t, err)
		for _, width := range []int{0, 64, 66} {
			t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
				invalid := observation
				invalid.Signatures = [][]byte{make([]byte, width)}
				raw, err := types.EncodeObservation(invalid)
				require.EqualError(t, err, "LXP_NON_CANONICAL")
				require.Nil(t, raw)
			})
		}
	})
}
