package keeper

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func openStreamLifecycle(t *testing.T, h *payHarness) types.StreamOpen {
	t.Helper()
	h.keeper.SetParams(h.ctx, types.Params{StreamEnabled: true})
	gate, err := types.KernelModuleEnableKey(types.StreamModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(gate, payGateEnabledRecord())
	actor := h.actors[0]
	salt := [32]byte{0x73}
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor.id[:]...), salt[:]...))
	main := paySubmitMain(t, h, actor, asset)
	beneficiary := paySubmitMain(t, h, h.actors[1], asset)
	assetName, assetAccount, err := lxphost.AssetAccountID(actor.public, asset)
	require.NoError(t, err)
	payload := append([]byte{0, 1}, asset[:]...)
	payload = append(payload, salt[:]...)
	payload = append(payload, 1, 'T', 1, 'T', 6)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 1000)
	payload = append(payload, 1, 0)
	paySubmitExecute(t, h, actor, 0x00010001, payload, [32]byte{40})
	paySubmitExecute(t, h, actor, 0x00010004, append([]byte{0, 1}, asset[:]...), [32]byte{41})
	payload = append(append([]byte{0, 1}, asset[:]...), assetAccount[:]...)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 200)
	paySubmitExecute(t, h, actor, 0x0001000a, payload, [32]byte{42})

	var amount [16]byte
	binary.BigEndian.PutUint64(amount[8:], 200)
	key := [32]byte{0x74}
	material := append(append(append(append([]byte(nil), assetAccount[:]...), main[:]...), asset[:]...), amount[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(material, key[:]...))
	require.NoError(t, err)
	account, err := h.host.ReadAccount(h.ctx, assetName, assetAccount)
	require.NoError(t, err)
	transfer := binary.BigEndian.AppendUint64(material, account.NextSequence)
	transfer = append(transfer, key[:]...)
	transfer = binary.BigEndian.AppendUint64(transfer, uint64(payHarnessTimeMs)+10000)
	transfer = append(transfer, contextHash[:]...)
	transfer = append(transfer, 0)
	suffix := binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint32(nil, 7), 3)
	message := append([]byte{0x53, 1}, transfer...)
	message = append(message, 1)
	message = append(message, assetAccount[:]...)
	message = append(message, contextHash[:]...)
	message = append(message, suffix...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	payload = append([]byte{0x53, 1, 0, 10}, transfer...)
	payload = append(payload, 1)
	payload = append(payload, assetAccount[:]...)
	payload = append(payload, actor.public[:]...)
	payload = append(payload, ed25519.Sign(actor.private, digest[:])...)
	payload = append(payload, contextHash[:]...)
	payload = append(payload, suffix...)
	paySubmitExecute(t, h, actor, 0x00010005, payload, key)
	balance, err := h.host.ReadBalance(h.ctx, main, asset)
	require.NoError(t, err)
	require.Equal(t, "200", balance.String())

	id := [32]byte{50}
	streamAccount, err := codec.DeriveAccountID([]byte("agent:" + string(actor.did) + ":stream:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	open := types.StreamOpen{
		StreamID: id, StreamAccount: streamAccount, Recipient: beneficiary, AssetID: asset,
		Mode: types.StreamModeTime, Rate: codec.U128{Lo: 2}, RateUnit: 1000,
		StartTimestamp: uint64(payHarnessTimeMs), TotalCap: codec.U128{Lo: 1000},
		InitialFunding: codec.U128{Lo: 200},
	}
	payload, err = types.EncodeStreamOpen(open)
	require.NoError(t, err)
	receipt, err := h.keeper.Submit(h.ctx, actor.address, "streamOpen", payload, [32]byte{51})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	balance, err = h.host.ReadBalance(h.ctx, streamAccount, asset)
	require.NoError(t, err)
	require.Equal(t, "200", balance.String())
	return open
}

func streamLifecycleExpected(t *testing.T, h *payHarness, activityType uint32, payload []byte, key [32]byte) lxphost.Receipt {
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
	receipt, err := h.host.HostExecute(lxphost.WithSequence(branch, last+1), lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: uint64(h.ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(h.ctx.BlockTime().UnixMilli()) + h.host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}, authority)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	return receipt
}

func TestStreamPauseResumeStopsAccrual(t *testing.T) {
	h := newPayHarness(t)
	open := openStreamLifecycle(t, h)
	caller := h.actors[0].address
	payload, err := types.EncodeStreamID(open.StreamID)
	require.NoError(t, err)
	require.Len(t, payload, 34)
	require.Equal(t, append([]byte{0, 1}, open.StreamID[:]...), payload)

	h.ctx = h.ctx.WithBlockHeight(2).WithBlockTime(time.UnixMilli(payHarnessTimeMs + 5000))
	h.keeper.SetParams(h.ctx, types.Params{StreamEnabled: true, Paused: true})
	pauseKey := [32]byte{52}
	expected := streamLifecycleExpected(t, h, 0x00040005, payload, pauseKey)
	paused, err := h.keeper.StreamPause(h.ctx, caller, open.StreamID, pauseKey)
	require.NoError(t, err)
	require.Equal(t, expected, paused)
	record, err := h.keeper.GetStream(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.True(t, record.Paused)
	require.Equal(t, codec.U128{Lo: 10}, record.AccruedTotal)
	require.Equal(t, codec.U128{}, record.SettledTotal)
	require.Equal(t, uint64(payHarnessTimeMs+5000), record.LastAccrualTimestamp)

	h.ctx = h.ctx.WithBlockHeight(3).WithBlockTime(time.UnixMilli(payHarnessTimeMs + 25000))
	accrued, unsettled, err := h.keeper.StreamAccrued(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.Equal(t, "10", accrued.String())
	require.Equal(t, "10", unsettled.String())
	before := payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.StreamResume(h.ctx, caller, open.StreamID, [32]byte{53})
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))

	h.keeper.SetParams(h.ctx, types.Params{StreamEnabled: true})
	_, err = h.keeper.StreamResume(h.ctx, h.actors[1].address, open.StreamID, [32]byte{54})
	require.Error(t, err)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	resumeKey := [32]byte{55}
	expected = streamLifecycleExpected(t, h, 0x00040006, payload, resumeKey)
	resumed, err := h.keeper.StreamResume(h.ctx, caller, open.StreamID, resumeKey)
	require.NoError(t, err)
	require.Equal(t, expected, resumed)
	require.Equal(t, paused.GlobalSequence+1, resumed.GlobalSequence)
	record, err = h.keeper.GetStream(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.False(t, record.Paused)
	require.Equal(t, codec.U128{Lo: 10}, record.AccruedTotal)
	require.Equal(t, uint64(payHarnessTimeMs+25000), record.LastAccrualTimestamp)
	accrued, unsettled, err = h.keeper.StreamAccrued(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.Equal(t, "10", accrued.String())
	require.Equal(t, "10", unsettled.String())
	h.ctx = h.ctx.WithBlockHeight(4).WithBlockTime(time.UnixMilli(payHarnessTimeMs + 28000))
	accrued, unsettled, err = h.keeper.StreamAccrued(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.Equal(t, "16", accrued.String())
	require.Equal(t, "16", unsettled.String())
	balance, err := h.host.ReadBalance(h.ctx, open.StreamAccount, open.AssetID)
	require.NoError(t, err)
	require.Equal(t, "200", balance.String())

	before = payStoreSnapshot(h.ctx, h.hostKey)
	for _, call := range []func() (lxphost.Receipt, error){
		func() (lxphost.Receipt, error) { return h.keeper.StreamPause(h.ctx, caller, [32]byte{}, [32]byte{56}) },
		func() (lxphost.Receipt, error) { return h.keeper.StreamResume(h.ctx, caller, [32]byte{}, [32]byte{57}) },
	} {
		receipt, err := call()
		require.EqualError(t, err, "LXP_NON_CANONICAL")
		require.Equal(t, lxphost.Receipt{}, receipt)
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	}
}

func TestStreamRateChangeSettlesAtOldRate(t *testing.T) {
	h := newPayHarness(t)
	open := openStreamLifecycle(t, h)
	caller := h.actors[0].address
	newRate, key := codec.U128{Lo: 5}, [32]byte{60}
	payload, err := types.EncodeStreamRateChange(open.StreamID, newRate, key)
	require.NoError(t, err)
	require.Len(t, payload, 82)
	require.Equal(t, []byte{0, 1}, payload[:2])
	require.Equal(t, open.StreamID[:], payload[2:34])
	rateBytes := newRate.Bytes()
	require.Equal(t, rateBytes[:], payload[34:50])
	require.Equal(t, key[:], payload[50:82])
	require.Equal(t, types.KernelActivity{Type: 0x00040008, Module: types.StreamModuleID, PayloadBytes: 82}, types.MethodActivity["streamRateChange"])

	h.ctx = h.ctx.WithBlockHeight(2).WithBlockTime(time.UnixMilli(payHarnessTimeMs + 5000))
	before := payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.StreamRateChange(h.ctx, h.actors[1].address, open.StreamID, newRate, key)
	require.Error(t, err)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	h.keeper.SetParams(h.ctx, types.Params{StreamEnabled: true, Paused: true})
	_, err = h.keeper.StreamRateChange(h.ctx, caller, open.StreamID, newRate, key)
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	h.keeper.SetParams(h.ctx, types.Params{})
	_, err = h.keeper.StreamRateChange(h.ctx, caller, open.StreamID, newRate, key)
	require.ErrorIs(t, err, types.ErrFamilyDisabled)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	h.keeper.SetParams(h.ctx, types.Params{StreamEnabled: true})
	_, err = h.keeper.StreamRateChange(h.ctx, common.Address{}, open.StreamID, newRate, key)
	require.ErrorIs(t, err, types.ErrUnboundCaller)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))

	expected := streamLifecycleExpected(t, h, 0x00040008, payload, key)
	receipt, err := h.keeper.StreamRateChange(h.ctx, caller, open.StreamID, newRate, key)
	require.NoError(t, err)
	require.Equal(t, expected, receipt)
	require.Equal(t, uint64(payHarnessTimeMs+5000), receipt.Timestamp)
	record, err := h.keeper.GetStream(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.Equal(t, newRate, record.Rate)
	require.Equal(t, codec.U128{Lo: 10}, record.AccruedTotal)
	require.Equal(t, codec.U128{Lo: 10}, record.SettledTotal)
	require.Equal(t, uint64(payHarnessTimeMs+5000), record.LastAccrualTimestamp)
	result, err := h.keeper.GetStreamResult(h.ctx, key)
	require.NoError(t, err)
	require.Equal(t, open.StreamID, result.StreamID)
	require.Equal(t, uint16(8), result.Ordinal)
	require.Equal(t, codec.U128{Lo: 10}, result.Paid)
	require.Equal(t, codec.U128{}, result.Refunded)
	require.Equal(t, uint8(1), result.LegCount)
	require.NotEqual(t, [32]byte{}, result.TransferSetRoot)
	for _, tc := range []struct {
		account [32]byte
		amount  string
	}{
		{open.StreamAccount, "190"}, {open.Recipient, "10"},
	} {
		balance, err := h.host.ReadBalance(h.ctx, tc.account, open.AssetID)
		require.NoError(t, err)
		require.Equal(t, tc.amount, balance.String())
	}
	h.ctx = h.ctx.WithBlockHeight(3).WithBlockTime(time.UnixMilli(payHarnessTimeMs + 8000))
	accrued, unsettled, err := h.keeper.StreamAccrued(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.Equal(t, "25", accrued.String())
	require.Equal(t, "15", unsettled.String())

	before = payStoreSnapshot(h.ctx, h.hostKey)
	for _, tc := range []struct {
		id   [32]byte
		rate codec.U128
		key  [32]byte
	}{
		{[32]byte{}, newRate, [32]byte{61}},
		{open.StreamID, codec.U128{}, [32]byte{62}},
		{open.StreamID, newRate, [32]byte{}},
	} {
		receipt, err := h.keeper.StreamRateChange(h.ctx, caller, tc.id, tc.rate, tc.key)
		require.EqualError(t, err, "LXP_NON_CANONICAL")
		require.Equal(t, lxphost.Receipt{}, receipt)
		require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	}
}
