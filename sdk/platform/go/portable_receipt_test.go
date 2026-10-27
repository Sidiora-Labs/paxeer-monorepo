package layerx

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestPortableReceiptRoundTripsAtReceipted(t *testing.T) {
	count := 0
	for _, c := range sequencerReceiptCases(t) {
		if c.Vector != "V1" {
			continue
		}
		count++
		t.Run(c.ID, func(t *testing.T) {
			keys := sequencerFixtureKeys(t, c)
			p, err := ExportPortableReceipt(c.ReceiptJSON, sequencerFixtureBytes(t, c.RawTransaction), keys, nil)
			if err != nil {
				t.Fatal(err)
			}
			if p.Format != "layerx-receipt-proof-v2" || p.VerificationLevel != "receipted" || p.RawTransaction != c.RawTransaction || p.SignedReceipt != c.SignedReceipt || p.ReceiptDigest != c.ReceiptDigest || p.ReceiptKey != portableKey(keys.Pins[0]) {
				t.Fatal("export lost receipt evidence")
			}
			data, err := p.Export()
			if err != nil {
				t.Fatal(err)
			}
			verified, err := VerifyPortableReceipt(data, keys)
			if err != nil {
				t.Fatal(err)
			}
			if verified.Rung != "receipted" || verified.Level != "receipted" || verified.Status != 1 || verified.Revert != nil || !bytes.Equal(verified.CanonicalBytes, sequencerFixtureBytes(t, c.SignedReceipt)) || !bytes.Equal(verified.ReceiptDigest[:], sequencerFixtureBytes(t, c.ReceiptDigest)) {
				t.Fatal("portable verification lost authenticated receipt")
			}
			var restored PortableReceipt
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			reexported, err := restored.Export()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, reexported) {
				t.Fatal("portable receipt did not re-export byte-identically")
			}
			if _, err := restored.Verify(keys); err != nil {
				t.Fatal(err)
			}
			rebuilt, err := ExportPortableReceipt(restored.ReceiptJSON, sequencerFixtureBytes(t, restored.RawTransaction), keys, nil)
			if err != nil {
				t.Fatal(err)
			}
			rebuiltBytes, err := rebuilt.Export()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, rebuiltBytes) {
				t.Fatal("verified receipt did not export identically")
			}
		})
	}
	if count != 4 {
		t.Fatalf("executed %d V1 cases, want 4", count)
	}
	t.Run("committed", func(t *testing.T) {
		c := sequencerReceiptCases(t)[0]
		f := newCommittedFixture(t, c.ID, [4]int64{1, 1, 1, 1}, 0)
		evidence := PortableCommittedEvidence{BlockHeader: f.header, ReceiptInclusion: f.inclusion, Commit: f.commit, ValidatorSet: f.validators}
		p, err := ExportPortableReceipt(c.ReceiptJSON, sequencerFixtureBytes(t, c.RawTransaction), sequencerFixtureKeys(t, c), &evidence)
		if err != nil {
			t.Fatal(err)
		}
		data, err := p.Export()
		if err != nil {
			t.Fatal(err)
		}
		verified, err := VerifyPortableReceipt(data, sequencerFixtureKeys(t, c))
		if err != nil {
			t.Fatal(err)
		}
		if verified.Rung != "committed" || verified.Level != "committed" || verified.ReceiptDigest != f.receipt.ReceiptDigest {
			t.Fatal("committed evidence was not verified")
		}
		var restored PortableReceipt
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		again, err := restored.Export()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, again) {
			t.Fatal("committed bundle changed on re-export")
		}
		restored.Commit.Signatures[0].Signature[0] ^= 1
		if result, err := restored.Verify(sequencerFixtureKeys(t, c)); !errors.Is(err, ErrVerification) || result.Rung != "" {
			t.Fatal("tampered commit was accepted")
		}
	})
}

