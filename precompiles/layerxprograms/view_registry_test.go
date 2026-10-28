package layerxprograms_test

import (
	"crypto/sha256"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func deployRegistryViewProgram(t *testing.T, h *harness, withInterface bool) (program, authority, codeHash [32]byte, encoding []byte) {
	t.Helper()
	ctx := h.stateDB.Ctx()
	resolved, _, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	authority = resolved.Principal
	program = [32]byte{0xa1, 0x72}
	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	codeHash = sha256.Sum256(wasm)
	abiVersion := uint16(5)
	if withInterface {
		abiVersion = 1
		encoding = append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
		encoding = binary.BigEndian.AppendUint16(encoding, abiVersion)
		encoding = binary.BigEndian.AppendUint16(encoding, 1)
		encoding = binary.BigEndian.AppendUint16(encoding, 11)
		encoding = append(encoding, []byte("layerx_call")...)
		encoding = append(encoding, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64, 0, 0, 0, 0, 0, 0)
	}
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, authority[:]...)
	store := ctx.KVStore(h.key)
	store.Set(append([]byte{1, 0, 9}, []byte("progmet/active/v1")...), metering)
	store.Set(append([]byte{1, 0, 9}, binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)...), metering)
	payload, err := lp.EncodeDeployPayload(program, abiVersion, 1, authority, codeHash, encoding, wasm)
	require.NoError(t, err)
	method := lp.ProgramsABI.Methods["deploy"]
	receipt, err := lp.Submit(ctx, h.executor, lp.NewCallContext(h.caller, &method), 0x00090001, payload, [16]byte{})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	return program, authority, codeHash, encoding
}

func TestProgramsViewGetProgramRecord(t *testing.T) {
	h := newHarness(t)
	program, authority, codeHash, _ := deployRegistryViewProgram(t, h, false)
	ctx := h.stateDB.Ctx()
	_, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	lastSequence, err := h.host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	head := binary.BigEndian.AppendUint16(nil, 1)
	head = binary.BigEndian.AppendUint32(head, 0)
	manifestDigest := sha256.Sum256([]byte{0, 1, 0, 0, 0, 0})
	head = append(head, manifestDigest[:]...)
	t.Run("missing state head", func(t *testing.T) {
		before := h.dump(h.stateDB.Ctx())
		input, err := lp.ProgramsABI.Pack("getStateHead", program)
		require.NoError(t, err)
		out, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
		require.NoError(t, err)
		values, err := lp.ProgramsABI.Methods["getStateHead"].Outputs.Unpack(out)
		require.NoError(t, err)
		require.Equal(t, []interface{}{head}, values)
		require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	})
	headKey := append([]byte{1, 0, 9}, []byte("progstor")...)
	headKey = append(headKey, program[:]...)
	headKey = append(headKey, 1)
	h.stateDB.Ctx().KVStore(h.key).Set(headKey, head)
	before := h.dump(h.stateDB.Ctx())
	for _, tc := range []struct {
		name string
		want []interface{}
	}{
		{"getProgram", []interface{}{uint8(1), authority, codeHash, uint16(5), uint32(1), true}},
		{"getProgramOwner", []interface{}{authority}},
		{"getStateHead", []interface{}{head}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, err := lp.ProgramsABI.Pack(tc.name, program)
			require.NoError(t, err)
			out, _, err := h.evm.StaticCall(common.HexToAddress("0xb222"), h.precompile.Address(), input, 2000000)
			require.NoError(t, err)
			values, err := lp.ProgramsABI.Methods[tc.name].Outputs.Unpack(out)
			require.NoError(t, err)
			require.Equal(t, tc.want, values)
		})
	}
	afterSequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
	require.NoError(t, err)
	require.Equal(t, sequence, afterSequence)
	afterLastSequence, err := h.host.ReadLastSequence(h.stateDB.Ctx(), did)
	require.NoError(t, err)
	require.Equal(t, lastSequence, afterLastSequence)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	require.Empty(t, h.stateDB.Ctx().EventManager().Events())
}

