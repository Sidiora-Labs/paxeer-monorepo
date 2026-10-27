package layerxauth_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func authRevokeGrantBody(didID, delegate [32]byte, timestamp uint64, kind byte) []byte {
	body := []byte{0, 1, 0x20, 1, 1}
	field := func(value []byte) {
		body = binary.BigEndian.AppendUint32(body, uint32(len(value)))
		body = append(body, value...)
	}
	u64 := func(value uint64) { body = binary.BigEndian.AppendUint64(body, value) }
	u128 := func(value uint64) { u64(0); u64(value) }
	field(didID[:])
	field(didID[:])
	body = append(body, kind)
	field(delegate[:])
	u64(1 << 9)
	body = binary.BigEndian.AppendUint16(body, 4)
	body = binary.BigEndian.AppendUint16(body, 5)
	if kind == 2 {
		field(make([]byte, 32))
		body = append(body, make([]byte, 96)...)
		field(make([]byte, 32))
	} else {
		field(bytes.Repeat([]byte{0x11}, 32))
		u128(10)
		u128(20)
		u128(0)
		u64(1000)
		u128(15)
		u128(0)
		u64(timestamp - 1000)
		field(bytes.Repeat([]byte{0x22}, 32))
	}
	u64(timestamp - 1000)
	u64(timestamp + 1000)
	u64(41)
	body = append(body, 0)
	u64(0)
	field(make([]byte, ed25519.SignatureSize))
	return body
}

