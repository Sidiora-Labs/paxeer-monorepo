package keeper

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	proofcodec "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func newLedgerPauseKeeper(t *testing.T) (*Keeper, sdk.Context, sdk.StoreKey) {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(types.StoreKey)
	hostKey := sdk.NewKVStoreKey("lxphost")
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(hostKey, sdk.StoreTypeIAVL, db)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmtypes.Header{Height: 11, Time: time.Unix(1_800_000_000, 0).UTC()}, false)
	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: hostKey, NetworkID: 125, Epoch: 1,
		ParameterVersion: 1, TimestampWindowMs: 60_000, GasLimit: 10_000_000,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	cdc := codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
	return NewKeeper(cdc, key, host, nil, nil, govtypes.GovernanceAuthority()), ctx, hostKey
}

func ledgerPauseStore(t *testing.T, ctx sdk.Context, key sdk.StoreKey) map[string][]byte {
	t.Helper()
	rows := map[string][]byte{}
	it := ctx.KVStore(key).Iterator(nil, nil)
	for ; it.Valid(); it.Next() {
		rows[string(it.Key())] = append([]byte(nil), it.Value()...)
	}
	require.NoError(t, it.Close())
	return rows
}

func TestLedgerPauseRefusesWrites(t *testing.T) {
	k, ctx, hostKey := newLedgerPauseKeeper(t)
	methods := []string{"register", "pause", "unpause", "openAccount", "send", "draw", "payExact", "issueGrant", "mint", "burn"}
	require.Same(t, govtypes.ErrPausedScope, ErrPausedScope)
	for _, method := range methods {
		require.NoError(t, k.checkPause(ctx, method), method)
	}
	require.False(t, ctx.KVStore(k.storeKey).Has(types.ParamsKey))
	require.NoError(t, k.SetParams(ctx, types.Params{Paused: true}))
	before := ledgerPauseStore(t, ctx, k.storeKey)
	hostBefore := ledgerPauseStore(t, ctx, hostKey)
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			err := k.checkPause(ctx, method)
			require.ErrorIs(t, err, ErrPausedScope)
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, govtypes.ModuleName, codespace)
			require.Equal(t, uint32(735), code)
			require.Contains(t, err.Error(), "layerxledger."+method)
		})
	}
	require.Equal(t, before, ledgerPauseStore(t, ctx, k.storeKey))
	require.Equal(t, hostBefore, ledgerPauseStore(t, ctx, hostKey))
	branch, write := ctx.CacheContext()
	require.NoError(t, k.SetParams(branch, types.Params{Paused: false}))
	for _, method := range methods {
		require.NoError(t, k.checkPause(branch, method), method)
		require.ErrorIs(t, k.checkPause(ctx, method), ErrPausedScope, method)
	}
	write()
	for _, method := range methods {
		require.NoError(t, k.checkPause(ctx, method), method)
	}
	for _, paused := range []bool{true, false} {
		require.NoError(t, k.SetParams(ctx, types.Params{Paused: paused}))
		require.ErrorIs(t, k.checkPause(ctx, ""), sdkerrors.ErrInvalidRequest)
	}
}

