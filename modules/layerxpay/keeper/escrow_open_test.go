package keeper

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/stretchr/testify/require"
)

func payEscrowFundedArgs(t *testing.T, h *payHarness) EscrowOpenArgs {
	t.Helper()
	enablePaySubmit(t, h)
	actor := h.actors[0]
	salt := [32]byte{0x31}
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor.id[:]...), salt[:]...))
	owner := paySubmitMain(t, h, actor, asset)
	beneficiary := paySubmitMain(t, h, h.actors[1], asset)
	assetName, assetAccount, err := lxphost.AssetAccountID(actor.public, asset)
	require.NoError(t, err)
	payload := append([]byte{0, 1}, asset[:]...)
	payload = append(payload, salt[:]...)
	payload = append(payload, 1, 'T', 1, 'T', 6)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 2000)
	payload = append(payload, 1, 0)
	paySubmitExecute(t, h, actor, 0x00010001, payload, [32]byte{0x32})
	paySubmitExecute(t, h, actor, 0x00010004, append([]byte{0, 1}, asset[:]...), [32]byte{0x33})
	payload = append(append([]byte{0, 1}, asset[:]...), assetAccount[:]...)
	payload = binary.BigEndian.AppendUint64(payload, 0)
	payload = binary.BigEndian.AppendUint64(payload, 1000)
	paySubmitExecute(t, h, actor, 0x0001000a, payload, [32]byte{0x34})

	amount := (codec.U128{Lo: 1000}).Bytes()
	key := [32]byte{0x35}
	material := append(append(append(append([]byte(nil), assetAccount[:]...), owner[:]...), asset[:]...), amount[:]...)
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
	payEscrowBalance(t, h, owner, asset, "1000")

	id := [32]byte{0x36}
	custody, err := codec.DeriveAccountID([]byte("agent:" + string(actor.did) + ":escrow:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	return EscrowOpenArgs{
		EscrowID: id, Owner: owner, EscrowAccount: custody, Beneficiary: beneficiary,
		Arbiter: actor.id, AssetID: asset, Amount: codec.U128{Lo: 600},
		ExpiryMs: uint64(payHarnessTimeMs) + 1000000, DisputeWindowMs: 12345,
		TermsHash: [32]byte{0x37}, AgreementReference: [32]byte{0x38},
	}
}

func payEscrowBalance(t *testing.T, h *payHarness, account, asset [32]byte, expected string) {
	t.Helper()
	balance, err := h.host.ReadBalance(h.ctx, account, asset)
	require.NoError(t, err)
	require.Equal(t, expected, balance.String())
}

func payEscrowExpectedReceipt(t *testing.T, h *payHarness, actor payActor, activityType uint32, payload []byte, key [32]byte) lxphost.Receipt {
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
	receipt, err := h.host.HostExecute(lxphost.WithSequence(branch, last+1), lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: uint64(h.ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(h.ctx.BlockTime().UnixMilli()) + h.host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}, authority)
	require.NoError(t, err)
	return receipt
}

func payEscrowReceipt(t *testing.T, expected, receipt lxphost.Receipt) {
	t.Helper()
	require.Zero(t, receipt.ResultCode)
	require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(2), decoded.ModuleID)
	require.Equal(t, expected.Encoded, receipt.Encoded)
	require.Equal(t, expected, receipt)
}

func TestEscrowOpenLocksFunds(t *testing.T) {
	h := newPayHarness(t)
	a := payEscrowFundedArgs(t, h)
	actor := h.actors[0]
	last, err := h.host.ReadLastSequence(h.ctx, actor.did)
	require.NoError(t, err)
	payload := types.EncodeEscrowOpen(a.EscrowID, a.Owner, a.EscrowAccount, a.Beneficiary,
		a.Arbiter, a.AssetID, a.Amount, a.ExpiryMs, a.DisputeWindowMs, a.TermsHash, a.AgreementReference)
	require.Len(t, payload, 288)
	expected := payEscrowExpectedReceipt(t, h, actor, 0x00020001, payload, a.EscrowID)
	receipt, err := h.keeper.EscrowOpen(h.ctx, actor.address, a)
	require.NoError(t, err)
	payEscrowReceipt(t, expected, receipt)
	require.Equal(t, last+1, receipt.GlobalSequence)
	key := types.KernelKVKey(2, append([]byte("hold:"), a.EscrowID[:]...))
	record, err := types.DecodeEscrowRecord(h.ctx.KVStore(h.hostKey).Get(key))
	require.NoError(t, err)
	require.Equal(t, types.EscrowRecord{
		EscrowID: a.EscrowID, Owner: a.Owner, EscrowAccount: a.EscrowAccount,
		Beneficiary: a.Beneficiary, Arbiter: a.Arbiter, AssetID: a.AssetID,
		LockedAmount: a.Amount, State: 1, ExpiryMs: a.ExpiryMs,
		DisputeWindowMs: a.DisputeWindowMs, TermsHash: a.TermsHash,
		AgreementReference: a.AgreementReference,
	}, record)
	payEscrowBalance(t, h, a.Owner, a.AssetID, "400")
	payEscrowBalance(t, h, a.EscrowAccount, a.AssetID, "600")
	payEscrowBalance(t, h, a.Beneficiary, a.AssetID, "0")
	result, found := h.keeper.GetEscrowResult(h.ctx, a.EscrowID)
	require.True(t, found)
	require.Equal(t, uint16(1), result.Ordinal)
	require.Equal(t, a.EscrowID, result.EscrowID)
	require.Equal(t, a.Amount, result.Amount)
	require.Equal(t, receipt.GlobalSequence, result.GlobalSequence)
}

