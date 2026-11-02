package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgramsReplayedActivityReturnsStoredReceipt(t *testing.T) {
	h, args, payload := programsCallFixture(t)
	ctx := h.stateDB.Ctx()
	authority, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	const activityType uint32 = 0x00090003

	feeSchedule, err := h.host.ReadFeeSchedule(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(feeSchedule), 97)
	asset := [32]byte(feeSchedule[65:97])
	_, payer, err := lxphost.AssetAccountID(h.publicKey, asset)
	require.NoError(t, err)
	balanceBefore, err := h.host.ReadBalance(ctx, payer, asset)
	require.NoError(t, err)
	require.True(t, balanceBefore.IsPositive())
	identityBefore, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	globalBefore, err := h.host.ReadLastSequence(ctx, did)
	require.NoError(t, err)

	timestamp := uint64(ctx.BlockTime().UnixMilli())
	idempotency := binary.BigEndian.AppendUint32(nil, activityType)
	idempotency = binary.BigEndian.AppendUint64(idempotency, identityBefore)
	idempotency = append(idempotency, payload...)
	var feeLimit [16]byte
	args[8].(*big.Int).FillBytes(feeLimit[:])
	act := lxphost.Activity{
		ProtocolVersion: codec.StateCommitmentProtocolVersion,
		Type:            activityType,
		ActorDID:        []byte("did:layerx:" + hex.EncodeToString(authority.VerifiedKey[:])),
		AuthorityKey:    authority.VerifiedKey,
		AccountSequence: identityBefore,
		NotBefore:       timestamp,
		NotAfter:        timestamp,
		IdempotencyKey:  sha256.Sum256(idempotency),
		FeeLimit:        feeLimit,
		Payload:         payload,
	}
	first, err := h.host.Execute(lxphost.WithSequence(ctx, globalBefore+1), act, authority)
	require.NoError(t, err)
	require.Zero(t, first.ResultCode)
	require.NotNil(t, first.ProgramOutcome)
	require.Zero(t, first.ProgramOutcome.Status)
	require.True(t, first.FeeCharged.IsPositive())
	require.NotEmpty(t, first.Encoded)
	firstEncoded := bytes.Clone(first.Encoded)
	decoded, err := codec.DecodeReceipt(first.Encoded)
	require.NoError(t, err)
	require.Equal(t, first.Encoded, decoded.UnsignedBytes())
	require.Equal(t, int32(first.ResultCode), decoded.ResultCode)
	require.Equal(t, first.GlobalSequence, decoded.GlobalSequence)
	require.Equal(t, first.Timestamp, decoded.Timestamp)
	require.Equal(t, first.PreviousStateRoot, decoded.PreviousStateRoot)
	require.Equal(t, first.ResultingStateRoot, decoded.ResultingStateRoot)

	balanceAfterFirst, err := h.host.ReadBalance(ctx, payer, asset)
	require.NoError(t, err)
	assert.Equal(t, first.FeeCharged, balanceBefore.Sub(balanceAfterFirst), "first call must debit exactly one kernel fee")
	identityAfterFirst, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	globalAfterFirst, err := h.host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	assert.Equal(t, identityBefore+1, identityAfterFirst)
	assert.Equal(t, globalBefore+1, globalAfterFirst)
	assert.Equal(t, globalAfterFirst, first.GlobalSequence)
	stateAfterFirst := h.dump(ctx)

	last, err := h.host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	require.Equal(t, globalAfterFirst, last)
	replayed, replayErr := h.host.Execute(lxphost.WithSequence(ctx, last+1), act, authority)
	assert.ErrorIs(t, replayErr, lxphost.Code(-301), "resubmitting the executed actor sequence must be refused")
	assert.EqualError(t, replayErr, "LXP_ERR_SEQUENCE_REUSED")
	assert.Equal(t, "LXP_ERR_SEQUENCE_REUSED", lxphost.Code(-301).Error())
	assert.True(t, replayed.FeeCharged.IsNil() || replayed.FeeCharged.IsZero(), "sequence refusal must not charge a fee")
	assert.Equal(t, stateAfterFirst, h.dump(ctx), "replay must leave every persistent kernel row unchanged")
	assert.Equal(t, firstEncoded, first.Encoded, "refusal must preserve the executed call's receipt bytes")
	assert.Equal(t, firstEncoded, decoded.UnsignedBytes())
	identityAfterReplay, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	globalAfterReplay, err := h.host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	assert.Equal(t, identityAfterFirst, identityAfterReplay, "replay must not execute a second activity")
	assert.Equal(t, globalAfterFirst, globalAfterReplay)
	balanceAfterReplay, err := h.host.ReadBalance(ctx, payer, asset)
	require.NoError(t, err)
	assert.Equal(t, balanceAfterFirst, balanceAfterReplay)
	assert.Equal(t, first.FeeCharged, balanceBefore.Sub(balanceAfterReplay))
}
