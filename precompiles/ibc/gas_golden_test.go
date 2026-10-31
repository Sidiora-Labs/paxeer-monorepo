package ibc_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	pcommonv640 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v640"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/ibc"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestIbcGasGolden(t *testing.T) {
	const (
		planHeight  = int64(100)
		suppliedGas = uint64(1_000_000)
		receiver    = "cosmos1yykwxjzr2tv4mhx5tsf8090sdg96f2ax8fydk2"
	)
	// Zero-amount transfers succeed after validating the common arguments.
	// Pin this fixture's dynamic charge, including the real address-store read,
	// independently of channel state and timeout processing.
	golden := map[string]struct {
		args []interface{}
		gas  uint64
	}{
		"transfer": {
			args: []interface{}{receiver, "transfer", "channel-0", "uhpx", big.NewInt(0), uint64(1), uint64(1000), uint64(2_000_000_000_000_000_000), "gas-golden"},
			gas:  1_123,
		},
		"transferWithDefaultTimeout": {
			args: []interface{}{receiver, "transfer", "channel-0", "uhpx", big.NewInt(0), "gas-golden"},
			gas:  1_123,
		},
	}

	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(planHeight).WithBlockTime(time.Unix(1_700_000_000, 0))
	caller := common.HexToAddress("0x1234567890123456789012345678901234567890")
	a.EvmKeeper.SetAddressMapping(ctx, sdk.AccAddress(caller.Bytes()), caller)
	// IBC has a v6.4.0 version; record its earlier activation so the served
	// address remains available below the later merge upgrade.
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), "v6.4.0")
	require.Equal(t, int64(1), a.UpgradeKeeper.GetDoneHeight(ctx, "v6.4.0"))
	a.UpgradeKeeper.SetDone(ctx, pcommon.MergeUpgrade)
	require.Equal(t, planHeight, a.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))

	built, err := ibc.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	require.Equal(t, GetABI(t), built.GetABI())
	methods := built.GetABI().Methods
	require.Len(t, golden, len(methods), "every ABI method must have exactly one frozen gas row")
	for name := range golden {
		require.Contains(t, methods, name, "golden row must name an ABI method")
	}

	for name, method := range methods {
		t.Run(name, func(t *testing.T) {
			row, ok := golden[name]
			require.True(t, ok, "ABI method must have fixed arguments and literal gas")
			input, err := built.GetABI().Pack(name, row.args...)
			require.NoError(t, err)
			wantOutput, err := method.Outputs.Pack(true)
			require.NoError(t, err)
			for _, tracing := range []bool{false, true} {
				var charged [2][2]uint64
				for i, height := range []int64{planHeight - 1, planHeight} {
					t.Run(fmt.Sprintf("height=%d/tracing=%t", height, tracing), func(t *testing.T) {
						at, _ := ctx.WithBlockHeight(height).WithIsTracing(tracing).CacheContext()
						require.Equal(t, height >= planHeight, pcommon.MergePlanActive(at, &a.EvmKeeper))
						measure := func(run func(*vm.EVM) ([]byte, uint64, error)) uint64 {
							callCtx, _ := at.CacheContext()
							db := state.NewDBImpl(callCtx, &a.EvmKeeper, true)
							evm := &vm.EVM{StateDB: db, TxContext: vm.TxContext{Origin: caller}}
							output, remainingGas, err := run(evm)
							require.NoError(t, err, "precompile error: %v", db.GetPrecompileError())
							require.Equal(t, wantOutput, output)
							require.LessOrEqual(t, remainingGas, suppliedGas)
							gas := suppliedGas - remainingGas
							t.Logf("%s at height %d: dynamic gas %d", name, height, gas)
							require.Equal(t, row.gas, gas, "dynamic gas must equal the frozen per-method literal")
							return gas
						}
						charged[i][0] = measure(func(evm *vm.EVM) ([]byte, uint64, error) {
							return built.RunAndCalculateGas(evm, caller, caller, input, suppliedGas, nil, nil, false, false)
						})
						if tracing && height < planHeight {
							require.Equal(t, "v6.4.0", a.EvmKeeper.GetCustomPrecompilesVersions(at)[built.Address()])
							served, ok := a.EvmKeeper.CustomPrecompiles(at)[built.Address()].(*pcommonv640.DynamicGasPrecompile)
							require.True(t, ok, "historical tracing must serve the v6.4.0 dynamic IBC precompile")
							require.Equal(t, built.GetABI(), served.GetABI())
							charged[i][1] = measure(func(evm *vm.EVM) ([]byte, uint64, error) {
								return served.RunAndCalculateGas(evm, caller, caller, input, suppliedGas, nil, nil, false, false)
							})
						} else {
							if tracing {
								require.Equal(t, pcommon.MergeUpgrade, a.EvmKeeper.GetCustomPrecompilesVersions(at)[built.Address()])
							}
							served, ok := a.EvmKeeper.CustomPrecompiles(at)[built.Address()].(*pcommon.DynamicGasPrecompile)
							require.True(t, ok, "application must serve the dynamic IBC precompile at both heights")
							require.Equal(t, built.GetABI(), served.GetABI())
							charged[i][1] = measure(func(evm *vm.EVM) ([]byte, uint64, error) {
								return served.RunAndCalculateGas(evm, caller, caller, input, suppliedGas, nil, nil, false, false)
							})
						}
						require.Equal(t, charged[i][0], charged[i][1], "constructor and application must charge the same dynamic gas")
					})
				}
				require.Equal(t, charged[0], charged[1], "dynamic gas must not change at the merge plan height")
			}
		})
	}
}
