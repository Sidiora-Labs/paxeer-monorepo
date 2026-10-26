package layerx

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"math/bits"
	"strings"
	"unicode/utf8"
)

var (
	merkleLeafDomain           = []byte("LXP/v1/merkle-leaf\x00")
	merkleInternalDomain       = []byte("LXP/v1/merkle-internal\x00")
	batchHeaderDomain          = []byte("LXP/v1/batch-header\x00")
	receiptDomain              = []byte("LXP/v1/receipt\x00")
	checkpointDomain           = []byte("LXP/v2/checkpoint-certificate\x00")
	guarantorAttestationDomain = []byte("LXP/v2/guarantor-attestation\x00")
)

const (
	currentProtocolVersion = uint16(2)
	maximumMessageBytes    = 1_048_576
	maximumEffects         = 512
	maximumEffectBody      = 256
	batchHeaderBytes       = 354
	allAvailabilityClasses = 0x1f
)

const ProgramsModuleID uint16 = 9

const (
	ProgramOutcomeTagV1 uint32 = 0x50524731
	ProgramOutcomeTagV2 uint32 = 0x50524732
	ProgramOutcomeTagV3 uint32 = 0x50524733
	ProgramOutcomeTagV4 uint32 = 0x50524734
)

const (
	ReceiptCheckReceiptShape       ReceiptCheck = "receipt-shape"
	ReceiptCheckMissingSignature   ReceiptCheck = "missing-signature"
	ReceiptCheckProtocolVersion    ReceiptCheck = "protocol-version"
	ReceiptCheckResultCode         ReceiptCheck = "result-code"
	ReceiptCheckOperation          ReceiptCheck = "operation"
	ReceiptCheckGlobalSequence     ReceiptCheck = "global-sequence"
	ReceiptCheckModuleID           ReceiptCheck = "module-id"
	ReceiptCheckModuleVersion      ReceiptCheck = "module-version"
	ReceiptCheckTimestamp          ReceiptCheck = "timestamp"
	ReceiptCheckBatchID            ReceiptCheck = "batch-id"
	ReceiptCheckAsset              ReceiptCheck = "asset"
	ReceiptCheckPreviousStateRoot  ReceiptCheck = "previous-state-root"
	ReceiptCheckResultingStateRoot ReceiptCheck = "resulting-state-root"
	ReceiptCheckDebitBalance       ReceiptCheck = "debit-balance"
	ReceiptCheckCreditBalance      ReceiptCheck = "credit-balance"
	ReceiptCheckProgramOutcome     ReceiptCheck = "program-outcome"
	ReceiptCheckSequencerSignature ReceiptCheck = "sequencer-signature"
)

var RequiredNonzeroChecks = [...]ReceiptCheck{
	ReceiptCheckGlobalSequence,
	ReceiptCheckModuleID,
	ReceiptCheckModuleVersion,
	ReceiptCheckTimestamp,
	ReceiptCheckActivityID,
	ReceiptCheckResultingStateRoot,
}

type VerificationError struct{ Check ReceiptCheck }

func (*VerificationError) Error() string { return "Local verification failed." }
func (*VerificationError) Unwrap() error { return ErrVerification }

func receiptFailure(check ReceiptCheck) error { return &VerificationError{Check: check} }
func verificationFailure() error              { return newSDKError(ErrorVerificationFailure, RetryNever) }

type AuthorizedBatch struct {
	BatchID            [32]byte
	Asset              [32]byte
	PreviousStateRoot  [32]byte
	ResultingStateRoot [32]byte
	SequencerPublicKey [32]byte
}

type ProtocolReceiptFacts struct {
	ResultCode int32
	Asset      [32]byte
	Amount     Uint128
	FeeCharged Uint128
}

type ReceiptEffect struct {
	ModuleID        uint16
	Ordinal         uint16
	EventType       uint16
	Kind            uint8
	Monetary        bool
	TransferSetRoot [32]byte
	Body            []byte
}

type ProgramReceiptOutcome struct {
	EncodingVersion         uint8
	TerminalKind            uint8
	ResultCode              int32
	RuntimeVersion          uint16
	ABIVersion              uint16
	FeeScheduleVersion      uint32
	MeteringScheduleVersion uint32
	CPUFuel                 uint64
	MemoryBytes             uint64
	StorageReadBytes        uint64
	StorageWriteBytes       uint64
	OutputValues            uint32
	OutputBytes             uint64
	OccupancyByteBatches    Uint128
	OccupancyFeeUnits       Uint128
	FeeSchedulePrices       [7]uint64
	OccupancyAssetID        [32]byte
	OccupancyEvidenceDigest [32]byte
	OccupancyTransferRoot   [32]byte
	FeeUnits                Uint128
	CallGraphRoot           [32]byte
	TerminalPayloadRoot     [32]byte
	TransferRoot            [32]byte
	AppliedLegsDigest       [32]byte
}

type ProtocolReceipt struct {
	ProtocolVersion    uint16
	ActivityID         [32]byte
	GlobalSequence     uint64
	PreviousStateRoot  [32]byte
	ResultingStateRoot [32]byte
	ActivityRoot       [32]byte
	ResultCode         int32
	Effects            []ReceiptEffect
	FeeCharged         Uint128
	BatchID            [32]byte
	ModuleID           uint16
	ModuleVersion      uint32
	ParameterVersion   uint32
	Operation          uint8
	Asset              [32]byte
	Amount             Uint128
	From               [32]byte
	FromBalanceBefore  Uint128
	FromBalanceAfter   Uint128
	FromSequence       uint64
	To                 [32]byte
	ToBalanceBefore    Uint128
	ToBalanceAfter     Uint128
	TransferSetRoot    [32]byte
	AuthorizationHash  [32]byte
	ContextHash        [32]byte
	Timestamp          uint64
	ProgramOutcome     *ProgramReceiptOutcome
	SequencerSignature [64]byte
}

type VerifiedReceipt struct {
	Level          string
	Receipt        ProtocolReceipt
	CanonicalBytes []byte
	ReceiptDigest  [32]byte
	Facts          ProtocolReceiptFacts
}

type decodedReceipt struct {
	protocolVersion    uint16
	activityID         [32]byte
	previousStateRoot  [32]byte
	resultingStateRoot [32]byte
	resultCode         int32
	feeCharged         Uint128
	batchID            [32]byte
	operation          uint8
	asset              [32]byte
	amount             Uint128
	debitBefore        Uint128
	debitAfter         Uint128
	creditBefore       Uint128
	creditAfter        Uint128
	signature          [64]byte
	unsigned           []byte
	protocol           ProtocolReceipt
}

func VerifyReceiptOutcome(canonicalReceipt []byte, authorized AuthorizedBatch, selectedProtocol ...uint16) (VerifiedReceipt, error) {
	return verifyReceiptOutcomeProfile(canonicalReceipt, authorized, false, selectedProtocol...)
}

