package layerx

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	MaximumProgramCalldataBytes = 1_048_576
	MaximumProgramCapabilities  = 5
	maximumProgramLegacyValues  = 512
	maximumProgramSimulationAge = uint64(300_000)
)

type ProgramCapability string

const (
	ProgramStorageRead  ProgramCapability = "storage_read"
	ProgramStorageWrite ProgramCapability = "storage_write"
	ProgramTransfer     ProgramCapability = "transfer"
	ProgramEmitEvent    ProgramCapability = "emit_event"
	ProgramCompose      ProgramCapability = "compose"
)

type ProgramBudget struct {
	Fuel     uint64  `json:"fuel"`
	FeeLimit Uint128 `json:"fee_limit"`
}

type ProgramCall struct {
	From            [20]byte
	TransactionHash [32]byte
	RawTransaction  []byte
	NativeCall      *NativeProgramCall
	ProgramID       [32]byte
	Calldata        []byte
	Budget          ProgramBudget
	Capabilities    []ProgramCapability
	SignedActivity  []byte
}

type ProgramDiscovery struct {
	DeploymentReceiptDigest *string         `json:"deployment_receipt_digest,omitempty"`
	DiscoveryPublicKey      *string         `json:"discovery_public_key,omitempty"`
	DiscoverySignature      *string         `json:"discovery_signature,omitempty"`
	ValueAccounts           json.RawMessage `json:"value_accounts,omitempty"`
	ProgramID               string          `json:"program_id"`
	Lifecycle               string          `json:"lifecycle"`
	Version                 uint32          `json:"version"`
	CodeHash                string          `json:"code_hash"`
	ABIVersion              uint16          `json:"abi_version"`
	ReceiptDigest           string          `json:"receipt_digest"`
	StateRoot               string          `json:"state_root"`
	ObservedSequence        string          `json:"observed_sequence"`
	ObservedAt              string          `json:"observed_at"`
	ValidThrough            string          `json:"valid_through"`
	Verification            string          `json:"verification"`
}

type ProgramInterface struct {
	ProgramID        string        `json:"program_id"`
	Version          uint32        `json:"version"`
	CodeHash         string        `json:"code_hash"`
	ABIVersion       uint16        `json:"abi_version"`
	Interface        *string       `json:"interface"`
	InterfaceDigest  *string       `json:"interface_digest"`
	ReceiptDigest    string        `json:"receipt_digest"`
	StateRoot        string        `json:"state_root"`
	ObservedSequence string        `json:"observed_sequence"`
	ObservedAt       string        `json:"observed_at"`
	ValidThrough     string        `json:"valid_through"`
	Source           ProgramSource `json:"source"`
	Verification     string        `json:"verification"`
}

type ProgramSource struct {
	Status                   string
	SourceDigest             *[32]byte
	EnvironmentDigest        *[32]byte
	ExpectedCodeHash         *[32]byte
	ReproducedArtifactDigest *[32]byte
	Pipeline                 string
}

func (source *ProgramSource) UnmarshalJSON(encoded []byte) error {
	fields, err := exactObject(encoded)
	if err != nil {
		return err
	}
	var status string
	if json.Unmarshal(fields["status"], &status) != nil {
		return errors.New("invalid Programs source status")
	}
	result := ProgramSource{Status: status}
	switch status {
	case "unpublished":
		if !exactFields(fields, "status") {
			return errors.New("invalid unpublished Programs source")
		}
	case "verified":
		if !exactFields(fields, "status", "source_digest", "environment_digest", "pipeline") {
			return errors.New("invalid verified Programs source")
		}
		sourceDigest, sourceError := programHex32Raw(fields["source_digest"])
		environmentDigest, environmentError := programHex32Raw(fields["environment_digest"])
		var pipeline string
		if sourceError != nil || environmentError != nil || json.Unmarshal(fields["pipeline"], &pipeline) != nil || pipeline != "sha256-source-artifact-reproducible-build-v1" {
			return errors.New("invalid verified Programs source")
		}
		result.SourceDigest = &sourceDigest
		result.EnvironmentDigest = &environmentDigest
		result.Pipeline = pipeline
	case "mismatch":
		if !exactFields(fields, "status", "expected_code_hash", "reproduced_artifact_digest") {
			return errors.New("invalid mismatched Programs source")
		}
		expected, expectedError := programHex32Raw(fields["expected_code_hash"])
		reproduced, reproducedError := programHex32Raw(fields["reproduced_artifact_digest"])
		if expectedError != nil || reproducedError != nil {
			return errors.New("invalid mismatched Programs source")
		}
		result.ExpectedCodeHash = &expected
		result.ReproducedArtifactDigest = &reproduced
	default:
		return errors.New("unknown Programs source status")
	}
	*source = result
	return nil
}

type ProgramResponseAuthority struct {
	BatchID            string `json:"batch_id"`
	Asset              string `json:"asset"`
	PreviousStateRoot  string `json:"previous_state_root"`
	ResultingStateRoot string `json:"resulting_state_root"`
	SequencerPublicKey string `json:"sequencer_public_key"`
}

type ProgramUsage struct {
	CPUFuel           string `json:"cpu_fuel"`
	MemoryBytes       string `json:"memory_bytes"`
	StorageReadBytes  string `json:"storage_read_bytes"`
	StorageWriteBytes string `json:"storage_write_bytes"`
	OutputValues      uint32 `json:"output_values"`
	OutputBytes       string `json:"output_bytes"`
	FeeUnits          string `json:"fee_units"`
}

type ProgramLegacyValue struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type ProgramFailure struct {
	Kind      string  `json:"kind"`
	Limit     *uint32 `json:"limit,omitempty"`
	Attempted *uint32 `json:"attempted,omitempty"`
	Code      *int32  `json:"code,omitempty"`
}

type ProgramOutcome struct {
	Kind     string
	Code     *int32
	Response []byte
	Values   []ProgramLegacyValue
	Failure  *ProgramFailure
}

func (outcome *ProgramOutcome) UnmarshalJSON(encoded []byte) error {
	fields, err := exactObject(encoded)
	if err != nil {
		return err
	}
	var kind string
	if json.Unmarshal(fields["kind"], &kind) != nil {
		return errors.New("invalid Programs outcome tag")
	}
	switch kind {
	case "completed":
		if !exactFields(fields, "kind", "code", "response") {
			return errors.New("invalid completed Programs outcome")
		}
		var code int32
		var response string
		if json.Unmarshal(fields["code"], &code) != nil || json.Unmarshal(fields["response"], &response) != nil || !canonicalLowerHex(response, len(response)/2) || len(response)/2 > MaximumProgramCalldataBytes {
			return errors.New("invalid completed Programs outcome")
		}
		body, _ := hex.DecodeString(response)
		*outcome = ProgramOutcome{Kind: kind, Code: &code, Response: body}
	case "legacy_completed":
		if !exactFields(fields, "kind", "code", "values") {
			return errors.New("invalid legacy Programs outcome")
		}
		var code int32
		var values []ProgramLegacyValue
		if json.Unmarshal(fields["code"], &code) != nil || decodeStrict(fields["values"], &values) != nil || len(values) > maximumProgramLegacyValues {
			return errors.New("invalid legacy Programs outcome")
		}
		for _, value := range values {
			if !validLegacyValue(value) {
				return errors.New("invalid legacy Programs value")
			}
		}
		*outcome = ProgramOutcome{Kind: kind, Code: &code, Values: values}
	case "refused":
		if !exactFields(fields, "kind", "failure") {
			return errors.New("invalid refused Programs outcome")
		}
		failure, decodeError := decodeProgramFailure(fields["failure"])
		if decodeError != nil {
			return decodeError
		}
		*outcome = ProgramOutcome{Kind: kind, Failure: &failure}
	default:
		return errors.New("unknown Programs outcome tag")
	}
	return nil
}

type ProgramExecutionDocument struct {
	TransactionHash string                   `json:"transaction_hash"`
	RawTransaction  []byte                   `json:"-"`
	ReceiptKeys     ReceiptKeys              `json:"-"`
	State           string                   `json:"state"`
	ActivityID      string                   `json:"activity_id"`
	ProgramID       string                   `json:"program_id"`
	GuestABIVersion uint16                   `json:"guest_abi_version"`
	ModuleVersion   uint32                   `json:"module_version"`
	BatchID         string                   `json:"batch_id"`
	GlobalSequence  string                   `json:"global_sequence"`
	ResultCode      int32                    `json:"result_code"`
	StateRoot       string                   `json:"state_root"`
	Receipt         string                   `json:"receipt"`
	ReceiptDigest   string                   `json:"receipt_digest"`
	TerminalPayload string                   `json:"terminal_payload"`
	CallGraph       string                   `json:"call_graph"`
	Usage           ProgramUsage             `json:"usage"`
	Outcome         ProgramOutcome           `json:"outcome"`
	Verification    string                   `json:"verification"`
	Authority       ProgramResponseAuthority `json:"authority"`
	IdempotencyKey  string                   `json:"idempotency_key,omitempty"`
}

type ProgramSimulationEvidence struct {
	BoundaryID            string `json:"boundary_id"`
	ActivityID            string `json:"activity_id"`
	PreviousStateRoot     string `json:"previous_state_root"`
	HypotheticalStateRoot string `json:"hypothetical_state_root"`
	ObservedSequence      string `json:"observed_sequence"`
	ObservedAt            string `json:"observed_at"`
	Committed             bool   `json:"committed"`
	PublicKey             string `json:"public_key"`
	Signature             string `json:"signature"`
}

type ProgramSimulation struct {
	ResultCode         int32
	TerminalKind       uint8
	Response           []byte
	Committed          bool
	Execution          ProgramExecutionDocument
	SimulationEvidence ProgramSimulationEvidence
	Verification       VerifiedProgramReceipt
}

type ProgramSubmissionState string

const (
	ProgramSubmissionPrepared ProgramSubmissionState = "prepared"
	ProgramSubmissionPending  ProgramSubmissionState = "pending"
	ProgramSubmissionRefused  ProgramSubmissionState = "refused"
	ProgramSubmissionUnknown  ProgramSubmissionState = "unknown"
	ProgramSubmissionExecuted ProgramSubmissionState = "executed"
)

type ProgramSubmission struct {
	Transaction            *ProgramTransaction
	TransactionHash        [32]byte
	SequencerReceipt       *SequencerReceiptVerification
	IdempotencyResult      []byte
	State                  ProgramSubmissionState
	ActivityID             [32]byte
	IdempotencyKey         string
	RetainedSignedActivity []byte
	Execution              *ProgramExecutionDocument
	Verification           *VerifiedProgramReceipt
}

type ProgramTransferVerification string

const (
	ProgramTransfersReconstructed ProgramTransferVerification = "reconstructed"
	ProgramTransfersRecorded      ProgramTransferVerification = "recorded_terminal_root_not_locally_reconstructable"
)

type VerifiedProgramReceipt struct {
	SequencerReceipt         *SequencerReceiptVerification
	TransferVerification     ProgramTransferVerification
	Verification             VerifiedReceipt
	TerminalPayload          []byte
	CallGraph                []byte
	OccupancyPaymentAccounts [][32]byte
}

// OccupancyPayer offers a payer DID, and optionally one of the two payment
// accounts that DID derives, for a state-commitment occupancy charge.
type OccupancyPayer struct {
	DID     []byte
	Account []byte
}

const (
	programStateCommitmentProtocolVersion = 3
	programMaxOccupancyPayers             = 256
	programMaxPayerDIDBytes               = 255
)

type occupancyPaymentAccount struct {
	Payer   [32]byte
	Asset   [32]byte
	Account [32]byte
}

type programCallBinding struct {
	ActivityID     [32]byte
	IdempotencyKey [32]byte
	NotBefore      uint64
	NotAfter       uint64
}

func validateProgramCall(call ProgramCall) error {
	if call.Budget.Fuel == 0 || len(call.Calldata) > MaximumProgramCalldataBytes || len(call.Capabilities) > MaximumProgramCapabilities || len(call.SignedActivity) > MaximumProgramCalldataBytes {
		return newSDKError(ErrorInvalidArgument, RetryNever)
	}
	order := map[ProgramCapability]int{ProgramStorageRead: 1, ProgramStorageWrite: 2, ProgramTransfer: 3, ProgramEmitEvent: 4, ProgramCompose: 5}
	prior := 0
	for _, capability := range call.Capabilities {
		current, ok := order[capability]
		if !ok || current <= prior {
			return newSDKError(ErrorInvalidArgument, RetryNever)
		}
		prior = current
	}
	if call.NativeCall != nil {
		native := call.NativeCall
		if call.ProgramID != native.ProgramID || !bytes.Equal(call.Calldata, native.Calldata) || call.Budget.Fuel != native.Resources[0] {
			return lifecycleInvalid()
		}
		if _, err := EncodeNativeProgramCall(*native); err != nil {
			return err
		}
		if len(call.SignedActivity) == 0 {
			return nil
		}
	}
	_, err := bindSignedProgramCall(call)
	return err
}

