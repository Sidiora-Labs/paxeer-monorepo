package keeper

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

type authViewFixture struct {
	*harness
	private                                ed25519.PrivateKey
	public, delegate, capabilityKey, didID [32]byte
	did                                    []byte
	sessionID, capabilityID                [32]byte
	sessionBody, capabilityBody            []byte
	sessionActivity                        lxphost.Activity
	timestamp                              uint64
	asset, purpose                         [32]byte
}

func newAuthViewFixture(t *testing.T) *authViewFixture {
	t.Helper()
	f := &authViewFixture{harness: newHarness(t)}
	f.private = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{71}, ed25519.SeedSize))
	copy(f.public[:], f.private.Public().(ed25519.PublicKey))
	delegate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, ed25519.SeedSize))
	copy(f.delegate[:], delegate.Public().(ed25519.PublicKey))
	capabilityKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{73}, ed25519.SeedSize))
	copy(f.capabilityKey[:], capabilityKey.Public().(ed25519.PublicKey))
	f.did = []byte(evmtypes.LayerXDid(f.public))
	var err error
	f.didID, err = lxphost.DIDID(f.did)
	require.NoError(t, err)
	f.timestamp = uint64(f.ctx.BlockTime().UnixMilli())
	copy(f.asset[:], bytes.Repeat([]byte{0x11}, 32))
	copy(f.purpose[:], bytes.Repeat([]byte{0x22}, 32))
	registration := append(append([]byte{0x71, 1, 0, 2}, f.didID[:]...), f.public[:]...)
	f.execute(t, f.ctx, 41, f.activity(t, 0x00070001, 0, registration))
	f.sessionBody = f.grantBody(2)
	f.sessionID, err = codec.DomainHash(codec.DomainAuthorityHash, f.sessionBody)
	require.NoError(t, err)
	sessionPayload, err := types.EncodeSessionPayload(f.sessionBody, 100, [32]byte{0x33}, nil)
	require.NoError(t, err)
	f.sessionActivity = f.activity(t, 0x00070005, 1, sessionPayload)
	f.execute(t, f.ctx, 42, f.sessionActivity)
	f.capabilityBody = f.grantBody(3)
	f.capabilityID, err = codec.DomainHash(codec.DomainAuthorityHash, f.capabilityBody)
	require.NoError(t, err)
	capabilityPayload, err := types.EncodeCapabilityPayload(f.capabilityBody)
	require.NoError(t, err)
	f.execute(t, f.ctx, 43, f.activity(t, 0x00070008, 2, capabilityPayload))
	return f
}

func (f *authViewFixture) grantBody(kind byte) []byte {
	body := []byte{0, 1, 0x20, 1, 1}
	field := func(value []byte) {
		body = binary.BigEndian.AppendUint32(body, uint32(len(value)))
		body = append(body, value...)
	}
	u64 := func(value uint64) { body = binary.BigEndian.AppendUint64(body, value) }
	u128 := func(value uint64) { u64(0); u64(value) }
	field(f.didID[:])
	field(f.didID[:])
	body = append(body, kind)
	if kind == 2 {
		field(f.delegate[:])
	} else {
		field(f.capabilityKey[:])
	}
	u64(1 << 9)
	body = binary.BigEndian.AppendUint16(body, 4)
	body = binary.BigEndian.AppendUint16(body, 5)
	if kind == 2 {
		field(make([]byte, 32))
		body = append(body, make([]byte, 96)...)
		field(make([]byte, 32))
	} else {
		field(f.asset[:])
		u128(10)
		u128(20)
		u128(0)
		u64(1000)
		u128(15)
		u128(0)
		u64(f.timestamp - 1000)
		field(f.purpose[:])
	}
	u64(f.timestamp - 1000)
	u64(f.timestamp + 1000)
	u64(41)
	body = append(body, 0)
	u64(0)
	field(make([]byte, ed25519.SignatureSize))
	return body
}

func (f *authViewFixture) activity(t *testing.T, activityType uint32, accountSequence uint64, payload []byte) lxphost.Activity {
	t.Helper()
	activity := lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: f.did, AuthorityKey: f.public,
		AccountSequence: accountSequence, NotBefore: f.timestamp - 1000, NotAfter: f.timestamp + 1000,
		IdempotencyKey: sha256.Sum256(binary.BigEndian.AppendUint64(append([]byte(nil), f.did...), accountSequence)),
		Payload:        payload,
	}
	hash, err := f.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(f.private, hash[:])
	return activity
}

func (f *authViewFixture) execute(t *testing.T, ctx sdk.Context, sequence uint64, activity lxphost.Activity) lxphost.Receipt {
	t.Helper()
	receipt, err := f.host.Execute(lxphost.WithSequence(ctx, sequence), activity, lxphost.KernelAuthority{
		Actor: f.didID, Principal: f.didID, VerifiedKey: f.public,
	})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	return receipt
}

