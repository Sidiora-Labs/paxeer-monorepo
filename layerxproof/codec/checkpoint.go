package codec

import (
	"crypto/sha256"
	"encoding/binary"
)

const (
	// CheckpointWireVersion is the evidence wire version of the checkpoint
	// payload and of the settlement reference (EVIDENCE_WIRE_VERSION).
	CheckpointWireVersion uint16 = 1
	// MaxSettlementReferenceBytes is LXP_MAX_SETTLEMENT_REFERENCE_BYTES.
	MaxSettlementReferenceBytes = 1024
	// SettlementReferenceBytes is the exact width of a present reference.
	SettlementReferenceBytes = 110
	// GuarantorAttestationBytes is the wire width of one attestation: the
	// 189-byte signed statement, the signer address, the signature and v.
	GuarantorAttestationBytes = 274
	// GuarantorAttestationMessageBytes is LXP_ATTESTATION_MESSAGE_BYTES.
	GuarantorAttestationMessageBytes = 189
	// AvailabilityAll is LXP_GUARANTOR_AVAILABILITY_ALL: activities, receipts,
	// oracle inputs, state diff and recovery metadata.
	AvailabilityAll uint8 = 0x1f
)

// Availability classes of lxp_da_class, as bits of the attestation mask.
const (
	AvailabilityActivities       uint8 = 1 << 0
	AvailabilityReceipts         uint8 = 1 << 1
	AvailabilityOracleInputs     uint8 = 1 << 2
	AvailabilityStateDiff        uint8 = 1 << 3
	AvailabilityRecoveryMetadata uint8 = 1 << 4
)

// GuarantorAttestation is one guarantor's signed statement over a checkpoint
// (lxp_guarantor_attestation). The signature is secp256k1 over
// SHA256("LXP/v2/guarantor-attestation\0" || 189-byte statement) with an EVM
// recovery identifier, so the signer is recoverable with ecrecover.
type GuarantorAttestation struct {
	ProtocolVersion       uint16
	NetworkID             uint32
	PaxeerChainID         uint64
	SettlementContract    [20]byte
	Epoch                 uint64
	CheckpointID          [32]byte
	CheckpointHash        [32]byte
	GuarantorID           [32]byte
	BatchNumber           uint64
	DataAvailabilityRoot  [32]byte
	Replayed              bool
	DataPossessed         bool
	AvailabilityClassMask uint8
	AttestedAtMs          uint64
	Signer                [20]byte
	Signature             [64]byte
	SignatureV            uint8
	message               [GuarantorAttestationMessageBytes]byte
}

// SigningMessage returns the exact 189-byte signed statement.
func (a *GuarantorAttestation) SigningMessage() []byte {
	return append([]byte(nil), a.message[:]...)
}

// Digest is the secp256k1 signing digest of the statement.
func (a *GuarantorAttestation) Digest() [32]byte {
	return mustDomainHash(DomainGuarantorAttestation, a.message[:])
}

// SettlementReference is the 110-byte Paxeer settlement reference a
// certificate may carry once its registration was observed.
type SettlementReference struct {
	PaxeerChainID       uint64
	SettlementContract  [20]byte
	CheckpointID        [32]byte
	TransactionID       [32]byte
	ObservedBlockNumber uint64
	ObservedAtMs        uint64
}

// DecodeSettlementReference mirrors settlement_reference_decode.
func DecodeSettlementReference(encoded []byte) (*SettlementReference, error) {
	if len(encoded) != SettlementReferenceBytes {
		return nil, ErrNonCanonical
	}
	if binary.BigEndian.Uint16(encoded[0:2]) != CheckpointWireVersion {
		return nil, ErrVersionUnsupported
	}
	s := &SettlementReference{}
	s.PaxeerChainID = binary.BigEndian.Uint64(encoded[2:10])
	copy(s.SettlementContract[:], encoded[10:30])
	copy(s.CheckpointID[:], encoded[30:62])
	copy(s.TransactionID[:], encoded[62:94])
	s.ObservedBlockNumber = binary.BigEndian.Uint64(encoded[94:102])
	s.ObservedAtMs = binary.BigEndian.Uint64(encoded[102:110])
	if s.PaxeerChainID == 0 || s.ObservedBlockNumber == 0 || s.ObservedAtMs == 0 ||
		s.SettlementContract == ([20]byte{}) || s.CheckpointID == ([32]byte{}) || s.TransactionID == ([32]byte{}) {
		return nil, ErrNonCanonical
	}
	return s, nil
}

// CheckpointCertificate is the guarantor checkpoint certificate
// (lxp_guarantor_cert in the layerxd checkpoint payload form): the canonical
// batch header, the validity proof, up to 32 attestations strictly ascending
// by guarantor identifier, the declared threshold, and the settlement
// reference, which is empty before settlement and 110 bytes after it.
type CheckpointCertificate struct {
	HeaderBytes         []byte
	Header              *BatchHeader
	ValidityProof       []byte
	Attestations        []*GuarantorAttestation
	Threshold           uint8
	SettlementReference []byte
	Settlement          *SettlementReference
}

// CheckpointHash is lxp_checkpoint_certificate_hash:
// SHA256("LXP/v2/checkpoint-certificate\0" || header || u32be(len(proof)) || proof).
func CheckpointHash(header, validityProof []byte) [32]byte {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(validityProof))) //nolint:gosec
	h := sha256.New()
	h.Write([]byte(domainTags[DomainCheckpointCertificate]))
	h.Write(header)
	h.Write(length[:])
	h.Write(validityProof)
	var out [32]byte
	h.Sum(out[:0])
	return out
}

// CheckpointID returns the certificate's checkpoint hash.
func (c *CheckpointCertificate) CheckpointID() [32]byte {
	return CheckpointHash(c.HeaderBytes, c.ValidityProof)
}
