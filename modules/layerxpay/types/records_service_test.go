package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

func serviceRecordMarker(seed byte) [32]byte {
	var hash [32]byte
	for i := range hash {
		hash[i] = seed + byte(i)
	}
	return hash
}

func serviceRecordBytes(t *testing.T, length int, fields ...any) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteByte(1)
	for _, field := range fields {
		if err := binary.Write(&b, binary.BigEndian, field); err != nil {
			t.Fatal(err)
		}
	}
	if b.Len() != length {
		t.Fatalf("fixture length = %d, want %d", b.Len(), length)
	}
	return b.Bytes()
}

func serviceRecordCheck(t *testing.T, decode func([]byte) (any, error), b []byte, want any) {
	t.Helper()
	got, err := decode(b)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded = %#v, error = %v; want %#v", got, err, want)
	}
	for i := range b {
		b[i] ^= 0xff
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("decoded record aliases its input")
	}
	for i := range b {
		b[i] ^= 0xff
	}
}

func serviceRecordReject(t *testing.T, decode func([]byte) (any, error), b []byte) {
	t.Helper()
	got, err := decode(b)
	if !errors.Is(err, errBudgetNonCanonical) || err.Error() != "LXP_NON_CANONICAL" {
		t.Fatalf("length %d: error = %v, want shared LXP_NON_CANONICAL", len(b), err)
	}
	if !reflect.ValueOf(got).IsZero() {
		t.Fatalf("invalid record returned partial data: %#v", got)
	}
}

func serviceRecordMalformed(t *testing.T, decode func([]byte) (any, error), b []byte) {
	t.Helper()
	serviceRecordReject(t, decode, nil)
	for length := 0; length < len(b); length++ {
		serviceRecordReject(t, decode, b[:length])
	}
	for _, extra := range []int{1, 32, 73} {
		serviceRecordReject(t, decode, append(bytes.Clone(b), make([]byte, extra)...))
	}
	for _, version := range []byte{0, 2, 255} {
		invalid := bytes.Clone(b)
		invalid[0] = version
		serviceRecordReject(t, decode, invalid)
	}
}

