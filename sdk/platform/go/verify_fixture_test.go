package layerx

import (
	"encoding/hex"
	"testing"
)

const programOutcomeV3Vector = "505247330100000000000100010000000700000001000000000000000b000000000000000c000000000000000d000000000000000e00000001000000000000000f0000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000020000000000000003000000000000000400000000000000050000000000000006000000000000000700000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000201111111111111111111111111111111111111111111111111111111111111111000000202222222222222222222222222222222222222222222222222222222222222222000000200000000000000000000000000000000000000000000000000000000000000000"

func TestProgramOutcomeV3Vector(t *testing.T) {
	encoded := fixtureBytes(t, programOutcomeV3Vector)
	outcome, err := DecodeProgramReceiptOutcome(encoded, 1)
	if err != nil {
		t.Fatalf("decode Programs outcome: %v", err)
	}
	if outcome.EncodingVersion != 3 || outcome.ABIVersion != 1 || outcome.FeeUnits != NewUint128(0, 16) {
		t.Fatalf("Programs outcome scalar fields diverged")
	}
	var callGraphRoot, terminalPayloadRoot [32]byte
	for index := range callGraphRoot {
		callGraphRoot[index] = 0x11
		terminalPayloadRoot[index] = 0x22
	}
	if outcome.CallGraphRoot != callGraphRoot || outcome.TerminalPayloadRoot != terminalPayloadRoot {
		t.Fatalf("Programs outcome roots diverged")
	}
}

type receiptFixtureAuthority struct {
	BatchIDHex            string `json:"batch_id_hex"`
	AssetHex              string `json:"asset_hex"`
	PreviousStateRootHex  string `json:"previous_state_root_hex"`
	ResultingStateRootHex string `json:"resulting_state_root_hex"`
	SequencerPublicKeyHex string `json:"sequencer_public_key_hex"`
}

func fixtureBytes(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	return decoded
}

func fixture32(t *testing.T, value string) [32]byte {
	t.Helper()
	decoded := fixtureBytes(t, value)
	if len(decoded) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(decoded))
	}
	var out [32]byte
	copy(out[:], decoded)
	return out
}