func TestLedgerPauseKeepsRevokeAndViews(t *testing.T) {
	k, ctx, hostKey := newLedgerPauseKeeper(t)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{81}, ed25519.SeedSize))
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	did := []byte("did:layerx:" + hex.EncodeToString(key[:]))
	actor, err := lxphost.DIDID(did)
	require.NoError(t, err)
	var global uint64
	executeAs := func(activityType uint32, sequence uint64, payload []byte, signer ed25519.PrivateKey) {
		t.Helper()
		var key [32]byte
		copy(key[:], signer.Public().(ed25519.PublicKey))
		did := []byte("did:layerx:" + hex.EncodeToString(key[:]))
		actor, err := lxphost.DIDID(did)
		require.NoError(t, err)
		global++
		now := uint64(ctx.BlockTime().UnixMilli())
		activity := lxphost.Activity{
			ProtocolVersion: 3, Type: activityType, ActorDID: did, AuthorityKey: key,
			AccountSequence: sequence, NotBefore: now - 1000, NotAfter: now + 1000,
			IdempotencyKey: sha256.Sum256(binary.BigEndian.AppendUint64(nil, global)), Payload: payload,
		}
		hash, err := k.host.SigningHash(activity)
		require.NoError(t, err)
		activity.Signature = ed25519.Sign(signer, hash[:])
		receipt, err := k.host.Execute(lxphost.WithSequence(ctx, global), activity,
			lxphost.KernelAuthority{Actor: actor, Principal: actor, VerifiedKey: key})
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	}
	execute := func(activityType uint32, sequence uint64, payload []byte) {
		t.Helper()
		executeAs(activityType, sequence, payload, private)
	}
	salt := [32]byte{0x31}
	assetID := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor[:]...), salt[:]...))
	_, accountID, err := lxphost.AssetAccountID(key, assetID)
	require.NoError(t, err)
	registration, err := types.EncodeRegisterPayload(types.RegisterArgs{
		AssetID: assetID, Salt: salt, Symbol: "TKN", Name: "Token", Decimals: 6,
		SupplyCap: sdk.NewInt(1000), IssuerKind: types.AssetIssuerNative,
	})
	require.NoError(t, err)
	execute(0x00010001, 0, registration)
	execute(0x00010004, 1, types.EncodeOpenAccountPayload(assetID))
	mint, err := types.EncodeSupplyPayload(assetID, accountID, sdk.NewInt(250))
	require.NoError(t, err)
	execute(0x0001000a, 2, mint)
	recipientPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{82}, ed25519.SeedSize))
	var recipientKey [32]byte
	copy(recipientKey[:], recipientPrivate.Public().(ed25519.PublicKey))
	_, recipientID, err := lxphost.AssetAccountID(recipientKey, assetID)
	require.NoError(t, err)
	executeAs(0x00010004, 0, types.EncodeOpenAccountPayload(assetID), recipientPrivate)
	for sequence := uint64(0); sequence < 5; sequence++ {
		for _, transfer := range []struct {
			from, to      [32]byte
			key           [32]byte
			private       ed25519.PrivateKey
			actorSequence uint64
		}{
			{accountID, recipientID, key, private, sequence + 3},
			{recipientID, accountID, recipientKey, recipientPrivate, sequence + 1},
		} {
			idempotency := sha256.Sum256(binary.BigEndian.AppendUint64(nil, global+1))
			material := append(append([]byte(nil), transfer.from[:]...), transfer.to[:]...)
			material = append(material, assetID[:]...)
			material = binary.BigEndian.AppendUint64(material, 0)
			material = binary.BigEndian.AppendUint64(material, 10)
			contextHash, err := proofcodec.DomainHash(proofcodec.DomainContextHash,
				append(append([]byte(nil), material...), idempotency[:]...))
			require.NoError(t, err)
			common := binary.BigEndian.AppendUint64(material, sequence)
			common = append(common, idempotency[:]...)
			common = binary.BigEndian.AppendUint64(common, uint64(ctx.BlockTime().UnixMilli())+1000)
			common = append(common, contextHash[:]...)
			common = append(common, 0)
			tail := append(append([]byte{1}, transfer.from[:]...), contextHash[:]...)
			tail = binary.BigEndian.AppendUint32(tail, 125)
			tail = binary.BigEndian.AppendUint16(tail, 3)
			message := append(append([]byte{0x53, 1}, common...), tail...)
			hash, err := proofcodec.DomainHash(proofcodec.DomainSignaturePreimage, message)
			require.NoError(t, err)
			payload := append([]byte{0x53, 1, 0, 10}, common...)
			payload = append(payload, 1)
			payload = append(payload, transfer.from[:]...)
			payload = append(payload, transfer.key[:]...)
			payload = append(payload, ed25519.Sign(transfer.private, hash[:])...)
			payload = append(payload, contextHash[:]...)
			payload = binary.BigEndian.AppendUint32(payload, 125)
			payload = binary.BigEndian.AppendUint16(payload, 3)
			executeAs(0x00010005, transfer.actorSequence, payload, transfer.private)
		}
	}
	body := append(append([]byte(nil), accountID[:]...), recipientID[:]...)
	body = append(body, assetID[:]...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 40)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, 100)
	body = append(body, 0)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = binary.BigEndian.AppendUint64(body, uint64(ctx.BlockTime().UnixMilli())+1000)
	body = append(body, bytes.Repeat([]byte{0x22}, 32)...)
	body = append(body, 0)
	body = append(body, make([]byte, 32)...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = append(body, key[:]...)
	grantID, err := proofcodec.DomainHash(proofcodec.DomainAuthorityHash, append([]byte("LXP:GRANT:v1"), body...))
	require.NoError(t, err)
	grant := append(append([]byte(nil), grantID[:]...), body...)
	grant = append(grant, ed25519.Sign(private, grantID[:])...)
	require.NoError(t, types.ValidateIssueGrantPayload(grant))
	execute(0x00010007, 8, grant)

	require.NoError(t, k.SetParams(ctx, types.Params{Paused: true}))
	require.NoError(t, k.checkPause(ctx, "revokeGrant"))
	require.ErrorIs(t, k.checkPause(ctx, "RevokeGrant"), ErrPausedScope)
	require.ErrorIs(t, k.checkPause(ctx, "issueGrant"), ErrPausedScope)
	execute(0x00010008, 9, types.EncodeRevokeGrantPayload(grantID, 9))
	before := ledgerPauseStore(t, ctx, k.storeKey)
	hostBefore := ledgerPauseStore(t, ctx, hostKey)
	asset, err := k.GetAsset(ctx, assetID)
	require.NoError(t, err)
	require.Equal(t, assetID, asset.AssetID)
	require.Equal(t, "TKN", asset.Symbol)
	require.Equal(t, sdk.NewInt(250), asset.TotalUnits)
	balance, err := k.BalanceOf(ctx, accountID, assetID)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(250), balance)
	recipientBalance, err := k.BalanceOf(ctx, recipientID, assetID)
	require.NoError(t, err)
	require.Equal(t, sdk.ZeroInt(), recipientBalance)
	total, err := k.TotalUnits(ctx, assetID)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(250), total)
	owner, err := k.AccountOwner(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, actor, owner)
	state, err := k.GetPayerGrant(ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, grant, state.Grant)
	require.True(t, state.Revoked)
	require.Equal(t, uint64(16), state.RevokedAt)
	next, err := k.NextSequence(ctx, key, assetID)
	require.NoError(t, err)
	require.Equal(t, uint64(5), next)
	require.True(t, k.GetParams(ctx).Paused)
	require.Equal(t, before, ledgerPauseStore(t, ctx, k.storeKey))
	require.Equal(t, hostBefore, ledgerPauseStore(t, ctx, hostKey))
}
