package keeper

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func serviceDeliveryHarness(t *testing.T) (*payHarness, [32]byte, [32]byte, [32]byte) {
	t.Helper()
	h := newPayHarness(t)
	h.keeper.SetParams(h.ctx, types.Params{ServiceEnabled: true})
	key, err := types.KernelModuleEnableKey(types.ServiceModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(key, payGateEnabledRecord())
	require.NoError(t, h.keeper.CheckWrite(h.ctx, "serviceOfferPublish"))
	offerID, agreementID, commitmentID := [32]byte{0x81}, [32]byte{0x82}, [32]byte{0x83}
	terms, spec := [32]byte{0x84}, [32]byte{0x85}
	provider, buyer := h.actors[0], h.actors[1]
	payload, err := types.EncodeServiceOffer(offerID, [32]byte{0x86}, codec.U128{Lo: 50}, terms, spec,
		uint64(payHarnessTimeMs)+100000, 20000, 30000, 1, uint64(payHarnessTimeMs)+90000)
	require.NoError(t, err)
	_, err = h.keeper.Submit(h.ctx, provider.address, "serviceOfferPublish", payload, [32]byte{1})
	require.NoError(t, err)
	payload, err = types.EncodeServicePropose(agreementID, offerID, terms, [32]byte{0x87})
	require.NoError(t, err)
	_, err = h.keeper.Submit(h.ctx, buyer.address, "serviceAgreementPropose", payload, [32]byte{2})
	require.NoError(t, err)
	payload, err = types.EncodeServiceIdentifier(agreementID)
	require.NoError(t, err)
	_, err = h.keeper.Submit(h.ctx, provider.address, "serviceAgreementAccept", payload, [32]byte{3})
	require.NoError(t, err)
	payload, err = types.EncodeServiceCommit(commitmentID, agreementID, [32]byte{0x88}, [32]byte{}, uint64(payHarnessTimeMs)+100000, 1000)
	require.NoError(t, err)
	_, err = h.keeper.Submit(h.ctx, provider.address, "serviceCommitTask", payload, [32]byte{4})
	require.NoError(t, err)
	agreement, found := h.keeper.GetAgreement(h.ctx, agreementID)
	require.True(t, found)
	require.Equal(t, uint8(2), agreement.State)
	_, principal, err := lxphost.MainAccountID(provider.public)
	require.NoError(t, err)
	require.Equal(t, principal, agreement.Provider)
	return h, agreementID, commitmentID, spec
}

func serviceDeliveryReceipt(t *testing.T, receipt lxphost.Receipt, activity uint32) {
	t.Helper()
	require.Zero(t, receipt.ResultCode)
	require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, types.ServiceModuleID, decoded.ModuleID)
	require.Zero(t, decoded.ResultCode)
	require.Equal(t, receipt.GlobalSequence, decoded.GlobalSequence)
	require.Equal(t, receipt.PreviousStateRoot, decoded.PreviousStateRoot)
	require.Equal(t, receipt.ResultingStateRoot, decoded.ResultingStateRoot)
	require.Len(t, decoded.Effects, 1)
	require.Equal(t, uint16(activity>>16), decoded.Effects[0].ModuleID)
	require.Equal(t, uint16(0), decoded.Effects[0].Ordinal)
	require.Equal(t, uint16(activity), decoded.Effects[0].EventType)
	require.Len(t, receipt.Events, 1)
	require.Equal(t, uint16(activity), receipt.Events[0].Code)
	require.Equal(t, decoded.Effects[0].Body, receipt.Events[0].Body)
	require.Len(t, receipt.Events[0].Body, 73)
	require.Equal(t, receipt.GlobalSequence, binary.BigEndian.Uint64(receipt.Events[0].Body[65:73]))
}

func serviceDeliveryRefused(t *testing.T, h *payHarness, call func() (lxphost.Receipt, error), reason string) {
	t.Helper()
	hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	_, err := call()
	require.ErrorContains(t, err, reason)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
}

