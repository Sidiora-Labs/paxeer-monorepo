package wasmd_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pointer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pointerview"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/wasmd"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	wasmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const repairM1275Gas uint64 = 20_000_000

var repairM1275Caller = common.HexToAddress("0x1275000000000000000000000000000000000001")
var repairM1275Other = common.HexToAddress("0x1275000000000000000000000000000000000002")

func repairM1275StoreSnapshot(t *testing.T, store sdk.KVStore) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	iter := store.Iterator(nil, nil)
	defer func() { require.NoError(t, iter.Close()) }()
	for ; iter.Valid(); iter.Next() {
		result[string(iter.Key())] = bytes.Clone(iter.Value())
	}
	return result
}

func repairM1275Context(a *app.App, ctx sdk.Context) sdk.Context {
	ctx = ctx.WithIsEVM(true).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)).WithEventManager(sdk.NewEventManager())
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), app.V6120Upgrade)
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), app.MergeUpgrade)
	a.RefreshPrecompiles(ctx)
	for _, addr := range []common.Address{repairM1275Caller, repairM1275Other} {
		a.EvmKeeper.SetAddressMapping(ctx, sdk.AccAddress(addr.Bytes()), addr)
	}
	return ctx
}

func repairM1275App(t *testing.T) (*app.App, sdk.Context) {
	t.Helper()
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(1).WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	return a, repairM1275Context(a, ctx)
}

func repairM1275VM(t *testing.T, a *app.App, ctx sdk.Context, hooks *tracing.Hooks) (*state.DBImpl, *vm.EVM) {
	t.Helper()
	db := state.NewDBImpl(ctx, &a.EvmKeeper, false)
	db.SetLogger(hooks)
	block, err := a.EvmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(a.EvmKeeper.ChainID(ctx))
	precompiles := a.EvmKeeper.CustomPrecompiles(ctx)
	require.Contains(t, precompiles, wasmd.Address)
	evm := vm.NewEVM(*block, db, chain, vm.Config{Tracer: hooks}, precompiles)
	evm.TxContext.Origin = repairM1275Caller
	return db, evm
}

func repairM1275Instantiate(t *testing.T, a *app.App, ctx sdk.Context, file, init string) sdk.AccAddress {
	t.Helper()
	code, err := os.ReadFile(file)
	require.NoError(t, err)
	wk := wasmkeeper.NewDefaultPermissionKeeper(a.WasmKeeper)
	sender := sdk.AccAddress(repairM1275Caller.Bytes())
	id, err := wk.Create(ctx, sender, code, nil)
	require.NoError(t, err)
	addr, _, err := wk.Instantiate(ctx, id, sender, sender, []byte(init), "repair-m1275", sdk.NewCoins())
	require.NoError(t, err)
	return addr
}

func repairM1275Pack(t *testing.T, contract abi.ABI, name string, args ...interface{}) []byte {
	t.Helper()
	input, err := contract.Pack(name, args...)
	require.NoError(t, err)
	return input
}

func repairM1275Finalize(t *testing.T, db *state.DBImpl) {
	t.Helper()
	_, err := db.Finalize()
	require.NoError(t, err)
	require.NoError(t, db.Error())
}

func TestRepairM_1275_QuerySelector(t *testing.T) {
	TestQuery(t)
}

func TestRepairM_1275_ValueSuccess(t *testing.T) {
	TestExecuteBatchValueImmutability(t)
}

