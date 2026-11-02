package keeper

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

type serviceViewFixture struct {
	prefix string
	id     [32]byte
	wire   []byte
	want   any
	view   func(Keeper, sdk.Context, [32]byte) (any, bool)
}

func serviceViewID(seed byte) [32]byte {
	var id [32]byte
	for i := range id {
		id[i] = seed + byte(i)
	}
	return id
}

func serviceViewWire(t *testing.T, size int, fields ...any) []byte {
	t.Helper()
	var out bytes.Buffer
	out.WriteByte(1)
	for _, field := range fields {
		require.NoError(t, binary.Write(&out, binary.BigEndian, field))
	}
	require.Equal(t, size, out.Len())
	return out.Bytes()
}

func serviceViewFixtures(t *testing.T) []serviceViewFixture {
	t.Helper()
	id := serviceViewID
	offer := types.ServiceOffer{
		OfferID: id(1), ActivityID: id(11), OfferingAgent: id(12), AssetID: id(13),
		Price:     codec.U128{Hi: 0x1122334455667788, Lo: 0x8877665544332211},
		TermsHash: id(14), DeliverableSpecHash: id(15), DeliveryDeadlineMs: 12345,
		AcceptanceWindowMs: 23456, DisputeWindowMs: 34567, DefaultOutcome: 2,
		OfferExpiryMs: 45678, GlobalSequence: 101, Withdrawn: true, Accepted: false,
	}
	agreement := types.ServiceAgreement{
		AgreementID: id(2), OfferID: offer.OfferID, Provider: offer.OfferingAgent, Buyer: id(16),
		TermsHash: offer.TermsHash, EscrowID: id(17), DeliveryDeadlineMs: 12345,
		AcceptanceWindowEndMs: 23456, DisputeWindowEndMs: 34567, DefaultOutcome: 2,
		State: 7, AcceptedSequence: 102, RejectionReason: 0x1234,
		ContestedHashes: [][32]byte{id(18), id(19)}, DefaultApplied: true,
		OutcomeSequence: 103, OutcomeTimestampMs: 45678,
	}
	commitment := types.ServiceCommitment{
		CommitmentID: id(3), ActivityID: id(20), Provider: agreement.Provider,
		AgreementID: agreement.AgreementID, TaskHash: id(21), EscrowID: agreement.EscrowID,
		DeadlineMs: 12345, ResourceBound: 23456, GlobalSequence: 104,
		Abandoned: true, AbandonReason: 0x2345,
	}
	execution := types.ServiceExecution{
		AttestationID: id(4), ActivityID: id(22), AgreementID: agreement.AgreementID,
		CommitmentID: commitment.CommitmentID, ToolID: id(23), InputCommitmentHash: id(24),
		OutputCommitmentHash: id(25), ExecutionStart: 12345, ExecutionEnd: 23456,
		ResourceUnits: 34567, AttestorIdentity: id(26), AvailabilityReference: id(27),
		PublicKey: id(28), GlobalSequence: 105,
	}
	for i := range execution.Signature {
		execution.Signature[i] = byte(i + 1)
	}
	executionWire := serviceViewWire(t, 417,
		execution.AttestationID, execution.ActivityID, execution.AgreementID, execution.CommitmentID,
		execution.ToolID, execution.InputCommitmentHash, execution.OutputCommitmentHash,
		execution.ExecutionStart, execution.ExecutionEnd, execution.ResourceUnits,
		execution.AttestorIdentity, execution.AvailabilityReference, execution.PublicKey,
		execution.Signature, execution.GlobalSequence)
	preimage := append([]byte("LXP:SERVICE:ATTEST:v1"), executionWire[1:345]...)
	execution.CanonicalPayloadLength = uint16(len(preimage))
	copy(execution.CanonicalPayload[:], preimage)
	delivery := types.ServiceDelivery{
		AgreementID: agreement.AgreementID, Provider: agreement.Provider, GlobalSequence: 106,
		Deliverables: []types.ServiceDeliverable{
			{Hash: id(29), ArtifactSize: 0x1122334455667788, AvailabilityReference: id(30)},
			{Hash: id(31), ArtifactSize: 0x8877665544332211, AvailabilityReference: id(32)},
		},
	}
	deliveryWire := serviceViewWire(t, 138, id(5), id(33), delivery.AgreementID,
		delivery.Provider, byte(len(delivery.Deliverables)), delivery.GlobalSequence)
	for _, item := range delivery.Deliverables {
		deliveryWire = append(deliveryWire, serviceViewWire(t, 73, item.Hash, item.ArtifactSize, item.AvailabilityReference)...)
	}
	progress := types.ServiceProgress{
		ReportID: id(6), ActivityID: id(34), CommitmentID: commitment.CommitmentID,
		AgreementID: agreement.AgreementID, Provider: agreement.Provider, NoteHash: id(35),
		AvailabilityReference: id(36), ProgressBPS: 8000, ReportedAtMs: 12345, GlobalSequence: 107,
	}
	dispute := types.ServiceDispute{
		DisputeID: id(7), ActivityID: id(37), AgreementID: agreement.AgreementID, Raiser: agreement.Buyer,
		EvidenceHashes: [][32]byte{id(38), id(39)}, GlobalSequence: 108, Resolved: true,
		Ruling: 0x3456, ProviderBPS: 6789, EscrowResolutionID: id(40), ResolutionSequence: 109,
	}
	return []serviceViewFixture{
		{"offer::1", offer.OfferID, serviceViewWire(t, 252,
			offer.OfferID, offer.ActivityID, offer.OfferingAgent, offer.AssetID, offer.Price.Bytes(),
			offer.TermsHash, offer.DeliverableSpecHash, offer.DeliveryDeadlineMs, offer.AcceptanceWindowMs,
			offer.DisputeWindowMs, offer.DefaultOutcome, offer.OfferExpiryMs, offer.GlobalSequence,
			offer.Withdrawn, offer.Accepted), offer,
			func(k Keeper, ctx sdk.Context, id [32]byte) (any, bool) { return k.GetOffer(ctx, id) }},
		{"agree::1", agreement.AgreementID, serviceViewWire(t, 311,
			agreement.AgreementID, agreement.OfferID, agreement.Provider, agreement.Buyer, agreement.TermsHash,
			agreement.EscrowID, agreement.DeliveryDeadlineMs, agreement.AcceptanceWindowEndMs,
			agreement.DisputeWindowEndMs, agreement.DefaultOutcome, agreement.State, agreement.AcceptedSequence,
			agreement.RejectionReason, agreement.DefaultApplied, agreement.OutcomeSequence,
			agreement.OutcomeTimestampMs, byte(len(agreement.ContestedHashes)), agreement.ContestedHashes), agreement,
			func(k Keeper, ctx sdk.Context, id [32]byte) (any, bool) { return k.GetAgreement(ctx, id) }},
		{"commit:1", commitment.CommitmentID, serviceViewWire(t, 220,
			commitment.CommitmentID, commitment.ActivityID, commitment.Provider, commitment.AgreementID,
			commitment.TaskHash, commitment.EscrowID, commitment.DeadlineMs, commitment.ResourceBound,
			commitment.GlobalSequence, commitment.Abandoned, commitment.AbandonReason), commitment,
			func(k Keeper, ctx sdk.Context, id [32]byte) (any, bool) { return k.GetCommitment(ctx, id) }},
		{"execut:1", execution.AttestationID, executionWire, execution,
			func(k Keeper, ctx sdk.Context, id [32]byte) (any, bool) { return k.GetExecution(ctx, id) }},
		{"delivr:1", id(5), deliveryWire, delivery,
			func(k Keeper, ctx sdk.Context, id [32]byte) (any, bool) { return k.GetDelivery(ctx, id) }},
		{"progrs:1", progress.ReportID, serviceViewWire(t, 245,
			progress.ReportID, progress.ActivityID, progress.CommitmentID, progress.AgreementID,
			progress.Provider, progress.NoteHash, progress.AvailabilityReference, progress.ProgressBPS,
			progress.ReportedAtMs, progress.GlobalSequence), progress,
			func(k Keeper, ctx sdk.Context, id [32]byte) (any, bool) { return k.GetProgress(ctx, id) }},
		{"disput:1", dispute.DisputeID, serviceViewWire(t, 249,
			dispute.DisputeID, dispute.ActivityID, dispute.AgreementID, dispute.Raiser, dispute.GlobalSequence,
			dispute.Resolved, dispute.Ruling, dispute.ProviderBPS, dispute.EscrowResolutionID,
			dispute.ResolutionSequence, byte(len(dispute.EvidenceHashes)), dispute.EvidenceHashes), dispute,
			func(k Keeper, ctx sdk.Context, id [32]byte) (any, bool) { return k.GetDispute(ctx, id) }},
	}
}

