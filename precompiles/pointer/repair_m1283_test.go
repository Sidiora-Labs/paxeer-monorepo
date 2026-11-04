package pointer_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pointer"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	wasmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const repair1283Gas uint64 = 20_000_000

var repair1283Methods = []string{pointer.AddNativePointer, pointer.AddCW20Pointer, pointer.AddCW721Pointer, pointer.AddCW1155Pointer}

func repair1283Context(t *testing.T) (sdk.Context, sdk.AccAddress, common.Address) {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(100).WithBlockTime(time.Unix(1_700_000_000, 0)).WithEventManager(sdk.NewEventManager())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	caller := common.HexToAddress("0x1283000000000000000000000000000000000001")
	creator := sdk.AccAddress(caller.Bytes())
	a.EvmKeeper.SetAddressMapping(ctx, creator, caller)
	funds := sdk.NewCoins(sdk.NewInt64Coin(a.EvmKeeper.GetBaseDenom(ctx), 1_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, creator, funds))
	return ctx, creator, caller
}

func repair1283EVM(t *testing.T, ctx sdk.Context, caller common.Address) (*vm.EVM, *state.DBImpl, abi.ABI) {
	t.Helper()
	a := testkeeper.EVMTestApp
	p, err := pointer.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	db := state.NewDBImpl(ctx, &a.EvmKeeper, false)
	t.Cleanup(db.Cleanup)
	block, err := a.EvmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(repair1283Gas))
	require.NoError(t, err)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(a.EvmKeeper.ChainID(ctx)), vm.Config{}, map[common.Address]vm.PrecompiledContract{p.Address(): p})
	evm.SetTxContext(vm.TxContext{Origin: caller})
	return evm, db, p.GetABI()
}

func repair1283Pointer(ctx sdk.Context, method, target string) (common.Address, uint16, bool) {
	k := &testkeeper.EVMTestApp.EvmKeeper
	switch method {
	case pointer.AddNativePointer:
		return k.GetERC20NativePointer(ctx, target)
	case pointer.AddCW20Pointer:
		return k.GetERC20CW20Pointer(ctx, target)
	case pointer.AddCW721Pointer:
		return k.GetERC721CW721Pointer(ctx, target)
	case pointer.AddCW1155Pointer:
		return k.GetERC1155CW1155Pointer(ctx, target)
	default:
		panic("unknown pointer method")
	}
}

func repair1283Instantiate(t *testing.T, ctx sdk.Context, creator sdk.AccAddress, code, message []byte) string {
	t.Helper()
	wk := wasmkeeper.NewDefaultPermissionKeeper(testkeeper.EVMTestApp.WasmKeeper)
	id, err := wk.Create(ctx, creator, code, nil)
	require.NoError(t, err)
	addr, _, err := wk.Instantiate(ctx, id, creator, creator, message, "repair-m1283", sdk.NewCoins())
	require.NoError(t, err)
	return addr.String()
}

func TestRepairM_1283_CurrentABIAndSolidity(t *testing.T) {
	TestPointerABIFrozen(t)
}

func TestRepairM_1283_HistoricalReplayABI(t *testing.T) {
	versions, err := os.ReadFile("versions")
	require.NoError(t, err)
	for _, version := range strings.Fields(string(versions)) {
		t.Run(version, func(t *testing.T) {
			path := filepath.Join("legacy", strings.ReplaceAll(version, ".", ""), "abi.json")
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			historical, err := abi.JSON(bytes.NewReader(data))
			require.NoError(t, err)
			want := 4
			if version == "v5.5.2" || version == "v5.5.5" || version == "v5.6.2" || version == "v5.7.5" || version == "v5.8.0" || version == "v6.0.0" {
				want = 3
			}
			require.Len(t, historical.Methods, want)
			require.Empty(t, historical.Events)
			selectors := map[string]string{pointer.AddNativePointer: "c31d960f", pointer.AddCW20Pointer: "ce875775", pointer.AddCW721Pointer: "9e5720f0", pointer.AddCW1155Pointer: "a3b62916"}
			for name, method := range historical.Methods {
				require.Equal(t, "payable", method.StateMutability, "replay ABI is versioned separately")
				require.Equal(t, selectors[name], fmt.Sprintf("%x", method.ID))
			}
		})
	}
}

