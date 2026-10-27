package keeper

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func newExecuteKeeper(t *testing.T) (*Keeper, sdk.Context, *sdk.KVStoreKey, lxphost.Config) {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	hostKey := sdk.NewKVStoreKey("lxphost")
	ledgerKey := sdk.NewKVStoreKey(types.StoreKey)
	cms.MountStoreWithDB(hostKey, sdk.StoreTypeIAVL, db)
	cms.MountStoreWithDB(ledgerKey, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 11, Time: time.UnixMilli(1_800_000_000_123).UTC()}, false)
	cfg := lxphost.Config{StoreKey: hostKey, NetworkID: 125, Epoch: 1,
		ParameterVersion: 1, TimestampWindowMs: 60_000, GasLimit: 10_000_000}
	host, err := lxphost.NewHost(cfg)
	require.NoError(t, err)
	t.Cleanup(host.Close)
	return &Keeper{storeKey: ledgerKey, host: host}, ctx, hostKey, cfg
}

func executeActor(t *testing.T, seed byte) Actor {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	did := []byte("did:layerx:" + hex.EncodeToString(key[:]))
	id, err := lxphost.DIDID(did)
	require.NoError(t, err)
	return Actor{DIDKey: key, DID: did, Authority: lxphost.KernelAuthority{
		Bound: true, Actor: id, Principal: id, VerifiedKey: key,
	}}
}

func executeAssetPayload(actor Actor, salt byte) ([]byte, [32]byte) {
	saltBytes := bytes.Repeat([]byte{salt}, 32)
	material := append([]byte("LX:ASSET:v1"), actor.Authority.Actor[:]...)
	asset := sha256.Sum256(append(material, saltBytes...))
	payload := append([]byte{0, 1}, asset[:]...)
	payload = append(payload, saltBytes...)
	payload = append(payload, 3, 'T', 'K', 'N', 5, 'T', 'o', 'k', 'e', 'n', 6)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 1000)
	payload = append(payload, 2, 7)
	return append(payload, []byte("reserve")...), asset
}

func executeStore(t *testing.T, ctx sdk.Context, key sdk.StoreKey) map[string][]byte {
	t.Helper()
	rows := map[string][]byte{}
	iterator := ctx.KVStore(key).Iterator(nil, nil)
	for ; iterator.Valid(); iterator.Next() {
		rows[string(iterator.Key())] = append([]byte(nil), iterator.Value()...)
	}
	require.NoError(t, iterator.Close())
	return rows
}

func TestExecuteAdvancesGlobalSequence(t *testing.T) {
	k, ctx, hostKey, _ := newExecuteKeeper(t)
	firstActor, secondActor := executeActor(t, 91), executeActor(t, 92)
	ctx = lxphost.WithSequence(ctx, 1)
	for i, actor := range []Actor{firstActor, secondActor, firstActor} {
		payload, asset := executeAssetPayload(actor, byte(i+1))
		next, err := lxphost.NextSequence(ctx, hostKey)
		require.NoError(t, err)
		require.Equal(t, uint64(i+1), next)
		result, err := k.execute(ctx, actor, 0x00010001, payload, [32]byte{byte(i + 1)})
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), result.Receipt.ResultCode)
		require.Equal(t, next, result.Receipt.GlobalSequence)
		require.Equal(t, uint64(ctx.BlockTime().UnixMilli()), result.Receipt.Timestamp)
		require.NotEmpty(t, result.Receipt.Encoded)
		require.Equal(t, result.Receipt.Events, result.Events)
		decoded, err := codec.DecodeReceipt(result.Receipt.Encoded)
		require.NoError(t, err)
		require.Equal(t, uint16(3), decoded.ProtocolVersion)
		record, err := k.host.ReadAsset(ctx, asset)
		require.NoError(t, err)
		require.Equal(t, actor.Authority.Actor, record.IssuerDID)
		sequence, err := k.host.ReadIdentitySequence(ctx, actor.DID)
		require.NoError(t, err)
		want := uint64(1)
		if i == 2 {
			want = 2
		}
		require.Equal(t, want, sequence)
	}
	require.Empty(t, executeStore(t, ctx, k.storeKey))
	payload, _ := executeAssetPayload(firstActor, 1)
	rejected, err := k.execute(ctx, firstActor, 0x00010001, payload, [32]byte{4})
	require.ErrorIs(t, err, types.ErrAssetAlreadyRegistered)
	require.Equal(t, lxphost.Code(-411), rejected.Receipt.ResultCode)
	require.Equal(t, uint64(4), rejected.Receipt.GlobalSequence)
	next, err := lxphost.NextSequence(ctx, hostKey)
	require.NoError(t, err)
	require.Equal(t, uint64(5), next)

	ctx.KVStore(hostKey).Set([]byte{2}, binary.BigEndian.AppendUint64(nil, math.MaxUint64))
	before := executeStore(t, ctx, hostKey)
	_, err = k.execute(ctx, firstActor, 0x00010001, payload, [32]byte{5})
	require.ErrorIs(t, err, types.ErrSequenceExhausted)
	require.Equal(t, before, executeStore(t, ctx, hostKey))
}

