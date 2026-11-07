package layerxledger_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
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

func newTransferHarness(t *testing.T) (*harness, ledgerkeeper.Actor, [32]byte) {
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
	actor, err := h.keeper.Actor(h.ctx, h.caller)
	require.NoError(t, err)
	salt := [32]byte{0x31}
	material := append([]byte("LX:ASSET:v1"), actor.Authority.Actor[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	_, err = runTransferMethod(t, h, "register", asset, salt, "TST", "Transfer test", uint8(6),
		big.NewInt(1000), uint8(types.AssetIssuerNative), []byte{})
	require.NoError(t, err)
	return h, actor, asset
}

func runTransferMethod(t *testing.T, h *harness, name string, args ...interface{}) ([]byte, error) {
	t.Helper()
	input, err := layerxledger.GetABI().Pack(name, args...)
	require.NoError(t, err)
	return h.precompile.Run(h.evm, h.caller, common.Address{}, input, nil, false, false, nil)
}

func TestPrecompileOpenAccount(t *testing.T) {
	h, actor, asset := newTransferHarness(t)
	want, err := ledgerkeeper.AccountID(actor.DIDKey, asset)
	require.NoError(t, err)
	name, id, err := lxphost.AssetAccountID(actor.DIDKey, asset)
	require.NoError(t, err)
	require.Equal(t, want, id)
	_, err = h.host.ReadAccount(h.ctx, name, want)
	require.Error(t, err)
	data, err := runTransferMethod(t, h, "openAccount", asset)
	require.NoError(t, err)
	require.Equal(t, want[:], data)
	account, err := h.host.ReadAccount(h.ctx, name, want)
	require.NoError(t, err)
	require.Equal(t, want, account.ID)
	require.Equal(t, asset, account.AssetID)
	require.Equal(t, sdk.ZeroInt(), account.Balance)
	owner, err := h.keeper.AccountOwner(h.ctx, want)
	require.NoError(t, err)
	require.Equal(t, actor.Authority.Actor, owner)
	logs := h.stateDB.GetAllLogs()
	require.Len(t, logs, 2)
	require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), logs[1].Address)
	require.Equal(t, []common.Hash{layerxledger.GetABI().Events["AccountOpened"].ID, common.Hash(want)}, logs[1].Topics)
	require.Empty(t, logs[1].Data)

	t.Run("unknown asset reverts", func(t *testing.T) {
		data, err := runTransferMethod(t, h, "openAccount", [32]byte{0xff})
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, "LXP_ASSET_MISMATCH")
		require.Len(t, h.stateDB.GetAllLogs(), 2)
	})
}

