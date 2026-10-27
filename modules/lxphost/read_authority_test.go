//go:build cgo

package lxphost

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const authorityReadTime = int64(1700000000000)

type authorityReadFixture struct {
	chain    *testChain
	host     *Host
	actor    testActor
	delegate testActor
	id       [32]byte
	purpose  [32]byte
	activity Activity
}

func newAuthorityReadFixture(t *testing.T) authorityReadFixture {
	t.Helper()
	f := authorityReadFixture{chain: newTestChain(t), actor: newTestActor(t, 61), delegate: newTestActor(t, 62)}
	f.host = f.chain.host(t)
	registered := execute(t, f.host, f.chain.context(1, authorityReadTime, 41),
		f.actor.register(t, f.host, 0, authorityReadTime), f.actor.authority())
	require.Equal(t, Code(0), registered.ResultCode)
	f.activity = f.actor.grant(t, f.host, 1, authorityReadTime, f.delegate.public,
		uint64(authorityReadTime)-1000, uint64(authorityReadTime)+1000, 41)
	var err error
	f.id, err = codec.DomainHash(codec.DomainAuthorityHash, f.activity.Payload[8:])
	require.NoError(t, err)
	copy(f.purpose[:], bytes.Repeat([]byte{0x22}, 32))
	issued := execute(t, f.host, f.chain.context(1, authorityReadTime, 42), f.activity, f.actor.authority())
	require.Equal(t, Code(0), issued.ResultCode)
	return f
}

func (f authorityReadFixture) revoke(t *testing.T, h *Host, ctx sdk.Context) {
	t.Helper()
	sequence, ok := ctx.Value(sequenceContextKey{}).(uint64)
	require.True(t, ok)
	payload := append([]byte{0x71, 6, 0, 3}, f.id[:]...)
	payload = binary.BigEndian.AppendUint64(append(payload, 1), sequence)
	activity := f.actor.activity(t, h, 0x00070006, 2, authorityReadTime, payload)
	revoked := execute(t, h, ctx, activity, f.actor.authority())
	require.Equal(t, Code(0), revoked.ResultCode)
}

func TestReadGrantAndScopeAfterGrant(t *testing.T) {
	f := newAuthorityReadFixture(t)
	ctx := f.chain.context(1, authorityReadTime, 42)
	beforeRoot := f.chain.commit()
	before := f.chain.dump()
	var asset [32]byte
	copy(asset[:], bytes.Repeat([]byte{0x11}, 32))
	expectedScope := AuthorityScope{
		ModuleMask:         1 << 9,
		ActivityOrdinalMin: 4,
		ActivityOrdinalMax: 4,
		AssetID:            asset,
		MaximumPerActivity: sdk.NewInt(1),
		MaximumTotal:       sdk.NewInt(10),
		SpentTotal:         sdk.ZeroInt(),
		MaximumPerPeriod:   sdk.ZeroInt(),
		SpentThisPeriod:    sdk.ZeroInt(),
		PurposeHash:        f.purpose,
	}
	expected := AuthorityGrant{
		GrantID:                   f.id,
		Grantor:                   f.actor.id,
		Grantee:                   f.actor.id,
		Kind:                      3,
		Key:                       f.delegate.public,
		Scope:                     expectedScope,
		NotBefore:                 uint64(authorityReadTime) - 1000,
		NotAfter:                  uint64(authorityReadTime) + 1000,
		GrantorRevocationSequence: 41,
		FeeBudget: AuthorityFeeBudget{
			MaximumPerActivity: sdk.ZeroInt(),
			MaximumTotal:       sdk.ZeroInt(),
			SpentTotal:         sdk.ZeroInt(),
			MaximumPerPeriod:   sdk.ZeroInt(),
			SpentThisPeriod:    sdk.ZeroInt(),
		},
	}
	for _, h := range []*Host{f.host, f.chain.host(t)} {
		grant, err := h.ReadGrant(ctx, f.id)
		require.NoError(t, err)
		require.Equal(t, expected, grant)
		scope, err := h.ReadCapabilityScope(ctx, f.id)
		require.NoError(t, err)
		require.Equal(t, expectedScope, scope)
		grant.Key[0] ^= 1
		scope.PurposeHash[0] ^= 1
		again, err := h.ReadGrant(ctx, f.id)
		require.NoError(t, err)
		require.Equal(t, expected, again)
		missing, err := h.ReadGrant(ctx, [32]byte{0xff})
		require.Equal(t, Code(-7), err)
		require.Equal(t, AuthorityGrant{}, missing)
		missingScope, err := h.ReadCapabilityScope(ctx, [32]byte{0xff})
		require.Equal(t, Code(-7), err)
		require.Equal(t, AuthorityScope{}, missingScope)
	}
	require.Equal(t, before, f.chain.dump())
	require.Equal(t, beforeRoot, f.chain.commit())
}

