package types

import (
	"encoding/binary"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

type ServiceOffer struct {
	OfferID             [32]byte
	ActivityID          [32]byte
	OfferingAgent       [32]byte
	AssetID             [32]byte
	Price               codec.U128
	TermsHash           [32]byte
	DeliverableSpecHash [32]byte
	DeliveryDeadlineMs  uint64
	AcceptanceWindowMs  uint64
	DisputeWindowMs     uint64
	DefaultOutcome      uint8
	OfferExpiryMs       uint64
	GlobalSequence      uint64
	Withdrawn           bool
	Accepted            bool
}

type ServiceAgreement struct {
	AgreementID           [32]byte
	OfferID               [32]byte
	Provider              [32]byte
	Buyer                 [32]byte
	TermsHash             [32]byte
	EscrowID              [32]byte
	DeliveryDeadlineMs    uint64
	AcceptanceWindowEndMs uint64
	DisputeWindowEndMs    uint64
	DefaultOutcome        uint8
	State                 uint8
	AcceptedSequence      uint64
	RejectionReason       uint16
	ContestedHashes       [][32]byte
	DefaultApplied        bool
	OutcomeSequence       uint64
	OutcomeTimestampMs    uint64
}

type ServiceCommitment struct {
	CommitmentID   [32]byte
	ActivityID     [32]byte
	Provider       [32]byte
	AgreementID    [32]byte
	TaskHash       [32]byte
	DeadlineMs     uint64
	ResourceBound  uint64
	EscrowID       [32]byte
	GlobalSequence uint64
	Abandoned      bool
	AbandonReason  uint16
}

type ServiceExecution struct {
	AttestationID          [32]byte
	ActivityID             [32]byte
	AgreementID            [32]byte
	CommitmentID           [32]byte
	ToolID                 [32]byte
	InputCommitmentHash    [32]byte
	OutputCommitmentHash   [32]byte
	ExecutionStart         uint64
	ExecutionEnd           uint64
	ResourceUnits          uint64
	AttestorIdentity       [32]byte
	AvailabilityReference  [32]byte
	PublicKey              [32]byte
	Signature              [64]byte
	GlobalSequence         uint64
	CanonicalPayloadLength uint16
	CanonicalPayload       [384]byte
}

type ServiceDelivery struct {
	AgreementID    [32]byte
	Provider       [32]byte
	Deliverables   []ServiceDeliverable
	GlobalSequence uint64
}

type ServiceProgress struct {
	ReportID              [32]byte
	ActivityID            [32]byte
	CommitmentID          [32]byte
	AgreementID           [32]byte
	Provider              [32]byte
	NoteHash              [32]byte
	AvailabilityReference [32]byte
	ProgressBPS           uint32
	ReportedAtMs          uint64
	GlobalSequence        uint64
}

type ServiceDispute struct {
	DisputeID          [32]byte
	ActivityID         [32]byte
	AgreementID        [32]byte
	Raiser             [32]byte
	EvidenceHashes     [][32]byte
	GlobalSequence     uint64
	Resolved           bool
	Ruling             uint16
	ProviderBPS        uint32
	EscrowResolutionID [32]byte
	ResolutionSequence uint64
}

func DecodeServiceOffer(b []byte) (ServiceOffer, error) {
	if len(b) != 252 || b[0] != 1 || b[250] > 1 || b[251] > 1 {
		return ServiceOffer{}, errBudgetNonCanonical
	}
	return ServiceOffer{
		OfferID: [32]byte(b[1:33]), ActivityID: [32]byte(b[33:65]),
		OfferingAgent: [32]byte(b[65:97]), AssetID: [32]byte(b[97:129]),
		Price:     codec.U128{Hi: binary.BigEndian.Uint64(b[129:137]), Lo: binary.BigEndian.Uint64(b[137:145])},
		TermsHash: [32]byte(b[145:177]), DeliverableSpecHash: [32]byte(b[177:209]),
		DeliveryDeadlineMs: binary.BigEndian.Uint64(b[209:217]),
		AcceptanceWindowMs: binary.BigEndian.Uint64(b[217:225]),
		DisputeWindowMs:    binary.BigEndian.Uint64(b[225:233]), DefaultOutcome: b[233],
		OfferExpiryMs:  binary.BigEndian.Uint64(b[234:242]),
		GlobalSequence: binary.BigEndian.Uint64(b[242:250]), Withdrawn: b[250] == 1, Accepted: b[251] == 1,
	}, nil
}

func DecodeServiceAgreement(b []byte) (ServiceAgreement, error) {
	if len(b) < 247 || b[0] != 1 || b[229] > 1 {
		return ServiceAgreement{}, errBudgetNonCanonical
	}
	hashes, err := serviceRecordHashes(b[247:], int(b[246]))
	if err != nil {
		return ServiceAgreement{}, err
	}
	return ServiceAgreement{
		AgreementID: [32]byte(b[1:33]), OfferID: [32]byte(b[33:65]),
		Provider: [32]byte(b[65:97]), Buyer: [32]byte(b[97:129]),
		TermsHash: [32]byte(b[129:161]), EscrowID: [32]byte(b[161:193]),
		DeliveryDeadlineMs:    binary.BigEndian.Uint64(b[193:201]),
		AcceptanceWindowEndMs: binary.BigEndian.Uint64(b[201:209]),
		DisputeWindowEndMs:    binary.BigEndian.Uint64(b[209:217]),
		DefaultOutcome:        b[217], State: b[218], AcceptedSequence: binary.BigEndian.Uint64(b[219:227]),
		RejectionReason: binary.BigEndian.Uint16(b[227:229]), ContestedHashes: hashes, DefaultApplied: b[229] == 1,
		OutcomeSequence: binary.BigEndian.Uint64(b[230:238]), OutcomeTimestampMs: binary.BigEndian.Uint64(b[238:246]),
	}, nil
}

func DecodeServiceCommitment(b []byte) (ServiceCommitment, error) {
	if len(b) != 220 || b[0] != 1 || b[217] > 1 {
		return ServiceCommitment{}, errBudgetNonCanonical
	}
	return ServiceCommitment{
		CommitmentID: [32]byte(b[1:33]), ActivityID: [32]byte(b[33:65]),
		Provider: [32]byte(b[65:97]), AgreementID: [32]byte(b[97:129]),
		TaskHash: [32]byte(b[129:161]), DeadlineMs: binary.BigEndian.Uint64(b[193:201]),
		ResourceBound: binary.BigEndian.Uint64(b[201:209]), EscrowID: [32]byte(b[161:193]),
		GlobalSequence: binary.BigEndian.Uint64(b[209:217]), Abandoned: b[217] == 1,
		AbandonReason: binary.BigEndian.Uint16(b[218:220]),
	}, nil
}

func DecodeServiceExecution(b []byte) (ServiceExecution, error) {
	if len(b) != ServiceExecutionBytes+1 || b[0] != 1 {
		return ServiceExecution{}, errBudgetNonCanonical
	}
	r := ServiceExecution{
		AttestationID: [32]byte(b[1:33]), ActivityID: [32]byte(b[33:65]),
		AgreementID: [32]byte(b[65:97]), CommitmentID: [32]byte(b[97:129]),
		ToolID: [32]byte(b[129:161]), InputCommitmentHash: [32]byte(b[161:193]),
		OutputCommitmentHash: [32]byte(b[193:225]), ExecutionStart: binary.BigEndian.Uint64(b[225:233]),
		ExecutionEnd: binary.BigEndian.Uint64(b[233:241]), ResourceUnits: binary.BigEndian.Uint64(b[241:249]),
		AttestorIdentity: [32]byte(b[249:281]), AvailabilityReference: [32]byte(b[281:313]),
		PublicKey: [32]byte(b[313:345]), Signature: [64]byte(b[345:409]), GlobalSequence: binary.BigEndian.Uint64(b[409:417]),
	}
	tag := "LXP:SERVICE:ATTEST:v1"
	r.CanonicalPayloadLength = uint16(copy(r.CanonicalPayload[:], tag) + copy(r.CanonicalPayload[len(tag):], b[1:345]))
	return r, nil
}

func DecodeServiceDelivery(b []byte) (ServiceDelivery, error) {
	if len(b) < 138 || b[0] != 1 {
		return ServiceDelivery{}, errBudgetNonCanonical
	}
	count := int(b[129])
	if count > ServiceMaxDeliverables || len(b) != 138+73*count {
		return ServiceDelivery{}, errBudgetNonCanonical
	}
	items := make([]ServiceDeliverable, count)
	for i := range items {
		offset := 138 + 73*i
		if b[offset] != 1 {
			return ServiceDelivery{}, errBudgetNonCanonical
		}
		items[i] = ServiceDeliverable{
			Hash: [32]byte(b[offset+1 : offset+33]), ArtifactSize: binary.BigEndian.Uint64(b[offset+33 : offset+41]),
			AvailabilityReference: [32]byte(b[offset+41 : offset+73]),
		}
	}
	return ServiceDelivery{
		AgreementID: [32]byte(b[65:97]), Provider: [32]byte(b[97:129]),
		Deliverables: items, GlobalSequence: binary.BigEndian.Uint64(b[130:138]),
	}, nil
}

func DecodeServiceProgress(b []byte) (ServiceProgress, error) {
	if len(b) != 245 || b[0] != 1 {
		return ServiceProgress{}, errBudgetNonCanonical
	}
	return ServiceProgress{
		ReportID: [32]byte(b[1:33]), ActivityID: [32]byte(b[33:65]),
		CommitmentID: [32]byte(b[65:97]), AgreementID: [32]byte(b[97:129]),
		Provider: [32]byte(b[129:161]), NoteHash: [32]byte(b[161:193]),
		AvailabilityReference: [32]byte(b[193:225]), ProgressBPS: binary.BigEndian.Uint32(b[225:229]),
		ReportedAtMs: binary.BigEndian.Uint64(b[229:237]), GlobalSequence: binary.BigEndian.Uint64(b[237:245]),
	}, nil
}

func DecodeServiceDispute(b []byte) (ServiceDispute, error) {
	if len(b) < 185 || b[0] != 1 || b[137] > 1 {
		return ServiceDispute{}, errBudgetNonCanonical
	}
	hashes, err := serviceRecordHashes(b[185:], int(b[184]))
	if err != nil {
		return ServiceDispute{}, err
	}
	return ServiceDispute{
		DisputeID: [32]byte(b[1:33]), ActivityID: [32]byte(b[33:65]),
		AgreementID: [32]byte(b[65:97]), Raiser: [32]byte(b[97:129]), EvidenceHashes: hashes,
		GlobalSequence: binary.BigEndian.Uint64(b[129:137]), Resolved: b[137] == 1,
		Ruling: binary.BigEndian.Uint16(b[138:140]), ProviderBPS: binary.BigEndian.Uint32(b[140:144]),
		EscrowResolutionID: [32]byte(b[144:176]), ResolutionSequence: binary.BigEndian.Uint64(b[176:184]),
	}, nil
}

func serviceRecordHashes(b []byte, count int) ([][32]byte, error) {
	if count > ServiceMaxDeliverables || len(b) != 32*count {
		return nil, errBudgetNonCanonical
	}
	hashes := make([][32]byte, count)
	for i := range hashes {
		hashes[i] = [32]byte(b[32*i : 32*(i+1)])
	}
	return hashes, nil
}
