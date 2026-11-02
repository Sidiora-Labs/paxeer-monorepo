package layerxchains_test

import (
	"encoding/json"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	chainskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/keeper"
	chains "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	lc "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxchains"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type chainsHarness struct {
	app        *app.App
	keeper     *chainskeeper.Keeper
	keepers    utils.Keepers
	precompile *lc.Precompile
	db         *state.DBImpl
	evm        *vm.EVM
	caller     common.Address
	authority  common.Address
}

func newChainsHarness(t *testing.T) *chainsHarness {
	t.Helper()
	application := testkeeper.EVMTestApp
	ctx, _ := application.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(42).WithBlockTime(time.Unix(1700000000, 0)).WithEVMTxHash(common.HexToHash("0x2022"))
	caller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	authority := common.HexToAddress("0x2222222222222222222222222222222222222222")
	params := chains.DefaultParams()
	params.Authority = authority
	params.MinBond = sdk.NewInt(100000000)
	params.CreationFee = sdk.NewInt(1000000)
	params.MinGasAllowance = 100000
	params.MaxGasAllowance = 10000000
	params.MaxTotalGasAllowance = 10000000
	application.LayerXChainsKeeper.SetParams(ctx, params)
	evmParams := application.EvmKeeper.GetParams(ctx)
	evmParams.MinimumFeePerGas = sdk.NewDec(42000000)
	application.EvmKeeper.SetParams(ctx, evmParams)
	funds := sdk.NewCoins(sdk.NewCoin(application.EvmKeeper.GetBaseDenom(ctx), params.MinBond.Add(params.CreationFee).MulRaw(10)))
	require.NoError(t, application.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, application.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, sdk.AccAddress(caller.Bytes()), funds))
	keepers := application.GetPrecompileKeepers()
	p, err := lc.NewServedPrecompile(application.LayerXChainsKeeper, keepers.EVMK())
	require.NoError(t, err)
	db := state.NewDBImpl(ctx.WithEventManager(sdk.NewEventManager()), &application.EvmKeeper, false)
	block, err := application.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	config := evmtypes.DefaultChainConfig().EthereumConfig(application.EvmKeeper.ChainID(ctx))
	evm := vm.NewEVM(*block, db, config, vm.Config{}, map[common.Address]vm.PrecompiledContract{p.Address(): p})
	return &chainsHarness{application, application.LayerXChainsKeeper, keepers, p, db, evm, caller, authority}
}

func (h *chainsHarness) call(t *testing.T, caller common.Address, value *big.Int, name string, args ...interface{}) ([]byte, uint64, error) {
	t.Helper()
	a := h.precompile.GetABI()
	input, err := a.Pack(name, args...)
	require.NoError(t, err)
	return h.evm.Call(caller, h.precompile.Address(), input, 2000000, uint256.MustFromBig(value))
}

func (h *chainsHarness) read(t *testing.T, name string, args ...interface{}) []interface{} {
	t.Helper()
	a := h.precompile.GetABI()
	input, err := a.Pack(name, args...)
	require.NoError(t, err)
	output, left, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
	require.NoError(t, err)
	require.GreaterOrEqual(t, uint64(2000000)-left, h.precompile.RequiredGas(input))
	result, err := a.Methods[name].Outputs.Unpack(output)
	require.NoError(t, err)
	return result
}

func (h *chainsHarness) create(t *testing.T) uint32 {
	t.Helper()
	params := h.read(t, "params")
	value := new(big.Int).Add(params[1].(*big.Int), params[2].(*big.Int))
	output, left, err := h.call(t, h.caller, value, "createChain", "demo", uint64(500000), uint64(65536), uint16(100))
	require.NoError(t, err)
	a := h.precompile.GetABI()
	input, err := a.Pack("createChain", "demo", uint64(500000), uint64(65536), uint16(100))
	require.NoError(t, err)
	require.Equal(t, uint64(60000+30*len(input[4:])), h.precompile.RequiredGas(input))
	require.GreaterOrEqual(t, uint64(2000000)-left, h.precompile.RequiredGas(input))
	values, err := a.Methods["createChain"].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Equal(t, []interface{}{uint32(1)}, values)
	return values[0].(uint32)
}

