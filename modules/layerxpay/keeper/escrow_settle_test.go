package keeper

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/stretchr/testify/require"
)

func TestEscrowReleaseAndTimeout(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		t.Run(fmt.Sprintf("timeout_%t", timeout), func(t *testing.T) {
			h := newPayHarness(t)
			id, key := [32]byte{100}, [32]byte{101}
			openPayload := paySubmitFundedEscrow(t, h, id)
			expiry := uint64(payHarnessTimeMs + 1000)
			binary.BigEndian.PutUint64(openPayload[208:216], expiry)
			opened, err := h.keeper.Submit(h.ctx, h.actors[0].address, "escrowOpen", openPayload, [32]byte{102})
			require.NoError(t, err)
			require.Zero(t, opened.ResultCode)
			record, found := h.keeper.GetEscrow(h.ctx, id)
			require.True(t, found)
			require.Equal(t, codec.U128{Lo: 50}, record.LockedAmount)
			escrowSettleBalance(t, h, record.Owner, record.AssetID, 150)
			escrowSettleBalance(t, h, record.EscrowAccount, record.AssetID, 50)

			caller := h.actors[0]
			activityType := uint32(0x00020004)
			ordinal, state := uint16(4), uint8(4)
			if timeout {
				caller = h.actors[1]
				activityType, ordinal, state = 0x00020005, 5, 7
				h.ctx = h.ctx.WithBlockTime(time.UnixMilli(int64(expiry - 1)))
				escrowSettleRefusal(t, h, -304, "LXP_NOT_YET_VALID", func() (lxphost.Receipt, error) {
					return h.keeper.EscrowTimeout(h.ctx, caller.address, id, key)
				})
				h.ctx = h.ctx.WithBlockTime(time.UnixMilli(int64(expiry)))
			} else {
				escrowSettleRefusal(t, h, -207, "LXP_UNAUTHORIZED_DEBIT", func() (lxphost.Receipt, error) {
					return h.keeper.EscrowRelease(h.ctx, h.actors[1].address, id, key)
				})
			}

			payload := make([]byte, 64)
			copy(payload[:32], id[:])
			copy(payload[32:], key[:])
			expected := escrowSettleExpected(t, h, caller, activityType, payload, key)
			var settled lxphost.Receipt
			if timeout {
				settled, err = h.keeper.EscrowTimeout(h.ctx, caller.address, id, key)
			} else {
				settled, err = h.keeper.EscrowRelease(h.ctx, caller.address, id, key)
			}
			require.NoError(t, err)
			require.Equal(t, expected, settled)
			record, found = h.keeper.GetEscrow(h.ctx, id)
			require.True(t, found)
			require.Equal(t, state, record.State)
			require.Equal(t, codec.U128{}, record.LockedAmount)
			require.Equal(t, codec.U128{}, record.CapturedAmount)
			result, found := h.keeper.GetEscrowResult(h.ctx, key)
			require.True(t, found)
			require.Equal(t, id, result.EscrowID)
			require.Equal(t, ordinal, result.Ordinal)
			require.Equal(t, state, result.StateAfter)
			require.Equal(t, record.EscrowAccount, result.From)
			require.Equal(t, record.Owner, result.To)
			require.Equal(t, codec.U128{Lo: 50}, result.Amount)
			require.Equal(t, settled.GlobalSequence, result.GlobalSequence)
			require.Equal(t, uint64(h.ctx.BlockTime().UnixMilli()), result.TimestampMs)
			escrowSettleBalance(t, h, record.Owner, record.AssetID, 200)
			escrowSettleBalance(t, h, record.Beneficiary, record.AssetID, 0)
			escrowSettleBalance(t, h, record.EscrowAccount, record.AssetID, 0)
			escrowSettleRefusal(t, h, -700, "LXP_ESCROW_STATE", func() (lxphost.Receipt, error) {
				if timeout {
					return h.keeper.EscrowTimeout(h.ctx, caller.address, id, [32]byte{103})
				}
				return h.keeper.EscrowRelease(h.ctx, caller.address, id, [32]byte{103})
			})
		})
	}
}