func TestProgramsViewGetStateHeadEmptyAndUnknown(t *testing.T) {
	h := newHarness(t)
	program, _, _, _ := deployRegistryViewProgram(t, h, false)
	_, err := h.host.ReadProgramStateHead(h.stateDB.Ctx(), program)
	require.ErrorIs(t, err, lxphost.Code(-7))
	t.Run("empty storage", func(t *testing.T) {
		before := h.dump(h.stateDB.Ctx())
		input, err := lp.ProgramsABI.Pack("getStateHead", program)
		require.NoError(t, err)
		out, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
		require.NoError(t, err)
		values, err := lp.ProgramsABI.Methods["getStateHead"].Outputs.Unpack(out)
		require.NoError(t, err)
		require.Len(t, values, 1)
		head, ok := values[0].([]byte)
		require.True(t, ok)
		require.Len(t, head, 38)
		require.Equal(t, uint16(1), binary.BigEndian.Uint16(head[:2]))
		require.Zero(t, binary.BigEndian.Uint32(head[2:6]))
		digest := sha256.Sum256([]byte{0, 1, 0, 0, 0, 0})
		require.Equal(t, digest[:], head[6:])
		require.Equal(t, before, h.dump(h.stateDB.Ctx()))
		require.Empty(t, h.stateDB.Ctx().EventManager().Events())
	})
	t.Run("unknown program", func(t *testing.T) {
		before := h.dump(h.stateDB.Ctx())
		input, err := lp.ProgramsABI.Pack("getStateHead", [32]byte{0xff})
		require.NoError(t, err)
		out, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, err := abi.UnpackRevert(out)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(reason, "LXP_NOT_FOUND:"), "unexpected revert: %s", reason)
		require.Equal(t, before, h.dump(h.stateDB.Ctx()))
		require.Empty(t, h.stateDB.Ctx().EventManager().Events())
	})
}

func TestProgramsViewGetInterfaceAndAbsent(t *testing.T) {
	for _, name := range []string{"getProgram", "getInterface", "getProgramOwner", "getStateHead"} {
		t.Run("absent/"+name, func(t *testing.T) {
			h := newHarness(t)
			before := h.dump(h.stateDB.Ctx())
			input, err := lp.ProgramsABI.Pack(name, [32]byte{0xff})
			require.NoError(t, err)
			out, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err := abi.UnpackRevert(out)
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(reason, "LXP_NOT_FOUND:"), "unexpected revert: %s", reason)
			require.Equal(t, before, h.dump(h.stateDB.Ctx()))
		})
	}
	t.Run("deployed interface", func(t *testing.T) {
		h := newHarness(t)
		program, _, _, encoding := deployRegistryViewProgram(t, h, true)
		ctx := h.stateDB.Ctx()
		_, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
		require.NoError(t, err)
		sequence, err := h.host.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		lastSequence, err := h.host.ReadLastSequence(ctx, did)
		require.NoError(t, err)
		before := h.dump(h.stateDB.Ctx())
		input, err := lp.ProgramsABI.Pack("getInterface", program)
		require.NoError(t, err)
		out, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
		require.NoError(t, err)
		values, err := lp.ProgramsABI.Methods["getInterface"].Outputs.Unpack(out)
		require.NoError(t, err)
		require.Equal(t, []interface{}{uint32(1), sha256.Sum256(encoding), encoding}, values)
		afterSequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
		require.NoError(t, err)
		require.Equal(t, sequence, afterSequence)
		afterLastSequence, err := h.host.ReadLastSequence(h.stateDB.Ctx(), did)
		require.NoError(t, err)
		require.Equal(t, lastSequence, afterLastSequence)
		require.Equal(t, before, h.dump(h.stateDB.Ctx()))
		require.Empty(t, h.stateDB.Ctx().EventManager().Events())
	})
}
