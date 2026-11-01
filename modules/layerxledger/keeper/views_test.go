package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

type ledgerViewActor struct {
	private ed25519.PrivateKey
	key     [32]byte
	did     []byte
	id      [32]byte
}

func newLedgerViewActor(t *testing.T, seed byte) ledgerViewActor {
	t.Helper()
	a := ledgerViewActor{private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))}
	copy(a.key[:], a.private.Public().(ed25519.PublicKey))
	a.did = []byte("did:layerx:" + hex.EncodeToString(a.key[:]))
	var err error
	a.id, err = lxphost.DIDID(a.did)
	require.NoError(t, err)
	return a
}

type ledgerViewFixture struct {
	*harness
	actor     ledgerViewActor
	recipient ledgerViewActor
	asset     [32]byte
	account   [32]byte
	to        [32]byte
	global    uint64
}

func (f *ledgerViewFixture) execute(t *testing.T, ctx sdk.Context, actor ledgerViewActor, activityType uint32, sequence uint64, payload []byte, idempotency [32]byte) {
	t.Helper()
	f.global++
	if idempotency == [32]byte{} {
		idempotency = sha256.Sum256(binary.BigEndian.AppendUint64(nil, f.global))
	}
	now := uint64(ctx.BlockTime().UnixMilli())
	activity := lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did,
		AuthorityKey: actor.key, AccountSequence: sequence,
		NotBefore: now - 1000, NotAfter: now + 1000,
		IdempotencyKey: idempotency, Payload: payload,
	}
	hash, err := f.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(actor.private, hash[:])
	receipt, err := f.host.Execute(lxphost.WithSequence(ctx, f.global), activity,
		lxphost.KernelAuthority{Actor: actor.id, Principal: actor.id, VerifiedKey: actor.key})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
}

func newLedgerViewFixture(t *testing.T) *ledgerViewFixture {
	t.Helper()
	f := &ledgerViewFixture{harness: newHarness(t), actor: newLedgerViewActor(t, 81), recipient: newLedgerViewActor(t, 82)}
	salt := bytes.Repeat([]byte{0x31}, 32)
	material := append(append([]byte("LX:ASSET:v1"), f.actor.id[:]...), salt...)
	f.asset = sha256.Sum256(material)
	_, account, err := lxphost.AssetAccountID(f.actor.key, f.asset)
	require.NoError(t, err)
	f.account = account
	_, f.to, err = lxphost.AssetAccountID(f.recipient.key, f.asset)
	require.NoError(t, err)
	registration := append([]byte{0, 1}, f.asset[:]...)
	registration = append(registration, salt...)
	registration = append(registration, 3, 'T', 'K', 'N', 5, 'T', 'o', 'k', 'e', 'n', 6)
	registration = binary.BigEndian.AppendUint64(registration, 0)
	registration = binary.BigEndian.AppendUint64(registration, 1000)
	registration = append(registration, 2, 7)
	registration = append(registration, []byte("reserve")...)
	f.execute(t, f.ctx, f.actor, 0x00010001, 0, registration, [32]byte{})
	open := append([]byte{0, 1}, f.asset[:]...)
	f.execute(t, f.ctx, f.actor, 0x00010004, 1, open, [32]byte{})
	f.execute(t, f.ctx, f.recipient, 0x00010004, 0, open, [32]byte{})
	mint := append(append([]byte{0, 1}, f.asset[:]...), f.account[:]...)
	mint = binary.BigEndian.AppendUint64(mint, 0)
	mint = binary.BigEndian.AppendUint64(mint, 250)
	f.execute(t, f.ctx, f.actor, 0x0001000a, 2, mint, [32]byte{})
	return f
}

func ledgerViewStore(t *testing.T, ctx sdk.Context, key sdk.StoreKey) map[string][]byte {
	t.Helper()
	rows := map[string][]byte{}
	iterator := ctx.KVStore(key).Iterator(nil, nil)
	for ; iterator.Valid(); iterator.Next() {
		rows[string(iterator.Key())] = append([]byte(nil), iterator.Value()...)
	}
	require.NoError(t, iterator.Close())
	return rows
}

func (f *ledgerViewFixture) payerGrant(t *testing.T) ([]byte, [32]byte) {
	t.Helper()
	body := append([]byte(nil), f.account[:]...)
	body = append(body, f.to[:]...)
	body = append(body, f.asset[:]...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 40)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 100)
	body = append(body, 0)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, uint64(f.ctx.BlockTime().UnixMilli())+1000)
	body = append(body, bytes.Repeat([]byte{0x22}, 32)...)
	body = append(body, 0)
	body = append(body, make([]byte, 32)...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = append(body, f.actor.key[:]...)
	id, err := codec.DomainHash(codec.DomainAuthorityHash, append([]byte("LXP:GRANT:v1"), body...))
	require.NoError(t, err)
	grant := append(append([]byte(nil), id[:]...), body...)
	return append(grant, ed25519.Sign(f.actor.private, id[:])...), id
}

