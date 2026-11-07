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

func newStreamSettleHarness(t *testing.T, mode uint8) (*payHarness, types.StreamOpen, [32]byte) {
	t.Helper()
	h := newPayHarness(t)
	h.actors = append(h.actors, h.registerActor(t, 3))
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

	id := [32]byte{0x81}
	custody, err := codec.DeriveAccountID([]byte("agent:" + string(actor.did) + ":stream:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	h.keeper.SetParams(h.ctx, types.Params{StreamEnabled: true})
	moduleKey, err := types.KernelModuleEnableKey(types.StreamModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(moduleKey, payGateEnabledRecord())
	open := types.StreamOpen{
		StreamID: id, StreamAccount: custody, Recipient: beneficiary, AssetID: asset,
		Mode: mode, Rate: codec.U128{Lo: 2}, RateUnit: 1000,
		StartTimestamp: uint64(payHarnessTimeMs), TotalCap: codec.U128{Lo: 1000},
		InitialFunding: codec.U128{Lo: 100},
	}
	if mode == types.StreamModeMetered {
		open.Rate, open.RateUnit = codec.U128{Lo: 3}, 2
		open.MeterAuthorities = [][32]byte{h.actors[1].public}
	}
	payload, err = types.EncodeStreamOpen(open)
	require.NoError(t, err)
	receipt, err := h.keeper.Submit(h.ctx, actor.address, "streamOpen", payload, [32]byte{0x82})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	requireStreamBalances(t, h, open, main, "100", "100", "0")
	return h, open, main
}

func requireStreamBalances(t *testing.T, h *payHarness, open types.StreamOpen, payer [32]byte, payerBalance, streamBalance, recipientBalance string) {
	t.Helper()
	for _, expected := range []struct {
		id      [32]byte
		balance string
	}{
		{payer, payerBalance}, {open.StreamAccount, streamBalance}, {open.Recipient, recipientBalance},
	} {
		balance, err := h.host.ReadBalance(h.ctx, expected.id, open.AssetID)
		require.NoError(t, err)
		require.Equal(t, expected.balance, balance.String())
	}
}

func requireStreamSubmission(t *testing.T, h *payHarness, actor payActor, activityType uint32, payload []byte, key [32]byte, submit func() (lxphost.Receipt, error)) lxphost.Receipt {
	t.Helper()
	last, err := h.host.ReadLastSequence(h.ctx, actor.did)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	_, principal, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	authority := actor.authority()
	authority.Principal, authority.Bound = principal, true
	expectedBranch, _ := h.ctx.CacheContext()
	expected, err := h.host.HostExecute(lxphost.WithSequence(expectedBranch, last+1), lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: uint64(h.ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(h.ctx.BlockTime().UnixMilli()) + h.host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}, authority)
	require.NoError(t, err)
	require.Zero(t, expected.ResultCode)
	receipt, err := submit()
	require.NoError(t, err)
	require.Equal(t, expected, receipt)
	require.Zero(t, receipt.ResultCode)
	require.Equal(t, uint64(h.ctx.BlockTime().UnixMilli()), receipt.Timestamp)
	require.Equal(t, last+1, receipt.GlobalSequence)
	require.Equal(t, payStoreSnapshot(expectedBranch, h.hostKey), payStoreSnapshot(h.ctx, h.hostKey))
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(4), decoded.ModuleID)
	after, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	require.Equal(t, sequence+1, after)
	return receipt
}

func TestStreamSettlePaysAccrued(t *testing.T) {
	h, open, payer := newStreamSettleHarness(t, types.StreamModeTime)
	outsider := h.actors[2]
	paySubmitMain(t, h, outsider, open.AssetID)
	h.ctx = h.ctx.WithBlockTime(time.UnixMilli(payHarnessTimeMs + 10500)).WithBlockHeight(2)
	before := payStoreSnapshot(h.ctx, h.hostKey)
	refused, err := h.keeper.StreamSettle(h.ctx, outsider.address, open.StreamID, [32]byte{0x83})
	require.ErrorIs(t, err, lxphost.Code(-207))
	require.Equal(t, lxphost.Code(-207), refused.ResultCode)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))

	key := [32]byte{0x84}
	payload, err := types.EncodeStreamKeyed(open.StreamID, key)
	require.NoError(t, err)
	require.Len(t, payload, 66)
	require.Equal(t, []byte{0, 1}, payload[:2])
	require.Equal(t, open.StreamID[:], payload[2:34])
	require.Equal(t, key[:], payload[34:66])
	recipient := h.actors[1]
	receipt := requireStreamSubmission(t, h, recipient, 0x00040004, payload, key, func() (lxphost.Receipt, error) {
		return h.keeper.StreamSettle(h.ctx, recipient.address, open.StreamID, key)
	})
	require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
	requireStreamBalances(t, h, open, payer, "100", "79", "21")
	record, err := h.keeper.GetStream(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.Equal(t, codec.U128{Lo: 21}, record.AccruedTotal)
	require.Equal(t, record.AccruedTotal, record.SettledTotal)
	require.Equal(t, uint64(payHarnessTimeMs+10500), record.LastAccrualTimestamp)
	result, err := h.keeper.GetStreamResult(h.ctx, key)
	require.NoError(t, err)
	require.Equal(t, uint16(4), result.Ordinal)
	require.Equal(t, open.StreamID, result.StreamID)
	require.Equal(t, codec.U128{Lo: 21}, result.Paid)
	require.Equal(t, codec.U128{}, result.Refunded)
	require.Equal(t, uint8(1), result.LegCount)
	require.NotEqual(t, [32]byte{}, result.TransferSetRoot)

	h.ctx = h.ctx.WithBlockTime(time.UnixMilli(payHarnessTimeMs + 15250)).WithBlockHeight(3)
	key = [32]byte{0x85}
	payload, err = types.EncodeStreamKeyed(open.StreamID, key)
	require.NoError(t, err)
	requireStreamSubmission(t, h, h.actors[0], 0x00040004, payload, key, func() (lxphost.Receipt, error) {
		return h.keeper.StreamSettle(h.ctx, h.actors[0].address, open.StreamID, key)
	})
	requireStreamBalances(t, h, open, payer, "100", "70", "30")
	record, err = h.keeper.GetStream(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.Equal(t, codec.U128{Lo: 30}, record.AccruedTotal)
	require.Equal(t, record.AccruedTotal, record.SettledTotal)
	require.Equal(t, codec.U128{Lo: 500}, record.RemainderCarry)
	result, err = h.keeper.GetStreamResult(h.ctx, key)
	require.NoError(t, err)
	require.Equal(t, codec.U128{Lo: 9}, result.Paid)

	before = payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.StreamSettle(h.ctx, h.actors[0].address, [32]byte{}, key)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	_, err = h.keeper.StreamSettle(h.ctx, h.actors[0].address, open.StreamID, [32]byte{})
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
}

func streamMeterSignature(t *testing.T, actor payActor, streamID [32]byte, reading uint64) [64]byte {
	t.Helper()
	message := append([]byte("LXP:STREAM:METER:v1"), streamID[:]...)
	message = binary.BigEndian.AppendUint64(message, reading)
	message = append(message, actor.public[:]...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(actor.private, digest[:]))
	return signature
}

func TestStreamMeterThenClose(t *testing.T) {
	h, open, payer := newStreamSettleHarness(t, types.StreamModeMetered)
	payerActor, meter := h.actors[0], h.actors[1]
	const reading uint64 = 15
	signature := streamMeterSignature(t, meter, open.StreamID, reading)
	key := [32]byte{0x90}
	payload, err := types.EncodeStreamMeter(open.StreamID, reading, meter.public, signature)
	require.NoError(t, err)
	require.Len(t, payload, 138)
	require.Equal(t, []byte{0, 1}, payload[:2])
	require.Equal(t, open.StreamID[:], payload[2:34])
	require.Equal(t, reading, binary.BigEndian.Uint64(payload[34:42]))
	require.Equal(t, meter.public[:], payload[42:74])
	require.Equal(t, signature[:], payload[74:138])
	before := payStoreSnapshot(h.ctx, h.hostKey)
	wrongSigner := streamMeterSignature(t, payerActor, open.StreamID, reading)
	refused, err := h.keeper.StreamMeter(h.ctx, payerActor.address, open.StreamID, reading, payerActor.public, wrongSigner, key)
	require.ErrorIs(t, err, lxphost.Code(-719))
	require.Equal(t, lxphost.Code(-719), refused.ResultCode)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	badSignature := signature
	badSignature[0] ^= 1
	refused, err = h.keeper.StreamMeter(h.ctx, payerActor.address, open.StreamID, reading, meter.public, badSignature, key)
	require.ErrorIs(t, err, lxphost.Code(-719))
	require.Equal(t, lxphost.Code(-719), refused.ResultCode)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	requireStreamSubmission(t, h, payerActor, 0x00040003, payload, key, func() (lxphost.Receipt, error) {
		return h.keeper.StreamMeter(h.ctx, payerActor.address, open.StreamID, reading, meter.public, signature, key)
	})
	requireStreamBalances(t, h, open, payer, "100", "100", "0")
	record, err := h.keeper.GetStream(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.Equal(t, reading, record.CumulativeMeter)
	require.Equal(t, codec.U128{Lo: 22}, record.AccruedTotal)
	require.Equal(t, codec.U128{Lo: 1}, record.RemainderCarry)
	require.Equal(t, codec.U128{}, record.SettledTotal)

	before = payStoreSnapshot(h.ctx, h.hostKey)
	regression := streamMeterSignature(t, meter, open.StreamID, reading-1)
	refused, err = h.keeper.StreamMeter(h.ctx, meter.address, open.StreamID, reading-1, meter.public, regression, [32]byte{0x91})
	require.ErrorIs(t, err, lxphost.Code(-720))
	require.Equal(t, lxphost.Code(-720), refused.ResultCode)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	key = [32]byte{0x92}
	refused, err = h.keeper.StreamClose(h.ctx, meter.address, open.StreamID, key)
	require.ErrorIs(t, err, lxphost.Code(-207))
	require.Equal(t, lxphost.Code(-207), refused.ResultCode)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))

	h.keeper.SetParams(h.ctx, types.Params{StreamEnabled: true, Paused: true})
	h.ctx = h.ctx.WithBlockTime(time.UnixMilli(payHarnessTimeMs + 10000)).WithBlockHeight(2)
	before = payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.StreamSettle(h.ctx, payerActor.address, open.StreamID, [32]byte{0x93})
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	_, err = h.keeper.StreamMeter(h.ctx, payerActor.address, open.StreamID, reading, meter.public, signature, [32]byte{0x94})
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	refused, err = h.keeper.StreamClose(h.ctx, meter.address, open.StreamID, key)
	require.ErrorIs(t, err, lxphost.Code(-207))
	require.Equal(t, lxphost.Code(-207), refused.ResultCode)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))

	payload, err = types.EncodeStreamKeyed(open.StreamID, key)
	require.NoError(t, err)
	require.Len(t, payload, 66)
	receipt := requireStreamSubmission(t, h, payerActor, 0x00040007, payload, key, func() (lxphost.Receipt, error) {
		return h.keeper.StreamClose(h.ctx, payerActor.address, open.StreamID, key)
	})
	require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
	requireStreamBalances(t, h, open, payer, "178", "0", "22")
	result, err := h.keeper.GetStreamResult(h.ctx, key)
	require.NoError(t, err)
	require.Equal(t, uint16(7), result.Ordinal)
	require.Equal(t, open.StreamID, result.StreamID)
	require.Equal(t, codec.U128{Lo: 22}, result.Paid)
	require.Equal(t, codec.U128{Lo: 78}, result.Refunded)
	require.Equal(t, uint8(2), result.LegCount)
	require.NotEqual(t, [32]byte{}, result.TransferSetRoot)
	record, err = h.keeper.GetStream(h.ctx, open.StreamID)
	require.NoError(t, err)
	require.True(t, record.Closed)
	require.False(t, record.Paused)
	require.False(t, record.Underfunded)
	require.Equal(t, codec.U128{Lo: 22}, record.SettledTotal)
	require.Equal(t, record.SettledTotal, record.AccruedTotal)
	require.Equal(t, codec.U128{}, record.RemainderCarry)
	require.Equal(t, uint64(payHarnessTimeMs+10000), record.LastAccrualTimestamp)

	before = payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.StreamClose(h.ctx, payerActor.address, [32]byte{}, key)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	_, err = h.keeper.StreamClose(h.ctx, payerActor.address, open.StreamID, [32]byte{})
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	_, err = h.keeper.StreamMeter(h.ctx, common.Address{}, open.StreamID, reading, meter.public, [64]byte{}, key)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
}