func TestPortableReceiptRefusesVersionOneFormat(t *testing.T) {
	c := sequencerReceiptCases(t)[0]
	keys := sequencerFixtureKeys(t, c)
	p, err := ExportPortableReceipt(c.ReceiptJSON, sequencerFixtureBytes(t, c.RawTransaction), keys, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := p.Export()
	if err != nil {
		t.Fatal(err)
	}
	refuse := func(t *testing.T, data []byte, pins ReceiptKeys) {
		t.Helper()
		result, err := VerifyPortableReceipt(data, pins)
		if !errors.Is(err, ErrVerification) || result.Rung != "" || len(result.CanonicalBytes) != 0 {
			t.Fatalf("expected verification refusal, got %+v, %v", result, err)
		}
	}
	mutate := func(t *testing.T, change func(map[string]json.RawMessage)) []byte {
		t.Helper()
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		change(fields)
		out, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	t.Run("v1", func(t *testing.T) {
		refuse(t, mutate(t, func(fields map[string]json.RawMessage) {
			fields["format"] = json.RawMessage(`"layerx-receipt-proof-v1"`)
		}), keys)
	})
	for _, field := range []string{"batchId", "asset", "previousStateRoot", "resultingStateRoot"} {
		for _, location := range []string{"bundle", "receiptKey", "receiptJson"} {
			t.Run(field+"/"+location, func(t *testing.T) {
				refuse(t, mutate(t, func(fields map[string]json.RawMessage) {
					if location == "bundle" {
						fields[field] = json.RawMessage(`null`)
						return
					}
					var nested map[string]json.RawMessage
					if err := json.Unmarshal(fields[location], &nested); err != nil {
						t.Fatal(err)
					}
					nested[field] = json.RawMessage(`null`)
					encoded, err := json.Marshal(nested)
					if err != nil {
						t.Fatal(err)
					}
					fields[location] = encoded
				}), keys)
			})
		}
	}
	for _, mutation := range []string{"transaction", "signature", "digest", "key", "key-range", "receipt-view", "committed-without-evidence", "orphan-commit", "missing-transaction", "unknown-field"} {
		t.Run(mutation, func(t *testing.T) {
			refuse(t, mutate(t, func(fields map[string]json.RawMessage) {
				switch mutation {
				case "transaction":
					fields["rawTransaction"] = json.RawMessage(`"0x01"`)
				case "signature":
					var view sequencerReceiptJSON
					if err := json.Unmarshal(fields["receiptJson"], &view); err != nil {
						t.Fatal(err)
					}
					signed := sequencerFixtureBytes(t, c.SignedReceipt)
					signed[len(signed)-1] ^= 1
					view.Receipt = portableHex(signed)
					fields["signedReceipt"], err = json.Marshal(view.Receipt)
					if err != nil {
						t.Fatal(err)
					}
					fields["receiptJson"], err = json.Marshal(view)
					if err != nil {
						t.Fatal(err)
					}
				case "digest":
					fields["receiptDigest"], err = json.Marshal(portableHex(make([]byte, 32)))
				case "key", "key-range":
					key := p.ReceiptKey
					if mutation == "key" {
						key.PublicKey = portableHex(make([]byte, 32))
					} else {
						key.LastHeight = "0xffff"
					}
					fields["receiptKey"], err = json.Marshal(key)
				case "receipt-view":
					var view sequencerReceiptJSON
					if err := json.Unmarshal(fields["receiptJson"], &view); err != nil {
						t.Fatal(err)
					}
					view.GasUsed = "0x1"
					fields["receiptJson"], err = json.Marshal(view)
				case "committed-without-evidence":
					fields["verificationLevel"] = json.RawMessage(`"committed"`)
				case "orphan-commit":
					fields["commit"] = json.RawMessage(`null`)
				case "missing-transaction":
					delete(fields, "rawTransaction")
				case "unknown-field":
					fields["unexpected"] = json.RawMessage(`true`)
				}
				if err != nil {
					t.Fatal(err)
				}
			}), keys)
		})
	}
	t.Run("unpinned", func(t *testing.T) { refuse(t, data, ReceiptKeys{PreviousSequenceHash: keys.PreviousSequenceHash}) })
	t.Run("predecessor", func(t *testing.T) { changed := keys; changed.PreviousSequenceHash[0] ^= 1; refuse(t, data, changed) })
	t.Run("duplicate-field", func(t *testing.T) {
		refuse(t, append([]byte(`{"format":"layerx-receipt-proof-v2",`), data[1:]...), keys)
	})
	t.Run("trailing-document", func(t *testing.T) { refuse(t, append(append([]byte(nil), data...), []byte(` {}`)...), keys) })
}