func TestServiceDeliverAcceptFlow(t *testing.T) {
	h, agreementID, commitmentID, spec := serviceDeliveryHarness(t)
	provider, buyer := h.actors[0].address, h.actors[1].address
	reportID, note, reference := [32]byte{10}, [32]byte{11}, [32]byte{12}
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceProgressReport(h.ctx, provider, reportID, commitmentID, note, reference, 10001, [32]byte{13})
	}, "LXP_PARAMETER_BOUNDS")
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceProgressReport(h.ctx, buyer, reportID, commitmentID, note, reference, 5000, [32]byte{13})
	}, "LXP_AGREEMENT_STATE")
	receipt, err := h.keeper.ServiceProgressReport(h.ctx, provider, reportID, commitmentID, note, reference, 5000, [32]byte{13})
	require.NoError(t, err)
	serviceDeliveryReceipt(t, receipt, 0x00050008)
	progress, found := h.keeper.GetProgress(h.ctx, reportID)
	require.True(t, found)
	require.Equal(t, commitmentID, progress.CommitmentID)
	require.Equal(t, agreementID, progress.AgreementID)
	require.Equal(t, note, progress.NoteHash)
	require.Equal(t, reference, progress.AvailabilityReference)
	require.Equal(t, uint32(5000), progress.ProgressBPS)
	require.Equal(t, receipt.GlobalSequence, progress.GlobalSequence)
	require.Equal(t, uint64(payHarnessTimeMs), progress.ReportedAtMs)
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceProgressReport(h.ctx, provider, [32]byte{14}, commitmentID, note, reference, 5000, [32]byte{15})
	}, "LXP_METER_REGRESSION")
	complete, err := h.keeper.ServiceProgressReport(h.ctx, provider, [32]byte{14}, commitmentID, note, reference, 10000, [32]byte{15})
	require.NoError(t, err)
	serviceDeliveryReceipt(t, complete, 0x00050008)
	require.Equal(t, receipt.GlobalSequence+1, complete.GlobalSequence)
	highWater, found := h.keeper.ProgressHighWater(h.ctx, commitmentID)
	require.True(t, found)
	require.Equal(t, uint32(10000), highWater)

	deliveryID := [32]byte{16}
	items := []types.ServiceDeliverable{
		{Hash: spec, ArtifactSize: 1024, AvailabilityReference: [32]byte{18}},
		{Hash: spec, ArtifactSize: 512, AvailabilityReference: [32]byte{17}},
	}
	itemsBefore := append([]types.ServiceDeliverable(nil), items...)
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceDeliver(h.ctx, provider, deliveryID, agreementID, nil, [32]byte{19})
	}, "LXP_LENGTH_LIMIT")
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceDeliver(h.ctx, buyer, deliveryID, agreementID, items, [32]byte{19})
	}, "LXP_AGREEMENT_STATE")
	delivered, err := h.keeper.ServiceDeliver(h.ctx, provider, deliveryID, agreementID, items, [32]byte{19})
	require.NoError(t, err)
	serviceDeliveryReceipt(t, delivered, 0x00050009)
	require.Equal(t, itemsBefore, items)
	require.Equal(t, complete.GlobalSequence+1, delivered.GlobalSequence)
	delivery, found := h.keeper.GetDelivery(h.ctx, deliveryID)
	require.True(t, found)
	require.Equal(t, agreementID, delivery.AgreementID)
	require.Equal(t, []types.ServiceDeliverable{items[1], items[0]}, delivery.Deliverables)
	require.Equal(t, delivered.GlobalSequence, delivery.GlobalSequence)
	agreement, found := h.keeper.GetAgreement(h.ctx, agreementID)
	require.True(t, found)
	require.Equal(t, uint8(3), agreement.State)
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceAccept(h.ctx, provider, agreementID, [32]byte{20})
	}, "LXP_AGREEMENT_STATE")
	accepted, err := h.keeper.ServiceAccept(h.ctx, buyer, agreementID, [32]byte{20})
	require.NoError(t, err)
	serviceDeliveryReceipt(t, accepted, 0x0005000a)
	require.Equal(t, delivered.GlobalSequence+1, accepted.GlobalSequence)
	agreement, found = h.keeper.GetAgreement(h.ctx, agreementID)
	require.True(t, found)
	require.Equal(t, uint8(4), agreement.State)
	require.Equal(t, accepted.GlobalSequence, agreement.OutcomeSequence)
	require.Equal(t, accepted.Timestamp, agreement.OutcomeTimestampMs)
	require.False(t, agreement.DefaultApplied)
}