func TestPrecompileAuthRevokeGrant(t *testing.T) {
	for _, kind := range []byte{2, 3} {
		name := "session"
		if kind == 3 {
			name = "capability"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			host, err := lxphost.NewHost(lxphost.Config{
				StoreKey: h.hostKey, NetworkID: 7, ParameterVersion: 1,
				TimestampWindowMs: 60000, GasLimit: 1000000,
			})
			require.NoError(t, err)
			t.Cleanup(host.Close)
			private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
			actor, err := h.keeper.Actor(h.ctx, h.caller)
			require.NoError(t, err)
			timestamp := uint64(h.ctx.BlockTime().UnixMilli())
			execute := func(sequence, accountSequence uint64, activityType uint32, payload []byte) {
				t.Helper()
				activity := lxphost.Activity{
					ProtocolVersion: 3, Type: activityType, ActorDID: actor.DID,
					AuthorityKey: actor.DIDKey, AccountSequence: accountSequence,
					NotBefore: timestamp - 1000, NotAfter: timestamp + 1000,
					IdempotencyKey: sha256.Sum256(payload), Payload: payload,
				}
				hash, err := host.SigningHash(activity)
				require.NoError(t, err)
				activity.Signature = ed25519.Sign(private, hash[:])
				receipt, err := host.Execute(lxphost.WithSequence(h.ctx, sequence), activity, lxphost.KernelAuthority{
					Actor: actor.Authority.Actor, Principal: actor.Authority.Principal, VerifiedKey: actor.DIDKey,
				})
				require.NoError(t, err)
				require.Equal(t, lxphost.Code(0), receipt.ResultCode)
			}
			registration := append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...)
			registration = append(registration, actor.DIDKey[:]...)
			execute(41, 0, 0x00070001, registration)
			var delegate [32]byte
			copy(delegate[:], ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, ed25519.SeedSize)).Public().(ed25519.PublicKey))
			body := authRevokeGrantBody(actor.Authority.Actor, delegate, timestamp, kind)
			grantID, err := codec.DomainHash(codec.DomainAuthorityHash, body)
			require.NoError(t, err)
			if kind == 2 {
				payload, err := types.EncodeSessionPayload(body, 100, [32]byte{0x33}, nil)
				require.NoError(t, err)
				execute(42, 1, 0x00070005, payload)
			} else {
				payload, err := types.EncodeCapabilityPayload(body)
				require.NoError(t, err)
				execute(42, 1, 0x00070008, payload)
			}
			live, reason, err := h.keeper.IsLive(h.ctx, grantID, 42, timestamp)
			require.NoError(t, err)
			require.True(t, live)
			require.Empty(t, reason)

			otherPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{75}, ed25519.SeedSize))
			var otherPublic [32]byte
			copy(otherPublic[:], otherPrivate.Public().(ed25519.PublicKey))
			evmKeeper := &testkeeper.EVMTestApp.EvmKeeper
			message, err := evmtypes.LayerXBindMessage(evmKeeper.ChainID(h.ctx), h.unbound, 0)
			require.NoError(t, err)
			var signature [ed25519.SignatureSize]byte
			copy(signature[:], ed25519.Sign(otherPrivate, message))
			_, err = evmKeeper.BindLayerX(h.ctx, h.unbound, otherPublic, signature)
			require.NoError(t, err)
			otherActor, err := h.keeper.Actor(h.ctx, h.unbound)
			require.NoError(t, err)
			require.NotEqual(t, actor.Authority.Actor, otherActor.Authority.Actor)

			a := h.precompile.GetABI()
			input, err := a.Pack("revokeGrant", grantID, uint8(1), uint64(43))
			require.NoError(t, err)
			evm, db := h.evm()
			before := hostSnapshot(t, db.Ctx(), h.hostKey)
			data, err := h.precompile.Run(evm, h.unbound, h.precompile.Address(), input, nil, false, false, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			assertLxpRevert(t, data, "LXP_UNAUTHORIZED_DEBIT")
			require.ErrorIs(t, db.GetPrecompileError(), types.ErrUnauthorizedDebit)
			require.Equal(t, before, hostSnapshot(t, db.Ctx(), h.hostKey))
			require.Empty(t, db.GetAllLogs())
			require.Empty(t, db.Ctx().EventManager().Events())

			evm, db = h.evm()
			data, err = h.precompile.Run(evm, h.caller, h.precompile.Address(), input, nil, false, false, nil)
			require.NoError(t, err)
			require.Empty(t, data)
			require.Nil(t, db.GetPrecompileError())
			logs := db.GetAllLogs()
			require.Len(t, logs, 2)
			record := append(append([]byte(nil), grantID[:]...), 1)
			record = binary.BigEndian.AppendUint64(record, 43)
			var identityBody []byte
			for _, value := range hostSnapshot(t, db.Ctx(), h.hostKey) {
				if bytes.HasPrefix(value, append([]byte("LXGI1"), actor.Authority.Actor[:]...)) {
					require.Nil(t, identityBody)
					identityBody = value
				}
			}
			require.Len(t, identityBody, 223)
			for i, expected := range []struct {
				name string
				id   common.Hash
				body []byte
			}{
				{layerxauth.GrantRevoked, grantID, record},
				{layerxauth.IdentityStateUpdated, actor.Authority.Actor, identityBody},
			} {
				require.Equal(t, common.HexToAddress(layerxauth.AuthAddress), logs[i].Address)
				require.Equal(t, []common.Hash{a.Events[expected.name].ID, expected.id}, logs[i].Topics)
				decoded, err := a.Events[expected.name].Inputs.NonIndexed().Unpack(logs[i].Data)
				require.NoError(t, err)
				require.Equal(t, []interface{}{expected.body}, decoded)
			}
			live, reason, err = h.keeper.IsLive(db.Ctx(), grantID, 43, timestamp)
			require.NoError(t, err)
			require.False(t, live)
			require.Equal(t, "LXP_AUTH_REVOKED", reason)
			grant, err := host.ReadGrant(db.Ctx(), grantID)
			require.NoError(t, err)
			require.True(t, grant.Revoked)
			require.Equal(t, uint64(43), grant.RevokedAtSequence)
			revocation, err := h.keeper.RevocationSequence(db.Ctx(), actor.DIDKey)
			require.NoError(t, err)
			require.Equal(t, uint64(43), revocation)
			next, err := lxphost.NextSequence(db.Ctx(), h.hostKey)
			require.NoError(t, err)
			require.Equal(t, uint64(44), next)
		})
	}
}
