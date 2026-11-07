// Proves: MergeFailFastPrecompileAddresses + AllCustomPrecompilesFailFastMerge -> vm.NewEVM / vm.EVM.Call / vm.RunPrecompiledContract -> utils.ShouldExecutionAbort and the importing Go consumer.
//
// The consumer uses the initialized, exported registry, a real in-memory geth
// StateDB and the real EVM dispatcher. Each call is repeated; a gas trace at
// RunPrecompiledContract proves that the producer executes again rather than
// serving a cached result. This interface has no durable state to reopen.
// A successful crossing here means a typed abort reaches the fallback decision,
// not successful execution of the native operation on the giga path. The real
// ledger ABI decoder must reject the producer's empty result as malformed for
// balanceOf, including when its input was produced by that same ABI.
package precompiles_test

import (
	"math/big"
	"testing"

	gigaprecompiles "github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/precompiles"
	gigautils "github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProofNodeEngineExportedRegistry(t *testing.T) {
	expectedMerge := []common.Address{
		common.HexToAddress("0x101A"),
		common.HexToAddress("0x101C"),
		common.HexToAddress("0x101D"),
		common.HexToAddress("0x101E"),
		common.HexToAddress("0x0901"),
		common.HexToAddress("0x0902"),
		common.HexToAddress("0x0903"),
		common.HexToAddress("0x0904"),
		common.HexToAddress("0x0905"),
		common.HexToAddress("0x0906"),
		common.HexToAddress("0x0907"),
		common.HexToAddress("0x0908"),
	}
	require.ElementsMatch(t, expectedMerge, gigaprecompiles.MergeFailFastPrecompileAddresses)
	require.NotContains(t, gigaprecompiles.MergeFailFastPrecompileAddresses, common.HexToAddress("0x101B"))
	require.Contains(t, gigaprecompiles.LateFailFastPrecompileAddresses, common.HexToAddress("0x101B"))

	expected := make(map[common.Address]struct{})
	for _, addresses := range [][]common.Address{
		gigaprecompiles.FailFastPrecompileAddresses,
		gigaprecompiles.LateFailFastPrecompileAddresses,
		gigaprecompiles.MergeFailFastPrecompileAddresses,
	} {
		for _, address := range addresses {
			require.NotContains(t, expected, address, "address sets must be disjoint")
			expected[address] = struct{}{}
			require.Same(t, gigaprecompiles.FailFastSingleton, gigaprecompiles.AllCustomPrecompilesFailFastMerge[address])
		}
	}
	require.Len(t, gigaprecompiles.AllCustomPrecompilesFailFastMerge, len(expected))

	for address := range expected {
		t.Run(address.Hex(), func(t *testing.T) {
			ev, dispatches := proofNodeEngineEVM(t)
			for call := 1; call <= 2; call++ {
				result, remaining, err := ev.Call(common.Address{}, address, nil, 100_000, new(uint256.Int))
				require.Same(t, gigaprecompiles.ErrInvalidPrecompileCall, err)
				require.Nil(t, result)
				require.Equal(t, uint64(100_000), remaining, "abort preserves gas for fallback")
				require.Implements(t, (*vm.AbortError)(nil), err)
				require.True(t, gigautils.ShouldExecutionAbort(err), "consumer must select ordinary execution")
				require.Equal(t, call, *dispatches, "each observation must execute the precompile")
			}
		})
	}
}

func TestProofNodeEngineMalformedResult(t *testing.T) {
	ledgerABI := layerxledger.GetABI()
	input, err := ledgerABI.Pack("balanceOf", [32]byte{1}, [32]byte{2})
	require.NoError(t, err)
	method, err := ledgerABI.MethodById(input[:4])
	require.NoError(t, err)
	args, err := method.Inputs.Unpack(input[4:])
	require.NoError(t, err)
	require.Len(t, args, 2)

	for _, tc := range []struct {
		name  string
		input []byte
	}{
		{name: "valid ABI call", input: input},
		{name: "truncated ABI arguments", input: input[:len(input)-1]},
		{name: "truncated selector", input: input[:3]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ev, dispatches := proofNodeEngineEVM(t)
			for call := 1; call <= 2; call++ {
				result, remaining, callErr := ev.Call(common.Address{}, common.HexToAddress(layerxledger.LedgerAddress), tc.input, 100_000, new(uint256.Int))
				require.Same(t, gigaprecompiles.ErrInvalidPrecompileCall, callErr)
				require.True(t, gigautils.ShouldExecutionAbort(callErr))
				require.Nil(t, result)
				require.Equal(t, uint64(100_000), remaining)
				require.Equal(t, call, *dispatches)

				// Decode exactly the bytes the producer returned. An empty abort
				// result cannot be consumed as a balance of zero (or any balance).
				decoded, decodeErr := method.Outputs.Unpack(result)
				require.Error(t, decodeErr)
				require.Empty(t, decoded)
			}
		})
	}
}

func TestProofNodeEngineUnknownAddress(t *testing.T) {
	ev, dispatches := proofNodeEngineEVM(t)
	address := common.HexToAddress("0xdeadbeef")
	require.NotContains(t, gigaprecompiles.AllCustomPrecompilesFailFastMerge, address)
	for call := 0; call < 2; call++ {
		// An unregistered, empty account retains ordinary EVM semantics. This
		// control proves the registry, rather than every call, causes aborts.
		result, remaining, err := ev.Call(common.Address{}, address, nil, 100_000, new(uint256.Int))
		require.NoError(t, err)
		require.Empty(t, result)
		require.Equal(t, uint64(100_000), remaining)
		require.False(t, gigautils.ShouldExecutionAbort(err))
		require.Zero(t, *dispatches)
	}
}

func proofNodeEngineEVM(t *testing.T) (*vm.EVM, *int) {
	t.Helper()
	db := state.NewDatabaseForTesting()
	t.Cleanup(func() { require.NoError(t, db.TrieDB().Close()) })
	statedb, err := state.New(types.EmptyRootHash, db)
	require.NoError(t, err)
	dispatches := new(int)
	hooks := &tracing.Hooks{
		OnGasChange: func(_, _ uint64, reason tracing.GasChangeReason) {
			if reason == tracing.GasChangeCallPrecompiledContract {
				*dispatches++
			}
		},
	}
	ev := vm.NewEVM(vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		BlockNumber: big.NewInt(1),
		Difficulty:  new(big.Int),
		GasLimit:    1_000_000,
	}, statedb, params.AllEthashProtocolChanges, vm.Config{Tracer: hooks}, gigaprecompiles.AllCustomPrecompilesFailFastMerge)
	return ev, dispatches
}
