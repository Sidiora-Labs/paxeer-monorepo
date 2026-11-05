package layerx

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func lifecycleFixture(t *testing.T, name string) (uint16, []byte, []byte, map[string]json.RawMessage) {
	t.Helper()
	raw, err := os.ReadFile("../conformance/fixtures/native-program-" + name + "-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	var ordinal uint16
	if err := json.Unmarshal(fixture["ordinal"], &ordinal); err != nil {
		t.Fatal(err)
	}
	for field, expected := range map[string]uint16{"protocol_version": 3, "module": 9} {
		var value uint16
		if err := json.Unmarshal(fixture[field], &value); err != nil || value != expected {
			t.Fatalf("invalid %s", field)
		}
	}
	return ordinal, lifecycleFixtureBytes(t, fixture, "payload_hex"), lifecycleFixtureBytes(t, fixture, "signed_activity_hex"), fixture
}

func lifecycleFixtureBytes(t *testing.T, fixture map[string]json.RawMessage, field string) []byte {
	t.Helper()
	var value string
	if err := json.Unmarshal(fixture[field], &value); err != nil {
		t.Fatal(err)
	}
	bytes, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

func lifecycleReencode(ordinal uint16, payload []byte) ([]byte, error) {
	switch ordinal {
	case 1:
		value, err := DecodeNativeProgramDeploy(payload)
		if err != nil {
			return nil, err
		}
		return EncodeNativeProgramDeploy(value)
	case 2:
		value, err := DecodeNativeProgramUpgrade(payload)
		if err != nil {
			return nil, err
		}
		return EncodeNativeProgramUpgrade(value)
	case 7:
		value, err := DecodeNativeProgramWindDown(payload)
		if err != nil {
			return nil, err
		}
		return EncodeNativeProgramWindDown(value)
	default:
		return nil, lifecycleInvalid()
	}
}

func TestNativeLifecycleCFixtures(t *testing.T) {
	for _, name := range []string{"deploy", "upgrade", "wind-down-route", "wind-down-deprecate", "wind-down-tombstone", "wind-down-exit"} {
		t.Run(name, func(t *testing.T) {
			ordinal, payload, signed, fixture := lifecycleFixture(t, name)
			encoded, err := lifecycleReencode(ordinal, payload)
			if err != nil || !bytes.Equal(encoded, payload) {
				t.Fatalf("C wire differs: %v", err)
			}
			request, err := NewNativeProgramLifecycleRequest(ordinal, payload, signed)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(request.binding.ActivityID[:], lifecycleFixtureBytes(t, fixture, "activity_id_hex")) || !bytes.Equal(request.binding.IdempotencyKey[:], lifecycleFixtureBytes(t, fixture, "idempotency_key_hex")) {
				t.Fatal("signed request binding differs")
			}
			for length := 0; length < len(payload); length++ {
				if _, err := lifecycleReencode(ordinal, payload[:length]); err == nil {
					t.Fatalf("accepted prefix %d", length)
				}
			}
			if _, err := lifecycleReencode(ordinal, append(append([]byte{}, payload...), 0)); err == nil {
				t.Fatal("accepted trailing byte")
			}
			changedPayload := append([]byte{}, payload...)
			changedPayload[0] ^= 1
			if _, err := NewNativeProgramLifecycleRequest(ordinal, changedPayload, signed); err == nil {
				t.Fatal("accepted different payload")
			}
			for _, offset := range []int{1, 7, 17} {
				changed := append([]byte{}, signed...)
				changed[offset] ^= 1
				if _, err := NewNativeProgramLifecycleRequest(ordinal, payload, changed); err == nil {
					t.Fatalf("accepted changed protocol/type at %d", offset)
				}
			}
			unsignedRequest, unsignedErr := NewNativeProgramLifecycleRequest(ordinal, payload, nil)
			if ordinal == 1 || ordinal == 2 {
				if unsignedErr != nil {
					t.Fatalf("externally signed EVM lifecycle request rejected: %v", unsignedErr)
				}
				if !bytes.Equal(unsignedRequest.payload, payload) || len(unsignedRequest.signedActivity) != 0 || unsignedRequest.binding != (programCallBinding{}) {
					t.Fatal("unsigned EVM lifecycle request changed its payload or claimed a native activity binding")
				}
				unsignedCalldata, err := unsignedRequest.EVMCalldata()
				if err != nil {
					t.Fatal(err)
				}
				signedCalldata, err := request.EVMCalldata()
				if err != nil || !bytes.Equal(unsignedCalldata, signedCalldata) {
					t.Fatalf("native signed activity changed the EVM lifecycle calldata: %v", err)
				}
			} else if unsignedErr == nil {
				t.Fatal("accepted unsigned native wind-down")
			}
			for length := 1; length < len(signed); length++ {
				if _, err := NewNativeProgramLifecycleRequest(ordinal, payload, signed[:length]); err == nil {
					t.Fatalf("accepted signed prefix %d", length)
				}
			}
			if len(signed) < 69 || signed[len(signed)-69] != 12 || binary.BigEndian.Uint32(signed[len(signed)-68:]) != 64 {
				t.Fatal("C fixture signature framing")
			}
			unsigned := append([]byte{}, signed[:len(signed)-69]...)
			unsigned[4] = 11
			digest := domainDigest([]byte("LXP/v1/signature-preimage\x00"), unsigned)
			if !ed25519.Verify(lifecycleFixtureBytes(t, fixture, "public_key_hex"), digest[:], signed[len(signed)-64:]) {
				t.Fatal("C fixture signature rejected")
			}
		})
	}
}

func TestNativeLifecycleRefusals(t *testing.T) {
	for _, name := range []string{"deploy", "upgrade"} {
		ordinal, payload, _, _ := lifecycleFixture(t, name)
		for _, offset := range []int{35, 68, len(payload) - 1} {
			changed := append([]byte{}, payload...)
			changed[offset] ^= 1
			if _, err := lifecycleReencode(ordinal, changed); err == nil {
				t.Fatalf("accepted %s mutation %d", name, offset)
			}
		}
	}
	_, payload, _, _ := lifecycleFixture(t, "deploy")
	deploy, err := DecodeNativeProgramDeploy(payload)
	if err != nil {
		t.Fatal(err)
	}
	deploy.Interface = make([]byte, 953)
	if _, err := EncodeNativeProgramDeploy(deploy); err == nil {
		t.Fatal("oversized interface")
	}
	deploy.Interface = []byte{}
	if _, err := EncodeNativeProgramDeploy(deploy); err == nil {
		t.Fatal("empty deploy interface")
	}
	deploy.Interface = nil
	deploy.Policy = 0
	deploy.Authority[0] = 1
	if _, err := EncodeNativeProgramDeploy(deploy); err == nil {
		t.Fatal("immutable authority")
	}
	_, payload, _, _ = lifecycleFixture(t, "wind-down-route")
	route, err := DecodeNativeProgramWindDown(payload)
	if err != nil {
		t.Fatal(err)
	}
	route.Seed = make([]byte, 129)
	if _, err := EncodeNativeProgramWindDown(route); err == nil {
		t.Fatal("oversized seed")
	}
}

// TestLifecycleVerifierRejectsCallOutcomeReceipt constructs a signed CALL outcome
// bound to the retained v4 activity: no retained fixture carries a CALL receipt.
func TestLifecycleVerifierRejectsCallOutcomeReceipt(t *testing.T) {
	raw, err := os.ReadFile("../conformance/fixtures/native-program-call-v4.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for field, expected := range map[string]uint16{"protocol_version": 3, "module": 9, "ordinal": 3} {
		var value uint16
		if err := json.Unmarshal(fixture[field], &value); err != nil || value != expected {
			t.Fatalf("invalid %s in retained CALL activity", field)
		}
	}
	activity := lifecycleFixtureBytes(t, fixture, "activity_id_hex")
	if len(activity) != 32 {
		t.Fatal("invalid CALL activity ID length")
	}
	receipt := ProtocolReceipt{
		ProtocolVersion: 3, GlobalSequence: 1, ResultingStateRoot: [32]byte{1},
		ModuleID: 9, ModuleVersion: 4, ParameterVersion: 1, Timestamp: 1,
		ResultCode: 1,
	}
	copy(receipt.ActivityID[:], activity)
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	var sequencer [32]byte
	copy(sequencer[:], key.Public().(ed25519.PublicKey))
	signReceipt := func(outcome []byte) []byte {
		unsigned := encodeSequencerReceipt(receipt, false)
		unsigned = append(unsigned[:len(unsigned)-1], outcome...)
		unsigned = append(unsigned, 0)
		digest := domainDigest(receiptDomain, unsigned)
		canonical := append(unsigned[:len(unsigned)-1], 1)
		canonical = appendUint32(canonical, ed25519.SignatureSize)
		return append(canonical, ed25519.Sign(key, digest[:])...)
	}
	control := signReceipt(nil)
	decodedControl, err := decodeProtocolReceipt(control)
	if err != nil {
		t.Fatal(err)
	}
	controlDigest := domainDigest(receiptDomain, decodedControl.unsigned)
	if !ed25519.Verify(sequencer[:], controlDigest[:], decodedControl.signature[:]) {
		t.Fatal("legacy lifecycle control signature is invalid")
	}
	if verified, err := VerifyProgramLifecycleReceipt(control, receipt.ActivityID, sequencer); err == nil || verified.Level != "" {
		t.Fatalf("valid retired lifecycle receipt accepted: %+v %v", verified, err)
	}

	// A v4 revert outcome has no occupancy or transfer effects.
	outcome := appendUint32(nil, ProgramOutcomeTagV4)
	outcome = append(outcome, 2) // Reverted terminal.
	outcome = appendUint32(outcome, uint32(receipt.ResultCode))
	outcome = appendUint16(outcome, 1) // Runtime version.
	outcome = appendUint16(outcome, 4) // ABI version.
	outcome = appendUint32(outcome, 1) // Fee schedule version.
	outcome = appendUint32(outcome, 1) // Metering schedule version.
	for range [4]uint64{} {            // CPU, memory, storage read and storage write.
		outcome = appendUint64(outcome, 0)
	}
	outcome = appendUint32(outcome, 0) // Output values.
	outcome = appendUint64(outcome, 0) // Output bytes.
	for range [4]uint64{} {            // Occupancy byte-batches and fee units (u128).
		outcome = appendUint64(outcome, 0)
	}
	for range [7]uint64{} { // Fee schedule prices.
		outcome = appendUint64(outcome, 0)
	}
	appendRoot := func(root [32]byte) {
		outcome = appendUint32(outcome, uint32(len(root)))
		outcome = append(outcome, root[:]...)
	}
	for range [3]uint8{} { // Occupancy asset, evidence digest and transfer root.
		appendRoot([32]byte{})
	}
	outcome = appendUint64(outcome, 0) // Fee units (u128).
	outcome = appendUint64(outcome, 0)
	appendRoot([32]byte{1}) // Call graph root.
	appendRoot([32]byte{2}) // Terminal payload root.
	appendRoot([32]byte{})  // Transfer root.
	appendRoot([32]byte{3}) // Applied legs digest.
	receipt.Operation = 3
	canonical := signReceipt(outcome)
	decoded, err := decodeProtocolReceipt(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.protocol.Operation != 3 || decoded.protocol.ProgramOutcome == nil || decoded.protocol.ProgramOutcome.EncodingVersion != 4 || decoded.activityID != receipt.ActivityID {
		t.Fatal("constructed receipt is not a v4 CALL outcome for the retained activity")
	}
	digest := domainDigest(receiptDomain, decoded.unsigned)
	if !ed25519.Verify(sequencer[:], digest[:], decoded.signature[:]) {
		t.Fatal("constructed CALL receipt signature is invalid")
	}
	_, err = VerifyProgramLifecycleReceipt(canonical, decoded.activityID, sequencer)
	refusal, ok := err.(*VerificationError)
	if !ok || refusal.Check != ReceiptCheckReceiptShape {
		t.Fatalf("want CALL outcome refusal at receipt shape, got %v", err)
	}
}