func TestDecodeServiceFixedRecords(t *testing.T) {
	offer := ServiceOffer{
		OfferID: serviceRecordMarker(1), ActivityID: serviceRecordMarker(2), OfferingAgent: serviceRecordMarker(3),
		AssetID: serviceRecordMarker(4), Price: codec.U128{Hi: 0x8172635445362718, Lo: 0xf1e2d3c4b5a69788},
		TermsHash: serviceRecordMarker(5), DeliverableSpecHash: serviceRecordMarker(6),
		DeliveryDeadlineMs: 0x1122334455667788, AcceptanceWindowMs: 0x2233445566778899,
		DisputeWindowMs: 0x33445566778899aa, DefaultOutcome: 2, OfferExpiryMs: 0x445566778899aabb,
		GlobalSequence: 0xfedcba9876543210, Withdrawn: true, Accepted: false,
	}
	offerBytes := serviceRecordBytes(t, 252,
		offer.OfferID, offer.ActivityID, offer.OfferingAgent, offer.AssetID, offer.Price.Bytes(),
		offer.TermsHash, offer.DeliverableSpecHash, offer.DeliveryDeadlineMs, offer.AcceptanceWindowMs,
		offer.DisputeWindowMs, offer.DefaultOutcome, offer.OfferExpiryMs, offer.GlobalSequence, offer.Withdrawn, offer.Accepted)
	commitment := ServiceCommitment{
		CommitmentID: serviceRecordMarker(11), ActivityID: serviceRecordMarker(12), Provider: serviceRecordMarker(13),
		AgreementID: serviceRecordMarker(14), TaskHash: serviceRecordMarker(15), DeadlineMs: 0x1122334455667788,
		ResourceBound: 0x8877665544332211, EscrowID: serviceRecordMarker(16), GlobalSequence: 0xfedcba9876543210,
		Abandoned: true, AbandonReason: 0x1234,
	}
	commitmentBytes := serviceRecordBytes(t, 220,
		commitment.CommitmentID, commitment.ActivityID, commitment.Provider, commitment.AgreementID, commitment.TaskHash,
		commitment.EscrowID, commitment.DeadlineMs, commitment.ResourceBound, commitment.GlobalSequence, commitment.Abandoned, commitment.AbandonReason)
	progress := ServiceProgress{
		ReportID: serviceRecordMarker(21), ActivityID: serviceRecordMarker(22), CommitmentID: serviceRecordMarker(23),
		AgreementID: serviceRecordMarker(24), Provider: serviceRecordMarker(25), NoteHash: serviceRecordMarker(26),
		AvailabilityReference: serviceRecordMarker(27), ProgressBPS: 9876, ReportedAtMs: 0x1122334455667788,
		GlobalSequence: 0xfedcba9876543210,
	}
	progressBytes := serviceRecordBytes(t, 245,
		progress.ReportID, progress.ActivityID, progress.CommitmentID, progress.AgreementID, progress.Provider,
		progress.NoteHash, progress.AvailabilityReference, progress.ProgressBPS, progress.ReportedAtMs, progress.GlobalSequence)
	execution := ServiceExecution{
		AttestationID: serviceRecordMarker(31), ActivityID: serviceRecordMarker(32), AgreementID: serviceRecordMarker(33),
		CommitmentID: serviceRecordMarker(34), ToolID: serviceRecordMarker(35), InputCommitmentHash: serviceRecordMarker(36),
		OutputCommitmentHash: serviceRecordMarker(37), ExecutionStart: 0x1122334455667788, ExecutionEnd: 0x2233445566778899,
		ResourceUnits: 0x8877665544332211, AttestorIdentity: serviceRecordMarker(38), AvailabilityReference: serviceRecordMarker(39),
		PublicKey: serviceRecordMarker(40), GlobalSequence: 0xfedcba9876543210,
	}
	for i := range execution.Signature {
		execution.Signature[i] = byte(i + 128)
	}
	executionBytes := serviceRecordBytes(t, 417,
		execution.AttestationID, execution.ActivityID, execution.AgreementID, execution.CommitmentID,
		execution.ToolID, execution.InputCommitmentHash, execution.OutputCommitmentHash, execution.ExecutionStart,
		execution.ExecutionEnd, execution.ResourceUnits, execution.AttestorIdentity, execution.AvailabilityReference,
		execution.PublicKey, execution.Signature, execution.GlobalSequence)
	preimage := serviceRecordBytes(t, 345,
		execution.AttestationID, execution.ActivityID, execution.AgreementID, execution.CommitmentID,
		execution.ToolID, execution.InputCommitmentHash, execution.OutputCommitmentHash, execution.ExecutionStart,
		execution.ExecutionEnd, execution.ResourceUnits, execution.AttestorIdentity, execution.AvailabilityReference, execution.PublicKey)
	expectedPayload := append([]byte("LXP:SERVICE:ATTEST:v1"), preimage[1:]...)
	execution.CanonicalPayloadLength = uint16(len(expectedPayload))
	copy(execution.CanonicalPayload[:], expectedPayload)
	cases := []struct {
		name   string
		wire   []byte
		want   any
		decode func([]byte) (any, error)
		flags  []int
	}{
		{"offer", offerBytes, offer, func(b []byte) (any, error) { return DecodeServiceOffer(b) }, []int{250, 251}},
		{"commitment", commitmentBytes, commitment, func(b []byte) (any, error) { return DecodeServiceCommitment(b) }, []int{217}},
		{"progress", progressBytes, progress, func(b []byte) (any, error) { return DecodeServiceProgress(b) }, nil},
		{"execution", executionBytes, execution, func(b []byte) (any, error) { return DecodeServiceExecution(b) }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serviceRecordCheck(t, tc.decode, tc.wire, tc.want)
			serviceRecordMalformed(t, tc.decode, tc.wire)
			for _, offset := range tc.flags {
				for _, flag := range []byte{2, 255} {
					invalid := bytes.Clone(tc.wire)
					invalid[offset] = flag
					serviceRecordReject(t, tc.decode, invalid)
				}
			}
		})
	}
	offer.Withdrawn, offer.Accepted = false, true
	offerBytes[250], offerBytes[251] = 0, 1
	serviceRecordCheck(t, cases[0].decode, offerBytes, offer)
	commitment.Abandoned, commitment.AbandonReason = false, 0
	commitmentBytes[217], commitmentBytes[218], commitmentBytes[219] = 0, 0, 0
	serviceRecordCheck(t, cases[1].decode, commitmentBytes, commitment)
	for _, layout := range []struct {
		value  any
		fields []string
	}{
		{ServiceOffer{}, []string{"OfferID", "ActivityID", "OfferingAgent", "AssetID", "Price", "TermsHash", "DeliverableSpecHash", "DeliveryDeadlineMs", "AcceptanceWindowMs", "DisputeWindowMs", "DefaultOutcome", "OfferExpiryMs", "GlobalSequence", "Withdrawn", "Accepted"}},
		{ServiceAgreement{}, []string{"AgreementID", "OfferID", "Provider", "Buyer", "TermsHash", "EscrowID", "DeliveryDeadlineMs", "AcceptanceWindowEndMs", "DisputeWindowEndMs", "DefaultOutcome", "State", "AcceptedSequence", "RejectionReason", "ContestedHashes", "DefaultApplied", "OutcomeSequence", "OutcomeTimestampMs"}},
		{ServiceCommitment{}, []string{"CommitmentID", "ActivityID", "Provider", "AgreementID", "TaskHash", "DeadlineMs", "ResourceBound", "EscrowID", "GlobalSequence", "Abandoned", "AbandonReason"}},
		{ServiceDelivery{}, []string{"AgreementID", "Provider", "Deliverables", "GlobalSequence"}},
		{ServiceProgress{}, []string{"ReportID", "ActivityID", "CommitmentID", "AgreementID", "Provider", "NoteHash", "AvailabilityReference", "ProgressBPS", "ReportedAtMs", "GlobalSequence"}},
		{ServiceDispute{}, []string{"DisputeID", "ActivityID", "AgreementID", "Raiser", "EvidenceHashes", "GlobalSequence", "Resolved", "Ruling", "ProviderBPS", "EscrowResolutionID", "ResolutionSequence"}},
	} {
		typ := reflect.TypeOf(layout.value)
		if typ.NumField() != len(layout.fields) {
			t.Fatalf("%s has %d fields, want %d", typ.Name(), typ.NumField(), len(layout.fields))
		}
		for i, name := range layout.fields {
			if typ.Field(i).Name != name {
				t.Fatalf("%s field %d = %s, want %s in Solidity order", typ.Name(), i, typ.Field(i).Name, name)
			}
		}
	}
}

