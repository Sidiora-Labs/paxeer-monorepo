package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func TestVectorV3EveryRefusalIsRefused(t *testing.T) {
	var fixture v3Vector
	if err := readJSON("vectors/vector-v3-receipt-refusals.json", &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Name != "vector-v3-receipt-refusals" || len(fixture.Provenance) == 0 {
		t.Fatal("V3 must identify the vector and its source")
	}
	expected := map[string]error{
		"key-id-not-pinned":           seqkeeper.ErrReceiptKeyNotPinned,
		"height-outside-range":        seqkeeper.ErrReceiptKeyNotPinned,
		"activity-id-not-raw-hash":    errV3Activity,
		"sequence-hash-mismatch":      errV3Sequence,
		"tampered-log":                evmrpc.ErrSequencerReceiptMismatch,
		"non-canonical-receipt-bytes": codec.ErrNonCanonical,
	}
	check := func(t *testing.T, vector *v3Vector) {
		t.Helper()
		if err := verifyV3Receipt(vector, &vector.Base); err != nil {
			t.Fatalf("positive V1 baseline must verify: %v", err)
		}
		base, err := codec.DecodeReceipt(vector.Base.Receipt)
		if err != nil {
			t.Fatal(err)
		}
		if len(vector.Cases) != len(expected) {
			t.Fatalf("want six refusals, got %d", len(vector.Cases))
		}
		seen := make(map[string]bool)
		for _, entry := range vector.Cases {
			want, ok := expected[entry.Name]
			if !ok || seen[entry.Name] {
				t.Fatalf("unexpected or repeated mutation %q", entry.Name)
			}
			seen[entry.Name] = true
			t.Run(entry.Name, func(t *testing.T) {
				if entry.Expected != want.Error() {
					t.Fatalf("wrong expected refusal: %q", entry.Expected)
				}
				if err := verifyV3Receipt(vector, &entry.ReceiptJSON); !errors.Is(err, want) {
					t.Fatalf("want %v, got %v", want, err)
				}
				_, signatureErr := verify.ReceiptSignature(entry.ReceiptJSON.Receipt, [32]byte(vector.Base.SigningKey))
				if entry.Name == "non-canonical-receipt-bytes" {
					if !errors.Is(signatureErr, codec.ErrNonCanonical) {
						t.Fatalf("canonical decoder must refuse: %v", signatureErr)
					}
				} else if signatureErr != nil {
					t.Fatalf("refusal must not be caused by a broken signature: %v", signatureErr)
				}
				data, err := json.Marshal(entry.ReceiptJSON)
				if err != nil {
					t.Fatal(err)
				}
				var restored evmrpc.SequencerReceiptResult
				if err := json.Unmarshal(data, &restored); err != nil {
					t.Fatal(err)
				}
				body := base.UnsignedBytes()
				got := bytes.Clone(entry.ReceiptJSON.Receipt)
				switch entry.Name {
				case "key-id-not-pinned":
					mutated, err := codec.DecodeReceipt(got)
					if err != nil {
						t.Fatal(err)
					}
					if mutated.ContextHash == base.ContextHash {
						t.Fatal("key id was not mutated")
					}
					for _, pin := range vector.ReceiptKeys {
						if bytes.Equal(pin.KeyId, mutated.ContextHash[:]) {
							t.Fatal("mutated key id is pinned")
						}
					}
					copy(body[len(body)-41:len(body)-9], mutated.ContextHash[:])
					restored.KeyID = vector.Base.KeyID
				case "height-outside-range":
					mutated, err := codec.DecodeReceipt(got)
					if err != nil {
						t.Fatal(err)
					}
					height, err := seqkeeper.ReceiptHeight(mutated)
					if err != nil {
						t.Fatal(err)
					}
					originalHeight, err := seqkeeper.ReceiptHeight(base)
					if err != nil {
						t.Fatal(err)
					}
					pin, err := vector.ReceiptKeys.KeyForHeight(base.ContextHash, originalHeight)
					if err != nil {
						t.Fatal(err)
					}
					if height != pin.LastHeight+1 {
						t.Fatal("height must be just beyond the inclusive key range")
					}
					offset := 166 + 16 + 4
					for _, effect := range base.Effects {
						offset += 48 + len(effect.Body)
					}
					binary.BigEndian.PutUint64(body[offset+24:offset+32], height)
					restored.Height = vector.Base.Height
				case "activity-id-not-raw-hash":
					body[10] ^= 1
					restored.TxHash = vector.Base.TxHash
				case "sequence-hash-mismatch":
					body[90] ^= 1
				case "tampered-log":
					restored.Logs[0].Data[0] ^= 1
				case "non-canonical-receipt-bytes":
					got[len(got)-69] = 1
					if !bytes.Equal(got, vector.Base.Receipt) {
						t.Fatal("non-canonical mutation must only alter the presence flag")
					}
				}
				decoded, err := codec.DecodeReceipt(got)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(decoded.UnsignedBytes(), body) {
					t.Fatal("receipt has changes beyond the named mutation")
				}
				restored.Receipt = bytes.Clone(vector.Base.Receipt)
				if !reflect.DeepEqual(restored, vector.Base) {
					t.Fatal("JSON has changes beyond the named mutation")
				}
				if err := verifyV3Receipt(vector, &restored); err != nil {
					t.Fatalf("restoring the one mutation must restore acceptance: %v", err)
				}
			})
		}
	}
	t.Run("recorded", func(t *testing.T) { check(t, &fixture) })
	if vectors["vector-v3-receipt-refusals"] == nil {
		t.Fatal("V3 recorder is not registered")
	}
	paxd, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("COMMITTED_FIXTURE_EXEC_PAXD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var previousKey []byte
	for run := 0; run < 2; run++ {
		n, err := prepareHome(ctx, paxd)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(n.home) })
		private, err := v3ReceiptPrivateKey(n)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(previousKey, n.receiptPublicKey) {
			t.Fatal("recorder reused its per-run receipt key")
		}
		previousKey = bytes.Clone(n.receiptPublicKey)
		var current v3Vector
		encoded, err := json.Marshal(fixture)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &current); err != nil {
			t.Fatal(err)
		}
		keyID := sha256.Sum256(n.receiptPublicKey)
		current.ReceiptKeys = seqkeeper.ReceiptKeys{{KeyId: keyID[:], PublicKey: bytes.Clone(n.receiptPublicKey), FirstHeight: 1, LastHeight: 1<<63 - 1}}
		base, err := codec.DecodeReceipt(current.Base.Receipt)
		if err != nil {
			t.Fatal(err)
		}
		unsigned := base.UnsignedBytes()
		copy(unsigned[len(unsigned)-41:len(unsigned)-9], keyID[:])
		current.Base.Receipt, err = v3Sign(unsigned, private)
		if err != nil {
			t.Fatal(err)
		}
		current.Base.SigningKey = hexutil.Bytes(bytes.Clone(n.receiptPublicKey))
		current.Base.KeyID = common.Hash(keyID)
		generated, err := deriveV3(current, private)
		clear(private)
		if err != nil {
			t.Fatal(err)
		}
		t.Run("per-run-key-"+string(rune('1'+run)), func(t *testing.T) { check(t, generated) })
		if err := verifyV3Receipt(&fixture, &fixture.Base); err != nil {
			t.Fatalf("derivation changed the source fixture: %v", err)
		}
	}
}
