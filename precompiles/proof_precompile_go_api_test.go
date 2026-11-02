// Proves: InitializePrecompiles/GetCustomPrecompiles -> GetPrecompileInfo/GetPrecompileInfoE/PrecompileInfo -> cli.BuildPrecompileCall, cli.BuildEvmPointerCall, tests.RegisterCW20PointerTx, messages.ERC20toCWAssets, msgs.FeeDenomPreference and ABI.Unpack.
//
// Repairs: setup.go/GetPrecompileInfoE accepted a zero destination; native_tx.go,
// rpc/tests/tx.go and tests/occ/messages/test_msgs.go ignored the producer's
// destination; rpc/tests/tx.go discarded packing errors; tx.go/CmdCallPrecompile
// silently ignored surplus arguments. The CLI builders below are the command's
// own path before signing or RPC. The sequencer's private position_test.go/
// xwebInfo cannot be imported; its GetPrecompileInfoE -> ABI.Pack -> ABI.Unpack
// path is exercised here with the real served xweb precompile, including a
// truncated producer response. This metadata API has no durable state to reopen.
// Outside scope: tests/testutil/processblock/tx.go/TxConfig has an empty interface
// registry, so its decoder is expected to refuse MsgEVMTransaction; the node's
// App.GetTxConfig decoder must accept the same producer bytes.
package precompiles_test