func bindSignedProgramCall(call ProgramCall) (programCallBinding, error) {
	if call.Budget.Fuel == 0 || len(call.Calldata) > MaximumProgramCalldataBytes || len(call.Capabilities) > MaximumProgramCapabilities || len(call.SignedActivity) == 0 || len(call.SignedActivity) > MaximumProgramCalldataBytes {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	decoder := wireDecoder{value: call.SignedActivity}
	envelopeVersion := decoder.u16()
	if envelopeVersion != 1 && envelopeVersion != 2 && envelopeVersion != 3 || decoder.u16() != 0x1001 || decoder.u8() != 12 {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	field := func(expected byte) bool { return decoder.u8() == expected && !decoder.failed }
	if !field(1) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	protocolVersion := decoder.u16()
	if protocolVersion != envelopeVersion || !field(2) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	_ = decoder.u32()
	if !field(3) || decoder.u32() != 0x0009_0003 || !field(4) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	_ = decoder.bounded(255)
	if !field(5) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	_ = decoder.bounded(524_288)
	if !field(6) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	_ = decoder.u64()
	if !field(7) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	notBefore := decoder.u64()
	notAfter := decoder.u64()
	if notAfter < notBefore || !field(8) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	idempotencyKey := decoder.array32()
	if !field(9) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	envelopeFeeLimit := decoder.u128()
	if call.NativeCall != nil && (protocolVersion != 3 || envelopeFeeLimit != call.Budget.FeeLimit) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	if !field(10) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	payloadHash := decoder.array32()
	if !field(11) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	payload := decoder.bounded(524_288)
	if !field(12) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	_ = decoder.bounded(128)
	if decoder.failed || decoder.offset != len(call.SignedActivity) || domainDigest([]byte("LXP/v1/payload-hash\x00"), payload) != payloadHash || !programPayloadMatchesCall(payload, call) {
		return programCallBinding{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	return programCallBinding{ActivityID: domainDigest([]byte("LXP/v1/activity-id\x00"), call.SignedActivity), IdempotencyKey: idempotencyKey, NotBefore: notBefore, NotAfter: notAfter}, nil
}

func programPayloadMatchesCall(payload []byte, call ProgramCall) bool {
	if call.NativeCall != nil {
		native := call.NativeCall
		encoded, err := EncodeNativeProgramCall(*native)
		return err == nil && native.ProgramID == call.ProgramID && bytes.Equal(native.Calldata, call.Calldata) && native.Resources[0] == call.Budget.Fuel && len(call.Capabilities) == 0 && bytes.Equal(payload, encoded)
	}

	domain := []byte("LayerX/programs/call/v1\x00")
	if !bytes.HasPrefix(payload, domain) {
		return false
	}
	decoder := wireDecoder{value: payload[len(domain):]}
	programID := decoder.fixed(32)
	fuel := decoder.u64()
	feeLimit := decoder.u128()
	capabilityCount := decoder.u16()
	if decoder.failed || int(capabilityCount) > MaximumProgramCapabilities {
		return false
	}
	capabilities := decoder.fixed(int(capabilityCount))
	calldataLength := decoder.u32()
	calldata := decoder.fixed(int(calldataLength))
	if decoder.failed || decoder.offset != len(decoder.value) || len(programID) != 32 || bytes.Equal(programID, make([]byte, 32)) || fuel == 0 || fuel != call.Budget.Fuel || feeLimit != call.Budget.FeeLimit || !bytes.Equal(programID, call.ProgramID[:]) || !bytes.Equal(calldata, call.Calldata) || len(capabilities) != len(call.Capabilities) {
		return false
	}
	previous := byte(0)
	for index, tag := range capabilities {
		if tag < 1 || tag > 5 || tag <= previous || ProgramCapability([]string{"", "storage_read", "storage_write", "transfer", "emit_event", "compose"}[tag]) != call.Capabilities[index] {
			return false
		}
		previous = tag
	}
	return true
}

func VerifyProgramReceipt(execution ProgramExecutionDocument, selectedProtocol ...uint16) (VerifiedProgramReceipt, error) {
	return VerifyProgramReceiptWithPayers(execution, nil, selectedProtocol...)
}

// VerifyProgramReceiptWithPayers verifies like VerifyProgramReceipt and proves
// the offered occupancy payers, which a state-commitment receipt with a paid
// occupancy charge requires.
func VerifyProgramReceiptWithPayers(execution ProgramExecutionDocument, payers []OccupancyPayer, selectedProtocol ...uint16) (VerifiedProgramReceipt, error) {
	verified, err := VerifySequencerReceipt([]byte(execution.Receipt), execution.RawTransaction, execution.ReceiptKeys)
	if err != nil {
		return VerifiedProgramReceipt{}, err
	}
	expected, ok := receiptHex(execution.TransactionHash)
	if !ok || len(expected) != 32 || !bytes.Equal(expected, verified.Receipt.ActivityID[:]) {
		return VerifiedProgramReceipt{}, verificationFailure()
	}
	return VerifiedProgramReceipt{Verification: verified.VerifiedReceipt, SequencerReceipt: &verified}, nil
}

type programTerminalProjection struct {
	Outcome                 ProgramOutcome
	RuntimeVersion          uint16
	FeeScheduleVersion      uint32
	MeteringScheduleVersion uint32
	CPUFuel                 uint64
	MemoryBytes             uint64
	StorageReadBytes        uint64
	StorageWriteBytes       uint64
	OutputValues            uint32
	OutputBytes             uint64
	FeeUnits                Uint128
	Candidate               bool
	Successful              bool
	EmbeddedGraph           []byte
	Occupancy               []byte
	TransferAuthorization   []byte
	TransferRoot            [32]byte
}

func verifyProgramTerminal(execution ProgramExecutionDocument, receipt ProtocolReceipt, terminal []byte, graph []byte, payers []OccupancyPayer) (ProgramTransferVerification, [][32]byte, error) {
	receiptOutcome := receipt.ProgramOutcome
	if receiptOutcome == nil {
		return "", nil, errors.New("missing Programs receipt outcome")
	}
	var occupancyAccounts [][32]byte
	programID, err := programHex32(execution.ProgramID)
	if err != nil {
		return "", nil, err
	}
	if len(terminal) == 0 || len(terminal) > 1_048_576 || sha256.Sum256(terminal) != receiptOutcome.TerminalPayloadRoot || len(graph) == 0 || sha256.Sum256(graph) != receiptOutcome.CallGraphRoot {
		return "", nil, errors.New("Programs terminal or graph root mismatch")
	}
	inner, err := unwrapAppliedProgramTerminal(terminal, *receiptOutcome)
	if err != nil {
		return "", nil, err
	}
	nativeV5 := receipt.ProtocolVersion == 3 && (receipt.ModuleVersion == 4 || receipt.ModuleVersion == 5) && (receiptOutcome.ABIVersion == ProgramAbiV3 || receiptOutcome.ABIVersion == ProgramAbiV4 || receiptOutcome.ABIVersion == ProgramAbiV5) && uint32(receiptOutcome.ABIVersion) <= receipt.ModuleVersion
	projection, err := decodeProgramTerminalProfile(receiptOutcome.TerminalKind, receiptOutcome.ABIVersion, inner, programID, receiptOutcome.ResultCode, nativeV5)
	nativeStandalone := nativeV5 && !projection.Candidate
	usageMismatch := !nativeStandalone && (projection.RuntimeVersion != receiptOutcome.RuntimeVersion || projection.Candidate && projection.FeeScheduleVersion != receiptOutcome.FeeScheduleVersion || projection.MeteringScheduleVersion != receiptOutcome.MeteringScheduleVersion || projection.CPUFuel != receiptOutcome.CPUFuel || projection.MemoryBytes != receiptOutcome.MemoryBytes || projection.StorageReadBytes != receiptOutcome.StorageReadBytes || projection.StorageWriteBytes != receiptOutcome.StorageWriteBytes || projection.OutputValues != receiptOutcome.OutputValues || projection.OutputBytes != receiptOutcome.OutputBytes || !projection.FeeUnits.Equal(receiptOutcome.FeeUnits))
	if err != nil || usageMismatch || !programOutcomesEqual(projection.Outcome, execution.Outcome) {
		return "", nil, errors.New("Programs terminal projection mismatch")
	}
	if projection.Candidate && !bytes.Equal(projection.EmbeddedGraph, graph) {
		return "", nil, errors.New("Programs embedded call graph mismatch")
	}
	occupancyRequired := (receipt.ProtocolVersion == 2 || receipt.ProtocolVersion == 3) && projection.Successful
	if occupancyRequired != (projection.Occupancy != nil) {
		return "", nil, errors.New("Programs occupancy attachment mismatch")
	}
	if projection.Occupancy == nil {
		if receiptOutcome.OccupancyEvidenceDigest != ([32]byte{}) || receiptOutcome.OccupancyTransferRoot != ([32]byte{}) || receiptOutcome.OccupancyByteBatches != (Uint128{}) || receiptOutcome.OccupancyFeeUnits != (Uint128{}) {
			return "", nil, errors.New("Programs receipt carries unattached occupancy")
		}
	} else if len(projection.Occupancy) == 0 {
		if receiptOutcome.OccupancyEvidenceDigest != ([32]byte{}) || receiptOutcome.OccupancyTransferRoot != ([32]byte{}) || receiptOutcome.OccupancyByteBatches != (Uint128{}) || receiptOutcome.OccupancyFeeUnits != (Uint128{}) {
			return "", nil, errors.New("Programs empty occupancy attachment mismatch")
		}
	} else if sha256.Sum256(projection.Occupancy) != receiptOutcome.OccupancyEvidenceDigest {
		return "", nil, errors.New("Programs occupancy digest mismatch")
	} else {
		occupancy, occupancyError := decodeProgramOccupancy(projection.Occupancy, receiptOutcome.OccupancyAssetID)
		if occupancyError != nil || !occupancy.ByteBatches.Equal(receiptOutcome.OccupancyByteBatches) || !occupancy.FeeUnits.Equal(receiptOutcome.OccupancyFeeUnits) {
			return "", nil, errors.New("Programs occupancy settlement mismatch")
		}
		proven, rootError := verifyProgramOccupancyTransferRoot(occupancy.PaidByPayer, receipt.ProtocolVersion, receiptOutcome.OccupancyAssetID, receiptOutcome.OccupancyTransferRoot, payers)
		if rootError != nil {
			return "", nil, rootError
		}
		occupancyAccounts = proven
	}
	recorded := receiptOutcome.EncodingVersion != 4 && projection.TransferAuthorization == nil && receiptOutcome.TransferRoot != ([32]byte{})
	authorityRequired := projection.Candidate || receiptOutcome.EncodingVersion == 4 && projection.Successful
	if !recorded && ((!authorityRequired && projection.TransferAuthorization != nil) || authorityRequired && (projection.TransferAuthorization != nil) != (receiptOutcome.TransferRoot != ([32]byte{}))) {
		return "", nil, errors.New("Programs transfer authority presence mismatch")
	}
	if receiptOutcome.EncodingVersion == 4 && projection.TransferAuthorization != nil && !programAuthorizationV2(projection.TransferAuthorization) {
		return "", nil, errors.New("Programs V2 transfer authority required")
	}
	if projection.TransferAuthorization != nil && (projection.TransferRoot != receiptOutcome.TransferRoot || verifyProgramTransferAuthorization(projection.TransferAuthorization, projection.TransferRoot) != nil) {
		return "", nil, errors.New("Programs transfer authority root mismatch")
	}
	if recorded {
		return ProgramTransfersRecorded, occupancyAccounts, nil
	}
	return ProgramTransfersReconstructed, occupancyAccounts, nil
}

func programOutcomesEqual(left ProgramOutcome, right ProgramOutcome) bool {
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case "completed":
		return left.Code != nil && right.Code != nil && *left.Code == *right.Code && bytes.Equal(left.Response, right.Response)
	case "legacy_completed":
		if left.Code == nil || right.Code == nil || *left.Code != *right.Code || len(left.Values) != len(right.Values) {
			return false
		}
		for index := range left.Values {
			if left.Values[index].Type != right.Values[index].Type || left.Values[index].Value != right.Values[index].Value {
				return false
			}
		}
		return true
	case "refused":
		return left.Failure != nil && right.Failure != nil && reflectProgramFailure(*left.Failure, *right.Failure)
	default:
		return false
	}
}

func reflectProgramFailure(left ProgramFailure, right ProgramFailure) bool {
	return left.Kind == right.Kind && equalProgramUint32(left.Limit, right.Limit) && equalProgramUint32(left.Attempted, right.Attempted) && equalProgramInt32(left.Code, right.Code)
}

func equalProgramUint32(left *uint32, right *uint32) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func equalProgramInt32(left *int32, right *int32) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func decodeProgramTerminal(kind uint8, abi uint16, encoded []byte, expectedProgram [32]byte, resultCode int32) (programTerminalProjection, error) {
	return decodeProgramTerminalProfile(kind, abi, encoded, expectedProgram, resultCode, false)
}

func decodeProgramTerminalProfile(kind uint8, abi uint16, encoded []byte, expectedProgram [32]byte, resultCode int32, nativeV5 bool) (programTerminalProjection, error) {
	inner := encoded
	projection := programTerminalProjection{}
	authorityDomain := []byte("LXP/program-execution-with-transfer-authority/v2\x00")
	occupancyDomain := []byte("LXP/program-execution-with-occupancy/v1\x00")
	if bytes.HasPrefix(inner, authorityDomain) {
		cursor := programTerminalCursor{value: inner[len(authorityDomain):]}
		inner = cursor.sized32()
		projection.TransferAuthorization = append([]byte{}, cursor.sized32()...)
		copy(projection.TransferRoot[:], cursor.take(32))
		if cursor.failed || !cursor.finished() {
			return programTerminalProjection{}, errors.New("invalid Programs transfer authority wrapper")
		}
	}
	if bytes.HasPrefix(inner, occupancyDomain) {
		cursor := programTerminalCursor{value: inner[len(occupancyDomain):]}
		inner = cursor.sized32()
		projection.Occupancy = append([]byte{}, cursor.sized32()...)
		if cursor.failed || !cursor.finished() {
			return programTerminalProjection{}, errors.New("invalid Programs occupancy wrapper")
		}
	}
	if bytes.HasPrefix(inner, authorityDomain) || bytes.HasPrefix(inner, occupancyDomain) {
		return programTerminalProjection{}, errors.New("invalid nested Programs terminal wrapper")
	}
	legacyV2 := []byte("LXP/program-execution/v2\x00")
	legacyV3 := []byte("LXP/program-execution/v3\x00")
	candidateV4 := []byte("LXP/program-execution/v4\x00")
	candidateV5 := []byte("LXP/program-execution/v5\x00")
	switch {
	case bytes.HasPrefix(inner, legacyV2), bytes.HasPrefix(inner, legacyV3):
		if kind != 1 || abi != 1 {
			return programTerminalProjection{}, errors.New("Programs legacy terminal kind mismatch")
		}
		traced := bytes.HasPrefix(inner, legacyV3)
		domain := legacyV2
		if traced {
			domain = legacyV3
		}
		cursor := programTerminalCursor{value: inner[len(domain):]}
		projection.RuntimeVersion = cursor.u16()
		terminalABI := cursor.u16()
		projection.MeteringScheduleVersion = cursor.u32()
		countValue := cursor.take(16)
		if len(countValue) != 16 || !allProgramZero(countValue[:8]) {
			return programTerminalProjection{}, errors.New("Programs legacy value count overflow")
		}
		count := binary.BigEndian.Uint64(countValue[8:])
		if count > maximumProgramLegacyValues {
			return programTerminalProjection{}, errors.New("Programs legacy value count exceeds bound")
		}
		values := make([]ProgramLegacyValue, 0, count)
		for index := uint64(0); index < count; index++ {
			switch cursor.byte() {
			case 1:
				values = append(values, ProgramLegacyValue{Type: "i32", Value: float64(cursor.i32())})
			case 2:
				values = append(values, ProgramLegacyValue{Type: "i64", Value: strconv.FormatInt(cursor.i64(), 10)})
			default:
				return programTerminalProjection{}, errors.New("invalid Programs legacy value tag")
			}
		}
		projection.CPUFuel = cursor.u64()
		projection.MemoryBytes = cursor.u64()
		projection.StorageReadBytes = cursor.u64()
		projection.StorageWriteBytes = cursor.u64()
		projection.OutputValues = cursor.u32()
		projection.OutputBytes = 0
		projection.FeeUnits = cursor.u128()
		if traced {
			if cursor.byte() != 1 || len(cursor.sized64()) > 34+512*52 {
				return programTerminalProjection{}, errors.New("invalid Programs legacy trace")
			}
		}
		if cursor.failed || !cursor.finished() || terminalABI != abi || projection.RuntimeVersion == 0 || projection.MeteringScheduleVersion == 0 {
			return programTerminalProjection{}, errors.New("invalid Programs legacy terminal")
		}
		code := resultCode
		projection.Outcome = ProgramOutcome{Kind: "legacy_completed", Code: &code, Values: values}
		projection.Successful = true
		return projection, nil
	case bytes.HasPrefix(inner, candidateV4), bytes.HasPrefix(inner, candidateV5):
		v5 := bytes.HasPrefix(inner, candidateV5)
		if v5 && (!nativeV5 || abi != ProgramAbiV3 && abi != ProgramAbiV4 && abi != ProgramAbiV5) || !v5 && abi != 2 {
			return programTerminalProjection{}, errors.New("Programs terminal profile ABI mismatch")
		}
		domain := candidateV4
		if v5 {
			domain = candidateV5
		}
		projection.Candidate = true
		cursor := programTerminalCursor{value: inner[len(domain):]}
		projection.RuntimeVersion = cursor.u16()
		projection.FeeScheduleVersion = cursor.u32()
		projection.MeteringScheduleVersion = cursor.u32()
		valueCount := cursor.u64()
		if valueCount > maximumProgramLegacyValues {
			return programTerminalProjection{}, errors.New("Programs candidate value count exceeds bound")
		}
		for index := uint64(0); index < valueCount; index++ {
			switch cursor.byte() {
			case 1:
				_ = cursor.i32()
			case 2:
				_ = cursor.i64()
			default:
				return programTerminalProjection{}, errors.New("invalid Programs candidate value tag")
			}
		}
		projection.CPUFuel = cursor.u64()
		projection.MemoryBytes = cursor.u64()
		projection.StorageReadBytes = cursor.u64()
		projection.StorageWriteBytes = cursor.u64()
		projection.OutputValues = cursor.u32()
		projection.OutputBytes = cursor.u64()
		projection.FeeUnits = cursor.u128()
		switch cursor.byte() {
		case 0:
		case 1:
			if len(cursor.sized64()) > 34+512*52 {
				return programTerminalProjection{}, errors.New("Programs candidate trace exceeds bound")
			}
		default:
			return programTerminalProjection{}, errors.New("invalid Programs candidate trace tag")
		}
		var program [32]byte
		copy(program[:], cursor.take(32))
		terminalABI := cursor.u16()
		outcomeTag := cursor.byte()
		switch outcomeTag {
		case 0:
			code := cursor.i32()
			response := append([]byte(nil), cursor.sized64()...)
			if code < 0 || len(response) > MaximumProgramCalldataBytes || kind != 1 {
				return programTerminalProjection{}, errors.New("invalid Programs candidate response")
			}
			projection.Outcome = ProgramOutcome{Kind: "completed", Code: &code, Response: response}
			projection.Successful = true
		case 1:
			if !validCanonicalProgramFailure(cursor.sized64()) || kind != 2 {
				return programTerminalProjection{}, errors.New("invalid Programs candidate failure")
			}
			code := resultCode
			projection.Outcome = ProgramOutcome{Kind: "refused", Failure: &ProgramFailure{Kind: "guest_refused", Code: &code}}
		case 2:
			if !consumeProgramMeterRefusal(&cursor, projection) || kind != 3 {
				return programTerminalProjection{}, errors.New("invalid Programs candidate resource refusal")
			}
			projection.Outcome = ProgramOutcome{Kind: "refused", Failure: &ProgramFailure{Kind: "resource"}}
		default:
			return programTerminalProjection{}, errors.New("invalid Programs candidate outcome tag")
		}
		projection.EmbeddedGraph = append([]byte(nil), cursor.sized64()...)
		if cursor.failed || !cursor.finished() || len(projection.EmbeddedGraph) > MaximumProgramCalldataBytes || program != expectedProgram || terminalABI != abi || !v5 && abi != 2 || projection.RuntimeVersion == 0 || projection.FeeScheduleVersion == 0 || projection.MeteringScheduleVersion == 0 {
			return programTerminalProjection{}, errors.New("invalid Programs candidate terminal")
		}
		return projection, nil
	default:
		if nativeV5 && (abi == ProgramAbiV3 || abi == ProgramAbiV4 || abi == ProgramAbiV5) {
			return decodeNativeProgramCallbackSettlement(kind, inner, resultCode, projection)
		}
		return decodeProgramFailureTerminal(kind, abi, inner, resultCode, projection)
	}
}

func decodeNativeProgramCallbackSettlement(kind uint8, inner []byte, resultCode int32, projection programTerminalProjection) (programTerminalProjection, error) {
	settlement := []byte("LXP/programs/settlement-failure/v1\x00")
	callback := []byte("LXP/programs/callback-failure/v1\x00")
	valid := bytes.HasPrefix(inner, settlement) && len(inner) == len(settlement)+1 && validProgramTransferError(inner[len(settlement)]) || bytes.HasPrefix(inner, callback) && len(inner) == len(callback)+5
	if kind != 2 || !valid || projection.Occupancy != nil || projection.TransferAuthorization != nil {
		return programTerminalProjection{}, errors.New("invalid native Programs callback or settlement refusal")
	}
	code := resultCode
	projection.Outcome = ProgramOutcome{Kind: "refused", Failure: &ProgramFailure{Kind: "guest_refused", Code: &code}}
	return projection, nil
}

func decodeProgramFailureTerminal(kind uint8, abi uint16, inner []byte, resultCode int32, projection programTerminalProjection) (programTerminalProjection, error) {
	if abi != 1 && abi != 2 {
		return programTerminalProjection{}, errors.New("invalid Programs failure ABI")
	}
	failureDomain := []byte("LXP/programs/failure-detail/v1\x00")
	resourceDomain := []byte("LXP/programs/resource-detail/v1\x00")
	settlementDomain := []byte("LXP/programs/settlement-failure/v1\x00")
	callbackDomain := []byte("LXP/programs/callback-failure/v1\x00")
	code := resultCode
	switch {
	case bytes.HasPrefix(inner, failureDomain):
		cursor := programTerminalCursor{value: inner[len(failureDomain):]}
		tag := cursor.byte()
		payload := cursor.sized32()
		if cursor.failed || !cursor.finished() || tag < 1 || tag > 4 || len(payload) == 0 || kind != 2 {
			return programTerminalProjection{}, errors.New("invalid Programs failure terminal")
		}
		if !validProgramFailureDetail(tag, payload) {
			return programTerminalProjection{}, errors.New("invalid Programs authenticated failure")
		}
		projection.Outcome = ProgramOutcome{Kind: "refused", Failure: &ProgramFailure{Kind: "guest_refused", Code: &code}}
	case bytes.HasPrefix(inner, resourceDomain):
		cursor := programTerminalCursor{value: inner[len(resourceDomain):]}
		if !consumeStandaloneProgramResource(&cursor) || cursor.failed || !cursor.finished() || kind != 3 {
			return programTerminalProjection{}, errors.New("invalid Programs resource terminal")
		}
		projection.Outcome = ProgramOutcome{Kind: "refused", Failure: &ProgramFailure{Kind: "resource"}}
	case bytes.HasPrefix(inner, settlementDomain):
		if kind != 2 || len(inner) != len(settlementDomain)+1 || !validProgramTransferError(inner[len(settlementDomain)]) {
			return programTerminalProjection{}, errors.New("invalid Programs settlement terminal")
		}
		projection.Outcome = ProgramOutcome{Kind: "refused", Failure: &ProgramFailure{Kind: "guest_refused", Code: &code}}
	case bytes.HasPrefix(inner, callbackDomain):
		if kind != 2 || len(inner) != len(callbackDomain)+5 {
			return programTerminalProjection{}, errors.New("invalid Programs callback terminal")
		}
		projection.Outcome = ProgramOutcome{Kind: "refused", Failure: &ProgramFailure{Kind: "guest_refused", Code: &code}}
	default:
		return programTerminalProjection{}, errors.New("unknown Programs terminal domain")
	}
	return projection, nil
}

func validCanonicalProgramFailure(encoded []byte) bool {
	if len(encoded) < 40 {
		return false
	}
	program := encoded[:32]
	class := binary.BigEndian.Uint32(encoded[32:36])
	reasonLength := binary.BigEndian.Uint32(encoded[36:40])
	if allProgramZero(program) || reasonLength > 4096 || int(reasonLength) != len(encoded)-40 || !(class >= 1 && class <= 5 || class == 254 || class == 255) {
		return false
	}
	return (class != 254 && class != 255) || reasonLength == 0
}

func validProgramFailureDetail(tag byte, payload []byte) bool {
	if tag == 1 {
		return validCanonicalProgramFailure(payload)
	}
	cursor := programTerminalCursor{value: payload}
	switch tag {
	case 2:
		if !consumeProgramCompositionFailure(&cursor) {
			return false
		}
	case 3:
		if !consumeProgramEntrypointFailure(&cursor) {
			return false
		}
	case 4:
		if !consumeProgramABIFailure(&cursor) {
			return false
		}
	default:
		return false
	}
	return cursor.finished()
}

func consumeProgramCompositionFailure(cursor *programTerminalCursor) bool {
	switch cursor.byte() {
	case 1, 9, 10, 11, 20, 21, 22:
	case 2:
		expected := cursor.byte()
		actual := cursor.byte()
		if (expected != 1 && expected != 2) || (actual != 1 && actual != 2) {
			return false
		}
	case 23:
		_ = cursor.take(76)
		_ = cursor.take(76)
	case 3, 4:
		_ = cursor.take(32)
	case 5, 6, 7:
		_ = cursor.u32()
		_ = cursor.u32()
	case 8:
		_ = cursor.take(32)
		_ = cursor.u32()
		_ = cursor.u32()
	case 12:
		_ = cursor.i32()
	case 13:
		_ = cursor.u64()
		_ = cursor.u64()
	case 14:
		_ = cursor.take(32)
		_ = cursor.i32()
	case 15:
		return validCanonicalProgramFailure(cursor.rest())
	case 16:
		return consumeNestedProgramABI(cursor)
	case 17:
		return consumeProgramFault(cursor)
	case 18:
		return consumeProgramMeterFailure(cursor)
	case 19:
		return consumeProgramResponseFailure(cursor)
	default:
		return false
	}
	return !cursor.failed
}

func consumeProgramEntrypointFailure(cursor *programTerminalCursor) bool {
	switch cursor.byte() {
	case 1:
		_ = cursor.u64()
		_ = cursor.u64()
	case 2, 3, 4:
	case 5, 6:
		_ = cursor.i32()
	case 7:
		return consumeProgramFault(cursor)
	case 8:
		return consumeProgramMeterFailure(cursor)
	default:
		return false
	}
	return !cursor.failed
}

func consumeProgramABIFailure(cursor *programTerminalCursor) bool {
	tag := cursor.byte()
	if tag >= 1 && tag <= 10 || tag >= 13 && tag <= 15 {
		return !cursor.failed
	}
	if tag == 11 {
		storage := cursor.byte()
		return !cursor.failed && storage >= 1 && storage <= 11
	}
	return tag == 12 && consumeProgramMeterFailure(cursor)
}

func consumeNestedProgramABI(cursor *programTerminalCursor) bool {
	return consumeProgramABIFailure(cursor)
}

func consumeProgramMeterFailure(cursor *programTerminalCursor) bool {
	switch cursor.byte() {
	case 1:
		resource := cursor.byte()
		limit := cursor.u64()
		attempted := cursor.u64()
		return !cursor.failed && resource >= 1 && resource <= 7 && attempted > limit
	case 2:
		resource := cursor.byte()
		return !cursor.failed && resource >= 1 && resource <= 7
	case 3:
		return !cursor.failed
	default:
		return false
	}
}

func consumeProgramFault(cursor *programTerminalCursor) bool {
	tag := cursor.byte()
	if tag == 1 || tag == 2 || tag == 16 {
		name := cursor.sized32()
		return !cursor.failed && utf8.Valid(name)
	}
	if tag >= 3 && tag <= 13 || tag == 15 {
		return !cursor.failed
	}
	return tag == 14 && consumeProgramMeterFailure(cursor)
}

func consumeProgramResponseFailure(cursor *programTerminalCursor) bool {
	switch cursor.byte() {
	case 1, 2:
		_ = cursor.u64()
		_ = cursor.u64()
	case 3, 4:
	case 5:
		_ = cursor.i32()
		_ = cursor.i32()
	case 6:
		return consumeProgramMeterFailure(cursor)
	default:
		return false
	}
	return !cursor.failed
}

func consumeProgramMeterRefusal(cursor *programTerminalCursor, projection programTerminalProjection) bool {
	tag := cursor.byte()
	resource := cursor.byte()
	if resource > 6 {
		return false
	}
	if tag == 1 {
		return !cursor.failed
	}
	if tag != 0 {
		return false
	}
	limit := cursor.u64()
	attempted := cursor.u64()
	usage := []uint64{projection.CPUFuel, projection.MemoryBytes, projection.StorageReadBytes, projection.StorageWriteBytes, uint64(projection.OutputValues), projection.OutputBytes, 0}[resource]
	return !cursor.failed && attempted > limit && usage <= limit
}

func consumeStandaloneProgramResource(cursor *programTerminalCursor) bool {
	tag := cursor.byte()
	resource := cursor.byte()
	if resource < 1 || resource > 7 {
		return false
	}
	if tag == 2 {
		return !cursor.failed
	}
	if tag != 1 {
		return false
	}
	limit := cursor.u64()
	attempted := cursor.u64()
	return !cursor.failed && attempted > limit
}

func validProgramTransferError(tag byte) bool { return tag >= 1 && tag <= 12 }

type programOccupancyProjection struct {
	ByteBatches Uint128
	FeeUnits    Uint128
	PaidByPayer map[[32]byte]Uint128
}

func decodeProgramOccupancy(encoded []byte, asset [32]byte) (programOccupancyProjection, error) {
	const domain = "LXP/storage-occupancy-settlement/v3\x00"
	if len(encoded) > 65_536 || asset == ([32]byte{}) {
		return programOccupancyProjection{}, errors.New("invalid Programs occupancy settlement")
	}
	if bytes.HasPrefix(encoded, []byte("LXP/storage-occupancy-settlement/v1\x00")) || bytes.HasPrefix(encoded, []byte("LXP/storage-occupancy-settlement/v2\x00")) {
		return decodeLegacyProgramOccupancy(encoded, asset)
	}
	if !bytes.HasPrefix(encoded, []byte(domain)) {
		return programOccupancyProjection{}, errors.New("invalid Programs occupancy settlement")
	}
	cursor := programTerminalCursor{value: encoded[len(domain):]}
	batch := cursor.u64()
	if cursor.u32() == 0 {
		return programOccupancyProjection{}, errors.New("invalid Programs occupancy schedule")
	}
	prices := [7]uint64{}
	for index := range prices {
		prices[index] = cursor.u64()
	}
	declaredBatches, declaredFees, declaredPaid, declaredArrears := cursor.u128(), cursor.u128(), cursor.u128(), cursor.u128()
	count := cursor.u32()
	if count > 256 {
		return programOccupancyProjection{}, errors.New("Programs occupancy positions exceed bound")
	}
	var batches, fees, paid, arrears Uint128
	var priorNamespace []byte
	paidByPayer := make(map[[32]byte]Uint128)
	for index := uint32(0); index < count; index++ {
		namespaceLength := cursor.byte()
		namespace := append([]byte(nil), cursor.take(int(namespaceLength))...)
		if (namespaceLength != 33 && namespaceLength != 65) || !validProgramNamespace(namespace) || priorNamespace != nil && bytes.Compare(priorNamespace, namespace) >= 0 {
			return programOccupancyProjection{}, errors.New("invalid Programs occupancy namespace")
		}
		priorNamespace = namespace
		payer := cursor.array32()
		rootProgram := cursor.array32()
		activity := cursor.array32()
		if payer == ([32]byte{}) || rootProgram == ([32]byte{}) || namespace[32] == 0 && !bytes.Equal(namespace[33:], payer[:]) {
			return programOccupancyProjection{}, errors.New("invalid Programs occupancy authority")
		}
		fromBatch, toBatch, recorded, finalBytes := cursor.u64(), cursor.u64(), cursor.u64(), cursor.u64()
		byteBatches, price, accrued := cursor.u128(), cursor.u64(), cursor.u128()
		priorArrears, amountDue, authorizedAdded := cursor.u128(), cursor.u128(), cursor.u128()
		disposition := cursor.byte()
		arrearsAfter := cursor.u128()
		maximumBytes, maximumPrice := cursor.u64(), cursor.u64()
		_ = cursor.u128()
		mandate := cursor.array32()
		intervals, underflow := subtractProgramUint64(toBatch, fromBatch)
		computedBatches, overflow := multiplyProgramU64(recorded, intervals)
		computedFees, feeOverflow := multiplyProgramUint128U64(computedBatches, price)
		computedDue, dueOK := priorArrears.Add(computedFees)
		migration := disposition == 5
		mandateExpected := programOccupancyMandate(payer, rootProgram, activity, namespace, maximumBytes, maximumPrice, authorizedAdded)
		invalid := cursor.failed || toBatch != batch || disposition < 1 || disposition > 5 || underflow || overflow || feeOverflow || !dueOK || !byteBatches.Equal(computedBatches) || !accrued.Equal(computedFees) || !amountDue.Equal(computedDue) || finalBytes > maximumBytes || !migration && price != prices[6] || !migration && (mandate == ([32]byte{}) || activity == ([32]byte{})) || migration && (price != 0 || accrued != (Uint128{}) || priorArrears != (Uint128{}) || amountDue != (Uint128{}) || arrearsAfter != (Uint128{}) || mandate != ([32]byte{}) || activity != ([32]byte{}) || !bytes.Equal(rootProgram[:], namespace[:32])) || authorizedAdded != (Uint128{}) && mandate != mandateExpected || (disposition == 4) != (price > maximumPrice) || disposition == 1 && arrearsAfter != (Uint128{}) || disposition != 1 && !arrearsAfter.Equal(amountDue)
		if invalid {
			return programOccupancyProjection{}, errors.New("invalid Programs occupancy charge")
		}
		var ok bool
		if batches, ok = batches.Add(byteBatches); !ok {
			return programOccupancyProjection{}, errors.New("Programs occupancy batch overflow")
		}
		if fees, ok = fees.Add(accrued); !ok {
			return programOccupancyProjection{}, errors.New("Programs occupancy fee overflow")
		}
		if disposition == 1 {
			if paid, ok = paid.Add(amountDue); !ok {
				return programOccupancyProjection{}, errors.New("Programs occupancy paid overflow")
			}
			current := paidByPayer[payer]
			if current, ok = current.Add(amountDue); !ok {
				return programOccupancyProjection{}, errors.New("Programs occupancy payer overflow")
			}
			paidByPayer[payer] = current
		} else if arrears, ok = arrears.Add(arrearsAfter); !ok {
			return programOccupancyProjection{}, errors.New("Programs occupancy arrears overflow")
		}
	}
	if cursor.failed || !cursor.finished() || !batches.Equal(declaredBatches) || !fees.Equal(declaredFees) || !paid.Equal(declaredPaid) || !arrears.Equal(declaredArrears) {
		return programOccupancyProjection{}, errors.New("Programs occupancy totals mismatch")
	}
	return programOccupancyProjection{ByteBatches: batches, FeeUnits: fees, PaidByPayer: paidByPayer}, nil
}

func decodeLegacyProgramOccupancy(encoded []byte, asset [32]byte) (programOccupancyProjection, error) {
	v2 := []byte("LXP/storage-occupancy-settlement/v2\x00")
	domain := []byte("LXP/storage-occupancy-settlement/v1\x00")
	versioned := bytes.HasPrefix(encoded, v2)
	if versioned {
		domain = v2
	}
	cursor := programTerminalCursor{value: encoded[len(domain):]}
	batch := cursor.u64()
	if versioned && cursor.u32() == 0 {
		return programOccupancyProjection{}, errors.New("invalid legacy Programs occupancy schedule")
	}
	prices := [7]uint64{}
	for index := range prices {
		prices[index] = cursor.u64()
	}
	declaredBatches, declaredFees := cursor.u128(), cursor.u128()
	count := cursor.u64()
	if count > 256 {
		return programOccupancyProjection{}, errors.New("legacy Programs occupancy positions exceed bound")
	}
	var batches, fees Uint128
	paid := make(map[[32]byte]Uint128)
	for index := uint64(0); index < count; index++ {
		length := cursor.byte()
		namespace := cursor.take(int(length))
		payer := cursor.array32()
		fromBatch, toBatch, recorded := cursor.u64(), cursor.u64(), cursor.u64()
		_ = cursor.u64()
		byteBatches, price, accrued := cursor.u128(), cursor.u64(), cursor.u128()
		intervals, underflow := subtractProgramUint64(toBatch, fromBatch)
		computed, overflow := multiplyProgramU64(recorded, intervals)
		computedFee, feeOverflow := multiplyProgramUint128U64(computed, price)
		if !validProgramNamespace(namespace) || payer == ([32]byte{}) || len(namespace) == 65 && namespace[32] == 0 && !bytes.Equal(namespace[33:], payer[:]) || toBatch != batch || underflow || overflow || feeOverflow || !byteBatches.Equal(computed) || price != prices[6] || !accrued.Equal(computedFee) {
			return programOccupancyProjection{}, errors.New("invalid legacy Programs occupancy charge")
		}
		var ok bool
		if batches, ok = batches.Add(byteBatches); !ok {
			return programOccupancyProjection{}, errors.New("legacy Programs occupancy overflow")
		}
		if fees, ok = fees.Add(accrued); !ok {
			return programOccupancyProjection{}, errors.New("legacy Programs occupancy overflow")
		}
		current := paid[payer]
		if current, ok = current.Add(accrued); !ok {
			return programOccupancyProjection{}, errors.New("legacy Programs occupancy payer overflow")
		}
		paid[payer] = current
	}
	if cursor.failed || !cursor.finished() || !batches.Equal(declaredBatches) || !fees.Equal(declaredFees) {
		return programOccupancyProjection{}, errors.New("legacy Programs occupancy totals mismatch")
	}
	return programOccupancyProjection{ByteBatches: batches, FeeUnits: fees, PaidByPayer: paid}, nil
}

func validProgramNamespace(value []byte) bool {
	return len(value) == 65 && (value[32] == 0 || value[32] == 2) && !allProgramZero(value[:32]) && !allProgramZero(value[33:]) || len(value) == 33 && value[32] == 1 && !allProgramZero(value[:32])
}
func subtractProgramUint64(left, right uint64) (uint64, bool) {
	if left < right {
		return 0, true
	}
	return left - right, false
}
func multiplyProgramU64(left, right uint64) (Uint128, bool) {
	value := new(big.Int).Mul(new(big.Int).SetUint64(left), new(big.Int).SetUint64(right))
	return programBigUint128(value)
}
func multiplyProgramUint128U64(left Uint128, right uint64) (Uint128, bool) {
	value := new(big.Int).Mul(programUint128Big(left), new(big.Int).SetUint64(right))
	return programBigUint128(value)
}
func programUint128Big(value Uint128) *big.Int {
	var encoded [16]byte
	binary.BigEndian.PutUint64(encoded[:8], value.High())
	binary.BigEndian.PutUint64(encoded[8:], value.Low())
	return new(big.Int).SetBytes(encoded[:])
}
func programBigUint128(value *big.Int) (Uint128, bool) {
	if value.Sign() < 0 || value.BitLen() > 128 {
		return Uint128{}, true
	}
	bytesValue := value.FillBytes(make([]byte, 16))
	return NewUint128(binary.BigEndian.Uint64(bytesValue[:8]), binary.BigEndian.Uint64(bytesValue[8:])), false
}

func programOccupancyMandate(payer, rootProgram, activity [32]byte, namespace []byte, maximumBytes, maximumPrice uint64, ceiling Uint128) [32]byte {
	material := []byte("LXP/storage-occupancy-mandate/v1\x00")
	material = append(material, payer[:]...)
	material = append(material, rootProgram[:]...)
	material = append(material, activity[:]...)
	material = append(material, byte(len(namespace)))
	material = append(material, namespace...)
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], maximumBytes)
	material = append(material, number[:]...)
	binary.BigEndian.PutUint64(number[:], maximumPrice)
	material = append(material, number[:]...)
	var wide [16]byte
	binary.BigEndian.PutUint64(wide[:8], ceiling.High())
	binary.BigEndian.PutUint64(wide[8:], ceiling.Low())
	material = append(material, wide[:]...)
	return sha256.Sum256(material)
}

func programOccupancyFeeTreasury() [32]byte {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], 11)
	return domainDigest([]byte("LX:ACCOUNT:v1"), length[:], []byte("system:fees"))
}