func verifyReceiptOutcomeProfile(canonicalReceipt []byte, authorized AuthorizedBatch, nativeV5 bool, selectedProtocol ...uint16) (VerifiedReceipt, error) {
	receipt, err := decodeProtocolReceipt(canonicalReceipt)
	if err != nil {
		return VerifiedReceipt{}, err
	}
	if !selectedProtocolMatches(receipt.protocolVersion, selectedProtocol) {
		return VerifiedReceipt{}, receiptFailure(ReceiptCheckProtocolVersion)
	}
	program := receipt.protocol.ModuleID == ProgramsModuleID && (receipt.operation == 0 || receipt.operation == 3)
	if program {
		module := receipt.protocol.ModuleVersion
		validModule := receipt.protocolVersion == 3 && (module == 4 || module == 5) || receipt.protocolVersion == 2 && (module == 2 || module == 3 || receipt.operation == 3 && module == 1)
		if !validModule {
			return VerifiedReceipt{}, receiptFailure(ReceiptCheckModuleVersion)
		}
		if receipt.operation == 0 && receipt.resultCode != 0 {
			return VerifiedReceipt{}, receiptFailure(ReceiptCheckResultCode)
		}
		if receipt.operation == 3 {
			outcome := receipt.protocol.ProgramOutcome
			if outcome == nil {
				return VerifiedReceipt{}, receiptFailure(ReceiptCheckReceiptShape)
			}
			nativeABI := nativeV5 && receipt.protocolVersion == 3 && (outcome.ABIVersion == ProgramAbiV3 || outcome.ABIVersion == ProgramAbiV4 || outcome.ABIVersion == ProgramAbiV5)
			if (!nativeABI && outcome.ABIVersion != 1 && outcome.ABIVersion != 2) || outcome.RuntimeVersion != 1 {
				return VerifiedReceipt{}, receiptFailure(ReceiptCheckProtocolVersion)
			}
			if nativeABI && uint32(outcome.ABIVersion) > module {
				return VerifiedReceipt{}, receiptFailure(ReceiptCheckModuleVersion)
			}
		}
	}
	if !program && receipt.operation == 0 {
		return VerifiedReceipt{}, receiptFailure(ReceiptCheckOperation)
	}
	if zero32(receipt.activityID) {
		return VerifiedReceipt{}, receiptFailure(ReceiptCheckActivityID)
	}
	if receipt.batchID != authorized.BatchID {
		return VerifiedReceipt{}, receiptFailure(ReceiptCheckBatchID)
	}
	if !program && (receipt.asset != authorized.Asset || zero32(receipt.asset)) {
		return VerifiedReceipt{}, receiptFailure(ReceiptCheckAsset)
	}
	if receipt.previousStateRoot != authorized.PreviousStateRoot {
		return VerifiedReceipt{}, receiptFailure(ReceiptCheckPreviousStateRoot)
	}
	if receipt.resultingStateRoot != authorized.ResultingStateRoot {
		return VerifiedReceipt{}, receiptFailure(ReceiptCheckResultingStateRoot)
	}
	if !program && receipt.resultCode == 0 {
		debitAfter, ok := receipt.debitBefore.Sub(receipt.amount)
		if !ok || !debitAfter.Equal(receipt.debitAfter) {
			return VerifiedReceipt{}, receiptFailure(ReceiptCheckDebitBalance)
		}
		creditAfter, ok := receipt.creditBefore.Add(receipt.amount)
		if !ok || !creditAfter.Equal(receipt.creditAfter) {
			return VerifiedReceipt{}, receiptFailure(ReceiptCheckCreditBalance)
		}
	}
	digest := domainDigest(receiptDomain, receipt.unsigned)
	if !ed25519.Verify(authorized.SequencerPublicKey[:], digest[:], receipt.signature[:]) {
		return VerifiedReceipt{}, receiptFailure(ReceiptCheckSequencerSignature)
	}
	return VerifiedReceipt{
		Level:          "sequencer-signed",
		Receipt:        receipt.protocol,
		CanonicalBytes: append([]byte(nil), canonicalReceipt...),
		ReceiptDigest:  digest,
		Facts: ProtocolReceiptFacts{
			ResultCode: receipt.resultCode,
			Asset:      receipt.asset,
			Amount:     receipt.amount,
			FeeCharged: receipt.feeCharged,
		},
	}, nil
}

func VerifyProgramLifecycleReceipt(canonical []byte, expectedActivity [32]byte, sequencer [32]byte) (VerifiedReceipt, error) {
	decoded, err := decodeProtocolReceipt(canonical)
	if err != nil {
		return VerifiedReceipt{}, err
	}
	receipt := decoded.protocol
	if receipt.ProtocolVersion != 3 || receipt.ModuleID != 9 || receipt.ModuleVersion != 4 || receipt.Operation != 0 || receipt.ProgramOutcome != nil || receipt.ActivityID != expectedActivity || zero32(expectedActivity) || zero32(sequencer) {
		return VerifiedReceipt{}, receiptFailure(ReceiptCheckReceiptShape)
	}
	digest := domainDigest(receiptDomain, decoded.unsigned)
	if !ed25519.Verify(sequencer[:], digest[:], decoded.signature[:]) {
		return VerifiedReceipt{}, receiptFailure(ReceiptCheckSequencerSignature)
	}
	return VerifiedReceipt{Level: "sequencer-signed", Receipt: receipt, CanonicalBytes: append([]byte{}, canonical...), ReceiptDigest: digest,
		Facts: ProtocolReceiptFacts{ResultCode: receipt.ResultCode, Asset: receipt.Asset, Amount: receipt.Amount, FeeCharged: receipt.FeeCharged}}, nil
}

func VerifyReceipt(canonicalReceipt []byte, authorized AuthorizedBatch, selectedProtocol ...uint16) (VerifiedReceipt, error) {
	verified, err := VerifyReceiptOutcome(canonicalReceipt, authorized, selectedProtocol...)
	if err != nil {
		return VerifiedReceipt{}, err
	}
	if verified.Facts.ResultCode != 0 {
		return VerifiedReceipt{}, receiptFailure(ReceiptCheckResultCode)
	}
	return verified, nil
}

func decodeProtocolReceipt(value []byte) (decodedReceipt, error) {
	return decodeReceiptProfile(value, true)
}