func authViewStores(ctx sdk.Context, keys ...sdk.StoreKey) []map[string][]byte {
	out := make([]map[string][]byte, len(keys))
	for i, key := range keys {
		out[i] = map[string][]byte{}
		iterator := ctx.KVStore(key).Iterator(nil, nil)
		for ; iterator.Valid(); iterator.Next() {
			out[i][string(iterator.Key())] = append([]byte(nil), iterator.Value()...)
		}
		iterator.Close()
	}
	return out
}

func TestAuthGrantViews(t *testing.T) {
	f := newAuthViewFixture(t)
	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true, IdempotencyRetention: 4096}))
	before := authViewStores(f.ctx, f.hostKey, f.harness.key)
	for _, grant := range []struct {
		id   [32]byte
		body []byte
	}{
		{f.sessionID, f.sessionBody}, {f.capabilityID, f.capabilityBody},
	} {
		body, err := f.keeper.GetGrant(f.ctx, grant.id)
		require.NoError(t, err)
		require.Equal(t, grant.body, body)
		scope, err := f.keeper.CapabilityScope(f.ctx, grant.id)
		require.NoError(t, err)
		require.Equal(t, grant.body[114:294], scope)
		body[0] ^= 1
		scope[0] ^= 1
		again, err := f.keeper.GetGrant(f.ctx, grant.id)
		require.NoError(t, err)
		require.Equal(t, grant.body, again)
	}
	expected := append([]byte("LXGS2"), f.sessionID[:]...)
	expected = append(expected, f.didID[:]...)
	expected = append(expected, f.public[:]...)
	action := [32]byte{0x33}
	expected = append(expected, action[:]...)
	expected = append(expected, f.delegate[:]...)
	expected = binary.BigEndian.AppendUint64(expected, 100)
	expected = binary.BigEndian.AppendUint64(expected, 1<<9)
	expected = binary.BigEndian.AppendUint16(expected, 4)
	expected = binary.BigEndian.AppendUint16(expected, 5)
	expected = binary.BigEndian.AppendUint64(expected, f.timestamp-1000)
	expected = binary.BigEndian.AppendUint64(expected, f.timestamp+1000)
	expected = binary.BigEndian.AppendUint64(expected, 41)
	summary, err := f.keeper.SessionKey(f.ctx, f.sessionID)
	require.NoError(t, err)
	require.Equal(t, expected, summary)
	summary[0] ^= 1
	again, err := f.keeper.SessionKey(f.ctx, f.sessionID)
	require.NoError(t, err)
	require.Equal(t, expected, again)
	revocation, err := f.keeper.RevocationSequence(f.ctx, f.public)
	require.NoError(t, err)
	require.Equal(t, uint64(41), revocation)
	_, err = f.keeper.RevocationSequence(f.ctx, [32]byte{0xff})
	require.Equal(t, lxphost.Code(-200), err)
	for _, read := range []func(sdk.Context, [32]byte) ([]byte, error){f.keeper.GetGrant, f.keeper.SessionKey, f.keeper.CapabilityScope} {
		missing, err := read(f.ctx, [32]byte{0xff})
		require.Equal(t, lxphost.Code(-7), err)
		require.Empty(t, missing)
	}
	require.Equal(t, before, authViewStores(f.ctx, f.hostKey, f.harness.key))
}

func TestAuthIsLive(t *testing.T) {
	f := newAuthViewFixture(t)
	before := authViewStores(f.ctx, f.hostKey, f.harness.key)
	for _, tc := range []struct {
		timestamp uint64
		reason    string
	}{
		{f.timestamp, ""}, {f.timestamp - 1000, ""},
		{f.timestamp - 1001, "LXP_NOT_YET_VALID"}, {f.timestamp + 1000, "LXP_AUTH_EXPIRED"},
	} {
		live, reason, err := f.keeper.IsLive(f.ctx.WithBlockTime(time.UnixMilli(1)), f.capabilityID, 43, tc.timestamp)
		require.NoError(t, err)
		require.Equal(t, tc.reason == "", live)
		require.Equal(t, tc.reason, reason)
	}
	branch, _ := f.ctx.CacheContext()
	revoke := f.activity(t, 0x00070006, 3, types.EncodeRevokePayload(f.capabilityID, 1, 44))
	f.execute(t, branch, 44, revoke)
	branchBefore := authViewStores(branch, f.hostKey, f.harness.key)
	live, reason, err := f.keeper.IsLive(branch, f.capabilityID, 44, f.timestamp)
	require.NoError(t, err)
	require.False(t, live)
	require.Equal(t, "LXP_AUTH_REVOKED", reason)
	live, reason, err = f.keeper.IsLive(branch, f.capabilityID, 44, f.timestamp+1000)
	require.NoError(t, err)
	require.False(t, live)
	require.Equal(t, "LXP_AUTH_EXPIRED", reason)
	live, reason, err = f.keeper.IsLive(f.ctx, f.capabilityID, 43, f.timestamp)
	require.NoError(t, err)
	require.True(t, live)
	require.Empty(t, reason)
	_, _, err = f.keeper.IsLive(f.ctx, f.capabilityID, 43, math.MaxUint64)
	require.ErrorIs(t, err, types.ErrNonCanonical)
	require.Equal(t, branchBefore, authViewStores(branch, f.hostKey, f.harness.key))
	require.Equal(t, before, authViewStores(f.ctx, f.hostKey, f.harness.key))
}

