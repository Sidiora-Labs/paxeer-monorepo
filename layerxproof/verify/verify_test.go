package verify_test

import (
	"errors"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
)

func load(t *testing.T, section string) []testvectors.Vector {
	t.Helper()
	fixture, err := testvectors.Load()
	if err != nil {
		t.Fatal(err)
	}
	cases := fixture[section]
	if len(cases) == 0 {
		t.Fatalf("fixture section %s is empty", section)
	}
	return cases
}

func field(t *testing.T, v testvectors.Vector, key string) []byte {
	t.Helper()
	raw, err := v.Bytes(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func array(t *testing.T, v testvectors.Vector, key string) [32]byte {
	t.Helper()
	out, err := v.Array32(key)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func signature(t *testing.T, v testvectors.Vector, key string) [64]byte {
	t.Helper()
	raw := field(t, v, key)
	if len(raw) != 64 {
		t.Fatalf("%s: %s is %d bytes", v.Name, key, len(raw))
	}
	var out [64]byte
	copy(out[:], raw)
	return out
}

func number(t *testing.T, v testvectors.Vector, key string) uint64 {
	t.Helper()
	out, err := v.Uint64(key)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEd25519MatchesRustStrictVerifier(t *testing.T) {
	for _, v := range load(t, "ed25519") {
		publicKey := array(t, v, "public_key")
		sig := signature(t, v, "signature")
		message := field(t, v, "message")
		var err error
		if _, domainSeparated := v.Fields["domain"]; domainSeparated {
			domain := codec.Domain(number(t, v, "domain"))
			digest, hashErr := codec.DomainHash(domain, message)
			if hashErr != nil {
				t.Fatalf("%s: %v", v.Name, hashErr)
			}
			if digest != array(t, v, "digest") {
				t.Fatalf("%s: domain digest differs from Rust SignatureMessage", v.Name)
			}
			err = verify.Ed25519Digest(publicKey, sig, digest)
		} else {
			err = verify.Ed25519(publicKey, sig, message)
		}
		if (err == nil) != v.Valid {
			t.Fatalf("%s: Go verdict %v, Rust valid=%v", v.Name, err, v.Valid)
		}
		if err != nil && !errors.Is(err, verify.ErrBadSignature) {
			t.Fatalf("%s: refusal %v is not ErrBadSignature", v.Name, err)
		}
	}
}

func TestReceiptSignatureMatchesRust(t *testing.T) {
	want := map[string]error{
		"bit-flipped-signature":   verify.ErrSequencerSignature,
		"bit-flipped-body":        verify.ErrSequencerSignature,
		"wrong-sequencer-key":     verify.ErrSequencerSignature,
		"missing-signature":       verify.ErrReceiptMissingSignature,
		"legacy-protocol-version": verify.ErrReceiptProtocolVersion,
	}
	for _, v := range load(t, "receipts") {
		verified, err := verify.ReceiptSignature(field(t, v, "receipt"), array(t, v, "public_key"))
		if (err == nil) != v.Valid {
			t.Fatalf("%s: Go verdict %v, Rust valid=%v", v.Name, err, v.Valid)
		}
		if v.Valid && verified.Digest != array(t, v, "digest") {
			t.Fatalf("%s: digest differs", v.Name)
		}
		if expected, ok := want[v.Name]; ok && !errors.Is(err, expected) {
			t.Fatalf("%s: got %v, want %v", v.Name, err, expected)
		}
	}
}

func TestStateProofMatchesRust(t *testing.T) {
	for _, v := range load(t, "state") {
		_, err := verify.StateProof(field(t, v, "witness"), array(t, v, "state_root"))
		if (err == nil) != v.Valid {
			t.Fatalf("%s: Go verdict %v, Rust valid=%v", v.Name, err, v.Valid)
		}
	}
}

func TestAccountProofBindsIdentityAndAsset(t *testing.T) {
	var witness testvectors.Vector
	for _, v := range load(t, "state") {
		if v.Name == "valid-account" {
			witness = v
		}
	}
	var value testvectors.Vector
	for _, v := range load(t, "accounts") {
		if v.Name == "valid-account-value" {
			value = v
		}
	}
	if witness.Name == "" || value.Name == "" {
		t.Fatal("fixture lacks the account witness")
	}
	raw, root := field(t, witness, "witness"), array(t, witness, "state_root")
	accountID, asset := array(t, value, "account_id"), array(t, value, "asset_id")
	account, err := verify.AccountProof(raw, root, accountID, &asset)
	if err != nil {
		t.Fatal(err)
	}
	if account.Balance.Lo != number(t, value, "balance") {
		t.Fatal("proven balance differs")
	}
	other := asset
	other[0] ^= 1
	if _, err := verify.AccountProof(raw, root, accountID, &other); !errors.Is(err, verify.ErrAssetIdentity) {
		t.Fatalf("foreign asset gave %v", err)
	}
	stranger := accountID
	stranger[0] ^= 1
	if _, err := verify.AccountProof(raw, root, stranger, &asset); err == nil {
		t.Fatal("foreign account accepted")
	}
	for _, v := range load(t, "state") {
		if v.Name == "valid-module-promoted" {
			if _, err := verify.AccountProof(field(t, v, "witness"), array(t, v, "state_root"),
				accountID, nil); !errors.Is(err, verify.ErrNotAccountProof) {
				t.Fatalf("module witness as account gave %v", err)
			}
		}
	}
}