func TestRepairM_1283_ZeroValueRegistrationAndValueRefusal(t *testing.T) {
	for _, method := range repair1283Methods {
		t.Run(method, func(t *testing.T) {
			ctx, creator, caller := repair1283Context(t)
			a := testkeeper.EVMTestApp
			target := "repair1283"
			if method == pointer.AddNativePointer {
				a.BankKeeper.SetDenomMetaData(ctx, banktypes.Metadata{Base: target, Name: "Repair Token", Symbol: "RPR", DenomUnits: []*banktypes.DenomUnit{{Denom: target}}})
			} else {
				fixture := map[string]string{pointer.AddCW20Pointer: "cw20_base", pointer.AddCW721Pointer: "cw721_base", pointer.AddCW1155Pointer: "cw1155_base"}[method]
				code, err := os.ReadFile("../../contracts/wasm/" + fixture + ".wasm")
				require.NoError(t, err)
				message := []byte(fmt.Sprintf(`{"name":"Repair Token","symbol":"RPR","minter":"%s"}`, creator.String()))
				if method == pointer.AddCW20Pointer {
					message = []byte(`{"name":"Repair Token","symbol":"RPR","decimals":6,"initial_balances":[]}`)
				}
				target = repair1283Instantiate(t, ctx, creator, code, message)
			}
			for _, value := range []uint64{1, 1_000_000_000_000, 0} {
				t.Run(fmt.Sprintf("value_%d", value), func(t *testing.T) {
					evm, db, api := repair1283EVM(t, ctx, caller)
					input, err := api.Pack(method, target)
					require.NoError(t, err)
					beforeCaller := db.GetBalance(caller).ToBig()
					beforePrecompile := db.GetBalance(common.HexToAddress(pointer.PointerAddress)).ToBig()
					module := a.EvmKeeper.GetEVMAddressOrDefault(db.Ctx(), a.EvmKeeper.AccountKeeper().GetModuleAddress(evmtypes.ModuleName))
					nonce := db.GetNonce(module)
					ret, _, err := evm.Call(caller, common.HexToAddress(pointer.PointerAddress), input, repair1283Gas, uint256.NewInt(value))
					if value != 0 {
						require.ErrorIs(t, err, vm.ErrExecutionReverted)
						require.EqualError(t, db.GetPrecompileError(), "sending funds to a non-payable function")
						require.Empty(t, ret)
						require.Equal(t, nonce, db.GetNonce(module))
						require.Equal(t, beforeCaller, db.GetBalance(caller).ToBig())
						require.Equal(t, beforePrecompile, db.GetBalance(common.HexToAddress(pointer.PointerAddress)).ToBig())
						require.Empty(t, db.Ctx().EventManager().Events())
						_, _, exists := repair1283Pointer(db.Ctx(), method, target)
						require.False(t, exists)
					} else {
						require.NoError(t, err)
						require.NoError(t, db.GetPrecompileError())
						outputs, err := api.Unpack(method, ret)
						require.NoError(t, err)
						require.Len(t, outputs, 1)
						addr, version, exists := repair1283Pointer(db.Ctx(), method, target)
						require.True(t, exists)
						require.NotZero(t, version)
						require.Equal(t, outputs[0], addr)
						require.NotEmpty(t, db.GetCode(addr))
						repair1283ReadMetadata(t, evm, caller, addr, method, "Repair Token", "RPR")
					}
					_, err = db.Finalize()
					require.NoError(t, err)
					_, _, exists := repair1283Pointer(ctx, method, target)
					require.Equal(t, value == 0, exists, "only a successful registration reaches the backing keeper store")
				})
			}
		})
	}
}

func repair1283ReadMetadata(t *testing.T, evm *vm.EVM, caller, addr common.Address, method, name, symbol string) {
	t.Helper()
	kind := map[string]string{pointer.AddNativePointer: "native", pointer.AddCW20Pointer: "cw20", pointer.AddCW721Pointer: "cw721", pointer.AddCW1155Pointer: "cw1155"}[method]
	api := artifacts.GetParsedABI(kind)
	for field, expected := range map[string]string{"name": name, "symbol": symbol} {
		input, err := api.Pack(field)
		require.NoError(t, err)
		ret, _, err := evm.StaticCall(caller, addr, input, repair1283Gas)
		require.NoError(t, err)
		out, err := api.Unpack(field, ret)
		require.NoError(t, err)
		require.Equal(t, []interface{}{expected}, out)
	}
}