func transferRecipient(t *testing.T, h *harness) ledgerkeeper.Actor {
	t.Helper()
	var seed [ed25519.SeedSize]byte
	seed[0] = 2
	private := ed25519.NewKeyFromSeed(seed[:])
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	caller := common.Address{0x23, 2}
	evm := &testkeeper.EVMTestApp.EvmKeeper
	message, err := evmtypes.LayerXBindMessage(evm.ChainID(h.ctx), caller, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = evm.BindLayerX(h.ctx, caller, key, signature)
	require.NoError(t, err)
	actor, err := h.keeper.Actor(h.ctx, caller)
	require.NoError(t, err)
	return actor
}

func openTransferAccount(t *testing.T, h *harness, caller common.Address, actor ledgerkeeper.Actor, asset [32]byte) [32]byte {
	t.Helper()
	data, err := assetCall(t, h, caller, "openAccount", asset)
	require.NoError(t, err)
	values, err := h.precompile.GetABI().Methods["openAccount"].Outputs.Unpack(data)
	require.NoError(t, err)
	id := values[0].([32]byte)
	name, expected, err := lxphost.AssetAccountID(actor.DIDKey, asset)
	require.NoError(t, err)
	require.Equal(t, expected, id)
	account, err := h.host.ReadAccount(h.ctx, name, id)
	require.NoError(t, err)
	require.Equal(t, id, account.ID)
	require.Equal(t, asset, account.AssetID)
	require.Equal(t, actor.DIDKey, account.AuthorityKey)
	require.True(t, account.Balance.IsZero())
	return id
}

func transferEnvelope(t *testing.T, h *harness, actor ledgerkeeper.Actor, from, to, asset [32]byte,
	amount int64, sequence uint64, key [32]byte) []byte {
	t.Helper()
	material := append(append(append([]byte(nil), from[:]...), to[:]...), asset[:]...)
	var encoded [16]byte
	big.NewInt(amount).FillBytes(encoded[:])
	material = append(material, encoded[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(material, key[:]...))
	require.NoError(t, err)
	payload, err := types.EncodeSendPayload(types.SendArgs{
		From: from, To: to, Asset: asset, Amount: sdk.NewInt(amount), Sequence: sequence,
		IdempotencyKey: key, ExpiresAt: uint64(h.ctx.BlockTime().UnixMilli()) + 1000,
		ContextHash: contextHash, PublicKey: actor.DIDKey, NetworkID: 125, ProtocolVersion: 3,
	})
	require.NoError(t, err)
	var seed [ed25519.SeedSize]byte
	seed[0] = 1
	private := ed25519.NewKeyFromSeed(seed[:])
	authorization := len(payload) - 167
	message := append(append([]byte(nil), payload[:2]...), payload[4:authorization]...)
	message = append(message, payload[authorization:authorization+33]...)
	message = append(message, payload[authorization+129:]...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	copy(payload[authorization+65:authorization+129], ed25519.Sign(private, digest[:]))
	require.NoError(t, types.ValidateSendEnvelope(payload))
	return payload
}

func TestPrecompileSend(t *testing.T) {
	h, actor, asset := newTransferHarness(t)
	recipient := transferRecipient(t, h)
	from := openTransferAccount(t, h, h.caller, actor, asset)
	to := openTransferAccount(t, h, common.Address{0x23, 2}, recipient, asset)
	_, err := runTransferMethod(t, h, "mint", asset, from, big.NewInt(250))
	require.NoError(t, err)
	beforeLogs := len(h.stateDB.GetAllLogs())
	assertBalance := func(account [32]byte, want int64) {
		t.Helper()
		balance, err := h.keeper.BalanceOf(h.ctx, account, asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(want), balance)
		bank := testkeeper.EVMTestApp.BankKeeper.GetBalance(h.ctx, lxphost.BankAddress(account), types.MirrorDenom(asset))
		require.Equal(t, balance, bank.Amount)
	}
	assertBalance(from, 250)
	assertBalance(to, 0)
	payload := transferEnvelope(t, h, actor, from, to, asset, 40, 0, [32]byte{0x45})
	data, err := runTransferMethod(t, h, "send", payload)
	require.NoError(t, err)
	require.Empty(t, data)
	assertBalance(from, 210)
	assertBalance(to, 40)
	logs := h.stateDB.GetAllLogs()
	require.Len(t, logs, beforeLogs+1)
	log := logs[beforeLogs]
	event := layerxledger.GetABI().Events["Transfer"]
	require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), log.Address)
	require.Equal(t, []common.Hash{event.ID, common.Hash(from), common.Hash(to), common.Hash(asset)}, log.Topics)
	fields, err := event.Inputs.NonIndexed().Unpack(log.Data)
	require.NoError(t, err)
	require.Equal(t, []interface{}{big.NewInt(40), uint8(5)}, fields)

	for _, tc := range []struct {
		name    string
		payload []byte
		reason  string
	}{
		{"foreign account", transferEnvelope(t, h, actor, to, from, asset, 5, 0, [32]byte{0x46}), "LXP_UNAUTHORIZED_DEBIT"},
		{"malformed envelope", []byte{0x53, 1}, "LXP_MALFORMED_SEND"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hostBefore := storeSnapshot(t, h.ctx.KVStore(h.hostKey))
			ledgerBefore := storeSnapshot(t, h.ctx.KVStore(h.storeKey))
			data, err := runTransferMethod(t, h, "send", tc.payload)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			assertLedgerRevert(t, data, tc.reason)
			require.Equal(t, hostBefore, storeSnapshot(t, h.ctx.KVStore(h.hostKey)))
			require.Equal(t, ledgerBefore, storeSnapshot(t, h.ctx.KVStore(h.storeKey)))
			require.Len(t, h.stateDB.GetAllLogs(), beforeLogs+1)
			assertBalance(from, 210)
			assertBalance(to, 40)
		})
	}
}