func TestLedgerViews(t *testing.T) {
	f := newLedgerViewFixture(t)
	grant, grantID := f.payerGrant(t)
	f.execute(t, f.ctx, f.actor, 0x00010007, 3, grant, [32]byte{})
	for _, paused := range []bool{false, true} {
		require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: paused}))
		if paused {
			f.execute(t, f.ctx, f.actor, 0x00010002, 4, append([]byte{0, 1}, f.asset[:]...), [32]byte{})
		}
		before := ledgerViewStore(t, f.ctx, f.hostKey)
		ledgerBefore := ledgerViewStore(t, f.ctx, f.storeKey)

		wantAsset, err := f.host.ReadAsset(f.ctx, f.asset)
		require.NoError(t, err)
		asset, err := f.keeper.GetAsset(f.ctx, f.asset)
		require.NoError(t, err)
		require.Equal(t, wantAsset, asset)
		require.Equal(t, "TKN", asset.Symbol)
		require.Equal(t, "Token", asset.Name)
		require.Equal(t, sdk.NewInt(1000), asset.SupplyCap)
		require.Equal(t, paused, asset.Paused)

		wantBalance, err := f.host.ReadBalance(f.ctx, f.account, f.asset)
		require.NoError(t, err)
		balance, err := f.keeper.BalanceOf(f.ctx, f.account, f.asset)
		require.NoError(t, err)
		require.Equal(t, wantBalance, balance)
		require.Equal(t, sdk.NewInt(250), balance)
		balance, err = f.keeper.BalanceOf(f.ctx, f.to, f.asset)
		require.NoError(t, err)
		require.Equal(t, sdk.ZeroInt(), balance)

		wantTotal, err := f.host.ReadTotalUnits(f.ctx, f.asset)
		require.NoError(t, err)
		total, err := f.keeper.TotalUnits(f.ctx, f.asset)
		require.NoError(t, err)
		require.Equal(t, wantTotal, total)
		require.Equal(t, sdk.NewInt(250), total)

		wantOwner, err := f.host.ReadAccountOwner(f.ctx, f.account)
		require.NoError(t, err)
		owner, err := f.keeper.AccountOwner(f.ctx, f.account)
		require.NoError(t, err)
		require.Equal(t, wantOwner, owner)
		require.Equal(t, f.actor.id, owner)

		wantGrant, err := f.host.ReadPayerGrant(f.ctx, grantID)
		require.NoError(t, err)
		state, err := f.keeper.GetPayerGrant(f.ctx, grantID)
		require.NoError(t, err)
		require.Equal(t, wantGrant, state)
		require.Equal(t, lxphost.PayerGrantState{Grant: grant, DrawnTotal: sdk.ZeroInt(), DrawnThisPeriod: sdk.ZeroInt()}, state)
		state.Grant[0] ^= 0xff
		state, err = f.keeper.GetPayerGrant(f.ctx, grantID)
		require.NoError(t, err)
		require.Equal(t, wantGrant, state)

		missingAsset, err := f.keeper.GetAsset(f.ctx, [32]byte{})
		require.ErrorIs(t, err, types.ErrAssetMismatch)
		require.Equal(t, lxphost.AssetRecord{}, missingAsset)
		balance, err = f.keeper.BalanceOf(f.ctx, f.account, [32]byte{})
		require.ErrorIs(t, err, types.ErrAssetMismatch)
		require.Equal(t, sdk.ZeroInt(), balance)
		balance, err = f.keeper.BalanceOf(f.ctx, [32]byte{}, f.asset)
		require.Equal(t, lxphost.Code(-208), err)
		require.Equal(t, sdk.ZeroInt(), balance)
		total, err = f.keeper.TotalUnits(f.ctx, [32]byte{})
		require.ErrorIs(t, err, types.ErrAssetMismatch)
		require.Equal(t, sdk.ZeroInt(), total)
		owner, err = f.keeper.AccountOwner(f.ctx, [32]byte{})
		require.Equal(t, lxphost.Code(-208), err)
		require.Equal(t, [32]byte{}, owner)
		state, err = f.keeper.GetPayerGrant(f.ctx, [32]byte{})
		require.ErrorIs(t, err, types.ErrNoPayerGrant)
		require.Equal(t, lxphost.PayerGrantState{}, state)
		require.Equal(t, before, ledgerViewStore(t, f.ctx, f.hostKey))
		require.Equal(t, ledgerBefore, ledgerViewStore(t, f.ctx, f.storeKey))
	}

	f.host.Close()
	_, err := f.keeper.GetAsset(f.ctx, f.asset)
	require.ErrorIs(t, err, types.ErrNonCanonical)
	_, err = f.keeper.BalanceOf(f.ctx, f.account, f.asset)
	require.ErrorIs(t, err, types.ErrNonCanonical)
	_, err = f.keeper.TotalUnits(f.ctx, f.asset)
	require.ErrorIs(t, err, types.ErrNonCanonical)
	_, err = f.keeper.AccountOwner(f.ctx, f.account)
	require.ErrorIs(t, err, types.ErrNonCanonical)
	_, err = f.keeper.GetPayerGrant(f.ctx, grantID)
	require.ErrorIs(t, err, types.ErrNonCanonical)
}