func decodeReceiptProfile(value []byte, kernel bool) (decodedReceipt, error) {
	if len(value) > maximumMessageBytes || len(value) < 4 || !(bytes.Equal(value[:4], []byte{0, 1, 0x52, 1}) || bytes.Equal(value[:4], []byte{0, 2, 0x52, 1}) || bytes.Equal(value[:4], []byte{0, 3, 0x52, 1})) {
		return decodedReceipt{}, receiptFailure(ReceiptCheckReceiptShape)
	}
	decoder := wireDecoder{value: value}
	envelopeVersion := decoder.u16()
	if (envelopeVersion != 1 && envelopeVersion != 2 && envelopeVersion != 3) || decoder.u16() != 0x5201 {
		return decodedReceipt{}, receiptFailure(ReceiptCheckDecode)
	}
	receipt := decodedReceipt{protocolVersion: decoder.u16()}
	if receipt.protocolVersion != envelopeVersion {
		return decodedReceipt{}, receiptFailure(ReceiptCheckProtocolVersion)
	}
	receipt.protocol.ProtocolVersion = receipt.protocolVersion
	receipt.activityID = decoder.array32()
	receipt.protocol.ActivityID = receipt.activityID
	receipt.protocol.GlobalSequence = decoder.u64()
	receipt.previousStateRoot = decoder.array32()
	receipt.protocol.PreviousStateRoot = receipt.previousStateRoot
	receipt.resultingStateRoot = decoder.array32()
	receipt.protocol.ResultingStateRoot = receipt.resultingStateRoot
	receipt.protocol.ActivityRoot = decoder.array32()
	receipt.resultCode = decoder.i32()
	receipt.protocol.ResultCode = receipt.resultCode
	effectCount := decoder.u32()
	if decoder.failed || effectCount > maximumEffects {
		return decodedReceipt{}, receiptFailure(ReceiptCheckDecode)
	}
	for index := uint32(0); index < effectCount; index++ {
		effect := ReceiptEffect{
			ModuleID:  decoder.u16(),
			Ordinal:   decoder.u16(),
			EventType: decoder.u16(),
		}
		kind := decoder.u8()
		monetary := decoder.u8()
		if kind == 0 || kind > 3 || monetary > 1 || monetary == 1 && kind != 2 {
			return decodedReceipt{}, receiptFailure(ReceiptCheckDecode)
		}
		effect.Kind = kind
		effect.Monetary = monetary == 1
		effect.TransferSetRoot = decoder.array32()
		effect.Body = append([]byte(nil), decoder.bounded(maximumEffectBody)...)
		receipt.protocol.Effects = append(receipt.protocol.Effects, effect)
	}
	receipt.feeCharged = decoder.u128()
	receipt.protocol.FeeCharged = receipt.feeCharged
	receipt.batchID = decoder.array32()
	receipt.protocol.BatchID = receipt.batchID
	receipt.protocol.ModuleID = decoder.u16()
	receipt.protocol.ModuleVersion = decoder.u32()
	receipt.protocol.ParameterVersion = decoder.u32()
	receipt.operation = decoder.u8()
	receipt.protocol.Operation = receipt.operation
	receipt.asset = decoder.array32()
	receipt.protocol.Asset = receipt.asset
	receipt.amount = decoder.u128()
	receipt.protocol.Amount = receipt.amount
	receipt.protocol.From = decoder.array32()
	receipt.debitBefore = decoder.u128()
	receipt.protocol.FromBalanceBefore = receipt.debitBefore
	receipt.debitAfter = decoder.u128()
	receipt.protocol.FromBalanceAfter = receipt.debitAfter
	receipt.protocol.FromSequence = decoder.u64()
	receipt.protocol.To = decoder.array32()
	receipt.creditBefore = decoder.u128()
	receipt.protocol.ToBalanceBefore = receipt.creditBefore
	receipt.creditAfter = decoder.u128()
	receipt.protocol.ToBalanceAfter = receipt.creditAfter
	receipt.protocol.TransferSetRoot = decoder.array32()
	receipt.protocol.AuthorizationHash = decoder.array32()
	receipt.protocol.ContextHash = decoder.array32()
	receipt.protocol.Timestamp = decoder.u64()
	if decoder.failed {
		return decodedReceipt{}, receiptFailure(ReceiptCheckDecode)
	}
	if kernel {
		if receipt.protocol.GlobalSequence == 0 {
			return decodedReceipt{}, receiptFailure(ReceiptCheckGlobalSequence)
		}
		if receipt.protocol.ModuleID == 0 {
			return decodedReceipt{}, receiptFailure(ReceiptCheckModuleID)
		}
		if receipt.protocol.ModuleVersion == 0 {
			return decodedReceipt{}, receiptFailure(ReceiptCheckModuleVersion)
		}
		if receipt.protocol.Timestamp == 0 {
			return decodedReceipt{}, receiptFailure(ReceiptCheckTimestamp)
		}
		if zero32(receipt.protocol.ActivityID) {
			return decodedReceipt{}, receiptFailure(ReceiptCheckActivityID)
		}
		if zero32(receipt.protocol.ResultingStateRoot) {
			return decodedReceipt{}, receiptFailure(ReceiptCheckResultingStateRoot)
		}
	}
	if kernel && len(value)-decoder.offset > 69 {
		outcome, ok := decodeProgramReceiptOutcomeFrom(&decoder, receipt.protocolVersion)
		if !ok || receipt.protocol.ModuleID != ProgramsModuleID || outcome.ResultCode != receipt.resultCode || outcome.TerminalKind == 1 && outcome.TransferRoot != receipt.protocol.TransferSetRoot || outcome.TerminalKind != 1 && receipt.protocol.TransferSetRoot != [32]byte{} {
			return decodedReceipt{}, receiptFailure(ReceiptCheckProgramOutcome)
		}
		receipt.protocol.ProgramOutcome = &outcome
	}
	signatureMarker := decoder.offset
	present := decoder.u8()
	if decoder.failed {
		return decodedReceipt{}, receiptFailure(ReceiptCheckDecode)
	}
	if present == 0 {
		return decodedReceipt{}, receiptFailure(ReceiptCheckMissingSignature)
	}
	if present != 1 {
		return decodedReceipt{}, receiptFailure(ReceiptCheckCanonicalEncoding)
	}
	signature := decoder.bounded(64)
	if decoder.failed || len(signature) != 64 || decoder.offset != len(value) {
		return decodedReceipt{}, receiptFailure(ReceiptCheckCanonicalEncoding)
	}
	copy(receipt.signature[:], signature)
	receipt.protocol.SequencerSignature = receipt.signature
	receipt.unsigned = make([]byte, signatureMarker+1)
	copy(receipt.unsigned, value[:signatureMarker])
	receipt.unsigned[signatureMarker] = 0
	return receipt, nil
}