func TestRepairM_1283_UntrustedMetadata(t *testing.T) {
	cases := map[string]string{
		"absent_fields":  `{}`,
		"absent_name":    `{"symbol":"RPR"}`,
		"absent_symbol":  `{"name":"Repair"}`,
		"null_name":      `{"name":null,"symbol":"RPR"}`,
		"null_symbol":    `{"name":"Repair","symbol":null}`,
		"numeric_name":   `{"name":42,"symbol":"RPR"}`,
		"numeric_symbol": `{"name":"Repair","symbol":42}`,
		"object_name":    `{"name":{},"symbol":"RPR"}`,
		"object_symbol":  `{"name":"Repair","symbol":{}}`,
		"array_name":     `{"name":[],"symbol":"RPR"}`,
		"array_symbol":   `{"name":"Repair","symbol":[]}`,
		"boolean_name":   `{"name":true,"symbol":"RPR"}`,
		"boolean_symbol": `{"name":"Repair","symbol":false}`,
		"empty_name":     `{"name":"","symbol":"RPR"}`,
		"empty_symbol":   `{"name":"Repair","symbol":""}`,
		"non_object":     `[]`,
		"null_object":    `null`,
		"malformed_json": `{"name":`,
	}
	for _, field := range []string{"name", "symbol"} {
		limit := pointer.MaxMetadataNameBytes
		if field == "symbol" {
			limit = pointer.MaxMetadataSymbolBytes
		}
		data := map[string]string{"name": "Repair", "symbol": "RPR"}
		data[field] = strings.Repeat("x", limit+1)
		raw, err := json.Marshal(data)
		require.NoError(t, err)
		cases["oversized_"+field] = string(raw)
		data[field] = strings.Repeat("é", limit/2+1)
		raw, err = json.Marshal(data)
		require.NoError(t, err)
		cases["oversized_utf8_"+field] = string(raw)
	}
	for _, method := range repair1283Methods[1:] {
		t.Run(method, func(t *testing.T) {
			for name, response := range cases {
				t.Run(name, func(t *testing.T) {
					ctx, creator, caller := repair1283Context(t)
					target := repair1283Instantiate(t, ctx, creator, repair1283MetadataContract([]byte(response)), []byte(`{}`))
					query := []byte(`{"contract_info":{}}`)
					if method == pointer.AddCW20Pointer {
						query = []byte(`{"token_info":{}}`)
					}
					address, err := sdk.AccAddressFromBech32(target)
					require.NoError(t, err)
					actual, err := testkeeper.EVMTestApp.WasmKeeper.QuerySmartSafe(ctx, address, query)
					require.NoError(t, err)
					require.Equal(t, []byte(response), actual, "the real Wasm VM must produce the hostile query response")
					evm, db, api := repair1283EVM(t, ctx, caller)
					input, err := api.Pack(method, target)
					require.NoError(t, err)
					k := &testkeeper.EVMTestApp.EvmKeeper
					module := k.GetEVMAddressOrDefault(db.Ctx(), k.AccountKeeper().GetModuleAddress(evmtypes.ModuleName))
					nonce := db.GetNonce(module)
					var callErr error
					require.NotPanics(t, func() {
						_, _, callErr = evm.Call(caller, common.HexToAddress(pointer.PointerAddress), input, repair1283Gas, uint256.NewInt(0))
					})
					require.ErrorIs(t, callErr, vm.ErrExecutionReverted)
					require.ErrorIs(t, db.GetPrecompileError(), pointer.ErrInvalidMetadata)
					require.Equal(t, nonce, db.GetNonce(module), "no deployment may precede validation")
					require.Empty(t, db.Ctx().EventManager().Events())
					_, err = db.Finalize()
					require.NoError(t, err)
					for _, registration := range repair1283Methods[1:] {
						_, _, exists := repair1283Pointer(ctx, registration, target)
						require.False(t, exists, "invalid metadata must leave no committed pointer")
					}
				})
			}
		})
	}
}

func TestRepairM_1283_MetadataSizeBoundaries(t *testing.T) {
	for _, method := range repair1283Methods[1:] {
		for _, lengths := range []struct{ name, symbol string }{
			{"N", "S"},
			{strings.Repeat("n", pointer.MaxMetadataNameBytes), strings.Repeat("s", pointer.MaxMetadataSymbolBytes)},
			{strings.Repeat("é", pointer.MaxMetadataNameBytes/2), strings.Repeat("é", pointer.MaxMetadataSymbolBytes/2)},
		} {
			t.Run(fmt.Sprintf("%s_%d_%d_%x", method, len(lengths.name), len(lengths.symbol), []byte(lengths.name)[:1]), func(t *testing.T) {
				ctx, creator, caller := repair1283Context(t)
				response, err := json.Marshal(map[string]string{"name": lengths.name, "symbol": lengths.symbol})
				require.NoError(t, err)
				target := repair1283Instantiate(t, ctx, creator, repair1283MetadataContract(response), []byte(`{}`))
				evm, db, api := repair1283EVM(t, ctx, caller)
				input, err := api.Pack(method, target)
				require.NoError(t, err)
				ret, _, err := evm.Call(caller, common.HexToAddress(pointer.PointerAddress), input, repair1283Gas, uint256.NewInt(0))
				require.NoError(t, err)
				require.NoError(t, db.GetPrecompileError())
				out, err := api.Unpack(method, ret)
				require.NoError(t, err)
				addr := out[0].(common.Address)
				require.NotEmpty(t, db.GetCode(addr))
				repair1283ReadMetadata(t, evm, caller, addr, method, lengths.name, lengths.symbol)
				_, err = db.Finalize()
				require.NoError(t, err)
				stored, _, exists := repair1283Pointer(ctx, method, target)
				require.True(t, exists)
				require.Equal(t, addr, stored)
			})
		}
	}
}

