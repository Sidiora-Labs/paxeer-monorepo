package types

import (
	"encoding/hex"
	"testing"
)

func TestDeriveIdempotencyKeyDomainSeparated(t *testing.T) {
	var txHash [32]byte
	for i := range txHash {
		txHash[i] = byte(i)
	}
	otherTxHash := txHash
	otherTxHash[31] ^= 1
	selector := [4]byte{0x12, 0x34, 0x56, 0x78}
	otherSelector := selector
	otherSelector[3] ^= 1
	cases := []struct {
		name        string
		txHash      [32]byte
		callOrdinal uint32
		selector    [4]byte
		want        string
	}{
		{"zero inputs", [32]byte{}, 0, [4]byte{}, "ac1f094b6b0ad4ab26c59878ddfd3181f9f18735f497a5bc32aafea366c2ca81"},
		{"base", txHash, 0x01020304, selector, "76342a6426c82fd343027d8784fe6e95e5f977a3063963dd3c9d0b2e88fd8422"},
		{"transaction hash", otherTxHash, 0x01020304, selector, "cb7ca72398084d10d191d87c399980e9ef72d712341eb1af7722010d53937b2f"},
		{"call ordinal", txHash, 0x01020305, selector, "7690303c1fe40daa9f3bcf4c8d13214b504b33d7330fbab0023b21ea26e06df8"},
		{"selector", txHash, 0x01020304, otherSelector, "6a74e8e951ee43c91362945b152a2e0a7d90adeb41b0c0369c89a617a1672db5"},
		{"maximum ordinal", txHash, 0xffffffff, selector, "a4fbcff574e4cfc5c20df8a69f236f6a219fc13381db3360a855e4f01a899533"},
	}
	seen := make(map[[32]byte]string)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveIdempotencyKey(tc.txHash, tc.callOrdinal, tc.selector)
			assertDerivedDigestGolden(t, got, tc.want)
			if again := DeriveIdempotencyKey(tc.txHash, tc.callOrdinal, tc.selector); again != got {
				t.Fatalf("repeated derivation = %x, want %x", again, got)
			}
			if previous, ok := seen[got]; ok {
				t.Fatalf("key equals %q despite different inputs", previous)
			}
			seen[got] = tc.name
		})
	}
	t.Run("zero digest rehash", func(t *testing.T) {
		assertDerivedDigestGolden(t, nonzeroDerivedDigest([32]byte{}), "66687aadf862bd776c8fc18b8e9f8e20089714856ee233b3902a591d0d5f2925")
		digest := DeriveIdempotencyKey(txHash, 0x01020304, selector)
		if got := nonzeroDerivedDigest(digest); got != digest {
			t.Fatalf("nonzero digest changed: got %x, want %x", got, digest)
		}
	})
}

func TestDeriveStreamIDNonZeroAndPayerBound(t *testing.T) {
	var txHash, payer [32]byte
	for i := range txHash {
		txHash[i] = byte(i)
		payer[i] = byte(i + 32)
	}
	otherTxHash := txHash
	otherTxHash[31] ^= 1
	otherPayer := payer
	otherPayer[31] ^= 1
	cases := []struct {
		name        string
		txHash      [32]byte
		callOrdinal uint32
		payer       [32]byte
		want        string
	}{
		{"zero inputs", [32]byte{}, 0, [32]byte{}, "025db2b235276118a558e93177d4599d196a2ec61a03c10f3fed27f36de526ef"},
		{"base", txHash, 0x01020304, payer, "3a8230eb26c7be24ded0316012ffa86425e41a61f992740edfd35e6223501730"},
		{"transaction hash", otherTxHash, 0x01020304, payer, "89ffa34ca508b419b6cd3c7b092782a10262dc41bf6b40c2ad426ec932440769"},
		{"call ordinal", txHash, 0x01020305, payer, "ab3fe27840e2b3423a0bda3110372767140710e2b10ec979e4eb2ab633d00366"},
		{"payer in same call", txHash, 0x01020304, otherPayer, "ab50ffe07a52f056b782007fc7ba19186247960cfd1253bb82416c6bf7863673"},
		{"maximum ordinal", txHash, 0xffffffff, payer, "713eba82c6696e1eb6c4c4183162289678df782496f0420de3e2953b3d068060"},
	}
	seen := make(map[[32]byte]string)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveStreamID(tc.txHash, tc.callOrdinal, tc.payer)
			assertDerivedDigestGolden(t, got, tc.want)
			if again := DeriveStreamID(tc.txHash, tc.callOrdinal, tc.payer); again != got {
				t.Fatalf("repeated derivation = %x, want %x", again, got)
			}
			if previous, ok := seen[got]; ok {
				t.Fatalf("stream ID equals %q despite different inputs", previous)
			}
			seen[got] = tc.name
		})
	}
}

func assertDerivedDigestGolden(t *testing.T, got [32]byte, want string) {
	t.Helper()
	if got == ([32]byte{}) {
		t.Fatal("derived digest is zero")
	}
	if hex.EncodeToString(got[:]) != want {
		t.Fatalf("derived digest = %x, want %s", got, want)
	}
}