func decodeProgramReceiptOutcomeFrom(decoder *wireDecoder, protocolVersion uint16) (ProgramReceiptOutcome, bool) {
	var outcome ProgramReceiptOutcome
	switch decoder.u32() {
	case ProgramOutcomeTagV1:
		outcome.EncodingVersion = 1
	case ProgramOutcomeTagV2:
		outcome.EncodingVersion = 2
	case ProgramOutcomeTagV3:
		outcome.EncodingVersion = 3
	case ProgramOutcomeTagV4:
		outcome.EncodingVersion = 4
	default:
		return ProgramReceiptOutcome{}, false
	}
	outcome.TerminalKind = decoder.u8()
	outcome.ResultCode = decoder.i32()
	outcome.RuntimeVersion = decoder.u16()
	outcome.ABIVersion = decoder.u16()
	outcome.FeeScheduleVersion = decoder.u32()
	outcome.MeteringScheduleVersion = 1
	if outcome.EncodingVersion >= 3 {
		outcome.MeteringScheduleVersion = decoder.u32()
	}
	outcome.CPUFuel = decoder.u64()
	outcome.MemoryBytes = decoder.u64()
	outcome.StorageReadBytes = decoder.u64()
	outcome.StorageWriteBytes = decoder.u64()
	outcome.OutputValues = decoder.u32()
	outcome.OutputBytes = decoder.u64()
	if outcome.EncodingVersion >= 2 {
		outcome.OccupancyByteBatches = decoder.u128()
		outcome.OccupancyFeeUnits = decoder.u128()
		for index := range outcome.FeeSchedulePrices {
			outcome.FeeSchedulePrices[index] = decoder.u64()
		}
		outcome.OccupancyAssetID = decoder.array32()
		outcome.OccupancyEvidenceDigest = decoder.array32()
		outcome.OccupancyTransferRoot = decoder.array32()
	}
	outcome.FeeUnits = decoder.u128()
	outcome.CallGraphRoot = decoder.array32()
	outcome.TerminalPayloadRoot = decoder.array32()
	outcome.TransferRoot = decoder.array32()
	if outcome.EncodingVersion == 4 {
		outcome.AppliedLegsDigest = decoder.array32()
	}
	occupancyZero := outcome.OccupancyByteBatches == (Uint128{}) && outcome.OccupancyFeeUnits == (Uint128{}) && outcome.OccupancyAssetID == [32]byte{} && outcome.OccupancyEvidenceDigest == [32]byte{} && outcome.OccupancyTransferRoot == [32]byte{}
	validVersion := protocolVersion == 1 && (outcome.EncodingVersion == 1 || outcome.EncodingVersion == 3) || (protocolVersion == 2 || protocolVersion == 3) && (outcome.EncodingVersion == 2 || outcome.EncodingVersion == 3) || protocolVersion == 3 && outcome.EncodingVersion == 4
	if decoder.failed || (outcome.EncodingVersion == 4) == (outcome.AppliedLegsDigest == [32]byte{}) || outcome.TerminalKind < 1 || outcome.TerminalKind > 3 || outcome.RuntimeVersion == 0 || outcome.ABIVersion == 0 || outcome.FeeScheduleVersion == 0 || outcome.MeteringScheduleVersion != 1 || outcome.TerminalPayloadRoot == [32]byte{} || outcome.TerminalKind == 1 && outcome.ResultCode != 0 || outcome.TerminalKind != 1 && (outcome.ResultCode == 0 || outcome.ResultCode <= -1000) || outcome.TerminalKind != 1 && outcome.TransferRoot != [32]byte{} || !validVersion || outcome.EncodingVersion == 1 && !occupancyZero || outcome.EncodingVersion >= 2 && outcome.TerminalKind != 1 && !occupancyZero || outcome.EncodingVersion == 2 && outcome.TerminalKind == 1 && (outcome.OccupancyAssetID == [32]byte{} || outcome.OccupancyEvidenceDigest == [32]byte{}) || outcome.EncodingVersion >= 3 && (outcome.OccupancyAssetID == [32]byte{}) != (outcome.OccupancyEvidenceDigest == [32]byte{}) || protocolVersion == 1 && outcome.EncodingVersion >= 3 && !occupancyZero || (protocolVersion == 2 || protocolVersion == 3) && outcome.EncodingVersion >= 3 && outcome.TerminalKind == 1 && (outcome.OccupancyAssetID == [32]byte{} || outcome.OccupancyEvidenceDigest == [32]byte{}) {
		return ProgramReceiptOutcome{}, false
	}
	return outcome, true
}

func DecodeProgramReceiptOutcome(canonicalOutcome []byte, protocolVersion uint16) (ProgramReceiptOutcome, error) {
	if len(canonicalOutcome) == 0 || len(canonicalOutcome) > maximumMessageBytes {
		return ProgramReceiptOutcome{}, receiptFailure(ReceiptCheckReceiptShape)
	}
	decoder := wireDecoder{value: append([]byte(nil), canonicalOutcome...)}
	outcome, ok := decodeProgramReceiptOutcomeFrom(&decoder, protocolVersion)
	if !ok || decoder.failed || decoder.offset != len(canonicalOutcome) {
		return ProgramReceiptOutcome{}, receiptFailure(ReceiptCheckCanonicalEncoding)
	}
	return outcome, nil
}

type MerkleProof struct {
	LeafIndex uint32
	LeafCount uint32
	Siblings  [][32]byte
}

func VerifyMerkleInclusion(canonicalLeaf []byte, proof MerkleProof, expectedRoot [32]byte) error {
	if proof.LeafCount == 0 || proof.LeafIndex >= proof.LeafCount || len(proof.Siblings) > 32 || len(proof.Siblings) != proofDepth(proof.LeafCount) {
		return verificationFailure()
	}
	current := domainDigest(merkleLeafDomain, canonicalLeaf)
	index := proof.LeafIndex
	count := proof.LeafCount
	for _, sibling := range proof.Siblings {
		if index^1 >= count && sibling != current {
			return verificationFailure()
		}
		if index%2 == 0 {
			current = domainDigest(merkleInternalDomain, current[:], sibling[:])
		} else {
			current = domainDigest(merkleInternalDomain, sibling[:], current[:])
		}
		index /= 2
		count = (count + 1) / 2
	}
	if current != expectedRoot {
		return verificationFailure()
	}
	return nil
}

func proofDepth(count uint32) int {
	depth := 0
	for count > 1 {
		count = (count + 1) / 2
		depth++
	}
	return depth
}

type BatchHeader struct {
	ProtocolVersion      uint16
	NetworkID            uint32
	Epoch                uint64
	BatchNumber          uint64
	FirstSequence        uint64
	LastSequence         uint64
	PreviousStateRoot    [32]byte
	ResultingStateRoot   [32]byte
	ActivityMerkleRoot   [32]byte
	ReceiptMerkleRoot    [32]byte
	EventMerkleRoot      [32]byte
	DataAvailabilityRoot [32]byte
	OracleRoot           [32]byte
	TimestampMillis      uint64
	SequencerID          [32]byte
}

func DecodeBatchHeader(canonicalHeader []byte) (BatchHeader, error) {
	if len(canonicalHeader) != batchHeaderBytes {
		return BatchHeader{}, verificationFailure()
	}
	decoder := wireDecoder{value: canonicalHeader}
	if decoder.u16() != 1 || decoder.u16() != 0x1701 || decoder.u8() != 15 {
		return BatchHeader{}, verificationFailure()
	}
	field := func(expected uint8) bool { return decoder.u8() == expected }
	var header BatchHeader
	if !field(1) {
		return BatchHeader{}, verificationFailure()
	}
	header.ProtocolVersion = decoder.u16()
	if !field(2) {
		return BatchHeader{}, verificationFailure()
	}
	header.NetworkID = decoder.u32()
	if !field(3) {
		return BatchHeader{}, verificationFailure()
	}
	header.Epoch = decoder.u64()
	if !field(4) {
		return BatchHeader{}, verificationFailure()
	}
	header.BatchNumber = decoder.u64()
	if !field(5) {
		return BatchHeader{}, verificationFailure()
	}
	header.FirstSequence = decoder.u64()
	if !field(6) {
		return BatchHeader{}, verificationFailure()
	}
	header.LastSequence = decoder.u64()
	if !field(7) {
		return BatchHeader{}, verificationFailure()
	}
	header.PreviousStateRoot = decoder.array32()
	if !field(8) {
		return BatchHeader{}, verificationFailure()
	}
	header.ResultingStateRoot = decoder.array32()
	if !field(9) {
		return BatchHeader{}, verificationFailure()
	}
	header.ActivityMerkleRoot = decoder.array32()
	if !field(10) {
		return BatchHeader{}, verificationFailure()
	}
	header.ReceiptMerkleRoot = decoder.array32()
	if !field(11) {
		return BatchHeader{}, verificationFailure()
	}
	header.EventMerkleRoot = decoder.array32()
	if !field(12) {
		return BatchHeader{}, verificationFailure()
	}
	header.DataAvailabilityRoot = decoder.array32()
	if !field(13) {
		return BatchHeader{}, verificationFailure()
	}
	header.OracleRoot = decoder.array32()
	if !field(14) {
		return BatchHeader{}, verificationFailure()
	}
	header.TimestampMillis = decoder.u64()
	if !field(15) {
		return BatchHeader{}, verificationFailure()
	}
	header.SequencerID = decoder.array32()
	if decoder.failed || decoder.offset != len(canonicalHeader) {
		return BatchHeader{}, verificationFailure()
	}
	return header, nil
}

