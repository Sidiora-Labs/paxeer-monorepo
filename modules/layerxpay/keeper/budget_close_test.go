package keeper

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/stretchr/testify/require"
)

func budgetExitFixture(t *testing.T) (*payHarness, [32]byte, [32]byte, [32]byte, [32]byte) {
	t.Helper()
	h := newPayHarness(t)
	id := [32]byte{60}
	escrow := paySubmitFundedEscrow(t, h, id)
	var owner, asset, recipient [32]byte
	copy(owner[:], escrow[32:64])
	copy(asset[:], escrow[160:192])
	copy(recipient[:], escrow[96:128])
	account, err := codec.DeriveAccountID([]byte("agent:" + string(h.actors[0].did) + ":budget:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	h.keeper.SetParams(h.ctx, types.Params{BudgetEnabled: true})
	enableKey, err := types.KernelModuleEnableKey(types.BudgetModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(enableKey, payGateEnabledRecord())
	payload, err := types.EncodeBudgetCreate(id, account, asset, [32]byte{61},
		codec.U128{Lo: 100}, codec.U128{}, codec.U128{Lo: 100},
		100000, uint64(payHarnessTimeMs), uint64(payHarnessTimeMs)+1000000, 7, 1)
	require.NoError(t, err)
	_, err = h.keeper.Submit(h.ctx, h.actors[0].address, "budgetCreate", payload, [32]byte{62})
	require.NoError(t, err)
	payload, err = types.EncodeBudgetSpend(id, recipient, codec.U128{Lo: 20})
	require.NoError(t, err)
	_, err = h.keeper.Submit(h.ctx, h.actors[0].address, "budgetSpend", payload, [32]byte{63})
	require.NoError(t, err)
	budgetExitBalance(t, h, owner, asset, "100")
	budgetExitBalance(t, h, account, asset, "80")
	budgetExitBalance(t, h, recipient, asset, "20")
	h.keeper.SetParams(h.ctx, types.Params{BudgetEnabled: true, Paused: true})
	_, err = h.keeper.Submit(h.ctx, h.actors[0].address, "budgetSpend", payload, [32]byte{64})
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	return h, id, owner, account, asset
}

func budgetExitBalance(t *testing.T, h *payHarness, account, asset [32]byte, want string) {
	t.Helper()
	balance, err := h.host.ReadBalance(h.ctx, account, asset)
	require.NoError(t, err)
	require.Equal(t, want, balance.String())
}

func budgetExitRecord(t *testing.T, h *payHarness, id [32]byte) types.BudgetRecord {
	t.Helper()
	raw := h.ctx.KVStore(h.hostKey).Get(types.KernelKVKey(types.BudgetModuleID, append([]byte("budget:"), id[:]...)))
	record, err := types.DecodeBudgetRecord(raw)
	require.NoError(t, err)
	require.Equal(t, id, record.BudgetID)
	return record
}

func budgetExitExpected(t *testing.T, h *payHarness, activityType uint32, payload []byte, key [32]byte) (lxphost.Receipt, map[string][]byte) {
	t.Helper()
	actor := h.actors[0]
	last, err := h.host.ReadLastSequence(h.ctx, actor.did)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	_, principal, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	authority := actor.authority()
	authority.Bound, authority.Principal = true, principal
	branch, _ := h.ctx.CacheContext()
	now := uint64(h.ctx.BlockTime().UnixMilli())
	receipt, err := h.host.HostExecute(lxphost.WithSequence(branch, last+1), lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: now, NotAfter: now + h.host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}, authority)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	return receipt, payStoreSnapshot(branch, h.hostKey)
}

func TestBudgetCloseDrainsToOwner(t *testing.T) {
	h, id, owner, account, asset := budgetExitFixture(t)
	actor := h.actors[0]
	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	for _, sequence := range []uint64{6, 7} {
		receipt, err := h.keeper.BudgetClose(h.ctx, actor.address, id, sequence, [32]byte{65})
		require.ErrorIs(t, err, lxphost.Code(-212))
		require.Equal(t, lxphost.Code(-212), receipt.ResultCode)
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	}
	_, err := h.keeper.BudgetClose(h.ctx, h.actors[1].address, id, 19, [32]byte{66})
	require.Error(t, err)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	for _, invalid := range []struct {
		id       [32]byte
		sequence uint64
	}{{[32]byte{}, 19}, {id, 0}} {
		receipt, err := h.keeper.BudgetClose(h.ctx, actor.address, invalid.id, invalid.sequence, [32]byte{67})
		require.EqualError(t, err, "LXP_NON_CANONICAL")
		require.Equal(t, lxphost.Receipt{}, receipt)
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	}
	key := [32]byte{68}
	payload := binary.BigEndian.AppendUint64(append([]byte{0, 1}, id[:]...), 19)
	require.Len(t, payload, 42)
	encoded, err := types.EncodeBudgetClose(id, 19)
	require.NoError(t, err)
	require.Equal(t, payload, encoded)
	expected, expectedStore := budgetExitExpected(t, h, 0x00030007, payload, key)
	receipt, err := h.keeper.BudgetClose(h.ctx, actor.address, id, 19, key)
	require.NoError(t, err)
	require.Equal(t, expected, receipt)
	require.Equal(t, expectedStore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	budgetExitBalance(t, h, owner, asset, "180")
	budgetExitBalance(t, h, account, asset, "0")
	record := budgetExitRecord(t, h, id)
	require.True(t, record.Closed)
	require.False(t, record.Revoked)
	require.Equal(t, uint64(19), record.RevocationSequence)
	before := payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.BudgetClose(h.ctx, actor.address, id, 20, [32]byte{69})
	require.ErrorIs(t, err, lxphost.Code(-7))
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
}

func TestBudgetDefundAndRevoke(t *testing.T) {
	h, id, owner, account, asset := budgetExitFixture(t)
	actor := h.actors[0]
	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	for _, invalid := range []struct {
		id      [32]byte
		amount  codec.U128
		message string
	}{{[32]byte{}, codec.U128{Lo: 30}, "LXP_NON_CANONICAL"}, {id, codec.U128{}, "LXP_INVALID_AMOUNT"}} {
		receipt, err := h.keeper.BudgetDefund(h.ctx, actor.address, invalid.id, invalid.amount, [32]byte{70})
		require.EqualError(t, err, invalid.message)
		require.Equal(t, lxphost.Receipt{}, receipt)
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	}
	_, err := h.keeper.BudgetDefund(h.ctx, h.actors[1].address, id, codec.U128{Lo: 30}, [32]byte{71})
	require.Error(t, err)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	receipt, err := h.keeper.BudgetDefund(h.ctx, actor.address, id, codec.U128{Lo: 81}, [32]byte{72})
	require.ErrorIs(t, err, lxphost.Code(-714))
	require.Equal(t, lxphost.Code(-714), receipt.ResultCode)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	key := [32]byte{73}
	amount := codec.U128{Lo: 30}.Bytes()
	payload := append(append([]byte{0, 1}, id[:]...), amount[:]...)
	require.Len(t, payload, 50)
	encoded, err := types.EncodeBudgetDefund(id, codec.U128{Lo: 30})
	require.NoError(t, err)
	require.Equal(t, payload, encoded)
	expected, expectedStore := budgetExitExpected(t, h, 0x00030008, payload, key)
	receipt, err = h.keeper.BudgetDefund(h.ctx, actor.address, id, codec.U128{Lo: 30}, key)
	require.NoError(t, err)
	require.Equal(t, expected, receipt)
	require.Equal(t, expectedStore, payStoreSnapshot(h.ctx, h.hostKey))
	budgetExitBalance(t, h, owner, asset, "130")
	budgetExitBalance(t, h, account, asset, "50")
	record := budgetExitRecord(t, h, id)
	require.Equal(t, codec.U128{Lo: 70}, record.PerPeriodLimit)
	require.Equal(t, codec.U128{Lo: 20}, record.SpentThisPeriod)
	require.Equal(t, uint64(7), record.RevocationSequence)
	require.False(t, record.Closed)
	require.False(t, record.Revoked)
	hostBefore = payStoreSnapshot(h.ctx, h.hostKey)
	for _, invalid := range []struct {
		id       [32]byte
		sequence uint64
	}{{[32]byte{}, 23}, {id, 0}} {
		receipt, err := h.keeper.BudgetRevoke(h.ctx, actor.address, invalid.id, invalid.sequence, [32]byte{74})
		require.EqualError(t, err, "LXP_NON_CANONICAL")
		require.Equal(t, lxphost.Receipt{}, receipt)
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	}
	for _, sequence := range []uint64{6, 7} {
		receipt, err := h.keeper.BudgetRevoke(h.ctx, actor.address, id, sequence, [32]byte{75})
		require.ErrorIs(t, err, lxphost.Code(-212))
		require.Equal(t, lxphost.Code(-212), receipt.ResultCode)
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	}
	_, err = h.keeper.BudgetRevoke(h.ctx, h.actors[1].address, id, 23, [32]byte{76})
	require.Error(t, err)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	key = [32]byte{77}
	payload = binary.BigEndian.AppendUint64(append([]byte{0, 1}, id[:]...), 23)
	require.Len(t, payload, 42)
	encoded, err = types.EncodeBudgetRevoke(id, 23)
	require.NoError(t, err)
	require.Equal(t, payload, encoded)
	expected, expectedStore = budgetExitExpected(t, h, 0x00030009, payload, key)
	receipt, err = h.keeper.BudgetRevoke(h.ctx, actor.address, id, 23, key)
	require.NoError(t, err)
	require.Equal(t, expected, receipt)
	require.Equal(t, expectedStore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	budgetExitBalance(t, h, owner, asset, "180")
	budgetExitBalance(t, h, account, asset, "0")
	record = budgetExitRecord(t, h, id)
	require.True(t, record.Revoked)
	require.False(t, record.Closed)
	require.Equal(t, uint64(23), record.RevocationSequence)
	require.Equal(t, codec.U128{Lo: 20}, record.PerPeriodLimit)
	before := payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.BudgetRevoke(h.ctx, actor.address, id, 24, [32]byte{78})
	require.ErrorIs(t, err, lxphost.Code(-716))
	_, err = h.keeper.BudgetDefund(h.ctx, actor.address, id, codec.U128{Lo: 1}, [32]byte{79})
	require.ErrorIs(t, err, lxphost.Code(-716))
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
}
