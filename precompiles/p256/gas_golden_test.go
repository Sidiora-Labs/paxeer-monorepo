package p256_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/p256"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestP256GasGolden(t *testing.T) {
	const planHeight int64 = 100
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx([]byte{}).CacheContext()
	app.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), "v6.4.0")
	app.UpgradeKeeper.SetDone(ctx.WithBlockHeight(planHeight), pcommon.MergeUpgrade)
	require.Equal(t, planHeight, app.UpgradeKeeper.GetDoneHeight(ctx, pcommon.MergeUpgrade))

	p, err := p256.NewPrecompile(app.GetPrecompileKeepers())
	require.NoError(t, err)
	frozen := map[string]struct {
		args []interface{}
		gas  uint64
	}{
		"verify": {
			args: []interface{}{common.FromHex(
				"0000000000000000000000000000000000000000000000000000000000000001" +
					"6b17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c296" +
					"6b17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c297" +
					"6b17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c296" +
					"4fe342e2fe1a7f9b8ee7eb4a7c0f9e162bce33576b315ececbb6406837bf51f5"),
			},
			gas: 67200,
		},
	}
	methods := p.GetABI().Methods
	require.Len(t, methods, len(frozen), "every ABI method must have a frozen gas row")
	for name, method := range methods {
		t.Run(name, func(t *testing.T) {
			row, ok := frozen[name]
			require.True(t, ok, "missing frozen gas row for %s", method.Sig)
			input, err := p.Pack(name, row.args...)
			require.NoError(t, err)
			wantOutput, err := method.Outputs.Pack(common.LeftPadBytes([]byte{1}, 32))
			require.NoError(t, err)

			var gas [2]uint64
			for i, height := range []int64{planHeight - 1, planHeight} {
				at := ctx.WithBlockHeight(height)
				require.Equal(t, height >= planHeight,
					app.UpgradeKeeper.IsUpgradeActiveAtHeight(at, pcommon.MergeUpgrade, height))
				served, ok := app.EvmKeeper.CustomPrecompiles(at)[p.Address()]
				require.True(t, ok, "p256 must be served at height %d", height)
				require.IsType(t, p, served)
				gas[i] = served.RequiredGas(input)
				t.Logf("%s height=%d RequiredGas=%d", method.Sig, height, gas[i])
				require.Equal(t, row.gas, gas[i], "%s at height %d", method.Sig, height)

				stateDB := &state.DBImpl{}
				stateDB.WithCtx(at)
				evm := &vm.EVM{StateDB: stateDB}
				output, err := served.Run(evm, common.Address{}, common.Address{}, input, nil, true, false, nil)
				require.NoError(t, err, "%s at height %d", method.Sig, height)
				require.Equal(t, wantOutput, output, "%s at height %d", method.Sig, height)
			}
			require.Equal(t, gas[0], gas[1], "%s gas changed across the merge plan height", method.Sig)
		})
	}
}
