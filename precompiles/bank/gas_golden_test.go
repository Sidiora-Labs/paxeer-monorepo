package bank_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestBankGasGolden(t *testing.T) {
	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.GetContextForDeliverTx(nil).CacheContext()
	k := &testApp.EvmKeeper
	const planHeight = int64(100)
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(planHeight), pcommon.MergeUpgrade)
	p, err := bank.NewPrecompile(testApp.GetPrecompileKeepers())
	require.NoError(t, err)
	executor, ok := p.GetExecutor().(*bank.PrecompileExecutor)
	require.True(t, ok)

	from := common.HexToAddress("0x0000000000000000000000000000000000000641")
	to := common.HexToAddress("0x0000000000000000000000000000000000000642")
	const denom = "uhpx"
	contextHash := [32]byte{0x42}

	// Freeze the bare minimum RequiredGas, not execution-dependent store gas.
	// These literals intentionally do not use DefaultGasCost or KVGasConfig:
	// changing either the read/write classification or its costs must fail here.
	// Key by canonical signature so the two send overloads stay distinct.
	golden := map[string]struct {
		args []interface{}
		gas  uint64
	}{
		"all_balances(address)": {
			args: []interface{}{from}, gas: 1096,
		},
		"balance(address,string)": {
			args: []interface{}{from, denom}, gas: 1384,
		},
		"decimals(string)": {
			args: []interface{}{denom}, gas: 1288,
		},
		"name(string)": {
			args: []interface{}{denom}, gas: 1288,
		},
		"send(address,address,string,uint256)": {
			args: []interface{}{from, to, denom, big.NewInt(7)}, gas: 7760,
		},
		"send(address,address,string,uint256,bytes32)": {
			args: []interface{}{from, to, denom, big.NewInt(7), contextHash}, gas: 8720,
		},
		"sendNative(string)": {
			args: []interface{}{sdk.AccAddress(to.Bytes()).String()}, gas: 5840,
		},
		"supply(string)": {
			args: []interface{}{denom}, gas: 1288,
		},
		"symbol(string)": {
			args: []interface{}{denom}, gas: 1288,
		},
	}
	require.Len(t, p.ABI.Methods, len(golden), "every bank ABI method needs a gas golden")
	for name, method := range p.ABI.Methods {
		t.Run(method.Sig, func(t *testing.T) {
			row, covered := golden[method.Sig]
			require.True(t, covered, "missing gas golden for %s", method.Sig)
			input, err := p.ABI.Pack(name, row.args...)
			require.NoError(t, err)

			var gas [2]uint64
			for i, height := range []int64{planHeight - 1, planHeight} {
				callCtx := ctx.WithBlockHeight(height)
				evm := &vm.EVM{StateDB: state.NewDBImpl(callCtx, k, false)}
				preparedCtx, preparedMethod, args, err := p.Prepare(evm, input)
				require.NoError(t, err)
				require.Equal(t, height, preparedCtx.BlockHeight())
				require.Equal(t, height >= planHeight, pcommon.MergePlanActive(preparedCtx, k))
				require.Equal(t, method.Sig, preparedMethod.Sig)
				require.Equal(t, row.args, args)

				// DynamicGasPrecompile exposes RequiredGas on its bank executor.
				// Like Precompile.RequiredGas, pass only ABI arguments, without
				// the four-byte selector. The quote is context-independent even
				// for the context-hash overload, whose execution is plan-gated.
				gas[i] = executor.RequiredGas(input[4:], preparedMethod)
				require.Equal(t, row.gas, gas[i], "RequiredGas at height %d", height)
				t.Logf("height=%d RequiredGas=%d", height, gas[i])
			}
			require.Equal(t, gas[0], gas[1], "merge plan changed RequiredGas")
		})
	}
}