func TestReadIsLiveAfterRevoke(t *testing.T) {
	f := newAuthorityReadFixture(t)
	ctx := f.chain.context(1, authorityReadTime, 42)
	before := f.chain.dump()
	live, err := f.host.ReadIsLive(ctx, f.id, 42)
	require.NoError(t, err)
	require.True(t, live)
	for _, timestamp := range []int64{authorityReadTime - 1001, authorityReadTime + 1000} {
		live, err = f.host.ReadIsLive(f.chain.context(1, timestamp, 42), f.id, 42)
		require.NoError(t, err)
		require.False(t, live)
	}
	live, err = f.host.ReadIsLive(ctx, [32]byte{0xff}, 42)
	require.Equal(t, Code(-7), err)
	require.False(t, live)
	require.Equal(t, before, f.chain.dump())

	branch, _ := ctx.CacheContext()
	f.revoke(t, f.host, WithSequence(branch, 43))
	live, err = f.host.ReadIsLive(branch, f.id, 43)
	require.NoError(t, err)
	require.False(t, live)
	live, err = f.host.ReadIsLive(ctx, f.id, 42)
	require.NoError(t, err)
	require.True(t, live)
	require.Equal(t, before, f.chain.dump())

	f.revoke(t, f.host, f.chain.context(1, authorityReadTime, 43))
	beforeRoot := f.chain.commit()
	before = f.chain.dump()
	for _, h := range []*Host{f.host, f.chain.host(t)} {
		live, err = h.ReadIsLive(ctx, f.id, 43)
		require.NoError(t, err)
		require.False(t, live)
		grant, err := h.ReadGrant(ctx, f.id)
		require.NoError(t, err)
		require.True(t, grant.Revoked)
		require.Equal(t, uint64(43), grant.RevokedAtSequence)
	}
	require.Equal(t, before, f.chain.dump())
	require.Equal(t, beforeRoot, f.chain.commit())
}

func TestReadIdentityAndCheckActivityLeaveStoreUnchanged(t *testing.T) {
	f := newAuthorityReadFixture(t)
	ctx := f.chain.context(1, authorityReadTime, 42)
	beforeRoot := f.chain.commit()
	before := f.chain.dump()
	for _, h := range []*Host{f.host, f.chain.host(t)} {
		identity, err := h.ReadIdentityState(ctx, f.actor.did)
		require.NoError(t, err)
		require.Equal(t, f.actor.id, identity.DIDID)
		require.Equal(t, uint8(1), identity.Status)
		require.Equal(t, f.actor.public, identity.PrimaryKey)
		require.Equal(t, uint64(41), identity.RevocationSequence)
		require.Equal(t, uint64(2), identity.NextSequence)
		require.False(t, identity.HasPendingKey)
		require.Equal(t, f.actor.id, identity.Authority.Actor)
		require.Equal(t, f.actor.id, identity.Authority.Principal)
		require.Equal(t, f.actor.public, identity.Authority.VerifiedKey)
		require.Equal(t, uint8(1), identity.Authority.Kind)

		authority, err := h.ReadCheckActivity(ctx, f.actor.did, f.id, f.purpose)
		require.NoError(t, err)
		require.Equal(t, f.actor.id, authority.Actor)
		require.Equal(t, f.actor.id, authority.Principal)
		require.Equal(t, f.delegate.public, authority.VerifiedKey)
		require.Equal(t, uint8(3), authority.Kind)
		require.Equal(t, f.id, authority.GrantID)
		scope, err := h.ReadCapabilityScope(ctx, f.id)
		require.NoError(t, err)
		require.Equal(t, scope, authority.Scope)
		preimage := append(append([]byte{3}, f.id[:]...), f.delegate.public[:]...)
		expectedHash, err := codec.DomainHash(codec.DomainAuthorityHash, preimage)
		require.NoError(t, err)
		require.Equal(t, expectedHash, authority.AuthorityHash)

		other := newTestActor(t, 63)
		missingIdentity, err := h.ReadIdentityState(ctx, other.did)
		require.Equal(t, Code(-200), err)
		require.Equal(t, IdentityState{}, missingIdentity)
		_, err = h.ReadIdentityState(ctx, nil)
		require.Equal(t, Code(-200), err)
		_, err = h.ReadCheckActivity(ctx, f.actor.did, [32]byte{0xff}, f.purpose)
		require.Equal(t, Code(-7), err)
		_, err = h.ReadCheckActivity(ctx, other.did, f.id, f.purpose)
		require.Equal(t, Code(-200), err)
		_, err = h.ReadCheckActivity(ctx, f.actor.did, f.id, [32]byte{})
		require.Equal(t, Code(-204), err)
		_, err = h.ReadCheckActivity(f.chain.context(1, authorityReadTime+1000, 42), f.actor.did, f.id, f.purpose)
		require.Equal(t, Code(-202), err)
		require.Equal(t, before, f.chain.dump())
	}
	require.Equal(t, beforeRoot, f.chain.commit())

	other := newTestActor(t, 64)
	registered := execute(t, f.host, f.chain.context(1, authorityReadTime, 43),
		other.register(t, f.host, 0, authorityReadTime), other.authority())
	require.Equal(t, Code(0), registered.ResultCode)
	beforeRoot = f.chain.commit()
	before = f.chain.dump()
	_, err := f.host.ReadCheckActivity(ctx, other.did, f.id, f.purpose)
	require.Equal(t, Code(-207), err)
	require.Equal(t, before, f.chain.dump())

	branch, _ := ctx.CacheContext()
	f.revoke(t, f.host, WithSequence(branch, 44))
	_, err = f.host.ReadCheckActivity(branch, f.actor.did, f.id, f.purpose)
	require.Equal(t, Code(-203), err)
	_, err = f.host.ReadCheckActivity(ctx, f.actor.did, f.id, f.purpose)
	require.NoError(t, err)
	require.Equal(t, before, f.chain.dump())
	require.Equal(t, beforeRoot, f.chain.commit())
}
