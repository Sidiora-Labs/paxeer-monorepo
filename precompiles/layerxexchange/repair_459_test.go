package layerxexchange_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	exchangekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxexchange"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func repair459Harness(t *testing.T) (*harness, [32]byte, [32]byte, [32]byte) {
	t.Helper()
	h := newHarness(t)
	app := testkeeper.EVMTestApp
	hostKey, ledgerKey := sdk.NewKVStoreKey("lxphost"), sdk.NewKVStoreKey(ledgertypes.StoreKey)
	h.at(withStore(withStore(h.ctx.WithChainID("hyperpax_125-1"), hostKey), ledgerKey))
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: hostKey, NetworkID: 125, Epoch: 1,
		ParameterVersion: 1, TimestampWindowMs: 60_000, GasLimit: 10_000_000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	auth := authkeeper.NewAccountKeeper(app.AppCodec(), app.GetKey(authtypes.StoreKey),
		app.GetSubspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{ledgertypes.ModuleName: {authtypes.Minter}})
	bank := bankkeeper.NewBaseKeeper(app.AppCodec(), app.GetKey(banktypes.StoreKey), auth,
		app.GetSubspace(banktypes.ModuleName), nil)
	ledger := ledgerkeeper.NewKeeper(app.AppCodec(), ledgerKey, host, bank, &app.EvmKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String())
	bind := func(caller common.Address, seedByte byte) ledgerkeeper.Actor {
		var seed [ed25519.SeedSize]byte
		seed[0] = seedByte
		private := ed25519.NewKeyFromSeed(seed[:])
		var key [32]byte
		copy(key[:], private.Public().(ed25519.PublicKey))
		message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(h.ctx), caller, 0)
		require.NoError(t, err)
		var signature [64]byte
		copy(signature[:], ed25519.Sign(private, message))
		_, err = app.EvmKeeper.BindLayerX(h.ctx, caller, key, signature)
		require.NoError(t, err)
		actor, err := ledger.Actor(h.ctx, caller)
		require.NoError(t, err)
		return actor
	}
	actor := bind(h.caller, 1)
	foreignActor := bind(common.Address{0x45, 0x09, 2}, 2)
	salt := [32]byte{0x31}
	material := append([]byte("LX:ASSET:v1"), actor.Authority.Actor[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	_, err = ledger.RegisterAsset(h.ctx, actor, ledgertypes.RegisterArgs{AssetID: asset, Salt: salt,
		Symbol: "R459", Name: "Repair 459", Decimals: 6, SupplyCap: sdk.NewInt(1000),
		IssuerKind: ledgertypes.AssetIssuerNative}, [32]byte{0x31})
	require.NoError(t, err)
	open := func(actor ledgerkeeper.Actor, nonce byte) [32]byte {
		_, err := ledger.OpenAccount(h.ctx, actor, asset, [32]byte{nonce})
		require.NoError(t, err)
		_, id, err := lxphost.AssetAccountID(actor.DIDKey, asset)
		require.NoError(t, err)
		owner, err := host.ReadAccountOwner(h.ctx, id)
		require.NoError(t, err)
		require.Equal(t, actor.Authority.Actor, owner)
		return id
	}
	owned, foreign := open(actor, 0x32), open(foreignActor, 0x33)
	main, err := evmtypes.LayerXMainAccountID(actor.DIDKey)
	require.NoError(t, err)
	require.NotEqual(t, main, owned)
	h.ctx = h.ctx.WithEventManager(sdk.NewEventManager())
	h.at(h.ctx)
	return h, main, owned, foreign
}

func repair459Method(t *testing.T, h *harness, signature string) *abi.Method {
	t.Helper()
	for _, method := range h.precompile.GetABI().Methods {
		if method.Sig == signature {
			return &method
		}
	}
	t.Fatalf("ABI method missing: %s", signature)
	return nil
}

func repair459Input(t *testing.T, method *abi.Method, args ...interface{}) []byte {
	t.Helper()
	packed, err := method.Inputs.Pack(args...)
	require.NoError(t, err)
	return append(append([]byte(nil), method.ID...), packed...)
}

func repair459Run(t *testing.T, h *harness, input []byte, readOnly bool, value *big.Int) ([]byte, error) {
	t.Helper()
	snapshot := h.stateDB.Snapshot()
	h.ctx = h.stateDB.Ctx()
	data, err := h.precompile.Run(h.evm, h.caller, h.caller, input, value, readOnly, false, nil)
	if err != nil {
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		cause := h.stateDB.GetPrecompileError()
		require.Error(t, cause)
		h.stateDB.RevertToSnapshot(snapshot)
		h.ctx = h.stateDB.Ctx()
		return data, cause
	}
	return data, nil
}

func repair459Intent(t *testing.T, h *harness, output []byte, account [32]byte, nonce uint64) types.Intent {
	t.Helper()
	require.Len(t, output, 32)
	id := common.BytesToHash(output)
	require.Equal(t, h.intentID(types.IntentKind_INTENT_KIND_PLACE, nonce), [32]byte(id))
	intent, found := h.keeper.GetIntent(h.ctx, id)
	require.True(t, found)
	require.Equal(t, types.Intent{IntentId: custodytypes.Hash32(id), Kind: types.IntentKind_INTENT_KIND_PLACE,
		Status: types.IntentStatus_INTENT_STATUS_PENDING, Owner: custodytypes.Address(h.caller),
		Nonce: nonce, Height: h.ctx.BlockHeight(), Account: custodytypes.Hash32(account),
		AssetId: custodytypes.Hash32(h.native), MarketId: custodytypes.Hash32(marketID),
		Side: uint32(types.SideBuy), Price: "30000", Quantity: "2", TimeInForce: uint32(types.TimeInForceGoodTillCancelled)}, intent)
	return intent
}

func TestRepair459PlaceOrderOverloads(t *testing.T) {
	h, main, owned, _ := repair459Harness(t)
	legacy := repair459Method(t, h, layerxexchange.PlaceOrderSignature)
	named := repair459Method(t, h, layerxexchange.PlaceOrderAccountSignature)
	require.NotEqual(t, legacy.ID, named.ID)
	require.NotEqual(t, legacy.Name, named.Name)
	input := repair459Input(t, named, marketID, owned, types.SideBuy, big.NewInt(30000), big.NewInt(2), types.TimeInForceGoodTillCancelled)
	require.Equal(t, layerxexchange.Gas(uint64(len(input)-4), 0, 0, 3), h.precompile.RequiredGas(input))
	output, err := repair459Run(t, h, input, false, nil)
	require.NoError(t, err)
	repair459Intent(t, h, output, owned, 1)
	input = repair459Input(t, legacy, marketID, types.SideBuy, big.NewInt(30000), big.NewInt(2), types.TimeInForceGoodTillCancelled)
	require.Equal(t, layerxexchange.Gas(uint64(len(input)-4), 0, 0, 3), h.precompile.RequiredGas(input))
	output, err = repair459Run(t, h, input, false, nil)
	require.NoError(t, err)
	repair459Intent(t, h, output, main, 2)
	require.Equal(t, uint64(2), h.keeper.GetIntentCount(h.ctx))
	require.Equal(t, uint64(2), h.keeper.GetOwnerNonce(h.ctx, h.caller))
	logs := h.logs("OrderPlaced(bytes32,bytes32,address,uint8,uint256,uint256,uint8,uint64)")
	require.Len(t, logs, 2)
	for i, log := range logs {
		require.Equal(t, common.Hash(marketID), log.Topics[2])
		decoded, err := h.precompile.GetABI().Events[layerxexchange.OrderPlacedEvent].Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		require.Equal(t, []interface{}{types.SideBuy, big.NewInt(30000), big.NewInt(2), types.TimeInForceGoodTillCancelled, uint64(i + 1)}, decoded)
	}
	view := h.view(layerxexchange.GetIntentMethod, h.intentID(types.IntentKind_INTENT_KIND_PLACE, 1))
	record := abi.ConvertType(view[0], new(layerxexchange.Intent)).(*layerxexchange.Intent)
	require.Equal(t, owned, record.Account)
	before := h.keeper.ExportGenesis(h.ctx)
	var storeKey sdk.StoreKey
	for _, key := range h.ctx.MultiStore().StoreKeys() {
		if key.Name() == types.StoreKey {
			storeKey = key
		}
	}
	require.NotNil(t, storeKey)
	h.keeper = exchangekeeper.NewKeeper(testkeeper.EVMTestApp.AppCodec(), storeKey, h.custody, &testkeeper.EVMTestApp.EvmKeeper)
	h.keeper.SetUpgradeKeeper(h.upgrade)
	h.precompile, err = layerxexchange.NewPrecompile(keepers{Keepers: testkeeper.EVMTestApp.GetPrecompileKeepers(), exchange: h.keeper})
	require.NoError(t, err)
	h.at(h.ctx)
	require.Equal(t, before, h.keeper.ExportGenesis(h.ctx))
	view = h.view(layerxexchange.GetIntentMethod, h.intentID(types.IntentKind_INTENT_KIND_PLACE, 1))
	record = abi.ConvertType(view[0], new(layerxexchange.Intent)).(*layerxexchange.Intent)
	require.Equal(t, owned, record.Account)
}

func TestRepair459AccountRefusals(t *testing.T) {
	h, _, owned, foreign := repair459Harness(t)
	named := repair459Method(t, h, layerxexchange.PlaceOrderAccountSignature)
	before := h.keeper.ExportGenesis(h.ctx)
	for _, account := range [][32]byte{foreign, {0x99}, {}} {
		input := repair459Input(t, named, marketID, account, types.SideBuy, big.NewInt(30000), big.NewInt(2), types.TimeInForceGoodTillCancelled)
		_, err := repair459Run(t, h, input, false, nil)
		require.ErrorIs(t, err, types.ErrInvalidIntent)
		_, err = h.keeper.RecordPlaceOrderIntentForAccount(h.ctx, h.caller, marketID, account,
			types.SideBuy, sdk.NewInt(30000), sdk.NewInt(2), types.TimeInForceGoodTillCancelled)
		require.ErrorIs(t, err, types.ErrInvalidIntent)
		require.Equal(t, before, h.keeper.ExportGenesis(h.ctx))
		require.Empty(t, h.stateDB.GetAllLogs())
	}
	input := repair459Input(t, named, marketID, owned, types.SideBuy, big.NewInt(30000), big.NewInt(2), types.TimeInForceGoodTillCancelled)
	_, err := repair459Run(t, h, input, true, nil)
	require.ErrorContains(t, err, "staticcall")
	_, err = repair459Run(t, h, input, false, big.NewInt(1))
	require.Error(t, err)
	_, err = repair459Run(t, h, input[:len(input)-32], false, nil)
	require.Error(t, err)
	require.Equal(t, before, h.keeper.ExportGenesis(h.ctx))
	require.Empty(t, h.stateDB.GetAllLogs())
	_, _, err = testkeeper.EVMTestApp.EvmKeeper.UnbindLayerX(h.ctx, h.caller)
	require.NoError(t, err)
	_, err = repair459Run(t, h, input, false, nil)
	require.ErrorIs(t, err, types.ErrInvalidIntent)
	require.Equal(t, before, h.keeper.ExportGenesis(h.ctx))
}

func TestRepair459TypeScriptCalldata(t *testing.T) {
	h, _, owned, _ := repair459Harness(t)
	var fixture struct {
		Signature string `json:"signature"`
		AccountID string `json:"accountId"`
		Data      string `json:"data"`
		Producer  string `json:"producer"`
	}
	encoded, err := os.ReadFile("testdata/repair_459_typescript.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(encoded, &fixture))
	require.Equal(t, "services/agent/sdk/typescript/src/exchange.ts#exchangePlaceOrderCall", fixture.Producer)
	require.Equal(t, layerxexchange.PlaceOrderAccountSignature, fixture.Signature)
	require.Equal(t, common.Hash(owned).Hex(), fixture.AccountID)
	input := common.FromHex(fixture.Data)
	exchangeABI := h.precompile.GetABI()
	method, err := exchangeABI.MethodById(input[:4])
	require.NoError(t, err)
	require.Equal(t, layerxexchange.PlaceOrderAccountSignature, method.Sig)
	args, err := method.Inputs.Unpack(input[4:])
	require.NoError(t, err)
	require.Equal(t, []interface{}{marketID, owned, types.SideBuy, big.NewInt(30000), big.NewInt(2), types.TimeInForceGoodTillCancelled}, args)
	snapshot := h.stateDB.Snapshot()
	output, err := repair459Run(t, h, input, false, nil)
	require.NoError(t, err)
	fromSDK := repair459Intent(t, h, output, owned, 1)
	h.stateDB.RevertToSnapshot(snapshot)
	h.ctx = h.stateDB.Ctx()
	output, err = repair459Run(t, h, repair459Input(t, method,
		marketID, owned, types.SideBuy, big.NewInt(30000), big.NewInt(2), types.TimeInForceGoodTillCancelled), false, nil)
	require.NoError(t, err)
	require.Equal(t, fromSDK, repair459Intent(t, h, output, owned, 1))
}

func TestRepair459LegacyUnboundOrder(t *testing.T) {
	h := newHarness(t)
	output, err := h.call(layerxexchange.PlaceOrderMethod, marketID, types.SideBuy,
		big.NewInt(30000), big.NewInt(2), types.TimeInForceGoodTillCancelled)
	require.NoError(t, err)
	intent, found := h.keeper.GetIntent(h.ctx, output[0].([32]byte))
	require.True(t, found)
	require.Empty(t, intent.Account)
	require.Equal(t, custodytypes.Address(h.caller), intent.Owner)
	require.Equal(t, uint64(1), intent.Nonce)
}
