package layerxledger_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
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
	_, err = h.keeper.RegisterAsset(h.ctx, actor, types.RegisterArgs{
		AssetID: asset, Salt: salt, Symbol: "TST", Name: "Transfer test", Decimals: 6,
		SupplyCap: sdk.NewInt(1000), IssuerKind: types.AssetIssuerNative,
	}, [32]byte{0x32})
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
	require.Len(t, logs, 1)
	require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), logs[0].Address)
	require.Equal(t, []common.Hash{layerxledger.GetABI().Events["AccountOpened"].ID, common.Hash(want)}, logs[0].Topics)
	require.Empty(t, logs[0].Data)

	t.Run("unknown asset reverts", func(t *testing.T) {
		data, err := runTransferMethod(t, h, "openAccount", [32]byte{0xff})
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, "LXP_ASSET_MISMATCH")
		require.Len(t, h.stateDB.GetAllLogs(), 1)
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

func transferMainAccount(t *testing.T, h *harness, actor ledgerkeeper.Actor, asset [32]byte) [32]byte {
	t.Helper()
	name, id, err := lxphost.MainAccountID(actor.DIDKey)
	require.NoError(t, err)
	row := binary.BigEndian.AppendUint32(nil, 32)
	row = append(row, id[:]...)
	row = binary.BigEndian.AppendUint32(row, uint32(len(name)))
	row = append(row, name...)
	row = append(row, 1)
	row = append(row, make([]byte, 16)...)
	row = binary.BigEndian.AppendUint32(row, 32)
	row = append(row, asset[:]...)
	row = append(row, 1)
	row = binary.BigEndian.AppendUint64(row, 0)
	row = binary.BigEndian.AppendUint64(row, 0)
	row = append(row, 0, 0)
	row = binary.BigEndian.AppendUint32(row, 32)
	row = append(row, actor.DIDKey[:]...)
	row = append(row, 1)
	h.ctx.KVStore(h.hostKey).Set(append([]byte{0x06}, id[:]...), row)
	account, err := h.host.ReadAccount(h.ctx, name, id)
	require.NoError(t, err)
	require.Equal(t, id, account.ID)
	require.Equal(t, actor.DIDKey, account.AuthorityKey)
	require.Equal(t, sdk.ZeroInt(), account.Balance)
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
	_, err := h.keeper.OpenAccount(h.ctx, actor, asset, [32]byte{0x41})
	require.NoError(t, err)
	_, err = h.keeper.OpenAccount(h.ctx, recipient, asset, [32]byte{0x42})
	require.NoError(t, err)
	holding, err := ledgerkeeper.AccountID(actor.DIDKey, asset)
	require.NoError(t, err)
	to, err := ledgerkeeper.AccountID(recipient.DIDKey, asset)
	require.NoError(t, err)
	from := transferMainAccount(t, h, actor, asset)
	_, err = h.keeper.Mint(h.ctx, actor, asset, holding, sdk.NewInt(250), [32]byte{0x43})
	require.NoError(t, err)
	key := [32]byte{0x44}
	payload := transferEnvelope(t, h, actor, holding, from, asset, 250, 0, key)
	ctx, err := ledgerkeeper.WithSequence(h.ctx)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(ctx, actor.DID)
	require.NoError(t, err)
	now := uint64(ctx.BlockTime().UnixMilli())
	receipt, err := h.host.Execute(ctx, lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00010005, ActorDID: actor.DID, AuthorityKey: actor.DIDKey,
		AccountSequence: sequence, NotBefore: now, NotAfter: now, IdempotencyKey: key, Payload: payload,
	}, actor.Authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	require.NoError(t, h.keeper.Mirror(h.ctx, 0x00010005, receipt))
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
	payload = transferEnvelope(t, h, actor, from, to, asset, 40, 0, [32]byte{0x45})
	data, err := runTransferMethod(t, h, "send", payload)
	require.NoError(t, err)
	require.Empty(t, data)
	assertBalance(from, 210)
	assertBalance(to, 40)
	logs := h.stateDB.GetAllLogs()
	require.Len(t, logs, 1)
	event := layerxledger.GetABI().Events["Transfer"]
	require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), logs[0].Address)
	require.Equal(t, []common.Hash{event.ID, common.Hash(from), common.Hash(to), common.Hash(asset)}, logs[0].Topics)
	fields, err := event.Inputs.NonIndexed().Unpack(logs[0].Data)
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
			require.Len(t, h.stateDB.GetAllLogs(), 1)
			assertBalance(from, 210)
			assertBalance(to, 40)
		})
	}
}
