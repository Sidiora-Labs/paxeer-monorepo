package keeper

import (
	"crypto/ed25519"
	"fmt"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestAuthActorUnbound(t *testing.T) {
	h := newHarness(t)
	app := testkeeper.EVMTestApp
	for _, caller := range []common.Address{{}, common.HexToAddress("0x2620000000000000000000000000000000000001")} {
		_, bound := app.EvmKeeper.GetLayerXDid(h.ctx, caller)
		require.False(t, bound)
		actor, err := h.keeper.Actor(h.ctx, caller)
		require.ErrorIs(t, err, types.ErrAuthScope)
		require.Equal(t, Actor{}, actor)
	}
}

func TestAuthActorBound(t *testing.T) {
	h := newHarness(t)
	ctx := h.ctx.WithChainID("hyperpax_125-1")
	app := testkeeper.EVMTestApp
	var previousID [32]byte

	for i := byte(1); i <= 2; i++ {
		var seed [ed25519.SeedSize]byte
		seed[0] = i
		private := ed25519.NewKeyFromSeed(seed[:])
		var key [ed25519.PublicKeySize]byte
		copy(key[:], private.Public().(ed25519.PublicKey))
		caller := common.Address{0x26, 0x20, i}
		message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(ctx), caller, 0)
		require.NoError(t, err)
		var signature [ed25519.SignatureSize]byte
		copy(signature[:], ed25519.Sign(private, message))
		nonce, err := app.EvmKeeper.BindLayerX(ctx, caller, key, signature)
		require.NoError(t, err)
		require.Zero(t, nonce)
		boundKey, bound := app.EvmKeeper.GetLayerXDid(ctx, caller)
		require.True(t, bound)
		require.Equal(t, key, boundKey)

		actor, err := h.keeper.Actor(ctx, caller)
		require.NoError(t, err)
		did := []byte(fmt.Sprintf("did:layerx:%x", key))
		id, err := lxphost.DIDID(did)
		require.NoError(t, err)
		require.NotEqual(t, [32]byte{}, id)
		require.NotEqual(t, previousID, id)
		previousID = id
		require.Equal(t, key, actor.DIDKey)
		require.Equal(t, did, actor.DID)
		require.Equal(t, lxphost.KernelAuthority{
			Bound: true, Actor: id, Principal: id, VerifiedKey: key,
		}, actor.Authority)

		_, _, err = app.EvmKeeper.UnbindLayerX(ctx, caller)
		require.NoError(t, err)
		actor, err = h.keeper.Actor(ctx, caller)
		require.ErrorIs(t, err, types.ErrAuthScope)
		require.Equal(t, Actor{}, actor)
	}
}
