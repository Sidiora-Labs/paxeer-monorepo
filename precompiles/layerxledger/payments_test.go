package layerxledger_test

import (
	"bytes"
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
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type paymentFixture struct {
	*harness
	bank                              bankkeeper.BaseKeeper
	payer, payee                      ledgerkeeper.Actor
	payeeCaller                       common.Address
	asset, from, to, purpose, context [32]byte
	grant                             []byte
	grantID                           [32]byte
}

func newPaymentFixture(t *testing.T) *paymentFixture {
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
	f := &paymentFixture{harness: h, bank: bank,
		payeeCaller: common.HexToAddress("0x2320000000000000000000000000000000000002")}
	f.payer, err = h.keeper.Actor(h.ctx, h.caller)
	require.NoError(t, err)
	var seed [ed25519.SeedSize]byte
	seed[0] = 2
	private := ed25519.NewKeyFromSeed(seed[:])
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(h.ctx), f.payeeCaller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = app.EvmKeeper.BindLayerX(h.ctx, f.payeeCaller, key, signature)
	require.NoError(t, err)
	f.payee, err = h.keeper.Actor(h.ctx, f.payeeCaller)
	require.NoError(t, err)
	salt := [32]byte{0x41}
	material := append([]byte("LX:ASSET:v1"), f.payer.Authority.Actor[:]...)
	f.asset = sha256.Sum256(append(material, salt[:]...))
	_, err = h.keeper.RegisterAsset(h.ctx, f.payer, types.RegisterArgs{
		AssetID: f.asset, Salt: salt, Symbol: "PAY", Name: "Payment token", Decimals: 6,
		SupplyCap: sdk.NewInt(1000), IssuerKind: 1,
	}, [32]byte{1})
	require.NoError(t, err)
	_, err = h.keeper.OpenAccount(h.ctx, f.payer, f.asset, [32]byte{2})
	require.NoError(t, err)
	_, err = h.keeper.OpenAccount(h.ctx, f.payee, f.asset, [32]byte{3})
	require.NoError(t, err)
	f.from, err = ledgerkeeper.AccountID(f.payer.DIDKey, f.asset)
	require.NoError(t, err)
	f.to, err = ledgerkeeper.AccountID(f.payee.DIDKey, f.asset)
	require.NoError(t, err)
	_, err = h.keeper.Mint(h.ctx, f.payer, f.asset, f.from, sdk.NewInt(250), [32]byte{4})
	require.NoError(t, err)
	f.purpose = [32]byte(bytes.Repeat([]byte{0x22}, 32))
	f.context, err = codec.DomainHash(codec.DomainContextHash, f.purpose[:])
	require.NoError(t, err)
	body := append([]byte(nil), f.from[:]...)
	body = append(body, f.to[:]...)
	body = append(body, f.asset[:]...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 40)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 100)
	body = append(body, 0)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, uint64(h.ctx.BlockTime().UnixMilli())+1000)
	body = append(body, f.purpose[:]...)
	body = append(body, 0)
	body = append(body, make([]byte, 32)...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = append(body, f.payer.DIDKey[:]...)
	f.grantID, err = codec.DomainHash(codec.DomainAuthorityHash, append([]byte("LXP:GRANT:v1"), body...))
	require.NoError(t, err)
	seed[0] = 1
	private = ed25519.NewKeyFromSeed(seed[:])
	f.grant = append(append([]byte(nil), f.grantID[:]...), body...)
	f.grant = append(f.grant, ed25519.Sign(private, f.grantID[:])...)
	require.Len(t, f.grant, 346)
	return f
}

func (f *paymentFixture) call(t *testing.T, caller common.Address, name string, args ...interface{}) ([]byte, error) {
	t.Helper()
	input, err := f.precompile.GetABI().Pack(name, args...)
	require.NoError(t, err)
	return f.precompile.Run(f.evm, caller, caller, input, nil, name == "getPayerGrant", false, nil)
}

func (f *paymentFixture) assertLog(t *testing.T, log *ethtypes.Log, name string, indexed [][32]byte, values ...interface{}) {
	t.Helper()
	event := f.precompile.GetABI().Events[name]
	wantTopics := []common.Hash{event.ID}
	for _, id := range indexed {
		wantTopics = append(wantTopics, common.Hash(id))
	}
	require.Equal(t, common.HexToAddress(layerxledger.LedgerAddress), log.Address)
	require.Equal(t, wantTopics, log.Topics)
	decoded, err := event.Inputs.NonIndexed().Unpack(log.Data)
	require.NoError(t, err)
	require.Len(t, decoded, len(values))
	for i, want := range values {
		require.Equal(t, want, decoded[i])
	}
}

func (f *paymentFixture) assertBalances(t *testing.T, from, to int64) {
	t.Helper()
	for _, balance := range []struct {
		account [32]byte
		amount  int64
	}{{f.from, from}, {f.to, to}} {
		actual, err := f.keeper.BalanceOf(f.ctx, balance.account, f.asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(balance.amount), actual)
		require.Equal(t, actual, f.bank.GetBalance(f.ctx, lxphost.BankAddress(balance.account), types.MirrorDenom(f.asset)).Amount)
	}
}

