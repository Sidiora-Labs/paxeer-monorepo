package json_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/json"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestJsonGasGolden(t *testing.T) {
	golden := map[string]struct {
		args []interface{}
		want interface{}
		gas  uint64
	}{
		"extractAsBytes": {
			args: []interface{}{[]byte(`{"key":"golden value"}`), "key"},
			want: []byte("golden value"),
			gas:  19200,
		},
		"extractAsBytesList": {
			args: []interface{}{[]byte(`{"key":["alpha","beta","gamma","delta"]}`), "key"},
			want: [][]byte{[]byte(`"alpha"`), []byte(`"beta"`), []byte(`"gamma"`), []byte(`"delta"`)},
			gas:  22400,
		},
		"extractAsUint256": {
			args: []interface{}{[]byte(`{"key":"42"}`), "key"},
			want: big.NewInt(42),
			gas:  19200,
		},
		"extractAsBytesFromArray": {
			args: []interface{}{[]byte(`["zero","one"]`), uint16(1)},
			want: []byte("one"),
			gas:  12800,
		},
	}

	app := testkeeper.EVMTestApp
	k := &app.EvmKeeper
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	const planHeight = int64(100)
	app.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), "v6.4.0")
	app.UpgradeKeeper.SetDone(ctx.WithBlockHeight(planHeight), pcommon.MergeUpgrade)
	require.Equal(t, planHeight, app.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))

	built, err := json.NewPrecompile(app.GetPrecompileKeepers())
	require.NoError(t, err)
	jsonABI := built.GetABI()
	require.Len(t, golden, len(jsonABI.Methods), "every JSON method must have a frozen gas row")
	for name := range jsonABI.Methods {
		require.Contains(t, golden, name, "missing JSON gas row")
	}

	for name, tc := range golden {
		t.Run(name, func(t *testing.T) {
			input, err := jsonABI.Pack(name, tc.args...)
			require.NoError(t, err)
			require.Equal(t, tc.gas, built.RequiredGas(input))

			var gas [2]uint64
			for i, height := range []int64{planHeight - 1, planHeight} {
				callCtx := ctx.WithBlockHeight(height)
				require.Equal(t, height >= planHeight, pcommon.MergePlanActive(callCtx, k))
				served, ok := k.CustomPrecompiles(callCtx)[common.HexToAddress(json.JSONAddress)]
				require.True(t, ok, "JSON must be served at height %d", height)
				require.IsType(t, built, served)
				gas[i] = served.RequiredGas(input)
				t.Logf("%s height=%d RequiredGas=%d", name, height, gas[i])
				require.Equal(t, tc.gas, gas[i], "frozen JSON gas at height %d", height)

				evm := &vm.EVM{StateDB: state.NewDBImpl(callCtx, k, false)}
				result, err := served.Run(evm, common.Address{}, common.Address{}, input, big.NewInt(0), true, false, nil)
				require.NoError(t, err, "JSON call at height %d", height)
				output, err := jsonABI.Methods[name].Outputs.Unpack(result)
				require.NoError(t, err)
				require.Equal(t, []interface{}{tc.want}, output, "JSON result at height %d", height)
			}
			require.Equal(t, gas[0], gas[1], "JSON gas must not change at the merge plan height")
		})
	}
}