import (
	"maps"
	"math"
	"math/big"
	"testing"

	evmcli "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	spottypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/feetoken"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pointer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pricefeed"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	rpctests "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/tests"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/messages"
	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/processblock"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/processblock/msgs"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func proofGoAPIRegistry(t *testing.T) {
	t.Helper()
	before, initialized := precompiles.PrecompileNamesToInfo, precompiles.Initialized
	precompiles.PrecompileNamesToInfo = map[string]precompiles.PrecompileInfo{}
	precompiles.Initialized = false
	t.Cleanup(func() {
		precompiles.PrecompileNamesToInfo, precompiles.Initialized = before, initialized
	})
	require.NoError(t, precompiles.InitializePrecompiles(true, testkeeper.EVMTestApp.GetPrecompileKeepers()))
}

func TestProofPrecompileGoApiTradingProducers(t *testing.T) {
	proofGoAPIRegistry(t)
	rows := []struct {
		name, address, method, signature, argument string
		want                                       interface{}
	}{
		{"perps", perps.PerpsAddress, "getCurrentFundingRate", "getCurrentFundingRate(uint256)", "7", big.NewInt(7)},
		{"pricefeed", pricefeed.PriceFeedAddress, "getPriceBySymbol", "getPriceBySymbol(string)", "BTC", "BTC"},
		{"spot_pecor", spottypes.PECORAddress, "setSwapFee", "setSwapFee(uint256)", "11", big.NewInt(11)},
		{"spot_orders", spottypes.PECOROrdersAddress, "cancelLimitOrder", "cancelLimitOrder(uint256)", "12", big.NewInt(12)},
		{"spot_stoporders", spottypes.PECORStopOrdersAddress, "cancelStopLossOrder", "cancelStopLossOrder(uint256)", "13", big.NewInt(13)},
		{"spot_quoter", spottypes.PECORQuoterAddress, "getTokenPrice", "getTokenPrice(address)", "0x0000000000000000000000000000000000001234", common.HexToAddress("0x1234")},
		{"spot_vault", spottypes.PECORVaultAddress, "getTokenDecimals", "getTokenDecimals(address)", "0x0000000000000000000000000000000000001234", common.HexToAddress("0x1234")},
	}
	for round := 0; round < 2; round++ {
		// Rebuild from real constructors, discarding all prior registry entries.
		// Every consumer invocation must resolve the freshly produced entry.
		precompiles.PrecompileNamesToInfo = map[string]precompiles.PrecompileInfo{}
		require.NoError(t, precompiles.InitializePrecompiles(true, testkeeper.EVMTestApp.GetPrecompileKeepers()))
		for _, row := range rows {
			t.Run(row.name+big.NewInt(int64(round)).String(), func(t *testing.T) {
				producer := precompiles.PrecompileNamesToInfo[row.name]
				info, payload, err := evmcli.BuildPrecompileCall(row.name, []string{row.method, row.argument})
				require.NoError(t, err)
				require.Equal(t, common.HexToAddress(row.address), info.Address)
				require.Equal(t, producer, precompiles.GetPrecompileInfo(row.name))
				require.Equal(t, crypto.Keccak256([]byte(row.signature))[:4], payload[:4])
				method, err := producer.ABI.MethodById(payload[:4])
				require.NoError(t, err)
				require.Equal(t, row.method, method.Name)
				values, err := method.Inputs.Unpack(payload[4:])
				require.NoError(t, err)
				require.Equal(t, []interface{}{row.want}, values)
				_, err = method.Inputs.Unpack(payload[4:5])
				require.Error(t, err, "consumer must refuse truncated calldata")
				_, _, err = evmcli.BuildPrecompileCall(row.name, []string{row.method})
				require.Error(t, err)
				_, _, err = evmcli.BuildPrecompileCall(row.name, []string{row.method, row.argument, "surplus"})
				require.Error(t, err)
				malformed := producer
				malformed.ABI.Methods = maps.Clone(producer.ABI.Methods)
				delete(malformed.ABI.Methods, row.method)
				precompiles.PrecompileNamesToInfo[row.name] = malformed
				_, _, err = evmcli.BuildPrecompileCall(row.name, []string{row.method, row.argument})
				require.ErrorContains(t, err, "not found", "a consumer must resolve producer metadata on every call")
				precompiles.PrecompileNamesToInfo[row.name] = producer
			})
		}
	}
	_, _, err := evmcli.BuildPrecompileCall("perps", []string{"unknown_method"})
	require.ErrorContains(t, err, "not found")
	_, _, err = evmcli.BuildPrecompileCall("perps", nil)
	require.ErrorContains(t, err, "method is required")
	command := evmcli.CmdCallPrecompile()
	require.ErrorContains(t, command.RunE(command, []string{"perps", "getFees", "surplus"}), "expects 0 arguments")
	_, err = precompiles.GetPrecompileInfoE("unknown_precompile")
	require.Error(t, err)

	info := precompiles.GetPrecompileInfo("perps")
	info.Address = common.Address{}
	precompiles.PrecompileNamesToInfo["perps"] = info
	_, _, err = evmcli.BuildPrecompileCall("perps", []string{"getFees"})
	require.EqualError(t, err, "perps precompile has no address")
	require.Panics(t, func() { precompiles.GetPrecompileInfo("perps") })
}

func TestProofPrecompileGoApiTransactionConsumers(t *testing.T) {
	proofGoAPIRegistry(t)
	app := testkeeper.EVMTestApp
	ctx := app.GetContextForDeliverTx(nil)
	const contract = "pax1contract"
	for round := 0; round < 2; round++ {
		info, payload, err := evmcli.BuildEvmPointerCall("CW20", contract)
		require.NoError(t, err)
		want, err := info.ABI.Pack(pointer.AddCW20Pointer, contract)
		require.NoError(t, err)
		require.Equal(t, want, payload)
		rpcTx := ethtypes.NewTx(rpctests.RegisterCW20PointerTx(uint64(round), contract))
		require.Equal(t, &info.Address, rpcTx.To())
		require.Equal(t, want, rpcTx.Data())
		require.Equal(t, uint64(round), rpcTx.Nonce())
		generated := messages.ERC20toCWAssets(&occutils.TestContext{
			TestApp: app, Ctx: ctx, CW20Addrs: []string{contract},
		}, 1)
		require.Len(t, generated, 1)
		msg := generated[0].Msg.(*evmtypes.MsgEVMTransaction)
		ethTx, _ := msg.AsTransaction()
		require.Equal(t, &info.Address, ethTx.To())
		require.Equal(t, want, ethTx.Data())
		sender, err := ethtypes.Sender(generated[0].EVMSigner.EvmSigner, ethTx)
		require.NoError(t, err)
		require.Equal(t, generated[0].EVMSigner.EvmAddress, sender)
	}
	_, _, err := evmcli.BuildEvmPointerCall("UNKNOWN", contract)
	require.ErrorContains(t, err, "invalid pointer type")

	// A malformed producer ABI must fail before the RPC helper creates a tx.
	info := precompiles.GetPrecompileInfo(pointer.PrecompileName)
	info.ABI.Methods = maps.Clone(info.ABI.Methods)
	delete(info.ABI.Methods, pointer.AddCW20Pointer)
	precompiles.PrecompileNamesToInfo[pointer.PrecompileName] = info
	require.Panics(t, func() { rpctests.RegisterCW20PointerTx(0, contract) })
	_, _, err = evmcli.BuildEvmPointerCall("CW20", contract)
	require.ErrorContains(t, err, "not found")
}

func TestProofPrecompileGoApiFeeTokenConsumer(t *testing.T) {
	app := processblock.NewTestApp(t)
	proofGoAPIRegistry(t)
	signer := msgs.FeeTokenSignerFromSeed(49)
	info := precompiles.GetPrecompileInfo(feetoken.PrecompileName)
	for round := 0; round < 2; round++ {
		tx := msgs.FeeDenomPreference(app, signer, uint64(round), big.NewInt(1_000_000_000), "uhpx")
		encoded, err := processblock.TxConfig.TxEncoder()(tx)
		require.NoError(t, err)
		_, helperErr := processblock.TxConfig.TxDecoder()(encoded)
		require.ErrorContains(t, helperErr, "unable to resolve type URL /paxprotocol.paxchain.evm.MsgEVMTransaction")
		decoded, err := app.GetTxConfig().TxDecoder()(encoded)
		require.NoError(t, err)
		msg := decoded.GetMsgs()[0].(*evmtypes.MsgEVMTransaction)
		ethTx, _ := msg.AsTransaction()
		require.Equal(t, &info.Address, ethTx.To())
		method, err := info.ABI.MethodById(ethTx.Data()[:4])
		require.NoError(t, err)
		require.Equal(t, feetoken.SetFeeDenomMethod, method.Name)
		values, err := method.Inputs.Unpack(ethTx.Data()[4:])
		require.NoError(t, err)
		require.Equal(t, []interface{}{"uhpx"}, values)
		recovered, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(ethTx.ChainId()), ethTx)
		require.NoError(t, err)
		require.Equal(t, signer.EVMAddr, recovered)
	}
	info.Address = common.Address{}
	precompiles.PrecompileNamesToInfo[feetoken.PrecompileName] = info
	require.Panics(t, func() { msgs.FeeDenomPreference(app, signer, 2, big.NewInt(1), "uhpx") })
}

