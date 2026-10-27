package layerxprograms_test

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"sort"
	"strings"
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func FuzzProgramsDecodeArgs(f *testing.F) {
	names := make([]string, 0, len(lp.ProgramsABI.Methods))
	for name := range lp.ProgramsABI.Methods {
		names = append(names, name)
	}
	sort.Strings(names)
	require.NotEmpty(f, names)

	for index, name := range names {
		method := lp.ProgramsABI.Methods[name]
		packed, err := method.Inputs.Pack(programsFuzzArguments(f, method)...)
		require.NoError(f, err, name)
		decoded, err := method.Inputs.Unpack(packed)
		require.NoError(f, err, name)
		roundTrip, err := method.Inputs.Pack(decoded...)
		require.NoError(f, err, name)
		require.Equal(f, packed, roundTrip, name)
		f.Add(uint64(index), packed)
		f.Add(uint64(index), append(bytes.Clone(packed), 0xff))
		if len(packed) == 0 {
			continue
		}
		for _, length := range []int{0, 1, 31, len(packed) - 1} {
			f.Add(uint64(index), bytes.Clone(packed[:length]))
		}
		head := 0
		for _, input := range method.Inputs {
			switch input.Type.T {
			case abi.BytesTy, abi.StringTy, abi.SliceTy:
				badOffset := bytes.Clone(packed)
				copy(badOffset[head:head+32], bytes.Repeat([]byte{0xff}, 32))
				f.Add(uint64(index), badOffset)
				unaligned := bytes.Clone(packed)
				unaligned[head+31]++
				f.Add(uint64(index), unaligned)
				offset := int(binary.BigEndian.Uint64(packed[head+24 : head+32]))
				badLength := bytes.Clone(packed)
				copy(badLength[offset:offset+32], bytes.Repeat([]byte{0xff}, 32))
				f.Add(uint64(index), badLength)
				if input.Type.T == abi.BytesTy || input.Type.T == abi.StringTy {
					length := int(binary.BigEndian.Uint64(packed[offset+24 : offset+32]))
					if length%32 != 0 {
						badPadding := bytes.Clone(packed)
						badPadding[offset+32+length] = 0xff
						f.Add(uint64(index), badPadding)
					}
				}
				if input.Type.T == abi.SliceTy {
					badAmount := bytes.Clone(packed)
					badAmount[offset+32+3*32] = 1
					f.Add(uint64(index), badAmount)
				}
			case abi.UintTy, abi.ArrayTy:
				badInteger := bytes.Clone(packed)
				badInteger[head] = 0xff
				f.Add(uint64(index), badInteger)
			}
			if input.Type.T == abi.ArrayTy {
				head += 32 * input.Type.Size
			} else {
				head += 32
			}
		}
	}

	codeNames := make(map[string]bool)
	for code := int32(-999); code < 0; code++ {
		codeNames[pcommon.LXPCodeName(code)] = true
	}

	f.Fuzz(func(t *testing.T, methodIndex uint64, arguments []byte) {
		method := lp.ProgramsABI.Methods[names[methodIndex%uint64(len(names))]]
		input := append(bytes.Clone(method.ID), arguments...)
		h := newHarness(t)
		before := h.dump(h.stateDB.Ctx())
		var output []byte
		var remaining uint64
		var callErr error
		require.NotPanics(t, func() {
			output, remaining, callErr = vm.RunPrecompiledContract(
				h.precompile, h.evm, h.caller, h.precompile.Address(), input,
				h.precompile.RequiredGas(input), big.NewInt(0), nil, true, false,
			)
		}, method.Name)
		require.Zero(t, remaining, method.Name)
		require.Equal(t, before, h.dump(h.stateDB.Ctx()), method.Name)
		if callErr == nil {
			values, err := method.Outputs.Unpack(output)
			require.NoError(t, err, method.Name)
			canonical, err := method.Outputs.Pack(values...)
			require.NoError(t, err, method.Name)
			require.True(t, bytes.Equal(canonical, output), "%s returned a non-canonical output", method.Name)
			return
		}
		require.ErrorIs(t, callErr, vm.ErrExecutionReverted, method.Name)
		reason, err := abi.UnpackRevert(output)
		require.NoError(t, err, method.Name)
		code, _, found := strings.Cut(reason, ":")
		require.True(t, found, "%s returned an unnamed error: %q", method.Name, reason)
		require.True(t, strings.HasPrefix(code, "LXP_"), "%s returned a non-LXP error: %q", method.Name, reason)
		require.True(t, codeNames[code], "%s returned a code absent from LXPCodeName: %q", method.Name, reason)
		underlying := h.stateDB.GetPrecompileError()
		require.Error(t, underlying, method.Name)
		require.Equal(t, pcommon.LXPReason(underlying), output, method.Name)
		if _, decodeErr := method.Inputs.Unpack(arguments); decodeErr == nil {
			require.ErrorIs(t, underlying, lp.ErrReadOnlyWrite, method.Name)
		}
	})
}

func programsFuzzArguments(t testing.TB, method abi.Method) []interface{} {
	t.Helper()
	arguments := make([]interface{}, len(method.Inputs))
	for i, input := range method.Inputs {
		switch input.Type.String() {
		case "bytes32":
			arguments[i] = [32]byte{1}
		case "uint8":
			arguments[i] = uint8(1)
		case "uint16":
			arguments[i] = uint16(1)
		case "uint32":
			arguments[i] = uint32(1)
		case "uint64":
			arguments[i] = uint64(1)
		case "uint128":
			arguments[i] = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
		case "string":
			arguments[i] = "main"
		case "bytes":
			arguments[i] = []byte{1, 0x80, 0xff}
		case "uint64[7]":
			arguments[i] = [7]uint64{0, 1, 2, 3, 1 << 32, 1 << 63, ^uint64(0)}
		case "(bytes32,bytes32,bytes32,uint128)[]":
			arguments[i] = []lp.TransferLeg{{From: [32]byte{1}, Asset: [32]byte{2}, To: [32]byte{3}, Amount: big.NewInt(1)}}
		default:
			t.Fatalf("%s needs a seed for ABI type %s", method.Name, input.Type)
		}
	}
	return arguments
}
