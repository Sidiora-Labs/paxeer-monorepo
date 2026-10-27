package keeper

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func authExecuteActor(t *testing.T, f *authViewFixture) Actor {
	t.Helper()
	f.ctx = f.ctx.WithChainID("hyperpax_125-1")
	caller := common.Address{0x26, 0x64, 1}
	evm := &testkeeper.EVMTestApp.EvmKeeper
	message, err := evmtypes.LayerXBindMessage(evm.ChainID(f.ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(f.private, message))
	_, err = evm.BindLayerX(f.ctx, caller, f.public, signature)
	require.NoError(t, err)
	actor, err := f.keeper.Actor(f.ctx, caller)
	require.NoError(t, err)
	require.True(t, actor.Authority.Bound)
	require.Equal(t, f.didID, actor.Authority.Actor)
	return actor
}

func authExecutePayload(t *testing.T, f *authViewFixture, method string) ([]byte, [32]byte) {
	t.Helper()
	if method == "revoke" {
		return types.EncodeRevokePayload(f.capabilityID, 1, 44), f.capabilityID
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{74}, ed25519.SeedSize))
	var key [32]byte
	copy(key[:], private.Public().(ed25519.PublicKey))
	kind := byte(3)
	if method == "session" {
		kind = 2
		f.delegate = key
	} else {
		f.capabilityKey = key
	}
	body := f.grantBody(kind)
	id, err := codec.DomainHash(codec.DomainAuthorityHash, body)
	require.NoError(t, err)
	var payload []byte
	if kind == 2 {
		payload, err = types.EncodeSessionPayload(body, 100, [32]byte{0x64}, nil)
	} else {
		payload, err = types.EncodeCapabilityPayload(body)
	}
	require.NoError(t, err)
	return payload, id
}

func authExecuteSignedReceipt(t *testing.T, receipt lxphost.Receipt) []byte {
	t.Helper()
	require.NotEmpty(t, receipt.Encoded)
	require.Equal(t, byte(0), receipt.Encoded[len(receipt.Encoded)-1])
	signed := append([]byte(nil), receipt.Encoded[:len(receipt.Encoded)-1]...)
	signed = append(signed, 1)
	signed = binary.BigEndian.AppendUint32(signed, ed25519.SignatureSize)
	return append(signed, make([]byte, ed25519.SignatureSize)...)
}

func TestAuthExecuteIdempotentReplay(t *testing.T) {
	for _, method := range []string{"session", "capability", "revoke"} {
		t.Run(method, func(t *testing.T) {
			f := newAuthViewFixture(t)
			actor := authExecuteActor(t, f)
			payload, grantID := authExecutePayload(t, f, method)
			key := [32]byte{0x64}
			paramsBefore := authViewStores(f.ctx, f.harness.key)
			result, err := f.keeper.execute(f.ctx, actor, payload, key)
			require.NoError(t, err)
			require.Equal(t, lxphost.Code(0), result.ResultCode)
			require.Equal(t, uint64(44), result.GlobalSequence)
			require.Equal(t, f.timestamp, result.Timestamp)
			require.NotEmpty(t, result.Encoded)
			require.NotEmpty(t, result.Events)
			grant, err := f.keeper.GetGrant(f.ctx, grantID)
			require.NoError(t, err)
			require.NotEmpty(t, grant)
			stored, err := f.keeper.ResultByIdempotency(f.ctx, actor.DIDKey, key)
			require.NoError(t, err)
			require.Equal(t, authExecuteSignedReceipt(t, result.Receipt), stored)
			before := authViewStores(f.ctx, f.hostKey, f.harness.key)

			_, err = f.keeper.execute(f.ctx, actor, payload, key)
			require.ErrorIs(t, err, types.ErrIdempotentReplay)
			require.Equal(t, before, authViewStores(f.ctx, f.hostKey, f.harness.key))
			again, err := f.keeper.GetGrant(f.ctx, grantID)
			require.NoError(t, err)
			require.Equal(t, grant, again)
			stored, err = f.keeper.ResultByIdempotency(f.ctx, actor.DIDKey, key)
			require.NoError(t, err)
			require.Equal(t, authExecuteSignedReceipt(t, result.Receipt), stored)
			next, err := lxphost.NextSequence(f.ctx, f.hostKey)
			require.NoError(t, err)
			require.Equal(t, uint64(45), next)
			require.Equal(t, paramsBefore, authViewStores(f.ctx, f.harness.key))
		})
	}
}

func TestAuthExecuteSequenceReused(t *testing.T) {
	f := newAuthViewFixture(t)
	actor := authExecuteActor(t, f)
	payload, grantID := authExecutePayload(t, f, "capability")
	first, err := f.keeper.execute(f.ctx, actor, payload, [32]byte{0x65})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), first.ResultCode)
	require.Equal(t, uint64(44), first.GlobalSequence)
	grant, err := f.keeper.GetGrant(f.ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, f.grantBody(3), grant)
	paramsBefore := authViewStores(f.ctx, f.harness.key)
	key := [32]byte{0x66}

	refused, err := f.keeper.execute(f.ctx, actor, payload, key)
	require.ErrorIs(t, err, types.ErrSequenceReused)
	require.Equal(t, lxphost.Code(-301), refused.ResultCode)
	require.Equal(t, uint64(45), refused.GlobalSequence)
	require.NotEmpty(t, refused.Encoded)
	again, err := f.keeper.GetGrant(f.ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, grant, again)
	stored, err := f.keeper.ResultByIdempotency(f.ctx, actor.DIDKey, key)
	require.NoError(t, err)
	require.Equal(t, authExecuteSignedReceipt(t, refused.Receipt), stored)
	next, err := lxphost.NextSequence(f.ctx, f.hostKey)
	require.NoError(t, err)
	require.Equal(t, uint64(46), next)
	accountSequence, err := f.host.ReadIdentitySequence(f.ctx, actor.DID)
	require.NoError(t, err)
	require.Equal(t, uint64(5), accountSequence)
	require.Equal(t, paramsBefore, authViewStores(f.ctx, f.harness.key))
}