func putServiceViewFixture(t *testing.T, ctx sdk.Context, storeKey sdk.StoreKey, fixture serviceViewFixture) {
	t.Helper()
	key := append([]byte(fixture.prefix), fixture.id[:]...)
	require.Len(t, key, 40)
	store := ctx.KVStore(storeKey)
	if fixture.prefix != "delivr:1" {
		store.Set(types.KernelKVKey(5, key), fixture.wire)
		return
	}
	store.Set(types.KernelKVKey(5, key), fixture.wire[:138])
	for i := 0; i < int(fixture.wire[129]); i++ {
		itemKey := append(append([]byte("delitm:1"), fixture.id[:]...), byte(i))
		require.Len(t, itemKey, 41)
		store.Set(types.KernelKVKey(5, itemKey), fixture.wire[138+73*i:138+73*(i+1)])
	}
}

func TestServiceViewsReadKernelRecords(t *testing.T) {
	h := newPayHarness(t)
	fixtures := serviceViewFixtures(t)
	for _, fixture := range fixtures {
		putServiceViewFixture(t, h.ctx, h.hostKey, fixture)
		key := append([]byte(fixture.prefix), fixture.id[:]...)
		// Conflicting bytes catch reads from the pay store, a bare key, or
		// another module's namespace in the shared host store.
		h.ctx.KVStore(h.storeKey).Set(types.KernelKVKey(5, key), []byte{255})
		h.ctx.KVStore(h.hostKey).Set(key, []byte{255})
		h.ctx.KVStore(h.hostKey).Set(types.KernelKVKey(4, key), []byte{255})
	}
	deliveryFixture, progressFixture := fixtures[4], fixtures[5]
	delivery := deliveryFixture.want.(types.ServiceDelivery)
	progress := progressFixture.want.(types.ServiceProgress)
	// Higher key order does not mean newer delivery; another agreement's
	// newest delivery must also be excluded.
	older := deliveryFixture
	older.id, older.wire = serviceViewID(90), bytes.Clone(deliveryFixture.wire)
	copy(older.wire[1:33], older.id[:])
	binary.BigEndian.PutUint64(older.wire[130:138], delivery.GlobalSequence-1)
	putServiceViewFixture(t, h.ctx, h.hostKey, older)
	other := older
	other.id, other.wire = serviceViewID(91), bytes.Clone(older.wire)
	copy(other.wire[1:33], other.id[:])
	otherAgreement := serviceViewID(92)
	copy(other.wire[65:97], otherAgreement[:])
	binary.BigEndian.PutUint64(other.wire[130:138], delivery.GlobalSequence+100)
	putServiceViewFixture(t, h.ctx, h.hostKey, other)
	lower := progressFixture
	lower.id, lower.wire = serviceViewID(93), bytes.Clone(progressFixture.wire)
	copy(lower.wire[1:33], lower.id[:])
	binary.BigEndian.PutUint32(lower.wire[225:229], 4000)
	binary.BigEndian.PutUint64(lower.wire[237:245], progress.GlobalSequence+100)
	putServiceViewFixture(t, h.ctx, h.hostKey, lower)
	otherProgress := lower
	otherProgress.id, otherProgress.wire = serviceViewID(94), bytes.Clone(lower.wire)
	copy(otherProgress.wire[1:33], otherProgress.id[:])
	otherCommitment := serviceViewID(95)
	copy(otherProgress.wire[65:97], otherCommitment[:])
	binary.BigEndian.PutUint32(otherProgress.wire[225:229], 10000)
	putServiceViewFixture(t, h.ctx, h.hostKey, otherProgress)
	hostBefore, payBefore := payStoreSnapshot(h.ctx, h.hostKey), payStoreSnapshot(h.ctx, h.storeKey)
	for _, fixture := range fixtures {
		t.Run(fixture.prefix, func(t *testing.T) {
			got, found := fixture.view(h.keeper, h.ctx, fixture.id)
			require.True(t, found)
			require.Equal(t, fixture.want, got)
		})
	}
	latest, found := h.keeper.LatestDelivery(h.ctx, delivery.AgreementID)
	require.True(t, found)
	require.Equal(t, delivery, latest)
	highWater, found := h.keeper.ProgressHighWater(h.ctx, progress.CommitmentID)
	require.True(t, found)
	require.Equal(t, uint32(8000), highWater)
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, payBefore, payStoreSnapshot(h.ctx, h.storeKey))
	// Equal sequences select the last key in ascending kernel iteration.
	tied := older
	tied.wire = bytes.Clone(older.wire)
	binary.BigEndian.PutUint64(tied.wire[130:138], delivery.GlobalSequence)
	copy(tied.wire[97:129], otherAgreement[:])
	putServiceViewFixture(t, h.ctx, h.hostKey, tied)
	latest, found = h.keeper.LatestDelivery(h.ctx, delivery.AgreementID)
	require.True(t, found)
	delivery.Provider = otherAgreement
	require.Equal(t, delivery, latest)
	// Corrupt committed data is an invariant failure, never an unknown id.
	for _, fixture := range fixtures {
		t.Run(fixture.prefix+" corrupt", func(t *testing.T) {
			key := types.KernelKVKey(5, append([]byte(fixture.prefix), fixture.id[:]...))
			original := bytes.Clone(h.ctx.KVStore(h.hostKey).Get(key))
			h.ctx.KVStore(h.hostKey).Set(key, []byte{255})
			require.Panics(t, func() { fixture.view(h.keeper, h.ctx, fixture.id) })
			h.ctx.KVStore(h.hostKey).Set(key, original)
		})
	}
	itemKey := types.KernelKVKey(5, append(append([]byte("delitm:1"), deliveryFixture.id[:]...), byte(1)))
	h.ctx.KVStore(h.hostKey).Delete(itemKey)
	require.Panics(t, func() { h.keeper.GetDelivery(h.ctx, deliveryFixture.id) })
}

