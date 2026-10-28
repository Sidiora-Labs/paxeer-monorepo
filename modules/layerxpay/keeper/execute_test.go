package keeper

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func enablePaySubmit(t *testing.T, h *payHarness) {
	t.Helper()
	h.keeper.SetParams(h.ctx, types.Params{EscrowEnabled: true})
	key, err := types.KernelModuleEnableKey(types.EscrowModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(key, payGateEnabledRecord())
}

func paySubmitExecute(t *testing.T, h *payHarness, actor payActor, activityType uint32, payload []byte, key [32]byte) lxphost.Receipt {
	t.Helper()
	last, err := h.host.ReadLastSequence(h.ctx, actor.did)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	activity := lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.public,
		AccountSequence: sequence, NotBefore: uint64(h.ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(h.ctx.BlockTime().UnixMilli()) + h.host.TimestampWindowMs(),
		IdempotencyKey: key, Payload: payload,
	}
	hash, err := h.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(actor.private, hash[:])
	receipt, err := h.host.HostExecute(lxphost.WithSequence(h.ctx, last+1), activity, actor.authority())
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	return receipt
}

func paySubmitMain(t *testing.T, h *payHarness, actor payActor, asset [32]byte) [32]byte {
	t.Helper()
	name, id, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	row := field(nil, id[:])
	row = field(row, []byte(name))
	row = append(row, 1)
	row = append(row, make([]byte, 16)...)
	row = field(row, asset[:])
	row = append(row, 1)
	row = binary.BigEndian.AppendUint64(row, 0)
	row = binary.BigEndian.AppendUint64(row, 1)
	row = append(row, 0, 0)
	row = field(row, actor.public[:])
	row = append(row, 1)
	h.ctx.KVStore(h.hostKey).Set(append([]byte{0x06}, id[:]...), row)
	return id
}

func paySubmitFundedEscrow(t *testing.T, h *payHarness, id [32]byte) []byte {
	t.Helper()
	enablePaySubmit(t, h)
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

	custody, err := codec.DeriveAccountID([]byte("agent:" + string(actor.did) + ":escrow:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	return types.EncodeEscrowOpen(id, main, custody, beneficiary, [32]byte{}, asset,
		codec.U128{Lo: 50}, uint64(payHarnessTimeMs)+1000000, 0, [32]byte{}, [32]byte{})
}

func TestSubmitCommitsKernelActivity(t *testing.T) {
	h := newPayHarness(t)
	const window uint64 = 17000
	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: h.hostKey, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: window, GasLimit: 1000000,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	h.host, h.keeper.host = host, host
	require.Equal(t, window, h.host.TimestampWindowMs())
	id, key := [32]byte{10}, [32]byte{11}
	payload := paySubmitFundedEscrow(t, h, id)
	actor := h.actors[0]
	last, err := h.host.ReadLastSequence(h.ctx, actor.did)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	_, principal, err := lxphost.MainAccountID(actor.public)
	require.NoError(t, err)
	authority := actor.authority()
	authority.Bound, authority.Principal = true, principal
	expectedBranch, _ := h.ctx.CacheContext()
	expected, err := h.host.HostExecute(lxphost.WithSequence(expectedBranch, last+1), lxphost.Activity{
		ProtocolVersion: 3, Type: types.MethodActivity["escrowOpen"].Type,
		ActorDID: actor.did, AuthorityKey: actor.public, AccountSequence: sequence,
		NotBefore: uint64(payHarnessTimeMs), NotAfter: uint64(payHarnessTimeMs) + window,
		IdempotencyKey: key, Payload: payload,
	}, authority)
	require.NoError(t, err)
	require.Zero(t, expected.ResultCode)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	receipt, err := h.keeper.Submit(h.ctx, actor.address, "escrowOpen", payload, key)
	require.NoError(t, err)
	require.Equal(t, expected, receipt)
	require.Zero(t, receipt.ResultCode)
	require.Equal(t, last+1, receipt.GlobalSequence)
	require.Equal(t, uint64(payHarnessTimeMs), receipt.Timestamp)
	require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, uint16(3), decoded.ProtocolVersion)
	require.Equal(t, types.EscrowModuleID, decoded.ModuleID)
	sequenceAfter, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	require.Equal(t, sequence+1, sequenceAfter)
	require.NotEqual(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, payStoreSnapshot(expectedBranch, h.hostKey), payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	record, found := h.keeper.GetEscrow(h.ctx, id)
	require.True(t, found)
	require.Equal(t, codec.U128{Lo: 50}, record.LockedAmount)
	require.Equal(t, uint8(1), record.State)

	other := h.actors[1]
	otherSequence, err := h.host.ReadIdentitySequence(h.ctx, other.did)
	require.NoError(t, err)
	captured, err := h.keeper.Submit(h.ctx, other.address, "escrowCapture",
		types.EncodeEscrowCapture(id, codec.U128{Lo: 50}, [32]byte{13}), [32]byte{14})
	require.NoError(t, err)
	require.Zero(t, captured.ResultCode)
	require.Equal(t, receipt.GlobalSequence+1, captured.GlobalSequence)
	lastAfter, err := h.host.ReadLastSequence(h.ctx, actor.did)
	require.NoError(t, err)
	require.Equal(t, captured.GlobalSequence, lastAfter)
	otherSequenceAfter, err := h.host.ReadIdentitySequence(h.ctx, other.did)
	require.NoError(t, err)
	require.Equal(t, otherSequence+1, otherSequenceAfter)
	record, found = h.keeper.GetEscrow(h.ctx, id)
	require.True(t, found)
	require.Equal(t, uint8(3), record.State)
	require.Equal(t, codec.U128{Lo: 50}, record.CapturedAmount)
	require.Equal(t, codec.U128{}, record.LockedAmount)
}

func TestSubmitKernelRefusalCarriesRegistryCode(t *testing.T) {
	h := newPayHarness(t)
	id := [32]byte{20}
	payload := paySubmitFundedEscrow(t, h, id)
	actor := h.actors[0]
	_, err := h.keeper.Submit(h.ctx, actor.address, "escrowOpen", payload, [32]byte{21})
	require.NoError(t, err)
	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	recordBefore, found := h.keeper.GetEscrow(h.ctx, id)
	require.True(t, found)
	receipt, err := h.keeper.Submit(h.ctx, actor.address, "escrowCapture",
		types.EncodeEscrowCapture(id, codec.U128{Lo: 50}, [32]byte{22}), [32]byte{23})
	require.Error(t, err)
	require.Equal(t, lxphost.Code(-708), receipt.ResultCode)
	require.ErrorIs(t, err, receipt.ResultCode)
	code := pcommon.LXPCodeName(int32(receipt.ResultCode))
	require.Equal(t, "LXP_UNAUTHORIZED_CAPTURE", code)
	require.ErrorContains(t, err, code)
	reason, decodeErr := abi.UnpackRevert(pcommon.LXPReason(err))
	require.NoError(t, decodeErr)
	require.Contains(t, reason, code)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	recordAfter, found := h.keeper.GetEscrow(h.ctx, id)
	require.True(t, found)
	require.Equal(t, recordBefore, recordAfter)

	t.Run("write_and_authority_checks", func(t *testing.T) {
		h.keeper.SetParams(h.ctx, types.Params{})
		_, err := h.keeper.Submit(h.ctx, common.Address{}, "escrowOpen", nil, [32]byte{24})
		require.ErrorIs(t, err, types.ErrFamilyDisabled)
		enablePaySubmit(t, h)
		_, err = h.keeper.Submit(h.ctx, common.Address{}, "escrowOpen", nil, [32]byte{24})
		require.ErrorIs(t, err, types.ErrUnboundCaller)
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	})
}

func TestSubmitReusedKeyExecutesAtNextSequence(t *testing.T) {
	h := newPayHarness(t)
	id, key := [32]byte{30}, [32]byte{31}
	payload := paySubmitFundedEscrow(t, h, id)
	actor := h.actors[0]
	first, err := h.keeper.Submit(h.ctx, actor.address, "escrowOpen", payload, key)
	require.NoError(t, err)
	require.Zero(t, first.ResultCode)
	sequence, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	h.ctx = h.ctx.WithBlockTime(time.UnixMilli(payHarnessTimeMs + 120000)).WithBlockHeight(2)
	released, err := h.keeper.Submit(h.ctx, actor.address, "escrowRelease", types.EncodeEscrowRelease(id, [32]byte{32}), key)
	require.NoError(t, err)
	require.Zero(t, released.ResultCode)
	require.Equal(t, first.GlobalSequence+1, released.GlobalSequence)
	require.Equal(t, uint64(payHarnessTimeMs+120000), released.Timestamp)
	require.NotEqual(t, first.Encoded, released.Encoded)
	sequenceAfter, err := h.host.ReadIdentitySequence(h.ctx, actor.did)
	require.NoError(t, err)
	require.Equal(t, sequence+1, sequenceAfter)
	recordBefore, found := h.keeper.GetEscrow(h.ctx, id)
	require.True(t, found)
	require.Equal(t, uint8(4), recordBefore.State)
	require.Equal(t, codec.U128{}, recordBefore.LockedAmount)
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	refused, err := h.keeper.Submit(h.ctx, actor.address, "escrowRelease", types.EncodeEscrowRelease(id, [32]byte{33}), key)
	require.Error(t, err)
	require.Equal(t, lxphost.Code(-700), refused.ResultCode)
	require.ErrorIs(t, err, refused.ResultCode)
	require.ErrorContains(t, err, pcommon.LXPCodeName(int32(refused.ResultCode)))
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	recordAfter, found := h.keeper.GetEscrow(h.ctx, id)
	require.True(t, found)
	require.Equal(t, recordBefore, recordAfter)
}
