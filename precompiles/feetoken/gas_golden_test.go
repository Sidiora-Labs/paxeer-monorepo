package feetoken_test

import (
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/feetoken"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestFeeTokenGasGolden(t *testing.T) {
	const mergeHeight int64 = 12
	const suppliedGas uint64 = 100_000
	caller := common.HexToAddress("0x1234")
	golden := []struct {
		method    string
		args      []interface{}
		argBytes  int
		gas       uint64
		readOnly  bool
		wantDenom string
	}{
		{method: "setFeeDenom", args: []interface{}{"usid"}, argBytes: 96, gas: 9536, wantDenom: "usid"},
		{method: "getFeeDenom", args: []interface{}{caller}, argBytes: 32, gas: 3512, readOnly: true, wantDenom: "uasset"},
		{method: "clearFeeDenom", argBytes: 0, gas: 8000, wantDenom: "uhpx"},
	}
	precompile, err := feetoken.NewPrecompile(testkeeper.EVMTestApp.GetPrecompileKeepers())
	require.NoError(t, err)
	methods := precompile.GetABI().Methods
	require.Len(t, methods, len(golden), "every ABI method must have a frozen gas row")
	covered := make(map[string]bool, len(golden))
	for _, row := range golden {
		require.Contains(t, methods, row.method)
		require.False(t, covered[row.method], "duplicate golden row for %s", row.method)
		covered[row.method] = true
	}

	for _, row := range golden {
		t.Run(row.method, func(t *testing.T) {
			var required, charged [2]uint64
			for i, height := range []int64{mergeHeight - 1, mergeHeight} {
				h := newHarness(t)
				h.db.WithCtx(h.db.Ctx().WithBlockHeight(height))
				ctx := h.db.Ctx()
				upgrades := testkeeper.EVMTestApp.EvmKeeper.UpgradeKeeper()
				upgrades.SetDone(ctx.WithBlockHeight(mergeHeight), pcommon.MergeUpgrade)
				require.Equal(t, i == 1, upgrades.IsUpgradeActiveAtHeight(ctx, pcommon.MergeUpgrade, height))
				h.call(feetoken.SetFeeDenomMethod, "uasset")
				require.Equal(t, "uasset", testkeeper.EVMTestApp.EvmKeeper.GetAccountFeeDenom(ctx, caller))

				input := h.input(row.method, row.args...)
				require.Len(t, input, 4+row.argBytes)
				required[i] = h.precompile.RequiredGas(input)
				require.Equal(t, row.gas, required[i], "RequiredGas at height %d", height)
				result, left, err := vm.RunPrecompiledContract(h.precompile, h.evm, caller, caller, input, suppliedGas, nil, nil, row.readOnly, false)
				require.NoError(t, err, "call at height %d", height)
				require.LessOrEqual(t, left, suppliedGas)
				charged[i] = suppliedGas - left
				require.Equal(t, row.gas, charged[i], "charged gas at height %d", height)
				require.Equal(t, required[i], charged[i])
				output, err := h.precompile.GetABI().Unpack(row.method, result)
				require.NoError(t, err)
				if row.readOnly {
					require.Equal(t, []interface{}{row.wantDenom}, output)
				} else {
					require.Empty(t, output)
				}
				require.Equal(t, row.wantDenom, testkeeper.EVMTestApp.EvmKeeper.GetAccountFeeDenom(ctx, caller))
				t.Logf("%s height=%d RequiredGas=%d charged=%d", row.method, height, required[i], charged[i])
			}
			require.Equal(t, required[0], required[1], "RequiredGas across the merge plan height")
			require.Equal(t, charged[0], charged[1], "charged gas across the merge plan height")
		})
	}
}