// This deliberately hostile CosmWasm guest returns the supplied transcript from
// its query export. It is uploaded, instantiated and queried by the production
// keeper/VM, never substituted for a keeper or precompile. The binary encoding
// below needs no compiler or external tool in the gate. Each query instance has
// a fresh memory and a bump allocator for the host's Region arguments.
func repair1283MetadataContract(response []byte) []byte {
	u32 := func(n uint32) []byte {
		var out []byte
		for n >= 128 {
			out = append(out, byte(n)|0x80)
			n >>= 7
		}
		return append(out, byte(n))
	}
	// Positive signed LEB128 for i32.const.
	constant := func(n uint32) []byte {
		var out []byte
		for {
			b := byte(n & 0x7f)
			n >>= 7
			if n == 0 && b&0x40 == 0 {
				return append(out, b)
			}
			out = append(out, b|0x80)
		}
	}
	name := func(s string) []byte { return append(u32(uint32(len(s))), []byte(s)...) }
	module := []byte{0, 'a', 's', 'm', 1, 0, 0, 0}
	section := func(id byte, payload []byte) {
		module = append(module, id)
		module = append(module, u32(uint32(len(payload)))...)
		module = append(module, payload...)
	}
	section(1, []byte{5, 0x60, 0, 0, 0x60, 1, 0x7f, 1, 0x7f, 0x60, 1, 0x7f, 0, 0x60, 3, 0x7f, 0x7f, 0x7f, 1, 0x7f, 0x60, 2, 0x7f, 0x7f, 1, 0x7f})
	section(3, []byte{5, 0, 1, 2, 3, 4})
	section(5, []byte{1, 0, 16}) // one memory, 16 pages, no maximum
	global := []byte{1, 0x7f, 1, 0x41}
	global = append(global, constant(65536)...)
	section(6, append(global, 0x0b))
	exports := []byte{6}
	for i, export := range []string{"interface_version_8", "allocate", "deallocate", "instantiate", "query"} {
		exports = append(exports, name(export)...)
		exports = append(exports, 0, byte(i))
	}
	exports = append(exports, name("memory")...)
	exports = append(exports, 2, 0)
	section(7, exports)
	initResult := []byte(`{"ok":{"messages":[],"attributes":[],"events":[],"data":null}}`)
	queryResult := []byte(`{"ok":"` + base64.StdEncoding.EncodeToString(response) + `"}`)
	// Static response Regions at 1024 and 4096, below the allocator's heap.
	bodies := [][]byte{
		{0, 0x0b},
		// local ptr; Region.offset=ptr+12, capacity=size, length=size; heap+=size+12.
		{1, 1, 0x7f, 0x23, 0, 0x21, 1,
			0x20, 1, 0x20, 1, 0x41, 12, 0x6a, 0x36, 2, 0,
			0x20, 1, 0x20, 0, 0x36, 2, 4,
			0x20, 1, 0x20, 0, 0x36, 2, 8,
			0x20, 1, 0x41, 12, 0x6a, 0x20, 0, 0x6a, 0x24, 0,
			0x20, 1, 0x0b},
		{0, 0x0b},
		append(append([]byte{0, 0x41}, constant(1024)...), 0x0b),
		append(append([]byte{0, 0x41}, constant(4096)...), 0x0b),
	}
	code := []byte{byte(len(bodies))}
	for _, body := range bodies {
		code = append(code, u32(uint32(len(body)))...)
		code = append(code, body...)
	}
	section(10, code)
	data := []byte{2}
	for i, payload := range [][]byte{initResult, queryResult} {
		offset := uint32(1024)
		if i == 1 {
			offset = 4096
		}
		region := make([]byte, 12)
		binary.LittleEndian.PutUint32(region, offset+12)
		binary.LittleEndian.PutUint32(region[4:], uint32(len(payload)))
		binary.LittleEndian.PutUint32(region[8:], uint32(len(payload)))
		region = append(region, payload...)
		data = append(data, 0, 0x41)
		data = append(data, constant(offset)...)
		data = append(data, 0x0b)
		data = append(data, u32(uint32(len(region)))...)
		data = append(data, region...)
	}
	section(11, data)
	return module
}
