package layerxledger_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"math/big"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func newAssetHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	app := testkeeper.EVMTestApp
	auth := authkeeper.NewAccountKeeper(app.AppCodec(), app.GetKey(authtypes.StoreKey),
		app.GetSubspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{types.ModuleName: {authtypes.Minter}})
	bank := bankkeeper.NewBaseKeeper(app.AppCodec(), app.GetKey(banktypes.StoreKey), auth,
		app.GetSubspace(banktypes.ModuleName), nil)
	h.keeper = ledgerkeeper.NewKeeper(app.AppCodec(), h.storeKey, h.host, bank, &app.EvmKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String())
	h.keepers.ledger = h.keeper
	var err error
	h.precompile, err = layerxledger.NewPrecompile(h.keepers)
	require.NoError(t, err)
	h.bindCaller(t)
	return h
}

func assetCall(t *testing.T, h *harness, caller common.Address, name string, args ...interface{}) ([]byte, error) {
	t.Helper()
	input, err := h.precompile.GetABI().Pack(name, args...)
	require.NoError(t, err)
	method := h.precompile.GetABI().Methods[name]
	return h.precompile.Run(h.evm, caller, caller, input, nil, method.IsConstant(), false, nil)
}

func assetView(t *testing.T, h *harness, name string, args ...interface{}) []interface{} {
	t.Helper()
	data, err := assetCall(t, h, h.caller, name, args...)
	require.NoError(t, err)
	values, err := h.precompile.GetABI().Methods[name].Outputs.Unpack(data)
	require.NoError(t, err)
	return values
}

func registerTestAsset(t *testing.T, h *harness, custody bool) ([32]byte, [32]byte, ledgerkeeper.Actor) {
	t.Helper()
	actor, err := h.keeper.Actor(h.ctx, h.caller)
	require.NoError(t, err)
	salt := [32]byte{0x41}
	material := append(append([]byte("LX:ASSET:v1"), actor.Authority.Actor[:]...), salt[:]...)
	assetID := sha256.Sum256(material)
	kind, reference := uint8(types.AssetIssuerNative), []byte{}
	if custody {
		kind, reference = types.AssetIssuerPaxeerCustody, []byte("reserve")
	}
	data, err := assetCall(t, h, h.caller, "register", assetID, salt, "TKN", "Token", uint8(6), big.NewInt(1000), kind, reference)
	require.NoError(t, err)
	require.Equal(t, assetID[:], data)
	logs := h.stateDB.GetAllLogs()
	require.Len(t, logs, 1)
	require.Equal(t, h.precompile.Address(), logs[0].Address)
	require.Equal(t, []common.Hash{h.precompile.GetABI().Events[layerxledger.AssetRegistered].ID, common.Hash(assetID)}, logs[0].Topics)
	require.Empty(t, logs[0].Data)
	return assetID, salt, actor
}