func TestServiceViewsUnknownEmpty(t *testing.T) {
	h := newPayHarness(t)
	fixtures := serviceViewFixtures(t)
	checkUnknown := func(id [32]byte) {
		t.Helper()
		for _, fixture := range fixtures {
			got, found := fixture.view(h.keeper, h.ctx, id)
			require.False(t, found, fixture.prefix)
			require.Zero(t, got, fixture.prefix)
		}
		latest, found := h.keeper.LatestDelivery(h.ctx, id)
		require.False(t, found)
		require.Equal(t, types.ServiceDelivery{}, latest)
		highWater, found := h.keeper.ProgressHighWater(h.ctx, id)
		require.False(t, found)
		require.Zero(t, highWater)
	}
	checkUnknown([32]byte{})
	for _, fixture := range fixtures {
		putServiceViewFixture(t, h.ctx, h.storeKey, fixture)
	}
	// Even complete records in the pay store cannot satisfy a service view.
	for _, fixture := range fixtures {
		checkUnknown(fixture.id)
	}
	for _, fixture := range fixtures {
		putServiceViewFixture(t, h.ctx, h.hostKey, fixture)
	}
	hostBefore, payBefore := payStoreSnapshot(h.ctx, h.hostKey), payStoreSnapshot(h.ctx, h.storeKey)
	checkUnknown(serviceViewID(200))
	checkUnknown([32]byte{})
	require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	require.Equal(t, payBefore, payStoreSnapshot(h.ctx, h.storeKey))
}