func TestEscrowDisputeResolveSplit(t *testing.T) {
	for _, bps := range []uint32{0, 3333, 10000} {
		t.Run(fmt.Sprintf("bps_%d", bps), func(t *testing.T) {
			h := newPayHarness(t)
			arbiter := h.registerActor(t, 3)
			id, openKey, resolveKey := [32]byte{110}, [32]byte{111}, [32]byte{112}
			payload := paySubmitFundedEscrow(t, h, id)
			var asset [32]byte
			copy(asset[:], payload[160:192])
			arbiterAccount := paySubmitMain(t, h, arbiter, asset)
			copy(payload[128:160], arbiterAccount[:])
			binary.BigEndian.PutUint64(payload[216:224], uint64(payHarnessTimeMs+10000))
			opened, err := h.keeper.Submit(h.ctx, h.actors[0].address, "escrowOpen", payload, [32]byte{113})
			require.NoError(t, err)
			require.Zero(t, opened.ResultCode)
			record, found := h.keeper.GetEscrow(h.ctx, id)
			require.True(t, found)
			require.Equal(t, arbiterAccount, record.Arbiter)
			escrowSettleRefusal(t, h, -207, "LXP_UNAUTHORIZED_DEBIT", func() (lxphost.Receipt, error) {
				return h.keeper.EscrowDisputeOpen(h.ctx, arbiter.address, id, openKey)
			})
			expected := escrowSettleExpected(t, h, h.actors[1], 0x00020006, id[:], openKey)
			disputed, err := h.keeper.EscrowDisputeOpen(h.ctx, h.actors[1].address, id, openKey)
			require.NoError(t, err)
			require.Equal(t, expected, disputed)
			record, found = h.keeper.GetEscrow(h.ctx, id)
			require.True(t, found)
			require.Equal(t, uint8(5), record.State)
			require.Equal(t, codec.U128{Lo: 50}, record.LockedAmount)
			escrowSettleBalance(t, h, record.Owner, asset, 150)
			escrowSettleBalance(t, h, record.Beneficiary, asset, 0)
			escrowSettleBalance(t, h, record.EscrowAccount, asset, 50)
			escrowSettleRefusal(t, h, -712, "LXP_HOLD_DISPUTED", func() (lxphost.Receipt, error) {
				return h.keeper.EscrowRelease(h.ctx, h.actors[0].address, id, [32]byte{114})
			})
			escrowSettleRefusal(t, h, -207, "LXP_UNAUTHORIZED_DEBIT", func() (lxphost.Receipt, error) {
				return h.keeper.EscrowDisputeResolve(h.ctx, h.actors[0].address, id, bps, resolveKey)
			})
			escrowSettleRefusal(t, h, -3, "LXP_NON_CANONICAL", func() (lxphost.Receipt, error) {
				return h.keeper.EscrowDisputeResolve(h.ctx, arbiter.address, id, 10001, resolveKey)
			})

			resolvePayload := make([]byte, 68)
			copy(resolvePayload[:32], id[:])
			binary.BigEndian.PutUint32(resolvePayload[32:36], bps)
			copy(resolvePayload[36:], resolveKey[:])
			expected = escrowSettleExpected(t, h, arbiter, 0x00020007, resolvePayload, resolveKey)
			resolved, err := h.keeper.EscrowDisputeResolve(h.ctx, arbiter.address, id, bps, resolveKey)
			require.NoError(t, err)
			require.Equal(t, expected, resolved)
			beneficiaryAmount := uint64(50) * uint64(bps) / 10000
			ownerAmount := uint64(50) - beneficiaryAmount
			record, found = h.keeper.GetEscrow(h.ctx, id)
			require.True(t, found)
			require.Equal(t, uint8(6), record.State)
			require.Equal(t, codec.U128{}, record.LockedAmount)
			require.Equal(t, codec.U128{Lo: beneficiaryAmount}, record.CapturedAmount)
			result, found := h.keeper.GetEscrowResult(h.ctx, resolveKey)
			require.True(t, found)
			require.Equal(t, uint16(7), result.Ordinal)
			require.Equal(t, uint8(6), result.StateAfter)
			require.Equal(t, codec.U128{Lo: beneficiaryAmount}, result.Amount)
			require.Equal(t, codec.U128{Lo: ownerAmount}, result.SecondaryAmount)
			require.Equal(t, record.EscrowAccount, result.From)
			require.Equal(t, record.Beneficiary, result.To)
			require.Equal(t, resolved.GlobalSequence, result.GlobalSequence)
			escrowSettleBalance(t, h, record.Owner, asset, 150+ownerAmount)
			escrowSettleBalance(t, h, record.Beneficiary, asset, beneficiaryAmount)
			escrowSettleBalance(t, h, record.EscrowAccount, asset, 0)
			escrowSettleBalance(t, h, arbiterAccount, asset, 0)
		})
	}
}

func escrowSettleExpected(t *testing.T, h *payHarness, actor payActor, activityType uint32, payload []byte, key [32]byte) lxphost.Receipt {
	t.Helper()
	last, err := h.host.ReadLastSequence(h.ctx, actor.did)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	_, principal, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	authority := actor.authority()
	authority.Bound, authority.Principal = true, principal
	branch, _ := h.ctx.CacheContext()
	expected, err := h.host.HostExecute(lxphost.WithSequence(branch, last+1), lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: uint64(h.ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(h.ctx.BlockTime().UnixMilli()) + h.host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}, authority)
	require.NoError(t, err)
	require.Zero(t, expected.ResultCode)
	return expected
}

func escrowSettleBalance(t *testing.T, h *payHarness, account, asset [32]byte, want uint64) {
	t.Helper()
	balance, err := h.host.ReadBalance(h.ctx, account, asset)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprint(want), balance.String())
}

func escrowSettleRefusal(t *testing.T, h *payHarness, code lxphost.Code, name string, invoke func() (lxphost.Receipt, error)) {
	t.Helper()
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	receipt, err := invoke()
	require.Error(t, err)
	require.Equal(t, code, receipt.ResultCode)
	require.ErrorIs(t, err, code)
	require.Equal(t, name, pcommon.LXPCodeName(int32(code)))
	require.ErrorContains(t, err, name)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
}