func programOccupancyPayingPayers(paid map[[32]byte]Uint128) [][32]byte {
	payers := make([][32]byte, 0, len(paid))
	for payer, amount := range paid {
		if amount != (Uint128{}) {
			payers = append(payers, payer)
		}
	}
	sort.Slice(payers, func(i, j int) bool { return bytes.Compare(payers[i][:], payers[j][:]) < 0 })
	return payers
}

func programOccupancyTransferLeaf(from, treasury, asset [32]byte, paid Uint128) [32]byte {
	leaf := []byte{0}
	leaf = append(leaf, from[:]...)
	leaf = append(leaf, treasury[:]...)
	leaf = append(leaf, asset[:]...)
	var amount [16]byte
	binary.BigEndian.PutUint64(amount[:8], paid.High())
	binary.BigEndian.PutUint64(amount[8:], paid.Low())
	leaf = append(leaf, amount[:]...)
	leaf = append(leaf, 0, 23)
	return domainDigest([]byte("LXP/v1/merkle-leaf\x00"), leaf)
}

func programOccupancyMerkleRoot(level [][32]byte) [32]byte {
	for len(level) > 1 {
		next := make([][32]byte, 0, (len(level)+1)/2)
		for index := 0; index < len(level); index += 2 {
			right := level[index]
			if index+1 < len(level) {
				right = level[index+1]
			}
			next = append(next, domainDigest([]byte("LXP/v1/merkle-internal\x00"), level[index][:], right[:]))
		}
		level = next
	}
	if len(level) == 0 {
		return [32]byte{}
	}
	return level[0]
}

