package ed25519

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zip215CaseVectorV1 struct {
	Name            string `json:"name"`
	PublicKey       string `json:"public_key"`
	Message         string `json:"message"`
	Signature       string `json:"signature"`
	VerifySignature bool   `json:"verify_signature"`
}

func TestZip215VectorV1(t *testing.T) {
	identity := "01" + strings.Repeat("00", 31)
	orderFour := strings.Repeat("00", 32)
	nonCanonicalIdentity := "ee" + strings.Repeat("ff", 30) + "7f"
	nonCanonicalOrderFour := "ed" + strings.Repeat("ff", 30) + "7f"
	negativeZero := "01" + strings.Repeat("00", 30) + "80"
	zeroScalar := strings.Repeat("00", 32)
	oneScalar := "01" + strings.Repeat("00", 31)
	groupOrder := "edd3f55c1a631258d69cf7a2def9de14" + strings.Repeat("00", 15) + "10"
	basepoint := "58" + strings.Repeat("66", 31)
	message := []byte("ZIP-215 vector v1")
	key := TestSecretKey([]byte("zip215-vector-v1"))
	validSignature := key.Sign(message)
	cases := []zip215CaseVectorV1{
		{"small_order_identity", identity, hex.EncodeToString(message), identity + zeroScalar, true},
		{"small_order_four", orderFour, hex.EncodeToString(message), orderFour + zeroScalar, true},
		{"non_canonical_public_key", nonCanonicalIdentity, hex.EncodeToString(message), identity + zeroScalar, true},
		{"non_canonical_r", identity, hex.EncodeToString(message), nonCanonicalIdentity + zeroScalar, true},
		{"non_canonical_both", nonCanonicalIdentity, hex.EncodeToString(message), nonCanonicalIdentity + zeroScalar, true},
		{"non_canonical_order_four", nonCanonicalOrderFour, hex.EncodeToString(message), nonCanonicalOrderFour + zeroScalar, true},
		{"negative_zero_public_key", negativeZero, hex.EncodeToString(message), identity + zeroScalar, true},
		{"negative_zero_r", identity, hex.EncodeToString(message), negativeZero + zeroScalar, true},
		{"non_canonical_scalar", identity, hex.EncodeToString(message), identity + groupOrder, false},
		{"small_order_wrong_scalar", identity, hex.EncodeToString(message), identity + oneScalar, false},
		{"small_order_wrong_r", identity, hex.EncodeToString(message), basepoint + zeroScalar, false},
		{"ordinary_signature", hex.EncodeToString(key.Public().Bytes()), hex.EncodeToString(message), hex.EncodeToString(validSignature.Bytes()), true},
		{"ordinary_changed_message", hex.EncodeToString(key.Public().Bytes()), hex.EncodeToString([]byte("ZIP-215 vector v2")), hex.EncodeToString(validSignature.Bytes()), false},
	}
	for i := range cases {
		c := &cases[i]
		rawKey, err := hex.DecodeString(c.PublicKey)
		if err != nil {
			t.Fatal(err)
		}
		pub, err := PublicKeyFromBytes(rawKey)
		if err != nil {
			t.Fatal(err)
		}
		msg, err := hex.DecodeString(c.Message)
		if err != nil {
			t.Fatal(err)
		}
		rawSig, err := hex.DecodeString(c.Signature)
		if err != nil {
			t.Fatal(err)
		}
		sig, err := SignatureFromBytes(rawSig)
		if err != nil {
			t.Fatal(err)
		}
		verified := pub.Verify(msg, sig) == nil
		if verified != c.VerifySignature {
			t.Fatalf("%s: Verify = %v, want %v", c.Name, verified, c.VerifySignature)
		}
		c.VerifySignature = verified
	}
	vector := struct {
		Version       int                  `json:"version"`
		BytesEncoding string               `json:"bytes_encoding"`
		Verification  string               `json:"verification"`
		Cases         []zip215CaseVectorV1 `json:"cases"`
	}{1, "hex", "ZIP-215", cases}
	computed, err := json.MarshalIndent(vector, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	computed = append(computed, '\n')
	path := filepath.Join("testdata", "zip215_v1.json")
	if os.Getenv("LXP_WRITE_VECTORS") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, computed, 0644); err != nil {
			t.Fatal(err)
		}
		return
	}
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(computed, committed) {
		t.Fatalf("%s differs from computed vector\ncomputed:\n%s\ncommitted:\n%s", path, computed, committed)
	}
}
