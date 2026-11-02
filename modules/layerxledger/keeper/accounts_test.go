package keeper_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func accountActor(t *testing.T, h *harness, seed byte) ledgerkeeper.Actor {
	t.Helper()
	var seedBytes [ed25519.SeedSize]byte
	seedBytes[0] = seed
	private := ed25519.NewKeyFromSeed(seedBytes[:])
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	caller := common.Address{0x24, 0x03, seed}
	evm := &testkeeper.EVMTestApp.EvmKeeper
	message, err := evmtypes.LayerXBindMessage(evm.ChainID(h.ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = evm.BindLayerX(h.ctx, caller, key, signature)
	require.NoError(t, err)
	actor, err := h.keeper.Actor(h.ctx, caller)
	require.NoError(t, err)
	return actor
}

func accountActivity(t *testing.T, h *harness, actor ledgerkeeper.Actor, activityType uint32, payload []byte, key [32]byte) lxphost.Receipt {
	t.Helper()
	global, err := lxphost.NextSequence(h.ctx, h.hostKey)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.ctx, actor.DID)
	require.NoError(t, err)
	now := uint64(h.ctx.BlockTime().UnixMilli())
	receipt, err := h.host.Execute(lxphost.WithSequence(h.ctx, global), lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.DID,
		AuthorityKey: actor.Authority.VerifiedKey, AccountSequence: sequence,
		NotBefore: now, NotAfter: now, IdempotencyKey: key, Payload: payload,
	}, actor.Authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	return receipt
}

func accountAsset(t *testing.T, h *harness, issuer ledgerkeeper.Actor, saltByte byte) [32]byte {
	t.Helper()
	salt := [32]byte{saltByte}
	material := append([]byte("LX:ASSET:v1"), issuer.Authority.Actor[:]...)
	assetID := sha256.Sum256(append(material, salt[:]...))
	payload, err := types.EncodeRegisterPayload(types.RegisterArgs{
		AssetID: assetID, Salt: salt, Symbol: "TKN", Name: "Token", Decimals: 6,
		SupplyCap: sdk.NewInt(1000), IssuerKind: types.AssetIssuerNative,
	})
	require.NoError(t, err)
	accountActivity(t, h, issuer, 0x00010001, payload, [32]byte{0x11, saltByte})
	return assetID
}

func TestOpenAccount(t *testing.T) {
	h := newHarness(t)
	h.ctx = h.ctx.WithChainID("hyperpax_125-1")
	issuer := accountActor(t, h, 71)
	holder := accountActor(t, h, 72)
	assets := [][32]byte{accountAsset(t, h, issuer, 1), accountAsset(t, h, issuer, 2)}
	ids := map[[32]byte]bool{}
	for i, assetID := range assets {
		for j, actor := range []ledgerkeeper.Actor{issuer, holder} {
			nextAssetID := accountAsset(t, h, issuer, byte(4+2*i+j))
			nextName, nextID, err := lxphost.AssetAccountID(actor.DIDKey, nextAssetID)
			require.NoError(t, err)
			derivedNextID, err := ledgerkeeper.AccountID(actor.DIDKey, nextAssetID)
			require.NoError(t, err)
			require.Equal(t, nextID, derivedNextID)
			_, err = h.host.ReadAccount(h.ctx, nextName, nextID)
			require.Error(t, err)
			name, wantID, err := lxphost.AssetAccountID(actor.DIDKey, assetID)
			require.NoError(t, err)
			id, err := ledgerkeeper.AccountID(actor.DIDKey, assetID)
			require.NoError(t, err)
			require.Equal(t, wantID, id)
			require.False(t, ids[id], "each DID and asset pair must have a distinct account")
			ids[id] = true
			_, err = h.host.ReadAccount(h.ctx, name, id)
			require.Error(t, err)
			key := [32]byte{0x21, byte(i), byte(j)}
			next, err := lxphost.NextSequence(h.ctx, h.hostKey)
			require.NoError(t, err)
			sequence, err := h.host.ReadIdentitySequence(h.ctx, actor.DID)
			require.NoError(t, err)
			result, err := h.keeper.OpenAccount(h.ctx, actor, assetID, key)
			require.NoError(t, err)
			require.Equal(t, lxphost.Code(0), result.Receipt.ResultCode)
			require.Equal(t, next, result.Receipt.GlobalSequence)
			require.NotEmpty(t, result.Receipt.Encoded)
			require.NotEmpty(t, result.Events)
			require.Equal(t, result.Receipt.Events, result.Events)
			events, err := ledgerkeeper.LedgerEvents(0x00010004, result.Receipt, ledgerkeeper.EventArgs{})
			require.NoError(t, err)
			require.Equal(t, []ledgerkeeper.Event{{Name: "AccountOpened", Fields: map[string]any{"accountId": id}}}, events)
			account, err := h.host.ReadAccount(h.ctx, name, id)
			require.NoError(t, err)
			require.Equal(t, id, account.ID)
			require.Equal(t, assetID, account.AssetID)
			require.True(t, account.HasAsset)
			require.Equal(t, sdk.ZeroInt(), account.Balance)
			owner, err := h.host.ReadAccountOwner(h.ctx, id)
			require.NoError(t, err)
			didID, err := lxphost.DIDID(actor.DID)
			require.NoError(t, err)
			require.Equal(t, didID, owner)
			afterSequence, err := h.host.ReadIdentitySequence(h.ctx, actor.DID)
			require.NoError(t, err)
			require.Equal(t, sequence+1, afterSequence)
			before := ledgerViewStore(t, h.ctx, h.hostKey)
			reopened, err := h.keeper.OpenAccount(h.ctx, actor, nextAssetID, key)
			require.NoError(t, err)
			require.Equal(t, lxphost.Code(0), reopened.Receipt.ResultCode)
			require.Equal(t, next+1, reopened.Receipt.GlobalSequence)
			require.Equal(t, result.Receipt.ResultingStateRoot, reopened.Receipt.PreviousStateRoot)
			require.NotEqual(t, result.Receipt.ResultingStateRoot, reopened.Receipt.ResultingStateRoot)
			require.NotEqual(t, result.Receipt.Encoded, reopened.Receipt.Encoded)
			require.Equal(t, reopened.Receipt.Events, reopened.Events)
			reopenedEvents, err := ledgerkeeper.LedgerEvents(0x00010004, reopened.Receipt, ledgerkeeper.EventArgs{})
			require.NoError(t, err)
			require.Equal(t, []ledgerkeeper.Event{{Name: "AccountOpened", Fields: map[string]any{"accountId": nextID}}}, reopenedEvents)
			afterSequence, err = h.host.ReadIdentitySequence(h.ctx, actor.DID)
			require.NoError(t, err)
			require.Equal(t, sequence+2, afterSequence)
			require.NotEqual(t, before, ledgerViewStore(t, h.ctx, h.hostKey))
			after, err := h.host.ReadAccount(h.ctx, name, id)
			require.NoError(t, err)
			require.Equal(t, account, after)
			nextAccount, err := h.host.ReadAccount(h.ctx, nextName, nextID)
			require.NoError(t, err)
			require.Equal(t, nextID, nextAccount.ID)
			require.Equal(t, nextAssetID, nextAccount.AssetID)
			require.True(t, nextAccount.HasAsset)
			require.Equal(t, sdk.ZeroInt(), nextAccount.Balance)
			nextOwner, err := h.host.ReadAccountOwner(h.ctx, nextID)
			require.NoError(t, err)
			require.Equal(t, didID, nextOwner)
		}
	}

	require.NoError(t, h.keeper.SetParams(h.ctx, types.Params{Paused: true}))
	before := ledgerViewStore(t, h.ctx, h.hostKey)
	_, err := h.keeper.OpenAccount(h.ctx, ledgerkeeper.Actor{}, assets[0], [32]byte{0x22})
	require.ErrorIs(t, err, ledgerkeeper.ErrPausedScope)
	require.Contains(t, err.Error(), "layerxledger.openAccount")
	require.Equal(t, before, ledgerViewStore(t, h.ctx, h.hostKey))
}

func TestOpenAccountPausedAsset(t *testing.T) {
	h := newHarness(t)
	h.ctx = h.ctx.WithChainID("hyperpax_125-1")
	issuer := accountActor(t, h, 73)
	holder := accountActor(t, h, 74)
	assetID := accountAsset(t, h, issuer, 3)
	accountActivity(t, h, issuer, 0x00010002, types.EncodePausePayload(assetID), [32]byte{0x31})
	asset, err := h.host.ReadAsset(h.ctx, assetID)
	require.NoError(t, err)
	require.True(t, asset.Paused)
	require.False(t, h.keeper.GetParams(h.ctx).Paused)
	result, err := h.keeper.OpenAccount(h.ctx, holder, assetID, [32]byte{0x32})
	require.ErrorIs(t, err, types.ErrAssetPaused)
	require.Equal(t, lxphost.Code(-403), result.Receipt.ResultCode)
	name, id, err := lxphost.AssetAccountID(holder.DIDKey, assetID)
	require.NoError(t, err)
	_, err = h.host.ReadAccount(h.ctx, name, id)
	require.Error(t, err, "a refused open must not create the account")
	accountActivity(t, h, issuer, 0x00010003, types.EncodePausePayload(assetID), [32]byte{0x33})
	_, err = h.keeper.OpenAccount(h.ctx, holder, assetID, [32]byte{0x34})
	require.NoError(t, err)
	owner, err := h.host.ReadAccountOwner(h.ctx, id)
	require.NoError(t, err)
	require.Equal(t, holder.Authority.Actor, owner)
}