func TestPrecompileSendReopen(t *testing.T) {
	f := newRepair144Fixture(t)
	f.p.success(t, f.sender, "mint", f.asset, f.from, big.NewInt(250))
	payload := f.envelope(t, 0, 40, 0xa1)
	branch, _ := f.p.ctx.CacheContext()
	ctx, err := ledgerkeeper.WithSequence(branch)
	require.NoError(t, err)
	identitySequence, err := f.p.app.LayerXHost.ReadIdentitySequence(ctx, f.payer.DID)
	require.NoError(t, err)
	now := uint64(ctx.BlockTime().UnixMilli())
	want, err := f.p.app.LayerXHost.Execute(ctx, lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00010005, ActorDID: f.payer.DID,
		AuthorityKey: f.payer.DIDKey, AccountSequence: identitySequence,
		NotBefore: now, NotAfter: now, IdempotencyKey: [32]byte{0xa1}, Payload: payload,
	}, f.payer.Authority)
	require.NoError(t, err)
	require.Zero(t, want.ResultCode)
	require.NotEmpty(t, want.Encoded)
	f.balances(t, 250, 0)
	f.refused(t, f.recipient, "LXP_UNAUTHORIZED_DEBIT", "send", payload)
	_, logs := f.p.success(t, f.sender, "send", payload)
	got := ledgerParityReceipt(t, f.p.app.LayerXHost, len(want.Encoded))
	require.Equal(t, want.Encoded, got.CanonicalBytes())
	require.Len(t, logs, 1)
	proofLedgerLog(t, f.p.abi, logs[0], "Transfer", map[string]interface{}{
		"from": f.from, "to": f.to, "assetId": f.asset, "amount": big.NewInt(40), "kind": uint8(5),
	})
	f.sequence(t, f.from, 1)
	f.sequence(t, f.to, 0)
	f.balances(t, 210, 40)
	next, err := lxphost.NextSequence(f.p.ctx, f.p.app.GetKey(app.LayerXHostStoreKey))
	require.NoError(t, err)
	identity, err := f.p.app.LayerXHost.ReadIdentitySequence(f.p.ctx, f.payer.DID)
	require.NoError(t, err)
	f.reopen(t)
	f.sequence(t, f.from, 1)
	f.sequence(t, f.to, 0)
	f.balances(t, 210, 40)
	recovered, err := lxphost.NextSequence(f.p.ctx, f.p.app.GetKey(app.LayerXHostStoreKey))
	require.NoError(t, err)
	require.Equal(t, next, recovered)
	recoveredIdentity, err := f.p.app.LayerXHost.ReadIdentitySequence(f.p.ctx, f.payer.DID)
	require.NoError(t, err)
	require.Equal(t, identity, recoveredIdentity)
	f.refused(t, f.recipient, "LXP_UNAUTHORIZED_DEBIT", "send", f.envelope(t, 1, 10, 0xa2))
	f.refused(t, f.sender, "LXP_SEQUENCE_MISMATCH", "send", f.envelope(t, 0, 10, 0xa3))
	f.p.success(t, f.sender, "send", f.envelope(t, 1, 10, 0xa4))
	f.sequence(t, f.from, 2)
	f.balances(t, 200, 50)
}
