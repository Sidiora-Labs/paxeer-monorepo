package keeper

import (
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/stretchr/testify/require"
)

func authSessionWire(grant []byte, expiry uint64, action [32]byte, successor *types.SessionSuccessor) []byte {
	payload := []byte{0x71, 5, 1, 3}
	if successor != nil {
		payload[2], payload[3] = 2, 5
	}
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(grant)))
	payload = append(payload, grant...)
	payload = binary.BigEndian.AppendUint64(payload, expiry)
	field := func(value [32]byte) {
		payload = binary.BigEndian.AppendUint32(payload, 32)
		payload = append(payload, value[:]...)
	}
	field(action)
	if successor != nil {
		field(successor.PredecessorID)
		field(successor.ExpectedCommitment)
	}
	return payload
}

func authSessionEvents(t *testing.T, f *authViewFixture, result Result, grantID [32]byte, body []byte, expiry uint64, action [32]byte) {
	t.Helper()
	require.Equal(t, lxphost.Code(0), result.ResultCode)
	require.NotEmpty(t, result.Encoded)
	summary, err := f.host.ReadSessionKey(f.ctx, grantID)
	require.NoError(t, err)
	require.Len(t, summary, 209)
	require.Equal(t, []byte("LXGS2"), summary[:5])
	require.Equal(t, grantID[:], summary[5:37])
	require.Equal(t, f.didID[:], summary[37:69])
	require.Equal(t, f.public[:], summary[69:101])
	require.Equal(t, action[:], summary[101:133])
	require.Equal(t, f.delegate[:], summary[133:165])
	require.Equal(t, expiry, binary.BigEndian.Uint64(summary[165:173]))
	stored, err := f.keeper.GetGrant(f.ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, body, stored)
	events, err := AuthEvents(result.Receipt)
	require.NoError(t, err)
	require.Len(t, events, 4)
	require.Equal(t, Event{Name: "SessionGranted", GrantID: grantID, Body: summary}, events[0])
	require.Equal(t, Event{Name: "GrantBody", GrantID: grantID, Sub: 5, Chunk: 0, Body: body[:256]}, events[1])
	require.Equal(t, Event{Name: "GrantBody", GrantID: grantID, Sub: 5, Chunk: 1, Body: body[256:]}, events[2])
	require.Equal(t, "IdentityStateUpdated", events[3].Name)
	require.Equal(t, f.didID, events[3].DIDID)
	live, err := f.host.ReadIsLive(f.ctx, grantID, result.GlobalSequence)
	require.NoError(t, err)
	require.True(t, live)
}

func TestGrantSession(t *testing.T) {
	f := newAuthViewFixture(t)
	actor := authExecuteActor(t, f)
	_, grantID := authExecutePayload(t, f, "session")
	body := f.grantBody(2)
	action, idempotency := [32]byte{0x81}, [32]byte{0x82}
	const expiry = uint64(123)
	before := authViewStores(f.ctx, f.hostKey, f.harness.key)

	t.Run("rejects before execution", func(t *testing.T) {
		result, err := f.keeper.GrantSession(f.ctx, actor, nil, expiry, action, idempotency)
		require.ErrorIs(t, err, types.ErrNonCanonical)
		require.Equal(t, Result{}, result)
		result, err = f.keeper.GrantSession(f.ctx, Actor{}, body, expiry, action, idempotency)
		require.ErrorIs(t, err, types.ErrAuthScope)
		require.Equal(t, Result{}, result)
		require.Equal(t, before, authViewStores(f.ctx, f.hostKey, f.harness.key))
	})

	t.Run("pause precedes encoding", func(t *testing.T) {
		ctx, _ := f.ctx.CacheContext()
		params := f.keeper.GetParams(ctx)
		params.Paused = true
		require.NoError(t, f.keeper.SetParams(ctx, params))
		paused := authViewStores(ctx, f.hostKey, f.harness.key)
		result, err := f.keeper.GrantSession(ctx, actor, nil, expiry, action, idempotency)
		require.ErrorIs(t, err, govtypes.ErrPausedScope)
		require.Contains(t, err.Error(), "layerxauth.grantSession")
		require.Equal(t, Result{}, result)
		require.Equal(t, paused, authViewStores(ctx, f.hostKey, f.harness.key))
	})

	branch, _ := f.ctx.CacheContext()
	want, err := f.keeper.execute(branch, actor, authSessionWire(body, expiry, action, nil), idempotency)
	require.NoError(t, err)
	result, err := f.keeper.GrantSession(f.ctx, actor, body, expiry, action, idempotency)
	require.NoError(t, err)
	require.Equal(t, want.Encoded, result.Encoded)
	require.Equal(t, uint64(44), result.GlobalSequence)
	authSessionEvents(t, f, result, grantID, body, expiry, action)
	stored, err := f.keeper.ResultByIdempotency(f.ctx, actor.DIDKey, idempotency)
	require.NoError(t, err)
	require.Equal(t, authExecuteSignedReceipt(t, result.Receipt), stored)

	t.Run("replay preserves grant", func(t *testing.T) {
		before := authViewStores(f.ctx, f.hostKey, f.harness.key)
		_, err := f.keeper.GrantSession(f.ctx, actor, body, expiry, action, idempotency)
		require.ErrorIs(t, err, types.ErrIdempotentReplay)
		require.Equal(t, before, authViewStores(f.ctx, f.hostKey, f.harness.key))
	})

	t.Run("kernel refusal retains receipt", func(t *testing.T) {
		refused, err := f.keeper.GrantSession(f.ctx, actor, body, expiry, action, [32]byte{0x83})
		require.ErrorIs(t, err, types.ErrSequenceReused)
		require.Equal(t, lxphost.Code(-301), refused.ResultCode)
		require.ErrorIs(t, err, FromKernel(refused.ResultCode))
		require.NotEmpty(t, refused.Encoded)
		require.Empty(t, refused.Events)
		authSessionEvents(t, f, result, grantID, body, expiry, action)
	})
}

