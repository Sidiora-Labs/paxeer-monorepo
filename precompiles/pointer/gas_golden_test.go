package pointer_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pointer"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	wasmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPointerGasGolden(t *testing.T) {
	const (
		planHeight  int64  = 101
		suppliedGas uint64 = 10_000_000
	)
	testApp := testkeeper.EVMTestApp
	p, err := pointer.NewPrecompile(testApp.GetPrecompileKeepers())
	require.NoError(t, err)
	golden := map[string]struct {
		gas  uint64
		wasm string
	}{
		"addNativePointer": {gas: 1_118_163},
		"addCW20Pointer":   {gas: 2_510_495, wasm: "cw20_base"},
		"addCW721Pointer":  {gas: 2_471_656, wasm: "cw721_base"},
		"addCW1155Pointer": {gas: 3_257_032, wasm: "cw1155_base"},
	}
	require.Len(t, p.ABI.Methods, len(golden), "every ABI method needs a gas golden")
	for name := range p.ABI.Methods {
		_, ok := golden[name]
		require.True(t, ok, "missing gas golden for %s", name)
	}

	baseCtx := testApp.GetContextForDeliverTx(nil).
		WithBlockHeight(planHeight).
		WithBlockTime(time.Unix(1_700_000_000, 0).UTC())
	baseCtx = baseCtx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(baseCtx))
	baseCtx, _ = baseCtx.CacheContext()
	testApp.UpgradeKeeper.SetDone(baseCtx, pcommon.MergeUpgrade)
	creator := sdk.AccAddress(common.HexToAddress("0x1111111111111111111111111111111111111111").Bytes())
	caller := common.HexToAddress("0x2222222222222222222222222222222222222222")

	for _, name := range []string{"addNativePointer", "addCW20Pointer", "addCW721Pointer", "addCW1155Pointer"} {
		t.Run(name, func(t *testing.T) {
			row := golden[name]
			fixtureCtx, _ := baseCtx.CacheContext()
			arg := "test"
			if row.wasm == "" {
				testApp.BankKeeper.SetDenomMetaData(fixtureCtx, banktypes.Metadata{
					Base: "test", Name: "base_name", Symbol: "base_symbol",
					DenomUnits: []*banktypes.DenomUnit{{Exponent: 6, Denom: "denom", Aliases: []string{"DENOM"}}},
				})
			} else {
				code, err := os.ReadFile("../../contracts/wasm/" + row.wasm + ".wasm")
				require.NoError(t, err)
				wk := wasmkeeper.NewDefaultPermissionKeeper(testApp.WasmKeeper)
				codeID, err := wk.Create(fixtureCtx, creator, code, nil)
				require.NoError(t, err)
				msg := fmt.Sprintf(`{"name":"test","symbol":"test","minter":"%s"}`, creator.String())
				if name == "addCW20Pointer" {
					msg = `{"name":"test","symbol":"test","decimals":6,"initial_balances":[]}`
				}
				addr, _, err := wk.Instantiate(fixtureCtx, codeID, creator, creator, []byte(msg), "gas-golden", sdk.NewCoins())
				require.NoError(t, err)
				arg = addr.String()
			}
			input, err := p.ABI.Pack(name, arg)
			require.NoError(t, err)
			method := p.ABI.Methods[name]
			var charged [2]uint64
			for i, height := range []int64{planHeight - 1, planHeight} {
				t.Run(fmt.Sprintf("height_%d", height), func(t *testing.T) {
					ctx, _ := fixtureCtx.CacheContext()
					ctx = ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager())
					ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
					require.Equal(t, height == planHeight, pcommon.MergePlanActive(ctx, &testApp.EvmKeeper))
					statedb := state.NewDBImpl(ctx, &testApp.EvmKeeper, false)
					blockCtx, err := testApp.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(suppliedGas))
					require.NoError(t, err)
					cfg := types.DefaultChainConfig().EthereumConfig(testApp.EvmKeeper.ChainID(ctx))
					evm := vm.NewEVM(*blockCtx, statedb, cfg, vm.Config{}, testApp.EvmKeeper.CustomPrecompiles(ctx))
					ret, remaining, err := p.RunAndCalculateGas(evm, caller, caller, input, suppliedGas, nil, nil, false, false)
					require.NoError(t, err)
					require.NoError(t, statedb.GetPrecompileError())
					require.Less(t, remaining, suppliedGas)
					charged[i] = suppliedGas - remaining
					t.Logf("%s height=%d charged=%d", name, height, charged[i])
					outputs, err := method.Outputs.Unpack(ret)
					require.NoError(t, err)
					require.Len(t, outputs, 1)
					addr, ok := outputs[0].(common.Address)
					require.True(t, ok)
					require.NotEqual(t, common.Address{}, addr)
					require.NotEmpty(t, statedb.GetCode(addr), "registration must deploy a real pointer")
					assert.Equal(t, row.gas, charged[i], "%s at height %d", name, height)
				})
			}
			require.NotZero(t, charged[0])
			require.NotZero(t, charged[1])
			require.Equal(t, charged[0], charged[1], "merge activation must preserve %s gas", name)
		})
	}
}