type SequencerAuthorization struct {
	SequencerID      [32]byte
	PublicKey        [32]byte
	FirstBatchNumber uint64
	LastBatchNumber  uint64
}

type InclusionKind string

const (
	InclusionActivity InclusionKind = "activity"
	InclusionReceipt  InclusionKind = "receipt"
	InclusionEvent    InclusionKind = "event"
	InclusionState    InclusionKind = "state"
)

type InclusionVerification struct {
	Level        string
	Header       BatchHeader
	HeaderDigest [32]byte
	Root         [32]byte
}

func VerifyBatchInclusion(kind InclusionKind, canonicalLeaf []byte, proof MerkleProof, canonicalHeader []byte, headerSignature []byte, authorization SequencerAuthorization, selectedProtocol ...uint16) (InclusionVerification, error) {
	header, err := DecodeBatchHeader(canonicalHeader)
	if err != nil {
		return InclusionVerification{}, err
	}
	if !selectedProtocolMatches(header.ProtocolVersion, selectedProtocol) {
		return InclusionVerification{}, verificationFailure()
	}
	if header.BatchNumber < authorization.FirstBatchNumber || header.BatchNumber > authorization.LastBatchNumber || header.SequencerID != authorization.SequencerID || len(headerSignature) != ed25519.SignatureSize {
		return InclusionVerification{}, verificationFailure()
	}
	digest := domainDigest(batchHeaderDomain, canonicalHeader)
	if !ed25519.Verify(authorization.PublicKey[:], digest[:], headerSignature) {
		return InclusionVerification{}, verificationFailure()
	}
	var root [32]byte
	level := "batch-included"
	switch kind {
	case InclusionActivity:
		root = header.ActivityMerkleRoot
	case InclusionReceipt:
		root = header.ReceiptMerkleRoot
	case InclusionEvent:
		root = header.EventMerkleRoot
	case InclusionState:
		root = header.ResultingStateRoot
		level = "state-proven"
	default:
		return InclusionVerification{}, verificationFailure()
	}
	if err := VerifyMerkleInclusion(canonicalLeaf, proof, root); err != nil {
		return InclusionVerification{}, err
	}
	return InclusionVerification{Level: level, Header: header, HeaderDigest: digest, Root: root}, nil
}

type CheckpointAttestation struct {
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
	AttestedAtMillis      uint64
	Signer                [20]byte
	Signature             [64]byte
	SignatureV            uint8
}

type GuarantorKey struct {
	GuarantorID [32]byte
	PublicKey   [33]byte
	Bonded      bool
}

type CheckpointCertificate struct {
	CanonicalHeader     []byte
	ValidityProof       []byte
	Attestations        []CheckpointAttestation
	Threshold           uint32
	SettlementReference []byte
}

type CheckpointVerificationInput struct {
	Certificate                   CheckpointCertificate
	BondedSet                     []GuarantorKey
	RegisteredCheckpointID        [32]byte
	ExpectedPaxeerChainID         uint64
	ExpectedSettlementContract    [20]byte
	RegisteredSettlementReference []byte
	AvailabilityObtained          bool
}

type Secp256k1Verifier interface {
	VerifyRecoverableSecp256k1(context.Context, [33]byte, [64]byte, uint8, [20]byte, [32]byte) bool
}

type CheckpointVerification struct {
	Level        string
	CheckpointID [32]byte
	Achieved     uint32
	Required     uint32
	Header       BatchHeader
}

func VerifyCheckpoint(ctx context.Context, input CheckpointVerificationInput, signatures Secp256k1Verifier, selectedProtocol ...uint16) (CheckpointVerification, error) {
	certificate := input.Certificate
	if ctx == nil || signatures == nil || !input.AvailabilityObtained || uint64(len(certificate.ValidityProof)) > uint64(^uint32(0)) || certificate.Threshold == 0 ||
		input.ExpectedPaxeerChainID == 0 || input.ExpectedSettlementContract == ([20]byte{}) {
		return CheckpointVerification{}, verificationFailure()
	}
	header, err := DecodeBatchHeader(certificate.CanonicalHeader)
	if err != nil {
		return CheckpointVerification{}, err
	}
	if !selectedProtocolMatches(header.ProtocolVersion, selectedProtocol) {
		return CheckpointVerification{}, verificationFailure()
	}
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(certificate.ValidityProof)))
	checkpointID := domainDigest(checkpointDomain, certificate.CanonicalHeader, length, certificate.ValidityProof)
	if checkpointID != input.RegisteredCheckpointID {
		return CheckpointVerification{}, verificationFailure()
	}
	bonded := make(map[[32]byte]GuarantorKey, len(input.BondedSet))
	for _, member := range input.BondedSet {
		if member.Bonded {
			bonded[member.GuarantorID] = member
		}
	}
	seen := make(map[[32]byte]struct{}, len(certificate.Attestations))
	var achieved uint32
	for _, attestation := range certificate.Attestations {
		if err := ctx.Err(); err != nil {
			return CheckpointVerification{}, err
		}
		if _, duplicate := seen[attestation.GuarantorID]; duplicate ||
			attestation.ProtocolVersion != header.ProtocolVersion || attestation.NetworkID != header.NetworkID || attestation.Epoch != header.Epoch ||
			attestation.PaxeerChainID != input.ExpectedPaxeerChainID || attestation.SettlementContract != input.ExpectedSettlementContract ||
			attestation.CheckpointID != checkpointID || attestation.CheckpointHash != checkpointID ||
			attestation.BatchNumber != header.BatchNumber || attestation.DataAvailabilityRoot != header.DataAvailabilityRoot ||
			!attestation.Replayed || !attestation.DataPossessed || attestation.AvailabilityClassMask != allAvailabilityClasses || attestation.AttestedAtMillis == 0 ||
			attestation.Signer == ([20]byte{}) || (attestation.SignatureV != 27 && attestation.SignatureV != 28) {
			return CheckpointVerification{}, verificationFailure()
		}
		member, ok := bonded[attestation.GuarantorID]
		if !ok {
			return CheckpointVerification{}, verificationFailure()
		}
		seen[attestation.GuarantorID] = struct{}{}
		message := make([]byte, 0, 189)
		message = appendUint16(message, attestation.ProtocolVersion)
		message = appendUint32(message, attestation.NetworkID)
		message = appendUint64(message, attestation.PaxeerChainID)
		message = append(message, attestation.SettlementContract[:]...)
		message = appendUint64(message, attestation.Epoch)
		message = append(message, attestation.CheckpointID[:]...)
		message = append(message, attestation.CheckpointHash[:]...)
		message = append(message, attestation.GuarantorID[:]...)
		message = appendUint64(message, attestation.BatchNumber)
		message = append(message, attestation.DataAvailabilityRoot[:]...)
		message = append(message, boolByte(attestation.Replayed), boolByte(attestation.DataPossessed), attestation.AvailabilityClassMask)
		message = appendUint64(message, attestation.AttestedAtMillis)
		digest := domainDigest(guarantorAttestationDomain, message)
		if !signatures.VerifyRecoverableSecp256k1(ctx, member.PublicKey, attestation.Signature, attestation.SignatureV, attestation.Signer, digest) {
			return CheckpointVerification{}, verificationFailure()
		}
		achieved++
	}
	if achieved < certificate.Threshold {
		return CheckpointVerification{}, verificationFailure()
	}
	level := "checkpoint-finalised"
	if certificate.SettlementReference != nil {
		if len(certificate.SettlementReference) == 0 || !bytes.Equal(certificate.SettlementReference, input.RegisteredSettlementReference) {
			return CheckpointVerification{}, verificationFailure()
		}
		level = "settlement-anchored"
	}
	return CheckpointVerification{Level: level, CheckpointID: checkpointID, Achieved: achieved, Required: certificate.Threshold, Header: header}, nil
}