func serviceAgreementFixture(t *testing.T, count int) ([]byte, any) {
	t.Helper()
	r := ServiceAgreement{
		AgreementID: serviceRecordMarker(51), OfferID: serviceRecordMarker(52), Provider: serviceRecordMarker(53),
		Buyer: serviceRecordMarker(54), TermsHash: serviceRecordMarker(55), EscrowID: serviceRecordMarker(56),
		DeliveryDeadlineMs: 0x1122334455667788, AcceptanceWindowEndMs: 0x2233445566778899,
		DisputeWindowEndMs: 0x33445566778899aa, DefaultOutcome: 2, State: 5, AcceptedSequence: 0x445566778899aabb,
		RejectionReason: 0x1234, ContestedHashes: make([][32]byte, count), DefaultApplied: true,
		OutcomeSequence: 0x5566778899aabbcc, OutcomeTimestampMs: 0x66778899aabbccdd,
	}
	for i := range r.ContestedHashes {
		r.ContestedHashes[i] = serviceRecordMarker(byte(100 - i))
	}
	b := serviceRecordBytes(t, 247+32*count,
		r.AgreementID, r.OfferID, r.Provider, r.Buyer, r.TermsHash, r.EscrowID, r.DeliveryDeadlineMs,
		r.AcceptanceWindowEndMs, r.DisputeWindowEndMs, r.DefaultOutcome, r.State, r.AcceptedSequence, r.RejectionReason,
		r.DefaultApplied, r.OutcomeSequence, r.OutcomeTimestampMs, byte(count), r.ContestedHashes)
	return b, r
}