func TestRepairM_1275_PointerArtifactProvenance(t *testing.T) {
	var manifest map[string]struct {
		Source          string `json:"source"`
		SourceSHA256    string `json:"source_sha256"`
		RuntimeOffset   int    `json:"runtime_offset"`
		Solc            string `json:"solc"`
		SelectorOffsets []int  `json:"selector_offsets"`
	}
	data, err := os.ReadFile("../common/pointer_artifacts/manifest.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &manifest))
	require.Len(t, manifest, 3)
	for _, kind := range []string{"cw20", "cw721", "cw1155"} {
		t.Run(kind, func(t *testing.T) {
			entry, ok := manifest[kind]
			require.True(t, ok)
			source := artifacts.GetBin(kind)
			require.Equal(t, fmt.Sprintf("%x", sha256.Sum256(source)), entry.SourceSHA256)
			require.Equal(t, "modules/evm/artifacts/"+kind+"/", entry.Source[:len("modules/evm/artifacts/"+kind+"/")])
			compiler := []byte{0, 8, 26}
			if kind == "cw20" {
				compiler[2] = 21
			}
			require.Equal(t, fmt.Sprintf("%d.%d.%d", compiler[0], compiler[1], compiler[2]), entry.Solc)
			require.True(t, bytes.HasSuffix(source, append(append([]byte("dsolcC"), compiler...), 0, 0x33)), "compiler metadata must match the recorded compiler")
			require.Equal(t, []byte{0xf3, 0xfe}, source[entry.RuntimeOffset-2:entry.RuntimeOffset])
			want := bytes.Clone(source[entry.RuntimeOffset:])
			require.NotEmpty(t, entry.SelectorOffsets)
			for _, offset := range entry.SelectorOffsets {
				require.Equal(t, []byte{0x63, 0x0c, 0x3c, 0x20, 0xed}, want[offset-1:offset+4], "only a PUSH4 selector operand may be relocated")
				copy(want[offset:offset+4], []byte{0x7e, 0x34, 0xd1, 0xdd})
			}
			runtime, err := pcommon.CWPointerRuntime(kind)
			require.NoError(t, err)
			require.Equal(t, want, runtime, "all bytes outside the recorded selector operands must remain identical")
			require.NotContains(t, string(runtime), string([]byte{0x63, 0x0c, 0x3c, 0x20, 0xed}))
		})
	}
}

func TestRepairM_1275_DelegatecallSuccess(t *testing.T) {
	for _, kind := range []string{"cw20", "cw721", "cw1155"} {
		t.Run(kind, func(t *testing.T) {
			a, ctx := repairM1275App(t)
			sender := sdk.AccAddress(repairM1275Caller.Bytes())
			other := sdk.AccAddress(repairM1275Other.Bytes())
			init := fmt.Sprintf(`{"name":"Repair token","symbol":"RPR","minter":"%s"}`, sender)
			if kind == "cw20" {
				init = `{"name":"Repair token","symbol":"RPR","decimals":6,"initial_balances":[]}`
			}
			contract := repairM1275Instantiate(t, a, ctx, "../../contracts/wasm/"+kind+"_base.wasm", init)
			pointerPrecompile, err := pointer.NewPrecompile(a.GetPrecompileKeepers())
			require.NoError(t, err)
			addrPrecompile, err := addr.NewPrecompile(a.GetPrecompileKeepers())
			require.NoError(t, err)
			pointerABI := pointerPrecompile.ABI
			register := map[string]string{"cw20": "addCW20Pointer", "cw721": "addCW721Pointer", "cw1155": "addCW1155Pointer"}[kind]
			db, evm := repairM1275VM(t, a, ctx, nil)
			out, _, err := evm.Call(repairM1275Caller, common.HexToAddress(pointer.PointerAddress), repairM1275Pack(t, pointerABI, register, contract.String()), repairM1275Gas, uint256.NewInt(0))
			require.NoError(t, err, "%x: %v", out, db.GetPrecompileError())
			values, err := pointerABI.Methods[register].Outputs.Unpack(out)
			require.NoError(t, err)
			require.Len(t, values, 1)
			registered := values[0].(common.Address)
			require.NotEmpty(t, db.GetCode(registered), "registration must deploy the production pointer bytecode")
			runtime, err := pcommon.CWPointerRuntime(kind)
			require.NoError(t, err)
			require.Equal(t, runtime, db.GetCode(registered), "registration must install the linked production artifact")
			repairM1275Finalize(t, db)
			db.Cleanup()

			assertRegistry := func() {
				t.Helper()
				viewABI := pointerview.GetABI()
				method := map[string]string{"cw20": "getCW20Pointer", "cw721": "getCW721Pointer", "cw1155": "getCW1155Pointer"}[kind]
				viewDB, viewVM := repairM1275VM(t, a, ctx, nil)
				defer viewDB.Cleanup()
				result, _, err := viewVM.StaticCall(repairM1275Caller, common.HexToAddress(pointerview.PointerViewAddress), repairM1275Pack(t, viewABI, method, contract.String()), repairM1275Gas)
				require.NoError(t, err, "%v", viewDB.GetPrecompileError())
				decoded, err := viewABI.Methods[method].Outputs.Unpack(result)
				require.NoError(t, err)
				require.Len(t, decoded, 3)
				require.Equal(t, registered, decoded[0])
				require.Positive(t, decoded[1].(uint16))
				require.Equal(t, true, decoded[2])
			}
			assertRegistry()

			contractABI := *artifacts.GetParsedABI(kind)
			assertAuthority := func(amount int64, approved bool) {
				t.Helper()
				for _, owner := range []common.Address{repairM1275Caller, repairM1275Other} {
					method := "isApprovedForAll"
					if kind == "cw20" {
						method = "allowance"
					}
					expectedAddressInputs := [][]byte{repairM1275Pack(t, addrPrecompile.ABI, addr.GetPaxAddressMethod, owner)}
					if kind == "cw721" {
						if owner == repairM1275Caller && approved {
							expectedAddressInputs = append(expectedAddressInputs, repairM1275Pack(t, addrPrecompile.ABI, addr.GetEvmAddressMethod, other.String()))
						}
					} else {
						expectedAddressInputs = append(expectedAddressInputs, repairM1275Pack(t, addrPrecompile.ABI, addr.GetPaxAddressMethod, repairM1275Other))
					}
					var addressInputs [][]byte
					viewDB, viewVM := repairM1275VM(t, a, ctx, &tracing.Hooks{OnEnter: func(_ int, typ byte, from, to common.Address, input []byte, _ uint64, _ *big.Int) {
						if to == common.HexToAddress(addr.AddrAddress) {
							require.Equal(t, byte(vm.STATICCALL), typ)
							require.Equal(t, registered, from)
							require.GreaterOrEqual(t, len(input), 4)
							addressInputs = append(addressInputs, bytes.Clone(input))
						}
					}})
					defer viewDB.Cleanup()
					result, _, err := viewVM.StaticCall(repairM1275Caller, registered, repairM1275Pack(t, contractABI, method, owner, repairM1275Other), repairM1275Gas)
					require.NoError(t, err, "%x: %v", result, viewDB.GetPrecompileError())
					require.Equal(t, expectedAddressInputs, addressInputs, "assert every current address selector, argument and call order")
					decoded, err := contractABI.Methods[method].Outputs.Unpack(result)
					require.NoError(t, err)
					require.Len(t, decoded, 1)
					if kind == "cw20" {
						want := int64(0)
						if owner == repairM1275Caller {
							want = amount
						}
						require.Zero(t, big.NewInt(want).Cmp(decoded[0].(*big.Int)), "allowance must equal the exact expected integer")
					} else {
						require.Equal(t, owner == repairM1275Caller && approved, decoded[0])
					}
				}
			}
			assertAuthority(0, false)
			before := repairM1275StoreSnapshot(t, ctx.KVStore(a.GetKey(wasmtypes.StoreKey)))
			var delegateInput, delegateOutput []byte
			delegateDepth, calls := -1, 0
			hooks := &tracing.Hooks{
				OnEnter: func(depth int, typ byte, from, to common.Address, input []byte, _ uint64, _ *big.Int) {
					if to == wasmd.Address && typ == byte(vm.DELEGATECALL) {
						require.Equal(t, registered, from)
						delegateInput = bytes.Clone(input)
						delegateDepth = depth
						calls++
					}
				},
				OnExit: func(depth int, output []byte, _ uint64, err error, reverted bool) {
					if depth == delegateDepth {
						require.NoError(t, err)
						require.False(t, reverted)
						delegateOutput = bytes.Clone(output)
					}
				},
			}
			method := "setApprovalForAll"
			args := []interface{}{repairM1275Other, true}
			if kind == "cw20" {
				method, args = "approve", []interface{}{repairM1275Other, big.NewInt(5)}
			}
			db, evm = repairM1275VM(t, a, ctx, hooks)
			out, gas, err := evm.Call(repairM1275Caller, registered, repairM1275Pack(t, contractABI, method, args...), repairM1275Gas, uint256.NewInt(0))
			require.NoError(t, err, "%x: %v", out, db.GetPrecompileError())
			require.NoError(t, db.GetPrecompileError())
			require.Positive(t, gas)
			require.Less(t, gas, repairM1275Gas)
			require.Equal(t, 1, calls, "pointer must execute the DELEGATECALL opcode to 0x1002")
			wasmdABI := wasmd.GetABI()
			require.Equal(t, wasmdABI.Methods[wasmd.ExecuteMethod].ID, delegateInput[:4])
			expected, err := wasmdABI.Methods[wasmd.ExecuteMethod].Outputs.Pack([]byte{})
			require.NoError(t, err)
			require.Equal(t, expected, delegateOutput, "assert the actual inner ABI return bytes")
			returned, err := contractABI.Methods[method].Outputs.Unpack(out)
			require.NoError(t, err)
			if kind == "cw20" {
				require.Equal(t, []interface{}{true}, returned)
			} else {
				require.Empty(t, returned)
			}
			repairM1275Finalize(t, db)
			db.Cleanup()
			require.NotEqual(t, before, repairM1275StoreSnapshot(t, ctx.KVStore(a.GetKey(wasmtypes.StoreKey))))
			assertAuthority(5, true)

			message := []byte(fmt.Sprintf(`{"approve_all":{"operator":"%s"}}`, other))
			if kind == "cw20" {
				message = []byte(fmt.Sprintf(`{"increase_allowance":{"spender":"%s","amount":"1"}}`, other))
			}
			batch := repairM1275Pack(t, wasmdABI, wasmd.ExecuteBatchMethod, []wasmd.ExecuteMsg{
				{ContractAddress: contract.String(), Msg: message, Coins: []byte("[]")},
				{ContractAddress: contract.String(), Msg: message, Coins: []byte("[]")},
			})
			db, evm = repairM1275VM(t, a, ctx, nil)
			out, gas, err = evm.DelegateCall(repairM1275Caller, registered, wasmd.Address, batch, repairM1275Gas, uint256.NewInt(0))
			require.NoError(t, err, "%v", db.GetPrecompileError())
			require.Positive(t, gas)
			expected, err = wasmdABI.Methods[wasmd.ExecuteBatchMethod].Outputs.Pack([][]byte{{}, {}})
			require.NoError(t, err)
			require.Equal(t, expected, out)
			repairM1275Finalize(t, db)
			db.Cleanup()
			assertAuthority(7, true)

			for _, input := range [][]byte{delegateInput, batch} {
				before = repairM1275StoreSnapshot(t, ctx.KVStore(a.GetKey(wasmtypes.StoreKey)))
				refusalCtx := ctx.WithEventManager(sdk.NewEventManager())
				db, evm = repairM1275VM(t, a, refusalCtx, nil)
				_, gas, err = evm.DelegateCall(repairM1275Caller, repairM1275Other, wasmd.Address, input, repairM1275Gas, uint256.NewInt(0))
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.ErrorContains(t, db.GetPrecompileError(), "is not a pointer of "+contract.String())
				require.Zero(t, gas)
				require.Empty(t, db.GetAllLogs())
				require.Equal(t, before, repairM1275StoreSnapshot(t, db.Ctx().KVStore(a.GetKey(wasmtypes.StoreKey))))
				repairM1275Finalize(t, db)
				require.Empty(t, refusalCtx.EventManager().Events())
				db.Cleanup()
				require.Equal(t, before, repairM1275StoreSnapshot(t, ctx.KVStore(a.GetKey(wasmtypes.StoreKey))))
			}
			assertAuthority(7, true)
			// Re-registration follows the keeper's existing-pointer deployment path
			// and must preserve both the address and the CosmWasm authority state.
			db, evm = repairM1275VM(t, a, ctx, nil)
			out, _, err = evm.Call(repairM1275Caller, common.HexToAddress(pointer.PointerAddress), repairM1275Pack(t, pointerABI, register, contract.String()), repairM1275Gas, uint256.NewInt(0))
			require.NoError(t, err, "%x: %v", out, db.GetPrecompileError())
			values, err = pointerABI.Methods[register].Outputs.Unpack(out)
			require.NoError(t, err)
			require.Equal(t, []interface{}{registered}, values)
			require.Equal(t, runtime, db.GetCode(registered))
			repairM1275Finalize(t, db)
			db.Cleanup()
			assertRegistry()
			assertAuthority(7, true)
		})
	}
}

// This EVM fixture emits LOG0, forwards the real ABI calldata and call value to
// 0x1002, and bubbles the complete return or revert data to its EVM caller.
func repairM1275BatchCaller() []byte {
	code := []byte{
		0x36, 0x60, 0x00, 0x60, 0x00, 0x37,
		0x60, 0x00, 0x60, 0x00, 0xa0,
		0x60, 0x00, 0x60, 0x00, 0x36, 0x60, 0x00, 0x34, 0x61, 0x10, 0x02, 0x5a, 0xf1,
		0x3d, 0x60, 0x00, 0x60, 0x00, 0x3e,
		0x60, 0x00, 0x57,
		0x3d, 0x60, 0x00, 0xfd,
		0x5b, 0x3d, 0x60, 0x00, 0xf3,
	}
	code[31] = 37 // JUMPDEST for successful CALL
	return code
}

func TestRepairM_1275_BatchRollbackProof(t *testing.T) {
	dir := t.TempDir()
	open := func(restarting bool) (*app.App, sdk.Context) {
		t.Helper()
		disk, err := sdk.NewLevelDB("repair-m1275", dir)
		require.NoError(t, err)
		encoding := app.MakeEncodingConfig()
		a := app.New(disk, nil, true, map[int64]bool{}, dir, 1, true,
			tmcfg.DefaultConfig().SetRoot(dir), encoding, nil, app.TestAppOpts{ReceiptBackend: "pebbledb"}, app.EmptyWasmOpts, nil)
		if !restarting {
			genesis, err := json.Marshal(app.NewDefaultGenesisState(encoding.Marshaler))
			require.NoError(t, err)
			_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
				Time: time.Unix(1_800_000_000, 0).UTC(), ChainId: "pax-test",
				ConsensusParams: app.DefaultConsensusParams, AppStateBytes: genesis,
			})
			require.NoError(t, err)
		}
		header := tmproto.Header{ChainID: "pax-test", Height: a.LastBlockHeight() + 1, Time: time.Unix(1_800_000_000, 0).UTC()}
		var ctx sdk.Context
		if restarting {
			ctx, _ = a.NewUncachedContext(false, header).CacheContext()
		} else {
			ctx = a.NewContext(false, header)
		}
		return a, repairM1275Context(a, ctx)
	}
	a, ctx := open(false)
	t.Cleanup(func() {
		if a != nil {
			require.NoError(t, a.Close())
		}
	})
	sender := sdk.AccAddress(repairM1275Caller.Bytes())
	receiver := sdk.AccAddress(repairM1275Other.Bytes())
	forwarder := common.HexToAddress("0x1275000000000000000000000000000000000003")
	forwarderPax := sdk.AccAddress(forwarder.Bytes())
	a.EvmKeeper.SetAddressMapping(ctx, forwarderPax, forwarder)
	a.EvmKeeper.SetCode(ctx, forwarder, repairM1275BatchCaller())
	contract := repairM1275Instantiate(t, a, ctx, "../../contracts/wasm/cw20_base.wasm", fmt.Sprintf(
		`{"name":"Rollback token","symbol":"ROLL","decimals":6,"initial_balances":[{"address":"%s","amount":"100"}]}`, forwarderPax))
	funds := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", funds))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", sender, funds))
	coins, err := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1000)).MarshalJSON()
	require.NoError(t, err)
	first := wasmd.ExecuteMsg{ContractAddress: contract.String(), Coins: coins,
		Msg: []byte(fmt.Sprintf(`{"transfer":{"recipient":"%s","amount":"7"}}`, receiver))}
	second := wasmd.ExecuteMsg{ContractAddress: contract.String(), Coins: []byte("[]"),
		Msg: []byte(fmt.Sprintf(`{"transfer":{"recipient":"%s","amount":"1000000"}}`, receiver))}
	wasmdABI := wasmd.GetABI()
	value := uint256.NewInt(1_000_000_000_000_000)
	beforeState := repairM1275StoreSnapshot(t, ctx.KVStore(a.GetKey(wasmtypes.StoreKey)))
	addresses := []sdk.AccAddress{sender, forwarderPax, contract, a.EvmKeeper.GetPaxAddressOrDefault(ctx, wasmd.Address)}
	balances := func(readCtx sdk.Context) []sdk.Coin {
		result := make([]sdk.Coin, len(addresses))
		for i, addr := range addresses {
			result[i] = a.BankKeeper.GetBalance(readCtx, addr, "uhpx")
		}
		return result
	}
	beforeBalances := balances(ctx)

	// A successful control proves the first message writes both token storage
	// and native balances, and the EVM fixture emits a real log. Discard its cache.
	controlCtx, _ := ctx.CacheContext()
	controlCtx = controlCtx.WithEventManager(sdk.NewEventManager())
	db, evm := repairM1275VM(t, a, controlCtx, nil)
	out, remaining, err := evm.Call(repairM1275Caller, forwarder, repairM1275Pack(t, wasmdABI, wasmd.ExecuteBatchMethod, []wasmd.ExecuteMsg{first}), repairM1275Gas, value)
	require.NoError(t, err, "%x: %v", out, db.GetPrecompileError())
	require.Positive(t, remaining)
	decoded, err := wasmdABI.Methods[wasmd.ExecuteBatchMethod].Outputs.Unpack(out)
	require.NoError(t, err)
	require.Len(t, decoded[0].([][]byte), 1)
	require.Len(t, db.GetAllLogs(), 1)
	require.Equal(t, forwarder, db.GetAllLogs()[0].Address)
	require.NotEqual(t, beforeState, repairM1275StoreSnapshot(t, db.Ctx().KVStore(a.GetKey(wasmtypes.StoreKey))))
	require.Equal(t, sdk.NewInt(1000), a.BankKeeper.GetBalance(db.Ctx(), contract, "uhpx").Amount)
	require.Equal(t, sdk.NewInt(9000), a.BankKeeper.GetBalance(db.Ctx(), sender, "uhpx").Amount)
	repairM1275Finalize(t, db)
	require.NotEmpty(t, controlCtx.EventManager().Events())
	db.Cleanup()
	require.Equal(t, beforeState, repairM1275StoreSnapshot(t, ctx.KVStore(a.GetKey(wasmtypes.StoreKey))))
	require.Equal(t, beforeBalances, balances(ctx))

	callCtx := ctx.WithEventManager(sdk.NewEventManager())
	observedLogs := 0
	db, evm = repairM1275VM(t, a, callCtx, &tracing.Hooks{OnLog: func(*ethtypes.Log) { observedLogs++ }})
	input := repairM1275Pack(t, wasmdABI, wasmd.ExecuteBatchMethod, []wasmd.ExecuteMsg{first, second})
	_, remaining, err = evm.Call(repairM1275Caller, forwarder, input, repairM1275Gas, value)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.ErrorContains(t, db.GetPrecompileError(), "Cannot Sub")
	require.ErrorContains(t, db.GetPrecompileError(), "1000000", "refusal must come from the second message")
	require.Less(t, remaining, repairM1275Gas)
	require.Equal(t, 1, observedLogs, "the reverted log must actually have been emitted")
	require.Empty(t, db.GetAllLogs())
	require.Equal(t, beforeState, repairM1275StoreSnapshot(t, db.Ctx().KVStore(a.GetKey(wasmtypes.StoreKey))))
	require.Equal(t, beforeBalances, balances(db.Ctx()))
	repairM1275Finalize(t, db)
	require.Empty(t, callCtx.EventManager().Events())
	require.Equal(t, beforeState, repairM1275StoreSnapshot(t, ctx.KVStore(a.GetKey(wasmtypes.StoreKey))))
	require.Equal(t, beforeBalances, balances(ctx))

	// Persist the production receipt made from the reverted StateDB, then stop
	// the entire application and reopen every store from the same on-disk home.
	txHash := common.HexToHash("0x1275")
	receipt, err := a.EvmKeeper.WriteReceipt(ctx, db, &core.Message{
		From: repairM1275Caller, To: &forwarder, GasPrice: big.NewInt(0), Data: input,
	}, 0, txHash, repairM1275Gas-remaining, vm.ErrExecutionReverted.Error())
	require.NoError(t, err)
	require.Zero(t, receipt.Status)
	require.Empty(t, receipt.Logs)
	db.Cleanup()
	require.NoError(t, a.SetSealedStateToCommit(ctx, ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	height := a.LastBlockHeight()
	require.Positive(t, height)
	require.NoError(t, a.Close())
	a = nil
	a, ctx = open(true)
	require.Equal(t, height, a.LastBlockHeight())
	require.Equal(t, beforeState, repairM1275StoreSnapshot(t, ctx.KVStore(a.GetKey(wasmtypes.StoreKey))))
	require.Equal(t, beforeBalances, balances(ctx))
	recovered, err := a.EvmKeeper.GetReceipt(ctx, txHash)
	require.NoError(t, err)
	require.Equal(t, receipt.Status, recovered.Status)
	require.Equal(t, receipt.VmError, recovered.VmError)
	require.Empty(t, recovered.Logs)
	result, err := a.WasmKeeper.QuerySmartSafe(ctx, contract, []byte(fmt.Sprintf(`{"balance":{"address":"%s"}}`, receiver)))
	require.NoError(t, err)
	require.JSONEq(t, `{"balance":"0"}`, string(result))
	result, err = a.WasmKeeper.QuerySmartSafe(ctx, contract, []byte(fmt.Sprintf(`{"balance":{"address":"%s"}}`, forwarderPax)))
	require.NoError(t, err)
	require.JSONEq(t, `{"balance":"100"}`, string(result))
}
