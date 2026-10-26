package xweb_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	"github.com/stretchr/testify/require"
)

func programGasObservation(t *testing.T, signatures, responseLength int) []byte {
	t.Helper()
	observation := types.Observation{
		Origin:     types.OriginProgram,
		NetworkID:  1,
		ProgramID:  types.Hash32{1},
		RequestID:  7,
		Kind:       types.KindFetch,
		FullLength: uint32(responseLength),
		Response:   bytes.Repeat([]byte{0xff}, responseLength),
		Signatures: make([][]byte, signatures),
	}
	for i := range observation.Signatures {
		observation.Signatures[i] = make([]byte, types.SignatureLength)
	}
	raw, err := types.EncodeObservation(observation)
	require.NoError(t, err)
	return raw
}

func packProgramGasObservation(t *testing.T, observation []byte) []byte {
	t.Helper()
	method := xweb.NewPrecompileWithKeeper(nil, nil, nil).Methods[xweb.FulfilProgramMethod]
	input, err := method.Inputs.Pack(observation)
	require.NoError(t, err)
	return input
}

func TestProgramFulfilGasCountsObservationSignatures(t *testing.T) {
	for signatures := 1; signatures <= types.ObservationMaxSignatures; signatures++ {
		for _, responseLength := range []int{0, 1, 31, 32, 33, types.MaxResponseBytes} {
			t.Run(fmt.Sprintf("signatures_%d/response_%d", signatures, responseLength), func(t *testing.T) {
				observation := programGasObservation(t, signatures, responseLength)
				input := packProgramGasObservation(t, observation)
				want := uint64(30000) + 16*uint64(len(input)) + 8000*uint64(signatures)
				require.Equal(t, want, xweb.ProgramFulfilGas(input))
			})
		}
	}
}

func TestProgramFulfilGasChargesSixteenForUndecodableInput(t *testing.T) {
	observation := programGasObservation(t, 3, 33)
	countOffset := types.ObservationHeaderBytes + 33
	invalidOffset := packProgramGasObservation(t, observation)
	for i := 0; i < 32; i++ {
		invalidOffset[i] = 0xff
	}
	invalidLength := packProgramGasObservation(t, observation)
	for i := 32; i < 64; i++ {
		invalidLength[i] = 0xff
	}
	validInput := packProgramGasObservation(t, observation)
	for _, tc := range []struct {
		name  string
		input []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"short_abi", []byte{1, 2, 3}},
		{"missing_abi_length", make([]byte, 32)},
		{"invalid_abi_offset", invalidOffset},
		{"invalid_abi_length", invalidLength},
		{"truncated_abi_data", validInput[:len(validInput)-32]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := uint64(30000) + 16*uint64(len(tc.input)) + 8000*16
			require.Equal(t, want, xweb.ProgramFulfilGas(tc.input))
		})
	}
	for _, tc := range []struct {
		name        string
		observation []byte
	}{
		{"empty_observation", nil},
		{"short_header", observation[:types.ObservationHeaderBytes-1]},
		{"header_only", observation[:types.ObservationHeaderBytes]},
		{"truncated_response", observation[:countOffset-1]},
		{"missing_count", observation[:countOffset]},
		{"response_length_exceeds_observation", func() []byte {
			raw := bytes.Clone(observation)
			binary.BigEndian.PutUint32(raw[142:146], uint32(len(raw)))
			return raw
		}()},
		{"maximum_response_length", func() []byte {
			raw := bytes.Clone(observation)
			binary.BigEndian.PutUint32(raw[142:146], ^uint32(0))
			return raw
		}()},
		{"zero_count", func() []byte {
			raw := bytes.Clone(observation)
			raw[countOffset] = 0
			return raw
		}()},
		{"seventeen_count", func() []byte {
			raw := bytes.Clone(observation)
			raw[countOffset] = 17
			return raw
		}()},
		{"maximum_count_byte", func() []byte {
			raw := bytes.Clone(observation)
			raw[countOffset] = 255
			return raw
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := packProgramGasObservation(t, tc.observation)
			want := uint64(30000) + 16*uint64(len(input)) + 8000*16
			require.Equal(t, want, xweb.ProgramFulfilGas(input))
		})
	}
}