func TestGrantSessionSuccessor(t *testing.T) {
	f := newAuthViewFixture(t)
	actor := authExecuteActor(t, f)
	_, grantID := authExecutePayload(t, f, "session")
	body := f.grantBody(2)
	action, idempotency := [32]byte{0x91}, [32]byte{0x92}
	successor := types.SessionSuccessor{PredecessorID: f.sessionID, ExpectedCommitment: [32]byte{0x93}}
	const expiry = uint64(124)
	before := authViewStores(f.ctx, f.hostKey, f.harness.key)
	predecessor, err := f.host.ReadGrant(f.ctx, f.sessionID)
	require.NoError(t, err)
	require.False(t, predecessor.Revoked)
	live, err := f.host.ReadIsLive(f.ctx, f.sessionID, 43)
	require.NoError(t, err)
	require.True(t, live)
	summary, err := f.host.ReadSessionKey(f.ctx, f.sessionID)
	require.NoError(t, err)

	t.Run("rejects before execution", func(t *testing.T) {
		result, err := f.keeper.GrantSessionSuccessor(f.ctx, actor, nil, expiry, action, successor.PredecessorID, successor.ExpectedCommitment, idempotency)
		require.ErrorIs(t, err, types.ErrNonCanonical)
		require.Equal(t, Result{}, result)
		result, err = f.keeper.GrantSessionSuccessor(f.ctx, Actor{}, body, expiry, action, successor.PredecessorID, successor.ExpectedCommitment, idempotency)
		require.ErrorIs(t, err, types.ErrAuthScope)
		require.Equal(t, Result{}, result)
		require.Equal(t, before, authViewStores(f.ctx, f.hostKey, f.harness.key))
	})

	t.Run("pause precedes encoding", func(t *testing.T) {
		ctx, _ := f.ctx.CacheContext()
		params := f.keeper.GetParams(ctx)
		params.Paused = true
		require.NoError(t, f.keeper.SetParams(ctx, params))
		paused := authViewStores(ctx, f.hostKey, f.harness.key)
		result, err := f.keeper.GrantSessionSuccessor(ctx, actor, nil, expiry, action, successor.PredecessorID, successor.ExpectedCommitment, idempotency)
		require.ErrorIs(t, err, govtypes.ErrPausedScope)
		require.Contains(t, err.Error(), "layerxauth.grantSessionSuccessor")
		require.Equal(t, Result{}, result)
		require.Equal(t, paused, authViewStores(ctx, f.hostKey, f.harness.key))
	})

	branch, _ := f.ctx.CacheContext()
	want, err := f.keeper.execute(branch, actor, authSessionWire(body, expiry, action, &successor), idempotency)
	require.ErrorIs(t, err, types.ErrAuthScope)
	refused, err := f.keeper.GrantSessionSuccessor(f.ctx, actor, body, expiry, action, successor.PredecessorID, successor.ExpectedCommitment, idempotency)
	require.ErrorIs(t, err, types.ErrAuthScope)
	require.Equal(t, lxphost.Code(-204), refused.ResultCode)
	require.ErrorIs(t, err, FromKernel(refused.ResultCode))
	require.Equal(t, want.Encoded, refused.Encoded)
	require.NotEmpty(t, refused.Encoded)
	require.Empty(t, refused.Events)
	require.Equal(t, uint64(44), refused.GlobalSequence)
	stored, err := f.keeper.ResultByIdempotency(f.ctx, actor.DIDKey, idempotency)
	require.NoError(t, err)
	require.Equal(t, authExecuteSignedReceipt(t, refused.Receipt), stored)
	after, err := f.host.ReadGrant(f.ctx, f.sessionID)
	require.NoError(t, err)
	require.Equal(t, predecessor, after)
	again, err := f.host.ReadSessionKey(f.ctx, f.sessionID)
	require.NoError(t, err)
	require.Equal(t, summary, again)
	live, err = f.host.ReadIsLive(f.ctx, f.sessionID, refused.GlobalSequence)
	require.NoError(t, err)
	require.True(t, live)
	_, err = f.host.ReadGrant(f.ctx, grantID)
	require.Equal(t, lxphost.Code(-7), err)
	_, err = f.host.ReadSessionKey(f.ctx, grantID)
	require.Equal(t, lxphost.Code(-7), err)
	require.Equal(t, before[1], authViewStores(f.ctx, f.harness.key)[0])
}