func TestEscrowCaptureAndPartialCapture(t *testing.T) {
	h := newPayHarness(t)
	a := payEscrowFundedArgs(t, h)
	owner, beneficiary := h.actors[0], h.actors[1]
	opened, err := h.keeper.EscrowOpen(h.ctx, owner.address, a)
	require.NoError(t, err)

	before := payStoreSnapshot(h.ctx, h.hostKey)
	_, err = h.keeper.EscrowCapture(h.ctx, owner.address, a.EscrowID, codec.U128{Lo: 600}, [32]byte{0x39})
	require.ErrorIs(t, err, lxphost.Code(-708))
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))

	partialKey := [32]byte{0x40}
	partialPayload := types.EncodeEscrowCapture(a.EscrowID, codec.U128{Lo: 200}, partialKey)
	require.Len(t, partialPayload, 80)
	expectedPartial := payEscrowExpectedReceipt(t, h, beneficiary, 0x00020003, partialPayload, partialKey)
	partial, err := h.keeper.EscrowPartialCapture(h.ctx, beneficiary.address, a.EscrowID, codec.U128{Lo: 200}, partialKey)
	require.NoError(t, err)
	payEscrowReceipt(t, expectedPartial, partial)
	require.Equal(t, opened.GlobalSequence+1, partial.GlobalSequence)
	record, found := h.keeper.GetEscrow(h.ctx, a.EscrowID)
	require.True(t, found)
	require.Equal(t, uint8(2), record.State)
	require.Equal(t, codec.U128{Lo: 200}, record.CapturedAmount)
	require.Equal(t, codec.U128{Lo: 400}, record.LockedAmount)
	payEscrowBalance(t, h, a.Owner, a.AssetID, "400")
	payEscrowBalance(t, h, a.EscrowAccount, a.AssetID, "400")
	payEscrowBalance(t, h, a.Beneficiary, a.AssetID, "200")
	partialResult, found := h.keeper.GetEscrowResult(h.ctx, partialKey)
	require.True(t, found)
	require.Equal(t, uint16(3), partialResult.Ordinal)
	require.Equal(t, a.EscrowID, partialResult.EscrowID)
	require.Equal(t, record.State, partialResult.StateAfter)
	require.Equal(t, record.CapturedAmount, partialResult.CapturedAfter)
	require.Equal(t, record.LockedAmount, partialResult.LockedAfter)
	require.Equal(t, codec.U128{Lo: 200}, partialResult.Amount)
	require.Equal(t, partial.GlobalSequence, partialResult.GlobalSequence)

	captureKey := [32]byte{0x41}
	capturePayload := types.EncodeEscrowCapture(a.EscrowID, codec.U128{Lo: 400}, captureKey)
	require.Len(t, capturePayload, 80)
	expectedCapture := payEscrowExpectedReceipt(t, h, beneficiary, 0x00020002, capturePayload, captureKey)
	captured, err := h.keeper.EscrowCapture(h.ctx, beneficiary.address, a.EscrowID, codec.U128{Lo: 400}, captureKey)
	require.NoError(t, err)
	payEscrowReceipt(t, expectedCapture, captured)
	require.Equal(t, partial.GlobalSequence+1, captured.GlobalSequence)
	record, found = h.keeper.GetEscrow(h.ctx, a.EscrowID)
	require.True(t, found)
	require.Equal(t, uint8(3), record.State)
	require.Equal(t, codec.U128{Lo: 600}, record.CapturedAmount)
	require.Equal(t, codec.U128{}, record.LockedAmount)
	payEscrowBalance(t, h, a.Owner, a.AssetID, "400")
	payEscrowBalance(t, h, a.EscrowAccount, a.AssetID, "0")
	payEscrowBalance(t, h, a.Beneficiary, a.AssetID, "600")
	result, found := h.keeper.GetEscrowResult(h.ctx, captureKey)
	require.True(t, found)
	require.Equal(t, uint16(2), result.Ordinal)
	require.Equal(t, a.EscrowID, result.EscrowID)
	require.Equal(t, record.State, result.StateAfter)
	require.Equal(t, record.CapturedAmount, result.CapturedAfter)
	require.Equal(t, record.LockedAmount, result.LockedAfter)
	require.Equal(t, codec.U128{Lo: 400}, result.Amount)
	require.Equal(t, captured.GlobalSequence, result.GlobalSequence)
	storedPartial, found := h.keeper.GetEscrowResult(h.ctx, partialKey)
	require.True(t, found)
	require.Equal(t, partialResult, storedPartial)
}
