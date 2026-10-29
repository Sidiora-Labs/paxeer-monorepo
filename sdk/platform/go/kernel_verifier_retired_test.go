package layerx

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"reflect"
	"testing"
)

func TestVerificationRungsAreReceiptedAndCommittedOnly(t *testing.T) {
	want := []VerificationLevel{"receipted", "committed"}
	if got := VerificationLevels(); !reflect.DeepEqual(got, want) {
		t.Fatalf("verification rungs = %q, want %q", got, want)
	}
	for _, level := range want {
		if !level.Valid() {
			t.Fatalf("verification rung %q is invalid", level)
		}
	}
	for _, level := range []VerificationLevel{
		"", "submitted", "executed", "batched", "finalised",
		"sequencer-signed", "batch-included", "state-proven",
		"checkpoint-finalised", "settlement-anchored", "unknown",
	} {
		if level.Valid() {
			t.Fatalf("retired or unknown verification rung %q is valid", level)
		}
	}
}

func TestKernelReceiptBytesAreRefusedBySequencerVerifier(t *testing.T) {
	// Protocol-v3 SEND receipt from the kernel codec conformance vector.
	const kernelReceiptHex = "0003520100030000002001010101010101010101010101010101010101010101010101010101010101010000000000000009000000200202020202020202020202020202020202020202020202020202020202020202000000200303030303030303030303030303030303030303030303030303030303030303000000200808080808080808080808080808080808080808080808080808080808080808000000000000000000000000000000000000000000000001000000200404040404040404040404040404040404040404040404040404040404040404000100000001000000010100000020050505050505050505050505050505050505050505050505050505050505050500000000000000000000000000000019000000200606060606060606060606060606060606060606060606060606060606060606000000000000000000000000000000640000000000000000000000000000004b00000000000000010000002007070707070707070707070707070707070707070707070707070707070707070000000000000000000000000000000a00000000000000000000000000000023000000200909090909090909090909090909090909090909090909090909090909090909000000200a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a000000200b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b00000000000003e80100000040d933bba9e0fed523766e49289851174abbce76403c16c574f0a516553d105e34c61ca73bd7cb501d2b6d106c43841c5c6d737733cfdd8e885a3d3e308a634c0a"
	const kernelPublicKeyHex = "ed4928c628d1c2c6eae90338905995612959273a5c63f93636c14614ac8737d1"
	canonical := fixtureBytes(t, kernelReceiptHex)
	decoded, err := decodeProtocolReceipt(canonical)
	if err != nil {
		t.Fatalf("decode canonical kernel receipt: %v", err)
	}
	if decoded.protocolVersion != 3 || decoded.protocol.ModuleID != 1 || decoded.protocol.Operation != 1 ||
		!bytes.Equal(canonical, encodeSequencerReceipt(decoded.protocol, true)) {
		t.Fatal("kernel receipt is not a canonical protocol-v3 SEND receipt")
	}
	digest := domainDigest(receiptDomain, decoded.unsigned)
	if !ed25519.Verify(fixtureBytes(t, kernelPublicKeyHex), digest[:], decoded.signature[:]) {
		t.Fatal("kernel receipt signature is invalid")
	}

	c := sequencerReceiptCases(t)[0]
	if c.Vector != "V1" {
		t.Fatalf("sequencer control vector = %q, want V1", c.Vector)
	}
	raw := sequencerFixtureBytes(t, c.RawTransaction)
	keys := sequencerFixtureKeys(t, c)
	if verified, err := VerifySequencerReceipt(c.ReceiptJSON, raw, keys); err != nil || verified.Rung != "receipted" {
		t.Fatalf("sequencer receipt control: rung %q, error %v", verified.Rung, err)
	}
	verified, err := VerifySequencerReceipt(canonical, raw, keys)
	var refusal *VerificationError
	if !errors.As(err, &refusal) || !errors.Is(err, ErrVerification) || refusal.Check != ReceiptCheckDecode {
		t.Fatalf("want kernel receipt refusal at decode, got %v (%+v)", err, refusal)
	}
	if !reflect.DeepEqual(verified, SequencerReceiptVerification{}) {
		t.Fatal("kernel receipt refusal returned verified evidence")
	}
}