func TestPrecompileIssueRevokeGrant(t *testing.T) {
	f := newPaymentFixture(t)
	data, err := f.call(t, f.caller, "issueGrant", f.grant)
	require.NoError(t, err)
	require.Equal(t, f.grantID[:], data)
	logs := f.stateDB.GetAllLogs()
	require.Len(t, logs, 1)
	f.assertLog(t, logs[0], "PayerGrantIssued", [][32]byte{f.grantID})
	assertGrant := func(revoked bool, revokedAt uint64) {
		t.Helper()
		data, err := f.call(t, f.caller, "getPayerGrant", f.grantID)
		require.NoError(t, err)
		values, err := f.precompile.GetABI().Methods["getPayerGrant"].Outputs.Unpack(data)
		require.NoError(t, err)
		require.Len(t, values, 7)
		require.Equal(t, f.grant, values[0])
		require.Zero(t, values[1].(*big.Int).Sign())
		require.Zero(t, values[2].(*big.Int).Sign())
		require.Equal(t, []interface{}{uint64(0), revokedAt, revoked, false}, values[3:])
	}
	assertGrant(false, 0)
	sequence, err := f.host.ReadIdentitySequence(f.ctx, f.payer.DID)
	require.NoError(t, err)
	global, err := lxphost.NextSequence(f.ctx, f.hostKey)
	require.NoError(t, err)
	data, err = f.call(t, f.caller, "revokeGrant", f.grantID, sequence)
	require.NoError(t, err)
	require.Empty(t, data)
	logs = f.stateDB.GetAllLogs()
	require.Len(t, logs, 2)
	f.assertLog(t, logs[1], "PayerGrantRevoked", [][32]byte{f.grantID})
	assertGrant(true, global)
	f.assertBalances(t, 250, 0)
}

func TestPrecompileDraw(t *testing.T) {
	f := newPaymentFixture(t)
	_, err := f.call(t, f.caller, "issueGrant", f.grant)
	require.NoError(t, err)
	idempotency := [32]byte{0x51}
	data, err := f.call(t, f.payeeCaller, "draw", f.grantID, f.to, big.NewInt(15), idempotency, uint64(0), f.purpose, f.context)
	require.NoError(t, err)
	require.Empty(t, data)
	logs := f.stateDB.GetAllLogs()
	require.Len(t, logs, 3)
	f.assertLog(t, logs[1], "Transfer", [][32]byte{f.from, f.to, f.asset}, big.NewInt(15), uint8(6))
	f.assertLog(t, logs[2], "PaymentDrawn", [][32]byte{f.grantID, f.from, f.to}, f.asset, big.NewInt(15), idempotency, f.context)
	f.assertBalances(t, 235, 15)
	grant, err := f.keeper.GetPayerGrant(f.ctx, f.grantID)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(15), grant.DrawnTotal)
	sequence, err := f.keeper.NextSequence(f.ctx, f.payee.DIDKey, f.asset)
	require.NoError(t, err)
	require.Equal(t, uint64(1), sequence)
	before := storeSnapshot(t, f.ctx.KVStore(f.hostKey))
	data, err = f.call(t, f.payeeCaller, "draw", f.grantID, f.to, big.NewInt(41), [32]byte{0x52}, sequence, f.purpose, f.context)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	assertLedgerRevert(t, data, "LXP_GRANT_SCOPE_VIOLATION")
	require.Len(t, f.stateDB.GetAllLogs(), 3)
	require.Equal(t, before, storeSnapshot(t, f.ctx.KVStore(f.hostKey)))
	f.assertBalances(t, 235, 15)
	after, err := f.keeper.GetPayerGrant(f.ctx, f.grantID)
	require.NoError(t, err)
	require.Equal(t, grant, after)
	for _, invalid := range []struct {
		caller             common.Address
		recipient, purpose [32]byte
		reason             string
	}{
		{f.caller, f.to, f.purpose, "LXP_UNAUTHORIZED_DEBIT"},
		{f.payeeCaller, f.from, f.purpose, "LXP_UNAUTHORIZED_DEBIT"},
		{f.payeeCaller, f.to, [32]byte{}, "LXP_CONTEXT_MISMATCH"},
	} {
		data, err := f.call(t, invalid.caller, "draw", f.grantID, invalid.recipient, big.NewInt(1), [32]byte{0x53}, sequence, invalid.purpose, f.context)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, invalid.reason)
		require.Equal(t, before, storeSnapshot(t, f.ctx.KVStore(f.hostKey)))
		require.Len(t, f.stateDB.GetAllLogs(), 3)
	}
}

func TestPrecompilePayExact(t *testing.T) {
	f := newPaymentFixture(t)
	now := uint64(f.ctx.BlockTime().UnixMilli())
	reference := [32]byte{0x61}
	data, err := f.call(t, f.caller, "payExact", f.from, f.to, f.asset, big.NewInt(3), now+1000, reference)
	require.NoError(t, err)
	require.Empty(t, data)
	logs := f.stateDB.GetAllLogs()
	require.Len(t, logs, 2)
	f.assertLog(t, logs[0], "Transfer", [][32]byte{f.from, f.to, f.asset}, big.NewInt(3), uint8(5))
	f.assertLog(t, logs[1], "PaymentExact", [][32]byte{reference, f.from, f.to}, f.asset, big.NewInt(3))
	f.assertBalances(t, 247, 3)
	sequence, err := f.keeper.NextSequence(f.ctx, f.payer.DIDKey, f.asset)
	require.NoError(t, err)
	require.Equal(t, uint64(1), sequence)
	before := storeSnapshot(t, f.ctx.KVStore(f.hostKey))
	for _, expiresAt := range []uint64{now, now - 1, 0} {
		data, err = f.call(t, f.caller, "payExact", f.from, f.to, f.asset, big.NewInt(3), expiresAt, [32]byte{0x62})
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLedgerRevert(t, data, "LXP_EXPIRED")
		require.Equal(t, before, storeSnapshot(t, f.ctx.KVStore(f.hostKey)))
		require.Len(t, f.stateDB.GetAllLogs(), 2)
		f.assertBalances(t, 247, 3)
	}
}
