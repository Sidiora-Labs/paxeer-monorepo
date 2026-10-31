package wasmd_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/wasmd"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	wasmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWasmdGasGolden(t *testing.T) {
	const planHeight int64 = 100
	const suppliedGas uint64 = 1_000_000
	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(planHeight - 1).
		WithBlockTime(time.Unix(1_700_000_000, 0).UTC()).
		WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)).WithIsEVM(true)
	k := &testApp.EvmKeeper
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(planHeight), pcommon.MergeUpgrade)
	require.Equal(t, planHeight, k.UpgradeKeeper().GetDoneHeight(ctx, pcommon.MergeUpgrade))
	require.False(t, pcommon.MergePlanActive(ctx, k))
	require.True(t, pcommon.MergePlanActive(ctx.WithBlockHeight(planHeight), k))

	caller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	sender := sdk.AccAddress(caller.Bytes())
	k.SetAddressMapping(ctx, sender, caller)
	testApp.AccountKeeper.SetAccount(ctx, testApp.AccountKeeper.NewAccountWithAddress(ctx, sender))
	wasmKeeper := wasmkeeper.NewDefaultPermissionKeeper(testApp.WasmKeeper)
	code, err := os.ReadFile("../../tests/fixtures/example/cosmwasm/echo/artifacts/echo.wasm")
	require.NoError(t, err)
	codeID, err := wasmKeeper.Create(ctx, sender, code, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(4), codeID)
	contract, _, err := wasmKeeper.Instantiate(ctx, codeID, sender, sender, []byte("{}"), "gas golden", sdk.NewCoins())
	require.NoError(t, err)
	p, err := wasmd.NewPrecompile(testApp.GetPrecompileKeepers())
	require.NoError(t, err)
	require.Equal(t, wasmd.Address, p.Address())

	message := []byte(`{"echo":{"message":"gas golden"}}`)
	coins := []byte("[]")
	wantEcho := []byte(fmt.Sprintf("received gas golden from %s with", sender.String()))
	cases := []struct {
		method string
		args   []interface{}
		gas    uint64
	}{
		{"instantiate", []interface{}{uint64(4), sender.String(), []byte("{}"), "gas golden", coins}, 91_615},
		{"execute", []interface{}{contract.String(), message, coins}, 65_060},
		{"execute_batch", []interface{}{[]wasmd.ExecuteMsg{
			{ContractAddress: contract.String(), Msg: message, Coins: coins},
			{ContractAddress: contract.String(), Msg: message, Coins: coins},
		}}, 130_120},
		{"query", []interface{}{contract.String(), []byte(`{"info":{}}`)}, 63_787},
	}
	require.Len(t, p.ABI.Methods, len(cases), "every ABI method must have a gas golden")
	seen := make(map[string]bool, len(cases))
	for _, tc := range cases {
		require.Contains(t, p.ABI.Methods, tc.method)
		require.False(t, seen[tc.method], "duplicate gas golden: %s", tc.method)
		seen[tc.method] = true
		t.Run(tc.method, func(t *testing.T) {
			input, err := p.ABI.Pack(tc.method, tc.args...)
			require.NoError(t, err)
			var charged [2]uint64
			var results [2][]byte
			for i, height := range []int64{planHeight - 1, planHeight} {
				callCtx, _ := ctx.CacheContext()
				callCtx = callCtx.WithBlockHeight(height).
					WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)).WithEventManager(sdk.NewEventManager())
				require.Equal(t, i == 1, pcommon.MergePlanActive(callCtx, k))
				db := state.NewDBImpl(callCtx, k, true)
				evm := &vm.EVM{StateDB: db}
				result, remaining, err := p.RunAndCalculateGas(evm, caller, caller, input, suppliedGas, nil, nil, false, false)
				require.NoError(t, err, "height %d", height)
				require.NoError(t, db.GetPrecompileError(), "height %d", height)
				require.Less(t, remaining, suppliedGas, "height %d must charge dynamic gas", height)
				require.NotZero(t, remaining, "height %d must complete within the gas budget", height)
				charged[i] = suppliedGas - remaining
				results[i] = result
				outputs, err := p.ABI.Methods[tc.method].Outputs.Unpack(result)
				require.NoError(t, err)
				switch tc.method {
				case "instantiate":
					require.Len(t, outputs, 2)
					address, err := sdk.AccAddressFromBech32(outputs[0].(string))
					require.NoError(t, err)
					require.NotEqual(t, contract, address)
					info := testApp.WasmKeeper.GetContractInfo(db.Ctx(), address)
					require.NotNil(t, info)
					require.Equal(t, codeID, info.CodeID)
					require.Equal(t, sender.String(), info.Creator)
					require.Empty(t, outputs[1].([]byte))
				case "execute":
					require.Equal(t, []interface{}{wantEcho}, outputs)
				case "execute_batch":
					require.Equal(t, []interface{}{[][]byte{wantEcho, wantEcho}}, outputs)
				case "query":
					require.Equal(t, []interface{}{[]byte(`{"message":"query test"}`)}, outputs)
				}
			}
			require.Equal(t, results[0], results[1], "merge plan must preserve the method's result")
			require.Equal(t, charged[0], charged[1], "merge plan must preserve dynamic gas")
			for i, height := range []int64{planHeight - 1, planHeight} {
				t.Logf("height %d: dynamic gas %d", height, charged[i])
				assert.Equal(t, tc.gas, charged[i], "frozen dynamic gas at height %d", height)
			}
		})
	}
}