func programOccupancyTransferRoot(paid map[[32]byte]Uint128, asset [32]byte) [32]byte {
	treasury := programOccupancyFeeTreasury()
	payers := programOccupancyPayingPayers(paid)
	level := make([][32]byte, 0, len(payers))
	for _, payer := range payers {
		level = append(level, programOccupancyTransferLeaf(payer, treasury, asset, paid[payer]))
	}
	return programOccupancyMerkleRoot(level)
}

// verifyProgramOccupancyTransferRoot rebuilds the committed occupancy transfer
// root. Protocol versions before the state commitment pay from the payer
// principal; the state commitment pays from one of the two accounts the payer
// DID derives, so each offered payer is proven against the settlement and the
// selection the receipt committed to is recovered.
func verifyProgramOccupancyTransferRoot(paid map[[32]byte]Uint128, protocolVersion uint16, asset [32]byte, committedRoot [32]byte, payers []OccupancyPayer) ([][32]byte, error) {
	if protocolVersion != programStateCommitmentProtocolVersion {
		if programOccupancyTransferRoot(paid, asset) != committedRoot {
			return nil, errors.New("Programs occupancy transfer root mismatch")
		}
		return nil, nil
	}
	if asset == ([32]byte{}) {
		return nil, errors.New("invalid Programs occupancy settlement")
	}
	paying := programOccupancyPayingPayers(paid)
	accounts, err := programProvenPayingAccounts(paying, payers, asset)
	if err != nil {
		return nil, err
	}
	if len(accounts) > 2*programMaxOccupancyPayers {
		return nil, errors.New("Programs occupancy payment account bound")
	}
	type programOccupancyPayerSelection struct {
		Paid   Uint128
		Proven [][32]byte
	}
	selected := make([]programOccupancyPayerSelection, 0, len(paying))
	selections := 1
	for _, payer := range paying {
		proven := make([][32]byte, 0, 2)
		for _, account := range accounts {
			if account.Payer != payer || account.Asset != asset {
				continue
			}
			known := false
			for _, candidate := range proven {
				known = known || candidate == account.Account
			}
			if !known {
				proven = append(proven, account.Account)
			}
		}
		if len(proven) == 0 {
			return nil, errors.New("Programs occupancy payment account unproven")
		}
		selections *= len(proven)
		if selections > programMaxOccupancyPayers {
			return nil, errors.New("Programs occupancy payment account bound")
		}
		selected = append(selected, programOccupancyPayerSelection{Paid: paid[payer], Proven: proven})
	}
	if len(selected) > programMaxOccupancyPayers {
		return nil, errors.New("Programs occupancy payment account bound")
	}
	treasury := programOccupancyFeeTreasury()
	for selection := 0; selection < selections; selection++ {
		remaining := selection
		chosen := make([][32]byte, 0, len(selected))
		level := make([][32]byte, 0, len(selected))
		for _, entry := range selected {
			account := entry.Proven[remaining%len(entry.Proven)]
			remaining /= len(entry.Proven)
			level = append(level, programOccupancyTransferLeaf(account, treasury, asset, entry.Paid))
			chosen = append(chosen, account)
		}
		if programOccupancyMerkleRoot(level) == committedRoot {
			return chosen, nil
		}
	}
	return nil, errors.New("Programs occupancy transfer root mismatch")
}

func programProvenPayingAccounts(paying [][32]byte, payers []OccupancyPayer, asset [32]byte) ([]occupancyPaymentAccount, error) {
	accounts := make([]occupancyPaymentAccount, 0, 2*len(payers))
	if len(paying) == 0 || len(paying) > programMaxOccupancyPayers {
		return accounts, nil
	}
	for _, payer := range payers {
		main, mainError := programOccupancyPaymentAccount(payer.DID, asset, false)
		if mainError != nil {
			continue
		}
		charged := false
		for _, candidate := range paying {
			charged = charged || candidate == main.Payer
		}
		if !charged {
			continue
		}
		scoped, scopedError := programOccupancyPaymentAccount(payer.DID, asset, true)
		if scopedError != nil {
			return nil, scopedError
		}
		if payer.Account == nil {
			accounts = append(accounts, main, scoped)
			continue
		}
		switch {
		case bytes.Equal(payer.Account, main.Account[:]):
			accounts = append(accounts, main)
		case bytes.Equal(payer.Account, scoped.Account[:]):
			accounts = append(accounts, scoped)
		default:
			return nil, errors.New("Programs occupancy payment account unproven")
		}
	}
	return accounts, nil
}

func programOccupancyPaymentAccount(did []byte, asset [32]byte, assetScoped bool) (occupancyPaymentAccount, error) {
	unproven := errors.New("Programs occupancy payment account unproven")
	if len(did) == 0 || len(did) > programMaxPayerDIDBytes || asset == ([32]byte{}) {
		return occupancyPaymentAccount{}, unproven
	}
	var length [4]byte
	binary.BigEndian.PutUint16(length[:2], uint16(len(did)))
	payer := domainDigest([]byte("LXP/v1/did-id\x00"), length[:2], did)
	if payer == ([32]byte{}) {
		return occupancyPaymentAccount{}, unproven
	}
	name := make([]byte, 0, 13+len(did)+64)
	name = append(name, []byte("agent:")...)
	name = append(name, did...)
	if assetScoped {
		name = append(name, []byte(":asset:")...)
		name = append(name, []byte(hex.EncodeToString(asset[:]))...)
	} else {
		name = append(name, []byte(":main")...)
	}
	binary.BigEndian.PutUint32(length[:], uint32(len(name)))
	return occupancyPaymentAccount{Payer: payer, Asset: asset, Account: domainDigest([]byte("LX:ACCOUNT:v1"), length[:], name)}, nil
}

func programAuthorizationV2(encoded []byte) bool {
	domain := []byte("LayerX/programs/402LXP/account-bound-set/v1\x00")
	if bytes.HasPrefix(encoded, domain) {
		cursor := programTerminalCursor{value: encoded[len(domain):]}
		encoded = cursor.take(int(cursor.u32()))
		if cursor.failed {
			return false
		}
	}
	return bytes.HasPrefix(encoded, []byte("LayerX/programs/402LXP/transfer-set/v2\x00"))
}