func TestAuthCheckActivity(t *testing.T) {
	f := newAuthViewFixture(t)
	before := authViewStores(f.ctx, f.hostKey, f.harness.key)
	for _, tc := range []struct {
		name         string
		activityType uint32
		asset        [32]byte
		amount       sdk.Int
		reason       string
	}{
		{"inside", 0x00090004, f.asset, sdk.NewInt(5), ""},
		{"boundary", 0x00090005, f.asset, sdk.NewInt(10), ""},
		{"module", 0x00080004, f.asset, sdk.NewInt(5), "LXP_AUTH_SCOPE"},
		{"large module", 0x00400004, f.asset, sdk.NewInt(5), "LXP_AUTH_SCOPE"},
		{"low ordinal", 0x00090003, f.asset, sdk.NewInt(5), "LXP_AUTH_SCOPE"},
		{"high ordinal", 0x00090006, f.asset, sdk.NewInt(5), "LXP_AUTH_SCOPE"},
		{"asset", 0x00090004, [32]byte{2}, sdk.NewInt(5), "LXP_AUTH_SCOPE"},
		{"amount", 0x00090004, f.asset, sdk.NewInt(11), "LXP_AUTH_SCOPE"},
		{"negative", 0x00090004, f.asset, sdk.NewInt(-1), "LXP_AUTH_SCOPE"},
		{"nil", 0x00090004, f.asset, sdk.Int{}, "LXP_AUTH_SCOPE"},
		{"overflow", 0x00090004, f.asset, sdk.NewIntFromBigInt(new(big.Int).Lsh(big.NewInt(1), 128)), "LXP_AUTH_SCOPE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason, err := f.keeper.CheckActivity(f.ctx, f.capabilityID, tc.activityType, tc.asset, tc.amount)
			require.NoError(t, err)
			require.Equal(t, tc.reason, reason)
		})
	}
	reason, err := f.keeper.CheckActivity(f.ctx.WithBlockTime(time.UnixMilli(int64(f.timestamp+1000))), f.capabilityID, 0x00090004, f.asset, sdk.NewInt(5))
	require.NoError(t, err)
	require.Equal(t, "LXP_AUTH_EXPIRED", reason)
	branch, _ := f.ctx.CacheContext()
	f.execute(t, branch, 44, f.activity(t, 0x00070006, 3, types.EncodeRevokePayload(f.capabilityID, 1, 44)))
	branchBefore := authViewStores(branch, f.hostKey, f.harness.key)
	reason, err = f.keeper.CheckActivity(branch, f.capabilityID, 0x00090004, f.asset, sdk.NewInt(5))
	require.NoError(t, err)
	require.Equal(t, "LXP_AUTH_REVOKED", reason)
	require.Equal(t, branchBefore, authViewStores(branch, f.hostKey, f.harness.key))
	require.Equal(t, before, authViewStores(f.ctx, f.hostKey, f.harness.key))
}

func TestAuthResultByIdempotency(t *testing.T) {
	f := newAuthViewFixture(t)
	before := authViewStores(f.ctx, f.hostKey, f.harness.key)
	stored, err := f.host.ReadIdempotentReceipt(f.ctx, f.did, f.sessionActivity.IdempotencyKey)
	require.NoError(t, err)
	require.NotEmpty(t, stored.Encoded)
	encoded, err := f.keeper.ResultByIdempotency(f.ctx, f.public, f.sessionActivity.IdempotencyKey)
	require.NoError(t, err)
	require.Equal(t, stored.Encoded, encoded)
	decoded, err := codec.DecodeReceipt(encoded)
	require.NoError(t, err)
	require.Equal(t, uint64(42), decoded.GlobalSequence)
	encoded[0] ^= 1
	again, err := f.keeper.ResultByIdempotency(f.ctx, f.public, f.sessionActivity.IdempotencyKey)
	require.NoError(t, err)
	require.Equal(t, stored.Encoded, again)
	missing, err := f.keeper.ResultByIdempotency(f.ctx, f.public, [32]byte{0xff})
	require.NoError(t, err)
	require.Empty(t, missing)
	missing, err = f.keeper.ResultByIdempotency(f.ctx, f.delegate, f.sessionActivity.IdempotencyKey)
	require.NoError(t, err)
	require.Empty(t, missing)
	require.Equal(t, before, authViewStores(f.ctx, f.hostKey, f.harness.key))
}