func serviceDisputeFixture(t *testing.T, count int) ([]byte, any) {
	t.Helper()
	r := ServiceDispute{
		DisputeID: serviceRecordMarker(61), ActivityID: serviceRecordMarker(62), AgreementID: serviceRecordMarker(63),
		Raiser: serviceRecordMarker(64), EvidenceHashes: make([][32]byte, count), GlobalSequence: 0xfedcba9876543210,
		Resolved: true, Ruling: 0x1234, ProviderBPS: 8765, EscrowResolutionID: serviceRecordMarker(65),
		ResolutionSequence: 0x1122334455667788,
	}
	for i := range r.EvidenceHashes {
		r.EvidenceHashes[i] = serviceRecordMarker(byte(120 - i))
	}
	b := serviceRecordBytes(t, 185+32*count,
		r.DisputeID, r.ActivityID, r.AgreementID, r.Raiser, r.GlobalSequence, r.Resolved, r.Ruling, r.ProviderBPS,
		r.EscrowResolutionID, r.ResolutionSequence, byte(count), r.EvidenceHashes)
	return b, r
}

func serviceDeliveryFixture(t *testing.T, count int) ([]byte, any) {
	t.Helper()
	r := ServiceDelivery{
		AgreementID: serviceRecordMarker(73), Provider: serviceRecordMarker(74),
		Deliverables: make([]ServiceDeliverable, count), GlobalSequence: 0xfedcba9876543210,
	}
	b := serviceRecordBytes(t, 138, serviceRecordMarker(71), serviceRecordMarker(72), r.AgreementID, r.Provider, byte(count), r.GlobalSequence)
	for i := range r.Deliverables {
		item := ServiceDeliverable{
			Hash: serviceRecordMarker(byte(160 - i)), ArtifactSize: 0x1122334455667788 + uint64(i),
			AvailabilityReference: serviceRecordMarker(byte(200 - i)),
		}
		r.Deliverables[i] = item
		b = append(b, serviceRecordBytes(t, 73, item.Hash, item.ArtifactSize, item.AvailabilityReference)...)
	}
	return b, r
}

func TestDecodeServiceVariableRecords(t *testing.T) {
	cases := []struct {
		name        string
		fixture     func(*testing.T, int) ([]byte, any)
		decode      func([]byte) (any, error)
		countOffset int
		flagOffset  int
	}{
		{"agreement", serviceAgreementFixture, func(b []byte) (any, error) { return DecodeServiceAgreement(b) }, 246, 229},
		{"dispute", serviceDisputeFixture, func(b []byte) (any, error) { return DecodeServiceDispute(b) }, 184, 137},
		{"delivery", serviceDeliveryFixture, func(b []byte) (any, error) { return DecodeServiceDelivery(b) }, 129, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for count := 0; count <= ServiceMaxDeliverables; count++ {
				t.Run(fmt.Sprintf("count_%d", count), func(t *testing.T) {
					b, want := tc.fixture(t, count)
					serviceRecordCheck(t, tc.decode, b, want)
					serviceRecordMalformed(t, tc.decode, b)
					for _, wrongCount := range []byte{0, 1, 2, 3, 16, 17, 255} {
						if int(wrongCount) != count {
							invalid := bytes.Clone(b)
							invalid[tc.countOffset] = wrongCount
							serviceRecordReject(t, tc.decode, invalid)
						}
					}
					if tc.flagOffset >= 0 {
						for _, flag := range []byte{2, 255} {
							invalid := bytes.Clone(b)
							invalid[tc.flagOffset] = flag
							serviceRecordReject(t, tc.decode, invalid)
						}
					} else {
						for i := 0; i < count; i++ {
							for _, version := range []byte{0, 2, 255} {
								invalid := bytes.Clone(b)
								invalid[138+73*i] = version
								serviceRecordReject(t, tc.decode, invalid)
							}
						}
					}
				})
			}
			for _, count := range []int{17, 255} {
				b, _ := tc.fixture(t, count)
				serviceRecordReject(t, tc.decode, b)
			}
		})
	}
	b, want := serviceAgreementFixture(t, 2)
	agreement := want.(ServiceAgreement)
	agreement.DefaultApplied, b[229] = false, 0
	serviceRecordCheck(t, cases[0].decode, b, agreement)
	b, want = serviceDisputeFixture(t, 3)
	dispute := want.(ServiceDispute)
	dispute.Resolved, b[137] = false, 0
	serviceRecordCheck(t, cases[1].decode, b, dispute)
}