func programPrincipalPaymentAccount(principal, asset [32]byte, name []byte) ([32]byte, error) {
	bad := errors.New("invalid Programs payment account binding")
	if len(name) > 512 || !bytes.HasPrefix(name, []byte("agent:")) || bytes.Contains(name, []byte("::")) {
		return [32]byte{}, bad
	}
	for _, b := range name {
		if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || bytes.ContainsRune([]byte("._-:"), rune(b))) {
			return [32]byte{}, bad
		}
	}
	tail := name[6:]
	var did []byte
	if bytes.HasSuffix(tail, []byte(":main")) {
		did = tail[:len(tail)-5]
	} else {
		suffix := []byte(":asset:" + hex.EncodeToString(asset[:]))
		if !bytes.HasSuffix(tail, suffix) {
			return [32]byte{}, bad
		}
		did = tail[:len(tail)-len(suffix)]
	}
	if len(did) == 0 || did[0] == ':' || did[len(did)-1] == ':' {
		return [32]byte{}, bad
	}
	var length [4]byte
	binary.BigEndian.PutUint16(length[:2], uint16(len(did)))
	if domainDigest([]byte("LXP/v1/did-id\x00"), length[:2], did) != principal {
		return [32]byte{}, bad
	}
	binary.BigEndian.PutUint32(length[:], uint32(len(name)))
	return domainDigest([]byte("LX:ACCOUNT:v1"), length[:], name), nil
}

func verifyProgramTransferAuthorization(encoded []byte, expected [32]byte) error {
	boundDomain := []byte("LayerX/programs/402LXP/account-bound-set/v1\x00")
	var names *programTerminalCursor
	if bytes.HasPrefix(encoded, boundDomain) {
		wrapper := programTerminalCursor{value: encoded[len(boundDomain):]}
		encoded = wrapper.take(int(wrapper.u32()))
		if wrapper.failed || bytes.HasPrefix(encoded, boundDomain) {
			return errors.New("invalid Programs account-bound set")
		}
		names = &wrapper
	}
	v1, v2 := []byte("LayerX/programs/402LXP/transfer-set/v1\x00"), []byte("LayerX/programs/402LXP/transfer-set/v2\x00")
	candidate := bytes.HasPrefix(encoded, v2)
	domain := v1
	if candidate {
		domain = v2
	} else if !bytes.HasPrefix(encoded, v1) {
		return errors.New("invalid Programs transfer-set domain")
	}
	cursor := programTerminalCursor{value: encoded[len(domain):]}
	program, principal, invocation := cursor.array32(), cursor.array32(), cursor.array32()
	if program == ([32]byte{}) || principal == ([32]byte{}) || invocation == ([32]byte{}) || !consumeProgramFrame(&cursor) {
		return errors.New("invalid Programs transfer-set authority")
	}
	events := cursor.take(int(cursor.u32()))
	if !validProgramEvents(events) {
		return errors.New("invalid Programs transfer-set events")
	}
	callCount := cursor.u64()
	if callCount > 64 {
		return errors.New("Programs transfer-set call bound")
	}
	for index := uint64(0); index < callCount; index++ {
		caller, callee, callPrincipal := cursor.array32(), cursor.array32(), cursor.array32()
		if caller == ([32]byte{}) || callee == ([32]byte{}) || callPrincipal == ([32]byte{}) || !consumeProgramFrame(&cursor) || !consumeProgramFrame(&cursor) {
			return errors.New("invalid Programs transfer call")
		}
		grants := cursor.take(int(cursor.u32()))
		if !validProgramCapabilities(grants, candidate) {
			return errors.New("invalid Programs transfer grants")
		}
	}
	legCount := cursor.u64()
	if legCount == 0 || legCount > 256 {
		return errors.New("invalid Programs transfer leg count")
	}
	leaves := make([][32]byte, 0, legCount)
	total := Uint128{}
	for index := uint64(0); index < legCount; index++ {
		frameStart := cursor.offset
		if !consumeProgramFrame(&cursor) {
			return errors.New("invalid Programs transfer frame")
		}
		frame := append([]byte(nil), cursor.value[frameStart:cursor.offset]...)
		source := principal
		sourceTag := byte(0)
		var binding decodedProgramAuthority
		if candidate {
			sourceTag = cursor.byte()
			switch sourceTag {
			case 1:
				source = cursor.array32()
				if source != principal {
					return errors.New("Programs principal source mismatch")
				}
			case 2:
				authority := cursor.take(int(cursor.u32()))
				decoded, err := decodeProgramAuthority(authority)
				if err != nil {
					return err
				}
				binding = decoded
				source = decoded.source
			case 3:
				fundingPrincipal := cursor.array32()
				if fundingPrincipal != principal {
					return errors.New("Programs funding principal mismatch")
				}
				encodedBinding := cursor.take(int(cursor.u32()))
				decoded, err := decodeProgramFunding(encodedBinding)
				if err != nil {
					return err
				}
				binding = decoded
				source = fundingPrincipal
			default:
				return errors.New("invalid Programs transfer source")
			}
		}
		asset, to := cursor.array32(), cursor.array32()
		amount := cursor.u128()
		legProgram := cursor.array32()
		if asset == ([32]byte{}) || to == ([32]byte{}) || amount == (Uint128{}) || legProgram == ([32]byte{}) || sourceTag == 2 && (binding.owner != legProgram || !bytes.Equal(binding.frame, frame) || binding.asset != asset || binding.to != to || binding.amount != amount) || sourceTag == 3 && (binding.owner != legProgram || binding.source != to || binding.asset != asset) {
			return errors.New("invalid Programs transfer leg")
		}
		if names != nil {
			name := names.take(int(names.u16()))
			if names.failed {
				return errors.New("truncated Programs payment account")
			}
			if sourceTag == 2 {
				if len(name) != 0 {
					return errors.New("named Programs source")
				}
			} else {
				var err error
				source, err = programPrincipalPaymentAccount(source, asset, name)
				if err != nil {
					return err
				}
			}
		}
		var amountOK bool
		total, amountOK = total.Add(amount)
		if !amountOK {
			return errors.New("Programs transfer total overflow")
		}
		leg := []byte{0}
		leg = append(leg, source[:]...)
		leg = append(leg, to[:]...)
		leg = append(leg, asset[:]...)
		var wide [16]byte
		binary.BigEndian.PutUint64(wide[:8], amount.High())
		binary.BigEndian.PutUint64(wide[8:], amount.Low())
		leg = append(leg, wide[:]...)
		leg = append(leg, 0, 1)
		leaves = append(leaves, domainDigest([]byte("LXP/v1/merkle-leaf\x00"), leg))
	}
	if cursor.failed || !cursor.finished() || names != nil && (names.failed || !names.finished()) {
		return errors.New("trailing Programs transfer authorization")
	}
	for len(leaves) > 1 {
		next := make([][32]byte, 0, (len(leaves)+1)/2)
		for index := 0; index < len(leaves); index += 2 {
			right := leaves[index]
			if index+1 < len(leaves) {
				right = leaves[index+1]
			}
			next = append(next, domainDigest([]byte("LXP/v1/merkle-internal\x00"), leaves[index][:], right[:]))
		}
		leaves = next
	}
	if len(leaves) != 1 || leaves[0] != expected {
		return errors.New("Programs transfer authorization root mismatch")
	}
	return nil
}

func consumeProgramFrame(cursor *programTerminalCursor) bool {
	path := cursor.take(8)
	depth := cursor.byte()
	if cursor.failed || depth > 8 {
		return false
	}
	for index, value := range path {
		if index < int(depth) && value == 0 || index >= int(depth) && value != 0 {
			return false
		}
	}
	return true
}
func validProgramEvents(encoded []byte) bool {
	domain := []byte("LayerX/programs/events/v1\x00")
	if !bytes.HasPrefix(encoded, domain) {
		return false
	}
	cursor := programTerminalCursor{value: encoded[len(domain):]}
	count := cursor.u32()
	if count > 64 {
		return false
	}
	for index := uint32(0); index < count; index++ {
		if cursor.array32() == ([32]byte{}) || cursor.array32() == ([32]byte{}) || !consumeProgramFrame(&cursor) {
			return false
		}
		if len(cursor.take(int(cursor.u32()))) > 64 || len(cursor.take(int(cursor.u32()))) > 65_536 {
			return false
		}
	}
	return !cursor.failed && cursor.finished()
}
func validProgramCapabilities(encoded []byte, v2 bool) bool {
	grants, err := DecodeNativeCapabilities(encoded)
	if err != nil {
		return false
	}
	for _, grant := range grants {
		switch grant.(type) {
		case NativeProgramSpend, NativeBalanceView:
			if !v2 {
				return false
			}
		}
	}
	return true
}
func compareProgramCapabilityKey(left, right [][]byte) int {
	for index := 0; index < len(left) && index < len(right); index++ {
		if compared := bytes.Compare(left[index], right[index]); compared != 0 {
			return compared
		}
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return 0
}

type decodedProgramAuthority struct {
	source, owner, asset, to [32]byte
	amount                   Uint128
	frame                    []byte
}

func decodeProgramAuthority(encoded []byte) (decodedProgramAuthority, error) {
	domain := []byte("LayerX/programs/402LXP/program-authority/v1\x00")
	if !bytes.HasPrefix(encoded, domain) {
		return decodedProgramAuthority{}, errors.New("invalid Programs program authority")
	}
	cursor := programTerminalCursor{value: encoded[len(domain):]}
	owner := cursor.array32()
	seed := cursor.take(int(cursor.u16()))
	source := cursor.array32()
	frameStart := cursor.offset
	if owner == ([32]byte{}) || len(seed) > 128 || source != deriveProgramAccount(owner, seed) || !consumeProgramFrame(&cursor) {
		return decodedProgramAuthority{}, errors.New("invalid Programs program authority")
	}
	frame := append([]byte(nil), cursor.value[frameStart:cursor.offset]...)
	asset, to, amount := cursor.array32(), cursor.array32(), cursor.u128()
	if asset == ([32]byte{}) || to == ([32]byte{}) || amount == (Uint128{}) || cursor.failed || !cursor.finished() {
		return decodedProgramAuthority{}, errors.New("invalid Programs program authority")
	}
	return decodedProgramAuthority{source: source, owner: owner, asset: asset, to: to, amount: amount, frame: frame}, nil
}
func decodeProgramFunding(encoded []byte) (decodedProgramAuthority, error) {
	domain := []byte("LayerX/programs/402LXP/program-funding/v1\x00")
	if !bytes.HasPrefix(encoded, domain) {
		return decodedProgramAuthority{}, errors.New("invalid Programs funding authority")
	}
	cursor := programTerminalCursor{value: encoded[len(domain):]}
	owner := cursor.array32()
	seed := cursor.take(int(cursor.u16()))
	destination := cursor.array32()
	asset := cursor.array32()
	if owner == ([32]byte{}) || len(seed) > 128 || destination != deriveProgramAccount(owner, seed) || asset == ([32]byte{}) || cursor.failed || !cursor.finished() {
		return decodedProgramAuthority{}, errors.New("invalid Programs funding authority")
	}
	return decodedProgramAuthority{source: destination, owner: owner, asset: asset}, nil
}
func deriveProgramAccount(program [32]byte, seed []byte) [32]byte {
	material := []byte("LayerX/programs/program-account/v1\x00")
	material = append(material, program[:]...)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(seed)))
	material = append(material, length[:]...)
	material = append(material, seed...)
	return sha256.Sum256(material)
}

func allProgramZero(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return false
		}
	}
	return true
}

type programTerminalCursor struct {
	value  []byte
	offset int
	failed bool
}

func (cursor *programTerminalCursor) take(length int) []byte {
	if cursor.failed || length < 0 || cursor.offset > len(cursor.value)-length {
		cursor.failed = true
		return nil
	}
	value := cursor.value[cursor.offset : cursor.offset+length]
	cursor.offset += length
	return value
}

func (cursor *programTerminalCursor) byte() byte {
	value := cursor.take(1)
	if len(value) != 1 {
		return 0
	}
	return value[0]
}

func (cursor *programTerminalCursor) u16() uint16 {
	value := cursor.take(2)
	if len(value) != 2 {
		return 0
	}
	return binary.BigEndian.Uint16(value)
}

func (cursor *programTerminalCursor) u32() uint32 {
	value := cursor.take(4)
	if len(value) != 4 {
		return 0
	}
	return binary.BigEndian.Uint32(value)
}

func (cursor *programTerminalCursor) i32() int32 { return int32(cursor.u32()) }

func (cursor *programTerminalCursor) u64() uint64 {
	value := cursor.take(8)
	if len(value) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(value)
}

func (cursor *programTerminalCursor) i64() int64 { return int64(cursor.u64()) }

func (cursor *programTerminalCursor) u128() Uint128 {
	return Uint128{high: cursor.u64(), low: cursor.u64()}
}

func (cursor *programTerminalCursor) array32() [32]byte {
	var value [32]byte
	copy(value[:], cursor.take(32))
	return value
}

func (cursor *programTerminalCursor) sized32() []byte {
	length := cursor.u32()
	if uint64(length) > uint64(maximumProgramTerminalBytes) {
		cursor.failed = true
		return nil
	}
	return cursor.take(int(length))
}

func (cursor *programTerminalCursor) sized64() []byte {
	length := cursor.u64()
	if length > uint64(maximumProgramTerminalBytes) {
		cursor.failed = true
		return nil
	}
	return cursor.take(int(length))
}

func (cursor *programTerminalCursor) rest() []byte {
	if cursor.failed {
		return nil
	}
	value := cursor.value[cursor.offset:]
	cursor.offset = len(cursor.value)
	return value
}

func (cursor *programTerminalCursor) finished() bool {
	return !cursor.failed && cursor.offset == len(cursor.value)
}

const maximumProgramTerminalBytes = MaximumProgramCalldataBytes + 8192

func decodeProgramHex(value string) ([]byte, error) {
	if len(value) > 2*MaximumProgramCalldataBytes || len(value)%2 != 0 || !canonicalLowerHex(value, len(value)/2) {
		return nil, errors.New("program evidence is not bounded canonical hexadecimal")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, errors.New("program evidence is not hexadecimal")
	}
	return decoded, nil
}

func programCallWire(call ProgramCall) map[string]any {
	if call.NativeCall != nil {
		n := call.NativeCall
		resources := make([]string, 7)
		for i, v := range n.Resources {
			resources[i] = strconv.FormatUint(v, 10)
		}
		return map[string]any{"payload_encoding": "native-v1", "program_id": hex.EncodeToString(call.ProgramID[:]), "calldata": hex.EncodeToString(call.Calldata), "budget": map[string]string{"fuel": strconv.FormatUint(call.Budget.Fuel, 10), "fee_limit": call.Budget.FeeLimit.String()}, "signed_activity": hex.EncodeToString(call.SignedActivity), "native_call": map[string]any{"guest_abi": n.GuestABI, "entrypoint": n.Entrypoint, "capabilities_hex": hex.EncodeToString(n.Capabilities), "access_declaration_hex": hex.EncodeToString(n.AccessDeclaration), "response_capacity": n.ResponseCapacity, "resources": resources}}
	}

	return map[string]any{
		"program_id":      hex.EncodeToString(call.ProgramID[:]),
		"calldata":        hex.EncodeToString(call.Calldata),
		"budget":          map[string]any{"fuel": programUint64(call.Budget.Fuel), "fee_limit": call.Budget.FeeLimit.String()},
		"capabilities":    call.Capabilities,
		"signed_activity": hex.EncodeToString(call.SignedActivity),
	}
}

