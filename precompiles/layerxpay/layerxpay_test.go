package layerxpay

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestPayPrecompileRegistersMergeConstructor(t *testing.T) {
	h := newPayPrecompileHarness(t)
	address := common.HexToAddress("0x000000000000000000000000000000000000101B")
	require.Equal(t, address, h.precompile.Address())
	require.Equal(t, "layerxpay", h.precompile.GetName())
	require.ErrorContains(t, precompiles.RegisterMergeConstructor(address, NewMergePrecompile), "already registered")
	for _, keepers := range []utils.Keepers{nil, &utils.EmptyKeepers{}, payPrecompileKeepers{PrecompileKeepers: h.keepers.PrecompileKeepers}} {
		p, err := NewMergePrecompile(keepers, precompiles.MergeUpgrade)
		require.ErrorContains(t, err, "LXP_PRECOMPILE_NO_CONTEXT")
		require.Nil(t, p)
	}
	p, err := NewMergePrecompile(h.keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.Equal(t, address, p.Address())
	require.Equal(t, PrecompileName, p.GetName())
	custom, err := precompiles.GetCustomPrecompilesE(precompiles.MergeUpgrade, h.keepers)
	require.NoError(t, err)
	require.Contains(t, custom, address)
	require.Contains(t, custom[address], precompiles.MergeUpgrade)
	registered, ok := custom[address][precompiles.MergeUpgrade].(precompiles.IPrecompile)
	require.True(t, ok)
	require.Equal(t, PrecompileName, registered.GetName())
	require.Equal(t, address, registered.Address())
}

func TestPayPrecompileChargesGasTable(t *testing.T) {
	h := newPayPrecompileHarness(t)
	a := h.precompile.GetABI()
	require.Len(t, a.Methods, 57)
	require.Len(t, methodGas, len(a.Methods))
	const supplied uint64 = 2000000
	before := h.keeper.ExportGenesis(h.db.Ctx())
	for name, method := range a.Methods {
		t.Run(name, func(t *testing.T) {
			args := make([]interface{}, len(method.Inputs))
			for i, input := range method.Inputs {
				args[i] = zeroArgument(input.Type)
			}
			input, err := a.Pack(name, args...)
			require.NoError(t, err)
			row, ok := methodGas[name]
			require.True(t, ok)
			require.Equal(t, !method.IsConstant(), row.Write)
			want := row.Gas(input[4:])
			require.Equal(t, want, h.precompile.RequiredGas(input))
			require.Equal(t, want, h.precompile.GetExecutor().RequiredGas(input[4:], &method))
			perByte := uint64(3)
			if row.Write {
				perByte = 30
			}
			require.Equal(t, want+4*perByte, RequiredGas(input), "the standalone estimator includes the selector")
			for _, length := range []int{4, 36, 132, 356, 4096} {
				calldata := make([]byte, length)
				copy(calldata, method.ID)
				require.Equal(t, row.Gas(calldata[4:]), h.precompile.RequiredGas(calldata))
			}
			output, left, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, supplied)
			require.Equal(t, supplied-want, left)
			if method.IsConstant() && name == "progressHighWater" {
				require.NoError(t, err)
				decoded, decodeErr := method.Outputs.Unpack(output)
				require.NoError(t, decodeErr)
				require.Equal(t, []interface{}{uint32(0)}, decoded)
			} else {
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				reason, decodeErr := abi.UnpackRevert(output)
				require.NoError(t, decodeErr)
				require.True(t, strings.HasPrefix(reason, "LXP_"), reason)
				if row.Write {
					require.Contains(t, reason, "staticcall")
				}
			}
			_, left, err = h.evm.StaticCall(h.caller, h.precompile.Address(), input, want-1)
			require.ErrorIs(t, err, vm.ErrOutOfGas)
			require.Zero(t, left)
		})
	}
	for name, want := range map[string]uint64{"getEscrow": 1096, "escrowOpen": 70560, "serviceDisputeResolve": 103840} {
		method := a.Methods[name]
		args := make([]interface{}, len(method.Inputs))
		for i, input := range method.Inputs {
			args[i] = zeroArgument(input.Type)
		}
		input, err := a.Pack(name, args...)
		require.NoError(t, err)
		require.Equal(t, want, h.precompile.RequiredGas(input), name)
	}
	for _, input := range [][]byte{nil, {1}, {1, 2, 3}, {0xde, 0xad, 0xbe, 0xef}} {
		require.Equal(t, pcommon.UnknownMethodCallGas, h.precompile.RequiredGas(input))
		output, left, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, supplied)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Equal(t, supplied-pcommon.UnknownMethodCallGas, left)
		reason, decodeErr := abi.UnpackRevert(output)
		require.NoError(t, decodeErr)
		require.True(t, strings.HasPrefix(reason, "LXP_PRECOMPILE_"), reason)
	}
	input, err := a.Pack("progressHighWater", [32]byte{1})
	require.NoError(t, err)
	output, err := h.precompile.Run(h.evm, h.caller, h.caller, input, big.NewInt(1), true, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, decodeErr := abi.UnpackRevert(output)
	require.NoError(t, decodeErr)
	require.True(t, strings.HasPrefix(reason, "LXP_PRECOMPILE_NON_PAYABLE:"), reason)
	output, err = h.precompile.Run(h.evm, h.caller, h.caller, input, big.NewInt(0), true, true, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, decodeErr = abi.UnpackRevert(output)
	require.NoError(t, decodeErr)
	require.Contains(t, reason, "delegatecall")
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Empty(t, h.db.GetAllLogs())
}