func bindAssetHolder(t *testing.T, h *harness) (common.Address, ledgerkeeper.Actor) {
	t.Helper()
	caller := common.HexToAddress("0x2320000000000000000000000000000000000002")
	var seed [ed25519.SeedSize]byte
	seed[0] = 2
	private := ed25519.NewKeyFromSeed(seed[:])
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	app := testkeeper.EVMTestApp
	message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(h.ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = app.EvmKeeper.BindLayerX(h.ctx, caller, key, signature)
	require.NoError(t, err)
	actor, err := h.keeper.Actor(h.ctx, caller)
	require.NoError(t, err)
	return caller, actor
}

func TestPrecompileRegister(t *testing.T) {
	for _, custody := range []bool{false, true} {
		name := "native"
		if custody {
			name = "custody"
		}
		t.Run(name, func(t *testing.T) {
			h := newAssetHarness(t)
			assetID, salt, actor := registerTestAsset(t, h, custody)
			kind, custodyKind, reference := uint8(types.AssetIssuerNative), uint8(types.AssetIssuerNative), []byte{}
			if custody {
				kind, custodyKind, reference = types.AssetIssuerPaxeerCustody, types.AssetIssuerPaxeerCustody, []byte("reserve")
			}
			values := assetView(t, h, "getAsset", assetID)
			require.Len(t, values, 12)
			require.Equal(t, []interface{}{assetID, "TKN", "Token", uint8(6), big.NewInt(1000), actor.Authority.Actor,
				salt, kind, custodyKind, reference, false}, values[:11])
			require.Zero(t, values[11].(*big.Int).Cmp(big.NewInt(0)))
			before := len(h.stateDB.GetAllLogs())
			data, err := assetCall(t, h, h.caller, "register", assetID, salt, "TKN", "Token", uint8(6), big.NewInt(1000), kind, reference)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			assertLedgerRevert(t, data, "LXP_ASSET_ALREADY_REGISTERED")
			require.Len(t, h.stateDB.GetAllLogs(), before)
			require.Equal(t, values, assetView(t, h, "getAsset", assetID))
		})
	}
}

func TestPrecompilePauseUnpause(t *testing.T) {
	h := newAssetHarness(t)
	assetID, _, _ := registerTestAsset(t, h, false)
	other, _ := bindAssetHolder(t, h)
	for _, operation := range []struct {
		name, event string
		paused      bool
	}{
		{"pause", layerxledger.AssetPaused, true}, {"unpause", layerxledger.AssetUnpaused, false},
	} {
		t.Run(operation.name, func(t *testing.T) {
			before := len(h.stateDB.GetAllLogs())
			data, err := assetCall(t, h, other, operation.name, assetID)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			assertLedgerRevert(t, data, "LXP_UNAUTHORIZED_DEBIT")
			require.Len(t, h.stateDB.GetAllLogs(), before)
			require.Equal(t, !operation.paused, assetView(t, h, "getAsset", assetID)[10])
			data, err = assetCall(t, h, h.caller, operation.name, assetID)
			require.NoError(t, err)
			require.Empty(t, data)
			require.Equal(t, operation.paused, assetView(t, h, "getAsset", assetID)[10])
			logs := h.stateDB.GetAllLogs()
			require.Len(t, logs, before+1)
			require.Equal(t, h.precompile.Address(), logs[before].Address)
			require.Equal(t, []common.Hash{h.precompile.GetABI().Events[operation.event].ID, common.Hash(assetID)}, logs[before].Topics)
			require.Empty(t, logs[before].Data)
		})
	}
}

func TestPrecompileMintBurn(t *testing.T) {
	h := newAssetHarness(t)
	assetID, _, issuer := registerTestAsset(t, h, false)
	holderCaller, holder := bindAssetHolder(t, h)
	_, err := h.keeper.OpenAccount(h.ctx, issuer, assetID, [32]byte{1})
	require.NoError(t, err)
	_, err = h.keeper.OpenAccount(h.ctx, holder, assetID, [32]byte{2})
	require.NoError(t, err)
	account, err := ledgerkeeper.AccountID(holder.DIDKey, assetID)
	require.NoError(t, err)
	_, issuance, err := lxphost.IssuanceAccountID(assetID)
	require.NoError(t, err)
	assertUnits := func(want int64) {
		t.Helper()
		require.Zero(t, assetView(t, h, "balanceOf", account, assetID)[0].(*big.Int).Cmp(big.NewInt(want)))
		require.Zero(t, assetView(t, h, "totalUnits", assetID)[0].(*big.Int).Cmp(big.NewInt(want)))
		require.Zero(t, assetView(t, h, "getAsset", assetID)[11].(*big.Int).Cmp(big.NewInt(want)))
	}
	assertTransfer := func(index int, from, to [32]byte, amount int64, kind uint8) {
		t.Helper()
		logs := h.stateDB.GetAllLogs()
		require.Len(t, logs, index+1)
		log := logs[index]
		event := h.precompile.GetABI().Events[layerxledger.Transfer]
		require.Equal(t, h.precompile.Address(), log.Address)
		require.Equal(t, []common.Hash{event.ID, common.Hash(from), common.Hash(to), common.Hash(assetID)}, log.Topics)
		values, err := event.Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		require.Equal(t, []interface{}{big.NewInt(amount), kind}, values)
	}
	assertUnits(0)
	data, err := assetCall(t, h, holderCaller, "mint", assetID, account, big.NewInt(1))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLedgerRevert(t, data, "LXP_UNAUTHORIZED_DEBIT")
	require.Len(t, h.stateDB.GetAllLogs(), 1)
	assertUnits(0)
	data, err = assetCall(t, h, h.caller, "mint", assetID, account, big.NewInt(600))
	require.NoError(t, err)
	require.Empty(t, data)
	assertUnits(600)
	assertTransfer(1, issuance, account, 600, 10)
	data, err = assetCall(t, h, h.caller, "mint", assetID, account, big.NewInt(401))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLedgerRevert(t, data, "LXP_INSUFFICIENT_BALANCE")
	assertUnits(600)
	require.Len(t, h.stateDB.GetAllLogs(), 2)
	data, err = assetCall(t, h, h.caller, "burn", assetID, account, big.NewInt(1))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLedgerRevert(t, data, "LXP_UNAUTHORIZED_DEBIT")
	assertUnits(600)
	require.Len(t, h.stateDB.GetAllLogs(), 2)
	data, err = assetCall(t, h, holderCaller, "burn", assetID, account, big.NewInt(200))
	require.NoError(t, err)
	require.Empty(t, data)
	assertUnits(400)
	assertTransfer(2, account, issuance, 200, 11)
}
