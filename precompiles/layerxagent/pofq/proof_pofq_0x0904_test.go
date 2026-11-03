// Proves: pofq.NewMergePrecompile and PrecompileExecutor.scoreFill/scoreBatch/updateRollingScore -> app.App.EvmKeeper.CustomPrecompiles and vm.EVM.Call/StaticCall -> agentcommon.MethodGas and abi.Arguments.Unpack.
//
// The listed Go consumers only price calls; this test also stands at the ABI
// consumer boundary and decodes actual EVM output. No application, dispatcher,
// keeper, executor or codec is replaced. The helper is stateless: every method
// is executed twice, with a fresh merge constructor and EVM on the second pass,
// and each execution consumes gas and enters the real EVM frame. Changed inputs
// between repeated calls must change the result. Damaged copies of real output
// are refused by the same ABI decoder that consumed the successful result.
//
// Repairs precompiles/layerxagent/pofq/merge.go NewMergePrecompile: an empty
// activation plan previously constructed an unusable precompile successfully;
// it must fail immediately with ErrNotActive, as the streams constructor does.
package pofq_test

import (
	"bytes"
	"math"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/pofq"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProofPofq0x0904(t *testing.T) {
	application := app.Setup(t, false, true, false)
	keepers := application.GetPrecompileKeepers()
	k := &application.EvmKeeper
	ctx := application.GetContextForDeliverTx(nil).WithBlockHeight(100)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager())
	application.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	address := common.HexToAddress("0x0000000000000000000000000000000000000904")
	caller := common.HexToAddress("0x211031")

	rejected, err := pofq.NewMergePrecompile(keepers, "")
	require.Nil(t, rejected)
	require.ErrorIs(t, err, agentcommon.ErrNotActive)
	require.EqualError(t, err, "LXP_AGENT_NOT_ACTIVE: no merge upgrade activation reader bound")
	rejected, err = pofq.NewMergePrecompile(nil, precompiles.MergeUpgrade)
	require.Nil(t, rejected)
	require.ErrorIs(t, err, agentcommon.ErrNoContext)

	fills := []pofq.Fill{
		{FillPrice: big.NewInt(1000), OraclePrice: big.NewInt(1000), Size: big.NewInt(1)},
		{FillPrice: big.NewInt(1500), OraclePrice: big.NewInt(1000), Size: big.NewInt(3)},
	}
	type vector struct {
		name      string
		signature string
		selector  []byte
		args      []interface{}
		want      []string
		gas       uint64
	}
	vectors := []vector{
		{pofq.ScoreFillMethod, "scoreFill(int256,int256)", []byte{0xe0, 0x87, 0xa6, 0xba},
			[]interface{}{big.NewInt(1010), big.NewInt(1000)}, []string{"990000000000000000"}, 1204},
		{pofq.ScoreBatchMethod, "scoreBatch((int256,int256,uint128)[])", []byte{0x1b, 0x1f, 0xba, 0x48},
			[]interface{}{fills}, []string{"625000000000000000", "4"}, 1780},
		{pofq.UpdateRollingScoreMethod, "updateRollingScore(uint256,uint256,uint256,uint256,uint16)", []byte{0xa8, 0xf3, 0x2b, 0x0e},
			[]interface{}{big.NewInt(1000000000000000000), big.NewInt(10), big.NewInt(500000000000000000), big.NewInt(10), uint16(5000)},
			[]string{"666666666666666666", "15"}, 1492},
	}
	var previous precompiles.IPrecompile
	for pass := 0; pass < 2; pass++ {
		// This is the production registry and its registered merge constructor.
		// The application's own height/module gates select the served instance.
		set := precompiles.GetCustomPrecompiles(app.LatestUpgrade, keepers)
		producer, ok := set[address][precompiles.MergeUpgrade].(*pofq.PofqPrecompile)
		require.True(t, ok, "precompiles/layerxagent/pofq/merge.go: registry must serve the real pofq producer")
		require.NotNil(t, producer)
		require.Equal(t, address, producer.Address())
		require.Equal(t, pofq.PrecompileName, producer.GetName())
		if previous != nil {
			require.NotSame(t, previous, producer)
		}
		previous = producer
		k.SetCustomPrecompiles(set, app.LatestUpgrade)
		require.NotContains(t, k.CustomPrecompiles(ctx.WithBlockHeight(99)), address)
		served := k.CustomPrecompiles(ctx)
		require.Same(t, producer, served[address])
		callCtx, _ := ctx.CacheContext()
		db := state.NewDBImpl(callCtx, k, false)
		block, err := k.GetVMBlockContext(callCtx, core.GasPool(math.MaxUint64))
		require.NoError(t, err)
		chain := evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(callCtx))
		frames := 0
		hooks := &tracing.Hooks{OnEnter: func(_ int, _ byte, _ common.Address, to common.Address, _ []byte, _ uint64, _ *big.Int) {
			if to == address {
				frames++
			}
		}}
		evm := vm.NewEVM(*block, db, chain, vm.Config{Tracer: hooks}, served)
		a := producer.GetABI()
		call := func(input []byte, gas uint64) ([]byte, uint64, error) {
			before := frames
			var output []byte
			var left uint64
			var callErr error
			if pass == 0 {
				output, left, callErr = evm.StaticCall(caller, address, input, gas)
			} else {
				output, left, callErr = evm.Call(caller, address, input, gas, uint256.NewInt(0))
			}
			require.Equal(t, before+1, frames, "every request must enter the EVM")
			require.NoError(t, db.Err())
			return output, left, callErr
		}
		decode := func(name string, output []byte, want []string) {
			values, err := a.Methods[name].Outputs.Unpack(output)
			require.NoError(t, err)
			require.Len(t, values, len(want))
			require.Len(t, output, 32*len(want))
			for i, text := range want {
				value, ok := values[i].(*big.Int)
				require.True(t, ok)
				require.Equal(t, text, value.String())
			}
			// The damaged payload comes from this real producer invocation.
			values, err = a.Methods[name].Outputs.Unpack(output[:len(output)-1])
			require.Error(t, err)
			require.Empty(t, values)
		}
		for _, v := range vectors {
			method := a.Methods[v.name]
			require.Equal(t, v.signature, method.Sig)
			require.Equal(t, v.selector, method.ID)
			require.Equal(t, v.selector, crypto.Keccak256([]byte(v.signature))[:4])
			require.True(t, method.IsConstant())
			input, err := a.Pack(v.name, v.args...)
			require.NoError(t, err)
			require.Equal(t, v.gas, agentcommon.MethodGas(input[:4], len(input)))
			require.Equal(t, v.gas, producer.RequiredGas(input))
			output, left, err := call(input, v.gas)
			require.NoError(t, err)
			require.Zero(t, left, "full input including selector must be charged")
			decode(v.name, output, v.want)
			// Even on the same instance a second call pays for fresh execution.
			again, left, err := call(input, v.gas)
			require.NoError(t, err)
			require.Zero(t, left)
			require.Equal(t, output, again)
			decode(v.name, again, v.want)
			_, left, err = call(input, v.gas-1)
			require.ErrorIs(t, err, vm.ErrOutOfGas)
			require.Zero(t, left)
		}
		// Actual decoded outputs cross the ABI again as rolling-score inputs.
		batchInput, err := a.Pack(pofq.ScoreBatchMethod, fills)
		require.NoError(t, err)
		output, _, err := call(batchInput, 10000)
		require.NoError(t, err)
		batch, err := a.Methods[pofq.ScoreBatchMethod].Outputs.Unpack(output)
		require.NoError(t, err)
		rollingInput, err := a.Pack(pofq.UpdateRollingScoreMethod, big.NewInt(0), big.NewInt(0), batch[0], batch[1], uint16(5000))
		require.NoError(t, err)
		output, _, err = call(rollingInput, 10000)
		require.NoError(t, err)
		decode(pofq.UpdateRollingScoreMethod, output, []string{"625000000000000000", "4"})
		changed, err := a.Pack(pofq.ScoreFillMethod, big.NewInt(1500), big.NewInt(1000))
		require.NoError(t, err)
		output, _, err = call(changed, 10000)
		require.NoError(t, err)
		decode(pofq.ScoreFillMethod, output, []string{"500000000000000000"})
		original, err := a.Pack(vectors[0].name, vectors[0].args...)
		require.NoError(t, err)
		output, _, err = call(original, 10000)
		require.NoError(t, err)
		decode(pofq.ScoreFillMethod, output, vectors[0].want)
		require.Empty(t, db.GetAllLogs())
		require.Empty(t, db.Ctx().EventManager().Events())

		refuse := func(input []byte, prefix string) {
			output, _, err := call(input, 100000)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err := abi.UnpackRevert(output)
			require.NoError(t, err)
			require.Contains(t, reason, prefix)
			// Malformed copies of real refusal output are also rejected.
			_, err = abi.UnpackRevert(output[:len(output)-32])
			require.Error(t, err)
		}
		zeroOracle, err := a.Pack(pofq.ScoreFillMethod, big.NewInt(1000), big.NewInt(0))
		require.NoError(t, err)
		refuse(zeroOracle, "LXP_AGENT_OROB_ORACLE_PRICE: oraclePrice cannot be zero")
		refuse(batchInput[:len(batchInput)-1], "LXP_AGENT_ABI_DECODE:")
		refuse([]byte{0xde, 0xad, 0xbe, 0xef}, "LXP_PRECOMPILE_UNKNOWN_METHOD:")
		badDecay, err := a.Pack(pofq.UpdateRollingScoreMethod, big.NewInt(1), big.NewInt(1), big.NewInt(1), big.NewInt(1), uint16(10000))
		require.NoError(t, err)
		refuse(badDecay, "LXP_AGENT_POFQ_DECAY_RANGE: decayBps must be < 10000")
		tooMany := make([]pofq.Fill, pofq.MergeMaxBatch+1)
		for i := range tooMany {
			tooMany[i] = fills[0]
		}
		oversized, err := a.Pack(pofq.ScoreBatchMethod, tooMany)
		require.NoError(t, err)
		refuse(oversized, "LXP_AGENT_BATCH_TOO_LARGE:")
		// Corrupt the real producer's uint128 size at its actual ABI word.
		malformed := bytes.Clone(batchInput)
		malformed[132] = 1
		refuse(malformed, "LXP_AGENT_ABI_DECODE:")
	}
}
