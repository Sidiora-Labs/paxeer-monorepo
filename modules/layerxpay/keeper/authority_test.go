package keeper

import (
	"encoding/hex"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestAuthorityForBoundCaller(t *testing.T) {
	h := newPayHarness(t)
	payBefore := payStoreSnapshot(h.ctx, h.storeKey)
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	for _, actor := range h.actors {
		t.Run(actor.address.Hex(), func(t *testing.T) {
			key, bound := h.evm.GetLayerXDid(h.ctx, actor.address)
			require.True(t, bound)
			require.Equal(t, actor.public, key)
			authority, err := h.keeper.AuthorityFor(h.ctx, actor.address)
			require.NoError(t, err)
			require.Equal(t, []byte("did:layerx:"+hex.EncodeToString(key[:])), authority.ActorDID)
			require.Equal(t, actor.did, authority.ActorDID)
			require.Equal(t, key, authority.AuthorityKey)
			id, err := lxphost.DIDID(authority.ActorDID)
			require.NoError(t, err)
			require.Equal(t, actor.id, id)
			require.Equal(t, lxphost.KernelAuthority{Actor: id, Principal: id, VerifiedKey: key}, authority.Kernel)
		})
	}
	require.Equal(t, payBefore, payStoreSnapshot(h.ctx, h.storeKey))
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
}

func TestAuthorityForUnboundCallerRefused(t *testing.T) {
	h := newPayHarness(t)
	actor := h.actors[0]
	_, err := h.keeper.AuthorityFor(h.ctx, actor.address)
	require.NoError(t, err)
	key, _, err := h.evm.UnbindLayerX(h.ctx, actor.address)
	require.NoError(t, err)
	require.Equal(t, actor.public, key)
	payBefore := payStoreSnapshot(h.ctx, h.storeKey)
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	for _, caller := range []common.Address{
		{}, common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"), actor.address,
	} {
		t.Run(caller.Hex(), func(t *testing.T) {
			_, bound := h.evm.GetLayerXDid(h.ctx, caller)
			require.False(t, bound)
			authority, err := h.keeper.AuthorityFor(h.ctx, caller)
			require.ErrorIs(t, err, types.ErrUnboundCaller)
			require.ErrorContains(t, err, caller.Hex())
			require.Equal(t, CallerAuthority{}, authority)
			require.Equal(t, "LXP_PAY_UNBOUND_CALLER", types.PayRevertCode(err))
		})
	}
	require.Equal(t, payBefore, payStoreSnapshot(h.ctx, h.storeKey))
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
}