func TestProofPrecompileGoApiServedResult(t *testing.T) {
	proofGoAPIRegistry(t)
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(8).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	custom := precompiles.GetCustomPrecompiles(precompiles.XWebUpgrade, app.GetPrecompileKeepers())
	info, err := precompiles.GetPrecompileInfoE(xweb.PrecompileName)
	require.NoError(t, err)
	served := custom[info.Address][precompiles.XWebUpgrade]
	require.NotNil(t, served)
	named, ok := served.(precompiles.IPrecompile)
	require.True(t, ok)
	require.Equal(t, info.Address, named.Address())
	require.Equal(t, info.ABI, named.GetABI())
	stateDB := state.NewDBImpl(ctx, &app.EvmKeeper, false)
	block, err := app.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	cfg := evmtypes.DefaultChainConfig().EthereumConfig(app.EvmKeeper.ChainID(ctx))
	evm := vm.NewEVM(*block, stateDB, cfg, vm.Config{}, map[common.Address]vm.PrecompiledContract{info.Address: served})
	input, err := info.ABI.Pack(xweb.FeeMethod)
	require.NoError(t, err)
	for round := int64(1); round <= 2; round++ {
		// Changing the actual keeper between calls exposes a cached response.
		params := app.XWebKeeper.GetParams(stateDB.Ctx())
		params.Fee = sdk.NewInt(round)
		require.NoError(t, app.XWebKeeper.SetParams(stateDB.Ctx(), params))
		out, _, err := served.(vm.DynamicGasPrecompiledContract).RunAndCalculateGas(evm,
			common.Address{}, common.Address{}, input, 1_000_000, big.NewInt(0), nil, true, false)
		require.NoError(t, err)
		values, err := info.ABI.Unpack(xweb.FeeMethod, out)
		require.NoError(t, err)
		require.Equal(t, []interface{}{sdk.NewInt(round).Mul(state.SdkUhpxToSweiMultiplier).BigInt()}, values)
		_, err = info.ABI.Unpack(xweb.FeeMethod, out[:len(out)-1])
		require.Error(t, err, "consumer must refuse a malformed producer response")
	}
}