func requireExecuteReceipt(t *testing.T, receipt lxphost.Receipt) {
	t.Helper()
	require.NotEmpty(t, receipt.Encoded)
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(3), decoded.ProtocolVersion)
	require.Equal(t, receipt.Encoded, decoded.CanonicalBytes())
	require.Equal(t, receipt.GlobalSequence, decoded.GlobalSequence)
	require.Equal(t, int32(receipt.ResultCode), decoded.ResultCode)
	require.Equal(t, receipt.Timestamp, decoded.Timestamp)
	require.Equal(t, receipt.PreviousStateRoot, decoded.PreviousStateRoot)
	require.Equal(t, receipt.ResultingStateRoot, decoded.ResultingStateRoot)
}

func TestExecuteReusedKeyNextSequence(t *testing.T) {
	t.Run("open_account", func(t *testing.T) {
		k, ctx, hostKey, _ := newExecuteKeeper(t)
		issuer, actor := executeActor(t, 95), executeActor(t, 96)
		registration, asset := executeAssetPayload(issuer, 1)
		registered, err := k.execute(ctx, issuer, 0x00010001, registration, [32]byte{1})
		require.NoError(t, err)
		require.Equal(t, uint64(1), registered.Receipt.GlobalSequence)
		payload := append([]byte{0, 1}, asset[:]...)
		key := [32]byte{2}
		first, err := k.execute(ctx, actor, 0x00010004, payload, key)
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), first.Receipt.ResultCode)
		require.Equal(t, uint64(2), first.Receipt.GlobalSequence)
		requireExecuteReceipt(t, first.Receipt)
		_, accountID, err := lxphost.AssetAccountID(actor.DIDKey, asset)
		require.NoError(t, err)
		owner, err := k.host.ReadAccountOwner(ctx, accountID)
		require.NoError(t, err)
		require.Equal(t, actor.Authority.Actor, owner)
		sequence, err := k.host.ReadIdentitySequence(ctx, actor.DID)
		require.NoError(t, err)
		require.Equal(t, uint64(1), sequence)

		registration, nextAsset := executeAssetPayload(issuer, 2)
		registered, err = k.execute(ctx, issuer, 0x00010001, registration, [32]byte{1})
		require.NoError(t, err)
		require.Equal(t, uint64(3), registered.Receipt.GlobalSequence)
		before := executeStore(t, ctx, hostKey)
		payload = append([]byte{0, 1}, nextAsset[:]...)
		second, err := k.execute(ctx, actor, 0x00010004, payload, key)
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), second.Receipt.ResultCode)
		require.Equal(t, uint64(4), second.Receipt.GlobalSequence)
		requireExecuteReceipt(t, second.Receipt)
		require.NotEqual(t, first.Receipt.Encoded, second.Receipt.Encoded)
		require.Equal(t, second.Receipt.Events, second.Events)
		require.NotEqual(t, before, executeStore(t, ctx, hostKey))
		owner, err = k.host.ReadAccountOwner(ctx, accountID)
		require.NoError(t, err)
		require.Equal(t, actor.Authority.Actor, owner)
		_, nextAccountID, err := lxphost.AssetAccountID(actor.DIDKey, nextAsset)
		require.NoError(t, err)
		owner, err = k.host.ReadAccountOwner(ctx, nextAccountID)
		require.NoError(t, err)
		require.Equal(t, actor.Authority.Actor, owner)
		sequence, err = k.host.ReadIdentitySequence(ctx, actor.DID)
		require.NoError(t, err)
		require.Equal(t, uint64(2), sequence)
		next, err := lxphost.NextSequence(ctx, hostKey)
		require.NoError(t, err)
		require.Equal(t, uint64(5), next)
		require.Empty(t, executeStore(t, ctx, k.storeKey))
	})

	k, ctx, hostKey, cfg := newExecuteKeeper(t)
	actor := executeActor(t, 93)
	payload, _ := executeAssetPayload(actor, 1)
	key := [32]byte{0x31}
	first, err := k.execute(ctx, actor, 0x00010001, payload, key)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), first.Receipt.ResultCode)
	require.Equal(t, uint64(1), first.Receipt.GlobalSequence)
	requireExecuteReceipt(t, first.Receipt)
	sequence, err := k.host.ReadIdentitySequence(ctx, actor.DID)
	require.NoError(t, err)
	require.Equal(t, uint64(1), sequence)
	previous := first.Receipt
	for i, reopen := range []bool{false, true} {
		if reopen {
			host, err := lxphost.NewHost(cfg)
			require.NoError(t, err)
			t.Cleanup(host.Close)
			k.host = host
			ctx = ctx.WithBlockHeight(12).WithBlockTime(ctx.BlockTime().Add(time.Second))
		}
		before := executeStore(t, ctx, hostKey)
		payload, asset := executeAssetPayload(actor, byte(i+2))
		result, err := k.execute(ctx, actor, 0x00010001, payload, key)
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), result.Receipt.ResultCode)
		require.Equal(t, uint64(i+2), result.Receipt.GlobalSequence)
		requireExecuteReceipt(t, result.Receipt)
		require.Equal(t, uint64(ctx.BlockTime().UnixMilli()), result.Receipt.Timestamp)
		require.NotEqual(t, previous.Encoded, result.Receipt.Encoded)
		require.Equal(t, result.Receipt.Events, result.Events)
		require.NotEqual(t, before, executeStore(t, ctx, hostKey))
		record, err := k.host.ReadAsset(ctx, asset)
		require.NoError(t, err)
		require.Equal(t, actor.Authority.Actor, record.IssuerDID)
		sequence, err := k.host.ReadIdentitySequence(ctx, actor.DID)
		require.NoError(t, err)
		require.Equal(t, uint64(i+2), sequence)
		next, err := lxphost.NextSequence(ctx, hostKey)
		require.NoError(t, err)
		require.Equal(t, uint64(i+3), next)
		previous = result.Receipt
	}
	other := executeActor(t, 94)
	otherPayload, _ := executeAssetPayload(other, 1)
	result, err := k.execute(ctx, other, 0x00010001, otherPayload, key)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), result.Receipt.ResultCode)
	require.Equal(t, uint64(4), result.Receipt.GlobalSequence)
	requireExecuteReceipt(t, result.Receipt)
	require.NotEqual(t, first.Receipt.Encoded, result.Receipt.Encoded)
	require.Empty(t, executeStore(t, ctx, k.storeKey))
}