type wireDecoder struct {
	value  []byte
	offset int
	failed bool
}

func (decoder *wireDecoder) fixed(length int) []byte {
	if decoder.failed || length < 0 || decoder.offset > len(decoder.value)-length {
		decoder.failed = true
		return nil
	}
	result := decoder.value[decoder.offset : decoder.offset+length]
	decoder.offset += length
	return result
}

func (decoder *wireDecoder) u8() uint8 {
	value := decoder.fixed(1)
	if len(value) != 1 {
		return 0
	}
	return value[0]
}

func (decoder *wireDecoder) u16() uint16 {
	value := decoder.fixed(2)
	if len(value) != 2 {
		return 0
	}
	return binary.BigEndian.Uint16(value)
}

func (decoder *wireDecoder) u32() uint32 {
	value := decoder.fixed(4)
	if len(value) != 4 {
		return 0
	}
	return binary.BigEndian.Uint32(value)
}

func (decoder *wireDecoder) i32() int32 { return int32(decoder.u32()) }

func (decoder *wireDecoder) u64() uint64 {
	value := decoder.fixed(8)
	if len(value) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(value)
}

func (decoder *wireDecoder) u128() Uint128 {
	return Uint128{high: decoder.u64(), low: decoder.u64()}
}

func (decoder *wireDecoder) bounded(maximum uint32) []byte {
	length := decoder.u32()
	if decoder.failed || length > maximum {
		decoder.failed = true
		return nil
	}
	return decoder.fixed(int(length))
}

func (decoder *wireDecoder) array32() [32]byte {
	value := decoder.bounded(32)
	var result [32]byte
	if len(value) != len(result) {
		decoder.failed = true
		return result
	}
	copy(result[:], value)
	return result
}

