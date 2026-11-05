package common_test

import (
	"crypto/ed25519"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxverify"
	putils "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type retirementOptionExecutor struct {
	common.PrecompileExecutor
}

func (e *retirementOptionExecutor) EVMKeeper() putils.EVMKeeper {
	return &testkeeper.EVMTestApp.EvmKeeper
}

func retirementOptionCall(t *testing.T) (*common.Precompile, []byte, []byte) {
	t.Helper()
	stateless, err := layerxverify.NewPrecompile(nil)
	require.NoError(t, err)
	p := common.NewPrecompile(stateless.GetABI(), &retirementOptionExecutor{stateless.GetExecutor()}, stateless.Address(), stateless.GetName())
	seed := [ed25519.SeedSize]byte{1}
	key := ed25519.NewKeyFromSeed(seed[:])
	var publicKey [ed25519.PublicKeySize]byte
	copy(publicKey[:], key.Public().(ed25519.PublicKey))
	message := []byte("retirement option")
	input, err := p.GetABI().Pack(layerxverify.VerifyEd25519Method, publicKey,
		layerxverify.RawMessageDomain, message, ed25519.Sign(key, message))
	require.NoError(t, err)
	method := p.GetABI().Methods[layerxverify.VerifyEd25519Method]
	want, err := method.Outputs.Pack(true)
	require.NoError(t, err)
	return p, input, want
}

func runRetirementOption(ctx sdk.Context, p *common.Precompile, input []byte) ([]byte, error) {
	db := state.NewDBImpl(ctx, &testkeeper.EVMTestApp.EvmKeeper, false)
	return p.Run(&vm.EVM{StateDB: db}, ethcommon.Address{}, ethcommon.Address{}, input, nil, true, false, nil)
}

func TestWithRetirementRevertsFromPlanHeight(t *testing.T) {
	const planHeight = int64(200)
	k := &testkeeper.EVMTestApp.EvmKeeper
	ctx, _ := testkeeper.EVMTestApp.GetContextForDeliverTx(nil).CacheContext()
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(planHeight), common.MergeUpgrade)
	baseline, input, want := retirementOptionCall(t)
	gas := baseline.RequiredGas(input)
	require.Equal(t, layerxverify.Gas(uint64(len(input)-4), 1, 0), gas)
	res, err := runRetirementOption(ctx.WithBlockHeight(planHeight), baseline, input)
	require.NoError(t, err)
	require.Equal(t, want, res)

	for _, height := range []int64{planHeight, planHeight + 1} {
		for _, methods := range [][]string{nil, {layerxverify.VerifyEd25519Method}} {
			for _, reasons := range []string{"plain", "revert reasons", "LXP reasons"} {
				t.Run(fmt.Sprintf("height=%d/methods=%v/%s", height, methods, reasons), func(t *testing.T) {
					p, _, _ := retirementOptionCall(t)
					require.Same(t, p, p.WithRetirement(common.MergeUpgrade, methods...))
					switch reasons {
					case "revert reasons":
						p.WithRevertReasons()
					case "LXP reasons":
						p.WithLXPReasons()
					}
					for _, call := range [][]byte{input, input[:4]} {
						require.Equal(t, baseline.RequiredGas(call), p.RequiredGas(call))
						res, err := runRetirementOption(ctx.WithBlockHeight(height), p, call)
						require.ErrorIs(t, err, vm.ErrExecutionReverted)
						require.Equal(t, common.RetiredRevert(p.GetName()), res)
						require.Equal(t, "LXP_MODULE_DISABLED: module disabled; layerxVerify retired", resolutionDecodeReason(t, res))
					}
				})
			}
		}
	}

	t.Run("unlisted method still serves", func(t *testing.T) {
		p, _, _ := retirementOptionCall(t)
		p.WithRetirement(common.MergeUpgrade, layerxverify.VerifyReceiptMethod)
		res, err := runRetirementOption(ctx.WithBlockHeight(planHeight), p, input)
		require.NoError(t, err)
		require.Equal(t, want, res)
		require.Equal(t, gas, p.RequiredGas(input))
	})

	t.Run("executor without keeper still serves", func(t *testing.T) {
		executor := baseline.GetExecutor().(*retirementOptionExecutor).PrecompileExecutor
		p := common.NewPrecompile(baseline.GetABI(), executor, baseline.Address(), baseline.GetName()).WithRetirement(common.MergeUpgrade)
		for _, height := range []int64{planHeight, planHeight + 1} {
			res, err := runRetirementOption(ctx.WithBlockHeight(height), p, input)
			require.NoError(t, err)
			require.Equal(t, want, res)
			require.Equal(t, gas, p.RequiredGas(input))
		}
	})
}

func TestWithRetirementServesBelowPlanHeight(t *testing.T) {
	const planHeight = int64(200)
	k := &testkeeper.EVMTestApp.EvmKeeper
	ctx, _ := testkeeper.EVMTestApp.GetContextForDeliverTx(nil).CacheContext()
	baseline, input, want := retirementOptionCall(t)
	for _, applied := range []bool{false, true} {
		if applied {
			k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(planHeight), common.MergeUpgrade)
		}
		for _, methods := range [][]string{nil, {layerxverify.VerifyEd25519Method}} {
			t.Run(fmt.Sprintf("applied=%t/methods=%v", applied, methods), func(t *testing.T) {
				p, _, _ := retirementOptionCall(t)
				p.WithRetirement(common.MergeUpgrade, methods...)
				callCtx := ctx.WithBlockHeight(planHeight - 1)
				meter := sdk.NewGasMeter(1_000_000, 1, 1)
				plain, err := runRetirementOption(callCtx.WithGasMeter(meter), baseline, input)
				require.NoError(t, err)
				require.Equal(t, want, plain)
				plainGas := meter.GasConsumed()
				meter = sdk.NewGasMeter(1_000_000, 1, 1)
				res, err := runRetirementOption(callCtx.WithGasMeter(meter), p, input)
				require.NoError(t, err)
				require.Equal(t, plain, res)
				require.Equal(t, plainGas, meter.GasConsumed())
				require.Equal(t, baseline.RequiredGas(input), p.RequiredGas(input))
				require.Positive(t, p.RequiredGas(input))
			})
		}
	}
}