func TestServiceRejectAndToolAttest(t *testing.T) {
	h, agreementID, commitmentID, spec := serviceDeliveryHarness(t)
	provider, buyer := h.actors[0], h.actors[1]
	attestationID, activityKey := [32]byte{30}, [32]byte{31}
	_, principal, err := lxphost.MainAccountID(provider.public)
	require.NoError(t, err)
	execution := make([]byte, types.ServiceExecutionBytes)
	for i, field := range [][32]byte{attestationID, activityKey, agreementID, commitmentID, {32}, {33}, {34}} {
		copy(execution[32*i:32*(i+1)], field[:])
	}
	binary.BigEndian.PutUint64(execution[224:232], uint64(payHarnessTimeMs)-100)
	binary.BigEndian.PutUint64(execution[232:240], uint64(payHarnessTimeMs))
	binary.BigEndian.PutUint64(execution[240:248], 250)
	copy(execution[248:280], principal[:])
	availability := [32]byte{35}
	copy(execution[280:312], availability[:])
	copy(execution[312:344], provider.public[:])
	message := append([]byte("LXP:SERVICE:ATTEST:v1"), execution[:344]...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	copy(execution[344:408], ed25519.Sign(provider.private, digest[:]))
	before := bytes.Clone(execution)
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceToolExecAttest(h.ctx, provider.address, execution[:415], activityKey)
	}, "LXP_NON_CANONICAL")
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceToolExecAttest(h.ctx, provider.address, execution, [32]byte{36})
	}, "LXP_INVALID_ATTESTATION")
	tampered := bytes.Clone(execution)
	tampered[344] ^= 1
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceToolExecAttest(h.ctx, provider.address, tampered, activityKey)
	}, "LXP_INVALID_ATTESTATION")
	attested, err := h.keeper.ServiceToolExecAttest(h.ctx, provider.address, execution, activityKey)
	require.NoError(t, err)
	serviceDeliveryReceipt(t, attested, 0x00050007)
	require.Equal(t, before, execution)
	wantRecord := append([]byte{1}, execution...)
	binary.BigEndian.PutUint64(wantRecord[409:417], attested.GlobalSequence)
	require.Equal(t, wantRecord, h.ctx.KVStore(h.hostKey).Get(serviceKey("execut:1", attestationID)))
	stored, found := h.keeper.GetExecution(h.ctx, attestationID)
	require.True(t, found)
	require.Equal(t, agreementID, stored.AgreementID)
	require.Equal(t, commitmentID, stored.CommitmentID)
	require.Equal(t, activityKey, stored.ActivityID)
	require.Equal(t, message, stored.CanonicalPayload[:stored.CanonicalPayloadLength])
	require.True(t, ed25519.Verify(ed25519.PublicKey(stored.PublicKey[:]), digest[:], stored.Signature[:]))

	items := []types.ServiceDeliverable{{Hash: spec, ArtifactSize: 4096, AvailabilityReference: availability}}
	delivered, err := h.keeper.ServiceDeliver(h.ctx, provider.address, [32]byte{37}, agreementID, items, [32]byte{38})
	require.NoError(t, err)
	serviceDeliveryReceipt(t, delivered, 0x00050009)
	hashes := [][32]byte{spec}
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceReject(h.ctx, buyer.address, agreementID, 7, [][32]byte{spec, spec}, [32]byte{39})
	}, "LXP_DUPLICATE_ENTRY")
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceReject(h.ctx, buyer.address, agreementID, 7, [][32]byte{{40}}, [32]byte{39})
	}, "LXP_DELIVERABLE_MISMATCH")
	serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) {
		return h.keeper.ServiceReject(h.ctx, provider.address, agreementID, 7, hashes, [32]byte{39})
	}, "LXP_AGREEMENT_STATE")
	rejected, err := h.keeper.ServiceReject(h.ctx, buyer.address, agreementID, 7, hashes, [32]byte{39})
	require.NoError(t, err)
	serviceDeliveryReceipt(t, rejected, 0x0005000b)
	require.Equal(t, delivered.GlobalSequence+1, rejected.GlobalSequence)
	require.Equal(t, [][32]byte{spec}, hashes)
	agreement, found := h.keeper.GetAgreement(h.ctx, agreementID)
	require.True(t, found)
	require.Equal(t, uint8(5), agreement.State)
	require.Equal(t, uint16(7), agreement.RejectionReason)
	require.Equal(t, hashes, agreement.ContestedHashes)
	require.Equal(t, rejected.GlobalSequence, agreement.OutcomeSequence)
	require.Equal(t, rejected.Timestamp, agreement.OutcomeTimestampMs)

	for _, tc := range []struct {
		name string
		call func(common.Address) (lxphost.Receipt, error)
	}{
		{"attest", func(caller common.Address) (lxphost.Receipt, error) {
			return h.keeper.ServiceToolExecAttest(h.ctx, caller, execution, activityKey)
		}},
		{"progress", func(caller common.Address) (lxphost.Receipt, error) {
			return h.keeper.ServiceProgressReport(h.ctx, caller, [32]byte{41}, commitmentID, [32]byte{42}, availability, 5000, [32]byte{43})
		}},
		{"deliver", func(caller common.Address) (lxphost.Receipt, error) {
			return h.keeper.ServiceDeliver(h.ctx, caller, [32]byte{44}, agreementID, items, [32]byte{45})
		}},
		{"accept", func(caller common.Address) (lxphost.Receipt, error) {
			return h.keeper.ServiceAccept(h.ctx, caller, agreementID, [32]byte{46})
		}},
		{"reject", func(caller common.Address) (lxphost.Receipt, error) {
			return h.keeper.ServiceReject(h.ctx, caller, agreementID, 7, hashes, [32]byte{47})
		}},
	} {
		t.Run(tc.name+"_submit_guards", func(t *testing.T) {
			serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) { return tc.call(common.Address{}) }, "LXP_PAY_UNBOUND_CALLER")
			h.keeper.SetParams(h.ctx, types.Params{})
			serviceDeliveryRefused(t, h, func() (lxphost.Receipt, error) { return tc.call(provider.address) }, "LXP_MODULE_DISABLED")
			h.keeper.SetParams(h.ctx, types.Params{ServiceEnabled: true})
		})
	}
}
