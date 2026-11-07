package xweb_test

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// Proves: XWebConsumer / IXWeb -> abi.json selectors 2d4bf7ad, 2b2e13fc, fc0d26f4 -> Precompile.RunAndCalculateGas / PrecompileExecutor.Execute / Keeper.FulfilProgram.
// Repairs contracts/src/xweb/XWebConsumer.sol's missing production forwarding
// path; native void calls must reach 0x1019 without requiring bytecode.
// Both proofs pin source literals independently; neither reads the other half.
func TestProofIXWebSelectorsAndRun(t *testing.T) {
	const observationHex = "020000000000000000000000000000000000000000000000000000000000000001713200000000000000000000000000000000000000000000000000000000000000000000000000110262da65e513a2dc07e8c56bb6e148a96d6d259e496ec215d58c89522fa9649ce1f8f995a5abc8e450e1af6753d023f6d54dfc48c9d935dfdb79149ce3a0786c8800000016000000034100fe0273f7410dfe41b13773a2cac7aec955172a6fb46bf1cf5cf968836ec6868947936ada9883852b613ef69d87f3d219c3a49cd3f75c5cd21b0f28b783935bb177041ca78c17608f791dacdb345c507aa82d99c6b19628356157f1de602b853d4ea97379281a6836360579bc0eb4929e4aa629aaea4ad9e94186ce4544a660243667f61b"
	const fulfilHex = "2d4bf7ad00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000118020000000000000000000000000000000000000000000000000000000000000001713200000000000000000000000000000000000000000000000000000000000000000000000000110262da65e513a2dc07e8c56bb6e148a96d6d259e496ec215d58c89522fa9649ce1f8f995a5abc8e450e1af6753d023f6d54dfc48c9d935dfdb79149ce3a0786c8800000016000000034100fe0273f7410dfe41b13773a2cac7aec955172a6fb46bf1cf5cf968836ec6868947936ada9883852b613ef69d87f3d219c3a49cd3f75c5cd21b0f28b783935bb177041ca78c17608f791dacdb345c507aa82d99c6b19628356157f1de602b853d4ea97379281a6836360579bc0eb4929e4aa629aaea4ad9e94186ce4544a660243667f61b0000000000000000"
	const resultHex = "2b2e13fc71320000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000011"
	const networkHex = "fc0d26f4"
	decode := func(value string) []byte {
		t.Helper()
		decoded, err := hex.DecodeString(value)
		require.NoError(t, err)
		return decoded
	}

	rawABI, err := os.ReadFile("abi.json")
	require.NoError(t, err, "precompiles/xweb/abi.json must be present")
	methods, err := abi.JSON(strings.NewReader(string(rawABI)))
	require.NoError(t, err, "precompiles/xweb/abi.json must parse")
	h := newProgramDispatchHarness(t)
	require.Equal(t, common.HexToAddress("0x1019"), precompile)
	observation, raw := programDispatchObservation(t, h, decode("4100fe"), 2)
	require.Equal(t, decode(observationHex), raw, "real observation encoder and signing path")
	require.Equal(t, types.Hash32{0x71, 0x32}, observation.ProgramID)
	require.Equal(t, uint64(17), observation.RequestID)

	inputs := make(map[string][]byte)
	for _, call := range []struct {
		method string
		args   []interface{}
		pinned string
	}{
		{xweb.FulfilProgramMethod, []interface{}{raw}, fulfilHex},
		{xweb.GetProgramResultMethod, []interface{}{[32]byte(observation.ProgramID), observation.RequestID}, resultHex},
		{xweb.GetKernelNetworkIdMethod, nil, networkHex},
	} {
		input, err := methods.Pack(call.method, call.args...)
		require.NoError(t, err, call.method)
		require.Equal(t, decode(call.pinned), input, "%s calldata", call.method)
		method, err := methods.MethodById(input[:4])
		require.NoError(t, err)
		require.Equal(t, call.method, method.Name)
		installed, err := h.precompile.MethodById(input[:4])
		require.NoError(t, err)
		require.Equal(t, method.Sig, installed.Sig)
		inputs[call.method] = input
	}

	ret, _, err := h.evm.StaticCall(stranger, precompile, inputs[xweb.GetKernelNetworkIdMethod], supplied)
	require.NoError(t, err)
	network, err := methods.Methods[xweb.GetKernelNetworkIdMethod].Outputs.Unpack(ret)
	require.NoError(t, err)
	require.Equal(t, uint32(1), h.keeper.GetParams(h.ctx()).KernelNetworkId)
	require.Equal(t, []interface{}{h.keeper.GetParams(h.ctx()).KernelNetworkId}, network)

	ret, _, err = h.evm.Call(stranger, precompile, inputs[xweb.FulfilProgramMethod], supplied, uint256.NewInt(0))
	require.NoError(t, err)
	require.Empty(t, ret)
	require.Zero(t, h.keeper.ProgramPendingCount(h.ctx()))
	require.Equal(t, uint64(1), h.keeper.ProgramCommittedCount(h.ctx()))
	stored, err := h.keeper.GetProgramResult(h.ctx(), observation.ProgramID, observation.RequestID)
	require.NoError(t, err)
	require.Equal(t, observation.ContentDigest, stored.ContentDigest)
	require.Equal(t, observation.FullLength, stored.FullLength)
	require.Equal(t, uint32(len(observation.Response)), stored.ResponseLength)
	require.Equal(t, observation.Response, stored.Response)

	ret, _, err = h.evm.StaticCall(stranger, precompile, inputs[xweb.GetProgramResultMethod], supplied)
	require.NoError(t, err)
	result, err := methods.Methods[xweb.GetProgramResultMethod].Outputs.Unpack(ret)
	require.NoError(t, err)
	require.Equal(t, []interface{}{[32]byte(stored.ContentDigest), stored.FullLength, stored.ResponseLength, stored.Response}, result)

	// modules/xweb/keeper/program_fulfil.go Keeper.FulfilProgram refuses an
	// unknown program's pending record with the existing LXP_UNKNOWN_FIELD.
	unknown := observation
	unknown.ProgramID = types.Hash32{0x72}
	unknownRaw := signProgramDispatchObservation(t, h, unknown, 2)
	unknownInput, err := methods.Pack(xweb.FulfilProgramMethod, unknownRaw)
	require.NoError(t, err)
	ret, _, err = h.evm.Call(stranger, precompile, unknownInput, supplied, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(ret)
	require.NoError(t, err)
	require.Equal(t, "LXP_UNKNOWN_FIELD", reason)
	require.Zero(t, h.keeper.ProgramPendingCount(h.ctx()))
	require.Equal(t, uint64(1), h.keeper.ProgramCommittedCount(h.ctx()))
	after, err := h.keeper.GetProgramResult(h.ctx(), observation.ProgramID, observation.RequestID)
	require.NoError(t, err)
	require.Equal(t, stored, after)
	_, err = h.keeper.GetProgramResult(h.ctx(), unknown.ProgramID, unknown.RequestID)
	require.EqualError(t, err, "LXP_UNKNOWN_FIELD")
}