func (f *ledgerViewFixture) send(t *testing.T, ctx sdk.Context, sequence uint64) {
	t.Helper()
	idempotency := sha256.Sum256(binary.BigEndian.AppendUint64([]byte("ledger-view-send"), sequence))
	material := append(append([]byte(nil), f.account[:]...), f.to[:]...)
	material = append(material, f.asset[:]...)
	material = binary.BigEndian.AppendUint64(material, 0)
	material = binary.BigEndian.AppendUint64(material, 10)
	context, err := codec.DomainHash(codec.DomainContextHash, append(append([]byte(nil), material...), idempotency[:]...))
	require.NoError(t, err)
	common := binary.BigEndian.AppendUint64(material, sequence)
	common = append(common, idempotency[:]...)
	common = binary.BigEndian.AppendUint64(common, uint64(ctx.BlockTime().UnixMilli())+1000)
	common = append(common, context[:]...)
	common = append(common, 0)
	tail := append(append([]byte{1}, f.account[:]...), context[:]...)
	tail = binary.BigEndian.AppendUint32(tail, 125)
	tail = binary.BigEndian.AppendUint16(tail, 3)
	message := append(append([]byte{0x53, 1}, common...), tail...)
	hash, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	payload := append([]byte{0x53, 1, 0, 10}, common...)
	payload = append(payload, 1)
	payload = append(payload, f.account[:]...)
	payload = append(payload, f.actor.key[:]...)
	payload = append(payload, ed25519.Sign(f.actor.private, hash[:])...)
	payload = append(payload, context[:]...)
	payload = binary.BigEndian.AppendUint32(payload, 125)
	payload = binary.BigEndian.AppendUint16(payload, 3)
	f.execute(t, ctx, f.actor, 0x00010005, sequence+3, payload, idempotency)
}

func TestNextSequence(t *testing.T) {
	f := newLedgerViewFixture(t)
	assertSequence := func(ctx sdk.Context, want uint64) {
		t.Helper()
		before := ledgerViewStore(t, ctx, f.hostKey)
		ledgerBefore := ledgerViewStore(t, ctx, f.storeKey)
		name, accountID, err := lxphost.AssetAccountID(f.actor.key, f.asset)
		require.NoError(t, err)
		account, err := f.host.ReadAccount(ctx, name, accountID)
		require.NoError(t, err)
		next, err := f.keeper.NextSequence(ctx, f.actor.key, f.asset)
		require.NoError(t, err)
		require.Equal(t, want, next)
		require.Equal(t, account.NextSequence, next)
		require.Equal(t, before, ledgerViewStore(t, ctx, f.hostKey))
		require.Equal(t, ledgerBefore, ledgerViewStore(t, ctx, f.storeKey))
	}
	assertSequence(f.ctx, 0)
	f.send(t, f.ctx, 0)
	assertSequence(f.ctx, 1)
	f.send(t, f.ctx, 1)
	assertSequence(f.ctx, 2)
	branch, write := f.ctx.CacheContext()
	f.send(t, branch, 2)
	assertSequence(branch, 3)
	assertSequence(f.ctx, 2)
	write()
	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true}))
	assertSequence(f.ctx, 3)
	next, err := f.keeper.NextSequence(f.ctx, f.recipient.key, f.asset)
	require.NoError(t, err)
	require.Zero(t, next)
	before := ledgerViewStore(t, f.ctx, f.hostKey)
	for _, pair := range [][2][32]byte{{f.actor.key, [32]byte{}}, {[32]byte{1}, f.asset}} {
		next, err = f.keeper.NextSequence(f.ctx, pair[0], pair[1])
		require.Equal(t, lxphost.Code(-208), err)
		require.Zero(t, next)
	}
	f.host.Close()
	next, err = f.keeper.NextSequence(f.ctx, f.actor.key, f.asset)
	require.ErrorIs(t, err, types.ErrNonCanonical)
	require.Zero(t, next)
	require.Equal(t, before, ledgerViewStore(t, f.ctx, f.hostKey))
}
