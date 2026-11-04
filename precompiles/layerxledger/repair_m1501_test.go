package layerxledger_test

import (
	"math"
	"math/big"
	"os"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1501_InterfaceUnits(t *testing.T) {
	TestRepair_144_PayExactExpiresAtMs(t)
}

func TestRepairM_1501_ClientReceipt(t *testing.T) {
	TestLedgerEVMTransactionReceipt(t)
}

func TestRepairM_1501_GasConformance(t *testing.T) {
	TestLedgerGasConformance(t)
}

func TestRepairM_1501_BankMirror(t *testing.T) {
	TestBankMirrorMatchesKernelBalances(t)
}

func TestRepairM_1501_KernelParity(t *testing.T) {
	TestLedgerPrecompileKernelParity(t)
}

func TestRepairM_1501_TransfersRecovery(t *testing.T) {
	t.Run("openAccount", TestPrecompileOpenAccount)
	t.Run("send", TestPrecompileSend)
	t.Run("reopen", TestPrecompileSendReopen)
}

func TestRepairM_1501_ViewsRecovery(t *testing.T) {
	t.Run("views", TestPrecompileLedgerViews)
	t.Run("reopen", TestPrecompileNextSequenceReopen)
}

func repairM1501WebCall(t *testing.T, f *repair144Fixture, contract abi.ABI, name string, value *big.Int, args ...interface{}) ([]byte, []*ethtypes.Log, error) {
	t.Helper()
	input, err := contract.Pack(name, args...)
	require.NoError(t, err)
	db := state.NewDBImpl(f.p.ctx.WithEventManager(sdk.NewEventManager()), &f.p.app.EvmKeeper, false)
	block, err := f.p.app.EvmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	config := evmtypes.DefaultChainConfig().EthereumConfig(f.p.app.EvmKeeper.ChainID(f.p.ctx))
	evm := vm.NewEVM(*block, db, config, vm.Config{}, f.p.app.EvmKeeper.CustomPrecompiles(db.Ctx()))
	evm.TxContext.Origin = f.sender
	address := common.HexToAddress(xweb.XWebAddress)
	require.Contains(t, f.p.app.EvmKeeper.CustomPrecompiles(db.Ctx()), address)
	var data []byte
	if contract.Methods[name].IsConstant() {
		data, _, err = evm.StaticCall(f.sender, address, input, 2_000_000)
	} else {
		data, _, err = evm.Call(f.sender, address, input, 2_000_000, uint256.MustFromBig(value))
	}
	if err == nil {
		_, finalizeErr := db.Finalize()
		require.NoError(t, finalizeErr)
	}
	return data, db.GetAllLogs(), err
}

func TestRepairM_1501_RetiredGrantsAndWebPayments(t *testing.T) {
	f := newRepair144Fixture(t)
	web, err := xweb.NewPrecompile(f.p.app.GetPrecompileKeepers())
	require.NoError(t, err)
	contract := web.GetABI()
	read := func(paused bool) *big.Int {
		t.Helper()
		data, logs, err := repairM1501WebCall(t, f, contract, "getParams", nil)
		require.NoError(t, err)
		require.Empty(t, logs)
		require.Len(t, data, 5*32, "the wallet consumes five canonical ABI words")
		values, err := contract.Methods["getParams"].Outputs.Unpack(data)
		require.NoError(t, err)
		params := abi.ConvertType(values[0], new(xweb.ParamsView)).(*xweb.ParamsView)
		require.Equal(t, paused, params.Paused)
		want := f.p.app.XWebKeeper.Fee(f.p.ctx).MulRaw(1_000_000_000_000).BigInt()
		require.Zero(t, params.Fee.Cmp(want))
		feeData, _, err := repairM1501WebCall(t, f, contract, "fee", nil)
		require.NoError(t, err)
		fees, err := contract.Methods["fee"].Outputs.Unpack(feeData)
		require.NoError(t, err)
		require.Zero(t, fees[0].(*big.Int).Cmp(params.Fee))
		return params.Fee
	}
	f.refused(t, f.sender, "LXP_NO_PAYER_GRANT", "getPayerGrant", [32]byte{0x99})
	read(true)
	params := f.p.app.XWebKeeper.GetParams(f.p.ctx)
	require.NoError(t, f.p.app.XWebKeeper.UpdateParams(f.p.ctx, xwebtypes.MsgSetParams{
		Authority: params.Authority, Fee: sdk.NewInt(75), MaxPayloadBytes: 256,
		MaxCallbackGas: 300_000, TimeoutBlocks: 100,
		KernelNetworkId:  uint32(f.p.app.EvmKeeper.ChainID(f.p.ctx).Uint64()),
		ProgramFeeDenoms: []string{xwebtypes.ProgramFeeDenomPAX},
	}))
	require.NoError(t, f.p.app.XWebKeeper.Unpause(f.p.ctx, xwebtypes.MsgUnpause{Authority: params.Authority}))
	fee := read(false)
	payer := f.p.app.EvmKeeper.GetPaxAddressOrDefault(f.p.ctx, f.sender)
	funds := sdk.NewCoins(sdk.NewInt64Coin(sdk.MustGetBaseDenom(), 1000))
	require.NoError(t, f.p.app.BankKeeper.MintCoins(f.p.ctx, "mint", funds))
	require.NoError(t, f.p.app.BankKeeper.SendCoinsFromModuleToAccount(f.p.ctx, "mint", payer, funds))
	before := f.p.app.BankKeeper.GetBalance(f.p.ctx, payer, sdk.MustGetBaseDenom()).Amount
	payload := []byte("https://paxeer.app/")
	data, logs, err := repairM1501WebCall(t, f, contract, "request", fee, uint8(xwebtypes.KindFetch), payload, uint64(200_000))
	require.NoError(t, err)
	values, err := contract.Methods["request"].Outputs.Unpack(data)
	require.NoError(t, err)
	id := values[0].(uint64)
	require.Positive(t, id)
	request, found := f.p.app.XWebKeeper.GetRequest(f.p.ctx, id)
	require.True(t, found)
	require.Equal(t, xwebtypes.Address20(f.sender), request.Requester)
	require.Equal(t, xwebtypes.Keccak(payload), request.PayloadHash)
	require.Equal(t, sdk.NewInt(75), request.Fee)
	require.Len(t, logs, 1)
	require.Equal(t, common.HexToAddress(xweb.XWebAddress), logs[0].Address)
	event := contract.Events["XWebRequested"]
	require.Equal(t, []common.Hash{event.ID, common.BigToHash(new(big.Int).SetUint64(id)), common.BytesToHash(f.sender.Bytes())}, logs[0].Topics)
	fields, err := event.Inputs.NonIndexed().Unpack(logs[0].Data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{uint8(xwebtypes.KindFetch), payload, uint64(200_000), fee, uint64(request.TimeoutHeight)}, fields)
	after := f.p.app.BankKeeper.GetBalance(f.p.ctx, payer, sdk.MustGetBaseDenom()).Amount
	require.True(t, before.SubRaw(75).Equal(after))
	f.refused(t, f.sender, "LXP_NO_PAYER_GRANT", "getPayerGrant", [32]byte{0x99})
	f.reopen(t)
	read(false)
	recovered, found := f.p.app.XWebKeeper.GetRequest(f.p.ctx, id)
	require.True(t, found)
	require.Equal(t, request, recovered)
	require.True(t, after.Equal(f.p.app.BankKeeper.GetBalance(f.p.ctx, payer, sdk.MustGetBaseDenom()).Amount))
	f.refused(t, f.sender, "LXP_NO_PAYER_GRANT", "getPayerGrant", [32]byte{0x99})

	// The bounded Go gate checks consumer wiring alongside real node reads;
	// it does not invoke a separate JavaScript toolchain.
	source, err := os.ReadFile("../../sdk/wallet/src/modules/web-data.ts")
	require.NoError(t, err)
	sdkSource := string(source)
	require.NotContains(t, sdkSource, "getPayerGrant(bytes32)")
	require.Contains(t, sdkSource, "throw new WebPayerGrantsRetiredError()")
	require.Contains(t, sdkSource, "readonly code = 'payer_grants_retired'")
	require.Contains(t, sdkSource, "abiSelector('getParams()')")
	require.Contains(t, sdkSource, "decodeWebDataPaymentAvailability(await ethCall(provider, target, PARAMS_SELECTOR))")
	source, err = os.ReadFile("../../apps/wallet/src/surfaces/useSurface.ts")
	require.NoError(t, err)
	uiSource := string(source)
	require.NotContains(t, uiSource, "async function issuedGrants(")
	require.NotContains(t, uiSource, "async function indexedGrants(")
	require.NotContains(t, uiSource, ".caps(ids)")
	require.Contains(t, uiSource, "webData(observedProvider).paymentAvailability()")
	require.Contains(t, uiSource, "state: 'retired'")
	require.Contains(t, uiSource, "Web requests are available")
}
