package keeper_test

import (
	"crypto/ed25519"
	"fmt"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestActorUnboundCaller(t *testing.T) {
	h := newHarness(t)
	app := testkeeper.EVMTestApp
	k := ledgerkeeper.NewKeeper(app.AppCodec(), h.storeKey, nil, app.BankKeeper, &app.EvmKeeper, h.authority)
	hostStore := h.ctx.KVStore(h.hostKey)
	require.Nil(t, hostStore.Get([]byte("actor-sentinel")))
	hostStore.Set([]byte("actor-sentinel"), []byte("unchanged"))

	for _, caller := range []common.Address{{}, common.HexToAddress("0x2320000000000000000000000000000000000001")} {
		_, bound := app.EvmKeeper.GetLayerXDid(h.ctx, caller)
		require.False(t, bound)
		actor, err := k.Actor(h.ctx, caller)
		require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
		require.Equal(t, ledgerkeeper.Actor{}, actor)
	}

	iter := hostStore.Iterator(nil, nil)
	defer iter.Close()
	require.True(t, iter.Valid())
	require.Equal(t, []byte("actor-sentinel"), iter.Key())
	require.Equal(t, []byte("unchanged"), iter.Value())
	iter.Next()
	require.False(t, iter.Valid(), "rejecting an unbound caller must leave the host store unchanged")
}

func TestActorBoundCaller(t *testing.T) {
	h := newHarness(t)
	ctx := h.ctx.WithChainID("hyperpax_125-1")
	app := testkeeper.EVMTestApp
	k := ledgerkeeper.NewKeeper(app.AppCodec(), h.storeKey, nil, app.BankKeeper, &app.EvmKeeper, h.authority)

	for i := byte(1); i <= 2; i++ {
		var seed [ed25519.SeedSize]byte
		seed[0] = i
		private := ed25519.NewKeyFromSeed(seed[:])
		var key [32]byte
		copy(key[:], private.Public().(ed25519.PublicKey))
		caller := common.Address{0x23, 0x20, i}
		message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(ctx), caller, 0)
		require.NoError(t, err)
		var signature [ed25519.SignatureSize]byte
		copy(signature[:], ed25519.Sign(private, message))
		_, err = app.EvmKeeper.BindLayerX(ctx, caller, key, signature)
		require.NoError(t, err)
		boundKey, bound := app.EvmKeeper.GetLayerXDid(ctx, caller)
		require.True(t, bound)
		require.Equal(t, key, boundKey)

		actor, err := k.Actor(ctx, caller)
		require.NoError(t, err)
		did := []byte(fmt.Sprintf("did:layerx:%x", key))
		id, err := lxphost.DIDID(did)
		require.NoError(t, err)
		require.NotEqual(t, [32]byte{}, id)
		require.Equal(t, key, actor.DIDKey)
		require.Equal(t, did, actor.DID)
		require.Equal(t, lxphost.KernelAuthority{
			Bound: true, Actor: id, Principal: id, VerifiedKey: key,
		}, actor.Authority)

		_, _, err = app.EvmKeeper.UnbindLayerX(ctx, caller)
		require.NoError(t, err)
		actor, err = k.Actor(ctx, caller)
		require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
		require.Equal(t, ledgerkeeper.Actor{}, actor)
	}
}