func programUint64(value uint64) string { return strconv.FormatUint(value, 10) }

func canonicalProgramKey(key IdempotencyKey) bool {
	return key.valid() && canonicalLowerHex(key.String(), 32)
}

type Programs struct {
	sender                    [20]byte
	didID                     [32]byte
	receiptKeys               ReceiptKeys
	maxFeePerGas              Uint128
	maxPriorityFeePerGas      Uint128
	protocolVersion           uint16
	client                    *Client
	trustedSequencerPublicKey [32]byte
	clockMilliseconds         func() uint64
	maximumSimulationAge      uint64
	headsMu                   sync.RWMutex
	heads                     map[[32]byte]programHeadObservation
}

type programHeadObservation struct {
	CodeHash      [32]byte
	ABIVersion    uint16
	Version       uint32
	Authenticated bool
	StateRoot     [32]byte
	Sequence      uint64
	ObservedAt    uint64
	ValidThrough  uint64
}

type ProgramTrustOptions struct {
	Sender                           [20]byte
	DIDID                            [32]byte
	ReceiptKeys                      ReceiptKeys
	MaxFeePerGas                     Uint128
	MaxPriorityFeePerGas             Uint128
	ProtocolVersion                  uint16
	ClockMilliseconds                func() uint64
	MaximumSimulationAgeMilliseconds uint64
}

func NewPrograms(client *Client, trustedSequencerPublicKey [32]byte) (*Programs, error) {
	return NewProgramsWithTrustOptions(client, trustedSequencerPublicKey, ProgramTrustOptions{
		ClockMilliseconds:                currentProgramTimeMilliseconds,
		MaximumSimulationAgeMilliseconds: maximumProgramSimulationAge,
	})
}