func TestChainsPrecompileRegistersMergeConstructor(t *testing.T) {
	h := newChainsHarness(t)
	address := common.HexToAddress(lc.ChainsAddress)
	require.Equal(t, address, h.precompile.Address())
	require.Equal(t, lc.PrecompileName, h.precompile.GetName())
	require.ErrorContains(t, precompiles.RegisterMergeConstructor(address, lc.NewMergePrecompile), "already registered")
	for _, keepers := range []utils.Keepers{nil, &utils.EmptyKeepers{}, &app.PrecompileKeepers{}} {
		p, err := lc.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
		require.ErrorContains(t, err, "LXP_PRECOMPILE_NO_CONTEXT")
		require.Nil(t, p)
	}
	p, err := lc.NewPrecompile(nil)
	require.ErrorContains(t, err, "LXP_PRECOMPILE_NO_CONTEXT")
	require.Nil(t, p)
	served, err := lc.NewServedPrecompile(h.keeper, nil)
	require.ErrorContains(t, err, "LXP_PRECOMPILE_NO_CONTEXT")
	require.Nil(t, served)
	merged, err := lc.NewMergePrecompile(h.keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.Implements(t, (*vm.DynamicGasPrecompiledContract)(nil), merged)
	custom, err := precompiles.GetCustomPrecompilesE(precompiles.MergeUpgrade, h.keepers)
	require.NoError(t, err)
	registered := custom[address][precompiles.MergeUpgrade]
	require.Implements(t, (*vm.DynamicGasPrecompiledContract)(nil), registered)
	require.Equal(t, lc.PrecompileName, registered.(precompiles.IPrecompile).GetName())
	require.Same(t, h.keeper, h.keepers.(lc.LayerXChainsKeepers).LayerXChainsK())
	a := h.precompile.GetABI()
	require.Len(t, a.Methods, 12)
	require.Len(t, a.Events, 4)
}

func TestChainsPrecompileCreatesReadsAndPausesAChain(t *testing.T) {
	h := newChainsHarness(t)
	before := h.app.BankKeeper.GetBalance(h.db.Ctx(), sdk.AccAddress(h.caller.Bytes()), h.app.EvmKeeper.GetBaseDenom(h.db.Ctx())).Amount
	id := h.create(t)
	a := h.precompile.GetABI()
	got := h.read(t, "getChain", id)
	data, err := os.ReadFile("../../tests/vectors/chains/abi-vectors.json")
	require.NoError(t, err)
	var vectors struct {
		Events []struct {
			Name   string `json:"name"`
			Topic0 string `json:"topic0"`
		} `json:"events"`
		GetChain struct {
			Returndata string `json:"returndata"`
			Decoded    struct {
				ID       uint32 `json:"id"`
				Name     string `json:"name"`
				Operator string `json:"operator"`
				ChainID  uint64 `json:"chainId"`
			} `json:"decoded"`
		} `json:"getChain"`
	}
	require.NoError(t, json.Unmarshal(data, &vectors))
	encoded, err := a.Methods["getChain"].Outputs.Pack(got...)
	require.NoError(t, err)
	expected, err := a.Methods["getChain"].Outputs.Unpack(common.FromHex(vectors.GetChain.Returndata))
	require.NoError(t, err)
	expected[9] = h.app.EvmKeeper.GetMinimumFeePerGas(h.db.Ctx()).TruncateInt().BigInt()
	expectedData, err := a.Methods["getChain"].Outputs.Pack(expected...)
	require.NoError(t, err)
	require.Equal(t, expectedData, encoded)
	require.Equal(t, []interface{}{id}, h.read(t, "chainByName", "demo"))
	require.Equal(t, []interface{}{uint32(1)}, h.read(t, "chainCount"))
	require.Equal(t, []interface{}{uint64(125000001)}, h.read(t, "chainIdOf", id))
	params := h.read(t, "params")
	require.Equal(t, h.authority, params[0])
	require.Equal(t, uint64(9500000), params[5])
	logs := h.db.GetAllLogs()
	require.Len(t, logs, 1)
	require.Equal(t, h.precompile.Address(), logs[0].Address)
	require.Equal(t, []common.Hash{common.HexToHash(vectors.Events[0].Topic0), common.BigToHash(new(big.Int).SetUint64(uint64(vectors.GetChain.Decoded.ID))), common.BytesToHash(common.HexToAddress(vectors.GetChain.Decoded.Operator).Bytes())}, logs[0].Topics)
	require.Equal(t, "ChainCreated", vectors.Events[0].Name)
	eventData, err := a.Events["ChainCreated"].Inputs.NonIndexed().Pack(vectors.GetChain.Decoded.Name, vectors.GetChain.Decoded.ChainID)
	require.NoError(t, err)
	require.Equal(t, eventData, logs[0].Data)
	after := h.app.BankKeeper.GetBalance(h.db.Ctx(), sdk.AccAddress(h.caller.Bytes()), h.app.EvmKeeper.GetBaseDenom(h.db.Ctx())).Amount
	require.True(t, before.Sub(after).Equal(sdk.NewInt(101000000)))
	require.True(t, h.app.BankKeeper.GetBalance(h.db.Ctx(), h.app.EvmKeeper.GetPaxAddressOrDefault(h.db.Ctx(), h.precompile.Address()), h.app.EvmKeeper.GetBaseDenom(h.db.Ctx())).IsZero())
	_, _, err = h.call(t, h.caller, big.NewInt(0), "pauseChain", id)
	require.NoError(t, err)
	require.Equal(t, uint8(2), h.read(t, "getChain", id)[3])
	require.Equal(t, uint64(10000000), h.read(t, "params")[5])
	statusData, err := a.Events["ChainStatusChanged"].Inputs.NonIndexed().Pack(uint8(2))
	require.NoError(t, err)
	logs = h.db.GetAllLogs()
	require.Len(t, logs, 2)
	require.Equal(t, []common.Hash{common.HexToHash(vectors.Events[1].Topic0), common.BigToHash(big.NewInt(1))}, logs[1].Topics)
	require.Equal(t, statusData, logs[1].Data)
	input, err := a.Pack("getChain", id)
	require.NoError(t, err)
	require.Equal(t, uint64(3000+3*32), h.precompile.RequiredGas(input))
	_, left, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, h.precompile.RequiredGas(input)-1)
	require.ErrorIs(t, err, vm.ErrOutOfGas)
	require.Zero(t, left)

	for _, name := range []string{"resumeChain", "governancePause", "governancePause", "setOperator", "fundChain", "retireChain"} {
		caller, value, args := h.caller, big.NewInt(0), []interface{}{id}
		event := "ChainStatusChanged"
		switch name {
		case "governancePause":
			caller = h.authority
			args = append(args, h.read(t, "getChain", id)[3] == uint8(1))
		case "setOperator":
			args = append(args, h.caller)
			event = "ChainOperatorChanged"
		case "fundChain":
			value = new(big.Int).Set(state.UhpxToSweiMultiplier)
			event = "ChainFunded"
		}
		_, _, err := h.call(t, caller, value, name, args...)
		require.NoError(t, err, name)
		emitted := h.db.GetAllLogs()
		require.Equal(t, a.Events[event].ID, emitted[len(emitted)-1].Topics[0], name)
	}
	retired := h.read(t, "getChain", id)
	require.Equal(t, uint8(4), retired[3])
	require.Zero(t, retired[8].(*big.Int).Sign())
	require.Equal(t, []interface{}{id}, h.read(t, "chainByName", "demo"))
}

