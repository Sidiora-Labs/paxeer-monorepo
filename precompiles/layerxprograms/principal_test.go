package layerxprograms_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func bindProgramsPrincipal(t *testing.T, ctx sdk.Context, k *evmkeeper.Keeper, caller common.Address, seed byte) [32]byte {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	var didKey [32]byte
	copy(didKey[:], private.Public().(ed25519.PublicKey))
	nonce := k.GetLayerXBindNonce(ctx, caller)
	message, err := evmtypes.LayerXBindMessage(k.ChainID(ctx), caller, nonce)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, message))
	consumed, err := k.BindLayerX(ctx, caller, didKey, signature)
	require.NoError(t, err)
	require.Equal(t, nonce, consumed)
	return didKey
}

func TestProgramsPrincipalFromBoundDID(t *testing.T) {
	ctx := testkeeper.EVMTestApp.GetContextForDeliverTx(nil)
	ctx = ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore())
	k := &testkeeper.EVMTestApp.EvmKeeper
	origin := common.HexToAddress("0x13252301")
	contract := common.HexToAddress("0x13252302")
	originKey := bindProgramsPrincipal(t, ctx, k, origin, 0x55)
	contractKey := bindProgramsPrincipal(t, ctx, k, contract, 0x56)
	var principals [][32]byte
	for _, tc := range []struct {
		caller common.Address
		key    [32]byte
	}{
		{origin, originKey},
		{contract, contractKey},
	} {
		authority, actorDID, err := lp.PrincipalOf(ctx, k, tc.caller)
		require.NoError(t, err)
		_, accountID, err := lxphost.MainAccountID(tc.key)
		require.NoError(t, err)
		require.Equal(t, tc.key, authority.VerifiedKey)
		require.NotEqual(t, accountID, authority.Principal)
		require.NotEqual(t, tc.key, authority.Principal)
		require.True(t, authority.Bound)
		wantDID := []byte("did:layerx:" + hex.EncodeToString(tc.key[:]))
		require.Equal(t, wantDID, actorDID)
		wantActorID, err := lxphost.DIDID(wantDID)
		require.NoError(t, err)
		require.Equal(t, wantActorID, authority.Actor)
		require.Equal(t, wantActorID, authority.Principal)
		actorID, err := lxphost.DIDID(actorDID)
		require.NoError(t, err)
		require.Equal(t, wantActorID, actorID)
		principals = append(principals, authority.Principal)
	}
	require.NotEqual(t, principals[0], principals[1])
}

func TestProgramsPrincipalAuthorityKindAndHash(t *testing.T) {
	ctx := testkeeper.EVMTestApp.GetContextForDeliverTx(nil)
	ctx = ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore())
	k := &testkeeper.EVMTestApp.EvmKeeper
	caller := common.HexToAddress("0x13302101")
	otherCaller := common.HexToAddress("0x13302102")
	key := bindProgramsPrincipal(t, ctx, k, caller, 0x55)
	otherKey := bindProgramsPrincipal(t, ctx, k, otherCaller, 0x56)
	require.NotEqual(t, key, otherKey)
	var hashes [][32]byte
	for _, tc := range []struct {
		caller common.Address
		key    [32]byte
	}{
		{caller, key},
		{otherCaller, otherKey},
	} {
		authority, _, err := lp.PrincipalOf(ctx, k, tc.caller)
		require.NoError(t, err)
		require.Equal(t, uint8(1), lxphost.AuthorityKindOwner)
		require.Equal(t, lxphost.AuthorityKindOwner, authority.Kind)
		require.Equal(t, [32]byte{}, authority.GrantID)
		require.Equal(t, tc.key, authority.VerifiedKey)
		wantHash, err := lxphost.AuthorityHash(lxphost.AuthorityKindOwner, [32]byte{}, tc.key)
		require.NoError(t, err)
		require.NotEqual(t, [32]byte{}, wantHash)
		require.Equal(t, wantHash, authority.AuthorityHash)
		hashes = append(hashes, authority.AuthorityHash)
	}
	require.NotEqual(t, hashes[0], hashes[1])
	for _, kind := range []uint8{0, 255} {
		_, err := lxphost.AuthorityHash(kind, [32]byte{}, key)
		require.Error(t, err)
	}
}

func TestProgramsPrincipalUnboundRefused(t *testing.T) {
	ctx := testkeeper.EVMTestApp.GetContextForDeliverTx(nil)
	ctx = ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore())
	k := &testkeeper.EVMTestApp.EvmKeeper
	boundCaller := common.HexToAddress("0x13252303")
	unboundCaller := common.HexToAddress("0x13252304")
	bindProgramsPrincipal(t, ctx, k, boundCaller, 0x57)
	authority, actorDID, err := lp.PrincipalOf(ctx, k, boundCaller)
	require.NoError(t, err)
	require.NotEqual(t, lxphost.KernelAuthority{}, authority)
	require.NotEmpty(t, actorDID)
	_, _, err = k.UnbindLayerX(ctx, boundCaller)
	require.NoError(t, err)
	for _, caller := range []common.Address{unboundCaller, boundCaller} {
		_, bound := k.GetLayerXDid(ctx, caller)
		require.False(t, bound)
		authority, actorDID, err := lp.PrincipalOf(ctx, k, caller)
		require.ErrorIs(t, err, lp.ErrUnboundCaller)
		require.Equal(t, lxphost.KernelAuthority{}, authority)
		require.Nil(t, actorDID)
		reason, unpackErr := abi.UnpackRevert(pcommon.LXPReason(err))
		require.NoError(t, unpackErr)
		require.Equal(t, "LXP_ADDR_EVM_NOT_BOUND: EVM address is not bound to a LayerX DID", reason)
	}
}