func NewProgramsWithTrustOptions(client *Client, trustedSequencerPublicKey [32]byte, options ProgramTrustOptions) (*Programs, error) {
	if client == nil || trustedSequencerPublicKey == ([32]byte{}) || options.ClockMilliseconds == nil || options.MaximumSimulationAgeMilliseconds == 0 {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	if options.ProtocolVersion == 0 {
		options.ProtocolVersion = 2
	}
	if options.ProtocolVersion != 2 && options.ProtocolVersion != 3 {
		return nil, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	return &Programs{
		sender: options.Sender, didID: options.DIDID, receiptKeys: options.ReceiptKeys, maxFeePerGas: options.MaxFeePerGas, maxPriorityFeePerGas: options.MaxPriorityFeePerGas,
		protocolVersion: options.ProtocolVersion,
		client:          client, trustedSequencerPublicKey: trustedSequencerPublicKey,
		clockMilliseconds:    options.ClockMilliseconds,
		maximumSimulationAge: options.MaximumSimulationAgeMilliseconds,
		heads:                make(map[[32]byte]programHeadObservation),
	}, nil
}

func (programs *Programs) Discover(ctx context.Context, program [32]byte) (ProgramDiscovery, error) {
	if program == ([32]byte{}) {
		return ProgramDiscovery{}, lifecycleInvalid()
	}
	data, err := programs.view(ctx, layerxProgramsAddress, programABI("b1ffb3d4", program[:]), [20]byte{})
	if err != nil {
		return ProgramDiscovery{}, err
	}
	if len(data) != 192 {
		return ProgramDiscovery{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	policy, e1 := programABIUint(data, 0, 8)
	abi, e2 := programABIUint(data, 96, 16)
	version, e3 := programABIUint(data, 128, 32)
	active, e4 := programABIUint(data, 160, 1)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || policy > 1 || version == 0 || !SupportsProgramGuestAbi(uint16(abi)) {
		return ProgramDiscovery{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	lifecycle := "deprecated"
	if active == 1 {
		lifecycle = "active"
	}
	return ProgramDiscovery{ProgramID: hex.EncodeToString(program[:]), Lifecycle: lifecycle, CodeHash: hex.EncodeToString(data[64:96]), ABIVersion: uint16(abi), Version: uint32(version), Verification: "pending-state-read"}, nil
}

func (programs *Programs) Interface(ctx context.Context, program [32]byte) (ProgramInterface, error) {
	if program == ([32]byte{}) {
		return ProgramInterface{}, lifecycleInvalid()
	}
	data, err := programs.view(ctx, layerxProgramsAddress, programABI("8efc7240", program[:]), [20]byte{})
	if err != nil {
		return ProgramInterface{}, err
	}
	version, err := programABIUint(data, 0, 32)
	if err != nil {
		return ProgramInterface{}, err
	}
	encoding, err := programABIBytes(data, 64, 96)
	if err != nil {
		return ProgramInterface{}, err
	}
	value, digest := hex.EncodeToString(encoding), hex.EncodeToString(data[32:64])
	return ProgramInterface{ProgramID: hex.EncodeToString(program[:]), Version: uint32(version), Interface: &value, InterfaceDigest: &digest, Verification: "pending-state-read"}, nil
}

func (programs *Programs) Simulate(ctx context.Context, call ProgramCall) (ProgramSimulation, error) {
	data, err := call.EVMCalldata(true)
	if err != nil {
		return ProgramSimulation{}, err
	}
	from := call.From
	if from == ([20]byte{}) {
		from = programs.sender
	}
	result, err := programs.view(ctx, layerxProgramsAddress, data, from)
	if err != nil {
		return ProgramSimulation{}, err
	}
	if len(result) < 128 {
		return ProgramSimulation{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	code := int32(binary.BigEndian.Uint32(result[28:32]))
	prefix := byte(0)
	if code < 0 {
		prefix = 255
	}
	if !bytes.Equal(result[:28], bytes.Repeat([]byte{prefix}, 28)) {
		return ProgramSimulation{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	kind, err := programABIUint(result, 32, 8)
	if err != nil {
		return ProgramSimulation{}, err
	}
	response, err := programABIBytes(result, 64, 96)
	if err != nil {
		return ProgramSimulation{}, err
	}
	return ProgramSimulation{ResultCode: code, TerminalKind: uint8(kind), Response: response, Committed: false}, nil
}

func (programs *Programs) Submit(ctx context.Context, call ProgramCall, key IdempotencyKey) (ProgramSubmission, error) {
	data, err := call.EVMCalldata(false)
	if err != nil {
		return ProgramSubmission{}, err
	}
	tx, err := programs.buildTransaction(ctx, call.From, data)
	if err != nil {
		return ProgramSubmission{}, err
	}
	return ProgramSubmission{State: ProgramSubmissionPrepared, Transaction: &tx, IdempotencyKey: key.String()}, nil
}

func (programs *Programs) Receipt(ctx context.Context, key IdempotencyKey, expectedTransaction [32]byte) (ProgramSubmission, error) {
	if !canonicalProgramKey(key) || programs.didID == ([32]byte{}) || expectedTransaction == ([32]byte{}) {
		return ProgramSubmission{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	keyBytes, _ := hex.DecodeString(key.String())
	result, err := programs.view(ctx, layerxAuthAddress, programABI("2eda9f2b", programs.didID[:], keyBytes), [20]byte{})
	if err != nil {
		return ProgramSubmission{}, err
	}
	recorded, err := programABIBytes(result, 0, 32)
	if err != nil {
		return ProgramSubmission{}, err
	}
	submission, err := programs.Activity(ctx, expectedTransaction)
	if err != nil {
		return ProgramSubmission{}, err
	}
	submission.IdempotencyKey, submission.IdempotencyResult = key.String(), recorded
	return submission, nil
}

func (programs *Programs) Activity(ctx context.Context, transaction [32]byte) (ProgramSubmission, error) {
	if transaction == ([32]byte{}) {
		return ProgramSubmission{}, newSDKError(ErrorInvalidArgument, RetryNever)
	}
	hash := "0x" + hex.EncodeToString(transaction[:])
	var tx *struct {
		Hash string `json:"hash"`
	}
	if err := programs.rpc(ctx, "eth_getTransactionByHash", []any{hash}, &tx); err != nil {
		return ProgramSubmission{}, err
	}
	if tx == nil {
		return ProgramSubmission{State: ProgramSubmissionUnknown, TransactionHash: transaction}, nil
	}
	if tx.Hash != hash {
		return ProgramSubmission{}, verificationFailure()
	}
	var receipt json.RawMessage
	if err := programs.rpc(ctx, "pax_sequencerReceipt", []any{hash}, &receipt); err != nil {
		return ProgramSubmission{}, err
	}
	var view struct {
		Rung string `json:"rung"`
	}
	if json.Unmarshal(receipt, &view) != nil {
		return ProgramSubmission{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	if view.Rung == "submitted" {
		return ProgramSubmission{State: ProgramSubmissionPending, TransactionHash: transaction}, nil
	}
	var raw string
	if err := programs.rpc(ctx, "eth_getRawTransactionByHash", []any{hash}, &raw); err != nil {
		return ProgramSubmission{}, err
	}
	rawBytes, ok := receiptHex(raw)
	if !ok || receiptKeccak256(rawBytes) != transaction {
		return ProgramSubmission{}, verificationFailure()
	}
	verified, err := VerifySequencerReceipt(receipt, rawBytes, programs.receiptKeys)
	if err != nil {
		return ProgramSubmission{}, err
	}
	state := ProgramSubmissionExecuted
	if verified.Status == 0 {
		state = ProgramSubmissionRefused
	}
	return ProgramSubmission{State: state, TransactionHash: transaction, SequencerReceipt: &verified}, nil
}

func (programs *Programs) raw(ctx context.Context, operation string, requiresKey bool, request any, options CallOptions) (json.RawMessage, error) {
	var out json.RawMessage
	err := programs.client.call(ctx, PlaneAgent, operation, requiresKey, request, &out, options)
	return out, err
}

func (programs *Programs) rememberHead(program [32]byte, head programHeadObservation) error {
	programs.headsMu.Lock()
	defer programs.headsMu.Unlock()
	if current, found := programs.heads[program]; found &&
		(current.Authenticated && !head.Authenticated || head.Sequence < current.Sequence || head.ObservedAt < current.ObservedAt ||
			head.Sequence == current.Sequence && (head.StateRoot != current.StateRoot || head.ValidThrough < current.ValidThrough || head.CodeHash != current.CodeHash || head.ABIVersion != current.ABIVersion || head.Version != current.Version)) {
		return errors.New("Programs head rollback or conflict")
	}
	programs.heads[program] = head
	return nil
}

func programCallHeadAdmitted(call ProgramCall, head programHeadObservation) bool {
	if call.NativeCall == nil || call.NativeCall.GuestABI == ProgramAbiV1 || call.NativeCall.GuestABI == ProgramAbiV2 {
		return true
	}
	return SupportsProgramGuestAbi(call.NativeCall.GuestABI) && head.Authenticated && head.ABIVersion == call.NativeCall.GuestABI
}

func (programs *Programs) rememberedHead(program [32]byte) (programHeadObservation, bool) {
	programs.headsMu.RLock()
	defer programs.headsMu.RUnlock()
	head, found := programs.heads[program]
	return head, found
}

func (programs *Programs) headIsCurrent(program [32]byte, expected programHeadObservation) bool {
	programs.headsMu.RLock()
	defer programs.headsMu.RUnlock()
	current, found := programs.heads[program]
	return found && current == expected
}

func decodeProgramSimulation(raw json.RawMessage, expectedProgram [32]byte, binding programCallBinding,
	prior programHeadObservation, trustedSequencerPublicKey [32]byte, now uint64, maximumAge uint64, selectedProtocol ...uint16) (ProgramSimulation, error) {
	var document struct {
		Committed          bool                      `json:"committed"`
		Execution          ProgramExecutionDocument  `json:"execution"`
		SimulationEvidence ProgramSimulationEvidence `json:"simulation_evidence"`
	}
	if decodeStrict(raw, &document) != nil || document.Committed || document.Execution.State != "simulated" || document.Execution.IdempotencyKey != "" {
		return ProgramSimulation{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	programID, err := programHex32(document.Execution.ProgramID)
	activityID, activityError := programHex32(document.Execution.ActivityID)
	if err != nil || activityError != nil || programID != expectedProgram || activityID != binding.ActivityID {
		return ProgramSimulation{}, newSDKError(ErrorVerificationFailure, RetryNever)
	}
	verified, verifyError := verifyExecutionAuthority(document.Execution, trustedSequencerPublicKey, selectedProtocol...)
	if verifyError != nil || verifySimulationEvidence(document.Execution, document.SimulationEvidence, binding,
		prior, trustedSequencerPublicKey, now, maximumAge) != nil {
		return ProgramSimulation{}, newSDKError(ErrorVerificationFailure, RetryNever)
	}
	return ProgramSimulation{Committed: false, Execution: document.Execution, SimulationEvidence: document.SimulationEvidence, Verification: verified}, nil
}

func decodeProgramSubmission(raw json.RawMessage, expectedProgram *[32]byte, expectedActivity *[32]byte, expectedKey string, expectedSigned []byte, trustedSequencerPublicKey [32]byte, selectedProtocol ...uint16) (ProgramSubmission, error) {
	fields, err := exactObject(raw)
	if err != nil {
		return ProgramSubmission{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	var state ProgramSubmissionState
	if json.Unmarshal(fields["state"], &state) != nil {
		return ProgramSubmission{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	if state == ProgramSubmissionUnknown || state == "pending" {
		if !(exactFields(fields, "state", "activity_id", "idempotency_key") || exactFields(fields, "state", "activity_id", "idempotency_key", "retained_signed_activity")) {
			return ProgramSubmission{}, newSDKError(ErrorDecodeFailure, RetryNever)
		}
		var activityText, key string
		if json.Unmarshal(fields["activity_id"], &activityText) != nil || json.Unmarshal(fields["idempotency_key"], &key) != nil || !canonicalLowerHex(key, 32) || expectedKey != "" && key != expectedKey {
			return ProgramSubmission{}, newSDKError(ErrorDecodeFailure, RetryNever)
		}
		activity, activityError := programHex32(activityText)
		var retained []byte
		var retainedError error
		if retainedRaw := fields["retained_signed_activity"]; len(retainedRaw) != 0 {
			var retainedText string
			if json.Unmarshal(retainedRaw, &retainedText) != nil {
				return ProgramSubmission{}, newSDKError(ErrorDecodeFailure, RetryNever)
			}
			retained, retainedError = decodeProgramHex(retainedText)
			if retainedError == nil && len(retained) == 0 {
				retainedError = errors.New("empty retained signed activity")
			}
		}
		if activityError != nil || retainedError != nil || expectedActivity != nil && activity != *expectedActivity || expectedSigned != nil && !bytes.Equal(retained, expectedSigned) {
			return ProgramSubmission{}, newSDKError(ErrorVerificationFailure, RetryNever)
		}
		return ProgramSubmission{State: ProgramSubmissionUnknown, ActivityID: activity, IdempotencyKey: key, RetainedSignedActivity: retained}, nil
	}
	if state != ProgramSubmissionExecuted && state != ProgramSubmissionRefused {
		return ProgramSubmission{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	var execution ProgramExecutionDocument
	if decodeStrict(raw, &execution) != nil || execution.State != string(state) || execution.IdempotencyKey == "" || !canonicalLowerHex(execution.IdempotencyKey, 32) || expectedKey != "" && execution.IdempotencyKey != expectedKey {
		return ProgramSubmission{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	activity, activityError := programHex32(execution.ActivityID)
	programID, programError := programHex32(execution.ProgramID)
	if activityError != nil || programError != nil || expectedActivity != nil && activity != *expectedActivity || expectedProgram != nil && programID != *expectedProgram {
		return ProgramSubmission{}, newSDKError(ErrorVerificationFailure, RetryNever)
	}
	if state == ProgramSubmissionRefused && execution.Outcome.Kind != "refused" || state == ProgramSubmissionExecuted && execution.Outcome.Kind != "completed" && execution.Outcome.Kind != "legacy_completed" {
		return ProgramSubmission{}, newSDKError(ErrorVerificationFailure, RetryNever)
	}
	verified, verifyError := verifyExecutionAuthority(execution, trustedSequencerPublicKey, selectedProtocol...)
	if verifyError != nil {
		return ProgramSubmission{}, newSDKError(ErrorVerificationFailure, RetryNever)
	}
	return ProgramSubmission{State: state, ActivityID: activity, IdempotencyKey: execution.IdempotencyKey, Execution: &execution, Verification: &verified}, nil
}

func verifyExecutionAuthority(execution ProgramExecutionDocument, trustedSequencerPublicKey [32]byte, selectedProtocol ...uint16) (VerifiedProgramReceipt, error) {
	authorityBatch, authorityBatchError := programHex32(execution.Authority.BatchID)
	_, authorityAssetError := programHex32(execution.Authority.Asset)
	_, authorityPreviousError := programHex32(execution.Authority.PreviousStateRoot)
	authorityResulting, authorityResultingError := programHex32(execution.Authority.ResultingStateRoot)
	authorityPublicKey, authorityKeyError := programHex32(execution.Authority.SequencerPublicKey)
	if authorityBatchError != nil || authorityAssetError != nil || authorityPreviousError != nil || authorityResultingError != nil || authorityKeyError != nil {
		return VerifiedProgramReceipt{}, errors.New("invalid response authority")
	}
	if trustedSequencerPublicKey == ([32]byte{}) || authorityPublicKey != trustedSequencerPublicKey {
		return VerifiedProgramReceipt{}, errors.New("Programs sequencer authority does not match the pinned key")
	}
	stateRoot, rootError := programHex32(execution.StateRoot)
	batchID, batchError := programHex32(execution.BatchID)
	if rootError != nil || batchError != nil || stateRoot != authorityResulting || batchID != authorityBatch || execution.Verification != "receipt-terminal-and-call-graph-verified" || !canonicalUnsigned(execution.GlobalSequence, 64) || !validProgramUsage(execution.Usage) {
		return VerifiedProgramReceipt{}, errors.New("invalid Programs execution authority")
	}
	verified, verifyError := VerifyProgramReceipt(execution, selectedProtocol...)
	if verifyError != nil {
		return VerifiedProgramReceipt{}, verifyError
	}
	receipt := verified.Verification.Receipt
	receiptOutcome := receipt.ProgramOutcome
	sequence, sequenceError := strconv.ParseUint(execution.GlobalSequence, 10, 64)
	feeUnits, feeError := ParseUint128(execution.Usage.FeeUnits)
	if receiptOutcome == nil || sequenceError != nil || receipt.GlobalSequence != sequence || feeError != nil || receipt.ResultCode != execution.ResultCode || receiptOutcome.CPUFuel != mustProgramUint64(execution.Usage.CPUFuel) || receiptOutcome.MemoryBytes != mustProgramUint64(execution.Usage.MemoryBytes) || receiptOutcome.StorageReadBytes != mustProgramUint64(execution.Usage.StorageReadBytes) || receiptOutcome.StorageWriteBytes != mustProgramUint64(execution.Usage.StorageWriteBytes) || receiptOutcome.OutputValues != execution.Usage.OutputValues || receiptOutcome.OutputBytes != mustProgramUint64(execution.Usage.OutputBytes) || !receiptOutcome.FeeUnits.Equal(feeUnits) || execution.Outcome.Kind == "completed" && (receiptOutcome.TerminalKind != 1 || execution.Outcome.Code == nil || *execution.Outcome.Code != receiptOutcome.ResultCode) || execution.Outcome.Kind == "legacy_completed" && (receiptOutcome.TerminalKind != 1 || execution.Outcome.Code == nil || *execution.Outcome.Code != receiptOutcome.ResultCode) || execution.Outcome.Kind == "refused" && receiptOutcome.TerminalKind != 2 && receiptOutcome.TerminalKind != 3 {
		return VerifiedProgramReceipt{}, errors.New("Programs outcome is not receipt-bound")
	}
	return verified, nil
}

func verifySimulationEvidence(execution ProgramExecutionDocument, evidence ProgramSimulationEvidence,
	binding programCallBinding, prior programHeadObservation, trustedSequencerPublicKey [32]byte,
	now uint64, maximumAge uint64) error {
	if evidence.Committed || !canonicalUnsigned(evidence.ObservedSequence, 64) || !canonicalUnsigned(evidence.ObservedAt, 64) {
		return errors.New("invalid simulation evidence")
	}
	boundary, boundaryError := programHex32(evidence.BoundaryID)
	activity, activityError := programHex32(evidence.ActivityID)
	previous, previousError := programHex32(evidence.PreviousStateRoot)
	hypothetical, hypotheticalError := programHex32(evidence.HypotheticalStateRoot)
	publicKey, publicError := programHex32(evidence.PublicKey)
	signature, signatureError := decodeProgramHex(evidence.Signature)
	executionActivity, executionActivityError := programHex32(execution.ActivityID)
	executionRoot, executionRootError := programHex32(execution.StateRoot)
	if boundaryError != nil || activityError != nil || previousError != nil || hypotheticalError != nil || publicError != nil || signatureError != nil || len(signature) != ed25519.SignatureSize || executionActivityError != nil || executionRootError != nil || activity != executionActivity || activity != binding.ActivityID || hypothetical != executionRoot {
		return errors.New("invalid simulation evidence")
	}
	_, authorityBatchError := programHex32(execution.Authority.BatchID)
	_, authorityAssetError := programHex32(execution.Authority.Asset)
	authorityPrevious, authorityPreviousError := programHex32(execution.Authority.PreviousStateRoot)
	authorityResulting, authorityResultingError := programHex32(execution.Authority.ResultingStateRoot)
	authorityPublicKey, authorityKeyError := programHex32(execution.Authority.SequencerPublicKey)
	sequence, sequenceError := strconv.ParseUint(evidence.ObservedSequence, 10, 64)
	if authorityBatchError != nil || authorityAssetError != nil || authorityPreviousError != nil || authorityResultingError != nil || authorityKeyError != nil || sequenceError != nil || sequence == math.MaxUint64 || trustedSequencerPublicKey == ([32]byte{}) || authorityPublicKey != trustedSequencerPublicKey || publicKey != trustedSequencerPublicKey || execution.Verification != "receipt-terminal-and-call-graph-verified" || authorityPrevious != previous || previous != prior.StateRoot || authorityResulting != hypothetical || sequence != prior.Sequence || execution.GlobalSequence != strconv.FormatUint(sequence+1, 10) {
		return errors.New("simulation authority mismatch")
	}
	expectedBoundary := sha256.Sum256(append([]byte("LayerX/emulator/simulation-boundary/v1\x00"), publicKey[:]...))
	if boundary != expectedBoundary {
		return errors.New("simulation boundary mismatch")
	}
	observedAt, _ := strconv.ParseUint(evidence.ObservedAt, 10, 64)
	if !validProgramSimulationTime(observedAt, binding, prior, now, maximumAge) {
		return errors.New("stale simulation evidence")
	}
	signed := make([]byte, 0, 32*4+8*2+64)
	signed = append(signed, []byte("LayerX/agent/program-simulation-evidence/v1\x00")...)
	signed = append(signed, boundary[:]...)
	signed = append(signed, activity[:]...)
	signed = append(signed, previous[:]...)
	signed = append(signed, hypothetical[:]...)
	var integer [8]byte
	binary.BigEndian.PutUint64(integer[:], sequence)
	signed = append(signed, integer[:]...)
	binary.BigEndian.PutUint64(integer[:], observedAt)
	signed = append(signed, integer[:]...)
	signed = append(signed, 0)
	digest := sha256.Sum256(signed)
	if !ed25519.Verify(publicKey[:], digest[:], signature) {
		return errors.New("simulation signature mismatch")
	}
	return nil
}

func validProgramSimulationTime(observedAt uint64, binding programCallBinding,
	prior programHeadObservation, now uint64, maximumAge uint64) bool {
	return maximumAge != 0 && binding.NotAfter >= binding.NotBefore &&
		observedAt >= binding.NotBefore && observedAt <= binding.NotAfter &&
		observedAt <= now && now-observedAt <= maximumAge &&
		prior.ObservedAt <= now && now <= prior.ValidThrough &&
		observedAt >= prior.ObservedAt && observedAt <= prior.ValidThrough
}

func validProgramHeadTime(head programHeadObservation, now uint64) bool {
	return head.ObservedAt <= now && now <= head.ValidThrough && head.ObservedAt <= head.ValidThrough
}

func decodeProgramHead(stateRoot string, sequence string, observedAt string, validThrough string) (programHeadObservation, error) {
	root, rootError := programHex32(stateRoot)
	parsedSequence, sequenceError := strconv.ParseUint(sequence, 10, 64)
	parsedObservedAt, observedError := strconv.ParseUint(observedAt, 10, 64)
	parsedValidThrough, validError := strconv.ParseUint(validThrough, 10, 64)
	if rootError != nil || sequenceError != nil || observedError != nil || validError != nil || parsedValidThrough < parsedObservedAt {
		return programHeadObservation{}, errors.New("invalid Programs head")
	}
	return programHeadObservation{StateRoot: root, Sequence: parsedSequence,
		ObservedAt: parsedObservedAt, ValidThrough: parsedValidThrough}, nil
}

func currentProgramTimeMilliseconds() uint64 {
	now := time.Now().UnixMilli()
	if now < 0 {
		return 0
	}
	return uint64(now)
}

func verifyProgramDiscoveryProof(value ProgramDiscovery, trustedSequencerPublicKey [32]byte) (bool, error) {
	if value.DeploymentReceiptDigest != nil && !canonicalLowerHex(*value.DeploymentReceiptDigest, 32) {
		return false, errors.New("invalid deployment receipt digest")
	}
	if value.DiscoveryPublicKey == nil && value.DiscoverySignature == nil {
		if value.ABIVersion == ProgramAbiV3 || value.ABIVersion == ProgramAbiV4 || value.ABIVersion == ProgramAbiV5 {
			return false, errors.New("signed discovery required for native ABI")
		}
		return false, nil
	}
	if value.DiscoveryPublicKey == nil || value.DiscoverySignature == nil || trustedSequencerPublicKey == ([32]byte{}) || !canonicalLowerHex(*value.DiscoverySignature, ed25519.SignatureSize) {
		return false, errors.New("invalid Programs discovery authority")
	}
	publicKey, keyError := programHex32(*value.DiscoveryPublicKey)
	program, programError := programHex32(value.ProgramID)
	code, codeError := programHex32(value.CodeHash)
	root, rootError := programHex32(value.StateRoot)
	receiptDigest, receiptError := programHex32(value.ReceiptDigest)
	sequence, sequenceError := strconv.ParseUint(value.ObservedSequence, 10, 64)
	observed, observedError := strconv.ParseUint(value.ObservedAt, 10, 64)
	validThrough, validError := strconv.ParseUint(value.ValidThrough, 10, 64)
	if keyError != nil || publicKey != trustedSequencerPublicKey || programError != nil || codeError != nil || rootError != nil || receiptError != nil || sequenceError != nil || observedError != nil || validError != nil || !canonicalUnsigned(value.ObservedSequence, 64) || !canonicalUnsigned(value.ObservedAt, 64) || !canonicalUnsigned(value.ValidThrough, 64) || value.Version == 0 || !SupportsProgramGuestAbi(value.ABIVersion) {
		return false, errors.New("invalid Programs discovery binding")
	}
	material := append([]byte("LayerX/program-discovery-proof/v1\x00"), program[:]...)
	material = append(material, 1)
	material = binary.BigEndian.AppendUint32(material, value.Version)
	material = append(material, code[:]...)
	material = binary.BigEndian.AppendUint16(material, value.ABIVersion)
	material = binary.BigEndian.AppendUint64(material, sequence)
	material = binary.BigEndian.AppendUint64(material, observed)
	material = binary.BigEndian.AppendUint64(material, validThrough)
	material = append(material, root[:]...)
	digest := sha256.Sum256(material)
	signature, signatureError := hex.DecodeString(*value.DiscoverySignature)
	if signatureError != nil || digest != receiptDigest || !ed25519.Verify(ed25519.PublicKey(publicKey[:]), digest[:], signature) {
		return false, errors.New("Programs discovery signature mismatch")
	}
	return true, nil
}

func validDiscovery(value ProgramDiscovery, expectedID string, now uint64) bool {
	return value.ProgramID == expectedID && canonicalLowerHex(value.CodeHash, 32) && canonicalLowerHex(value.ReceiptDigest, 32) && canonicalLowerHex(value.StateRoot, 32) && value.Version != 0 && SupportsProgramGuestAbi(value.ABIVersion) && (value.Lifecycle == "active" || value.Lifecycle == "deprecated" || value.Lifecycle == "tombstoned") && canonicalUnsigned(value.ObservedSequence, 64) && canonicalUnsigned(value.ObservedAt, 64) && canonicalUnsigned(value.ValidThrough, 64) && decimalAtLeast(value.ValidThrough, value.ObservedAt) && decimalAtLeast(strconv.FormatUint(now, 10), value.ObservedAt) && decimalAtLeast(value.ValidThrough, strconv.FormatUint(now, 10)) && value.Verification == "registry-receipt-and-current-head-verified"
}

func validProgramInterface(value ProgramInterface, expectedID string, now uint64) bool {
	if value.ProgramID != expectedID || !canonicalLowerHex(value.CodeHash, 32) || !canonicalLowerHex(value.ReceiptDigest, 32) || !canonicalLowerHex(value.StateRoot, 32) || value.Version == 0 || !SupportsProgramGuestAbi(value.ABIVersion) || !canonicalUnsigned(value.ObservedSequence, 64) || !canonicalUnsigned(value.ObservedAt, 64) || !canonicalUnsigned(value.ValidThrough, 64) || !decimalAtLeast(value.ValidThrough, value.ObservedAt) || !decimalAtLeast(strconv.FormatUint(now, 10), value.ObservedAt) || !decimalAtLeast(value.ValidThrough, strconv.FormatUint(now, 10)) || value.Verification != "deployment-interface-and-current-head-verified" || value.Source.Status == "" {
		return false
	}
	if value.Interface == nil || value.InterfaceDigest == nil {
		return false
	}
	decoded, err := decodeProgramHex(*value.Interface)
	digest, digestError := programHex32(*value.InterfaceDigest)
	return err == nil && len(decoded) != 0 && len(decoded) <= 952 && digestError == nil && sha256.Sum256(decoded) == digest && programInterfaceIdentityMatches(decoded, value.CodeHash, value.ABIVersion)
}

func programInterfaceIdentityMatches(encoded []byte, codeHash string, abi uint16) bool {
	domain := []byte("LayerX/program-interface/v1\x00")
	if len(encoded) <= len(domain)+36 {
		return false
	}
	prefix := string(encoded[:len(domain)])
	switch abi {
	case ProgramAbiV1:
		if prefix != string(domain) {
			return false
		}
	case ProgramAbiV2:
		if prefix != string(domain) && prefix != "LayerX/program-interface/v2\x00" {
			return false
		}
	case ProgramAbiV3:
		if prefix != "LayerX/program-interface/v3\x00" {
			return false
		}
	case ProgramAbiV4:
		if prefix != "LayerX/program-interface/v4\x00" {
			return false
		}
	default:
		return false
	}
	code, err := programHex32(codeHash)
	count := binary.BigEndian.Uint16(encoded[len(domain)+34:])
	return err == nil && bytes.Equal(encoded[len(domain):len(domain)+32], code[:]) && binary.BigEndian.Uint16(encoded[len(domain)+32:]) == abi && count > 0 && count <= 256
}

func validProgramUsage(usage ProgramUsage) bool {
	return canonicalUnsigned(usage.CPUFuel, 64) && canonicalUnsigned(usage.MemoryBytes, 64) && canonicalUnsigned(usage.StorageReadBytes, 64) && canonicalUnsigned(usage.StorageWriteBytes, 64) && canonicalUnsigned(usage.OutputBytes, 64) && canonicalUnsigned(usage.FeeUnits, 128)
}

func canonicalUnsigned(value string, bits int) bool {
	if value == "" || value != "0" && value[0] == '0' {
		return false
	}
	integer, ok := new(big.Int).SetString(value, 10)
	return ok && integer.Sign() >= 0 && integer.BitLen() <= bits
}

func decimalAtLeast(value string, minimum string) bool {
	parsed, parsedOK := new(big.Int).SetString(value, 10)
	lower, lowerOK := new(big.Int).SetString(minimum, 10)
	return parsedOK && lowerOK && parsed.Cmp(lower) >= 0
}

func mustProgramUint64(value string) uint64 {
	parsed, _ := strconv.ParseUint(value, 10, 64)
	return parsed
}

func validLegacyValue(value ProgramLegacyValue) bool {
	switch value.Type {
	case "i32":
		number, ok := value.Value.(float64)
		return ok && math.Trunc(number) == number && number >= math.MinInt32 && number <= math.MaxInt32
	case "i64":
		text, ok := value.Value.(string)
		if !ok || text == "" || text == "-0" || len(text) > 20 || text[0] == '0' && len(text) > 1 || text[0] == '-' && (len(text) == 1 || text[1] == '0' && len(text) > 2) {
			return false
		}
		_, err := strconv.ParseInt(text, 10, 64)
		return err == nil
	default:
		return false
	}
}

func decodeProgramFailure(raw json.RawMessage) (ProgramFailure, error) {
	fields, err := exactObject(raw)
	if err != nil {
		return ProgramFailure{}, err
	}
	var kind string
	if json.Unmarshal(fields["kind"], &kind) != nil {
		return ProgramFailure{}, errors.New("invalid Programs failure")
	}
	result := ProgramFailure{Kind: kind}
	switch kind {
	case "unknown_program", "reentrancy", "authority", "resource", "response", "fault":
		if !exactFields(fields, "kind") {
			return ProgramFailure{}, errors.New("invalid Programs failure")
		}
	case "depth_exceeded", "fanout_exceeded":
		if !exactFields(fields, "kind", "limit", "attempted") || json.Unmarshal(fields["limit"], &result.Limit) != nil || json.Unmarshal(fields["attempted"], &result.Attempted) != nil || result.Limit == nil || result.Attempted == nil {
			return ProgramFailure{}, errors.New("invalid Programs failure")
		}
	case "guest_refused":
		if !exactFields(fields, "kind", "code") || json.Unmarshal(fields["code"], &result.Code) != nil || result.Code == nil {
			return ProgramFailure{}, errors.New("invalid Programs failure")
		}
	default:
		return ProgramFailure{}, errors.New("unknown Programs failure")
	}
	return result, nil
}

func exactObject(encoded []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if decodeStrict(encoded, &fields) != nil || fields == nil {
		return nil, errors.New("invalid Programs object")
	}
	return fields, nil
}

func exactFields(fields map[string]json.RawMessage, required ...string) bool {
	if len(fields) != len(required) {
		return false
	}
	for _, name := range required {
		if len(fields[name]) == 0 {
			return false
		}
	}
	return true
}

func decodeStrict(encoded []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

func programHex32(value string) ([32]byte, error) {
	if !canonicalLowerHex(value, 32) {
		return [32]byte{}, errors.New("invalid bytes32")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return [32]byte{}, err
	}
	return [32]byte(decoded), nil
}

func programHex32Raw(value json.RawMessage) ([32]byte, error) {
	var text string
	if json.Unmarshal(value, &text) != nil {
		return [32]byte{}, errors.New("invalid bytes32")
	}
	return programHex32(text)
}

func PlatformSDKPrograms() string {
	return "server-attested-registry-and-locally-verified-program-execution-v1"
}

func unwrapAppliedProgramTerminal(encoded []byte, outcome ProgramReceiptOutcome) ([]byte, error) {
	if outcome.EncodingVersion != 4 {
		return encoded, nil
	}
	domain := []byte("LXP/programs/terminal-applied-legs/v1\x00")
	cursor := programTerminalCursor{value: encoded}
	if !bytes.Equal(cursor.take(len(domain)), domain) {
		return nil, errors.New("Programs applied terminal domain")
	}
	inner, legs := cursor.sized32(), cursor.sized32()
	if cursor.failed || !cursor.finished() || len(inner) == 0 || len(inner) > 1_048_576 || len(legs) > 256*115 || len(legs)%115 != 0 || sha256.Sum256(legs) != outcome.AppliedLegsDigest {
		return nil, errors.New("Programs applied legs bounds or digest")
	}
	leaves := make([][32]byte, 0, len(legs)/115)
	for offset := 0; offset < len(legs); offset += 115 {
		leg := legs[offset : offset+115]
		if leg[0] != 0 || leg[113] != 0 || leg[114] != 1 || allProgramZero(leg[1:33]) || allProgramZero(leg[33:65]) || allProgramZero(leg[65:97]) || allProgramZero(leg[97:113]) {
			return nil, errors.New("Programs applied leg canonical fields")
		}
		leaves = append(leaves, domainDigest([]byte("LXP/v1/merkle-leaf\x00"), leg))
	}
	for len(leaves) > 1 {
		next := make([][32]byte, 0, (len(leaves)+1)/2)
		for index := 0; index < len(leaves); index += 2 {
			right := leaves[index]
			if index+1 < len(leaves) {
				right = leaves[index+1]
			}
			next = append(next, domainDigest([]byte("LXP/v1/merkle-internal\x00"), leaves[index][:], right[:]))
		}
		leaves = next
	}
	root := [32]byte{}
	if len(leaves) != 0 {
		root = leaves[0]
	}
	if root != outcome.TransferRoot {
		return nil, errors.New("Programs applied transfer root mismatch")
	}
	return inner, nil
}

const layerxProgramsAddress = "0x000000000000000000000000000000000000101d"
const layerxAuthAddress = "0x000000000000000000000000000000000000101c"

type ProgramTransaction struct {
	Type                 string `json:"type"`
	ChainID              string `json:"chainId"`
	From                 string `json:"from"`
	To                   string `json:"to"`
	Nonce                string `json:"nonce"`
	Gas                  string `json:"gas"`
	MaxFeePerGas         string `json:"maxFeePerGas"`
	MaxPriorityFeePerGas string `json:"maxPriorityFeePerGas"`
	Value                string `json:"value"`
	Data                 string `json:"data"`
	AccessList           []any  `json:"accessList"`
}

type ProgramRPCTransport struct {
	endpoint string
	client   *http.Client
}

func NewProgramRPCTransport(endpoint string, client *http.Client) (*ProgramRPCTransport, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Fragment != "" {
		return nil, lifecycleInvalid()
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &ProgramRPCTransport{endpoint: endpoint, client: client}, nil
}

func (transport *ProgramRPCTransport) Call(ctx context.Context, call TransportCall) (json.RawMessage, error) {
	switch call.Operation {
	case "eth_call", "eth_getTransactionCount", "eth_estimateGas", "eth_gasPrice", "eth_maxPriorityFeePerGas", "eth_getTransactionByHash", "eth_getRawTransactionByHash", "pax_sequencerReceipt":
	default:
		return nil, lifecycleInvalid()
	}
	body, err := json.Marshal(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      uint64          `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}{"2.0", 1, call.Operation, call.Request})
	if err != nil {
		return nil, lifecycleInvalid()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, transport.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, lifecycleInvalid()
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := transport.client.Do(request)
	if err != nil {
		return nil, transportError(ctx, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, newSDKError(ErrorTransportFailure, RetryNever)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 8*MaximumProgramCalldataBytes+1))
	if err != nil || len(payload) > 8*MaximumProgramCalldataBytes {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	var envelope struct {
		JSONRPC string           `json:"jsonrpc"`
		ID      uint64           `json:"id"`
		Result  json.RawMessage  `json:"result"`
		Error   *ProgramRPCError `json:"error"`
	}
	if json.Unmarshal(payload, &envelope) != nil || envelope.JSONRPC != "2.0" || envelope.ID != 1 || (envelope.Error != nil) == (len(envelope.Result) != 0) {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	if envelope.Error != nil {
		return nil, envelope.Error
	}
	return envelope.Result, nil
}

type ProgramRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (err *ProgramRPCError) Error() string { return err.Message }

func (programs *Programs) rpc(ctx context.Context, method string, params []any, out any) error {
	if programs == nil || programs.client == nil || programs.client.transport == nil || ctx == nil {
		return lifecycleInvalid()
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return lifecycleInvalid()
	}
	raw, err := programs.client.transport.Call(ctx, TransportCall{Plane: PlaneAgent, Operation: method, Request: encoded})
	if err != nil {
		return err
	}
	if json.Unmarshal(raw, out) != nil {
		return newSDKError(ErrorDecodeFailure, RetryNever)
	}
	return nil
}

func (programs *Programs) view(ctx context.Context, to string, data []byte, from [20]byte) ([]byte, error) {
	tx := map[string]string{"to": to, "data": "0x" + hex.EncodeToString(data)}
	if from != ([20]byte{}) {
		tx["from"] = "0x" + hex.EncodeToString(from[:])
	}
	var result string
	if err := programs.rpc(ctx, "eth_call", []any{tx, "pending"}, &result); err != nil {
		return nil, err
	}
	decoded, ok := receiptHex(result)
	if !ok || len(decoded) > MaximumProgramCalldataBytes+8192 {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	return decoded, nil
}

func (programs *Programs) buildTransaction(ctx context.Context, from [20]byte, data []byte) (ProgramTransaction, error) {
	if from == ([20]byte{}) {
		from = programs.sender
	}
	if from == ([20]byte{}) {
		return ProgramTransaction{}, lifecycleInvalid()
	}
	tx := ProgramTransaction{Type: "0x2", ChainID: "0x7d", From: "0x" + hex.EncodeToString(from[:]), To: layerxProgramsAddress, Value: "0x0", Data: "0x" + hex.EncodeToString(data), AccessList: []any{}}
	if err := programs.rpc(ctx, "eth_getTransactionCount", []any{tx.From, "pending"}, &tx.Nonce); err != nil {
		return ProgramTransaction{}, err
	}
	if !programQuantity(tx.Nonce, 64) {
		return ProgramTransaction{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	fee, tip := programs.maxFeePerGas, programs.maxPriorityFeePerGas
	if fee == (Uint128{}) {
		if err := programs.rpc(ctx, "eth_gasPrice", []any{}, &tx.MaxFeePerGas); err != nil {
			return ProgramTransaction{}, err
		}
	} else {
		tx.MaxFeePerGas = programQuantity128(fee)
	}
	if tip == (Uint128{}) {
		if err := programs.rpc(ctx, "eth_maxPriorityFeePerGas", []any{}, &tx.MaxPriorityFeePerGas); err != nil {
			return ProgramTransaction{}, err
		}
	} else {
		tx.MaxPriorityFeePerGas = programQuantity128(tip)
	}
	if !programQuantity(tx.MaxFeePerGas, 128) || !programQuantity(tx.MaxPriorityFeePerGas, 128) {
		return ProgramTransaction{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	maximum, _ := new(big.Int).SetString(tx.MaxFeePerGas[2:], 16)
	priority, _ := new(big.Int).SetString(tx.MaxPriorityFeePerGas[2:], 16)
	if maximum.Sign() == 0 || priority.Cmp(maximum) > 0 {
		return ProgramTransaction{}, lifecycleInvalid()
	}
	estimate := map[string]any{"type": tx.Type, "chainId": tx.ChainID, "from": tx.From, "to": tx.To, "nonce": tx.Nonce, "value": tx.Value, "data": tx.Data, "maxFeePerGas": tx.MaxFeePerGas, "maxPriorityFeePerGas": tx.MaxPriorityFeePerGas, "accessList": tx.AccessList}
	if err := programs.rpc(ctx, "eth_estimateGas", []any{estimate, "pending"}, &tx.Gas); err != nil {
		return ProgramTransaction{}, err
	}
	if !programQuantity(tx.Gas, 64) || tx.Gas == "0x0" {
		return ProgramTransaction{}, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	return tx, nil
}

func programQuantity128(value Uint128) string {
	n, _ := new(big.Int).SetString(value.String(), 10)
	return "0x" + n.Text(16)
}

func programQuantity(value string, bits int) bool {
	if !strings.HasPrefix(value, "0x") || len(value) < 3 || (len(value) > 3 && value[2] == '0') {
		return false
	}
	for _, c := range value[2:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	n, ok := new(big.Int).SetString(value[2:], 16)
	return ok && n.BitLen() <= bits
}

func programABIWord(value uint64) []byte {
	out := make([]byte, 32)
	binary.BigEndian.PutUint64(out[24:], value)
	return out
}

func programABIFee(value Uint128) []byte {
	out := make([]byte, 32)
	binary.BigEndian.PutUint64(out[16:], value.High())
	binary.BigEndian.PutUint64(out[24:], value.Low())
	return out
}

func programABI(selector string, words ...[]byte) []byte {
	out, _ := hex.DecodeString(selector)
	for _, word := range words {
		out = append(out, word...)
	}
	return out
}

func programABIDynamic(selector string, head [][]byte, positions []int, values [][]byte) []byte {
	tail := []byte{}
	for i, position := range positions {
		head[position] = programABIWord(uint64(32*len(head) + len(tail)))
		tail = append(tail, programABIWord(uint64(len(values[i])))...)
		tail = append(tail, values[i]...)
		tail = append(tail, make([]byte, (32-len(values[i])%32)%32)...)
	}
	return append(programABI(selector, head...), tail...)
}

func programABIUint(data []byte, offset, bits int) (uint64, error) {
	if offset < 0 || offset > len(data)-32 {
		return 0, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	n := new(big.Int).SetBytes(data[offset : offset+32])
	if n.BitLen() > bits {
		return 0, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	return n.Uint64(), nil
}

func programABIBytes(data []byte, offset, headSize int) ([]byte, error) {
	start, err := programABIUint(data, offset, 64)
	if err != nil || start != uint64(headSize) || start > uint64(len(data)) {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	length, err := programABIUint(data, int(start), 64)
	if err != nil || length > MaximumProgramCalldataBytes || start+32+(length+31)/32*32 != uint64(len(data)) {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	body := data[int(start)+32 : int(start)+32+int(length)]
	padding := data[int(start)+32+int(length):]
	if !bytes.Equal(padding, make([]byte, len(padding))) {
		return nil, newSDKError(ErrorDecodeFailure, RetryNever)
	}
	return append([]byte{}, body...), nil
}