func TestChainsPrecompileRefusesNonOperatorAndNonAuthority(t *testing.T) {
	h := newChainsHarness(t)
	id := h.create(t)
	stranger := common.HexToAddress("0x3333333333333333333333333333333333333333")
	for _, tc := range []struct {
		name, reason string
		args         []interface{}
	}{
		{"pauseChain", "LXP_UNAUTHORIZED_DEBIT:", []interface{}{id}},
		{"resumeChain", "LXP_UNAUTHORIZED_DEBIT:", []interface{}{id}},
		{"retireChain", "LXP_UNAUTHORIZED_DEBIT:", []interface{}{id}},
		{"setOperator", "LXP_UNAUTHORIZED_DEBIT:", []interface{}{id, stranger}},
		{"governancePause", "LXP_AUTH_SCOPE:", []interface{}{id, true}},
	} {
		output, _, err := h.call(t, stranger, big.NewInt(0), tc.name, tc.args...)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		reason, err := abi.UnpackRevert(output)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(reason, tc.reason), reason)
		require.Equal(t, uint8(1), h.read(t, "getChain", id)[3])
	}
	params := h.read(t, "params")
	below := new(big.Int).Add(params[1].(*big.Int), params[2].(*big.Int))
	below.Sub(below, state.UhpxToSweiMultiplier)
	before := h.app.BankKeeper.GetBalance(h.db.Ctx(), sdk.AccAddress(h.caller.Bytes()), h.app.EvmKeeper.GetBaseDenom(h.db.Ctx()))
	output, _, err := h.call(t, h.caller, below, "createChain", "other", uint64(500000), uint64(65536), uint16(100))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(reason, "LXP_INVALID_AMOUNT:"), reason)
	require.Equal(t, before, h.app.BankKeeper.GetBalance(h.db.Ctx(), sdk.AccAddress(h.caller.Bytes()), h.app.EvmKeeper.GetBaseDenom(h.db.Ctx())))
	require.Equal(t, []interface{}{uint32(1)}, h.read(t, "chainCount"))
	require.Len(t, h.db.GetAllLogs(), 1)
	a := h.precompile.GetABI()
	input, err := a.Pack("pauseChain", id)
	require.NoError(t, err)
	output, _, err = h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err = abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Contains(t, reason, "staticcall")
	require.Equal(t, uint8(1), h.read(t, "getChain", id)[3])
}

func TestChainsGateNamesTheModule(t *testing.T) {
	count := 0
	for _, entry := range precompiles.MergeAddressTable {
		if entry.Address == common.HexToAddress(lc.ChainsAddress) {
			count++
			require.Equal(t, chains.ModuleName, entry.Name)
		}
	}
	require.Equal(t, 1, count)
	require.Equal(t, chains.ModuleName, lc.PrecompileName)
	h := newChainsHarness(t)
	require.Same(t, h.app.LayerXChainsKeeper, h.keepers.(lc.LayerXChainsKeepers).LayerXChainsK())
}