func domainDigest(domain []byte, values ...[]byte) [32]byte {
	hash := sha256.New()
	hash.Write(domain)
	for _, value := range values {
		hash.Write(value)
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func zero32(value [32]byte) bool { return value == [32]byte{} }

func putUint64(target []byte, value uint64) { binary.BigEndian.PutUint64(target, value) }

func appendUint16(target []byte, value uint16) []byte {
	var encoded [2]byte
	binary.BigEndian.PutUint16(encoded[:], value)
	return append(target, encoded[:]...)
}

func appendUint32(target []byte, value uint32) []byte {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	return append(target, encoded[:]...)
}

func appendUint64(target []byte, value uint64) []byte {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	return append(target, encoded[:]...)
}

func boolByte(value bool) byte {
	if value {
		return 1
	}
	return 0
}

func selectedProtocolMatches(actual uint16, selected []uint16) bool {
	version := currentProtocolVersion
	if len(selected) > 1 {
		return false
	}
	if len(selected) == 1 {
		version = selected[0]
	}
	return (version == 2 || version == 3) && actual == version
}

type ReceiptKey struct {
	KeyID       [32]byte
	PublicKey   [32]byte
	FirstHeight uint64
	LastHeight  uint64
}

type ReceiptKeys struct {
	Pins                 []ReceiptKey
	PreviousSequenceHash [32]byte
}

type ReceiptRevert struct {
	Code       string
	Reason     string
	ReturnData []byte
}

func (r *ReceiptRevert) Error() string {
	if r.Reason != "" {
		return r.Reason
	}
	return "execution reverted"
}

type SequencerReceiptVerification struct {
	VerifiedReceipt
	Rung       string
	Status     uint64
	ReturnData []byte
	Logs       []SequencerReceiptLog
	Revert     *ReceiptRevert
}

type SequencerReceiptLog struct {
	Address  string   `json:"address"`
	Topics   []string `json:"topics"`
	Data     string   `json:"data"`
	LogIndex string   `json:"logIndex"`
}

type sequencerReceiptJSON struct {
	Rung              string                `json:"rung"`
	TxHash            string                `json:"txHash"`
	Nonce             string                `json:"nonce"`
	Seq               string                `json:"seq"`
	Height            string                `json:"height"`
	Index             string                `json:"index"`
	Timestamp         string                `json:"timestamp"`
	Status            string                `json:"status"`
	ReturnData        string                `json:"returnData"`
	Logs              []SequencerReceiptLog `json:"logs"`
	GasUsed           string                `json:"gasUsed"`
	CumulativeGasUsed string                `json:"cumulativeGasUsed"`
	EffectiveGasPrice string                `json:"effectiveGasPrice"`
	FeeDenom          string                `json:"feeDenom"`
	FeeAmount         string                `json:"feeAmount"`
	Sponsor           string                `json:"sponsor"`
	From              string                `json:"from"`
	To                string                `json:"to"`
	ContractAddress   string                `json:"contractAddress"`
	VMError           string                `json:"vmError"`
	KeyID             string                `json:"keyId"`
	Receipt           string                `json:"receipt"`
}

func VerifySequencerReceipt(receiptBytes, rawTransaction []byte, keys ReceiptKeys) (SequencerReceiptVerification, error) {
	fail := func(check ReceiptCheck) (SequencerReceiptVerification, error) {
		return SequencerReceiptVerification{}, receiptFailure(check)
	}
	if len(receiptBytes) == 0 || len(receiptBytes) > 4*maximumMessageBytes {
		return fail(ReceiptCheckDecode)
	}
	var view sequencerReceiptJSON
	if err := json.Unmarshal(receiptBytes, &view); err != nil {
		return fail(ReceiptCheckDecode)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(receiptBytes, &fields); err != nil {
		return fail(ReceiptCheckDecode)
	}
	for _, name := range []string{"rung", "txHash", "nonce", "seq", "height", "index", "timestamp", "status", "returnData", "logs", "gasUsed", "cumulativeGasUsed", "effectiveGasPrice", "feeDenom", "feeAmount", "sponsor", "from", "to", "contractAddress", "vmError", "keyId", "receipt"} {
		if v, ok := fields[name]; !ok || bytes.Equal(v, []byte("null")) {
			return fail(ReceiptCheckFields)
		}
	}
	canonical, ok := receiptHex(view.Receipt)
	if !ok {
		return fail(ReceiptCheckDecode)
	}
	if len(canonical) < 6 {
		return fail(ReceiptCheckDecode)
	}
	version := binary.BigEndian.Uint16(canonical[:2])
	if (version != ReceiptVersionV2 && version != ReceiptVersionV3) || binary.BigEndian.Uint16(canonical[4:6]) != version {
		return fail(ReceiptCheckReceiptVersion)
	}
	decoded, err := decodeReceiptProfile(canonical, false)
	if err != nil {
		if e, ok := err.(*VerificationError); ok && (e.Check == ReceiptCheckCanonicalEncoding || e.Check == ReceiptCheckMissingSignature) {
			return fail(ReceiptCheckCanonicalEncoding)
		}
		return fail(ReceiptCheckDecode)
	}
	r := decoded.protocol
	if !bytes.Equal(canonical, encodeSequencerReceipt(r, true)) {
		return fail(ReceiptCheckCanonicalEncoding)
	}
	if zero32(r.ActivityID) {
		return fail(ReceiptCheckActivityID)
	}
	if !bytes.Equal(r.BatchID[:24], make([]byte, 24)) {
		return fail(ReceiptCheckFields)
	}
	height := binary.BigEndian.Uint64(r.BatchID[24:])
	var key *ReceiptKey
	knownID := false
	for i := range keys.Pins {
		pin := &keys.Pins[i]
		if pin.KeyID != r.ContextHash || zero32(pin.KeyID) {
			continue
		}
		knownID = true
		if pin.FirstHeight <= height && height <= pin.LastHeight {
			key = pin
			break
		}
	}
	if !knownID {
		return fail(ReceiptCheckKeyID)
	}
	if key == nil {
		return fail(ReceiptCheckKeyHeight)
	}
	if len(rawTransaction) == 0 || len(rawTransaction) > maximumMessageBytes || receiptKeccak256(rawTransaction) != r.ActivityID {
		return fail(ReceiptCheckRawTransaction)
	}
	status, err := verifySequencerFields(view, r, height)
	if err != nil {
		return SequencerReceiptVerification{}, err
	}
	data, ok := receiptHex(view.ReturnData)
	summary := r.Effects[0].Body
	if !ok || uint64(len(data)) != uint64(binary.BigEndian.Uint32(summary[100:104])) || sha256.Sum256(data) != r.ActivityRoot {
		return fail(ReceiptCheckReturnData)
	}
	if err := verifySequencerLogs(view.Logs, r); err != nil {
		return SequencerReceiptVerification{}, err
	}
	seqBytes := appendUint64(nil, r.GlobalSequence)
	sequenceHash := domainDigest([]byte(SequenceDomain+"\x00"), keys.PreviousSequenceHash[:], seqBytes, r.ActivityID[:])
	if r.PreviousStateRoot != keys.PreviousSequenceHash || r.ResultingStateRoot != sequenceHash {
		return fail(ReceiptCheckSequenceHash)
	}
	unsigned := encodeSequencerReceipt(r, false)
	digest := domainDigest([]byte(ReceiptDomain+"\x00"), unsigned)
	if !ed25519.Verify(key.PublicKey[:], digest[:], r.SequencerSignature[:]) {
		return fail(ReceiptCheckSignature)
	}
	result := SequencerReceiptVerification{
		VerifiedReceipt: VerifiedReceipt{Level: "receipted", Receipt: r, CanonicalBytes: append([]byte(nil), canonical...), ReceiptDigest: digest,
			Facts: ProtocolReceiptFacts{ResultCode: r.ResultCode, Asset: r.Asset, Amount: r.Amount, FeeCharged: r.FeeCharged}},
		Rung: "receipted", Status: status, ReturnData: append([]byte(nil), data...), Logs: view.Logs,
	}
	if status == 0 {
		result.Revert = decodeReceiptRevert(data)
	}
	return result, nil
}

func receiptHex(value string) ([]byte, bool) {
	if !strings.HasPrefix(value, "0x") || len(value)%2 != 0 {
		return nil, false
	}
	decoded, err := hex.DecodeString(value[2:])
	return decoded, err == nil
}

func receiptHexEquals(value string, expected []byte) bool {
	decoded, ok := receiptHex(value)
	return ok && bytes.Equal(decoded, expected)
}

func receiptQuantity(value string, width int) (*big.Int, bool) {
	if !strings.HasPrefix(value, "0x") || len(value) < 3 || len(value) > 3 && value[2] == '0' {
		return nil, false
	}
	for _, c := range value[2:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return nil, false
		}
	}
	n, ok := new(big.Int).SetString(value[2:], 16)
	return n, ok && n.BitLen() <= width
}

func receiptQuantityEquals(value string, expected uint64) bool {
	n, ok := receiptQuantity(value, 64)
	return ok && n.Uint64() == expected
}

func verifySequencerFields(v sequencerReceiptJSON, r ProtocolReceipt, height uint64) (uint64, error) {
	fail := func() (uint64, error) { return 0, receiptFailure(ReceiptCheckFields) }
	if v.Rung != "receipted" || r.ModuleID <= 11 || r.ModuleVersion != 1 || r.ParameterVersion != 0 || r.ProgramOutcome != nil || r.GlobalSequence == 0 || height == 0 || r.Timestamp == 0 || len(r.Effects) == 0 || r.ResultCode < 0 || r.ResultCode > 1 {
		return fail()
	}
	status := uint64(1 - r.ResultCode)
	if !receiptHexEquals(v.TxHash, r.ActivityID[:]) || !receiptHexEquals(v.KeyID, r.ContextHash[:]) ||
		!receiptQuantityEquals(v.Nonce, r.FromSequence) || !receiptQuantityEquals(v.Seq, r.GlobalSequence) ||
		!receiptQuantityEquals(v.Height, height) || !receiptQuantityEquals(v.Timestamp, r.Timestamp) || !receiptQuantityEquals(v.Status, status) {
		return fail()
	}
	if !bytes.Equal(r.From[:12], make([]byte, 12)) || !bytes.Equal(r.To[:12], make([]byte, 12)) || !receiptHexEquals(v.From, r.From[12:]) || !receiptHexEquals(v.To, r.To[12:]) {
		return fail()
	}
	summary := r.Effects[0]
	b := summary.Body
	if summary.ModuleID != r.ModuleID || summary.Ordinal != 0 || summary.EventType != 0 || summary.Kind != 1 || summary.Monetary || !zero32(summary.TransferSetRoot) || len(b) < 109 {
		return fail()
	}
	denomLen := int(b[108])
	if denomLen == 0 || denomLen > 64 || len(b) != 109+denomLen || !utf8.Valid(b[109:]) || v.FeeDenom != string(b[109:]) || sha256.Sum256(b[109:]) != r.Asset {
		return fail()
	}
	if !receiptQuantityEquals(v.GasUsed, binary.BigEndian.Uint64(b[:8])) ||
		!receiptQuantityEquals(v.CumulativeGasUsed, binary.BigEndian.Uint64(b[8:16])) ||
		!receiptQuantityEquals(v.EffectiveGasPrice, binary.BigEndian.Uint64(b[16:24])) ||
		!receiptQuantityEquals(v.Index, uint64(binary.BigEndian.Uint32(b[24:28]))) ||
		!receiptHexEquals(v.ContractAddress, b[28:48]) || !receiptHexEquals(v.Sponsor, b[48:68]) {
		return fail()
	}
	vmHash := sha256.Sum256([]byte(v.VMError))
	if !bytes.Equal(vmHash[:], b[68:100]) {
		return fail()
	}
	fee, ok := receiptQuantity(v.FeeAmount, 128)
	if !ok || fee.String() != r.FeeCharged.String() {
		return fail()
	}
	if uint64(binary.BigEndian.Uint32(b[104:108])) != uint64(len(r.Effects)-1) {
		return fail()
	}
	return status, nil
}

func verifySequencerLogs(logs []SequencerReceiptLog, r ProtocolReceipt) error {
	fail := func() error { return receiptFailure(ReceiptCheckLogs) }
	if logs == nil || len(logs) != len(r.Effects)-1 {
		return fail()
	}
	for i, log := range logs {
		effect := r.Effects[i+1]
		body := effect.Body
		if effect.ModuleID != r.ModuleID || int(effect.Ordinal) != i+1 || effect.EventType != 0 || effect.Kind != 3 || effect.Monetary || !zero32(effect.TransferSetRoot) || len(body) < 21 {
			return fail()
		}
		count := int(body[20])
		if count > 4 || len(body) != 21+32*count+32 || len(log.Topics) != count || !receiptHexEquals(log.Address, body[:20]) || !receiptQuantityEquals(log.LogIndex, uint64(i)) {
			return fail()
		}
		for j, topic := range log.Topics {
			if !receiptHexEquals(topic, body[21+j*32:21+(j+1)*32]) {
				return fail()
			}
		}
		data, ok := receiptHex(log.Data)
		digest := sha256.Sum256(data)
		if !ok || !bytes.Equal(digest[:], body[len(body)-32:]) {
			return fail()
		}
	}
	return nil
}

func decodeReceiptRevert(data []byte) *ReceiptRevert {
	r := &ReceiptRevert{ReturnData: append([]byte(nil), data...)}
	if len(data) < 68 || !bytes.Equal(data[:4], []byte{0x08, 0xc3, 0x79, 0xa0}) || !bytes.Equal(data[4:28], make([]byte, 24)) || binary.BigEndian.Uint64(data[28:36]) != 32 || !bytes.Equal(data[36:60], make([]byte, 24)) {
		return r
	}
	length := binary.BigEndian.Uint64(data[60:68])
	if length > uint64(len(data)-68) || !utf8.Valid(data[68:68+int(length)]) {
		return r
	}
	r.Reason = string(data[68 : 68+int(length)])
	if strings.HasPrefix(r.Reason, "LXP_") {
		r.Code = strings.SplitN(r.Reason, ":", 2)[0]
	}
	return r
}

func encodeSequencerReceipt(r ProtocolReceipt, signed bool) []byte {
	b := appendUint16(nil, r.ProtocolVersion)
	b = appendUint16(b, 0x5201)
	b = appendUint16(b, r.ProtocolVersion)
	arr := func(v [32]byte) { b = appendUint32(b, 32); b = append(b, v[:]...) }
	amount := func(v Uint128) { b = appendUint64(b, v.high); b = appendUint64(b, v.low) }
	arr(r.ActivityID)
	b = appendUint64(b, r.GlobalSequence)
	arr(r.PreviousStateRoot)
	arr(r.ResultingStateRoot)
	arr(r.ActivityRoot)
	b = appendUint32(b, uint32(r.ResultCode))
	b = appendUint32(b, uint32(len(r.Effects)))
	for _, e := range r.Effects {
		b = appendUint16(b, e.ModuleID)
		b = appendUint16(b, e.Ordinal)
		b = appendUint16(b, e.EventType)
		b = append(b, e.Kind, boolByte(e.Monetary))
		arr(e.TransferSetRoot)
		b = appendUint32(b, uint32(len(e.Body)))
		b = append(b, e.Body...)
	}
	amount(r.FeeCharged)
	arr(r.BatchID)
	b = appendUint16(b, r.ModuleID)
	b = appendUint32(b, r.ModuleVersion)
	b = appendUint32(b, r.ParameterVersion)
	b = append(b, r.Operation)
	arr(r.Asset)
	amount(r.Amount)
	arr(r.From)
	amount(r.FromBalanceBefore)
	amount(r.FromBalanceAfter)
	b = appendUint64(b, r.FromSequence)
	arr(r.To)
	amount(r.ToBalanceBefore)
	amount(r.ToBalanceAfter)
	arr(r.TransferSetRoot)
	arr(r.AuthorizationHash)
	arr(r.ContextHash)
	b = appendUint64(b, r.Timestamp)
	if signed {
		b = append(b, 1)
		b = appendUint32(b, 64)
		b = append(b, r.SequencerSignature[:]...)
	} else {
		b = append(b, 0)
	}
	return b
}

func receiptKeccak256(input []byte) [32]byte {
	const rate = 136
	var state [25]uint64
	absorb := func(block []byte) {
		for i := 0; i < rate/8; i++ {
			state[i] ^= binary.LittleEndian.Uint64(block[i*8:])
		}
		receiptKeccakF(&state)
	}
	for len(input) >= rate {
		absorb(input[:rate])
		input = input[rate:]
	}
	var last [rate]byte
	copy(last[:], input)
	last[len(input)] = 1
	last[rate-1] |= 0x80
	absorb(last[:])
	var out [32]byte
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint64(out[i*8:], state[i])
	}
	return out
}

func receiptKeccakF(a *[25]uint64) {
	rounds := [24]uint64{0x0000000000000001, 0x0000000000008082, 0x800000000000808a, 0x8000000080008000, 0x000000000000808b, 0x0000000080000001, 0x8000000080008081, 0x8000000000008009, 0x000000000000008a, 0x0000000000000088, 0x0000000080008009, 0x000000008000000a, 0x000000008000808b, 0x800000000000008b, 0x8000000000008089, 0x8000000000008003, 0x8000000000008002, 0x8000000000000080, 0x000000000000800a, 0x800000008000000a, 0x8000000080008081, 0x8000000000008080, 0x0000000080000001, 0x8000000080008008}
	rotations := [25]int{0, 1, 62, 28, 27, 36, 44, 6, 55, 20, 3, 10, 43, 25, 39, 41, 45, 15, 21, 8, 18, 2, 61, 56, 14}
	for _, rc := range rounds {
		var c, d [5]uint64
		for x := 0; x < 5; x++ {
			c[x] = a[x] ^ a[x+5] ^ a[x+10] ^ a[x+15] ^ a[x+20]
		}
		for x := 0; x < 5; x++ {
			d[x] = c[(x+4)%5] ^ bits.RotateLeft64(c[(x+1)%5], 1)
		}
		var b [25]uint64
		for y := 0; y < 5; y++ {
			for x := 0; x < 5; x++ {
				i := x + 5*y
				b[y+5*((2*x+3*y)%5)] = bits.RotateLeft64(a[i]^d[x], rotations[i])
			}
		}
		for y := 0; y < 5; y++ {
			for x := 0; x < 5; x++ {
				a[x+5*y] = b[x+5*y] ^ (^b[(x+1)%5+5*y] & b[(x+2)%5+5*y])
			}
		}
		a[0] ^= rc
	}
}
